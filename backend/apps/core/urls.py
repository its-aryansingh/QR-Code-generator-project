"""Core URL configuration."""

from django.urls import path

from .views.health import health_alias_view, healthz_view, readyz_view

urlpatterns = [
    path("healthz", healthz_view, name="healthz"),
    path("readyz", readyz_view, name="readyz"),
    path("health", health_alias_view, name="health"),
]
