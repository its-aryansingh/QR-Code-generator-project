"""Google and GitHub sign-in.

OAuth 2.0 authorization-code flow, done entirely server side:

1. `start`    — the browser is sent to the provider with `state`, a PKCE S256
                challenge and (Google) a `nonce`. The matching secrets live in a
                signed, HttpOnly, SameSite=Lax cookie scoped to the OAuth paths.
2. `callback` — `state` is checked against the cookie, the code is exchanged
                (with the PKCE verifier and the client secret), the identity is
                verified (Google: RS256 ID token against Google's JWKS, `aud`,
                `iss`, `exp`, `nonce`; GitHub: `/user` + verified `/user/emails`)
                and resolved to a QRit user.
3. `exchange` — the callback hands the browser a single-use, 2-minute code
                instead of tokens, and the frontend swaps it for the usual
                access/refresh pair. Tokens never appear in a URL.

Account linking rules (see `resolve_sign_in`):
- A provider identity that is already linked always wins (matched on the
  provider's stable user id, not the email).
- Otherwise the provider must vouch for the email (`email_verified`), or the
  sign-in is refused; an unverified provider email is never used to log into
  an existing account.
- If the matching QRit account never verified its email, whoever set its
  password never proved they own the address, so that password is discarded
  and every session revoked before the identity is linked (pre-account
  takeover defence).
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import logging
import secrets
import uuid
from dataclasses import dataclass
from datetime import timedelta
from functools import lru_cache
from urllib.parse import urlencode

import jwt
import requests
from django.conf import settings
from django.core import signing
from django.db import IntegrityError, transaction
from django.utils import timezone

from api.models import AuthHandoffCode, OAuthAccount, User
from api.utils.auth import (
    generate_api_key,
    hash_password,
    hash_token,
    normalize_email,
    revoke_all_user_tokens,
)

logger = logging.getLogger(__name__)

PROVIDERS = ("google", "github")
PROVIDER_LABELS = {"google": "Google", "github": "GitHub"}
STATE_COOKIE = "qrit_oauth"
STATE_COOKIE_PATH = "/api/v1/auth/oauth/"
DEFAULT_NEXT = "/dashboard"
LINK_INTENT_MAX_AGE_SECONDS = 300

_STATE_SALT = "qrit.oauth.state"
_LINK_SALT = "qrit.oauth.link"
_GITHUB_HEADERS = {
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
    "User-Agent": "QRit-OAuth",
}


class OAuthError(Exception):
    """A sign-in failure with a stable code the frontend turns into a message."""

    def __init__(self, code: str, detail: str = ""):
        super().__init__(detail or code)
        self.code = code
        self.detail = detail


@dataclass(frozen=True)
class ProviderProfile:
    provider: str
    provider_user_id: str
    email: str
    email_verified: bool
    name: str = ""
    avatar_url: str = ""


@dataclass(frozen=True)
class SignInResult:
    user: User
    is_new_user: bool
    password_reset: bool


# --------------------------------------------------------------------- config

def provider_enabled(provider: str) -> bool:
    if provider == "google":
        return bool(settings.GOOGLE_CLIENT_ID and settings.GOOGLE_CLIENT_SECRET)
    if provider == "github":
        return bool(settings.GITHUB_CLIENT_ID and settings.GITHUB_CLIENT_SECRET)
    return False


def enabled_providers() -> dict:
    return {p: provider_enabled(p) for p in PROVIDERS}


def callback_url(provider: str) -> str:
    """The redirect URI registered with the provider (must match exactly)."""
    return f"{settings.OAUTH_CALLBACK_BASE_URL}/api/v1/auth/oauth/{provider}/callback"


def frontend_url(path: str, **params) -> str:
    base = settings.APP_BASE_URL.rstrip("/")
    query = urlencode({k: v for k, v in params.items() if v not in (None, "")})
    return f"{base}{path}" + (f"?{query}" if query else "")


def safe_next(value) -> str:
    """Only same-site relative paths survive; anything else becomes /dashboard.

    Blocks `//evil.com`, `/\\evil.com`, absolute URLs, control characters and
    oversized values, so `next` can never turn the callback into an open
    redirect.
    """
    if not isinstance(value, str) or not value:
        return DEFAULT_NEXT
    if len(value) > 512 or not value.startswith("/") or value.startswith("//"):
        return DEFAULT_NEXT
    if "\\" in value or any(ord(ch) < 32 or ord(ch) == 127 for ch in value):
        return DEFAULT_NEXT
    return value


# ------------------------------------------------------------- state and PKCE

def new_pkce_pair() -> tuple[str, str]:
    verifier = secrets.token_urlsafe(48)  # 64 chars, within RFC 7636's 43..128
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")
    return verifier, challenge


def dump_state(payload: dict) -> str:
    return signing.dumps(payload, salt=_STATE_SALT, compress=True)


def load_state(raw: str) -> dict:
    try:
        return signing.loads(raw, salt=_STATE_SALT, max_age=settings.OAUTH_STATE_MAX_AGE_SECONDS)
    except signing.SignatureExpired:
        raise OAuthError("state_expired")
    except signing.BadSignature:
        raise OAuthError("invalid_state")


def make_link_intent(user_id: str, provider: str) -> str:
    """Short-lived proof that a signed-in user asked to connect `provider`."""
    return signing.dumps(
        {"u": str(user_id), "p": provider, "j": secrets.token_urlsafe(8)}, salt=_LINK_SALT
    )


def read_link_intent(raw: str, provider: str) -> str:
    try:
        data = signing.loads(raw, salt=_LINK_SALT, max_age=LINK_INTENT_MAX_AGE_SECONDS)
    except signing.SignatureExpired:
        raise OAuthError("link_expired")
    except signing.BadSignature:
        raise OAuthError("invalid_state")
    if data.get("p") != provider or not data.get("u"):
        raise OAuthError("invalid_state")
    return data["u"]


def authorize_url(provider: str, *, state: str, code_challenge: str, nonce: str | None = None) -> str:
    if provider == "google":
        params = {
            "client_id": settings.GOOGLE_CLIENT_ID,
            "redirect_uri": callback_url("google"),
            "response_type": "code",
            "scope": "openid email profile",
            "state": state,
            "code_challenge": code_challenge,
            "code_challenge_method": "S256",
            "nonce": nonce or "",
            "prompt": "select_account",
            "access_type": "online",
        }
        return f"{settings.GOOGLE_AUTHORIZE_URL}?{urlencode(params)}"
    if provider == "github":
        params = {
            "client_id": settings.GITHUB_CLIENT_ID,
            "redirect_uri": callback_url("github"),
            "scope": "read:user user:email",
            "state": state,
            "code_challenge": code_challenge,
            "code_challenge_method": "S256",
            "allow_signup": "true",
        }
        return f"{settings.GITHUB_AUTHORIZE_URL}?{urlencode(params)}"
    raise OAuthError("unknown_provider")


# ------------------------------------------------------------ provider calls

def _http() -> requests.Session:
    """Separate hook so tests can swap the transport."""
    return requests.Session()


@lru_cache(maxsize=4)
def _jwk_client(url: str) -> jwt.PyJWKClient:
    # Keys are cached in-process; PyJWKClient refetches when it meets a new kid.
    return jwt.PyJWKClient(url, cache_keys=True, lifespan=3600, timeout=settings.OAUTH_HTTP_TIMEOUT_SECONDS)


def _json(resp: requests.Response) -> dict:
    try:
        data = resp.json()
    except ValueError:
        return {}
    return data if isinstance(data, dict) else {}


def verify_google_id_token(id_token: str, nonce: str | None = None) -> dict:
    if not settings.GOOGLE_CLIENT_ID:
        raise OAuthError("not_configured")
    if not id_token or not isinstance(id_token, str):
        raise OAuthError("invalid_token")
    try:
        signing_key = _jwk_client(settings.GOOGLE_JWKS_URL).get_signing_key_from_jwt(id_token)
        claims = jwt.decode(
            id_token,
            signing_key.key,
            algorithms=["RS256"],
            audience=settings.GOOGLE_CLIENT_ID,
            options={"require": ["exp", "iat", "iss", "aud", "sub"]},
            leeway=60,
        )
    except jwt.PyJWKClientConnectionError as exc:
        logger.warning("Google JWKS unreachable: %s", exc)
        raise OAuthError("provider_unavailable")
    except jwt.PyJWTError as exc:
        logger.info("Rejected Google ID token: %s", exc)
        raise OAuthError("invalid_token")
    if claims.get("iss") not in settings.GOOGLE_ISSUERS:
        raise OAuthError("invalid_token", "unexpected issuer")
    if nonce is not None and not hmac.compare_digest(str(claims.get("nonce", "")), nonce):
        raise OAuthError("invalid_token", "nonce mismatch")
    return claims


def google_profile_from_claims(claims: dict) -> ProviderProfile:
    return ProviderProfile(
        provider="google",
        provider_user_id=str(claims["sub"]),
        email=normalize_email(claims.get("email") or ""),
        email_verified=claims.get("email_verified") in (True, "true", "True"),
        name=(claims.get("name") or "").strip(),
        avatar_url=claims.get("picture") or "",
    )


def exchange_google_code(code: str, verifier: str, nonce: str | None) -> ProviderProfile:
    try:
        resp = _http().post(
            settings.GOOGLE_TOKEN_URL,
            data={
                "code": code,
                "client_id": settings.GOOGLE_CLIENT_ID,
                "client_secret": settings.GOOGLE_CLIENT_SECRET,
                "redirect_uri": callback_url("google"),
                "grant_type": "authorization_code",
                "code_verifier": verifier,
            },
            headers={"Accept": "application/json"},
            timeout=settings.OAUTH_HTTP_TIMEOUT_SECONDS,
        )
    except requests.RequestException as exc:
        logger.warning("Google token endpoint unreachable: %s", exc)
        raise OAuthError("provider_unavailable")
    data = _json(resp)
    if resp.status_code != 200 or not data.get("id_token"):
        logger.warning("Google code exchange failed (%s): %s", resp.status_code, data.get("error"))
        raise OAuthError("exchange_failed", str(data.get("error") or resp.status_code))
    return google_profile_from_claims(verify_google_id_token(data["id_token"], nonce=nonce))


def exchange_github_code(code: str, verifier: str) -> ProviderProfile:
    session = _http()
    timeout = settings.OAUTH_HTTP_TIMEOUT_SECONDS
    try:
        resp = session.post(
            settings.GITHUB_TOKEN_URL,
            data={
                "client_id": settings.GITHUB_CLIENT_ID,
                "client_secret": settings.GITHUB_CLIENT_SECRET,
                "code": code,
                "redirect_uri": callback_url("github"),
                "code_verifier": verifier,
            },
            headers={"Accept": "application/json", "User-Agent": "QRit-OAuth"},
            timeout=timeout,
        )
    except requests.RequestException as exc:
        logger.warning("GitHub token endpoint unreachable: %s", exc)
        raise OAuthError("provider_unavailable")
    data = _json(resp)
    token = data.get("access_token")
    if resp.status_code != 200 or not token:
        # GitHub reports a bad code as HTTP 200 with an `error` field.
        logger.warning("GitHub code exchange failed (%s): %s", resp.status_code, data.get("error"))
        raise OAuthError("exchange_failed", str(data.get("error") or resp.status_code))

    headers = {**_GITHUB_HEADERS, "Authorization": f"Bearer {token}"}
    try:
        user_resp = session.get(f"{settings.GITHUB_API_URL}/user", headers=headers, timeout=timeout)
        emails_resp = session.get(f"{settings.GITHUB_API_URL}/user/emails", headers=headers, timeout=timeout)
    except requests.RequestException as exc:
        logger.warning("GitHub API unreachable: %s", exc)
        raise OAuthError("provider_unavailable")
    if user_resp.status_code != 200:
        raise OAuthError("exchange_failed", f"/user returned {user_resp.status_code}")
    gh_user = _json(user_resp)
    if not gh_user.get("id"):
        raise OAuthError("exchange_failed", "/user returned no id")

    emails = emails_resp.json() if emails_resp.status_code == 200 else []
    if not isinstance(emails, list):
        emails = []
    verified = [e for e in emails if isinstance(e, dict) and e.get("verified") and e.get("email")]
    chosen = next((e for e in verified if e.get("primary")), verified[0] if verified else None)
    if chosen:
        email, email_verified = chosen["email"], True
    else:
        # Nothing verified: keep whatever GitHub shows so the error can say so,
        # but it is never trusted for account matching.
        primary = next((e for e in emails if isinstance(e, dict) and e.get("primary")), None)
        email, email_verified = (primary or {}).get("email") or gh_user.get("email") or "", False

    return ProviderProfile(
        provider="github",
        provider_user_id=str(gh_user["id"]),
        email=normalize_email(email),
        email_verified=email_verified,
        name=(gh_user.get("name") or gh_user.get("login") or "").strip(),
        avatar_url=gh_user.get("avatar_url") or "",
    )


def exchange_code(provider: str, code: str, verifier: str, nonce: str | None) -> ProviderProfile:
    if provider == "google":
        return exchange_google_code(code, verifier, nonce)
    if provider == "github":
        return exchange_github_code(code, verifier)
    raise OAuthError("unknown_provider")


# --------------------------------------------------------- account resolution

def _fill_profile_gaps(user: User, profile: ProviderProfile) -> None:
    updates = {}
    if profile.name and not user.name:
        updates["name"] = profile.name[:255]
    if profile.avatar_url and not user.avatar_url:
        updates["avatar_url"] = profile.avatar_url
    if updates:
        updates["updated_at"] = timezone.now()
        User.objects.filter(pk=user.pk).update(**updates)
        for key, value in updates.items():
            setattr(user, key, value)


def _resolve(profile: ProviderProfile) -> SignInResult:
    now = timezone.now()
    account = (
        OAuthAccount.objects.select_for_update()
        .select_related("user")
        .filter(provider=profile.provider, provider_user_id=profile.provider_user_id)
        .first()
    )
    if account:
        updates = {"last_login_at": now}
        if profile.email and profile.email_verified and profile.email != account.email:
            updates["email"] = profile.email
        OAuthAccount.objects.filter(pk=account.pk).update(**updates)
        _fill_profile_gaps(account.user, profile)
        return SignInResult(account.user, is_new_user=False, password_reset=False)

    if not profile.email:
        raise OAuthError("email_missing")
    if not profile.email_verified:
        raise OAuthError("email_unverified")

    user = User.objects.select_for_update().filter(email__iexact=profile.email).first()
    is_new_user = False
    password_reset = False
    if user:
        if OAuthAccount.objects.filter(user=user, provider=profile.provider).exists():
            # This QRit account is already tied to a different Google/GitHub
            # account; refuse rather than silently switching identities.
            raise OAuthError("provider_already_linked")
        if not user.email_verified:
            user.email_verified = True
            user.email_verified_at = now
            user.password_hash = hash_password(secrets.token_urlsafe(32))
            user.has_usable_password = False
            user.password_changed_at = now
            user.failed_login_attempts = 0
            user.locked_until = None
            user.updated_at = now
            user.save(update_fields=[
                "email_verified", "email_verified_at", "password_hash", "has_usable_password",
                "password_changed_at", "failed_login_attempts", "locked_until", "updated_at",
            ])
            revoke_all_user_tokens(str(user.id))
            password_reset = True
    else:
        user = User(
            id=uuid.uuid4(),
            email=profile.email,
            password_hash=hash_password(secrets.token_urlsafe(32)),
            has_usable_password=False,
            name=profile.name[:255] if profile.name else "",
            avatar_url=profile.avatar_url or None,
            email_verified=True,
            email_verified_at=now,
            api_key=generate_api_key(),
            api_calls_reset_at=now.date(),
            created_at=now,
            updated_at=now,
        )
        user.save(force_insert=True)
        is_new_user = True

    OAuthAccount.objects.create(
        user=user,
        provider=profile.provider,
        provider_user_id=profile.provider_user_id,
        email=profile.email,
        last_login_at=now,
    )
    _fill_profile_gaps(user, profile)
    return SignInResult(user, is_new_user=is_new_user, password_reset=password_reset)


def resolve_sign_in(profile: ProviderProfile) -> SignInResult:
    """Find or create the QRit user for a verified provider identity."""
    for attempt in range(2):
        try:
            with transaction.atomic():
                return _resolve(profile)
        except IntegrityError:
            # Two callbacks for the same new identity raced; the second attempt
            # finds the row the first one created.
            if attempt:
                raise
    raise AssertionError("unreachable")


def link_identity(user_id: str, profile: ProviderProfile) -> OAuthAccount:
    """Attach a provider identity to an already signed-in user."""
    with transaction.atomic():
        existing = (
            OAuthAccount.objects.select_for_update()
            .filter(provider=profile.provider, provider_user_id=profile.provider_user_id)
            .first()
        )
        if existing:
            if str(existing.user_id) != str(user_id):
                raise OAuthError("account_in_use")
            OAuthAccount.objects.filter(pk=existing.pk).update(last_login_at=timezone.now())
            return existing
        if OAuthAccount.objects.filter(user_id=user_id, provider=profile.provider).exists():
            raise OAuthError("provider_already_linked")
        if not User.objects.filter(pk=user_id).exists():
            raise OAuthError("invalid_state")
        return OAuthAccount.objects.create(
            user_id=user_id,
            provider=profile.provider,
            provider_user_id=profile.provider_user_id,
            email=profile.email or None,
        )


# ------------------------------------------------------------- handoff codes

def issue_handoff(result: SignInResult, provider: str, next_path: str) -> str:
    code = secrets.token_urlsafe(32)
    AuthHandoffCode.objects.create(
        code_hash=hash_token(code),
        user=result.user,
        provider=provider,
        next_path=safe_next(next_path),
        is_new_user=result.is_new_user,
        password_reset=result.password_reset,
        expires_at=timezone.now() + timedelta(seconds=settings.OAUTH_HANDOFF_TTL_SECONDS),
    )
    return code


def redeem_handoff(code) -> AuthHandoffCode:
    if not isinstance(code, str) or not code or len(code) > 128:
        raise OAuthError("invalid_code")
    now = timezone.now()
    code_hash = hash_token(code)
    with transaction.atomic():
        claimed = AuthHandoffCode.objects.filter(
            code_hash=code_hash, used_at__isnull=True, expires_at__gt=now
        ).update(used_at=now)
        if claimed != 1:
            raise OAuthError("invalid_code")
        row = AuthHandoffCode.objects.select_related("user").get(code_hash=code_hash)
    # Opportunistic cleanup keeps the table tiny without a scheduled job.
    AuthHandoffCode.objects.filter(expires_at__lt=now - timedelta(hours=1)).delete()
    return row
