# QRit v2 — Production Blueprint

**Dynamic QR codes, scan analytics and a premium SaaS experience**

| | |
|---|---|
| Prepared | 25 September 2026 |
| Scope | Product research → architecture → data model → API → analytics → security → phased build plan → Gemini code-generation prompt |
| Starting point | The existing `qr_code_project` (QRit v1: Django 4.2 + DRF, Next.js 16, PostgreSQL, Stripe) |
| Companion files | `GEMINI_PROMPT.md` (paste into Gemini), `schema.sql`, `ingest.sql`, `analytics_queries.sql` |

**How to read this document.** Sections 1–3 explain *why*, sections 4–9 define *what*, section 10 defines *in what order*, and section 11 is the handoff prompt for Gemini. Every SQL block in this document was loaded into PostgreSQL and exercised with test data before being written down, including replaying the same ingest batch twice to prove counts do not double.

**Assumptions made explicit (change any of these and tell me — the plan adapts):**

1. **v2 is a rewrite in the same repository, not an in-place refactor.** v1 is treated as the working reference for features, UI and plan limits. Its code is not carried forward (reasons in §1.3).
2. **The product is aimed at India first and priced for the world.** v1 already has UPI and WhatsApp QR types, so INR pricing and UPI-friendly billing matter.
3. **You are the primary engineer**, with Gemini generating code and you reviewing it. The stack uses languages you already know: Go for the backend and TypeScript for the frontend.
4. **Launch scale:** up to about 5 million scans per month and 10,000 paying workspaces, with a clear, documented path beyond that. The design avoids over-building (no Kafka, no Kubernetes, no ClickHouse on day one) while never painting itself into a corner.
5. **Brand name "QRit" and the short domain are placeholders.** The domains used throughout are `example.com` (marketing and dashboard), `qr.example.com` (platform short links), `api.example.com` (public API) and `cdn.example.com` (images).

---

# 1. Executive summary

## 1.1 What we are building

A QR platform with three jobs, in order of importance:

1. **Never break a printed code.** Once a QR code is on a menu, a poster or ten thousand product boxes, the short link inside it is a promise. Everything else is secondary to that promise.
2. **Make the code editable and measurable.** You can change where it points at any time, with a full history. Every scan is counted honestly: bots and duplicates are removed, and no personal data is kept.
3. **Make the product feel premium.** The first QR code should take under 10 seconds with no signup. Downloads should be pixel-identical to the preview and checked for scannability before download. The dashboard should load in about a second.

## 1.2 The seven decisions that shape everything

| # | Decision | Why |
|---|---|---|
| D1 | **The redirect service is its own Go binary, separate from the API.** | The redirect runs on every scan and has a p99 budget under 50 ms. Keeping it apart means a slow dashboard query or an API deploy can never delay a scan. |
| D2 | **Scans go through a queue before touching Postgres** (Redis Streams → Go ingest worker → partitioned table + rollups). | The redirect never waits on a database write. Analytics survive restarts (v1 loses scans held in threads when the process dies). |
| D3 | **Destinations are immutable versions.** An edit creates a new version; a restore copies an old version into a new one. | Full audit history, one-click undo, scheduled switch-overs, and each scan is tied to the version that served it. |
| D4 | **Short codes are 7 upper-case Crockford base32 characters, and the whole short URL is encoded upper-case.** | QR "alphanumeric mode" only covers upper-case, so this saves one QR version. Measured: `HTTPS://QRIT.LINK/7K2M9QX` makes a 29×29 grid at error-correction level H versus 33×33 for the mixed-case form. That means about 14% larger modules at the same print size, which scans better with a logo. |
| D5 | **One TypeScript renderer shared by the browser and the server.** | The on-screen preview, PNG, SVG and PDF come from the same code, so what you see is exactly what prints. (v1's server download ignores all styling.) |
| D6 | **Privacy by construction.** No raw IP is stored. A visitor is a 16-byte hash made with a daily secret salt that is thrown away after 48 hours. No cookies are set on the redirect domain. | Supports GDPR and India's DPDP Act without a consent banner on scans, and turns privacy into a selling point. |
| D7 | **"Your printed codes never die."** Downgrading or cancelling never deactivates a dynamic code. Codes over the plan limit become read-only but keep redirecting. | The industry's biggest trust complaint is codes being switched off after a trial (§2.3). Redirects cost us almost nothing, so this promise is cheap to keep and a strong way to win customers. |

## 1.3 Where v1 stands (audit of `qr_code_project`)

v1 is feature-rich. It has workspaces, RBAC, campaigns, routing rules, custom domains, webhooks, lead pages, bulk jobs, GS1 and SSO scaffolding. It is also a good specification: v2's entitlements table, routing semantics and page inventory come straight from it. However, several of its foundations can't be patched into a production product:

| Area | v1 today | Consequence | v2 fix |
|---|---|---|---|
| Scan recording | `threading.Thread(daemon=True)` per scan, inside Gunicorn | Scans are lost on every deploy/restart; no backpressure | Buffered channel → Redis Stream → idempotent batch ingest (§7) |
| Personal data | Raw `ip_address`, full UA, lat/long stored per scan | GDPR/DPDP liability; no deletion story | Daily-salted visitor hash, city-level geo only (§7.4) |
| QR password | Stored and compared as plain text (`submitted_password != qr.password`) | Leaks on any DB exposure; timing-unsafe | argon2id hash + rate-limited attempts (§8.4) |
| Webhooks | Sent synchronously from the scan thread with `urllib`, to any URL | Server-side request forgery (SSRF) into internal networks; slow endpoints tie up threads | River job queue, SSRF-safe dialer, signed + retried (§8.6) |
| Destination history | `redirect_url` overwritten in place | No undo, no audit, no scheduling | Immutable `qr_versions` (§5) |
| Analytics reads | `COUNT(*)` over raw scans per dashboard load | Dashboard slows linearly with traffic | 15-minute + daily-dimension rollups (§7.6) |
| Bots / duplicates | UA "bot" becomes a device type and is still counted | Inflated numbers (link previews, email scanners) | Bot + duplicate classification, excluded from counts (§7.3) |
| Rate limiting | Per-process Python dict | Wrong as soon as there are 2 instances | Redis GCRA limiter (§8.5) |
| Auth tokens | JWT + refresh token in `localStorage` (zustand persist) | Any XSS steals sessions | httpOnly cookies, same-origin proxy, rotation with reuse detection (§8.3) |
| Rendering parity | Browser uses `qr-code-styling`; server uses `qrcode` (black/white, resized bitmap) | Downloads via API ≠ preview | Shared TS renderer + render service (§4.6) |
| Tenancy | Parallel user-scoped and workspace-scoped APIs; plan stored on both users and workspaces; nullable `workspace_id` on QR records | Authorization bugs waiting to happen | Workspace is the only tenant; every query is scoped (§8.2) |
| Framework | Django 4.2 | Security support ended April 2026 | Rewrite (Go) — see D1 and §12 |

## 1.4 Headline targets

| Metric | Target |
|---|---|
| Redirect latency (origin, cache hit) | p50 ≤ 10 ms, p99 ≤ 50 ms |
| Redirect availability | 99.95% monthly; redirects continue from cache during a database outage |
| Scan → visible in dashboard | p95 ≤ 5 s |
| Dashboard analytics query (90-day range, 1k QR codes) | p95 ≤ 300 ms |
| Landing-page generator JS | ≤ 120 KB gzip initial; LCP ≤ 1.8 s on mid-range Android over 4G |
| Time to first downloaded QR (new visitor) | ≤ 10 s, zero sign-up |

## 1.5 Build plan at a glance

Six phases (§10). Each is shippable and each ends with acceptance criteria you can test:

| Phase | Outcome | Rough effort (1 dev + Gemini) |
|---|---|---|
| 1. MVP | Static generator (no signup), accounts, workspaces, basic dynamic codes with a working redirect | 3 weeks |
| 2. Analytics | Event pipeline, rollups, per-QR and workspace analytics, bot/duplicate filtering | 2 weeks |
| 3. Dynamic QR | Versions, scheduling, rules/A-B, password, expiry, scan limits, hosted pages, custom domains | 3 weeks |
| 4. Premium dashboard | Campaigns, templates, bulk, teams, billing, API keys, webhooks, command palette, polish | 3 weeks |
| 5. Hardening | Abuse pipeline, load tests, security review, observability, backups/DR, v1 migration | 2 weeks |
| 6. Launch | Pricing live, SEO pages, docs, status page, staged rollout | 1 week |

---

# 2. Competitive research

Pricing and limits below were checked in September 2026 against vendor pages where they publish numbers, and against dated third-party reviews where they don't (sources at the end). Treat exact prices as a snapshot. Treat the patterns as the durable finding.

## 2.1 The field

| Product | Positioning | Free tier | Paid entry → top self-serve | Dynamic code allowance | Notable strength | Notable weakness |
|---|---|---|---|---|---|---|
| **QR Code Generator (qr-code-generator.com, part of Bitly)** | Mass-market, top SEO rank | Static only + 14-day trial of paid features | Starter / Advanced / Professional (EUR, prices not published on the page at time of check) | 2 → 50 → 250; Starter capped at 10k scans | Brand recognition, polished editor | **When the trial ends, dynamic codes are deactivated and point to a service page** (their own support article) |
| **Bitly (Connections Platform)** | Links first, QR second | 2 QR/month, no analytics | Core $10 → Growth $35 ($29 annual) → Premium $300 ($199 annual) | 5 → 10 → 200 QR created per month | Scans never limited; strong link analytics | QR creation allowances are tight; QR is secondary to links |
| **QR TIGER** | Feature breadth, bulk, API | 3 dynamic, 500 scans each, logo pop-up | Regular $7 → Advanced $16 → Premium $37 (annual) | 12 → 200 → 600 | Mature CSV bulk, white-label scan pages, API | Heavy gating (bulk needs Advanced), monthly billing ~38% more, free tier quickly exhausted |
| **Uniqode (ex-Beaconstac)** | Enterprise/compliance | Unlimited static, no dynamic, no analytics | Starter $9 → Core $49 (annual only); Plus / Business+ quoted | Tiered | SOC 2, HIPAA, SSO, integrations (HubSpot, Salesforce, GA) | Annual-only billing; custom domain as a $2,000/yr add-on |
| **Flowcode** | Brand/marketing, first-party data | 2 dynamic, 500 scans | Pro Plus $25 → Growth/Enterprise $250+ | 50 on Pro Plus, 6,000 scans | Landing pages, data capture, strong brand design | Basic design (shapes, frames) paywalled; support gated at $250/mo |
| **Hovercode** | Fair, simple, dev-friendly | 3 dynamic, unlimited scans, 3-month history | Pro $12 → Business $39 → Business Plus $99 | 100 total → 600/mo → 2,000/mo | No scan caps, custom domains from Pro, API/webhooks on Business | Smaller brand; fewer hosted-page types |
| **QR Code Monkey** (from general knowledge, not re-checked this month) | Free static designer | Unlimited static, no signup | — | none | Instant, high-res SVG/PNG/PDF/EPS, no email wall | No tracking or editing (static only) |

## 2.2 What the strongest products have in common

1. **The generator is the landing page.** The best-ranking tools put the input field, type tabs and a live preview above the fold. The visitor gets a real, downloadable result before being asked for anything.
2. **Dynamic is the business model; static is the funnel.** Every vendor gives away static codes and charges for editability, tracking and scale.
3. **Short-link redirect layer with 302s.** Dynamic codes encode a short URL on a vendor domain (or a customer's domain on higher tiers), which responds with a temporary redirect. Custom domains are sold on trust and deliverability: iOS and Android camera apps show the domain before opening, and a branded domain gets more taps than an unknown shortener.
4. **Hosted "page" types** (vCard, menu, PDF, link-in-bio, coupon, app-store router) raise average revenue per user: the vendor hosts the content, so the code stays dynamic and trackable.
5. **Analytics are simple and visual.** Scans over time, unique vs total, location (country/city), device/OS, time of day. The depth sold at higher tiers is export, API/webhooks, GA4/UTM integration and longer history, not fundamentally different metrics.
6. **Design customisation as a hook.** Dot shapes, eye shapes, colours/gradients, logo, frames with a call to action ("Scan me"). Templates and brand kits for teams.
7. **Enterprise positioning = compliance + control.** SSO/SCIM, audit logs, custom domains, SLAs, SOC 2/HIPAA, role-based access, bulk + API.
8. **Bulk + API for operations buyers** (retail SKUs, events, logistics): CSV upload, ZIP download, REST API.

## 2.3 What to avoid (industry trust failures)

| Anti-pattern | Who does it | Our rule |
|---|---|---|
| **Dynamic codes deactivated after a trial/cancellation** ("subscription trap"). The printed code starts showing a vendor page. | qr-code-generator.com documents this; many review sites and regional press call it out as the top complaint | **Never.** Codes keep redirecting on every plan including after cancellation; over-limit codes become read-only (D7). Stated on the pricing page. |
| "Free" generator that silently creates a *dynamic* trial code | Common among SEO-driven generators | Static vs dynamic is an explicit, labelled toggle; anonymous users can only create static codes, which live entirely in the pixels. |
| Download behind an email wall | Several | Static PNG/SVG download with zero sign-up. |
| Scan caps on paid plans | QR Code Generator Starter (10k), Flowcode Pro Plus (6k), QR TIGER free | Scans are never capped or charged. We gate on scale (number of dynamic codes, seats, domains, history), not on success. |
| Paywalling basic design | Flowcode | Colours, dot/eye shapes and logo are free. Premium = templates, brand lock, frames library, bulk, custom domain. |
| Large monthly-billing penalty / annual-only | QR TIGER (≈38%), Uniqode (annual only) | Monthly available on every plan; annual saves ~20%. |
| Custom domain as an expensive add-on | Uniqode ($2k/yr) | 1 custom domain included from Pro. |

## 2.4 What to emulate, and what to do better

**Emulate**
- QR Code Monkey's instant, no-signup static designer with vector exports.
- Hovercode's fairness: unlimited scans, custom domain early, clear limits.
- QR TIGER's bulk CSV mapping (column mapping preview, PNG/SVG/PDF ZIP).
- Uniqode's enterprise checklist (SSO, audit log, roles, custom domains, compliance docs).
- Bitly's link-level analytics clarity and the idea of a link *preview* page (`/{code}+`).

**Do better**
- **Scannability you can trust.** Every download is decoded by a real decoder in the browser before it is offered ("Verified scannable" badge). A live score warns about contrast, quiet zone, logo size and density. Competitors warn at best; none verify.
- **Honest numbers.** Bot/preview/duplicate hits are filtered *and shown* ("312 automated hits filtered"), which builds trust in the analytics.
- **Version history with undo and scheduled switch-overs.** Most competitors overwrite the destination.
- **Local-time-correct analytics** for every timezone, including +05:30 (IST), a real gap in tools that bucket by UTC hour.
- **Speed.** Sub-50 ms redirect at p99, 1-second dashboard, 120 KB landing bundle.
- **India-native touches:** UPI QR (static, NPCI `upi://pay` format), WhatsApp, INR pricing, and UPI Autopay subscriptions via Razorpay.
- **GS1 Digital Link** support as a Business feature, since retail is moving to 2D codes at the till under **GS1 Sunrise 2027**. The same QR must read as a GTIN at POS and open a product page on a phone.

## 2.5 Architecture patterns the leaders almost certainly use (inferred)

These are inferences from observable behaviour (response headers, redirect chains, latency, product features), not insider knowledge:

| Concern | Likely pattern | Evidence / reasoning | Our choice |
|---|---|---|---|
| Redirect | Dedicated edge/redirect service behind a CDN, `302`/`307`, `Cache-Control: no-store` | Scans must be counted, so responses can't be cached at the CDN | Go service behind Cloudflare, 302 + `no-store` |
| Link lookup | Key-value cache (Redis/edge KV) in front of a relational DB | Sub-50 ms redirects at scale | In-process LRU → Redis → Postgres |
| Event capture | Async queue/stream, then batch into an analytics store | Scans appear in dashboards after seconds, not instantly | Redis Streams → Go ingest → Postgres rollups (ClickHouse later) |
| Geo | CDN geo headers or MaxMind | City-level accuracy consistent with IP geolocation | Cloudflare headers first, MaxMind GeoLite2 fallback |
| Rendering | Client-side canvas/SVG for preview; server-side renderer for API/bulk | High-res EPS/PDF exports, bulk ZIPs | Shared TS renderer + Node render service |
| Hosted pages | Server-rendered templates on the short domain or a sub-path | vCard/menu pages load fast and are indexable-off | Next.js `/p/{code}` pages, ISR-cached |
| Multi-tenant | Workspace/org model with roles; custom domains via managed TLS (Cloudflare for SaaS, Caddy on-demand TLS, or AWS ACM) | Branded domains issued within minutes | Cloudflare for SaaS custom hostnames |

## 2.6 Design standards that follow from the research

1. **Result first:** the generator, preview and download work with zero friction.
2. **Say what it costs, never trap:** limits are visible in-product *before* they are hit.
3. **Print-grade output:** vector by default, correct quiet zone, verified decoding.
4. **Numbers you can defend:** documented definitions for "scan" and "unique scan".
5. **Calm, dense, fast UI:** a Linear/Vercel-grade dashboard aesthetic rather than a marketing-heavy one.
6. **Accessible:** WCAG 2.2 AA, keyboard-first, `prefers-reduced-motion`, dark mode.

---

# 3. UX/UI findings and specification

## 3.1 Landing page = generator (the conversion engine)

**Layout (desktop ≥ 1024 px):** a 2-column hero. Left column (60%): type tabs → content form → design accordion. Right column (40%): sticky preview card → scannability meter → download buttons. **Mobile:** single column; the preview collapses into a sticky bottom bar (64 px thumbnail + "Download" button) that expands into a sheet.

**Above-the-fold sequence (what the visitor sees in order):**
1. H1 "Free QR code generator — custom designs, no sign-up" plus a single line of trust copy: "Static codes work forever. Dynamic codes never get switched off."
2. Type tabs, with the 6 most used visible and the rest under "More": **URL · Text · Wi-Fi · vCard · WhatsApp · UPI** | More: Email, SMS, Phone, Event, Location, App store, PDF*, Links page*, GS1*. Items marked * need dynamic mode, so they show a small "Dynamic" chip.
3. The content form is pre-focused on the URL field. Pasting a URL immediately renders the preview (no "Generate" button; rendering is live).
4. **Static/Dynamic segmented control** directly above the preview:
   - *Static (free forever):* "The content is inside the code. It can't be edited or tracked."
   - *Dynamic:* "Edit the destination any time and see scan analytics. Free account, 3 codes."
   Choosing Dynamic while signed out opens the **inline auth sheet** (Google + email magic link). The in-progress design is saved to `sessionStorage` and attached to the new account's first dynamic code after signup. The visitor never loses their work.
5. Download row: **PNG** (primary), **SVG**, **PDF** (PDF is rendered server-side). The size selector defaults to 1024 px, and a print-size helper appears on hover ("Prints sharp up to 8.7 cm at 300 dpi").

**Below the fold:** a 3-step explainer, "Why dynamic?" with a mini analytics mock, a template gallery (12 presets you can click to apply), a use-case grid (restaurants, retail, events, real estate), pricing teaser, FAQ (with schema.org FAQ markup), footer. The per-type SEO pages (`/wifi-qr-code`, `/upi-qr-code`, …; v1 already has these) reuse the same generator with that tab preselected and type-specific copy and FAQ.

**Friction removed (vs competitors):** no email wall, no forced dynamic trial, no pop-ups, no ads, no watermark on static codes, no cookie wall. The generator never blocks the page (render is off the main thread only if logo decode > 16 ms).

## 3.2 The QR editor (used in both landing and dashboard)

Three stacked sections with a sticky preview:

**A. Content.** Each content type has its own form with type-specific validation, all built with react-hook-form and zod. Rules include:
- URL: auto-prefix `https://`; warn on `http://`; block `javascript:`, `data:` and similar schemes.
- Wi-Fi: SSID, password, encryption WPA/WEP/none, hidden flag. Encoded as `WIFI:T:WPA;S:...;P:...;H:false;;` with `\`-escaping of `;,:\"`.
- vCard: 3.0 fields.
- UPI: VPA validation `^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$`, payee name, optional amount and note. Encoded as `upi://pay?pa=…&pn=…&am=…&cu=INR&tn=…`.

For dynamic mode, the URL entered here becomes version 1's destination.

**B. Design** (accordion, first panel open):
1. *Presets:* 12 curated designs plus the workspace's templates. A locked brand template disables panels 2–5.
2. *Pattern:* module shape (square, dots, rounded, extra-rounded, classy, classy-rounded) and colour or gradient (linear/radial, 2 stops, rotation).
3. *Corners (finder patterns):* outer shape (square, rounded, circle, leaf) and inner shape (square, rounded, circle, dot), each with its own colour.
4. *Logo:* upload PNG/JPG/SVG ≤ 2 MB. SVG is rasterised server-side to a 512 px PNG so no script-bearing SVG is ever stored. Size slider from 10–30% of width (default 22%); "Clear modules behind logo" toggle (default on); padding.
5. *Frame:* none, bottom banner, top banner, rounded box, speech bubble. CTA text (max 24 chars, default "SCAN ME"), frame colour, text colour.
6. *Background:* colour or transparent (transparent is PNG/SVG only; a warning explains that print needs contrast).

**C. Finish.** Name (for dynamic), folder, campaign, tags, and the Save / Download buttons.

**Scannability meter** (always visible under the preview). Score 0–100 with a coloured label (Excellent ≥ 85, Good ≥ 70, Risky ≥ 50, Won't scan < 50), computed live from:

| Check | Rule | Effect |
|---|---|---|
| Contrast | WCAG contrast ratio of the darkest module colour vs background | ≥ 7 : 0 penalty; 4–7 : −10; 2–4 : −35 + warning; < 2 : **block download** |
| Inverted colours | Foreground lighter than background | −20 + warning ("Some older scanners can't read light-on-dark codes") |
| Quiet zone | < 4 modules | 2–3: −10; < 2: −30 |
| Logo size | Width ratio | > 0.25: −15; > 0.30: **block** |
| Error correction | Logo present and ECC < Q | Automatically raised to H (shown as info) |
| Density | QR version | > 10 : −10 ("Use a dynamic code to shorten the content") |
| **Decode check** | Rasterise at 256 px and 512 px, decode with `jsQR` (lazy-loaded Web Worker) | Fail → **block download** with "We couldn't read this design. Try more contrast or a smaller logo." Pass → "Verified scannable" badge |

**Behaviour contracts:**
- Preview re-renders synchronously on every input change. SVG generation for a version ≤ 10 code must take < 4 ms; the logo image decode is cached.
- The decode check is debounced to 400 ms after the last change and runs in a Web Worker so it never blocks input.
- Undo/redo (⌘Z / ⇧⌘Z) for design changes, with 50 steps held in the editor store.
- "Copy design" and "Paste design" (JSON on clipboard) for power users.

## 3.3 Dashboard information architecture

```
Sidebar (collapsible, 240px / 64px)
├─ Workspace switcher (avatar + name + plan chip)
├─ Overview            /w/{ws}
├─ QR codes            /w/{ws}/qr            ← default landing after login
├─ Analytics           /w/{ws}/analytics
├─ Campaigns           /w/{ws}/campaigns     (Business)
├─ Templates           /w/{ws}/templates
├─ Domains             /w/{ws}/domains
├─ ─────
├─ Team                /w/{ws}/settings/team
├─ Settings            /w/{ws}/settings/{general|billing|api-keys|webhooks|audit-log|security}
└─ Usage meter (e.g. "Dynamic codes 37 / 100") + Upgrade button
Top bar: breadcrumb · global search (⌘K palette) · "New QR code" (N) · help · avatar menu
```

**QR codes list (`/w/{ws}/qr`)**
- **Desktop table columns:** checkbox · 40 px thumbnail · name + type icon · short link (copy button) · destination host (truncated, full on hover) · 7-day sparkline · total scans · status pill · updated (relative time) · row menu.
- **Mobile:** cards with thumbnail, name, scans, and a status dot.
- **Filters bar:** search (debounced 250 ms, trigram on name, exact match on short code), type, status, folder, campaign, tag, and sort (newest, most scanned, name). Filter state lives in the URL query string.
- **Bulk select:** move to folder, tag, pause, resume, archive, download ZIP.
- Pagination is cursor-based with a "Load more" button (no page numbers). Keyboard: `/` focuses search, `j`/`k` move the row focus, `Enter` opens.
- **Empty state:** an illustration, "Create your first dynamic QR code", and a secondary "Import from CSV" action.

**QR detail (`/w/{ws}/qr/{id}`)** has a header card: large preview (click to download), name (editable inline), short link with copy/open/preview-page buttons, current destination with an **Edit destination** button, and status controls. Tabs:
1. **Overview.** KPI tiles (Total scans, Unique scans, Scans today, Last scan) with comparison to the previous period, a scans-over-time chart, and top countries and devices.
2. **Audience.** Country table + world map (lazy-loaded), cities, OS, browser, language, UTM source.
3. **Versions.** A vertical timeline. Each entry shows version number, destination, author, relative time, note, and a "Current", "Scheduled for …" or "Restored from v3" badge, plus a Restore button (with a confirm dialog). Scheduled versions can be cancelled.
4. **Rules** (Business). An ordered rule list with drag handles, a condition builder, and A/B split sliders that must total 100%. A **"Test a scan"** panel (country, device, OS, language, local time) calls the resolve-preview endpoint and highlights the rule that would match.
5. **Design.** The editor, pre-loaded. Saving creates a new design hash; the printed code doesn't change, and a warning explains that visual changes only affect new prints.
6. **Settings.** Password, schedule start, expiry, scan limit, fallback URL, UTM auto-append, domain (read-only after creation), archive/delete (danger zone).

**Edit destination dialog (the core dynamic action)**
- Shows the current destination and a new URL field with live validation (scheme, host, workspace allow-list) and an async safety check (spinner, then a ✓ or a reason).
- "When": *Now* (default) or *Schedule…* (date-time picker in the workspace timezone, which is displayed).
- Optional change note (max 200 chars).
- Primary button: "Update destination". On success, a toast says "Destination updated · v5" with an **Undo** action for 10 seconds, which restores v4 as v6. The cache is invalidated server-side, and the toast shows "Live in < 1 s".

## 3.4 Analytics UX

- **Sticky filter bar:** date range (Today, 7D, 30D, 90D, 12M, Custom; plan-limited ranges show a lock icon and an upsell tooltip), QR multi-select, campaign, country, device. The workspace timezone is shown ("Times in Asia/Kolkata").
- **KPI row (4 tiles):** Total scans · Unique scans · Avg. scans/day · Top location. Each has a delta vs the previous equal period (▲ 12.4% green / ▼ red, with the absolute number on hover) and a mini sparkline.
- **Main chart:** area chart of scans with a unique-scans line. Granularity is automatic (15-min buckets for ≤ 1 day, hourly for ≤ 7 days, daily for ≤ 180 days, weekly beyond) with a manual override. The tooltip shows both series and the local date.
- **Breakdown grid (2×3 cards):** Countries (flag + horizontal bar list, top 10 with "View all"), Cities, Devices, OS, Browsers, Referrers/UTM. Bar lists, not pie charts: easier to compare and accessible.
- **Heatmap:** weekday × hour-of-day (local), a 7×24 grid with an accessible table fallback.
- **Rules/A-B card** (if rules exist): scans per rule/variant and share of total.
- **Data-quality footnote:** "312 automated hits (link previews, security scanners) and 41 repeat scans within 10 s were excluded." with a link to the definitions page.
- **Live badge:** "● 3 scans in the last 5 min", polled every 10 s.
- **Export:** CSV of the current view (Pro), raw scan log (Business).
- **Empty state:** "No scans yet. Scan this code with your phone to test." with the QR code shown large, so the user can scan it off the screen and watch the counter increment live. This is a delightful first-run moment.

## 3.5 Conversion and upgrade patterns

- **Limits are visible early:** the usage meter in the sidebar and "3 of 3 dynamic codes used" in the editor. An upgrade card appears inline where the gated feature lives (locked Rules tab → card with a feature preview GIF and plan comparison), never as a surprise modal after work is done.
- **Server-driven gating:** `GET /entitlements` returns features and limits. The UI renders locked states from this data, and the API enforces the same table with a `402 upgrade_required` problem response (the v1 pattern, kept).
- **Trials:** a 14-day Pro trial with no card. When it ends, the workspace returns to Free, and codes beyond 3 dynamic become **read-only** (still redirecting, still counting scans, analytics visible for 30 days). The reminder email says exactly that.
- **Pricing page:** monthly/annual toggle (annual default, "save 20%"), a USD/INR switch that auto-detects locale, a feature matrix, and an FAQ that leads with "What happens to my codes if I cancel? They keep working."

## 3.6 Trust and security signals in the UI

- A short-link preview page at `/{code}+` shows the destination host, "Created with QRit", and **Report this code**. It's linked from a small "ⓘ" on hosted pages.
- Custom domains are prominent (branded short links get more taps than unknown shorteners).
- The footer carries a status page link, security/privacy pages, and a data-processing addendum (DPA) download.
- In-app: the audit log (Business), a sessions list with "sign out other devices", and two-factor authentication (TOTP) in Phase 5.

## 3.7 Visual design system

**Principles:** calm neutral surfaces, one accent colour, dense but breathable, motion that explains rather than decorates.

| Token | Light | Dark |
|---|---|---|
| `--bg` | `#FFFFFF` | `#0A0A0B` |
| `--bg-subtle` | `#F7F7F8` | `#111113` |
| `--surface` | `#FFFFFF` | `#16161A` |
| `--border` | `#E6E6EA` | `#26262C` |
| `--text` | `#0B0B0F` | `#F4F4F6` |
| `--text-muted` | `#5B5B66` | `#A1A1AA` |
| `--accent` | `#5B5BF7` (indigo) | `#7C7CFF` |
| `--accent-fg` | `#FFFFFF` | `#0A0A0B` |
| `--success` / `--warning` / `--danger` | `#16A34A` / `#D97706` / `#DC2626` | `#22C55E` / `#F59E0B` / `#EF4444` |
| Chart series | `#5B5BF7`, `#14B8A6`, `#F59E0B`, `#EC4899`, `#64748B` | lighter tints of the same |

- **Typography:** Geist Sans (UI) + Geist Mono (codes and short links), self-hosted via `next/font`. Scale: 12 / 13 / 14 (base) / 16 / 20 / 24 / 32 / 40, with line-height 1.5 for body and 1.2 for headings. Tabular numbers (`font-variant-numeric: tabular-nums`) in all metrics.
- **Spacing:** 4-pt base; component padding 12/16; section gaps 24/32.
- **Radius:** 6 (inputs), 10 (cards), 14 (dialogs).
- **Elevation:** borders first, shadows only for overlays (`0 8px 30px rgba(0,0,0,.12)`).
- **Motion:** 120 ms (hover), 180 ms (open/close), ease `cubic-bezier(.2,.8,.2,1)`; disabled under `prefers-reduced-motion`.
- **Components:** shadcn/ui (Radix primitives) as the base. Custom components: `KpiTile`, `BarList`, `TimeSeriesChart`, `Heatmap`, `StatusPill`, `CopyButton`, `QrThumb`, `UsageMeter`, `UpgradeCard`, `DateRangePicker`, `EmptyState`, `CommandPalette`.
- **Feedback:** skeletons for every async region (never spinners on full pages); optimistic updates for pause/resume/rename; toasts (sonner) bottom-right, with undo where reversible.
- **Accessibility:** focus rings 2 px accent; all charts have a "View as table" toggle; colour is never the only signal (icons with status pills); 44×44 px minimum touch targets.

## 3.8 Mobile experience

- The editor is fully usable on 360 px width: the bottom-sheet preview and one-thumb downloads.
- The dashboard list collapses to cards; the detail page's tabs become a horizontally scrollable segmented control.
- Hosted pages (vCard, links page, menu, PDF, event) are mobile-first: under 50 KB of HTML+CSS, system fonts plus one brand font, and no client JS except "Save contact" / "Add to calendar".
- The PWA manifest lets users add the dashboard to the home screen (useful for checking scans on the go).

---

# 4. Product architecture

## 4.1 High-level architecture

```
                                   ┌───────────────────────────── Cloudflare ─────────────────────────────┐
  Browser / Phone camera ─────────▶│ DNS · TLS · WAF · rate-limit rules · CDN · Cloudflare for SaaS (custom │
                                   │ hostnames) · R2 object storage (+ cdn.example.com)                    │
                                   └──────┬───────────────────────┬───────────────────────┬────────────────┘
                                          │ example.com           │ qr.example.com         │ api.example.com
                                          │ (marketing+dashboard) │ + customer domains     │ (public API)
                                          ▼                       ▼                        ▼
                              ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
                              │ web  (Next.js 16)    │  │ redirect  (Go)       │  │ api  (Go)            │
                              │ RSC marketing, SEO,  │  │ GET/HEAD/POST /{code}│  │ REST /v1, OpenAPI    │
                              │ dashboard, /p/{code} │◀─┤ /{code}+ preview     │  │ auth, RBAC, plans,   │
                              │ hosted pages         │  │ reverse-proxies /p/* │  │ QR, versions, rules, │
                              │ /api/v1/* ──rewrite──┼──┼──────────────────────┼─▶│ analytics queries,   │
                              └─────────┬────────────┘  │ LRU → Redis → PG     │  │ domains, billing,    │
                                        │               └───┬──────────┬───────┘  │ webhooks config      │
                                        │ render (preview   │ XADD      │ read     └───┬──────┬───────┬───┘
                                        │ is client-side)   ▼ (async)   │ (miss)       │      │       │
                                        │           ┌─────────────┐     │              │      │       │ HTTP
                                        │           │ Redis 7     │◀────┼──────────────┘      │       ▼
                                        │           │ cache, rate │     │  DEL/PUBLISH        │  ┌──────────────┐
                                        │           │ limits,     │     │  invalidation       │  │ render (Node)│
                                        │           │ Streams,    │     │                     │  │ SVG/PNG/PDF  │
                                        │           │ pub/sub     │     │                     │  │ shared TS    │
                                        │           └──────┬──────┘     │                     │  │ renderer     │
                                        │      XREADGROUP  │            │                     │  └──────┬───────┘
                                        │                  ▼            ▼                     ▼         │ PUT
                                        │           ┌─────────────┐  ┌──────────────────────────┐       ▼
                                        │           │ ingest (Go) │─▶│ PostgreSQL 17            │   R2 bucket
                                        │           │ UA parse,   │  │ OLTP + partitioned       │  (images, logos,
                                        │           │ bots, dedup,│  │ scan_events + rollups    │   exports)
                                        │           │ COPY, rollup│  │ + River job tables       │
                                        │           └─────────────┘  └──────────▲───────────────┘
                                        │                                       │
                                        │           ┌─────────────────────────┐ │
                                        └──────────▶│ worker (Go, River)      │─┘ webhooks · emails · bulk ·
                                                    │ scheduled + async jobs  │   exports · domain checks ·
                                                    └─────────────────────────┘   safety rescans · partitions
External: Resend (email) · Stripe + Razorpay (billing) · Google Web Risk API (URL safety) · MaxMind GeoLite2 (geo/ASN, local file)
```

**Six deployables, two languages:**

| Service | Language | Responsibility | Scales on |
|---|---|---|---|
| `web` | TypeScript (Next.js 16, Node 24) | Marketing + SEO pages, dashboard UI, hosted pages `/p/{code}`, same-origin proxy to `api` | Page views |
| `api` | Go 1.27 | Control plane: auth, workspaces, QR CRUD, versions, rules, domains, analytics reads, billing, keys, webhooks config, entitlements | Dashboard/API traffic |
| `redirect` | Go 1.27 | Data plane: resolve short code → evaluate state/rules → emit scan event → 302. Serves password form, status pages, `/{code}+` preview | Scans |
| `ingest` | Go 1.27 | Consume scan stream → enrich (UA, bot, duplicate) → idempotent batch write → rollups → realtime counters → webhook fan-out | Scan volume |
| `worker` | Go 1.27 (River) | Durable jobs: webhook delivery, email, bulk create, exports, domain verification, safety rescans, scheduled version activation, partition maintenance, retention | Job volume |
| `render` | TypeScript (Node 24, Fastify) | Stateless SVG/PNG/PDF rendering using the same `@qrit/qr-render` package as the browser | Bulk/API renders |

All Go services live in **one Go module** with four `cmd/` entry points. Domain logic, most importantly the rule engine and URL validation, is shared by `api` (for previews) and `redirect` (for real scans), so the two can never disagree.

## 4.2 Request flows

**A. Create a dynamic QR code** (`POST /v1/workspaces/{ws}/qr-codes`, `Idempotency-Key` header required)
1. Auth middleware resolves the principal (session cookie or API key) → RBAC check `qr:write` (editor+).
2. Idempotency middleware: if `(ws, key)` exists with the same request hash → replay the stored response; different hash → `409 idempotency_key_reused`.
3. Entitlements: `count(dynamic, not deleted)` < plan limit, else `402 limit_reached`.
4. Validate content (per-type zod-equivalent Go validators) and destination URL (§8.1). Check the workspace allow-list if set.
5. Safety: Google Web Risk `uris.search` with an 800 ms timeout. `SAFE` → `safe`; a threat → `422 destination_unsafe`; timeout → `pending` plus a River job to re-check within 1 minute (the code works meanwhile; if the re-check flags it, the code is blocked and the owner emailed).
6. Generate the short code (§4.5); retry up to 5 times on unique violation.
7. One transaction: insert `qr_codes`, insert `qr_versions` v1, set `current_version_id`, attach tags, insert `audit_logs`, and enqueue River jobs transactionally (safety re-check if pending, webhook `qr.created`).
8. After commit: write the resolved-link cache entry to Redis (warm cache, so the first scan is a hit).
9. `201` with the QR resource including `short_url`, `image_urls` (svg/png/pdf) and `version`.

**B. Edit the destination** (`POST …/qr-codes/{id}/versions`)
1. RBAC `qr:write`; reject if `is_read_only` (`402 read_only_over_limit`) or `status = blocked`.
2. Validate + safety check as above.
3. Transaction: `SELECT … FROM qr_codes WHERE id=$1 AND workspace_id=$2 FOR UPDATE` → `version_no = max + 1` → insert the version. If `effective_at <= now()`, set `current_version_id`; else enqueue River job `ActivateVersion` scheduled at `effective_at`. Audit log `qr.version.created` with before/after destination.
4. After commit: `DEL link:{domain_id}:{code}` and `PUBLISH qr:invalidate {domain_id}:{code}`. The redirect instances evict their in-process entry immediately; worst-case staleness if a message is lost is the 30-second in-process TTL.
5. The web app shows the "Destination updated · v5 · Undo" toast.

**C. Scan** — the full lifecycle is in §7.1.

**D. Download image** (`GET …/qr-codes/{id}/image?format=png&size=2048`)
1. `render_key = sha256(canonical(payload) + canonical(design) + format + size + RENDERER_VERSION)`.
2. `HEAD r2://qrit-assets/qr/{render_key}.{ext}` → exists: `302` to `https://cdn.example.com/qr/{render_key}.{ext}` (`Cache-Control: public, max-age=31536000, immutable`).
3. Miss: `POST render:8080/render` → `PUT` to R2 → `302`. For a dynamic code the payload is the short URL, which never changes, so the image changes only when the design does.

**E. Analytics query**: `api` → Redis response cache (60 s; 5 min when the range ends before today) → rollup queries (§7.6) → JSON. Never scans raw events except for ranges under 3 days and the paginated raw log.

## 4.3 Cache strategy

| Data | Layer | Key | TTL | Invalidation |
|---|---|---|---|---|
| Resolved link (see §7.2 payload) | `redirect` in-process LRU (100k entries, ~40 MB) | `{host}/{code}` | 30 s, capped at `next_change_at` | Redis pub/sub `qr:invalidate` |
| Resolved link | Redis | `link:v1:{domain_id}:{code}` | 10 min | `DEL` after every write that affects resolution |
| Unknown code (negative) | LRU + Redis | same key, value `"-"` | 60 s | `DEL` on create |
| Hostname → domain | `redirect` in-process map | hostname | refresh every 60 s | pub/sub `domain:invalidate` |
| Last-known-good links | `redirect` in-process | `{host}/{code}` | 24 h, used **only** when Redis *and* Postgres are unreachable | — |
| Daily visitor salt | Redis `salt:{yyyy-mm-dd}` (SET NX, 48 h TTL) + in-process | date | 48 h | — (rotates by date) |
| Entitlements | `api` in-process | workspace id | 30 s | pub/sub `ent:invalidate` from billing webhooks |
| Analytics responses | Redis | `an:{ws}:{sha1(query)}` | 60 s / 300 s | TTL only |
| QR images | R2 + Cloudflare CDN | `qr/{render_key}.{ext}` | 1 year, immutable | Content-addressed |
| Hosted pages | Next.js ISR (`/p/{code}`) | tag `qr:{id}` | 300 s | `api` calls `POST web/api/revalidate` (shared secret) after version change |
| Marketing/SEO pages | Static + CDN | — | until deploy | Deploy |

## 4.4 Frontend architecture (`apps/web`)

- **Next.js 16 App Router**, React 19, TypeScript `strict`, Tailwind CSS v4, shadcn/ui (Radix).
- **Rendering split:** marketing, pricing, SEO type pages and hosted pages are **React Server Components** (static or ISR, zero auth). The dashboard is client-rendered inside RSC layouts. Server state uses **TanStack Query v5** (stale-while-revalidate, optimistic updates); editor state uses **Zustand**; URL state (filters, date range) uses `nuqs`.
- **API access:** `openapi-fetch` + types generated by `openapi-typescript` from `api/openapi.yaml`. Calls go to the same-origin `/api/v1/*`; `next.config.ts` rewrites these to `API_INTERNAL_URL`. Cookies are therefore first-party and CORS is unnecessary for the app.
- **Auth guard:** `src/proxy.ts` (Next.js 16's replacement for `middleware.ts`) redirects `/w/*` to `/login?next=…` when the `qrit_rt` cookie is absent (a UX guard only; the API is the authority).
- **Charts:** Recharts, dynamically imported per chart; the world map uses `d3-geo` + `world-atlas` 110m TopoJSON, lazy-loaded only on the Audience tab.
- **Bundle discipline:** the landing route imports only the generator, the renderer and the `qrcode` matrix library. `jsQR` (decode check), `recharts`, `d3-geo` and the logo cropper are dynamic imports. The budget is enforced in CI with `@next/bundle-analyzer` + size-limit.

```
apps/web/src/
  app/
    (marketing)/page.tsx                      # generator landing
    (marketing)/[type]-qr-code/page.tsx       # SEO pages (generateStaticParams over content types)
    (marketing)/pricing/page.tsx
    (marketing)/legal/{privacy,terms,dpa,acceptable-use}/page.tsx
    (auth)/{login,register,verify-email,reset-password,magic}/page.tsx
    invite/[token]/page.tsx
    w/[workspace]/layout.tsx                  # AppShell: sidebar, topbar, command palette
    w/[workspace]/page.tsx                    # overview
    w/[workspace]/qr/page.tsx                 # list
    w/[workspace]/qr/new/page.tsx             # editor
    w/[workspace]/qr/[id]/{page,audience,versions,rules,design,settings}/page.tsx
    w/[workspace]/analytics/page.tsx
    w/[workspace]/campaigns/{page,[id]/page}.tsx
    w/[workspace]/templates/page.tsx
    w/[workspace]/domains/page.tsx
    w/[workspace]/settings/{general,team,billing,api-keys,webhooks,audit-log,security}/page.tsx
    p/[code]/page.tsx                         # hosted pages (vcard, links, file, event, menu)
    admin/abuse/page.tsx                      # staff only
    api/revalidate/route.ts                   # ISR on-demand revalidation (secret)
  features/
    qr-editor/{store.ts, schema.ts, content-forms/*, design-panel/*, preview/*, scannability/*}
    qr-list/  qr-detail/  analytics/  campaigns/  templates/  domains/
    team/  billing/  api-keys/  webhooks/  audit-log/  auth/
  components/{ui/*, layout/*, charts/*, feedback/*}
  lib/{api/client.ts, api/query-keys.ts, auth.ts, env.ts, format.ts, tz.ts, entitlements.ts}
  styles/globals.css                          # tokens from §3.7
```

## 4.5 Short codes and domains

- **Alphabet:** Crockford base32 upper-case `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (32 symbols; no I, L, O, U). **Length 7** → 32⁷ ≈ 34.4 billion codes. Generated with `crypto/rand`, no sequential ids (no enumeration).
- **Collision handling:** unique index `(domain_id, short_code)`, retry ≤ 5, alert if retries > 1 happen more than 0.1% of the time.
- **Case-insensitive resolution:** the redirect upper-cases the path, and maps look-alike characters (`O→0`, `I/L→1`). Printed codes always carry the upper-case URL, `HTTPS://QR.EXAMPLE.COM/7K2M9QX`, so the payload uses QR alphanumeric mode (D4).
- **Reserved codes:** a denylist of offensive words, checked on the decoded string; `HEALTH01` is reserved as the synthetic-monitoring canary.
- **Never reused:** the unique index includes soft-deleted rows; hard purges copy codes to `short_code_tombstones`.
- **Domains:** the platform short domain `qr.example.com` is separate from the app domain for **reputation isolation**: if abuse gets the short domain flagged, the app and email domains stay clean, and vice versa. Phase 5 adds a second platform short domain for **free-tier** dynamic codes, so paid customers' links never share reputation with anonymous free signups. Custom domains: the customer adds `CNAME go.brand.com → domains.qr.example.com`; we create a Cloudflare for SaaS custom hostname, poll until the TLS certificate is active, then mark the domain `active`. `https://go.brand.com/` → `root_redirect_url`; unknown code → `not_found_url` or the branded 404.

## 4.6 QR rendering

One pure TypeScript package, `packages/qr-render`, exports:

```ts
export function renderSvg(input: { payload: string; design: DesignV1; sizePx?: number }): {
  svg: string;             // standalone SVG, viewBox in modules, shapes as <path>
  version: number;         // QR version chosen
  ecc: 'L' | 'M' | 'Q' | 'H';
  modules: number;         // grid size incl. no quiet zone
  warnings: ScanWarning[]; // same checks as the scannability meter (minus decode)
};
```

- **Matrix:** `qrcode` (node-qrcode) `QRCode.create(payload, { errorCorrectionLevel })`. It picks the most efficient segment modes automatically, so upper-case URLs get alphanumeric mode.
- **Shapes:** each module shape is a function `(x, y, neighbours) → path d`; neighbours enable rounded/"classy" joins. Finder patterns are drawn separately (outer 7×7 ring + inner 3×3) and excluded from the module pass. With `logo.clear_modules`, modules intersecting the logo box plus padding are skipped.
- **Gradients:** SVG `<linearGradient>` / `<radialGradient>` with deterministic ids (`g-{hash}`) so multiple previews on one page don't collide.
- **Frames:** the QR is placed inside a frame template SVG with the CTA text rendered as `<text>` (Geist, embedded as a subset in PDF output).
- **Server outputs** (`apps/render`): PNG via `@resvg/resvg-js` (fonts loaded from disk), PDF via `pdfkit` + `svg-to-pdfkit` (vector, embedded font), SVG as-is. EPS is deferred (open question §13).
- **Determinism test:** every preset × 5 payloads renders a snapshot SVG, and the PNG must decode back to the exact payload with `jsQR` in CI (§10 testing).

## 4.7 Backend codebase structure (`services/`, Go module `github.com/<you>/qrit/services`)

```
services/
  cmd/{api,redirect,ingest,worker,migrate}/main.go
  internal/
    config/            # typed env config (caarlos0/env), validated at boot, fail fast
    platform/
      db/              # pgxpool setup, tx helper (WithTx), health
      redisx/          # go-redis client, pub/sub helper, GCRA limiter
      httpx/           # router (chi), middleware: request id, recover, logging, CORS (api only), problem+json
      obs/             # slog JSON logger, OpenTelemetry tracer + Prometheus metrics
      clock/ idgen/    # injectable time; UUIDv7 + short-code generator
      crypto/          # argon2id, AES-GCM secrets, HMAC helpers
    auth/              # sessions, cookies, JWT (EdDSA), OAuth Google, magic link, API keys, principal
    rbac/              # role → permission matrix, Require(perm) middleware
    workspace/         # workspaces, members, invites
    entitlements/      # plan table (code), Check/Limit helpers, 402 problems
    qr/                # QR aggregate: service, repository (sqlc), handlers, content encoders per type
    version/           # immutable versions, scheduling, restore
    routing/           # rule engine — PURE, shared by api + redirect
    urlsafety/         # URL normalisation/validation, Web Risk client, allow/deny lists
    shortcode/         # alphabet, generation, normalisation, denylist
    domains/           # custom domains, Cloudflare for SaaS client, verification
    resolve/           # link resolution + cache (LRU, Redis, PG), invalidation listener — used by redirect
    scan/              # event schema (protobuf-free JSON), visitor hashing, salts, bot detection, UA parse
    ingest/            # stream consumer, batcher, pipeline SQL, DLQ
    analytics/         # query service over rollups, tz handling, response cache
    realtime/          # per-minute Redis counters
    webhooks/          # config CRUD, signer, SSRF-safe dialer, River delivery worker
    billing/           # Provider interface; stripe/, razorpay/; webhook handlers; plan sync
    render/            # client for render service; render_key; R2 storage
    storage/           # S3-compatible client (R2 / MinIO)
    email/             # Resend client + templates (html/template)
    audit/             # Record(ctx, action, target, before, after)
    abuse/             # reports, staff actions, auto-block
    jobs/              # River client, job args + workers registry
    idempotency/       # middleware + store
    apierr/            # typed errors → RFC 9457 problem details
  db/
    migrations/        # goose SQL: 00001_init.sql (= schema.sql), 00002_…
    queries/           # sqlc query files by aggregate
    sqlc.yaml
  api/openapi.yaml     # (symlink to /api/openapi.yaml)
```

Layering rule: `handler → service → repository`. Handlers parse/validate and map errors; services hold business rules and transactions; repositories are sqlc-generated plus thin wrappers. Services depend on interfaces (`Clock`, `IDGen`, `SafetyChecker`, `Cache`) for testability. **No package imports `handler` code from another domain package; cross-domain calls go service → service.**

## 4.8 Configuration, errors and logging conventions

- **Config:** each binary parses only its own env struct at boot and exits non-zero with a list of missing/invalid variables. Secrets are never logged; the struct has a `Redacted()` method for startup logging. Web uses `src/lib/env.ts` (zod) with a separate `server`/`client` schema so a server secret can't be bundled.
- **Errors:** all API errors are `application/problem+json` (RFC 9457):
  ```json
  { "type": "https://docs.example.com/errors/limit_reached", "title": "Plan limit reached",
    "status": 402, "code": "limit_reached", "detail": "Your Free plan allows 3 dynamic QR codes.",
    "instance": "req_01J8…", "limit": 3, "current": 3, "required_plan": "pro" }
  ```
  Validation errors add `"errors": [{"field": "destination_url", "code": "scheme_not_allowed", "message": "…"}]`. Stable `code` values are the contract; `title`/`detail` are for humans.
- **Logging:** `log/slog` JSON with `service, env, request_id, trace_id, workspace_id, principal_id, route, status, duration_ms`. Redirect access logs are sampled (1% of 2xx/3xx, 100% of 4xx/5xx). **No IP addresses in logs**, only `/24` prefixes.
- **Tracing:** OpenTelemetry (OTLP → Grafana Cloud). Trace ids propagate `web → api → render/Postgres/Redis`.
- **Metrics:** Prometheus `/metrics` on an internal port for every Go service and `render` (see §9.5 for the SLO metrics).

## 4.9 Deployment

| Component | Platform | Size (launch) | Notes |
|---|---|---|---|
| Edge | Cloudflare (Pro plan) | — | DNS, WAF managed rules, rate-limit rules on `/v1/auth/*`, Cloudflare for SaaS for custom hostnames, R2 + custom CDN hostname |
| `web` | Fly.io, region `bom` (Mumbai) | 2 × shared-cpu-2x, 1 GB | `output: 'standalone'`; health `/api/health` |
| `api` | Fly.io `bom` | 2 × shared-cpu-2x, 512 MB | `release_command = "/app/migrate up"` (goose) |
| `redirect` | Fly.io `bom` (+ `sin`, `fra`, `iad` in Phase 5) | 2 × performance-1x, 512 MB, autoscale 2→10 | Anycast via Fly; min 2 machines always on |
| `ingest` | Fly.io `bom` | 1 × shared-cpu-2x (2 for HA later) | Consumer group handles failover |
| `worker` | Fly.io `bom` | 1 × shared-cpu-2x | River leader election built in |
| `render` | Fly.io `bom` | 2 × shared-cpu-2x, 1 GB | Autoscale on CPU |
| PostgreSQL 17 | Neon (AWS `ap-south-1`), direct (non-pooled) connections | 2 CU, autoscaling; read replica in Phase 5 | Point-in-time restore (window depends on plan — pick ≥ 7 days); branch per preview env |
| Redis 7 | Upstash Redis via Fly (same region) | pay-as-you-go | Streams, pub/sub, cache, limits. Move to self-hosted Redis on Fly when command costs exceed ~$50/mo |
| Object storage | Cloudflare R2 | — | Buckets: `qrit-assets` (public via CDN), `qrit-private` (exports, bulk inputs; signed URLs) |
| Email | Resend | — | Transactional only; SPF/DKIM/DMARC on `mail.example.com` |
| Monitoring | Grafana Cloud (logs/metrics/traces), Sentry (errors), Better Stack (uptime + status page) | Free/entry tiers | |

**Environments:** `local` (docker compose: postgres:17, redis:7, minio, mailpit, all services with hot reload), `preview` (per-PR web deploy on Fly + Neon branch; Phase 5), `staging`, `production`. **CI/CD:** GitHub Actions → lint + test + build images → push to Fly registry → deploy `staging` on merge to `main` → manual promote to `production` (a `workflow_dispatch` with the same image digest). Migrations follow expand → migrate → contract; never a destructive change in the same deploy that stops using a column.

**Why not Kubernetes/Kafka/ClickHouse now:** at launch scale, one Postgres with partitioning and rollups handles more than 50× the target load (§9.3). Each of those systems is an explicit, documented upgrade path triggered by a measurable threshold, not a day-one cost.

---

# 5. Data model

## 5.1 Entity map

```
users ─┬─< oauth_accounts
       ├─< sessions (rotating refresh tokens, family revoke)
       ├─< email_tokens (verify / reset / magic link)
       └─< workspace_members >─ workspaces ─┬─< invites
                                             ├─< domains (+ platform domains with workspace_id NULL)
                                             ├─< folders (tree) · tags · campaigns · templates · files
                                             ├─< qr_codes ─┬─< qr_versions (immutable, scheduled via effective_at)
                                             │             ├─< qr_code_tags >─ tags
                                             │             └─ (scan_events, scan_stats_15m, scan_stats_daily_dim,
                                             │                 scan_visitors_daily — keyed by qr_code_id, no FK)
                                             ├─< api_keys · webhooks ─< webhook_deliveries
                                             ├── subscriptions (1:1) · jobs · audit_logs · idempotency_keys
                                             └── abuse_reports (via qr_code_id / hostname)
billing_events (provider webhook ledger) · short_code_tombstones
```

**Invariants enforced by the database** (not just by code):
- A dynamic code has a domain and a short code and no static payload; a static code is the reverse (`qr_dynamic_has_link`).
- Types that only work as static (Wi-Fi, UPI, text, location) can't be dynamic; dynamic-only types (links page, file, app store router, GS1 Digital Link) can't be static. A GS1 code must carry a valid 14-digit GTIN, unique per domain.
- `(domain_id, short_code)` is unique across live, archived and soft-deleted rows, so a printed code is never re-issued.
- Versions are unique per `(qr_code_id, version_no)`; a URL version has a URL, and a hosted-page version has page content.
- One owner per workspace; one pending invite per email per workspace; one default template per workspace.
- Provider webhook events and outgoing webhook deliveries are unique by id, which makes both idempotent.
- Scan-event analytics tables deliberately have **no foreign keys**. They are high-volume, append-only, partitioned, and must never block or cascade on OLTP writes. Deleting a QR code leaves its analytics until retention removes it.

**Soft delete and archival:**
- `archived_at`: user action. The code stops redirecting (branded "no longer active" page or `fallback_url`), stays in lists under an "Archived" filter, and can be restored any time.
- `deleted_at`: user action. The code is hidden from lists, restorable for 30 days from Settings → Trash, then purged by a job (row removed, short code tombstoned, analytics dropped).
- Workspace deletion: 30-day grace period, with an email to all admins, then a cascading purge.

## 5.2 Schema (PostgreSQL 17, verified)

This is migration `00001_init.sql`. It was loaded into PostgreSQL and exercised with fixture data (§7.5).

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

**Partition maintenance** (`worker`, daily at 01:00 UTC): create `scan_events_YYYY_MM` for the next 3 months if missing; drop partitions older than 13 months; delete `scan_visitors_daily` rows older than 2 days; delete expired `idempotency_keys`; move any rows found in `scan_events_default` to their proper partition (and alert, because it means maintenance fell behind).

**Database roles:** `qrit_migrator` (DDL) and `qrit_app` (DML). `qrit_app` has `INSERT, SELECT` only on `audit_logs` (`REVOKE UPDATE, DELETE`). The analytics read path can use a read replica via a separate pool (`DATABASE_REPLICA_URL`, Phase 5).

## 5.3 `design` JSON (DesignV1), stored on `qr_codes.design` and `templates.design`

```jsonc
{
  "v": 1,
  "ecc": "auto",                 // auto | L | M | Q | H   (auto = H if logo else M)
  "quiet_zone": 4,               // modules, 0–10
  "modules": {
    "shape": "rounded",          // square | dots | rounded | extra-rounded | classy | classy-rounded
    "color": "#111111",
    "gradient": null             // or {"type":"linear|radial","rotation":45,"stops":[{"offset":0,"color":"#5B5BF7"},{"offset":1,"color":"#14B8A6"}]}
  },
  "finder": {
    "outer_shape": "rounded",    // square | rounded | circle | leaf
    "outer_color": "#111111",
    "inner_shape": "dot",        // square | rounded | circle | dot
    "inner_color": "#5B5BF7"
  },
  "background": { "color": "#FFFFFF", "transparent": false },
  "logo": {                      // or null
    "file_id": "0192…",          // files.id (purpose=logo); URL resolved at render time
    "size_ratio": 0.22,          // 0.10–0.30 of symbol width
    "padding": 1,                // modules of clear space around the logo box
    "clear_modules": true,
    "shape": "square"            // square | circle (mask)
  },
  "frame": {                     // or null
    "style": "banner-bottom",    // banner-bottom | banner-top | rounded-box | speech
    "text": "SCAN ME",           // ≤ 24 chars
    "text_color": "#FFFFFF",
    "color": "#111111"
  }
}
```
`design_hash = sha256(canonical_json(design))`. Canonical JSON = sorted keys, no whitespace, lower-case hex colours. The API rejects unknown keys (`422 invalid_design`) so the renderer never meets a field it doesn't understand. A future `v: 2` is added alongside, never mutated in place.

## 5.4 `rules` JSON (on `qr_versions.rules`)

Evaluated top to bottom; **the first match wins**; if nothing matches → the version's default destination.

```jsonc
[
  {
    "id": "r_ios",                         // stable id; recorded on scan events as rule_id
    "name": "iPhone users → App Store",
    "enabled": true,
    "when": { "all": [                     // "all" | "any"; one level of grouping
      { "field": "os", "op": "in", "value": ["iOS", "iPadOS"] }
    ]},
    "destination_url": "https://apps.apple.com/app/id000000"
  },
  {
    "id": "r_lunch",
    "name": "Lunch menu 11:00–15:00 on weekdays",
    "enabled": true,
    "when": { "all": [
      { "field": "local_time", "op": "between", "value": ["11:00", "15:00"] },
      { "field": "weekday",    "op": "in",      "value": [1,2,3,4,5] }
    ]},
    "destination_url": "https://example.org/menu/lunch"
  },
  {
    "id": "r_ab",
    "name": "Landing page test",
    "enabled": true,
    "when": null,                           // always matches
    "split": [                              // weights must total 100
      { "variant": "A", "weight": 50, "destination_url": "https://example.org/a" },
      { "variant": "B", "weight": 50, "destination_url": "https://example.org/b" }
    ]
  }
]
```

| `field` | Allowed `op` | `value` | Source at scan time |
|---|---|---|---|
| `country` | `in`, `not_in` | ISO-3166 alpha-2 list | CF header → MaxMind |
| `region` | `in`, `not_in` | `"IN/UP"`-style list | MaxMind subdivision |
| `device_type` | `in`, `not_in` | `mobile`, `tablet`, `desktop` | Lightweight UA classifier in `redirect` |
| `os` | `in`, `not_in` | `iOS`, `iPadOS`, `Android`, `Windows`, `macOS`, `Linux`, `Other` | Lightweight UA classifier |
| `language` | `in`, `not_in` | primary subtags (`en`, `hi`) | first `Accept-Language` entry |
| `local_time` | `between` | `["HH:MM","HH:MM"]` (may wrap midnight) | now in **workspace timezone** |
| `weekday` | `in` | 1 = Mon … 7 = Sun | now in workspace timezone |
| `date` | `between` | `["YYYY-MM-DD","YYYY-MM-DD"]` (either may be `null`) | now in workspace timezone |
| `scan_count` | `gte`, `lt` | integer | `qr_codes.total_scans` from the cache (eventually consistent, ± seconds) |

- **Split stickiness:** `bucket = uint32(sha256(qr_code_id ‖ ip ‖ user_agent)[0:4]) % 100`. This is computed in memory and discarded (the IP is never stored), so the same phone keeps seeing the same variant. The event records `rule_id = "r_ab:A"`.
- **Limits:** ≤ 20 rules per version, ≤ 10 conditions per rule, ≤ 5 split variants. Every destination is validated and safety-checked like a primary destination.
- **One engine:** the engine is a pure function in `internal/routing`: `Resolve(v ResolvedVersion, req RequestFacts, now time.Time, tz *time.Location) (url string, ruleID string)`. Both `redirect` and `api`'s `POST …/resolve-preview` call it.

## 5.5 `hosted_page` JSON (on `qr_versions.hosted_page`)

A discriminated union on `kind`, validated server-side, rendered by `web` at `/p/{code}`:

```jsonc
{ "kind": "vcard", "theme": {"accent": "#5B5BF7", "background": "#FFFFFF"},
  "vcard": { "first_name": "Aryan", "last_name": "", "org": "", "title": "", "phones": [{"type":"cell","value":"+91…"}],
             "emails": [{"type":"work","value":"…"}], "website": "", "address": {…}, "photo_file_id": null, "note": "" } }
{ "kind": "links_page", "title": "…", "avatar_file_id": null, "bio": "…",
  "links": [{"label": "Instagram", "url": "https://…", "icon": "instagram"}] }          // ≤ 30 links
{ "kind": "file", "file_id": "…", "title": "Menu", "cta": "View menu" }                   // PDF/image ≤ 20 MB
{ "kind": "event", "title": "…", "starts_at": "…", "ends_at": "…", "timezone": "Asia/Kolkata",
  "location": "…", "description": "…", "url": null }                                      // "Add to calendar" (.ics)
```

## 5.6 Scan event (stream message) schema

The `redirect` service publishes one JSON message per request to Redis Stream `scans` (`XADD scans MAXLEN ~ 5000000 * e <json>`):

```jsonc
{
  "id": "01926f3e-…",                 // UUIDv7 = event_id (idempotency key end-to-end)
  "ts": "2026-09-24T18:40:03.123Z",   // occurred_at (redirect clock, NTP-synced)
  "ws": "…", "qr": "…", "ver": "…", "cmp": "…" | null, "dom": "…",
  "rule": "r_ab:A" | null,
  "out": "redirect",                   // outcome enum (schema CHECK)
  "m": "GET",                          // GET | HEAD | POST
  "vh": "base64(16 bytes)",            // visitor hash (daily salt) — NO IP ANYWHERE
  "ua": "Mozilla/5.0 …",               // raw UA; parsed by ingest, never stored raw
  "dc": false,                         // IP belongs to a hosting/datacenter ASN (MaxMind ASN)
  "geo": { "cc": "IN", "rg": "UP", "ct": "Noida" },
  "lang": "en",
  "ref": "l.instagram.com" | null,     // referrer host only
  "utm": { "s": "flyer", "m": null, "c": null }   // from the *scanned* URL's query, if present
}
```

## 5.7 Plans and entitlements (code, not a table)

`internal/entitlements/plans.go` holds the single source of truth, served to the UI via `GET /entitlements` and enforced by the API. Prices live in the billing provider; limits live here.

| Key | Free | Pro | Business | Enterprise |
|---|---|---|---|---|
| `dynamic_codes` | 3 | 100 | 1,000 | custom (default 100,000) |
| `static_codes` | ∞ | ∞ | ∞ | ∞ |
| `scans` | ∞ (never limited) | ∞ | ∞ | ∞ |
| `analytics_history_days` (visible) | 30 | 365 | 1,095 | custom |
| `seats` | 1 | 1 (+ paid seats) | 5 (+ paid seats) | custom |
| `owned_workspaces` | 1 | 3 | 10 | custom |
| `custom_domains` | 0 | 1 | 5 | 50 |
| `bulk_rows_per_job` | 0 | 500 | 5,000 | 50,000 |
| `api_requests_per_min` | 0 (no keys) | 0 (no keys) | 600 | 3,000 |
| `webhooks` | 0 | 0 | 10 | 50 |
| `templates` | 0 | 10 | 100 | ∞ |
| features | static designer, basic dynamic, password | + scheduling, expiry, scan limit, UTM append, hosted pages, templates, CSV export, remove "Made with" on hosted pages | + rules & A/B, campaigns, API, webhooks, raw scan log, brand-locked templates, audit log (1 yr), GS1 Digital Link, roles | + SSO/SAML, SCIM, audit log (unlimited), SLA, dedicated short domain, data residency, invoicing |

---

# 6. API design

## 6.1 Conventions

| Topic | Rule |
|---|---|
| Base URL | Dashboard: same-origin `https://example.com/api/v1` (rewritten to `api`). Public API: `https://api.example.com/v1` |
| Contract | `api/openapi.yaml` (OpenAPI 3.1) is the source of truth. Go handlers are hand-written chi handlers with typed request/response structs; a **contract test** replays every integration-test request through `kin-openapi` and fails if a request or response doesn't match the spec. The TS client types come from `openapi-typescript` + `openapi-fetch`. CI fails if generated code is stale. |
| Auth — dashboard | Host-only httpOnly cookies (no `Domain` attribute): `qrit_at` (access JWT, EdDSA, 10 min, `SameSite=Lax`) and `qrit_rt` (opaque refresh, 30 days, `SameSite=Strict`), both `Path=/` so the Next.js `proxy.ts` guard can see them. State-changing requests must also send the `X-CSRF-Token` header (double-submit, matching the readable `qrit_csrf` cookie). |
| Auth — public API | `Authorization: Bearer qk_live_<32 base32 chars>`. A key is bound to one workspace, so API routes also accept `/v1/qr-codes` without `{ws}` (resolved from the key). |
| IDs | UUIDv7 strings. Short codes are separate, public identifiers. |
| Pagination | Cursor: `?limit=50&cursor=<opaque>` → `{ "data": [...], "next_cursor": "…" | null }`. Max limit 100. |
| Filtering/sorting | Explicit query params (`status=active&type=url&sort=-scans`); no generic filter DSL. |
| Idempotency | `Idempotency-Key` header **required** on `POST` create endpoints (QR, versions, bulk jobs, invites, domains, API keys, webhooks). Stored 24 h per workspace. |
| Concurrency | `PATCH` on QR metadata accepts `If-Match: "<updated_at-etag>"` → `412` on mismatch (prevents two editors silently overwriting each other). |
| Rate-limit headers | `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (IETF draft), plus `Retry-After` on `429` |
| Errors | RFC 9457 problem+json with a stable `code` (§4.8) |
| Time | RFC 3339 UTC in payloads; analytics accept `tz` (IANA) and default to the workspace timezone |
| Versioning | URI major version (`/v1`); additive changes only within v1; deprecations announced via the `Deprecation` + `Sunset` headers |

## 6.2 Endpoints — control plane (`api`)

**Auth and account**

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/v1/auth/register` | — | `{email, password, name}` → creates user + personal workspace, sends verification email, sets cookies |
| POST | `/v1/auth/login` | — | `{email, password}` → cookies. Lockout: 5 failures/15 min per account + IP limiter |
| POST | `/v1/auth/magic-link` | — | `{email}` → emails a one-time link (always `202`, no account enumeration) |
| POST | `/v1/auth/magic-link/verify` | — | `{token}` → cookies |
| GET | `/v1/auth/oauth/google/start` · `/callback` | — | OAuth 2.0 + PKCE; links by verified email |
| POST | `/v1/auth/refresh` | refresh cookie | Rotates the refresh token; reuse of an old token revokes the whole family |
| POST | `/v1/auth/logout` | session | Revokes the current session |
| POST | `/v1/auth/verify-email` · `/password/forgot` · `/password/reset` | — | Token flows (tokens stored hashed, single use, 1 h / 24 h expiry) |
| GET / PATCH | `/v1/me` | session | Profile, timezone, locale |
| GET / DELETE | `/v1/me/sessions[/{id}]` | session | List/revoke devices |

**Workspaces and team**

| Method | Path | Min role | Purpose |
|---|---|---|---|
| GET / POST | `/v1/workspaces` | — | List mine / create |
| GET / PATCH / DELETE | `/v1/workspaces/{ws}` | analyst / admin / owner | Read, update name/timezone/brand/settings, schedule deletion |
| GET | `/v1/workspaces/{ws}/entitlements` | analyst | Plan, limits, usage, feature flags (drives locked UI) |
| GET | `/v1/workspaces/{ws}/members` | analyst | |
| PATCH / DELETE | `/v1/workspaces/{ws}/members/{userId}` | admin | Change role / remove (can't remove or demote the owner) |
| POST | `/v1/workspaces/{ws}/transfer-ownership` | owner | `{user_id}` (must be an admin) |
| GET / POST | `/v1/workspaces/{ws}/invites` | admin | Create invite `{email, role}` (seat check) |
| DELETE | `/v1/workspaces/{ws}/invites/{id}` | admin | Revoke |
| GET / POST | `/v1/invites/{token}` · `/v1/invites/{token}/accept` | — / session | Preview / accept |

**QR codes**

| Method | Path | Min role | Purpose |
|---|---|---|---|
| POST | `/v1/workspaces/{ws}/qr-codes` | editor | Create static or dynamic (Idempotency-Key) |
| GET | `/v1/workspaces/{ws}/qr-codes` | analyst | List: `q, type, mode, status, folder_id, campaign_id, tag, sort (created_at, -created_at, name, -total_scans), limit, cursor` |
| GET | `/v1/workspaces/{ws}/qr-codes/{id}` | analyst | Detail incl. current version, 7-day sparkline |
| PATCH | `/v1/workspaces/{ws}/qr-codes/{id}` | editor | Metadata only: `name, folder_id, campaign_id, tags, design, starts_at, expires_at, scan_limit, fallback_url, password (write-only; null clears)` |
| POST | `…/qr-codes/{id}/pause` · `/resume` · `/archive` · `/unarchive` | editor | State transitions (audited, cache invalidated) |
| DELETE | `…/qr-codes/{id}` | admin | Soft delete (30-day restore) |
| POST | `…/qr-codes/{id}/restore` | admin | Undo soft delete |
| POST | `…/qr-codes/{id}/duplicate` | editor | New code (new short code) with the same design + destination |
| GET | `…/qr-codes/{id}/versions` | analyst | History, newest first |
| POST | `…/qr-codes/{id}/versions` | editor | **Change destination**: `{destination_kind, destination_url \| hosted_page, rules?, utm?, effective_at?, change_note?}` |
| POST | `…/qr-codes/{id}/versions/{versionId}/restore` | editor | Creates a new version copying the target (`restored_from`) |
| DELETE | `…/qr-codes/{id}/versions/{versionId}` | editor | Only a *scheduled* (future) version can be cancelled |
| POST | `…/qr-codes/{id}/resolve-preview` | analyst | `{country, region, device_type, os, language, at}` → `{destination_url, rule_id, reason}` |
| GET | `…/qr-codes/{id}/image` | analyst | `format=svg\|png\|pdf, size=256..4096, frame=true` → `302` to CDN |
| POST | `/v1/workspaces/{ws}/qr-codes/bulk-actions` | editor | `{ids[], action: move\|tag\|untag\|pause\|resume\|archive, params}` (≤ 500 ids) |
| POST | `/v1/workspaces/{ws}/jobs` | editor | `{kind: bulk_create\|bulk_download\|export_scans\|export_qr_codes, input_file_id?, params}` |
| GET | `/v1/workspaces/{ws}/jobs/{jobId}` | analyst | Status, progress, errors, `output_url` (signed, 15 min) |
| POST | `/v1/workspaces/{ws}/files` | editor | Multipart upload (logo ≤ 2 MB, hosted asset ≤ 20 MB, CSV ≤ 10 MB) → `{id, url}` |

**Organisation**: `folders`, `tags`, `campaigns`, `templates` each have `GET/POST /v1/workspaces/{ws}/{resource}` and `GET/PATCH/DELETE /v1/workspaces/{ws}/{resource}/{id}` (editor to write, analyst to read). There is also `GET /v1/workspaces/{ws}/campaigns/{id}/analytics` (same shape as §6.3 summary).

**Domains**

| Method | Path | Min role | Purpose |
|---|---|---|---|
| GET / POST | `/v1/workspaces/{ws}/domains` | admin | Add `{hostname}` → returns the CNAME target + TXT verification record |
| POST | `/v1/workspaces/{ws}/domains/{id}/verify` | admin | Trigger a check now (also polled by a job every 5 min for 72 h) |
| PATCH / DELETE | `/v1/workspaces/{ws}/domains/{id}` | admin | `root_redirect_url`, `not_found_url`, set default; delete only if no dynamic codes use it |

**Integrations and governance**

| Method | Path | Min role | Purpose |
|---|---|---|---|
| GET / POST | `/v1/workspaces/{ws}/api-keys` | admin | Create returns the full key **once** |
| DELETE | `/v1/workspaces/{ws}/api-keys/{id}` | admin | Revoke |
| GET / POST / PATCH / DELETE | `/v1/workspaces/{ws}/webhooks[/{id}]` | admin | Events: `qr.created, qr.updated, qr.version.created, qr.archived, scan.created` |
| POST | `/v1/workspaces/{ws}/webhooks/{id}/test` | admin | Sends a signed `ping` |
| GET | `/v1/workspaces/{ws}/webhooks/{id}/deliveries` | admin | Last 100 with status, code, latency; `POST …/deliveries/{id}/retry` |
| GET | `/v1/workspaces/{ws}/audit-logs` | admin | `action, actor_id, target_id, from, to, cursor` |
| GET | `/v1/workspaces/{ws}/billing` | admin | Plan, status, seats, period end, invoices link |
| POST | `/v1/workspaces/{ws}/billing/checkout` | owner | `{plan_id, interval, provider?}` → `{url}` (hosted checkout) |
| POST | `/v1/workspaces/{ws}/billing/portal` | owner | `{url}` |
| POST | `/v1/billing/webhooks/stripe` · `/razorpay` | signature | Provider events → `billing_events` (dedupe) → subscription sync |

**Public (unauthenticated)**

| Method | Path | Purpose |
|---|---|---|
| POST | `/v1/public/render` | `{payload, design, format: pdf, size}` → PDF for the anonymous generator (PNG/SVG are rendered in the browser). 20/min per IP prefix, 200/day |
| POST | `/v1/public/abuse-reports` | `{url, reason, details?, email?}` with Cloudflare Turnstile token |
| GET | `/v1/public/qr-types` | Content types + field schemas (drives the generator forms) |

## 6.3 Analytics endpoints

All analytics routes need the analyst role or higher and accept the common filters `from, to (ISO dates, local to tz), tz, qr_ids (comma list), campaign_id, country, device_type`.

| Method | Path | Returns |
|---|---|---|
| GET | `/v1/workspaces/{ws}/analytics/summary?compare=previous` | `{scans, unique_scans, avg_per_day, top_country, bot_hits, blocked_hits, previous: {...}, deltas: {...}}` |
| GET | `/v1/workspaces/{ws}/analytics/timeseries?granularity=auto\|15m\|hour\|day\|week` | `{granularity, points: [{t, scans, unique_scans}]}` zero-filled |
| GET | `/v1/workspaces/{ws}/analytics/breakdown?dimension=country\|region\|city\|device\|os\|browser\|language\|referrer\|utm_source\|rule\|version&limit=10` | `{dimension, rows: [{key, label, scans, unique_scans, share}], other: {scans}}` |
| GET | `/v1/workspaces/{ws}/analytics/heatmap` | `{cells: [{weekday, hour, scans}]}` (7×24, local) |
| GET | `/v1/workspaces/{ws}/analytics/top-qr-codes?limit=10` | Ranked list |
| GET | `/v1/workspaces/{ws}/analytics/realtime` | `{last_5m, last_30m, per_minute: [30 points]}` from Redis |
| GET | `/v1/workspaces/{ws}/analytics/scans?cursor` | Raw scan log (Business): time, QR, outcome, country/city, device/OS/browser, rule, version |

Plan enforcement: if `from` is older than `analytics_history_days`, the response is clamped and includes `"clamped": true, "available_from": "…"` rather than an error, so the UI can show "Upgrade to see earlier data" while still rendering.

## 6.4 Data plane (`redirect`) routes

| Method | Path | Behaviour |
|---|---|---|
| GET | `/{code}` | Resolve → `302 Location: …` (or a status page). `Cache-Control: private, no-store`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-Robots-Tag: noindex`. **No cookies.** |
| HEAD | `/{code}` | Same status + `Location`; recorded as a bot hit (`method=HEAD`) |
| POST | `/{code}` | Password form submit (`application/x-www-form-urlencoded`, field `password`) |
| GET | `/{code}+` | Preview page: destination host, owner-set name, "Report" link. Not counted as a scan |
| GET | `/p/{code}`, `/_next/*` | Reverse-proxied to `web` (hosted pages on the same host, including custom domains) |
| GET | `/` | Domain's `root_redirect_url` or the marketing site (platform domain) |
| GET | `/healthz` · `/readyz` | Liveness / readiness (Redis reachable **or** LRU warm; PG optional) |

## 6.5 Example payloads

**Create a dynamic QR code**

```http
POST /v1/workspaces/0192…/qr-codes
Idempotency-Key: 5f0c2b1e-8a9d-4c1e-9e0b-1d2c3b4a5f60
Content-Type: application/json

{
  "mode": "dynamic",
  "content_type": "url",
  "name": "Table tent — lunch menu",
  "destination": { "kind": "url", "url": "https://example.org/menu" },
  "utm": { "source": "qr", "medium": "print", "campaign": "table-tent" },
  "design": { "v": 1, "ecc": "auto", "quiet_zone": 4,
              "modules": {"shape": "rounded", "color": "#111111", "gradient": null},
              "finder": {"outer_shape": "rounded", "outer_color": "#111111", "inner_shape": "dot", "inner_color": "#5B5BF7"},
              "background": {"color": "#FFFFFF", "transparent": false}, "logo": null,
              "frame": {"style": "banner-bottom", "text": "SCAN FOR MENU", "text_color": "#FFFFFF", "color": "#111111"} },
  "folder_id": null, "campaign_id": null, "tags": ["restaurant"]
}
```
```http
HTTP/1.1 201 Created
Location: /v1/workspaces/0192…/qr-codes/0192…

{
  "id": "0192…", "mode": "dynamic", "content_type": "url", "name": "Table tent — lunch menu",
  "status": "active", "safety_status": "safe", "is_read_only": false,
  "short_code": "7K2M9QX", "short_url": "https://qr.example.com/7K2M9QX",
  "encoded_payload": "HTTPS://QR.EXAMPLE.COM/7K2M9QX",
  "current_version": { "id": "0192…", "version_no": 1, "destination_kind": "url",
                       "destination_url": "https://example.org/menu", "effective_at": "2026-09-25T06:10:00Z" },
  "design": { … }, "design_hash": "9f2c…",
  "image_urls": { "svg": "…/image?format=svg", "png": "…/image?format=png&size=1024", "pdf": "…/image?format=pdf" },
  "total_scans": 0, "unique_scans": 0, "last_scanned_at": null,
  "created_at": "2026-09-25T06:10:00Z", "updated_at": "2026-09-25T06:10:00Z"
}
```

**Schedule a destination change**

```http
POST /v1/workspaces/0192…/qr-codes/0192…/versions
Idempotency-Key: 8b1d…
{ "destination_kind": "url", "destination_url": "https://example.org/menu/diwali",
  "effective_at": "2026-10-20T00:00:00+05:30", "change_note": "Diwali menu" }
→ 201 { "id": "…", "version_no": 4, "status": "scheduled", "effective_at": "2026-10-19T18:30:00Z", … }
```

**Limit reached**

```http
HTTP/1.1 402 Payment Required
Content-Type: application/problem+json
{ "type": "https://docs.example.com/errors/limit_reached", "title": "Plan limit reached", "status": 402,
  "code": "limit_reached", "detail": "Your Free plan allows 3 dynamic QR codes.",
  "limit": 3, "current": 3, "required_plan": "pro", "instance": "req_01J8…" }
```

**Webhook delivery** (`POST` to the customer URL)

```http
Content-Type: application/json
Webhook-Id: 0192…                     (= event id; dedupe on this)
Webhook-Timestamp: 1790310003
Webhook-Signature: v1,base64(HMAC_SHA256(secret, "{id}.{timestamp}.{body}"))
{ "type": "scan.created", "created_at": "…", "workspace_id": "…",
  "data": { "qr_code_id": "…", "short_code": "7K2M9QX", "version_no": 4, "rule_id": null,
            "country": "IN", "city": "Noida", "device_type": "mobile", "os": "Android", "unique": true } }
```
Signature scheme follows the Standard Webhooks convention: receivers reject timestamps older than 5 minutes. Retries use exponential backoff (1 m, 5 m, 30 m, 2 h, 6 h, 12 h, 24 h), then `dead`. 20 consecutive failures auto-disable the endpoint and email the admins.

## 6.6 RBAC matrix

| Permission | owner | admin | editor | analyst |
|---|:-:|:-:|:-:|:-:|
| View QR codes, analytics, campaigns | ✓ | ✓ | ✓ | ✓ |
| Create/edit QR, versions, rules, folders, tags, campaigns, templates | ✓ | ✓ | ✓ | |
| Use a locked template only (can't change design) | | | ✓ if template locked | |
| Delete/restore QR codes, manage domains, API keys, webhooks, audit log | ✓ | ✓ | | |
| Invite/remove members, change roles (except owner) | ✓ | ✓ | | |
| Billing, delete workspace, transfer ownership | ✓ | | | |

API keys carry scopes (`qr:read`, `qr:write`, `analytics:read`, `webhooks:write`) and act with the permissions of an editor limited to those scopes.

---

# 7. Analytics and tracking design

The pipeline has four separated stages. Each has one job and one failure mode.

```
 capture (redirect)  →  transport (Redis Stream)  →  enrich + persist (ingest)  →  serve (api reads rollups)
 no IP leaves here       durable, replayable           idempotent, batched           never touches raw at scale
```

## 7.1 Scan lifecycle, end to end

The timings in brackets are the budget for a cache hit on the origin.

1. **Scan.** The phone camera decodes `HTTPS://QR.EXAMPLE.COM/7K2M9QX` and shows a preview chip with the host `qr.example.com`. The user taps.
2. **Edge.** Cloudflare terminates TLS at the nearest point of presence. WAF managed rules apply, plus a coarse rate limit of 600 requests/min per IP on the short host (only floods are stopped; a real crowd scanning one poster at an event stays well below this per IP). Cloudflare adds `CF-Connecting-IP`, `CF-IPCountry` and, via the "Add visitor location headers" managed transform, `cf-ipcity` / `cf-region-code`. The request is forwarded to the nearest `redirect` machine.
3. **Parse and normalise** [< 0.1 ms]. The code is upper-cased; `O→0` and `I,L→1` are mapped; a trailing `/` or `+` is detected. Anything that doesn't match `^[0-9A-HJKMNP-TV-Z]{7}$` goes to the legacy lookup (v1 codes, §10 Phase 5) or straight to the fast 404 page. No DB lookup for garbage, which protects against brute-force scanning.
4. **Domain.** `Host` → `domain_id` from the in-process map. An unknown host gets a plain 404.
5. **Resolve** [LRU hit ~0.01 ms; Redis ~0.5–1 ms; Postgres ~2–5 ms]. Lookup order: in-process LRU → Redis `link:v1:{domain_id}:{code}` → Postgres (one query joining `qr_codes`, the effective `qr_versions` row, `workspaces.timezone` and the next scheduled `effective_at`). The result is cached in both layers. Concurrent misses for the same key are collapsed with `singleflight`, so a viral code can't stampede the database. The cache payload:
   ```jsonc
   { "qr": "…", "ws": "…", "dom": "…", "cmp": "…"|null, "status": "active", "safety": "safe",
     "starts_at": null, "expires_at": null, "scan_limit": null, "total_scans": 1834,
     "has_password": false, "pw_hash": null, "fallback_url": null, "tz": "Asia/Kolkata",
     "ver": { "id": "…", "no": 4, "kind": "url", "url": "https://example.org/menu", "rules": [...], "utm": {...} },
     "next_change_at": "2026-10-19T18:30:00Z",   // entry is stale after this instant
     "cached_at": "…" }
   ```
6. **State gates** (first match decides the outcome, which is recorded on the event):

   | Condition | Outcome | Response |
   |---|---|---|
   | `status = blocked` or `safety = blocked` | `blocked` | `410` branded page: "This link has been disabled for violating our terms." Never redirects. |
   | `status in (paused, archived)` | `paused` | `302 fallback_url` if set, else `410` "This QR code is not active." |
   | `now < starts_at` | `not_started` | `302 fallback_url` or `404` "Not active yet." |
   | `now ≥ expires_at` | `expired` | `302 fallback_url` or `410` "This QR code has expired." |
   | `scan_limit` reached (`total_scans ≥ scan_limit`) | `limit_reached` | `302 fallback_url` or `410`. Eventually consistent by a few seconds, which is acceptable and documented. |
   | `has_password` and GET | `password_prompt` | `200` minimal HTML form (no JS, 3 KB). POST verifies argon2id (§8.4) → `password_ok` + redirect, or `password_fail` + `401` form. |
   | Geo restriction rule (a rule with no destination that means "deny") | `geo_blocked` | `451` page |

7. **Request facts** [~0.05 ms]:
   - The IP is taken from `CF-Connecting-IP`, trusted only when the TCP peer is inside Cloudflare's published ranges; otherwise the socket address is used.
   - Geo: country from the CF header, else a MaxMind GeoLite2-City lookup on an in-memory mmdb (~µs). Region and city come from CF headers when present, else MaxMind.
   - `dc` = the IP's ASN is in the datacenter/hosting list (MaxMind GeoLite2-ASN + a curated list: AWS, GCP, Azure, DigitalOcean, OVH, Hetzner…).
   - A lightweight UA classifier (a regex table of about 40 patterns) gives `os` and `device_type`, used only for rules.
   - `lang` is the first primary subtag of `Accept-Language`.
8. **Visitor hash** [~0.002 ms]: `vh = HMAC-SHA256(key = salt[today_utc], msg = qr_id ‖ 0x00 ‖ ip ‖ 0x00 ‖ user_agent)[0:16]`. The salt is 32 random bytes created once per UTC day (`SET salt:{date} NX EX 172800` in Redis) and never written to Postgres or logs. After 48 hours it is gone, so hashes can no longer be linked to an IP even by us. **The raw IP is dropped here and never leaves this function.**
9. **Destination** [~0.01 ms]: `routing.Resolve(version, facts, now, tz)` → `(url, rule_id)`. Then UTM parameters are appended if the version has `utm` and the destination doesn't already carry that key. The final URL is re-validated (scheme allow-list) as defence in depth.
10. **Emit event** [~0.001 ms, non-blocking]: the event is pushed onto a bounded in-process channel (capacity 100,000). A background goroutine drains it every 50 ms or 500 events, whichever comes first, and pipelines `XADD scans MAXLEN ~ 5000000 * e <json>` to Redis. If Redis is unreachable the batch is retried with backoff while the channel absorbs the backlog. If the channel fills (Redis down for minutes at full load), events are dropped and `scan_events_dropped_total` is incremented, which pages on-call. The redirect never waits on analytics.
11. **Respond** [total origin time p50 ~1 ms, p99 < 10 ms on cache hit]:
    ```
    HTTP/1.1 302 Found
    Location: https://example.org/menu?utm_source=qr&utm_medium=print&utm_campaign=table-tent
    Cache-Control: private, no-store, max-age=0
    Referrer-Policy: strict-origin-when-cross-origin
    X-Robots-Tag: noindex, nofollow
    Server-Timing: resolve;dur=0.4
    ```
    `302` (not `301`): browsers and intermediaries must never cache the mapping, or later edits wouldn't take effect. Hosted pages are served through the same host: `302 /p/7K2M9QX` goes to `web` via the reverse proxy, and the scan was already recorded on the first hop with outcome `hosted_page`.
12. **Fallbacks when things break:**

    | Failure | Behaviour |
    |---|---|
    | Redis down, Postgres up | LRU → Postgres (singleflight); events buffered in the channel; rate limiter fails **open** for redirect |
    | Postgres down, Redis up | Redis cache serves; misses serve the "temporarily unavailable, try again" page with `503 Retry-After: 30` |
    | Both down | Last-known-good in-process entries (24 h) keep serving known codes; unknown → `503` page |
    | Destination site down | Not our concern at redirect time (we never fetch the destination). A daily `worker` job HEAD-checks active destinations of Pro+ codes and emails the owner on 3 consecutive failures ("Your QR code points to a page that isn't loading") |
    | Code deleted/unknown | Branded 404 (or the domain's `not_found_url`), with a "Report" link. Negative-cached for 60 s |

## 7.2 Definitions (published on the docs "How we count" page)

| Term | Definition |
|---|---|
| **Scan** | A request to a dynamic code that reached its destination (`redirect`, `hosted_page`, or `password_ok`), is not classified as automated, and is not a duplicate. |
| **Unique scan** | The first counted scan of a visitor for a QR code within a **UTC day**. A visitor is `(IP, User-Agent)` scoped to that QR code via the daily-salted hash. Unique scans over a range = sum of daily uniques. This is a deliberate privacy trade-off: we can't tell whether Monday's visitor returned on Tuesday. |
| **Duplicate** | A repeat request from the same visitor to the same code within **10 seconds** (camera apps and in-app browsers often fire twice). Stored but not counted. |
| **Automated hit** | See §7.3. Stored with `is_bot = true` and a `bot_reason`, shown as "filtered", not counted. |
| **Blocked hit** | A request that did not reach a destination because of state (expired, paused, limit, geo, blocked). Counted separately (`blocked_hits`) so owners see that people are scanning an expired code. |

## 7.3 Bot and automation filtering (`internal/scan/botdetect`, run in `ingest`)

A hit is flagged `is_bot` if any rule matches (first reason recorded):

1. `method = HEAD` → `head_request` (link-preview fetchers and uptime checks).
2. UA matches the curated list (≈ 150 patterns, updated monthly): link previewers (`facebookexternalhit`, `WhatsApp`, `TelegramBot`, `Slackbot`, `Twitterbot`, `LinkedInBot`, `Discordbot`, `SkypeUriPreview`, `Applebot`), search engines, **email security scanners** that follow URLs (Microsoft Defender Safe Links, Proofpoint, Mimecast, Barracuda), HTTP libraries (`curl`, `python-requests`, `Go-http-client`, `okhttp` without an app token, `axios`), headless browsers (`HeadlessChrome`, `PhantomJS`) → `ua_bot`.
3. Empty or missing UA → `ua_missing`.
4. `dc = true` (datacenter ASN) **and** the UA is a desktop/headless browser → `datacenter`. A real phone on mobile data is never in a datacenter ASN; this catches scanners that fake a browser UA.
5. More than 30 hits from the same visitor hash to the same code in 60 s → `velocity` (Redis counter `v:{qr}:{vh}` with a 60 s TTL).

Bots still get redirected: breaking link previews would make QR links look broken in chats. The owner sees "N automated hits filtered", and Business plans can include bot hits in exports.

## 7.4 Privacy-safe storage

| Data | Stored? | Form |
|---|---|---|
| IP address | **Never** | Used in memory for geo, ASN, hash and split bucket, then dropped |
| User-Agent | **Not raw** | Parsed into device type, OS (+ major version), browser (+ major version) |
| Location | Coarse | Country, region, city only. No latitude/longitude (v1 stored them; v2 removes them) |
| Visitor identity | Pseudonymous, unlinkable after 48 h | 16-byte daily-salted HMAC, per QR code |
| Cookies / device IDs | None on redirect or hosted pages | — |
| Referrer | Host only | e.g. `l.instagram.com` |
| Password attempts | Rate-limit counter keyed by `vh`, 15-min TTL | Redis only |

This lets the privacy policy say, truthfully: *we don't store IP addresses, and we can't identify or track individual scanners across days or across QR codes.* That's a strong position under GDPR and India's DPDP Act, and it's why no consent banner is needed on the redirect. Workspace owners still need a privacy notice for *their own* destination pages; our DPA covers our role as processor.

## 7.5 Ingest pipeline (`cmd/ingest`)

**Consumer loop:**
- `XREADGROUP GROUP ingest {hostname} COUNT 1000 BLOCK 1000 STREAMS scans >`, accumulating a batch of up to 1,000 messages or 1 s.
- For each message:
  - Parse the UA with `uap-go` (full regex set; ~20–50 µs, which is fine off the hot path).
  - Classify bots (§7.3).
  - Duplicate check: `SET dup:{qr}:{vh} {event_id} NX PX 10000`. It's a duplicate only if the key exists **with a different event id**, which makes replays safe.
- Run the batch transaction below.
- After `COMMIT`: `XACK`; `INCRBY` the realtime counters `rt:{ws}:{minute}` and `rt:{ws}:{qr}:{minute}` (TTL 2 h); enqueue River `scan.created` webhook jobs for workspaces that subscribe (one job per event, only for workspaces with an active subscription, resolved from a 30 s in-process cache).
- **Recovery:** every 30 s, `XAUTOCLAIM scans ingest {hostname} 60000 0-0 COUNT 1000` takes over messages stuck on dead consumers. A message delivered more than 5 times moves to stream `scans:dlq` and raises an alert.
- **Backpressure:** if the stream length exceeds 100,000 (lag), the batch size is raised to 5,000 and an alert fires.

**The batch transaction** (verified: running the same batch twice leaves every counter unchanged; a second batch with repeat visitors adds scans but not uniques; events either side of IST midnight land on the correct local days):

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

**Why this is exactly-once in effect:** the stream gives at-least-once delivery. `event_id` is minted once at the redirect and is the primary key. Only rows returned by `INSERT … ON CONFLICT DO NOTHING RETURNING` feed the uniques, rollups and counters, all in the same transaction. A crash before `COMMIT` rolls everything back; a crash after `COMMIT` but before `XACK` causes redelivery, which inserts nothing and therefore counts nothing.

## 7.6 Aggregation model

| Table | Grain | Used for | Why this grain |
|---|---|---|---|
| `scan_stats_15m` | QR × 15-minute bucket (UTC-aligned) | Totals, time series, heatmap, top QR codes, campaign totals | 15-minute buckets align with every real timezone offset (+05:30, +05:45, −03:30), so local-day and local-hour charts are exact |
| `scan_stats_daily_dim` | QR × UTC day × dimension × key | Breakdowns (country, city, device, OS, browser, language, referrer, UTM source, rule, version) for ranges ≥ 3 days | Compact. Using UTC days affects at most a partial day at each edge of a multi-day range; shares barely move |
| `scan_events` (raw) | Event | Breakdowns for ranges < 3 days (exact), raw scan log, exports, re-aggregation | Truth; retained 13 months |

**Size estimate** at 5 M counted scans/month across ~20k active codes: `scan_events` ≈ 5 M rows × ~180 B ≈ 0.9 GB/month (+ indexes ≈ 1.6 GB/month; 13-month cap ≈ 21 GB). `scan_stats_15m` holds at most one row per code per active bucket; typically ≤ 2 M rows/month. Well within one Postgres instance.

**Query service (`internal/analytics`):**
- Converts `(from, to, tz)` into half-open UTC instants at local midnights.
- Clamps to the plan's history.
- Picks granularity (`15m` ≤ 1 day, `hour` ≤ 7 days, `day` ≤ 180 days, else `week`).
- For `hour` and `week`, uses the same pattern as the daily query with `date_trunc('hour'|'week', bucket_start AT TIME ZONE tz)`.
- Caches responses in Redis.

**The verified sqlc queries:**

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

For breakdowns under 3 days, `BreakdownRaw*` queries exist per dimension column (country shown; the others differ only in the column).

**Reconciliation (integrity):** a nightly job recomputes yesterday's totals per workspace from `scan_events` and compares them with `scan_stats_15m`. Any mismatch is logged with the workspace id, alerts, and can be repaired by the `ingest rebuild --day=YYYY-MM-DD` command. That command deletes the day's rollups and rebuilds them from raw events in one transaction. This is also how a bot-rule update can be applied retroactively.

## 7.7 Realtime

`ingest` increments per-minute counters in Redis after each commit. `GET /analytics/realtime` reads 30 keys with `MGET` (< 1 ms). The dashboard polls every 10 s while the tab is visible (Page Visibility API) and pauses when hidden. Server-Sent Events (SSE) are an optional later upgrade; polling is simpler and adequate at this scale.

## 7.8 Retention

| Data | Retention |
|---|---|
| `scan_events` | 13 months (monthly partitions dropped) |
| Rollups | 5 years (tiny); **visibility** is limited by plan, so an upgrade instantly reveals the history that already exists (a strong upgrade moment) |
| `scan_visitors_daily` | 2 days |
| Visitor salt | 48 hours (Redis only) |
| Audit logs | Free/Pro: 90 days · Business: 1 year · Enterprise: unlimited (a job deletes by plan) |
| Webhook deliveries | 30 days |
| Deleted QR codes | 30-day restore window, then purge (analytics dropped with the next partition drop) |

## 7.9 Campaign and source attribution

- **Campaign:** `qr_codes.campaign_id` is stamped onto each event and rollup row *at scan time*, so moving a code between campaigns doesn't rewrite history.
- **UTM out:** a version's `utm` (or the campaign's default) is appended to destination URLs, so the customer's GA4/analytics attributes the visit to the QR campaign. This is the most requested marketing integration.
- **UTM in:** if the printed URL itself carries `?utm_source=…` (for example, the same code printed with different params on different media, `…/7K2M9QX?S=FLYER`; keep it upper-case to stay in alphanumeric mode), the redirect captures `utm_source`/`s` into the event, so one code can be split by placement without creating more codes.
- **Referrer:** rare for camera scans (usually empty); present when the short link is clicked inside apps (Instagram, WhatsApp). Shown as "Direct" when empty.

---

# 8. Security and abuse prevention

## 8.1 Destination validation (prevents open-redirect abuse)

The redirect service is **not** an open redirect: it only ever sends visitors to destinations that an authenticated workspace member stored, and nothing in the scanned URL can change the destination (query params are only *read* for UTM capture). Destination validation (`internal/urlsafety`) runs on every write (create, version, rule, split variant, fallback URL, root/not-found URL):

1. Trim; length ≤ 2,048; reject control characters and whitespace inside.
2. Parse with `net/url`. **Scheme allow-list:** `https`, `http` (allowed unless the workspace sets `require_https`; shows a warning), `mailto`, `tel`, `sms`, `geo`, and app deep links from a curated list (`whatsapp`, `instagram`, `spotify`, `upi` for dynamic app routing). Reject everything else, including `javascript`, `data`, `vbscript`, `file`, `blob`, `about`, `chrome`, `intent`, and unknown schemes.
3. Host required for http(s). Convert to punycode (IDNA 2008, `golang.org/x/net/idna` Lookup profile). **Reject mixed-script confusables** (e.g. Cyrillic `а` in `pаypal.com`) using the Unicode confusables skeleton compared against a top-10k-domains list. Reject userinfo (`https://paypal.com@evil.tld`).
4. Reject our own short domains as destinations (redirect loops/chains) and any known URL shortener chain beyond depth 1 (`bit.ly`, `t.co`, etc.; the list is configurable). This stops attackers laundering a flagged URL through us.
5. Workspace policy: `allowed_destination_hosts` (exact or `*.brand.com`), when set, must match (Business "brand safety").
6. **Reputation:** Google **Web Risk API** (`uris.search`, threat types `MALWARE`, `SOCIAL_ENGINEERING`, `UNWANTED_SOFTWARE`), since the free Safe Browsing API is restricted to non-commercial use. Results are cached 30 min per URL. Verdicts: safe → `safe`; threat → `422 destination_unsafe`; timeout → `pending` + a re-check job.
7. The redirect re-checks the scheme allow-list at serve time (defence in depth against a bad row).

## 8.2 Authorization and tenancy

- **Workspace is the only tenant boundary.** Every tenant table has `workspace_id`; every sqlc query takes `workspace_id` as a required parameter, and the handler gets it only from the authenticated route context (`/workspaces/{ws}` checked against membership, or the API key's workspace).
- **Cross-tenant ids return `404`**, never `403`, so existence doesn't leak.
- **Defence in depth (Phase 5):** Postgres Row-Level Security (RLS) on the tenant tables with `USING (workspace_id = current_setting('app.workspace_id')::uuid)`, set via `SET LOCAL` in the per-request transaction wrapper. The ingest/worker role bypasses RLS explicitly.
- **Tests:** a table-driven "tenant isolation" suite calls every route with a principal from workspace B against resources in workspace A and expects `404`. CI fails on any route that isn't in the suite (the router is introspected).

## 8.3 Authentication and session safety

| Control | Implementation |
|---|---|
| Passwords | argon2id (m = 19 MiB, t = 2, p = 1, the OWASP baseline), 16-byte salt, PHC string. Min 10 chars; checked against a bundled top-100k breached list (a k-anonymity HIBP call is optional). Rehash on login if parameters change |
| Access token | JWT signed with **Ed25519**, 10 min, claims `sub, sid, iat, exp`, no roles (roles are looked up per request with a 30 s cache so revocations apply quickly). Key rotation with `kid` |
| Refresh token | 32 random bytes, stored as sha256; rotated on each use; **reuse detection** (a presented token that was already rotated → revoke the whole `family_id`, email the user) |
| Cookies | Host-only (no `Domain`), `Path=/`, `Secure; HttpOnly; SameSite=Lax` (access) / `Strict` (refresh). `Secure` is disabled only when `APP_ENV=local` |
| CSRF | SameSite plus a double-submit token (`qrit_csrf` cookie ↔ `X-CSRF-Token` header) on every non-GET cookie-authenticated request |
| Login protection | Per-IP-prefix and per-account limits; progressive delays; lockout 15 min after 5 failures; generic error messages (no account enumeration); Cloudflare Turnstile after 3 failures |
| Email flows | Tokens are 32 random bytes, stored hashed, single-use, expiring (verify 24 h, reset 1 h, magic 15 min). A password reset revokes all sessions |
| Two-factor authentication (Phase 5) | TOTP (RFC 6238) with 10 recovery codes (hashed); required for workspace owners on Enterprise |
| Security headers (web) | CSP with nonces (`script-src 'self' 'nonce-…'`; `img-src 'self' data: blob: https://cdn.example.com`; `frame-ancestors 'none'`), HSTS preload, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy` (camera=() etc.) |

## 8.4 Password-protected codes

The code's password is stored as argon2id with lighter parameters (m = 8 MiB) because it's verified on the redirect path. Attempts are limited to 5 per 15 min per `(qr, visitor_hash)` and 100 per 15 min per QR (a Redis counter); beyond that, `429` with a friendly page. There is no password hint and no enumeration of whether a code is protected beyond showing the form.

## 8.5 Rate limiting (Redis GCRA, `go-redis/redis_rate`)

| Surface | Key | Limit |
|---|---|---|
| Redirect (origin, abuse only) | IP /24 prefix | 1,200/min; exceed → `429` page. Cloudflare rule 600/min per IP in front. Fails **open** if Redis is down |
| Redirect password POST | `(qr, vh)` / `qr` | 5 / 15 min · 100 / 15 min |
| Auth endpoints | IP prefix / email | login 10/15 min/IP, 5 failures/account; register 5/h/IP; magic link 3/15 min/email |
| Public render | IP prefix | 20/min, 200/day |
| Abuse reports | IP prefix | 10/h |
| Dashboard API | session | 600/min |
| Public API | API key | plan `api_requests_per_min` (Business 600 / Enterprise 3,000; no API on Free/Pro) |
| Creation velocity | workspace | new workspaces (< 24 h old, Free): max 10 dynamic codes/hour, 30/day |

## 8.6 Webhooks and other outbound requests (SSRF)

All outbound HTTP to customer-supplied URLs (webhooks, destination health checks, domain verification) goes through `platform/httpx.SafeClient`:
- It resolves DNS itself and **rejects private, loopback, link-local, CGNAT, multicast and metadata ranges** (`10/8, 172.16/12, 192.168/16, 127/8, 169.254/16, 100.64/10, ::1, fc00::/7, fe80::/10`, plus `169.254.169.254` and `fd00:ec2::254`).
- It pins the resolved IP for the connection (a custom `DialContext`), which prevents DNS rebinding.
- HTTPS only for webhooks. Redirects are not followed. Timeouts: 5 s total, 2 s connect. Response bodies are capped at 16 KB. `User-Agent: QRit-Webhooks/1.0`.
- Webhook secrets are 32 random bytes, stored AES-256-GCM encrypted with `APP_ENCRYPTION_KEY` (key id prefix for rotation). They're shown once, with a "roll secret" action.

## 8.7 Abuse prevention (quishing)

QR phishing is a large, growing attack class. Keepnet Labs reported QR-based phishing emails rising from about 47k in August to about 249k in November 2025. A dynamic QR platform is an attractive tool for attackers because the destination can be swapped *after* a code has passed review. Controls:

1. **Verified email required** before any dynamic code goes live. Static codes need no account.
2. **Safety check on every destination write** (§8.1 step 6), including every version, rule and split variant. Swapping to a malicious URL later is caught at the moment of the swap.
3. **Continuous rescans:** a daily job re-checks all active destinations of Free workspaces and a 7-day rolling sample of paid ones (Web Risk lists change). A threat hit sets `safety_status = blocked` and `status = blocked`, invalidates the cache, emails the owner, and opens an abuse case.
4. **Creation velocity limits** for new workspaces (§8.5), and signals: disposable email domains blocked (curated list), many workspaces from one IP prefix, destinations on newly registered domains (optional WHOIS-age check in Phase 5).
5. **Reporting:** the `/{code}+` preview page and every status page show "Report this code". Reports go into the staff abuse queue (`/admin/abuse`), where one click blocks the code (410 warning page), suspends the workspace, or dismisses the report. Auto-block after 3 independent reports within 24 h for Free workspaces, pending review.
6. **Reputation isolation:** separate short domains for Free vs paid, and the app, email and short domains kept apart (§4.5). Register the short domains in Google Search Console and monitor Web Risk / Safe Browsing status for our own domains daily. Pre-register with major email security vendors' false-positive channels.
7. **Terms and acceptable use policy** published; the abuse contact is `abuse@example.com`, also in `security.txt`.

## 8.8 Analytics integrity

- Only the `redirect` service can create scan events (the stream isn't reachable from outside the private network; Redis is on Fly's private network with an ACL user whose only permission on `scans` is `XADD`).
- Idempotent ingest (§7.5) plus nightly reconciliation (§7.6).
- Bot, duplicate and velocity classification (§7.3) prevents counter inflation and makes "scan limit" attacks (burning a competitor's `scan_limit`) ineffective, because bots don't count toward it.
- Clock sanity: events with `ts` more than 5 min in the future or more than 7 days in the past are rejected to the DLQ.

## 8.9 Idempotency and consistency

- `Idempotency-Key` middleware (§6.1): the key row is inserted first with `status_code NULL` (in-flight → a concurrent duplicate gets `409 request_in_progress`), and the stored response is written at the end.
- Provider webhooks: `billing_events (provider, provider_event_id)` unique → process-once.
- Outgoing webhooks: `webhook_deliveries (webhook_id, event_id)` unique; receivers get `Webhook-Id` to dedupe.
- Scheduled version activation job: idempotent (`UPDATE … SET current_version_id = $v WHERE id = $qr AND (current_version_id IS DISTINCT FROM $v)`), then cache invalidation.
- River jobs are enqueued **inside** the same transaction as the business write (River's transactional enqueue), so there are no lost or phantom jobs.

## 8.10 Safe deletion, archival and audit logging

- Deletion is always soft first (§5.1). Only owners/admins can delete; there is a confirm dialog that requires typing the code's name for codes with more than 1,000 scans.
- Archived and deleted codes never redirect to anything but the branded status page or the owner's `fallback_url`. A short code is never reissued.
- **Audit log** entries are written in the same transaction as the change for every mutation: auth events (login, password change, 2FA), member/role changes, QR create/update/version/state changes, domain/API key/webhook changes, billing changes, and staff actions (`actor_type = staff`). Each entry records actor, IP prefix, UA, request id, and a before/after diff of the changed fields (secrets redacted). The table is append-only at the database-permission level and exportable as CSV (Business).

## 8.11 Secrets, dependencies and compliance hygiene

- Secrets live only in Fly secrets / GitHub Actions encrypted secrets; `.env` files are local-only and in `.gitignore` (v1 has a committed `backend/.env` in the folder; verify it isn't in git history, and rotate if it is).
- `govulncheck` and `npm audit --omit=dev` in CI; Renovate for dependency PRs; `gitleaks` pre-commit and in CI.
- `security.txt`, a responsible-disclosure page, and a DPA template. SOC 2 is a post-launch enterprise item (§13).

---

# 9. Performance and scaling

## 9.1 Service-level objectives (SLOs)

| SLO | Target | Measured by |
|---|---|---|
| Redirect availability | 99.95% of requests to valid codes return 302/200 (not 5xx) per 30 days | `redirect_requests_total{class}` |
| Redirect latency (origin) | p50 ≤ 10 ms, p99 ≤ 50 ms | `redirect_duration_seconds` histogram |
| End-to-end redirect (phone on 4G, India) | p75 ≤ 250 ms including TLS at the Cloudflare edge | Synthetic probes (Better Stack) from Mumbai, Delhi, Singapore |
| Scan freshness | p95 scan-to-rollup ≤ 5 s | `ingest_lag_seconds` (now − event ts at commit) |
| API | p95 ≤ 200 ms (non-analytics), ≤ 300 ms (analytics, 90 days) | `http_server_duration_seconds{route}` |
| Dashboard | LCP ≤ 1.5 s (desktop), ≤ 2.5 s (mid Android, 4G); INP ≤ 200 ms | `useReportWebVitals` → `/api/vitals` → Grafana |
| Landing generator | Initial JS ≤ 120 KB gz; preview re-render ≤ 16 ms per keystroke | size-limit in CI; performance marks |

## 9.2 Redirect hot-path budget (cache hit, origin)

| Step | Budget |
|---|---|
| Router + normalise + domain map | 0.05 ms |
| LRU lookup | 0.01 ms |
| MaxMind lookups (city + ASN, in-memory mmdb) | 0.02 ms |
| UA mini-classifier + HMAC + rule eval | 0.03 ms |
| Channel push | < 0.01 ms |
| Response write | 0.05 ms |
| **Total** | **≈ 0.2 ms CPU**. A single `performance-1x` core serves ~5–10k redirects/s. Redis-hit path adds ~1 ms; PG-miss path ~3–5 ms (rare; singleflight) |

**Go-specific settings:**
- `GOMAXPROCS` from the container CPU quota (automatic since Go 1.25).
- `GOGC=200` with `GOMEMLIMIT` set to 80% of machine memory.
- `http.Server{ReadHeaderTimeout: 5s, IdleTimeout: 60s}`; keep-alive to Cloudflare.
- Pre-warm the LRU at boot with the top 10k codes by scans in the last 24 h.
- pgx pool 10 conns per redirect instance (misses are rare), 25 for `api`.
- A zero-allocation code path for the hit case (benchmarked with `go test -bench`, target ≤ 5 allocs/op).

## 9.3 Capacity math

| Load | Scans/month | Avg rps | Peak rps (×20) | Redirect instances | Ingest | Postgres |
|---|---|---|---|---|---|---|
| Launch | 5 M | 2 | 40 | 2 (HA minimum) | 1 | 2 CU |
| Growth | 50 M | 19 | 400 | 2–3 | 1 | 4 CU, read replica for analytics |
| Scale | 500 M | 190 | 4,000 | 4–6 across 3 regions | 2–3 (partitioned consumer groups) | → move `scan_events` to ClickHouse (below) |

Ingest throughput: `COPY` of 1,000 rows plus the rollup upserts takes ≈ 30–60 ms, so one consumer handles ~15–30k events/s. That's far above the growth stage.

## 9.4 Scaling path (explicit triggers, no premature infrastructure)

| Trigger | Action |
|---|---|
| Redirect p99 > 50 ms from non-Indian traffic (> 30% of scans outside South Asia) | Add Fly regions `sin`, `fra`, `iad` for `redirect`, each with an Upstash regional read replica. Events are still sent to the primary-region stream (async; latency doesn't matter) |
| Analytics queries p95 > 300 ms or DB CPU > 60% | Route analytics reads to a Postgres read replica (`DATABASE_REPLICA_URL`) |
| `scan_events` > 200 M rows or rollup upserts > 40% of DB time | Stand up **ClickHouse** (ClickHouse Cloud or self-hosted). `ingest` writes raw events to ClickHouse (`ReplacingMergeTree` on `event_id`) and keeps Postgres rollups, or moves rollups to ClickHouse materialized views. The stream design means this is a new sink, not a rewrite |
| Redis Streams memory > 2 GB or > 50k events/s sustained | Move transport to Redpanda/Kafka; same message schema |
| > 50k dynamic codes on one custom-domain customer | Dedicated redirect pool per enterprise tenant (routing by hostname at Cloudflare) |

## 9.5 Frontend performance

- **Landing:** RSC static render, the generator is the only client island, `next/font` for Geist (subset, `display: swap`), no third-party scripts except privacy-friendly analytics (Plausible or self-hosted Umami, loaded `afterInteractive`). Images are AVIF/WebP via the CDN, with explicit width/height.
- **Renderer speed:** the matrix is computed once per payload (memoised by payload + ECC); shape paths are memoised by `(shape, neighbour mask)`; the logo is decoded once into an `ImageBitmap`. The decode check runs in a Web Worker.
- **Dashboard:** route-level code splitting, TanStack Query prefetch on link hover, skeletons, `staleTime` 30 s for lists and 60 s for analytics. The QR list virtualises rows beyond 200 (`@tanstack/react-virtual`). Thumbnails are the CDN images at 80 px (`size=160` for retina), lazy-loaded.
- **Low-end devices:** no heavy animation libraries (CSS transitions only), `content-visibility: auto` on long lists, charts render ≤ 400 points (server downsampling picks the granularity), `prefers-reduced-motion` respected.

## 9.6 Load and performance testing

- **k6 scenarios** (`tests/load/`):
  1. `redirect_hot.js`: 5k rps for 10 min against 1k warm codes → p99 ≤ 50 ms, 0 errors.
  2. `redirect_cold.js`: 500 rps across 1 M distinct codes (cache misses) → p99 ≤ 150 ms, DB CPU < 70%.
  3. `viral_single_code.js`: 3k rps on one code with cache flushed at t = 0 → singleflight verified (DB queries for that code ≤ instance count).
  4. `ingest_burst.js`: 2 M events injected into the stream → drained in < 3 min, reconciliation passes.
  5. `dashboard_api.js`: 200 concurrent analysts over 90-day ranges → p95 ≤ 300 ms.
- **Chaos drills** (staging, before launch): kill Redis → redirects continue (LRU + PG), events buffer and then drain; kill Postgres → cached codes keep redirecting; kill `ingest` for 10 min → lag recovers, counts reconcile; deploy `redirect` during load → zero failed requests (graceful drain: stop accepting, flush the channel, exit within 10 s).

## 9.7 Observability

**Dashboards (Grafana):** redirect RED metrics (rate, errors, duration) by region/outcome; cache hit ratio by layer (target LRU ≥ 90%, combined ≥ 99%); stream length and ingest lag; DLQ size; dropped events; DB connections, slow queries (`pg_stat_statements` top 20); River queue depth and failed jobs; webhook success rate; Web Risk latency and error rate; business metrics (signups, dynamic codes created, scans/day, MRR).

**Alerts (paging):** redirect 5xx > 0.5% for 5 min; redirect p99 > 150 ms for 10 min; `scan_events_dropped_total` increases; ingest lag > 60 s for 5 min; DLQ > 0; canary code `HEALTH01` probe fails from 2+ locations; Postgres storage > 80%; certificate expiry < 14 days on any active custom domain.

**Alerts (ticket):** reconciliation mismatch; webhook endpoint auto-disabled; abuse queue > 20 open reports; any `scan_events_default` rows.

---

# 10. Step-by-step implementation plan

**Working method with Gemini:** generate one phase at a time with the prompt in §11. After each phase: run `make check` (lint + typecheck + unit + integration tests), fix, commit, tag (`v2-phase-N`), and only then ask for the next phase. Never let Gemini regenerate files that you have already reviewed unless you name them.

Effort estimates assume one developer reviewing and fixing Gemini output, working full-time.

---

## Phase 1 — MVP: static generator, accounts, basic dynamic codes (≈ 3 weeks)

**Goals**
- Monorepo, tooling, CI and local stack running with one command.
- Anonymous static generator on the landing page (all static types, full design, PNG/SVG download, scannability meter + decode check).
- Accounts (email+password, Google, magic link), personal workspace, sessions.
- Dynamic URL codes: create, list, detail, edit destination (as versions from day one: v1, v2… even before the history UI exists), pause/resume, archive.
- `redirect` service resolving codes (LRU → Redis → PG) and emitting events to the stream. Events are consumed in Phase 2; until then the stream simply retains them.

**Files/modules to create**
- Root: `pnpm-workspace.yaml`, `turbo.json`, `Makefile`, `docker-compose.yml`, `.github/workflows/ci.yml`, `.editorconfig`, `.gitignore`, `README.md`, `api/openapi.yaml` (auth, me, workspaces, qr-codes, versions, entitlements).
- `packages/qr-render` (complete: matrix, shapes, finder, gradients, logo, frames, warnings) + tests.
- `packages/api-client` (generated).
- `apps/web`: `(marketing)/page.tsx`, `[type]-qr-code/page.tsx`, `(auth)/*`, `w/[workspace]/{layout,page,qr/page,qr/new/page,qr/[id]/page}.tsx`, `features/qr-editor/*`, `features/qr-list/*`, `features/auth/*`, `components/ui/*`, `lib/*`, `proxy.ts`, `next.config.ts`.
- `services`: `cmd/{api,redirect,migrate}`; `internal/{config,platform/*,auth,rbac,workspace,entitlements,qr,version,shortcode,urlsafety (validation only; Web Risk stubbed),resolve,scan (event + visitor hash),apierr,audit,idempotency,email}`; `db/migrations/00001_init.sql`; `db/queries/{users,sessions,workspaces,qr,versions,audit}.sql`; `sqlc.yaml`.
- `apps/render` (Fastify: `/render` for svg/png/pdf, `/healthz`).

**Dependencies**
- Go: `go-chi/chi/v5`, `jackc/pgx/v5`, `sqlc` (tool), `pressly/goose/v3`, `redis/go-redis/v9`, `golang-jwt/jwt/v5`, `golang.org/x/crypto` (argon2), `google/uuid` (v7), `caarlos0/env/v11`, `oschwald/geoip2-golang`, `hashicorp/golang-lru/v2`, `golang.org/x/sync/singleflight`, `getkin/kin-openapi` (contract tests), `stretchr/testify`, `testcontainers-go`.
- TS: `next@16`, `react@19`, `tailwindcss@4`, shadcn/ui, `@tanstack/react-query@5`, `zustand@5`, `react-hook-form`, `zod`, `nuqs`, `qrcode`, `jsqr`, `openapi-fetch`, `openapi-typescript`, `sonner`, `lucide-react`; render: `fastify`, `@resvg/resvg-js`, `pdfkit`, `svg-to-pdfkit`; test: `vitest`, `@playwright/test`.

**Data model changes:** create the full schema from §5.2 now (tables for later phases stay empty). Seed: the platform domain `qr.example.com` (from `PLATFORM_SHORT_DOMAIN`).

**API:** auth (register, login, logout, refresh, verify-email, forgot/reset, magic link, Google), `me`, `workspaces` (list/create/get/patch), `entitlements`, QR create/list/get/patch/pause/resume/archive/unarchive, versions create/list, image, `public/qr-types`, `public/render` (PDF only).

**UI:** landing generator (§3.1–3.2), auth pages, app shell, QR list (table + cards), new QR (editor, dynamic mode), QR detail (header card + Overview with placeholder analytics + Settings), edit-destination dialog with undo toast.

**Testing tasks**
- `qr-render`: snapshot SVGs for 12 presets × 5 payloads; decode round-trip with jsQR for every snapshot rendered to PNG via resvg (in Node); property test: random designs within allowed ranges and score ≥ 50 must decode.
- Go unit tests: shortcode (alphabet, normalisation, denylist), urlsafety (≥ 60 table cases incl. `javascript:`, userinfo, IDN confusables, our own domain), content encoders (Wi-Fi escaping, vCard, UPI), argon2 helpers, JWT, refresh rotation + reuse detection.
- Go integration (testcontainers Postgres + Redis): register → login → create QR → version → resolve → 302; tenant isolation for every route built so far.
- Playwright: anonymous static download (PNG decodes to the input), sign up → create dynamic → `curl` the short URL → 302 to the destination → edit destination → the next request goes to the new URL within 1 s.

**Risks:** renderer quality (shapes/joins) takes longer than expected → timebox to 4 shapes first. Gemini inventing its own schema or endpoints → the prompt forbids deviation; review diffs against §5/§6. Google OAuth consent screen verification lead time → start in week 1.

**Acceptance criteria**
- `make up` starts everything; `make check` is green in CI.
- A new visitor downloads a styled PNG and SVG in ≤ 10 s with no signup, and both decode.
- A signed-up user creates a dynamic code; the redirect returns `302` with `Cache-Control: private, no-store`; changing the destination is live within 1 s; v1/v2 rows exist.
- A resolve cache hit is ≤ 5 ms p99 locally (`hey -z 30s`).
- No IP address is present in any table or log line (`grep` test over a DB dump and logs).

---

## Phase 2 — Analytics (≈ 2 weeks)

**Goals:** the scan pipeline end to end, trustworthy numbers, and analytics UI for one code and for the workspace.

**Files/modules:** `cmd/ingest`; `internal/{scan/botdetect, scan/uaparse, ingest (consumer, batcher, pipeline.sql from §7.5, dlq), analytics (service, queries.sql from §7.6, tz, cache), realtime}`; `cmd/worker` with River (partition maintenance, visitor-ledger pruning, reconciliation); `db/queries/analytics.sql`. Web: `features/analytics/*` (KpiTile, TimeSeriesChart, BarList, Heatmap, WorldMap lazy, FilterBar, DateRangePicker, DataQualityNote, LiveBadge), `w/[workspace]/analytics/page.tsx`, QR detail Overview + Audience tabs, overview page widgets, list sparklines.

**Dependencies:** `ua-parser/uap-go`, `riverqueue/river` + `riverpgxv5`, `recharts`, `d3-geo`, `topojson-client`, `world-atlas`.

**Data model changes:** none (tables exist). Add the River migration (`river migrate-up`) as `00002_river.sql`. Create partitions 3 months ahead at boot if missing.

**API:** analytics `summary`, `timeseries`, `breakdown`, `heatmap`, `top-qr-codes`, `realtime`; `sparkline` embedded in the list response (7 daily points).

**UI:** §3.4 completely, the empty-state "scan this to test" moment with live polling, and the "N automated hits filtered" footnote.

**Testing tasks**
- Ingest replay test: the same batch twice → identical counters (the SQL is already verified; add it as a Go integration test).
- Repeat-visitor test (uniques unchanged) and IST boundary test (events at 23:50 and 00:10 IST land on different local days).
- Bot fixtures: 40 real UA strings (WhatsApp, Slackbot, Safe Links, curl, HeadlessChrome, iPhone Safari, Android Chrome, Samsung Internet) with expected classification.
- Duplicate within 10 s; velocity rule.
- Reconciliation: inject a mismatch → job reports it → `ingest rebuild` fixes it.
- Load: `ingest_burst.js` (2 M events drain < 3 min on a laptop-sized Postgres).

**Risks:** UA parsing cost (keep off the redirect hot path, as designed); rollup contention on one viral code (single-row upsert per batch per 15 min → fine; verify under load); MaxMind licence key setup (free account; download via `geoipupdate` in the Dockerfile build stage).

**Acceptance criteria**
- A phone scan appears in the dashboard within 5 s (p95) and in the realtime badge within 10 s.
- Totals equal `count(*)` over counted raw events for random workspaces (reconciliation green 7 days running in staging).
- Link previews (WhatsApp/Slack paste) and a `curl` don't increase scans; they appear in "automated hits filtered".
- A 90-day workspace analytics page with 1k codes and 1 M events loads in ≤ 300 ms p95 (API) and ≤ 1.5 s LCP.

---

## Phase 3 — Dynamic QR depth (≈ 3 weeks)

**Goals:** everything that makes dynamic codes powerful and safe.

**Scope**
- **Versions UI:** timeline, restore, **scheduled changes** (River `ActivateVersion` job at `effective_at`; cache payload honours `next_change_at`), cancel scheduled.
- **Code settings:** password (argon2id), start/expiry, scan limit, fallback URL, UTM append.
- **Rules and A/B** (Business): the rule builder UI, `internal/routing` engine, resolve-preview, rule/variant analytics.
- **Hosted pages:** vCard, links page, file (PDF/image), event (with .ics); `web/app/p/[code]` + redirect reverse proxy + ISR revalidation hook.
- **Custom domains:** add → DNS instructions → Cloudflare for SaaS custom hostname → verification job → active; root and not-found URLs; domain picker on create.
- **Safety:** the Web Risk client, pending/re-check job, block flow, `/{code}+` preview page, public abuse report endpoint (+ Turnstile), and a minimal staff queue.

**Files/modules:** `internal/{routing, domains (+ cloudflare client), urlsafety/webrisk, abuse}`, `version/scheduler.go`, River workers (`ActivateVersion`, `SafetyRecheck`, `DomainVerify`); redirect: password form, status pages (Go `html/template`, inline CSS, ≤ 5 KB each, EN + HI strings), preview page, `/p/*` proxy. Web: `features/qr-detail/{versions,rules,settings}`, `features/domains`, `app/p/[code]`, `app/admin/abuse`.

**Dependencies:** `cloudflare/cloudflare-go/v4` (custom hostnames), Web Risk REST (plain `net/http`), `ics` generation (hand-written, ~50 lines).

**Data model changes:** none structural. Add a migration for the RLS policies (disabled by default; enabled in Phase 5).

**API:** version restore/cancel, `resolve-preview`, domains CRUD + verify, `public/abuse-reports`, files upload (logos/hosted assets), staff endpoints under `/v1/admin/*` (staff-only middleware).

**Testing tasks**
- Routing engine: ≥ 80 table cases (every field/op, wrap-around times in IST, weekday in tz, split distribution within ±2% over 100k synthetic visitors, stickiness).
- Scheduled version flips at the right instant (fake clock) and the redirect honours `next_change_at` without waiting for invalidation.
- Password brute-force limit.
- Custom domain happy path against the Cloudflare API sandbox (or a mocked client) + e2e with `Host:` header overrides.
- Hosted pages Lighthouse ≥ 95 performance/accessibility on mobile.
- Security: a destination swap to a Web Risk test URL is rejected; a rescan hit blocks the live code within 1 minute.

**Risks:** Cloudflare for SaaS setup (fallback origin, TXT/HTTP validation) has sharp edges → spike it in week 1 of the phase. Rule-builder UX complexity → ship templates ("iOS/Android app store router", "Business hours", "A/B 50/50") before the free-form builder.

**Acceptance criteria**
- A version scheduled for 00:00 IST takes effect by 00:00:01 IST (fake-clock test + staging check).
- The app-store router sends iPhone UA → App Store and Android UA → Play Store, and resolve-preview agrees for the same inputs.
- `go.customer-domain.test` serves codes with valid TLS within 10 minutes of the correct CNAME.
- A reported malicious code can be blocked from `/admin/abuse` in 1 click and shows the warning page within 1 s.

---

## Phase 4 — Premium dashboard: teams, campaigns, templates, bulk, billing, API (≈ 3 weeks)

**Scope**
- **Teams:** invites, roles (§6.6), seat enforcement, ownership transfer, member removal.
- **Organisation:** folders (tree, drag-drop move), tags, campaigns (+ campaign analytics page with goal progress), templates (brand-locked mode).
- **Bulk:** CSV upload → column mapping UI with a 5-row preview → validation report → River `BulkCreate` job (chunks of 500 in transactions) → progress polling → ZIP download (`BulkDownload` job via `render`; SVG + PNG + PDF + `manifest.csv`).
- **Exports:** QR list CSV, scans CSV (Business raw log).
- **Billing:** `billing.Provider` interface with Stripe and Razorpay implementations; checkout, portal, webhooks → `billing_events` → subscription sync → entitlements invalidation; trial (14-day Pro, no card); **downgrade handling** (mark codes beyond the limit as `is_read_only` in newest-first order; never deactivate).
- **Public API:** API keys (create/revoke/scopes), key auth middleware, per-key limits, and the docs site generated from OpenAPI (Scalar or Redoc at `/docs/api`).
- **Webhooks:** CRUD, test, deliveries log, retry, auto-disable; SSRF-safe client.
- **Polish:** ⌘K command palette (navigate, create, search codes), keyboard shortcuts, onboarding checklist (create → download → scan → invite), usage meter, upgrade cards, pricing page with USD/INR.

**Files/modules:** `internal/{billing/{provider.go,stripe,razorpay}, webhooks, jobs/bulk*, jobs/export*}`, `platform/httpx/safeclient.go`; web `features/{team,campaigns,templates,billing,api-keys,webhooks,audit-log,bulk}`, `components/layout/CommandPalette.tsx`, `(marketing)/pricing`.

**Dependencies:** `stripe/stripe-go` (current major), `razorpay/razorpay-go`, `encoding/csv`, `archive/zip`; web `cmdk`, `@dnd-kit/core`, `papaparse` (client-side preview only).

**Data model changes:** none structural; seed Stripe/Razorpay price ids via env.

**API:** team, invites, folders, tags, campaigns, templates, jobs, files, api-keys, webhooks (+ deliveries/retry), billing, audit-logs.

**Testing tasks:** role matrix tests for every route (§6.6); bulk job with 5,000 rows incl. 50 invalid rows → a precise error report, and a re-run with the same Idempotency-Key doesn't duplicate; billing webhook replay (the same event twice → one state change); downgrade from Business (400 codes) to Free → 397 read-only, all 400 still redirect; webhook signature verification sample in Node + Python in docs; SSRF tests (webhook to `http://169.254.169.254`, to a DNS name resolving to `10.0.0.1`, redirect-to-private).

**Risks:** Stripe availability for an Indian entity (see §13) → Razorpay first if needed, and the interface keeps this swappable. Bulk ZIPs are large → stream to R2 multipart, never buffer in memory.

**Acceptance criteria**
- An editor invited by an admin can create codes but can't see Billing or API keys; an analyst can't create.
- 5,000-row bulk create completes in < 2 min with a downloadable ZIP.
- Cancelling a paid plan in the provider's portal returns the workspace to Free within 1 minute of the webhook, with all codes still redirecting.
- A third-party script using an API key can create a code, change its destination and read its analytics following the docs alone.

---

## Phase 5 — Hardening (≈ 2 weeks)

**Scope**
- Security: RLS enabled; 2FA (TOTP); CSP nonces verified; session management UI; `gitleaks`/`govulncheck` gates; external pen-test checklist (OWASP ASVS L2 self-assessment).
- Abuse: separate Free-tier short domain, disposable-email blocklist, creation-velocity limits, rescan jobs, auto-block on reports, abuse runbook.
- Reliability: multi-region `redirect` (`sin`, `fra`, `iad`), a Postgres read replica for analytics, backups + restore drill (restore the PITR snapshot into staging, run reconciliation), graceful shutdown verified, chaos drills (§9.6).
- Observability: dashboards and alerts (§9.7), status page, canary code `HEALTH01` probes.
- Performance: all k6 scenarios pass at 2× the launch target.
- **v1 migration** (only if v1 has real users/codes, §13): export v1 `users`, `workspaces`, `qr_records` (dynamic), `qr_scans` (aggregates only, not raw IPs) → import script `cmd/migrate-v1` → dynamic v1 codes get `legacy_short_code`; the v2 redirect serves `/r/{legacyCode}` on the v1 host (case-sensitive lookup) with identical behaviour. Users get a password-reset email (bcrypt hashes can be verified once at login and rehashed to argon2id instead, if you prefer seamless login).

**Acceptance criteria:** zero high/critical findings open; restore drill completed in < 1 h with verified data; redirect 99.95% availability over a 7-day staging soak with chaos events; every alert fires in a drill and routes to the on-call channel; migrated v1 codes redirect correctly (sample of 100% of dynamic codes via script).

---

## Phase 6 — Launch (≈ 1 week)

**Scope:** production environment provisioned from the same manifests as staging; DNS cut-over for the short domains; SEO pages (all content types, `sitemap.xml`, `robots.txt`, OpenGraph images, schema.org `SoftwareApplication` + `FAQPage`); docs (getting started, "How we count", API reference, webhooks, custom domains, printing guide); legal pages (terms, privacy, DPA, acceptable use, security.txt); transactional email templates (EN, plus HI for key flows); launch analytics (product funnel events via the privacy-friendly analytics tool: `generator_download`, `signup`, `dynamic_created`, `first_scan`, `upgrade`).

**Rollout:** private beta (50 users, 1 week) → fix list → public launch (Product Hunt, Indian startup communities, college networks) → watch the SLO dashboards hourly for 72 h. Feature flags (`flags` table or env) for rules/bulk/billing so issues can be switched off without a deploy.

**Acceptance criteria:** the first 1,000 external scans reconcile exactly; no P1 incidents in 72 h; signup → first dynamic code conversion is measured and baselined; the pricing page and checkout work in USD and INR.

---

# 11. Gemini code-generation handoff prompt

**How to use it**
1. Open a new Gemini chat, using the strongest coding model available with the longest output setting.
2. Paste the **entire contents of `GEMINI_PROMPT.md`**. That file is self-contained and includes the full schema, ingest and analytics SQL that are only referenced below.
3. Gemini will output `docs/manifest/phase-1.txt` and then the files. Each time it prints `<<CONTINUE FROM: …>>`, reply `continue`.
4. Save the files into the repo, run `make gen && make check`, and fix or re-prompt for specific files (`Regenerate only services/internal/routing/engine.go; tests in engine_test.go fail with: <paste>`).
5. When Phase 1 is green, commit and tag it, then send `Phase 2`. Repeat through Phase 6.

**Tips from experience with code-generation models**
- Keep one chat per phase if the conversation gets long. Start the new chat with the prompt plus `The repository already contains Phases 1–N. Begin Phase N+1. Here is docs/manifest/phase-N.txt: …`.
- When Gemini drifts (new table, renamed field), reply only: `Violation of rule 8: <what>. Re-emit <file> conforming to the spec.`
- Review the security-critical files by hand, line by line. The prompt fixes their behaviour, but a human must verify them: `auth/*`, `urlsafety/*`, `platform/httpx/safeclient.go`, `resolve/*`, `ingest/pipeline.go`, `idempotency/*`, `billing/*/webhook.go`.

The full prompt follows. The three SQL blocks marked *(see §5.2 / §7.5 / §7.6)* are included verbatim in `GEMINI_PROMPT.md`.

````text
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
-- (see §5.2 — full DDL is included verbatim in GEMINI_PROMPT.md)
```

- `00002_river.sql`: River's migration SQL for the River version in go.mod (use `rivermigrate` programmatically in `cmd/migrate` instead of copying SQL if simpler — DECISION allowed).
- Down migrations drop in reverse dependency order.
- `sqlc.yaml`: engine postgresql, schema `db/migrations`, queries `db/queries`, gen go `package: dbgen`, `out: internal/platform/db/dbgen`, `sql_package: pgx/v5`, `emit_json_tags: true`, `emit_pointers_for_null_types: true`, `emit_empty_slices: true`, overrides: `uuid` → `github.com/google/uuid.UUID` (nullable → `*uuid.UUID` via `github.com/google/uuid.NullUUID` is NOT used; use pointer), `timestamptz` → `time.Time`, `jsonb` → `encoding/json.RawMessage`, `citext` → `string`.

**Ingest batch transaction** (implement in `internal/ingest/pipeline.go` as these exact statements; load `staging` with `pgx.CopyFrom`):

```sql
-- (see §7.5 — full SQL is included verbatim in GEMINI_PROMPT.md)
```

**Analytics queries** (put in `db/queries/analytics.sql` verbatim; add `BreakdownRaw<Dimension>` siblings for region, city, device, os, browser, language, referrer, utm_source, rule, version using the same pattern; add `TimeseriesHourly` and `TimeseriesWeekly` using `date_trunc('hour'|'week', bucket_start AT TIME ZONE tz)` with zero-fill):

```sql
-- (see §7.6 — full SQL is included verbatim in GEMINI_PROMPT.md)
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
````

---

# 12. Final recommended stack

| Layer | Choice | Version (Sept 2026) | Why this, not the alternative |
|---|---|---|---|
| Edge | Cloudflare (Pro) + Cloudflare for SaaS + R2 | — | Global TLS/WAF/CDN, geo headers for free, managed custom-domain certificates, no-egress-fee object storage. Alternative: Caddy on-demand TLS (self-managed, more ops) |
| Frontend | Next.js (App Router) + React + TypeScript | 16.x / 19.2 / strict | RSC for fast SEO pages + rich client dashboard in one app; v1 already uses it |
| UI kit | Tailwind CSS v4 + shadcn/ui (Radix) | 4.x | Accessible primitives, you own the code, and it gives the premium look without a heavy design library |
| Client data | TanStack Query v5, Zustand v5, nuqs, react-hook-form + zod | — | Server cache vs local state vs URL state are cleanly separated |
| Charts | Recharts v3 (lazy) + d3-geo map | — | Enough for bar/area/heatmap, and small when code-split |
| QR rendering | Own `@qrit/qr-render` on `qrcode` (node-qrcode); resvg + pdfkit on the server | — | One renderer for the browser and server gives an exact match. `qr-code-styling` (v1) is canvas/DOM-oriented and hard to run identically on the server |
| Backend language | Go | 1.27 | Low-latency redirect, cheap concurrency, single static binaries, your strongest backend language. Django 4.2 (v1) is out of security support |
| HTTP | chi v5 | — | stdlib-compatible, minimal, no framework lock-in |
| Data access | pgx v5 + sqlc + goose | — | Type-safe SQL you can read; no ORM surprises on a performance-sensitive schema |
| Jobs | River (Postgres-backed) | — | Transactional enqueue with business writes, no extra broker; periodic jobs and retries built in |
| Event transport | Redis Streams (consumer groups) | Redis 7 | Durable enough, simple, already needed for cache/limits; swap to Redpanda/Kafka at > 50k events/s |
| Database | PostgreSQL (Neon, `ap-south-1`) | 17 | Partitioning + rollups cover the growth stage; branching for previews; read replicas when needed |
| Analytics store (later) | ClickHouse | — | Only when `scan_events` > 200 M rows (§9.4); the ingest design makes it a new sink |
| Cache / limits | Redis (Upstash via Fly) | 7 | Managed, private networking; self-host on Fly if cost grows |
| Hosting | Fly.io (`bom` primary; `sin`/`fra`/`iad` for redirect later) | — | Mumbai region close to users, multi-region anycast for the redirect, simple Docker deploys. Alternatives: Railway/Render (simpler, no multi-region anycast), AWS ECS (more ops) |
| Email | Resend | — | Already integrated in v1; good DX |
| Payments | Razorpay (INR, UPI Autopay) + Stripe (international) behind one interface | — | Indian customers pay with UPI/cards in INR; see §13 on Stripe onboarding |
| URL safety | Google Web Risk API | v1 | Commercial-use licence (Safe Browsing API is non-commercial) |
| Geo | Cloudflare headers → MaxMind GeoLite2 City + ASN | — | Free, local lookups in microseconds, no IP leaves our infrastructure |
| Observability | OpenTelemetry → Grafana Cloud; Sentry; Better Stack uptime + status page | — | Traces, metrics, logs in one place; generous free tiers at launch |
| CI/CD | GitHub Actions → Fly deploy; Renovate; gitleaks; govulncheck | — | Standard, cheap, reproducible |
| Testing | Go testing + testify + testcontainers; Vitest; Playwright; k6 | — | Real Postgres/Redis in tests, real browser e2e, load tests with thresholds |

**Estimated launch run-rate:** roughly **$80–150/month** (Fly machines, Neon, Upstash, Cloudflare Pro, domains), excluding payment fees. Verify against current vendor pricing before committing.

**If you'd rather keep Django** (not recommended, but viable): keep the control plane in Django 5.2 LTS + DRF with the same schema (use `managed = False` models over the goose migrations or port them to Django migrations), and still build `redirect` and `ingest` in Go exactly as specified. That split keeps the hot path fast. The price is two backend languages and a duplicated rule engine (Python for resolve-preview, Go for scans), which must then share test vectors.

---

# 13. Risks and open questions

## 13.1 Risks

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R1 | **The short domain gets flagged** by Safe Browsing / email filters because of abuse → every customer's codes show warnings | Medium | Severe | Separate app, email and short domains; free-tier short domain isolated (Phase 5); verified email before dynamic; Web Risk on every write + rescans; fast staff takedown; Search Console monitoring; custom domains for paid tiers |
| R2 | Gemini output drifts from the spec or contains subtle security bugs | High | High | Strict prompt rules, manifest-first generation, `make check` gates, contract tests, tenant-isolation and RBAC test suites, manual review list (§11) |
| R3 | Renderer visual quality / scannability regressions | Medium | High | Decode round-trip tests for every preset in CI; in-browser decode check blocks unscannable downloads |
| R4 | Analytics numbers disagree with customers' GA4 | High (always some) | Medium | Published definitions; bot/duplicate transparency; UTM append so GA4 sees the same campaign; raw log export |
| R5 | "Never deactivate" costs grow with free users | Low | Low–Medium | Redirects cost ~0.2 ms CPU each; free dynamic codes are capped at 3; abusive accounts are blocked by the abuse pipeline, not by the plan |
| R6 | Vendor sprawl (Fly, Neon, Upstash, Cloudflare, Resend, Grafana) | Medium | Medium | Everything is containerised with plain env config; Postgres/Redis are standard protocols; a documented exit path per vendor |
| R7 | Timezone/DST edge cases in schedules and analytics | Medium | Medium | All storage in UTC; 15-minute rollups; IANA zones only; DST tests for America/New_York and Europe/London alongside IST |
| R8 | Stream loss window (≤ 50 ms of buffered events on hard crash) | Low | Low | Graceful shutdown drains the buffer; loss only on `SIGKILL`/host failure; `scan_events_dropped_total` alerting |
| R9 | Custom-domain TLS provisioning failures | Medium | Medium | Clear DNS instructions + live verification UI; retries for 72 h; certificate-expiry alerts |
| R10 | Solo-developer bus factor and scope creep | High | High | Phase gates with acceptance criteria; hosted page types limited to 4 at launch; enterprise features follow the Enterprise Plan's phase gates (Enterprise Core first) |

## 13.2 Open questions (decisions I need from you)

1. **Is v1 live with real users or printed codes?** If yes, Phase 5 includes `cmd/migrate-v1` and the v1 host must keep serving `/r/{code}` forever. If v1 was only a college/demo deployment, drop the migration work and the `legacy_short_code` handling (saves about 3 days).
2. ~~Go for the whole backend, or keep Django for the control plane?~~ **Answered 26 Sep 2026: Go (v2 blueprint).** Enterprise features are specified in `QRit_v2_Enterprise_Plan.md`.
3. **Legal entity and payments:** will you bill as an Indian entity? Stripe has restricted new Indian business signups in the past, so verify current availability before relying on it; Razorpay is the safe default for INR and UPI Autopay. For international customers, consider a merchant-of-record (e.g. Paddle or Lemon Squeezy) to avoid handling global sales tax/VAT yourself. The `billing.Provider` interface supports adding one.
4. **Brand and domains:** is "QRit" final? Check trademark conflicts and buy a **short** domain for links (≤ 8 characters before the TLD keeps the QR at version 2–3). Buy the free-tier short domain at the same time.
5. **Pricing:** the proposed tiers are Free / Pro ~$9–12 (₹499) / Business ~$39–49 (₹2,999) / Enterprise custom, with gates as in §5.7. Confirm, or tell me your target customer (restaurants vs agencies vs retail brands). That choice changes which features to gate.
6. **EPS export:** print shops still ask for EPS. It's deferred; if your target customers are print-heavy, add it in Phase 4 (render SVG → EPS via a small Ghostscript sidecar).
7. **Hosted page types at launch:** vCard, links page, file (PDF/menu), event. Confirm, or swap one for coupon/feedback-form (v1 had lead pages).
8. **Data residency:** is India-only storage (Neon `ap-south-1`) a selling point for your market, or should EU customers get EU storage later (Enterprise)?
9. **AI features** (e.g. "describe your QR" → design, AI-art QR codes): deliberately excluded from v2 to protect scannability and scope. Revisit after launch with data.

## 13.3 Assumptions recorded in this document

- The GeoLite2 licence (free, with attribution) is acceptable; city-level accuracy is approximate, and the UI labels locations "approximate".
- A daily-scoped, per-QR "unique scan" definition is acceptable for the product (§7.2).
- The Cloudflare Pro plan is sufficient; "Add visitor location headers" is available as a managed transform on the zone (verify on your plan; if it isn't, MaxMind covers city/region).
- The v1 committed `backend/.env` contains only local development secrets. Verify it isn't in git history, and rotate anything that was ever used in production.

---

## Sources

- [QR Code Generator — pricing (plans, dynamic code and scan limits)](https://www.qr-code-generator.com/pricing/)
- [QR Code Generator support — what happens to dynamic codes when the trial expires](https://support.qr-code-generator.com/hc/en-us/articles/7665046137613-What-happens-to-my-account-and-QR-Codes-when-the-trial-expires)
- [Front Desk Review — QR Code Generator pricing & compliance (checked 3 Sep 2026)](https://frontdeskreview.com/software/qr-code-generators/qr-code-generator-com/)
- [Front Desk Review — Uniqode pricing (checked 3 Sep 2026)](https://frontdeskreview.com/software/qr-code-generators/uniqode/)
- [QR TIGER — pricing](https://www.qrcode-tiger.com/payment)
- [QR-Verse — QR TIGER teardown 2026](https://qr-verse.com/en/blog/qr-tiger-teardown-2026)
- [Hovercode — pricing](https://hovercode.com/pricing/)
- [Hovercode — 14 dynamic QR generators compared](https://hovercode.com/blog/best-dynamic-qr-generators/)
- [Mobiqode — Flowcode review (Mar 2026)](https://www.mobiqode.com/blog/flowcode-review/)
- [Permanent QR Codes — Bitly link & QR limits by plan (2026)](https://permanentqrcodes.com/blog/bitly-link-and-qr-code-limits/)
- [AZ Big Media — the end of QR code subscription traps](https://azbigmedia.com/blogs/the-end-of-qr-code-subscription-traps-why-the-industry-is-shifting/)
- [North Penn Now — your QR code stops working the moment you stop paying (Apr 2026)](https://northpennnow.com/news/2026/apr/02/your-qr-code-stops-working-the-moment-you-stop-paying/)
- [Keepnet Labs — QR phishing (quishing) statistics](https://keepnetlabs.com/blog/qr-code-phishing-trends-in-depth-analysis-of-rising-quishing-statistics)
- [Unitag — GS1 Sunrise 2027 explained](https://www.unitag.io/blog/gs1-2027-deadline/)
- [Go release history (Go 1.27)](https://go.dev/doc/devel/release)
- [VersionLog — Django releases and support timeline](https://versionlog.com/django/)
