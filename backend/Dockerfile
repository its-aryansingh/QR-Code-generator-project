# syntax=docker/dockerfile:1.7
FROM python:3.14-slim-bookworm AS build
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_PYTHON_DOWNLOADS=never
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --no-install-project
COPY . .
RUN uv sync --locked --no-dev \
 && DJANGO_SETTINGS_MODULE=qrit.settings.build /app/.venv/bin/python manage.py collectstatic --noinput --verbosity 0

FROM python:3.14-slim-bookworm
# xmlsec1: pysaml2. libpq5: psycopg. postgresql-client-17 (PGDG): pg_dump/psql for ops and the v1 import rehearsal.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg xmlsec1 libpq5 \
 && install -d /usr/share/postgresql-common/pgdg \
 && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
 && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
 && apt-get update && apt-get install -y --no-install-recommends postgresql-client-17 \
 && apt-get purge -y curl gnupg && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 10001 app
COPY --from=build --chown=app:app /app /app
ENV PATH="/app/.venv/bin:$PATH" PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1
WORKDIR /app
USER app
# Default = api. Other services override the start command.
CMD ["sh", "-c", "gunicorn qrit.asgi:application -k uvicorn_worker.UvicornWorker -c gunicorn.conf.py"]
