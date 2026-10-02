# QRit v1 → v3 Data Migration Specification

**Date:** 3 October 2026 · **Branch:** `v3` · **Owner:** Aryan
**Audience:** Gemini (implements `backend/apps/legacy/`) and Aryan (runs the import, approves cutover).
**Read with:** `QRit_v3_Plan.md` (§6.4 schema deltas, §7.23 legacy layer, §15 summary) and `RAILWAY_DEPLOY.md` (§6 cutover, §7 rollback).

This document is the contract for moving **every** piece of live v1 data into v3 and keeping every v1 link working. Where it says MUST, the behaviour is tested (§13). Field names on the left are v1 columns (`reference/v1-django/api/models.py`); on the right are v3 columns (the v2 schema in `reference/go-v2/db/migrations/00001…00007` plus the plan's §6.4 deltas).

---

## Contents

1. Principles
2. v1 inventory and what happens to each table
3. Compatibility decisions (C1–C12)
4. The `import_v1` command
5. Entity mappings (field level)
6. Serving v1 links after cutover
7. Printed v1 dynamic codes (`needs_reprint`)
8. Verification (`verify_v1_import`)
9. Customer communication
10. Cutover data procedure
11. Rollback (data side)
12. After the migration
13. Tests
14. Open items for Aryan

---

## 1. Principles

1. **v1 is read-only to the importer.** The importer connects to v1 with a role that can only `SELECT` (`V1_DATABASE_URL`). Nothing in v1 is ever modified by v3 code.
2. **Nothing a customer can see is lost silently.** Every v1 row is either migrated, transformed with a recorded reason, or skipped with a recorded reason. The import report (§4.7) lists every transformation and skip.
3. **Every v1 link keeps working.** `/r/<code>` on every host that served v1 links resolves to the same destination as in v1, for the same visitor, at the same time (§6, verified in §8).
4. **IDs are preserved** where v1 has a UUID for the same thing: users, workspaces, folders, campaigns, templates, QR codes, lead pages (→ forms), leads (→ form submissions), webhooks, API keys, scans (→ `scan_events.event_id`). New v3-only rows (organisations, versions, tags, domains, policies) get UUIDv7 ids recorded in `legacy_id_map`.
5. **Idempotent and resumable.** Running the import twice gives the same result. A crash part-way through is fixed by running it again. Each batch commits its rows and its `legacy_id_map` entries in the same transaction.
6. **No plaintext secrets survive.** v1 stores QR passwords, webhook secrets, user API keys and invite tokens in plaintext. v3 stores argon2 hashes (QR passwords), envelope-encrypted values (webhook secrets), SHA-256 hashes (API keys, invite tokens). v1 lead answers are encrypted with the organisation's data key.
7. **No raw IP addresses in v3.** v1 scans keep raw IPs and coordinates; v3 does not import them (§5.21).
8. **The importer writes through the services' low-level writers**, not through HTTP and not with ad-hoc SQL, so invariants (owner bindings, plan sync triggers, design hashes, RLS) hold. It does not emit integration events, webhooks or emails, and writes one audit row per organisation (`legacy.import_completed`), not one per entity.

---

## 2. v1 inventory and what happens to each table

| v1 table | Rows are… | v3 destination | Notes |
|---|---|---|---|
| `users` | migrated | `users`, `oauth_accounts` (never), organisation per user (§5.2) | Passwords kept as `bcrypt$…` (C2) |
| `workspaces` | migrated | `workspaces`, `workspace_policies`, `files` (logos) | Moves into the owner's organisation |
| `workspace_members` | migrated | `workspace_members`, `org_members`, `role_bindings` | `viewer` → `analyst` |
| `workspace_invites` | migrated if pending and unexpired | `invites` | Token hashed; old invite links keep working (C9) |
| `folders` | migrated | `folders` | Colour/icon/description dropped (no v3 columns) |
| `campaigns` | migrated | `campaigns` | Status and UTM mapped |
| `qr_templates` | migrated | `templates` | Design converted to DesignV1 (§5.12) |
| `qr_records` | migrated | `qr_codes`, `qr_versions`, `tags`, `qr_code_tags` | Core of the migration (§5.13–5.16) |
| `routing_rules` | migrated | rules JSON in `qr_versions.rules` | §5.15 |
| `qr_scans` | migrated | `scan_events` (partitioned), rollups rebuilt | No raw IP, no coordinates (§5.21) |
| `lead_capture_pages` | migrated | `forms` (+ `legacy_slug`, `presentation`) | §5.17 |
| `leads` | migrated | `form_submissions` (encrypted) | §5.18 |
| `webhooks` | migrated | `webhooks` (`signature_scheme='legacy_v1'`) | Secret encrypted (C6) |
| `webhook_logs` | **skipped** | — | Delivery history starts fresh |
| `workspace_api_keys` | migrated | `api_keys` (`legacy_kind='v1_workspace'`) | bcrypt kept until first use (C5) |
| `users.api_key` (`ak_…`) | migrated if used (§5.20) | `api_keys` (`legacy_kind='v1_user'`) | SHA-256 of the key |
| `custom_domains` + `workspaces.custom_domain` | migrated | `domains` (status `pending`) + `LEGACY_HOSTS` | Legacy `/r/` keeps working on them (§6) |
| `security_policies` | migrated | `workspace_policies`, `org_security_policies`, `alert_rules` | §5.6 |
| `sso_configs`, `workspaces.sso_*` | **skipped, reported** | — | v1 had no working SSO login; admins set it up again |
| `audit_logs` | migrated | `audit_logs` (then sealed) | `action = 'legacy.' + v1 action` |
| `bulk_jobs` | **skipped** | — | Job history not kept |
| `qr_files` | **skipped** | — | Unused in v1 |
| `free_tier_usages` | **skipped** | — | Daily counters for the public generator |
| `refresh_tokens`, `login_attempts`, `password_reset_tokens`, `email_verifications` | **skipped** | — | Everyone signs in again; outstanding reset links expire (announced, §9) |
| Stripe (via `users.stripe_*`) | migrated | `subscriptions` (organisation-level) | §5.22 |

**Pre-flight command** `python manage.py v1_inventory --source "$V1_DATABASE_URL" [--json out.json]` prints, without writing anything to v3:
- row counts per table and the date range of `qr_scans.scanned_at`;
- users whose emails collide case-insensitively (v1's unique index is case-sensitive; v3's `citext` is not) — **these block the import until resolved** (§5.1);
- codes per `qr_type`, static vs dynamic, dynamic codes whose destination is not `http(s)` (§5.13);
- routing rules per condition, rules with unknown conditions or unparseable values, malformed `geo_restrictions`;
- workspaces with a `custom_domain` or `custom_domains` rows and their status — needed for `LEGACY_HOSTS` (§6);
- the share of `qr_scans` with `country_code` set in the last 30 days, and the number of live codes whose rules or `geo_restrictions` depend on the country — if v1 production resolved geo (CDN headers or `GEOIP_LOOKUP_URL`), v3 must too for legacy traffic (§6);
- webhooks, and webhooks whose URL resolves to a private address;
- API keys used in the last 90 days (user `ak_` and workspace `qk_`);
- Stripe subscriptions by status and price id — needed for `STRIPE_LEGACY_PRICE_MAP` (§5.22);
- logos stored as `data:` URLs vs remote URLs;
- the largest workspaces by codes and scans (to size batches).

Aryan runs it against production (read-only) before P13 and answers §14 from its output.

---

## 3. Compatibility decisions

These are fixed. Record any change in `docs/v3/DECISIONS.md` first.

| # | Decision |
|---|---|
| **C1** Legacy links | Imported dynamic codes keep their 8-character, case-sensitive v1 code in `qr_codes.legacy_short_code` and get a **new** 7-character v3 `short_code` on the platform domain. `/r/<legacy_short_code>` keeps working on every v1 host (§6). The v3 short link is what new downloads encode. |
| **C2** Passwords | v1 bcrypt hashes are stored as `bcrypt$<v1 hash>` in `users.password_hash`. `PASSWORD_HASHERS` lists Argon2 first and `django.contrib.auth.hashers.BCryptPasswordHasher` second, so Django verifies the old hash and re-hashes with Argon2 on the first successful login. Pin `bcrypt` 4.x (5.x raises on passwords longer than 72 bytes, which v1 accepted and silently truncated). |
| **C3** Sessions | v1 access/refresh tokens are not honoured. Every user signs in again after cutover (announced). |
| **C4** Google accounts | v1 created Google sign-in users with a random password and linked by email only. For **imported** users (a `legacy_id_map` row with `entity='user'` exists and no `oauth_accounts` row yet), the first Google sign-in whose ID token has `email_verified=true` and the same email links the account automatically and creates the `oauth_accounts` row. Every other account follows the normal v3 linking rule (plan §7.2). |
| **C5** API keys | v1 user keys `ak_<32 hex>` (plaintext in v1) become v3 keys with `key_hash = sha256(full key)` immediately. v1 workspace keys `qk_<8 hex>.<secret>` are stored with `prefix = 'qk_<8 hex>'`, `legacy_bcrypt_hash = <v1 key_hash>` (bcrypt of the secret part only) and a random placeholder `key_hash` (32 random bytes, never matches); on first successful use v3 verifies bcrypt, writes `key_hash = sha256(full key)` and clears `legacy_bcrypt_hash`. Both kinds are accepted in `Authorization: Bearer …` and `X-API-Key: …`. The v1 compatibility endpoints (plan §7.23) keep the v1 request/response shapes for 6 months. |
| **C6** Webhooks | Migrated webhooks get `signature_scheme='legacy_v1'`: v3 signs them exactly like v1 (`X-QRit-Signature: sha256=<hex HMAC-SHA256(body, secret)>`, `X-QRit-Event: <event>`) and sends the **v1 payload shape** for `scan.created` (§5.19). The owner can switch to Standard Webhooks (`both`, then `standard`) in the dashboard. |
| **C7** Printed dynamic codes | v1 "dynamic" images encode the destination itself, not the short link. Imported dynamic codes get `needs_reprint = true` and `v1_printed_payload` = the v1 `content` that v1's image generation encoded. The UI and an email explain that printed copies go straight to that destination and can't be changed; downloading the new image fixes it (§7). |
| **C8** Rules semantics | v1 routing rules are translated to v3 rules that give the same result for the same request (§5.15). One small engine extension is needed: a split variant with `"fallthrough": true` (continue with the next rule) to express v1 weight rules exactly. Add it to the Python rules engine, the rules validator (at most one fallthrough variant per split; it has no `destination_url`; weights still sum to 100) and the editor ("remaining N% → next rule"). The v3 rule limit is 50 per version (the Go validator had 20; raised so imported v1 codes fit). |
| **C9** Invites | Pending, unexpired v1 invites are migrated with `token_hash = sha256(v1 token)`; v3's accept endpoint hashes whatever token string arrives, so `APP_URL/invite/<v1 token>` keeps working until it expires. |
| **C10** Timezone | v1 evaluated `time_of_day` and `date_range` in UTC. Imported workspaces get `timezone = 'UTC'`, so rules behave the same. The rule editor states that rules use the workspace timezone; changing it shifts imported time rules (warn in the UI when a workspace with time rules changes timezone). |
| **C11** Plans | Plans move from users to organisations. Mapping and grandfathered limits in §5.3. |
| **C12** Lead pages | v1 lead pages stay at `APP_URL/p/<slug>` (served by the v3 web app from `forms.legacy_slug`). Their consent text becomes a v3 consent notice that the owner is asked to review (§5.17). |

---

## 4. The `import_v1` command

### 4.1 Interface

```
python manage.py import_v1 --source "$V1_DATABASE_URL"
        [--dry-run]                       # read, map, validate; write nothing; produce the report
        [--since 2026-11-20T18:00:00Z]    # delta pass (§4.5)
        [--only users,orgs,workspaces,...] # subset of entities (dependencies must already be imported)
        [--legacy-hosts host1,host2]      # defaults to env LEGACY_HOSTS
        [--batch-size 1000] [--scan-batch-size 10000]
        [--report-dir s3://…|/tmp/…]      # default: storage prefix migration/v1/<run_id>/
        [--email-dup-strategy fail|keep-latest-login]   # default fail (§5.1)
        [--skip-logo-fetch]               # do not download remote logo URLs
        [--stripe-sync / --no-stripe-sync]  # read live Stripe subscriptions (default on in production)
        [--import-all-user-keys]          # also import v1 user keys not used in the last 90 days (§5.20)
```

Exit codes: `0` success, `2` validation errors in dry run, `3` blocking conflicts (e.g. email duplicates), `1` crash. The command takes a session-level advisory lock (`pg_advisory_lock(hashtext('import_v1'))`) on the v3 database so two runs can't overlap.

Supporting commands (same app): `v1_inventory` (§2), `verify_v1_import` (§8), `send_v1_migration_emails` (§9), `export_post_cutover_changes` (§11).

### 4.2 Code layout (`backend/apps/legacy/`)

```
v1/
  rows.py          frozen dataclasses for every v1 table (typed, exactly the v1 columns)
  reader.py        V1Reader: one psycopg (sync) connection, SET TRANSACTION READ ONLY, REPEATABLE READ snapshot,
                   server-side cursors, iter_<table>(after=None, since=None) generators ordered by `id`
                   (audit logs: by (COALESCE(created_at, '-infinity'), id) — the order they must be sealed in)
mapping/           PURE functions, no I/O, fully unit-tested
  users.py, plans.py, roles.py, slugs.py, content.py, design.py, rules.py, geo.py,
  scans.py, leads.py, webhooks.py, keys.py, audit.py, checksums.py
importers/         one module per entity; each exposes run(ctx, rows) and uses the services' low-level writers
  base.py          ImportContext (run id, maps, report, keyring, storage, dry_run), batching, legacy_id_map upserts
semantics/
  v1_redirect.py   v1 resolution logic as a pure function (port of reference/v1-django/api/views/redirect.py
                   + api/utils/routing.py) — used only by verify_v1_import
management/commands/
  v1_inventory.py, import_v1.py, verify_v1_import.py, send_v1_migration_emails.py, export_post_cutover_changes.py
templates/email/   migration emails (§9)
tests/
  fixtures/v1_seed.sql   v1 schema + seed rows covering every case in §13
```

The v1 reader uses raw SQL against the v1 schema (column names from `reference/v1-django/api/models.py`). It MUST NOT import v1 Django code.

### 4.3 Execution model

- **Snapshot:** the reader opens one `REPEATABLE READ, READ ONLY` transaction for the whole run (for very long runs on large data, one snapshot per entity is acceptable; the delta pass covers the gap).
- **Order** (each step needs the previous ones): users → organisations (+ org members, org policies, DEKs) → workspaces (+ personal workspaces, workspace policies, branding files) → workspace members + role bindings → invites → domains (legacy host rows + custom domains) → folders → tags → campaigns → templates → QR codes + versions + tag links → forms → form submissions → webhooks → API keys → subscriptions → audit logs → scans (+ partitions) → rollups and counters → finalise (audit summary rows, grandfathered limits recomputed).
- **Batches:** 1,000 rows per transaction (scans: 10,000 via `COPY`). Each batch writes its target rows and its `legacy_id_map` rows together and commits. The run id and the batch's last key (the v1 `id`; for audit logs the `(created_at, id)` pair — v1 `created_at` is nullable, so it is read as `COALESCE(created_at, '-infinity')`) are written to `legacy_import_runs` (a small table in the `legacy` app: `run_id, started_at, finished_at, mode, since, entity, last_key, counts jsonb, status`) so a re-run resumes after the last committed batch of the interrupted entity.
- **Idempotency:** every importer looks up `legacy_id_map (entity, v1_id)`; when found it compares the stored `checksum` with the checksum of the current v1 row and updates the v3 row only if it changed; when absent it inserts. Inserts that preserve a v1 id use `ON CONFLICT (id) DO NOTHING` + a map row, so a crash between the two can't duplicate data.
- **Checksum:** `sha256` hex of the canonical JSON (sorted keys, ISO-8601 UTC timestamps, UUIDs as strings) of all v1 columns of the row (for QR codes, plus its routing rules sorted by id).
- **RLS:** the importer connects as the app role and does **not** set `app.workspace_id`, so the tenant policies (`qrit_current_workspace() IS NULL OR …`) allow writes to every workspace. It never connects as a superuser.
- **Dry run:** the same code path inside a transaction that is rolled back at the end of each batch, plus the full report. A dry run against production data is the first gate in §10.

### 4.4 `legacy_id_map`

`legacy_id_map(entity text, v1_id uuid, v3_id text, imported_at timestamptz, checksum text, PRIMARY KEY (entity, v1_id))` — `v3_id` is text because some targets have non-UUID keys (audit log ids are `bigint`).

| entity | v1_id | v3_id |
|---|---|---|
| `user` | users.id | users.id (same) |
| `org_for_user` | users.id | organizations.id |
| `personal_workspace` | users.id | workspaces.id (created when needed, §5.4) |
| `workspace` | workspaces.id | same |
| `workspace_member` | workspace_members.id | `"<workspace_id>:<user_id>"` |
| `invite` | workspace_invites.id | invites.id (same) |
| `custom_domain` | custom_domains.id | domains.id |
| `folder`, `campaign`, `template`, `qr_code`, `form`, `form_submission`, `webhook`, `api_key` | v1 id | same id |
| `qr_version` | qr_records.id | qr_versions.id of the imported version |
| `user_api_key` | users.id | api_keys.id |
| `subscription` | users.id | subscriptions.id |
| `audit_log` | audit_logs.id | audit_logs.id (bigint as text) |
| `scan` | — | not mapped (scans keep their id as `event_id`; idempotency via the primary key) |

### 4.5 Delta pass (`--since`)

Used during the cutover window after a full import (§10). `--since` is the start time of the previous successful run minus 10 minutes (overlap is safe because everything is idempotent).

Many v1 writes don't touch `updated_at` (campaign assignment through `metadata`, codes detached when a folder or workspace is deleted, folder re-parenting, hard-deleted routing rules), so timestamps can't be trusted for change detection:

| v1 tables | How changes are found |
|---|---|
| every table **except** the three below (users, workspaces, members, invites, folders, campaigns, templates, `qr_records` + `routing_rules`, lead pages, webhooks, API keys, custom domains, security policies) | **re-read fully** and compare each row's checksum with `legacy_id_map.checksum`; a QR code's checksum includes its routing rules (sorted by id), so a changed, added or deleted rule re-imports the code. These tables are small enough (≤ a few hundred thousand rows) to re-read in minutes |
| `qr_scans` | `scanned_at > since`, inserted through the staging-table path (§5.21) so re-reads are harmless; rollups rebuilt for touched days |
| `leads`, `audit_logs` | `created_at > since` (append-only in v1; rows with NULL `created_at` were all imported by the full run) |

**Deletions:** for every entity except scans and audit logs, ids present in `legacy_id_map` but missing in v1 are deleted in v1 → apply in v3: QR codes soft-deleted (`deleted_at`, short code kept), members/invites/keys/webhooks/domains/folders/tags removed, forms deactivated, users soft-deleted. Each one goes in the report.

**Changed dynamic codes:** if the v3 code still has exactly one version and it was created by the importer (`change_note = 'Imported from v1'`), the importer replaces that version's `destination_url` and `rules` in place (an import-only writer; the code is not live in v3 yet). Otherwise it creates a new version through the normal version writer.

### 4.6 Performance targets

On staging-sized hardware (Railway 2 vCPU / 2 GB for the migrator): 100,000 codes ≤ 10 min; 1,000,000 scans ≤ 10 min (`COPY` in 10,000-row batches, partitions created first, indexes kept); the whole production import SHOULD finish in under 1 hour, and the delta pass in under 5 minutes, so the maintenance window stays within ≈ 45 minutes. If the inventory shows more than 20 M scans, import scans older than the cutover month **before** the window (they don't change) and only the current month in the delta.

### 4.7 Report

Written as `report.json` + `skips.csv` + `transforms.csv` under the report directory (object storage by default; the files may contain workspace names and code titles but never passwords, secrets, lead answers or IPs).

```json
{
  "run_id": "…", "mode": "full|delta|dry_run", "since": null,
  "started_at": "…", "finished_at": "…",
  "counts": {"users": {"read": 0, "inserted": 0, "updated": 0, "unchanged": 0, "skipped": 0}, "…": {}},
  "transforms": {"frame_dropped": 0, "viewer_to_analyst": 0, "weight_rule_split": 0, "…": 0},
  "skips": {"dynamic_non_http_destination": 0, "orphan_code": 0, "…": 0},
  "blocking": [{"code": "email_case_duplicate", "v1_ids": ["…", "…"]}]
}
```

Each CSV row: `entity, v1_id, code, detail`. Reason codes are listed with each mapping in §5 and collected in `apps/legacy/mapping/reasons.py`.

---

## 5. Entity mappings

Conventions: "—" = not carried over. `NULL` timestamps in v1 fall back to the row's `created_at`, then to the import time (`transform: timestamp_defaulted`). Strings are trimmed; empty strings become `NULL` where the v3 column is nullable.

### 5.1 Users → `users`

| v1 | v3 | Rule |
|---|---|---|
| `id` | `id` | same |
| `email` | `email` (citext) | lower-cased for comparison. Case-insensitive duplicates are **blocking** (`email_case_duplicate`). With `--email-dup-strategy keep-latest-login` the account with the latest `last_login_at` (then `updated_at`) keeps the email; the others are imported soft-deleted (`deleted_at = import time`) with email `v1dup-<v1 id>@users.invalid`, their workspaces still owned by them, and listed in the report so Aryan can contact them |
| `password_hash` | `password_hash` | `"bcrypt$" + v1 hash` when it starts with `$2a$`, `$2b$` or `$2y$`; otherwise `NULL` (`password_unusable`) |
| `name` | `name` | `''` when NULL |
| `avatar_url` | `avatar_url` | only `https://` URLs; else NULL |
| `email_verified`, `email_verified_at` | `email_verified_at` | `email_verified_at` or `created_at` when `email_verified` is true; else NULL |
| `last_login_at` | `last_login_at` | same |
| `created_at`, `updated_at` | same | |
| `company` | `organizations.name` (§5.2) | |
| `plan`, `plan_expires_at`, `subscription_status`, `subscription_ends_at`, `stripe_*` | organisation plan + `subscriptions` (§5.3, §5.22) | |
| `api_key`, `api_calls_today`, `api_calls_reset_at` | `api_keys` (§5.20) | |
| `default_workspace_id` | — | v3 lists workspaces from `/me` |
| `failed_login_attempts`, `locked_until`, `last_login_ip`, `password_changed_at` | — | |
| — | `locale='en'`, `timezone='UTC'`, `is_staff=false` | Aryan's own account is made staff manually after cutover |

### 5.2 Organisation per user → `organizations`, `org_members`, `org_security_policies`, `org_data_keys`

Every v1 user gets exactly one organisation (`legacy_id_map entity='org_for_user'`). All workspaces the user owns in v1 move into it.

| v3 column | Value |
|---|---|
| `id` | new UUIDv7 |
| `name` | `users.company` if set, else `users.name`, else the email local part |
| `slug` | slug of the first owned workspace's slug, else of the email local part; normalised to `^[a-z0-9](-?[a-z0-9])*$`, 3–48 chars; on conflict append `-2`, `-3`, … (`slug_changed`) |
| `kind` | `standard` if any owned workspace has members other than the owner, else `personal` |
| `plan_id` | §5.3 |
| `grandfathered_limits` | §5.3 |
| `billing_email` | user email |
| `tax_country` | `IN` (default; the owner updates the billing profile) |
| `data_region` | `in` |
| `created_at` | user `created_at` |

`org_members`: the user as `org_owner` (`source='creator'`); every other user who is a member of any workspace in this organisation as `member` (`source='invite'`). `org_security_policies`: one row with defaults, then adjusted from v1 security policies (§5.6). `org_data_keys`: one DEK created through `core.crypto.Keyring.ensure(org_id)` before any encrypted data is written.

### 5.3 Plans and grandfathered limits

The organisation's plan comes from the owner's v1 `users.plan`. A paid v1 plan whose `plan_expires_at` is in the past **and** whose `subscription_status` is not `active`/`trialing`/`past_due` counts as `free` (`plan_expired_downgraded`).

| v1 plan | v3 `organizations.plan_id` | Why |
|---|---|---|
| `free` | `free` | |
| `starter` | `pro` | closest v3 tier |
| `pro` | `business` | v1 pro included routing, webhooks, audit log, GS1, custom domain, templates — all business features in v3 |
| `enterprise` | `enterprise` | no contract is created; staff create one later if needed |

`grandfathered_limits` (merged by the entitlements service like a contract override, taking the **maximum** of plan default and grandfathered value) keeps what each customer already has, so nothing becomes read-only or blocked at cutover:

| Key (v3 entitlement names) | Grandfathered value |
|---|---|
| `dynamic_codes` | `max(plan default, number of imported live dynamic codes in the org)`; for v1 `starter`/`pro`/`enterprise` also at least the v1 `max_qr_codes` (500 / 10,000 / 1,000,000) |
| `seats` | `max(plan default, distinct active members across the org's workspaces)`; for paid v1 plans at least the v1 `max_members` (3 / 25 / 1,000) |
| `owned_workspaces` | `max(plan default, number of imported workspaces)`; paid v1 plans at least v1 `max_workspaces` (2 / 10 / 100) |
| `custom_domains` | `max(plan default, number of imported custom domains)` |
| `templates` | `max(plan default, number of imported templates)` |
| `webhooks` | `max(plan default, number of imported webhooks)` |
| `analytics_history_days` | `max(plan default, v1 analytics_retention_days)` (30 / 90 / 365 / 1,095) |
| `api_requests_per_min` | when the org has a v1 API key used in the last 90 days and the v3 plan has 0: `10` |
| `features` (list of v3 feature keys) | each key below **when the organisation's imported data uses it and its v3 plan lacks it**: `api` (a v1 key used in the last 90 days), `webhooks` (imported webhooks), `rules` (codes with routing rules or geo restrictions), `campaigns` (imported campaigns or codes linked to one — v1 starter had campaigns, v3 pro doesn't), `scheduling` (codes with `starts_at`), `expiry` (codes with `expires_at`), `scan_limit` (codes with `scan_limit`), `locked_templates` (locked templates), `remove_branding` (v1 `remove_branding = true`). Custom domains are a **limit**, not a feature (row above) |

Grandfathered limits stay while the organisation keeps its imported subscription or stays on its plan. They are **cleared** (audited `billing.grandfathering_ended`) when the organisation changes plan through v3 checkout; the checkout page shows "Your current allowances from QRit v1 end when you change plan" before confirming.

### 5.4 Workspaces → `workspaces`, `workspace_policies`, branding files

| v1 | v3 | Rule |
|---|---|---|
| `id` | `id` | same |
| `name` | `name` | |
| `slug` | `slug` | normalised like org slugs; conflicts get `-2`… (`slug_changed`) |
| `owner_id` | `owner_id`, `org_id` = owner's organisation | |
| `plan` | `plan_id` | the organisation's plan (the v3 trigger keeps it in sync); v1 `workspaces.plan` is ignored |
| `description` | `settings.description` | |
| `brand_color` | `brand.primary_color` | `#RRGGBB` (expand `#RGB`, else default) |
| `brand_logo` or `logo_url` | `brand.logo_file_id` | first non-empty; `data:` URLs decoded, remote `https` URLs fetched with the SSRF-safe client (5 s, ≤ 2 MB); PNG/JPEG/WebP only, re-encoded to PNG ≤ 1024 px; stored as `files(purpose='logo')`. Failure → `logo_fetch_failed`, no logo |
| `favicon_url` | `brand.favicon_file_id` | same processing |
| `custom_footer` | `brand.footer_text` | plain text, ≤ 200 chars, HTML stripped |
| `remove_branding` | `brand.hide_branding` | hides "Powered by QRit" on hosted, password and error pages when the plan has `remove_branding` (plan §7.9 Pages); grandfathered as a feature if used (§5.3) |
| `custom_css` | — | **dropped** (`custom_css_dropped`) — v3 does not accept customer CSS |
| `custom_domain` | §5.10 | |
| `max_members`, `max_qr_codes`, `max_folders` | — | replaced by plan entitlements |
| `sso_enabled`, `sso_provider`, `sso_config` | — | `sso_not_migrated` report row when enabled |
| — | `timezone = 'UTC'` (C10), `is_sandbox = false` | |

**Personal workspace:** if a user owns no v1 workspace, or has v1 codes/lead pages with `workspace_id IS NULL`, the importer creates a workspace "Personal" (slug from the org slug + `-personal` if needed) in their organisation and records it as `personal_workspace`. User-scoped v1 codes go there.

### 5.5 Members → `workspace_members`, `org_members`, `role_bindings`

| v1 role | v3 primary role |
|---|---|
| (the `workspaces.owner_id` user) | `owner` — exactly one owner row per workspace |
| `owner` (member row that isn't the real owner) | `admin` (v1 already treated it as admin) |
| `admin` | `admin` |
| `editor` | `editor` |
| `viewer` | `analyst` (`viewer_to_analyst`) |
| anything else | `analyst` (`role_unknown`) |

`created_at = joined_at`. Every member is mirrored into `role_bindings` with `orgs.services.bind_member_role` and added to `org_members` (§5.2). Members pointing to a missing user are skipped (`member_user_missing`).

### 5.6 Security policies → `workspace_policies`, `org_security_policies`, `alert_rules`

| v1 `security_policies` | v3 | Rule |
|---|---|---|
| `allowed_domains` (CSV) | `workspace_policies.allowed_destination_hosts` | each host `h` becomes `h` **and** `*.h` (v1 matched the host and all subdomains); lower-cased, deduplicated |
| `blocked_domains` | `blocked_destination_hosts` | same expansion |
| `require_https` | `require_https` | v1 value. **Workspaces without a v1 policy row get `false`** (v1 enforced nothing for them; v3 defaults to `true`) |
| `require_approval` | `approval_mode` | `true` → `all_destination_changes` when the org plan has `approvals`; otherwise `off` + `approval_not_enabled` report row |
| `scan_alert_threshold` > 0 | `alert_rules(name='Scan threshold (v1)', kind='scan_threshold', target_type='workspace', target_id=NULL, params={"threshold": N}, channels={"emails": [<owner and admin emails>], "integration_ids": []})` | only when the plan has `alerts`; otherwise report `alert_not_enabled` |
| `password_min_length` | `org_security_policies.password_min_length` | `max(10, highest value across the org's workspaces)` |
| `session_timeout_minutes` > 0 | `org_security_policies.session_idle_minutes` | lowest non-zero value across the org's workspaces, clamped to 5–10,080 |

Workspaces without a v1 policy get a `workspace_policies` row with defaults except `require_https = false`.

### 5.7 Invites → `invites`

Only rows with `status='pending'` and `expires_at > now()`. `token_hash = sha256(v1 token as UTF-8)`; `role` mapped like members (`viewer` → `analyst`, `owner` → `admin`); `invited_by` = v1 inviter if imported, else the workspace owner; `email` lower-cased; duplicates per `(workspace, email)` keep the newest.

### 5.8 Folders → `folders`

`parent_id` kept when the parent is imported in the same workspace, else `NULL` (`folder_parent_missing`); cycles broken at the first repeated node (`folder_cycle_broken`); depth > 8 → re-parented to the depth-8 ancestor (`folder_flattened`). Names must be unique per `(workspace, parent)` → append ` (2)`, ` (3)`… (`folder_renamed`). `sort_order` → `position`. `color`, `icon`, `description`, `qr_count`, `scan_count` dropped (`folder_cosmetics_dropped`, counted once per workspace).

### 5.9 Tags → `tags`, `qr_code_tags`

v1 tags are CSV strings on `qr_records.tags`. Split on commas, trim, drop empties, ≤ 50 chars; one `tags` row per distinct case-insensitive name per workspace (first spelling wins, default colour), and `qr_code_tags` links. `campaigns.tags` are dropped (`campaign_tags_dropped`).

### 5.10 Domains → `domains`

1. **Legacy host rows:** for every host in `--legacy-hosts` (the v1 backend's Railway domain, the host of v1 `SHORT_LINK_BASE_URL`, and any domain attached to the v1 backend service) **that is not a customer domain from step 2**, create a platform domain row: `workspace_id NULL`, `hostname = host`, `status='active'`, `tls_status='active'`, `verification_token='legacy-v1'`. These rows give imported and live legacy scans a `domain_id`.
2. **Customer domains:** each v1 `custom_domains` row, and each `workspaces.custom_domain` not already listed, becomes a workspace domain row: `hostname` lower-cased, `status='pending'`, `tls_status='pending'`, new `verification_token`, `verified_at = NULL`. It does **not** serve v3 short links until the customer points DNS at `CF_SAAS_CNAME_TARGET` and Cloudflare verifies it (plan §7.7). Until then its **legacy** links keep working because the hostname is in `LEGACY_HOSTS` and stays attached to the reused v1 backend service (§6); legacy scans on that host — imported and live — use **this workspace row** as `domain_id`, even while it is `pending` (`domains.hostname` is unique, so a host never gets both a platform row and a workspace row). `is_primary` → the workspace's `default_domain_id` once active. Report `custom_domain_needs_dns` per domain.

### 5.11 Campaigns → `campaigns`

| v1 | v3 |
|---|---|
| `id`, `workspace_id`, `name`, `starts_at`, `ends_at`, `created_by`, timestamps | same |
| `status` | `draft`→`draft`, `scheduled`→`active` (dates kept), `active`→`active`, `paused`→`paused`, `completed`→`ended`, `archived`→`archived` |
| `scan_goal` | `goal_scans` (`0` → `NULL`) |
| `utm_source/medium/campaign/term/content` | `utm` = `{source, medium, campaign, term, content}` (only non-empty keys) |
| `description`, `color`, `tags` | dropped (`campaign_cosmetics_dropped`) |

### 5.12 Designs: v1 `customization` / template `design` → DesignV1

v1 stored two JSON shapes, both produced by the v1 frontend for the client-side `qr-code-styling` preview (the v1 server renderer ignored customisation and drew plain black-on-white). `mapping/design.py::from_v1(obj) -> (DesignV1, list[reason])`:

**Shape A** (dashboard "create", has `foreground_color` or `body_style`):

| v1 key | DesignV1 |
|---|---|
| `foreground_color` | `modules.color`, `finder.outer_color`, `finder.inner_color` |
| `background_color` | `"transparent"` → `background.transparent=true`, `background.color="#FFFFFF"`; else `background.color` |
| `body_style` | `modules.shape` (see shape table) |
| `corner_style` | `finder.outer_shape` (see shape table); `finder.inner_shape = square` |
| `gradient {type, start_color, end_color, rotation}` | `modules.gradient {type, rotation (0..359), stops: [{offset 0, start_color}, {offset 1, end_color}]}` |
| `logo {url, size}` | `logo.file_id` (stored like workspace logos), `logo.size_ratio = size` clamped to 0.10–0.30 (`logo_size_clamped` when changed), `clear_modules = true`, `shape = square` |
| `frame {style}` | **dropped** (`frame_dropped`) — v1 never drew frames into images, so adding one would change the look of re-downloads |

**Shape B** (home-page generator, has `qrColor` or `selectedDotStyle`):

| v1 key | DesignV1 |
|---|---|
| `qrColor` | `modules.color`, finder colours |
| `bgColor`, `transparentBg` | `background` |
| `selectedDotStyle` | `modules.shape` |
| `selectedCornerStyle` | `finder.outer_shape` |
| `eyeInnerStyle` | `finder.inner_shape` (`dot` → `dot`, `square` → `square`, `rounded` → `rounded`, else `square`) |
| `useGradient`, `gradientType`, `gradientStart`, `gradientEnd`, `gradientRotation` | `modules.gradient` (only when `useGradient`) |
| `quietZone` | `quiet_zone` (clamp 0–10) |
| `errorLevel` | `ecc` (`L/M/Q/H`, else `auto`) |
| `logoFile`, `logoSize`, `logoMargin` | `logo.file_id`; `logo.size_ratio` = `logoSize/100` when `logoSize > 1` (percent, e.g. the default 40) else `logoSize` (the slider stores a ratio 0.1–0.4), clamped to 0.10–0.30; `logo.padding = min(4, round(logoMargin/2))` |
| `selectedFrame` | dropped (`frame_dropped`) |
| `bgImage` | dropped (`bg_image_dropped`) |
| `designTab` | ignored |

**Shape tables** (qr-code-styling names → DesignV1):

| v1 module (`body_style` / `selectedDotStyle`) | DesignV1 `modules.shape` |
|---|---|
| `square`, `dots`, `rounded`, `extra-rounded`, `classy`, `classy-rounded` | same name |
| `diamond`, anything else | `square` (`shape_unsupported`) |

| v1 corner (`corner_style` / `selectedCornerStyle`) | DesignV1 `finder.outer_shape` |
|---|---|
| `square`, `none`, `shield` | `square` |
| `dot` | `circle` |
| `extra-rounded`, `rounded-sm`, `classy` | `rounded` |
| `leaf` | `leaf` |

Colours: `#RGB` → `#RRGGBB`, lower-cased; anything else (names, `rgb()`) → the DesignV1 default (`color_invalid`). Empty or unrecognised objects → the default design. A logo present → `ecc = auto` resolves to `H`. The result is canonicalised and hashed (`design_hash`). After conversion the importer runs `render.scannability.check()`; designs with an **error** (e.g. contrast < 2) are kept but reported (`design_scannability_error`) so the owner sees the warning in the editor.

Templates: `qr_templates.design` goes through the same function. `is_locked`, `is_default` kept (only the newest default per workspace stays default — `template_default_deduplicated`). `description`, `preview_url`, `usage_count` dropped.

### 5.13 QR codes → `qr_codes` (common fields)

| v1 | v3 | Rule |
|---|---|---|
| `id` | `id` | same |
| `workspace_id` | `workspace_id` | imported workspace; `NULL` or missing → the owner's personal workspace (`code_moved_to_personal`) |
| `user_id` | `created_by` | `NULL` with a workspace → workspace owner; `NULL` without a workspace → skip (`orphan_code`) |
| `title` | `name` | `NULL` → first 60 chars of the destination/content |
| `folder_id` | `folder_id` | only if the folder is in the same workspace |
| `tags` | `qr_code_tags` | §5.9 |
| `metadata.campaign_id` | `campaign_id` | only if in the same workspace |
| `metadata.template_id` | `template_id` | only if in the same workspace |
| `customization` | `design`, `design_hash` | §5.12 |
| `is_active` | `status` | `true` → `active`; `false` → `paused` |
| `scan_count` | `total_scans` | v1 counter (bots included, as v1 counted them) |
| — | `unique_scans`, `last_scanned_at` | computed from the imported scans (§5.21) |
| `created_at`, `updated_at` | same | |
| `size`, `qr_type_id` | — | download size is chosen at download time |
| `metadata` (everything else) | `static_content.legacy_v1.metadata` (static) or dropped (dynamic) | kept for reference only |
| — | `safety_status = 'pending'` | the safety worker checks destinations gradually after cutover; `pending` is served |

**Which v3 mode and type** (`mapping/content.py::classify(v1) -> (mode, content_type, payload|destination, reasons)`):

1. v1 `is_dynamic = false` → **static**, `static_payload = content` exactly (byte-for-byte what v1 encoded), `content_type` from the payload shape (step 3). v1 `password`, `max_scans`, `expires_at`, `scheduled_at`, `geo_restrictions` on a static code had no effect in v1 → dropped (`static_lifecycle_dropped`).
2. v1 `is_dynamic = true`: destination = `redirect_url` if non-empty, else `content`.
   - destination starts with `http://` or `https://` → **dynamic**, §5.14.
   - anything else (`mailto:`, `tel:`, vCard text, …) → **static** with `static_payload = content` (`dynamic_non_http_destination`). v1 could never redirect these (Django's redirect response only allows http/https/ftp), and the printed image encodes `content`, so a static code is the honest equivalent. `legacy_short_code` is not set.
3. `content_type` from the payload shape (first match): `^https?://wa\.me/` → `whatsapp`; `^https?://` → `url`; `^mailto:` → `email`; `^tel:` → `phone`; `^(SMSTO|sms):` (case-insensitive) → `sms`; `^WIFI:` → `wifi`; `^BEGIN:VCARD` → `vcard`; `^BEGIN:(VEVENT|VCALENDAR)` → `event`; `^upi://` → `upi`; `^geo:` → `location`; else `text`. For dynamic codes the type is `url` or `whatsapp`. v1 types without a v3 equivalent (`bitcoin`, `mecard`, `multilink`, `social`, `facebook`, `instagram`, `apps`, `pdf`, `images`, `video`, `mp3`, `menu`, `coupon`, `business`, `links`, `gs1`) land on whatever their payload is (usually `url` or `text`); the v1 type name is kept in `static_content.legacy_v1.qr_type` (static) and in the report (`type_mapped`). v1 `gs1` codes become `url` codes; GS1 resolver setup is a manual step for the owner (`gs1_manual_setup`).
4. Static payload validation: the v3 per-type validators are **not** applied to imported static payloads (they would reject strings v1 accepted). Only the renderer's capacity check runs; a payload that doesn't fit a version-40 QR at ECC L is impossible (v1 rendered it) and would be a bug.

### 5.14 Dynamic codes → `qr_codes` + `qr_versions` (version 1)

| v3 | Value |
|---|---|
| `mode` | `dynamic` |
| `domain_id` | the platform domain (`SHORT_DOMAIN`) |
| `short_code` | new 7-char Crockford code (normal generator, unique per domain) |
| `legacy_short_code` | v1 `short_code` exactly (case-sensitive). v1 codes are 8 chars of `[A-Za-z0-9_-]`; anything else → kept as is + `legacy_code_unusual` |
| `legacy_host` | the workspace's v1 `custom_domain` if set, else the host of v1 `SHORT_LINK_BASE_URL` (what v1 displayed) |
| `v1_printed_payload` | v1 `content` — what v1's server-side image generation encoded (`views/qr.py`, `views/ws_qr.py`); when `redirect_url` differs from `content` (edited later), report `printed_payload_differs` so the banner text is right |
| `needs_reprint` | `true` (C7) |
| `password_hash` | argon2id of the v1 plaintext `password` (empty → `NULL`) |
| `starts_at` | v1 `scheduled_at` |
| `expires_at` | v1 `expires_at` |
| `scan_limit` | v1 `max_scans` (`0`/negative → `NULL`) |
| `fallback_url` | `NULL` (v1 had none) |
| `current_version_id` | the version below |

Version 1 (`qr_versions`): `version_no = 1`, `destination_kind = 'url'`, `destination_url` = destination, `rules` = §5.15, `utm = {}` (v1 had already baked campaign UTM into the destination), `effective_at = created_at`, `safety_status = 'pending'`, `approval_status = 'not_required'`, `created_by` = v1 `user_id`, `change_note = 'Imported from v1'`. URL-safety validation is **not** applied to version 1 (v1 served these destinations; blocking them at import would break live links); the safety worker reviews them after cutover like any `pending` version.

### 5.15 Routing rules and geo restrictions → `qr_versions.rules`

v3 rule shape (plan §7.7): `{id, name, enabled, when: {all: [{field, op, value}]}, destination_url, split?, block?}`. The first matching enabled rule wins, in array order.

**Order:** (1) the geo-restriction rule, if any (v1 checked geo before rules); (2) v1 rules sorted by `priority DESC, created_at ASC` (v1's `ordering`). Inactive v1 rules are imported with `enabled = false`. More than 50 rules → the first 50 are kept (`rule_limit_exceeded`, the rest listed in the report).

**Geo restrictions** (`geo_restrictions` text):

| v1 value | v3 rule |
|---|---|
| `allow:US,CA` (v1 treated **any** mode other than `block`, case-insensitive, as an allow-list) | `{id: "v1-geo", name: "Allowed countries (v1)", enabled: true, when: {all: [{field: "country", op: "not_in", value: ["US","CA",""]}]}, block: true}` — the empty string keeps v1's "unknown country is allowed" behaviour |
| `block:RU,CN` | `{id: "v1-geo", name: "Blocked countries (v1)", enabled: true, when: {all: [{field: "country", op: "in", value: ["RU","CN"]}]}, block: true}` |
| empty, no `:`, or no codes | no rule (`geo_restriction_ignored` when non-empty) |

**Rules** — one v3 rule per v1 rule: `id = str(v1 rule id)`, `name = v1 name or describe_condition(v1)` (port of v1 `describe_condition`), `destination_url = v1 destination_url`, `when.all = [condition]`:

| v1 `condition` / `operator` / `value` | v3 condition |
|---|---|
| `country`, `in`/`not_in`, `"us, gb"` | `{field: "country", op, value: ["US","GB"]}`; for `not_in` append `""` (v1 never matched a scan with unknown country) |
| `device`, `in`/`not_in`, `"mobile,tablet"` | `{field: "device_type", op, value: [...]}`; v1 value `bot` is removed (`rule_value_dropped`) — v3 marks bots separately; if no values remain → rule `enabled=false` |
| `os`, `in`/`not_in` | `{field: "os", op, value: [...]}` with each v1 value (a ua-parser family name) translated to the v3 vocabulary of the ported Go classifier: `iOS`→`iOS`, `Android`→`Android`, `Mac OS X`/`Mac OS`/`macOS`→`macOS`, `Windows` (any `Windows …`)→`Windows`, `Linux`/`Ubuntu`/`Fedora`/`Debian`→`Linux`, anything else→`other` (`rule_value_mapped`; duplicates removed). No `""` is appended (ua-parser always returned a family, `Other` at worst) |
| `language`, `in`/`not_in`, `"en,hi"` | `{field: "language", op, value: ["en","hi"]}` (both v1 and v3 compare the primary subtag, lower-case) |
| `time_of_day`, `in`, `"09:00-17:30"` | `{field: "local_time", op: "between", value: ["09:00","17:30"]}` (UTC per C10; wrap past midnight supported by both). v1 accepted `"9"` or `"9:5"` (minutes optional, no padding) — normalise to zero-padded `HH:MM` |
| `time_of_day`, `not_in`, `"09:00-17:30"` | the **complement window** `["17:31","08:59"]` (v3 `local_time` has no negation); a full-day window → rule `enabled=false` (`rule_never_matches`) |
| `date_range`, any operator, `"2026-01-01..2026-03-31"` (either side may be blank) | `{field: "date", op: "between", value: ["2026-01-01","2026-03-31"]}` (`""` for an open side; dates re-formatted zero-padded `YYYY-MM-DD` because v3 compares strings while v1 parsed `2026-1-5`); unparseable dates → `enabled=false` (`rule_value_invalid`) — v1 never matched those either |
| `scan_count`, `lt`, `"100"` | `{field: "scan_count", op: "lt", value: 100}` |
| `scan_count`, any other operator | `{field: "scan_count", op: "gte", value: N}` (v1's default was `>=`) |
| `weight`, `"30"` | no condition; `split: [{variant: "v1", weight: 30, destination_url: <rule destination>}, {variant: "rest", weight: 70, fallthrough: true}]` (C8); `≤ 0` → `enabled=false`; `≥ 100` → plain rule without split |
| unknown condition or unparseable number | `enabled=false` (`rule_unknown_condition` / `rule_value_invalid`) — v1 never matched these |

**Weight buckets.** v1 bucketed visitors with `int(sha256(f"{qr_id}:{ip}:{ua}").hexdigest()[:8], 16) % 100`; v3 uses the ported Go formula (big-endian uint32 of the first 4 bytes of `sha256(qr_id ‖ ip ‖ ua)` mod 100). The split **ratio** is unchanged, but an individual visitor may land in a different variant after cutover. This is accepted and mentioned in the email to owners of codes with weight rules.

**Known, accepted differences** (listed in the verification as expected):
- empty `Accept-Language`: v1 language `""`, v3 `"en"`;
- device and OS classification of rare user agents may differ slightly (v3 ports the Go classifier; v1 used ua-parser);
- v1 compared times to the second, v3 to the minute (`17:30` includes the whole minute 17:30 in v3).

### 5.16 Tags, folders and campaign links on codes

Applied in §5.13. A code whose folder or campaign wasn't imported (missing or other workspace) keeps `NULL` and gets `code_link_dropped`.

### 5.17 Lead capture pages → `forms`

| v1 | v3 | Rule |
|---|---|---|
| `id`, `workspace_id` (or personal workspace), `name`, `is_active`, timestamps | same | |
| `slug` | `legacy_slug` | unique |
| `form_fields` (JSON text `[{name, type, label, required, placeholder, options}]`) | `fields` `[{key, type, label: {en}, required, placeholder: {en}, options}]` | `type`: `text`, `email`, `textarea`, `select`, `date`, `checkbox` same; `tel` → `phone`; `number`, `url` → `text`; anything else → `text` (`field_type_mapped`). `key` = `name` normalised to `[a-z0-9_]{1,40}` (deduplicated). ≤ 25 fields (extra dropped, `fields_truncated`). Invalid JSON → one `email` field (`fields_invalid`) |
| `headline`, `subheadline`, `button_text`, `button_color`, `background_color`, `text_color`, `thank_you_message`, `redirect_url` | `presentation.{same keys}` | colours normalised; `redirect_url` only `http(s)` |
| `hero_image` | `presentation.hero_file_id` | processed like logos |
| `consent_text`, `requires_opt_in`, `privacy_policy` | `notice` | `{"en": {"intro": consent_text or "We use these details to respond to your request.", "purposes": [{"id": "contact", "label": "Respond to my request", "required": true}] + ([{"id": "marketing", "label": consent_text or "Send me updates", "required": false}] if requires_opt_in), "privacy_policy_url": privacy_policy if it is an https URL, "privacy_policy_text": privacy_policy otherwise}, "controller": <org name>, "grievance_contact": <owner email>, "retention": "365 days", "withdrawal": "Use the link in your confirmation email"}`; `notice_version = 1` |
| — | `retention_days = 365`, `double_opt_in = false`, `notify_emails = {}` | |
| — | `presentation.notice_needs_review = true` | the dashboard asks the owner to review the generated notice; cleared on save |
| `qr_record_id`, `views`, `submissions` | — | counts come from submissions; the linked code keeps pointing at the `/p/<slug>` URL it already encodes |

### 5.18 Leads → `form_submissions`

| v1 | v3 | Rule |
|---|---|---|
| `id`, `page_id` → `form_id`, `created_at` | same | |
| — | `workspace_id` | the form's workspace |
| — | `qr_code_id` | the page's `qr_record_id` if imported |
| `data` (JSON text of the submitted body) | `data_ct`, `key_id` | parse (invalid JSON → `{"raw": data}`), drop `opt_in`, add `_source = source` if set, encrypt with the org DEK (`Keyring.encrypt`) |
| `email` (or `data.email`) | `email_bidx` | `Keyring.blind_index(org, lower(email))` |
| `opted_in` | `consent` | `{"notice_version": 1, "purposes_accepted": ["contact"] + (["marketing"] if opted_in), "lang": "en", "at": created_at, "source": "v1_import"}` |
| — | `status = 'confirmed'`, `confirmed_at = created_at` | |
| — | `delete_after` | `max(created_at + 365 days, cutover date + 90 days)` so no old lead is erased in the first 90 days |
| `ip_address`, `user_agent` | — | not migrated (data minimisation) |

### 5.19 Webhooks → `webhooks`

| v1 | v3 | Rule |
|---|---|---|
| `id`, `workspace_id`, `url`, `created_at`, `updated_at` | same | `url` checked with the SSRF guard; private/loopback/link-local targets → `is_active=false`, `disabled_reason='private_address'` (`webhook_private_url`) |
| `secret` | `secret_ciphertext` | `Keyring.encrypt(org, secret or "")` — an empty v1 secret stays empty so receivers' verification keeps working (`webhook_secret_empty`; the UI asks the owner to rotate) |
| `events` (CSV) | `events` (`text[]`) | keep `scan.created`, `qr.created`, `qr.updated`, `qr.deleted`; drop others (`webhook_event_dropped`) |
| `is_active` | `is_active` | |
| `fail_count` | `consecutive_failures` | |
| `description`, `last_triggered` | — | dropped (`webhook_description_dropped`); v3 webhooks have no description column |
| — | `signature_scheme = 'legacy_v1'` | C6 |

**Legacy delivery format** (`integrations/legacy_v1.py`), used when `signature_scheme` is `legacy_v1` or `both`:
- `scan.created` body is v1's shape: `{"event": "scan.created", "timestamp": "<UTC ISO-8601 without offset, like v1>", "data": {"qr_id", "qr_title", "workspace_id", "short_code" (the legacy code if set, else the v3 code), "scan_id" (event id), "destination", "matched_rule" (rule id or null), "device_type", "browser", "os", "country", "city", "scanned_at"}}`.
- Other events (v1 never sent them) use the v3 payload.
- Headers: `Content-Type: application/json`, `X-QRit-Event`, `X-QRit-Signature: sha256=<hex>`; with `both` also the Standard Webhooks headers.
- Unlike v1, deliveries are asynchronous with retries (plan §7.14), and v3 batches `scan.created` (≤ 1 delivery per code per 10 s). For `legacy_v1` webhooks v3 sends **one delivery per scan** (no batching) to keep the v1 contract; owners who switch to `standard` get batching.

### 5.20 API keys → `api_keys`

**Workspace keys** (`workspace_api_keys`):

| v1 | v3 |
|---|---|
| `id`, `workspace_id`, `name`, `created_by`, `created_at`, `expires_at`, `revoked_at`, `last_used_at` | same |
| `prefix` (`qk_<8 hex>`) | `prefix` |
| `key_hash` (bcrypt of the secret part) | `legacy_bcrypt_hash`; `key_hash` = 32 random bytes; `legacy_kind = 'v1_workspace'` |
| `scopes` (CSV) | `scopes` (`text[]`), keeping `qr:read`, `qr:write`, `analytics:read`, `webhooks:write`, `leads:read` |
| `calls_today`, `calls_reset_at`, `total_calls` | — |
| — | `environment = 'live'`, `ip_allowlist = {}` |

**User keys** (`users.api_key`, `ak_<32 hex>`): imported only when used in the last 90 days (`api_calls_reset_at >= cutover − 90 days` and `api_calls_today > 0`), or when the user has `--import-all-user-keys` set (default off). Target: the user's personal workspace (or their first owned workspace), `name = 'v1 personal API key'`, `prefix = first 12 chars` (16 on a unique conflict), `key_hash = sha256(full key)`, `legacy_kind = 'v1_user'`, scopes `qr:read, qr:write, analytics:read`, `created_by` = user. Unused keys are reported (`user_key_unused_skipped`) and the announcement email tells those users to create a new key if they need one.

**Verification at request time** (`developer/auth.py`): a bearer/X-API-Key value is looked up by `sha256(value)` first. If not found and it matches `^qk_[0-9a-f]{8}\.` → find `prefix` with `legacy_bcrypt_hash IS NOT NULL`, `bcrypt.checkpw(secret, legacy_bcrypt_hash)` (rate-limited 10/min/prefix), then in one transaction set `key_hash = sha256(value)`, clear `legacy_bcrypt_hash`. Cache negative results 60 s.

### 5.21 Scans → `scan_events`

Before inserting, the importer creates monthly partitions from the earliest v1 scan month to the current month by calling the `SECURITY DEFINER` function `qrit_ensure_scan_partitions(from, to)` (plan §6.1 — the app role can't run DDL). Each batch of 10,000 rows is `COPY`ed into a temporary staging table (`CREATE TEMP TABLE … (LIKE scan_events) ON COMMIT DROP`) and then moved with `INSERT INTO scan_events SELECT … FROM staging ON CONFLICT (event_id, occurred_at) DO NOTHING`, so re-runs, resumes and the delta pass never fail on duplicates.

| v1 `qr_scans` | v3 `scan_events` | Rule |
|---|---|---|
| `id` | `event_id` | same UUID |
| `scanned_at` | `occurred_at` | `NULL` → skip (`scan_no_timestamp`) |
| `qr_id` | `qr_code_id`, `workspace_id`, `campaign_id`, `version_id` (version 1) | scans of codes that weren't imported are skipped (`scan_code_missing`) |
| — | `domain_id` | the domain row of the code's `legacy_host` — a legacy platform row, or the workspace's customer-domain row for v1 custom domains (§5.10) |
| — | `rule_id` | `NULL` (v1 didn't record it) |
| — | `outcome = 'redirect'`, `method = 'GET'`, `is_duplicate = false` | |
| `device_type` | `device_type`, `is_bot`, `bot_reason` | `mobile`/`tablet`/`desktop` same; `bot` → `device_type='other'`, `is_bot=true`, `bot_reason='v1_ua'`; NULL → NULL |
| `ip_address`, `user_agent` | `visitor_hash` | 16 bytes of `HMAC-SHA256(import_salt, qr_id ‖ 0x00 ‖ ip ‖ 0x00 ‖ ua)`; `import_salt` = 32 random bytes generated per run, **never stored** (so the hash can't be reversed or linked to live v3 hashes) |
| — | `is_unique` | true for the first scan of each `(visitor_hash, qr_code_id, UTC day)` within the import, computed in SQL after loading |
| `os`, `os_version`, `browser`, `browser_version` | same | OS and browser families translated to the v3 vocabulary (same table as §5.15; browsers: `Chrome`/`Chrome Mobile`/`Chrome Mobile iOS`→`Chrome`, `Mobile Safari`/`Safari`→`Safari`, `Firefox*`→`Firefox`, `Edge`→`Edge`, `Samsung Internet`→`Samsung Internet`, else `other`); versions truncated to 64 chars |
| `country_code` | `country` | upper-case 2 letters, else NULL |
| `region`, `city` | same | truncated to 64 / 80 chars |
| `language` | `language` | primary subtag, lower-case (same function as ingest) |
| `referrer` | `referrer_host` | host part only |
| — | `utm_*` | NULL |
| — | `source = 'v1_import'` | excluded from anomaly detection and alert baselines |
| `country_name`, `latitude`, `longitude` | — | not migrated |

After loading: `analytics.services.rebuild_rollups(day_from, day_to, qr_code_ids=imported)` recomputes `scan_stats_15m`, `scan_stats_daily_dim` and `scan_visitors_daily` (last 2 days only) for the touched days; then `qr_codes.unique_scans = count(is_unique)` and `last_scanned_at = max(occurred_at)` per code. `total_scans` stays the v1 counter (§5.13). Retention jobs then apply the plan's raw-event window as usual.

### 5.22 Stripe subscriptions → `subscriptions`

For users with `stripe_subscription_id` (and, when `--stripe-sync` is on, a Stripe subscription in status `active`, `trialing`, `past_due` or `paused`):

| v3 | Value |
|---|---|
| `org_id` | the user's organisation; `workspace_id = NULL` |
| `provider` | `stripe` |
| `provider_customer_id`, `provider_subscription_id` | v1 `stripe_customer_id`, `stripe_subscription_id` |
| `plan_id`, `billing_interval` | from the subscription's price id through `STRIPE_LEGACY_PRICE_MAP` (JSON `{"price_…": {"plan": "pro", "interval": "month"}}`, filled from the inventory); unknown price → the §5.3 plan mapping and `month` (`stripe_price_unmapped`) |
| `status`, `current_period_start/end`, `cancel_at_period_end`, `canceled_at` | from Stripe (`--stripe-sync`) or v1 `subscription_status` / `subscription_ends_at`, mapped to the v3 CHECK: `active`, `trialing`, `past_due`, `paused`, `canceled`, `incomplete` keep their name; `unpaid` → `past_due`; `incomplete_expired` → not imported (`subscription_expired_skipped`); a subscription whose mapped plan is `free` is not imported |
| `seats` | `1` |

The v1 Stripe webhook endpoint `POST /api/v1/billing/webhook` stays mapped to the v3 Stripe webhook handler on the `api` service (plan §7.23), so the existing Stripe endpoint and signing secret (`STRIPE_WEBHOOK_SECRET`) keep working. Keep **one** registered Stripe endpoint: when convenient after cutover, edit that endpoint's URL in the Stripe dashboard to `https://<API_DOMAIN>/v1/billing/stripe/webhook` (editing the URL keeps its signing secret); both paths verify with the same secret. The handler finds the subscription by `provider_subscription_id`, falling back to `metadata.userId` (v1 checkout metadata) → `org_for_user`.

### 5.23 Audit logs → `audit_logs`

Inserted in `created_at` order (so sealing order follows time), before the worker seals them:

| v1 | v3 |
|---|---|
| `workspace_id` | `workspace_id` (if imported, else NULL) and `org_id` (the workspace's org, else the user's org) |
| `user_id` | `actor_type='user'`, `actor_id` |
| `action` | `'legacy.' + action` (e.g. `legacy.qr.create`) — the `legacy.` prefix keeps them out of the v3 action catalogue test |
| `resource`, `resource_id` | `target_type`, `target_id` |
| `details` | `changes = {"v1_details": <parsed JSON or string>}` (redacted with `audit.redact`) |
| `ip_address` | `ip_prefix` (`/24` or `/48`) |
| `user_agent`, `created_at` | same |
| — | `request_id = 'v1-import'` |

At the end of the run each organisation gets one system row `legacy.import_completed` with the counts. Sealing then proceeds normally; the chain starts with the imported history.

---

## 6. Serving v1 links after cutover

**Hosts.** `LEGACY_HOSTS` = the v1 backend service's Railway domain, the host of v1 `SHORT_LINK_BASE_URL` (if different), every custom domain attached to the v1 backend service, and every hostname from §5.10. The v1 frontend's host is **not** a legacy link host: v1 displayed `NEXT_PUBLIC_APP_URL/r/<code>` on the code detail page but that path was a 404 in v1; in v3 the web app answers `/r/<code>` with a `308` redirect to `https://<v1 backend host>/r/<code>` so links copied from that page start working.

**Where `/r/<code>` is served.**
- The `api` service (which is the re-used v1 backend service and therefore owns the v1 backend's domains) routes `/r/<code>` to the same async view as the redirect service (`apps.redirect.views.legacy_redirect`).
- The `redirect` service also serves `/r/<code>` when the request host is in `LEGACY_HOSTS` or equals the code's `legacy_host` (for hosts later moved behind Cloudflare).

**Lookup.** `SELECT … FROM qr_codes WHERE legacy_short_code = $1 AND deleted_at IS NULL` (case-sensitive, exact, served by the unique partial index), then the normal pipeline (plan §7.9) with `domain_id` = the domain row of the request host (§5.10). Cached in the same L1/L2 caches under key `legacy:{code}`; the API's `qr:invalidate` message carries the legacy code too, and both the api and redirect processes evict it, so an owner's edit reaches `/r/<code>` immediately. Codes created later through the v1-compatible API (plan §7.23) also get a `legacy_short_code`, so v1 API clients that build `/r/<short_code>` keep working.

**Geo for legacy traffic.** v1 resolved the country from CDN headers or, if configured, an external lookup (`GEOIP_LOOKUP_URL`, `reference/v1-django/api/utils/geo.py`). Legacy traffic reaches the `api` service directly (the v1 backend's Railway domain can't be put behind Cloudflare), so the Worker's geo headers are missing. The api applies plan §7.1 rule 4: Cloudflare headers when the peer is a Cloudflare IP (a v1 custom domain proxied by Cloudflare), otherwise `LEGACY_GEOIP_LOOKUP_URL` — set it to the same service v1 used — called only for codes whose rules or geo restriction need the country. If v1 production had no geo source (inventory §2 shows almost no `country_code` values), leave it unset: country rules behaved as "unknown" in v1 and still do. If v1 had geo but the lookup can't be kept, country rules on legacy links change behaviour — list the affected codes from the inventory and tell their owners before cutover.

**Status codes compared with v1** (the destination is the same; only the error pages change):

| Case | v1 | v3 |
|---|---|---|
| Unknown code | 404 text | 404 page |
| Deactivated (`is_active=false`) | 404 "QR code not found" | `paused` → 410 page "This QR code is not active" |
| Scheduled, not started | 425 text | 404 page "Not active yet" |
| Expired / scan limit reached | 410 text | 410 page |
| Password | form; wrong → 401 | same, plus rate limit |
| Geo blocked | 451 text | 451 page |
| Redirect | 302 | 302 (`Cache-Control: private, no-store`) |

**Other v1 URLs kept on the `api` service:** `/health` (alias of `/healthz`, so the old health-check path keeps passing during the switch), `/api/v1/billing/webhook` (Stripe, §5.22), the v1 public API endpoints listed in plan §7.23. All other `/api/v1/*` paths answer `410 Gone` with a problem+json body pointing to `/v1`.

---

## 7. Printed v1 dynamic codes (`needs_reprint`)

- **Dashboard:** codes with `needs_reprint = true` show a banner on the code page and a filter chip in the list: "Printed copies of this code made with QRit v1 open **<v1_printed_payload host>** directly. Changing the destination here updates the short link and new downloads, but not those printed copies. Download the new image to make future prints editable." Buttons: **Download new image**, **Mark as reprinted** (sets `needs_reprint = false`, audited `qr.reprint_acknowledged`).
- **Editing the destination** of such a code shows the same warning in the confirmation dialog.
- **Analytics:** scans of printed v1 images never reach QRit (they go straight to the destination); the analytics page says so for these codes.
- The organisation's codes-needing-reprint count appears on the home page until all are acknowledged.

---

## 8. Verification (`verify_v1_import`)

```
python manage.py verify_v1_import --source "$V1_DATABASE_URL" [--sample 500] [--http-base https://<v1 backend host>] [--json out.json]
```

Exit `0` only when every check passes. Checks:

1. **Counts:** per entity, v1 rows = v3 rows mapped in `legacy_id_map` + skips in the last report. Any unexplained difference fails.
2. **Freshness:** recompute each v1 row's checksum and compare with `legacy_id_map.checksum`; any mismatch means v1 changed after the import → run the delta pass.
3. **Field round-trip:** for every imported QR code, recompute from v3 the v1-equivalent tuple `(legacy_short_code, destination, password present, starts_at, expires_at, scan_limit, active)` and compare with v1.
4. **Redirect equivalence** (the main gate): for all codes with rules, geo restrictions, passwords, limits or schedules, plus a random sample of `--sample` others, evaluate `semantics/v1_redirect.py` on the v1 row and the v3 resolver on the v3 row for a fixed matrix of synthetic requests: countries `US, IN, RU, ""`; devices mobile/tablet/desktop; OS `iOS, Android, Windows, ""`; languages `en-US, hi-IN, fr`; times 00:30, 12:00, 23:30 UTC on three dates around each date-range boundary; scan counts `0`, `limit−1`, `limit`; with and without the correct password. The outcome class (redirect URL, not_found, not_started, expired, limit, password, geo) MUST match, except the documented differences in §5.15 and §6. Weight rules are checked statistically: over 10,000 synthetic visitors the share of each variant is within ±2 points of the configured weight.
5. **HTTP smoke** (when `--http-base` is given, i.e. after cutover or on staging): for 50 sampled codes, `GET <base>/r/<code>` without following redirects returns the expected status and `Location`.
6. **Secrets:** for sampled password codes, `argon2.verify(v3 hash, v1 plaintext)`; for sampled webhooks, decrypting `secret_ciphertext` equals the v1 secret; for sampled `ak_` keys, `sha256(key)` finds the v3 key; for `qk_` keys, `legacy_bcrypt_hash` equals the v1 hash.
7. **Users:** every imported password hash starts with `bcrypt$$2` or is NULL with a `password_unusable` report row.
8. **Scans:** per code, v3 `scan_events` with `source='v1_import'` = v1 scans − skips; rollup sums equal event counts per day.
9. **Leads:** count per form; decrypting sampled submissions gives the v1 `data` (minus `opt_in`).
10. **Plans:** every organisation's effective entitlements are ≥ its current usage (no organisation is over a limit at cutover).
11. **RLS sanity:** connected as the app role inside `workspace_scope(ws)`, a query without a workspace filter on `qr_codes` returns only that workspace's rows (for 5 sampled workspaces).

Results go to stdout (summary) and the JSON file (details, no secrets or PII).

---

## 9. Customer communication

`python manage.py send_v1_migration_emails --kind <kind> [--dry-run] [--only-org <id>]` sends through Resend in batches (≤ 2 per second), records each send in `legacy_email_log (kind, user_id, sent_at)` so a re-run never sends twice, and supports `--dry-run` (renders and prints counts). Templates live in `apps/legacy/templates/email/` (plain text + simple HTML). Final wording is Aryan's; the content below is required.

| Kind | When | To | Must say |
|---|---|---|---|
| `announcement` | T−7 days | every v1 user | QRit is moving to a new version on **<date, time IST>** for about 45 minutes; links and printed codes keep working during and after; you'll sign in again (same password; Google sign-in still works); new dashboard address if it changes; password reset links sent before the switch stop working; contact address |
| `reprint_notice` | T−7 days | owners/admins of workspaces with dynamic v1 codes | printed v1 codes open their destination directly (explain plainly, no blame); after the switch each such code has a banner and a "Download new image" button; editable printed codes need the new image; count of affected codes |
| `api_notice` | T−7 days | owners of v1 API keys used in the last 90 days | keys keep working; v1 endpoints are deprecated on **<sunset date = cutover + 6 months>**; link to the v3 API docs and migration notes; webhook signatures unchanged until they switch |
| `leads_notice` | T+1 day | owners of migrated lead pages | review the consent notice generated from their v1 text; retention is now 365 days (configurable); leads are encrypted |
| `completed` | T+1 day | every v1 user | the switch is done; what's new (one paragraph); how to get help |

---

## 10. Cutover data procedure

(Infrastructure steps and timings are in `RAILWAY_DEPLOY.md` §6; this is the data part.)

1. **Rehearsal on staging (≥ 2 times):** restore the latest v1 production backup into a staging copy of the v1 database, run `v1_inventory`, `import_v1 --dry-run`, `import_v1`, `verify_v1_import --http-base <staging api host>`, time each step, fix issues, repeat until clean and within the window.
2. **T−1 day, production, v1 still live:** `import_v1` (full) into the production v3 database. v3 is not serving yet. `verify_v1_import` (checks 1, 3, 4, 6–11; freshness differences are expected).
3. **Maintenance window starts:** make v1 read-only — `ALTER DATABASE <v1 db> SET default_transaction_read_only = on;` then restart the v1 backend so every connection picks it up. Redirects keep working (v1 writes scans from a background thread and only logs failures); dashboard edits fail. Scans during the window are not recorded (accepted; the window is ≈ 45 minutes).
4. `import_v1 --since <start of step 2 − 10 min>` (delta).
5. `verify_v1_import` — all checks MUST pass (go/no-go gate).
6. Switch traffic (RAILWAY_DEPLOY §6), then `verify_v1_import --http-base https://<v1 backend host>` (check 5) against production.
7. Window ends. Send `completed` the next day.

---

## 11. Rollback (data side)

- The v1 database is never written by v3. Rolling back = make it writable again (`ALTER DATABASE <v1 db> SET default_transaction_read_only = off;`) and restore the v1 deployments (RAILWAY_DEPLOY §7).
- Changes made in v3 after cutover are not copied back automatically. `python manage.py export_post_cutover_changes --since <cutover>` writes a CSV of codes created or edited and destinations changed after the cutover, so they can be re-applied in v1 by hand or kept for the next attempt.
- The v3 database is kept as is; the next attempt starts again from a fresh full import into a new database (simplest) or a delta on top (allowed if no v3 user activity matters).

---

## 12. After the migration

- **v1 database:** keep it read-only for **30 days** after a successful cutover (rollback safety), take a final backup, then delete the v1 Postgres service. It contains plaintext QR passwords, webhook secrets, API keys and lead data; deleting it on time is part of the DPDP/GDPR minimisation story. Record the deletion in `DECISIONS.md`.
- **Legacy API** (`/api/v1/*` compatibility endpoints, `X-QRit-Signature` webhooks): removed after the sunset date announced in `api_notice`; the dashboard shows owners which keys/webhooks still use them from 60 days before.
- **Legacy links** (`/r/<code>`) are **never** removed — printed and shared links live for years.
- **Grandfathered limits** are reviewed after 12 months (business decision).
- Remove `--import-all-user-keys` and the `import_v1` command from production images after 30 days (keep the code in the repo for reference).

---

## 13. Tests

`apps/legacy/tests/` MUST cover:

- **Fixture database:** `fixtures/v1_seed.sql` creates the v1 schema (generated once from `reference/v1-django` migrations with Django 4.2 in a throwaway venv and committed as SQL) and seeds: users (bcrypt password with a known plaintext; a Google-created user; a case-duplicate email pair for the blocking test; plans free/starter/pro/enterprise; one expired paid plan), workspaces with members of every role, a pending and an expired invite, folders with a cycle and a missing parent and duplicate names, campaigns of every status, templates in both design shapes, codes of every v1 `qr_type`, static and dynamic, dynamic with `mailto:` destination, password/expiry/max scans/schedule, every routing condition and operator incl. `not_in`, weight, unknown condition, malformed values, geo `allow:`/`block:`/malformed, codes with `workspace_id NULL`, an orphan code, scans incl. bots and NULL fields across 3 months, lead pages with every field type and invalid JSON, leads with and without opt-in, webhooks with and without secret and a private URL, workspace and user API keys (one used recently, one unused), a custom domain, security policies with every field, audit logs.
- **Pure mapping tests:** `mapping/*` functions with table-driven cases and hypothesis (design colours/shapes, rules — including the `not_in` + `""` cases and the complement time window, content classification, slugs).
- **Importer tests:** full import of the fixture → expected counts and report codes; **run twice → identical row counts and checksums**; kill after N batches (simulate with an exception) and re-run → complete and identical; delta: modify/delete rows in the fixture DB → delta applies exactly those changes.
- **Behaviour tests:** a v1 bcrypt user logs in via `/v1/auth/login` and the hash becomes argon2; the Google-created user links on first Google sign-in (C4) and a non-imported account does not; an imported `ak_` key and `qk_…` key authenticate, and the `qk_` key's bcrypt hash is replaced after first use; a v1 invite link token accepts; `GET /r/<code>` on a legacy host returns exactly what `semantics/v1_redirect.py` returns for the redirect-equivalence matrix; a legacy webhook delivery verifies with v1's signature scheme and has v1's body shape; lead submissions decrypt to the v1 data; `verify_v1_import` passes on the fixture and fails when one row is altered.
- **Performance test (marked slow):** 100,000 generated codes and 1,000,000 scans import within the targets in §4.6 on CI hardware scaled proportionally.

---

## 14. Open items for Aryan

1. Run `v1_inventory` on production (read-only) and share the counts, the email-duplicate list, the v1 hosts (backend Railway domain, `SHORT_LINK_BASE_URL`, custom domains attached to the backend service) and the Stripe price ids.
2. Confirm the plan mapping (`starter` → `pro`, `pro` → `business`) and the grandfathering rules in §5.3.
3. Choose the email-duplicate strategy (§5.1) if the inventory finds any.
4. Approve the email wording (§9) and the dates (cutover, API sunset).
5. Confirm the 365-day default retention for migrated leads and the 30-day retention of the v1 database after cutover.
