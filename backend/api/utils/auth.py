import uuid
import bcrypt
import jwt
from datetime import datetime, timezone, timedelta
from functools import wraps
from django.conf import settings
from rest_framework.response import Response
from api.models import User


def hash_password(raw: str) -> str:
    return bcrypt.hashpw(raw.encode(), bcrypt.gensalt(rounds=12)).decode()


def check_password(raw: str, hashed: str) -> bool:
    return bcrypt.checkpw(raw.encode(), hashed.encode())


def sign_tokens(user_id: str, email: str, plan: str) -> dict:
    now = datetime.now(timezone.utc)
    access_payload = {
        "sub": user_id,
        "email": email,
        "plan": plan,
        "exp": now + timedelta(minutes=settings.JWT_EXPIRY_MINUTES),
        "iat": now,
    }
    refresh_payload = {
        "sub": user_id,
        "type": "refresh",
        "exp": now + timedelta(days=settings.REFRESH_EXPIRY_DAYS),
        "iat": now,
    }
    access = jwt.encode(access_payload, settings.JWT_SECRET, algorithm="HS256")
    refresh = jwt.encode(refresh_payload, settings.JWT_SECRET, algorithm="HS256")
    # Both spellings are emitted: the dashboard reads snake_case, while the
    # pre-Django clients read camelCase. Serving one silently stored
    # `undefined` as the bearer token in the other.
    return {
        "access_token": access,
        "refresh_token": refresh,
        "accessToken": access,
        "refreshToken": refresh,
        "token_type": "Bearer",
        "expires_in": settings.JWT_EXPIRY_MINUTES * 60,
    }


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
            request.auth_user = {
                "id": payload["sub"],
                "email": payload["email"],
                "plan": payload.get("plan", "free"),
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
