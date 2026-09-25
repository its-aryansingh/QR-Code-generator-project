# SYSTEM / TASK PROMPT FOR GEMINI — QRit v2 code generation

You are a **code generator**. Your only job is to produce the complete, buildable source code of the QRit v2 monorepo described below, phase by phase. The specification here is final. You do not design, discuss, summarise or suggest; you implement.

## 0. OUTPUT PROTOCOL (MANDATORY — violating any rule makes the output unusable)

1. Output **only files**. Each file uses exactly this form, with nothing between files except a single blank line:

   ### FILE: relative/path/from/repo/root.ext
   ```<language>
   <complete file content>
   ```

2. **No prose.** No introductions, explanations, summaries, bullet lists, "Here is…", "Note:", or closing remarks. The only non-file text allowed is the continuation marker in rule 5.
3. **Every file is complete.** Forbidden inside files: `TODO`, `FIXME`, `...`, `// implement`, `// rest of code`, `pass # placeholder`, stub functions that return zero values, or commented-out code. If a function is in scope for the current phase, implement it fully. If a feature belongs to a later phase, do not create its files yet.
4. **Start every phase** with `### FILE: docs/manifest/phase-<N>.txt`, a plain list of every path you will emit in that phase, in emission order. Then emit the files in exactly that order.
5. If you approach your output limit, stop **only at a file boundary** and print exactly one final line: `<<CONTINUE FROM: path/of/next/file>>`. When the user replies `continue`, resume with that file. Never split a file across responses. Never re-emit a file unless the user names it.
6. **Do not output generated code.** The outputs of `sqlc generate` and `openapi-typescript` are produced by `make gen`. Write their inputs (`*.sql` queries, `sqlc.yaml`, `openapi.yaml`) and write the code that *uses* them with the exact identifiers those tools produce (sqlc: query `-- name: GetQRCode :one` → method `GetQRCode`, params `GetQRCodeParams`, row `GetQRCodeRow` when not a whole table; openapi-typescript: `paths["/v1/…"]["post"]`).
7. **No questions.** If something is genuinely unspecified, choose the option most consistent with this spec and mark it in code with a one-line comment `// DECISION: <what and why>` (or `# DECISION:` / `{/* DECISION: */}`).
8. **Do not rename, add or remove** database tables/columns, API paths, JSON field names, env vars or directory names defined here. If code needs something not listed, add it in the most local place (a private helper, a private type) — never a new public contract.
9. Code must compile and pass `make check` after `make gen`. Imports must be real packages at their current stable major versions. No invented APIs.

## 1. STACK (use exactly these)

| Area | Choice |
|---|---|
| Monorepo | pnpm 10 workspaces + Turborepo 2 (TS packages); one Go module in `services/` |
| Backend language | Go 1.27 (`go 1.27` in go.mod) |
| HTTP router | `github.com/go-chi/chi/v5` |
| Postgres | PostgreSQL 17; driver `github.com/jackc/pgx/v5` (`pgxpool`); queries via **sqlc** (`sql_package: pgx/v5`); migrations via `github.com/pressly/goose/v3` (SQL files, embedded with `embed.FS`) |
| Redis | Redis 7; `github.com/redis/go-redis/v9`; rate limiting `github.com/go-redis/redis_rate/v10` (GCRA) |
| Jobs | `github.com/riverqueue/river` + `riverdriver/riverpgxv5` |
| Auth | `github.com/golang-jwt/jwt/v5` (EdDSA/Ed25519), `golang.org/x/crypto/argon2`, `golang.org/x/oauth2` (Google) |
| IDs | `github.com/google/uuid` (`uuid.NewV7()`) |
| Config | `github.com/caarlos0/env/v11` |
| Geo | `github.com/oschwald/geoip2-golang/v2` (GeoLite2-City + GeoLite2-ASN mmdb files) |
| UA parsing (ingest only) | `github.com/ua-parser/uap-go` |
| Cache | `github.com/hashicorp/golang-lru/v2/expirable`, `golang.org/x/sync/singleflight` |
| Logging/metrics/tracing | `log/slog` (JSON), `github.com/prometheus/client_golang`, `go.opentelemetry.io/otel` (OTLP HTTP exporter, optional) |
| Tests (Go) | `testing`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go` (postgres, redis modules), `github.com/getkin/kin-openapi` (contract tests) |
| Frontend | Next.js 16 (App Router), React 19.2, TypeScript (strict), Tailwind CSS v4, shadcn/ui (Radix), `@tanstack/react-query` v5, `zustand` v5, `react-hook-form` + `zod` + `@hookform/resolvers`, `nuqs`, `sonner`, `lucide-react`, `cmdk`, `recharts` v3, `d3-geo` + `topojson-client` + `world-atlas` |
| API client | `openapi-typescript` (types) + `openapi-fetch` |
| QR matrix | `qrcode` (node-qrcode) `QRCode.create` |
| Decode check | `jsqr` (in a Web Worker) |
| Render service | Node 24 LTS, `fastify` v5, `@resvg/resvg-js`, `pdfkit`, `svg-to-pdfkit` |
| Tests (TS) | `vitest`, `@testing-library/react`, `@playwright/test` |
| Lint/format | `golangci-lint` (config in repo), ESLint 9 flat config + `eslint-config-next`, Prettier |
| Containers | Multi-stage Dockerfiles (distroless static for Go, `node:24-slim` for web/render) |

## 2. REPOSITORY STRUCTURE (create exactly this layout)

```
qrit/
  api/openapi.yaml
  apps/
    web/                         # Next.js app (marketing + dashboard + hosted pages)
      src/app/…                  # routes listed in §9
      src/features/…             # feature modules listed in §9
      src/components/{ui,layout,charts,feedback}/…
      src/lib/{api/client.ts,api/query-keys.ts,auth.ts,env.ts,format.ts,tz.ts,entitlements.ts,cn.ts}
      src/styles/globals.css
      src/workers/decode.worker.ts
      e2e/*.spec.ts
      next.config.ts  proxy.ts (in src/; Next.js 16 file convention, exports `proxy`)  playwright.config.ts  vitest.config.ts  Dockerfile  package.json  tsconfig.json
    render/
      src/{server.ts,render.ts,fonts.ts,auth.ts}
      fonts/ (README.md that says to place Geist-Regular.ttf and Geist-SemiBold.ttf here; Dockerfile downloads them from the npm package `geist`)
      Dockerfile  package.json  tsconfig.json  vitest.config.ts
  packages/
    qr-render/src/{index.ts,matrix.ts,shapes.ts,finder.ts,gradient.ts,logo.ts,frame.ts,warnings.ts,design.ts,canonical.ts,contrast.ts}
    qr-render/test/*.test.ts
    api-client/{package.json,src/index.ts}   # src/schema.d.ts is GENERATED
    config/{eslint,tsconfig,prettier}
  services/
    go.mod
    cmd/{api,redirect,ingest,worker,migrate}/main.go
    internal/                    # packages listed in §3
    db/{migrations/*.sql, queries/*.sql, sqlc.yaml}
    Dockerfile                   # one image, ARG CMD selects binary
    .golangci.yml
  deploy/{fly/*.toml, cloudflare/README.md}
  tests/load/*.js                # k6
  docker-compose.yml  Makefile  pnpm-workspace.yaml  turbo.json  .github/workflows/ci.yml  .env.example  .gitignore  README.md
```

## 3. GO PACKAGES (services/internal) AND LAYERING

`config · platform/{db,redisx,httpx,obs,clock,idgen,crypto} · auth · rbac · workspace · entitlements · qr · version · routing · urlsafety · shortcode · domains · resolve · scan · ingest · analytics · realtime · webhooks · billing · render · storage · email · audit · abuse · jobs · idempotency · apierr`

- Layering: `handler.go` (HTTP parse/validate/map errors) → `service.go` (business rules, transactions) → `repo.go` (wraps sqlc `dbgen` package). Handlers never call repos directly. Cross-domain calls go service → service through small interfaces declared in the consumer package.
- sqlc output package: `services/internal/platform/db/dbgen` (one package for all queries).
- Transactions: `db.WithTx(ctx, pool, func(q *dbgen.Queries, tx pgx.Tx) error)`; River jobs that must accompany a write are inserted with `riverClient.InsertTx(ctx, tx, …)`.
- `routing` and `shortcode` and `urlsafety/validate.go` are **pure** (no I/O) and shared by `api` and `redirect`.
- Every exported function takes `context.Context` first. No package-level mutable state outside `main`.

## 4. NAMING CONVENTIONS

- Go: package names are single lowercase words; files `snake_case.go`; tests `_test.go` beside code; errors `var ErrXxx = errors.New("…")`; constructors `NewXxx`; interfaces named by capability (`SafetyChecker`, `Clock`, `LinkCache`).
- SQL: plural snake_case tables; indexes `{table}_{purpose}_idx`, unique `_uniq`; sqlc query names PascalCase verb-first (`CreateQRCode`, `ListQRCodes`, `GetResolvedLink`).
- HTTP: `/v1/...` kebab-case plural nouns; JSON fields `snake_case`; enum values lowercase `snake_case`; timestamps RFC 3339 UTC.
- TypeScript: React components `PascalCase.tsx`; everything else `kebab-case.ts`; hooks `use-xxx.ts` exporting `useXxx`; zod schemas `xxxSchema`; query keys from `lib/api/query-keys.ts` only.
- Env vars: `SCREAMING_SNAKE_CASE`, exactly as in §12.
- Redis keys: `link:v1:{domain_id}:{code}`, `salt:{yyyy-mm-dd}`, `dup:{qr}:{vh_b64}`, `v:{qr}:{vh_b64}`, `rt:{ws}:{yyyymmddHHMM}`, `rt:{ws}:{qr}:{yyyymmddHHMM}`, `rl:{scope}:{id}`, `an:{ws}:{sha1}`; stream `scans`, DLQ `scans:dlq`; pub/sub channels `qr:invalidate`, `domain:invalidate`, `ent:invalidate`.

## 5. DATABASE (use this DDL verbatim as `services/db/migrations/00001_init.sql`, wrapped in goose `-- +goose Up` / `-- +goose Down`)

```sql
-- =====================================================================
-- QRit v2 — PostgreSQL 17 schema (goose migration 00001_init.sql body)
-- Conventions:
--   * All ids are UUIDv7 generated by the application (time-ordered).
--   * Enumerations are TEXT + CHECK (cheap to evolve, no ALTER TYPE).
--   * Every tenant-owned table carries workspace_id; every query filters on it.
--   * Timestamps are timestamptz, stored in UTC.
--   * No raw IP addresses are stored anywhere. ip_prefix = /24 (v4) or /48 (v6).
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ---------------------------------------------------------------------
-- Identity
-- ---------------------------------------------------------------------
CREATE TABLE users (
    id                 uuid PRIMARY KEY,
    email              citext      NOT NULL UNIQUE,
    email_verified_at  timestamptz,
    password_hash      text,                         -- argon2id PHC string; NULL = OAuth-only
    name               text        NOT NULL DEFAULT '',
    avatar_url         text,
    locale             text        NOT NULL DEFAULT 'en',
    timezone           text        NOT NULL DEFAULT 'UTC',   -- IANA name
    is_staff           boolean     NOT NULL DEFAULT false,
    last_login_at      timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);

CREATE TABLE oauth_accounts (
    id                uuid PRIMARY KEY,
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          text NOT NULL CHECK (provider IN ('google')),
    provider_user_id  text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

-- Rotating refresh-token sessions with reuse detection (family revoke).
CREATE TABLE sessions (
    id                  uuid PRIMARY KEY,
    user_id             uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id           uuid  NOT NULL,
    refresh_token_hash  bytea NOT NULL UNIQUE,       -- sha256(opaque token)
    user_agent          text,
    ip_prefix           text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_used_at        timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,
    replaced_by         uuid REFERENCES sessions(id) ON DELETE SET NULL
);
CREATE INDEX sessions_user_active_idx ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_family_idx      ON sessions (family_id);

CREATE TABLE email_tokens (
    id          uuid PRIMARY KEY,
    user_id     uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     text  NOT NULL CHECK (purpose IN ('verify_email','reset_password','magic_link')),
    token_hash  bytea NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- Tenancy
-- ---------------------------------------------------------------------
CREATE TABLE workspaces (
    id                 uuid PRIMARY KEY,
    name               text   NOT NULL,
    slug               citext NOT NULL UNIQUE,
    owner_id           uuid   NOT NULL REFERENCES users(id),
    plan_id            text   NOT NULL DEFAULT 'free'
                       CHECK (plan_id IN ('free','pro','business','enterprise')),
    timezone           text   NOT NULL DEFAULT 'UTC',
    default_domain_id  uuid,                                   -- FK added below
    brand              jsonb  NOT NULL DEFAULT '{}'::jsonb,    -- {logo_file_id, primary_color, page_theme}
    settings           jsonb  NOT NULL DEFAULT '{}'::jsonb,    -- {require_https, allowed_destination_hosts[]}
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);

CREATE TABLE workspace_members (
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role          text NOT NULL CHECK (role IN ('owner','admin','editor','analyst')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id);
CREATE UNIQUE INDEX workspace_one_owner_uniq ON workspace_members (workspace_id) WHERE role = 'owner';

CREATE TABLE invites (
    id            uuid PRIMARY KEY,
    workspace_id  uuid   NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email         citext NOT NULL,
    role          text   NOT NULL CHECK (role IN ('admin','editor','analyst')),
    token_hash    bytea  NOT NULL UNIQUE,
    invited_by    uuid   NOT NULL REFERENCES users(id),
    expires_at    timestamptz NOT NULL,
    accepted_at   timestamptz,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX invites_pending_uniq ON invites (workspace_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Short-link domains. workspace_id NULL = platform-owned domain.
CREATE TABLE domains (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid   REFERENCES workspaces(id) ON DELETE CASCADE,
    hostname              citext NOT NULL UNIQUE,
    status                text   NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','verifying','active','failed','disabled')),
    verification_token    text   NOT NULL,
    provider_hostname_id  text,                         -- Cloudflare for SaaS custom hostname id
    tls_status            text   NOT NULL DEFAULT 'pending'
                          CHECK (tls_status IN ('pending','active','failed')),
    root_redirect_url     text,                         -- where https://domain/ goes
    not_found_url         text,                         -- where unknown codes go
    last_checked_at       timestamptz,
    verified_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX domains_workspace_idx ON domains (workspace_id);

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_default_domain_fk
    FOREIGN KEY (default_domain_id) REFERENCES domains(id) ON DELETE SET NULL;

-- ---------------------------------------------------------------------
-- Organisation of QR codes
-- ---------------------------------------------------------------------
CREATE TABLE folders (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id     uuid REFERENCES folders(id) ON DELETE CASCADE,
    name          text NOT NULL,
    position      integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (workspace_id, parent_id, name)
);

CREATE TABLE tags (
    id            uuid PRIMARY KEY,
    workspace_id  uuid   NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          citext NOT NULL,
    color         text   NOT NULL DEFAULT '#64748B',
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE campaigns (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          text NOT NULL,
    status        text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('draft','active','paused','ended','archived')),
    starts_at     timestamptz,
    ends_at       timestamptz,
    goal_scans    bigint CHECK (goal_scans IS NULL OR goal_scans > 0),
    utm           jsonb NOT NULL DEFAULT '{}'::jsonb,  -- {source, medium, campaign, term, content}
    created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX campaigns_ws_status_idx ON campaigns (workspace_id, status);

CREATE TABLE templates (
    id            uuid PRIMARY KEY,
    workspace_id  uuid    NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          text    NOT NULL,
    design        jsonb   NOT NULL,
    is_locked     boolean NOT NULL DEFAULT false,   -- editors must use it verbatim
    is_default    boolean NOT NULL DEFAULT false,
    created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX templates_one_default_uniq ON templates (workspace_id) WHERE is_default;

CREATE TABLE files (
    id            uuid PRIMARY KEY,
    workspace_id  uuid   NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    purpose       text   NOT NULL CHECK (purpose IN ('logo','hosted_asset','export','bulk_input','bulk_output')),
    storage_key   text   NOT NULL UNIQUE,
    mime_type     text   NOT NULL,
    size_bytes    bigint NOT NULL CHECK (size_bytes >= 0),
    sha256        bytea  NOT NULL,
    created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE INDEX files_ws_purpose_idx ON files (workspace_id, purpose);

-- ---------------------------------------------------------------------
-- QR codes and immutable destination versions
-- ---------------------------------------------------------------------
CREATE TABLE qr_codes (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by          uuid REFERENCES users(id) ON DELETE SET NULL,
    mode                text NOT NULL CHECK (mode IN ('static','dynamic')),
    content_type        text NOT NULL CHECK (content_type IN (
                            'url','text','email','phone','sms','whatsapp','wifi','vcard',
                            'event','upi','location','links_page','file','app_store','gs1')),
    name                text NOT NULL,
    domain_id           uuid REFERENCES domains(id),
    short_code          text,              -- 7 chars Crockford base32 upper-case; dynamic only
    legacy_short_code   text,              -- v1 migration only (case-sensitive)
    gs1_gtin            char(14),          -- GS1 Digital Link: served at /01/{gtin}
    static_payload      text,              -- exact string encoded in a static code
    static_content      jsonb,             -- structured form input that produced static_payload
    current_version_id  uuid,              -- FK added below; denormalised pointer
    design              jsonb NOT NULL DEFAULT '{}'::jsonb,
    design_hash         bytea,             -- sha256(canonical design json)
    template_id         uuid REFERENCES templates(id) ON DELETE SET NULL,
    folder_id           uuid REFERENCES folders(id)   ON DELETE SET NULL,
    campaign_id         uuid REFERENCES campaigns(id) ON DELETE SET NULL,
    status              text NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','paused','archived','blocked')),
    is_read_only        boolean NOT NULL DEFAULT false,  -- set on downgrade over plan limit
    starts_at           timestamptz,
    expires_at          timestamptz,
    scan_limit          bigint CHECK (scan_limit IS NULL OR scan_limit > 0),
    password_hash       text,              -- argon2id; NULL = no password
    fallback_url        text,              -- served when paused/expired/limit reached
    safety_status       text NOT NULL DEFAULT 'pending'
                        CHECK (safety_status IN ('pending','safe','flagged','blocked')),
    total_scans         bigint NOT NULL DEFAULT 0,   -- maintained by ingest
    unique_scans        bigint NOT NULL DEFAULT 0,   -- maintained by ingest
    last_scanned_at     timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    archived_at         timestamptz,
    deleted_at          timestamptz,               -- soft delete; restorable for 30 days
    CONSTRAINT qr_dynamic_has_link CHECK (
        (mode = 'dynamic' AND short_code IS NOT NULL AND domain_id IS NOT NULL AND static_payload IS NULL)
     OR (mode = 'static'  AND short_code IS NULL     AND static_payload IS NOT NULL)
    ),
    CONSTRAINT qr_static_types CHECK (
        mode = 'dynamic' OR content_type NOT IN ('links_page','file','app_store','gs1')
    ),
    CONSTRAINT qr_gs1_gtin CHECK (
        (content_type = 'gs1') = (gs1_gtin IS NOT NULL) AND (gs1_gtin IS NULL OR gs1_gtin ~ '^[0-9]{14}$')
    ),
    CONSTRAINT qr_dynamic_types CHECK (
        mode = 'static' OR content_type NOT IN ('wifi','upi','text','location')
    )
);
-- Never filtered by deleted_at: a short code is burned forever once issued.
CREATE UNIQUE INDEX qr_codes_domain_code_uniq   ON qr_codes (domain_id, short_code) WHERE short_code IS NOT NULL;
CREATE UNIQUE INDEX qr_codes_legacy_code_uniq   ON qr_codes (legacy_short_code) WHERE legacy_short_code IS NOT NULL;
CREATE UNIQUE INDEX qr_codes_domain_gtin_uniq   ON qr_codes (domain_id, gs1_gtin) WHERE gs1_gtin IS NOT NULL;
CREATE INDEX qr_codes_ws_list_idx      ON qr_codes (workspace_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX qr_codes_ws_folder_idx    ON qr_codes (workspace_id, folder_id)   WHERE deleted_at IS NULL;
CREATE INDEX qr_codes_ws_campaign_idx  ON qr_codes (workspace_id, campaign_id) WHERE deleted_at IS NULL;
CREATE INDEX qr_codes_name_trgm_idx    ON qr_codes USING gin (name gin_trgm_ops);

-- Short codes of hard-purged rows (account deletion) are reserved here forever.
CREATE TABLE short_code_tombstones (
    domain_id   uuid NOT NULL REFERENCES domains(id),
    short_code  text NOT NULL,
    purged_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (domain_id, short_code)
);

CREATE TABLE qr_code_tags (
    qr_code_id  uuid NOT NULL REFERENCES qr_codes(id) ON DELETE CASCADE,
    tag_id      uuid NOT NULL REFERENCES tags(id)     ON DELETE CASCADE,
    PRIMARY KEY (qr_code_id, tag_id)
);
CREATE INDEX qr_code_tags_tag_idx ON qr_code_tags (tag_id);

-- Immutable. A destination edit = a new row. Restore = copy of an old row as a new row.
CREATE TABLE qr_versions (
    id                uuid PRIMARY KEY,
    qr_code_id        uuid    NOT NULL REFERENCES qr_codes(id) ON DELETE CASCADE,
    version_no        integer NOT NULL CHECK (version_no > 0),
    destination_kind  text    NOT NULL CHECK (destination_kind IN ('url','hosted_page')),
    destination_url   text,
    hosted_page       jsonb,                                  -- vcard / links_page / file / event page content
    rules             jsonb   NOT NULL DEFAULT '[]'::jsonb,   -- routing rules (Section 5.4)
    utm               jsonb   NOT NULL DEFAULT '{}'::jsonb,   -- appended to destination if absent
    effective_at      timestamptz NOT NULL DEFAULT now(),     -- future = scheduled change
    safety_status     text    NOT NULL DEFAULT 'pending'
                      CHECK (safety_status IN ('pending','safe','flagged','blocked')),
    restored_from     uuid REFERENCES qr_versions(id),
    change_note       text,
    created_by        uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by_key    uuid,                                   -- api_keys.id when created via API
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (qr_code_id, version_no),
    CONSTRAINT qr_version_destination CHECK (
        (destination_kind = 'url'         AND destination_url IS NOT NULL AND hosted_page IS NULL)
     OR (destination_kind = 'hosted_page' AND hosted_page     IS NOT NULL)
    )
);
CREATE INDEX qr_versions_effective_idx ON qr_versions (qr_code_id, effective_at DESC, version_no DESC);

ALTER TABLE qr_codes
    ADD CONSTRAINT qr_codes_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES qr_versions(id) DEFERRABLE INITIALLY DEFERRED;

-- ---------------------------------------------------------------------
-- Scan analytics
-- ---------------------------------------------------------------------
-- Raw events. Monthly range partitions, created 3 months ahead by a job.
-- Retained 13 months, then partitions are dropped.
CREATE TABLE scan_events (
    event_id         uuid        NOT NULL,          -- UUIDv7 minted by redirect service
    occurred_at      timestamptz NOT NULL,
    workspace_id     uuid        NOT NULL,
    qr_code_id       uuid        NOT NULL,
    version_id       uuid,
    campaign_id      uuid,
    domain_id        uuid        NOT NULL,
    rule_id          text,
    outcome          text        NOT NULL CHECK (outcome IN (
                         'redirect','hosted_page','password_prompt','password_ok','password_fail',
                         'geo_blocked','paused','expired','not_started','limit_reached','blocked')),
    method           text        NOT NULL DEFAULT 'GET',
    is_bot           boolean     NOT NULL DEFAULT false,
    bot_reason       text,
    is_duplicate     boolean     NOT NULL DEFAULT false,
    is_unique        boolean     NOT NULL DEFAULT false,  -- first counted scan of visitor/QR/UTC-day
    visitor_hash     bytea       NOT NULL,               -- 16 bytes, daily-salted HMAC
    device_type      text CHECK (device_type IN ('mobile','tablet','desktop','other')),
    os               text,
    os_version       text,
    browser          text,
    browser_version  text,
    country          char(2),
    region           text,
    city             text,
    language         text,
    referrer_host    text,
    utm_source       text,
    utm_medium       text,
    utm_campaign     text,
    PRIMARY KEY (event_id, occurred_at)
) PARTITION BY RANGE (occurred_at);

CREATE INDEX scan_events_qr_time_idx ON scan_events (qr_code_id, occurred_at DESC);
CREATE INDEX scan_events_ws_time_idx ON scan_events (workspace_id, occurred_at DESC);

CREATE TABLE scan_events_default PARTITION OF scan_events DEFAULT;
CREATE TABLE scan_events_2026_09 PARTITION OF scan_events
    FOR VALUES FROM ('2026-09-01 00:00:00+00') TO ('2026-10-01 00:00:00+00');
CREATE TABLE scan_events_2026_10 PARTITION OF scan_events
    FOR VALUES FROM ('2026-10-01 00:00:00+00') TO ('2026-11-01 00:00:00+00');
CREATE TABLE scan_events_2026_11 PARTITION OF scan_events
    FOR VALUES FROM ('2026-11-01 00:00:00+00') TO ('2026-12-01 00:00:00+00');

-- Unique-visitor ledger. Only today and yesterday are needed; pruned daily.
CREATE TABLE scan_visitors_daily (
    qr_code_id     uuid  NOT NULL,
    day            date  NOT NULL,          -- UTC day
    visitor_hash   bytea NOT NULL,
    first_seen_at  timestamptz NOT NULL,
    PRIMARY KEY (qr_code_id, day, visitor_hash)
);

-- Rollups: the dashboard reads ONLY these (plus scan_events for the raw log).
-- 15-minute buckets (not hourly) so that local-time charts are exact in every
-- timezone, including +05:30 (IST) and +05:45 (NPT).
CREATE TABLE scan_stats_15m (
    qr_code_id    uuid        NOT NULL,
    bucket_start  timestamptz NOT NULL,       -- date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00')
    workspace_id  uuid        NOT NULL,
    campaign_id   uuid,                        -- campaign at time of scan
    scans         integer     NOT NULL DEFAULT 0,   -- counted: not bot, not duplicate, reached destination
    unique_scans  integer     NOT NULL DEFAULT 0,
    bot_hits      integer     NOT NULL DEFAULT 0,
    blocked_hits  integer     NOT NULL DEFAULT 0,   -- geo/paused/expired/limit/blocked outcomes
    PRIMARY KEY (qr_code_id, bucket_start)
);
CREATE INDEX scan_stats_15m_ws_idx       ON scan_stats_15m (workspace_id, bucket_start);
CREATE INDEX scan_stats_15m_campaign_idx ON scan_stats_15m (campaign_id, bucket_start) WHERE campaign_id IS NOT NULL;

CREATE TABLE scan_stats_daily_dim (
    qr_code_id    uuid    NOT NULL,
    day           date    NOT NULL,           -- UTC day
    dimension     text    NOT NULL CHECK (dimension IN (
                      'country','region','city','device','os','browser','language',
                      'referrer','rule','version','utm_source')),
    key           text    NOT NULL,           -- 'Unknown' when null; city key = 'IN/Noida'
    workspace_id  uuid    NOT NULL,
    campaign_id   uuid,
    scans         integer NOT NULL DEFAULT 0,
    unique_scans  integer NOT NULL DEFAULT 0,
    PRIMARY KEY (qr_code_id, day, dimension, key)
);
CREATE INDEX scan_stats_dim_ws_idx ON scan_stats_daily_dim (workspace_id, dimension, day);

-- ---------------------------------------------------------------------
-- Integrations
-- ---------------------------------------------------------------------
CREATE TABLE api_keys (
    id            uuid PRIMARY KEY,
    workspace_id  uuid   NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          text   NOT NULL,
    prefix        text   NOT NULL UNIQUE,     -- e.g. 'qk_live_7F3K2M9Q' (shown in UI)
    key_hash      bytea  NOT NULL UNIQUE,     -- sha256(full key)
    scopes        text[] NOT NULL DEFAULT ARRAY['qr:read','qr:write','analytics:read'],
    created_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    last_used_at  timestamptz,
    expires_at    timestamptz,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_keys_ws_idx ON api_keys (workspace_id) WHERE revoked_at IS NULL;

CREATE TABLE webhooks (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid    NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    url                   text    NOT NULL,
    secret_ciphertext     bytea   NOT NULL,       -- AES-256-GCM(secret) with APP_ENCRYPTION_KEY
    events                text[]  NOT NULL,       -- 'scan.created','qr.created','qr.version.created',...
    is_active             boolean NOT NULL DEFAULT true,
    consecutive_failures  integer NOT NULL DEFAULT 0,
    disabled_reason       text,
    created_by            uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhooks_ws_active_idx ON webhooks (workspace_id) WHERE is_active;

CREATE TABLE webhook_deliveries (
    id                uuid PRIMARY KEY,
    webhook_id        uuid    NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_id          uuid    NOT NULL,           -- idempotency key sent as Webhook-Id header
    event_type        text    NOT NULL,
    payload           jsonb   NOT NULL,
    status            text    NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','succeeded','failed','dead')),
    attempts          integer NOT NULL DEFAULT 0,
    last_status_code  integer,
    last_error        text,
    next_attempt_at   timestamptz,
    delivered_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (webhook_id, event_id)
);
CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries (webhook_id, created_at DESC);

-- ---------------------------------------------------------------------
-- Billing (plans + entitlements live in code: internal/entitlements)
-- ---------------------------------------------------------------------
CREATE TABLE subscriptions (
    id                        uuid PRIMARY KEY,
    workspace_id              uuid NOT NULL UNIQUE REFERENCES workspaces(id) ON DELETE CASCADE,
    provider                  text NOT NULL CHECK (provider IN ('stripe','razorpay')),
    provider_customer_id      text NOT NULL,
    provider_subscription_id  text NOT NULL UNIQUE,
    plan_id                   text NOT NULL CHECK (plan_id IN ('pro','business','enterprise')),
    billing_interval          text NOT NULL CHECK (billing_interval IN ('month','year')),
    status                    text NOT NULL CHECK (status IN (
                                  'trialing','active','past_due','paused','canceled','incomplete')),
    seats                     integer NOT NULL DEFAULT 1 CHECK (seats > 0),
    current_period_start      timestamptz,
    current_period_end        timestamptz,
    cancel_at_period_end      boolean NOT NULL DEFAULT false,
    canceled_at               timestamptz,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now()
);

-- Provider webhooks are processed exactly once.
CREATE TABLE billing_events (
    id                 uuid PRIMARY KEY,
    provider           text  NOT NULL,
    provider_event_id  text  NOT NULL,
    type               text  NOT NULL,
    payload            jsonb NOT NULL,
    processed_at       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_event_id)
);

-- ---------------------------------------------------------------------
-- Governance, safety, jobs, idempotency
-- ---------------------------------------------------------------------
-- Append-only. The application DB role has INSERT + SELECT only on this table.
CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id  uuid,
    actor_type    text NOT NULL CHECK (actor_type IN ('user','api_key','system','staff')),
    actor_id      uuid,
    action        text NOT NULL,                  -- 'qr.version.created', 'member.role.changed', ...
    target_type   text NOT NULL,
    target_id     uuid,
    changes       jsonb NOT NULL DEFAULT '{}'::jsonb,   -- {"before":{...},"after":{...}}
    ip_prefix     text,
    user_agent    text,
    request_id    text,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_ws_time_idx ON audit_logs (workspace_id, created_at DESC);
CREATE INDEX audit_logs_target_idx  ON audit_logs (target_type, target_id);

CREATE TABLE abuse_reports (
    id                  uuid PRIMARY KEY,
    qr_code_id          uuid REFERENCES qr_codes(id) ON DELETE SET NULL,
    hostname            citext NOT NULL,
    short_code          text   NOT NULL,
    reason              text   NOT NULL CHECK (reason IN ('phishing','malware','scam','spam','illegal','other')),
    details             text,
    reporter_email      citext,
    reporter_ip_prefix  text,
    status              text NOT NULL DEFAULT 'open' CHECK (status IN ('open','actioned','dismissed')),
    resolved_by         uuid REFERENCES users(id),
    resolved_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX abuse_reports_open_idx ON abuse_reports (created_at) WHERE status = 'open';

CREATE TABLE jobs (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('bulk_create','export_scans','export_qr_codes','bulk_download')),
    status          text NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued','running','succeeded','failed','canceled')),
    input_file_id   uuid REFERENCES files(id),
    output_file_id  uuid REFERENCES files(id),
    params          jsonb   NOT NULL DEFAULT '{}'::jsonb,
    total           integer NOT NULL DEFAULT 0,
    processed       integer NOT NULL DEFAULT 0,
    failed          integer NOT NULL DEFAULT 0,
    errors          jsonb   NOT NULL DEFAULT '[]'::jsonb,   -- [{row, field, message}] capped at 1000
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    finished_at     timestamptz
);
CREATE INDEX jobs_ws_time_idx ON jobs (workspace_id, created_at DESC);

CREATE TABLE idempotency_keys (
    workspace_id   uuid  NOT NULL,
    key            text  NOT NULL CHECK (length(key) BETWEEN 8 AND 128),
    method         text  NOT NULL,
    path           text  NOT NULL,
    request_hash   bytea NOT NULL,
    status_code    integer,                 -- NULL while in flight
    response_body  jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, key)
);
CREATE INDEX idempotency_expiry_idx ON idempotency_keys (expires_at);
```

- `00002_river.sql`: River's migration SQL for the River version in go.mod (use `rivermigrate` programmatically in `cmd/migrate` instead of copying SQL if simpler — DECISION allowed).
- Down migrations drop in reverse dependency order.
- `sqlc.yaml`: engine postgresql, schema `db/migrations`, queries `db/queries`, gen go `package: dbgen`, `out: internal/platform/db/dbgen`, `sql_package: pgx/v5`, `emit_json_tags: true`, `emit_pointers_for_null_types: true`, `emit_empty_slices: true`, overrides: `uuid` → `github.com/google/uuid.UUID` (nullable → `*uuid.UUID` via `github.com/google/uuid.NullUUID` is NOT used; use pointer), `timestamptz` → `time.Time`, `jsonb` → `encoding/json.RawMessage`, `citext` → `string`.

**Ingest batch transaction** (implement in `internal/ingest/pipeline.go` as these exact statements; load `staging` with `pgx.CopyFrom`):

```sql
-- =====================================================================
-- Ingest batch transaction (executed by cmd/ingest for every batch of
-- up to 1,000 stream messages). Exactly-once *effect*: replaying the same
-- batch changes nothing, because only rows that were actually inserted
-- into scan_events (captured in new_ev) feed the counters.
-- =====================================================================
BEGIN;
SET LOCAL TimeZone = 'UTC';

CREATE TEMP TABLE staging (LIKE scan_events INCLUDING DEFAULTS) ON COMMIT DROP;
-- Go: pgx.CopyFrom(ctx, pgx.Identifier{"staging"}, columns, rows)

CREATE TEMP TABLE new_ev (event_id uuid, occurred_at timestamptz) ON COMMIT DROP;

-- 1. Insert raw events; remember which ones are genuinely new.
WITH ins AS (
    INSERT INTO scan_events SELECT * FROM staging
    ON CONFLICT (event_id, occurred_at) DO NOTHING
    RETURNING event_id, occurred_at
)
INSERT INTO new_ev SELECT event_id, occurred_at FROM ins;

-- 2. Unique-visitor detection (per QR, per UTC day) for new counted events.
WITH cand AS (
    SELECT DISTINCT ON (s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash)
           s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date AS day,
           s.visitor_hash, s.occurred_at, s.event_id
    FROM staging s
    JOIN new_ev n USING (event_id, occurred_at)
    WHERE NOT s.is_bot AND NOT s.is_duplicate
      AND s.outcome IN ('redirect','hosted_page','password_ok')
    ORDER BY s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash, s.occurred_at
), firsts AS (
    INSERT INTO scan_visitors_daily (qr_code_id, day, visitor_hash, first_seen_at)
    SELECT qr_code_id, day, visitor_hash, occurred_at FROM cand
    ON CONFLICT DO NOTHING
    RETURNING qr_code_id, day, visitor_hash
)
UPDATE scan_events e SET is_unique = true
FROM cand c JOIN firsts f USING (qr_code_id, day, visitor_hash)
WHERE e.event_id = c.event_id AND e.occurred_at = c.occurred_at;

-- Working set with the "counted" flag resolved once.
CREATE TEMP TABLE batch ON COMMIT DROP AS
SELECT e.*,
       (NOT e.is_bot AND NOT e.is_duplicate
        AND e.outcome IN ('redirect','hosted_page','password_ok'))            AS counted,
       (NOT e.is_bot
        AND e.outcome IN ('geo_blocked','paused','expired','not_started',
                          'limit_reached','blocked'))                          AS blocked
FROM scan_events e
JOIN new_ev n USING (event_id, occurred_at);

-- 3. 15-minute rollup.
INSERT INTO scan_stats_15m AS h
       (qr_code_id, bucket_start, workspace_id, campaign_id, scans, unique_scans, bot_hits, blocked_hits)
SELECT qr_code_id,
       date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'),
       workspace_id,
       (array_agg(campaign_id ORDER BY occurred_at DESC))[1],
       count(*) FILTER (WHERE counted),
       count(*) FILTER (WHERE counted AND is_unique),
       count(*) FILTER (WHERE is_bot),
       count(*) FILTER (WHERE blocked)
FROM batch
GROUP BY qr_code_id, date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'), workspace_id
ON CONFLICT (qr_code_id, bucket_start) DO UPDATE SET
       scans        = h.scans        + EXCLUDED.scans,
       unique_scans = h.unique_scans + EXCLUDED.unique_scans,
       bot_hits     = h.bot_hits     + EXCLUDED.bot_hits,
       blocked_hits = h.blocked_hits + EXCLUDED.blocked_hits,
       campaign_id  = COALESCE(EXCLUDED.campaign_id, h.campaign_id);

-- 4. Daily dimension rollup (counted scans only).
INSERT INTO scan_stats_daily_dim AS d
       (qr_code_id, day, dimension, key, workspace_id, campaign_id, scans, unique_scans)
SELECT b.qr_code_id,
       (b.occurred_at AT TIME ZONE 'UTC')::date,
       x.dimension,
       x.key,
       b.workspace_id,
       (array_agg(b.campaign_id ORDER BY b.occurred_at DESC))[1],
       count(*),
       count(*) FILTER (WHERE b.is_unique)
FROM batch b
CROSS JOIN LATERAL (VALUES
    ('country',    COALESCE(b.country, 'Unknown')),
    ('region',     COALESCE(b.country || '/' || b.region, 'Unknown')),
    ('city',       COALESCE(b.country || '/' || b.city, 'Unknown')),
    ('device',     COALESCE(b.device_type, 'other')),
    ('os',         COALESCE(b.os, 'Unknown')),
    ('browser',    COALESCE(b.browser, 'Unknown')),
    ('language',   COALESCE(b.language, 'Unknown')),
    ('referrer',   COALESCE(b.referrer_host, 'Direct')),
    ('rule',       COALESCE(b.rule_id, 'default')),
    ('version',    COALESCE(b.version_id::text, 'Unknown')),
    ('utm_source', COALESCE(b.utm_source, 'None'))
) AS x(dimension, key)
WHERE b.counted
GROUP BY b.qr_code_id, (b.occurred_at AT TIME ZONE 'UTC')::date, x.dimension, x.key, b.workspace_id
ON CONFLICT (qr_code_id, day, dimension, key) DO UPDATE SET
       scans        = d.scans        + EXCLUDED.scans,
       unique_scans = d.unique_scans + EXCLUDED.unique_scans;

-- 5. Denormalised counters on qr_codes (list view + scan_limit enforcement).
UPDATE qr_codes q SET
       total_scans     = q.total_scans  + a.scans,
       unique_scans    = q.unique_scans + a.uniques,
       last_scanned_at = GREATEST(COALESCE(q.last_scanned_at, a.last_at), a.last_at)
FROM (
    SELECT qr_code_id,
           count(*) FILTER (WHERE counted)               AS scans,
           count(*) FILTER (WHERE counted AND is_unique) AS uniques,
           max(occurred_at) FILTER (WHERE counted)       AS last_at
    FROM batch GROUP BY qr_code_id
) a
WHERE q.id = a.qr_code_id AND a.scans > 0;

COMMIT;
-- After COMMIT: XACK every message id in the batch.
```

**Analytics queries** (put in `db/queries/analytics.sql` verbatim; add `BreakdownRaw<Dimension>` siblings for region, city, device, os, browser, language, referrer, utm_source, rule, version using the same pattern; add `TimeseriesHourly` and `TimeseriesWeekly` using `date_trunc('hour'|'week', bucket_start AT TIME ZONE tz)` with zero-fill):

```sql
-- =====================================================================
-- Analytics read queries (sqlc, internal/analytics/queries.sql).
-- Parameters: $ws workspace id, $from/$to timestamptz (half-open [from,to)),
-- $tz IANA zone, $qr uuid[] (NULL = all), $campaign uuid (NULL = all).
-- All ranges are computed in the viewer's timezone by the API and passed as
-- UTC instants aligned to local midnight.
-- =====================================================================

-- name: SummaryTotals :one
SELECT COALESCE(sum(scans),0)::bigint        AS scans,
       COALESCE(sum(unique_scans),0)::bigint AS unique_scans,
       COALESCE(sum(bot_hits),0)::bigint     AS bot_hits,
       COALESCE(sum(blocked_hits),0)::bigint AS blocked_hits
FROM scan_stats_15m
WHERE workspace_id = sqlc.arg(ws)
  AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
  AND (sqlc.narg(campaign)::uuid IS NULL OR campaign_id = sqlc.narg(campaign)::uuid);

-- name: TimeseriesDaily :many
-- Zero-filled local-day series.
WITH series AS (
    SELECT generate_series(
             (sqlc.arg(from_ts)::timestamptz AT TIME ZONE sqlc.arg(tz)::text)::date,
             ((sqlc.arg(to_ts)::timestamptz - interval '1 microsecond') AT TIME ZONE sqlc.arg(tz)::text)::date,
             interval '1 day')::date AS day
), agg AS (
    SELECT (bucket_start AT TIME ZONE sqlc.arg(tz)::text)::date AS day,
           sum(scans) AS scans, sum(unique_scans) AS unique_scans
    FROM scan_stats_15m
    WHERE workspace_id = sqlc.arg(ws)
      AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
      AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
      AND (sqlc.narg(campaign)::uuid IS NULL OR campaign_id = sqlc.narg(campaign)::uuid)
    GROUP BY 1
)
SELECT s.day, COALESCE(a.scans,0)::bigint AS scans, COALESCE(a.unique_scans,0)::bigint AS unique_scans
FROM series s LEFT JOIN agg a USING (day)
ORDER BY s.day;

-- name: Heatmap :many
-- Weekday (1=Mon..7=Sun) x local hour-of-day.
SELECT extract(isodow FROM bucket_start AT TIME ZONE sqlc.arg(tz)::text)::int AS weekday,
       extract(hour   FROM bucket_start AT TIME ZONE sqlc.arg(tz)::text)::int AS hour_of_day,
       sum(scans)::bigint AS scans
FROM scan_stats_15m
WHERE workspace_id = sqlc.arg(ws)
  AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
GROUP BY 1, 2;

-- name: BreakdownRollup :many
-- Ranges >= 3 days: UTC-day dimension rollups (edge days approximate by < 1 day of data).
SELECT key, sum(scans)::bigint AS scans, sum(unique_scans)::bigint AS unique_scans
FROM scan_stats_daily_dim
WHERE workspace_id = sqlc.arg(ws)
  AND dimension = sqlc.arg(dimension)
  AND day >= (sqlc.arg(from_ts)::timestamptz AT TIME ZONE 'UTC')::date
  AND day <= ((sqlc.arg(to_ts)::timestamptz - interval '1 microsecond') AT TIME ZONE 'UTC')::date
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
  AND (sqlc.narg(campaign)::uuid IS NULL OR campaign_id = sqlc.narg(campaign)::uuid)
GROUP BY key
ORDER BY scans DESC
LIMIT sqlc.arg(row_limit);

-- name: BreakdownRawCountry :many
-- Ranges < 3 days: exact, straight from raw events (one query per dimension column).
SELECT COALESCE(country, 'Unknown') AS key,
       count(*)::bigint AS scans,
       count(*) FILTER (WHERE is_unique)::bigint AS unique_scans
FROM scan_events
WHERE workspace_id = sqlc.arg(ws)
  AND occurred_at >= sqlc.arg(from_ts) AND occurred_at < sqlc.arg(to_ts)
  AND NOT is_bot AND NOT is_duplicate
  AND outcome IN ('redirect','hosted_page','password_ok')
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
GROUP BY 1
ORDER BY scans DESC
LIMIT sqlc.arg(row_limit);

-- name: TopQRCodes :many
SELECT s.qr_code_id, q.name, q.content_type, q.short_code,
       sum(s.scans)::bigint AS scans, sum(s.unique_scans)::bigint AS unique_scans
FROM scan_stats_15m s
JOIN qr_codes q ON q.id = s.qr_code_id AND q.workspace_id = s.workspace_id
WHERE s.workspace_id = sqlc.arg(ws)
  AND s.bucket_start >= sqlc.arg(from_ts) AND s.bucket_start < sqlc.arg(to_ts)
GROUP BY s.qr_code_id, q.name, q.content_type, q.short_code
ORDER BY scans DESC
LIMIT sqlc.arg(row_limit);
```

## 6. DOMAIN RULES

**6.1 Short codes.** Alphabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ`, length 7, `crypto/rand`, reject if the code contains any denylist word (ship a list of ≥ 50 offensive words in `shortcode/denylist.go`) or equals `HEALTH01`-style reserved codes. Normalise on input: upper-case, `O→0`, `I→1`, `L→1`, strip one trailing `/`. Valid regex after normalisation: `^[0-9A-HJKMNP-TV-Z]{7}$`. Retry creation up to 5 times on unique violation (`23505` on `qr_codes_domain_code_uniq`). The **encoded payload** of a dynamic code is `strings.ToUpper("https://" + hostname + "/" + code)`.

**6.2 URL validation (`urlsafety.Validate(raw string, policy Policy) (normalized string, err error)`).** Trim; length ≤ 2048; no control chars/spaces; parse; scheme allow-list `https, http, mailto, tel, sms, geo, whatsapp, instagram, spotify, upi`; `http` allowed unless `policy.RequireHTTPS`; http(s) requires a host; host → punycode via `idna.Lookup`; reject userinfo; reject hosts equal to or ending in any platform/custom short domain (`policy.OwnHosts`); reject known shorteners (`bit.ly, t.co, tinyurl.com, goo.gl, ow.ly, is.gd, buff.ly, rebrand.ly, cutt.ly, shorturl.at`) unless `policy.AllowShorteners`; enforce `policy.AllowedHosts` (exact or `*.suffix`) when non-empty; reject mixed-script hostnames (Latin + Cyrillic/Greek in one label). Errors are typed codes: `invalid_url, too_long, scheme_not_allowed, https_required, missing_host, userinfo_not_allowed, own_domain, shortener_not_allowed, host_not_allowed, confusable_host`.

**6.3 Safety.** `SafetyChecker.Check(ctx, url) (Verdict{Status: safe|unsafe|unknown, Threats []string})` backed by Google Web Risk `GET https://webrisk.googleapis.com/v1/uris:search?threatTypes=MALWARE&threatTypes=SOCIAL_ENGINEERING&threatTypes=UNWANTED_SOFTWARE&uri=…&key=…`, timeout 800 ms, Redis cache 30 min. `unsafe` → API `422 destination_unsafe`. `unknown` → store `safety_status='pending'` and insert River job `SafetyRecheck{QRCodeID, VersionID}` scheduled +1 min. When `WEB_RISK_API_KEY` is empty (local), use a checker that returns `safe` except for hosts ending in `.unsafe.test`.

**6.4 Content encoders (`qr/content`).** Deterministic string payloads for static types:
- `url` → normalised URL · `text` → as-is (≤ 1,000 chars)
- `email` → `mailto:{to}?subject=…&body=…` (percent-encoded) · `phone` → `tel:{e164}` · `sms` → `SMSTO:{e164}:{message}`
- `whatsapp` → `https://wa.me/{digits}?text={urlenc}`
- `wifi` → `WIFI:T:{WPA|WEP|nopass};S:{ssid};P:{password};H:{true|false};;` with `\` escaping of `\ ; , : "`
- `vcard` → vCard 3.0 with CRLF line endings, escaped `, ; \`
- `event` → `BEGIN:VEVENT…END:VEVENT` (UTC times)
- `upi` → `upi://pay?pa={vpa}&pn={name}&am={amount 2dp, optional}&cu=INR&tn={note}` (VPA regex `^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$`)
- `location` → `geo:{lat},{lng}`
- `app_store` (dynamic only) → a normal dynamic URL code whose version has `destination_url` = the fallback (website) URL and two generated rules: `os in [iOS, iPadOS]` → App Store URL, `os in [Android]` → Google Play URL
- `gs1` (dynamic only, Business) → `qr_codes.gs1_gtin` holds the 14-digit GTIN (validate the GS1 mod-10 check digit; pad GTIN-8/12/13 to 14 with leading zeros). The encoded payload is `HTTPS://{HOST}/01/{GTIN14}` instead of the short code URL; the redirect resolves `GET /01/{gtin14}` (optionally followed by further GS1 key segments such as `/10/{lot}` or `/21/{serial}`, which are ignored for resolution) via `(domain_id, gs1_gtin)`. A short code is still issued as a secondary link.

The same encoders exist in TypeScript in `apps/web/src/features/qr-editor/encoders.ts`; both implementations share the test vectors in `packages/qr-render/test/vectors.json` (≥ 30 cases).

**6.5 Design JSON (DesignV1).** Exactly these fields:
```jsonc
{ "v":1, "ecc":"auto|L|M|Q|H", "quiet_zone":0-10,
  "modules":{"shape":"square|dots|rounded|extra-rounded|classy|classy-rounded","color":"#RRGGBB","gradient":null|{"type":"linear|radial","rotation":0-359,"stops":[{"offset":0,"color":"#RRGGBB"},{"offset":1,"color":"#RRGGBB"}]}},
  "finder":{"outer_shape":"square|rounded|circle|leaf","outer_color":"#RRGGBB","inner_shape":"square|rounded|circle|dot","inner_color":"#RRGGBB"},
  "background":{"color":"#RRGGBB","transparent":false},
  "logo":null|{"file_id":"uuid","size_ratio":0.10-0.30,"padding":0-4,"clear_modules":true,"shape":"square|circle"},
  "frame":null|{"style":"banner-bottom|banner-top|rounded-box|speech","text":"≤24 chars","text_color":"#RRGGBB","color":"#RRGGBB"} }
```
Unknown keys → `422 invalid_design`. `ecc: auto` → `H` if logo else `M`. Canonical JSON (sorted keys, no whitespace, lower-case hex) → `design_hash = sha256`. Default design: square modules `#111111`, square finders `#111111`, white background, quiet zone 4, no logo, no frame.

**6.6 Scannability (packages/qr-render/warnings.ts + web meter).** Score starts at 100. Contrast (WCAG ratio of darkest foreground colour vs background): ≥7 → 0; 4–7 → −10; 2–4 → −35 + warning `low_contrast`; <2 → **blocked** `contrast_too_low`. Foreground lighter than background → −20 `inverted`. Quiet zone 2–3 → −10, <2 → −30. Logo ratio >0.25 → −15; >0.30 → **blocked**. Version >10 → −10 `dense`. Labels: ≥85 Excellent, ≥70 Good, ≥50 Risky, <50 Won't scan. The web decode check renders the SVG to a canvas at 256 and 512 px and decodes with jsQR in `decode.worker.ts`; failure → **blocked** `decode_failed`. Downloads are disabled while blocked.

**6.7 Routing rules.** JSON array on `qr_versions.rules`:
```jsonc
[{ "id":"r_x", "name":"…", "enabled":true,
   "when": null | {"all"|"any": [{"field":"country|region|device_type|os|language|local_time|weekday|date|scan_count","op":"in|not_in|between|gte|lt","value": …}]},
   "destination_url":"…"            // XOR
   "split":[{"variant":"A","weight":50,"destination_url":"…"}, …] }]   // weights sum to 100, ≤5 variants
```
Limits: ≤20 rules, ≤10 conditions per rule. Evaluate in order; first match wins; no match → version default. `local_time between ["HH:MM","HH:MM"]` may wrap midnight; `local_time`, `weekday` (1=Mon…7=Sun) and `date` use the workspace timezone. Split bucket = `binary.BigEndian.Uint32(sha256(qr_id || ip || ua)[:4]) % 100`; `rule_id` recorded as `"{id}:{variant}"`. Function signature: `routing.Resolve(v routing.Version, f routing.Facts, now time.Time, loc *time.Location) (destination string, ruleID string)`. The API validates rules (`422 invalid_rules` with field paths) and safety-checks every destination.

**6.8 Hosted page JSON.** Discriminated by `kind`: `vcard`, `links_page` (≤30 links), `file` (file_id of a PDF/image ≤20 MB), `event`. Fields as in the blueprint §5.5; validate strictly; rendered by `apps/web/src/app/p/[code]/page.tsx` via `GET /v1/public/hosted/{code}` (add this public endpoint; it returns only the page JSON + theme for active codes).

**6.9 Entitlements (`entitlements/plans.go`).** Plans `free, pro, business, enterprise` with keys: `dynamic_codes 3/100/1000/100000`, `analytics_history_days 30/365/1095/3650`, `seats 1/1/5/1000`, `owned_workspaces 1/3/10/100`, `custom_domains 0/1/5/50`, `bulk_rows_per_job 0/500/5000/50000`, `api_requests_per_min 0/0/600/3000`, `webhooks 0/0/10/50`, `templates 0/10/100/100000`; features: pro adds `scheduling, expiry, scan_limit, utm_append, hosted_pages, templates, csv_export, remove_branding`; business adds `rules, campaigns, api, webhooks, raw_scan_log, locked_templates, audit_log, gs1, roles`; enterprise adds `sso, scim, sla, dedicated_domain`. Scans are **never** limited. Violations return `402` problems `limit_reached` or `upgrade_required` with `required_plan`. On downgrade, codes beyond `dynamic_codes` (newest first) get `is_read_only = true`; they keep redirecting. Versions cannot be created for read-only codes (`402 read_only_over_limit`).

## 7. REDIRECT SERVICE BEHAVIOUR (`cmd/redirect`, packages `resolve`, `scan`, `routing`)

For `GET|HEAD|POST /{code}` on any host:
1. If the path matches `^/01/[0-9]{14}(/.*)?$`, resolve by GTIN (§6.4). Otherwise normalise the code (§6.1). Invalid format → if it matches `^[A-Za-z0-9_-]{6,12}$` try legacy lookup (`legacy_short_code`, case-sensitive, only on hosts listed in `LEGACY_HOSTS`), else render `404` page. Trailing `+` → preview page (not counted).
2. Host → domain (in-process map of `domains` with `status='active'`, reloaded every 60 s and on `domain:invalidate`). Unknown host → plain 404.
3. Resolve `ResolvedLink` via LRU (`expirable.LRU`, size `LRU_SIZE`, TTL `LRU_TTL`) → Redis `link:v1:{domain_id}:{code}` (10 min TTL; `"-"` = not found, 60 s) → Postgres query `GetResolvedLink` (joins `qr_codes`, the version with greatest `effective_at <= now()` then greatest `version_no`, `workspaces.timezone`, and `min(effective_at) > now()` as `next_change_at`). Collapse concurrent misses with `singleflight`. Entries are stale after `next_change_at`. Keep a last-known-good map (24 h) used only when both Redis and Postgres error.
4. State gates in this order → outcome → response: `blocked` (status or safety) → 410 page; `paused|archived` → 302 `fallback_url` or 410 page; before `starts_at` → 302 fallback or 404 page "not active yet"; after `expires_at` → 302 fallback or 410; `scan_limit` reached → 302 fallback or 410; password set → GET renders form (200), POST verifies argon2id (limits 5/15 min per `(qr, vh)`, 100/15 min per QR → 429 page) → `password_ok` or `password_fail` 401 form.
5. Facts: client IP from `CF-Connecting-IP` only if the peer is within `TRUSTED_PROXY_CIDRS`, else `RemoteAddr`; country from `CF-IPCountry` else MaxMind; region/city from `cf-region-code`/`cf-ipcity` else MaxMind; ASN from MaxMind ASN and `dc = ASN in datacenter list` (ship ≥ 40 ASNs); `os`/`device_type` from a regex mini-classifier (iPhone/iPad/Android/Windows/Mac/Linux; tablet if iPad or Android without "Mobile"); `language` = first primary subtag of `Accept-Language`.
6. Visitor hash: `HMAC-SHA256(salt(todayUTC), qrID || 0x00 || ip || 0x00 || ua)[:16]`. Salt: 32 random bytes, `SET salt:{date} <b64> NX EX 172800`, then `GET`; cached in-process per date. **The IP must not be stored, logged or put in the event.**
7. Destination: `routing.Resolve`; append version `utm` params (`utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`) only when absent; re-validate the scheme.
8. Event: build the JSON in §7.1 of this prompt and push it to a buffered channel (`EVENT_BUFFER_SIZE`); a goroutine flushes every 50 ms or 500 events with a pipelined `XADD scans MAXLEN ~ 5000000 * e <json>`; on channel full increment `scan_events_dropped_total` and drop. On SIGTERM: stop accepting, flush for up to 10 s, exit.
9. Respond: `302` with `Location`, `Cache-Control: private, no-store, max-age=0`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-Robots-Tag: noindex, nofollow`, `Server-Timing`. Never set cookies. Hosted pages: `302 /p/{code}`; `/p/*` and `/_next/*` are reverse-proxied to `WEB_INTERNAL_URL` preserving `Host`.
10. Status pages, password form and preview page: Go `html/template`, self-contained HTML with inline CSS (≤ 5 KB), mobile-first, light/dark via `prefers-color-scheme`, include "Report this code" link to `{APP_BASE_URL}/report?u={url}`.
11. `/healthz` 200 always; `/readyz` 200 if Redis ping OK or LRU has ≥ 1 entry.
12. Invalidation: subscribe to `qr:invalidate` (payload `{domain_id}:{code}`) and evict from LRU.

### 7.1 Stream event JSON (field names exact)
`{"id":uuidv7,"ts":rfc3339nano,"ws":uuid,"qr":uuid,"ver":uuid|null,"cmp":uuid|null,"dom":uuid,"rule":string|null,"out":"redirect|hosted_page|password_prompt|password_ok|password_fail|geo_blocked|paused|expired|not_started|limit_reached|blocked","m":"GET|HEAD|POST","vh":base64,"ua":string,"dc":bool,"geo":{"cc":string|null,"rg":string|null,"ct":string|null},"lang":string|null,"ref":string|null,"utm":{"s":string|null,"m":string|null,"c":string|null}}`

## 8. INGEST, ANALYTICS, WORKER BEHAVIOUR

- `cmd/ingest`: consumer group `ingest` on `scans` (create with `MKSTREAM` if missing), consumer name `INGEST_CONSUMER_NAME` (default hostname). Loop: `XREADGROUP COUNT INGEST_BATCH_SIZE BLOCK INGEST_BATCH_WAIT`. Per message: parse UA with uap-go (device_type mobile/tablet/desktop/other, os + major, browser + major); bot rules in order: `HEAD` → `head_request`; UA matches the bot pattern list (ship ≥ 100 patterns in `scan/botdetect/patterns.go`, including WhatsApp, facebookexternalhit, TelegramBot, Slackbot, Twitterbot, LinkedInBot, Discordbot, Applebot, Googlebot, bingbot, Safe Links/`Microsoft Office`, Proofpoint, Mimecast, Barracuda, curl, wget, python-requests, Go-http-client, axios, okhttp, HeadlessChrome, PhantomJS) → `ua_bot`; empty UA → `ua_missing`; `dc && (desktop || headless)` → `datacenter`; velocity > 30 per 60 s per `(qr, vh)` → `velocity`. Duplicate: `SET dup:{qr}:{vh} {event_id} NX PX 10000`; duplicate only if the key exists with a different value. Map `utm.s|m|c` to `utm_source|medium|campaign`. Reject events with `ts` > now+5 min or < now−7 days to `scans:dlq`. Then run the §5 batch transaction; after COMMIT: `XACK`, `INCRBY` realtime keys (TTL 7200 s), enqueue `WebhookScanCreated` River jobs for workspaces that have an active `scan.created` webhook. Every 30 s `XAUTOCLAIM` idle > 60 s; messages with delivery count > 5 → `scans:dlq`. Subcommand `ingest rebuild --day YYYY-MM-DD` recomputes that UTC day's rollups from `scan_events` in one transaction.
- `internal/analytics`: accepts `from`,`to` (local dates), `tz` (default workspace tz), converts to UTC instants at local midnight (`to` exclusive = next local midnight), clamps to plan history (`clamped:true, available_from`), granularity auto: range ≤ 1 day → `15m` (use `bucket_start` directly), ≤ 7 days → `hour`, ≤ 180 → `day`, else `week`. Breakdowns: range < 3 days → raw queries; else rollup. `share` = scans / total. Redis response cache 60 s (300 s if `to` < today). Summary includes `previous` period of equal length and `deltas` (percentage, 1 dp; previous 0 → null).
- `cmd/worker` River periodic jobs: `PartitionMaintenance` daily 01:00 UTC (create 3 months ahead; drop > 13 months; prune `scan_visitors_daily` < today−2; delete expired idempotency keys; alert on rows in `scan_events_default`); `Reconcile` daily 02:00 UTC; `SafetyRescan` daily; `AuditRetention` daily; `PurgeDeleted` daily (soft-deleted > 30 days → tombstone codes → delete). On-demand jobs: `ActivateVersion`, `SafetyRecheck`, `DomainVerify` (every 5 min up to 72 h), `WebhookDeliver` (backoff 1m,5m,30m,2h,6h,12h,24h then `dead`; auto-disable after 20 consecutive failures), `WebhookScanCreated`, `SendEmail`, `BulkCreate`, `BulkDownload`, `ExportScans`, `ExportQRCodes`.

## 9. API CONTRACT

Implement `api/openapi.yaml` (OpenAPI 3.1) covering every endpoint below, and implement handlers that satisfy it. Conventions:
- Base path `/v1`. Dashboard reaches it at `/api/v1` via Next.js rewrite; the Go server mounts routes at `/v1` and **also** at `/api/v1` (same handlers) so either works.
- Errors: `application/problem+json` `{type, title, status, code, detail, instance, errors?: [{field, code, message}], ...extras}`; `type = https://docs.example.com/errors/{code}`.
- Pagination: `limit` (≤100, default 50) + opaque base64url `cursor` → `{data, next_cursor}`.
- `Idempotency-Key` required on create POSTs (QR, versions, jobs, invites, domains, api-keys, webhooks); 24 h; same key + different body → `409 idempotency_key_reused`; in-flight → `409 request_in_progress`.
- `If-Match` on `PATCH /qr-codes/{id}` (ETag = `W/"<updated_at unix nanos>"`) → `412` on mismatch.
- Auth: cookies `qrit_at` (JWT EdDSA 10 min, HttpOnly, Secure (not in local), SameSite=Lax, Path=/), `qrit_rt` (opaque, 30 d, HttpOnly, Secure, SameSite=Strict, Path=/), `qrit_csrf` (readable, SameSite=Lax); non-GET cookie requests require header `X-CSRF-Token` equal to `qrit_csrf`. API keys: `Authorization: Bearer qk_live_<32 base32>`; stored as sha256; prefix = first 16 chars.
- Rate-limit headers `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` on 429.

Endpoints (roles: O owner, A admin, E editor, N analyst):
- Auth: `POST /auth/register`, `/auth/login`, `/auth/logout`, `/auth/refresh`, `/auth/magic-link`, `/auth/magic-link/verify`, `/auth/verify-email`, `/auth/resend-verification`, `/auth/password/forgot`, `/auth/password/reset`; `GET /auth/oauth/google/start`, `/auth/oauth/google/callback`; `GET|PATCH /me`; `GET /me/sessions`; `DELETE /me/sessions/{id}`.
- Workspaces: `GET|POST /workspaces`; `GET(N)|PATCH(A)|DELETE(O) /workspaces/{ws}`; `GET(N) /workspaces/{ws}/entitlements`; `GET(N) /workspaces/{ws}/members`; `PATCH|DELETE(A) /workspaces/{ws}/members/{userId}`; `POST(O) /workspaces/{ws}/transfer-ownership`; `GET|POST(A) /workspaces/{ws}/invites`; `DELETE(A) /workspaces/{ws}/invites/{id}`; `GET /invites/{token}`; `POST /invites/{token}/accept`.
- QR: `POST(E)|GET(N) /workspaces/{ws}/qr-codes`; `GET(N)|PATCH(E)|DELETE(A) /workspaces/{ws}/qr-codes/{id}`; `POST(E) …/{id}/pause|resume|archive|unarchive|duplicate`; `POST(A) …/{id}/restore`; `GET(N)|POST(E) …/{id}/versions`; `POST(E) …/{id}/versions/{versionId}/restore`; `DELETE(E) …/{id}/versions/{versionId}` (scheduled only); `POST(N) …/{id}/resolve-preview`; `GET(N) …/{id}/image?format=svg|png|pdf&size=256..4096`; `POST(E) /workspaces/{ws}/qr-codes/bulk-actions`.
- Organisation: `folders`, `tags`, `campaigns`, `templates` → `GET(N)|POST(E) /workspaces/{ws}/{res}`, `GET(N)|PATCH(E)|DELETE(E) /workspaces/{ws}/{res}/{id}`; `GET(N) /workspaces/{ws}/campaigns/{id}/analytics`.
- Files & jobs: `POST(E) /workspaces/{ws}/files` (multipart); `POST(E) /workspaces/{ws}/jobs`; `GET(N) /workspaces/{ws}/jobs/{id}`.
- Domains: `GET|POST(A) /workspaces/{ws}/domains`; `POST(A) …/{id}/verify`; `PATCH|DELETE(A) …/{id}`.
- Analytics (N): `/workspaces/{ws}/analytics/summary|timeseries|breakdown|heatmap|top-qr-codes|realtime|scans` with query `from,to,tz,qr_ids,campaign_id,country,device_type,granularity,dimension,limit,cursor,compare`.
- Integrations (A): `api-keys` (GET, POST, DELETE /{id}); `webhooks` (GET, POST, PATCH /{id}, DELETE /{id}, POST /{id}/test, GET /{id}/deliveries, POST /{id}/deliveries/{deliveryId}/retry); `GET /workspaces/{ws}/audit-logs`.
- Billing: `GET(A) /workspaces/{ws}/billing`; `POST(O) /workspaces/{ws}/billing/checkout` `{plan_id, interval, provider}` → `{url}`; `POST(O) /workspaces/{ws}/billing/portal` → `{url}`; `POST /billing/webhooks/stripe`, `POST /billing/webhooks/razorpay` (signature-verified; insert `billing_events` first; process once).
- Public: `GET /public/qr-types`; `POST /public/render` (PDF only; 20/min, 200/day per IP /24); `POST /public/abuse-reports` (Turnstile); `GET /public/hosted/{code}` (host from `X-Forwarded-Host`).
- Staff (is_staff): `GET /admin/abuse-reports`, `POST /admin/abuse-reports/{id}/action` `{action: block_code|suspend_workspace|dismiss}`.
- Webhook outbound headers: `Webhook-Id`, `Webhook-Timestamp`, `Webhook-Signature: v1,<base64 HMAC-SHA256(secret, id + "." + timestamp + "." + body)>`.

QR resource JSON (response): `id, mode, content_type, name, status, safety_status, is_read_only, short_code, short_url, encoded_payload, static_payload, static_content, current_version {id, version_no, destination_kind, destination_url, hosted_page, rules, utm, effective_at, change_note, created_at, created_by}, scheduled_version|null, design, design_hash, folder_id, campaign_id, tags[], starts_at, expires_at, scan_limit, has_password, fallback_url, total_scans, unique_scans, last_scanned_at, sparkline_7d[7], image_urls {svg,png,pdf}, created_at, updated_at, archived_at`.

Every mutation writes `audit_logs` in the same transaction (action names `{resource}.{verb}` e.g. `qr.created`, `qr.version.created`, `member.role_changed`, `api_key.revoked`) and invalidates caches after commit (`DEL link:…` + `PUBLISH qr:invalidate`, `web` revalidation for hosted pages).

## 10. SECURITY REQUIREMENTS (all mandatory)

argon2id (m=19456 KiB, t=2, p=1, 16-byte salt, PHC) for user passwords; (m=8192, t=1, p=1) for QR passwords. Refresh rotation with family reuse detection. Account lockout 15 min after 5 failures. Tokens (email/magic/reset/invite) = 32 random bytes, base64url, stored sha256, single use. Every repository query includes `workspace_id`; cross-tenant → 404. SSRF-safe outbound client (`platform/httpx/safeclient.go`): custom resolver + dialer that rejects private, loopback, link-local, CGNAT, multicast, unspecified, and metadata IPs, no redirects, 5 s timeout, 16 KB body cap. AES-256-GCM for webhook secrets (`APP_ENCRYPTION_KEY`, 32 bytes base64; ciphertext = keyID(1B) || nonce || sealed). Web CSP with per-request nonce set in `src/proxy.ts`. Logs never contain IPs, passwords, tokens, cookies, API keys, or full UAs on the redirect path. `audit_logs` has no UPDATE/DELETE code paths. SVG logo uploads are rasterised to PNG 512 px by `render` and the SVG is discarded; PNG/JPG uploads are re-encoded; ≤ 2 MB; magic-byte check.

## 11. UI SPECIFICATION (apps/web)

**Tokens** in `globals.css` as CSS variables for light and dark (`.dark` class via `next-themes`, default system): `--bg #FFFFFF/#0A0A0B, --bg-subtle #F7F7F8/#111113, --surface #FFFFFF/#16161A, --border #E6E6EA/#26262C, --text #0B0B0F/#F4F4F6, --text-muted #5B5B66/#A1A1AA, --accent #5B5BF7/#7C7CFF, --accent-fg #FFFFFF/#0A0A0B, --success #16A34A/#22C55E, --warning #D97706/#F59E0B, --danger #DC2626/#EF4444`; chart series `#5B5BF7, #14B8A6, #F59E0B, #EC4899, #64748B`. Fonts: Geist Sans + Geist Mono via the `geist` package. Radii 6/10/14. Motion 120/180 ms `cubic-bezier(.2,.8,.2,1)`, disabled under reduced motion. Numbers use `tabular-nums`.

**Routes**
- `(marketing)/page.tsx` — generator landing: H1, trust line, type tabs (URL, Text, Wi-Fi, vCard, WhatsApp, UPI + More: Email, SMS, Phone, Event, Location, App store*, PDF*, Links page*, GS1*; * = dynamic chip), content form (pre-focused), static/dynamic segmented control (dynamic while signed-out opens the auth sheet and stores the draft in `sessionStorage` key `qrit:draft`, restored after signup), design accordion (Presets, Pattern, Corners, Logo, Frame, Background), sticky preview + scannability meter + downloads (PNG sizes 512/1024/2048/4096, SVG, PDF via `/public/render`). Mobile: bottom preview bar that expands to a sheet. Below fold: explainer, template gallery (12 presets), use cases, pricing teaser, FAQ with JSON-LD.
- `(marketing)/[type]-qr-code/page.tsx` — `generateStaticParams` over content types; same generator with the tab preselected + type-specific copy/FAQ.
- `(marketing)/pricing/page.tsx` — monthly/annual toggle (annual default), USD/INR toggle (INR if `Accept-Language` or timezone suggests India), feature matrix from entitlements, FAQ leading with "What happens to my codes if I cancel? They keep working."
- `(auth)/{login,register,verify-email,reset-password,magic}` — Google button, email+password, magic link; errors from problem `code`.
- `w/[workspace]/layout.tsx` — AppShell: collapsible sidebar (Overview, QR codes, Analytics, Campaigns, Templates, Domains, divider, Team, Settings, usage meter + Upgrade), top bar (breadcrumb, ⌘K palette, "New QR code" button with `N` shortcut, avatar menu), workspace switcher.
- `w/[workspace]/qr/page.tsx` — table (checkbox, thumbnail 40 px, name+type icon, short link + copy, destination host, 7-day sparkline, total scans, status pill, relative updated, row menu) / cards on mobile; filters in URL (`nuqs`); search debounce 250 ms; "Load more" cursor pagination; bulk actions bar; keyboard `/`, `j`, `k`, `Enter`; skeleton rows; empty state with "Create" and "Import CSV".
- `w/[workspace]/qr/new/page.tsx` — editor (same components as landing) + Finish step (name, folder, campaign, tags) → create → redirect to detail with a success toast.
- `w/[workspace]/qr/[id]/…` — header card (preview click-to-download menu, inline-editable name, short link copy/open/preview, destination + "Edit destination", status controls) and tabs: Overview, Audience, Versions, Rules, Design, Settings. Edit-destination dialog: URL field with live validation + async safety state, When (Now / Schedule with datetime in workspace tz), note; on success toast "Destination updated · v{n}" with Undo (10 s) calling version restore.
- `w/[workspace]/analytics/page.tsx` — sticky filter bar (range presets Today/7D/30D/90D/12M/Custom with plan locks, QR multi-select, campaign, country, device, tz label), KPI row (4 tiles with delta + sparkline), main area chart (scans + uniques line; granularity auto with override), breakdown grid (countries with flag emoji bar list, cities, devices, OS, browsers, referrers/UTM), heatmap 7×24 with table toggle, rules/A-B card, data-quality footnote, Live badge polling `/realtime` every 10 s only while `document.visibilityState === 'visible'`, CSV export button (plan-gated).
- `w/[workspace]/{campaigns,templates,domains}` and `settings/{general,team,billing,api-keys,webhooks,audit-log,security}` — CRUD pages with dialogs, confirm dialogs for destructive actions (type the name to confirm for codes with > 1,000 scans), locked-feature `UpgradeCard` from entitlements.
- `p/[code]/page.tsx` — hosted pages (vcard with "Save contact" `.vcf` download; links page; file viewer with download; event with "Add to calendar" `.ics`), ISR `revalidate = 300`, tag `qr:{id}`; `api/revalidate/route.ts` (POST, header `x-revalidate-secret`).
- `admin/abuse/page.tsx` — staff table with actions.

**Component behaviours**
- Preview re-renders synchronously on every change using `@qrit/qr-render` (`useMemo` on payload+design). Decode check debounced 400 ms in the worker; shows "Verified scannable" badge on pass.
- Editor store (zustand) keeps undo/redo history of design changes (50 steps; ⌘Z / ⇧⌘Z).
- Mutations use TanStack Query with optimistic updates for pause/resume/rename/move/tag, rollback on error with a toast carrying the problem `detail`.
- `402` responses open the `UpgradeDialog` with `required_plan`; `401` triggers one refresh attempt then redirects to `/login?next=`.
- All async regions show skeletons; no full-page spinners. Every chart has "View as table".
- Accessibility: all interactive elements reachable by keyboard, visible focus ring 2 px accent, labels on inputs, `aria-live="polite"` for toasts and the scannability label, touch targets ≥ 44 px.

## 12. ENVIRONMENT VARIABLES (exact names; `.env.example` must list all with safe local defaults)

| Service | Variables |
|---|---|
| all Go | `APP_ENV` (local\|staging\|production), `LOG_LEVEL`, `DATABASE_URL`, `DATABASE_REPLICA_URL` (optional), `REDIS_URL`, `METRICS_ADDR` (:9090), `OTEL_EXPORTER_OTLP_ENDPOINT` (optional), `SENTRY_DSN` (optional) |
| api | `HTTP_ADDR` (:8080), `APP_BASE_URL`, `API_PUBLIC_URL`, `PLATFORM_SHORT_DOMAIN`, `PLATFORM_SHORT_DOMAIN_FREE` (optional), `CDN_BASE_URL`, `JWT_ED25519_PRIVATE_KEY` (base64 PKCS#8), `JWT_KEY_ID`, `APP_ENCRYPTION_KEY` (base64 32 B), `COOKIE_SECURE` (true), `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, `RESEND_API_KEY`, `EMAIL_FROM`, `WEB_RISK_API_KEY`, `RENDER_URL`, `RENDER_SHARED_SECRET`, `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_BUCKET_PUBLIC`, `S3_BUCKET_PRIVATE`, `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ZONE_ID`, `CUSTOM_DOMAIN_CNAME_TARGET`, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_PRO_MONTH`, `STRIPE_PRICE_PRO_YEAR`, `STRIPE_PRICE_BUSINESS_MONTH`, `STRIPE_PRICE_BUSINESS_YEAR`, `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, `RAZORPAY_WEBHOOK_SECRET`, `RAZORPAY_PLAN_PRO_MONTH`, `RAZORPAY_PLAN_PRO_YEAR`, `RAZORPAY_PLAN_BUSINESS_MONTH`, `RAZORPAY_PLAN_BUSINESS_YEAR`, `TURNSTILE_SECRET_KEY`, `WEB_REVALIDATE_URL`, `WEB_REVALIDATE_SECRET` |
| redirect | `HTTP_ADDR` (:8090), `APP_BASE_URL`, `WEB_INTERNAL_URL`, `MAXMIND_CITY_DB`, `MAXMIND_ASN_DB`, `TRUSTED_PROXY_CIDRS` (comma CIDRs or `cloudflare` to load the bundled list), `LRU_SIZE` (100000), `LRU_TTL` (30s), `EVENT_BUFFER_SIZE` (100000), `LEGACY_HOSTS` (optional) |
| ingest | `INGEST_BATCH_SIZE` (1000), `INGEST_BATCH_WAIT` (1s), `INGEST_CONSUMER_NAME` (optional) |
| worker | api's variables that jobs need (email, S3, render, Cloudflare, Web Risk, encryption key, web revalidate) |
| render | `PORT` (8081), `RENDER_SHARED_SECRET`, `FONT_DIR` (/app/fonts), `S3_*` (for logo fetch) |
| web | `NEXT_PUBLIC_APP_URL`, `NEXT_PUBLIC_SHORT_DOMAIN`, `NEXT_PUBLIC_CDN_URL`, `NEXT_PUBLIC_TURNSTILE_SITE_KEY`, `API_INTERNAL_URL`, `WEB_REVALIDATE_SECRET` |

Missing required variables → the process exits with a list of the missing names. When unset, optional integrations (Stripe, Razorpay, Cloudflare, Web Risk, Resend, OTEL, Sentry) switch to local fakes: log-only email (also sent to Mailpit via SMTP in compose), a safe-by-default safety checker, a no-op domain provider that marks domains active after the TXT check.

## 13. LOCAL DEV, BUILD AND CI

- `docker-compose.yml`: `postgres:17`, `redis:7`, `minio/minio` (+ bucket init), `axllent/mailpit`, and all services built from local Dockerfiles with `api` on 8080, `redirect` on 8090 (short domain `localhost:8090` in local), `render` 8081, `web` 3000. A `geoip` init step downloads GeoLite2 when `MAXMIND_LICENSE_KEY` is set, else the redirect runs without MaxMind (CF headers only).
- `Makefile` targets: `up`, `down`, `logs`, `migrate`, `gen` (sqlc + openapi-typescript), `lint`, `typecheck`, `test` (Go unit + TS unit), `test-int` (testcontainers), `e2e` (Playwright against compose), `load` (k6), `check` (= gen-check + lint + typecheck + test + test-int), `seed` (demo user `demo@example.com` / `demo-password-123`, workspace, 5 codes, 2,000 synthetic scans through the real stream).
- `.github/workflows/ci.yml`: jobs `go` (golangci-lint, `go test ./...`, integration with services), `web` (pnpm install --frozen-lockfile, lint, typecheck, vitest, next build, size-limit: landing ≤ 120 KB gz), `render`, `gen-check` (run `make gen`, `git diff --exit-code`), `e2e` (compose up, Playwright), `security` (govulncheck, `pnpm audit --prod`, gitleaks).

## 14. TESTING EXPECTATIONS (write these tests; they must pass)

- Go unit: `shortcode` (alphabet, normalisation, denylist), `urlsafety` (≥ 60 table cases), `routing` (≥ 80 cases incl. IST wrap-around, weekday in tz, split ±2% over 100k, stickiness), `qr/content` encoders (shared vectors), `auth` (argon2, JWT, refresh rotation + reuse → family revoked), `scan` (visitor hash stable within a day, different across days and QRs; bot classifier on ≥ 40 UA fixtures), `entitlements`, `apierr` mapping, `idempotency` middleware.
- Go integration (testcontainers): full flow register → create dynamic → redirect 302 → event in stream → ingest → rollup counts; ingest replay idempotency (same batch twice → identical counters); repeat visitor doesn't add uniques; IST boundary (23:50 and 00:10 IST → different local days); scheduled version activation with a fake clock; cache invalidation (destination change visible on the next request); tenant isolation over every route (principal from workspace B → 404); RBAC matrix per route; OpenAPI contract test (kin-openapi validates every request/response recorded during integration tests).
- TS unit: `qr-render` snapshot SVGs (12 presets × 5 payloads) + decode round-trip via resvg + jsQR in Node; scannability scoring; encoders (shared vectors); editor store undo/redo; analytics formatting (deltas, tz labels).
- Playwright e2e: anonymous PNG + SVG download that decodes to the input; signup → dynamic create → HTTP GET short URL → 302 → dashboard shows 1 scan within 10 s; edit destination → next GET goes to the new URL; downgrade simulation → codes read-only but still redirect; keyboard navigation of the QR list.
- k6 (`tests/load`): `redirect_hot.js`, `redirect_cold.js`, `viral_single_code.js`, `ingest_burst.js`, `dashboard_api.js` with thresholds from the spec (hot p99 < 50 ms).

## 15. IMPLEMENTATION ORDER

Generate **one phase per request**, beginning with Phase 1. Within a phase, emit files in dependency order: config/tooling → SQL/migrations/queries → Go platform packages → domain packages (pure first) → services → handlers → `cmd` mains → TS packages → web features → web routes → tests → Docker/CI/docs.

1. **Phase 1 — MVP:** repo tooling, compose, Makefile, CI; full schema migration; sqlc config + queries for users, sessions, email_tokens, workspaces, members, domains, qr_codes, qr_versions, audit_logs, idempotency_keys; `platform/*`, `apierr`, `config`, `auth` (password, magic link, Google, sessions, CSRF), `rbac`, `workspace`, `entitlements`, `shortcode`, `urlsafety` (validation + local safety fake), `qr` (+ content encoders), `version` (create/list, immediate only), `resolve`, `scan` (event + visitor hash + salt), `audit`, `idempotency`, `email`; `cmd/api`, `cmd/redirect`, `cmd/migrate`; `packages/qr-render` complete; `apps/render`; web: landing generator, SEO type pages, auth pages, app shell, QR list, new QR, QR detail (Overview placeholder cards, Settings basic), edit-destination dialog; tests for all of the above.
2. **Phase 2 — Analytics:** `cmd/ingest`, `scan/botdetect`, `scan/uaparse`, `ingest`, `analytics`, `realtime`, `cmd/worker` with River + `PartitionMaintenance` + `Reconcile`; analytics endpoints; web analytics page, QR Overview/Audience tabs, list sparklines, overview widgets, live badge; tests.
3. **Phase 3 — Dynamic depth:** scheduled versions + `ActivateVersion`, restore/cancel, password/expiry/limit/fallback/UTM, `routing` + rules UI + resolve-preview, hosted pages (`/public/hosted/{code}`, `/p/[code]`, redirect proxy, revalidation), custom domains (`domains` + Cloudflare client + `DomainVerify`), Web Risk checker + `SafetyRecheck`, preview page, abuse reports + staff queue, files upload + render rasterisation; tests.
4. **Phase 4 — Premium dashboard:** team/invites/roles/transfer, folders/tags/campaigns/templates (locked), bulk create/download + jobs UI, exports, billing (provider interface, Stripe, Razorpay, webhooks, trial, downgrade → read-only), API keys + key auth + limits + `/docs/api` page (Scalar), webhooks (SSRF-safe client, delivery worker, UI), audit log UI, command palette, onboarding checklist, pricing page; tests.
5. **Phase 5 — Hardening:** RLS policies migration + `SET LOCAL app.workspace_id`, TOTP 2FA, sessions UI, free-tier short domain, disposable-email list, velocity limits, `SafetyRescan`, auto-block, Fly configs (`deploy/fly/*.toml` for web, api, redirect, ingest, worker, render), Grafana dashboard JSON + alert rules in `deploy/observability/`, k6 scripts, `cmd/migrate-v1` importer (v1 tables `users`, `qr_records` → v2; bcrypt verify-and-rehash on first login).
6. **Phase 6 — Launch assets:** sitemap/robots/OG images, docs pages (`/docs/*` MDX: getting started, how we count, API, webhooks, custom domains, printing guide), legal pages, email templates (EN + HI), feature flags (`FEATURE_*` env), product funnel events.

## 16. FORBIDDEN

Storing or logging IP addresses; `localStorage`/`sessionStorage` for tokens (only `sessionStorage` for the anonymous draft); `301` redirects for dynamic codes; caching redirect responses; string-concatenated SQL; ORMs; global mutable state; `panic` for control flow; `any`/`interface{}` in exported Go signatures where a type is known; `any` in TypeScript (use `unknown` + zod); inline styles in React except CSS variables; client components for marketing pages beyond the generator island; new dependencies not listed in §1 without a `// DECISION:` comment; placeholder or partial files; prose output.

## 17. START

Begin Phase 1 now. First file: `docs/manifest/phase-1.txt`.
