import uuid
import jwt as pyjwt
from datetime import timedelta
from django.conf import settings
from django.db import transaction
from django.db.models import Q
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import (
    User, RefreshToken, LoginAttempt, PasswordResetToken, EmailVerification,
)
from api.utils.auth import (
    hash_password, check_password, sign_tokens, decode_token, require_auth,
    generate_api_key, normalize_email, validate_password_strength,
    generate_secure_token, hash_token, rotate_refresh_token,
    revoke_all_user_tokens,
)
from api.utils.email import send_password_reset_email, send_verification_email
from api.utils.ip import get_client_ip


def _check_ip_rate_limit(ip: str, window_minutes: int, max_attempts: int, email: str = None) -> bool:
    """Return True if the IP (and optional email) has exceeded the rate limit."""
    since = timezone.now() - timedelta(minutes=window_minutes)
    ip_count = LoginAttempt.objects.filter(ip_address=ip, created_at__gte=since).count()
    if ip_count >= max_attempts:
        return True
    if email:
        email_count = LoginAttempt.objects.filter(
            email=normalize_email(email), created_at__gte=since
        ).count()
        if email_count >= max_attempts:
            return True
    return False


def _record_login_attempt(email: str, ip: str, success: bool, reason: str = None, user_agent: str = None):
    LoginAttempt.objects.create(
        email=normalize_email(email),
        ip_address=ip,
        success=success,
        failure_reason=reason,
        user_agent=(user_agent or "")[:500],
    )


class RegisterView(APIView):
    def post(self, request):
        email = normalize_email(request.data.get("email", ""))
        password = request.data.get("password", "")
        name = request.data.get("name", "").strip()
        ip = get_client_ip(request)

        if not email or not password:
            return Response({"success": False, "error": "Email and password required"}, status=400)

        # Rate limit registrations
        since = timezone.now() - timedelta(hours=1)
        reg_count = LoginAttempt.objects.filter(
            ip_address=ip, created_at__gte=since, success=True,
            failure_reason="register"
        ).count()
        if reg_count >= settings.AUTH_MAX_REGISTER_PER_IP_PER_HOUR:
            return Response(
                {"success": False, "error": "Too many registration attempts. Please try again later."},
                status=429,
            )

        # Validate password strength
        password_errors = validate_password_strength(password)
        if password_errors:
            return Response(
                {"success": False, "error": password_errors[0], "errors": password_errors},
                status=400,
            )

        if User.objects.filter(email=email).exists():
            return Response({"success": False, "error": "Email already registered"}, status=409)

        user = User(
            id=uuid.uuid4(),
            email=email,
            password_hash=hash_password(password),
            name=name,
            api_key=generate_api_key(),
            api_calls_reset_at=timezone.now().date(),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        user.save(force_insert=True)

        # Record successful registration
        _record_login_attempt(email, ip, True, reason="register")

        # Send verification email
        try:
            token = generate_secure_token()
            EmailVerification.objects.create(
                user=user,
                token_hash=hash_token(token),
                expires_at=timezone.now() + timedelta(
                    hours=settings.AUTH_VERIFICATION_TOKEN_EXPIRY_HOURS
                ),
            )
            send_verification_email(email, name, token)
        except Exception:
            pass  # Don't block registration if email fails

        # Auto-login: issue tokens immediately
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": {
                "id": str(user.id), "email": user.email, "name": user.name,
                "plan": user.plan, "email_verified": user.email_verified,
            },
            **tokens,
        }}, status=201)


class LoginView(APIView):
    def post(self, request):
        email = normalize_email(request.data.get("email", ""))
        password = request.data.get("password", "")
        ip = get_client_ip(request)
        ua = request.headers.get("User-Agent", "")

        if not email or not password:
            return Response({"success": False, "error": "Email and password required"}, status=400)

        # IP-level rate limiting
        if _check_ip_rate_limit(ip, 15, settings.AUTH_MAX_LOGIN_PER_IP_PER_15MIN, email):
            return Response(
                {"success": False, "error": "Too many login attempts. Please try again later.",
                 "code": "rate_limited"},
                status=429,
            )

        try:
            user = User.objects.get(email=email)
        except User.DoesNotExist:
            _record_login_attempt(email, ip, False, reason="user_not_found", user_agent=ua)
            return Response({"success": False, "error": "Invalid email or password"}, status=401)

        # Check account lockout
        if user.locked_until and user.locked_until > timezone.now():
            remaining = int((user.locked_until - timezone.now()).total_seconds() / 60) + 1
            _record_login_attempt(email, ip, False, reason="account_locked", user_agent=ua)
            return Response(
                {"success": False,
                 "error": f"Account temporarily locked. Try again in {remaining} minute(s).",
                 "code": "account_locked",
                 "locked_until": user.locked_until.isoformat()},
                status=423,
            )

        if not check_password(password, user.password_hash):
            # Increment failed attempts
            user.failed_login_attempts = (user.failed_login_attempts or 0) + 1
            update_fields = ["failed_login_attempts"]
            if user.failed_login_attempts >= settings.AUTH_MAX_LOGIN_ATTEMPTS:
                user.locked_until = timezone.now() + timedelta(
                    minutes=settings.AUTH_LOCKOUT_DURATION_MINUTES
                )
                update_fields.append("locked_until")
            user.save(update_fields=update_fields)
            _record_login_attempt(email, ip, False, reason="wrong_password", user_agent=ua)

            remaining_attempts = max(
                0, settings.AUTH_MAX_LOGIN_ATTEMPTS - user.failed_login_attempts
            )
            error_msg = "Invalid email or password"
            if remaining_attempts <= 2 and remaining_attempts > 0:
                error_msg += f" ({remaining_attempts} attempt(s) remaining)"
            elif remaining_attempts == 0:
                error_msg = f"Account locked for {settings.AUTH_LOCKOUT_DURATION_MINUTES} minutes due to too many failed attempts"

            return Response({"success": False, "error": error_msg}, status=401)

        # Success — reset counters
        user.failed_login_attempts = 0
        user.locked_until = None
        user.last_login_at = timezone.now()
        user.last_login_ip = ip
        user.save(update_fields=[
            "failed_login_attempts", "locked_until", "last_login_at", "last_login_ip",
        ])
        _record_login_attempt(email, ip, True, user_agent=ua)

        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": {
                "id": str(user.id), "email": user.email, "name": user.name,
                "plan": user.plan, "avatar_url": user.avatar_url,
                "email_verified": user.email_verified,
            },
            **tokens,
        }})


class RefreshView(APIView):
    def post(self, request):
        token = (
            request.data.get("refresh_token")
            or request.data.get("refreshToken")
            or ""
        )
        if not token:
            return Response({"success": False, "error": "Refresh token required"}, status=400)
        try:
            payload = decode_token(token)
            if payload.get("type") != "refresh":
                return Response({"success": False, "error": "Invalid token type"}, status=401)

            user = User.objects.get(id=payload["sub"])
            jti = payload.get("jti")
            if not jti:
                return Response({"success": False, "error": "Invalid token format"}, status=401)

            with transaction.atomic():
                new_tokens = rotate_refresh_token(
                    old_jti=jti,
                    user_id=str(user.id),
                    email=user.email,
                    plan=user.plan or "free",
                )
            if new_tokens is None:
                return Response(
                    {"success": False, "error": "Token has been revoked. Please sign in again.",
                     "code": "token_revoked"},
                    status=401,
                )
            return Response({"success": True, "data": new_tokens})

        except (pyjwt.ExpiredSignatureError,):
            return Response({"success": False, "error": "Refresh token expired. Please sign in again."}, status=401)
        except (pyjwt.InvalidTokenError, User.DoesNotExist):
            return Response({"success": False, "error": "Invalid or expired refresh token"}, status=401)


class LogoutView(APIView):
    @require_auth
    def post(self, request):
        refresh_token = (
            request.data.get("refresh_token")
            or request.data.get("refreshToken")
            or ""
        )
        all_devices = request.data.get("all_devices", False)

        if all_devices:
            revoke_all_user_tokens(request.auth_user["id"])
        elif refresh_token:
            try:
                payload = decode_token(refresh_token)
                jti = payload.get("jti")
                if jti:
                    RefreshToken.objects.filter(jti=jti).update(revoked=True)
            except pyjwt.InvalidTokenError:
                pass  # Token is already invalid, that's fine

        return Response({"success": True, "data": {"message": "Logged out"}})


class MeView(APIView):
    @require_auth
    def get(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": {
            "id": str(user.id), "email": user.email, "name": user.name,
            "company": user.company, "avatar_url": user.avatar_url,
            "plan": user.plan, "plan_expires_at": user.plan_expires_at,
            "subscription_status": user.subscription_status, "created_at": user.created_at,
            "email_verified": user.email_verified,
        }})

    @require_auth
    def put(self, request):
        update_kwargs = {"updated_at": timezone.now()}
        # Only update fields that are explicitly provided
        if "name" in request.data:
            update_kwargs["name"] = request.data["name"]
        if "company" in request.data:
            update_kwargs["company"] = request.data["company"]
        if "avatar_url" in request.data:
            update_kwargs["avatar_url"] = request.data["avatar_url"]

        User.objects.filter(id=request.auth_user["id"]).update(**update_kwargs)
        user = User.objects.get(id=request.auth_user["id"])
        return Response({"success": True, "data": {
            "id": str(user.id), "email": user.email, "name": user.name,
            "company": user.company, "avatar_url": user.avatar_url, "plan": user.plan,
        }})


class ChangePasswordView(APIView):
    @require_auth
    def post(self, request):
        current = request.data.get("current_password") or request.data.get("currentPassword") or ""
        new_pass = request.data.get("new_password") or request.data.get("newPassword") or ""
        if not current or not new_pass:
            return Response({"success": False, "error": "Current and new password required"}, status=400)

        # Validate new password strength
        password_errors = validate_password_strength(new_pass)
        if password_errors:
            return Response(
                {"success": False, "error": password_errors[0], "errors": password_errors},
                status=400,
            )

        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        if not check_password(current, user.password_hash):
            return Response({"success": False, "error": "Current password is incorrect"}, status=400)

        User.objects.filter(pk=user.pk).update(
            password_hash=hash_password(new_pass),
            password_changed_at=timezone.now(),
            updated_at=timezone.now(),
        )
        # Revoke all existing refresh tokens — force re-login on other devices
        revoke_all_user_tokens(str(user.id))
        # Issue fresh tokens for the current session
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "message": "Password updated. Other sessions have been signed out.",
            **tokens,
        }})


class ForgotPasswordView(APIView):
    """Request a password reset link.

    Always returns success to prevent email enumeration.
    """

    def post(self, request):
        email = normalize_email(request.data.get("email") or "")
        if not email:
            return Response(
                {"success": False, "error": "Email is required"}, status=400
            )

        ip = get_client_ip(request)
        # Rate limit: max 3 reset requests per email per hour
        since = timezone.now() - timedelta(hours=1)
        recent = PasswordResetToken.objects.filter(
            user__email=email, created_at__gte=since
        ).count()
        if recent >= 3:
            # Still return success to prevent enumeration
            return Response({"success": True, "data": {
                "message": "If an account exists with that email, a password reset link has been sent."
            }})

        try:
            user = User.objects.get(email=email)
        except User.DoesNotExist:
            # Return success to prevent email enumeration
            return Response({"success": True, "data": {
                "message": "If an account exists with that email, a password reset link has been sent."
            }})

        # Invalidate previous tokens
        PasswordResetToken.objects.filter(user=user, used_at__isnull=True).update(
            used_at=timezone.now()
        )

        token = generate_secure_token()
        PasswordResetToken.objects.create(
            user=user,
            token_hash=hash_token(token),
            expires_at=timezone.now() + timedelta(
                hours=settings.AUTH_RESET_TOKEN_EXPIRY_HOURS
            ),
        )

        try:
            send_password_reset_email(user.email, user.name or "", token)
        except Exception:
            pass  # Don't leak whether send succeeded

        return Response({"success": True, "data": {
            "message": "If an account exists with that email, a password reset link has been sent."
        }})


class ResetPasswordView(APIView):
    """Reset a password using a token from the email link."""

    def post(self, request):
        token = request.data.get("token", "")
        new_password = request.data.get("password", "")

        if not token or not new_password:
            return Response(
                {"success": False, "error": "Token and new password are required"},
                status=400,
            )

        # Validate password strength
        password_errors = validate_password_strength(new_password)
        if password_errors:
            return Response(
                {"success": False, "error": password_errors[0], "errors": password_errors},
                status=400,
            )

        token_hash = hash_token(token)
        try:
            reset_token = PasswordResetToken.objects.select_related("user").get(
                token_hash=token_hash,
                used_at__isnull=True,
                expires_at__gt=timezone.now(),
            )
        except PasswordResetToken.DoesNotExist:
            return Response(
                {"success": False, "error": "Invalid or expired reset link. Please request a new one.",
                 "code": "invalid_token"},
                status=400,
            )

        user = reset_token.user
        with transaction.atomic():
            User.objects.filter(pk=user.pk).update(
                password_hash=hash_password(new_password),
                password_changed_at=timezone.now(),
                updated_at=timezone.now(),
                failed_login_attempts=0,
                locked_until=None,
            )
            reset_token.used_at = timezone.now()
            reset_token.save(update_fields=["used_at"])
            # Revoke all refresh tokens — force re-login on all devices
            revoke_all_user_tokens(str(user.id))

        return Response({"success": True, "data": {
            "message": "Password has been reset. Please sign in with your new password."
        }})


class VerifyEmailView(APIView):
    """Verify email address using a token from the verification link."""

    def post(self, request):
        token = request.data.get("token", "")
        if not token:
            return Response({"success": False, "error": "Verification token required"}, status=400)

        token_hash = hash_token(token)
        try:
            verification = EmailVerification.objects.select_related("user").get(
                token_hash=token_hash,
                verified_at__isnull=True,
                expires_at__gt=timezone.now(),
            )
        except EmailVerification.DoesNotExist:
            return Response(
                {"success": False, "error": "Invalid or expired verification link.",
                 "code": "invalid_token"},
                status=400,
            )

        now = timezone.now()
        verification.verified_at = now
        verification.save(update_fields=["verified_at"])
        User.objects.filter(pk=verification.user.pk).update(
            email_verified=True, email_verified_at=now,
        )
        return Response({"success": True, "data": {"message": "Email verified successfully."}})


class ResendVerificationView(APIView):
    @require_auth
    def post(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        if user.email_verified:
            return Response({"success": True, "data": {"message": "Email already verified."}})

        # Rate limit: 1 per minute
        since = timezone.now() - timedelta(minutes=1)
        recent = EmailVerification.objects.filter(
            user=user, created_at__gte=since
        ).count()
        if recent >= 1:
            return Response(
                {"success": False, "error": "Please wait a minute before requesting another verification email."},
                status=429,
            )

        token = generate_secure_token()
        EmailVerification.objects.create(
            user=user,
            token_hash=hash_token(token),
            expires_at=timezone.now() + timedelta(
                hours=settings.AUTH_VERIFICATION_TOKEN_EXPIRY_HOURS
            ),
        )
        try:
            send_verification_email(user.email, user.name or "", token)
        except Exception:
            return Response(
                {"success": False, "error": "Failed to send verification email. Please try again."},
                status=500,
            )

        return Response({"success": True, "data": {"message": "Verification email sent."}})
