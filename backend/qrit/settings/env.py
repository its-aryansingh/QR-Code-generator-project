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
    DJANGO_SETTINGS_MODULE: str = "qrit.settings.api"
    DJANGO_SECRET_KEY: str = "local-insecure-secret-key-change-in-production-min-50-characters-long"
    ALLOWED_HOSTS: str = "localhost,127.0.0.1,healthcheck.railway.app,.railway.app,.up.railway.app"

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
        return [h.strip() for h in self.ALLOWED_HOSTS.split(",") if h.strip()]

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
        if self.APP_ENV == "production":
            required_secrets = [
                ("DJANGO_SECRET_KEY", self.DJANGO_SECRET_KEY),
                ("APP_ENCRYPTION_KEY", self.APP_ENCRYPTION_KEY),
                ("JWT_ED25519_PRIVATE_KEY", self.JWT_ED25519_PRIVATE_KEY),
                ("SCAN_SALT_SECRET", self.SCAN_SALT_SECRET),
                ("EDGE_SHARED_SECRET", self.EDGE_SHARED_SECRET),
                ("SERIAL_MAC_KEY", self.SERIAL_MAC_KEY),
                ("VERIFY_TOKEN_KEY", self.VERIFY_TOKEN_KEY),
            ]
            missing = [
                name
                for name, val in required_secrets
                if not val or "insecure" in val or "dev" in val
            ]
            if missing:
                raise ValueError(
                    f"Production startup failed: the following secrets must be securely configured: {', '.join(missing)}"
                )
        return self


# Read environment once on import
env = Settings()
