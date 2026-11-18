# QRit

A QR code platform: dynamic QR codes, scan analytics, campaigns, lead capture, teams and workspaces.

| Folder | What it is |
|---|---|
| `backend/` | Django 6.1 API on Python 3.13+, managed with [uv](https://docs.astral.sh/uv/). PostgreSQL, Redis, procrastinate workers. |
| `frontend/` | Next.js 16 dashboard and marketing site (React 19, Tailwind, light/dark themes). |

## Run locally

### Backend with Docker

```bash
docker compose up -d --build
docker compose run --rm api python manage.py migrate
```

This starts PostgreSQL, Redis, Mailpit (mail UI on http://localhost:8025) and MinIO, plus:
- the API on http://localhost:8080
- the short-link redirect service on http://localhost:8090
- the background worker

### Backend without Docker

You need PostgreSQL 17 or later and, optionally, Redis.

```bash
cd backend
uv sync
cp ../.env.example .env     # then point DATABASE_URL at your database
uv run python manage.py migrate
uv run python manage.py runserver 8080
```

### Frontend

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

The app runs on http://localhost:3000. The browser calls `/api/v1` on the app's own origin, and the Next.js server forwards those calls to `BACKEND_INTERNAL_URL` (default `http://127.0.0.1:8080`). Leave `NEXT_PUBLIC_API_URL` unset. If you point it somewhere else, the dashboard bypasses the proxy and calls that server directly.

## Checks

CI (`.github/workflows/ci.yml`) runs these backend checks on every pull request:

```bash
cd backend
uv run ruff check . && uv run ruff format --check .
uv run mypy
uv run python manage.py makemigrations --check --dry-run
uv run pytest -n auto      # needs DATABASE_URL pointing at PostgreSQL
```

Before opening a pull request that touches the frontend, run `npm run typecheck` and `npm run build`.

## Deploy

Railway runs two services from this repository:
- **Backend:** root directory `backend`, built from `backend/Dockerfile`.
- **Frontend:** root directory `frontend`, built from `frontend/Dockerfile`. Set `BACKEND_INTERNAL_URL` to the backend's private address.

`.env.example` (backend) and `frontend/.env.example` list every setting. Production refuses the placeholder secrets.

## License

MIT
