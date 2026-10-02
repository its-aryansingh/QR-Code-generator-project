import logging
from django.db import connection
from django.http import JsonResponse
from django.urls import path

logger = logging.getLogger(__name__)


def health(request):
    """Production health check with database verification.

    Supports `?shallow=1` to bypass database verification for high-frequency
    load balancer liveness probes.
    """
    is_shallow = request.GET.get("shallow") in ("1", "true", "yes")

    if is_shallow:
        return JsonResponse({"status": "ok", "service": "qrit-django", "type": "shallow"})

    db_status = "connected"
    try:
        connection.ensure_connection()
    except Exception as exc:
        logger.error("Database health check failed: %s", exc)
        return JsonResponse(
            {
                "status": "unhealthy",
                "service": "qrit-django",
                "database": "disconnected",
                "error": "Database unavailable",
            },
            status=503,
        )

    return JsonResponse({"status": "ok", "service": "qrit-django", "database": db_status})


urlpatterns = [
    path("", health),
]
