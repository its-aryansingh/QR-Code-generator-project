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
| `EMAIL_BACKEND` | Optional | `console` for dev, `resend` for production | `resend` |
| `RESEND_API_KEY` | If Resend | Resend API key for transactional emails | `re_...` |
| `EMAIL_FROM` | If Resend | Verified sender email | `noreply@yourdomain.com` |
| `GOOGLE_CLIENT_ID` | Optional | Google OAuth Web Client ID | `...apps.googleusercontent.com` |
| `STRIPE_SECRET_KEY` | Optional | Stripe Live Secret Key | `sk_live_...` |
| `STRIPE_WEBHOOK_SECRET`| Optional | Stripe Webhook Signing Secret | `whsec_...` |
| `LOG_LEVEL` | Optional | Console logging level | `INFO` |

### Frontend (`frontend/.env.local` or platform build envs)

| Variable | Required in Prod? | Description |
| :--- | :--- | :--- |
| `NEXT_PUBLIC_API_URL` | **Yes** | Full backend API URL (e.g., `https://api.yourdomain.com/api/v1`) |
| `NEXT_PUBLIC_API_BASE_URL` | **Yes** | Backend root URL (e.g., `https://api.yourdomain.com`) |
| `NEXT_PUBLIC_APP_URL` | **Yes** | Frontend public URL (e.g., `https://app.yourdomain.com`) |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | Optional | Google OAuth Client ID for the Sign-In button |

> [!NOTE]
> In Next.js, `NEXT_PUBLIC_*` variables are baked into client JavaScript bundles at **build time**. Make sure they are set in your CI/CD or platform build environment *before* `npm run build` runs.

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
web: cd backend && gunicorn qrapp.wsgi:application --config gunicorn.conf.py --bind 0.0.0.0:$PORT
```

- In DigitalOcean or Heroku, the `release` phase runs migrations and gathers static files before routing traffic to the newly deployed container, ensuring zero-downtime releases.

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
