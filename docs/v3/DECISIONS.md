# QRit v3 — Decisions Log

Decisions made **after** `QRit_v3_Plan.md` was written, and answers to its open questions. This file outranks the plan (see `GEMINI_PROMPT_V3.md` §2). Keep entries short. Never edit an accepted entry — add a new one that supersedes it.

Decisions D1–D19 are in `QRit_v3_Plan.md` §1.3; compatibility decisions C1–C12 are in `V1_TO_V3_MIGRATION.md` §3. New entries start at **D-20**.

## Entry format

```
## D-<n> <short title>
- Date: YYYY-MM-DD · Phase: P<n> · Author: Aryan | Gemini
- Status: proposed | accepted | superseded by D-<m>
- Context: why a decision was needed (one or two sentences)
- Decision: what was decided
- Alternatives rejected: …
- Consequences: what changes (files, tests, docs)
```

Gemini may add entries with status `proposed`; Aryan marks them `accepted` during the phase review.

---

## Open questions for Aryan (answer here before the phase that needs them)

| # | Question | Needed by | Answer |
|---|---|---|---|
| Q1 | Product domain (`APP_DOMAIN`, `API_DOMAIN`), `SHORT_DOMAIN`, `SANDBOX_SHORT_DOMAIN` | P4 | |
| Q2 | v1 hosts for `LEGACY_HOSTS`: v1 backend Railway domain, v1 `SHORT_LINK_BASE_URL`, custom domains attached to the v1 backend service, v1 frontend domain | P12 | |
| Q3 | Cloudflare account/zones; Workers Paid OK; Cloudflare for SaaS enabled on the short-domain zone | P4 | |
| Q4 | Railway plan is Pro; existing project and service names (`backend`, `frontend`?); v1 Postgres version and database name | P13 | |
| Q5 | Razorpay account (Subscriptions + Payment Links); keep Stripe for international; live v1 Stripe price ids → `STRIPE_LEGACY_PRICE_MAP` | P7 | |
| Q6 | GST (with CA): GSTIN, state code, SAC, whether INR prices include GST, LUT ARN, e-invoicing applicability | P7 | |
| Q7 | Resend account and sending domain | P2 | |
| Q8 | External users of the v1 API (`/api/v1/qr/api/generate`)? Sunset date for the v1 API | P10 | |
| Q9 | Plan mapping `starter→pro`, `pro→business` and grandfathering rules (`V1_TO_V3_MIGRATION.md` §5.3) — confirm | P12 | |
| Q10 | Email-duplicate strategy if `v1_inventory` finds case-insensitive duplicates | P12 | |
| Q11 | Default retention for migrated leads (365 days) and v1 database retention after cutover (30 days) | P12 | |
| Q12 | Cutover date and window (night IST), go/no-go approver | P13 | |
| Q13 | Did v1 production resolve scan geo (CDN headers or `GEOIP_LOOKUP_URL`)? If a lookup service was used, which one (for `LEGACY_GEOIP_LOOKUP_URL`) | P4 | |

## Items to verify on staging (record the result as a decision)

From `RAILWAY_DEPLOY.md`: IaC option names (`rootDirectory`, `project()` registration, Postgres image tag, `preserve()` for existing domains); whether Railway rollback restores start command / health path / region; whether the built-in PgBouncer accepts the `qrit_app` role; whether the Redis template allows `CONFIG SET`; point-in-time recovery availability on the plan.

---

## D-20 GitHub OAuth Integration & CompositePrimaryKey Parity
- Date: 2026-10-04 · Phase: P2a · Author: Aryan / Antigravity
- Status: proposed
- Context: Railway deployment required complete end-to-end OAuth support for both Google and GitHub, resolving Django 6.1 CompositePrimaryKey model field attribute mapping, and handling psycopg PL/pgSQL format specifiers in migrations.
- Decision:
  1. Implemented `POST /v1/auth/github` using `safe_client` code exchange against GitHub API, user profile & verified email fetch, `OAuthAccount` linking, and auto-provisioning of personal Organization and Workspace.
  2. Mapped `session.auth_method` for GitHub logins to `'google'` to satisfy the PostgreSQL constraint `sessions_auth_method_check` without altering the authoritative v2 schema.
  3. Resolved Django 6.1 `models.CompositePrimaryKey` declarations across models (`GroupMember`, `UserRecoveryCode`, `OrgMember`, `ApprovalDecision`, `QRCodePixel`, `OrgDataKey`) to reference Python model attribute names rather than underlying column names.
  4. Executed PL/pgSQL functions/triggers in data migrations via raw connection cursors to prevent psycopg from misinterpreting `%` format specifiers as missing query parameters.
- Alternatives rejected:
  1. Altering PostgreSQL check constraints on `sessions`: rejected to maintain strict schema parity with Go reference migrations.
  2. Using third-party OAuth libraries (`django-allauth`, `social-auth`): rejected per Plan §4 fixed stack rule.
- Consequences:
  1. All 13 tests in `apps/accounts/tests/` pass cleanly against PostgreSQL, and `tests/test_schema_parity.py` passes 100%.
  2. Frontend `SocialAuthButtons` in login, register, and `/callback/github` seamlessly orchestrate both Google ID token and GitHub authorization code exchanges.
