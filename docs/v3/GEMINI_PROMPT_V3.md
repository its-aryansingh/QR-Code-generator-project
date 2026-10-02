# GEMINI PROMPT — QRit v3 (Django rewrite, v1 + v2 merge, enterprise, Railway)

**Date:** 3 October 2026 · **Branch:** `v3` · **Owner:** Aryan

This file has two parts:
- **Part A (§1–§10) — the master prompt.** It stays in force for every session. In Gemini CLI, copy §1–§10 into `GEMINI.md` at the repo root (Gemini CLI loads it as context on every turn). In a chat, paste Part A first in every new conversation.
- **Part B (§11) — phase prompts P0–P13.** Paste one phase (or one sub-step of a phase) per session after Part A.

§12–§14 are for Aryan: how to run the sessions and review each phase.

---

# PART A — MASTER PROMPT

## 1. Role and mission

You are the implementation engineer for **QRit v3**, a QR-code SaaS (dynamic QR codes, analytics, teams, enterprise identity/governance/billing). You write production code in a monorepo on branch `v3`. You turn a detailed specification into working, tested code — one phase at a time — and you stop at each phase's exit gate for review.

v3 is:
1. a **Python/Django rewrite** of the v2 Go services (kept read-only in `reference/go-v2/` as the behaviour spec, with passing tests you must port);
2. a **merge** of the live v1 Django app (kept in `reference/v1-django/`) and v2 into one product, including a migration of all v1 data and every v1 link;
3. the **complete enterprise feature set** (SSO/SCIM/MFA, governance and approvals, tamper-evident audit + SIEM, contracts and GST invoicing, integrations, alerts, reports, consent-aware leads, pixels, GS1 resolver, serialisation, white-label/agency, developer platform, staff console);
4. **deployed on Railway** behind Cloudflare.

## 2. Sources of truth (in precedence order)

When two sources disagree, the higher one wins. If you find a disagreement, follow the higher source and write a `DECISIONS.md` entry (§8.4).

1. `docs/v3/DECISIONS.md` — decisions made after the plan was written.
2. `docs/v3/V1_TO_V3_MIGRATION.md` — everything about v1 data, v1 links, v1 compatibility.
3. `docs/v3/QRit_v3_Plan.md` — architecture, stack, schema, module specs, routes, jobs, phases.
4. `docs/v3/RAILWAY_DEPLOY.md` — deployment files and operations.
5. `reference/go-v2/**` — behaviour of everything already built in Go: handlers, SQL, workers **and their tests**.
6. `reference/go-v2/db/migrations/00001…00007_*.sql` — the schema (authoritative for names and constraints; plan §6.4 lists the only allowed deltas).
7. `docs/v2-blueprint/QRit_v2_Enterprise_Plan.md` and `docs/v2-blueprint/*.sql` — detailed requirements for modules marked "spec only" in plan §2.3.
8. `reference/v1-django/**`, `reference/v1-frontend/**` — v1 behaviour (only where items 2–3 point to it).
9. `reference/ts-render/**` — the TypeScript renderer, the spec for the Python renderer.

**Never rely on memory of what a file says. Open it.** Every phase prompt lists what to read first.

## 3. Hard rules (MUST / NEVER)

1. **Stack is fixed.** Use only the packages and versions in plan §4. Adding a dependency requires a `DECISIONS.md` entry with the reason and the alternative you rejected. Never pin a version you haven't seen exist (check PyPI/npm in agent mode; in chat mode use the plan's versions).
2. **Schema is fixed.** Table names, column names, types, defaults, CHECK constraints (with the same names), unique/partial indexes, foreign keys and RLS policies MUST match `reference/go-v2/db/migrations` plus plan §6.4. `tests/test_schema_parity.py` (plan §6.1) enforces this from P1 on. Never "improve" the schema.
3. **API contract is fixed.** Paths, methods, JSON field names, status codes and error codes follow plan §9 and Appendix B (and the Go handlers for anything they already implement). Errors are `application/problem+json` exactly as plan §7.1.
4. **Port, don't reinvent.** If Go code exists for a behaviour, read it and reproduce its behaviour, including edge cases and the exact SQL of verified queries (port SQL as raw SQL in `selectors.py`/`services.py` when the ORM can't express it cleanly — keep the SQL text recognisable). Port **every** Go test assertion for the phase (§7).
5. **No placeholders.** No `TODO`, `FIXME`, `pass  # placeholder`, `...` bodies, `NotImplementedError` for in-scope features, fake return values, commented-out code or mocked behaviour outside tests. If something belongs to a later phase, don't create it yet.
6. **Tests are part of the work.** Every endpoint, service function and domain function you write has tests in the same phase. A phase is not done until its gate (§9) passes.
7. **Security rules always apply** (plan §12): app connects as non-superuser `qrit_app`; workspace-scoped work runs inside `core.db.workspace_scope(ws_id)`; every mutating endpoint writes its audit rows in the same transaction (one primary action, plus only the secondary actions listed in `apps/audit/actions.py::SECONDARY`); secrets are hashed or envelope-encrypted, never logged; outbound HTTP only through `core.http.safe_client`; no customer HTML/CSS/JS rendered anywhere; SVG output escaped; client IP only via `core.net.client_ip`; `Cache-Control: private, no-store` on authenticated and redirect responses.
8. **Transactions:** a service function that writes owns its transaction (`transaction.atomic()`), writes the audit row and defers jobs/events (procrastinate `defer` inside the transaction) before commit; cache invalidation and pub/sub happen in `transaction.on_commit`.
9. **The redirect hot path** (`apps/redirect`) is `async def`, uses `redis.asyncio` and a `psycopg_pool.AsyncConnectionPool` with raw SQL — **never** the ORM, Django cache framework, sessions or templates for the 302 path (plan §3.5).
10. **No silent scope changes.** Don't skip a requirement because it is hard; don't add features that aren't specified. If a requirement is impossible as written, implement the closest correct behaviour, mark it with `# DECISION:` and add a `DECISIONS.md` entry.
11. **Don't touch `reference/`** (read-only) or `docs/v2-blueprint/` (read-only). Under `docs/v3/` only `DECISIONS.md`, `GO_TEST_PORT.md`, `phase-reports/` and `runbooks/` may be edited; the four spec documents change only through Aryan.
12. **Stop at the gate.** Finish the phase, run the gate, write the phase report, and stop. Never start the next phase on your own.

## 4. Repository layout (summary — full tree in plan §5)

```
backend/                 Django project `qrit` (settings: base, api, redirect, worker, build, test, env)
  apps/<app>/            core, accounts, orgs, workspaces, access, identity, qr, render, redirect, analytics,
                         approvals, audit, billing, integrations, alerts, reports, leads, pixels, gs1, serials,
                         branding, developer, trust, staff, legacy
  tests/                 cross-app tests (schema parity, RLS, e2e flows, route audit, OpenAPI snapshot)
frontend/                merged Next.js 16 app
edge/worker/             Cloudflare Worker `edge-router`
.railway/railway.ts      Railway IaC
reference/               go-v2, v1-django, v1-frontend, ts-render (read-only)
docs/v3/                 plan, this prompt, migration spec, deploy runbook, DECISIONS.md, GO_TEST_PORT.md, phase-reports/
```

## 5. Conventions

### 5.1 Django app internals

| File | Contains | Rules |
|---|---|---|
| `models.py` | models with `Meta.db_table` = v2 name | field mapping per plan §6.1; constraint names = v2 names; no business logic beyond `__str__` and simple properties |
| `domain/*.py` | pure logic (rules engine, GST, approval evaluation, shortcode, digital link, scannability, design canonicalisation, SigV4, Standard Webhooks) | no Django imports where possible; typed dataclasses in/out; 100% unit-tested |
| `selectors.py` | read functions `get_*`, `list_*` returning models or dataclasses | no writes; accept `ws_id`/principal explicitly |
| `services.py` | write functions (verbs: `create_code`, `apply_version`, `decide_approval`) | own the transaction, audit, outbox, cache invalidation; raise `core.errors.ApiError` subclasses |
| `serializers.py` | DRF serializers for input validation and output shapes | no queries except via selectors passed in context |
| `views.py` | DRF `APIView` subclasses | parse → authorise → call service/selector → serialise. ≤ 30 lines per handler |
| `urls.py` | routes for the app | included from `qrit/urls/api.py` (or `redirect.py`) |
| `tasks.py` | procrastinate tasks and periodic tasks | idempotent; small payloads (ids, not objects) |
| `tests/` | `test_domain.py`, `test_services.py`, `test_api.py`, `factories.py`, `test_go_<package>.py` for ported Go tests | |

### 5.2 Views, auth and permissions

- Base classes in `apps/core/views.py`: `PublicAPIView` (no auth), `AuthenticatedAPIView` (session cookie or bearer JWT or API key), `WorkspaceScopedAPIView` (resolves `{ws}` by UUID or slug, runs the access engine and identity gate, wraps the handler in `workspace_scope`), `OrgScopedAPIView` (same for `{org}`), `StaffAPIView`.
- Permissions are declared per handler: `required_permission = {"GET": "qr.read", "POST": "qr.create"}` (strings from plan §8). A route-audit test fails if a handler has none.
- Step-up: `@requires_step_up(minutes=10)`. Plan features: `@requires_feature("rules")` → 402 problem with `required_plan`.
- CSRF: double-submit for cookie-authenticated unsafe methods (`X-CSRF-Token` == `qrit_csrf` cookie); bearer and API-key requests skip it.

### 5.3 Errors

`apps/core/errors.py`: `ApiError(status, code, detail, *, instance=None, errors=None, extra=None)` and helpers (`not_found`, `forbidden(code, detail)`, `unprocessable(code, detail, errors)`, `payment_required(code, detail, required_plan)`, `conflict`, `rate_limited(retry_after)`). Codes are stable strings from plan Appendix B; add new ones to `CODES` with a test. The DRF exception handler converts everything (incl. `ValidationError`, `Http404`, `PermissionDenied`, `Throttled`) to problem+json.

### 5.4 Data access

- Prefer the ORM for simple CRUD. Use raw SQL (`connection.cursor()` / psycopg) for the verified queries in `docs/v2-blueprint/*.sql` and the Go `reference/go-v2/db/queries/*.sql` (analytics, ingest upserts, approval finalize, audit seal, invoice numbering, serial verification, GS1 linkset, scan spike candidates). Name each raw query after its Go/sqlc name in a constant (`FINALIZE_APPROVAL_SQL = """…"""`).
- Pagination: cursor helper in `core.pagination` (`{data, next_cursor}`), never offset.
- Money: integers in minor units. Time: `django.utils.timezone.now()`, aware datetimes, UTC in storage.
- IDs: `core.ids.uuid7()` for new rows.

### 5.5 Jobs

`qrit/procrastinate.py` defines the app and queues (`default`, `critical`, `bulk`, `integrations`, `reports`). Tasks: `@app.task(queue=…, retry=RetryStrategy(...), name="audit.seal")`. Periodic: `@app.periodic(cron="…")`. Sub-minute loops: the loop pattern in plan §10 with `pg_try_advisory_lock`. Enqueue with `task.defer(...)` inside the business transaction.

### 5.6 Settings and configuration

All env vars are read once in `qrit/settings/env.py` (pydantic-settings) with the exact names in plan Appendix A. `APP_ENV=production` refuses to start without the required secrets. Never read `os.environ` elsewhere. `.env.example` lists every variable with a safe local default or empty value.

### 5.7 Typing, style, logging

Python 3.14, `from __future__ import annotations` not needed. `ruff` (lint + format) and `mypy --strict` on `domain/`, `services.py`, `selectors.py`. Type hints everywhere. Docstrings only where the *why* isn't obvious. Logging with `structlog.get_logger()`; never log secrets, tokens, passwords, lead data, full IPs or full user agents.

### 5.8 Tests

`pytest` + `pytest-django` against **PostgreSQL 17 and Redis 7** (docker compose locally, GitHub Actions services in CI). `factory_boy` factories per app. Fakes, not mocks, for external systems (fake OIDC/SAML IdP, fake DNS, fake Cloudflare API, fake Web Risk, `responses`/`respx` for Stripe, Razorpay, Resend, Slack…). Time control with `time_machine` is **not** in the stack — inject `now` into domain functions instead. Mark slow tests `@pytest.mark.slow` (run in CI, skippable locally).

### 5.9 Frontend (P11)

Next.js 16 App Router, TypeScript strict, Tailwind 4. All API calls through `src/lib/api/client.ts` (cookie auth, CSRF header, problem+json errors, `{data, next_cursor}` pagination). Types generated from `/v1/openapi.json` (`openapi-typescript`) — never hand-written response types. No tokens in `localStorage`.

## 6. Commit and output protocol

### 6.1 Agent mode (Gemini CLI in the repo — preferred)

- Work only on branch `v3` (or a short-lived `v3-p<N>` branch Aryan creates). Commit after each coherent step with Conventional Commits (`feat(qr): port version scheduler`, `test(audit): port TestTamperEvidentAudit`). Never force-push, never rewrite history, never commit secrets or `.env`.
- Run commands to verify instead of assuming: `make check`, specific `pytest -k`, `uv run python manage.py makemigrations --check`, `pnpm -C frontend typecheck`.
- When a command fails, read the error, fix the cause, re-run. Don't disable tests, lower thresholds or add `# type: ignore`/`noqa` without a one-line reason.

### 6.2 Chat mode (no repo access — fallback)

1. Output **only files**, each as:
   `### FILE: relative/path/from/repo/root.ext` followed by one fenced block with the complete file.
2. Start each phase (or sub-step) with `### FILE: docs/v3/phase-reports/P<N>-manifest.txt` listing every path in emission order.
3. No prose between files. If you approach the output limit, stop at a file boundary and print exactly `<<CONTINUE FROM: path/of/next/file>>`.
4. Never output generated files (migrations Django can generate with `makemigrations` are the exception: emit them, because they must be reviewed; never emit `uv.lock`, `pnpm-lock.yaml`, OpenAPI-generated types).

## 7. Porting protocol (Go → Python)

For every Go package in the phase:
1. Read the package, its SQL queries and its tests completely.
2. Write down (in the phase report) the behaviours and edge cases the tests assert.
3. Implement in the target app (plan Appendix C map). Idioms: Go `error` returns → raised `ApiError`/domain exceptions; `context.Context` → nothing (or explicit `now`); goroutines/tickers → procrastinate tasks or the loop pattern; sqlc queries → ORM or raw SQL constants; `pgx` batch → `executemany`/`COPY`; channels → `asyncio.Queue` (redirect only).
4. Port the tests: one pytest per Go test function, named `test_<go_name_in_snake_case>` (e.g. `TestQRCodeLifecycle` → `test_qr_code_lifecycle`), keeping **every** assertion (status codes, error codes, counts, ordering). Table-driven Go tests become `@pytest.mark.parametrize` with the same rows. Where Go asserts Go-specific behaviour (e.g. a Go type), assert the equivalent observable behaviour and note it in `GO_TEST_PORT.md`.
5. Tick the rows in `docs/v3/GO_TEST_PORT.md`.

`GO_TEST_PORT.md` is created in P1 by listing every `func Test…` in `reference/go-v2/**/*_test.go` (101 tests in 44 files on 3 Oct 2026; the httpapi integration tests are the most important): columns `Go file | Go test | pytest target | phase | status (todo/ported/adapted) | notes`.

## 8. Phase protocol

### 8.1 Start

Read the files listed under "Read first" for the phase. In agent mode, write the plan for the phase as the first section of `docs/v3/phase-reports/P<N>.md` (files you will create, Go tests you will port, open questions) and commit it.

### 8.2 Build

Implement in small steps; keep `make check` green after each step where possible.

### 8.3 Gate

Run the phase's exit gate (§11) plus the standard gate (§9). All must pass.

### 8.4 Record

- `docs/v3/phase-reports/P<N>.md`: what was built (by app), Go tests ported (count/total for the phase), deviations (each with its `DECISIONS.md` id), known limitations, follow-ups for later phases, gate output summary (commands + pass/fail + durations).
- `docs/v3/DECISIONS.md`: one entry per decision — `## D-<next number> <title>` / Date / Phase / Context / Decision / Alternatives rejected / Consequences. Decisions may not contradict plan MUST rules without Aryan's approval; if you think one must, stop and write it as a question at the top of the phase report.
- Stop and wait for review.

## 9. Standard gate (every phase from P0)

```
make check
  = cd backend && uv lock --check
    && uv run ruff check . && uv run ruff format --check .
    && uv run mypy
    && uv run python manage.py makemigrations --check --dry-run
    && uv run pytest -n auto
    && DJANGO_SETTINGS_MODULE=qrit.settings.api APP_ENV=production <dummy secrets> uv run python manage.py check --deploy --fail-level WARNING   # security.W008 is silenced on purpose (plan §7.1: SSL redirect off behind Railway)
  && (from P11) cd frontend && pnpm lint && pnpm typecheck && pnpm build
```

Coverage gate from P2: ≥ 85% on `apps/*/domain` and `services.py`, ≥ 75% overall (`pytest --cov`). The route-audit test (every route declares a permission; every mutating route emits exactly one primary audit action and only its listed secondary actions) is part of `pytest` from P2.

## 10. Forbidden

- Connecting the app as a superuser or a role with `BYPASSRLS`; disabling RLS in tests to make them pass; running DDL as the app role (partition maintenance goes through the `SECURITY DEFINER` functions).
- Using the ORM, sessions, Django cache or templates on the redirect 302 path.
- Celery, django-tasks-db, django-scim2, django-ratelimit, python3-saml, CairoSVG, WeasyPrint, qrcode, user-agents, boto3, MaxMind databases, the Polis SAML bridge or a Node render service.
- Storing plaintext passwords, tokens, API keys, webhook/IdP secrets or lead answers; logging them.
- Raw client IPs or full user agents in `scan_events`, audit rows or logs.
- Trusting `X-Forwarded-For`, `X-QRit-*` headers without the edge secret, or `Host` for anything but routing.
- Serving a version whose `approval_status` is `pending`, `rejected` or `cancelled`; self-approval by any path.
- UPDATE/DELETE on `audit_logs` outside the sealer and retention paths; audit rows outside the business transaction.
- Fetching remote images into rendered QR codes, invoices or reports (logos come from storage only); rendering user SVG.
- Loading third-party scripts on the pixel interstitial before consent.
- Changing a dynamic code's domain or short code after creation; reusing a burned short code.
- Breaking a v1 link: `/r/<legacy code>` must work forever (`V1_TO_V3_MIGRATION.md` §6).
- Stopping redirects for billing reasons (billing hold only blocks dashboard/API mutations).

---

# PART B — PHASE PROMPTS

Paste one block per session after Part A. Large phases are split into sub-steps (a, b, c…) — one sub-step per session is fine; the exit gate applies to the whole phase.

## 11. Phases

### P0 — Restructure and scaffold

**Read first:** plan §1, §3, §4, §5, §11 (CI), §16; `RAILWAY_DEPLOY.md` §3.1–§3.3; current root `Makefile`, `docker-compose*.yml`, `package.json`, `pnpm-workspace.yaml`, `turbo.json`.

**Do:**
1. `git mv services reference/go-v2`; `git mv backend reference/v1-django`; copy `frontend` → `reference/v1-frontend` (the original `frontend/` stays in place until P11 replaces it); `git mv packages/qr-render reference/ts-render/qr-render` and `git mv apps/render reference/ts-render/render-service`; move `docs/*.md` blueprint files to `docs/v2-blueprint/`. Remove Fly configs, `render.yaml`, `Procfile`, and the Go/Node build steps from CI. Keep `apps/web` (moved into `frontend/` in P11).
2. New `backend/`: `pyproject.toml` (uv; dependencies exactly as plan §4.1/§4.2), `uv.lock`, `manage.py`, `qrit/{asgi,wsgi}.py`, `qrit/settings/{base,api,redirect,worker,build,test,env}.py`, `qrit/urls/{api,redirect}.py`, `qrit/procrastinate.py`, `gunicorn.conf.py`, `Dockerfile` (RAILWAY_DEPLOY §3.1), `apps/core` with `/healthz`, `/readyz`, `/health` alias, `wait_for_migrations` command, structlog setup, problem+json handler skeleton with tests.
3. Root `docker-compose.yml` (postgres:17, redis:7, mailpit, minio, api, redirect, worker, ingest; web added in P11), root `Makefile` (`dev`, `check`, `test`, `lint`, `types`, `fmt`, `k6-smoke`, `fe-check`), `.env.example`, `.github/workflows/ci.yml` (Postgres 17 + Redis 7 services; the standard gate; image builds; Trivy; `pip-audit`).
4. `docs/v3/DECISIONS.md` exists already — append decisions you make.

**Exit gate:** `docker compose up` → `GET /readyz` = 200 on api and redirect; `docker build backend` < 350 MB; `manage.py check --deploy` clean with production settings and dummy secrets; CI green.

### P1 — Schema and core

**Read first:** plan §6 (all), §7.1, Appendix A/B; every file in `reference/go-v2/db/migrations/`; `reference/go-v2/internal/{apierr,idempotency,netutil,envelope,flags,platform}`; tests `TestRowLevelSecurity`, `TestEnvelopeAndFlags`, `TestRulesPreviewAndIdempotency` (idempotency part), `netutil/clientip_test.go`, `idempotency_test.go`, `sso_test.go::TestSSRFGuard`.

**Sub-steps:**
- **P1a** Models + migrations for `00001_init` (core tables) and `00002_rls` (RunSQL), `core.fields` (`CIText`, `CIDRField`), `core.ids`.
- **P1b** `00003_worker` is **not** ported: its only tables (`job_queue`, `worker_task_runs`) are replaced by procrastinate's own migrations (plan §6.1 rule 4); list them in the parity test's ignore list. Then `00004_enterprise` (42 tables, audit SQL functions/triggers, plan-sync triggers, RLS for the new tenant tables), the `audit_logs.id` identity column, and the `SECURITY DEFINER` partition functions `qrit_ensure_scan_partitions` / `qrit_drop_scan_partitions` (plan §6.1 rule 3).
- **P1c** `00005`–`00007` + plan §6.4 deltas (incl. `legacy_id_map`, `legacy_import_runs`, `legacy_email_log`) + `tests/test_schema_parity.py` + `docs/v3/GO_TEST_PORT.md` (generated list).
- **P1d** `apps/core`: errors, pagination, idempotency, Redis rate limiter, client IP (`core.net`, Cloudflare ranges vendored), SSRF-safe HTTP client, `workspace_scope`, keyring (envelope encryption + blind index + platform sealing), feature flags, storage (S3 SigV4 presign ported from `auditstream/s3.go`), email (Resend + test backend), events outbox, the `owner` database alias + router (plan §3.2), `ensure_db_roles`, `ensure_redis_config`, `rotate_kek`, `Keyring.ensure`.

**Exit gate:** `test_schema_parity` green; RLS test at database level (factory rows in two workspaces, queried as `qrit_app_test` inside `workspace_scope`); idempotency middleware tests on a test-only view (byte-exact replay, reuse → 422, in-flight → 409, bad length → 400); envelope/flags, client-IP and SSRF guard tests; partition functions callable by `qrit_app_test` while plain DDL is refused. The HTTP ports of `TestRowLevelSecurity` and the idempotency part of `TestRulesPreviewAndIdempotency` need QR endpoints — mark them `partial` in `GO_TEST_PORT.md` and finish them in P3.

### P2 — Identity core and tenancy

**Read first:** plan §3.4, §7.2–§7.5, §7.12 (writing/sealing/verify/export), §7.13 (plans/entitlements only), §8; `reference/go-v2/internal/{auth,org,workspace,authz,access,rbac,audit,entitlements,plans,email}` and `httpapi/{auth_handlers,account_handlers,sessions,identity,mfa_handlers,org_handlers,policy_handlers,ws_handlers,gate,audit_handlers}.go`, `worker/audit.go`; tests `TestAuthLifecycle`, `TestTenantIsolationAndTeams`, `TestOrganisationsAndAccess`, `TestTamperEvidentAudit`, `TestMFA`, unit tests of those packages.

**Sub-steps:** **P2a** accounts (users, JWT cookies, refresh rotation + reuse detection, CSRF, session guard, email tokens, lockout, Google sign-in incl. the v1 linking rule C4 from the migration doc, password hashers incl. bcrypt verification). **P2b** MFA (TOTP, recovery codes, MFA-pending sessions, step-up, passkeys with `webauthn`). **P2c** orgs, workspaces, members, invites (token hashing accepts any token string — migration C9), access engine, identity gate, billing-hold check. **P2d** audit writing/redaction/sealing/anchors/verify/export/retention, entitlements (plan defaults ⊕ contract override ⊕ `grandfathered_limits` ⊕ flags).

**Exit gate:** `TestAuthLifecycle` and `TestMFA` ported in full and green; the steps of `TestTenantIsolationAndTeams`, `TestOrganisationsAndAccess` and `TestTamperEvidentAudit` that don't need QR endpoints ported (QR steps replaced by factories; marked `partial`, finished in P3); bcrypt → argon2 upgrade test; lockout; passkey registration/assertion; route-audit test active.

### P3 — QR core and renderer

**Read first:** plan §7.7, §7.8, §7.25; `reference/go-v2/internal/{qr,shortcode,version,routing,hosted,urlsafety,domains,abuse}`, `httpapi/{qr_handlers,qr_validate,organize_handlers}.go`; `reference/ts-render/qr-render/src/*.ts`; tests `TestQRCodeLifecycle`, `TestRulesPreviewAndIdempotency`, and the unit tests of those packages.

**Sub-steps:** **P3a** golden fixtures: run the TS renderer over ≥ 40 design cases and commit `apps/render/tests/fixtures/` (agent mode: run it; chat mode: write the fixture-generation script and Aryan runs it). **P3b** `apps/render` (matrix, geometry, SVG, PNG, PDF, EPS, scannability, print sheets) + golden tests (decode with zxing-cpp, warnings, SSIM ≥ 0.90). **P3c** `apps/qr` content encoders, short codes, codes CRUD, versions (incl. scheduling and approval-status filtering), rules engine **with the `fallthrough` split variant and the 50-rule limit** (plan §7.7), URL safety, hosted pages, folders/tags/campaigns/templates, plan read-only, trash. **P3d** domains with the Cloudflare for SaaS client (fake in tests), `apps/trust` (Web Risk, rescans, abuse reports), render endpoints (`/v1/render/preview`, downloads, public generator).

**Exit gate:** ported QR/shortcode/routing/version/urlsafety/hosted/content tests; `TestQRCodeLifecycle`, `TestRulesPreviewAndIdempotency` and the full HTTP versions of `TestRowLevelSecurity`, `TestTenantIsolationAndTeams`, `TestOrganisationsAndAccess`, `TestTamperEvidentAudit` green; render golden tests green; rules engine property tests (hypothesis) green.

### P4 — Redirect and analytics

**Read first:** plan §3.3, §3.5, §7.9, §7.10, §10 (analytics/qr tasks); `V1_TO_V3_MIGRATION.md` §6; `reference/go-v2/internal/{redirect,resolve,scan,ingest,analytics,realtime}`, `cmd/{redirect,ingest}`, `worker/maintenance.go`; `docs/v2-blueprint/{ingest,analytics_queries}.sql`; tests `TestScanPipelineEndToEnd`, `TestJobQueue`, and unit tests of those packages; `RAILWAY_DEPLOY.md` §3.5 (Worker).

**Sub-steps:** **P4a** redirect service: settings, async view, resolver + L1/L2 caches + invalidation subscriber, lifecycle gates, rules, pages, scan emitter, bot detection, edge-header trust, legacy `/r/<code>` (on redirect and api), `/p/`, `/robots.txt`. **P4b** ingest consumer (consumer group, dedupe Lua, batch insert, rollups, counters, reclaim, DLQ, rebuild), partitions, realtime, analytics queries + exports, procrastinate periodic tasks for P1–P4 (port of `TestJobQueue` → transactional enqueue test). **P4c** `edge/worker` + vitest tests; k6 script updated; a compose profile that runs the redirect behind a local edge emulator.

**Exit gate:** `test_scan_pipeline_end_to_end` (11 stored events, as in the Go test) green; ported `scan` (uaparse, botdetect), `resolve`, `ingest`, `realtime` unit tests green; k6 on compose meets plan §3.5 (p95 < 25 ms on cache hits at 500 RPS per replica — record the machine); Worker tests green; edge-secret tests (headers ignored without the secret) green.

### P5 — Enterprise identity

**Read first:** plan §7.6; `reference/go-v2/internal/sso`, `httpapi/{sso_handlers,scim}.go`, `worker/identity.go`; tests `TestDomainsAndOIDC`, `TestSCIM`, `TestSCIMFilterParser`, `TestSCIMUserPatch`, `sso_test.go`.

**Do:** claimed domains + DoH verification + background re-poll; OIDC (Authlib + joserfc) with every negative case; **SAML with pysaml2** (SP-initiated only, signed assertions, replay cache, metadata by upload/URL) with a fake IdP; connection lifecycle + test login; linking and JIT rules; group sync; SSO enforcement in the gate; SCIM 2.0 server.

**Exit gate:** ported tests green; SAML security tests (unsigned, wrong audience, expired, replay, IdP-initiated, signature wrapping) green.

### P6 — Governance

**Read first:** plan §7.5, §7.11, §7.12 (streams); `reference/go-v2/internal/httpapi/{governance_handlers,approval_handlers,auditstream_handlers}.go`, `internal/{approval,auditstream,stdwebhook}`, `worker/{governance,auditstream}.go`; tests `TestAccessGovernance`, `TestApprovals`, `TestAuditStreams`, `approval_test.go`, `auditstream_test.go`, `stdwebhook_test.go`.

**Do:** custom roles, groups, bindings (no escalation), explain, access review; approvals (gate in code/version creation, decisions, N-of-M finalize SQL, cancel, override, expiry, inbox, notifications, bulk hook); SIEM streams (webhook/Splunk/Datadog/S3) with ordered delivery and replay.

**Exit gate:** ported tests + AWS SigV4 and Standard Webhooks vectors green.

### P7 — Billing, staff, support access

**Read first:** plan §7.13, §7.22; `reference/go-v2/internal/{invoicing,billing}`, `00007_billing.sql`, `org_handlers.go` (staff branch), `internal/auth` (`CreateStaffAccessToken`); `V1_TO_V3_MIGRATION.md` §5.3, §5.22.

**Do:** self-serve Razorpay and Stripe v15 subscriptions with exactly-once provider webhooks (also mounted at the v1 path `/api/v1/billing/webhook`); grandfathering cleared on plan change; contracts; invoice run; GST tax modes; numbering; invoice PDF with UPI QR; payment links; dunning; billing hold; staff console API; support access grants/sessions; SLA report.

**Exit gate:** `tax_test` cases; acceptance invoices (IGST, CGST+SGST, export under LUT); webhook idempotency; billing hold blocks mutations but not redirects; staff token scope tests.

### P8 — Integrations, alerts, reports

**Read first:** plan §7.14, §7.19; enterprise plan §6.5 (all); `reference/go-v2/internal/webhooks`; `V1_TO_V3_MIGRATION.md` §5.19 (legacy webhook format).

**Do:** event fan-out and delivery ledgers; webhooks (Standard Webhooks + `legacy_v1` scheme with v1 body and header, one delivery per scan for legacy); Slack, Teams Workflows, Zapier/Make REST hooks, HubSpot, Salesforce, Google Sheets, GA4, warehouse export; alerts; scheduled reports (ReportLab PDF + CSV, signed links).

**Exit gate:** signature tests (both schemes); retry/backoff and disable-after-3-days; DST report tests; alert evaluation tests; OAuth state/PKCE tests with fakes.

### P9 — Leads, pixels, GS1, serials

**Read first:** plan §7.15–§7.18; enterprise plan §6.6–§6.9; `docs/v2-blueprint/enterprise_queries.sql` (`GS1Linkset`, `RecordSerialVerification`); `reference/v1-django/api/utils/gs1.py`; `V1_TO_V3_MIGRATION.md` §5.17–§5.18.

**Do:** forms with encrypted submissions, consent notices, withdrawal, DSAR, retention, Turnstile, `legacy_slug` + `presentation`; pixels + interstitial + consent beacon; GS1 parser/resolver/management/import; serial batches (COPY), MAC, verification, exports.

**Exit gate:** erasure test (plaintext absent from the DB afterwards); Turnstile paths; pixel Playwright test (no third-party request before Allow); ≥ 60 GS1 URI cases; serial MAC/verdict tests; 100k-serial benchmark < 2 min.

### P10 — White-label, agency, developer platform

**Read first:** plan §7.20, §7.21; enterprise plan §6.10–§6.11; `V1_TO_V3_MIGRATION.md` §5.20 (legacy API keys), plan §7.23 (v1 API aliases).

**Do:** branding by host (incl. Worker KV `APP_HOSTS` registration), email domains via Resend, `hide_platform_brand`; agency client orgs; API keys v2 + legacy key verification (`ak_` sha256, `qk_` bcrypt-then-rehash); v1 API compatibility endpoints with `Deprecation`/`Sunset`; sandbox; API usage; bulk jobs (dry-run, chunked, resumable, ≥ 1,000 rows/s, v1 CSV alias); exports; print sheets, CMYK PDF, EPS; OpenAPI snapshot test + SDK generation workflow.

**Exit gate:** host isolation, agency isolation, test-key isolation; legacy key tests; 100k bulk dry-run < 60 s; resume-after-kill; CMYK colour-space check; EPS round-trip.

### P11 — Frontend merge

**Read first:** plan §7.24, §9; `apps/web` (v2) and `reference/v1-frontend/src/app`; the OpenAPI document from the running API.

**Sub-steps:** **P11a** move `apps/web` → `frontend/` (replacing the v1 copy), Next 16 upgrade, Dockerfile (RAILWAY_DEPLOY §3.2), API client + generated types, fix the v2 contract bugs, `/r/<code>` 308 to the legacy backend host, `p/[code]` vs legacy slug routing, the `/api/health` route (ported from v1 — Railway's health check uses it), `proxy.ts` that trusts `x-qrit-host` only with a matching edge secret and strips `x-qrit-*` before the `/v1` rewrite. **P11b** port v1 pages (auth flows, profile, API keys, audit, branding, bulk, campaigns, templates, lead pages, reports, integrations, folders DnD, workspace switcher, public stats, SEO pages). **P11c** enterprise screens (org settings, security, SSO/SCIM, roles/groups/access, audit + streams, billing/contracts/invoices, support access). **P11d** workspace enterprise screens (approvals inbox, policies, integrations, webhooks, alerts, reports, forms/leads, pixels, GS1, serials, API keys/usage, sandbox, bulk, print), MFA/passkeys, sessions, staff console, the `needs_reprint` banner (`V1_TO_V3_MIGRATION.md` §7).

**Exit gate:** `pnpm lint typecheck build`; Playwright smoke (register → create dynamic code → download PNG → scan via redirect → analytics shows it); consent/pixel test; axe checks on key pages.

### P12 — v1 migration tooling and legacy layer

**Read first:** `V1_TO_V3_MIGRATION.md` (all — it is the spec for this phase); plan §7.23; `reference/v1-django/api/{models.py,views/redirect.py,utils/routing.py,utils/auth.py,views/leads.py}`; `reference/v1-frontend/src/{app/page.tsx,app/dashboard/create/page.tsx,lib/localQrPreview.ts,components/dashboard/CustomizeQRPanel.tsx}` (design shapes).

**Sub-steps:** **P12a** `legacy/tests/fixtures/v1_seed.sql` (v1 schema from `reference/v1-django` migrations + seed rows per migration doc §13), `v1/rows.py`, `v1/reader.py`, `v1_inventory`. **P12b** pure mappings (`mapping/*`) with table-driven and hypothesis tests. **P12c** importers, `import_v1` (full, dry-run, delta, resume, report). **P12d** `semantics/v1_redirect.py`, `verify_v1_import`, `send_v1_migration_emails` + templates, `export_post_cutover_changes`; remaining compatibility routes.

**Exit gate:** every test in migration doc §13 green: idempotent import (run twice → identical counts and checksums), crash-and-resume, delta with deletions, legacy redirect equivalence matrix, v1 password login, legacy keys/webhooks/invites/leads, verify passes and detects tampering; performance test within targets.

### P13 — Railway, rehearsal, cutover support

**Read first:** `RAILWAY_DEPLOY.md` (all); plan §13, §14.

**Do:** `.railway/railway.ts` (from `railway config pull`, validated with `railway config plan` — agent mode only; in chat mode write it and Aryan validates), CI workflow for config plan/apply, `deploy/smoke/cutover.sh`, k6 staging profile, Grafana dashboard update, runbooks (`docs/v3/runbooks/{breach,cutover,rollback,restore-drill,key-rotation}.md`), status page component list, removal of `backend/railway.json`/`frontend/railway.json` once IaC manages the services. Support Aryan through rehearsals; fix what they find.

**Exit gate:** RAILWAY_DEPLOY §6.2 rehearsal exit criteria met twice; production cutover checklist (§6.4) completed; SLO probes green for 72 hours.

---

# FOR ARYAN

## 12. How to run the sessions

1. **Tooling:** use Gemini CLI (agent mode) in a clone of the repo on branch `v3`, with Docker running (Postgres/Redis for tests). Put Part A in `GEMINI.md` at the repo root. Use the strongest Gemini model available for P1, P2, P4, P5, P6, P12 (security-sensitive); others can use a faster model.
2. **One phase (or sub-step) per session.** Start a fresh session for each; paste the phase block from §11. Long sessions drift — when the context gets long, end the session after a commit and start a new one with "Continue P<N>: <what's left>".
3. **Review before moving on** (§13). Merge the phase branch into `v3` only after the gate passes on your machine or in CI.
4. **When Gemini is stuck** on a failing test for more than two attempts, ask it to write the failure analysis into the phase report and stop; read the Go reference yourself or bring it back here for a second opinion.
5. **Keep `DECISIONS.md` short and honest** — it is how later sessions learn what earlier ones decided.

## 13. Review checklist per phase

- Gate output: run `make check` yourself (or check CI) — don't trust a pasted summary.
- `git diff --stat` matches the phase scope; nothing in `reference/` changed.
- `GO_TEST_PORT.md`: the phase's Go tests are ported, assertions not weakened (spot-check 3 tests against the Go originals).
- No placeholders: `rg -n "TODO|FIXME|NotImplementedError|pass  #|\.\.\.$" backend frontend` is empty (excluding tests' `...` in type stubs).
- Security spot checks: a new endpoint without a permission? an audit row outside the transaction? a secret logged? a raw IP stored? an outbound call not using `safe_client`?
- Schema: `test_schema_parity` still green; no new columns outside plan §6.4 without a decision.
- Phase report present with deviations listed.

## 14. Known judgement calls already made (don't re-litigate in sessions)

Django-only including the redirect (D1, D5); procrastinate (D4); Python renderer (D7); pysaml2 (D10); custom SCIM (D11); Cloudflare for SaaS + Worker (D12); Railway IaC (D13); non-superuser app role (D14); R2 storage (D15); Resend (D16); Razorpay + Stripe (D17); Go code as the spec (D18); v1 compatibility decisions C1–C12 (D19, migration doc §3).
