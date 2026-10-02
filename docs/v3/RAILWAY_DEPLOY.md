# QRit v3 — Railway + Cloudflare Deployment Runbook

**Date:** 3 October 2026 · **Branch:** `v3` · **Owner:** Aryan
**Audience:** Gemini (writes the files in §3–§4 during P0/P13) and Aryan (creates accounts, runs the steps, approves go/no-go).
**Read with:** `QRit_v3_Plan.md` (§3 architecture, §14 summary, Appendix A variables) and `V1_TO_V3_MIGRATION.md` (§10 data cutover, §11 rollback).

Facts about Railway and Cloudflare below were checked against their documentation on 27 Sep – 3 Oct 2026 (sources at the end). Items marked **VERIFY** depend on behaviour the docs don't spell out; check them on staging before production and write the result into `docs/v3/DECISIONS.md`.

---

## Contents

1. Target layout
2. Accounts, domains and secrets (one-time)
3. Repository files (Dockerfiles, IaC, Worker)
4. Service settings and variables
5. Database roles, pooling, backups, Redis
6. Cutover plan (staging rehearsal → production)
7. Rollback
8. After go-live
9. Ongoing operations
10. Config-as-Code deadline (1 Dec 2026)
11. Sources

---

## 1. Target layout

**One Railway project — the existing v1 project** (rename it to `qrit` if you like). v3 re-uses the v1 `backend` and `frontend` services so their Railway domains and custom domains — which are on shared and printed links — keep working. Everything else is added next to them.

| Service | Kind | Source | Root dir | Region | Replicas | Public domains |
|---|---|---|---|---|---|---|
| `backend` → becomes **api** | existing service, re-deployed with v3 | GitHub repo, branch `main` (after cutover) | `backend` | Singapore `asia-southeast1-eqsg3a` | 2 | its existing Railway domain + existing custom domains (legacy `/r/` links) + `API_DOMAIN` |
| `frontend` → becomes **web** | existing service, re-deployed with v3 | same repo | `frontend` | Singapore | 2 | its existing domains + `APP_DOMAIN` |
| `redirect` | new | same repo | `backend` | Singapore | 2 (scale with load) | Railway domain only (Cloudflare Worker is the only intended client) |
| `worker` | new | same repo | `backend` | Singapore | 1 | none |
| `ingest` | new | same repo | `backend` | Singapore | 1 | none |
| `postgres-v3` | new, Railway Postgres template, **image tag 17** | template | — | Singapore | 1 | none (private only; TCP proxy off except during the import if needed) |
| `redis` | new, Railway Redis template, 7.x | template | — | Singapore | 1 | none |
| `migrator` | temporary (T−3 days → T+30 days) | same repo | `backend` | Singapore | 1 | none |
| v1 Postgres | existing; read-only after cutover; deleted at T+30 days | — | — | unchanged | — | none |

Environments: `production` (the existing one) and `staging` (new, a duplicate of production's service list with its own databases and domains; deploys from branch `v3` until cutover, then from `main`). PR environments are optional; if used, enable "focused" deploys so only services whose root directory changed are built.

**Edge:** Cloudflare in front of all short-link traffic (`SHORT_DOMAIN`, customer short domains, white-label dashboard hosts) with the Worker `edge-router` (plan D12). The product domain (`APP_DOMAIN`, `API_DOMAIN`) can stay DNS-only or be proxied with SSL mode **Full**.

---

## 2. Accounts, domains and secrets (one-time)

### 2.1 Accounts (Aryan)

- **Railway Pro** (needed for replicas in production, 20 custom domains per service, static outbound IPs, longer deployment retention for rollback).
- **Cloudflare:** zone for `SHORT_DOMAIN` (e.g. `qrit.link`) with **Cloudflare for SaaS** enabled; **Workers Paid** recommended (request volume); an API token with `Zone:SSL and Certificates:Edit`, `Zone:Custom Hostnames:Edit`, `Workers KV Storage:Edit`, `Zone:Read` for the SaaS zone → `CF_API_TOKEN`.
- **Object storage:** Cloudflare R2 bucket `qrit-prod` (and `qrit-staging`), S3 API token → `STORAGE_*`. Optional second bucket with Object Lock for audit anchors → `AUDIT_ANCHOR_BUCKET`.
- **Resend:** verified sending domain → `RESEND_API_KEY`, `EMAIL_FROM`.
- **Razorpay** (Subscriptions + Payment Links enabled), **Stripe** (existing v1 account), **Google** OAuth client (existing v1 client id), **Google Web Risk** API key, **Cloudflare Turnstile** site + secret, **Sentry** project, **Better Stack** (or similar) for probes and the status page.

### 2.2 Domains

| Variable | Example | Points to |
|---|---|---|
| `APP_DOMAIN` | `app.qrit.in` (or keep the v1 frontend domain) | `web` service (Railway custom domain) |
| `API_DOMAIN` | `api.qrit.in` | `api` service (Railway custom domain) |
| `SHORT_DOMAIN` | `qrit.link` | Cloudflare zone; Worker route `*/*` → `redirect` |
| `SANDBOX_SHORT_DOMAIN` | `test.qrit.link` | same zone, same Worker |
| `CF_SAAS_CNAME_TARGET` | `customers.qrit.link` | proxied record in the SaaS zone; customers CNAME their domains to it |
| `LEGACY_HOSTS` | `<v1 backend>.up.railway.app,<v1 custom api domain>,…` | already attached to the v1 backend service; keep them there (see `V1_TO_V3_MIGRATION.md` §6) |

### 2.3 Secrets

Generate once per environment (never reuse staging secrets in production). Store as Railway **shared variables** in each environment and reference them from services (`${{shared.NAME}}`).

| Variable | Generate with |
|---|---|
| `DJANGO_SECRET_KEY` | `python -c "import secrets;print(secrets.token_urlsafe(64))"` |
| `APP_ENCRYPTION_KEY` | `python -c "import os,base64;print(base64.b64encode(os.urandom(32)).decode())"` (KEK root — **never change it except through `manage.py rotate_kek`**, plan §7.1) |
| `JWT_ED25519_PRIVATE_KEY` | same command (a 32-byte seed is accepted); `JWT_KEY_ID` = `prod-2026-11` |
| `SCAN_SALT_SECRET`, `EDGE_SHARED_SECRET`, `SERIAL_MAC_KEY`, `VERIFY_TOKEN_KEY`, `METRICS_TOKEN` | `python -c "import secrets;print(secrets.token_urlsafe(32))"` |
| `DB_APP_PASSWORD`, `DB_V1_READER_PASSWORD` | same |

---

## 3. Repository files

Gemini writes these in P0 (Dockerfiles, compose, CI) and P13 (IaC, Worker, runbook automation).

### 3.1 `backend/Dockerfile` (one image for api, redirect, worker, ingest, migrator)

```dockerfile
# syntax=docker/dockerfile:1.7
FROM python:3.14-slim-bookworm AS build
COPY --from=ghcr.io/astral-sh/uv:0.12 /uv /uvx /bin/
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_PYTHON_DOWNLOADS=never
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --no-install-project
COPY . .
RUN uv sync --locked --no-dev \
 && DJANGO_SETTINGS_MODULE=qrit.settings.build /app/.venv/bin/python manage.py collectstatic --noinput --verbosity 0

FROM python:3.14-slim-bookworm
# xmlsec1: pysaml2. libpq5: psycopg. postgresql-client-17 (PGDG): pg_dump/psql for ops and the v1 import rehearsal.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg xmlsec1 libpq5 \
 && install -d /usr/share/postgresql-common/pgdg \
 && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
 && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
 && apt-get update && apt-get install -y --no-install-recommends postgresql-client-17 \
 && apt-get purge -y curl gnupg && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 10001 app
COPY --from=build --chown=app:app /app /app
ENV PATH="/app/.venv/bin:$PATH" PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1
WORKDIR /app
USER app
# Default = api. Other services override the start command (§4).
CMD ["sh", "-c", "gunicorn qrit.asgi:application -k uvicorn_worker.UvicornWorker -c gunicorn.conf.py"]
```

Notes:
- `collectstatic` runs **at build time** with `qrit.settings.build` (a settings module that needs no database or secrets). Railway's pre-deploy command runs in a separate container whose file changes are discarded, so static files must be in the image.
- Do not use BuildKit cache mounts unless they follow Railway's cache-id rules; plain layers are fine.
- Image size target < 350 MB (plan P0 gate).

### 3.2 `frontend/Dockerfile`

Multi-stage Node 22 LTS (or 24 LTS) with pnpm: install → `next build` (`output: "standalone"`) → runtime copies `.next/standalone`, `.next/static` and `public`; `ENV HOSTNAME=0.0.0.0 NODE_ENV=production`; `USER node`; `CMD ["node", "server.js"]`. Declare every `NEXT_PUBLIC_*` variable as an `ARG` in the build stage so Railway passes it at build time (`NEXT_PUBLIC_GOOGLE_CLIENT_ID`, `NEXT_PUBLIC_TURNSTILE_SITE_KEY`, `NEXT_PUBLIC_SHORT_DOMAIN`, `NEXT_PUBLIC_APP_URL`, `NEXT_PUBLIC_SENTRY_DSN`).

### 3.3 `gunicorn.conf.py`

`bind = f"[::]:{os.environ.get('PORT', '8080')}"`, `workers = int(os.environ.get('WEB_CONCURRENCY', 2))`, `worker_class = 'uvicorn_worker.UvicornWorker'`, `graceful_timeout = 30`, `timeout = 60`, `keepalive = 5`; leave `forwarded_allow_ips` at its default (`127.0.0.1`) so uvicorn never rewrites the client address from a spoofable `X-Forwarded-For` — client IP comes from `core.net` (plan §7.1) and HTTPS detection from `SECURE_PROXY_SSL_HEADER`; access log off (structlog logs requests).

### 3.4 `.railway/railway.ts` (Infrastructure as Code)

Start from `railway config pull` (imports the current project, including the v1 services) and edit it into the shape below. The API is `defineRailway`, `project`, `service`, `postgres`, `redis`, `preserve` from `railway/iac`; `ctx.environment` distinguishes environments. **VERIFY** every option against `railway config plan` output before applying — this is a sketch, not tested code.

```ts
import { defineRailway, project, service, postgres, redis, github, preserve } from "railway/iac";

export default defineRailway((ctx) => {
  const prod = ctx.environment === "production";
  const branch = prod ? "main" : "v3";
  const repo = "its-aryansingh/qrit"; // VERIFY repository name

  const db = postgres("postgres-v3");     // pin image tag 17 in the dashboard if IaC can't (VERIFY)
  const cache = redis("redis");

  // App-role URL built from the Postgres service's direct host (VERIFY variable names). When PgBouncer is enabled
  // later, api/redirect switch to a pooled URL built from the pooler's host (RAILWAY_DEPLOY §5.2).
  const appUrl = `postgresql://qrit_app:${ctx.shared.DB_APP_PASSWORD}@${db.env.PGHOST}:${db.env.PGPORT}/${db.env.PGDATABASE}`;

  const py = (name: string, start: string, extra: Record<string, unknown> = {}) =>
    service(name, {
      source: github(repo, { branch, rootDirectory: "backend" }),   // VERIFY rootDirectory placement
      start,
      replicas: { "asia-southeast1-eqsg3a": (extra.replicas as number) ?? 1 },
      env: {
        APP_ENV: prod ? "production" : "staging",
        DJANGO_SECRET_KEY: ctx.shared.DJANGO_SECRET_KEY,
        APP_ENCRYPTION_KEY: ctx.shared.APP_ENCRYPTION_KEY,
        REDIS_URL: cache.env.REDIS_URL,
        // api/redirect read DATABASE_URL; worker/ingest/migrator read DATABASE_UNPOOLED_URL (same value until PgBouncer).
        DATABASE_URL: appUrl,
        DATABASE_UNPOOLED_URL: appUrl,
        SENTRY_DSN: ctx.shared.SENTRY_DSN,
        ...(extra.env as Record<string, unknown> ?? {}),
      },
      ...(extra.service as object ?? {}),
    });

  const api = py("backend", "gunicorn qrit.asgi:application -k uvicorn_worker.UvicornWorker -c gunicorn.conf.py", {
    replicas: 2,
    env: { DJANGO_SETTINGS_MODULE: "qrit.settings.api", DATABASE_OWNER_URL: db.env.DATABASE_URL /* direct while PgBouncer is off; switch to db.env.DATABASE_UNPOOLED_URL when it is on */, DB_APP_PASSWORD: ctx.shared.DB_APP_PASSWORD },
    service: {
      preDeploy: "python manage.py migrate --database owner --noinput && python manage.py ensure_db_roles --database owner && python manage.py ensure_redis_config",
      healthcheck: "/readyz", healthcheckTimeout: 120,
      domains: prod ? ["api.qrit.in"] : ["api.staging.qrit.in"],   // existing v1 domains stay attached (VERIFY apply doesn't remove them; use preserve() if needed)
    },
  });
  const redirect = py("redirect", "granian --interface asginl --host :: --port $PORT --workers $WEB_CONCURRENCY qrit.asgi:application", {
    replicas: 2, env: { DJANGO_SETTINGS_MODULE: "qrit.settings.redirect" },
    service: { healthcheck: "/readyz", healthcheckTimeout: 120 },
  });
  py("worker", "python manage.py wait_for_migrations && python manage.py procrastinate worker --concurrency=$WORKER_CONCURRENCY",
     { env: { DJANGO_SETTINGS_MODULE: "qrit.settings.worker" } });
  py("ingest", "python manage.py wait_for_migrations && python manage.py ingest_scans",
     { env: { DJANGO_SETTINGS_MODULE: "qrit.settings.worker" } });

  service("frontend", {
    source: github(repo, { branch, rootDirectory: "frontend" }),
    replicas: { "asia-southeast1-eqsg3a": 2 },
    healthcheck: "/api/health",
    env: { API_INTERNAL_URL: `http://${api.env.RAILWAY_PRIVATE_DOMAIN}:8080`, EDGE_SHARED_SECRET: ctx.shared.EDGE_SHARED_SECRET /* … */ },
    domains: prod ? ["app.qrit.in"] : ["app.staging.qrit.in"],
  });

  return project("qrit", { services: [db, cache] });   // VERIFY how services are registered with project()
});
```

Settings the IaC reference does not document (set them in the dashboard and list them in `DECISIONS.md`): **restart policy** (`ON_FAILURE`, 10 retries) for every service; **watch paths** (`backend/**` for Python services, `frontend/**` for web); **Dockerfile builder** (automatic when a Dockerfile exists in the root directory); **static outbound IPs** for `worker` (lets customers allowlist webhook/SIEM source IPs); **Postgres image tag** 17 if not expressible; **serverless/sleep off** for every service; **CDN caching off** for `api` and `redirect`.

CI: the `railwayapp/config@v1` GitHub Action posts `railway config plan` on PRs and applies on merge (needs `RAILWAY_TOKEN`, a project token). Until cutover, apply manually; production `apply` is part of the cutover script (§6.3).

### 3.5 `edge/worker` (Cloudflare Worker `edge-router`)

`wrangler.toml`: `name = "edge-router"`, `main = "src/index.ts"`, `compatibility_date` = the date of writing, routes `*/*` on the SaaS zone (set in the dashboard or `routes = [{ pattern = "*/*", zone_name = "qrit.link" }]`), KV binding `APP_HOSTS`, variables `REDIRECT_ORIGIN`, `WEB_ORIGIN`, secret `EDGE_SHARED_SECRET` (`wrangler secret put`). Separate `[env.staging]`.

```ts
export interface Env { REDIRECT_ORIGIN: string; WEB_ORIGIN: string; EDGE_SHARED_SECRET: string; APP_HOSTS: KVNamespace }

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const url = new URL(req.url);
    const host = url.hostname.toLowerCase();
    const isAppHost = (await env.APP_HOSTS.get(host)) !== null;          // white-label dashboard hosts
    const target = new URL(url.pathname + url.search, isAppHost ? env.WEB_ORIGIN : env.REDIRECT_ORIGIN);

    const headers = new Headers();
    for (const [k, v] of req.headers) if (!k.toLowerCase().startsWith("x-qrit-")) headers.set(k, v); // drop spoofed edge headers
    const cf = (req as any).cf ?? {};
    const enc = (v: unknown) => encodeURIComponent(String(v ?? ""));         // header values must be ASCII
    headers.set("x-qrit-host", host);
    headers.set("x-qrit-client-ip", req.headers.get("cf-connecting-ip") ?? "");
    headers.set("x-qrit-geo-country", String(cf.country ?? ""));
    headers.set("x-qrit-geo-region", enc(cf.regionCode));
    headers.set("x-qrit-geo-city", enc(cf.city));
    headers.set("x-qrit-geo-asn", String(cf.asn ?? ""));
    headers.set("x-qrit-geo-timezone", String(cf.timezone ?? ""));
    headers.set("x-qrit-edge-secret", env.EDGE_SHARED_SECRET);

    const init: RequestInit = { method: req.method, headers, redirect: "manual" };
    if (req.method !== "GET" && req.method !== "HEAD") init.body = req.body;
    return fetch(target.toString(), init);                                  // pass 3xx through to the scanner
  },
};
```

The server side percent-decodes region and city, accepts the headers only when `X-QRit-Edge-Secret` matches (constant-time), and ignores `XX`/`T1` countries. The API writes `APP_HOSTS` entries (key = hostname, value = org id) through the Cloudflare API when a white-label host becomes active, and deletes them on removal. Tests: `@cloudflare/vitest-pool-workers` — header stripping, geo headers, POST body pass-through, redirects not followed, app-host routing.

---

## 4. Service settings and variables

Start commands, health checks and pre-deploy are in §3.4. Variables per service (names from plan Appendix A; `shared.` = shared variable; `Postgres`/`Redis` = references to those services):

| Variable | api | redirect | worker | ingest | web | migrator |
|---|---|---|---|---|---|---|
| `APP_ENV`, `LOG_LEVEL`, `SENTRY_DSN` | ✓ | ✓ | ✓ | ✓ | ✓ (`NEXT_PUBLIC_SENTRY_DSN`) | ✓ |
| `DJANGO_SETTINGS_MODULE` | `qrit.settings.api` | `qrit.settings.redirect` | `qrit.settings.worker` | `qrit.settings.worker` | — | `qrit.settings.worker` |
| `DJANGO_SECRET_KEY`, `APP_ENCRYPTION_KEY` | ✓ | ✓ | ✓ | ✓ | — | ✓ |
| `DATABASE_URL` (app role) | ✓ (pooled when PgBouncer is on) | ✓ (pooled when on) | — | — | — | — |
| `DATABASE_UNPOOLED_URL` (app role, direct) | — | — | ✓ | ✓ | — | ✓ |
| `DATABASE_OWNER_URL` (`postgres`, direct: the Postgres service's `DATABASE_URL` while PgBouncer is off, its `DATABASE_UNPOOLED_URL` once on) | pre-deploy only (`--database owner`) | — | — | — | — | ✓ (migrations during rehearsal) |
| `V1_DATABASE_URL` (read-only role on v1 Postgres, private network) | — | — | — | — | — | ✓ |
| `REDIS_URL` | ✓ | ✓ | ✓ | ✓ | — | ✓ |
| `ALLOWED_HOSTS` | API_DOMAIN, v1 backend domains, `healthcheck.railway.app`, `${{RAILWAY_PRIVATE_DOMAIN}}` | Railway domain, `healthcheck.railway.app` (host routing is done in the view) | — | — | — | — |
| `APP_BASE_URL`, `API_PUBLIC_URL` | ✓ | ✓ (`/v/` → web verify page, `/` fallback) | ✓ | — | `NEXT_PUBLIC_APP_URL` | ✓ |
| `SHORT_DOMAIN`, `SHORT_DOMAIN_SCHEME`, `SANDBOX_SHORT_DOMAIN`, `LEGACY_HOSTS` | ✓ | ✓ | ✓ | — | `NEXT_PUBLIC_SHORT_DOMAIN`, `LEGACY_BACKEND_HOST` (for the `/r/` 308) | ✓ |
| `EDGE_SHARED_SECRET`, `EDGE_SHARED_SECRET_PREVIOUS` | — | ✓ | — | — | ✓ | — |
| `JWT_ED25519_PRIVATE_KEY`, `JWT_KEY_ID`, `COOKIE_SECURE=true`, `COOKIE_DOMAIN` | ✓ | — | — | — | — | — |
| `SCAN_SALT_SECRET` | ✓ (legacy `/r/`) | ✓ | — | — | — | ✓ |
| `STORAGE_*` | ✓ | ✓ (hosted file pages, logos) | ✓ | — | — | ✓ |
| `AUDIT_ANCHOR_BUCKET` | — | — | ✓ | — | — | — |
| `RESEND_API_KEY`, `EMAIL_FROM` | ✓ | — | ✓ | — | — | ✓ |
| `GOOGLE_CLIENT_ID` | ✓ | — | — | — | `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | — |
| `WEB_RISK_API_KEY`, `DOH_URL` | ✓ | — | ✓ | — | — | — |
| `CF_API_TOKEN`, `CF_ZONE_ID`, `CF_SAAS_CNAME_TARGET`, `CF_KV_NAMESPACE_ID`, `CF_ACCOUNT_ID` | ✓ | — | ✓ | — | — | — |
| `TURNSTILE_SECRET_KEY` / `NEXT_PUBLIC_TURNSTILE_SITE_KEY` | ✓ | — | — | — | ✓ | — |
| `RAZORPAY_*`, `STRIPE_*` (incl. `STRIPE_LEGACY_PRICE_MAP`), `SELLER_*`, `PRICES_INCLUDE_GST` | ✓ | — | ✓ | — | — | ✓ (Stripe sync) |
| `SERIAL_MAC_KEY` | ✓ | ✓ | ✓ (`serials.generate`) | — | — | — |
| `VERIFY_TOKEN_KEY` | ✓ | ✓ | — | — | — | — |
| `STAFF_IP_ALLOWLIST`, `FREE_TIER_DAILY_LIMIT`, `CORS_ALLOWED_ORIGINS` | ✓ | — | — | — | — | — |
| `METRICS_TOKEN` | — | ✓ | ✓ | ✓ | — | — |
| `WEB_CONCURRENCY` | 2 | 2 | — | — | — | — |
| `WORKER_CONCURRENCY` | — | — | 8 | — | — | — |
| `AUTH_MAX_LOGIN_ATTEMPTS`, `JWT_PREVIOUS_PUBLIC_KEYS` | ✓ | — | — | — | — | — |
| `LEGACY_GEOIP_LOOKUP_URL` (optional, migration doc §6) | ✓ | ✓ | — | — | — | — |
| `DB_APP_PASSWORD` | ✓ (pre-deploy `ensure_db_roles`) | — | — | — | — | ✓ |
| `DB_POOL_MIN`, `DB_POOL_MAX` | 1, 4 | 1, 4 (async pool) | 1, 10 | 1, 3 | — | 1, 4 |
| `DISABLE_SERVER_SIDE_CURSORS` | `true` when pooled | `true` when pooled | — | — | — | — |
| `API_INTERNAL_URL` | — | — | — | — | `http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}` | — |

Set `PORT=8080` explicitly on api, redirect and web so private-network URLs are stable.

---

## 5. Database roles, pooling, backups, Redis

### 5.1 Roles on `postgres-v3`

- **Owner** = Railway's default `postgres` superuser. Used only by migrations (`DATABASE_OWNER_URL`, pre-deploy and the migrator). Owns every table, function and the extensions.
- **App role** `qrit_app` — `LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE`. Every running service connects as it, so Row-Level Security policies apply (plan §3.4). Created and kept in sync by `python manage.py ensure_db_roles` (idempotent; reads `DB_APP_PASSWORD`; runs as the owner in pre-deploy):

```sql
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'qrit_app') THEN
    CREATE ROLE qrit_app LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
END $$;
ALTER ROLE qrit_app PASSWORD {password};   -- composed with psycopg.sql.Literal (utility statements take no bind parameters)
GRANT CONNECT ON DATABASE {dbname} TO qrit_app;   -- psycopg.sql.Identifier
GRANT USAGE ON SCHEMA public TO qrit_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO qrit_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO qrit_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO qrit_app;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO qrit_app;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO qrit_app;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO qrit_app;
```

No `TRUNCATE`, no DDL, no `BYPASSRLS`. procrastinate's tables are covered by the default privileges (its migrations run as the owner). The only schema changes the app needs at run time — creating, attaching and dropping monthly `scan_events` partitions — go through the owner-owned `SECURITY DEFINER` functions `qrit_ensure_scan_partitions` and `qrit_drop_scan_partitions` (plan §6.1), which `qrit_app` may execute. The RLS test in CI uses an equivalent role `qrit_app_test`.

- **v1 reader** (on the **v1** Postgres, before the rehearsal):

```sql
CREATE ROLE qrit_v1_reader LOGIN PASSWORD '…';
GRANT CONNECT ON DATABASE railway TO qrit_v1_reader;   -- v1 database name: VERIFY
GRANT USAGE ON SCHEMA public TO qrit_v1_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO qrit_v1_reader;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO qrit_v1_reader;   -- pg_dump reads sequence state
ALTER ROLE qrit_v1_reader SET default_transaction_read_only = on;
```

### 5.2 Connections and PgBouncer

Start **without** PgBouncer. Budget (Postgres default `max_connections` = 100; keep usage ≤ 70):

| Consumer | Processes | Pool max | Connections |
|---|---|---|---|
| api (Django pool + async pool for legacy `/r/`) | 2 replicas × 2 workers | 4 + 2 | 24 |
| redirect | 2 replicas × 2 workers | 4 | 16 |
| worker | 1 | 10 | 10 |
| ingest | 1 | 3 | 3 |
| pre-deploy / migrator / ops | — | — | 5 |
| **Total** | | | **58** |

When replicas grow past this budget, enable Railway's built-in PgBouncer (Postgres service → Config → Connection Pooling). It changes the Postgres service's `DATABASE_URL` to the pooler and adds `DATABASE_UNPOOLED_URL` (direct). Transaction mode is the default and is incompatible with LISTEN/NOTIFY and session advisory locks — so:
- api and redirect move to the pooled host (rebuild their `qrit_app` `DATABASE_URL` from the pooler's host and port — VERIFY which variables Railway exposes for them; `DATABASE_OWNER_URL` switches to the Postgres service's `DATABASE_UNPOOLED_URL`); set `DISABLE_SERVER_SIDE_CURSORS=true`; disable psycopg auto-prepare on pooled connections (`prepare_threshold=None`) unless the pooler is confirmed to support prepared statements;
- worker (LISTEN/NOTIFY, advisory locks), ingest, migrations and the importer keep the **unpooled** URL;
- transaction-scoped features keep working through the pooler: `set_config('app.workspace_id', …, true)`, `pg_advisory_xact_lock`.
- **VERIFY** that the pooler accepts the custom `qrit_app` role (the docs don't say). If it doesn't, keep `qrit_app` direct and raise `max_connections` instead (`ALTER SYSTEM SET max_connections = 200;` + restart, if the instance has the memory).

### 5.3 Backups

- Postgres service → Backups: enable **daily, weekly and monthly** schedules on `postgres-v3` in production; enable **point-in-time recovery** if the plan offers it (VERIFY availability).
- Before the cutover window: take a **manual backup** of the v1 Postgres and of `postgres-v3`.
- Quarterly restore drill on staging: restore the latest production backup into staging's Postgres, run `verify_audit_chain` and the smoke tests; record the time taken.
- Object storage: enable R2 bucket versioning or a lifecycle copy for `qrit-prod`; the audit anchor bucket uses Object Lock.

### 5.4 Redis

- Railway Redis template, private only, password-protected (template default).
- `python manage.py ensure_redis_config` (pre-deploy and at api start): `CONFIG SET maxmemory-policy volatile-lru` (caches have TTLs and can be evicted; the `scans` stream has no TTL and is not), checks AOF persistence is on (`appendonly yes`) or logs a warning. **VERIFY** the template allows `CONFIG SET`; if not, set them through the service's start command.
- Redis holds caches, rate-limit counters, short-lived MFA/SSO state and the `scans` stream buffer. Losing it loses at most a few seconds of unprocessed scans and forces re-login only for in-flight SSO/MFA steps.

---

## 6. Cutover plan

### 6.1 Timeline overview

| When | What |
|---|---|
| T−21 days | Staging environment complete; P0–P12 merged into `v3`; Cloudflare staging zone/hosts working |
| T−14 days | **Rehearsal 1** on staging (§6.2) with a copy of production v1 data |
| T−10 days | Fix findings; **Rehearsal 2**; go/no-go criteria agreed; date fixed |
| T−7 days | `announcement`, `reprint_notice`, `api_notice` emails (migration doc §9) |
| T−3 days | Production prep (§6.3): new services and databases created, migrator deployed, roles created, v1 IaC snapshot saved |
| T−1 day | Full import into production v3 + verification (v1 still live) |
| T (night IST, ~45 min window) | Cutover (§6.4) |
| T+1 h | Rollback decision point (§7) |
| T+1 day | `completed` and `leads_notice` emails |
| T+3 days | Hypercare ends if SLO probes green for 72 h (plan P13 gate) |
| T+30 days | Delete the v1 Postgres (after a final backup) and the `migrator` service |

### 6.2 Staging rehearsal (repeat until clean)

1. Create a `v1-copy` Postgres service in staging (same major version as v1 production — VERIFY with `SELECT version()`).
2. From the staging `migrator` (`railway ssh --service migrator --environment staging --session rehearsal`): `pg_dump "$V1_PROD_PUBLIC_URL" -Fc -f /tmp/v1.dump` then `pg_restore --no-owner -d "$V1_COPY_URL" /tmp/v1.dump` and `rm /tmp/v1.dump`. (`V1_PROD_PUBLIC_URL` uses the v1 reader role through the v1 Postgres TCP proxy, enabled only for this step. The dump contains personal data — never download it to a laptop.)
3. `python manage.py migrate --database owner` and `ensure_db_roles --database owner` on staging's `postgres-v3` (or let the api pre-deploy do it).
4. `v1_inventory`, `import_v1 --dry-run`, `import_v1`, `verify_v1_import --http-base https://<staging api domain>` — record durations.
5. Simulate the window: `ALTER DATABASE … SET default_transaction_read_only = on` on `v1-copy`, change some rows first (with the flag off) to exercise the delta, run `import_v1 --since …`, `verify_v1_import`.
6. Run the smoke tests (§6.5) and the k6 redirect test against staging through Cloudflare.
7. **Practise the rollback** (§7) on staging, including Railway rollback of the re-used services — confirm what it restores (image, variables, start command, health check path, region). Write the findings into `DECISIONS.md`.

Exit: every step green, total window time ≤ 45 minutes, rollback practised.

### 6.3 Production prep (T−3 days)

1. `railway config pull > .railway/v1-snapshot.ts` on `main` — the exact v1 settings, for rollback. Also save `railway variables --service backend --json` and the same for `frontend` to a password manager (they contain secrets).
2. Create `postgres-v3` (image 17, Singapore) and `redis` in production; enable backups.
3. Create the shared variables (§2.3) for production.
4. Create the `migrator` service from branch `v3` (start command `sleep infinity`, variables per §4) — not public.
5. From the migrator: `python manage.py migrate --database owner`, `ensure_db_roles --database owner`, `ensure_redis_config`, `python manage.py check --deploy`.
6. Create the v1 reader role on v1 Postgres (§5.1).
7. Cloudflare: production Worker deployed with `REDIRECT_ORIGIN` left pointing at a maintenance page until the redirect service exists; `SHORT_DOMAIN` zone, SaaS fallback origin `AAAA 100::` (proxied), custom hostname target `CF_SAAS_CNAME_TARGET`, route `*/*` → `edge-router`.
8. Open the cutover PR `v3 → main` (do not merge). Confirm CI is green on it.
9. Lower DNS TTLs for `APP_DOMAIN`/`API_DOMAIN` if they change.

### 6.4 Cutover window (T)

| Step | Minute | Action | Check |
|---|---|---|---|
| 1 | 0 | Status page: maintenance (redirects unaffected) | — |
| 2 | 0 | v1 read-only: `ALTER DATABASE <v1 db> SET default_transaction_read_only = on;` then **restart** the v1 backend service | v1 `/r/<code>` still redirects; a dashboard save fails |
| 3 | 2 | Migrator: `import_v1 --source "$V1_DATABASE_URL" --since <T−1 run start − 10 min>` | report has no blocking items |
| 4 | 8 | `verify_v1_import` | exit 0 — **go/no-go 1** |
| 5 | 12 | Pause automatic deploys on `backend` and `frontend` (service settings; VERIFY the control's name in rehearsal), then merge the cutover PR into `main` | nothing redeploys yet; v1 still serves `/r/` |
| 6 | 13 | `railway config apply --yes` with the production v3 config (creates `redirect`, `worker`, `ingest`; updates `backend`/`frontend` settings, region and variables), then deploy `backend` and `frontend` from `main` and re-enable automatic deploys. **Order matters:** applying v3 settings while `main` still had v1 code would restart v1 with v3 settings. Railway keeps the old deployment serving until the new one passes its health check, so a failed build or health check leaves v1 live | all deployments healthy (`/readyz`, `/api/health`); api pre-deploy ran `migrate` (a no-op) and `ensure_db_roles` |
| 7 | 25 | Worker: point `REDIRECT_ORIGIN` at the production redirect service's Railway domain (`wrangler deploy --env production`) | `curl -I https://<SHORT_DOMAIN>/<test code>` → 302 |
| 8 | 27 | Smoke tests (§6.5) + `verify_v1_import --http-base https://<v1 backend host>` | all green — **go/no-go 2** |
| 9 | 40 | Status page: operational. Keep watching dashboards for 1 hour | error rate, p95, ingest lag |

If step 4 or 8 fails and can't be fixed within the window → rollback (§7).

### 6.5 Smoke tests (scripted: `deploy/smoke/cutover.sh`)

1. Legacy links: 20 sampled `https://<v1 backend host>/r/<code>` (incl. password, geo, rules, expired) → expected status/Location; same for one v1 custom domain if any.
2. `https://<v1 frontend host>/r/<code>` → 308 to the backend host.
3. Login with a known v1 test account (password) → `/me` shows its organisation and workspaces; the stored hash is now argon2.
4. Google sign-in with a v1 Google-created test account → links (C4).
5. Create a dynamic code, download PNG, decode it (zxing), request the encoded URL through Cloudflare → 302; within 60 s the scan appears in `/realtime` and analytics.
6. A v1 API key (`ak_…`) calls `POST /api/v1/qr/api/generate` → v1-shaped response with `Deprecation` headers.
7. A migrated webhook receives `scan.created` with a valid `X-QRit-Signature` (use a test webhook on request bin owned by Aryan).
8. `/p/<v1 lead slug>` renders; a test submission is stored encrypted.
9. Stripe: send a test event to `/api/v1/billing/webhook` from the Stripe dashboard → 200 and `billing_events` row.
10. `GET /v1/orgs/{org}/audit-logs/verify` → intact for a sampled org.

---

## 7. Rollback

**Decision window:** until T+1 hour (or any time a critical defect appears before v3 has accepted meaningful customer changes). After that, fix forward.

Steps:
1. Status page: maintenance.
2. Railway: roll back `backend` and `frontend` to their last v1 deployments (Railway restores the image and the service variables of that deployment). If the rehearsal showed that settings (start command, health check path, region, pre-deploy) are not restored, re-apply them with `railway config apply --file .railway/v1-snapshot.ts --yes`.
3. Make v1 writable: `ALTER DATABASE <v1 db> SET default_transaction_read_only = off;` and restart the v1 backend.
4. Cloudflare: point `REDIRECT_ORIGIN` to the maintenance page (no v3 short links were printed yet, but some may have been created during the hour).
5. Remove the `redirect`, `worker` and `ingest` deployments (or set replicas to 0 if supported) so they don't process anything.
6. `git revert` the merge on `main` so the next push doesn't redeploy v3.
7. `export_post_cutover_changes --since <T>` from the migrator; tell affected users (likely none) what to redo.
8. Post-mortem in `docs/v3/runbooks/cutover-postmortem.md`; plan the next attempt (fresh full import into a recreated `postgres-v3`).

---

## 8. After go-live

- **72 h hypercare:** synthetic probes every minute (canary code on `SHORT_DOMAIN`, one legacy link, `API_DOMAIN/readyz`, `APP_DOMAIN/api/health`), Sentry alerts to Aryan's phone, ingest lag and redirect p95 dashboards.
- Re-run `verify_v1_import --http-base …` daily for 3 days (checks legacy links stay correct).
- T+1 day: `completed` and `leads_notice` emails.
- T+7 days: review the safety worker's progress on imported `pending` destinations; review grandfathered-limit edge cases reported by customers.
- T+30 days: final backup of v1 Postgres → delete it; delete `migrator`; remove v1-only variables from `backend`/`frontend`; record in `DECISIONS.md`.
- Sunset date of the v1 API (`api_notice`): remove the compatibility endpoints (legacy `/r/` links stay forever).

---

## 9. Ongoing operations

- **Deploys:** merge to `main` → Railway builds affected services (watch paths). Migrations MUST be backward compatible with the previous release (expand → deploy → contract) because `redirect`, `worker` and `ingest` may run old code for a minute while `api`'s pre-deploy migrates. `wait_for_migrations` makes worker/ingest start only after migrations are applied; `/readyz` keeps new redirect replicas out of rotation until then.
- **Scaling:** redirect first (CPU-bound JSON + HTTP), then api; ingest to 2 when stream lag > 60 s; enable PgBouncer per §5.2.
- **Secrets rotation:** `EDGE_SHARED_SECRET` — accept two values during rotation (`EDGE_SHARED_SECRET_PREVIOUS`); `JWT_ED25519_PRIVATE_KEY` — rotate with a new `JWT_KEY_ID` and keep the previous public key in `JWT_PREVIOUS_PUBLIC_KEYS` for at least 10 minutes (access token lifetime); `APP_ENCRYPTION_KEY` — only with `manage.py rotate_kek` (re-wraps DEKs, re-seals platform secrets, recomputes blind indexes); Stripe/Razorpay webhook secrets — provider-side rolling.
- **Region:** everything stays in Singapore; if India-region hosting becomes available on Railway, move databases with a maintenance window (logical replication or dump/restore).
- **Cost watch:** Railway usage (replicas × memory), R2 storage, Cloudflare Workers requests, Resend volume — monthly review.

---

## 10. Config-as-Code deadline (1 Dec 2026)

Railway stops reading `railway.json`/`railway.toml` on **1 December 2026**. If the v3 cutover happens before that date, nothing else is needed: v3 removes `backend/railway.json` and `frontend/railway.json` and manages settings through `.railway/railway.ts`. If the cutover slips past mid-November, migrate v1 first on `main`: `railway config migrate --lang ts`, then re-add what the migrator is known to drop (Dockerfile path/builder, restart policy, region — railwayapp/cli issue #1199), `railway config plan` must show no changes, then delete the JSON files.

---

## 11. Sources (checked 27 Sep – 3 Oct 2026)

- Railway: Infrastructure as Code https://docs.railway.com/infrastructure-as-code and reference https://docs.railway.com/infrastructure-as-code/reference · Config as Code deprecation https://docs.railway.com/config-as-code · deployment actions (rollback restores image and variables; retention limits) https://docs.railway.com/deployments/deployment-actions · `railway ssh` https://docs.railway.com/cli/ssh · PgBouncer https://docs.railway.com/databases/postgresql-pgbouncer · pre-deploy command https://docs.railway.com/deployments/pre-deploy-command · health checks https://docs.railway.com/deployments/healthchecks · regions https://docs.railway.com/deployments/regions · domains https://docs.railway.com/networking/domains/working-with-domains · private networking https://docs.railway.com/networking/private-networking/how-it-works · backups https://docs.railway.com/volumes/backups · static outbound IPs https://docs.railway.com/networking/static-outbound-ips · `railway config migrate` issue https://github.com/railwayapp/cli/issues/1199
- Cloudflare: Workers as the origin for Cloudflare for SaaS https://developers.cloudflare.com/cloudflare-for-platforms/cloudflare-for-saas/start/advanced-settings/worker-as-origin/ · Cloudflare for SaaS custom hostnames https://developers.cloudflare.com/cloudflare-for-platforms/cloudflare-for-saas/
- PostgreSQL PGDG apt repository https://wiki.postgresql.org/wiki/Apt
