"""DRF Authentication and Principal class for Ed25519 JWT sessions.

Plan §7.1 & Go auth parity:
- Extracts token from Bearer header or qrit_access cookie.
- Validates Ed25519 signature and active DB session.
- Enforces double-submit CSRF for cookie-authenticated unsafe requests.
"""

from dataclasses import dataclass
from uuid import UUID

from django.utils import timezone
from rest_framework.authentication import BaseAuthentication
from rest_framework.exceptions import AuthenticationFailed
from rest_framework.request import Request

from apps.core.errors import ApiError, forbidden

from .cookies import CSRF_COOKIE_NAME, extract_access_token
from .models import Session, User
from .tokens import get_token_manager


@dataclass
class Principal:
    """Represents the authenticated caller context."""

    user: User
    session: Session
    is_cookie: bool
    user_id: UUID
    session_id: UUID
    staff_grant_id: UUID | None = None
    api_key_id: UUID | None = None

    @property
    def is_api_key(self) -> bool:
        return self.api_key_id is not None


class SessionAuthentication(BaseAuthentication):
    """DRF Authentication class validating Ed25519 access JWTs and active sessions."""

    def authenticate(self, request: Request) -> tuple[User, Principal] | None:
        token_str, is_cookie = extract_access_token(request)
        if not token_str:
            return None

        tm = get_token_manager()
        try:
            claims = tm.verify_access_token(token_str)
        except ApiError as err:
            raise AuthenticationFailed(detail=err.detail) from err
        except Exception as err:
            raise AuthenticationFailed(detail="Invalid access token") from err

        try:
            user_id = UUID(claims["sub"])
            session_id = UUID(claims["sid"])
        except (ValueError, KeyError) as err:
            raise AuthenticationFailed(detail="Malformed token claims") from err

        # Verify active session in DB
        session = (
            Session.objects.select_related("user")
            .filter(
                id=session_id,
                user_id=user_id,
                revoked_at__isnull=True,
                expires_at__gt=timezone.now(),
                user__deleted_at__isnull=True,
            )
            .first()
        )
        if session is None:
            raise AuthenticationFailed(detail="Session is invalid, expired, or revoked")

        # Double-submit CSRF enforcement for cookie-authenticated mutating requests
        if is_cookie and request.method not in ("GET", "HEAD", "OPTIONS"):
            csrf_cookie = request.COOKIES.get(CSRF_COOKIE_NAME)
            csrf_header = request.headers.get("X-CSRF-Token") or request.META.get(
                "HTTP_X_CSRF_TOKEN"
            )

            if not csrf_cookie:
                raise forbidden("forbidden", "missing csrf cookie")
            if not csrf_header or csrf_header != csrf_cookie:
                raise forbidden("forbidden", "csrf token mismatch")

        staff_grant_id: UUID | None = None
        if "sg" in claims and claims["sg"]:
            try:
                staff_grant_id = UUID(claims["sg"])
            except ValueError:
                pass

        principal = Principal(
            user=session.user,
            session=session,
            is_cookie=is_cookie,
            user_id=user_id,
            session_id=session_id,
            staff_grant_id=staff_grant_id,
        )

        return session.user, principal
