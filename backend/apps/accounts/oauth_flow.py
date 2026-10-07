"""Redirect-based Google and GitHub sign-in: state, PKCE and nonce.

The frontend serves `/api/v1/*` from its own domain and forwards it to this
service, so the cookie set here is first-party for the browser.

1. `GET /api/v1/auth/<provider>/start?next=/dashboard` stores a random state,
   a PKCE verifier, a nonce (Google) and `next` in a signed, HttpOnly,
   SameSite=Lax cookie and redirects to the provider. The provider sends the
   browser back to `{APP_BASE_URL}/callback/<provider>`.
2. That frontend page POSTs `{code, state}` to `/api/v1/auth/<provider>`. The
   view checks the state against the cookie before exchanging the code (with
   the verifier), so a callback URL carrying someone else's code is refused in
   any browser that did not start the flow (login CSRF).
"""

import base64
import hashlib
import hmac
import secrets
from typing import Any
from urllib.parse import urlencode

from django.conf import settings
from django.core import signing
from django.http import HttpRequest, HttpResponseBase

from apps.core.errors import ApiError

from .cookies import is_cookie_secure

PROVIDERS = ("google", "github")
STATE_COOKIE = "qrit_oauth"
STATE_COOKIE_PATH = "/api/v1/auth/"
STATE_MAX_AGE_SECONDS = 600
DEFAULT_NEXT = "/dashboard"
_SALT = "qrit.accounts.oauth.state"


def provider_configured(provider: str) -> bool:
    if provider == "google":
        return bool(settings.GOOGLE_CLIENT_ID and settings.GOOGLE_CLIENT_SECRET)
    if provider == "github":
        return bool(settings.GITHUB_CLIENT_ID and settings.GITHUB_CLIENT_SECRET)
    return False


def callback_url(provider: str) -> str:
    """The redirect URI registered with the provider (must match exactly)."""
    return f"{settings.APP_BASE_URL.rstrip('/')}/callback/{provider}"


def frontend_url(path: str, **params: str) -> str:
    query = urlencode({k: v for k, v in params.items() if v})
    return f"{settings.APP_BASE_URL.rstrip('/')}{path}" + (f"?{query}" if query else "")


def safe_next(value: Any) -> str:
    """Only same-site relative paths; anything else becomes /dashboard."""
    if not isinstance(value, str) or not value.startswith("/") or value.startswith("//"):
        return DEFAULT_NEXT
    if len(value) > 512 or "\\" in value or any(ord(ch) < 32 or ord(ch) == 127 for ch in value):
        return DEFAULT_NEXT
    return value


def _pkce_pair() -> tuple[str, str]:
    verifier = secrets.token_urlsafe(48)
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    return verifier, base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")


def authorize_redirect(provider: str, next_path: str) -> tuple[str, str]:
    """Return (provider URL to redirect to, signed cookie value)."""
    state = secrets.token_urlsafe(24)
    verifier, challenge = _pkce_pair()
    nonce = secrets.token_urlsafe(24) if provider == "google" else ""
    cookie = signing.dumps(
        {"p": provider, "s": state, "v": verifier, "n": nonce, "next": safe_next(next_path)},
        salt=_SALT,
        compress=True,
    )
    if provider == "google":
        params = {
            "client_id": settings.GOOGLE_CLIENT_ID,
            "redirect_uri": callback_url("google"),
            "response_type": "code",
            "scope": "openid email profile",
            "state": state,
            "nonce": nonce,
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "prompt": "select_account",
        }
        return f"{settings.GOOGLE_AUTHORIZE_URL}?{urlencode(params)}", cookie
    params = {
        "client_id": settings.GITHUB_CLIENT_ID,
        "redirect_uri": callback_url("github"),
        "scope": "read:user user:email",
        "state": state,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
        "allow_signup": "true",
    }
    return f"{settings.GITHUB_AUTHORIZE_URL}?{urlencode(params)}", cookie


def set_state_cookie(response: HttpResponseBase, value: str) -> None:
    response.set_cookie(
        STATE_COOKIE,
        value,
        max_age=STATE_MAX_AGE_SECONDS,
        path=STATE_COOKIE_PATH,
        secure=is_cookie_secure(),
        httponly=True,
        samesite="Lax",
    )


def clear_state_cookie(response: HttpResponseBase) -> None:
    response.delete_cookie(STATE_COOKIE, path=STATE_COOKIE_PATH, samesite="Lax")


def consume_state(request: HttpRequest, provider: str, returned_state: str) -> dict[str, str]:
    """Check the provider's `state` against this browser's cookie.

    Returns the stored verifier, nonce and next path. Raises 400 `invalid_state`
    when the cookie is missing, expired, tampered with, or for another provider.
    """
    raw = request.COOKIES.get(STATE_COOKIE, "")
    try:
        data = signing.loads(raw, salt=_SALT, max_age=STATE_MAX_AGE_SECONDS) if raw else None
    except signing.BadSignature:
        data = None
    if (
        not isinstance(data, dict)
        or data.get("p") != provider
        or not returned_state
        or not hmac.compare_digest(str(data.get("s", "")), returned_state)
    ):
        raise ApiError(
            status=400,
            code="invalid_state",
            detail="This sign-in expired or was started in another browser. Please try again.",
        )
    return {
        "verifier": str(data.get("v", "")),
        "nonce": str(data.get("n", "")),
        "next": safe_next(data.get("next")),
    }
