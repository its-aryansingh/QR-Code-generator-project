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
    return {
        "accessToken": jwt.encode(access_payload, settings.JWT_SECRET, algorithm="HS256"),
        "refreshToken": jwt.encode(refresh_payload, settings.JWT_SECRET, algorithm="HS256"),
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


def require_api_key(view_func):
    @wraps(view_func)
    def wrapper(self, request, *args, **kwargs):
        key = request.headers.get("X-API-Key") or ""
        auth = request.headers.get("Authorization", "")
        if not key and auth.startswith("Bearer ak_"):
            key = auth[7:]
        if not key:
            return Response({"success": False, "error": "API key required"}, status=401)

        try:
            user = User.objects.get(api_key=key)
        except User.DoesNotExist:
            return Response({"success": False, "error": "Invalid API key"}, status=401)

        from django.utils import timezone
        from django.db.models import F
        limit = settings.PLAN_API_LIMITS.get(user.plan or "free", 50)
        today = timezone.now().date()

        if not user.api_calls_reset_at or user.api_calls_reset_at < today:
            User.objects.filter(pk=user.pk).update(api_calls_today=1, api_calls_reset_at=today)
        elif (user.api_calls_today or 0) >= limit:
            return Response({"success": False, "error": "API rate limit exceeded", "limit": limit}, status=429)
        else:
            User.objects.filter(pk=user.pk).update(api_calls_today=F("api_calls_today") + 1)

        request.auth_user = {"id": str(user.id), "email": user.email, "plan": user.plan or "free"}
        return view_func(self, request, *args, **kwargs)
    return wrapper


def generate_api_key() -> str:
    return f"ak_{uuid.uuid4().hex}"
