"""API views for authentication, sessions, profile, and OAuth endpoints.

Plan §5.2, §7.1, §8:
- POST /v1/auth/register
- POST /v1/auth/login
- POST /v1/auth/logout
- POST /v1/auth/refresh
- POST /v1/auth/verify-email
- POST /v1/auth/resend-verification
- POST /v1/auth/password/forgot & /v1/auth/forgot-password
- POST /v1/auth/password/reset & /v1/auth/reset-password
- POST /v1/auth/google
- POST /v1/auth/github
- GET, PATCH /v1/me
- POST /v1/me/password
- GET /v1/me/sessions
- DELETE /v1/me/sessions/<id>
"""

from typing import Any
from uuid import UUID

from drf_spectacular.utils import extend_schema, inline_serializer
from rest_framework import serializers, status
from rest_framework.request import Request
from rest_framework.response import Response

from apps.core.errors import ApiError, forbidden, unprocessable
from apps.core.net import ip_key, ip_prefix
from apps.core.views import AuthenticatedAPIView, PublicAPIView
from apps.workspaces.models import WorkspaceMember

from .authentication import Principal
from .cookies import (
    REFRESH_COOKIE_NAME,
    clear_auth_cookies,
    extract_access_token,
    set_auth_cookies,
)
from .models import Session, User
from .serializers import (
    AuthResponseSerializer,
    ChangePasswordRequestSerializer,
    ForgotPasswordRequestSerializer,
    GitHubAuthRequestSerializer,
    GoogleAuthRequestSerializer,
    LoginRequestSerializer,
    MeResponseSerializer,
    RefreshResponseSerializer,
    RefreshTokenRequestSerializer,
    RegisterRequestSerializer,
    RegisterResponseSerializer,
    ResetPasswordRequestSerializer,
    SessionDTOSerializer,
    UpdateMeRequestSerializer,
    UserDTOSerializer,
    VerifyEmailRequestSerializer,
    WorkspaceDTOSerializer,
)
from .services import (
    change_password,
    forgot_password,
    github_login,
    google_login,
    list_user_sessions,
    login,
    logout,
    refresh_session,
    register,
    resend_verification,
    reset_password,
    revoke_user_session,
    update_profile,
    verify_email,
)
from .tokens import get_token_manager, hash_token


def _user(request: Request) -> User:
    """The signed-in user. AuthenticatedAPIView has already rejected anonymous callers."""
    user = request.user
    if not isinstance(user, User):
        raise forbidden(code="unauthorized", detail="Authentication credentials required")
    return user


def _principal(request: Request) -> Principal:
    principal = request.auth
    if not isinstance(principal, Principal):
        raise forbidden(code="unauthorized", detail="Authentication credentials required")
    return principal


def _flag(name: str, field: str) -> Any:
    return inline_serializer(name, {field: serializers.BooleanField()})


class RegisterView(PublicAPIView):
    """Register a new user account with default organization and workspace."""

    @extend_schema(request=RegisterRequestSerializer, responses={201: RegisterResponseSerializer})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = RegisterRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        user, ws, session, access_token, refresh_token, csrf_token = register(
            email=serializer.validated_data["email"],
            password=serializer.validated_data["password"],
            name=serializer.validated_data.get("name", ""),
            ip_key=ip_key(request),
            ip_prefix=ip_prefix(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
        )

        resp_data = {
            "user": UserDTOSerializer(user).data,
            "workspace": WorkspaceDTOSerializer(ws).data,
            "token": access_token,
            "access_token": access_token,
            "refresh_token": refresh_token,
        }
        response = Response(resp_data, status=status.HTTP_201_CREATED)
        set_auth_cookies(response, access_token, refresh_token, csrf_token)
        return response


class LoginView(PublicAPIView):
    """Authenticate with email and password."""

    @extend_schema(request=LoginRequestSerializer, responses={200: AuthResponseSerializer})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = LoginRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        user, session, access_token, refresh_token, csrf_token, mfa_required = login(
            email=serializer.validated_data["email"],
            password=serializer.validated_data["password"],
            ip_key=ip_key(request),
            ip_prefix=ip_prefix(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
        )

        resp_data = {
            "user": UserDTOSerializer(user).data,
            "token": access_token,
            "access_token": access_token,
            "refresh_token": refresh_token,
            "mfa_required": mfa_required,
        }
        response = Response(resp_data, status=status.HTTP_200_OK)
        set_auth_cookies(response, access_token, refresh_token, csrf_token)
        return response


class GoogleAuthView(PublicAPIView):
    """Sign in or register with Google OAuth ID token."""

    @extend_schema(request=GoogleAuthRequestSerializer, responses={200: AuthResponseSerializer})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = GoogleAuthRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        token = serializer.validated_data["token"]
        user, session, access_token, refresh_token, csrf_token, mfa_required = google_login(
            credential=token,
            ip_prefix=ip_prefix(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
        )

        resp_data = {
            "user": UserDTOSerializer(user).data,
            "token": access_token,
            "access_token": access_token,
            "refresh_token": refresh_token,
            "mfa_required": mfa_required,
        }
        response = Response(resp_data, status=status.HTTP_200_OK)
        set_auth_cookies(response, access_token, refresh_token, csrf_token)
        return response


class GitHubAuthView(PublicAPIView):
    """Sign in or register with GitHub OAuth authorization code."""

    @extend_schema(request=GitHubAuthRequestSerializer, responses={200: AuthResponseSerializer})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = GitHubAuthRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        code = serializer.validated_data["code"]
        redirect_uri = serializer.validated_data.get("redirect_uri")

        user, session, access_token, refresh_token, csrf_token, mfa_required = github_login(
            code=code,
            redirect_uri=redirect_uri,
            ip_prefix=ip_prefix(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
        )

        resp_data = {
            "user": UserDTOSerializer(user).data,
            "token": access_token,
            "access_token": access_token,
            "refresh_token": refresh_token,
            "mfa_required": mfa_required,
        }
        response = Response(resp_data, status=status.HTTP_200_OK)
        set_auth_cookies(response, access_token, refresh_token, csrf_token)
        return response


class RefreshView(PublicAPIView):
    """Rotate session refresh token with family reuse detection."""

    @extend_schema(
        request=RefreshTokenRequestSerializer, responses={200: RefreshResponseSerializer}
    )
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        token_str = ""
        if REFRESH_COOKIE_NAME in request.COOKIES:
            token_str = request.COOKIES[REFRESH_COOKIE_NAME]
        if not token_str:
            token_str = request.headers.get("X-Refresh-Token") or request.META.get(
                "HTTP_X_REFRESH_TOKEN", ""
            )
        if not token_str and request.data:
            serializer = RefreshTokenRequestSerializer(data=request.data)
            if serializer.is_valid():
                token_str = serializer.validated_data.get("refresh_token", "")

        if not token_str:
            raise ApiError(status=401, code="unauthorized", detail="refresh token required")

        session, access_token, new_refresh_token, csrf_token = refresh_session(
            refresh_token_plain=token_str,
            ip_prefix=ip_prefix(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
        )

        resp_data = {
            "token": access_token,
            "access_token": access_token,
            "refresh_token": new_refresh_token,
        }
        response = Response(resp_data, status=status.HTTP_200_OK)
        set_auth_cookies(response, access_token, new_refresh_token, csrf_token)
        return response


class LogoutView(PublicAPIView):
    """Revoke session and clear authentication cookies."""

    @extend_schema(request=RefreshTokenRequestSerializer, responses={204: None})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        session_id = None
        auth_principal = getattr(request, "auth", None)
        if auth_principal is not None and getattr(auth_principal, "session_id", None):
            session_id = auth_principal.session_id

        if not session_id:
            token, _ = extract_access_token(request)
            if token:
                try:
                    claims = get_token_manager().verify_access_token(token)
                    session_id = claims.get("sid")
                except Exception:
                    pass

        if not session_id:
            ref_tok = None
            if hasattr(request, "COOKIES") and REFRESH_COOKIE_NAME in request.COOKIES:
                ref_tok = request.COOKIES[REFRESH_COOKIE_NAME]
            body = request.data
            if not ref_tok and isinstance(body, dict):
                ref_tok = body.get("refresh_token")
            if ref_tok:
                token_hash = hash_token(ref_tok)
                sess = Session.objects.filter(refresh_token_hash=token_hash).first()
                if sess:
                    session_id = sess.id

        if session_id:
            logout(session_id)

        response = Response(status=status.HTTP_204_NO_CONTENT)
        clear_auth_cookies(response)
        return response


class VerifyEmailView(PublicAPIView):
    """Verify email address with single-use token."""

    @extend_schema(
        request=VerifyEmailRequestSerializer,
        responses={200: _flag("VerifyEmailResponse", "verified")},
    )
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = VerifyEmailRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        verify_email(token_plain=serializer.validated_data["token"])
        return Response({"verified": True}, status=status.HTTP_200_OK)


class ResendVerificationView(AuthenticatedAPIView):
    """Resend verification email to authenticated user."""

    @extend_schema(request=None, responses={202: _flag("ResendVerificationResponse", "sent")})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        resend_verification(_user(request))
        return Response({"sent": True}, status=status.HTTP_202_ACCEPTED)


class ForgotPasswordView(PublicAPIView):
    """Request password reset link."""

    @extend_schema(
        request=ForgotPasswordRequestSerializer,
        responses={202: _flag("ForgotPasswordResponse", "accepted")},
    )
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = ForgotPasswordRequestSerializer(data=request.data)
        if not serializer.is_valid():
            return Response({"accepted": True}, status=status.HTTP_202_ACCEPTED)

        forgot_password(
            email=serializer.validated_data["email"],
            ip_key=ip_key(request),
        )
        return Response({"accepted": True}, status=status.HTTP_202_ACCEPTED)


class ResetPasswordView(PublicAPIView):
    """Reset password using verified email token."""

    @extend_schema(request=ResetPasswordRequestSerializer, responses={204: None})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = ResetPasswordRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        reset_password(
            token_plain=serializer.validated_data["token"],
            new_password=serializer.validated_data["password"],
        )
        return Response(status=status.HTTP_204_NO_CONTENT)


class MeView(AuthenticatedAPIView):
    """Current user profile and memberships."""

    @extend_schema(responses={200: MeResponseSerializer})
    def get(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        user = _user(request)
        memberships = WorkspaceMember.objects.filter(user=user).select_related("workspace")

        workspaces_data = []
        for m in memberships:
            ws_data = WorkspaceDTOSerializer(m.workspace).data
            ws_data["role"] = m.role
            workspaces_data.append(ws_data)

        # Organizations
        from apps.orgs.models import OrgMember

        org_memberships = OrgMember.objects.filter(user=user).select_related("org")
        orgs_data = [
            {
                "id": str(om.org.id),
                "name": om.org.name,
                "slug": str(om.org.slug),
                "role": om.org_role,
            }
            for om in org_memberships
        ]

        resp = {
            "user": UserDTOSerializer(user).data,
            "workspaces": workspaces_data,
            "organizations": orgs_data,
        }
        return Response(resp, status=status.HTTP_200_OK)

    @extend_schema(request=UpdateMeRequestSerializer, responses={200: UserDTOSerializer})
    def patch(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = UpdateMeRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        user = update_profile(
            user=_user(request),
            name=serializer.validated_data.get("name"),
            locale=serializer.validated_data.get("locale"),
            timezone_name=serializer.validated_data.get("timezone"),
        )
        return Response(UserDTOSerializer(user).data, status=status.HTTP_200_OK)


class ChangePasswordView(AuthenticatedAPIView):
    """Change current user's password and sign out other sessions."""

    @extend_schema(request=ChangePasswordRequestSerializer, responses={204: None})
    def post(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        serializer = ChangePasswordRequestSerializer(data=request.data)
        if not serializer.is_valid():
            raise unprocessable(
                code="invalid_input", detail="validation failed", errors=serializer.errors
            )

        change_password(
            user=_user(request),
            current_password=serializer.validated_data.get("current_password", ""),
            new_password=serializer.validated_data["new_password"],
            current_session_id=_principal(request).session_id,
        )
        return Response(status=status.HTTP_204_NO_CONTENT)


class SessionsListView(AuthenticatedAPIView):
    """List active sessions for current user."""

    @extend_schema(
        responses={
            200: inline_serializer("SessionListResponse", {"data": SessionDTOSerializer(many=True)})
        }
    )
    def get(self, request: Request, *args: Any, **kwargs: Any) -> Response:
        sessions = list_user_sessions(
            user=_user(request),
            current_session_id=_principal(request).session_id,
        )
        return Response({"data": sessions}, status=status.HTTP_200_OK)


class SessionRevokeView(AuthenticatedAPIView):
    """Revoke a specific session."""

    @extend_schema(responses={204: None})
    def delete(self, request: Request, id: UUID, *args: Any, **kwargs: Any) -> Response:
        revoke_user_session(user=_user(request), session_id=id)
        return Response(status=status.HTTP_204_NO_CONTENT)
