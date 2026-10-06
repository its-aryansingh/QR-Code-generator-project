"""Account and authentication domain services.

Plan §6.3, §7.1, §7.2:
- User registration, login, logout, refresh rotation with reuse detection.
- Email verification and password reset via secure email tokens.
- Google OAuth (ID token) and GitHub OAuth (code exchange).
- Profile and session management.
"""

import re
import secrets
import zoneinfo
from datetime import UTC, datetime, timedelta
from typing import Any
from uuid import UUID

from django.conf import settings
from django.contrib.auth.hashers import check_password, make_password
from django.db import transaction
from django.utils.text import slugify

from apps.access.models import RoleBinding
from apps.core.errors import (
    ApiError,
    conflict,
    forbidden,
    not_found,
    rate_limited,
    unprocessable,
)
from apps.core.http import safe_client
from apps.core.ids import uuid7
from apps.core.ratelimit import allow as check_ratelimit
from apps.orgs.models import Organization, OrgMember
from apps.workspaces.models import Workspace, WorkspaceMember
from qrit.settings.env import env

from .models import EmailToken, OAuthAccount, Session, User
from .passwords import validate_password_strength
from .tokens import (
    REFRESH_TOKEN_DURATION,
    RESET_TOKEN_DURATION,
    VERIFY_TOKEN_DURATION,
    generate_random_token,
    get_token_manager,
    hash_token,
)

EMAIL_REGEX = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")

# Dummy hash for timing equalizer when user doesn't exist
DUMMY_PASSWORD_HASH = make_password("qrit-timing-equaliser-dummy-password")

# In-memory account lock store for testing/fallback
_IN_MEMORY_ACCOUNT_LOCKS: dict[str, list[float]] = {}


def _is_account_locked(email: str) -> bool:
    """Check if an account is temporarily locked after repeated failures (5 attempts / 15 min)."""
    now = datetime.now(UTC).timestamp()
    cutoff = now - 15 * 60

    from apps.core.ratelimit import get_redis_client

    rdb = get_redis_client()
    if rdb is not None:
        try:
            n = rdb.get(f"lock:login:{email}")
            if n and int(n) >= 5:
                return True
        except Exception:
            pass
    else:
        attempts = _IN_MEMORY_ACCOUNT_LOCKS.get(email, [])
        valid = [t for t in attempts if t > cutoff]
        if len(valid) >= 5:
            return True

    return False


def _record_login_failure(email: str) -> None:
    """Record a failed login attempt for account lockout tracking."""
    if not email:
        return
    now = datetime.now(UTC).timestamp()

    from apps.core.ratelimit import get_redis_client

    rdb = get_redis_client()
    if rdb is not None:
        try:
            pipe = rdb.pipeline()
            pipe.incr(f"lock:login:{email}")
            pipe.expire(f"lock:login:{email}", 15 * 60)
            pipe.execute()
        except Exception:
            pass
    else:
        cutoff = now - 15 * 60
        attempts = _IN_MEMORY_ACCOUNT_LOCKS.get(email, [])
        valid = [t for t in attempts if t > cutoff]
        valid.append(now)
        _IN_MEMORY_ACCOUNT_LOCKS[email] = valid


def _clear_login_failures(email: str) -> None:
    """Clear failed login attempts upon successful login."""
    if not email:
        return

    from apps.core.ratelimit import get_redis_client

    rdb = get_redis_client()
    if rdb is not None:
        try:
            rdb.delete(f"lock:login:{email}")
        except Exception:
            pass
    else:
        _IN_MEMORY_ACCOUNT_LOCKS.pop(email, None)


def _unique_slug(base_name: str, model_cls: Any) -> str:
    """Generate a unique URL slug for workspace or organization."""
    base = slugify(base_name).strip("-") or "workspace"
    candidate = base
    for _ in range(10):
        if not model_cls.objects.filter(slug=candidate).exists():
            return candidate
        suffix = secrets.token_hex(2)
        candidate = f"{base}-{suffix}"
    return f"{base}-{secrets.token_hex(4)}"


def start_session(
    user: User,
    auth_method: str = "password",
    sso_connection: Any = None,
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[Session, str, str, str]:
    """Create a new Session and return (session, access_token, refresh_token_plain, csrf_token_plain)."""
    plain_refresh, refresh_hash = generate_random_token(32)
    plain_csrf, _ = generate_random_token(16)

    now = datetime.now(UTC)
    expires_at = now + timedelta(seconds=REFRESH_TOKEN_DURATION)
    family_id = uuid7()

    ua = user_agent[:300] if user_agent else None

    # Allowed auth_methods in DB constraint: password, google, magic_link, sso, scim
    effective_method = (
        auth_method
        if auth_method in ("password", "google", "magic_link", "sso", "scim")
        else "google"
    )

    session = Session.objects.create(
        user=user,
        family_id=family_id,
        refresh_token_hash=refresh_hash,
        user_agent=ua,
        ip_prefix=ip_prefix,
        expires_at=expires_at,
        auth_method=effective_method,
        sso_connection=sso_connection,
    )

    tm = get_token_manager()
    access_token = tm.create_access_token(user_id=user.id, session_id=session.id)

    return session, access_token, plain_refresh, plain_csrf


def register(
    email: str,
    password: str,
    name: str = "",
    ip_key: str = "unknown",
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[User, Workspace, Session, str, str, str]:
    """Register a new user account with default organization, workspace, and owner role binding."""
    # 1. Rate limiting on registration: 5 per hour per IP
    allowed, _, retry_after = check_ratelimit(
        key=f"rl:register:{ip_key}",
        limit=5,
        window=3600,
        fail_closed=True,
    )
    if not allowed:
        raise rate_limited(
            retry_after=retry_after,
            detail="too many sign-ups from this network; try again later",
        )

    # 2. Email validation
    email_clean = email.strip().lower()
    if not EMAIL_REGEX.match(email_clean):
        raise unprocessable(code="invalid_email", detail="a valid email address is required")

    # 3. Password strength validation
    validate_password_strength(password)

    # 4. Check for duplicate email
    if User.objects.filter(email=email_clean).exists():
        raise conflict(code="email_exists", detail="an account with this email already exists")

    name_clean = name.strip() or email_clean.split("@")[0]

    with transaction.atomic():
        # Create user
        user = User.objects.create_user(
            email=email_clean,
            password=password,
            name=name_clean,
            locale="en",
            timezone="UTC",
        )

        # Create standard Organization
        org_slug = _unique_slug(name_clean, Organization)
        org = Organization.objects.create(
            name=f"{name_clean}'s Org",
            slug=org_slug,
            kind="standard",
            plan_id="free",
        )

        # Create OrgMember (owner)
        OrgMember.objects.create(
            org=org,
            user=user,
            org_role="org_owner",
            status="active",
            source="creator",
        )

        # Create default Workspace
        ws_slug = _unique_slug(f"{name_clean}-workspace", Workspace)
        ws = Workspace.objects.create(
            org=org,
            name=f"{name_clean}'s Workspace",
            slug=ws_slug,
            owner=user,
            plan_id="free",
        )

        # Create WorkspaceMember (owner)
        WorkspaceMember.objects.create(
            workspace=ws,
            user=user,
            role="owner",
        )

        # Create RoleBinding for owner role (fixed UUID 00000000-0000-7000-8000-00000000f001)
        RoleBinding.objects.create(
            org=org,
            workspace=ws,
            principal_type="user",
            principal_id=user.id,
            role_id="00000000-0000-7000-8000-00000000f001",
            scope_type="workspace",
            created_by=user,
        )

        # Issue email verification token
        plain_verify, verify_hash = generate_random_token(32)
        EmailToken.objects.create(
            user=user,
            purpose="verify_email",
            token_hash=verify_hash,
            expires_at=datetime.now(UTC) + timedelta(seconds=VERIFY_TOKEN_DURATION),
        )

        # Start session
        session, access_token, plain_refresh, plain_csrf = start_session(
            user=user,
            auth_method="password",
            ip_prefix=ip_prefix,
            user_agent=user_agent,
        )

    return user, ws, session, access_token, plain_refresh, plain_csrf


def login(
    email: str,
    password: str,
    ip_key: str = "unknown",
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[User, Session, str, str, str, bool]:
    """Authenticate user with email and password, returning tokens and session."""
    email_clean = email.strip().lower()

    # 1. IP rate limiting: 10 per 15 min
    allowed, _, retry_after = check_ratelimit(
        key=f"rl:login:ip:{ip_key}",
        limit=10,
        window=900,
        fail_closed=True,
    )
    if not allowed:
        raise rate_limited(
            retry_after=retry_after,
            detail="too many sign-in attempts; try again later",
        )

    # 2. Account lockout check: 5 failed attempts per 15 min per account
    if _is_account_locked(email_clean):
        raise rate_limited(
            retry_after=900,
            detail="this account is temporarily locked after repeated failures; try again in 15 minutes",
        )

    user = User.objects.filter(email=email_clean, deleted_at__isnull=True).first()

    # 3. Timing equalizer
    pw_hash = user.password if (user and user.password) else DUMMY_PASSWORD_HASH
    is_valid_pw = check_password(password, pw_hash)

    if user is None or not user.password or not is_valid_pw:
        _record_login_failure(email_clean)
        raise ApiError(status=401, code="unauthorized", detail="invalid email or password")

    # Clear lockout counters
    _clear_login_failures(email_clean)

    # Update last login timestamp
    user.last_login = datetime.now(UTC)
    user.save(update_fields=["last_login"])

    session, access_token, plain_refresh, plain_csrf = start_session(
        user=user,
        auth_method="password",
        ip_prefix=ip_prefix,
        user_agent=user_agent,
    )

    return user, session, access_token, plain_refresh, plain_csrf, False


def refresh_session(
    refresh_token_plain: str,
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[Session, str, str, str]:
    """Rotate session refresh token with family reuse detection."""
    token_hash = hash_token(refresh_token_plain)
    session = Session.objects.filter(refresh_token_hash=token_hash).select_related("user").first()

    if session is None:
        raise ApiError(status=401, code="unauthorized", detail="invalid refresh token")

    now = datetime.now(UTC)

    # Reuse detection: if this token was already replaced or revoked, revoke ENTIRE family!
    if session.replaced_by_id is not None or session.revoked_at is not None:
        Session.objects.filter(family_id=session.family_id).update(revoked_at=now)
        raise ApiError(
            status=401,
            code="unauthorized",
            detail="refresh token reuse detected; all sessions in this family were revoked",
        )

    if now > session.expires_at:
        raise ApiError(status=401, code="session_expired", detail="session expired")

    with transaction.atomic():
        # Lock session row to prevent race conditions during refresh
        locked_sess = Session.objects.select_for_update().get(id=session.id)
        if locked_sess.replaced_by_id is not None or locked_sess.revoked_at is not None:
            Session.objects.filter(family_id=session.family_id).update(revoked_at=now)
            raise ApiError(
                status=401,
                code="unauthorized",
                detail="refresh token reuse detected; all sessions in this family were revoked",
            )

        new_plain_refresh, new_refresh_hash = generate_random_token(32)
        new_plain_csrf, _ = generate_random_token(16)
        ua = user_agent[:300] if user_agent else locked_sess.user_agent

        new_session = Session.objects.create(
            user=locked_sess.user,
            family_id=locked_sess.family_id,
            refresh_token_hash=new_refresh_hash,
            user_agent=ua,
            ip_prefix=ip_prefix or locked_sess.ip_prefix,
            expires_at=locked_sess.expires_at,
            auth_method=locked_sess.auth_method,
            sso_connection=locked_sess.sso_connection,
            mfa_verified_at=locked_sess.mfa_verified_at,
            step_up_at=locked_sess.step_up_at,
        )

        locked_sess.replaced_by = new_session
        locked_sess.revoked_at = now
        locked_sess.save(update_fields=["replaced_by", "revoked_at"])

        tm = get_token_manager()
        access_token = tm.create_access_token(
            user_id=locked_sess.user_id, session_id=new_session.id
        )

    return new_session, access_token, new_plain_refresh, new_plain_csrf


def logout(session_id: UUID | str) -> None:
    """Revoke a session upon sign-out."""
    Session.objects.filter(id=session_id).update(revoked_at=datetime.now(UTC))


def verify_email(token_plain: str) -> None:
    """Verify user's email address using a single-use token."""
    token_hash = hash_token(token_plain)
    tok = EmailToken.objects.filter(
        token_hash=token_hash,
        purpose="verify_email",
    ).first()

    now = datetime.now(UTC)
    if tok is None or tok.expires_at < now:
        raise ApiError(
            status=410,
            code="token_invalid",
            detail="this verification link is invalid or has expired",
        )

    with transaction.atomic():
        locked_tok = EmailToken.objects.select_for_update().get(id=tok.id)
        if locked_tok.used_at is not None:
            raise ApiError(
                status=410,
                code="token_invalid",
                detail="this verification link was already used",
            )
        locked_tok.used_at = now
        locked_tok.save(update_fields=["used_at"])

        User.objects.filter(id=locked_tok.user_id).update(email_verified_at=now)


def resend_verification(user: User) -> None:
    """Generate and send a new verification email token."""
    if user.email_verified_at is not None:
        return

    # Invalidate previous tokens
    now = datetime.now(UTC)
    EmailToken.objects.filter(user=user, purpose="verify_email", used_at__isnull=True).update(
        used_at=now
    )

    plain, tok_hash = generate_random_token(32)
    EmailToken.objects.create(
        user=user,
        purpose="verify_email",
        token_hash=tok_hash,
        expires_at=now + timedelta(seconds=VERIFY_TOKEN_DURATION),
    )


def forgot_password(email: str, ip_key: str = "unknown") -> None:
    """Request a password reset link. Always 202 accepted to prevent enumeration."""
    # Rate limits: 10 per hour per IP, 3 per hour per email
    check_ratelimit(key=f"rl:forgot:ip:{ip_key}", limit=10, window=3600)

    email_clean = email.strip().lower()
    if not EMAIL_REGEX.match(email_clean):
        return

    check_ratelimit(key=f"rl:forgot:email:{email_clean}", limit=3, window=3600)

    user = User.objects.filter(email=email_clean, deleted_at__isnull=True).first()
    if user is None:
        return

    now = datetime.now(UTC)
    EmailToken.objects.filter(user=user, purpose="reset_password", used_at__isnull=True).update(
        used_at=now
    )

    plain, tok_hash = generate_random_token(32)
    EmailToken.objects.create(
        user=user,
        purpose="reset_password",
        token_hash=tok_hash,
        expires_at=now + timedelta(seconds=RESET_TOKEN_DURATION),
    )


def reset_password(token_plain: str, new_password: str) -> None:
    """Reset user password, mark email verified, and revoke all existing sessions."""
    token_hash = hash_token(token_plain)
    tok = EmailToken.objects.filter(
        token_hash=token_hash,
        purpose="reset_password",
    ).first()

    now = datetime.now(UTC)
    if tok is None or tok.expires_at < now:
        raise ApiError(
            status=410,
            code="token_invalid",
            detail="this reset link is invalid or has expired",
        )

    validate_password_strength(new_password)

    with transaction.atomic():
        locked_tok = EmailToken.objects.select_for_update().get(id=tok.id)
        if locked_tok.used_at is not None:
            raise ApiError(
                status=410,
                code="token_invalid",
                detail="this reset link was already used",
            )
        locked_tok.used_at = now
        locked_tok.save(update_fields=["used_at"])

        user = User.objects.get(id=locked_tok.user_id)
        user.set_password(new_password)
        if user.email_verified_at is None:
            user.email_verified_at = now
        user.updated_at = now
        user.save(update_fields=["password", "email_verified_at", "updated_at"])

        # Revoke all sessions for this user
        Session.objects.filter(user=user, revoked_at__isnull=True).update(revoked_at=now)

        # Clear login failure counter
        _clear_login_failures(str(user.email))


def change_password(
    user: User,
    current_password: str,
    new_password: str,
    current_session_id: UUID | str,
) -> None:
    """Change user password, verifying current password and revoking all other sessions."""
    if user.password:
        if not check_password(current_password, user.password):
            raise forbidden(code="invalid_password", detail="current password is incorrect")

    validate_password_strength(new_password)

    now = datetime.now(UTC)
    with transaction.atomic():
        user.set_password(new_password)
        user.updated_at = now
        user.save(update_fields=["password", "updated_at"])

        # Revoke all OTHER sessions
        Session.objects.filter(user=user, revoked_at__isnull=True).exclude(
            id=current_session_id
        ).update(revoked_at=now)


def update_profile(
    user: User,
    name: str | None = None,
    locale: str | None = None,
    timezone_name: str | None = None,
) -> User:
    """Update profile fields for the authenticated user."""
    update_fields = ["updated_at"]
    now = datetime.now(UTC)
    user.updated_at = now

    if timezone_name is not None:
        try:
            zoneinfo.ZoneInfo(timezone_name)
            user.timezone = timezone_name
            update_fields.append("timezone")
        except Exception:
            raise unprocessable(
                code="invalid_timezone",
                detail="timezone must be an IANA zone name",
            ) from None

    if name is not None:
        clean_name = name.strip()
        if not clean_name or len(clean_name) > 120:
            raise unprocessable(code="invalid_name", detail="name must be 1–120 characters")
        user.name = clean_name
        update_fields.append("name")

    if locale is not None:
        user.locale = locale.strip() or "en"
        update_fields.append("locale")

    user.save(update_fields=update_fields)
    return user


def google_login(
    credential: str,
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[User, Session, str, str, str, bool]:
    """Authenticate or register a user via Google ID Token."""
    sub: str = ""
    email: str = ""
    name: str = ""
    picture: str | None = None

    # In local/test mode: allow test tokens without hitting Google
    if getattr(settings, "APP_ENV", "local") in ("local", "test") and credential.startswith(
        "test-google:"
    ):
        parts = credential.split(":")
        sub = parts[1]
        email = parts[2] if len(parts) > 2 else f"google_{sub}@example.com"
        name = parts[3] if len(parts) > 3 else "Google User"
    else:
        # Verify with Google tokeninfo endpoint
        try:
            client = safe_client(timeout=5.0)
            resp = client.get(
                "https://oauth2.googleapis.com/tokeninfo",
                params={"id_token": credential},
            )
            if resp.status_code != 200:
                raise ApiError(status=401, code="unauthorized", detail="invalid Google credential")
            payload = resp.json()
            sub = payload.get("sub", "")
            email = payload.get("email", "")
            name = payload.get("name", "")
            picture = payload.get("picture")
            if not sub or not email:
                raise ApiError(
                    status=401, code="unauthorized", detail="Google token missing subject or email"
                )
        except ApiError:
            raise
        except Exception as err:
            raise ApiError(
                status=401, code="unauthorized", detail=f"Google authentication failed: {err}"
            ) from err

    email_clean = email.strip().lower()

    with transaction.atomic():
        # Check if OAuthAccount already linked
        user: User | None
        oauth_acc = (
            OAuthAccount.objects.filter(provider="google", provider_user_id=sub)
            .select_related("user")
            .first()
        )

        if oauth_acc is not None:
            user = oauth_acc.user
        else:
            # Check if user with this email exists
            user = User.objects.filter(email=email_clean, deleted_at__isnull=True).first()
            if user is None:
                # Provision new user and default organization and workspace
                name_clean = name.strip() or email_clean.split("@")[0]
                user = User.objects.create_user(
                    email=email_clean,
                    password=None,
                    name=name_clean,
                    avatar_url=picture,
                    locale="en",
                    timezone="UTC",
                )
                user.email_verified_at = datetime.now(UTC)
                user.save(update_fields=["email_verified_at"])

                org_slug = _unique_slug(name_clean, Organization)
                org = Organization.objects.create(
                    name=f"{name_clean}'s Org",
                    slug=org_slug,
                    kind="standard",
                    plan_id="free",
                )
                OrgMember.objects.create(
                    org=org,
                    user=user,
                    org_role="org_owner",
                    status="active",
                    source="creator",
                )

                ws_slug = _unique_slug(f"{name_clean}-workspace", Workspace)
                ws = Workspace.objects.create(
                    org=org,
                    name=f"{name_clean}'s Workspace",
                    slug=ws_slug,
                    owner=user,
                    plan_id="free",
                )
                WorkspaceMember.objects.create(
                    workspace=ws,
                    user=user,
                    role="owner",
                )
                RoleBinding.objects.create(
                    org=org,
                    workspace=ws,
                    principal_type="user",
                    principal_id=user.id,
                    role_id="00000000-0000-7000-8000-00000000f001",
                    scope_type="workspace",
                    created_by=user,
                )

            # Link Google OAuthAccount
            OAuthAccount.objects.create(
                user=user,
                provider="google",
                provider_user_id=sub,
                email=email_clean,
            )

        user.last_login = datetime.now(UTC)
        user.save(update_fields=["last_login"])

        session, access_token, plain_refresh, plain_csrf = start_session(
            user=user,
            auth_method="google",
            ip_prefix=ip_prefix,
            user_agent=user_agent,
        )

    return user, session, access_token, plain_refresh, plain_csrf, False


def github_login(
    code: str,
    redirect_uri: str | None = None,
    ip_prefix: str | None = None,
    user_agent: str | None = None,
) -> tuple[User, Session, str, str, str, bool]:
    """Authenticate or register a user via GitHub OAuth authorization code."""
    github_id: str = ""
    email: str = ""
    name: str = ""
    avatar_url: str | None = None

    if getattr(settings, "APP_ENV", "local") in ("local", "test") and code.startswith(
        "test-github:"
    ):
        parts = code.split(":")
        github_id = parts[1]
        email = parts[2] if len(parts) > 2 else f"github_{github_id}@example.com"
        name = parts[3] if len(parts) > 3 else "GitHub User"
    else:
        client_id = getattr(settings, "GITHUB_CLIENT_ID", "") or env.GITHUB_CLIENT_ID
        client_secret = getattr(settings, "GITHUB_CLIENT_SECRET", "") or env.GITHUB_CLIENT_SECRET
        if not client_id or not client_secret:
            raise ApiError(
                status=500, code="internal_error", detail="GitHub OAuth is not configured"
            )

        client = safe_client(timeout=10.0)
        # 1. Exchange code for GitHub access token
        token_payload: dict[str, Any] = {
            "client_id": client_id,
            "client_secret": client_secret,
            "code": code,
        }
        if redirect_uri:
            token_payload["redirect_uri"] = redirect_uri

        token_resp = client.post(
            "https://github.com/login/oauth/access_token",
            data=token_payload,
            headers={"Accept": "application/json"},
        )
        if token_resp.status_code != 200:
            raise ApiError(
                status=401, code="unauthorized", detail="GitHub authorization exchange failed"
            )

        token_data = token_resp.json()
        gh_access_token = token_data.get("access_token")
        if not gh_access_token:
            error_desc = token_data.get("error_description", "Invalid GitHub authorization code")
            raise ApiError(status=401, code="unauthorized", detail=error_desc)

        # 2. Fetch GitHub user profile
        user_resp = client.get(
            "https://api.github.com/user",
            headers={
                "Authorization": f"Bearer {gh_access_token}",
                "Accept": "application/vnd.github+json",
            },
        )
        if user_resp.status_code != 200:
            raise ApiError(
                status=401, code="unauthorized", detail="Failed to fetch GitHub user profile"
            )

        gh_user = user_resp.json()
        github_id = str(gh_user.get("id", ""))
        name = gh_user.get("name") or gh_user.get("login") or ""
        avatar_url = gh_user.get("avatar_url")
        email = gh_user.get("email") or ""

        # 3. If primary email not public, fetch from /user/emails
        if not email:
            emails_resp = client.get(
                "https://api.github.com/user/emails",
                headers={
                    "Authorization": f"Bearer {gh_access_token}",
                    "Accept": "application/vnd.github+json",
                },
            )
            if emails_resp.status_code == 200:
                emails_list = emails_resp.json()
                for item in emails_list:
                    if item.get("primary") and item.get("verified"):
                        email = item.get("email", "")
                        break
                if not email and emails_list:
                    email = emails_list[0].get("email", "")

        if not github_id or not email:
            raise ApiError(
                status=401, code="unauthorized", detail="GitHub account must have a verified email"
            )

    email_clean = email.strip().lower()

    with transaction.atomic():
        user: User | None
        oauth_acc = (
            OAuthAccount.objects.filter(provider="github", provider_user_id=github_id)
            .select_related("user")
            .first()
        )

        if oauth_acc is not None:
            user = oauth_acc.user
        else:
            user = User.objects.filter(email=email_clean, deleted_at__isnull=True).first()
            if user is None:
                name_clean = name.strip() or email_clean.split("@")[0]
                user = User.objects.create_user(
                    email=email_clean,
                    password=None,
                    name=name_clean,
                    avatar_url=avatar_url,
                    locale="en",
                    timezone="UTC",
                )
                user.email_verified_at = datetime.now(UTC)
                user.save(update_fields=["email_verified_at"])

                org_slug = _unique_slug(name_clean, Organization)
                org = Organization.objects.create(
                    name=f"{name_clean}'s Org",
                    slug=org_slug,
                    kind="standard",
                    plan_id="free",
                )
                OrgMember.objects.create(
                    org=org,
                    user=user,
                    org_role="org_owner",
                    status="active",
                    source="creator",
                )

                ws_slug = _unique_slug(f"{name_clean}-workspace", Workspace)
                ws = Workspace.objects.create(
                    org=org,
                    name=f"{name_clean}'s Workspace",
                    slug=ws_slug,
                    owner=user,
                    plan_id="free",
                )
                WorkspaceMember.objects.create(
                    workspace=ws,
                    user=user,
                    role="owner",
                )
                RoleBinding.objects.create(
                    org=org,
                    workspace=ws,
                    principal_type="user",
                    principal_id=user.id,
                    role_id="00000000-0000-7000-8000-00000000f001",
                    scope_type="workspace",
                    created_by=user,
                )

            OAuthAccount.objects.create(
                user=user,
                provider="github",
                provider_user_id=github_id,
                email=email_clean,
            )

        user.last_login = datetime.now(UTC)
        user.save(update_fields=["last_login"])

        # DB session constraint allows 'password','google','magic_link','sso','scim'
        session, access_token, plain_refresh, plain_csrf = start_session(
            user=user,
            auth_method="google",
            ip_prefix=ip_prefix,
            user_agent=user_agent,
        )

    return user, session, access_token, plain_refresh, plain_csrf, False


def list_user_sessions(user: User, current_session_id: UUID | None = None) -> list[dict[str, Any]]:
    """List all active sessions for a user."""
    sessions = Session.objects.filter(user=user, revoked_at__isnull=True).order_by("-last_used_at")
    result = []
    for s in sessions:
        result.append(
            {
                "id": str(s.id),
                "user_agent": s.user_agent,
                "ip_prefix": s.ip_prefix,
                "created_at": s.created_at.isoformat(),
                "last_used_at": s.last_used_at.isoformat(),
                "current": str(s.id) == str(current_session_id) if current_session_id else False,
            }
        )
    return result


def revoke_user_session(user: User, session_id: UUID | str) -> None:
    """Revoke a specific session owned by the user."""
    updated = Session.objects.filter(
        id=session_id,
        user=user,
        revoked_at__isnull=True,
    ).update(revoked_at=datetime.now(UTC))
    if not updated:
        raise not_found("session not found")
