"""Settings module for Docker build time (collectstatic only, no DB or secrets required)."""

from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent.parent.parent

SECRET_KEY = "build-dummy-secret-key-strictly-for-collectstatic"
DEBUG = False
ALLOWED_HOSTS = ["*"]

INSTALLED_APPS = [
    "django.contrib.staticfiles",
    "apps.core.apps.CoreConfig",
]

STATIC_URL = "/static/"
STATIC_ROOT = BASE_DIR / "staticfiles"

STORAGES = {
    "default": {
        "BACKEND": "django.core.files.storage.FileSystemStorage",
    },
    "staticfiles": {
        "BACKEND": "whitenoise.storage.CompressedManifestStaticFilesStorage",
    },
}

DATABASES: dict[str, Any] = {}
