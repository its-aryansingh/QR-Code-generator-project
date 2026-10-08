"""Redirect service URL Configuration for the async redirect hot path."""

from django.urls import include, path

from apps.qr.redirect import scan

urlpatterns = [
    # Health probes at root
    path("", include("apps.core.urls")),
    path("r/<str:code>", scan, name="qr-scan"),
]
