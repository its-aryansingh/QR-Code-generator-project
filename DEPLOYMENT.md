# QRit Production Deployment Guide

This guide covers deploying the QRit QR Code SaaS platform (Django 4.2 REST API + Next.js 16 App Router + PostgreSQL) to production.

---

## 1. Architecture Overview

```
[ Clients / Mobile / Web Browsers ]
                 │
                 ▼
         [ HTTPS / SSL ]
                 │
  ┌──────────────┴──────────────┐
  │                             │
  ▼                             ▼
[ Next.js Frontend ]   [ Django Backend (Gunicorn) ]
(Port 3000, Node.js)   (Port 8084, Multi-threaded)
                                │
                        [ WhiteNoise Static ]
                                │
                                ▼
                       [ PostgreSQL 16 ]
```

---

## 2. Environment Variables Checklist

### Backend (`backend/.env`)

| Variable | Required in Prod? | Description | Example / Default |
| :--- | :--- | :--- | :--- |
| `DEBUG` | **Yes** | Must be set to `False` | `False` |
| `PORT` | Optional | HTTP listen port | `8084` |
| `DJANGO_SECRET_KEY` | **Yes** | Min 50 characters, cryptographically random | `generate-long-random-string` |
| `ALLOWED_HOSTS` | **Yes** | Comma-separated list of accepted host headers | `api.yourdomain.com,yourdomain.com` |
| `DATABASE_URL` | **Yes** | PostgreSQL connection URL | `postgresql://user:pass@host:5432/dbname` |
| `DB_CONN_MAX_AGE` | Optional | Database connection pooling lifetime (sec) | `600` |
| `DB_SSL_REQUIRE` | Cloud DBs | Enforce SSL on PostgreSQL connections | `True` |
| `JWT_SECRET` | **Yes** | Min 64 chars. Server halts on startup if default! | `generate-64-char-random-secret` |
| `JWT_EXPIRY_MINUTES` | Optional | Access token lifetime | `15` |
| `REFRESH_EXPIRY_DAYS` | Optional | Refresh token lifetime | `7` |
| `APP_BASE_URL` | **Yes** | Public frontend URL | `https://app.yourdomain.com` |
| `API_BASE_URL` | **Yes** | Public backend API URL | `https://api.yourdomain.com` |
| `SHORT_LINK_BASE_URL` | Optional | Custom short-link domain (falls back to API) | `https://api.yourdomain.com` |
| `CORS_ALLOWED_ORIGINS`| **Yes** | Trusted frontend origins for cross-origin API | `https://app.yourdomain.com` |
| `CSRF_TRUSTED_ORIGINS`| **Yes** | Trusted origins for CSRF checks | `https://app.yourdomain.com` |
| `SECURE_SSL_REDIRECT` | Optional | Redirect all HTTP traffic to HTTPS | `True` |
| `EMAIL_BACKEND` | Optional | `console` for dev, `resend` for production (stored as `QRIT_EMAIL_BACKEND`) | `resend` |
| `RESEND_API_KEY` | If Resend | Resend API key for transactional emails | `re_...` |
| `EMAIL_FROM` | If Resend | Verified sender email | `noreply@yourdomain.com` |
| `GOOGLE_CLIENT_ID` | For Google sign-in | Google OAuth **Web application** client ID | `...apps.googleusercontent.com` |
| `GOOGLE_CLIENT_SECRET` | For Google sign-in | Secret of the same client | `GOCSPX-...` |
| `GITHUB_CLIENT_ID` | For GitHub sign-in | GitHub **OAuth App** client ID | `Ov23li...` |
| `GITHUB_CLIENT_SECRET` | For GitHub sign-in | Secret of the same OAuth App | `...` |
| `OAUTH_CALLBACK_BASE_URL` | Optional | Host the providers redirect back to. Defaults to `APP_BASE_URL` (the frontend proxies `/api/v1`) | `https://app.yourdomain.com` |
| `STRIPE_SECRET_KEY` | Optional | Stripe Live Secret Key | `sk_live_...` |
| `STRIPE_WEBHOOK_SECRET`| Optional | Stripe Webhook Signing Secret | `whsec_...` |
| `LOG_LEVEL` | Optional | Console logging level | `INFO` |

### Frontend (`frontend/.env.local` or platform variables)

| Variable | When read | Required in Prod? | Description |
| :--- | :--- | :--- | :--- |
| `BACKEND_INTERNAL_URL` | Runtime | **Yes** | Where the frontend's `/api/v1/*` proxy forwards to, e.g. `http://${{qrit-backend.RAILWAY_PRIVATE_DOMAIN}}:8084` |
| `NEXT_PUBLIC_API_URL` | Build | **No, leave unset** | Only for serving the API from a different origin. Unset, the browser calls the same-origin proxy |
| `NEXT_PUBLIC_API_BASE_URL` | Build | Recommended | Public backend URL, used to display short links (`/r/<code>`) |
| `NEXT_PUBLIC_APP_URL` | Build | Recommended | Frontend public URL |

> [!NOTE]
> The browser never talks to the backend directly. It calls `/api/v1/*` on the
> frontend, and `src/app/api/v1/[...path]/route.ts` forwards each request to
> `BACKEND_INTERNAL_URL`, which is read **at runtime**. One image therefore works
> in every environment, no CORS preflight is involved, and the OAuth state cookie
> and callback stay on the site the user is on. `NEXT_PUBLIC_*` values are still
> baked in at **build time**, which is why the API URL is no longer one of them.

---

## 3. Deployment Methods

### Method A: Render.com (Recommended Blueprint)

The repository includes a ready-to-use [`render.yaml`](file:///c:/Users/user/OneDrive/Desktop/Projects/qr_code_project/render.yaml) blueprint declaring the backend service, frontend service, and managed PostgreSQL instance.

1. Connect your repository to [Render](https://render.com).
2. Create a new **Blueprint** and select this repository.
3. Render will detect `render.yaml` and provision:
   - `qrit-db`: Managed PostgreSQL database.
   - `qrit-backend`: Python 3.11 web service with Gunicorn and automatic `collectstatic` + `migrate`.
   - `qrit-frontend`: Node.js 20 web service with Next.js standalone build.
4. Fill in any un-synced secrets (Stripe, Resend, Google OAuth) in the Render Dashboard.
5. Deploy!

### Method B: Docker Compose (Self-Hosted / VPS)

The [`docker-compose.yml`](file:///c:/Users/user/OneDrive/Desktop/Projects/qr_code_project/docker-compose.yml) orchestrates PostgreSQL, Gunicorn Backend, and Next.js Frontend with built-in healthchecks.

1. Copy `.env.example` to `.env` in the project root and populate production secrets:
   ```bash
   cp backend/.env.example .env
   ```
2. Build and start the cluster:
   ```bash
   docker compose up --build -d
   ```
3. Check service health:
   ```bash
   docker compose ps
   ```
4. View logs:
   ```bash
   docker compose logs -f
   ```

### Method C: DigitalOcean App Platform / Heroku / Fly.io

The repository includes standard `Procfile` configurations:

```procfile
release: cd backend && python manage.py collectstatic --noinput && python manage.py migrate --noinput
web: cd backend && gunicorn qrapp.wsgi:application --config gunicorn.conf.py
```

- In DigitalOcean or Heroku, the `release` phase runs migrations and gathers static files before routing traffic to the newly deployed container, ensuring zero-downtime releases.

### Method D: Railway.app (Monorepo Setup)

Deploy the monorepo as **two services from the same GitHub repository** plus a
PostgreSQL database. Both services ship a `railway.json` (Dockerfile build,
health check, restart policy); the backend's also runs migrations as a
pre-deploy step.

#### 1. Database
- **+ New -> Database -> PostgreSQL.**

#### 2. Backend service (`qrit-backend`)
- **+ New -> GitHub Repo** -> this repository. **Settings -> Root Directory:** `backend`.
- **Settings -> Networking:** generate a public domain only if you want short links
  (`/r/<code>`) served from it; the frontend reaches the API privately.
- **Variables:**

  | Variable | Value |
  | :--- | :--- |
  | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` |
  | `DEBUG` | `False` |
  | `PORT` | `8084` (pinned so the frontend can reference it) |
  | `DJANGO_SECRET_KEY` | 50+ random characters |
  | `JWT_SECRET` | 64+ random characters |
  | `ALLOWED_HOSTS` | your custom API domain, or leave unset (`*`) |
  | `APP_BASE_URL` | `https://${{qrit-frontend.RAILWAY_PUBLIC_DOMAIN}}` (or your custom frontend domain) |
  | `API_BASE_URL` / `SHORT_LINK_BASE_URL` | `https://${{RAILWAY_PUBLIC_DOMAIN}}` |
  | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | from Google Cloud (section 7) |
  | `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | from GitHub (section 7) |
  | `EMAIL_BACKEND`, `RESEND_API_KEY`, `EMAIL_FROM` | for real emails |

  Railway's own hosts (`healthcheck.railway.app`, `RAILWAY_PUBLIC_DOMAIN`,
  `RAILWAY_PRIVATE_DOMAIN`) are added to `ALLOWED_HOSTS` automatically, and the
  `APP_BASE_URL` origin is added to CORS/CSRF automatically.

#### 3. Frontend service (`qrit-frontend`)
- **+ New -> GitHub Repo** -> same repository. **Settings -> Root Directory:** `frontend`.
- **Settings -> Networking:** generate the public domain (this is the URL users visit).
- **Variables:**

  | Variable | Value |
  | :--- | :--- |
  | `BACKEND_INTERNAL_URL` | `http://${{qrit-backend.RAILWAY_PRIVATE_DOMAIN}}:${{qrit-backend.PORT}}` |
  | `NEXT_PUBLIC_API_BASE_URL` | `https://${{qrit-backend.RAILWAY_PUBLIC_DOMAIN}}` |
  | `NEXT_PUBLIC_APP_URL` | `https://${{RAILWAY_PUBLIC_DOMAIN}}` |

  **Delete `NEXT_PUBLIC_API_URL`** if an older setup defined it. If private
  networking is unavailable in your project, `BACKEND_INTERNAL_URL` can point at
  the backend's public `https://` domain instead.

#### What was breaking authentication on Railway (fixed)
| Symptom | Cause | Fix |
| :--- | :--- | :--- |
| Login/register requests went to `localhost:8084` | `NEXT_PUBLIC_API_URL` is baked at build time and defaulted to localhost in the Dockerfile | Same-origin `/api/v1` proxy, backend URL read at runtime |
| Deploys never turned healthy | Railway's HTTP health check was 301-redirected to HTTPS, and `healthcheck.railway.app` was not an allowed host | `/health` exempt from the HTTPS redirect; Railway hosts auto-allowed |
| Disallowed hosts returned 500 instead of 400 | `EMAIL_BACKEND=console` overwrote Django's mail setting and crashed the error mailer | App setting renamed to `QRIT_EMAIL_BACKEND` |
| Browser blocked API calls (CORS) | Frontend origin not in `CORS_ALLOWED_ORIGINS` | `APP_BASE_URL` origin always allowed; the proxy removes CORS from the path anyway |
| Fresh deploys hit missing tables | Migrations only ran when `RUN_MIGRATIONS` was set | `preDeployCommand: python manage.py migrate` |
| Backend unreachable on the private network | Gunicorn listened on IPv4 only, `--bind` ignored `$PORT` | Binds `[::]:$PORT` (dual-stack), falling back to IPv4 when the host has no IPv6 |
| Password change in Profile did nothing | It posted to `/user/profile`, which ignores passwords | Uses `/auth/change-password` |

---

## 4. Database Migrations

When releasing new code manually or in custom CI/CD:

```bash
cd backend
python manage.py migrate --noinput
```

To verify migration status:
```bash
python manage.py showmigrations
```

---

## 5. Health Checks

Both services feature automated health endpoints:

- **Backend Health**: `GET /health`
  - Deep check: verifies database connection and returns `200 OK` or `503 Service Unavailable`.
  - Shallow check: `GET /health?shallow=1` returns immediate `200 OK` for high-frequency load balancer pings without database overhead.
- **Frontend Health**: `GET /api/health`
  - Returns `{"status": "ok", "service": "qrit-frontend"}` for ingress/proxy health checks.

---

## 6. Production Security Features Included

- **Gunicorn WSGI**: Multi-worker, multi-threaded configuration with request recycling (`max_requests = 1000`) to prevent memory leaks.
- **WhiteNoise**: Static file compression and caching with unique manifest hashes.
- **Security Headers**:
  - `Strict-Transport-Security` (HSTS with preload)
  - `X-Frame-Options: DENY` (clickjacking protection)
  - `X-Content-Type-Options: nosniff` (MIME-type sniffing protection)
  - `Referrer-Policy: strict-origin-when-cross-origin`
- **Reverse Proxy SSL Header**: `SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")` prevents redirect loops behind Cloudflare, AWS ALB, or Render.
- **JWT Startup Check**: Server immediately halts on startup in production if default development secrets are detected.
- **Brute Force Protection**: Account lockout after 5 consecutive failed login attempts (15-minute window).
- **Token Rotation**: Persistent refresh tokens with automatic family revocation upon reuse detection.

---

## 7. Google and GitHub Sign-in

Sign-in uses the OAuth 2.0 authorization-code flow with PKCE, handled by the
backend (`backend/api/utils/oauth.py`). The frontend only links to
`/api/v1/auth/oauth/<provider>/start` and finishes on `/auth/callback`, where a
single-use, 2-minute code is exchanged for the normal JWT pair; tokens never
appear in a URL.

**Redirect URIs** (replace the domain with your frontend's public domain, i.e. `APP_BASE_URL`):

```
https://<frontend-domain>/api/v1/auth/oauth/google/callback
https://<frontend-domain>/api/v1/auth/oauth/github/callback
```

For local development add `http://localhost:3000/api/v1/auth/oauth/google/callback`
to the Google client, and create a second GitHub OAuth App for
`http://localhost:3000/api/v1/auth/oauth/github/callback` (a GitHub OAuth App
has exactly one callback URL).

### Google
1. [Google Cloud Console](https://console.cloud.google.com/) -> create or select a project.
2. **Google Auth Platform -> Branding**: app name, support email, developer contact.
3. **Audience**: *External*. While the app is in *Testing*, only listed test users
   can sign in; click **Publish app** for everyone. `openid`, `email` and
   `profile` are non-sensitive scopes, so no Google verification is needed.
4. **Clients -> Create client -> Web application**. Add the Google redirect URI
   above under *Authorized redirect URIs* (and the frontend origin under
   *Authorized JavaScript origins*).
5. Copy the **Client ID** and **Client secret** into the backend's
   `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

### GitHub
1. GitHub -> **Settings -> Developer settings -> OAuth Apps -> New OAuth App**
   (or the same under an organization).
2. *Homepage URL*: `https://<frontend-domain>`; *Authorization callback URL*:
   the GitHub redirect URI above.
3. **Register application -> Generate a new client secret**.
4. Copy the **Client ID** and secret into `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`.

Buttons appear on the login and register pages as soon as a provider's ID and
secret are both set (`GET /api/v1/auth/oauth/providers`); no frontend rebuild is needed.

### Account rules
- A Google/GitHub identity is matched on the provider's user ID first, so changing
  the email at the provider does not create a second account.
- Otherwise the provider must report the email as **verified**; GitHub's primary
  verified address is used. Unverified emails are refused.
- A verified provider email that matches an existing QRit account signs into
  that account. If that account had never verified its email, its password is
  discarded and its sessions revoked first (someone may have registered the
  address without owning it).
- Accounts created through Google/GitHub have no password; **Profile -> Set a
  password** adds one. Users can connect or disconnect providers there, and the
  last sign-in method cannot be removed.

### Troubleshooting
| Error on `/auth/callback` | Meaning |
| :--- | :--- |
| `redirect_uri_mismatch` at Google / "The redirect_uri is not associated" at GitHub | The registered redirect URI differs from `{APP_BASE_URL}/api/v1/auth/oauth/<provider>/callback` (scheme, domain and path must match exactly) |
| "Your sign-in session expired or was started in another browser tab" | The state cookie was missing: the flow took over 10 minutes, cookies are blocked, or the start and callback hosts differ (check `APP_BASE_URL` / `OAUTH_CALLBACK_BASE_URL`) |
| "Sign-in with Google isn't set up on this server yet" | `GOOGLE_CLIENT_ID` or `GOOGLE_CLIENT_SECRET` is missing on the backend |
| "...couldn't complete the sign-in" | The code exchange failed: usually a wrong client secret; the backend log line `OAuth <provider> callback failed` has the provider's error |
| "The API is unreachable right now" (502 from `/api/v1`) | `BACKEND_INTERNAL_URL` on the frontend is wrong, or the backend is down |

