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


class GoogleAuthView(APIView):
    """The login page offers a Google button but no exchange endpoint was ever
    implemented. Return an explicit, actionable error instead of a 404 so the
    UI can tell the user what is missing."""

    def post(self, request):
        return Response(
            {
                "success": False,
                "error": "Google sign-in is not configured on this deployment. "
                         "Use email and password, or set GOOGLE_CLIENT_ID on the server.",
                "code": "google_auth_unavailable",
            },
            status=501,
        )
