import os
import stripe
from django.conf import settings
from django.utils import timezone
from django.views import View
from django.http import HttpResponse, JsonResponse
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import User
from api.utils.auth import require_auth

PLAN_PRICES = {
    "starter": os.environ.get("STRIPE_PRICE_STARTER", ""),
    "pro": os.environ.get("STRIPE_PRICE_PRO", ""),
    "enterprise": os.environ.get("STRIPE_PRICE_ENTERPRISE", ""),
}


def _get_stripe():
    key = getattr(settings, "STRIPE_SECRET_KEY", "")
    if not key:
        return None
    stripe.api_key = key
    return stripe


class CheckoutView(APIView):
    @require_auth
    def post(self, request):
        s = _get_stripe()
        if not s:
            return Response({"success": False, "error": "Stripe not configured"}, status=500)
        plan = request.data.get("plan")
        if plan not in ("starter", "pro", "enterprise"):
            return Response({"success": False, "error": "Invalid plan"}, status=400)
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        customer_id = user.stripe_customer_id
        if not customer_id:
            customer = s.customers.create(email=user.email, name=user.name or "")
            customer_id = customer.id
            User.objects.filter(pk=user.pk).update(stripe_customer_id=customer_id)

        price_id = PLAN_PRICES.get(plan)
        if not price_id:
            return Response({"success": False, "error": "Plan price not configured"}, status=400)

        app_base_url = getattr(settings, "APP_BASE_URL", "http://localhost:3000")
        session = s.checkout.Session.create(
            customer=customer_id,
            mode="subscription",
            payment_method_types=["card"],
            line_items=[{"price": price_id, "quantity": 1}],
            success_url=f"{app_base_url}/dashboard/settings?upgraded=1",
            cancel_url=f"{app_base_url}/pricing",
            metadata={"userId": str(user.id), "plan": plan},
        )
        return Response({"success": True, "data": {"checkout_url": session.url}})


class SubscriptionView(APIView):
    @require_auth
    def get(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": {
            "plan": user.plan,
            "plan_expires_at": user.plan_expires_at.isoformat() if user.plan_expires_at else None,
            "subscription_status": user.subscription_status,
            "subscription_ends_at": user.subscription_ends_at.isoformat() if user.subscription_ends_at else None,
            "stripe_customer_id": user.stripe_customer_id,
            "stripe_subscription_id": user.stripe_subscription_id,
        }})


class CancelView(APIView):
    @require_auth
    def post(self, request):
        s = _get_stripe()
        if not s:
            return Response({"success": False, "error": "Stripe not configured"}, status=500)
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        if not user.stripe_subscription_id:
            return Response({"success": False, "error": "No active subscription"}, status=400)
        s.Subscription.cancel(user.stripe_subscription_id)
        User.objects.filter(pk=user.pk).update(subscription_status="canceled")
        return Response({"success": True, "data": {"message": "Subscription canceled"}})


class PortalView(APIView):
    @require_auth
    def post(self, request):
        s = _get_stripe()
        if not s:
            return Response({"success": False, "error": "Stripe not configured"}, status=500)
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        if not user.stripe_customer_id:
            return Response({"success": False, "error": "No billing account found"}, status=400)
        app_base_url = getattr(settings, "APP_BASE_URL", "http://localhost:3000")
        session = s.billing_portal.Session.create(
            customer=user.stripe_customer_id,
            return_url=f"{app_base_url}/dashboard/settings",
        )
        return Response({"success": True, "data": {"portal_url": session.url}})


class WebhookView(View):
    """Raw body Stripe webhook — must bypass DRF/CSRF."""
    def post(self, request):
        s = _get_stripe()
        webhook_secret = getattr(settings, "STRIPE_WEBHOOK_SECRET", "")
        if not s or not webhook_secret:
            return HttpResponse(status=400)
        sig = request.META.get("HTTP_STRIPE_SIGNATURE", "")
        try:
            event = s.Webhook.construct_event(request.body, sig, webhook_secret)
        except Exception as e:
            return HttpResponse(f"Webhook Error: {e}", status=400)

        ev_type = event["type"]
        if ev_type == "checkout.session.completed":
            sess = event["data"]["object"]
            meta = sess.get("metadata") or {}
            user_id = meta.get("userId")
            plan = meta.get("plan")
            if user_id and plan:
                User.objects.filter(id=user_id).update(
                    plan=plan,
                    subscription_status="active",
                    stripe_subscription_id=sess.get("subscription"),
                )
        elif ev_type == "customer.subscription.deleted":
            sub = event["data"]["object"]
            User.objects.filter(stripe_subscription_id=sub["id"]).update(
                plan="free", subscription_status="canceled", stripe_subscription_id=None
            )
        elif ev_type == "customer.subscription.updated":
            sub = event["data"]["object"]
            User.objects.filter(stripe_subscription_id=sub["id"]).update(
                subscription_status=sub["status"]
            )

        return JsonResponse({"received": True})
