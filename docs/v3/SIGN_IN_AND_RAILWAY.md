# Sign-in on Railway: v3 API + current web app

This is the setup on `main` today: the web app in `frontend/` (the v1 UI) talks
to the v3 API in `backend/`. It covers what each Railway service needs so that
email/password, Google and GitHub sign-in work end to end.

## How the pieces connect

```
browser ──https──▶ web (frontend/, Next.js)
                     │  /api/v1/*  (route handler, forwarded at runtime)
                     ▼
                   api (backend/, Django)  ◀──▶ Google / GitHub
                     │
                     ├── Postgres
                     └── Redis
```

- The browser only ever calls its own site. `frontend/src/app/api/v1/[...path]`
  forwards each `/api/v1/*` request to `BACKEND_INTERNAL_URL`, which is read **at
  runtime**. Nothing about the API's address is baked into the build.
- Cookies the API sets (sessions, the sign-in state cookie) are therefore
  first-party on the web domain. No CORS is involved.

## Google and GitHub sign-in flow

1. The button sends the browser to `/api/v1/auth/<provider>/start?next=/dashboard`.
2. The API stores a random `state`, a PKCE verifier and (Google) a nonce in a
   signed, HttpOnly, SameSite=Lax cookie (`qrit_oauth`, path `/api/v1/auth/`,
   10 minutes) and redirects to the provider.
3. The provider redirects to **`https://<web-domain>/callback/<provider>`**.
4. That page POSTs `{code, state}` to `/api/v1/auth/<provider>`. The API checks
   `state` against this browser's cookie before exchanging the code. A callback
   link carrying someone else's code is refused in any other browser, which
   prevents login CSRF. It then exchanges the code using the verifier and the
   client secret and verifies the identity:
   - Google: the ID token must have this client as `aud`, a Google issuer,
     `email_verified`, and the nonce.
   - GitHub: only a verified address from `/user/emails` is used.
5. The response carries the session tokens and `next`; the page signs in and
   navigates there.

Accounts are matched by the provider's user id first, then by a verified
email. If an existing account's email was never verified, its password is
dropped and its sessions are revoked before linking.

`GET /api/v1/auth/oauth/providers` tells the web app which buttons to show, so
no client ID is needed at build time.

## Provider consoles

Redirect URIs (use the **web** service's public domain):

```
https://<web-domain>/callback/google
https://<web-domain>/callback/github
```

**Google:**
1. Go to Google Cloud Console → Google Auth Platform → Clients → Create client → *Web application*.
2. Add the Google redirect URI above.
3. Publish the app under Audience. While it is in *Testing*, only listed test users can sign in.

**GitHub:**
1. Go to Settings → Developer settings → OAuth Apps → New OAuth App.
2. Set *Homepage URL* to `https://<web-domain>` and *Authorization callback URL* to the GitHub redirect URI above.
3. Generate a client secret.

A GitHub OAuth App has one callback URL, so create a second app for local development (`http://localhost:3000/callback/github`).

## Railway variables

### api service (`backend/`)

| Variable | Value |
| --- | --- |
| `APP_ENV` | `production` (required: it also turns `DEBUG` off) |
| `DJANGO_SECRET_KEY` | 50+ random characters |
| `JWT_ED25519_PRIVATE_KEY` | base64 PKCS#8 Ed25519 key, the **same** for every replica (see below) |
| `APP_ENCRYPTION_KEY`, `SCAN_SALT_SECRET`, `EDGE_SHARED_SECRET`, `SERIAL_MAC_KEY`, `VERIFY_TOKEN_KEY` | random secrets (startup refuses dev values) |
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` |
| `REDIS_URL` | `${{Redis.REDIS_URL}}` (login and sign-up rate limits) |
| `APP_BASE_URL` | `https://<web-domain>`: the redirect URIs are built from it |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | from Google |
| `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | from GitHub |
| `PORT` | `8080` (pinned so the web service can reference it) |

Generate the signing key once and store it as the variable:

```
python -c "from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey as K; from cryptography.hazmat.primitives import serialization as s; import base64; print(base64.b64encode(K.generate().private_bytes(s.Encoding.DER, s.PrivateFormat.PKCS8, s.NoEncryption())).decode())"
```

Settings for the service:
- **Pre-deploy command:** `python manage.py migrate --noinput`.
- **Health check path:** `/healthz`.

Railway's own hosts (`healthcheck.railway.app`, `RAILWAY_PUBLIC_DOMAIN`, `RAILWAY_PRIVATE_DOMAIN`) are always allowed, so `ALLOWED_HOSTS` only needs your custom API domain, if any.

### web service (`frontend/`)

| Variable | Value |
| --- | --- |
| `BACKEND_INTERNAL_URL` | `http://${{api.RAILWAY_PRIVATE_DOMAIN}}:${{api.PORT}}` |
| `NEXT_PUBLIC_API_URL` | **delete it** if it exists |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID`, `NEXT_PUBLIC_GITHUB_CLIENT_ID` | no longer used; delete them |

If private networking is unavailable, `BACKEND_INTERNAL_URL` can be the api's public `https://` domain.

## What was breaking sign-in

| Symptom | Cause | Fixed in |
| --- | --- | --- |
| Anyone could sign in as any user | Fake `test-google:`/`test-github:` credentials were accepted in production: `APP_ENV` was never a Django setting, so it always read `local` | #2 |
| Random 401 "Invalid access token"; everyone signed out on each deploy | `JWT_ED25519_PRIVATE_KEY` was never a Django setting, so each worker made its own signing key | #2 |
| Google token for another app accepted; GitHub unverified email accepted | No `aud`/`email_verified` checks; GitHub fell back to an unverified address | #2 |
| Login succeeded but the UI said "Login failed" | The UI expected v1's `{success, data}`; v3 returns bare JSON and RFC 7807 errors | this PR (adapter in `frontend/src/lib/api.ts` and `enterprise.ts`) |
| Signed out after 10 minutes | The dashboard's refresh only accepted v1's response shape | this PR |
| API unreachable from the web service | `NEXT_PUBLIC_API_URL` baked at build time (defaulted to localhost); `*.railway.internal` rejected by `ALLOWED_HOSTS` | this PR |
| GitHub login CSRF; Google One Tap often did nothing | Client-only `state` check that could be skipped; `prompt()` suppressed by browsers | this PR (server-checked state + PKCE, redirect flow for both) |
| OAuth-only users shown as having a password | `has_password` was `bool(password)`, which is true for Django's unusable-password marker | this PR |

## Known gap

v3 has not yet ported v1's dashboard endpoints (`/workspaces`, QR codes,
analytics…). After signing in, the v1 dashboard pages show "Request failed
(404)" until those phases land or `apps/web` replaces `frontend/` (plan §7.24).
Sign-in, sign-out, session refresh and the profile in the sidebar work.
