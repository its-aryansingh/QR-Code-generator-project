"""API URL Configuration for the control-plane API."""

from django.http import HttpRequest, JsonResponse
from django.urls import include, path, re_path
from drf_spectacular.views import SpectacularAPIView, SpectacularJSONAPIView

from apps.qr.redirect import scan


def api_not_found(request: HttpRequest, rest: str = "") -> JsonResponse:
    """Unknown API paths answer JSON, never Django's HTML 404 page.

    The dashboard parses every API response as JSON; an HTML page here is what
    crashed the Leads and Brand pages when a route was missing.
    """
    response = JsonResponse(
        {
            "type": "urn:qrit:error:not_found",
            "title": "Not Found",
            "status": 404,
            "detail": f"no API endpoint at {request.path}",
            "code": "not_found",
            "instance": request.path,
            "success": False,
            "error": "This API endpoint doesn't exist.",
        },
        status=404,
    )
    response["Content-Type"] = "application/problem+json"
    response["Cache-Control"] = "private, no-store"
    return response


urlpatterns = [
    # Health probes at root
    path("", include("apps.core.urls")),
    # OpenAPI schemas
    path("v1/openapi.json", SpectacularJSONAPIView.as_view(), name="openapi-json"),
    path("v1/openapi.yaml", SpectacularAPIView.as_view(), name="openapi-yaml"),
    # Accounts & Auth routes under both /v1/ and /api/v1/ (for frontend compatibility)
    path("v1/", include("apps.accounts.urls")),
    path("api/v1/", include("apps.accounts.urls")),
    # Dashboard: workspaces, QR codes, campaigns, team, analytics, audit, webhooks, leads, branding
    path("v1/", include("apps.workspaces.urls")),
    path("api/v1/", include("apps.workspaces.urls")),
    # Scans of dynamic QR codes (the web app forwards /r/<code> here)
    path("r/<str:code>", scan, name="qr-scan"),
    re_path(r"^(?:api/)?v1/(?P<rest>.*)$", api_not_found, name="api-not-found"),
]
