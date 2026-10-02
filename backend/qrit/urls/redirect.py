"""Redirect service URL Configuration for the async redirect hot path."""

from django.urls import include, path

urlpatterns = [
    # Health probes at root
    path("", include("apps.core.urls")),
]
