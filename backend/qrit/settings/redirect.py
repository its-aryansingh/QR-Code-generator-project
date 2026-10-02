"""Settings for the async redirect hot-path service."""

from .base import *  # noqa: F403
from .env import env

ROOT_URLCONF = "qrit.urls.redirect"

# Minimal apps: no auth, sessions, messages, or admin
INSTALLED_APPS = [
    "django.contrib.staticfiles",
    "apps.core.apps.CoreConfig",
]

# Minimal middleware on hot path: no Session, CSRF, Auth, Message middleware
MIDDLEWARE = [
    "django.middleware.security.SecurityMiddleware",
]

SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")

if env.APP_ENV in ("staging", "production"):
    SECURE_HSTS_SECONDS = 31536000
    SECURE_HSTS_INCLUDE_SUBDOMAINS = True
    SECURE_HSTS_PRELOAD = True
