"""API URL Configuration for the control-plane API."""

from django.urls import include, path
from drf_spectacular.views import SpectacularAPIView, SpectacularJSONAPIView

urlpatterns = [
    # Health probes at root
    path("", include("apps.core.urls")),
    # OpenAPI schemas
    path("v1/openapi.json", SpectacularJSONAPIView.as_view(), name="openapi-json"),
    path("v1/openapi.yaml", SpectacularAPIView.as_view(), name="openapi-yaml"),
    # Accounts & Auth routes under both /v1/ and /api/v1/ (for frontend compatibility)
    path("v1/", include("apps.accounts.urls")),
    path("api/v1/", include("apps.accounts.urls")),
]
