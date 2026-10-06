"""Aliases for the pre-Django API surface.

The dashboard was written against a route table that the Django rewrite
renamed, so a dozen pages were calling URLs that 404ed. Rather than rewrite
the client and strand any existing integration, the old paths are kept and
forwarded to the current handlers.
"""

from rest_framework.response import Response
from rest_framework.views import APIView

from api.views.analytics import DashboardView
from api.views.apikey import ApiKeyRegenerateView, ApiKeyView
from api.views.auth import MeView
from api.views.billing import CancelView, CheckoutView, PortalView, SubscriptionView


class UserProfileView(MeView):
    """`/user/profile` -> `/auth/me`"""


class AnalyticsSummaryView(DashboardView):
    """`/analytics/summary` -> `/analytics/dashboard`"""


class LegacyApiKeyView(ApiKeyView):
    """`/api/key` -> `/apikey`"""


class LegacyApiKeyRegenerateView(ApiKeyRegenerateView):
    """`/api/key/regenerate` -> `/apikey/regenerate`"""


class PaymentsCheckoutView(CheckoutView):
    """`/payments/checkout` -> `/billing/checkout`"""


class PaymentsCancelView(CancelView):
    """`/payments/cancel` -> `/billing/cancel`"""


class PaymentsSubscriptionView(SubscriptionView):
    """`/payments/subscription` -> `/billing/subscription`"""


class PaymentsPortalView(PortalView):
    """`/payments/portal` -> `/billing/portal`"""


from django.conf import settings
from django.utils import timezone

from api.models import User
from api.utils import oauth


class GoogleAuthView(APIView):
    """Sign in with a Google ID token (Google Identity Services / One Tap).

    The redirect flow under /auth/oauth/google/* is the primary path; this one
    stays for clients that already hold a GIS credential. It applies the same
    verification and account-linking rules: RS256 against Google's keys, our
    client id as audience, Google as issuer, and a verified email.
    """

    def post(self, request):
        credential = request.data.get("id_token") or request.data.get("credential") or ""
        if not credential:
            return Response({"success": False, "error": "Google ID token is required"}, status=400)
        if not settings.GOOGLE_CLIENT_ID:
            return Response(
                {"success": False,
                 "error": "Google sign-in is not configured on this deployment.",
                 "code": "not_configured"},
                status=501,
            )
        try:
            claims = oauth.verify_google_id_token(credential)
            result = oauth.resolve_sign_in(oauth.google_profile_from_claims(claims))
        except oauth.OAuthError as exc:
            status = {"invalid_token": 401, "provider_unavailable": 503}.get(exc.code, 400)
            messages = {
                "invalid_token": "Invalid Google ID token",
                "email_missing": "Your Google account did not share an email address.",
                "email_unverified": "Your Google email address is not verified.",
                "provider_already_linked": "This account is already connected to a different Google account.",
                "provider_unavailable": "Google could not be reached. Please try again.",
            }
            return Response(
                {"success": False, "error": messages.get(exc.code, "Google sign-in failed"), "code": exc.code},
                status=status,
            )

        from api.utils.auth import sign_tokens
        from api.utils.ip import get_client_ip
        from api.views.oauth import user_payload

        user = result.user
        User.objects.filter(pk=user.pk).update(
            last_login_at=timezone.now(), last_login_ip=get_client_ip(request)[:45],
        )
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": user_payload(user),
            **tokens,
            "is_new_user": result.is_new_user,
            "password_reset": result.password_reset,
        }})
