# QRit v3 — Implementation Plan (Django-only rewrite, v1 + v2 merge, enterprise, Railway)

**Date:** 28 September 2026 · **Branch:** `v3` (cut from `v2-enterprise`) · **Owner:** Aryan
**Audience:** Gemini (code generation) and Aryan (review, accounts, go/no-go).
**Companion documents (same folder):**

| File | Purpose |
|---|---|
| `QRit_v3_Plan.md` (this file) | What to build and why: architecture, stack, schema, every module, phases, parity matrix |
| `GEMINI_PROMPT_V3.md` | How Gemini must work: rules, conventions, per-phase prompts and exit gates |
| `V1_TO_V3_MIGRATION.md` | Field-level v1 → v3 data mapping, the `import_v1` command, legacy links, verification, rollback |
| `RAILWAY_DEPLOY.md` | Railway + Cloudflare setup, environment variables, database roles, staging rehearsal, production cutover |

**Reference material that stays in the repo (read-only for Gemini):**

| Path (after Phase 0) | What it is | How to use it |
|---|---|---|
| `reference/go-v2/` (was `services/`) | The v2 Go backend incl. enterprise stages 1–5 and half of 6, **with passing integration tests** | Behaviour spec. Port logic and test cases; do not port Go idioms |
| `reference/go-v2/db/migrations/00001…00007_*.sql` | The authoritative PostgreSQL schema | Table/column/constraint names in v3 MUST match (plus the deltas in §6.4) |
| `reference/v1-django/` (was `backend/`) | The live v1 Django app | Understand v1 behaviour and data; source for the ETL and compatibility routes |
| `reference/v1-frontend/` (copy of v1 `frontend/`) | v1 Next.js app | Source of pages that v2 lacks (§7.24) |
| `reference/ts-render/` (was `packages/qr-render` + `apps/render`) | TypeScript QR renderer | Spec for the Python renderer and generator of golden fixtures (§7.8) |
| `docs/v2-blueprint/*.md|sql` | v2 blueprint, enterprise plan, verified SQL | Detailed requirements for modules marked "spec-only" below |

Words: **MUST** = required for acceptance; **SHOULD** = default unless a written reason is recorded in `docs/v3/DECISIONS.md`; **MAY** = optional.

---

## Contents

1. Executive summary
2. Current state (verified 27–28 Sep 2026)
3. Target architecture
4. Technology stack (pinned)
5. Repository layout
6. Data model
7. Module specifications (7.1–7.25)
8. Permission catalogue and role matrix
9. API conventions and route table
10. Background jobs and schedules
11. Testing strategy
12. Security and compliance
13. Observability and SLOs
14. Deployment summary (details: `RAILWAY_DEPLOY.md`)
15. v1 data migration summary (details: `V1_TO_V3_MIGRATION.md`)
16. Phased delivery plan (P0–P13)
17. Feature parity matrix
18. Risks and open questions
19. Appendices (environment variables, error codes, Go → Python map, sources)

---

## 1. Executive summary

### 1.1 What v3 is

v3 is **one Python codebase** that replaces both the live v1 Django app and the unreleased v2 Go services, plus **one merged Next.js frontend**, deployed on **Railway** (Singapore region) behind **Cloudflare**:

- **Backend:** Django 6.1 on Python 3.14. Django serves the control-plane API, the dynamic-QR redirect hot path (a separate service running the same codebase with a redirect-only URLconf), background jobs (procrastinate on PostgreSQL), the scan-ingest consumer and all rendering (QR SVG/PNG/PDF/EPS, invoices, reports).
- **Frontend:** the v2 Next.js app (cookie auth, `/v1` same-origin rewrite, workspace routes) completed with every v1 feature that v2 lacks and the enterprise screens, on Next.js 16.
- **Data:** the v2 PostgreSQL schema (migrations 00001–00007) re-expressed as Django models and migrations, plus a small set of v3 deltas. **All v1 production data is migrated** by an idempotent `import_v1` command, and **every existing v1 link `/r/<code>` keeps working**.
- **Enterprise:** everything in the enterprise plan (SSO/SCIM/MFA, governance and approvals, tamper-evident audit + SIEM, contracts and GST invoicing, integrations, alerts, reports, lead capture with consent/DSAR, pixels, GS1 resolver, serialization, white-label/agency, developer platform, staff console and support access) is implemented in Django. Stages already built in Go are **ported from working code and tests**; the rest are built from the enterprise specification.

### 1.2 How the three requests map onto the plan

| Request | Where it is delivered | Phases |
|---|---|---|
| 1. Migrate v2 to Python | Django apps replacing every Go service; Python renderer replacing the TypeScript render service | P0–P4 |
| 2. Merge v1 and v2 and deploy to Railway as an upgraded version | Merged frontend, v1 compatibility layer, `import_v1`, Railway + Cloudflare infrastructure, staging rehearsal, production cutover | P11–P13 |
| 3. Integrate all enterprise features into the current build | Enterprise modules in Django (ported where Go exists, built from spec otherwise) | P5–P10 |

### 1.3 Decisions

| # | Decision | Choice | Why | Rejected alternatives |
|---|---|---|---|---|
| D1 | Backend framework | **Django only** (Aryan, 28 Sep 2026) | v1 is Django and live; one framework for API, redirect, workers, rendering | FastAPI (second framework), keeping Go |
| D2 | Django version | **6.1.x** (6.1.1 released 2 Sep 2026); upgrade to **6.2 LTS** after it ships (Apr 2027) | Current stable; built-in CSP; `django.tasks` API; DRF 3.18 supports it | 5.2 LTS (older; security-only now), 6.0 (support ends Apr 2027) |
| D3 | Python | **3.14.x** (3.13 as fallback) | All C-extension wheels needed are available for 3.14; 3.13 becomes security-only after Oct 2026 | 3.15 (not final), free-threaded 3.14t (no psycopg/xmlsec wheels) |
| D4 | Background jobs | **procrastinate 3.10** (PostgreSQL queue) | Transactional enqueue through Django's connection, cron-style periodic tasks, retries, no extra broker | Celery (separate broker, no transactional enqueue), django-tasks-db (no retries, no cron) |
| D5 | Redirect hot path | Separate Railway service `redirect` running Django with `qrit.settings.redirect` (minimal middleware), **async view**, in-process TTL cache → `redis.asyncio` → one raw SQL query via a psycopg **async** pool. The ORM is never used on the hot path | Django's async ORM and async cache are thread wrappers; raw async I/O keeps p95 low | Sync Django view on gunicorn threads (slower under load) |
| D6 | ASGI servers | API/worker HTTP: **gunicorn 26 + uvicorn-worker**; redirect: **granian 2.8** (`--interface asginl`) | Both are documented Django deployment options; granian is the fastest for the hot path | gunicorn's native ASGI worker (fixes still unreleased on PyPI) |
| D7 | QR rendering | **Python renderer**: segno matrix → shared geometry → SVG / PNG (resvg_py) / vector PDF and EPS (ReportLab 5) (Aryan, 28 Sep 2026) | Same renderer for preview, downloads, print sheets, invoices; no system libraries for PNG | CairoSVG/WeasyPrint (system libs), keeping the Node renderer |
| D8 | Dashboard preview | The editor calls `POST /v1/render/preview` (debounced) and shows the returned SVG | Preview must equal the download now that rendering is in Python | Client-side TS renderer (would diverge) |
| D9 | Schema ownership | Django models + migrations own the schema; database objects Django cannot express (RLS, triggers, SQL functions, partitioned tables) are created with `RunSQL` ported from 00001–00007 | One migration system; keeps the verified SQL | Unmanaged models over goose migrations |
| D10 | SAML | **pysaml2 7.5** natively (needs the `xmlsec1` binary in the image) | Maintained (CVE-2026-26007 fixed); removes the Polis bridge service | python3-saml (no release since 2023), BoxyHQ Polis sidecar |
| D11 | SCIM | Custom DRF implementation ported from `reference/go-v2/internal/httpapi/scim.go` | django-scim2 pins Django < 6.1 | django-scim2 |
| D12 | Edge and custom domains | **Cloudflare** in front of all short-link traffic: platform short domain proxied; customer domains via **Cloudflare for SaaS custom hostnames**; a small **Cloudflare Worker** forwards to the Railway origin with verified host and geo headers | Railway allows 20 custom domains per service on Pro and says it won't raise this for multi-tenant use; Cloudflare also gives country/city for analytics without a GeoIP licence | Adding customer domains on Railway directly; MaxMind GeoLite2 (licence unclear for SaaS display) |
| D13 | Railway configuration | **Infrastructure as Code** (`.railway/railway.ts`) for services, variables, replicas, health checks, pre-deploy; dashboard for settings IaC cannot express yet (restart policy, static IPs) | `railway.json`/`railway.toml` can't be adopted by new services and stop being read on **1 Dec 2026** | Config as Code files |
| D14 | Postgres | Railway Postgres **17** (pinned image tag), app runs as a **non-superuser role** so Row-Level Security applies; built-in PgBouncer enabled when replicas grow (transaction mode; migrations and the job worker use the unpooled URL) | RLS is bypassed by superusers; procrastinate needs LISTEN/NOTIFY | Running the app as `postgres` |
| D15 | Object storage | S3-compatible bucket (Cloudflare R2 by default; Railway Buckets acceptable) via `STORAGE_*` variables | Services have replicas; Railway volumes can't be used with replicas | Railway volumes |
| D16 | Email | **Resend** HTTPS API (already used by v1) | Railway blocks SMTP on Free/Hobby | SMTP |
| D17 | Payments | **Razorpay** for INR (subscriptions incl. UPI Autopay, invoice payment links), **Stripe v15** for international cards | India-first; v1 used Stripe | Stripe only |
| D18 | Go enterprise work | Kept in `reference/go-v2/` as the behaviour spec (Aryan, 28 Sep 2026) | 5½ stages are implemented and tested there | Discarding it |
| D19 | v1 compatibility | Fixed decisions C1–C12 in `V1_TO_V3_MIGRATION.md` §3 (legacy links, bcrypt passwords, Google linking for imported users, legacy API keys and webhook signatures, `needs_reprint`, rules translation incl. a `fallthrough` split variant, plan mapping `starter→pro`, `pro→business` with grandfathered limits) | Every v1 link, login, key and webhook keeps working | Forcing customers to re-create integrations |

### 1.4 Effort and order

Phases P0–P13 (§16) are sized for Gemini sessions with a review gate after each. Rough order of magnitude: P0–P4 core (≈40% of the work), P5–P10 enterprise (≈40%), P11–P13 frontend, migration and cutover (≈20%). The frontend phase P11 can run in parallel from P3 onward because the API contract is fixed in §9.

### 1.5 Top risks (full list in §18)

1. **v1 printed codes don't use short links.** v1 "dynamic" QR images encode the destination URL, not `/r/<code>`. Printed v1 codes therefore keep going straight to their destinations and can never be redirected. v3 must say this honestly in the UI and offer a re-download (§15, `V1_TO_V3_MIGRATION.md` §7).
2. **Redirect latency in Python.** Mitigated by D5/D6, a latency budget and a k6 gate (§13).
3. **Custom-domain routing through Cloudflare** needs the Worker (D12) to be tested early (P4 spike).
4. **GST correctness** (SAC code, e-invoicing threshold) needs confirmation from Aryan's CA before invoices go live.

---

## 2. Current state (verified 27–28 Sep 2026)

### 2.1 v1 — live on Railway (`main` branch; `backend/` + `frontend/`)

- **Stack:** Django 4.2.16 + DRF 3.15.2 (project `qrapp`, app `api`), PyJWT HS256 access tokens + JWT refresh tokens stored as sha256 with family rotation, bcrypt passwords (12 rounds), `qrcode[pil]` 7.4 rendering, Stripe 11, Resend email, Google ID-token sign-in, gunicorn gthread; Next.js 16.1.4 frontend with zustand (tokens in localStorage), axios, `qr-code-styling` client-side previews.
- **Railway:** two services (`backend/railway.json`: Dockerfile build, `gunicorn qrapp.wsgi:application`, health `/health`; `frontend/railway.json`: `node server.js`, health `/api/health`). Migrations are **not** run by the Railway config (`RUN_MIGRATIONS=true` in `entrypoint.sh` or a manual step).
- **Data (26 tables, UUID primary keys):** `users`, `qr_records`, `qr_scans`, `qr_files` (unused), `free_tier_usages`, `workspaces`, `workspace_members`, `workspace_invites`, `folders`, `audit_logs`, `webhooks`, `webhook_logs`, `lead_capture_pages`, `leads`, `campaigns`, `qr_templates`, `routing_rules`, `workspace_api_keys`, `custom_domains`, `sso_configs`, `refresh_tokens`, `login_attempts`, `password_reset_tokens`, `email_verifications`, `security_policies`, `bulk_jobs`. Inventory: `V1_TO_V3_MIGRATION.md` §2; field-level mappings: §5.
- **Redirect:** `GET|POST /r/<code>` on the backend host (or `https://<workspace.custom_domain>/r/<code>`); short codes are **8 characters, case-sensitive, `[A-Za-z0-9_-]`** (`secrets.token_urlsafe(8)[:8]`); checks inactive → scheduled (425) → expired (410) → max scans (410) → password form (plaintext compare) → geo allow/block (451) → routing rules → 302. Scans are written from a daemon thread (raw IP, lat/long, full UA) and webhooks are sent synchronously from that thread.
- **Defects v3 must not carry over:**
  1. v1 "dynamic" QR images encode the destination, not the short URL: server-generated images encode `content` (`views/qr.py`, `views/ws_qr.py`), the dashboard preview encodes `redirect_url || content` (`frontend/src/app/dashboard/qr-codes/[id]/page.tsx:83`). Editing a v1 dynamic code never changed printed codes.
  2. QR passwords, webhook secrets, legacy user API keys and invite tokens are stored in plaintext.
  3. `POST /api/v1/bulk/generate` is unauthenticated.
  4. `POST /workspaces/<ws>/qr` returns 405 (handler defined on the detail view).
  5. `public/analytics/<code>` filters on a non-existent field.
  6. `LeadsReportView` does no ownership check.
  7. Client IP taken blindly from the first `X-Forwarded-For` entry.
  8. Outbound webhook calls have no SSRF guard.
  9. Plans live on `users.plan`; `workspaces.plan` is copied once and never resynced.
  10. SSO/SCIM screens store configuration but no login flow exists.

### 2.2 v2 — Go, never deployed (`v2` branch; `services/`, `apps/web`, `apps/render`, `packages/`)

- **Services:** `api` (chi), `redirect`, `ingest` (Redis stream `scans` → `scan_events` + rollups), `worker` (Postgres job queue + periodic tasks), `migrate` (goose), `migrate-v1` (broken — see below).
- **Schema:** migrations `00001_init` (core), `00002_rls` (FORCE RLS on 15 tenant tables via `qrit_current_workspace()`), `00003_worker`, `00004_enterprise` (42 tables), `00005_identity`, `00006_governance`, `00007_billing`.
- **Works and is tested** (on `v2-enterprise`): auth lifecycle, QR lifecycle, rules preview, idempotency, tenant isolation + RLS, scan pipeline end-to-end, job queue, organisations and access engine, tamper-evident audit, envelope encryption and flags, MFA, domains + OIDC, SCIM, access governance, approvals, audit streams, GST invoicing core (unit tests).
- **Broken or missing:**
  - `apps/web` calls `/v1/workspaces/{ws}/qr…` but the API mounts `/qr-codes`; web expects `{items}` but the API returns `{data, next_cursor}`; login expects `user.workspaces[0].slug`; web calls `/billing/checkout|portal` that don't exist; analytics, domains, webhooks, security and billing pages are mocked.
  - No HTTP routes for webhooks, API keys, custom short domains, billing, Google OAuth, bulk jobs, QR exports, forms/leads, public generator; `internal/billing`, `internal/webhooks`, `internal/domains` are used only by their own tests.
  - The redirect service has no `/r/` route and never reads `legacy_short_code`.
  - `cmd/migrate-v1` assumes integer IDs and wrong column names and writes to a non-existent table — unusable.
  - `docker-compose.yml` build contexts can't see `packages/`; Fly configs build the `api` binary for every service.
  - v2 passwords are argon2id; v1 bcrypt hashes don't verify in Go.

### 2.3 Enterprise status (what Gemini ports vs builds from spec)

| Module (enterprise plan §) | State in Go | Reference code | Reference tests |
|---|---|---|---|
| Organisations, access engine, identity gate (6.1/5.3) | Done | `internal/org`, `internal/access`, `internal/authz`, `internal/httpapi/{org_handlers,gate,policy_handlers}.go` | `enterprise_test.go: TestOrganisationsAndAccess` |
| Tamper-evident audit (6.4) | Done (hash chain, seal, anchor, verify, export, retention) | `internal/audit`, `internal/worker/audit.go`, `httpapi/audit_handlers.go`, `00004` audit SQL | `TestTamperEvidentAudit` |
| Envelope encryption, feature flags | Done | `internal/envelope`, `internal/flags` | `TestEnvelopeAndFlags` |
| SSO OIDC + SAML bridge, claimed domains, MFA TOTP, SCIM (6.1) | Done | `internal/sso`, `httpapi/{identity,mfa_handlers,sso_handlers,scim}.go`, `internal/worker/identity.go` | `identity_test.go` (TestMFA, TestDomainsAndOIDC, TestSCIM), `scim_unit_test.go`, `internal/sso/sso_test.go` |
| Roles, groups, bindings, explain, access review (6.2) | Done | `httpapi/governance_handlers.go` | `TestAccessGovernance` |
| Approvals (6.3) | Done (single-code; bulk pending) | `internal/approval`, `httpapi/approval_handlers.go`, `internal/worker/governance.go` | `TestApprovals`, `internal/approval/approval_test.go` |
| SIEM audit streams (6.4) | Done | `internal/auditstream`, `internal/stdwebhook`, `httpapi/auditstream_handlers.go`, `internal/worker/auditstream.go` | `TestAuditStreams`, unit tests incl. AWS SigV4 and Standard Webhooks vectors |
| Contracts and GST invoicing (6.12.1) | Core only (no routes, PDF, worker) | `internal/invoicing`, `internal/billing/paymentlinks.go`, `00007_billing.sql` | `internal/invoicing/tax_test.go` |
| Support access, staff console (6.12.2–3) | Token + org context only | `internal/auth` (`CreateStaffAccessToken`), `org_handlers.go` staff branch | — |
| Integrations, alerts, reports (6.5) | Spec only | `docs/v2-blueprint/QRit_v2_Enterprise_Plan.md` §6.5 | — |
| Leads, consent, DSAR (6.6) | Spec only | Enterprise plan §6.6; tables in `00004` | — |
| Pixels (6.7) | Spec only | Enterprise plan §6.7 | — |
| GS1 resolver (6.8) | Spec only | Enterprise plan §6.8; `enterprise_queries.sql` `GS1Linkset`; v1 `api/utils/gs1.py` | — |
| Serialization (6.9) | Spec only | Enterprise plan §6.9; `RecordSerialVerification` | — |
| White-label, agency (6.10) | Spec only | Enterprise plan §6.10 | — |
| Developer platform, bulk, print-grade (6.11) | Spec only | Enterprise plan §6.11 | — |

### 2.4 Consequences for v3

- **Carry over:** the v2 schema and its verified SQL; the v2 API contract (paths under `/v1`, cookies, CSRF, problem+json); every behaviour covered by a Go test; v1's feature set and data; v1's `/r/<code>` links.
- **Rewrite:** everything executable in Python; the renderer; the frontend's broken calls.
- **Drop:** Go services, the Node render service, the Polis bridge, Fly configs, `cmd/migrate-v1`, v1's plaintext secrets and synchronous scan threads.

---

## 3. Target architecture

### 3.1 Topology

```
                          Scanners / browsers / API clients
                                        │
          ┌─────────────────────────────┼──────────────────────────────────┐
          │                      Cloudflare (proxied)                        │
          │  SHORT_DOMAIN (e.g. qrit.link)  ·  customer short domains (SaaS) │
          │  white-label app hosts (SaaS)   ·  Worker "edge-router"          │
          └──────────────┬──────────────────────────────┬───────────────────┘
                         │ X-QRit-Host, X-QRit-Geo-*,    │
                         │ X-QRit-Client-IP, edge secret │
                         ▼                               ▼
   ┌──────────────────────────────── Railway project "qrit" (asia-southeast1) ─────────────────────────┐
   │                                                                                                   │
   │  web (Next.js 16)  ──/v1/* rewrite over private network──►  api (Django ASGI, gunicorn+uvicorn)   │
   │   APP_DOMAIN                                                 API_DOMAIN, legacy v1 host (/r/…)     │
   │                                                                  │                               │
   │  redirect (Django ASGI, granian)  ◄── Cloudflare origin          │ transactional enqueue          │
   │   /{code} /p/{code} /r/{code} /v/{serial} /01/… /_px/…           ▼                               │
   │        │  XADD scans          worker (procrastinate: jobs + periodic)   ingest (stream consumer)  │
   │        ▼                              │                                   │                    │
   │     Redis 7  ◄────────────────────────┴───────────────────────────────────┘                    │
   │     Postgres 17 (app role = non-superuser; PgBouncer when scaled)                                │
   └───────────────────────────────────────────────────────────────────────────────────────────────┘
            │                     │                    │                 │
     S3-compatible storage   Resend (email)   Razorpay / Stripe   Google Web Risk, DoH, IdPs
```

### 3.2 Services

All Python services build from **one image** (`backend/Dockerfile`) and differ only by `DJANGO_SETTINGS_MODULE` and start command.

| Service | Root dir | Start command | Settings | Replicas (start) | Health check | Public domain |
|---|---|---|---|---|---|---|
| `web` | `frontend/` | `node server.js` (Next standalone) | — | 2 | `/api/health` | `APP_DOMAIN` (+ legacy v1 frontend domain) |
| `api` | `backend/` | `gunicorn qrit.asgi:application -k uvicorn_worker.UvicornWorker -c gunicorn.conf.py` | `qrit.settings.api` | 2 | `/readyz` | `API_DOMAIN` (+ legacy v1 backend domain, which also serves `/r/<code>`) |
| `redirect` | `backend/` | `granian --interface asginl --host :: --port $PORT --workers $WEB_CONCURRENCY qrit.asgi:application` | `qrit.settings.redirect` | 2+ | `/readyz` | Railway domain + `origin.SHORT_DOMAIN` (Cloudflare origin) |
| `worker` | `backend/` | `python manage.py wait_for_migrations && python manage.py procrastinate worker --concurrency=$WORKER_CONCURRENCY` | `qrit.settings.worker` | 1–2 | none (process) | none |
| `ingest` | `backend/` | `python manage.py wait_for_migrations && python manage.py ingest_scans` | `qrit.settings.worker` | 1 (2 when lag grows) | none | none |
| `postgres` | template | Railway Postgres 17 | — | 1 (HA later) | — | none (private) |
| `redis` | template | Railway Redis 7 | — | 1 | — | none (private) |

Pre-deploy (`api` service only): `python manage.py migrate --database owner --noinput && python manage.py ensure_db_roles --database owner && python manage.py ensure_redis_config`. Migrations run once per deploy with the **owner** database URL: `DATABASES` gets a second alias `owner` (built from `DATABASE_OWNER_URL` when it is set), a database router never sends application queries to it, and `migrate --database owner` applies Django's and procrastinate's migrations as the owner; the running services use the non-superuser app role (`DATABASE_URL`). `collectstatic` runs at **image build** time (`qrit.settings.build`), because Railway discards file changes made by the pre-deploy container. `worker` and `ingest` start with `python manage.py wait_for_migrations && …` so they never run against an older schema. Details in `RAILWAY_DEPLOY.md` §3–§5.

### 3.3 Request flows

**Dashboard.** Browser → `web` (same origin) → Next.js rewrite `/v1/:path*` → `http://backend.railway.internal:8080/v1/:path*` (the api service keeps its v1 Railway name `backend`, `RAILWAY_DEPLOY.md` §1). Auth is cookie-based (httpOnly `qrit_access` 10 min, `qrit_refresh` 30 days, readable `qrit_csrf` for double-submit via `X-CSRF-Token`). No tokens in localStorage.

**Public API.** Clients → `API_DOMAIN/v1/...` with `Authorization: Bearer qk_live_…` (API keys, §7.21) or bearer JWT. SCIM at `API_DOMAIN/scim/v2`. Provider webhooks at `API_DOMAIN/v1/billing/{razorpay|stripe}/webhook`.

**Scan (hot path).** Scanner → Cloudflare → Worker `edge-router` (adds verified host + geo) → `redirect` service:
1. Parse host (`X-QRit-Host` when the edge secret matches, else `Host`) and path.
2. Resolve `(domain_id, code)` through: in-process TTL cache (30 s, 100k entries) → Redis `link:v1:{domain}:{code}` (JSON, TTL until `next_change_at` or 1 h) → Postgres (one query, `resolvedLinkSQL` ported from `reference/go-v2/internal/redirect/store.go`). Negative results are cached for 30 s.
3. Evaluate lifecycle gates → routing rules → UTM append → response (302, hosted page, password form, interstitial, error page).
4. Emit a scan event to an in-process bounded queue; a background task `XADD`s batches to Redis stream `scans` (`MAXLEN ~ 5,000,000`, field `e` = JSON). If Redis is down, events are dropped with a counter (never block the redirect).
5. Invalidation: API publishes `qr:invalidate` / `domain:invalidate` on Redis pub/sub (after commit); every redirect process **and** every api process (the api serves legacy `/r/`) subscribes lazily on its first request — granian `asginl` has no lifespan events — and evicts. The `qr:invalidate` message carries `{domain_id, short_code, legacy_short_code}` so the `legacy:{code}` cache key is evicted too.

**Ingest.** `ingest` consumes `scans` with a consumer group (`XREADGROUP`, `XAUTOCLAIM` for stuck messages, DLQ stream `scans:dlq` after 5 deliveries), dedupes counted scans with the Lua script from `reference/go-v2/internal/ingest`, batch-inserts `scan_events` and upserts `scan_visitors_daily`, `scan_stats_15m`, `scan_stats_daily_dim` in one transaction per batch (≤ 1,000 events or 1 s).

**Jobs.** Services enqueue procrastinate jobs **inside the business transaction** (`task.defer()` within `transaction.atomic()` goes through Django's connection, so a rollback discards the job). The worker runs job handlers and periodic tasks (§10). The worker uses the **unpooled** database URL (LISTEN/NOTIFY and advisory locks).

**Rendering.** API endpoints call the Python renderer in-process for previews and small downloads (≤ 4096 px, single code). Print sheets, serial proof sheets, invoice PDFs and report PDFs run as jobs; outputs go to object storage and are delivered through short-lived signed URLs.

### 3.4 Tenancy and security model

- **Hierarchy:** organisation → workspaces → folders → codes. The organisation owns plan, billing, identity policy, audit chain and encryption keys. Every workspace belongs to one organisation (`workspaces.org_id NOT NULL`).
- **Authorisation:** permission-based (§8). The access engine (port of `reference/go-v2/internal/access/engine.go`) resolves a principal's grants per workspace from org role (owner → `*`; admin → admin role), direct and group role bindings (workspace- or folder-scoped), agency parent org, API key scopes (never above editor, never approve) and staff support grants. Grants are cached 30 s per process and invalidated through Redis pub/sub `authz:invalidate`.
- **Identity gate** (port of `gate.go`) runs on every workspace/org request after authentication, in this order: IP allowlist (dashboard or API list) → SSO enforcement (break-glass owners with verified MFA exempt) → MFA required → session idle / max lifetime → billing hold (mutations only).
- **Row-Level Security:** all tenant tables keep the `FORCE ROW LEVEL SECURITY` policies from `00002_rls.sql` and `00004_enterprise.sql`. Workspace-scoped views run inside `transaction.atomic()` and execute `SELECT set_config('app.workspace_id', %s, true)` first (helper `core.db.workspace_scope(ws_id)`). The application connects as role `qrit_app` (`NOSUPERUSER NOBYPASSRLS`), so policies apply even if a query forgets a `workspace_id` filter. Tests assert this (port `TestRowLevelSecurity`).
- **Audit:** every mutating endpoint writes its audit rows **in the same transaction** — exactly one *primary* action per endpoint, plus only the *secondary* actions listed for it in `apps/audit/actions.py::SECONDARY` (e.g. an approval decision writes `approval.decided` and, when it finalises, `qr.version.activated`); rows are sealed into a per-org SHA-256 hash chain by the worker (`audit_seal` SQL function), anchored daily, verifiable via API; the table is append-only (trigger).
- **Encryption at rest:** per-org data keys (DEKs) in `org_data_keys`, wrapped with a key derived from `APP_ENCRYPTION_KEY` (HKDF-SHA256), AES-256-GCM with AAD = org id, 4-byte key-id prefix; platform-level sealing for TOTP secrets. Port `reference/go-v2/internal/envelope`.

### 3.5 Redirect latency budget

Targets at the Railway origin, measured by the k6 script (`deploy/k6/redirect_load.js`, ported to hit the Python service): **p50 ≤ 8 ms, p95 ≤ 25 ms, p99 ≤ 60 ms at 500 RPS per replica on a cache hit**, and ≤ 40 ms p95 on a Redis hit. How:

- `qrit.settings.redirect`: `INSTALLED_APPS` limited to `core`, `redirect`, `qr` (models only for admin-free imports), no sessions, no auth, no CSRF, no messages; middleware = request id, security headers, client IP only. Every lookup the redirect service needs — codes, hosted pages and forms on `/p/`, GS1 linksets, serials, pixels, branding — is raw SQL in `apps/redirect/queries.py`, and its templates live in `apps/redirect/templates/`, so no other app must be installed. Pools, the pub/sub subscriber and the scan flusher are created **lazily per process on the first request** (no ASGI lifespan under `asginl`). The api service mounts the same async `/r/` view (CSRF-exempt) with its own small async pool (counted in `RAILWAY_DEPLOY.md` §5.2).
- The view is `async def`; Redis via `redis.asyncio` (one pool per process, RESP3, `max_connections=200`); Postgres via `psycopg_pool.AsyncConnectionPool` (`DB_POOL_MIN`/`DB_POOL_MAX`, default 1/4 per process; budget in `RAILWAY_DEPLOY.md` §5.2) with a prepared statement (auto-prepare disabled when PgBouncer transaction pooling is on); JSON via `orjson`.
- No ORM, no Django cache framework, no template engine on the 302 path (pages use pre-compiled Django templates rendered only for non-302 outcomes).
- Scan emission is non-blocking (bounded `asyncio.Queue`, background flusher every 50 ms or 200 events).
- `Cache-Control: private, no-store` on every dynamic response (Railway CDN caching stays off for this service).

---

## 4. Technology stack (pinned)

Versions verified on PyPI / official release notes on 27–28 Sep 2026. Pin with `uv` (`backend/pyproject.toml` + `uv.lock`); use compatible-release ranges (`~=`) as shown and let the lockfile pin exact versions. **Gemini MUST NOT introduce any dependency that is not in this table** without recording a decision in `docs/v3/DECISIONS.md`.

### 4.1 Runtime

| Area | Package | Version | Notes |
|---|---|---|---|
| Language | CPython | 3.14.x (3.14.7 current) | Image `python:3.14-slim-bookworm` (or trixie). 3.13 fallback |
| Framework | Django | `~=6.1.1` | PostgreSQL 15+ required. Upgrade to 6.2 LTS in 2027 |
| REST | djangorestframework | `~=3.18.1` | 3.18 returns list-serializer errors as dicts |
| OpenAPI | drf-spectacular | `~=0.30.0` | Serves `/v1/openapi.json`; source for SDKs |
| DB driver | psycopg[binary,pool] | `~=3.3.6` | Use `psycopg[c]` build in production image if the compiler stage is available; binary is acceptable |
| DB pool | psycopg-pool | `>=3.3.3` | Fixes 24-hour idle worker bug. Django pool: `OPTIONS={"pool": {...}}`, `CONN_MAX_AGE=0` |
| Jobs | procrastinate | `~=3.10.0` | `procrastinate.contrib.django`; transactional `defer()`; `@app.periodic(cron=...)` |
| Redis | redis[hiredis] | `~=8.1.0` | RESP3 default, 5 s timeouts, `redis.asyncio` for the hot path |
| ASGI (API) | gunicorn + uvicorn-worker + uvicorn | `~=26.2`, `~=0.4.0`, `~=0.54` | `-k uvicorn_worker.UvicornWorker` |
| ASGI (redirect) | granian | `~=2.8.3` | `--interface asginl` (Django has no lifespan) |
| JSON | orjson | latest 3.x | Hot path and ingest |
| Cache (in-process) | cachetools | latest 5.x | `TTLCache` on the redirect path |
| Settings | pydantic-settings | `~=2.15` | Typed env parsing in `qrit/settings/env.py` |
| Static files | whitenoise | `~=6.12` | API service only (DRF browsable API off in production) |
| CORS | django-cors-headers | `~=4.9` | Only for the public API domain; dashboard is same-origin |
| Crypto | cryptography | `~=50.0` | AES-GCM, HKDF, Ed25519 |
| Passwords | argon2-cffi, bcrypt | `~=25.1`, latest 4.x | `Argon2PasswordHasher` first; `BCryptPasswordHasher` kept only to verify migrated v1 hashes (auto-upgraded on login) |
| JWT | PyJWT[crypto] | `~=2.15` | EdDSA access tokens (Ed25519), `PyJWKClient` not used for IdPs (Authlib does that) |
| OIDC | Authlib + joserfc + requests | `~=1.8.0`, `~=1.7`, latest | Use `joserfc` (authlib.jose is deprecated) |
| SAML | pysaml2 | `~=7.5.5` | Needs `xmlsec1` binary (`apt-get install xmlsec1`) |
| TOTP | pyotp | `~=2.10` | |
| WebAuthn | webauthn (py_webauthn) | `~=3.0.1` | Passkeys as second factor |
| DNS | dnspython | `~=2.8` | DNS-over-HTTPS (`dns.query.https`) for domain TXT checks |
| HTTP client | httpx | latest 0.28.x | SSRF-guarded transport (§7.1); `respx` for tests |
| UA parsing | — (no library) | — | Port of `reference/go-v2/internal/scan/uaparse.go` (regex classifier) + its test, so values match the Go tests: device `desktop|mobile|tablet|bot|other|unknown`, OS `iOS|Android|macOS|Windows|Linux|other|unknown`, browser `Edge|Samsung Internet|Chrome|Safari|Firefox|other|unknown`; bot detection from `scan/botdetect.go` |
| Spreadsheets | openpyxl | latest 3.1.x | Bulk upload XLSX parsing (read-only, streaming mode) |
| QR matrix | segno | `~=1.6.6` | `qr.matrix`, micro QR, ECC L/M/Q/H |
| PNG | resvg_py | `~=0.5.0` | `resvg_py.svg_to_bytes(svg_string=...)`, no system libs |
| PDF/EPS | reportlab | `~=5.0.1` | Remote images denied by default (good) |
| Images | Pillow | `~=12.3` | Logo processing (resize, re-encode, strip metadata) |
| Decode check (tests) | zxing-cpp | latest | Verify rendered codes scan; test dependency only |
| Payments | razorpay, stripe | `~=2.0.1`, `~=15.6` | Stripe v15: objects are not dicts (`getattr`/`to_dict()`) |
| Email | resend | `~=2.48` | HTTPS API |
| Observability | sentry-sdk, structlog | `~=2.70`, `~=26.1` | |
| Phone numbers | phonenumberslite | latest | Form field validation (E.164) |

### 4.2 Development and CI

| Tool | Version | Use |
|---|---|---|
| uv | `0.12.x` | `uv lock --check`, `uv sync --locked --no-dev` in Docker |
| ruff | `0.16.x` | lint + format (`ruff check`, `ruff format --check`) |
| mypy + django-stubs + djangorestframework-stubs | `2.3.x`, `6.1.1`, `3.18.1` | `--strict` on `apps/*/services.py`, `selectors.py`, `domain/` |
| pytest, pytest-django, pytest-asyncio | `9.1`, `4.14`, `1.4` | |
| factory_boy, hypothesis | `3.3.3`, `6.x` | |
| pytest-xdist, pytest-cov | latest | `pytest -n auto`, coverage gate |
| numpy, scikit-image | latest | SSIM comparison in render golden tests (test dependency only) |
| responses, respx | `0.26`, `0.23` | requests-based SDKs (stripe, razorpay, resend, Authlib) / httpx |
| k6 | latest | Redirect load test |
| Playwright | latest | Frontend smoke + pixel/consent network assertions |

### 4.3 Frontend

Next.js 16 (App Router, `output: "standalone"`), React 19.2, TypeScript 5, Tailwind CSS 4, lucide icons, Recharts for charts, `jsqr` in a web worker for the "does it scan" check on previews. Upgrade to the latest Next.js 16.x patch at P11 start and record the version. **Remove** `@qrit/qr-render` and `qr-code-styling`; previews come from the API.

### 4.4 Rejected on purpose

Celery (extra broker, no transactional enqueue) · django-tasks-db (no retries/cron) · django-scim2 (pins Django < 6.1) · django-ratelimit (unmaintained; use the Redis limiter in §7.1) · python3-saml (no release since 2023) · CairoSVG / WeasyPrint (system libraries) · qrcode (no micro QR) · user-agents (unmaintained) · MaxMind GeoLite2 (licence unclear for showing geo analytics to customers; use Cloudflare geo, §7.10) · Polis SAML bridge · Node render service.

---

## 5. Repository layout (target)

```
/                                   monorepo root (branch v3)
├── .github/workflows/ci.yml        lint, type-check, tests (Postgres 17 + Redis 7 services), build images, OpenAPI diff
├── .railway/railway.ts             Railway Infrastructure as Code (RAILWAY_DEPLOY.md §3)
├── edge/worker/                    Cloudflare Worker "edge-router" (TypeScript, ~80 lines) + wrangler.toml
├── docker-compose.yml              dev: postgres:17, redis:7, mailpit, minio, api, redirect, worker, ingest, web
├── backend/                        Django project (Railway root dir for api/redirect/worker/ingest)
│   ├── Dockerfile                  multi-stage: uv sync → slim runtime (xmlsec1, libpq5), non-root user
│   ├── pyproject.toml, uv.lock
│   ├── manage.py
│   ├── gunicorn.conf.py            bind [::]:$PORT, workers from WEB_CONCURRENCY, graceful timeout 30 s
│   ├── qrit/
│   │   ├── asgi.py, wsgi.py
│   │   ├── settings/{base,api,redirect,worker,build,test,env}.py   (build = collectstatic at image build, no DB/secrets)
│   │   ├── urls/{api,redirect}.py
│   │   └── procrastinate.py        App wiring, queue names, retry strategies
│   ├── apps/
│   │   ├── core/          ids, problem+json, pagination, idempotency, rate limiting, client IP, SSRF-safe HTTP,
│   │   │                  RLS helper, envelope encryption, feature flags, health, storage, email, events outbox
│   │   ├── accounts/      users, sessions/JWT cookies, email tokens, Google sign-in, MFA (TOTP, recovery, WebAuthn), step-up
│   │   ├── orgs/          organizations, org members, claimed domains, security policy, transfer ownership
│   │   ├── workspaces/    workspaces, members, invites, workspace policy, sandbox
│   │   ├── access/        permission catalogue, roles, bindings, groups, engine, identity gate, explain, access review
│   │   ├── identity/      SSO (OIDC, SAML), user identities, SCIM 2.0 server
│   │   ├── qr/            codes, versions, short codes, domains, folders, tags, campaigns, templates, files,
│   │   │                  content encoders, rules engine, UTM, hosted pages, URL safety, trash, plan read-only
│   │   ├── render/        design schema, matrix, geometry, svg/png/pdf/eps backends, scannability, print sheets
│   │   ├── redirect/      hot-path view, resolver + caches, pages, scan emitter, legacy /r/, GS1, serial, pixel interstitial
│   │   ├── analytics/     scan events, partitions, ingest consumer, rollups, realtime, queries, exports
│   │   ├── approvals/     policy evaluation, requests, decisions, finalize, expiry, inbox
│   │   ├── audit/         entries, hash chain, anchors, verify, export, retention, SIEM streams
│   │   ├── billing/       plans + entitlements, subscriptions (Razorpay/Stripe), provider webhooks, contracts,
│   │   │                  GST invoicing, dunning, billing hold, payment links, invoice PDF
│   │   ├── integrations/  event fan-out, webhooks (Standard Webhooks + legacy v1), Slack, Teams, Zapier/Make REST hooks,
│   │   │                  HubSpot, Salesforce, Google Sheets, GA4, warehouse export
│   │   ├── alerts/        rules, evaluation, events
│   │   ├── reports/       schedules, report runs, PDF/CSV builders
│   │   ├── leads/         forms, submissions (encrypted), consent notices, withdrawal, DSAR, retention
│   │   ├── pixels/        pixels, interstitial settings, consent counters
│   │   ├── gs1/           Digital Link parser, items, links, linkset, resolver description, import
│   │   ├── serials/       batches, generation (COPY), MAC, verification, exports
│   │   ├── branding/      org branding, custom app hostnames, email domains, agency/client orgs
│   │   ├── developer/     API keys v2 (+ legacy keys), usage, sandbox, bulk jobs, exports, REST hooks registry
│   │   ├── trust/         abuse reports, Web Risk client, rescans, blocked codes
│   │   ├── staff/         staff console API, support access grants and sessions, SLA reports
│   │   └── legacy/        v1 compatibility routes, `import_v1` + `verify_v1_import` commands
│   └── tests/             cross-app tests (e2e API flows, RLS, contract, load fixtures); unit tests live in apps/*/tests/
├── frontend/                       merged Next.js 16 app (Railway root dir for web)
├── reference/
│   ├── go-v2/                      former services/ (read-only; excluded from builds)
│   ├── v1-django/                  former backend/ (read-only)
│   ├── v1-frontend/                former frontend/ (read-only)
│   └── ts-render/                  former packages/qr-render + apps/render (fixture generator)
├── deploy/k6/redirect_load.js      updated for the Python redirect
└── docs/
    ├── v2-blueprint/               existing specs (read-only)
    └── v3/                         this plan, prompt, migration spec, runbook, DECISIONS.md, runbooks/
```

**Per-app internal layout** (every app under `backend/apps/<name>/`):

```
models.py        Django models (db_table matches the v2 schema)
migrations/      generated migrations + hand-written RunSQL migrations
domain/          pure logic, no Django imports where possible (e.g. rules engine, GST tax, approval evaluation)
selectors.py     read queries (return models or dataclasses)
services.py      write operations; own transactions, audit rows, outbox events, cache invalidation
serializers.py   DRF serializers (validation + output shapes only)
views.py         DRF APIViews: parse → permission → call service/selector → serialize
urls.py          routes for this app (included by qrit/urls/api.py)
tasks.py         procrastinate tasks and periodic tasks
admin.py         (staff-only Django admin is disabled in production; use the staff API)
tests/           test_domain.py, test_services.py, test_api.py, factories.py
```

---

## 6. Data model

### 6.1 Principles

1. **The v2 schema is the v3 schema.** Every table, column name, type, default, CHECK, unique/partial index, foreign key and RLS policy in `reference/go-v2/db/migrations/00001…00007` MUST exist in v3 with the same name and meaning, plus the deltas in §6.4. This keeps the verified SQL (`docs/v2-blueprint/*.sql`, Go queries) valid and makes Go tests portable.
2. **Django owns migrations.** Each table gets a managed model with `Meta.db_table` set to the v2 name. Django field choices map as follows:

   | Postgres (v2) | Django field |
   |---|---|
   | `uuid PRIMARY KEY` | `UUIDField(primary_key=True, default=core.ids.uuid7)` (Python 3.14 `uuid.uuid7()`); imported v1 rows keep their v4 UUIDs |
   | `citext` | `core.fields.CIText` (a `TextField` subclass whose `db_type` returns `citext`); `CREATE EXTENSION citext` in the first migration |
   | `timestamptz` | `DateTimeField` (`USE_TZ=True`, `TIME_ZONE="UTC"`) |
   | `char(n)` (`tax_country`, `gs1_gtin`, `currency`, `country`…) | `core.fields.FixedCharField(max_length=n)` (a `CharField` whose `db_type` returns `char(n)`) |
   | `jsonb` | `JSONField` (with `encoder=core.json.Encoder` for UUID/Decimal/datetime) |
   | `text[]`, `uuid[]`, `cidr[]` | `django.contrib.postgres.fields.ArrayField` (`cidr` via `core.fields.CIDRField`) |
   | `bytea` | `BinaryField` |
   | money | `BigIntegerField` in minor units (never floats) |
   | `CHECK (x IN (...))` | `choices` on the field **and** `models.CheckConstraint` with the same name as v2 (e.g. `qr_dynamic_has_link`) |
   | partial unique index | `models.UniqueConstraint(..., condition=Q(...), name=<v2 index name>)` |
   | FK `ON DELETE CASCADE / SET NULL` | `on_delete=models.DB_CASCADE / models.DB_SET_NULL` (database-level `on_delete`, new in Django 6.1: `DB_CASCADE`, `DB_SET_NULL`, `DB_SET_DEFAULT`) so raw SQL deletes behave like v2. Note: `DB_CASCADE` does not send `pre_delete`/`post_delete` signals — v3 code MUST NOT rely on delete signals |
   | FK `ON DELETE RESTRICT` / no action | `on_delete=models.PROTECT` (Django has no database-level RESTRICT; the database keeps NO ACTION) |

3. **Database objects Django can't express** are created by hand-written migrations using `migrations.RunSQL(sql, reverse_sql)` copied from the Go migrations with goose annotations removed: RLS (`qrit_current_workspace()`, `ALTER TABLE … ENABLE/FORCE ROW LEVEL SECURITY`, policies), triggers and functions (`audit_logs_guard`, `audit_entry_bytes`, `audit_seal`, `audit_verify`, `approval_no_self_approval`, plan-sync triggers, `next_invoice_number`), the partitioned `scan_events` table and its partitions, extensions (`citext`, `pg_trgm`), trigram indexes, and seed rows (system roles). Models for tables created by `RunSQL` use `managed = False` (only `scan_events` and its partitions). Also by `RunSQL`: `audit_logs.id … GENERATED ALWAYS AS IDENTITY` (Django would emit `BY DEFAULT`), and two **`SECURITY DEFINER`** functions owned by the owner role — `qrit_ensure_scan_partitions(p_from date, p_to date)` and `qrit_drop_scan_partitions(p_before date)` (with `SET search_path = public, pg_temp`, `REVOKE ALL … FROM PUBLIC`, `GRANT EXECUTE … TO qrit_app`). Partition DDL needs ownership of `scan_events` and the app role has no DDL rights, so the worker (`analytics.ensure_partitions`, `analytics.drop_old_partitions`) and the v1 importer call these functions instead of running DDL; they port the logic of `reference/go-v2/internal/worker/maintenance.go::EnsurePartitions` (create, attach, move rows out of the default partition).
4. **No `job_queue` / `worker_task_runs`.** procrastinate owns its tables (`procrastinate_jobs`, `procrastinate_periodic_defers`, `procrastinate_events`, …) through its Django migrations. The user-facing `jobs` table (bulk jobs, exports) stays.
5. **One schema-parity test** (`tests/test_schema_parity.py`) applies the Go migrations to an empty database A and the Django migrations to an empty database B, then compares the catalogs (`information_schema.columns`, `pg_constraint`, `pg_indexes`, `pg_policies`, `pg_trigger`, `pg_proc` for the named functions) table by table. Normalisation: ignore goose/procrastinate/django bookkeeping tables and the §6.4 deltas; treat Django's `DEFERRABLE INITIALLY DEFERRED` foreign keys as equal to v2's immediate ones; treat `RESTRICT` and `NO ACTION` as equal; compare index definitions by columns, uniqueness and predicate, not by name when Django generated the name. This test gates P1.

### 6.2 Table ownership

| App | Tables |
|---|---|
| core | `idempotency_keys`, `feature_flags`, `org_data_keys` |
| accounts | `users`, `oauth_accounts`, `sessions`, `email_tokens`, `user_mfa_factors`, `user_recovery_codes` |
| orgs | `organizations`, `org_members`, `org_domains`, `org_security_policies` |
| workspaces | `workspaces`, `workspace_members`, `invites`, `workspace_policies` |
| access | `roles` (+ 5 seeded system roles), `role_bindings`, `groups`, `group_members` |
| identity | `sso_connections`, `user_identities`, `scim_directories`, `scim_users` |
| qr | `domains`, `folders`, `tags`, `campaigns`, `templates`, `files`, `qr_codes`, `short_code_tombstones`, `qr_code_tags`, `qr_versions` |
| analytics | `scan_events` (partitioned by month, `managed=False`), `scan_visitors_daily`, `scan_stats_15m`, `scan_stats_daily_dim` |
| approvals | `approval_requests`, `approval_decisions` |
| audit | `audit_logs`, `audit_anchors`, `audit_streams` |
| billing | `subscriptions`, `billing_events`, `contracts`, `invoice_sequences`, `invoices` |
| integrations | `webhooks`, `webhook_deliveries`, `integrations`, `integration_deliveries` |
| alerts / reports | `alert_rules`, `alert_events` / `report_schedules` |
| leads | `forms`, `form_submissions`, `dsar_requests` |
| pixels | `pixels`, `qr_code_pixels`, `pixel_consent_daily` |
| gs1 / serials | `gs1_items`, `gs1_links` / `serial_batches`, `serial_codes` |
| branding | `org_branding` |
| developer | `api_keys`, `api_usage_daily`, `jobs` |
| trust | `abuse_reports` |
| staff | `support_access_grants`, `support_sessions` |
| legacy | `legacy_id_map` (new, §6.4) |

Tenant tables with RLS (must keep `FORCE ROW LEVEL SECURITY`): `workspace_members`, `invites`, `folders`, `tags`, `campaigns`, `templates`, `files`, `qr_codes`, `api_keys`, `webhooks`, `subscriptions`, `jobs`, `idempotency_keys`, `domains`, `audit_logs`, `role_bindings`, `workspace_policies`, `approval_requests`, `integrations`, `alert_rules`, `report_schedules`, `forms`, `form_submissions`, `pixels`, `gs1_items`, `serial_batches`. Policies use `qrit_current_workspace() IS NULL OR workspace_id = qrit_current_workspace()`, so org-level code that doesn't set the variable (worker, staff) still works; the protection is against **wrong-workspace** reads inside a workspace-scoped request.

### 6.3 Key tables (summary; exact DDL in the Go migrations)

- `users` — `email citext unique`, `password_hash text NULL` (Django `password` field mapped with `db_column="password_hash"`, `null=True`), `name`, `avatar_url`, `locale`, `timezone`, `is_staff`, `last_login_at` (Django `last_login` mapped with `db_column`), soft delete `deleted_at`.
- `sessions` — refresh-token families: `id`, `user_id`, `family_id`, `refresh_token_hash bytea unique` (sha256 of the opaque token), `user_agent`, `ip_prefix`, `created_at`, `last_used_at`, `expires_at`, `revoked_at`, `replaced_by`, plus (00004/00005) `auth_method` (`password|google|magic_link|sso|scim`), `sso_connection_id`, `mfa_verified_at`, `step_up_at`, `mfa_pending`.
- `organizations` — `slug citext unique`, `kind` (`personal|standard|enterprise|agency`), `parent_org_id`, `plan_id` (`free|pro|business|enterprise`, the source of truth; `workspaces.plan_id` is kept in sync by triggers), `data_region`, `legal_name`, `billing_email`, `gstin` (regex-checked), `tax_country`, `billing_address jsonb`, `settings jsonb`, `billing_hold_since`.
- `qr_codes` — `workspace_id`, `mode` (`static|dynamic`), `content_type`, `name`, `domain_id` + `short_code` (7 Crockford base32, unique per domain; constraint `qr_dynamic_has_link`), `legacy_short_code text` (case-sensitive, globally unique partial index), `gs1_gtin`, `static_payload`, `static_content jsonb`, `current_version_id`, `design jsonb`, `design_hash`, `template_id`, `folder_id`, `campaign_id`, `status` (`active|paused|archived|blocked`), `is_read_only`, `starts_at`, `expires_at`, `scan_limit`, `password_hash`, `fallback_url`, `safety_status`, counters, soft delete.
- `qr_versions` — immutable: `version_no`, `destination_kind` (`url|hosted_page`), `destination_url`, `hosted_page jsonb`, `rules jsonb`, `utm jsonb`, `effective_at`, `safety_status`, `restored_from`, `change_note`, `created_by`, `created_by_key`, `approval_status` (`not_required|pending|approved|rejected|cancelled`), `approval_request_id`. **Every query that picks the version in effect MUST filter `approval_status IN ('not_required','approved')`** (resolver, current/next version, scheduled activation, safety rescans, cancel-scheduled).
- `scan_events` — partitioned by month on `occurred_at` (primary key `(event_id, occurred_at)`): `event_id` (UUIDv7 minted by the redirect), `occurred_at`, `workspace_id`, `qr_code_id`, `version_id`, `campaign_id`, `domain_id`, `rule_id`, `outcome` (`redirect|hosted_page|password_prompt|password_ok|password_fail|geo_blocked|paused|expired|not_started|limit_reached|blocked`), `method`, `is_bot`, `bot_reason`, `is_duplicate`, `is_unique` (first counted scan of visitor/code/UTC day), `visitor_hash bytea` (16 bytes, daily-salted HMAC), `device_type`, `os`, `os_version`, `browser`, `browser_version`, `country char(2)`, `region`, `city`, `language`, `referrer_host`, `utm_source`, `utm_medium`, `utm_campaign`, plus `serial` (00004). No raw IP, no full user agent.
- `audit_logs` — `id bigint identity`, `org_id`, `workspace_id`, `actor_type` (`user|api_key|system|staff`), `actor_id`, `action`, `target_type`, `target_id`, `changes jsonb`, `ip_prefix`, `user_agent`, `request_id`, `created_at`, `seq`, `prev_hash`, `hash`, `sealed_at`; append-only trigger.

### 6.4 v3 deltas (new migration `legacy/0001` and per-app migrations after the parity baseline)

| Change | Why |
|---|---|
| `legacy_id_map (entity text, v1_id uuid, v3_id text, imported_at timestamptz, checksum text, PRIMARY KEY (entity, v1_id))` (`v3_id` is text because audit log ids are bigint), `legacy_import_runs` (run bookkeeping) and `legacy_email_log` (migration emails sent) | Idempotent, resumable v1 import (`V1_TO_V3_MIGRATION.md` §4) |
| `qr_codes.legacy_host citext NULL` + index `(legacy_host, legacy_short_code)` | Serve v1 links on the v1 hosts and v1 workspace custom domains |
| `qr_codes.v1_printed_payload text NULL`, `qr_codes.needs_reprint boolean NOT NULL DEFAULT false` | Honest UI for v1 dynamic codes whose printed images encode the destination (§15) |
| `organizations.grandfathered_limits jsonb NOT NULL DEFAULT '{}'` | Keep v1 users' allowances (e.g. a v1 free user keeps every imported dynamic code editable; rules in `V1_TO_V3_MIGRATION.md` §5.3) merged by the plans service like contract overrides |
| `api_keys.legacy_bcrypt_hash text NULL`, `api_keys.legacy_kind text NULL CHECK (legacy_kind IN ('v1_user','v1_workspace'))` | v1 workspace keys are bcrypt(secret); verify once, then store sha256(full key) and clear the bcrypt hash |
| `webhooks.signature_scheme text NOT NULL DEFAULT 'standard' CHECK (signature_scheme IN ('standard','legacy_v1','both'))` | Migrated v1 webhooks keep `X-QRit-Signature: sha256=…` until the owner switches |
| `forms.legacy_slug text NULL UNIQUE`, `forms.presentation jsonb NOT NULL DEFAULT '{}'` (headline, subheadline, hero image file id, button text/colour, background/text colours, thank-you message, redirect URL) | v1 lead pages were served at `/p/<slug>` with page styling that v2 forms lack |
| Replace the full unique index `subscriptions_org_uniq (org_id)` from 00004 with a partial unique index on `org_id` where `status IN ('trialing','active','past_due','paused')` (`workspace_id` is already NULL-able since 00004) | A cancelled subscription must not block a new one for the same organisation |
| `qr_codes` content types: add v1 types that have no v2 equivalent as **hosted-page kinds**, not new content types (§7.7 table) | Keep the v2 content-type CHECK stable |
| `scan_events.source text NOT NULL DEFAULT 'live' CHECK (source IN ('live','v1_import'))` | Imported history is flagged and excluded from anomaly detection |
| `user_mfa_factors` — no change (already has WebAuthn columns) | Passkeys use `kind='webauthn'`, `credential_id`, `public_key`, `sign_count`, `aaguid`, `transports` |

### 6.5 Conventions

- IDs: UUIDv7 for new rows (time-ordered, good index locality). Short codes, API key prefixes, invite tokens and serials have their own formats (§7).
- Timestamps UTC; user-facing dates rendered in the workspace or user timezone.
- Soft delete (`deleted_at`) on users, workspaces, organizations and qr_codes; purge jobs remove rows after 30 days (codes) and per legal retention (users).
- JSON columns are validated by serializers before writes; no free-form JSON from clients reaches the database unvalidated.

---

## 7. Module specifications

Each module lists: purpose · tables · endpoints (full table in §9) · rules · background work · references to port · tests. "Port" means: reproduce the behaviour and every assertion of the referenced Go tests in pytest, adapting only transport details.

### 7.1 core

**Problem details.** Every error is `application/problem+json`: `{type: "https://qrit.io/errors/<code>", title, status, code, detail, instance?, errors?: [{field, code, message}], required_plan?, retry_after_seconds?}` (exact shape of `reference/go-v2/internal/apierr`). A DRF exception handler maps `ValidationError` → 422 `validation_failed` with field errors, `NotAuthenticated` → 401 `unauthorized`, `PermissionDenied` → 403 `forbidden`, `Http404` → 404 `not_found`, throttling → 429 `rate_limited` with `Retry-After`. The web client (`frontend/src/lib/api/client.ts`) depends on this shape. Error codes: Appendix B.

**Pagination.** Cursor-based: `?limit=` (default 50, max 200) and `?cursor=` (opaque base64url of `(created_at, id)`); responses `{data: [...], next_cursor: "..."|null}`. Offset pagination is not offered.

**Idempotency.** Header `Idempotency-Key` (8–128 characters, else 400 `invalid_idempotency_key` — as in Go and the `idempotency_keys` CHECK) on POST endpoints that create resources (codes, versions, invites, bulk jobs, invoices issued by staff). Stored in `idempotency_keys` (scope = workspace + user/key + method + path, request hash, stored response **as a JSON string** to keep byte-exact replays — Go fixed a jsonb key-reordering bug here). Same key + same body → replay (status + body + `Idempotent-Replayed: true`); same key + different body → 422 `idempotency_key_reused`; in-flight → 409. TTL 24 h. Port `reference/go-v2/internal/idempotency` and `TestRulesPreviewAndIdempotency`.

**Rate limiting.** Redis sliding-window/GCRA limiter (`core.ratelimit.allow(key, limit, window) -> (ok, remaining, retry_after)`), used by auth (register 5/hour/IP, login 10/15 min/IP + 5/15 min/account (Go `auth_handlers.go` values), MFA verify 5/15 min/session and 20/day/user, SSO start 30/min/IP, SCIM 600/min/directory, public generator per plan, API keys per plan `api_requests_per_min`, password gate 10/10 min/IP/code). Responses carry `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (IETF draft headers) on API-key traffic. When Redis is down, auth limits **fail closed** (503) and API-key limits fail open with a log line.

**Client IP.** Behind Railway the direct peer is Railway's edge, which sets `X-Real-IP` to the IP that connected to it. Rules:
1. On the `redirect` service, if `X-QRit-Edge-Secret` equals `EDGE_SHARED_SECRET` (constant-time compare), trust `X-QRit-Client-IP` and the `X-QRit-Geo-*` headers set by the Cloudflare Worker.
2. Otherwise use `X-Real-IP` if the service is on Railway (`RAILWAY_ENVIRONMENT_NAME` set); if that IP is in Cloudflare's published ranges (list vendored in `core/net/cloudflare_ranges.py`, refreshed by a monthly task), use `CF-Connecting-IP`.
3. Never trust `X-Forwarded-For` blindly (v1 defect 7). Store only an IP prefix (/24 IPv4, /48 IPv6) in audit and sessions (port `audit.TruncateIPToPrefix`).
4. **Geo** comes only from the edge headers of rule 1. One exception, for legacy `/r/` traffic reaching the api service directly (no Worker in front): use `CF-IPCountry`, `cf-region-code`, `cf-ipcity` when the peer is in Cloudflare's ranges (a v1 custom domain proxied by Cloudflare); otherwise, and only when the code's rules need the country, call `LEGACY_GEOIP_LOOKUP_URL` (v1's `GEOIP_LOOKUP_URL` contract) through the SSRF-safe client with a 1.5 s timeout, caching the country per IP prefix in Redis for 24 h. No answer → unknown country (v1 allow-lists fail open, block-lists don't match).

**Outbound HTTP (SSRF guard).** `core.http.safe_client(allow_private=False)` returns an `httpx.Client`/`AsyncClient` whose transport resolves DNS itself and refuses loopback, private, link-local (incl. 169.254.169.254), CGNAT 100.64/10, multicast, unspecified, `0/8` and `240/4` **after resolution** (connect to the resolved IP with SNI/Host of the original name, so DNS rebinding can't bypass it); no redirects; 10 s timeout; response size cap 1 MB unless raised. `allow_private=True` only when `APP_ENV in {"local","test"}`. Used for webhooks, integrations, IdP discovery/JWKS/token, DoH, Web Risk, SIEM streams, destination health checks. Port `reference/go-v2/internal/netutil/guard.go` behaviour and `internal/sso/sso_test.go::TestSSRFGuard`.

**Workspace scope / RLS.** `core.db.workspace_scope(ws_id)` = context manager: `transaction.atomic()` + `SELECT set_config('app.workspace_id', %s, true)`. The DRF base view `WorkspaceScopedAPIView` resolves `{ws}` (UUID or slug), runs the access engine and identity gate, then executes the handler inside `workspace_scope`. Port `TestRowLevelSecurity` (a query without a `workspace_id` filter inside the scope must not see other workspaces' rows when connected as `qrit_app`).

**Envelope encryption.** `core.crypto.Keyring`: `ensure(org_id)` (create the first DEK if missing; idempotent), `encrypt(org_id, plaintext) -> bytes`, `decrypt(org_id, ct)`, `rotate(org_id)`, `blind_index(org_id, value)` (= HMAC-SHA256 keyed with HKDF-SHA256(`APP_ENCRYPTION_KEY`, salt = org id bytes, info = `qrit-bidx-v1`) over `lower(trim(value))`, as in Go), `seal_platform(plaintext, label)`, `open_platform(ct, label)`. `manage.py rotate_kek` (reads the new key from `APP_ENCRYPTION_KEY_NEXT`) re-wraps every DEK, re-seals platform secrets and recomputes blind indexes in batches; afterwards the variables are swapped. AES-256-GCM, 12-byte nonce, AAD = org id bytes, ciphertext = `key_id (4 bytes BE) || nonce || ct`; DEKs in `org_data_keys` wrapped with a KEK = HKDF-SHA256(`APP_ENCRYPTION_KEY`, info=`"qrit/kek/v1"`). Port `reference/go-v2/internal/envelope` + `TestEnvelopeAndFlags`. Ciphertexts written by Go are not migrated (v2 has no production data), so byte-compatibility is not required, but the format above is fixed.

**Feature flags.** `feature_flags(key, org_id NULL=global, enabled)`; org value overrides global; cached 60 s; `core.flags.enabled(key, org_id)`. Port `internal/flags`.

**Events outbox.** `core.events.emit(tx_ctx, workspace_id|org_id, event, data)` defers the procrastinate task `integrations.fanout` in the current transaction. Event names are stable strings (Appendix C of the enterprise plan + §7.14).

**Health.** `/healthz` (process up, no dependencies) and `/readyz` (DB `SELECT 1`, Redis `PING`, migrations applied check cached 60 s) on api and redirect. Railway's health check host `healthcheck.railway.app` MUST be in `ALLOWED_HOSTS`.

**Storage.** `core.storage` wraps an S3-compatible bucket (`STORAGE_ENDPOINT`, `STORAGE_BUCKET`, `STORAGE_ACCESS_KEY_ID`, `STORAGE_SECRET_ACCESS_KEY`, `STORAGE_REGION`, `STORAGE_PUBLIC_BASE_URL` for public assets like logos). Private objects are served by **signed URLs** (SigV4 query presign, 15 min default, 7 days for report links). Implement presigning with the SigV4 code ported from `reference/go-v2/internal/auditstream/s3.go` (verified against AWS's published vector) — no boto3.

**Email.** `core.email.send(to, template, context, org_id=None)`: Resend API; templates in `apps/*/templates/email/*.txt|html` (plain text + simple HTML); white-label sender when the org has a verified email domain (§7.20). Local/test: `EmailBackend` that records messages (tests assert on it).

**Settings & env.** `qrit/settings/env.py` uses pydantic-settings; all variables in Appendix A. `APP_ENV` ∈ `local|test|staging|production`; outside `local`/`test` each settings module refuses to boot without its own required secrets: **all** Python services need `DJANGO_SECRET_KEY` and `APP_ENCRYPTION_KEY`; **api** also `JWT_ED25519_PRIVATE_KEY`, `COOKIE_SECURE=true` and `SCAN_SALT_SECRET` (it serves legacy `/r/`); **redirect** also `EDGE_SHARED_SECRET` and `SCAN_SALT_SECRET`; **worker** also `SERIAL_MAC_KEY`. `SECURE_SSL_REDIRECT` stays **off** (Railway's edge serves HTTPS publicly, while health checks and the web → api private-network calls are plain HTTP); `SECURE_PROXY_SSL_HEADER = ('HTTP_X_FORWARDED_PROTO', 'https')`; silence `security.W008` with that reason.

### 7.2 accounts (users, sessions, MFA)

- **Custom user model** `accounts.User` (`AUTH_USER_MODEL`), `AbstractBaseUser` with `password` mapped to `password_hash` (nullable) and `last_login` mapped to `last_login_at`. `PASSWORD_HASHERS = [Argon2PasswordHasher, BCryptPasswordHasher]` — v1 bcrypt hashes are imported as `bcrypt$<v1 hash>` and upgraded to argon2 on the next successful login (Django does this automatically when the stored hasher isn't the first). Password policy: ≥ 10 chars (org policy may raise it), not in the common-password list (port v1 `COMMON_PASSWORDS` + Django's `CommonPasswordValidator`), not equal to email.
- **Tokens and cookies** (port `reference/go-v2/internal/auth`): access token = EdDSA (Ed25519) JWT, 10 min, claims `sub`, `sid`, `iat`, `exp`, `iss="qrit"`, optional `sg` (staff grant); refresh token = opaque 32 random bytes, sha256 stored in `sessions.refresh_token_hash`, 30 days, **rotation with reuse detection** (reuse revokes the whole family). Cookies: `qrit_access` (httpOnly, `SameSite=Lax`, path `/`), `qrit_refresh` (httpOnly, `SameSite=Strict`), `qrit_csrf` (readable, `SameSite=Lax`); all `Secure` when `COOKIE_SECURE`. CSRF double-submit: non-safe methods with cookie auth need `X-CSRF-Token == qrit_csrf`. Bearer JWTs (API clients) skip CSRF. Staff tokens are bearer-only.
- **Session guard:** a middleware loads the session row (cached 20 s, evicted on revoke), rejects revoked/expired sessions, updates `last_used_at` at most once a minute, passes the pre-request session to the identity gate (idle detection). Refresh copies `auth_method`, `sso_connection_id`, `mfa_verified_at`, `step_up_at` to the rotated session.
- **Endpoints:** register, login (returns `{user, mfa_required}` and sets cookies; lockout after `AUTH_MAX_LOGIN_ATTEMPTS` = 5 failures for 15 min per account + per-IP limits), logout, refresh, verify email, resend verification, forgot/reset password (tokens in `email_tokens`, sha256, 1 h / 24 h), magic link (optional, flag `magic_link`), Google sign-in (`POST /v1/auth/google {credential}` verifies the Google ID token against `GOOGLE_CLIENT_ID` using Google's JWKS via Authlib/joserfc; links by `oauth_accounts(provider='google', provider_user_id=sub)`; never links to an existing password account unless the Google email is verified **and** the user signs in with the password once to confirm — show a "link accounts" flow), `/me` (includes `organizations[]`, `workspaces[]` with slugs, `mfa`), `PATCH /me`, `POST /me/password`, sessions list/revoke.
- **MFA** (port `identity.go`, `mfa_handlers.go`, `TestMFA`): TOTP enrol (secret held in Redis 10 min until confirmed; stored sealed with `seal_platform(label="totp:<user_id>")`), confirm (first factor issues 10 recovery codes `XXXX-XXXX`, sha256 stored), verify at login (`/v1/auth/mfa/verify` with `code` or `recovery_code`, each TOTP code usable once via Redis `totp:used:{factor}:{code}` 2 min), MFA-pending sessions are confined to `/v1/auth/mfa/*`, `/v1/auth/logout|refresh`, `GET /v1/me` (403 `mfa_verification_required` otherwise), remove factor (step-up; blocked with 409 `mfa_required_by_org` if an org requires MFA and it's the last factor), regenerate recovery codes (step-up).
- **Passkeys (new vs Go):** WebAuthn registration/authentication as a second factor with `webauthn` 3.0: `POST /v1/me/mfa/webauthn/options`, `POST /v1/me/mfa/webauthn` (verify attestation `none`, store `credential_id`, `public_key`, `sign_count`, `aaguid`, `transports`), login challenge `POST /v1/auth/mfa/webauthn/options` + verify via `/v1/auth/mfa/verify {webauthn: {...}}`. RP ID = `APP_DOMAIN` (and white-label hosts are separate RPs — passkeys registered on one host don't work on another; document it). Challenge stored in Redis 5 min.
- **Step-up:** `POST /v1/auth/step-up {password, code?}` sets `sessions.step_up_at`; `requires_step_up(minutes=10)` decorator returns 401 `step_up_required` with `instance="/v1/auth/step-up"`. SSO-only users get 403 `sso_reauth_required` → re-authenticate via `/v1/auth/sso/start?prompt=login`.
- **Tests:** port `TestAuthLifecycle`, `TestMFA`; add Google linking, bcrypt→argon2 upgrade, lockout, passkey registration/assertion with `webauthn` test vectors.

### 7.3 orgs

Organisations (personal org auto-created at registration, named after the user), members (`org_owner|org_admin|billing_admin|member`, status `active|suspended|deprovisioned`, source `invite|sso_jit|scim|creator`), transfer ownership (step-up), org security policy (`GET/PUT /v1/orgs/{org}/security-policy`: enforce SSO + break-glass owners (≤ 2, must have MFA), require MFA, allowed MFA kinds, session idle/max, dashboard and API IP allowlists (must include the caller's IP), password min length, invite email domains, API key max days, export permission), claimed domains (§7.6). Port `org_handlers.go`, `policy_handlers.go`, `internal/org`, `TestOrganisationsAndAccess`. GSTIN validation with checksum (port `validGSTINChecksum`).

### 7.4 workspaces

Workspaces under an org (count limited by plan), slugs (`^[a-z0-9](-?[a-z0-9])*$`, 3–48 chars, reserved list), members (primary system role `owner|admin|editor|reviewer|analyst`; exactly one owner; mirrored into `role_bindings` by `orgs.services.bind_member_role`), invites (token = 32 random bytes base32, sha256 stored, 7 days, email must match on accept, org `invite_email_domains` enforced, seat limit), leave, transfer ownership, workspace policy (`GET/PUT /v1/workspaces/{ws}/policies`: allowed/blocked destination hosts, require HTTPS, approval mode, approvals required 1–3, approval expiry hours, require template, pixel consent mode, disabled features), sandbox workspace (§7.21). Port `ws_handlers.go`, `TestTenantIsolationAndTeams`.

### 7.5 access (permissions, roles, groups, bindings, gate)

Port exactly: `internal/authz` (catalogue, system roles, org role permissions, API key scope map, `Grants` with folder scope), `internal/access/engine.go` (resolution order in §3.4; suspended/deprovisioned members get nothing; agency parent admins get admin), `gate.go` (identity gate + billing hold), `governance_handlers.go` (custom roles: catalogue-only, no `*`, `workspace.read` auto-added, reserved keys → 409, delete while bound → 409 `role_in_use`; groups: manual groups editable, SCIM/SSO groups read-only → 409 `group_managed_externally`; bindings: user or group, workspace or folder scope, **no privilege escalation** (caller must hold every permission of the role at that scope; `owner` only by owners), a user's workspace-wide built-in role is managed via members → 409 `use_membership`; explain access; access review JSON/CSV with step-up and a recorded completion in `organizations.settings.access_review`). Tests: port `TestAccessGovernance`; add: group removal revokes access within one request (cache invalidation).

### 7.6 identity (SSO, claimed domains, SCIM)

- **Claimed domains:** `POST /v1/orgs/{org}/domains {domain}` (reject public mail domains — port the list from `internal/sso/dns.go`); TXT record `_qrit-challenge.<domain>` = `qrit-domain-verification=<token>`; verify on demand (`POST …/verify`) and in the background (`identity.verify_pending_domains`, every 10 min for 72 h, backing off to hourly after 12 tries); several orgs may hold **pending** claims but only one can hold a domain **verified** (partial unique index `org_domains_verified_uniq`). Lookups via DNS-over-HTTPS (`DOH_URL`, default Cloudflare) through the SSRF-safe client.
- **OIDC** (Authlib): discovery (issuer must match exactly, cached 1 h), PKCE S256, `state` (Redis `sso:state:{state}` 10 min, single-use GETDEL), `nonce`, ID token verification (RS256/ES256/PS256 via JWKS with refresh on unknown `kid`, `iss`, `aud`, `exp` required, `iat`, `azp` when multiple audiences, 2 min leeway); client secret stored encrypted with the org DEK. Port `internal/sso/oidc.go` rules and `internal/sso/sso_test.go::TestVerifyIDToken` negative cases (wrong nonce/aud/iss, expired, no exp, foreign key, unknown kid, `alg=none`, HS256 confusion).
- **SAML** (pysaml2, new vs Go which used Polis): SP-initiated only (IdP-initiated rejected: `InResponseTo` must match a stored request id); signed assertions **and** signed responses required when the IdP supports it; audience = SP entity id `https://API_DOMAIN/v1/auth/saml/metadata/{connection_id}`; ACS `POST /v1/auth/saml/acs/{connection_id}`; clock skew 2 min; replay cache of assertion IDs in Redis until `NotOnOrAfter`; metadata by XML upload or HTTPS URL (fetched with the SSRF-safe client, refreshed daily); attribute mapping (`email`, `firstName`, `lastName`, `groups`) per connection; NameID persistent preferred. Test with a fake IdP built from pysaml2 test utilities (sign with a generated key) — cover: valid login, unsigned assertion rejected, wrong audience, expired, replayed assertion, IdP-initiated rejected, XML signature wrapping attempt rejected.
- **Connections:** `sso_connections` CRUD (step-up), status `draft → testing → active | disabled`; **test login** by an admin marks `test_passed_at` and activates; `/v1/auth/sso/start {email|org, redirect, prompt}` picks the org's active connection (by verified email domain or org slug); callback handles both protocols.
- **Linking and JIT** (port `linkSSOUser`): existing identity link → login; else existing account whose email domain the org has **verified** → link; existing account on an unverified domain → refuse (`sso_account_exists`); unknown user + JIT on → create (email verified) + org membership (`source='sso_jit'`) + default workspace role; JIT off → `sso_user_not_provisioned`. Entra: when `email` is missing, use `preferred_username`/`upn` but only on a verified domain; an OIDC email not vouched for (`email_verified` false/missing) is accepted only on a verified domain. Group sync from the configured claim: SSO-sourced groups only (never touch SCIM/manual groups of the same name).
- **SSO enforcement** in the identity gate: password sessions of an org with `enforce_sso` get 403 `sso_required` for that org's resources (other orgs unaffected); break-glass owners with verified MFA exempt.
- **SCIM 2.0** (custom DRF, port `scim.go` + `TestSCIM` + `TestSCIMFilterParser` + `TestSCIMUserPatch`): `/scim/v2/{ServiceProviderConfig,ResourceTypes,Schemas,Users,Groups}`; bearer token per directory (`scim_` + 32 base32 chars, sha256 stored, shown once, rotatable); filters `eq` joined by `and` on `userName` (case-insensitive), `externalId`, `id`, `emails[type eq "work"].value`, `displayName`; pagination `startIndex`/`count` (max 200); ETags `W/"n"` + `If-Match` → 412; Entra/Okta PATCH quirks (capitalised ops, `"True"/"False"` strings, path-less object values, `emails[type eq "work"].value` paths, enterprise extension attributes); `active=false` → org membership `suspended` and **sessions revoked for SCIM/SSO-managed accounts**; `DELETE` → `deprovisioned` or removed (directory setting); Groups PATCH answers 204 and is serialised per directory with an advisory lock; SCIM adopts an SSO-sourced group of the same name, conflicts with manual ones; existing accounts outside a verified domain can't be taken over (409). Rate limit 600/min/directory.

### 7.7 qr (codes, versions, domains, organisation)

**Content types** (`qr_codes.content_type`) and encoders — port `reference/go-v2/internal/qr/content.go` (and its 9 tests) and v1 `api/utils/qr.py` behaviour:

| Type | Static | Dynamic | Encoded / served as |
|---|---|---|---|
| `url` | ✓ | ✓ | URL (static) / short link → destination |
| `text` | ✓ | — | raw text (≤ 1,200 bytes) |
| `email` | ✓ | ✓ | `mailto:` (RFC 6068) / short link → `mailto:` |
| `phone` | ✓ | ✓ | `tel:+E164` |
| `sms` | ✓ | ✓ | `SMSTO:+E164:msg` |
| `whatsapp` | ✓ | ✓ | `https://wa.me/<digits>?text=` |
| `wifi` | ✓ | — | `WIFI:T:WPA;S:..;P:..;H:..;;` with escaping |
| `vcard` | ✓ | ✓ | vCard 3.0 text (static) / hosted `vcard` page with "Save contact" (dynamic) |
| `event` | ✓ | ✓ | iCalendar VEVENT / hosted `event` page |
| `upi` | ✓ | — | `upi://pay?pa=&pn=&am=&tn=&cu=INR` (validate VPA) |
| `location` | ✓ | — | `geo:lat,lng` |
| `links_page` | — | ✓ | hosted `links_page` |
| `file` | — | ✓ | hosted `file` page (PDF/image in storage, virus-scan flag optional) |
| `app_store` | — | ✓ | smart link: iOS → App Store URL, Android → Play URL, else fallback (rules preset) |
| `gs1` | — | ✓ | GS1 Digital Link served by the resolver (§7.17) |
| `form` | — | ✓ | hosted form (§7.15) |
| `serial_batch` | — | ✓ | anchor code for a serial batch (§7.18) |

Hosted page kinds (`qr_versions.hosted_page.kind`): `vcard`, `links_page`, `file`, `event`, `menu` (port `internal/hosted` validation + its tests; limits: ≤ 30 links, ≤ 64 KB JSON, HTTPS-only external URLs, text fields length-limited), plus `form` (renders the form referenced by id).

**Codes.** Create (static or dynamic; dynamic requires plan capacity — serialize the quota check per workspace with `pg_advisory_xact_lock(hashtextextended('qr-quota:'||ws,0))`), list (cursor, search trigram on name/short code, filters status/mode/folder/campaign/tag; folder-scoped users see only their subtree), get, update (name, folder, campaign, tags, design, lifecycle fields; static codes are immutable except name/folder/tags/design), pause/resume/archive/unarchive, delete (soft: the row and its short code stay; `qr.purge_deleted` hard-deletes after 30 days and writes the short code to `short_code_tombstones` **forever** — a short code is never reused, as in Go), restore, `resolve-preview` (evaluate rules for synthetic facts). Password-protected codes store argon2 `password_hash`. Design validated against DesignV1 (§7.8) and optional locked template (`template_locked` unless `qr.design.bypass_lock`).

**Short codes.** 7 characters of Crockford base32 `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (35 bits), case-insensitive on input (normalise to upper case, map `O→0`, `I/L→1`), denylist of offensive substrings, unique per domain, retried up to 8 times inside a savepoint on collision. Encoded payload for dynamic codes: `HTTPS://<HOST>/<CODE>` (upper-case so the QR uses alphanumeric mode). Port `internal/shortcode` + tests. Legacy v1 codes keep `legacy_short_code` and are served only at `/r/<code>` on legacy hosts (§7.9).

**Versions.** Immutable rows; `POST /versions` creates version N+1 with `effective_at` now or scheduled (Pro+, ≤ 2 years ahead); the current pointer moves when effective; restore = new version copying an old one; cancel a scheduled version (not pending-approval ones); the approvals gate (§7.11) may hold a version (202). Each version validates destination URLs with the URL-safety policy (scheme allowlist, no private hosts, no own short domains, shortener chains blocked unless the workspace allows, workspace allow/block lists, `require_https`) and Web Risk (`safe|pending|unsafe`; unsafe → 422 `destination_unsafe`; timeouts → `pending` and a worker rescans). Port `buildDraft`, `ApplyVersion`, `commitVersion`, `internal/urlsafety` + 3 test files, `internal/version` + tests, `TestQRCodeLifecycle`.

**Rules engine.** Port `internal/routing` exactly: `Rule{id, name, enabled, when: {all|any: [{field, op, value}]}, destination_url, split: [{variant, weight, destination_url}], block}`; fields `country, region, device_type, os, language, local_time, weekday, date, scan_count`; first matching enabled rule wins; `split` is deterministic per visitor exactly as in Go (bucket = big-endian uint32 of the first 4 bytes of `sha256(qr_code_id ‖ client_ip ‖ user_agent)` mod 100; the IP is used only in memory); a split variant may be `{"variant": "rest", "weight": N, "fallthrough": true}` (no destination) meaning "continue with the next rule" — needed for imported v1 weight rules (`V1_TO_V3_MIGRATION.md` C8); `block` → 451 page. Operators per field (v3 validates strictly; Go accepted any operator and the engine ignored unknown ones — if a ported Go test sends another operator, keep Go's behaviour for that case and record a decision): `country`, `region`, `device_type`, `os`, `language` → `in`/`not_in` with a list of strings; `weekday` → `in`/`not_in` with ISO days 1–7; `local_time` → `between` with `["HH:MM","HH:MM"]` (may wrap midnight); `date` → `between` with `["YYYY-MM-DD" or "", "YYYY-MM-DD" or ""]`; `scan_count` → `gte`/`gt`/`lt`/`lte` with an integer; `when` uses either `all` or `any`, not both. ≤ 50 rules (Go allowed 20; raised so imported v1 codes fit), ≤ 10 conditions each, ≤ 5 split variants with weights summing to 100. UTM append adds only absent keys (`internal/version.AppendUTM`).

**Domains.** Platform domain row for `SHORT_DOMAIN` ensured at startup (`workspace_id NULL`). Custom short domains (Business+): `POST /v1/workspaces/{ws}/domains {hostname}` → create a Cloudflare for SaaS custom hostname (API `POST /zones/{CF_ZONE_ID}/custom_hostnames` with `ssl: {method: "http", type: "dv"}`), show the CNAME target `CF_SAAS_CNAME_TARGET` (e.g. `customers.qrit.link`), poll status every 5 min for 72 h (`qr.poll_custom_domains`), `status/tls_status` from Cloudflare; `root_redirect_url` and `not_found_url` per domain; delete removes the Cloudflare hostname. Changing a code's domain is not allowed after creation (printed codes). Publish `domain:invalidate` on change.

**Organisation of codes.** Folders (tree ≤ 8 levels, no cycles, move, delete only when empty or with `?cascade=move_to_parent`), tags, campaigns (dates, goal, UTM defaults applied to new versions), templates (design presets; `is_locked` enforces design for editors). Port `organize_handlers.go` (tags audited).

**Plan read-only.** When a workspace exceeds its dynamic-code limit after a downgrade, the newest codes beyond the limit become `is_read_only` (still redirect; edits return 402 `read_only_over_limit`). Port worker `plans.read_only`.

**Trash/purge.** Soft-deleted codes restorable for 30 days; `qr.purge_deleted` daily.

### 7.8 render (Python QR renderer)

**Design schema — DesignV1** (port `reference/ts-render/qr-render/src/design.ts` exactly):

```
v: 1
ecc: auto | L | M | Q | H                 (auto = M, or H when a logo is present)
quiet_zone: 0..10 (default 4)
modules: {shape: square|dots|rounded|extra-rounded|classy|classy-rounded, color: #RRGGBB,
          gradient?: {type: linear|radial, rotation: 0..359, stops: [{offset 0..1, color}] (2..5)}}
finder: {outer_shape: square|rounded|circle|leaf, inner_shape: square|rounded|circle|dot,
         outer_color: #RRGGBB, inner_color: #RRGGBB}
background: {color: #RRGGBB, transparent: bool}
logo?: {file_id?: uuid, url?: https URL (legacy only), size_ratio: 0.10..0.30, padding: 0..4,
        clear_modules: bool, shape: square|circle}
frame?: {style: banner-bottom|banner-top|rounded-box|speech, text: ≤ 24 chars, text_color, color}
```

Canonicalisation (lower-case colours, sorted keys, defaults filled) produces `design_hash` = sha256 of canonical JSON (port `canonical.ts`).

**Pipeline** (`apps/render/`):
1. `matrix.py` — `segno.make(payload, error=<ecc>, micro=False, boost_error=False)`; expose `modules: list[list[bool]]`, `version`, `ecc`. Payloads for dynamic codes are upper-case URLs (alphanumeric mode).
2. `geometry.py` — pure functions producing a list of primitives (`Rect`, `RoundedRect`, `Circle`, `Path(d)`, `Text`, `Image`) in module units: data modules per shape (port `renderModulePath` / `shapes.ts`), finder outer/inner shapes (`finder.ts`), logo box and module clearing (`logo.ts`), frame layout (`frame.ts`), gradients (`gradient.ts`).
3. Backends: `svg.py` (string builder; every attribute value escaped; logos embedded as `data:` URIs of re-encoded PNG from storage — never remote URLs, never user SVG), `png.py` (`resvg_py.svg_to_bytes(svg_string=..., width=...)`), `pdf.py` (ReportLab canvas drawing the same primitives as vectors; RGB, or CMYK when requested: `DeviceCMYK` colours via a documented naive conversion with rich-black option), `eps.py` (ReportLab `renderPS.drawToFile` of a `Drawing` built from the primitives).
4. `scannability.py` — port `contrast.ts` + `warnings.ts::calculateScannability` **exactly** (score, label, `isBlocked`, warning codes, severities and thresholds as in the TS source — e.g. contrast < 2 → error `contrast_too_low` (blocked), < 4 → warning `low_contrast`, < 7 → score only; inverted → warning `inverted`; quiet zone < 2 → warning `quiet_zone_small`, < 4 → score only; logo > 0.30 → error `logo_too_large` (blocked), > 0.25 → warning `logo_large`). The golden tests compare these warnings with the TS output.
5. `print.py` — print size calculator (recommend ≥ max(2 cm, scan distance ÷ 10), module ≥ 0.4 mm) and label sheets (A4/Letter; Avery L7160, L7163, 5160; A5 table tent; 4×6 in; optional crop marks and 3 mm bleed; caption = code name + short URL) as ReportLab PDFs.

**Endpoints:** `POST /v1/render/preview {payload|qr_id, design}` → `{svg, version, ecc, warnings}` (≤ 30 req/10 s per user; responses cached in Redis by `(payload, design_hash)` for 10 min); `GET /v1/workspaces/{ws}/qr-codes/{id}/download?format=svg|png|pdf|pdf_cmyk|eps&size=64..4096` (audited as `qr.downloaded` only for bulk; PNG default 1024 px); `POST /v1/public/render` for the public (logged-out) generator: static codes only, ≤ 1024 px, `FREE_TIER_DAILY_LIMIT` renders per IP per day (default 10, as in v1). Bulk ZIP downloads are jobs.

**Golden fixtures (before the TS renderer is archived, P3):** run `reference/ts-render` over a design matrix of ≥ 40 cases (every shape × finder combo, gradients, logo sizes, every frame, transparent background, each ECC) and commit `backend/apps/render/tests/fixtures/<case>.{json,svg,png}`. Python tests assert for each case: (a) the Python PNG decodes to the same payload with zxing-cpp at 256 px and 1024 px; (b) the same warnings are produced; (c) structural checks on the SVG (finder shapes present, gradient defs, frame text escaped); (d) visual similarity ≥ 0.90 SSIM vs the TS PNG when both use the same QR version/mask (force segno `mask` to the TS mask recorded in the fixture). Exact byte equality is not required.

**Security:** SVG output never contains user-controlled raw markup (text escaped with `xml.sax.saxutils.escape` + quote escaping for attributes); logos are raster only (PNG/JPEG/WebP ≤ 2 MB, re-encoded to PNG ≤ 1024 px, EXIF stripped); ReportLab remote image fetching stays disabled.

### 7.9 redirect (hot path, legacy links, special paths)

**Routes** (`qrit/urls/redirect.py`, all `async def`):

| Path | Methods | Behaviour |
|---|---|---|
| `/{code}` | GET, HEAD, POST | Dynamic code on the request's domain (POST = password form submit). Codes are case-insensitive and normalised. A trailing `+` (`/{code}+`) shows the preview page (destination shown, no redirect, no counted scan) as in Go |
| `/p/{code}`, `/p/{code}.vcf`, `/p/{code}.ics` | GET, HEAD | Hosted page of a dynamic code (vcard, links, file, event, menu, form); `.vcf`/`.ics` downloads for vCard/event pages, as in Go |
| `/r/{code}` | GET, HEAD, POST | **Legacy v1 link**: lookup `legacy_short_code` (case-sensitive, exact) where the host is in `LEGACY_HOSTS` or equals the code's `legacy_host`; then the same pipeline |
| `/v/{serial12}` and `/V/{serial12}` | GET | Serial verification entry (§7.18) |
| `/01/{gtin}[/22/..][/10/..][/21/..]`, `/.well-known/gs1resolver` | GET, HEAD, OPTIONS | GS1 resolver (§7.17) |
| `/_px/{code}` | POST | Pixel consent beacon (§7.16) |
| `/` | GET | Domain's `root_redirect_url` or the marketing site |
| `/robots.txt` | GET | `User-agent: *` + `Disallow: /` |
| `/healthz`, `/readyz` | GET | Health |

**Pipeline** (port `reference/go-v2/internal/redirect/server.go`, `internal/resolve`, `TestScanPipelineEndToEnd`):
1. Resolve domain by host (cached map hostname → domain id, refreshed on `domain:invalidate`). Unknown host → 404 page.
2. Resolve link (§3.3 caches). DB unavailable and no cached copy → 503 "Temporarily unavailable" page (never a blank error); a last-known-good copy up to 24 h old MAY be served when the DB is down.
3. Lifecycle gates in order (port `resolve.EvaluateState`): safety `blocked` → 410 "Link unavailable"; `paused` → fallback URL (302) or 410 "This QR code is not active"; not started (`starts_at` future, or no approved version yet) → fallback or 404 "Not active yet"; expired → fallback or 410; scan limit reached → fallback or 410; password → 200 form (POST verifies argon2; wrong → 401 form with error; > 10 tries/10 min per IP prefix per code → 429).
4. Rules (country/region/city from edge headers; device/os/browser from the ported Go classifier (§4.1); language from `Accept-Language`; local time in the workspace timezone; scan count from the counter). `block` → 451 "Not available here".
5. Destination: UTM append; hosted page → render template (`/p/{code}` semantics); pixels attached → interstitial (§7.16); else `302 Found` with `Location`, `Cache-Control: private, no-store, max-age=0`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-Robots-Tag: noindex, nofollow` (the Go values asserted by `pipeline_test.go`).
6. Bots and link previewers (port `internal/scan/botdetect.go` list; datacenter ASN flag from the edge header `X-QRit-Geo-ASN` against a vendored list) are redirected normally but marked `is_bot`, and never increment scan limits.
7. HEAD returns the same status and `Location` without emitting a counted scan.
8. Emit the scan event (schema `internal/scan/scan.go::ScanEvent`: `id` UUIDv7, `ts`, `ws`, `qr`, `ver`, `cmp`, `dom`, `rule`, `out`, `m`, `vh` (base64 of 16-byte HMAC-SHA256 with the **daily salt** = HMAC-SHA256(`SCAN_SALT_SECRET`, UTC date) over `qr_id || 0x00 || ip || 0x00 || ua`), `ua` (for parsing in ingest only; not stored), `dc`, `geo`, `lang`, `ref` (host only), `utm`, `serial?`).

**Edge headers** (set only by the Worker, trusted only with the shared secret): `X-QRit-Host`, `X-QRit-Client-IP`, `X-QRit-Geo-Country`, `X-QRit-Geo-Region`, `X-QRit-Geo-City`, `X-QRit-Geo-ASN`, `X-QRit-Geo-Timezone`. Without the Worker (local dev, direct Railway domain) geo is empty and rules on geo don't match (documented).

**Pages:** Django templates in `apps/redirect/templates/redirect/*.html`, inline CSS only, no JS except the password form and the pixel interstitial, `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; img-src data: https:; form-action 'self'` (interstitial has its own CSP), workspace branding (logo, colour) when white-label is on, "Powered by QRit" unless the org has `hide_platform_brand`, or the plan includes `remove_branding` and the workspace has `brand.hide_branding` (or the hosted page has `hide_branding`, as in Go `internal/hosted`).

**Performance gate:** §3.5 budget, measured in CI against a docker-compose stack with k6 (`deploy/k6/redirect_load.js`, thresholds `http_req_duration{expected_response:true}: p(95)<25`) and again on staging.

### 7.10 analytics (scan pipeline and queries)

- **Partitions:** monthly partitions of `scan_events` created 1 month back to 3 months ahead (`analytics.ensure_partitions` at boot + daily); rows that landed in the default partition are moved when a partition is created (port `worker/maintenance.go::EnsurePartitions`).
- **Ingest consumer** (`manage.py ingest_scans`): consumer group `ingest` on stream `scans`; batch ≤ 1,000 events or 1 s; parse UA once (ported Go classifier), derive `device_type` (`mobile|tablet|desktop|other`; `bot` → `is_bot` + `other`, `unknown` → NULL), `os`, `browser` + versions, `referrer_host`, `language`; **dedupe** counted scans with the Redis Lua script (same visitor + code within 10 s — Go's default window — → `is_duplicate`, only for countable non-bot outcomes); `is_unique` = first counted scan of visitor/code/UTC day (`scan_visitors_daily` upsert); insert events with `COPY`/`execute_values`; upsert `scan_stats_15m` and `scan_stats_daily_dim` (dimensions exactly as the schema CHECK and Go: `country, region, city, device, os, browser, language, referrer, rule, version, utm_source`; city key = `CC/City`); increment `qr_codes.total_scans/unique_scans/last_scanned_at` in one statement per batch; ack after commit; `XAUTOCLAIM` messages idle > 60 s; after 5 deliveries move to `scans:dlq`. `ingest_scans --rebuild-day YYYY-MM-DD` recomputes rollups from events. Port `internal/ingest` (Process, WriteBatch, Reclaim, Rebuild) and `ingest.sql` from the blueprint.
- **Realtime:** per-minute Redis counters `rt:{ws}:{minute}` (TTL 2 h) updated by ingest; `GET /v1/workspaces/{ws}/realtime` returns the last 60 minutes (port `internal/realtime`).
- **Queries** (port `internal/analytics` + `docs/v2-blueprint/analytics_queries.sql`; all read rollups, never raw events except the scan log): `summary` (scans, unique, bots excluded, change vs previous period), `timeseries` (granularity 15m|hour|day|week with timezone-aware bucketing; 15m only for ranges ≤ 7 days), `breakdown?dimension=` (the same 11 keys; anything else → 422 `invalid_dimension`), `top-codes`, `heatmap` (weekday × hour in workspace tz), `scans` (raw scan log, Business+ `raw_scan_log`, cursor pagination, no visitor hash), `export` (CSV, feature `csv_export`, formula-injection safe cells). History window limited by plan `analytics_history_days`. Folder-scoped users only see codes they can read.
- **Retention:** raw `scan_events` kept per plan (free 30 days → enterprise contract); rollups kept for the plan window; partitions older than the maximum retention are dropped by `analytics.drop_old_partitions`.
- **Geo:** country/region/city/timezone/ASN from the Cloudflare Worker headers (Cloudflare `request.cf`). No GeoIP database is shipped. (If direct traffic without Cloudflare must be geolocated later, add DB-IP Lite MMDB — CC BY 4.0 — with attribution; record the decision.)
- **Tests:** port `TestScanPipelineEndToEnd` (use the corrected expectation of 11 stored events from the Go test, not the blueprint's 13), `internal/ingest` tests, `internal/analytics` tests; add timezone bucketing tests (IST, America/New_York DST).

### 7.11 approvals (maker–checker)

Port `reference/go-v2/internal/approval` (pure `Evaluate`, `Hosts`, `Describe`) and `httpapi/approval_handlers.go` + `worker/governance.go`, tests `TestApprovals` and `approval_test.go`.

- **Evaluation** by workspace `approval_mode`: `off`; `outside_allowlist` (any http(s) URL in the draft — destination, rule and split destinations, hosted-page `url`/`website` links — whose host doesn't match `allowed_destination_hosts` → reason `host_not_allowlisted`; with this mode the allowlist routes to approval instead of hard-blocking); `all_destination_changes` (every new version of an existing code → `policy_all_destination_changes`); `all_changes` (also new dynamic codes → `policy_new_code`). `blocked_destination_hosts` stays a hard 422 `host_blocked`; `require_https` a hard 422.
- **Gate** inside `create code` and `create/restore version` transactions: when reasons exist, insert `approval_requests` (kind `create|destination`, reasons, requester = user or the API key's creator — keys without a creator get 409 `approval_requires_user`, `required_approvals` and `expires_at` from policy, note = change note), insert the version with `approval_status='pending'` **without moving the current pointer**, audit `approval.requested`, defer `integrations.fanout(approval.requested)` and `approvals.notify` — all in the same transaction. Response: 202 `{version, approval: {id, status, reasons, reasons_text, required_approvals, expires_at}}`; for a new code 201 `{qr_code, pending_approval: {...}}`.
- **Never served while pending:** every version-in-effect query filters on approval status (§6.3). A new code awaiting approval shows the "Not active yet" page.
- **Decisions:** `POST /v1/workspaces/{ws}/approvals/{id}/decisions {decision: approve|reject, comment}` needs `qr.destination.approve` on the code's folder chain; the requester can't decide (403 `self_approval`; also enforced by the DB trigger `approval_no_self_approval`); rejects need a comment; API keys can't approve; N-of-M: finalize when approvals ≥ required. **Finalize** (one statement, port the verified `FinalizeApproval` query in `docs/v2-blueprint/enterprise_queries.sql` and Go's `finalizeApproval` in `approval_handlers.go`): request → approved; pending versions → approved with `effective_at = GREATEST(effective_at, now())`; then set the current pointer to the latest servable version, audit `qr.version.activated`, invalidate caches after commit.
- **Cancel** (requester or `policy.manage`), **override** (org owner, step-up, reason ≥ 10 chars, audit `approval.overridden`), **expiry** (`approvals.expire` every 5 min: expired requests → `expired`, versions → `cancelled`, audit + event), **inbox** `GET /v1/me/approvals` (workspaces where the caller holds approve, excluding own and already-decided, folder-aware), list/detail with current vs proposed versions and decisions, `can_decide`.
- **Notifications:** `approvals.notify` emails every eligible approver (org owners/admins + holders of `qr.destination.approve` via direct or group bindings at workspace scope or on the folder chain, minus the requester) and posts to Slack/Teams integrations via fan-out.
- **Bulk (new vs Go):** `bulk_update` jobs (§7.21) create one `approval_requests(kind='bulk_update', job_id)` covering all held versions; the approver sees a diff table and approves/rejects the batch as a whole.

### 7.12 audit (tamper-evident log, export, SIEM streams)

Port `internal/audit`, `httpapi/audit_handlers.go`, `worker/audit.go`, the audit SQL in `00004` (`audit_entry_bytes`, `audit_seal`, `audit_verify`, append-only trigger `audit_logs_guard`, anchors), `internal/auditstream`, `internal/stdwebhook`, `auditstream_handlers.go`, `worker/auditstream.go`; tests `TestTamperEvidentAudit`, `TestAuditStreams`, `auditstream_test.go` (incl. the AWS SigV4 known vector), `stdwebhook_test.go` (the Standard Webhooks spec vector).

- **Writing:** `audit.record(entry)` inside the business transaction; `org_id` defaults from the workspace; actor = user / api_key / system / staff (staff when the principal carries a support grant); `changes` = `{before, after}` of changed fields with secrets redacted by a key denylist (`password`, `secret`, `token`, `key`, `credential`, `client_secret`, …, port `audit.Redact`); IP stored as prefix.
- **Sealing:** `audit.seal` every 5 s → `SELECT audit_seal(org, 1000)` per org with unsealed rows; **anchors** daily (00:10 UTC) store the head hash per org (and upload `{org}/{day}.json` to the WORM bucket `AUDIT_ANCHOR_BUCKET` with object lock when configured); **verify** `GET /v1/orgs/{org}/audit-logs/verify` → `{intact, first_broken_seq?, verified_through_seq, anchored_at}`; **retention** by plan (Business 1 year, Enterprise contract, default 7 years; rows imported from v1 — `request_id='v1-import'` — count their retention from the cutover date) using the dedicated retention path from `00004` (`SET LOCAL qrit.audit_retention='on'`).
- **API:** org audit log list with filters (`workspace_id, actor_id, action, target_id, from, to, cursor`), workspace audit log (`audit.read`), export (`org.audit` + step-up, CSV/JSONL streamed; audited as `audit.exported`), streams CRUD/test/replay (enterprise feature `audit_streams`, ≤ 5 per org).
- **Streams:** kinds `webhook` (Standard Webhooks signing, secret `whsec_…` generated and shown once), `splunk_hec` (`POST {url}/services/collector/event`, `Authorization: Splunk <token>`), `datadog` (`POST https://http-intake.logs.{site}/api/v2/logs`, `DD-API-KEY`), `s3` (SigV4 PUT of `jsonl.gz`, key `{prefix}/{yyyy}/{mm}/{dd}/{hh}/{org}-{first_seq}-{last_seq}.jsonl.gz`, SSE AES256); new streams start at the current max seq; delivery `audit.stream` every 15 s in seq order, batches ≤ 500, cursor advances only on 2xx, `failing_since` recorded; after 24 h of failures status `error` + audit `audit_stream.paused` + org alert; re-activation resumes from the cursor; replay from any seq.
- **Action catalogue:** ≥ 70 stable action strings — use the list in the enterprise plan §6.4.2 plus the ones already emitted by the Go code; keep them in `apps/audit/actions.py` as constants and test that every mutating endpoint emits exactly one known primary action and only its listed secondary actions (route introspection test).

### 7.13 billing (plans, subscriptions, contracts, GST invoicing)

**Plans and entitlements** (port `internal/entitlements`, `internal/plans`): the organisation's plan is the source of truth; effective entitlements = plan defaults ⊕ active contract `limits_override` ⊕ `grandfathered_limits` (v1 users) ⊕ feature flags; workspace `disabled_features` can only remove. Limits and features:

| Plan | Dynamic codes | Analytics history | Seats | Workspaces | Custom domains | Bulk rows/job | API req/min | Webhooks | Templates |
|---|---|---|---|---|---|---|---|---|---|
| free | 3 | 30 days | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| pro | 100 | 365 | 1 | 3 | 1 | 500 | 0 | 0 | 10 |
| business | 1,000 | 1,095 | 5 | 10 | 5 | 5,000 | 600 | 10 | 100 |
| enterprise | 100,000 | 3,650 | 1,000 | 100 | 50 | 50,000 | 3,000 | 50 | 100,000 |

Features: **pro** `scheduling, expiry, scan_limit, utm_append, hosted_pages, templates, csv_export, remove_branding`; **business** adds `rules, campaigns, api, webhooks, raw_scan_log, locked_templates, audit_log, gs1, roles`; **enterprise** adds `sso, scim, sla, dedicated_domain, audit_streams`. Enterprise-plan modules add these feature keys: `approvals` (business+), `integrations` (business+), `alerts` (business+), `reports` (business+), `forms` (pro+), `pixels` (business+), `serials` (enterprise), `white_label` (enterprise), `agency` (enterprise contract), `sandbox` (business+). 402 responses carry `code: upgrade_required|limit_reached` and `required_plan`.

**Self-serve subscriptions** (new vs Go; v1 had Stripe on users): subscriptions belong to organisations (`subscriptions.org_id`).
- **Razorpay (INR, default for India):** plans pre-created in Razorpay, ids in `RAZORPAY_PLAN_{PRO,BUSINESS}_{MONTH,YEAR}`; `POST /v1/orgs/{org}/billing/checkout {plan, interval, provider?}` creates a Razorpay Subscription (UPI Autopay and cards supported by Razorpay Checkout) and returns `{provider:"razorpay", key_id, subscription_id}` for Razorpay Checkout on the frontend; cancel/change via API (`subscriptions.cancel`, plan change = cancel at cycle end + new subscription).
- **Stripe (non-INR):** Checkout Session in subscription mode (`STRIPE_PRICE_{PRO,BUSINESS}_{MONTH,YEAR}`), Billing Portal for payment methods and cancellation. Use the Stripe v15 SDK (`StripeClient`, `client.v1.*`; objects are not dicts).
- **Webhooks:** `POST /v1/billing/razorpay/webhook` (`X-Razorpay-Signature` = hex HMAC-SHA256 of the raw body with `RAZORPAY_WEBHOOK_SECRET`; event id from `x-razorpay-event-id`) and `POST /v1/billing/stripe/webhook` (`stripe.Webhook.construct_event`); both insert into `billing_events (provider, provider_event_id)` first — duplicates are acknowledged and skipped (exactly-once); handlers update `subscriptions` and `organizations.plan_id`; downgrade triggers plan read-only (§7.7).
- **Invoices for self-serve charges:** every successful charge (`subscription.charged` / `invoice.paid`) issues a **paid** GST invoice in v3 (so Indian customers get a compliant tax invoice); whether published INR prices include GST is controlled by `PRICES_INCLUDE_GST` (confirm with the CA, §18).

**Contracts and GST invoicing** (port `internal/invoicing` (tax.go, issue.go) + `tax_test.go`, `internal/billing/paymentlinks.go`, `00007_billing.sql`):
- Contracts (staff-created): term, interval (`month|quarter|year`), currency (`INR|USD|EUR|GBP`), amount (minor units, per interval, pre-tax), seats, `limits_override`, SLA, PO number, payment terms; one active contract per org; activation sets the org plan to `enterprise`.
- **Invoice run** `billing.issue_due_invoices` daily at 01:30 IST: for each active contract, every started billing period without an invoice (billed in advance; last period prorated by days) → issue; contracts past `ends_on` → `expired`. Idempotent: unique `(contract_id, period_start)` for non-void invoices, and the contract row is locked while issuing.
- **Tax modes:** supplier state `SELLER_STATE_CODE` = buyer state → CGST 9% + SGST 9%; other Indian state → IGST 18%; buyer outside India **and** currency ≠ INR → export under LUT, zero-rated, endorsement "Supply meant for export under Bond or Letter of Undertaking without payment of Integrated Tax" (+ LUT ref); buyer outside India paying INR → IGST 18%. Buyer state = `billing_address.state_code` or GSTIN digits 1–2; missing → issuance fails with a clear error. Place of supply `"27 - Maharashtra"` or `"96 - Other Countries"`. Half-up rounding per tax line. Seat true-up line when active members exceed contract seats. Amount in words (Indian lakh/crore for INR).
- **Numbering:** `SELECT next_invoice_number(issue_date)` → `QR/202627/00042` (consecutive per financial year April–March, ≤ 16 chars, IST dates).
- **Delivery job** `billing.deliver_invoice`: create a Razorpay Payment Link for INR invoices (`POST /v1/payment_links`, `reference_id` = invoice number, `notes.invoice_id`), render the PDF (ReportLab: seller/buyer blocks with GSTINs, place of supply, SAC, lines, tax breakup, total in words, endorsement, bank details, **UPI QR** `upi://pay?pa=SELLER_UPI_VPA&pn=…&am=…&cu=INR&tn=<number>` drawn with the renderer), store privately, email `billing_email` with a signed link and the payment link.
- **Payment:** Razorpay `payment_link.paid` webhook (amount must equal the invoice total) or staff "mark paid" (reference required) → `paid`; void only unpaid invoices (paid ones need a credit note — out of scope, documented).
- **Dunning** `billing.dunning` daily: reminders at due −7, 0, +7, +14 days (skipping missed slots), then **billing hold** 30 days after due (`organizations.billing_hold_since`): workspace **mutations** return 402 `billing_hold`; reads, exports of own data and **redirects keep working**; paying clears the hold. Audit `billing.hold_started|hold_cleared`.
- **Customer API:** `GET /v1/orgs/{org}/contracts`, `GET /v1/orgs/{org}/invoices`, `GET …/{id}`, `GET …/{id}/pdf` (`org.billing`), billing profile via `PATCH /v1/orgs/{org}` (legal name, GSTIN with checksum, billing email, address with `state_code`, tax country).
- **E-invoicing (IRN):** not implemented; the schema is additive (`irn`, `ack_no`, `signed_qr` later) — required once turnover crosses the government threshold (confirm with the CA).
- **Acceptance:** a ₹6,00,000/year contract for a Maharashtra buyer billed from Uttar Pradesh yields an IGST 18% invoice numbered `QR/202627/000NN`; a USD contract for a US buyer yields an export-under-LUT invoice with the endorsement and zero tax.

### 7.14 integrations (events, webhooks, providers)

Spec: enterprise plan §6.5.1 (read it in full). Django specifics:

- **Event fan-out:** `core.events.emit()` defers `integrations.fanout(event_id)` in the business transaction; the event payload is stored on the job. Fan-out resolves subscribers (webhooks, installations, alert channels, REST hooks) and defers one `integrations.deliver(delivery_id)` per target. `integration_deliveries` / `webhook_deliveries` are the idempotency ledgers (unique on `(target, event_id)`).
- **Event catalogue** (stable): `qr.created`, `qr.updated`, `qr.deleted`, `qr.version.created`, `qr.version.activated`, `qr.safety.blocked`, `scan.created` (sampled for REST hooks, batched for webhooks: at most one delivery per code per 10 s containing up to 100 scans), `lead.created`, `lead.withdrawn`, `approval.requested`, `approval.decided`, `approval.expired`, `alert.fired`, `serial.flagged`, `serial_batch.ready`, `member.added`, `member.removed`, `invoice.issued`, `invoice.paid`, `audit_stream.paused`, `ping`.
- **Webhooks (workspace, Business+):** CRUD, test, delivery log; secret `whsec_…` stored encrypted; payload `{id, type, created_at, workspace_id, data}`; **Standard Webhooks** headers (`webhook-id`, `webhook-timestamp`, `webhook-signature: v1,<base64>`); `signature_scheme='legacy_v1'` (migrated v1 webhooks) additionally/only sends `X-QRit-Signature: sha256=<hex HMAC-SHA256(body)>` + `X-QRit-Event`; retries with exponential backoff (1 m, 5 m, 30 m, 2 h, 6 h, 12 h, 24 h), disable after 3 days of failures with an email to admins; SSRF-safe client; 10 s timeout; response body stored truncated to 2 KB.
- **Providers (T2, by priority):** Slack (OAuth v2 `chat:write`, Block Kit), Microsoft Teams (**Workflows webhook URL**, Adaptive Card — Office 365 connectors are retired), Zapier/Make **REST hooks** (`POST /v1/hooks {target_url, event}`, `DELETE /v1/hooks/{id}`, `GET /v1/hooks/samples/{event}`; API-key auth), HubSpot (OAuth, contacts batch upsert), Salesforce (OAuth web-server flow, Lead create), Google Sheets (OAuth `drive.file`, append), GA4 Measurement Protocol (batched ≤ 25 events), warehouse export (S3/GCS HMAC/BigQuery load jobs; daily `jsonl.gz` + `_manifest.json`). OAuth state bound to session + workspace with PKCE where supported; tokens encrypted with the org DEK and refreshed 5 min before expiry; 20 consecutive failures → installation `error` + admin email.
- **API:** `GET /v1/integrations/catalog`, `GET/POST /v1/workspaces/{ws}/integrations`, `PATCH/DELETE …/{id}`, `POST …/{id}/test`, `GET …/{id}/deliveries`, OAuth start/callback, webhooks `GET/POST /v1/workspaces/{ws}/webhooks`, `PATCH/DELETE …/{id}`, `POST …/{id}/test`, `GET …/{id}/deliveries`, `POST …/{id}/rotate-secret`.

### 7.15 leads (forms, consent, DSAR)

Spec: enterprise plan §6.6. Django specifics:
- Forms up to 25 fields (text, email, phone E.164 via `phonenumberslite`, select, multi-select, checkbox, textarea, date, hidden auto-filled UTM/QR id), per-language labels (EN + HI shipped), one level of conditional visibility; hosted as `content_type='form'` at `/p/{code}` on the redirect service **or** rendered by the web app for v1 lead pages at `/p/<slug>` (`forms.legacy_slug`).
- Consent notice per language with itemised purposes, controller identity, grievance contact, retention, withdrawal method; submissions store `{notice_version, purposes_accepted[], lang, at}`; confirmation email with a one-click withdrawal link `/c/{token}`; double opt-in optional.
- Spam: Cloudflare Turnstile (server-side verify `TURNSTILE_SECRET_KEY`), honeypot, ≤ 10 submissions/10 min per IP prefix per form, duplicate-email window 5 min.
- Answers encrypted with the org DEK (§7.1); email blind index `HMAC-SHA256(org blind-index key, lower(email))` for DSAR and duplicates; retention job erases payloads after `retention_days`.
- Access: `lead.read` (audited `lead.viewed` per page view), export `lead.export` + step-up + org `export_permission`.
- DSAR: `POST /v1/orgs/{org}/dsar-requests {kind: access|erasure|correction, email}` → job; access returns a JSON export link; erasure removes payloads everywhere (a test scans the DB for the known plaintext afterwards); due-date SLA 30 days.
- Public API: `GET /v1/public/forms/{code}`, `POST /v1/public/forms/{code}/submissions`, `GET/POST /v1/public/consent/{token}`, `GET /v1/public/consent/{token}/confirm`.
- Events `lead.created`, `lead.withdrawn` (HubSpot/Salesforce opt-out sync).

### 7.16 pixels (consent-aware retargeting)

Spec: enterprise plan §6.7. Pixel ids only (Meta, Google, LinkedIn, TikTok; regex `^[A-Za-z0-9_-]{3,40}$`), never customer scripts. When a code has pixels, the redirect serves an **interstitial** (≤ 8 KB, `no-store`, per-response CSP listing only provider origins) instead of 302: brand + "Continue to {host}"; consent mode `opt_in_all` (default) or `opt_in_where_required` (EEA, UK, CH, IN get the prompt); Google Consent Mode v2 defaults denied; other pixels load only after Allow; redirect via `location.replace()` after pixel load events or 800 ms; `<noscript>` meta refresh; beacon `POST /_px/{code}` `{a: shown|accepted|declined|auto}` → Redis counters → `pixel_consent_daily` hourly. Playwright test: no third-party request before Allow; decline redirects < 300 ms.

### 7.17 gs1 (GS1-conformant resolver)

Spec: enterprise plan §6.8 (GS1-Conformant Resolver Standard 1.2.1). Port v1 `api/utils/gs1.py` (Digital Link builder/parser, GTIN mod-10) into `apps/gs1/domain/digital_link.py` and extend: AI order `01 → 22 → 10 → 21`, GS1 AI encodable character set 82, percent-decoding, length limits (lot/serial ≤ 20), 400 problem+json on invalid (never 200 for an error); default link of the most specific matching item (verified `GS1Linkset` ordering in `docs/v2-blueprint/enterprise_queries.sql`); `linkType` param → that link or **404** if unavailable; `linkType=linkset|all` or `Accept: application/linkset+json` → RFC 9264 linkset + context `Link` header; `300 Multiple Choices` on equal matches; `Accept-Language` negotiation (RFC 4647 lookup); query pass-through except `linkType`; `/.well-known/gs1resolver` description; **307** redirects with `Cache-Control: private, no-store`; CORS `*` with exposed `Link, Location, Content-Type`; OPTIONS/HEAD. Analytics: scan event with `rule_id = linkType`. Management API (items, links, CSV import) and "recall mode". ≥ 60 URI syntax test cases.

### 7.18 serials (serialisation and product authentication)

Spec: enterprise plan §6.9. Batches ≤ 10,000,000; generation job in chunks of 50,000 via `COPY` (psycopg `cursor.copy()`), serial = 9 random Crockford chars + 3-char MAC (first 15 bits of `HMAC-SHA256(SERIAL_MAC_KEY, serial9)`); printed payload `HTTPS://{HOST}/V/{SERIAL12}`; the redirect validates charset + MAC **before any DB access** (forgery → fast 404 "not a genuine code" + counter), valid → scan event with `serial` → 302 to `/v/{serial}` (web verify page) → page JS calls `POST /v1/public/verify/{serial} {t}` (`t` = HMAC of serial + 10-min window with `VERIFY_TOKEN_KEY`) → atomic `RecordSerialVerification` (verified query) → verdicts (first verification / previously verified n times / flagged: multi-country, max scans exceeded, velocity / void); flagged → `serial.flagged` event → alerts + webhooks. Exports CSV + PDF proof sheet (first 100 codes); void by list or all. Benchmark: 100,000 serials generated and exported < 2 min; 10 M < 15 min with flat memory.

### 7.19 alerts and scheduled reports

Spec: enterprise plan §6.5.2–6.5.3.
- **Alert rules:** `scan_spike`/`scan_drop` (z-score vs same 15-min bucket over the previous 7 days — verified `ScanSpikeCandidates`), `scan_threshold` (v1's `scan_alert_threshold`, now evaluated), `no_scans` (during active campaigns), `destination_down` (3 consecutive failed HEAD/GET checks, SSRF-safe, codes with scans in the last 7 days), `serial_anomaly`, `security_event` (subscribed actions, login-failure bursts ≥ 20/10 min), `approval_pending` (older than N hours); `alerts.evaluate` every 5 min; cooldown; `alert_events` rows + `alert.fired` fan-out (emails, integrations).
- **Reports:** `report_schedules` (daily/weekly/monthly at a local hour and timezone, DST-safe `next_run_at`); `reports.run` computes the same payloads as the dashboard (call the analytics selectors) and renders a **PDF with ReportLab** (cover with brand, KPI tiles, time-series and breakdown charts via `reportlab.graphics.charts`, top codes table, data-quality note) and/or CSV (zipped); stored privately; emailed as a **signed link valid 7 days**; send-now endpoint. Tests: DST transitions (America/New_York March/November), IST month-end, empty data, recipient limit, per-recipient unsubscribe link.

### 7.20 branding (white-label and agency)

Spec: enterprise plan §6.10.
- **Custom dashboard host** `org_branding.app_hostname`: Cloudflare for SaaS custom hostname pointing to the `web` service through the Worker (when the hostname becomes active the API adds it to the Worker's KV namespace `APP_HOSTS`, which makes the Worker send it to `web` instead of `redirect`; the Worker forwards `X-QRit-Host`; the web app resolves branding with `GET /v1/public/branding?host=` cached 5 min) → product name, logo, favicon, primary colour (contrast-checked, fallback when < 4.5:1), support URL. Cookies are host-only, so sessions are per host; SSO callbacks return to the starting host.
- **Email domain** via the Resend Domains API (create → show DKIM/SPF → verify); emails for the org's users come from `noreply@{email_domain}` with the org's product name.
- **`hide_platform_brand`** removes "QRit" from dashboard, emails, hosted pages and error pages (legal footer link excepted).
- **Agency:** `organizations.kind='agency'` creates client orgs (`parent_org_id`), agency admins act as admins in clients (access engine), every action is audited in the client's chain tagged `via_agency`, clients can't see the agency or siblings, billing consolidates on the agency contract. Tests: host-based branding isolation; agency tenant isolation.

### 7.21 developer (API keys, usage, sandbox, bulk, exports, OpenAPI)

Spec: enterprise plan §6.11 plus v1's API-key features.
- **API keys v2:** format `qk_live_<32 base32>` / `qk_test_<32 base32>` (160 bits); store `prefix` (first 12 chars, unique) + `key_hash = sha256(full key)`; shown once; scopes `qr:read`, `qr:write`, `analytics:read`, `webhooks:write`, `leads:read` (capped at editor; never approve); per-key `ip_allowlist`; org `api_key_max_days` forces `expires_at`; revoke; last-used tracking (Redis, flushed every minute). Auth header `Authorization: Bearer qk_…` or `X-API-Key: qk_…` (v1 compatibility). **Legacy keys** (imported): v1 user keys `ak_<32hex>` are stored as sha256 immediately (plaintext in v1); v1 workspace keys `qk_<8hex>.<secret>` carry `legacy_bcrypt_hash` — on first use verify bcrypt, then store sha256 of the full key and clear the bcrypt hash.
- **Test mode:** test keys only operate on the org's sandbox workspace (`workspaces.is_sandbox`, created by `POST /v1/orgs/{org}/sandbox`); sandbox codes resolve on `SANDBOX_SHORT_DOMAIN` behind a "Test mode" banner page; live keys can't touch sandbox and vice versa (404).
- **Usage:** Redis counters per key/minute → hourly flush to `api_usage_daily (requests, errors, throttled)`; `GET /v1/workspaces/{ws}/api-usage?from&to`.
- **Bulk (jobs table, kinds `bulk_create`, `bulk_update`, `bulk_download`, `export_qr_codes`, `export_scans`, `print_sheet`, …):** CSV/XLSX upload ≤ 100,000 rows (streaming parse), **dry-run** validation report, chunked transactions of 1,000 rows with a checkpoint (resumable after a worker restart), ≥ 1,000 rows/s, `bulk_update` destination changes pass through approvals (§7.11), progress `processed/failed/total`, error report CSV. v1 CSV columns (`content, title, qr_type, folder, campaign, tags, is_dynamic`) are accepted as an alias format.
- **OpenAPI:** drf-spectacular generates `/v1/openapi.json`; CI fails on unreviewed breaking changes (`oasdiff` or a schema snapshot test). SDKs (TypeScript, Python) generated on release tags; Postman collection; `Deprecation` + `Sunset` headers policy (§7.23).

### 7.22 staff console, support access, SLA

Spec: enterprise plan §6.12.2–6.12.4; port `CreateStaffAccessToken` and the staff branch of `orgContext`.
- **Staff console API** `/v1/admin/*` (not the Django admin): requires `users.is_staff`, an MFA-verified session and (if set) `STAFF_IP_ALLOWLIST`; every call audited. Endpoints: org search, org detail, plan change, contracts (create draft, activate, terminate), invoices (issue for a period, mark paid, void), feature flags per org, abuse queue, DSAR oversight, SLA reports, support sessions.
- **Support access:** org admins grant time-boxed access (`POST /v1/orgs/{org}/support-access {scope: read|read_write, reason, hours ≤ 168}`, step-up; revoke anytime). Staff start a session only while a grant is active (`POST /v1/admin/support-sessions {grant_id}` → `support_sessions` row + short-lived **bearer** staff token with claim `sg`; re-mint while active); staff principals get permissions from the grant scope only (read = analyst + audit.read; read_write = admin minus policy, API keys, roles, members), actions are audited with `actor_type='staff'` in the customer's chain, and `GET /v1/orgs/{org}/support-access/banner` lets the dashboard show a banner while a session is live.
- **SLA:** 99.95% monthly for redirects, 99.9% for dashboard/API, measured by external synthetic probes (Better Stack or similar, 1-min, 3 regions, canary code + `/readyz`); monthly per-org SLA PDF and service credits (10% < 99.95%, 25% < 99.0%, 50% < 95.0%) as a credit line on the next invoice.

### 7.23 legacy (v1 compatibility)

- **Links:** `/r/<code>` on legacy hosts (`LEGACY_HOSTS`: the v1 backend's Railway domain and any v1 custom domain; plus each imported code's `legacy_host`) → pipeline in §7.9. The `api` service also serves `/r/<code>` (same view, imported from `apps.redirect`) because the v1 backend Railway service — whose domain is on printed/shared links — becomes the v3 `api` service at cutover (`RAILWAY_DEPLOY.md` §6).
- **Public API aliases** for v1 API customers, 6-month deprecation window, each response with `Deprecation: true`, `Sunset: <date>`, `Link: <https://…/docs/api/migration>; rel="deprecation"`: `POST /api/v1/qr/api/generate` (X-API-Key) → create static/dynamic code via the v3 service and return v1's exact envelope: `201 {"success": true, "data": {<the fields of v1 `_serialize_qr` in `reference/v1-django/api/views/qr.py`>, "qr_base64": "…"}}`, errors `{"success": false, "error": "…"}` with v1's status codes (400/401/429). Dynamic codes created here also get a v1-format `legacy_short_code` (8 chars `[A-Za-z0-9_-]`) and `legacy_host` = the v1 backend host, and `short_code` in the response is that legacy code — v1 clients build `/r/<short_code>` themselves; `GET /api/v1/apikey`, `POST /api/v1/apikey/regenerate`, `GET /api/v1/apikey/usage`. Dashboard-internal v1 endpoints are not kept (the v1 frontend is replaced).
- **Lead pages:** the web app serves `/p/<slug>` for `forms.legacy_slug` (v1 lead pages) using the v3 public forms API. The web route `p/[code]` decides by format: v1 slugs always contain `-` (`<name>-<6 hex>`), v3 short codes never do.
- **Other v1 URLs on the `api` service:** `/health` (alias of `/healthz`), `POST /api/v1/billing/webhook` (Stripe, same handler as `/v1/billing/stripe/webhook`); every other `/api/v1/*` path not listed above answers `410 Gone` (problem+json pointing to `/v1`). The web app answers `/r/<code>` with `308` to `https://<v1 backend host>/r/<code>` (v1 showed such links on the code page although they 404'd in v1).
- **Webhooks:** `signature_scheme='legacy_v1'` for migrated webhooks (§7.14) until the owner switches to Standard Webhooks.
- **Auth:** v1 access/refresh tokens are **not** honoured; users sign in again (passwords still work thanks to the bcrypt hasher). Announce by email before cutover.

### 7.24 frontend (merged Next.js app)

- **Base:** move `apps/web` (v2) to `frontend/` (the v1 frontend Railway service root), keep its App Router structure (`(marketing)`, `(auth)`, `w/[workspace]/…`, `p/[code]`, `docs/api`, `abuse`), cookie auth, `/v1` rewrite to `API_INTERNAL_URL`, problem+json client. Upgrade to Next.js 16 (`output: "standalone"`, `HOSTNAME=0.0.0.0`, copy `public` and `.next/static` in the Dockerfile).
- **Fix the v2 contract bugs:** `/qr` → `/qr-codes`; `{items}` → `{data, next_cursor}`; login redirects to `/w/{me.workspaces[0].slug}/qr` using `GET /v1/me`; remove the hard-coded mock QR fallback; replace mocked analytics, domains, webhooks, security, billing pages with real API calls; `LiveBadge` stays on `/realtime`.
- **Port from v1** (reference in `reference/v1-frontend/src/app`): forgot/reset password, verify email, invite accept (`/invite/[token]`), Google sign-in button (GSI), profile, API keys, audit log viewer, branding/white-label, bulk CSV upload + ZIP download, campaigns, templates, lead pages + leads list, reports/exports, integrations, folders UI with drag-and-drop, workspace switcher, QR history/edit, public stats page, SEO landing pages for url, email, event, social-media and whatsapp QR types (v2 already has upi, vcard, wifi).
- **Enterprise screens:** org settings (members, security policy, domains, SSO connections + test flow, SCIM directories, roles matrix editor, groups, access review, audit log + chain badge + exports + streams, billing: plan/checkout (Razorpay Checkout, Stripe redirect), contracts, invoices, support access + live-session banner), workspace screens (access tab with bindings + explain drawer, approvals inbox with badge + diff view, policy form, integrations catalog, webhooks with delivery log, alerts, reports, forms builder + leads (decrypted view gated), pixels, GS1 catalog + link editor + recall mode, serial batches + map + flagged queue, API keys + usage chart, sandbox toggle, bulk jobs with dry-run report, print sheets), MFA (TOTP + passkeys + recovery codes), sessions list, staff console under `/admin` (hidden unless `is_staff`).
- **Editor preview:** debounced (150 ms) `POST /v1/render/preview`; show warnings; the `jsqr` worker decodes the returned SVG rasterised on a canvas and shows "Scans ✓".
- **Branding by host:** `proxy.ts` (Next 16 middleware) reads `x-qrit-host` **only when `x-qrit-edge-secret` equals `EDGE_SHARED_SECRET`** (otherwise `host`), fetches branding, sets CSS variables. It strips every `x-qrit-*` request header before the `/v1` rewrite to the API, and Sentry scrubs them.
- **Health:** keep the `/api/health` route handler (port `reference/v1-frontend/src/app/api/health/route.ts`; v2 has none) — Railway's health check and the probes use it.
- **i18n:** English everywhere; Hindi for public form/consent pages (and SHOULD for marketing).
- **Tests:** Playwright smoke (register → create dynamic code → download PNG → scan via redirect service → analytics shows the scan), consent/pixel network assertions, accessibility checks (axe) on key pages.

### 7.25 trust (safety and abuse)

Port `internal/urlsafety` (Web Risk client with timeouts → `pending`, rescans), worker tasks `safety.pending` (every 10 min) and `safety.rescan` (daily for active destinations), `qr.safety.blocked` handling (code blocked, event, audit, email), `internal/abuse` (disposable email domains at registration, velocity limiter), public abuse report `POST /v1/public/abuse-reports` (Turnstile) with a staff queue.

---

## 8. Permission catalogue and role matrix

Port `reference/go-v2/internal/authz/authz.go` verbatim (strings are stable API):

**Workspace permissions:** `workspace.read`, `workspace.update`, `workspace.delete`, `member.manage`, `role.manage`, `qr.read`, `qr.create`, `qr.update`, `qr.delete`, `qr.destination.update`, `qr.destination.approve`, `qr.design.bypass_lock`, `folder.manage`, `campaign.manage`, `template.manage`, `analytics.read`, `analytics.export`, `analytics.raw`, `domain.manage`, `apikey.manage`, `webhook.manage`, `integration.manage`, `policy.manage`, `audit.read`, `audit.export`, `form.manage`, `lead.read`, `lead.export`, `pixel.manage`, `alert.manage`, `report.manage`, `serial.manage`, `gs1.manage`, `bulk.run`, `billing.manage` (owner only via `*`).

**Org permissions** (from `org_members.org_role`): `org.manage`, `org.billing`, `org.sso`, `org.scim`, `org.policy`, `org.audit`, `org.members`, `org.branding`, `org.clients`.

| System role | Permissions |
|---|---|
| owner | `*` |
| admin | every workspace permission except `billing.manage` and `workspace.delete` |
| editor | `workspace.read, qr.read, qr.create, qr.update, qr.destination.update, folder.manage, campaign.manage, template.manage, analytics.read, analytics.export, form.manage, lead.read, alert.manage, report.manage, bulk.run` |
| reviewer | `workspace.read, qr.read, qr.destination.approve, analytics.read, audit.read, lead.read` |
| analyst | `workspace.read, qr.read, analytics.read, analytics.export` |

| Org role | Org permissions |
|---|---|
| org_owner | all nine; `*` in every workspace |
| org_admin | all except `org.billing`; admin in every workspace |
| billing_admin | `org.billing` |
| member | none (workspace access only through bindings) |

API key scopes: `qr:read` → workspace.read, qr.read; `qr:write` → + qr.create, qr.update, qr.destination.update; `analytics:read` → analytics.read; `webhooks:write` → webhook.manage; `leads:read` → lead.read (always capped at editor; never approve).

`require(perm)` passes folder-scoped holders through list endpoints (handlers then filter by folder chain); `require_workspace_wide(perm)` for settings-type endpoints; resource handlers call `can(request, perm, folder_id)` and return **404** (not 403) when the caller can't even read the resource.

---

## 9. API conventions and route table

### 9.1 Conventions

- Base path `/v1` (dashboard via same-origin rewrite; also served at `/api/v1/…` **only** for the v1 compatibility aliases in §7.23). JSON bodies, snake_case fields, RFC 3339 UTC timestamps, UUID ids, money in minor units with a `currency`.
- Workspace addressed by UUID or slug: `/v1/workspaces/{ws}/…`; organisation by UUID or slug: `/v1/orgs/{org}/…`.
- Lists: `{data: [...], next_cursor}`; creates: 201 with the resource and `Location`; accepted-for-approval: 202; deletes: 204.
- Errors: problem+json (§7.1, Appendix B). Validation: 422 with `errors[]`.
- Auth: cookies + CSRF (dashboard), bearer JWT, API key (`qk_…`), SCIM bearer, staff bearer.
- Idempotency-Key on creates; `ETag`/`If-Match` on policy documents and SCIM.
- OpenAPI at `/v1/openapi.json` (drf-spectacular), docs page in the web app `/docs/api`.

### 9.2 Route table

Legend: permission in brackets; *SU* = step-up required; *F:x* = plan feature `x` required.

**Auth and account**
- `POST /v1/auth/register` · `POST /v1/auth/login` · `POST /v1/auth/logout` · `POST /v1/auth/refresh` · `POST /v1/auth/verify-email` · `POST /v1/auth/resend-verification` · `POST /v1/auth/password/forgot` · `POST /v1/auth/password/reset` · `POST /v1/auth/magic-link` + `POST /v1/auth/magic-link/verify` (flag) · `POST /v1/auth/google`
- `POST /v1/auth/mfa/verify` · `POST /v1/auth/mfa/webauthn/options` · `POST /v1/auth/step-up`
- `POST /v1/auth/sso/start` · `GET /v1/auth/sso/callback` (OIDC) · `POST /v1/auth/saml/acs/{connection_id}` · `GET /v1/auth/saml/metadata/{connection_id}`
- `GET|PATCH /v1/me` · `POST /v1/me/password` · `GET /v1/me/sessions` · `DELETE /v1/me/sessions/{id}` · `GET /v1/me/mfa` · `POST /v1/me/mfa/totp` · `POST /v1/me/mfa/totp/confirm` · `POST /v1/me/mfa/webauthn/options` · `POST /v1/me/mfa/webauthn` · `DELETE /v1/me/mfa/{id}` *SU* · `POST /v1/me/mfa/recovery-codes` *SU* · `GET /v1/me/approvals` · `GET /v1/permissions`
- `GET /v1/invites/{token}` · `POST /v1/invites/{token}/accept`

**Organisations** (`/v1/orgs`)
- `GET|POST /v1/orgs` · `GET /{org}` · `PATCH /{org}` [org.manage] · `GET /{org}/entitlements` · `GET /{org}/features` · `GET|POST /{org}/workspaces` [org.manage for POST]
- `GET /{org}/members` · `PATCH|DELETE /{org}/members/{userId}` [org.members] · `POST /{org}/transfer-ownership` *SU* (owner)
- `GET /{org}/security-policy` · `PUT /{org}/security-policy` [org.policy] *SU*
- `GET|POST /{org}/domains` · `POST /{org}/domains/{id}/verify` · `PATCH|DELETE /{org}/domains/{id}` [org.sso]
- `GET|POST /{org}/sso-connections` (POST *SU*, *F:sso*) · `GET|PATCH|DELETE /{org}/sso-connections/{id}` (PATCH/DELETE *SU*) · `POST /{org}/sso-connections/{id}/test` · `GET /{org}/sso-connections/{id}/test-result` [org.sso]
- `GET|POST /{org}/scim-directories` (POST *SU*, *F:scim*) · `POST /{org}/scim-directories/{id}/token` *SU* · `PATCH /{org}/scim-directories/{id}` · `DELETE /{org}/scim-directories/{id}` *SU* [org.scim]
- `GET|POST /{org}/roles` · `GET|PATCH|DELETE /{org}/roles/{id}` [org.manage for writes; *F:roles*]
- `GET|POST /{org}/groups` · `GET|PATCH|DELETE /{org}/groups/{id}` · `POST /{org}/groups/{id}/members` · `DELETE /{org}/groups/{id}/members/{userId}` [org.members]
- `GET /{org}/access-review?format=json|csv` [org.audit] *SU* · `POST /{org}/access-review/complete` [org.audit]
- `GET /{org}/audit-logs` · `GET /{org}/audit-logs/verify` · `GET /{org}/audit-logs/export` *SU* [org.audit; *F:audit_log*]
- `GET|POST /{org}/audit-streams` · `PATCH|DELETE /{org}/audit-streams/{id}` · `POST /{org}/audit-streams/{id}/test` · `POST /{org}/audit-streams/{id}/replay` [org.audit; writes *SU*; *F:audit_streams*]
- `GET /{org}/contracts` · `GET /{org}/invoices` · `GET /{org}/invoices/{id}` · `GET /{org}/invoices/{id}/pdf` [org.billing]
- `POST /{org}/billing/checkout` · `GET /{org}/billing/subscription` · `POST /{org}/billing/cancel` · `POST /{org}/billing/portal` (Stripe) [org.billing]
- `GET|POST /{org}/support-access` (POST *SU*) · `DELETE /{org}/support-access/{id}` [org.manage] · `GET /{org}/support-access/banner` (any member)
- `GET|PUT /{org}/branding` · `POST /{org}/branding/app-hostname` · `POST /{org}/branding/email-domain` · `POST /{org}/branding/email-domain/verify` [org.branding; *F:white_label*]
- `GET|POST /{org}/clients` · `PATCH /{org}/clients/{child}` [org.clients; agency]
- `POST /{org}/dsar-requests` · `GET /{org}/dsar-requests` · `GET /{org}/dsar-requests/{id}` [org.manage]
- `POST /{org}/sandbox` [org.manage; *F:sandbox*]

**Workspaces** (`/v1/workspaces/{ws}`)
- `GET|POST /v1/workspaces` · `GET /{ws}` · `PATCH /{ws}` [workspace.update] · `DELETE /{ws}` [workspace.delete] · `GET /{ws}/entitlements` · `POST /{ws}/leave` · `POST /{ws}/transfer-ownership` [billing.manage]
- `GET /{ws}/members` · `PATCH|DELETE /{ws}/members/{userId}` [member.manage] · `GET|POST /{ws}/invites` · `DELETE /{ws}/invites/{id}` [member.manage]
- `GET /{ws}/policies` [workspace.read] · `PUT /{ws}/policies` [policy.manage]
- `GET /{ws}/roles` · `GET|POST /{ws}/role-bindings` · `DELETE /{ws}/role-bindings/{id}` · `GET /{ws}/access/explain` [role.manage]
- QR: `GET|POST /{ws}/qr-codes` [qr.read / qr.create] · `GET|PATCH|DELETE /{ws}/qr-codes/{id}` · `POST /{ws}/qr-codes/{id}/{pause|resume|archive|unarchive|restore}` · `GET|POST /{ws}/qr-codes/{id}/versions` · `POST /{ws}/qr-codes/{id}/versions/{vid}/restore` · `DELETE /{ws}/qr-codes/{id}/versions/{vid}` · `POST /{ws}/qr-codes/{id}/resolve-preview` · `GET /{ws}/qr-codes/{id}/download` · `POST /{ws}/qr-codes/bulk-action` (move, tag, untag, pause, resume, archive, assign_campaign, delete — v1 parity)
- Organise: `GET|POST /{ws}/folders` · `PATCH|DELETE /{ws}/folders/{id}` · same for `tags`, `campaigns` (+ `GET /{ws}/campaigns/{id}/analytics`), `templates`
- Domains: `GET|POST /{ws}/domains` · `GET|PATCH|DELETE /{ws}/domains/{id}` · `POST /{ws}/domains/{id}/check` [domain.manage; *F* per plan limit]
- Analytics: `GET /{ws}/analytics/{summary|timeseries|breakdown|top-codes|heatmap|scans|export}` · `GET /{ws}/realtime` [analytics.read; raw scans analytics.raw + *F:raw_scan_log*; export analytics.export + *F:csv_export*]
- Approvals: `GET /{ws}/approvals` · `GET /{ws}/approvals/{id}` [qr.read] · `POST /{ws}/approvals/{id}/decisions` [qr.destination.approve] · `POST /{ws}/approvals/{id}/cancel` · `POST /{ws}/approvals/{id}/override` *SU* (org owner)
- Audit: `GET /{ws}/audit-logs` [audit.read]
- Integrations: `GET|POST /{ws}/integrations` · `PATCH|DELETE /{ws}/integrations/{id}` · `POST /{ws}/integrations/{id}/test` · `GET /{ws}/integrations/{id}/deliveries` [integration.manage] · `GET|POST /{ws}/webhooks` · `PATCH|DELETE /{ws}/webhooks/{id}` · `POST /{ws}/webhooks/{id}/test` · `POST /{ws}/webhooks/{id}/rotate-secret` · `GET /{ws}/webhooks/{id}/deliveries` [webhook.manage]
- Alerts/reports: `GET|POST /{ws}/alert-rules` · `PATCH|DELETE /{ws}/alert-rules/{id}` · `GET /{ws}/alert-events` · `POST /{ws}/alert-events/{id}/resolve` [alert.manage] · `GET|POST /{ws}/report-schedules` · `PATCH|DELETE /{ws}/report-schedules/{id}` · `POST /{ws}/report-schedules/{id}/send-now` [report.manage]
- Leads: `GET|POST /{ws}/forms` · `GET|PATCH|DELETE /{ws}/forms/{id}` [form.manage] · `GET /{ws}/forms/{id}/submissions` [lead.read] · `POST /{ws}/forms/{id}/export` [lead.export] *SU*
- Pixels: `GET|POST /{ws}/pixels` · `PATCH|DELETE /{ws}/pixels/{id}` · `PUT /{ws}/qr-codes/{id}/pixels` [pixel.manage]
- GS1: `GET|POST /{ws}/gs1/items` · `PATCH|DELETE /{ws}/gs1/items/{id}` · `GET|POST /{ws}/gs1/items/{id}/links` · `PATCH|DELETE /{ws}/gs1/links/{linkId}` · `POST /{ws}/gs1/import` · `POST /{ws}/gs1/items/{id}/recall` [gs1.manage]
- Serials: `GET|POST /{ws}/serial-batches` · `GET /{ws}/serial-batches/{id}` · `GET /{ws}/serial-batches/{id}/codes` · `POST /{ws}/serial-batches/{id}/export` · `POST /{ws}/serial-batches/{id}/void` [serial.manage]
- Developer: `GET|POST /{ws}/api-keys` · `DELETE /{ws}/api-keys/{id}` [apikey.manage] · `GET /{ws}/api-usage` · `GET|POST /{ws}/jobs` (bulk: kinds per §7.21) · `GET /{ws}/jobs/{id}` · `POST /{ws}/jobs/{id}/cancel` · `GET /{ws}/jobs/{id}/download` [bulk.run / qr.read for downloads]

**Rendering and public**
- `POST /v1/render/preview` (authenticated) · `POST /v1/public/render` · `GET /v1/public/branding?host=` · `GET /v1/public/forms/{code}` · `POST /v1/public/forms/{code}/submissions` · `GET|POST /v1/public/consent/{token}` · `GET /v1/public/consent/{token}/confirm` · `POST /v1/public/verify/{serial}` · `POST /v1/public/abuse-reports` · `GET /v1/public/stats/{code}` (public stats page, only when the owner enabled it)
- REST hooks: `POST /v1/hooks` · `DELETE /v1/hooks/{id}` · `GET /v1/hooks/samples/{event}` (API key)
- Integrations: `GET /v1/integrations/catalog` · `GET /v1/integrations/oauth/{provider}/start` · `GET /v1/integrations/oauth/{provider}/callback`
- Billing webhooks: `POST /v1/billing/razorpay/webhook` · `POST /v1/billing/stripe/webhook`
- SCIM: `/scim/v2/*` (§7.6)
- Staff: `/v1/admin/*` (§7.22)
- Health: `/healthz`, `/readyz`; OpenAPI `/v1/openapi.json`
- Legacy: `/r/{code}`, `/api/v1/qr/api/generate`, `/api/v1/apikey*` (§7.23)

---

## 10. Background jobs and schedules

procrastinate queues: `default`, `critical` (approvals, auth emails, billing webhooks follow-ups), `bulk` (bulk jobs, serial generation, exports), `integrations` (deliveries), `reports`. One worker service processes all queues with `WORKER_CONCURRENCY`; split into per-queue services later if a queue needs isolation.

| Task | Trigger | Queue | Source to port / spec |
|---|---|---|---|
| `audit.seal` | every 5 s (periodic cron `* * * * *` with an internal 5 s loop for 55 s) | critical | `worker/audit.go` |
| `audit.anchor` | daily 00:10 UTC | default | `worker/audit.go` |
| `audit.retention` | daily 03:30 UTC | default | `worker/audit.go` |
| `audit.stream` | every 15 s (loop pattern) | integrations | `worker/auditstream.go` |
| `analytics.ensure_partitions` | at boot + daily 01:00 UTC | default | `worker/maintenance.go` |
| `analytics.reconcile_rollups` | daily 02:00 UTC (yesterday) | default | `ReconcileYesterday` |
| `analytics.drop_old_partitions` | monthly | default | new |
| `qr.activate_scheduled_versions` | every 30 s (loop) | critical | `ActivateScheduledVersions` (+ approval filter) |
| `qr.purge_deleted` | daily 04:00 UTC | default | `PurgeDeletedCodes` |
| `qr.plan_read_only` | hourly | default | `EnforcePlanReadOnly` |
| `qr.poll_custom_domains` | every 5 min | default | new (Cloudflare for SaaS status) |
| `trust.safety_pending` | every 10 min | default | `RecheckPendingSafety` |
| `trust.safety_rescan` | daily 03:00 UTC | default | `RescanActiveDestinations` |
| `identity.verify_pending_domains` | every 10 min | default | `worker/identity.go` |
| `identity.cleanup` | daily 05:30 UTC | default | `worker/identity.go` |
| `approvals.expire` | every 5 min | critical | `worker/governance.go` |
| `approvals.notify` | job (on request) | critical | `NotifyApprovers` |
| `integrations.fanout` / `integrations.deliver` | jobs | integrations | spec §7.14 |
| `integrations.refresh_tokens` | every 5 min | integrations | spec |
| `alerts.evaluate` | every 5 min | default | spec §7.19 |
| `alerts.destination_health` | hourly | default | spec |
| `reports.dispatch` / `reports.run` | every 5 min / job | reports | spec |
| `billing.issue_due_invoices` | daily 20:00 UTC (01:30 IST) | default | `invoicing.IssueDue` |
| `billing.deliver_invoice` | job | default | new (payment link, PDF, email) |
| `billing.dunning` | daily 04:30 UTC | default | `invoicing.Dunning` |
| `leads.retention` | daily | default | spec §7.15 |
| `leads.dsar` | job | default | spec |
| `pixels.flush_counters` | hourly | default | spec |
| `serials.generate` / `serials.export` | jobs | bulk | spec §7.18 |
| `developer.flush_api_usage` | hourly | default | spec |
| `developer.bulk_*`, `developer.export_*`, `render.print_sheet` | jobs | bulk | spec §7.21 |
| `core.cleanup` | daily 05:00 UTC | default | `worker/maintenance.go::Cleanup` (expired idempotency keys, email tokens, sessions, rate-limit keys) |
| `core.refresh_cloudflare_ranges` | monthly | default | new |

Loop pattern for sub-minute tasks: a periodic task every minute that loops internally for 55 s (sleeping between iterations) and holds a Postgres advisory lock so only one worker runs it. Every task is idempotent and safe to run twice.

---

## 11. Testing strategy

- **Levels:** domain unit tests (pure functions: rules, GST, approvals, shortcode, digital link, scannability, SigV4, Standard Webhooks), service tests (DB + Redis), API tests (DRF `APIClient` with real cookies/CSRF), end-to-end flows (`backend/tests/e2e/`), frontend Playwright smoke, k6 load.
- **Infrastructure:** tests run against **PostgreSQL 17 and Redis 7** (docker compose locally; GitHub Actions `services:` in CI). The test database is created by Django migrations; a second role `qrit_app_test` (non-superuser) is used by the RLS tests.
- **Porting the Go tests:** every Go test in `reference/go-v2/**/*_test.go` has a pytest counterpart named `test_<go_name_snake_case>` in the corresponding app, keeping every assertion (status codes, error codes, counts). A checklist file `docs/v3/GO_TEST_PORT.md` (generated in P1 by listing the Go test functions) is ticked off per phase. Key integration tests: `TestAuthLifecycle`, `TestQRCodeLifecycle`, `TestRulesPreviewAndIdempotency`, `TestTenantIsolationAndTeams`, `TestRowLevelSecurity`, `TestScanPipelineEndToEnd`, `TestJobQueue` (→ procrastinate transactional enqueue test), `TestOrganisationsAndAccess`, `TestTamperEvidentAudit`, `TestEnvelopeAndFlags`, `TestAccessGovernance`, `TestApprovals`, `TestAuditStreams`, `TestMFA`, `TestDomainsAndOIDC`, `TestSCIM`.
- **Fakes, not mocks, for external systems:** a fake OIDC IdP (RS256 JWKS, PKCE check — port the Go `fakeIdP`), fake SAML IdP (pysaml2), fake DNS resolver, fake Cloudflare API, fake Web Risk, `responses` for Stripe/Razorpay/Resend, a local HTTP receiver for webhooks/SIEM that verifies signatures.
- **Golden and property tests:** render fixtures (§7.8); hypothesis for shortcode normalisation, rules evaluation, GST rounding, SCIM filter parsing.
- **Schema parity** (§6.1) and **route audit** (every route has a permission; every mutating route emits its one primary audit action and only listed secondary actions).
- **Migration tests:** a v1 fixture database built by running the v1 Django migrations (`reference/v1-django`) in a throwaway venv (Django 4.2) and loading `legacy/tests/fixtures/v1_seed.sql` (users with bcrypt passwords, workspaces, codes of every type incl. dynamic with routing rules and geo restrictions, scans, leads, webhooks, API keys, custom domains); `import_v1` must be idempotent (run twice → same row counts, same checksums), `verify_v1_import` must pass, a v1 bcrypt password must log in, `/r/<code>` must redirect exactly as v1 did for a table of cases.
- **Coverage gate:** ≥ 85% lines on `apps/*/domain` and `services.py`, ≥ 75% overall; no untested endpoint.
- **CI pipeline:** `uv lock --check` → `ruff check` + `ruff format --check` → `mypy` → `pytest -n auto` (xdist) → Django `check --deploy` with production settings → build images → k6 smoke (redirect, 30 s) on the compose stack → OpenAPI diff → frontend `pnpm lint && pnpm typecheck && pnpm build && playwright test --project=smoke`.

---

## 12. Security and compliance

- Secrets only in Railway variables (sealed for keys); never in the repo; `.env.example` lists names only.
- Django `check --deploy` clean; `SECURE_HSTS_SECONDS=31536000` with preload on production domains; `SESSION_COOKIE_*` unused (JWT cookies are set by our code with `Secure`, `HttpOnly`, `SameSite`); `SECURE_CSP` via Django's built-in CSP middleware for API HTML (browsable API disabled in production) and templates; redirect pages have their own strict CSP (§7.9).
- **Cache-Control:** every authenticated response sets `Cache-Control: private, no-store` (defence against CDN mis-caching; Railway had a CDN incident on 30 Mar 2026); keep Railway CDN caching **off** for api and redirect.
- Passwords argon2id; tokens and keys hashed (sha256 for high-entropy secrets); per-org envelope encryption for secrets and PII (webhook secrets, OAuth tokens, SSO client secrets, SIEM credentials, form answers).
- SSRF guard on every outbound call; XML parsing only in pysaml2 (defusedxml-safe settings, no external entities); CSV injection neutralised in exports; SVG output escaped; uploads type-sniffed and re-encoded.
- Rate limits (§7.1); login lockout; MFA; step-up for sensitive actions; session idle/max; IP allowlists; SSO enforcement with break-glass.
- Tenant isolation: access engine + RLS with a non-superuser role; 404 for other tenants' resources.
- Privacy: no raw IPs in scan data (daily-salted visitor hash), IP prefixes in audit, lead data encrypted with blind index, DSAR and withdrawal flows, retention jobs, GeoIP not stored beyond city.
- DPDP (India) and GDPR: consent notices, withdrawal as easy as consent, breach runbook `docs/v3/runbooks/breach.md` (72 h), sub-processor list (Railway, Cloudflare, Resend, Razorpay, Stripe, Google Web Risk) in the trust page.
- Dependency hygiene: `uv lock` + Dependabot/Renovate; `pip-audit` in CI; container image scanning (Trivy) in CI.

---

## 13. Observability and SLOs

- **Logs:** structlog JSON to stdout (Railway log explorer); fields `ts, level, event, request_id, org_id, workspace_id, user_id, route, status, duration_ms`; redirect logs sampled (1% of 302s, all errors).
- **Errors:** Sentry (`SENTRY_DSN`, environment, release = `RAILWAY_GIT_COMMIT_SHA`; traces sample 5% API, 0.1% redirect).
- **Metrics:** `/metrics` (Prometheus text) on redirect and worker behind `METRICS_TOKEN`: redirect QPS by outcome, latency histogram, cache hit ratio (L1/L2/DB), emit queue depth and drops; ingest lag (stream length, pending), batch duration; job queue depth by queue, failures; webhook success rate. The Grafana dashboard JSON in `deploy/observability/` is updated for the new metric names.
- **SLOs:** redirect availability 99.95%/month, redirect p95 ≤ 50 ms end-to-end in India (via Cloudflare), API availability 99.9%, ingest lag p95 ≤ 60 s, webhook first-attempt success ≥ 99%.
- **Synthetic probes:** Better Stack (or similar) every minute from 3 regions: canary code on `SHORT_DOMAIN`, `API_DOMAIN/readyz`, `APP_DOMAIN/api/health`; public status page with components Redirects, Dashboard, API, Analytics pipeline, Webhooks & integrations, SSO.

---

## 14. Deployment summary (full runbook: `RAILWAY_DEPLOY.md`)

- One Railway project `qrit`, environments `production` and `staging` (+ PR environments with focused monorepo deploys), region **Southeast Asia (Singapore) `asia-southeast1-eqsg3a`** for every service and database.
- Services from §3.2; built from Dockerfiles in `backend/` and `frontend/` (Railway always uses a Dockerfile when present; no Nixpacks). Watch paths: `backend/**` for Python services, `frontend/**` for web.
- Configuration as **Infrastructure as Code** in `.railway/railway.ts` (`railway config plan|apply`); settings IaC can't express (restart policy, static outbound IPs, cron) are set in the dashboard and listed in the runbook. The existing v1 services' `railway.json` files are migrated with `railway config migrate --lang ts` (then re-check `restartPolicy`/`region`, which the migrator drops) before **1 Dec 2026**.
- The v1 `backend` and `frontend` Railway services are **re-used** as v3 `api` and `web` (same services → same Railway domains → v1 links and bookmarks keep working).
- Postgres 17: owner role for migrations (`DATABASE_OWNER_URL`, pre-deploy), app role `qrit_app` (NOSUPERUSER, NOBYPASSRLS) for all services; PgBouncer (transaction mode) enabled when api/redirect replicas × pool size approaches `max_connections` — then services use the pooled `DATABASE_URL`, while `worker` and migrations keep the unpooled URL; `DISABLE_SERVER_SIDE_CURSORS=True` when pooled.
- Backups: Railway scheduled backups (daily + weekly + monthly) and Point-in-Time Recovery on the production database; restore drill in staging each quarter.
- Cloudflare: zone for `SHORT_DOMAIN` (proxied), Cloudflare for SaaS (fallback origin = the Worker route), Worker `edge-router` (`edge/worker`), zone for the product domain (app/api) — DNS only or proxied with SSL mode **Full** (not Full Strict) as Railway recommends.

## 15. v1 data migration summary (full spec: `V1_TO_V3_MIGRATION.md`)

- `python manage.py import_v1 --source "$V1_DATABASE_URL" [--dry-run] [--since <ts>] [--only <entities>]` reads v1 **read-only** over the private network and writes v3 through the service layer's low-level writers (bypassing HTTP, keeping invariants), recording `legacy_id_map` rows; re-runnable (upserts by v1 id) and resumable; `--since` performs the delta pass during the cutover window.
- IDs are preserved for users, workspaces, codes, folders, campaigns, templates (v1 UUIDs become v3 ids).
- Every v1 user gets a personal organisation; v1 workspaces become workspaces in the owner's organisation; user-scoped v1 codes go to the owner's default workspace.
- Plans move from users to organisations: v1 `free` → `free`, `starter` → `pro`, `pro` → `business`, `enterprise` → `enterprise`; **grandfathered limits** keep what each customer already has (e.g. a free user with 12 dynamic codes keeps all 12 editable; paid v1 plans keep their v1 seat/workspace/code allowances) until they change plan.
- v1 dynamic codes → v3 dynamic codes on the platform domain with a new 7-char `short_code` **and** their `legacy_short_code`/`legacy_host` (so `/r/<code>` keeps working); `v1_printed_payload` records what v1 images encoded; `needs_reprint = true` when that payload is the destination rather than the short link — the UI explains that printed copies can't be redirected and offers the new image.
- Passwords → `bcrypt$<hash>` (upgraded on login); QR passwords → argon2; webhook secrets → encrypted, `signature_scheme='legacy_v1'`; API keys → sha256 (user keys) or `legacy_bcrypt_hash` (workspace keys); scans → `scan_events` without raw IPs (`visitor_hash` from a one-off salt, `source='v1_import'`), rollups rebuilt; leads → forms + encrypted submissions; audit logs → unsealed audit rows, then sealed in order; routing rules and geo restrictions → v3 rules JSON in version 1 of each code.
- `verify_v1_import` compares counts, checksums and a sample of end-to-end redirects between v1 and v3 and must pass before cutover.

---

## 16. Phased delivery plan

Each phase ends with its **exit gate**: all listed commands pass, the Go tests mapped to the phase are ported and green, `docs/v3/GO_TEST_PORT.md` is updated, and a short `docs/v3/phase-reports/P<n>.md` records what was built, deviations and follow-ups. Gemini MUST NOT start the next phase before the gate passes (see `GEMINI_PROMPT_V3.md`).

| Phase | Scope | Main deliverables | Exit gate (in addition to lint/type/test) |
|---|---|---|---|
| **P0** Restructure & scaffold | Move `services/` → `reference/go-v2/`, `backend/` → `reference/v1-django/`, copy `frontend/` → `reference/v1-frontend/`, `packages/qr-render` + `apps/render` → `reference/ts-render/`; new `backend/` Django 6.1 project with settings split, uv, ruff, mypy, pytest, Dockerfile, docker-compose, CI | Buildable empty project; `/healthz`, `/readyz`; CI green | `docker compose up` serves `/readyz` 200 on api and redirect; image < 350 MB; `manage.py check --deploy` clean with production settings |
| **P1** Schema & core | All models + RunSQL migrations for 00001–00007 (00003 is replaced by procrastinate) + §6.4 deltas; core module (§7.1) | Schema parity test; problem+json, pagination, idempotency, rate limiter, client IP, SSRF client, RLS helper, keyring, flags, storage, email, events outbox | `test_schema_parity` green; RLS test at database level (factory rows in two workspaces, queries as `qrit_app_test` inside `workspace_scope`); idempotency middleware tests on a test-only view; envelope/flags, client-IP and SSRF tests ported. (The HTTP versions of `TestRowLevelSecurity` and of the idempotency part of `TestRulesPreviewAndIdempotency` need QR endpoints and complete in P3.) |
| **P2** Identity core & tenancy | accounts (no SSO yet), orgs, workspaces, access engine + gate, audit (writing + sealing + verify + export), plans/entitlements | Auth lifecycle with cookies/CSRF, MFA TOTP + recovery + passkeys, orgs/workspaces/invites, roles/bindings basics | `TestAuthLifecycle` and `TestMFA` ported in full; the steps of `TestTenantIsolationAndTeams`, `TestOrganisationsAndAccess` and `TestTamperEvidentAudit` that don't need QR endpoints ported (QR steps stubbed with factories, marked in `GO_TEST_PORT.md` as `partial`) |
| **P3** QR core + renderer | qr app (§7.7), render app (§7.8) incl. golden fixtures from the TS renderer, domains incl. Cloudflare for SaaS client (fake in tests), organise, trust | CRUD, versions, rules preview, downloads, preview endpoint | `TestQRCodeLifecycle`, `TestRulesPreviewAndIdempotency` and the full HTTP versions of `TestRowLevelSecurity`, `TestTenantIsolationAndTeams`, `TestOrganisationsAndAccess`, `TestTamperEvidentAudit` ported; qr/shortcode/routing/version/urlsafety/hosted/content tests ported; render golden tests (decode + SSIM) green |
| **P4** Redirect + analytics | redirect service (§7.9) incl. legacy `/r/`, ingest, rollups, realtime, analytics API; Cloudflare Worker + local edge emulation | Hot path, pages, scan pipeline, partitions | `TestScanPipelineEndToEnd` and the `scan` (uaparse, botdetect), `resolve`, `ingest`, `realtime` unit tests ported; k6 thresholds (§3.5) on compose; Worker unit tests; edge-secret header trust tests |
| **P5** Enterprise identity | SSO OIDC + SAML, claimed domains + re-poll, SSO enforcement, SCIM, Google sign-in linking | | `TestDomainsAndOIDC`, `TestSCIM`, SCIM unit tests, SSO negative tests, SAML security tests |
| **P6** Governance | custom roles, groups, bindings, explain, access review, approvals (incl. bulk hook), audit streams | | `TestAccessGovernance`, `TestApprovals`, `TestAuditStreams` + vectors |
| **P7** Billing | self-serve Razorpay/Stripe, provider webhooks, contracts, GST invoices + PDF + UPI QR, payment links, dunning, billing hold, staff console, support access, SLA report | | `tax_test` cases; invoice acceptance cases (IGST, CGST+SGST, export LUT); webhook idempotency; billing hold blocks writes not redirects |
| **P8** Integrations, alerts, reports | fan-out, webhooks (+ legacy scheme), Slack, Teams, Zapier/Make REST hooks, HubSpot, Salesforce, Sheets, GA4, warehouse export; alerts; reports PDF | | Signature tests; retry/backoff; DST report tests; alert evaluation tests |
| **P9** Leads, pixels, GS1, serials | §7.15–7.18 | | Crypto/erasure scan test; Turnstile paths; pixel Playwright test; ≥ 60 GS1 URI cases; serial MAC/verdict tests; 100k serial benchmark |
| **P10** White-label, agency, developer platform | §7.20–7.21 incl. sandbox, API usage, bulk 100k with dry-run and resume, print sheets, CMYK PDF, EPS, OpenAPI + SDK generation | | Host isolation, agency isolation, test-key isolation, 100k dry-run < 60 s, resume-after-kill, CMYK colour-space check, EPS decode round-trip |
| **P11** Frontend merge | §7.24 (can start after P3, in parallel) | Merged Next 16 app with v1 pages, fixed contract, enterprise screens | `pnpm lint typecheck build`; Playwright smoke + consent test; axe checks |
| **P12** v1 migration tooling & legacy layer | `import_v1`, `verify_v1_import`, v1 fixture DB, compat routes (§7.23) | | Idempotent import on the fixture; legacy redirect table; v1 password login; compat API responses match v1 shapes |
| **P13** Railway, staging rehearsal, production cutover | `.railway/railway.ts`, Dockerfiles, Cloudflare setup, staging import from a v1 backup, load test, go-live runbook execution, post-launch hardening | Running production v3 | Runbook checklists (`RAILWAY_DEPLOY.md` §5–7) all ticked; SLO probes green for 72 h |

Dependencies: P1 → everything; P2 → P3 → P4; P5/P6 need P2; P7 needs P2 (+P3 for UPI QR rendering); P8 needs P6 events; P9 needs P4 (redirect) and P8 (events); P10 needs P3/P4/P8; P11 needs the API of the screens it builds; P12 needs P3/P4/P9 (forms); P13 needs all.

---

## 17. Feature parity matrix

| Capability | v1 | v2 Go | v3 module | Phase |
|---|---|---|---|---|
| Email/password auth, verify, reset | ✓ (JWT in localStorage) | ✓ (cookies) | accounts | P2 |
| Google sign-in | ✓ | config only | accounts | P2/P5 |
| MFA (TOTP, recovery) / passkeys | — | ✓ / — | accounts | P2 |
| Static QR types | ✓ (many) | ✓ | qr | P3 |
| Dynamic codes with editable destination | ✓ (printed codes not editable — v1 defect) | ✓ | qr + redirect | P3/P4 |
| Scheduling, expiry, scan limit, password | ✓ | ✓ | qr + redirect | P3/P4 |
| Routing rules / geo restrictions | ✓ (flat rules) | ✓ (rules JSON, split, block) | qr | P3 |
| Hosted pages (vCard, links, file, event, menu) | partial | ✓ | qr + redirect | P3/P4 |
| Design customisation, logo, frames | ✓ (client-side) | ✓ (TS renderer) | render (Python) | P3 |
| Bulk create / download | ✓ (sync, ≤ CSV) | — | developer | P10 |
| Folders, tags, campaigns, templates | ✓ | ✓ | qr | P3 |
| Analytics dashboard, exports | ✓ (raw ORM) | ✓ (rollups) | analytics | P4 |
| Realtime | — | ✓ | analytics | P4 |
| Teams, invites, roles | ✓ (4 roles) | ✓ (5 roles, bindings) | workspaces + access | P2/P6 |
| Organisations | — | ✓ | orgs | P2 |
| Audit log | ✓ (plain) | ✓ (hash chain) | audit | P2/P6 |
| Webhooks | ✓ (sync, plaintext secret) | lib only | integrations | P8 |
| API keys | ✓ (plaintext / bcrypt) | ✓ (sha256, scopes) | developer | P10 |
| Custom short domains | ✓ (manual) | ✓ (Cloudflare for SaaS design) | qr | P3 |
| Lead capture pages | ✓ | — | leads | P9 |
| Billing | Stripe (user-level) | lib only | billing | P7 |
| SSO / SCIM | config only | ✓ | identity | P5 |
| Approvals, policies | fields only | ✓ | approvals | P6 |
| SIEM streams | — | ✓ | audit | P6 |
| Contracts + GST invoices | — | core | billing | P7 |
| Integrations (Slack, Teams, Zapier, CRM, GA4, warehouse) | — | — | integrations | P8 |
| Alerts, scheduled reports | threshold field only | — | alerts, reports | P8 |
| Consent, DSAR, pixels | opt-in checkbox | — | leads, pixels | P9 |
| GS1 Digital Link | utils only | partial | gs1 | P9 |
| Serialisation / anti-counterfeit | — | — | serials | P9 |
| White-label, agency | branding fields | — | branding | P10 |
| Sandbox, usage, SDKs, print-grade | — | — | developer, render | P10 |
| Staff console, support access, SLA | — | token only | staff | P7 |
| Public generator (logged out) | ✓ | ✓ (TS) | render + web | P3/P11 |
| SEO landing pages | ✓ (8) | ✓ (3) | web | P11 |

---

## 18. Risks and open questions

### 18.1 Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Printed v1 dynamic codes encode destinations | Customers expect edits to reach printed codes | Import with `needs_reprint`, UI banner + one-click new image, email to affected users before cutover (template in the migration doc) |
| Python redirect latency | Slow scans, SLO breach | D5/D6 design, cache layers, k6 gate in CI and staging, scale `redirect` replicas; profile with py-spy if p95 > budget |
| Cloudflare Worker / SaaS routing | Custom domains don't resolve | P4 spike with a test hostname; fallback: Cloudflare Origin Rules host override (plan-dependent) |
| Railway limits (20 custom domains/service, volumes vs replicas, IaC gaps) | Deployment friction | Cloudflare for SaaS, object storage instead of volumes, dashboard settings documented |
| RLS bypass by superuser | Tenant data leak on a missed filter | Non-superuser app role, RLS test in CI, access engine as the primary control |
| Data migration errors | Lost links or data | Idempotent import, verification command, staging rehearsal on a real backup, read-only source, rollback window |
| GST compliance | Invalid invoices | CA review of SAC, price inclusivity, e-invoicing threshold, invoice layout before enabling |
| Scope size for code generation | Inconsistent code across sessions | Strict phase gates, reference tests, conventions in the prompt, DECISIONS log |
| Third-party SDK changes (Stripe v15, redis-py 8, Authlib 1.8) | Runtime errors | Pinned versions, contract tests with recorded responses |

### 18.2 Open questions for Aryan (answer in `docs/v3/DECISIONS.md`)

1. **Domains:** product domain (app/api), `SHORT_DOMAIN`, sandbox short domain, and the current v1 Railway domains / any custom domains on the v1 services (needed for `LEGACY_HOSTS`).
2. **Cloudflare:** account and zones available? Workers Paid plan acceptable (recommended for request volume)? Cloudflare for SaaS enabled on the short-domain zone?
3. **Railway plan:** Pro (needed for static IPs, 20 domains/service, more replicas, SMTP not required) — confirm.
4. **Payments:** Razorpay account with Subscriptions + Payment Links enabled? Keep Stripe for international? Which v1 Stripe prices are live (for the legacy price map)?
5. **GST (with your CA):** GSTIN, state code (default 09 Uttar Pradesh), SAC code, whether published INR prices include GST, LUT ARN, e-invoicing applicability.
6. **Email:** Resend account and sending domain.
7. **Maintenance window** for cutover (≈ 45 min, night IST) and who approves go/no-go.
8. **v1 API customers:** are there external users of `/api/v1/qr/api/generate`? (Decides whether the compatibility window can be shorter.)

---

## 19. Appendices

### Appendix A — Environment variables

| Variable | Services | Notes |
|---|---|---|
| `APP_ENV` | all | `local|test|staging|production` |
| `DJANGO_SETTINGS_MODULE` | python | `qrit.settings.api|redirect|worker` |
| `DJANGO_SECRET_KEY` | python | 50+ random chars |
| `ALLOWED_HOSTS` | api, redirect | comma list; always include `healthcheck.railway.app` |
| `DATABASE_URL` | python | app role `qrit_app` (pooled when PgBouncer is on) |
| `DATABASE_UNPOOLED_URL` | worker, ingest | app role, direct |
| `DATABASE_OWNER_URL` | api (pre-deploy only) | migration owner |
| `DB_POOL_MIN`, `DB_POOL_MAX` | python | pool sizes per process (default 1/4; per-service values in `RAILWAY_DEPLOY.md` §4) |
| `DB_APP_PASSWORD` | api (pre-deploy) | password `ensure_db_roles` sets for `qrit_app` |
| `V1_DATABASE_URL` | migrator | read-only role on the v1 database (`V1_TO_V3_MIGRATION.md`) |
| `LEGACY_BACKEND_HOST` | web | v1 backend host for the `/r/<code>` 308 |
| `REDIS_URL` | python | `${{Redis.REDIS_URL}}` |
| `APP_BASE_URL`, `API_PUBLIC_URL` | api, redirect, worker, migrator | `https://APP_DOMAIN`, `https://API_DOMAIN` |
| `SHORT_DOMAIN`, `SHORT_DOMAIN_SCHEME` | api, redirect | e.g. `qrit.link`, `https` |
| `SANDBOX_SHORT_DOMAIN` | api, redirect | |
| `LEGACY_HOSTS` | api, redirect | v1 hosts serving `/r/<code>` |
| `EDGE_SHARED_SECRET`, `EDGE_SHARED_SECRET_PREVIOUS` | redirect, web | must equal the Worker secret; the previous value is accepted during rotation |
| `API_INTERNAL_URL` | web | `http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}` (api service keeps its v1 name `backend`) |
| `JWT_ED25519_PRIVATE_KEY`, `JWT_KEY_ID` | api | base64 PKCS#8 / seed |
| `APP_ENCRYPTION_KEY` | python | 32 bytes base64; KEK root |
| `SCAN_SALT_SECRET` | api (legacy `/r/`), redirect, migrator | daily visitor-hash salt root |
| `COOKIE_SECURE`, `COOKIE_DOMAIN` | api | `true` in staging/production |
| `CORS_ALLOWED_ORIGINS` | api | only for third-party browser apps; dashboard is same-origin |
| `STORAGE_ENDPOINT`, `STORAGE_REGION`, `STORAGE_BUCKET`, `STORAGE_ACCESS_KEY_ID`, `STORAGE_SECRET_ACCESS_KEY`, `STORAGE_PUBLIC_BASE_URL` | api, redirect, worker, migrator | S3-compatible |
| `AUDIT_ANCHOR_BUCKET` | worker | optional WORM bucket |
| `RESEND_API_KEY`, `EMAIL_FROM` | api, worker | |
| `GOOGLE_CLIENT_ID` | api, web (`NEXT_PUBLIC_GOOGLE_CLIENT_ID`) | |
| `WEB_RISK_API_KEY` | api, worker | |
| `DOH_URL` | api, worker | default `https://cloudflare-dns.com/dns-query` |
| `CF_API_TOKEN`, `CF_ACCOUNT_ID`, `CF_ZONE_ID`, `CF_SAAS_CNAME_TARGET`, `CF_KV_NAMESPACE_ID` | api, worker | Cloudflare for SaaS custom hostnames; KV `APP_HOSTS` for white-label dashboard hosts |
| `TURNSTILE_SITE_KEY` (web), `TURNSTILE_SECRET_KEY` (api) | | |
| `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, `RAZORPAY_WEBHOOK_SECRET`, `RAZORPAY_PLAN_*` | api, worker | |
| `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_*`, `STRIPE_LEGACY_PRICE_MAP` | api, worker | |
| `SELLER_LEGAL_NAME`, `SELLER_GSTIN`, `SELLER_STATE_CODE`, `SELLER_ADDRESS`, `SELLER_SAC`, `SELLER_LUT_REF`, `SELLER_UPI_VPA`, `SELLER_BANK_DETAILS`, `PRICES_INCLUDE_GST` | api, worker | invoices |
| `SERIAL_MAC_KEY` | api, redirect, worker | serials (`serials.generate` runs on the worker) |
| `VERIFY_TOKEN_KEY` | api, redirect | serial verify-page token |
| `STAFF_IP_ALLOWLIST` | api | CIDRs |
| `FREE_TIER_DAILY_LIMIT` | api | public generator |
| `SENTRY_DSN`, `LOG_LEVEL`, `METRICS_TOKEN` | all | |
| `WEB_CONCURRENCY`, `WORKER_CONCURRENCY` | python | |
| `AUTH_MAX_LOGIN_ATTEMPTS` | api | account lockout threshold (default 5) |
| `JWT_PREVIOUS_PUBLIC_KEYS` | api | JSON list of `{kid, public_key}` accepted during JWT key rotation |
| `LEGACY_GEOIP_LOOKUP_URL` | api, redirect | optional; same contract as v1 `GEOIP_LOOKUP_URL` (`{ip}` placeholder), used only for legacy `/r/` codes with country rules (`V1_TO_V3_MIGRATION.md` §6) |
| `DISABLE_SERVER_SIDE_CURSORS` | python | `true` when PgBouncer transaction mode is on |

### Appendix B — Error codes (stable)

Use the codes the Go implementation already returns (extracted from `reference/go-v2/internal/httpapi`): `allowlist_required, already_decided, already_member, already_owner, api_key_not_allowed, api_keys_cannot_approve, approval_not_pending, approval_requires_user, billing_hold, binding_exists, break_glass_required, code_blocked, comment_required, design_required, destination_required, destination_unsafe, discovery_failed, domain_claimed, domain_exists, domain_unverified, duplicate_rule_id, dynamic_only_type, email_exists, email_mismatch, email_required, empty_body, folder_cycle, folder_exists, folder_not_empty, forbidden, granularity_too_fine, group_exists, group_managed_externally, gtin_exists, hosted_page_required, hosted_page_too_large, idempotency_key_reused, invite_domain_not_allowed, invite_exists, ip_not_allowed, limit_reached, metadata_required, mfa_code_required, mfa_not_enabled, mfa_required, mfa_required_by_org, mfa_verification_required, not_a_member, not_org_member, not_scheduled, oidc_fields_required, org_access_suspended, owner_cannot_leave, owner_immutable, owner_required, payload_too_large, permissions_required, privilege_escalation, public_domain, rate_limited, read_only_over_limit, reason_required, request_in_flight, reserved_key, role_exists, role_in_use, role_required, saml_metadata_rejected, seat_limit_reached, secret_required, self_approval, self_change, self_role_change, session_expired, session_required, slug_exists, sso_account_exists, sso_enforced, sso_not_ready, sso_reauth_required, sso_required, sso_user_not_provisioned, static_content_required, static_immutable, static_no_lifecycle, static_only_type, step_up_required, system_role, tag_exists, template_locked, template_required, test_required, too_deep, too_many_break_glass, too_many_codes, too_many_hosts, too_many_ranges, too_many_rules, too_many_streams, txt_record_not_found, unknown_permission, upgrade_required, use_membership, validation_failed, weak_password, wildcard_not_allowed, workspace_required, would_lock_out`, plus every `invalid_<field>` code (e.g. `invalid_email`, `invalid_design`, `invalid_rules`, `invalid_gstin`). New v3 codes MUST be added to `apps/core/errors.py::CODES` and documented in OpenAPI.

### Appendix C — Go → Python map

| Go package / file | v3 location |
|---|---|
| `internal/apierr`, `idempotency`, `netutil`, `platform/*`, `config`, `envelope`, `flags` | `apps/core` |
| `internal/auth`, `httpapi/{auth_handlers,account_handlers,sessions,identity,mfa_handlers}.go` | `apps/accounts` |
| `internal/org`, `httpapi/{org_handlers,policy_handlers}.go` | `apps/orgs` |
| `internal/workspace`, `httpapi/ws_handlers.go` | `apps/workspaces` |
| `internal/authz`, `internal/access`, `internal/rbac`, `httpapi/{gate,governance_handlers}.go` | `apps/access` |
| `internal/sso`, `httpapi/{sso_handlers,scim}.go`, `worker/identity.go` | `apps/identity` |
| `internal/{qr,shortcode,version,routing,hosted,urlsafety,domains}`, `httpapi/{qr_handlers,qr_validate,organize_handlers}.go` | `apps/qr` (+ `apps/trust` for urlsafety/abuse) |
| `packages/qr-render`, `apps/render` (TS) | `apps/render` |
| `internal/{redirect,resolve,scan}`, `cmd/redirect` | `apps/redirect` |
| `internal/{ingest,analytics,realtime}`, `cmd/ingest`, `httpapi/analytics_handlers.go`, `worker/maintenance.go` (partitions, rollups) | `apps/analytics` |
| `internal/approval`, `httpapi/approval_handlers.go`, `worker/governance.go` | `apps/approvals` |
| `internal/{audit,auditstream,stdwebhook}`, `httpapi/{audit_handlers,auditstream_handlers}.go`, `worker/{audit,auditstream}.go` | `apps/audit` (+ `stdwebhook` in `apps/core/webhooks.py`) |
| `internal/{entitlements,plans,billing,invoicing}` | `apps/billing` |
| `internal/webhooks` | `apps/integrations` |
| `internal/jobs`, `cmd/worker` | procrastinate (`qrit/procrastinate.py`, `apps/*/tasks.py`) |
| `internal/abuse` | `apps/trust` |
| `cmd/migrate` | `manage.py migrate` |
| `cmd/migrate-v1` (broken) | replaced by `apps/legacy` `import_v1` |

### Appendix D — Sources (checked 27–28 Sep 2026)

- Railway: Infrastructure as Code https://docs.railway.com/infrastructure-as-code · Config as Code deprecation https://docs.railway.com/config-as-code · monorepos https://docs.railway.com/deployments/monorepo · private networking https://docs.railway.com/networking/private-networking/how-it-works · PgBouncer https://docs.railway.com/databases/postgresql-pgbouncer · backups https://docs.railway.com/volumes/backups · PITR https://docs.railway.com/volumes/point-in-time-recovery · regions https://docs.railway.com/deployments/regions · domains https://docs.railway.com/networking/domains/working-with-domains · health checks https://docs.railway.com/deployments/healthchecks · pre-deploy https://docs.railway.com/deployments/pre-deploy-command · static IPs https://docs.railway.com/networking/static-outbound-ips · Django guide https://docs.railway.com/guides/django · CDN https://docs.railway.com/networking/cdn · plans https://docs.railway.com/pricing/plans
- Django 6.1 release notes https://docs.djangoproject.com/en/6.1/releases/6.1/ · tasks https://docs.djangoproject.com/en/6.1/topics/tasks/ · async https://docs.djangoproject.com/en/6.1/topics/async/ · CSP https://docs.djangoproject.com/en/6.1/ref/csp/ · connection pool https://docs.djangoproject.com/en/6.1/ref/databases/#connection-pool · Granian how-to https://docs.djangoproject.com/en/6.1/howto/deployment/asgi/granian/
- Python release schedules PEP 745 https://peps.python.org/pep-0745/, PEP 719 https://peps.python.org/pep-0719/
- DRF release notes https://www.django-rest-framework.org/community/release-notes/ · procrastinate Django https://procrastinate.readthedocs.io/en/stable/howto/django/basic_usage.html · psycopg pool news https://www.psycopg.org/psycopg3/docs/news_pool.html · redis-py 8.0 https://github.com/redis/redis-py/releases/tag/v8.0.0 · uvicorn release notes https://github.com/Kludex/uvicorn/blob/main/docs/release-notes.md · granian https://github.com/emmett-framework/granian
- segno https://github.com/heuer/segno · resvg_py (PyPI `resvg_py`) · ReportLab 5 https://docs.reportlab.com/releases/notes/whats-new-50/ · Authlib changelog https://github.com/authlib/authlib/blob/main/docs/upgrades/changelog.rst · pysaml2 changelog https://github.com/IdentityPython/pysaml2/blob/master/CHANGELOG.md · py_webauthn https://github.com/duo-labs/py_webauthn/blob/master/CHANGELOG.md · django-scim2 CHANGES https://github.com/15five/django-scim2/blob/master/CHANGES.txt · Stripe v15 migration https://github.com/stripe/stripe-python/wiki/Migration-guide-for-v15 · razorpay-python https://github.com/razorpay/razorpay-python · MaxMind GeoLite EULA https://www.maxmind.com/en/geolite2/eula
- Enterprise specification: `docs/v2-blueprint/QRit_v2_Enterprise_Plan.md` (GS1 Resolver Standard 1.2.1, Teams Workflows, DPDP) and its sources.
