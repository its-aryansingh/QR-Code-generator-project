"""Ed25519 JWT and opaque token utilities.

Plan §6.3, §7.1:
- Ed25519 access JWTs: 10 min lifetime, algorithm EdDSA, issuer "qrit".
- Claims: sub (user_id), sid (session_id), iat, exp, optional sg (staff grant).
- Refresh tokens: 32 random bytes hex-encoded, SHA-256 hashed into database.
"""

import base64
import hashlib
import secrets
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

import jwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey
from django.conf import settings
from django.core.exceptions import ImproperlyConfigured

from apps.core.errors import ApiError

ACCESS_TOKEN_DURATION = 10 * 60  # 10 minutes in seconds
REFRESH_TOKEN_DURATION = 30 * 24 * 3600  # 30 days in seconds
VERIFY_TOKEN_DURATION = 24 * 3600  # 24 hours
RESET_TOKEN_DURATION = 3600  # 1 hour


def hash_token(plain: str) -> bytes:
    """Return SHA-256 hash of a plaintext token string."""
    return hashlib.sha256(plain.encode("utf-8")).digest()


def generate_random_token(n: int = 32) -> tuple[str, bytes]:
    """Generate n cryptographically secure random bytes as hex string and its SHA-256 hash."""
    raw = secrets.token_bytes(n)
    plain = raw.hex()
    hashed = hash_token(plain)
    return plain, hashed


def parse_ed25519_private_key(raw: str | bytes) -> Ed25519PrivateKey:
    """Parse an Ed25519 private key from string (base64, PEM) or bytes."""
    if isinstance(raw, str):
        raw_str = raw.strip()
        # Try PEM format first
        if "BEGIN PRIVATE KEY" in raw_str:
            return serialization.load_pem_private_key(raw_str.encode("utf-8"), password=None)  # type: ignore[return-value]

        # Try base64
        try:
            b64_decoded = base64.b64decode(raw_str)
            if len(b64_decoded) == 32:
                return Ed25519PrivateKey.from_private_bytes(b64_decoded)
            try:
                return serialization.load_der_private_key(b64_decoded, password=None)  # type: ignore[return-value]
            except Exception:
                pass
        except Exception:
            pass

        raw_bytes = raw_str.encode("utf-8")
        if len(raw_bytes) == 32:
            return Ed25519PrivateKey.from_private_bytes(raw_bytes)
        raise ValueError(
            f"Cannot parse Ed25519 private key from provided string of length {len(raw_str)}"
        )

    if isinstance(raw, bytes):
        if len(raw) == 32:
            return Ed25519PrivateKey.from_private_bytes(raw)
        try:
            return serialization.load_der_private_key(raw, password=None)  # type: ignore[return-value]
        except Exception as err:
            raise ValueError(f"Cannot parse Ed25519 private key bytes: {err}") from err

    raise ValueError("Ed25519 private key must be string or bytes")


def _ephemeral_key_allowed() -> bool:
    return getattr(settings, "APP_ENV", "local") in ("local", "test")


class TokenManager:
    """Handles Ed25519 access JWT signing and verification."""

    def __init__(
        self,
        key_id: str | None = None,
        private_key: Ed25519PrivateKey | None = None,
    ) -> None:
        self.key_id = key_id or getattr(settings, "JWT_KEY_ID", "dev-key-1")

        if private_key is None:
            raw_key = getattr(settings, "JWT_ED25519_PRIVATE_KEY", "")
            try:
                private_key = parse_ed25519_private_key(raw_key) if raw_key else None
            except Exception as err:
                if not _ephemeral_key_allowed():
                    raise ImproperlyConfigured(
                        f"JWT_ED25519_PRIVATE_KEY is not a valid Ed25519 private key: {err}"
                    ) from err
            if private_key is None:
                # A key made up per process only works with a single process:
                # tokens signed by one worker fail on every other worker.
                if not _ephemeral_key_allowed():
                    raise ImproperlyConfigured("JWT_ED25519_PRIVATE_KEY must be set")
                private_key = Ed25519PrivateKey.generate()

        self._private_key = private_key
        self._public_key = self._private_key.public_key()

    @property
    def public_key(self) -> Ed25519PublicKey:
        return self._public_key

    def create_access_token(
        self,
        user_id: UUID | str,
        session_id: UUID | str,
        staff_grant: UUID | str | None = None,
        ttl_seconds: int = ACCESS_TOKEN_DURATION,
    ) -> str:
        """Create and sign a short-lived Ed25519 JWT."""
        now = datetime.now(UTC)
        exp = int(now.timestamp()) + ttl_seconds

        claims: dict[str, Any] = {
            "sub": str(user_id),
            "sid": str(session_id),
            "iat": int(now.timestamp()),
            "exp": exp,
            "iss": "qrit",
        }
        if staff_grant is not None:
            claims["sg"] = str(staff_grant)

        headers = {"kid": self.key_id}
        token = jwt.encode(
            claims,
            self._private_key,
            algorithm="EdDSA",
            headers=headers,
        )
        return token

    def verify_access_token(self, token_str: str) -> dict[str, Any]:
        """Verify and decode an Ed25519 JWT."""
        try:
            claims = jwt.decode(
                token_str,
                self._public_key,
                algorithms=["EdDSA"],
                issuer="qrit",
                options={"require": ["sub", "sid", "exp", "iat"]},
            )
            return claims
        except jwt.ExpiredSignatureError as err:
            raise ApiError(
                status=401, code="session_expired", detail="Access token has expired"
            ) from err
        except jwt.InvalidTokenError as err:
            raise ApiError(status=401, code="invalid_token", detail="Invalid access token") from err


_GLOBAL_TOKEN_MANAGER: TokenManager | None = None


def get_token_manager() -> TokenManager:
    """Return singleton TokenManager instance."""
    global _GLOBAL_TOKEN_MANAGER
    if _GLOBAL_TOKEN_MANAGER is None:
        _GLOBAL_TOKEN_MANAGER = TokenManager()
    return _GLOBAL_TOKEN_MANAGER
