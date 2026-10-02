.PHONY: all dev check test lint types fmt k6-smoke fe-check

all: check

dev:
	docker compose up

fmt:
	cd backend && uv run ruff format . && uv run ruff check --fix .

lint:
	cd backend && uv run ruff check . && uv run ruff format --check .

types:
	cd backend && uv run mypy

test:
	cd backend && uv run pytest -n auto

check:
	cd backend && uv lock --check
	cd backend && uv run ruff check . && uv run ruff format --check .
	cd backend && uv run mypy
	cd backend && uv run python manage.py makemigrations --check --dry-run
	cd backend && uv run pytest -n auto
	cd backend && DJANGO_SETTINGS_MODULE=qrit.settings.api APP_ENV=production DJANGO_SECRET_KEY=production-secret-key-at-least-50-characters-long-safe-and-random APP_ENCRYPTION_KEY=ZGV2ZWxvcG1lbnQtZW5jcnlwdGlvbi1rZXktMzJieXRlcw== JWT_ED25519_PRIVATE_KEY=MC4CAQAwBQYDK2VwBCIEIHsV70EaTfZ+kS1xM6g6eM+VpD7kP8E3jK1qR9sT2uX/ SCAN_SALT_SECRET=production-scan-salt-secret-32-bytes-minimum EDGE_SHARED_SECRET=production-edge-shared-secret-32-chars-min SERIAL_MAC_KEY=production-serial-mac-key-32-bytes-min VERIFY_TOKEN_KEY=production-verify-token-key-32-bytes-min uv run python manage.py check --deploy --fail-level WARNING

fe-check:
	cd frontend && pnpm lint && pnpm typecheck && pnpm build

k6-smoke:
	k6 run deploy/k6/redirect_load.js
