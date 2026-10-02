# QRit v3 — Implementation Package

QRit v3 replaces both the live v1 Django app and the unreleased v2 Go services with **one Django 6.1 codebase** (API, redirect hot path, workers, ingest, QR renderer) and **one merged Next.js 16 frontend**, carries over **all v1 data and links**, adds the **complete enterprise feature set**, and runs on **Railway (Singapore) behind Cloudflare**.

| Document | What it is | Who uses it |
|---|---|---|
| [`QRit_v3_Plan.md`](QRit_v3_Plan.md) | The master plan: decisions, current state, architecture, pinned stack, repo layout, schema, every module (7.1–7.25), permissions, routes, jobs, tests, security, observability, phases P0–P13, parity matrix, risks | Gemini (spec) · Aryan (review) |
| [`GEMINI_PROMPT_V3.md`](GEMINI_PROMPT_V3.md) | How Gemini works: master prompt (rules, conventions, porting and phase protocol, gates, forbidden list) + paste-ready prompts for each phase | Gemini · Aryan (runs sessions) |
| [`V1_TO_V3_MIGRATION.md`](V1_TO_V3_MIGRATION.md) | Field-level v1 → v3 mapping, `import_v1` / `verify_v1_import`, legacy links, printed codes, emails, rollback | Gemini (P12) · Aryan (runs the import) |
| [`RAILWAY_DEPLOY.md`](RAILWAY_DEPLOY.md) | Railway + Cloudflare setup, Dockerfiles, IaC, variables, DB roles, pooling, backups, rehearsal, cutover, rollback | Gemini (P0, P13) · Aryan (operates) |
| [`DECISIONS.md`](DECISIONS.md) | Decisions made after the plan + open questions to answer | Everyone |
| `GO_TEST_PORT.md` | Checklist of every Go test and its pytest port (created in P1) | Gemini · Aryan |
| `phase-reports/P<n>.md` | What each phase built, deviations, gate results | Aryan (review) |

## Start here (Aryan)

1. Answer the open questions in `DECISIONS.md` that P0–P4 need (domains, Cloudflare, Resend).
2. Collect the v1 facts the later phases need: the Railway domains and custom domains on the v1 backend and frontend services, the v1 `SHORT_LINK_BASE_URL`, rough row counts, and the live Stripe price ids. (The `v1_inventory` command built in P12 produces the full report.)
3. Set up Gemini CLI on branch `v3` with Part A of `GEMINI_PROMPT_V3.md` as `GEMINI.md`, then run **P0**.
4. Review each phase with `GEMINI_PROMPT_V3.md` §13 before starting the next.

## Reading order for Gemini

`GEMINI_PROMPT_V3.md` Part A → `DECISIONS.md` → plan §1–§6 → the phase's "Read first" list. `V1_TO_V3_MIGRATION.md` is required for P2 (C2, C4, C9), P3 (C8 rules extension), P4 (§6 legacy links), P7–P10 (compatibility pieces) and all of P12.

## Reference code in this branch

- `services/` (→ `reference/go-v2/` in P0): v2 Go backend with enterprise stages 1–5 and half of 6 implemented and tested (branch `v2-enterprise`, commits `a010458`, `72d7a28`, `5bc48b6` and earlier).
- `backend/`, `frontend/` (→ `reference/v1-django/`, `reference/v1-frontend/` in P0): the live v1 app.
- `packages/qr-render`, `apps/render` (→ `reference/ts-render/` in P0): the TypeScript renderer, used to generate golden fixtures for the Python renderer.
