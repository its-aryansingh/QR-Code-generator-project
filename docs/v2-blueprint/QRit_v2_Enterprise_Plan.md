# QRit v2 — Enterprise Implementation Plan

**Every enterprise feature working end to end, built on the v2 Go architecture**

| | |
|---|---|
| Prepared | 26 September 2026 |
| Builds on | `QRit_v2_Blueprint.md` (v2 architecture, Phases 1–4 are prerequisites) |
| Decision recorded | Target = v2 Go blueprint (chosen 26 Sep 2026). v1 Django is the feature reference only |
| Companion files | `GEMINI_PROMPT_ENTERPRISE.md` (paste after `GEMINI_PROMPT.md`), `schema_enterprise.sql` (migration 00003), `enterprise_queries.sql`, `tests/test_enterprise.py` |
| Verification | Migration and every enterprise query executed on PostgreSQL with fixtures. **38/38 automated checks pass**, including on a database already holding Phase 1–4 data and on an empty one (§8.3) |

**What this document adds to the blueprint.** The blueprint deferred SSO/SCIM "until a paying customer asks" and kept enterprise governance thin. This plan replaces that with a complete enterprise layer:

- an organization tenant above workspaces;
- SAML/OIDC single sign-on (SSO), SCIM provisioning and MFA;
- permission-based roles with folder scope;
- enforced security policies and maker–checker approvals;
- a tamper-evident audit log with SIEM (security monitoring) streaming;
- integrations, alerts and scheduled reports;
- consent-aware lead capture and retargeting pixels;
- a GS1-conformant resolver and serialized product authentication;
- GST-compliant enterprise billing, white-label and agency accounts;
- a developer platform and print-grade output;
- a compliance program (SOC 2 Type II, DPDP, GDPR).

**Reading order.** §1–4 explain what "top 1%" means and why v1 fell short; §5–7 specify the design; §8 is the verified schema; §9 is the build order; §10–12 cover compliance, packaging and risks; §13 is the Gemini handoff.

---

# 1. Executive summary

## 1.1 What "top 1%" means here, measurably

An enterprise buyer (a retail brand, bank, hospital network, FMCG company or agency) runs procurement against a security questionnaire and a feature checklist. They then roll the product out to hundreds of users and thousands of printed codes. "Top 1%" means passing that process without exceptions, and then outperforming competitors in use:

| Buyer requirement | Market today (§3) | QRit target |
|---|---|---|
| SSO | Enterprise-tier SAML at a few vendors; often a paid add-on ("SSO tax") | SAML **and** OIDC, **enforced** SSO with break-glass accounts, just-in-time (JIT) provisioning, IdP group → role mapping |
| User lifecycle | No leading QR vendor publicly documents SCIM | **SCIM 2.0** that passes Microsoft Entra's validator and Okta's SCIM tests; deprovisioning revokes access within one sync cycle |
| Access control | Fixed roles (admin/editor/reviewer) | Permission-based custom roles, group bindings, folder-scoped sharing, an "explain access" inspector, quarterly access-review export |
| Change control | Rare | Maker–checker approvals, with the four-eyes rule (no approving your own change) enforced **in the database** |
| Audit | Activity lists | Hash-chained, append-only audit log; daily write-once (WORM) anchors; verification endpoint; SIEM streaming (Splunk, Datadog, S3, webhook) |
| Security policy | Stored settings (v1) | Every policy has a named enforcement point, a stable error code and a test (§7) |
| Compliance | SOC 2 / GDPR claims | SOC 2 Type II program, DPDP-ready consent tooling (India), GDPR data-subject request (DSAR) tooling, VPAT, pen test, trust center |
| Retail 2D barcodes | GS1 link generation | A **GS1-conformant resolver** (Release 1.2.1, Aug 2026): link types, linksets, qualifier inheritance, resolver description file |
| Brand protection | Blog-level "authentication" | Serialized codes with message authentication codes (MACs) built in, verdicts that can't be spoofed, multi-country/velocity anomaly flags, and alerts |
| Marketing stack | Pixels, Zapier | Consent-aware pixels (Google Consent Mode v2 signals), native HubSpot/Salesforce/Sheets/Slack/Teams, GA4 Measurement Protocol, warehouse export |
| Billing | Card subscriptions | Annual contracts, PO numbers, **GST-compliant invoices** (financial-year numbering, CGST/SGST/IGST, export under LUT), seat true-ups |
| Reliability | 99.9–99.99% uptime claims | 99.95% contractual SLA with monthly per-org SLA reports; printed codes never deactivated (blueprint D7) |

## 1.2 Key decisions

| # | Decision | Why |
|---|---|---|
| ED1 | **Organization becomes the enterprise tenant.** Workspaces sit inside an org. Plan, subscription, SSO, SCIM, security policy, audit chain and billing live at org level | Enterprises have many teams (brands, regions) under one contract and one identity provider. Migration 00003 backfills one org per existing workspace, so nothing breaks |
| ED2 | **SAML through Ory Polis** (Apache-2.0, self-hosted), which converts SAML into an OIDC flow. Direct OIDC connections use `go-oidc`. **SCIM is implemented natively in Go** | SAML signature/XML parsing is the historic source of SSO bypass bugs (e.g. CVE-2022-41912 in Go's main SAML library). Delegating it to a dedicated, maintained service removes that risk without per-connection fees. SCIM is plain REST and sits close to our data model. WorkOS (≈$125 per connection per month for SSO, the same again for SCIM) stays a documented drop-in behind the `identity.Provider` interface |
| ED3 | **Authorization is permission-based.** 5 system roles + custom roles; bindings to users or groups; workspace or folder scope; one `authz.Can()` used by every handler | v1 had a 4-rank ladder, so "reviewer", folder sharing and custom roles were impossible |
| ED4 | **Approvals gate versions, not codes.** A pending version exists but is not *eligible* for resolution until approved. The redirect query filters on `approval_status` | The printed code keeps serving the last approved destination the whole time; no special "frozen" state |
| ED5 | **Audit chain sealed asynchronously.** Business transactions insert audit rows without locking. A worker seals them into a per-org SHA-256 chain every 5 s; daily anchors go to WORM storage | Tamper evidence with no write contention. Verified: direct DB edits with triggers disabled are detected |
| ED6 | **Serial verification is synchronous and separate from scan analytics.** The verify page's JavaScript calls `POST /public/verify/{serial}`; analytics still flow through the stream | Link-preview bots don't execute JavaScript, so they can't burn scan counts. The verdict is authoritative at the moment the page renders |
| ED7 | **Pixels only through a consent-aware interstitial**, opt-in by default in every region | EEA remarketing requires Consent Mode v2 signals, and India's DPDP Rules (full compliance 13 May 2027) require consent for personal-data processing. The ~300–800 ms cost is disclosed and measured |
| ED8 | **Everything enterprise is entitlement- and flag-gated**, with contract overrides at org level | Sales can close custom deals without code changes |

## 1.3 Timeline

Enterprise delivery (E0–E8, §9) is **≈ 21 engineering weeks** after blueprint Phases 1–4, for one developer using Gemini with review, plus a **SOC 2 Type II observation window of at least 3 months** that runs in parallel from E2. First enterprise-sellable milestone ("Enterprise Core": org, SSO, SCIM, roles, approvals, policies, audit, contracts) arrives after **E0–E2 plus the billing part of E6, ≈ 8 weeks**.

---

# 2. v1 enterprise audit: what exists and why it doesn't work

v1 (Django) has endpoints and UI for most enterprise features, and its design notes are thoughtful. The failure pattern is consistent: **settings are stored but nothing enforces them, or the protocol side was never built.** Findings from reading `governance.py`, `auth.py`, `rbac.py`, `audit.py`, `ws_integrations.py`, `bulk.py`, `workspaces.py`, `redirect.py` and the dashboard pages:

| v1 feature | What v1 does | Root cause | Severity | v2 enterprise fix (section) |
|---|---|---|---|---|
| SAML/OIDC SSO | Stores entity ID, SSO URL, cert; shows `acs_url = …/auth/sso/{ws}/acs` | **No ACS route exists** (the IdP posts to a 404); no SAML parsing, no login flow; `enforce_sso` read nowhere | Blocker | Polis bridge + OIDC, enforced at org access (§6.1) |
| SCIM | Generates a bcrypt-hashed token | **No `/scim/v2` endpoints at all** | Blocker | Native SCIM 2.0 (§6.1.6) |
| Security policy | Saves require_approval, session timeout, password length, scan alert threshold | **None of those fields are used anywhere** (grep: only the serializer reads them) | High | Enforcement matrix (§7) |
| Destination allow/deny list | `check_destination()` called on create/rules | Works, but no approval path and no punycode/IDN handling | Medium | `urlsafety` + approvals (§6.3) |
| Custom domains | TXT or HTTP-file verification, then `ssl_status="active"` | No certificate is issued; the redirect only serves `/r/<code>` on the API host (no Host routing); **HTTP verification fetches any user-supplied host → SSRF** (e.g. an internal IP) | Blocker + security | Cloudflare for SaaS + Host routing (blueprint §4.5); SSRF-safe client |
| Workspace API keys | `qk_<prefix>.<secret>` with bcrypt (cost 12) | Accepted by **one** endpoint (`qr/api/generate`); all workspace routes are JWT-only; bcrypt on every request (~250 ms); scopes never checked | High | API platform (§6.11) |
| Roles | owner > admin > editor > viewer ladder | No reviewer, no custom roles, no groups, no folder scope | Medium | Permission RBAC (§6.2) |
| Audit log | Writes rows after the change, swallowing errors | Outside the transaction (lost on failure); API-key actions never logged; FK `ON DELETE CASCADE` on users **deletes a user's audit history**; editable | High | Tamper-evident chain (§6.4) |
| Webhooks | Sent from the scan thread | Synchronous `urllib` to arbitrary URLs (SSRF); lost on restart; no retries/backoff | High | River delivery + SafeClient (blueprint §8.6) |
| Lead pages | Public form, stores email + data | No rate limit, no captcha, plain-text PII, no retention, no withdrawal, consent not enforced | High (DPDP/GDPR) | Encrypted forms + consent (§6.6) |
| Branding / white-label | Stores `custom_css`, `custom_footer` | Raw CSS injection surface; no custom dashboard domain or email domain | Medium | Tokenised theming, org branding (§6.10) |
| Reports | CSV of user-scoped data, built synchronously | Not workspace-scoped; unbounded memory; no scheduling | Medium | Jobs + scheduled reports (§6.5) |
| Bulk (legacy) | `BulkGenerateView` generates 100 codes per call | **No authentication decorator**, so it's a public CPU-exhaustion endpoint | High | Authenticated jobs (§6.11) |
| Invites | Creates a token, returns the link to the admin | **Never emailed**; token stored in plain text | Medium | Email + hashed tokens (blueprint) |
| GS1 Digital Link | Builds DL URIs | **No resolver route**; printed GS1 codes resolve nowhere | Blocker for GS1 | Conformant resolver (§6.8) |
| Templates / Security pages | Frontend pages | `templates/page.tsx` and `security/page.tsx` are placeholders linking elsewhere | Low | Real pages |

**Lesson carried into v2:** no enterprise setting ships without a named enforcement point, a negative test proving the enforcement, and an audit entry (§4.1 Definition of Done).

---

# 3. Market research: the enterprise bar in September 2026

## 3.1 What the leaders publicly offer

| Capability | Uniqode (enterprise leader) | Bitly Enterprise | QR TIGER | Flowcode | Hovercode Business+ |
|---|---|---|---|---|---|
| SSO | SAML on Enterprise; SSO on Business+ ($399/mo yearly) or as an add-on | "SSO and 2-factor authentication" | — (not found) | IdP sync mentioned, protocol unnamed | — |
| SCIM | Not documented | Not documented | — | Not documented | — |
| Roles / org structure | Editor, reviewer, administrator; multiple teams; per-sub-account feature toggles | Teams, groups, admin controls | Users per plan | Enterprise | Up to 10 workspaces |
| Compliance | SOC 2, HIPAA, GDPR | GDPR, CCPA | GDPR/CCPA claims | GDPR | — |
| Integrations | Zapier, Canva, Workato, Google Ads, Slack, Meta Pixel ("1,000+") | Webhooks, 800+ integrations | Zapier, HubSpot, GA, Canva | CRM/first-party data | Webhooks |
| Bulk / API | 2,000 codes per bulk; 2 M API requests/month | "Millions of links and QR codes" | Bulk 3,000/batch | Bulk via API | Bulk + API |
| Retargeting | Meta Pixel (browser); server-side via add-on; data synced every 24 h | — | Retargeting pixels | — | — |
| Anomaly detection | "Identifies inconsistencies in user scan behavior" | — | — | — | — |
| Uptime | "99.9% in last 12 months", 24×7 support | "99.99% over 12 years", 10 B+ clicks/month | — | — | — |
| Price signal | Business+ $399/mo (yearly) | ~$10k+/year, custom | Up to $37/mo self-serve | $250+/mo Growth | $99/mo |

## 3.2 Gaps we can own

1. **SCIM.** A September 2026 comparison of QR generators with SSO found SAML named publicly only by Uniqode, and SCIM by none. Automatic deprovisioning is what IT security teams actually require for offboarding. Shipping Entra- and Okta-validated SCIM is a clear lead.
2. **Enforced SSO without an "SSO tax".** Enforcement (not just a login button) is what protects codes when contractors leave. Include SSO + SCIM in Enterprise instead of selling them as $500+/year add-ons.
3. **Verifiable audit.** Competitors show activity feeds; a hash-chained, independently verifiable log with SIEM streaming answers "prove nobody changed this destination" in regulated industries.
4. **GS1 conformance for Sunrise 2027.** Most vendors generate GS1 Digital Link URIs; a resolver conformant to the GS1 standard (Release 1.2.1, ratified August 2026) is what retailers and brand-owner IT teams check.
5. **Real product authentication.** Vendors describe serialized codes and anomaly rules; few document codes with a built-in MAC (unforgeable without the key) plus verdicts that bots can't inflate.
6. **India-native enterprise billing.** GST-compliant invoices, UPI/NEFT payment, and export invoices under LUT. Global vendors invoice from abroad, which complicates Indian input-tax credit.

## 3.3 Build vs buy for identity

| Option | Cost at 20 enterprise customers | Risk | Verdict |
|---|---|---|---|
| WorkOS (SSO + Directory Sync) | ≈ 20 × ($125 + $125) = **$5,000/month** at list tier (volume tiers go down to $50/connection) | Low | Documented alternative; pass-through pricing needed |
| Ory Polis self-hosted (SAML→OIDC) + native SCIM + direct OIDC | Hosting only (one small container + Postgres schema) | Medium: we operate Polis and must pass conformance tests | **Chosen** |
| Hand-written SAML in Go (crewjam/saml or gosaml2) | Zero licence | **High**: XML signature wrapping and assertion-confusion bugs (e.g. CVE-2022-41912 auth bypass in crewjam/saml) | Rejected |

---

# 4. Scope and the definition of "production-ready"

## 4.1 Definition of Done (applies to every enterprise feature)

A feature is production-ready only when **all** of these are true:

1. **Enforced server-side** at a named enforcement point (handler, middleware, redirect, worker or DB constraint). UI state is never the control.
2. **Authorized** through `authz.Can()` with a catalogued permission; cross-tenant access returns 404.
3. **Audited** in the same transaction with a catalogued action name (§6.4.2).
4. **Entitlement-gated** (`402 upgrade_required`) and **feature-flagged** (can be disabled per org without a deploy).
5. **Idempotent** for creates (`Idempotency-Key`) and **rate-limited** where public.
6. **Specified in `openapi.yaml`** and documented on the docs site with an example.
7. **UI complete:** loading skeleton, empty state, error state (maps problem `code` to copy), locked/upgrade state, keyboard-accessible.
8. **Observable:** Prometheus counter/histogram, structured log fields, alert if on a critical path.
9. **Tested:** unit tests, integration tests with real Postgres/Redis (testcontainers), at least one **negative/security test proving the enforcement**, and an e2e happy path.
10. **Reversible:** migration has a Down; the feature can be switched off without data loss.

## 4.2 Scope tiers

| Tier | Modules | Why |
|---|---|---|
| **T1 — Enterprise Core** (must-have to sell) | Organizations, SSO (SAML/OIDC, enforced, JIT), SCIM, MFA, session/IP policies, permission RBAC + groups + folder scope, approvals, workspace content policies, tamper-evident audit + export + SIEM, contracts + GST invoicing, SLA + status, DPA/DSAR, SOC 2 program | These decide procurement |
| **T2 — Differentiators** | GS1 conformant resolver, serialization + authentication, anomaly alerts + scheduled reports, integrations (Slack, Teams, Zapier/Make, HubSpot, Salesforce, Sheets, GA4, warehouse), consent-aware lead forms, consent-aware pixels, white-label dashboard + email domain, agency client orgs, API platform (usage, sandbox, SDKs), print-grade output (CMYK PDF, EPS, label sheets), bulk 100k + update mode | These win head-to-head evaluations |
| **T3 — Later (designed, not built)** | Regional data planes (EU/US residency), Canva/Figma/Adobe plugins, NFC tags, SSO for hosted-page viewers (gated content), HIPAA/BAA | Build when a signed customer requires it; §6.12 records the design so nothing blocks it |

---

# 5. Architecture changes

## 5.1 Updated topology

```
                          Cloudflare (WAF · TLS · Cloudflare for SaaS: short domains, GS1 domains, white-label app hosts)
                                   │                         │                              │
                   example.com / app.agency.com      qr.example.com / go.brand.com     api.example.com
                                   ▼                         ▼                              ▼
                          ┌────────────────┐      ┌──────────────────────┐      ┌─────────────────────────────┐
                          │ web (Next.js)  │      │ redirect (Go)        │      │ api (Go)                    │
                          │ branding by    │      │ /{code} /V/{serial}  │      │ /v1 … + /scim/v2 …          │
                          │ Host; /p, /v   │◀─────┤ /01/{gtin}… (GS1)    │      │ authz · approvals · policies│
                          │ pages          │      │ pixel interstitial   │      │ identity (OIDC RP)          │
                          └────────────────┘      └──────────────────────┘      └───────┬───────────┬─────────┘
                                                                                        │ OIDC code │ admin API
                                                                                        ▼           ▼
                                                                                ┌───────────────────────────┐
                                                        customer IdP ◀── SAML ──│ sso-bridge (Ory Polis)    │
                                                     (Okta, Entra, Google, …)   │ SAML ⇄ OIDC, own schema   │
                                                                                └───────────────────────────┘
  worker (River): audit sealer & anchors · SIEM streams · integrations fan-out · alerts · scheduled reports ·
                  approvals expiry · serial generation · GS1 import · warehouse export · invoices · retention · DSAR
  ingest: unchanged pipeline (+ serial column)          render: + CMYK PDF, EPS (Ghostscript), label sheets, report PDFs
  Postgres: + schema_enterprise (00003) and `polis` schema     Redis: + authz cache, policy cache, pixel counters
```

**New deployable:** `sso-bridge` (the official Ory Polis Docker image, pinned version), private network only, except its SAML ACS endpoint (`/api/oauth/saml`), which is exposed at `sso.example.com`. It uses the same Postgres cluster in a separate schema and database role. Everything else is new packages in the existing Go module and new routes in `web`.

## 5.2 Tenancy model

```
organization (plan, contract, SSO, SCIM, security policy, audit chain, branding, billing, data_region)
 ├── org_members (org_owner | org_admin | billing_admin | member; status active/suspended/deprovisioned)
 ├── groups (manual | scim | sso) ── group_members
 ├── roles (system + custom permission sets)
 ├── workspaces (brand/region/team; workspace_policies; is_sandbox)
 │     └── role_bindings (user|group → role, workspace or folder scope)
 └── child organizations (agency → client orgs)
```

- **Request context:** every authenticated request resolves `principal → org → workspace` once, in middleware:
  - load the org security policy (Redis-cached 60 s, invalidated on change);
  - apply the identity gates (SSO enforcement, MFA, session idle/max, IP allowlist) *before* any handler runs;
  - attach `authz.Context`.
- **Org admins:** `org_owner` and `org_admin` have implicit `admin` on every workspace in their org (documented and audited, and visible in the access inspector). Agency `org_admin`s have implicit `admin` on client orgs; their actions are audited in the **client's** audit chain with `actor_type='user'` plus `changes.via_agency=<agency org id>`.

## 5.3 Authorization engine (`internal/authz`)

```go
type Principal struct { Kind string /* user|api_key|staff */; UserID, APIKeyID uuid.UUID; OrgID uuid.UUID; Scopes []string; SupportGrant *Grant }
func (e *Engine) Can(ctx context.Context, p Principal, perm Permission, res Resource) (Decision, error)
type Resource struct { WorkspaceID uuid.UUID; FolderID *uuid.UUID /* nil = workspace-level */ }
type Decision struct { Allowed bool; Via []Binding /* for the access inspector */; Reason string }
func (e *Engine) VisibleFolders(ctx context.Context, p Principal, ws uuid.UUID, perm Permission) (all bool, folderIDs []uuid.UUID, err error)
```

- **Resolution:**
  1. Load `EffectivePermissions` (verified query, §8.2), cached in Redis under `authz:{ws}:{user}` for 30 s. It is invalidated via pub/sub `authz:invalidate` on any binding, group-membership, role or org-member change.
  2. If the permission is granted workspace-wide (`folder_id IS NULL`) or via `*`, allow.
  3. Otherwise, if the resource has a folder, allow when any granted folder is in `FolderAncestors(resource.folder)`.
- **Listing:** list endpoints call `VisibleFolders`. If the grant is not workspace-wide, the list query gets `folder_id = ANY($descendants)`, where descendants are expanded with a recursive CTE from the granted folders. Users with folder-only access never see other folders' codes, counts or analytics.
- **API keys:** scopes map to permissions: `qr:read → qr.read`, `qr:write → qr.create, qr.update, qr.destination.update`, `analytics:read → analytics.read`, `webhooks:write → webhook.manage`, `leads:read → lead.read`. A key never exceeds the permissions of an `editor`, and it can't approve.
- **Staff:** only through an active `support_access_grants` row (§6.12). `read` grants map to `analyst` + `audit.read`; `read_write` grants map to `admin` minus `billing`, `sso`, `policy` and `apikey`.
- **Permission catalogue** (stable strings; UI groups them):
  - `workspace.read|update`, `member.manage`, `role.manage`
  - `qr.read|create|update|delete`, `qr.destination.update|approve`, `qr.design.bypass_lock`
  - `folder.manage`, `campaign.manage`, `template.manage`
  - `analytics.read|export|raw`
  - `domain.manage`, `apikey.manage`, `webhook.manage`, `integration.manage`, `policy.manage`
  - `audit.read|export`
  - `form.manage`, `lead.read|export`, `pixel.manage`, `alert.manage`, `report.manage`
  - `serial.manage`, `gs1.manage`, `bulk.run`
  - org-level (held via `org_role`): `org.manage`, `org.billing`, `org.sso`, `org.scim`, `org.policy`, `org.audit`, `org.members`, `org.branding`, `org.clients`

## 5.4 Domain events and fan-out

Every mutation that other systems care about emits a **domain event**. It is inserted as a River job (`FanOut{event}`) **in the same transaction** as the change, which makes the job table a transactional outbox. The `FanOut` worker delivers to:
- the webhook endpoints subscribed to that event type (blueprint §6.5 signing);
- integrations subscribed to it (§6.5);
- alert evaluation for `security_event` rules.

Audit streaming is separate: it reads the sealed chain by `seq`, so it is ordered and gap-free.

**Event catalogue (v1):**
- **QR:** `qr.created`, `qr.updated`, `qr.archived`, `qr.deleted`, `qr.version.created`, `qr.version.activated`
- **Approvals:** `approval.requested`, `approval.decided`, `approval.expired`
- **Scans and leads:** `scan.created` (Business+, sampled to ≤ 50/s per workspace, with a daily aggregate event beyond that), `lead.created`, `lead.withdrawn`
- **Alerts:** `alert.fired`
- **Serials:** `serial.flagged`, `serial_batch.ready`
- **People and policy:** `member.added`, `member.removed`, `member.role_changed`, `sso.login`, `scim.user.provisioned`, `scim.user.deprovisioned`, `policy.updated`
- **Other:** `apikey.created`, `export.completed`

Payload envelope: `{id, type, created_at, org_id, workspace_id, actor{type,id}, data{…}}`, the same shape everywhere.

## 5.5 Key flows

**SSO login (SAML via Polis, SP-initiated):**
1. The login page asks for an email first → `POST /v1/auth/sso/start {email}` → the domain is looked up in `org_domains` (verified).
2. If the org has an active SAML connection, respond with Polis's `/api/oauth/authorize` URL (tenant = org slug, product = `qrit`, PKCE, `state` stored in Redis for 10 min).
3. The browser goes to Polis → IdP → back to Polis's ACS. Polis redirects to `https://api.example.com/v1/auth/sso/callback?code=…&state=…`.
4. The API exchanges the code at Polis `/api/oauth/token` and calls `/api/oauth/userinfo` → profile `{id, email, firstName, lastName, groups[], requested{tenant}}`.
5. **Identity linking** (in order):
   - `user_identities (connection, subject)` → that user;
   - else an existing user with this email *and* the email domain is **verified by this org** → link;
   - else, if JIT is on → create the user (email verified), `org_members(source='sso_jit')`, and bind `default_role_key` in `default_workspace_id`;
   - else → `403 sso_user_not_provisioned`.
   It never links by email for unverified domains, which prevents account takeover through a rogue IdP.
6. Sync IdP groups: map to `groups(source='sso')` by `display_name`, and add/remove memberships to match the claim.
7. Create the session with `auth_method='sso'` and `sso_connection_id`, audit `auth.sso.login`, and set cookies.

**OIDC direct connections** skip Polis. `go-oidc` does discovery, PKCE, nonce and `iss`/`aud`/`exp` checks, and `email_verified` must be true.

**SSO enforcement** is per org, at the access gate: when `enforce_sso` is on, any session whose `auth_method <> 'sso'` (or whose SSO connection belongs to another org) is refused for that org's resources with `403 sso_required` + `login_url`. The exception is `sso_break_glass_user_ids` (≤ 2 owners, MFA mandatory, every use alerts all org admins).

**Approval (single change):**
1. The editor submits a new destination → `approval.Evaluate(policy, change)` returns reasons.
2. If there are reasons: in one transaction, insert `approval_requests` + `qr_versions(approval_status='pending', approval_request_id)`, audit `approval.requested`, and enqueue `FanOut(approval.requested)`.
3. Approvers see it in their inbox (`PendingApprovalsForApprover`) and are notified by email/Slack/Teams with deep links.
4. Each decision is an `approval_decisions` row; the DB trigger enforces four-eyes and pending status.
5. When approvals ≥ required → `FinalizeApproval` (verified) → cache invalidation → the new destination is live within 1 s. A reject marks the version `rejected`, notifies the requester with the comment, and the code keeps serving the last approved version throughout.
6. Expiry job: pending past `expires_at` → `expired`, versions `cancelled`.
7. Break-glass publish: an `org_owner` with a mandatory `override_reason`, step-up authentication, audited, and all approvers notified.

**Audit sealing:**
- Handlers insert `audit_logs` rows in the business transaction.
- `AuditSealer` (every 5 s) runs `SELECT DISTINCT org_id FROM audit_logs WHERE hash IS NULL` and calls `audit_seal(org, 1000)` for each; the function takes a per-org advisory lock.
- `AuditAnchor` (daily, 00:10 UTC) writes `{org_id, day, last_seq, head_hash}` to `audit_anchors` **and** to a WORM bucket `qrit-audit-worm` (Cloudflare R2 bucket lock or S3 Object Lock in compliance mode; retention = plan audit retention). It also emails the org's security contact a signed daily digest when configured.
- `GET /orgs/{org}/audit-logs/verify` runs `audit_verify` and compares the chain against the stored anchors, reporting `intact | broken_at_seq N | anchor_mismatch day D`.

---

# 6. Module specifications

Each module lists: data, API, behaviour and enforcement, UI, jobs, tests and acceptance criteria. Table names refer to migration 00003 (§8.1). Every endpoint follows blueprint §6.1 conventions: problem+json, cursor pagination, Idempotency-Key on creates, and `If-Match` on policy PUTs.

## 6.1 Identity: org domains, SSO, MFA, sessions, SCIM (T1)

### 6.1.1 Organizations and claimed domains

- **API:**

  | Method | Path | Permission |
  |---|---|---|
  | GET | `/v1/orgs` | member |
  | GET / PATCH | `/v1/orgs/{org}` | member / `org.manage` |
  | GET | `/v1/orgs/{org}/members` | `org.members` |
  | PATCH / DELETE | `/v1/orgs/{org}/members/{userId}` (org role, suspend) | `org.members` |
  | GET / POST | `/v1/orgs/{org}/workspaces` | member / `org.manage` |
  | GET / POST | `/v1/orgs/{org}/domains` | `org.sso` |
  | POST | `/v1/orgs/{org}/domains/{id}/verify` | `org.sso` |
  | DELETE | `/v1/orgs/{org}/domains/{id}` | `org.sso` |

- **Domain claim:** a TXT record `_qrit-challenge.<domain>` = `qrit-domain-verification=<token>`, checked with a DNS-over-HTTPS query to Cloudflare `1.1.1.1` (no outbound fetch to the customer's host, so there is no SSRF surface). Polled every 10 min for 72 h. Public-mail domains (gmail.com, outlook.com, yahoo.com, … from a bundled list of ~3,000) can't be claimed.
- **Auto-join:** users who verify an email on a claimed domain can join `auto_join_workspace_id` with the `analyst` role. This is off by default, and audited.

### 6.1.2 SSO connections

- **API:** `GET/POST /v1/orgs/{org}/sso-connections`, `GET/PATCH/DELETE …/{id}`, `POST …/{id}/test` (returns a test-login URL; on success the connection moves `draft → testing → active` with the tester's claims shown for confirmation), plus `POST /v1/auth/sso/start` and `GET /v1/auth/sso/callback`.
- **SAML setup UX:**
  - The admin picks the IdP: Okta, Entra ID, Google Workspace, JumpCloud, OneLogin, PingFederate or "Custom SAML". Each has a step-by-step guide with screenshots, our ACS URL (`https://sso.example.com/api/oauth/saml`) and SP entity ID.
  - The admin uploads IdP metadata XML or a metadata URL; the API creates the Polis connection through the Polis admin API.
  - Attribute mapping has defaults per IdP.
- **OIDC setup:** issuer URL + client id/secret (the secret is stored encrypted as `oidc_client_secret_ct`), and we show our redirect URI.
- **Guardrails:**
  - Only `active` connections can be enforced.
  - Enabling enforcement requires at least one successful test login in the last 24 h and at least 1 break-glass owner with MFA.
  - Disabling a connection while enforced is refused (`409 sso_enforced`).
- **Step-up re-authentication** (§6.1.4) is required for any change to connections or enforcement.

### 6.1.3 MFA

- **Factors:**
  - TOTP (`github.com/pquerna/otp`, 30 s, 6 digits, ±1 step window, secret encrypted).
  - WebAuthn/passkeys (`github.com/go-webauthn/webauthn`, resident keys allowed, user verification preferred).
  - 10 single-use recovery codes (sha256-hashed).
- **API:**
  - `GET /v1/me/mfa`
  - `POST /v1/me/mfa/totp` → `{secret, otpauth_uri}`, then `POST /v1/me/mfa/totp/confirm {code}`
  - `POST /v1/me/mfa/webauthn/register/begin|finish`
  - `DELETE /v1/me/mfa/{id}` (step-up; can't remove the last factor while the org requires MFA)
  - `POST /v1/me/mfa/recovery-codes` (regenerate, shown once)
  - login second step `POST /v1/auth/mfa/verify {totp|webauthn|recovery}` (with `/v1/auth/mfa/webauthn/begin`)
- **Login flow:** after the password/magic link succeeds, if the user has a factor **or** any of their orgs requires MFA, the session is created with `mfa_verified_at = NULL`. Only `/auth/mfa/*` and `/me/mfa/*` are allowed until verification. SSO sessions rely on the IdP's MFA (documented).
- **Rate limits:** 5 MFA attempts per 15 min per session, 20 per user per day, then the account is locked with an email notice.

### 6.1.4 Sessions, IP allowlists and step-up authentication

- **Session idle / max timeouts** (org policy): enforced
  - at `POST /auth/refresh` (reject if `now − last_used_at > idle` or `now − created_at > max`), and
  - in the access middleware, which checks the session row (cached 60 s) on every request.
  The strictest policy among the user's active orgs applies to that org's resources only.
- **IP allowlists:**
  - `dashboard_ip_allowlist` applies to cookie-authenticated requests to that org's resources; `api_ip_allowlist` to API keys. Individual keys can narrow this further with `api_keys.ip_allowlist`.
  - The client IP comes from `CF-Connecting-IP`, trusted only from Cloudflare ranges.
  - Saving an allowlist that excludes the admin's current IP is refused (`422 would_lock_out`).
  - Break-glass owners are exempt only with MFA.
- **Step-up:** `POST /v1/auth/step-up` (password + MFA, or SSO re-authentication with `prompt=login` / SAML `ForceAuthn`) sets `sessions.step_up_at`. Required within the last 10 min for:
  - SSO, SCIM and security-policy changes;
  - API key creation;
  - billing changes;
  - raw scan / lead exports;
  - audit stream creation;
  - workspace/org deletion;
  - break-glass publish.
  Without it the API returns `401 step_up_required`, and the UI opens the re-auth dialog and retries.

### 6.1.5 Offboarding guarantees

When an `org_members` row becomes `suspended` or `deprovisioned` (SCIM, admin, or removed from the IdP group):
1. Revoke all of the user's sessions for that org (the family revoke also covers other orgs if the user was SCIM-created — i.e. an org-managed identity).
2. Invalidate the authz cache.
3. Revoke API keys the user created (optional by policy; the default is **transfer ownership to the org, keep the key**, so integrations don't break).
4. Keep every QR code (codes belong to workspaces, never to users).
5. Audit `member.deprovisioned`.

The SLA is ≤ 60 s after the SCIM call.

### 6.1.6 SCIM 2.0 (native)

- **Base URL:** `https://api.example.com/scim/v2/` (the directory is identified by its bearer token `scim_<32 base32>`; sha256 lookup; `last_request_at` updated). TLS 1.2+ only (Cloudflare minimum TLS setting).
- **Discovery endpoints:** `/ServiceProviderConfig` (patch: true, filter: true (max 200), bulk: false, changePassword: false, sort: false, etag: true, authenticationSchemes: oauthbearertoken), `/ResourceTypes`, `/Schemas` (core User, enterprise User extension, Group).
- **Users:**

  | Operation | Behaviour |
  |---|---|
  | `GET /Users?filter=…&startIndex=1&count=100` | Filters `userName eq "…"`, `externalId eq "…"`, `emails[type eq "work"].value eq "…"`, combined with `and`; case-insensitive attribute names and operators |
  | `POST /Users` | 201 with `id` = `scim_users.id`; 409 `uniqueness` on duplicate `userName` |
  | `GET /Users/{id}` | 404 SCIM error when absent |
  | `PUT /Users/{id}` | Full replace |
  | `PATCH /Users/{id}` | `Operations` with op `Add\|Replace\|Remove` in any case; `path` may be absent (value object), a simple attribute, `emails[type eq "work"].value`, `name.givenName`, or `active`; boolean values accepted as JSON booleans **or** the strings `"True"`/`"False"` (an Entra quirk) |
  | `DELETE /Users/{id}` | 204 → `deprovision_action` (`suspend` or `remove` from org) |

  - `active=false` → org member `suspended` (§6.1.5); `active=true` → `active`.
  - Values are stored exactly as sent (`resource` jsonb, an Entra requirement). `userName` is the login email, and a verified-domain check links to existing users.
- **Groups:** `GET /Groups?filter=displayName eq "…"` (supports `excludedAttributes=members`), `POST` (empty members allowed), `GET`, `PUT`, `PATCH` (`members` add/remove by value, `displayName` replace; responds **204**), `DELETE`. `displayName` is unique per org (DB constraint). Group membership drives role bindings automatically.
- **Responses:** `Content-Type: application/scim+json`, `meta{resourceType, created, lastModified, location, version: W/"n"}`, `ListResponse` for every query including zero results, SCIM error schema `{schemas:[…Error], status, scimType, detail}`.
- **Idempotency and concurrency:** `If-Match` ETag honoured (412 on mismatch). Per-directory requests are serialized with an advisory lock around group-membership PATCHes, because Entra sends parallel requests.
- **Limits:** 600 requests/min per directory; 200 max `count`.

### Tests (6.1)
- **SAML:** CI starts a Keycloak container as a SAML IdP and runs a full login through Polis. Manual certification with Okta (developer org), Entra ID (free tenant), Google Workspace and JumpCloud before GA.
- **Account-takeover suite:**
  - a rogue IdP asserting `ceo@acme.com` for an unverified domain → refused;
  - an IdP-initiated login with no stored `state` → refused (IdP-initiated SAML is disabled by default);
  - a replayed Polis code → refused.
- **SCIM:** the Microsoft Entra SCIM Validator (all tests) and Okta's SCIM 2.0 spec tests, plus our own contract tests. Deprovision → sessions revoked within 60 s (integration test with a fake clock).
- **MFA:** RFC 6238 test vectors; WebAuthn using the `go-webauthn` virtual authenticator; lockout thresholds.
- **Policy gates:** each gate in §7 has a negative test.

### Acceptance (6.1)
- An Okta and an Entra tenant each log in via SAML, JIT-create a user in the right workspace with the right role from group mapping, and appear in the audit log.
- With enforcement on, a password login by a non-break-glass member gets `403 sso_required` for that org while still reaching their personal org.
- Entra SCIM Validator: 100% pass. Unassigning a user in Entra suspends them in QRit on the next provisioning cycle (Entra cycles every ~40 min; Okta pushes immediately) and revokes their sessions within 60 s of the call.

## 6.2 Access governance: roles, groups, folder sharing, access reviews (T1)

- **API:**
  - `GET /v1/permissions` (catalogue with groups and descriptions)
  - `GET/POST /v1/orgs/{org}/roles`, `PATCH/DELETE …/{id}` (system roles are read-only; a custom role can't be deleted while bound → 409)
  - `GET/POST /v1/orgs/{org}/groups`, `PATCH/DELETE …/{id}`, `POST/DELETE …/{id}/members` (SCIM groups are read-only in the UI)
  - `GET/POST /v1/workspaces/{ws}/role-bindings`, `DELETE …/{id}`
  - `GET /v1/workspaces/{ws}/access/explain?user_id=&permission=&qr_id=` → `{allowed, via:[{binding, role, scope, group?}]}`
  - `GET /v1/orgs/{org}/access-review` (CSV/JSON of every user × workspace × effective role × source × last login; step-up)
- **Guardrails:**
  - Can't remove the last `owner` binding of a workspace or the last `org_owner`.
  - An admin can't grant a permission they don't hold (no privilege escalation).
  - Custom roles can't contain `*`.
- **UI:**
  - *Roles* page: a permission matrix editor grouped by area, with a "compare roles" view.
  - *Access* tab per workspace: bindings table with principal chips (user/group) and a scope column.
  - *Share folder* dialog from the folder tree: pick users/groups + role.
  - *Explain access* drawer on any member row.
  - *Access review* page with a download button and "last reviewed by/at" (stored in org settings; SOC 2 evidence).
- **Tests:** privilege-escalation cases; folder-only user sees only their subtree in lists, search, analytics and exports; group removal → access removed within 30 s (cache invalidation).
- **Acceptance:** an "Agency Editor" custom role bound to the group "Agency" on folder "Campaigns/Diwali" can create codes only there, sees nothing else, and the inspector explains why.

## 6.3 Change control: approvals and content policies (T1)

- **Policy evaluation** (`internal/approval`, pure): `Evaluate(p WorkspacePolicy, c Change) (reasons []string)`. `Change` covers the new destination, all rule/split destinations, the create-vs-update kind, and the actor's permissions.

  | `approval_mode` | Needs approval when |
  |---|---|
  | `off` | never |
  | `outside_allowlist` | any destination host doesn't match `allowed_destination_hosts` → `host_not_allowlisted` |
  | `all_destination_changes` | any new version (destination/rules) → `policy_all_destination_changes` |
  | `all_changes` | also new dynamic codes (the code exists but serves the "not live yet" page until approved) → `policy_new_code` |

  `blocked_destination_hosts` is always a hard `422 host_blocked`, not an approval. `require_https` is a hard `422 https_required`.
- **API:**
  - `GET /v1/workspaces/{ws}/approvals?status=`
  - `GET /v1/me/approvals` (inbox across workspaces, `PendingApprovalsForApprover`)
  - `POST /v1/workspaces/{ws}/approvals/{id}/decisions {decision, comment}` (needs `qr.destination.approve`)
  - `POST …/{id}/cancel` (requester)
  - `POST …/{id}/override {reason}` (org_owner, step-up)
  - `GET/PUT /v1/workspaces/{ws}/policy` (`policy.manage`, If-Match)
- **Version create response:** `202 Accepted` with `{version, approval: {id, status: "pending", reasons, required_approvals, expires_at}}` instead of 201 when approval is required. The UI shows "Sent for approval" with the reasons in plain English.
- **Bulk:** a bulk-update job evaluates every row. Rows needing approval create pending versions linked to **one** `approval_requests(kind='bulk_update', job_id)`. The approver sees a diff table (code, current → proposed, reason) and approves or rejects the batch.
- **Notifications:** email + Slack/Teams (if integrated) to everyone with `qr.destination.approve` in the workspace. A reminder is sent at 24 h and an `approval_pending` alert fires at the policy's SLA.
- **UI:** an approvals inbox with a badge count in the sidebar; a diff view (old/new destination with host highlighted, Web Risk verdict, rule changes); approve/reject with comment; request history on the QR's Versions tab.
- **Tests:** four-eyes (DB trigger + service); the pending version never served (resolver test, verified §8.3); expiry; override audit; bulk approval with 1,000 rows; notification fan-out once per approver.
- **Acceptance:** in a workspace with `outside_allowlist`, an editor changes a destination to a non-allowlisted host. The code keeps redirecting to the old URL; the reviewer approves from a Slack deep link; the new URL is live within 1 s; the audit log shows request → decision → activation.

## 6.4 Audit: tamper-evident log, export, SIEM (T1)

### 6.4.1 Behaviour
- Written in the business transaction (never after it), with `org_id`, `workspace_id`, actor (`user|api_key|system|staff`), action, target, `changes {before, after}` limited to changed fields (secrets redacted by a field denylist), `ip_prefix`, `user_agent`, `request_id`.
- Sealed into the per-org chain by `AuditSealer` (5 s); anchored daily to WORM storage; verification endpoint (§5.5).
- **Retention by plan:** Business 1 year (DPDP Rules also expect security logs to be kept at least a year), Enterprise contract-defined (default 7 years). Purged by `AuditRetention` with `SET LOCAL qrit.audit_retention='on'` under a dedicated DB role; verification starts from the oldest remaining entry and is checked against its anchor.

### 6.4.2 Action catalogue (stable strings; ≥ 70 actions)
- **Auth:** `auth.login.succeeded|failed`, `auth.sso.login`, `auth.mfa.verified|failed`, `auth.step_up`, `auth.password.changed|reset`
- **Sessions and MFA:** `session.revoked`, `mfa.factor.added|removed`, `mfa.recovery.regenerated`
- **People:** `member.added|removed|suspended|deprovisioned|role_changed`, `group.*`, `role.*`, `binding.*`, `scim.user.*`, `scim.group.*`
- **Identity config:** `sso.connection.*`, `sso.enforcement.changed`, `domain.claim.*`, `policy.org.updated`, `policy.workspace.updated`
- **QR:** `qr.created|updated|archived|deleted|restored`, `qr.version.created|activated|restored|cancelled`, `approval.requested|decided|expired|overridden`
- **Integrations and keys:** `apikey.created|revoked`, `webhook.*`, `integration.*`
- **Data access:** `export.requested|downloaded`, `lead.viewed|exported|erased`, `dsar.*`
- **Commercial:** `billing.*`, `contract.*`, `invoice.issued`
- **Support and audit:** `support_access.granted|revoked|session_started`, `audit_stream.*`, `audit.verify.failed`

### 6.4.3 API and UI
- **API:**
  - `GET /v1/orgs/{org}/audit-logs?workspace_id&actor_id&action&target_id&from&to&cursor` (`org.audit` or `audit.read` for workspace-filtered)
  - `GET …/verify`
  - `POST …/exports {format: csv|jsonl, from, to}` → job with a signed download (step-up)
  - `GET/POST /v1/orgs/{org}/audit-streams`, `PATCH/DELETE …/{id}`, `POST …/{id}/test`
- **Streams:** delivered in `seq` order with at-least-once semantics (receivers dedupe on `seq`); batches of ≤ 500; the cursor advances only after a 2xx.

  | Kind | Delivery |
  |---|---|
  | `webhook` | Standard Webhooks signing |
  | `splunk_hec` | `POST {url}/services/collector/event`, `Authorization: Splunk <token>` |
  | `datadog` | `POST https://http-intake.logs.{site}/api/v2/logs`, `DD-API-KEY` |
  | `s3` | Hourly `jsonl.gz` objects `{prefix}/{yyyy}/{mm}/{dd}/{hh}.jsonl.gz` using customer-supplied credentials (encrypted) |

  After 24 h of errors the stream pauses and alerts org admins.
- **UI:** timeline with filters, actor avatars and human-readable sentences ("Priya changed the destination of *Table tent* from example.org/menu to example.org/diwali"), a JSON diff expander, a "Chain verified ✓ through #18,442 (anchored 25 Sep)" badge, export and stream settings.
- **Tests:** verified in §8.3 (sealing, UPDATE/DELETE blocked, DBA tamper detected, retention-safe verification). Plus: every mutating route produces exactly one audit row (the router introspection test from blueprint §8.2 extended), stream ordering across restarts, and redaction of secrets.
- **Acceptance:** an auditor downloads a month's export plus that day's anchor file and can recompute the chain with the documented algorithm (a 40-line reference script is published in docs).

---

## 6.5 Integrations, alerts and scheduled reports (T2)

### 6.5.1 Integration framework (`internal/integrations`)

```go
type Provider interface {
    Key() string                                   // "slack", "hubspot", …
    Events() []string                              // supported event types
    ValidateConfig(ctx context.Context, cfg json.RawMessage) error
    OAuth() *OAuthSpec                             // nil for token/webhook providers
    Deliver(ctx context.Context, in Installation, ev Event) error   // idempotent on ev.ID
}
```

- Installations live in `integrations` (credentials in `credentials_ct`).
- Deliveries go through River `IntegrationDeliver{installation_id, event_id}` with `integration_deliveries` as the idempotency ledger. Retries use webhook backoff; after 20 consecutive failures the installation moves to `error` and admins are emailed.
- All outbound HTTP uses `SafeClient` (blueprint §8.6).
- OAuth tokens refresh proactively 5 min before expiry. Refresh failures surface as "Reconnect" in the UI.
- **API:** `GET /v1/integrations/catalog`, `GET/POST /v1/workspaces/{ws}/integrations`, `PATCH/DELETE …/{id}`, `POST …/{id}/test`, `GET …/{id}/deliveries`, `GET /v1/integrations/oauth/{provider}/start?workspace_id=` → `GET /v1/integrations/oauth/{provider}/callback` (state bound to the session + workspace, PKCE where supported).

| Provider | Mechanism | Events → action | Notes |
|---|---|---|---|
| **Slack** | OAuth v2 (`chat:write`, `channels:read`), or incoming webhook URL | approval.requested/decided, alert.fired, serial.flagged, weekly digest → Block Kit message with deep links | Channel picker; one installation per channel |
| **Microsoft Teams** | Workflows webhook URL (Power Automate "post to channel when a webhook request is received") | Same events → Adaptive Card | Office 365 connector webhooks were permanently disabled in May 2026; Workflows webhooks accept MessageCard or Adaptive Card payloads (no interactive MessageCards, no custom bot name) |
| **Zapier / Make** | API-key auth + **REST hooks**: `POST /v1/hooks {target_url, event}` → 201 `{id}`, `DELETE /v1/hooks/{id}`, `GET /v1/hooks/samples/{event}` | Triggers: new QR, scan (sampled), new lead, approval decided. Actions: create QR, update destination, find QR by name/short code | Ship the Zapier app (Zapier Platform CLI) in `integrations/zapier/` and a Make custom app spec |
| **HubSpot** | OAuth (`crm.objects.contacts.write`, `crm.objects.contacts.read`) | lead.created → batch upsert contact by email (`/crm/v3/objects/contacts/batch/upsert`), mapped fields + `qrit_source_qr`, `qrit_campaign`, consent purposes | Field-mapping UI; respects the lead's withdrawn status |
| **Salesforce** | OAuth web-server flow (connected app) | lead.created → `POST /services/data/vXX.X/sobjects/Lead` with `LeadSource = 'QR: {qr_name}'` | Pin the API version in config |
| **Google Sheets** | OAuth, scope `drive.file` (least privilege; user picks/creates the sheet) | lead.created / daily scan summary → `spreadsheets.values.append` | Keeps Google's OAuth verification burden low |
| **GA4** | Measurement Protocol: `POST https://www.google-analytics.com/mp/collect?measurement_id=…&api_secret=…` (or `region1.` for EU) | Counted scans → event `qr_scan` {qr_id, qr_name, campaign, country, device_type}, batched ≤ 25 events/request | `client_id` is synthetic (a hash of the daily visitor hash), so GA4 counts these as new users. **UTM append (blueprint §7.9) stays the primary attribution**; MP is for "offline touchpoint" reporting and documented as such |
| **Warehouse export** | S3-compatible (keys), GCS (HMAC keys) or BigQuery (service-account JSON, load jobs) | Daily (Enterprise: hourly) export of `scan_events` (counted + flags, no visitor hash) and QR dimension table as `jsonl.gz`, schema v1 documented, `_manifest.json` per run | River `WarehouseExport`, resumable per partition-hour |

### 6.5.2 Alerts
- **Rules** (`alert_rules`):

  | Kind | Params | Evaluation |
  |---|---|---|
  | `scan_spike` / `scan_drop` | `z` (default 3), `min_scans` (20), `drop_ratio` (0.2) | Verified `ScanSpikeCandidates` query on the last complete 15-min bucket vs the same bucket over the previous 7 days |
  | `scan_threshold` | `threshold`, `window` (day/week) | v1's `scan_alert_threshold`, now actually evaluated |
  | `no_scans` | `hours` | Fires only during an active campaign's date range |
  | `destination_down` | — | 3 consecutive failed checks by the destination health job (HEAD, then GET on 405; SafeClient; 10 s timeout; codes with scans in the last 7 days) |
  | `serial_anomaly` | — | Fired from `RecordSerialVerification` flags |
  | `security_event` | `actions[]` (e.g. `sso.enforcement.changed`, `apikey.created`, `auth.login.failed` burst ≥ 20/10 min) | Subscribes to domain events |
  | `approval_pending` | `hours` | Request older than N hours |

- **Evaluation:** River periodic `AlertEvaluate` every 5 min; `cooldown_minutes` prevents repeats; each firing is an `alert_events` row + `FanOut(alert.fired)` to channels (emails and/or integration ids).
- **API:** `GET/POST /v1/workspaces/{ws}/alert-rules`, `PATCH/DELETE …/{id}`, `GET /v1/workspaces/{ws}/alert-events`, `POST /v1/workspaces/{ws}/alert-events/{id}/resolve`.
- **UI:** rules list with a "test fire" button, an alert history timeline, and an "Alerts" chip on affected QR codes.

### 6.5.3 Scheduled reports
- `report_schedules` (daily/weekly/monthly at a local hour in a timezone). River `ReportRun` computes the same analytics payloads as the dashboard (blueprint §6.3 service), and `render` builds a **PDF**:
  - cover with the workspace brand;
  - KPI tiles;
  - time-series and breakdown charts drawn as SVG by a small chart builder in `render` (no headless browser);
  - top codes table;
  - a data-quality note.
  Output is PDF and/or CSV zipped, stored privately, and emailed as a **signed link valid 7 days** (never an attachment > 5 MB). `next_run_at` is recomputed with DST-safe logic.
- **API:** `GET/POST /v1/workspaces/{ws}/report-schedules`, `PATCH/DELETE …/{id}`, `POST …/{id}/send-now`.
- **Tests:** DST transitions (America/New_York March/November), IST month-end, empty-data report, recipient limit, unsubscribe link per recipient.
- **Acceptance:** a weekly Monday 09:00 IST report arrives within 5 min of schedule, and its numbers equal the dashboard's for the same range.

## 6.6 Lead capture, consent and privacy operations (T2, compliance T1)

- **Forms:**
  - Up to 25 fields: text, email, phone (E.164 via `libphonenumber`), select, multi-select, checkbox, textarea, date, hidden (auto-filled UTM/QR id).
  - Per-language labels; conditional visibility (one level).
  - Hosted as a dynamic QR `content_type='form'` page at `/p/{code}`, or embedded as a block in a links page.
- **Consent notice (DPDP/GDPR):**
  - `forms.notice` holds the itemised purposes (each with a checkbox when optional), controller identity (workspace legal name), grievance/contact email, retention period, and the withdrawal method, per language (EN + HI ship; others optional).
  - The notice is shown separately from the fields; submission records `{notice_version, purposes_accepted[], lang, at}`.
  - The confirmation email carries a **one-click withdrawal link** (`/c/{token}`, making withdrawal as easy as giving consent).
- **Spam and abuse:** Cloudflare Turnstile (invisible), honeypot field, ≤ 10 submissions/10 min per IP prefix per form, and a duplicate-email window of 5 min.
- **Encryption at rest:**
  - Answers are AES-256-GCM encrypted with the **org data key (DEK)**. The DEK is wrapped by `APP_ENCRYPTION_KEY` (KMS-ready interface) and stored in `org_data_keys`.
  - The email blind index is `HMAC-SHA256(org blind-index key, lower(email))`, used for DSAR lookup and duplicate checks.
  - DEK rotation: new submissions use the newest `key_id`; a job re-encrypts old rows in the background.
- **Retention:** `delete_after = created_at + retention_days`. The daily `LeadRetention` job erases the payload (`data_ct` replaced with an empty ciphertext, `status='erased'`) and keeps the non-personal counts.
- **Access:** viewing decrypted leads needs `lead.read` (audited as `lead.viewed` per page view); export needs `lead.export` + step-up + org `export_permission`.
- **DSAR:**
  - `POST /v1/orgs/{org}/dsar-requests {kind: access|erasure|correction, email}` searches the blind index across all the org's forms, then produces a JSON export (access) or erases (erasure) inside a job, due within 30 days (the configurable SLA).
  - `dsar_requests` tracks status. The public withdrawal endpoint marks submissions `withdrawn` and fires `lead.withdrawn` to integrations (HubSpot/Salesforce sync sets their opt-out).
- **Public API:** `GET /v1/public/forms/{code}`, `POST /v1/public/forms/{code}/submissions` (Turnstile token), `GET/POST /v1/public/consent/{token}` (withdraw), `GET /v1/public/consent/{token}/confirm` (double opt-in).
- **Breach readiness (DPDP 72 h):** runbook in `docs/runbooks/breach.md`, a template notice, and an admin tool that lists affected data principals by org/form from the blind index without decrypting.
- **Tests:** crypto round-trip and key rotation; erasure leaves no plaintext (a test scans the DB for known strings); withdrawal propagates to integrations; Turnstile failure paths; the notice version is stored.
- **Acceptance:** a visitor submits the form in Hindi, receives a confirmation with a withdrawal link, and withdraws. The lead shows *withdrawn* within 1 s and HubSpot shows opted-out within 1 min. A DSAR access request returns that person's data across 3 forms.

## 6.7 Consent-aware retargeting pixels (T2)

- **Model:** `pixels` (provider meta|google|linkedin|tiktok + **ID only**; we never accept a customer's pasted script) and `qr_code_pixels` (Business+).
- **Redirect behaviour:** when a code has pixels, the redirect records the scan as usual, then serves an **interstitial page** (≤ 8 KB, `no-store`) instead of a 302:
  1. Shows the workspace brand and "Continue to {destination host}".
  2. Consent mode `opt_in_all` (default) shows a compact prompt: "Allow {brand} to measure ads with cookies? **Allow** / **No thanks**". `opt_in_where_required` shows it only when the visitor's country is in the EEA, UK, CH or IN (India is included because DPDP applies); other regions get a notice banner and pixels fire immediately.
  3. Google tags load with **Consent Mode v2 defaults denied** (`ad_storage`, `ad_user_data`, `ad_personalization`, `analytics_storage`), updated to granted only on Allow. Meta/LinkedIn/TikTok base codes load **only** on Allow (or notice-only regions).
  4. Redirects via `location.replace()` after the pixel scripts' load events or **800 ms**, whichever comes first. `<noscript><meta http-equiv="refresh" content="0;url=…">` is the fallback.
  5. A beacon `POST /_px/{code}` `{a: shown|accepted|declined|auto}` increments Redis counters, flushed to `pixel_consent_daily`.
- **Security:** the per-response CSP allows only the provider script origins; pixel IDs are validated by regex (`^[A-Za-z0-9_-]{3,40}$`); no customer HTML or JS is ever rendered.
- **Analytics UI:** pixel reach (accepted/shown), interstitial latency p50/p95, and a warning that interstitials add time.
- **Tests:** CSP snapshot; no third-party request before Allow in `opt_in_all` (Playwright network assertions); timeout path; no-JS path.
- **Acceptance:** a scan from India shows the prompt, and declining redirects within 300 ms with zero requests to facebook.net/googletagmanager.com. Accepting fires Meta + Google with consent granted.

---

## 6.8 GS1-conformant resolver (T2)

Target: **GS1-Conformant Resolver Standard, Release 1.2.1 (ratified August 2026)**. It is served on any active short domain or a dedicated GS1 domain (e.g. `id.brand.com`), by the `redirect` service, at paths starting `/01/` plus `/.well-known/gs1resolver`.

| Requirement (standard) | Implementation |
|---|---|
| HTTP GET, HEAD, OPTIONS over HTTPS | All three handled. OPTIONS returns `Allow` + CORS preflight headers |
| CORS for cross-origin JavaScript | `Access-Control-Allow-Origin: *`, `Access-Control-Expose-Headers: Link, Location, Content-Type` |
| Validate the Digital Link URI; 400 on failure | Parser in `internal/gs1`: AI order `01` → `22` → `10` → `21`, GTIN-14 with mod-10 check digit, AI value charset (GS1 AI encodable character set 82), percent-decoding, length limits (lot/serial ≤ 20). Invalid → `400` problem+json. **Never 200 for an error** |
| Redirect to the default link unless the request says otherwise | Default = `is_default` link of the most specific matching item (verified `GS1Linkset` ordering: e.g. a lot-level recall link overrides the product page for that lot only) |
| `linkType` parameter; **404 if the requested type is unavailable** (changed in 1.2) | `?linkType=gs1:nutritionalInfo` → that link (language-negotiated) or `404` |
| Linkset per RFC 9264 for `linkType=linkset` (or `all`) or `Accept: application/linkset+json` | `200` `application/linkset+json` listing all inherited links `{anchor, "https://gs1.org/voc/pip": [{href, title, hreflang, type}], …}` + `Link: <https://ref.gs1.org/standards/resolver/linkset-context>; rel="http://www.w3.org/ns/json-ld#context"` (context URL per the pinned standard version) |
| Qualifier inheritance: union of linksets across qualifier combinations | `GS1Linkset` returns candidates where each qualifier is NULL or equal; union ordered by specificity (verified) |
| `300 Multiple Choices` when several links match equally | Two links of the requested type with equal language match → 300 with the linkset body |
| Accept-Language negotiation; `gs1:defaultLinkMulti` | `hreflang` matching (RFC 4647 lookup); a default-multi group chooses by language |
| Pass through query-string key=value pairs when redirecting | All request query params are appended to the target, except `linkType` |
| Resolver description file at `/.well-known/gs1resolver` | JSON per the standard's schema: name, resolverRoot, supportedPrimaryKeys `["gtin"]`, supportedLinkTypes, contact |
| Decompression | EPC binary / DL compression: **MAY** in the standard. Deferred (§4.2 T3), returns 400 with a clear message |

- **Redirect status:** the standard does not mandate a specific code. We use **307 Temporary Redirect** (temporary, method-preserving, never cached), with `Cache-Control: private, no-store` like all dynamic redirects.
- **Analytics:** each resolution emits a scan event on `gs1_items.qr_code_id`, with `rule_id = linkType` (e.g. `gs1:pip`), so dashboards show "Product page 82%, Recall 11%, Nutrition 7%".
- **Management:**
  - API: `GET/POST /v1/workspaces/{ws}/gs1/items`, `PATCH/DELETE …/{id}`, `GET/POST …/{id}/links`, `PATCH/DELETE …/links/{linkId}`, `POST /v1/workspaces/{ws}/gs1/import` (CSV: gtin, lot, serial, link_type, href, title, lang, default).
  - UI: a product catalog with a GTIN search, link editor with the GS1 Web Vocabulary link-type picker, a **recall mode** button (adds a lot-level `gs1:recallStatus` default in one click), and a "test resolver" panel.
- **Printing:** the payload is the uncompressed DL URI, e.g. `HTTPS://ID.BRAND.COM/01/09506000134352/10/LOT42`. It stays in alphanumeric mode unless lot/serial contain lower-case, so the UI warns and suggests upper-case lots.
- **Tests:** a table of ≥ 60 URI syntax cases (valid/invalid check digits, AI order, encodings); the linkset JSON validated against RFC 9264 structure; the resolver description validated against GS1's published schema; GS1's reference conformance tests where available (document the results in the trust center).
- **Acceptance:** scanning a product's DL QR opens its product page; `?linkType=gs1:nutritionalInfo` opens nutrition; activating recall for LOT42 changes only LOT42's default target within 1 s; `curl -H 'Accept: application/linkset+json'` returns the full linkset.

## 6.9 Serialization and product authentication (T2)

- **Batch creation:** `POST /v1/workspaces/{ws}/serial-batches {name, quantity ≤ 10,000,000, gtin?, rules, verify_page, design}`.
  1. Creates the anchor `qr_codes(content_type='serial_batch', mode='dynamic')` + `serial_batches(status='generating')`.
  2. River `SerialGenerate` creates codes in chunks of 50,000 via `COPY`. Each serial = 9 random Crockford base32 chars (45 bits) + 3-char MAC. MAC = first 15 bits of `HMAC-SHA256(SERIAL_MAC_KEY, serial9)`, base32-encoded.
  3. Progress is exposed via `generated`, then `status='ready'` + `serial_batch.ready` event.
- **Printed payload:** `HTTPS://{HOST}/V/{SERIAL12}` (upper-case alphanumeric, so compact QR). Exports for variable-data printing: CSV (`serial, url, gtin, batch`) for BarTender/NiceLabel/Esko, plus a PDF proof sheet of the first 100 codes.
- **Scan path:**
  - The redirect validates charset + MAC **before any DB access**; a forgery gets a fast 404 "not a genuine code" page and an `invalid_serial` counter.
  - Valid → scan event (with `serial`) → `302 /v/{serial}` (web verify page, same host).
  - The page's JavaScript calls `POST /v1/public/verify/{serial} {t}`, where `t` = an HMAC of (serial, 10-min timestamp) embedded by the page. The API runs **`RecordSerialVerification`** (verified; atomic) and returns the verdict.
- **Verdicts:**

  | Status after update | Page |
  |---|---|
  | `active`, scan_count = 1 | "✓ Authentic — first verification" + product/batch/manufacture info |
  | `active`, scan_count > 1 | "✓ Authentic — previously verified {n} times, first on {date} in {city}" |
  | `flagged` (`multi_country` / `max_scans_exceeded` / velocity) | "⚠ This code has been scanned in unusual circumstances. It may be a copy." + a report form (photo upload optional) |
  | `void` | "✕ This code is not valid" (recalled/destroyed stock) |

- **Anomaly fan-out:** a transition to `flagged` → `serial.flagged` event → `serial_anomaly` alerts + webhook.
- **Management:**
  - API: `GET /v1/workspaces/{ws}/serial-batches`, `GET …/{id}` (stats: verified %, flagged count, countries map), `GET …/{id}/codes?status=flagged&cursor`, `POST …/{id}/export` (job), `POST …/{id}/void {serials[] | all}`.
  - UI: batch list, a verification map, a flagged-codes queue with a "mark reviewed" action.
- **Stated limit (on the product page and in docs):** a QR code can be photocopied. This system *detects* copies through impossible scan patterns; it can't prevent copying. Copy-proof secure graphics are out of scope.
- **Tests:** MAC forgery rejection without a DB call; the verdict rules (verified: multi-country and max-scans flags); link-preview bots don't increment (no JS); the 10 M-code generation benchmark (< 15 min, memory flat).
- **Acceptance:** 100,000 serials generated and exported in < 2 min. Scanning the same label in India then the UAE within 24 h flags it and posts to Slack.

## 6.10 White-label and agency accounts (T2)

- **Custom dashboard host:**
  - `org_branding.app_hostname` (e.g. `app.agency.com`): the customer adds a CNAME to `apps.example.com`, which creates a Cloudflare for SaaS custom hostname (same machinery as short domains) with status polling.
  - `web` reads `Host` in `proxy.ts`, fetches `GET /v1/public/branding?host=` (cached 5 min at the edge), and renders with the org's product name, logo, favicon, primary colour (mapped onto the design tokens; contrast-checked, falling back to the default accent when contrast < 4.5:1) and support URL.
  - Cookies are host-only, so sessions are per host. SSO callbacks use the custom host when the login started there.
- **Email domain:** `org_branding.email_domain` is set up via the Resend Domains API (create → show DKIM/SPF records → verify). Transactional emails for that org's users come from `noreply@{email_domain}` with the org's product name. Without it, emails come from the platform domain with the org name.
- **`hide_platform_brand`:** removes every "QRit" mention from the dashboard, emails, hosted pages, status pages and the `/{code}+` preview. The legal footer keeps "Service operated by <platform legal entity>" in the terms link only (a legal requirement).
- **Agency model:**
  - `organizations.kind='agency'` can create **client orgs** (`parent_org_id`): `GET/POST /v1/orgs/{org}/clients`, `PATCH /v1/orgs/{org}/clients/{child}` (plan within the agency contract).
  - Agency admins switch context via the org switcher; every action lands in the client's audit chain tagged `via_agency`.
  - Client users can't see the agency or other clients.
  - Billing consolidates on the agency contract (client orgs have no subscription of their own).
- **Tests:** host-based branding isolation (org A's branding never renders on org B's host); email-domain verification states; agency tenant isolation (client A's data unreachable from client B, even for agency members without a binding).
- **Acceptance:** an agency runs three client orgs from `app.agency.com` with its own logo and email domain, and no QRit branding appears anywhere a client can see.

## 6.11 Developer platform, bulk at scale and print-grade output (T2)

- **API keys v2:**
  - `environment` live|test. **Test keys only operate on the org's sandbox workspace** (`workspaces.is_sandbox`, created on demand via `POST /v1/orgs/{org}/sandbox`), whose codes resolve on `sandbox.qr.example.com` behind a "Test mode" banner page.
  - Per-key `ip_allowlist`; org `api_key_max_days` forces an expiry.
  - Scopes are enforced by authz (§5.3). Hashing stays **sha256 of a 160-bit random key** (not bcrypt: the key is high-entropy, and lookup must be O(1)).
  - Usage: Redis counters → hourly flush to `api_usage_daily`; `GET /v1/workspaces/{ws}/api-usage?from&to`; a UI chart per key with throttled/error rates.
- **SDKs:** generated from `openapi.yaml` in CI on release tags:
  - TypeScript `@qrit/sdk` (openapi-fetch wrapper with retries and idempotency keys);
  - Python `qrit` (openapi-python-client);
  - Go `github.com/<you>/qrit-go` (oapi-codegen client only).
  Plus a Postman collection, an API changelog page, and `Deprecation`/`Sunset` headers policy.
- **Bulk at scale (100k rows):**
  - Streaming CSV parse; **dry-run mode** returns a full validation report before anything is written.
  - Modes: `bulk_create`, **`bulk_update`** (match by `id` or `short_code`; change destination, name, folder, tags, status; destination changes go through approvals §6.3), `bulk_download`.
  - Chunked transactions of 1,000 rows with checkpointing, so a restarted job resumes. Progress runs at ≥ 1,000 rows/s.
- **Print-grade output (render service):**

  | Output | Implementation |
  |---|---|
  | SVG | as-is (vector) |
  | PDF RGB | pdfkit + svg-to-pdfkit (vector, embedded font) |
  | **PDF CMYK** | same, with colours converted to CMYK (`[c,m,y,k]` via a naive or profile-based conversion; pure black modules → `0,0,0,100` rich-black option off by default) |
  | **EPS** | Ghostscript sidecar `gs -sDEVICE=eps2write` from the vector PDF (in the render container) |
  | **Label sheets** | job kind `print_sheet`: N-up layouts (A4/Letter; templates Avery L7160/L7163/5160, table-tent A5, 4×6 in), optional crop marks and 3 mm bleed, a code name + short URL caption under each code |

  The **print size calculator** in the download dialog recommends a printed size ≥ max(2 cm, scan distance ÷ 10) and module size ≥ 0.4 mm, and warns below those.
- **Tests:** test-key isolation (a test key touching a live workspace → 404); a 100k-row dry-run in < 60 s; resume after kill; CMYK PDF colour-space check (parse the PDF and assert DeviceCMYK); EPS decode round-trip (rasterise with Ghostscript, then jsQR).
- **Acceptance:** a retailer uploads 50,000 SKUs, dry-runs, fixes 12 flagged rows, creates them, and downloads print sheets of Avery L7163 labels whose codes all decode.

## 6.12 Enterprise billing, support access, SLA, staff console and later-tier designs

### 6.12.1 Contracts and GST invoicing (T1)
- **Org-level billing:** self-serve subscriptions (Razorpay for INR incl. UPI Autopay; Stripe for international where the entity is eligible) move to `subscriptions.org_id`.
- **Enterprise deals** use `contracts`: term, interval, currency, amount, seats, `limits_override` (merged over plan entitlements by `entitlements.Effective(org)`), SLA, PO number, payment terms. Staff create and activate them in the staff console (§6.12.3).
- **Invoices:** River `InvoiceRun` issues an invoice at each contract interval.
  - **Number** = `next_invoice_number(issue_date)` (verified: unique, consecutive per financial year April–March, resets each April, ≤ 16 chars as GST requires).
  - **Tax mode:** seller state = buyer state → **CGST 9% + SGST 9%**; different Indian state → **IGST 18%**; buyer outside India and paid in convertible foreign exchange → **export under LUT, zero-rated**, with the mandatory endorsement "Supply meant for export under Bond or Letter of Undertaking without payment of Integrated Tax" (DB constraint verified). Buyer GSTIN is validated by regex (verified) and shown on the invoice.
  - **SAC code:** configurable (`SELLER_SAC`), and **must be confirmed with your CA** (SaaS is commonly filed under heading 9983 sub-codes or 997331).
  - **E-invoicing (IRN via the IRP)** becomes mandatory once aggregate turnover crosses the government threshold (₹5 crore at last check; confirm with your CA). The schema reserves space (`invoices` is additive; add `irn`, `ack_no`, `signed_qr` columns then).
- **Delivery:** the PDF is rendered by `render` (pdfkit) with seller/buyer blocks, a tax breakup and a payment QR (UPI intent `upi://pay?pa=…&am=…&tn=<invoice no>` for INR invoices). It is emailed to `billing_email` and available at `GET /v1/orgs/{org}/invoices`, `GET …/{id}/pdf`.
- **Payment:** Razorpay payment link per invoice (INR) or bank transfer (NEFT/RTGS/SWIFT). Marked paid via the Razorpay webhook or staff action.
- **Dunning:** reminders at due −7, 0, +7, +14 days. After +30 days, dashboard edits become read-only for that org. **Codes never stop redirecting** (blueprint D7).
- **Seat true-up:** a quarterly snapshot of active members vs contract seats → an overage line on the next invoice (prorated).

### 6.12.2 Customer-controlled support access (T1)
- An org admin grants time-boxed access (`support_access_grants`, ≤ 7 days, `read` or `read_write`, reason required).
- Staff start a support session through the staff console only while a grant is active. Every staff action is audited in the customer's chain (`actor_type='staff'`), and a banner shows in the customer's dashboard while a session is live.
- There is no other way for staff to read customer data (DB access for staff is break-glass, logged outside the app).

### 6.12.3 Staff console (`/admin`, `is_staff` + MFA + IP allowlist)
Org search, plan and contract management, invoice actions, feature flags per org (`feature_flags`), support sessions (grant-gated), abuse queue (blueprint), DSAR oversight, SLA reports, system health links.

### 6.12.4 SLA and status (T1)
- **Contract SLA:** 99.95% monthly for redirects and 99.9% for dashboard/API.
- **Measurement:** synthetic probes (Better Stack, 1-min, 3 regions) of the canary code and API health.
- **Monthly per-org SLA report** (PDF), with service credits of 10% below 99.95%, 25% below 99.0% and 50% below 95.0%, applied as a credit note on the next invoice.
- **Status page:** components Redirects, Dashboard, API, Analytics pipeline, Webhooks & integrations, SSO. Email/RSS subscriptions; incident history ≥ 12 months.

### 6.12.5 T3 designs (recorded so they're never blocked)
- **Regional data planes (EU/US):**
  - `organizations.data_region` pins an org's data to a regional stack (Postgres + Redis + stream + R2 bucket + ingest/worker).
  - A small global control plane keeps `org → region` and `domain → region` routing.
  - The redirect edge (multi-region already) resolves the domain's region and reads that region's cache/DB; the dashboard API routes by org region.
  - Migrating an existing org between regions is an offline job. Build when the first EU enterprise contract requires residency.
- **Gated hosted pages** (SSO for page viewers, e.g. internal training material): an OIDC login step in `web` before rendering `/p/{code}`, with the org's IdP.
- **HIPAA:** not offered. The trust center states that PHI must not be stored in QR content or forms.

---

# 7. Policy enforcement matrix

Every stored policy maps to exactly one enforcement point, one error code, and at least one negative test. This table is the contract. v1 failed precisely here.

| Policy (table.column) | Enforcement point | Response when violated | Negative test |
|---|---|---|---|
| `org_security_policies.enforce_sso` | `middleware/orggate.go` (before handlers, per org resource) | `403 sso_required` + `login_url` | Password session → org resource denied; break-glass with MFA allowed |
| `…sso_break_glass_user_ids` | same | allowed only when `mfa_verified_at` set; alert fired | Break-glass without MFA → `403 mfa_required` |
| `…require_mfa` | `orggate.go` | `403 mfa_required` + `enroll_url` | Non-SSO session without MFA denied |
| `…allowed_mfa_kinds` | `/me/mfa` enrolment + `/auth/mfa/verify` | `422 mfa_kind_not_allowed` | Enrolling TOTP when only WebAuthn allowed |
| `…session_idle_minutes` / `session_max_hours` | `/auth/refresh` + access middleware session check | `401 session_expired` | Fake clock beyond idle → refresh refused |
| `…dashboard_ip_allowlist` | `orggate.go` (cookie principals) | `403 ip_not_allowed` | Request from a non-listed IP (test injects CF header from trusted proxy) |
| `…api_ip_allowlist` + `api_keys.ip_allowlist` | API-key auth middleware | `403 ip_not_allowed` | Key used from a non-listed IP |
| `…password_min_length` | register, password change/reset for users whose email domain is claimed by the org | `422 password_too_short` | 9-char password rejected at min 10 |
| `…invite_email_domains` | `POST /workspaces/{ws}/invites`, org member add | `422 invite_domain_not_allowed` | Invite `x@gmail.com` refused |
| `…api_key_max_days` | `POST /api-keys` | `422 api_key_expiry_required` | Key without `expires_at` refused |
| `…export_permission` | every export endpoint (analytics, raw scans, leads, audit) | `403 export_disabled` / `403 export_admins_only` | Editor export refused when `admins_only` |
| `workspace_policies.allowed_destination_hosts` | `approval.Evaluate` in version/QR/rules/bulk services | `202` pending approval (or `422 host_not_allowed` when mode is `off` and a list is set) | Non-listed host goes to approval, never live |
| `…blocked_destination_hosts` | `urlsafety.Validate` policy step | `422 host_blocked` | Blocked host refused everywhere (incl. split variants, fallback URL) |
| `…require_https` | `urlsafety.Validate` | `422 https_required` | `http://` refused |
| `…approval_mode` / `approvals_required` / `approval_expiry_hours` | `approval` service + DB trigger + resolver filter | `202 approval_required`; pending versions never served | Resolver serves previous version while pending (verified §8.3) |
| `…require_template` | QR create/design update | `422 template_required` | Create without `template_id` refused |
| `templates.is_locked` | design update | `403 template_locked` unless `qr.design.bypass_lock` | Editor edits locked design → refused |
| `…disabled_features` | entitlements middleware (feature check) | `403 feature_disabled` | Disabled `pixels` → pixel endpoints refused |
| `…pixel_consent_mode` | redirect interstitial renderer | n/a (behaviour) | Playwright: no third-party request before Allow |
| `forms.retention_days` | `LeadRetention` job | data erased | Fake clock past retention → ciphertext gone |
| `contracts.limits_override` | `entitlements.Effective(org)` | `402 limit_reached` uses overridden limit | Contract raises dynamic codes to 50,000 → allowed |
| `support_access_grants` | staff principal resolution | `403 no_support_grant` | Staff call without active grant refused |
| `org_members.status` | `EffectivePermissions` (joins `status='active'`) + session revoke | `404` for all org resources | Suspended member sees nothing (verified §8.3) |

---

# 8. Data model: migration 00003 and verified queries

## 8.1 Migration `00003_enterprise.sql`

This applies on top of the blueprint's `00001_init.sql`. It includes backfills (one org per existing workspace, role bindings from memberships, subscriptions moved to orgs, default policy rows). It was loaded on a database already holding Phase 1–4 data **and** on an empty one.

```sql
-- =====================================================================
-- QRit v2 — Enterprise layer. goose migration 00003_enterprise.sql
-- Applies on top of 00001_init.sql (+ 00002_river). Safe on an empty DB
-- and on a DB that already holds Phase 1–4 data (backfills included).
-- Conventions as in 00001. CIDR lists below are *policy configuration*
-- entered by admins, not visitor data; the "no visitor IPs" rule still holds.
-- =====================================================================

-- ---------------------------------------------------------------------
-- E1. Organizations: the enterprise tenant above workspaces
-- ---------------------------------------------------------------------
CREATE TABLE organizations (
    id             uuid PRIMARY KEY,
    name           text   NOT NULL,
    slug           citext NOT NULL UNIQUE,
    kind           text   NOT NULL DEFAULT 'standard'
                   CHECK (kind IN ('personal','standard','enterprise','agency')),
    parent_org_id  uuid REFERENCES organizations(id) ON DELETE RESTRICT,   -- agency → client orgs
    plan_id        text   NOT NULL DEFAULT 'free'
                   CHECK (plan_id IN ('free','pro','business','enterprise')),
    data_region    text   NOT NULL DEFAULT 'in' CHECK (data_region IN ('in','eu','us')),
    legal_name     text,
    billing_email  citext,
    gstin          text CHECK (gstin IS NULL OR gstin ~ '^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$'),
    tax_country    char(2) NOT NULL DEFAULT 'IN',
    billing_address jsonb NOT NULL DEFAULT '{}'::jsonb,  -- {line1, line2, city, state_code, postal_code, country}
    settings       jsonb  NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz,
    CONSTRAINT org_no_self_parent CHECK (parent_org_id IS NULL OR parent_org_id <> id)
);
CREATE INDEX organizations_parent_idx ON organizations (parent_org_id) WHERE parent_org_id IS NOT NULL;

CREATE TABLE org_members (
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_role    text NOT NULL DEFAULT 'member'
                CHECK (org_role IN ('org_owner','org_admin','billing_admin','member')),
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deprovisioned')),
    source      text NOT NULL DEFAULT 'invite' CHECK (source IN ('invite','sso_jit','scim','creator')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX org_members_user_idx ON org_members (user_id);
CREATE UNIQUE INDEX org_one_owner_uniq ON org_members (org_id) WHERE org_role = 'org_owner';

-- Backfill: one organization per existing workspace (same name/slug), owner carried over.
ALTER TABLE workspaces ADD COLUMN org_id uuid REFERENCES organizations(id) ON DELETE CASCADE;

INSERT INTO organizations (id, name, slug, kind, plan_id, created_at)
SELECT w.id, w.name, w.slug, 'standard', w.plan_id, w.created_at FROM workspaces w;   -- org id := workspace id (1:1 at migration time)
UPDATE workspaces SET org_id = id;
INSERT INTO org_members (org_id, user_id, org_role, source, created_at)
SELECT w.id, w.owner_id, 'org_owner', 'creator', w.created_at FROM workspaces w;
INSERT INTO org_members (org_id, user_id, org_role, source, created_at)
SELECT wm.workspace_id, wm.user_id, 'member', 'invite', wm.created_at
FROM workspace_members wm
WHERE wm.role <> 'owner'
ON CONFLICT DO NOTHING;

ALTER TABLE workspaces ALTER COLUMN org_id SET NOT NULL;
CREATE INDEX workspaces_org_idx ON workspaces (org_id);
COMMENT ON COLUMN workspaces.plan_id IS 'DEPRECATED since 00003: entitlements resolve from organizations.plan_id. Dropped in a later contract migration.';

-- Subscriptions move to the organization (consolidated billing).
ALTER TABLE subscriptions ADD COLUMN org_id uuid REFERENCES organizations(id) ON DELETE CASCADE;
UPDATE subscriptions s SET org_id = w.org_id FROM workspaces w WHERE w.id = s.workspace_id;
ALTER TABLE subscriptions ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE subscriptions ALTER COLUMN workspace_id DROP NOT NULL;
CREATE UNIQUE INDEX subscriptions_org_uniq ON subscriptions (org_id);

-- Claimed email domains: SSO routing ("you@acme.com → Acme's IdP") and auto-join.
CREATE TABLE org_domains (
    id                  uuid PRIMARY KEY,
    org_id              uuid   NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    domain              citext NOT NULL UNIQUE,
    verification_token  text   NOT NULL,
    verified_at         timestamptz,
    auto_join           boolean NOT NULL DEFAULT false,      -- verified-domain users may join without invite
    auto_join_workspace_id uuid REFERENCES workspaces(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- E2. Identity: SSO connections, linked identities, MFA, SCIM
-- ---------------------------------------------------------------------
CREATE TABLE sso_connections (
    id                      uuid PRIMARY KEY,
    org_id                  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    protocol                text NOT NULL CHECK (protocol IN ('saml','oidc')),
    label                   text NOT NULL,                      -- "Okta", "Entra ID"
    status                  text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','testing','active','disabled')),
    bridge_tenant           text,        -- SAML: Ory Polis tenant (= org slug); product = 'qrit'
    oidc_issuer             text,        -- OIDC direct: https://login.microsoftonline.com/{tid}/v2.0
    oidc_client_id          text,
    oidc_client_secret_ct   bytea,       -- AES-256-GCM
    jit_provisioning        boolean NOT NULL DEFAULT true,
    default_workspace_id    uuid REFERENCES workspaces(id) ON DELETE SET NULL,
    default_role_key        text NOT NULL DEFAULT 'analyst',
    attribute_mapping       jsonb NOT NULL DEFAULT '{"email":"email","first_name":"firstName","last_name":"lastName","groups":"groups"}'::jsonb,
    last_login_at           timestamptz,
    created_by              uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sso_oidc_fields CHECK (protocol <> 'oidc' OR (oidc_issuer IS NOT NULL AND oidc_client_id IS NOT NULL))
);
CREATE INDEX sso_connections_org_idx ON sso_connections (org_id);

CREATE TABLE user_identities (
    id              uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connection_id   uuid NOT NULL REFERENCES sso_connections(id) ON DELETE CASCADE,
    subject         text NOT NULL,        -- SAML NameID / OIDC sub (never email alone)
    email_at_login  citext NOT NULL,
    raw_claims      jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connection_id, subject)
);

CREATE TABLE user_mfa_factors (
    id              uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('totp','webauthn')),
    name            text NOT NULL,
    totp_secret_ct  bytea,
    credential_id   bytea UNIQUE,
    public_key      bytea,
    sign_count      bigint NOT NULL DEFAULT 0,
    aaguid          uuid,
    transports      text[],
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz,
    CONSTRAINT mfa_kind_fields CHECK (
        (kind = 'totp' AND totp_secret_ct IS NOT NULL)
     OR (kind = 'webauthn' AND credential_id IS NOT NULL AND public_key IS NOT NULL))
);
CREATE TABLE user_recovery_codes (
    user_id    uuid  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL,
    used_at    timestamptz,
    PRIMARY KEY (user_id, code_hash)
);

ALTER TABLE sessions
    ADD COLUMN auth_method       text NOT NULL DEFAULT 'password'
        CHECK (auth_method IN ('password','google','magic_link','sso')),
    ADD COLUMN sso_connection_id uuid REFERENCES sso_connections(id) ON DELETE SET NULL,
    ADD COLUMN mfa_verified_at   timestamptz,
    ADD COLUMN step_up_at        timestamptz;   -- last re-auth for sensitive actions

CREATE TABLE scim_directories (
    id                  uuid PRIMARY KEY,
    org_id              uuid  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    label               text  NOT NULL,
    token_prefix        text  NOT NULL UNIQUE,         -- 'scim_7F3K2M9Q' shown in UI
    token_hash          bytea NOT NULL UNIQUE,         -- sha256(token)
    status              text  NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    deprovision_action  text  NOT NULL DEFAULT 'suspend' CHECK (deprovision_action IN ('suspend','remove')),
    last_request_at     timestamptz,
    created_by          uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE scim_users (
    id            uuid PRIMARY KEY,                       -- returned to the IdP as SCIM "id"
    directory_id  uuid   NOT NULL REFERENCES scim_directories(id) ON DELETE CASCADE,
    user_id       uuid   NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id   text,
    user_name     text   NOT NULL,                        -- stored exactly as sent (Entra requirement)
    active        boolean NOT NULL DEFAULT true,
    resource      jsonb  NOT NULL,                        -- last full SCIM representation
    version       integer NOT NULL DEFAULT 1,             -- ETag W/"n"
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (directory_id, user_name),
    UNIQUE (directory_id, user_id)
);
CREATE UNIQUE INDEX scim_users_external_uniq ON scim_users (directory_id, external_id) WHERE external_id IS NOT NULL;

-- Groups: manual (created in UI) or SCIM-sourced. Roles can be bound to groups.
CREATE TABLE groups (
    id            uuid PRIMARY KEY,
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name  text NOT NULL,
    source        text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','scim','sso')),   -- sso = synced from the IdP groups claim at login
    directory_id  uuid REFERENCES scim_directories(id) ON DELETE CASCADE,
    external_id   text,
    version       integer NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, display_name),                       -- Entra matches groups on displayName
    CONSTRAINT group_scim_dir CHECK ((source = 'scim') = (directory_id IS NOT NULL))
);
CREATE TABLE group_members (
    group_id  uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    added_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id);

-- ---------------------------------------------------------------------
-- E3. Authorization: permission-based roles, bindings to users or groups,
--     at workspace or folder scope
-- ---------------------------------------------------------------------
CREATE TABLE roles (
    id           uuid PRIMARY KEY,
    org_id       uuid REFERENCES organizations(id) ON DELETE CASCADE,   -- NULL = system role
    key          text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{1,40}$'),
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    permissions  text[] NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (org_id, key)
);

INSERT INTO roles (id, org_id, key, name, description, permissions) VALUES
 ('00000000-0000-7000-8000-00000000f001', NULL, 'owner', 'Owner', 'Everything, including billing and deletion',
  ARRAY['*']),
 ('00000000-0000-7000-8000-00000000f002', NULL, 'admin', 'Admin', 'Manage people, domains, keys, integrations and policies',
  ARRAY['workspace.read','workspace.update','member.manage','role.manage','qr.read','qr.create','qr.update','qr.delete',
        'qr.destination.update','qr.destination.approve','qr.design.bypass_lock','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','analytics.raw','domain.manage','apikey.manage',
        'webhook.manage','integration.manage','policy.manage','audit.read','audit.export','form.manage',
        'lead.read','lead.export','pixel.manage','alert.manage','report.manage','serial.manage','gs1.manage','bulk.run']),
 ('00000000-0000-7000-8000-00000000f003', NULL, 'editor', 'Editor', 'Create and edit QR codes and campaigns',
  ARRAY['workspace.read','qr.read','qr.create','qr.update','qr.destination.update','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','form.manage','lead.read','alert.manage','report.manage','bulk.run']),
 ('00000000-0000-7000-8000-00000000f004', NULL, 'reviewer', 'Reviewer', 'Approve or reject changes; read everything',
  ARRAY['workspace.read','qr.read','qr.destination.approve','analytics.read','audit.read','lead.read']),
 ('00000000-0000-7000-8000-00000000f005', NULL, 'analyst', 'Analyst', 'Read-only access to codes and analytics',
  ARRAY['workspace.read','qr.read','analytics.read','analytics.export']);

CREATE TABLE role_bindings (
    id              uuid PRIMARY KEY,
    org_id          uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id)    ON DELETE CASCADE,
    principal_type  text NOT NULL CHECK (principal_type IN ('user','group')),
    principal_id    uuid NOT NULL,
    role_id         uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    scope_type      text NOT NULL DEFAULT 'workspace' CHECK (scope_type IN ('workspace','folder')),
    folder_id       uuid REFERENCES folders(id) ON DELETE CASCADE,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT binding_scope CHECK ((scope_type = 'folder') = (folder_id IS NOT NULL)),
    UNIQUE NULLS NOT DISTINCT (workspace_id, principal_type, principal_id, role_id, folder_id)
);
CREATE INDEX role_bindings_principal_idx ON role_bindings (principal_type, principal_id);
CREATE INDEX role_bindings_ws_idx        ON role_bindings (workspace_id);

-- Backfill bindings from existing memberships; allow the new reviewer role on workspace_members.
INSERT INTO role_bindings (id, org_id, workspace_id, principal_type, principal_id, role_id, created_at)
SELECT gen_random_uuid(), w.org_id, wm.workspace_id, 'user', wm.user_id, r.id, wm.created_at
FROM workspace_members wm
JOIN workspaces w ON w.id = wm.workspace_id
JOIN roles r ON r.org_id IS NULL AND r.key = wm.role;

ALTER TABLE workspace_members DROP CONSTRAINT workspace_members_role_check;
ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_role_check
    CHECK (role IN ('owner','admin','editor','reviewer','analyst','custom'));
COMMENT ON COLUMN workspace_members.role IS 'Display-only primary role. Authorization reads role_bindings.';

-- ---------------------------------------------------------------------
-- E4. Policies (enforced — see plan §E4 enforcement matrix)
-- ---------------------------------------------------------------------
CREATE TABLE org_security_policies (
    org_id                  uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    enforce_sso             boolean NOT NULL DEFAULT false,
    sso_break_glass_user_ids uuid[] NOT NULL DEFAULT '{}',     -- ≤ 2 owners who may still use password+MFA
    require_mfa             boolean NOT NULL DEFAULT false,     -- applies to non-SSO logins
    allowed_mfa_kinds       text[]  NOT NULL DEFAULT ARRAY['totp','webauthn'],
    session_idle_minutes    integer NOT NULL DEFAULT 0 CHECK (session_idle_minutes = 0 OR session_idle_minutes BETWEEN 5 AND 10080),
    session_max_hours       integer NOT NULL DEFAULT 0 CHECK (session_max_hours = 0 OR session_max_hours BETWEEN 1 AND 2160),
    dashboard_ip_allowlist  cidr[]  NOT NULL DEFAULT '{}',
    api_ip_allowlist        cidr[]  NOT NULL DEFAULT '{}',
    password_min_length     integer NOT NULL DEFAULT 10 CHECK (password_min_length BETWEEN 10 AND 128),
    invite_email_domains    citext[] NOT NULL DEFAULT '{}',     -- empty = any
    api_key_max_days        integer NOT NULL DEFAULT 0 CHECK (api_key_max_days >= 0),   -- 0 = no expiry required
    export_permission       text    NOT NULL DEFAULT 'role'  CHECK (export_permission IN ('role','admins_only','disabled')),
    updated_by              uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_policies (
    workspace_id               uuid PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    allowed_destination_hosts  text[]  NOT NULL DEFAULT '{}',   -- exact or '*.suffix'; empty = any
    blocked_destination_hosts  text[]  NOT NULL DEFAULT '{}',
    require_https              boolean NOT NULL DEFAULT true,
    approval_mode              text    NOT NULL DEFAULT 'off'
                               CHECK (approval_mode IN ('off','outside_allowlist','all_destination_changes','all_changes')),
    approvals_required         integer NOT NULL DEFAULT 1 CHECK (approvals_required BETWEEN 1 AND 3),
    approval_expiry_hours      integer NOT NULL DEFAULT 168 CHECK (approval_expiry_hours BETWEEN 1 AND 720),
    require_template           boolean NOT NULL DEFAULT false,  -- editors must pick a template
    pixel_consent_mode         text    NOT NULL DEFAULT 'opt_in_all'
                               CHECK (pixel_consent_mode IN ('opt_in_all','opt_in_where_required')),
    disabled_features          text[]  NOT NULL DEFAULT '{}',   -- org admin can switch features off per workspace
    updated_by                 uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at                 timestamptz NOT NULL DEFAULT now()
);
INSERT INTO workspace_policies (workspace_id) SELECT id FROM workspaces;
INSERT INTO org_security_policies (org_id) SELECT id FROM organizations;

-- ---------------------------------------------------------------------
-- E5. Approval workflow (maker–checker) on destination versions
-- ---------------------------------------------------------------------
ALTER TABLE qr_versions ADD COLUMN approval_status text NOT NULL DEFAULT 'not_required'
    CHECK (approval_status IN ('not_required','pending','approved','rejected','cancelled'));
CREATE INDEX qr_versions_pending_idx ON qr_versions (qr_code_id) WHERE approval_status = 'pending';

CREATE TABLE approval_requests (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind                text NOT NULL CHECK (kind IN ('create','destination','rules','bulk_update')),
    qr_code_id          uuid REFERENCES qr_codes(id) ON DELETE CASCADE,   -- single-code requests
    job_id              uuid REFERENCES jobs(id)     ON DELETE CASCADE,   -- bulk requests
    reasons             text[] NOT NULL,         -- e.g. {'host_not_allowlisted','policy_all_changes'}
    requested_by        uuid NOT NULL REFERENCES users(id),
    required_approvals  integer NOT NULL CHECK (required_approvals BETWEEN 1 AND 3),
    status              text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','approved','rejected','cancelled','expired')),
    note                text,
    override_reason     text,                    -- set when an org owner used break-glass publish
    expires_at          timestamptz NOT NULL,
    decided_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_subject CHECK (
        (kind = 'bulk_update' AND job_id IS NOT NULL AND qr_code_id IS NULL)
     OR (kind <> 'bulk_update' AND qr_code_id IS NOT NULL AND job_id IS NULL))
);
ALTER TABLE qr_versions ADD COLUMN approval_request_id uuid REFERENCES approval_requests(id) ON DELETE SET NULL;
ALTER TABLE qr_versions ADD CONSTRAINT qr_version_pending_has_request
    CHECK (approval_status NOT IN ('pending','approved','rejected') OR approval_request_id IS NOT NULL);
CREATE INDEX qr_versions_approval_req_idx ON qr_versions (approval_request_id) WHERE approval_request_id IS NOT NULL;
CREATE INDEX approval_requests_ws_pending_idx ON approval_requests (workspace_id, created_at) WHERE status = 'pending';

CREATE TABLE approval_decisions (
    request_id   uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    approver_id  uuid NOT NULL REFERENCES users(id),
    decision     text NOT NULL CHECK (decision IN ('approve','reject')),
    comment      text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, approver_id)
);

-- Four-eyes rule enforced in the database, not only in code.
CREATE FUNCTION approval_no_self_approval() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM approval_requests r
               WHERE r.id = NEW.request_id AND r.requested_by = NEW.approver_id) THEN
        RAISE EXCEPTION 'requester cannot decide on their own approval request'
            USING ERRCODE = 'check_violation';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM approval_requests r
                   WHERE r.id = NEW.request_id AND r.status = 'pending' AND r.expires_at > now()) THEN
        RAISE EXCEPTION 'approval request is not pending' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER approval_decisions_guard BEFORE INSERT ON approval_decisions
    FOR EACH ROW EXECUTE FUNCTION approval_no_self_approval();

-- ---------------------------------------------------------------------
-- E6. Tamper-evident audit log (hash chain per org, sealed asynchronously)
-- ---------------------------------------------------------------------
ALTER TABLE audit_logs
    ADD COLUMN org_id     uuid,
    ADD COLUMN seq        bigint,          -- 1..n per org, assigned at sealing
    ADD COLUMN prev_hash  bytea,
    ADD COLUMN hash       bytea,
    ADD COLUMN sealed_at  timestamptz;
UPDATE audit_logs a SET org_id = w.org_id FROM workspaces w WHERE w.id = a.workspace_id;
CREATE INDEX audit_logs_unsealed_idx ON audit_logs (org_id, id) WHERE hash IS NULL;
CREATE UNIQUE INDEX audit_logs_org_seq_uniq ON audit_logs (org_id, seq) WHERE seq IS NOT NULL;

-- Canonical bytes of an entry. jsonb::text is deterministic (keys sorted, fixed spacing).
CREATE FUNCTION audit_entry_bytes(a audit_logs) RETURNS bytea LANGUAGE sql IMMUTABLE AS $$
    SELECT convert_to(concat_ws(E'\x1f',
        a.id::text, coalesce(a.org_id::text,''), coalesce(a.workspace_id::text,''), a.actor_type,
        coalesce(a.actor_id::text,''), a.action, a.target_type, coalesce(a.target_id::text,''),
        a.changes::text, coalesce(a.ip_prefix,''), coalesce(a.request_id,''),
        (extract(epoch FROM a.created_at) * 1000000)::bigint::text), 'UTF8')
$$;

-- Append-only: rows may only be touched once, by the sealer, filling the chain columns.
CREATE FUNCTION audit_logs_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF current_setting('qrit.audit_retention', true) = 'on' THEN RETURN OLD; END IF;
        RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF OLD.hash IS NOT NULL
       OR NEW.id IS DISTINCT FROM OLD.id OR NEW.org_id IS DISTINCT FROM OLD.org_id
       OR NEW.workspace_id IS DISTINCT FROM OLD.workspace_id OR NEW.actor_type IS DISTINCT FROM OLD.actor_type
       OR NEW.actor_id IS DISTINCT FROM OLD.actor_id OR NEW.action IS DISTINCT FROM OLD.action
       OR NEW.target_type IS DISTINCT FROM OLD.target_type OR NEW.target_id IS DISTINCT FROM OLD.target_id
       OR NEW.changes IS DISTINCT FROM OLD.changes OR NEW.ip_prefix IS DISTINCT FROM OLD.ip_prefix
       OR NEW.user_agent IS DISTINCT FROM OLD.user_agent OR NEW.request_id IS DISTINCT FROM OLD.request_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_guard();

-- Seal up to p_limit unsealed entries of one org, in id order. Called by the worker every 5 s.
-- The advisory lock serialises sealers per org; inserts are never blocked.
CREATE FUNCTION audit_seal(p_org uuid, p_limit integer DEFAULT 1000) RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    v_prev bytea; v_seq bigint; r audit_logs; n integer := 0; v_hash bytea;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('audit_seal:' || p_org::text, 0));
    SELECT hash, seq INTO v_prev, v_seq FROM audit_logs
     WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq DESC LIMIT 1;
    v_prev := coalesce(v_prev, '\x'::bytea); v_seq := coalesce(v_seq, 0);
    FOR r IN SELECT * FROM audit_logs WHERE org_id = p_org AND hash IS NULL ORDER BY id LIMIT p_limit LOOP
        v_seq  := v_seq + 1;
        v_hash := sha256(v_prev || int8send(v_seq) || audit_entry_bytes(r));
        UPDATE audit_logs SET seq = v_seq, prev_hash = v_prev, hash = v_hash, sealed_at = now() WHERE id = r.id;
        v_prev := v_hash; n := n + 1;
    END LOOP;
    RETURN n;
END $$;

-- Verify the chain; returns the first broken seq, or NULL when intact.
-- After retention pruning, verification starts from the oldest remaining entry's prev_hash,
-- which the worker cross-checks against the WORM anchor for that day.
CREATE FUNCTION audit_verify(p_org uuid) RETURNS bigint LANGUAGE plpgsql STABLE AS $$
DECLARE v_prev bytea; r audit_logs;
BEGIN
    SELECT CASE WHEN seq = 1 THEN '\x'::bytea ELSE prev_hash END INTO v_prev
      FROM audit_logs WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq LIMIT 1;
    FOR r IN SELECT * FROM audit_logs WHERE org_id = p_org AND seq IS NOT NULL ORDER BY seq LOOP
        IF r.prev_hash IS DISTINCT FROM v_prev
           OR r.hash IS DISTINCT FROM sha256(v_prev || int8send(r.seq) || audit_entry_bytes(r)) THEN
            RETURN r.seq;
        END IF;
        v_prev := r.hash;
    END LOOP;
    RETURN NULL;
END $$;

-- Daily anchors: the head hash per org per day, also written to WORM storage (R2 object lock).
CREATE TABLE audit_anchors (
    org_id      uuid NOT NULL,
    day         date NOT NULL,
    last_seq    bigint NOT NULL,
    head_hash   bytea NOT NULL,
    object_key  text,                    -- r2://qrit-audit-worm/{org}/{day}.json
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, day)
);

CREATE TABLE audit_streams (
    id            uuid PRIMARY KEY,
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('webhook','splunk_hec','datadog','s3')),
    config        jsonb NOT NULL,         -- endpoint/region/bucket/prefix (no secrets)
    secret_ct     bytea,
    cursor_seq    bigint NOT NULL DEFAULT 0,   -- last streamed seq
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','error')),
    last_error    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------
-- E7. Integrations, alerts, scheduled reports
-- ---------------------------------------------------------------------
CREATE TABLE integrations (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider         text NOT NULL CHECK (provider IN (
                         'slack','teams','zapier','make','hubspot','salesforce','google_sheets','ga4',
                         'warehouse_s3','warehouse_gcs','warehouse_bigquery')),
    name             text NOT NULL,
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','error','disabled')),
    config           jsonb NOT NULL DEFAULT '{}'::jsonb,
    credentials_ct   bytea,                 -- OAuth tokens / API secrets, AES-256-GCM
    events           text[] NOT NULL DEFAULT '{}',
    last_success_at  timestamptz,
    last_error       text,
    created_by       uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX integrations_ws_idx ON integrations (workspace_id) WHERE status = 'active';

CREATE TABLE integration_deliveries (
    id               uuid PRIMARY KEY,
    integration_id   uuid NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL,
    event_type       text NOT NULL,
    status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed','dead')),
    attempts         integer NOT NULL DEFAULT 0,
    last_error       text,
    next_attempt_at  timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    delivered_at     timestamptz,
    UNIQUE (integration_id, event_id)
);

CREATE TABLE alert_rules (
    id                uuid PRIMARY KEY,
    workspace_id      uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name              text NOT NULL,
    kind              text NOT NULL CHECK (kind IN ('scan_spike','scan_drop','scan_threshold','no_scans',
                          'destination_down','serial_anomaly','security_event','approval_pending')),
    target_type       text NOT NULL CHECK (target_type IN ('workspace','qr_code','campaign','serial_batch')),
    target_id         uuid,
    params            jsonb NOT NULL DEFAULT '{}'::jsonb,   -- {"z": 3, "min_scans": 20} / {"threshold": 1000}
    channels          jsonb NOT NULL DEFAULT '{"emails":[],"integration_ids":[]}'::jsonb,
    cooldown_minutes  integer NOT NULL DEFAULT 60 CHECK (cooldown_minutes BETWEEN 5 AND 10080),
    is_active         boolean NOT NULL DEFAULT true,
    last_fired_at     timestamptz,
    created_by        uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alert_target CHECK ((target_type = 'workspace') = (target_id IS NULL))
);
CREATE TABLE alert_events (
    id         uuid PRIMARY KEY,
    rule_id    uuid NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    fired_at   timestamptz NOT NULL DEFAULT now(),
    payload    jsonb NOT NULL,
    resolved_at timestamptz
);
CREATE INDEX alert_events_rule_idx ON alert_events (rule_id, fired_at DESC);

CREATE TABLE report_schedules (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         text NOT NULL,
    frequency    text NOT NULL CHECK (frequency IN ('daily','weekly','monthly')),
    weekday      integer CHECK (weekday BETWEEN 1 AND 7),
    month_day    integer CHECK (month_day BETWEEN 1 AND 28),
    hour_local   integer NOT NULL DEFAULT 9 CHECK (hour_local BETWEEN 0 AND 23),
    timezone     text NOT NULL,
    filters      jsonb NOT NULL DEFAULT '{}'::jsonb,   -- same shape as analytics filters
    format       text NOT NULL DEFAULT 'pdf' CHECK (format IN ('pdf','csv','both')),
    recipients   citext[] NOT NULL CHECK (cardinality(recipients) BETWEEN 1 AND 50),
    is_active    boolean NOT NULL DEFAULT true,
    next_run_at  timestamptz NOT NULL,
    last_run_at  timestamptz,
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_schedules_due_idx ON report_schedules (next_run_at) WHERE is_active;

-- ---------------------------------------------------------------------
-- E8. Lead capture (encrypted PII, DPDP/GDPR consent) and retargeting pixels
-- ---------------------------------------------------------------------
CREATE TABLE org_data_keys (
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_id      integer NOT NULL,
    dek_ct      bytea NOT NULL,          -- data key wrapped with APP_ENCRYPTION_KEY (KMS later)
    created_at  timestamptz NOT NULL DEFAULT now(),
    retired_at  timestamptz,
    PRIMARY KEY (org_id, key_id)
);

CREATE TABLE forms (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            text NOT NULL,
    fields          jsonb NOT NULL,        -- [{key,type,label{en,hi},required,options}] ≤ 25
    notice          jsonb NOT NULL,        -- itemised consent notice per language + purposes[] + controller + grievance contact
    notice_version  integer NOT NULL DEFAULT 1,
    double_opt_in   boolean NOT NULL DEFAULT false,
    retention_days  integer NOT NULL DEFAULT 180 CHECK (retention_days BETWEEN 1 AND 1095),
    notify_emails   citext[] NOT NULL DEFAULT '{}',
    is_active       boolean NOT NULL DEFAULT true,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE form_submissions (
    id                 uuid PRIMARY KEY,
    form_id            uuid NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    workspace_id       uuid NOT NULL,
    qr_code_id         uuid,
    key_id             integer NOT NULL,
    data_ct            bytea NOT NULL,           -- AES-256-GCM(JSON answers) with the org DEK
    email_bidx         bytea,                    -- HMAC-SHA256(org blind-index key, lower(email)) for DSAR lookup
    consent            jsonb NOT NULL,           -- {notice_version, purposes_accepted[], lang, at}
    status             text NOT NULL DEFAULT 'confirmed'
                       CHECK (status IN ('pending_confirmation','confirmed','withdrawn','erased')),
    delete_after       date NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    confirmed_at       timestamptz,
    withdrawn_at       timestamptz
);
CREATE INDEX form_submissions_ws_idx    ON form_submissions (workspace_id, created_at DESC);
CREATE INDEX form_submissions_bidx_idx  ON form_submissions (email_bidx) WHERE email_bidx IS NOT NULL;
CREATE INDEX form_submissions_purge_idx ON form_submissions (delete_after) WHERE status <> 'erased';

CREATE TABLE dsar_requests (
    id              uuid PRIMARY KEY,
    org_id          uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('access','erasure','correction')),
    subject_bidx    bytea NOT NULL,
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','rejected')),
    due_at          timestamptz NOT NULL,
    result_file_id  uuid REFERENCES files(id),
    handled_by      uuid REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz
);

CREATE TABLE pixels (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider      text NOT NULL CHECK (provider IN ('meta','google','linkedin','tiktok')),
    external_id   text NOT NULL CHECK (external_id ~ '^[A-Za-z0-9_-]{3,40}$'),   -- pixel / tag id only, never code
    name          text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider, external_id)
);
CREATE TABLE qr_code_pixels (
    qr_code_id  uuid NOT NULL REFERENCES qr_codes(id) ON DELETE CASCADE,
    pixel_id    uuid NOT NULL REFERENCES pixels(id)   ON DELETE CASCADE,
    PRIMARY KEY (qr_code_id, pixel_id)
);
CREATE TABLE pixel_consent_daily (
    qr_code_id  uuid NOT NULL,
    day         date NOT NULL,
    shown       integer NOT NULL DEFAULT 0,
    accepted    integer NOT NULL DEFAULT 0,
    declined    integer NOT NULL DEFAULT 0,
    auto_fired  integer NOT NULL DEFAULT 0,     -- regions where notice-only applies (workspace setting)
    PRIMARY KEY (qr_code_id, day)
);

-- ---------------------------------------------------------------------
-- E9. GS1 Digital Link conformant resolver data
-- ---------------------------------------------------------------------
CREATE TABLE gs1_items (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain_id     uuid NOT NULL REFERENCES domains(id),
    qr_code_id    uuid REFERENCES qr_codes(id) ON DELETE SET NULL,   -- analytics anchor
    gtin          char(14) NOT NULL CHECK (gtin ~ '^[0-9]{14}$'),
    cpv           text,     -- AI 22
    lot           text,     -- AI 10
    serial        text,     -- AI 21
    title         text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (domain_id, gtin, cpv, lot, serial)
);
CREATE INDEX gs1_items_lookup_idx ON gs1_items (domain_id, gtin);

CREATE TABLE gs1_links (
    id            uuid PRIMARY KEY,
    item_id       uuid NOT NULL REFERENCES gs1_items(id) ON DELETE CASCADE,
    link_type     text NOT NULL CHECK (link_type ~ '^gs1:[A-Za-z]+$'),   -- gs1:pip, gs1:nutritionalInfo, gs1:recallStatus …
    href          text NOT NULL,
    title         text NOT NULL,
    hreflang      text[] NOT NULL DEFAULT '{}',
    media_type    text,
    is_default    boolean NOT NULL DEFAULT false,
    position      integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX gs1_links_one_default_uniq ON gs1_links (item_id) WHERE is_default;
CREATE INDEX gs1_links_item_idx ON gs1_links (item_id, link_type);

-- ---------------------------------------------------------------------
-- E10. Serialization / product authentication
-- ---------------------------------------------------------------------
ALTER TABLE qr_codes DROP CONSTRAINT qr_codes_content_type_check;
ALTER TABLE qr_codes ADD CONSTRAINT qr_codes_content_type_check CHECK (content_type IN (
    'url','text','email','phone','sms','whatsapp','wifi','vcard','event','upi','location',
    'links_page','file','app_store','gs1','form','serial_batch'));
ALTER TABLE qr_codes DROP CONSTRAINT qr_static_types;
ALTER TABLE qr_codes ADD CONSTRAINT qr_static_types CHECK (
    mode = 'dynamic' OR content_type NOT IN ('links_page','file','app_store','gs1','form','serial_batch'));

CREATE TABLE serial_batches (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    qr_code_id      uuid NOT NULL UNIQUE REFERENCES qr_codes(id),     -- anchor: analytics + domain + design
    name            text NOT NULL,
    gtin            char(14) CHECK (gtin IS NULL OR gtin ~ '^[0-9]{14}$'),
    quantity        integer NOT NULL CHECK (quantity BETWEEN 1 AND 10000000),
    rules           jsonb NOT NULL DEFAULT '{"max_scans":20,"multi_country_window_hours":24,"velocity_per_hour":10}'::jsonb,
    verify_page     jsonb NOT NULL DEFAULT '{}'::jsonb,   -- product name, image, batch info, support link
    status          text NOT NULL DEFAULT 'generating' CHECK (status IN ('generating','ready','void')),
    generated       integer NOT NULL DEFAULT 0,
    created_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE serial_codes (
    serial          text PRIMARY KEY CHECK (serial ~ '^[0-9A-HJKMNP-TV-Z]{12}$'),   -- 9 random + 3 MAC chars
    batch_id        uuid NOT NULL REFERENCES serial_batches(id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','flagged','void')),
    scan_count      integer NOT NULL DEFAULT 0,
    first_scan_at   timestamptz,
    first_country   char(2),
    first_city      text,
    last_scan_at    timestamptz,
    countries       char(2)[] NOT NULL DEFAULT '{}',
    flagged_reason  text
);
CREATE INDEX serial_codes_batch_idx ON serial_codes (batch_id, status);

ALTER TABLE scan_events ADD COLUMN serial text;   -- set for serial verifications

-- ---------------------------------------------------------------------
-- E11. Enterprise billing: contracts, GST-compliant invoices
-- ---------------------------------------------------------------------
CREATE TABLE contracts (
    id                  uuid PRIMARY KEY,
    org_id              uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                text NOT NULL,
    starts_on           date NOT NULL,
    ends_on             date NOT NULL CHECK (ends_on > starts_on),
    billing_interval    text NOT NULL CHECK (billing_interval IN ('month','quarter','year')),
    currency            char(3) NOT NULL CHECK (currency IN ('INR','USD','EUR','GBP')),
    amount_minor        bigint NOT NULL CHECK (amount_minor >= 0),     -- per interval, pre-tax
    seats               integer NOT NULL CHECK (seats > 0),
    limits_override     jsonb NOT NULL DEFAULT '{}'::jsonb,            -- merged over plan entitlements
    sla_uptime          numeric(5,3) NOT NULL DEFAULT 99.950,
    po_number           text,
    payment_terms_days  integer NOT NULL DEFAULT 30,
    status              text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','expired','terminated')),
    created_by          uuid REFERENCES users(id),
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contracts_one_active_uniq ON contracts (org_id) WHERE status = 'active';

-- GST: invoice numbers are unique and consecutive per financial year (April–March), ≤ 16 chars.
CREATE TABLE invoice_sequences (
    fy        text PRIMARY KEY CHECK (fy ~ '^[0-9]{4}-[0-9]{2}$'),   -- '2026-27'
    last_seq  integer NOT NULL DEFAULT 0
);
CREATE FUNCTION next_invoice_number(p_date date) RETURNS text LANGUAGE plpgsql AS $$
DECLARE v_fy text; v_seq integer; y integer;
BEGIN
    y := CASE WHEN extract(month FROM p_date) >= 4 THEN extract(year FROM p_date)::int
              ELSE extract(year FROM p_date)::int - 1 END;
    v_fy := y::text || '-' || lpad(((y + 1) % 100)::text, 2, '0');
    INSERT INTO invoice_sequences (fy, last_seq) VALUES (v_fy, 1)
    ON CONFLICT (fy) DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
    RETURNING last_seq INTO v_seq;
    RETURN 'QR/' || replace(v_fy, '-', '') || '/' || lpad(v_seq::text, 5, '0');   -- e.g. QR/202627/00042 (15 chars)
END $$;

CREATE TABLE invoices (
    id               uuid PRIMARY KEY,
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    contract_id      uuid REFERENCES contracts(id),
    number           text NOT NULL UNIQUE CHECK (length(number) <= 16),
    issue_date       date NOT NULL,
    due_date         date NOT NULL,
    currency         char(3) NOT NULL,
    tax_mode         text NOT NULL CHECK (tax_mode IN ('cgst_sgst','igst','export_lut','reverse_charge','none')),
    place_of_supply  text NOT NULL,          -- GST state code, or 'Outside India'
    seller           jsonb NOT NULL,         -- legal name, GSTIN, address, SAC 998314
    buyer            jsonb NOT NULL,         -- legal name, GSTIN (optional), address, state code
    lines            jsonb NOT NULL,         -- [{description, sac, qty, unit_minor, amount_minor}]
    subtotal_minor   bigint NOT NULL,
    cgst_minor       bigint NOT NULL DEFAULT 0,
    sgst_minor       bigint NOT NULL DEFAULT 0,
    igst_minor       bigint NOT NULL DEFAULT 0,
    total_minor      bigint NOT NULL,
    endorsement      text,                   -- 'Supply meant for export under LUT without payment of IGST'
    status           text NOT NULL DEFAULT 'issued' CHECK (status IN ('issued','paid','void')),
    pdf_file_id      uuid REFERENCES files(id),
    paid_at          timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoice_totals CHECK (total_minor = subtotal_minor + cgst_minor + sgst_minor + igst_minor),
    CONSTRAINT invoice_export_zero CHECK (tax_mode <> 'export_lut' OR (cgst_minor = 0 AND sgst_minor = 0 AND igst_minor = 0 AND endorsement IS NOT NULL))
);

-- ---------------------------------------------------------------------
-- E12. White-label, developer platform, support access, feature flags
-- ---------------------------------------------------------------------
CREATE TABLE org_branding (
    org_id               uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    app_hostname         citext UNIQUE,          -- dashboard on app.agency.com (Cloudflare for SaaS)
    app_hostname_status  text NOT NULL DEFAULT 'none' CHECK (app_hostname_status IN ('none','pending','active','failed')),
    provider_hostname_id text,
    email_domain         citext UNIQUE,          -- mail.agency.com (Resend domain, DKIM)
    email_domain_status  text NOT NULL DEFAULT 'none' CHECK (email_domain_status IN ('none','pending','active','failed')),
    email_provider_id    text,
    product_name         text,                   -- replaces "QRit" in UI/emails when set
    logo_file_id         uuid REFERENCES files(id),
    favicon_file_id      uuid REFERENCES files(id),
    primary_color        text CHECK (primary_color IS NULL OR primary_color ~ '^#[0-9A-Fa-f]{6}$'),
    support_url          text,
    hide_platform_brand  boolean NOT NULL DEFAULT false,
    updated_at           timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workspaces ADD COLUMN is_sandbox boolean NOT NULL DEFAULT false;   -- test-mode API keys operate only here

ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK (kind IN (
    'bulk_create','bulk_update','bulk_download','export_scans','export_qr_codes','print_sheet',
    'serial_generate','serial_export','gs1_import','warehouse_export','audit_export','dsar_export','lead_export','report_run'));

ALTER TABLE api_keys
    ADD COLUMN environment   text  NOT NULL DEFAULT 'live' CHECK (environment IN ('live','test')),
    ADD COLUMN ip_allowlist  cidr[] NOT NULL DEFAULT '{}';

CREATE TABLE api_usage_daily (
    api_key_id  uuid NOT NULL,
    day         date NOT NULL,
    requests    integer NOT NULL DEFAULT 0,
    errors      integer NOT NULL DEFAULT 0,
    throttled   integer NOT NULL DEFAULT 0,
    PRIMARY KEY (api_key_id, day)
);

CREATE TABLE support_access_grants (
    id          uuid PRIMARY KEY,
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    granted_by  uuid NOT NULL REFERENCES users(id),
    scope       text NOT NULL DEFAULT 'read' CHECK (scope IN ('read','read_write')),
    reason      text NOT NULL,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT support_grant_max_7d CHECK (expires_at <= created_at + interval '7 days')
);

CREATE TABLE feature_flags (
    key         text NOT NULL,
    org_id      uuid REFERENCES organizations(id) ON DELETE CASCADE,   -- NULL = global default
    enabled     boolean NOT NULL,
    updated_by  uuid REFERENCES users(id),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (key, org_id)
);
```

## 8.2 Enterprise queries (sqlc)

```sql
-- =====================================================================
-- Enterprise queries (sqlc: services/db/queries/enterprise_*.sql)
-- Every query below was executed against fixture data (plan §E-V).
-- =====================================================================

-- name: EffectivePermissions :many
-- All permissions a user holds in a workspace, from direct and group bindings.
-- folder_id NULL = workspace-wide; otherwise the grant applies to that folder subtree.
WITH RECURSIVE principal AS (
    SELECT 'user'::text AS principal_type, sqlc.arg(user_id)::uuid AS principal_id
    UNION ALL
    SELECT 'group', gm.group_id FROM group_members gm WHERE gm.user_id = sqlc.arg(user_id)::uuid
)
SELECT DISTINCT unnest(r.permissions) AS permission, b.folder_id
FROM role_bindings b
JOIN principal p ON p.principal_type = b.principal_type AND p.principal_id = b.principal_id
JOIN roles r ON r.id = b.role_id
JOIN org_members om ON om.org_id = b.org_id AND om.user_id = sqlc.arg(user_id)::uuid AND om.status = 'active'
WHERE b.workspace_id = sqlc.arg(workspace_id)::uuid;

-- name: FolderAncestors :many
-- A folder-scoped grant on F applies to F and every descendant; the service checks
-- whether any granted folder_id is in the ancestor chain of the target QR's folder.
WITH RECURSIVE chain AS (
    SELECT id, parent_id FROM folders WHERE id = sqlc.arg(folder_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
    UNION ALL
    SELECT f.id, f.parent_id FROM folders f JOIN chain c ON f.id = c.parent_id
)
SELECT id FROM chain;

-- name: GetResolvedLink :one
-- Redirect resolution. Only versions that need no approval or were approved are eligible.
SELECT q.id, q.workspace_id, q.domain_id, q.campaign_id, q.status, q.safety_status,
       q.starts_at, q.expires_at, q.scan_limit, q.total_scans, q.password_hash, q.fallback_url,
       w.timezone,
       v.id AS version_id, v.version_no, v.destination_kind, v.destination_url, v.rules, v.utm,
       (SELECT min(v2.effective_at) FROM qr_versions v2
         WHERE v2.qr_code_id = q.id AND v2.effective_at > now()
           AND v2.approval_status IN ('not_required','approved')) AS next_change_at
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
      AND v1.approval_status IN ('not_required','approved')
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = sqlc.arg(domain_id)::uuid AND q.short_code = sqlc.arg(short_code)::text;

-- name: GS1Linkset :many
-- Conformant-resolver lookup with qualifier inheritance: every item whose qualifiers are
-- NULL or equal to the request's is a candidate; more specific items rank first.
-- The service builds the RFC 9264 linkset from all rows, and picks the default link
-- (or the requested linkType / language) from the most specific item that has one.
SELECT i.id AS item_id, i.qr_code_id, i.title AS item_title,
       (i.cpv IS NOT NULL)::int + (i.lot IS NOT NULL)::int + (i.serial IS NOT NULL)::int AS specificity,
       l.link_type, l.href, l.title, l.hreflang, l.media_type, l.is_default, l.position
FROM gs1_items i
JOIN gs1_links l ON l.item_id = i.id
WHERE i.domain_id = sqlc.arg(domain_id)::uuid
  AND i.gtin = sqlc.arg(gtin)::text
  AND (i.cpv    IS NULL OR i.cpv    = sqlc.narg(cpv)::text)
  AND (i.lot    IS NULL OR i.lot    = sqlc.narg(lot)::text)
  AND (i.serial IS NULL OR i.serial = sqlc.narg(serial)::text)
ORDER BY specificity DESC, l.is_default DESC, l.position, l.link_type;

-- name: ScanSpikeCandidates :many
-- Anomaly detection (every 5 min): last complete 15-min bucket vs the same bucket on each of
-- the previous 7 days. Returns codes whose z-score >= z_min and scans >= min_scans (spike),
-- or scans <= baseline_mean * drop_ratio when the baseline is meaningful (drop).
WITH cur AS (
    SELECT qr_code_id, workspace_id, scans
    FROM scan_stats_15m
    WHERE bucket_start = sqlc.arg(bucket)::timestamptz
      AND (sqlc.narg(workspace_id)::uuid IS NULL OR workspace_id = sqlc.narg(workspace_id)::uuid)
), hist AS (
    SELECT s.qr_code_id, d AS days_ago, COALESCE(s.scans, 0) AS scans
    FROM generate_series(1, 7) d
    CROSS JOIN (SELECT DISTINCT qr_code_id FROM cur) c
    LEFT JOIN scan_stats_15m s
           ON s.qr_code_id = c.qr_code_id
          AND s.bucket_start = sqlc.arg(bucket)::timestamptz - make_interval(days => d)
), base AS (
    SELECT qr_code_id, avg(scans)::float8 AS mean, stddev_pop(scans)::float8 AS sd
    FROM hist GROUP BY qr_code_id
)
SELECT c.qr_code_id, c.workspace_id, c.scans, b.mean, b.sd,
       CASE WHEN b.sd > 0 THEN (c.scans - b.mean) / b.sd ELSE NULL END AS z
FROM cur c JOIN base b USING (qr_code_id)
WHERE c.scans >= sqlc.arg(min_scans)::int
  AND ((b.sd > 0 AND (c.scans - b.mean) / b.sd >= sqlc.arg(z_min)::float8)
       OR (b.sd = 0 AND c.scans >= GREATEST(b.mean * 3, sqlc.arg(min_scans)::int)));

-- name: RecordSerialVerification :one
-- Called synchronously by POST /v1/public/verify/{serial} (issued by the verify page's JS, so
-- link previews don't burn counts). Applies the batch rules atomically and returns the verdict.
-- Scan analytics for the same visit still flow through the normal stream/ingest path.
WITH b AS (
    SELECT sb.rules FROM serial_batches sb JOIN serial_codes sc ON sc.batch_id = sb.id
    WHERE sc.serial = sqlc.arg(serial)::text
)
UPDATE serial_codes sc SET
    scan_count    = sc.scan_count + 1,
    first_scan_at = COALESCE(sc.first_scan_at, sqlc.arg(at)::timestamptz),
    first_country = COALESCE(sc.first_country, sqlc.narg(country)::char(2)),
    first_city    = COALESCE(sc.first_city, sqlc.narg(city)::text),
    last_scan_at  = sqlc.arg(at)::timestamptz,
    countries     = CASE WHEN sqlc.narg(country)::char(2) IS NULL OR sqlc.narg(country)::char(2) = ANY(sc.countries)
                         THEN sc.countries ELSE sc.countries || sqlc.narg(country)::char(2) END,
    status        = CASE
        WHEN sc.status = 'void' THEN 'void'
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'flagged'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'flagged'
        ELSE sc.status END,
    flagged_reason = CASE
        WHEN sc.status IN ('flagged','void') THEN sc.flagged_reason
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'max_scans_exceeded'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'multi_country'
        ELSE NULL END
FROM b
WHERE sc.serial = sqlc.arg(serial)::text
RETURNING sc.serial, sc.status, sc.flagged_reason, sc.scan_count, sc.first_scan_at, sc.first_country, sc.first_city;

-- name: PendingApprovalsForApprover :many
-- The approver's inbox: pending requests in workspaces where they hold qr.destination.approve,
-- excluding their own requests and ones they already decided. Single-code requests show the
-- proposed destination; bulk requests show the number of versions they cover.
SELECT ar.id, ar.workspace_id, ar.kind, ar.reasons, ar.requested_by, ar.required_approvals,
       ar.expires_at, ar.created_at, q.name AS qr_name,
       v.destination_url, v.effective_at,
       (SELECT count(*) FROM qr_versions vv WHERE vv.approval_request_id = ar.id) AS version_count
FROM approval_requests ar
LEFT JOIN qr_codes q ON q.id = ar.qr_code_id
LEFT JOIN LATERAL (
    SELECT destination_url, effective_at FROM qr_versions
    WHERE approval_request_id = ar.id ORDER BY version_no DESC LIMIT 1
) v ON ar.kind <> 'bulk_update'
WHERE ar.status = 'pending' AND ar.expires_at > now()
  AND ar.workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[])
  AND ar.requested_by <> sqlc.arg(user_id)::uuid
  AND NOT EXISTS (SELECT 1 FROM approval_decisions d WHERE d.request_id = ar.id AND d.approver_id = sqlc.arg(user_id)::uuid)
ORDER BY ar.created_at;

-- name: FinalizeApproval :exec
-- Run in the decision transaction once approvals >= required_approvals.
-- Approved versions become eligible; a past effective_at is moved to the approval instant so
-- the approved version becomes current rather than slotting in behind newer versions.
WITH req AS (
    UPDATE approval_requests SET status = 'approved', decided_at = now()
    WHERE id = sqlc.arg(request_id)::uuid AND status = 'pending'
    RETURNING id
)
UPDATE qr_versions v SET approval_status = 'approved', effective_at = GREATEST(v.effective_at, now())
FROM req WHERE v.approval_request_id = req.id AND v.approval_status = 'pending';
```

## 8.3 Verification report

`tests/test_enterprise.py` builds a fresh database from the blueprint schema, loads pre-existing Phase 1–4 data, applies 00003, loads enterprise fixtures, runs every query above through the same parameter substitution sqlc performs, and asserts on the results. Result on PostgreSQL 16 (the syntax used is also valid on 17): **38/38 passed**.

| Area | Checks |
|---|---|
| Backfill | Org created per workspace with the plan carried over; owner + members in `org_members`; role bindings mirror memberships; subscription moved to org; default policy rows |
| Authorization | Analyst (direct) + reviewer (via group) workspace-wide; editor only on a folder subtree; folder ancestor chain; suspended org member has **no** permissions |
| Approvals | Pending version **not served**; requester self-approval blocked by trigger; pending version without request rejected; appears in reviewer inbox, hidden from requester; approved version served; no decisions after close; `next_change_at` ignores pending future versions |
| Audit chain | Sealer chains rows; verifies intact; UPDATE and DELETE blocked; **DBA edit with trigger disabled detected at the exact seq**; restoring original bytes re-verifies; verification still works after retention pruning |
| GS1 | Lot-level recall link becomes the default for that lot and inherits GTIN links; other lots get GTIN links only |
| Alerts | 40 scans vs ~5 baseline flagged (z ≈ 46); `min_scans` floor suppresses noise |
| Serialization | First scan authentic; same-country rescan stays active; second country within 24 h → `multi_country`; 21st scan → `max_scans_exceeded` |
| GST | Invoice numbers consecutive per financial year and reset on 1 April; malformed GSTIN rejected; export invoice without the LUT endorsement rejected |
| Regression | Blueprint ingest batch applied twice on the enterprise schema → counts unchanged (idempotent); migration applies on an empty database |

---

# 9. Phased delivery plan

**Prerequisite:** blueprint Phases 1–4 complete and green. Blueprint Phase 5 items (RLS, multi-region redirect, chaos drills) are folded into E8. Each phase ends with `make check` green, the listed acceptance criteria demonstrated, a tag `v2-e<N>`, and a changelog entry.

```
Week:  1   3   5   7   9   11  13  15  17  19  21
E0 ███                                             Foundations
E1    ██████                                       Identity (SSO · MFA · SCIM · policies gates)
E2          ████                                   Governance (RBAC UI · approvals · audit)
E6b             ██                                 Contracts + GST invoicing  ← "Enterprise Core" sellable (≈ week 8)
E3               █████                             Integrations · alerts · reports
E4                    ████                         Leads + consent · pixels
E5                        █████                    GS1 resolver · serialization
E6a                            ███                 White-label · agency
E7                               ███               Developer platform · bulk 100k · print
E8                                  ██████         Hardening · certification · launch
SOC 2 Type II:     [readiness ─────][observation ≥ 3 months ──────────][audit]
```

## E0 — Foundations (2 weeks)
- **Goals:** migration 00003; org layer in API and UI; entitlements v2; authz engine; envelope encryption; step-up; feature flags; domain-event fan-out; audit sealer + anchors.
- **Go packages:** `org`, `authz`, `entitlements` (v2: `Effective(org)` = plan ⊕ contract ⊕ workspace `disabled_features`), `crypto/envelope` (DEK wrap/unwrap, blind index), `auth/stepup`, `flags`, `events` (catalogue + `FanOut` worker), `audit` (catalogue, redaction, `AuditSealer`, `AuditAnchor`, verify).
- **Web:** org switcher in the sidebar header, org settings (general, members), and the "step-up" dialog component.
- **API:** `/v1/orgs*` (members, workspaces), `/v1/auth/step-up`, `/v1/orgs/{org}/audit-logs` (+ verify), `/v1/permissions`.
- **Tests:** `test_enterprise.py` in CI (a Go port under `services/db/tests`); authz unit tests (≥ 50 cases); router-introspection test (every route declares a permission and an audit action or `audit:none` with a justification).
- **Acceptance:** existing Phase 1–4 users log in and see their workspace inside an auto-created org with no behaviour change; the audit chain verifies.

## E1 — Identity (3 weeks)
- **Goals:** §6.1 complete: org domains, SAML via Polis, OIDC direct, JIT + group sync, enforcement + break-glass, MFA (TOTP + WebAuthn + recovery), session policies, IP allowlists, SCIM 2.0.
- **New deployable:** `sso-bridge` (Ory Polis, pinned image; Fly app `qrit-sso`; Postgres schema `polis`; env in §13).
- **Go packages:** `identity` (Provider interface; `polis`, `oidc`, `workos` stub), `mfa`, `scim` (handlers, filter parser, PATCH engine, ETags), `middleware/orggate`.
- **Web:**
  - login v2 (email-first → SSO discovery);
  - MFA enrolment and challenge screens;
  - Settings → Security (org policy form with lock-out guard);
  - Settings → SSO (IdP picker, guides, metadata upload, test login, enforcement toggle);
  - Settings → Directory sync (SCIM token shown once, endpoint URL, last sync, provisioned users list).
- **Tests:** Keycloak SAML in CI; Entra SCIM Validator run (document the results); Okta SCIM tests; account-takeover suite; each §7 identity gate.
- **Risks:** Polis upgrade cadence (pin + Renovate; run the SAML e2e before bumping); IdP quirks (per-IdP guides + attribute defaults).
- **Acceptance:** §6.1 acceptance.

## E2 — Governance (2 weeks)
- **Goals:** §6.2 access governance UI; §6.3 approvals end to end (single + bulk); workspace content policies; §6.4 audit UI, export, SIEM streams.
- **Go packages:** `approval` (Evaluate, Decide, Finalize, expiry job, override), `auditstream` (webhook, splunk_hec, datadog, s3 writers).
- **Web:** Roles, Groups, Access tab, Share folder, Explain access, Access review; Approvals inbox + diff view; Policy form; Audit timeline + verify badge + exports + streams.
- **Acceptance:** §6.2–§6.4 acceptance. **Start SOC 2 readiness** (policies, vendor inventory, access reviews produced by this phase's export).

## E6b — Contracts and GST invoicing (1 week, pulled forward)
- **Goals:** §6.12.1 (contracts, `InvoiceRun`, GST tax modes, invoice PDF with UPI QR, Razorpay payment links, dunning, seat true-up) + staff console contract/invoice screens.
- **Acceptance:** staff create a ₹6,00,000/year contract for a Maharashtra buyer billed from Uttar Pradesh → IGST 18% invoice numbered `QR/202627/000NN`. A USD contract for a US buyer → export-under-LUT invoice with the endorsement and zero tax.
- **Milestone: Enterprise Core is sellable.**

## E3 — Integrations, alerts, reports (2.5 weeks)
- **Goals:** §6.5 framework + Slack, Teams, Zapier/Make (REST hooks + Zapier app), HubSpot, Salesforce, Google Sheets, GA4 MP, warehouse export; alert rules + evaluation; scheduled PDF/CSV reports.
- **External setup:** Slack app, HubSpot developer app, Salesforce connected app, Google Cloud OAuth client (`drive.file`), Zapier developer account.
- **Acceptance:** §6.5 acceptance, plus each integration's "test" button delivering a sample event.

## E4 — Leads, consent, pixels (2 weeks)
- **Goals:** §6.6 forms builder, hosted form pages, encryption + blind index, consent + withdrawal, retention, DSAR, breach tooling; §6.7 pixel interstitial.
- **Acceptance:** §6.6 and §6.7 acceptance; a DPDP checklist review against the Rules (notice, withdrawal, retention, breach) recorded in `docs/compliance/dpdp.md`.

## E5 — GS1 resolver and serialization (2.5 weeks)
- **Goals:** §6.8 conformant resolver + catalog UI + CSV import + recall mode; §6.9 serial batches, MAC, verify page + API, anomaly flags, exports.
- **Acceptance:** §6.8–§6.9 acceptance; conformance results published.

## E6a — White-label and agency (1.5 weeks)
- **Goals:** §6.10 custom app host, email domain, hide platform brand, agency client orgs, consolidated billing.
- **Acceptance:** §6.10 acceptance.

## E7 — Developer platform, bulk, print (1.5 weeks)
- **Goals:** §6.11 API keys v2 (test mode + sandbox, IP allowlists, expiry policy, usage), SDK generation + publishing, 100k bulk with dry-run/update/resume, CMYK PDF, EPS, label sheets, print calculator.
- **Acceptance:** §6.11 acceptance.

## E8 — Hardening, certification, launch (3 weeks)
- **Security:**
  - OWASP ASVS L2 self-assessment;
  - external pen test (for Indian BFSI/government buyers use a **CERT-In empanelled** auditor);
  - fix all high/critical findings;
  - RLS on tenant tables (blueprint §8.2);
  - secret rotation drill;
  - dependency and container scanning gates.
- **Reliability:** multi-region redirect; DR drill (restore to staging, reconcile, verify audit chain against anchors); load tests with enterprise features on (approvals-heavy workspace, 10 M serial batch, GS1 resolver at 2k rps, pixel interstitial); SLA report generation.
- **Compliance:** VPAT (WCAG 2.2 AA audit + fixes); trust center (`trust.example.com`: security overview, subprocessors with change notifications, DPA, SOC 2 status/bridge letter, pen-test summary under NDA, status page link); security questionnaire pack (CAIQ-Lite / SIG-Lite answers).
- **Launch:** enterprise pricing page + "Talk to sales" flow, a demo org with sample data, sales deck, onboarding runbook (SSO/SCIM/domain setup call checklist).
- **Acceptance:** zero open high/critical findings; DR restore < 1 h with an intact audit chain; enterprise load tests pass SLOs; the first pilot customer completes SSO + SCIM + custom domain setup unaided using the docs.

---

# 10. Compliance and procurement program

| Item | Plan | Timing |
|---|---|---|
| **SOC 2 Type II** | Use a compliance-automation platform (Vanta or Drata) and a licensed CPA auditor. Scope: Security (mandatory) + Availability + Confidentiality. Type II needs an observation window of **≥ 3 months**; a realistic first report is 4–6 months after readiness. Budget **$20k–$80k** for the first year (auditor + tooling + remediation) | Readiness from E2; observation during E3–E8; report ≈ 2 months after E8 |
| Controls the product itself provides (evidence) | Access reviews (§6.2 export), MFA/SSO enforcement, audit chain + anchors, change management (GitHub PR reviews + CI gates), encryption at rest/in transit, backups + DR drill, incident runbooks, vendor list (subprocessors page), offboarding SLA (§6.1.5) | Built in E0–E8 |
| **India DPDP Act + Rules (notified Nov 2025)** | Consent notices (itemised, per language), withdrawal as easy as consent, retention/erasure, breach notification to the Data Protection Board and affected people with a detailed report **within 72 h**, security-log retention ≥ 1 year, processor contracts (our DPA). Consent Manager framework operational **13 Nov 2026**; **full compliance 13 May 2027** | E4 features + DPA update before 13 May 2027 |
| **GDPR / UK GDPR** | DPA with SCCs, subprocessors list with change notice, DSAR tooling (§6.6), records of processing, EU representative when EU revenue starts | E4 + legal |
| **Accessibility** | WCAG 2.2 AA audit → VPAT 2.5 (the format enterprise procurement asks for) | E8 |
| **Pen test** | Annual, external; CERT-In empanelled auditor for Indian regulated buyers; publish a summary letter | E8, then yearly |
| **Trust center** | Security overview, architecture diagram, subprocessors, DPA, SOC 2 status, pen-test letter (NDA), uptime history, `security.txt`, responsible disclosure | E8 |
| **Not offered** | HIPAA/BAA (no PHI by design), FedRAMP | Stated plainly in the trust center |

---

# 11. Enterprise packaging (proposal)

Market signals:
- Uniqode Business+ ≈ $399/month billed yearly, with SSO;
- Bitly Enterprise ≈ $10,000+/year (99.9% SLA, SSO, CSM, 2-year data);
- WorkOS-style identity costs ≈ $125–250 per customer per month if bought.

| Plan | Price (proposal) | Includes |
|---|---|---|
| Business (self-serve) | as in blueprint (~$39–49/mo, ₹2,999) | Blueprint features + approvals (1 approver), audit log 1 year, Slack/Teams/Zapier/Sheets, alerts, scheduled reports, forms, pixels |
| **Enterprise** | **from $6,000/year or ₹5,00,000/year** (annual contract, invoiced; 25 seats; overage per seat) | Everything: **SSO (SAML/OIDC) + SCIM included, no SSO tax**, enforced SSO, MFA policies, IP allowlists, custom roles + folder sharing, multi-approver workflows, tamper-evident audit + SIEM streaming + 7-year retention, HubSpot/Salesforce/GA4/warehouse, GS1 resolver, 10 custom domains, white-label dashboard + email domain, 99.95% SLA with credits, priority support, security review assistance |
| Add-ons | priced per deal | Serialization (per million codes/year), extra domains, agency client orgs (per client), dedicated short domain, EU data residency (when built), premium support (24×7) |

**Principles:** never charge for scans; never deactivate codes; include security features (SSO/SCIM/MFA/audit) in Enterprise rather than selling them piecemeal, since that is how we differentiate from the "SSO tax" pattern.

---

# 12. Risks and open questions

## 12.1 Risks

| # | Risk | Impact | Mitigation |
|---|---|---|---|
| ER1 | SSO/SCIM edge cases across IdPs (attribute names, Entra's string booleans, Okta's PATCH shapes) | Failed enterprise onboarding | Conformance suites (Entra validator, Okta tests) in the release checklist; per-IdP guides; "test connection" before enforcement |
| ER2 | Operating Ory Polis (upgrades, availability) | SSO outage = login outage for enforced orgs | 2 instances; health-checked; pinned versions; break-glass owners; WorkOS adapter behind the same interface as a fallback |
| ER3 | Scope: 21 weeks is aggressive for one developer | Slips | Enterprise Core first (≈ 8 weeks) and sellable; T2 modules can ship independently behind flags |
| ER4 | Gemini-generated security code (SAML callback, SCIM PATCH, crypto, authz) | Vulnerabilities | Mandatory human review list (§13), negative tests for every gate, external pen test before GA |
| ER5 | Consent prompts on the pixel interstitial reduce click-through | Customer dissatisfaction | Opt-in per code, measured accept rate and latency, clear docs; UTM append remains the default attribution |
| ER6 | Tax/legal errors in invoicing (SAC, e-invoicing threshold, LUT validity) | Compliance exposure | CA review before first invoice; configurable SAC; LUT renewal reminder each financial year; e-invoicing extension ready |
| ER7 | Audit chain operational mistakes (a retention run without anchors) | Unverifiable history | Anchors are written before any retention delete; retention job refuses to run if the day's anchor is missing |
| ER8 | Serialized codes at 10 M scale bloat Postgres | Cost/perf | ~60 B/row → 10 M ≈ 0.6 GB + index; partition `serial_codes` by batch hash if > 100 M rows; archive void/expired batches |

## 12.2 Open questions for you

1. **Enterprise price point:** is $6,000/year (₹5 lakh) the right starting anchor for your target buyers (Indian mid-market brands vs global enterprises)?
2. **Identity approach:** confirm Ory Polis (self-hosted, no per-connection fees) vs WorkOS (managed, ≈ $125+/connection/month).
3. **Legal entity and GST registration:** needed before E6b (invoices need a GSTIN, state code, SAC confirmation from your CA, and an LUT filed for export invoices).
4. **Which T2 modules matter most for your first customers?** If they're retail/FMCG, move E5 (GS1 + serialization) ahead of E3/E4; if marketing agencies, move E6a (white-label/agency) forward.
5. **First design partner:** having one real enterprise pilot during E1–E2 (to test SSO/SCIM with their actual IdP) de-risks the whole program more than anything else in this plan.

---

# 13. Gemini handoff

1. Keep using `GEMINI_PROMPT.md` for blueprint Phases 1–4.
2. For enterprise phases, start a new Gemini chat and paste **`GEMINI_PROMPT.md` followed by `GEMINI_PROMPT_ENTERPRISE.md`**. The add-on inherits every output rule and adds the enterprise stack, migration 00003 verbatim, the verified queries verbatim, the enforcement matrix, endpoints, UI, env vars, tests and the E0–E8 order.
3. Ask for one phase at a time (`Enterprise phase E0`), run `make gen && make check`, and also run `python3 tests/test_enterprise.py` (or its Go port) against the migrated database.
4. **Human review list (mandatory, line by line) before merging each phase:**
   - `identity/*` (SSO callback, linking rules)
   - `scim/*` (PATCH engine, filters)
   - `mfa/*`
   - `middleware/orggate.go`
   - `authz/*`
   - `approval/*`
   - `crypto/envelope/*`
   - `audit/*`
   - `gs1/parser.go`
   - `serial/mac.go`
   - `billing/invoice*.go`
   - the redirect pixel interstitial template and its CSP

## Sources

- [Uniqode — QR code solution for enterprise](https://www.uniqode.com/qr-code-generator/for-enterprise)
- [Bitly — enterprise solutions](https://bitly.com/pages/solutions/enterprise)
- [Linkly — Bitly Enterprise pricing (Feb 2026)](https://linklyhq.com/blog/bitly-enterprise-pricing)
- [PC Tech Magazine — QR code generators with SSO compared (Sep 2026)](https://pctechmag.com/2026/09/qr-code-generators-with-sso-single-sign-on-5-compared/)
- [Uniqode Help — retargeting with Meta Pixel](https://docs.uniqode.com/en/articles/6043414-retarget-users-using-qr-codes-with-uniqode-meta-facebook-pixel-integration)
- [Uniqode — QR code product authentication](https://www.uniqode.com/blog/qr-code-security/qr-codes-product-authentication)
- [SSOJet — WorkOS pricing at scale (Jul 2026)](https://ssojet.com/blog/workos-pricing-at-scale-50-100-200-connections)
- [Ory Polis (formerly BoxyHQ SAML Jackson) — GitHub](https://github.com/ory/polis)
- [crewjam/saml — GitHub](https://github.com/crewjam/saml) · [CVE-2022-41912 advisory](https://advisories.gitlab.com/pkg/golang/github.com/crewjam/saml/CVE-2022-41912/)
- [Microsoft Learn — develop a SCIM endpoint for Entra ID provisioning](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups)
- [GS1-Conformant Resolver Standard (Release 1.2.1, Aug 2026)](https://ref.gs1.org/standards/resolver/)
- [Google — send Measurement Protocol events (GA4)](https://developers.google.com/analytics/devguides/collection/protocol/ga4/sending-events)
- [Google Ads Help — consent mode updates for EEA traffic](https://support.google.com/google-ads/answer/13695607?hl=en)
- [Fisher Phillips — India's DPDP Rules: key deadlines](https://www.fisherphillips.com/en/insights/insights/indias-new-data-privacy-rules-are-here)
- [PIB — DPDP Rules, 2025 notified](https://static.pib.gov.in/WriteReadData/specificdocs/documents/2025/nov/doc20251117695301.pdf)
- [Tally — GST on software and SaaS exports](https://tallysolutions.com/gst/gst-software-saas-exports-international-clients/)
- [Vanta — SOC 2 for startups](https://www.vanta.com/collection/soc-2/soc-2-for-startups)
- [Microsoft 365 Message Center MC1181996 — Office 365 connectors retirement in Teams](https://mc.merill.net/message/MC1181996)
- [Cloudflare R2 — bucket locks (retention policies)](https://developers.cloudflare.com/r2/buckets/bucket-locks/)
