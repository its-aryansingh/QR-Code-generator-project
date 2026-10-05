"""Authentication cookie management and token extraction.

Plan §7.1 & Go auth parity:
- AccessCookieName: "qrit_access" (httpOnly, SameSite=Lax, 10 min)
- RefreshCookieName: "qrit_refresh" (httpOnly, SameSite=Strict, 30 days)
- CSRFCookieName: "qrit_csrf" (httpOnly=False for JS double-submit, SameSite=Lax, 30 days)
- CSRFHeaderName: "X-CSRF-Token"
"""

from typing import Any

from django.conf import settings
from django.http import HttpRequest, HttpResponse

from .tokens import ACCESS_TOKEN_DURATION, REFRESH_TOKEN_DURATION

ACCESS_COOKIE_NAME = "qrit_access"
REFRESH_COOKIE_NAME = "qrit_refresh"
CSRF_COOKIE_NAME = "qrit_csrf"
CSRF_HEADER_NAME = "X-CSRF-Token"


def is_cookie_secure() -> bool:
    """Determine if cookies should have the Secure flag set."""
    app_env = getattr(settings, "APP_ENV", "local")
    if app_env in ("staging", "production"):
        return True
    return bool(getattr(settings, "COOKIE_SECURE", False))


def set_auth_cookies(
    response: HttpResponse,
    access_token: str,
    refresh_token: str | None = None,
    csrf_token: str | None = None,
    secure: bool | None = None,
) -> None:
    """Set authentication and CSRF cookies on the response."""
    sec = is_cookie_secure() if secure is None else secure
    cookie_domain = getattr(settings, "COOKIE_DOMAIN", None) or None

    response.set_cookie(
        key=ACCESS_COOKIE_NAME,
        value=access_token,
        max_age=ACCESS_TOKEN_DURATION,
        path="/",
        domain=cookie_domain,
        secure=sec,
        httponly=True,
        samesite="Lax",
    )

    if refresh_token is not None:
        response.set_cookie(
            key=REFRESH_COOKIE_NAME,
            value=refresh_token,
            max_age=REFRESH_TOKEN_DURATION,
            path="/",
            domain=cookie_domain,
            secure=sec,
            httponly=True,
            samesite="Strict",
        )

    if csrf_token is not None:
        response.set_cookie(
            key=CSRF_COOKIE_NAME,
            value=csrf_token,
            max_age=REFRESH_TOKEN_DURATION,
            path="/",
            domain=cookie_domain,
            secure=sec,
            httponly=False,  # JavaScript readable for double-submit
            samesite="Lax",
        )


def clear_auth_cookies(response: HttpResponse, secure: bool | None = None) -> None:
    """Clear all authentication cookies."""
    cookie_domain = getattr(settings, "COOKIE_DOMAIN", None) or None

    for name in (ACCESS_COOKIE_NAME, REFRESH_COOKIE_NAME, CSRF_COOKIE_NAME):
        response.delete_cookie(
            key=name,
            path="/",
            domain=cookie_domain,
            samesite="Lax",
        )


def extract_access_token(request: HttpRequest | Any) -> tuple[str | None, bool]:
    """Extract the access JWT from Authorization header or qrit_access cookie.

    Returns:
        tuple (token_str: str | None, is_cookie: bool)
    """
    auth_header = request.headers.get("Authorization") or request.META.get("HTTP_AUTHORIZATION", "")
    if auth_header.startswith("Bearer "):
        token = auth_header[7:].strip()
        if token:
            return token, False

    if hasattr(request, "COOKIES") and ACCESS_COOKIE_NAME in request.COOKIES:
        token = request.COOKIES[ACCESS_COOKIE_NAME]
        if token:
            return token, True

    return None, False
