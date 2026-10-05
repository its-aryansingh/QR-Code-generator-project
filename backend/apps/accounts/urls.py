"""URL configuration for accounts and auth endpoints."""

from django.urls import path

from .views import (
    ChangePasswordView,
    ForgotPasswordView,
    GitHubAuthView,
    GoogleAuthView,
    LoginView,
    LogoutView,
    MeView,
    RefreshView,
    RegisterView,
    ResendVerificationView,
    ResetPasswordView,
    SessionRevokeView,
    SessionsListView,
    VerifyEmailView,
)

urlpatterns = [
    path("auth/register", RegisterView.as_view(), name="auth-register"),
    path("auth/login", LoginView.as_view(), name="auth-login"),
    path("auth/logout", LogoutView.as_view(), name="auth-logout"),
    path("auth/refresh", RefreshView.as_view(), name="auth-refresh"),
    path("auth/verify-email", VerifyEmailView.as_view(), name="auth-verify-email"),
    path(
        "auth/resend-verification",
        ResendVerificationView.as_view(),
        name="auth-resend-verification",
    ),
    path("auth/password/forgot", ForgotPasswordView.as_view(), name="auth-password-forgot"),
    path("auth/forgot-password", ForgotPasswordView.as_view(), name="auth-forgot-password-alias"),
    path("auth/password/reset", ResetPasswordView.as_view(), name="auth-password-reset"),
    path("auth/reset-password", ResetPasswordView.as_view(), name="auth-reset-password-alias"),
    path("auth/google", GoogleAuthView.as_view(), name="auth-google"),
    path("auth/github", GitHubAuthView.as_view(), name="auth-github"),
    path("me", MeView.as_view(), name="me"),
    path("me/password", ChangePasswordView.as_view(), name="me-password"),
    path("me/sessions", SessionsListView.as_view(), name="me-sessions"),
    path("me/sessions/<uuid:id>", SessionRevokeView.as_view(), name="me-session-revoke"),
]
