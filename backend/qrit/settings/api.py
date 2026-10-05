"""Settings for the control-plane API service."""

from .base import *  # noqa: F403
from .env import env

ROOT_URLCONF = "qrit.urls.api"

# Reverse proxy / Railway header configuration
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")

if env.APP_ENV in ("staging", "production"):
    SECURE_HSTS_SECONDS = 31536000
    SECURE_HSTS_INCLUDE_SUBDOMAINS = True
    SECURE_HSTS_PRELOAD = True
    SESSION_COOKIE_SECURE = True
    CSRF_COOKIE_SECURE = True
else:
    SESSION_COOKIE_SECURE = env.COOKIE_SECURE
    CSRF_COOKIE_SECURE = env.COOKIE_SECURE

CSRF_COOKIE_NAME = "qrit_csrf"
CSRF_HEADER_NAME = "HTTP_X_CSRF_TOKEN"
CSRF_COOKIE_HTTPONLY = False  # Readable by frontend client for double-submit
CSRF_TRUSTED_ORIGINS = env.csrf_trusted_origins_list

CORS_ALLOWED_ORIGINS = env.cors_allowed_origins_list
CORS_ALLOWED_ORIGIN_REGEXES = [
    r"^https:\/\/.*\.up\.railway\.app$",
    r"^https:\/\/.*\.railway\.app$",
]
CORS_ALLOW_CREDENTIALS = True
