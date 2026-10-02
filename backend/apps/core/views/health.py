"""Health and readiness check views.

Plan §3.2 & §7.1:
- /healthz: process liveness (200 OK)
- /readyz: database and Redis readiness probe (200 when ready, 503 when unavailable)
- /health: alias for readiness probe (Railway compatibility)
"""

from typing import Any

import redis
from django.db import connection
from django.http import JsonResponse

from qrit.settings.env import env


def check_database() -> tuple[bool, str | None]:
    """Check database connectivity and query execution."""
    try:
        connection.ensure_connection()
        with connection.cursor() as cursor:
            cursor.execute("SELECT 1")
            cursor.fetchone()
        return True, None
    except Exception as e:
        return False, str(e)


def check_redis() -> tuple[bool, str | None]:
    """Check Redis connectivity via ping."""
    client = None
    try:
        client = redis.from_url(env.REDIS_URL, socket_timeout=2)
        if client.ping():
            return True, None
        return False, "Redis ping returned false"
    except Exception as e:
        return False, str(e)
    finally:
        if client is not None:
            try:
                client.close()
            except Exception:
                pass


def healthz_view(request: Any) -> JsonResponse:
    """Liveness probe: verifies process is running."""
    return JsonResponse({"status": "ok"})


def readyz_view(request: Any) -> JsonResponse:
    """Readiness probe: verifies database and cache connectivity."""
    db_ok, db_err = check_database()
    redis_ok, redis_err = check_redis()

    if db_ok and redis_ok:
        return JsonResponse(
            {
                "status": "ready",
                "database": "ok",
                "redis": "ok",
            }
        )

    return JsonResponse(
        {
            "status": "unavailable",
            "database": "ok" if db_ok else (db_err or "error"),
            "redis": "ok" if redis_ok else (redis_err or "error"),
        },
        status=503,
    )


# /health is an alias for /readyz (v1 and Railway compatibility)
health_alias_view = readyz_view
