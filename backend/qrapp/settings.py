import os
from pathlib import Path
from dotenv import load_dotenv

load_dotenv()

BASE_DIR = Path(__file__).resolve().parent.parent


def _csv_env(name, default=""):
    raw = os.environ.get(name, default)
    return [item.strip() for item in raw.split(",") if item.strip()]


SECRET_KEY = os.environ.get("DJANGO_SECRET_KEY", "django-insecure-change-in-production")
DEBUG = os.environ.get("DEBUG", "False") == "True"
ALLOWED_HOSTS = _csv_env("ALLOWED_HOSTS", "*") or ["*"]

INSTALLED_APPS = [
    "django.contrib.staticfiles",
    "django.contrib.contenttypes",
    "django.contrib.auth",
    "corsheaders",
    "rest_framework",
    "api",
]

MIDDLEWARE = [
    "django.middleware.security.SecurityMiddleware",
    "corsheaders.middleware.CorsMiddleware",
    "whitenoise.middleware.WhiteNoiseMiddleware",
    "django.middleware.common.CommonMiddleware",
    "django.middleware.clickjacking.XFrameOptionsMiddleware",
]

X_FRAME_OPTIONS = "DENY"
SECURE_CONTENT_TYPE_NOSNIFF = True
SECURE_REFERRER_POLICY = "strict-origin-when-cross-origin"
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")
if not DEBUG:
    SECURE_SSL_REDIRECT = os.environ.get("SECURE_SSL_REDIRECT", "True") == "True"
    SECURE_HSTS_SECONDS = int(os.environ.get("SECURE_HSTS_SECONDS", 31536000))
    SECURE_HSTS_INCLUDE_SUBDOMAINS = True
    SECURE_HSTS_PRELOAD = True
    SESSION_COOKIE_SECURE = True
    CSRF_COOKIE_SECURE = True

ROOT_URLCONF = "qrapp.urls"
WSGI_APPLICATION = "qrapp.wsgi.application"

# ---------------------------------------------------------------- CORS
# The dashboard is a separate origin from the API, so every authenticated
# request is a preflighted cross-origin request. Without this the entire
# enterprise dashboard fails before it sends a single byte.
CORS_ALLOWED_ORIGINS = _csv_env(
    "CORS_ALLOWED_ORIGINS",
    "http://localhost:3000,http://127.0.0.1:3000,http://localhost:3001,http://127.0.0.1:3001",
)
CORS_ALLOWED_ORIGIN_REGEXES = _csv_env("CORS_ALLOWED_ORIGIN_REGEXES")
CORS_ALLOW_ALL_ORIGINS = os.environ.get("CORS_ALLOW_ALL_ORIGINS", "False") == "True"
CORS_ALLOW_CREDENTIALS = True
CORS_ALLOW_HEADERS = [
    "accept", "accept-encoding", "authorization", "content-type", "dnt",
    "origin", "user-agent", "x-csrftoken", "x-requested-with",
    "x-api-key", "x-workspace-id",
]
CORS_EXPOSE_HEADERS = ["content-disposition", "x-ratelimit-limit", "x-ratelimit-remaining"]
CSRF_TRUSTED_ORIGINS = _csv_env("CSRF_TRUSTED_ORIGINS") or [
    o for o in CORS_ALLOWED_ORIGINS if o.startswith("http")
]

# ---------------------------------------------------------------- Database
import dj_database_url

DATABASES = {
    "default": dj_database_url.config(
        default=f"sqlite:///{BASE_DIR / 'db.sqlite3'}",
        conn_max_age=int(os.environ.get("DB_CONN_MAX_AGE", 600)),
        ssl_require=os.environ.get("DB_SSL_REQUIRE", "False") == "True",
    )
}

# ---------------------------------------------------------------- Auth
JWT_SECRET = os.environ.get("JWT_SECRET", "your-super-secret-key-change-in-production")
if not DEBUG and JWT_SECRET == "your-super-secret-key-change-in-production":
    raise RuntimeError(
        "FATAL: JWT_SECRET is set to the insecure default. "
        "Set a strong, unique JWT_SECRET environment variable before running in production."
    )
JWT_EXPIRY_MINUTES = int(os.environ.get("JWT_EXPIRY_MINUTES", 60))
REFRESH_EXPIRY_DAYS = int(os.environ.get("REFRESH_EXPIRY_DAYS", 30))

# Auth security
AUTH_MAX_LOGIN_ATTEMPTS = int(os.environ.get("AUTH_MAX_LOGIN_ATTEMPTS", 5))
AUTH_LOCKOUT_DURATION_MINUTES = int(os.environ.get("AUTH_LOCKOUT_DURATION_MINUTES", 15))
AUTH_RESET_TOKEN_EXPIRY_HOURS = int(os.environ.get("AUTH_RESET_TOKEN_EXPIRY_HOURS", 1))
AUTH_VERIFICATION_TOKEN_EXPIRY_HOURS = int(os.environ.get("AUTH_VERIFICATION_TOKEN_EXPIRY_HOURS", 24))
AUTH_MAX_REGISTER_PER_IP_PER_HOUR = int(os.environ.get("AUTH_MAX_REGISTER_PER_IP_PER_HOUR", 5))
AUTH_MAX_LOGIN_PER_IP_PER_15MIN = int(os.environ.get("AUTH_MAX_LOGIN_PER_IP_PER_15MIN", 10))

# ---------------------------------------------------------------- Stripe
STRIPE_SECRET_KEY = os.environ.get("STRIPE_SECRET_KEY", "")
STRIPE_WEBHOOK_SECRET = os.environ.get("STRIPE_WEBHOOK_SECRET", "")
STRIPE_PRICE_STARTER = os.environ.get("STRIPE_PRICE_STARTER", "")
STRIPE_PRICE_PRO = os.environ.get("STRIPE_PRICE_PRO", "")
STRIPE_PRICE_ENTERPRISE = os.environ.get("STRIPE_PRICE_ENTERPRISE", "")

# ---------------------------------------------------------------- App URLs
APP_BASE_URL = os.environ.get("APP_BASE_URL", "http://localhost:3000")
API_BASE_URL = os.environ.get("API_BASE_URL", "http://localhost:8084")
SHORT_LINK_BASE_URL = os.environ.get("SHORT_LINK_BASE_URL", API_BASE_URL)

# ---------------------------------------------------------------- Google OAuth
GOOGLE_CLIENT_ID = os.environ.get("GOOGLE_CLIENT_ID", "")

# ---------------------------------------------------------------- Email
EMAIL_BACKEND = os.environ.get("EMAIL_BACKEND", "console")  # 'console' or 'resend'
RESEND_API_KEY = os.environ.get("RESEND_API_KEY", "")
EMAIL_FROM = os.environ.get("EMAIL_FROM", "noreply@qrit.app")

FREE_TIER_DAILY_LIMIT = int(os.environ.get("FREE_TIER_DAILY_LIMIT", 10))

PLAN_API_LIMITS = {
    "free": 50,
    "starter": 500,
    "pro": 1000,
    "enterprise": 10000,
}

# Plan entitlements drive every enterprise gate in the API and are echoed to
# the dashboard so the UI can disable rather than fail.
PLAN_ENTITLEMENTS = {
    "free": {
        "max_workspaces": 1, "max_members": 1, "max_qr_codes": 50,
        "max_folders": 5, "max_campaigns": 1, "max_templates": 0,
        "max_api_keys": 1, "max_custom_domains": 0, "bulk_batch_size": 25,
        "analytics_retention_days": 30,
        "features": [],
    },
    "starter": {
        "max_workspaces": 2, "max_members": 3, "max_qr_codes": 500,
        "max_folders": 25, "max_campaigns": 10, "max_templates": 3,
        "max_api_keys": 2, "max_custom_domains": 0, "bulk_batch_size": 250,
        "analytics_retention_days": 90,
        "features": ["bulk", "webhooks", "campaigns"],
    },
    "pro": {
        "max_workspaces": 10, "max_members": 25, "max_qr_codes": 10000,
        "max_folders": 200, "max_campaigns": 100, "max_templates": 25,
        "max_api_keys": 10, "max_custom_domains": 1, "bulk_batch_size": 2000,
        "analytics_retention_days": 365,
        "features": [
            "bulk", "webhooks", "campaigns", "templates", "routing",
            "audit_log", "white_label", "custom_domain", "gs1", "exports",
        ],
    },
    "enterprise": {
        "max_workspaces": 100, "max_members": 1000, "max_qr_codes": 1000000,
        "max_folders": 5000, "max_campaigns": 5000, "max_templates": 500,
        "max_api_keys": 50, "max_custom_domains": 25, "bulk_batch_size": 10000,
        "analytics_retention_days": 1095,
        "features": [
            "bulk", "webhooks", "campaigns", "templates", "routing",
            "audit_log", "white_label", "custom_domain", "gs1", "exports",
            "sso", "scim", "security_policy", "priority_support", "sla",
        ],
    },
}

# ---------------------------------------------------------------- DRF
REST_FRAMEWORK = {
    "DEFAULT_AUTHENTICATION_CLASSES": [],
    "DEFAULT_PERMISSION_CLASSES": [],
    "DEFAULT_RENDERER_CLASSES": ["rest_framework.renderers.JSONRenderer"],
    "UNAUTHENTICATED_USER": None,
}

STATIC_URL = "/static/"
STATIC_ROOT = BASE_DIR / "staticfiles"
STORAGES = {
    "default": {"BACKEND": "django.core.files.storage.FileSystemStorage"},
    "staticfiles": {"BACKEND": "whitenoise.storage.CompressedManifestStaticFilesStorage"},
}

LANGUAGE_CODE = "en-us"
TIME_ZONE = "UTC"
USE_I18N = False
USE_TZ = True
DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"

LOGGING = {
    "version": 1,
    "disable_existing_loggers": False,
    "formatters": {
        "standard": {
            "format": "%(asctime)s [%(levelname)s] %(name)s: %(message)s",
            "datefmt": "%Y-%m-%d %H:%M:%S",
        },
    },
    "handlers": {
        "console": {
            "class": "logging.StreamHandler",
            "formatter": "standard",
        },
    },
    "root": {
        "handlers": ["console"],
        "level": os.environ.get("LOG_LEVEL", "INFO"),
    },
    "loggers": {
        "django.request": {
            "handlers": ["console"],
            "level": "ERROR",
            "propagate": False,
        },
    },
}
