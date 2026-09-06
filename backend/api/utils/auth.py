import hashlib
import re
import secrets
import uuid
import bcrypt
import jwt
from datetime import datetime, timezone, timedelta
from functools import wraps
from django.conf import settings
from django.utils import timezone as django_tz
from rest_framework.response import Response
from api.models import User, RefreshToken

# Top 100 most common passwords (subset for fast rejection)
_COMMON_PASSWORDS = frozenset([
    "password", "123456", "12345678", "qwerty", "abc123", "monkey", "1234567",
    "letmein", "trustno1", "dragon", "baseball", "iloveyou", "master", "sunshine",
    "ashley", "michael", "shadow", "123123", "654321", "superman", "qazwsx",
    "password1", "password123", "welcome", "charlie", "donald", "admin",
    "qwerty123", "football", "starwars", "access", "hello", "passw0rd",
    "12345678", "1234567890", "000000", "696969", "mustang", "batman",
    "whatever", "princess", "login", "welcome1", "1qaz2wsx", "123456789",
    "qwerty1", "pass@123", "1q2w3e4r", "123qwe", "zaq12wsx",
])


def normalize_email(email: str) -> str:
    """Lowercase and strip whitespace from email."""
    return (email or "").strip().lower()


def validate_password_strength(password: str) -> list[str]:
    """Validate password meets production-grade requirements.

    Returns a list of human-readable error strings (empty = valid).
    """
    errors = []
    if len(password) < 8:
        errors.append("Password must be at least 8 characters")
    if not re.search(r"[A-Z]", password):
        errors.append("Password must contain at least one uppercase letter")
    if not re.search(r"[a-z]", password):
        errors.append("Password must contain at least one lowercase letter")
    if not re.search(r"\d", password):
        errors.append("Password must contain at least one number")
    if not re.search(r"[!@#$%^&*()_+\-=\[\]{};':\"\\|,.<>/?]", password):
        errors.append("Password must contain at least one special character")
    if password.lower() in _COMMON_PASSWORDS:
        errors.append("This password is too common — please choose a stronger one")
    return errors


def hash_password(raw: str) -> str:
    return bcrypt.hashpw(raw.encode(), bcrypt.gensalt(rounds=12)).decode()


def check_password(raw: str, hashed: str) -> bool:
    try:
        return bcrypt.checkpw(raw.encode(), hashed.encode())
    except Exception:
        return False


def generate_secure_token() -> str:
    """Generate a URL-safe, 64-character random token for password resets
    and email verification."""
    return secrets.token_urlsafe(48)


def hash_token(token: str) -> str:
    """Hash a secure token for storage (SHA-256 — fast lookup, no need for bcrypt
    since the tokens are high-entropy random)."""
    return hashlib.sha256(token.encode()).hexdigest()


def sign_tokens(user_id: str, email: str, plan: str) -> dict:
    """Issue a new access + refresh token pair.

    The refresh token is persisted in the database to support rotation
    and revocation. A new token family is created for every fresh login.
    """
    now = datetime.now(timezone.utc)
    access_jti = uuid.uuid4().hex
    refresh_jti = uuid.uuid4().hex
    family_id = uuid.uuid4()

    access_payload = {
        "sub": user_id,
        "email": email,
        "plan": plan,
        "jti": access_jti,
        "type": "access",
        "exp": now + timedelta(minutes=settings.JWT_EXPIRY_MINUTES),
        "iat": now,
    }
    refresh_payload = {
        "sub": user_id,
        "jti": refresh_jti,
        "type": "refresh",
        "family": str(family_id),
        "exp": now + timedelta(days=settings.REFRESH_EXPIRY_DAYS),
        "iat": now,
    }
    access = jwt.encode(access_payload, settings.JWT_SECRET, algorithm="HS256")
    refresh = jwt.encode(refresh_payload, settings.JWT_SECRET, algorithm="HS256")

    # Persist refresh token for rotation / revocation
    RefreshToken.objects.create(
        user_id=user_id,
        jti=refresh_jti,
        token_hash=hash_token(refresh),
        family_id=family_id,
        expires_at=now + timedelta(days=settings.REFRESH_EXPIRY_DAYS),
    )

    return {
        "access_token": access,
        "refresh_token": refresh,
        "token_type": "Bearer",
        "expires_in": settings.JWT_EXPIRY_MINUTES * 60,
    }


def rotate_refresh_token(old_jti: str, user_id: str, email: str, plan: str) -> dict | None:
    """Issue a new token pair while revoking the old refresh token.

    Implements **refresh token rotation with reuse detection**:
    - If the old token is already revoked, the entire family is revoked
      (a stolen token was reused) and None is returned.
    - Otherwise, the old token is revoked and a new pair is issued
      in the same family.
    """
    try:
        old_token = RefreshToken.objects.select_for_update().get(jti=old_jti)
    except RefreshToken.DoesNotExist:
        return None

    # Reuse detection: if already revoked, someone stole it
    if old_token.revoked:
        # Revoke entire family
        RefreshToken.objects.filter(family_id=old_token.family_id).update(revoked=True)
        return None

    # Revoke old token
    old_token.revoked = True
    old_token.save(update_fields=["revoked"])

    # Issue new pair in same family
    now = datetime.now(timezone.utc)
    new_access_jti = uuid.uuid4().hex
    new_refresh_jti = uuid.uuid4().hex

    access_payload = {
        "sub": user_id,
        "email": email,
        "plan": plan,
        "jti": new_access_jti,
        "type": "access",
        "exp": now + timedelta(minutes=settings.JWT_EXPIRY_MINUTES),
        "iat": now,
    }
    refresh_payload = {
        "sub": user_id,
        "jti": new_refresh_jti,
        "type": "refresh",
        "family": str(old_token.family_id),
        "exp": now + timedelta(days=settings.REFRESH_EXPIRY_DAYS),
        "iat": now,
    }
    access = jwt.encode(access_payload, settings.JWT_SECRET, algorithm="HS256")
    refresh = jwt.encode(refresh_payload, settings.JWT_SECRET, algorithm="HS256")

    new_rt = RefreshToken.objects.create(
        user_id=user_id,
        jti=new_refresh_jti,
        token_hash=hash_token(refresh),
        family_id=old_token.family_id,
        expires_at=now + timedelta(days=settings.REFRESH_EXPIRY_DAYS),
    )
    old_token.replaced_by = new_rt
    old_token.save(update_fields=["replaced_by"])

    return {
        "access_token": access,
        "refresh_token": refresh,
        "token_type": "Bearer",
        "expires_in": settings.JWT_EXPIRY_MINUTES * 60,
    }


def revoke_all_user_tokens(user_id: str):
    """Revoke every outstanding refresh token for a user (e.g. password change)."""
    RefreshToken.objects.filter(user_id=user_id, revoked=False).update(revoked=True)


def decode_token(token: str) -> dict:
    return jwt.decode(token, settings.JWT_SECRET, algorithms=["HS256"])


def require_auth(view_func):
    @wraps(view_func)
    def wrapper(self, request, *args, **kwargs):
        auth = request.headers.get("Authorization", "")
        if not auth.startswith("Bearer "):
            return Response({"success": False, "error": "Unauthorized"}, status=401)
        try:
            payload = decode_token(auth[7:])
            if payload.get("type") != "access":
                return Response({"success": False, "error": "Invalid token type"}, status=401)
            request.auth_user = {
                "id": payload["sub"],
                "email": payload.get("email", ""),
                "plan": payload.get("plan", "free"),
                "jti": payload.get("jti"),
            }
        except jwt.ExpiredSignatureError:
            return Response({"success": False, "error": "Token expired"}, status=401)
        except jwt.InvalidTokenError:
            return Response({"success": False, "error": "Invalid token"}, status=401)
        return view_func(self, request, *args, **kwargs)
    return wrapper


def _authenticate_workspace_key(raw_key):
    """Resolve a workspace-scoped key of the form `qk_xxxxxxxx.<secret>`.

    Only the hash is stored, so the lookup goes prefix -> bcrypt verify.
    Returns (context, error_response).
    """
    from django.utils import timezone as tz

    from api.models import ApiKey, Workspace
    from api.utils.entitlements import limit_for

    prefix, sep, secret = raw_key.partition(".")
    if not sep:
        return None, None  # not a workspace key; caller falls back to legacy

    key = ApiKey.objects.filter(prefix=prefix, revoked_at__isnull=True).first()
    if not key or not bcrypt.checkpw(secret.encode(), key.key_hash.encode()):
        return None, Response({"success": False, "error": "Invalid API key"}, status=401)
    if key.expires_at and key.expires_at < tz.now():
        return None, Response({"success": False, "error": "This API key has expired"}, status=401)

    workspace = Workspace.objects.filter(id=key.workspace_id).first()
    plan = (workspace.plan if workspace else "free") or "free"
    limit = settings.PLAN_API_LIMITS.get(plan, 50)
    today = tz.now().date()

    from django.db.models import F as _F

    if not key.calls_reset_at or key.calls_reset_at < today:
        ApiKey.objects.filter(pk=key.pk).update(
            calls_today=1, calls_reset_at=today, total_calls=_F("total_calls") + 1,
            last_used_at=tz.now(),
        )
    elif int(key.calls_today or 0) >= limit:
        return None, Response(
            {"success": False, "error": "API rate limit exceeded", "limit": limit,
             "code": "rate_limited"},
            status=429,
        )
    else:
        ApiKey.objects.filter(pk=key.pk).update(
            calls_today=_F("calls_today") + 1, total_calls=_F("total_calls") + 1,
            last_used_at=tz.now(),
        )

    return {
        "id": str(key.created_by) if key.created_by else str(workspace.owner_id),
        "email": None,
        "plan": plan,
        "workspace_id": str(key.workspace_id),
        "scopes": [s.strip() for s in (key.scopes or "").split(",") if s.strip()],
        "api_key_id": str(key.id),
        "daily_limit": limit,
        "max_batch": limit_for(plan, "bulk_batch_size"),
    }, None


def require_api_key(view_func):
    """Authenticate a machine caller.

    Accepts workspace-scoped keys (`qk_....<secret>`) and the legacy
    per-user `ak_` keys so existing integrations keep working.
    """

    @wraps(view_func)
    def wrapper(self, request, *args, **kwargs):
        key = request.headers.get("X-API-Key") or ""
        auth = request.headers.get("Authorization", "")
        if not key and auth.startswith("Bearer "):
            candidate = auth[7:]
            if candidate.startswith(("ak_", "qk_")):
                key = candidate
        if not key:
            return Response({"success": False, "error": "API key required"}, status=401)

        context, error = _authenticate_workspace_key(key)
        if error is not None:
            return error
        if context is not None:
            request.auth_user = context
            return view_func(self, request, *args, **kwargs)

        try:
            user = User.objects.get(api_key=key)
        except User.DoesNotExist:
            return Response({"success": False, "error": "Invalid API key"}, status=401)

        from django.utils import timezone as tz
        from django.db.models import F

        limit = settings.PLAN_API_LIMITS.get(user.plan or "free", 50)
        today = tz.now().date()

        if not user.api_calls_reset_at or user.api_calls_reset_at < today:
            User.objects.filter(pk=user.pk).update(api_calls_today=1, api_calls_reset_at=today)
        elif (user.api_calls_today or 0) >= limit:
            return Response(
                {"success": False, "error": "API rate limit exceeded", "limit": limit,
                 "code": "rate_limited"},
                status=429,
            )
        else:
            User.objects.filter(pk=user.pk).update(api_calls_today=F("api_calls_today") + 1)

        request.auth_user = {
            "id": str(user.id), "email": user.email, "plan": user.plan or "free",
            "workspace_id": str(user.default_workspace_id) if user.default_workspace_id else None,
            "scopes": ["qr:read", "qr:write", "analytics:read"],
            "daily_limit": limit,
        }
        return view_func(self, request, *args, **kwargs)

    return wrapper


def generate_api_key() -> str:
    return f"ak_{uuid.uuid4().hex}"
