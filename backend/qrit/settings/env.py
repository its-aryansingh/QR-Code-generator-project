"""Typed environment configuration using pydantic-settings.

Appendix A of QRit v3 Plan defines the authoritative names and defaults.
Never read os.environ elsewhere in the codebase.
"""

from typing import Literal

from pydantic import model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # Core environment
    APP_ENV: Literal["local", "test", "staging", "production"] = "local"
    # Set by Railway on every deploy; implies APP_ENV when APP_ENV is unset.
    RAILWAY_ENVIRONMENT_NAME: str = ""
    DJANGO_SETTINGS_MODULE: str = "qrit.settings.api"
    DJANGO_SECRET_KEY: str = "local-insecure-secret-key-change-in-production-min-50-characters-long"
    ALLOWED_HOSTS: str = "localhost,127.0.0.1,healthcheck.railway.app,.railway.app,.up.railway.app"
    # Injected by Railway into every service.
    RAILWAY_PRIVATE_DOMAIN: str = ""
    RAILWAY_PUBLIC_DOMAIN: str = ""

    # Databases
    DATABASE_URL: str = "postgresql://qrit_app:qrit_dev_password@localhost:5432/qrit"
    DATABASE_UNPOOLED_URL: str = ""
    DATABASE_OWNER_URL: str = ""
    DB_POOL_MIN: int = 0
    DB_POOL_MAX: int = 4
    DB_APP_PASSWORD: str = ""
    V1_DATABASE_URL: str = ""
    DISABLE_SERVER_SIDE_CURSORS: bool = False

    # Redis
    REDIS_URL: str = "redis://localhost:6379/0"

    # Every plan gets every feature and the top limits while true. Set to
    # false to enforce the plan table in apps/workspaces/entitlements.py.
    ALL_FEATURES_UNLOCKED: bool = True

    # Hosts and domains
    APP_BASE_URL: str = "http://localhost:3000"
    API_PUBLIC_URL: str = "http://localhost:8080"
    API_INTERNAL_URL: str = "http://localhost:8080"
    LEGACY_BACKEND_HOST: str = "localhost:8080"
    SHORT_DOMAIN: str = "localhost:8090"
    SHORT_DOMAIN_SCHEME: str = "http"
    SANDBOX_SHORT_DOMAIN: str = "sandbox.localhost:8090"
    LEGACY_HOSTS: str = ""

    # Edge and trust
    EDGE_SHARED_SECRET: str = "local-edge-secret-at-least-32-chars-long"
    EDGE_SHARED_SECRET_PREVIOUS: str = ""

    # Auth & Tokens
    JWT_ED25519_PRIVATE_KEY: str = (
        # 32-byte dummy Ed25519 seed in base64 for local dev
        "MC4CAQAwBQYDK2VwBCIEIHsV70EaTfZ+kS1xM6g6eM+VpD7kP8E3jK1qR9sT2uX/"
    )
    JWT_KEY_ID: str = "dev-key-1"
    JWT_PREVIOUS_PUBLIC_KEYS: str = "[]"
    APP_ENCRYPTION_KEY: str = "ZGV2ZWxvcG1lbnQtZW5jcnlwdGlvbi1rZXktMzJieXRlcw=="
    SCAN_SALT_SECRET: str = "local-scan-salt-secret-32-bytes-minimum"
    COOKIE_SECURE: bool = False
    COOKIE_DOMAIN: str = ""
    CORS_ALLOWED_ORIGINS: str = ""
    AUTH_MAX_LOGIN_ATTEMPTS: int = 5

    # Storage (S3-compatible)
    STORAGE_ENDPOINT: str = "http://localhost:9000"
    STORAGE_REGION: str = "auto"
    STORAGE_BUCKET: str = "qrit-dev"
    STORAGE_ACCESS_KEY_ID: str = "minioadmin"
    STORAGE_SECRET_ACCESS_KEY: str = "minioadminpassword"
    STORAGE_PUBLIC_BASE_URL: str = "http://localhost:9000/qrit-dev"
    AUDIT_ANCHOR_BUCKET: str = ""

    # External APIs
    RESEND_API_KEY: str = ""
    EMAIL_FROM: str = "QRit <notifications@qrit.link>"
    GOOGLE_CLIENT_ID: str = ""
    GOOGLE_CLIENT_SECRET: str = ""
    GITHUB_CLIENT_ID: str = ""
    GITHUB_CLIENT_SECRET: str = ""
    # Provider endpoints. Only end-to-end tests point these elsewhere.
    GOOGLE_AUTHORIZE_URL: str = "https://accounts.google.com/o/oauth2/v2/auth"
    GOOGLE_TOKEN_URL: str = "https://oauth2.googleapis.com/token"
    GOOGLE_TOKENINFO_URL: str = "https://oauth2.googleapis.com/tokeninfo"
    GITHUB_AUTHORIZE_URL: str = "https://github.com/login/oauth/authorize"
    GITHUB_TOKEN_URL: str = "https://github.com/login/oauth/access_token"
    GITHUB_API_URL: str = "https://api.github.com"
    WEB_RISK_API_KEY: str = ""
    DOH_URL: str = "https://cloudflare-dns.com/dns-query"
    CF_API_TOKEN: str = ""
    CF_ACCOUNT_ID: str = ""
    CF_ZONE_ID: str = ""
    CF_SAAS_CNAME_TARGET: str = ""
    CF_KV_NAMESPACE_ID: str = ""
    TURNSTILE_SITE_KEY: str = ""
    TURNSTILE_SECRET_KEY: str = ""

    # Billing & GST
    RAZORPAY_KEY_ID: str = ""
    RAZORPAY_KEY_SECRET: str = ""
    RAZORPAY_WEBHOOK_SECRET: str = ""
    STRIPE_SECRET_KEY: str = ""
    STRIPE_WEBHOOK_SECRET: str = ""
    STRIPE_LEGACY_PRICE_MAP: str = "{}"
    SELLER_LEGAL_NAME: str = "QRit Technologies"
    SELLER_GSTIN: str = ""
    SELLER_STATE_CODE: str = "27"
    SELLER_ADDRESS: str = ""
    SELLER_SAC: str = "998314"
    SELLER_LUT_REF: str = ""
    SELLER_UPI_VPA: str = ""
    SELLER_BANK_DETAILS: str = "{}"
    PRICES_INCLUDE_GST: bool = True

    # Security & Staff
    SERIAL_MAC_KEY: str = "local-serial-mac-key-32-bytes-minimum"
    VERIFY_TOKEN_KEY: str = "local-verify-token-key-32-bytes-minimum"
    STAFF_IP_ALLOWLIST: str = ""
    FREE_TIER_DAILY_LIMIT: int = 5

    # Observability & Concurrency
    SENTRY_DSN: str = ""
    LOG_LEVEL: str = "INFO"
    METRICS_TOKEN: str = "local-metrics-token"
    WEB_CONCURRENCY: int = 2
    WORKER_CONCURRENCY: int = 4
    LEGACY_GEOIP_LOOKUP_URL: str = ""

    @property
    def allowed_hosts_list(self) -> list[str]:
        hosts = [h.strip() for h in self.ALLOWED_HOSTS.split(",") if h.strip()]
        # Railway's health checker and the service's own Railway domains are
        # always accepted: the web app reaches this service on its private
        # domain (*.railway.internal), which a restricted ALLOWED_HOSTS (or the
        # default above) would otherwise reject with 400 DisallowedHost.
        for host in (
            "healthcheck.railway.app",
            self.RAILWAY_PRIVATE_DOMAIN,
            self.RAILWAY_PUBLIC_DOMAIN,
        ):
            if host and host not in hosts and "*" not in hosts:
                hosts.append(host)
        return hosts

    @property
    def cors_allowed_origins_list(self) -> list[str]:
        origins = [o.strip() for o in self.CORS_ALLOWED_ORIGINS.split(",") if o.strip()]
        if self.APP_BASE_URL and self.APP_BASE_URL not in origins:
            origins.append(self.APP_BASE_URL)
        return origins

    @property
    def csrf_trusted_origins_list(self) -> list[str]:
        origins = set(self.cors_allowed_origins_list)
        if self.APP_BASE_URL:
            origins.add(self.APP_BASE_URL)
        if self.API_PUBLIC_URL:
            origins.add(self.API_PUBLIC_URL)
        # Trust Railway subdomains in Railway deployments
        origins.add("https://*.up.railway.app")
        origins.add("https://*.railway.app")
        return sorted(origins)

    @property
    def legacy_hosts_list(self) -> list[str]:
        return [h.strip() for h in self.LEGACY_HOSTS.split(",") if h.strip()]

    @model_validator(mode="after")
    def validate_production_secrets(self) -> "Settings":
        # On Railway an unset APP_ENV must not fall back to "local", which
        # turns DEBUG on and accepts the placeholder secrets committed above.
        if "APP_ENV" not in self.model_fields_set and self.RAILWAY_ENVIRONMENT_NAME:
            name = self.RAILWAY_ENVIRONMENT_NAME.strip().lower()
            self.APP_ENV = "staging" if name == "staging" else "production"

        if self.APP_ENV in ("staging", "production"):
            fields = Settings.model_fields
            missing = [
                name
                for name in (*_PRODUCTION_SECRETS, *_PRODUCTION_ENDPOINTS)
                if not getattr(self, name) or getattr(self, name) == fields[name].default
            ]
            if missing:
                raise ValueError(
                    f"{self.APP_ENV} startup failed: set real values for {', '.join(missing)} "
                    "(empty or the placeholder from qrit/settings/env.py)"
                )
        return self


# Secrets that must not keep their committed placeholder outside local/test:
# with the placeholder JWT key anyone who has read this repository can mint
# access tokens.
_PRODUCTION_SECRETS = (
    "DJANGO_SECRET_KEY",
    "APP_ENCRYPTION_KEY",
    "JWT_ED25519_PRIVATE_KEY",
    "SCAN_SALT_SECRET",
    "EDGE_SHARED_SECRET",
    "SERIAL_MAC_KEY",
    "VERIFY_TOKEN_KEY",
)

# Endpoints whose localhost defaults only work on a developer machine. Left
# unset in production, the database default failed as an unexplained 2-second
# pool timeout, and the APP_BASE_URL default sends Google/GitHub sign-in back
# to localhost:3000.
_PRODUCTION_ENDPOINTS = (
    "DATABASE_URL",
    "APP_BASE_URL",
)


# Read environment once on import
env = Settings()
