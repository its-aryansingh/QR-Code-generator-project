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

class GoogleAuthView(APIView):
    """Verify a Google ID token and login or create the user."""

    def post(self, request):
        id_token_str = request.data.get("id_token") or request.data.get("credential") or ""
        if not id_token_str:
            return Response(
                {"success": False, "error": "Google ID token is required"},
                status=400,
            )

        google_client_id = getattr(settings, "GOOGLE_CLIENT_ID", "")
        if not google_client_id:
            return Response(
                {"success": False,
                 "error": "Google sign-in is not configured on this deployment.",
                 "code": "google_auth_unavailable"},
                status=501,
            )

        try:
            from google.oauth2 import id_token as google_id_token
            from google.auth.transport import requests as google_requests

            idinfo = google_id_token.verify_oauth2_token(
                id_token_str,
                google_requests.Request(),
                google_client_id,
            )
        except ValueError:
            return Response(
                {"success": False, "error": "Invalid Google ID token"},
                status=401,
            )
        except Exception:
            return Response(
                {"success": False, "error": "Failed to verify Google token"},
                status=500,
            )

        email = (idinfo.get("email") or "").strip().lower()
        if not email:
            return Response(
                {"success": False, "error": "Google account has no email"},
                status=400,
            )

        name = idinfo.get("name", "")
        avatar = idinfo.get("picture", "")

        try:
            user = User.objects.get(email=email)
            # Update avatar if missing
            if avatar and not user.avatar_url:
                User.objects.filter(pk=user.pk).update(avatar_url=avatar)
        except User.DoesNotExist:
            import uuid as _uuid
            from api.utils.auth import generate_api_key, hash_password
            import secrets

            user = User(
                id=_uuid.uuid4(),
                email=email,
                password_hash=hash_password(secrets.token_urlsafe(32)),
                name=name,
                avatar_url=avatar,
                email_verified=True,
                email_verified_at=timezone.now(),
                api_key=generate_api_key(),
                api_calls_reset_at=timezone.now().date(),
                created_at=timezone.now(),
                updated_at=timezone.now(),
            )
            user.save(force_insert=True)

        # Update last login
        from api.utils.ip import get_client_ip
        User.objects.filter(pk=user.pk).update(
            last_login_at=timezone.now(),
            last_login_ip=get_client_ip(request),
        )

        from api.utils.auth import sign_tokens
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": {
                "id": str(user.id), "email": user.email, "name": user.name or name,
                "plan": user.plan, "avatar_url": user.avatar_url or avatar,
                "email_verified": True,
            },
            **tokens,
        }})
