"""Plan entitlements.

One table in settings drives both the API gates and the flags the dashboard
reads, so a disabled feature renders as a locked upgrade card instead of a
failed request.
"""

from django.conf import settings
from rest_framework.response import Response

FEATURE_LABELS = {
    "bulk": "Bulk generation",
    "webhooks": "Webhooks",
    "campaigns": "Campaigns",
    "templates": "Brand templates",
    "routing": "Smart routing",
    "audit_log": "Audit log",
    "white_label": "White label",
    "custom_domain": "Custom domains",
    "gs1": "GS1 Digital Link",
    "exports": "Data exports",
    "sso": "SAML single sign-on",
    "scim": "SCIM provisioning",
    "security_policy": "Security policies",
    "priority_support": "Priority support",
    "sla": "Uptime SLA",
}

MIN_PLAN_FOR_FEATURE = {}
for _plan in ("free", "starter", "pro", "enterprise"):
    for _feature in settings.PLAN_ENTITLEMENTS[_plan]["features"]:
        MIN_PLAN_FOR_FEATURE.setdefault(_feature, _plan)


def plan_entitlements(plan):
    return settings.PLAN_ENTITLEMENTS.get(plan or "free", settings.PLAN_ENTITLEMENTS["free"])


def limit_for(plan, key):
    return plan_entitlements(plan).get(key, 0)


def has_feature(plan, feature):
    return feature in plan_entitlements(plan)["features"]


def entitlements_payload(plan):
    """Full capability descriptor for the dashboard."""
    ent = plan_entitlements(plan)
    return {
        "plan": plan or "free",
        "limits": {k: v for k, v in ent.items() if k != "features"},
        "features": {
            name: {
                "enabled": name in ent["features"],
                "label": label,
                "min_plan": MIN_PLAN_FOR_FEATURE.get(name, "enterprise"),
            }
            for name, label in FEATURE_LABELS.items()
        },
        "api_daily_limit": settings.PLAN_API_LIMITS.get(plan or "free", 50),
    }


def feature_denied(plan, feature):
    """Return a 402 response when `plan` lacks `feature`, else None."""
    if has_feature(plan, feature):
        return None
    return Response(
        {
            "success": False,
            "error": f"{FEATURE_LABELS.get(feature, feature)} requires the "
                     f"{MIN_PLAN_FOR_FEATURE.get(feature, 'enterprise')} plan",
            "code": "upgrade_required",
            "feature": feature,
            "current_plan": plan or "free",
            "required_plan": MIN_PLAN_FOR_FEATURE.get(feature, "enterprise"),
        },
        status=402,
    )


def limit_exceeded(plan, key, current, label):
    """Return a 402 response when adding one more would exceed the plan cap."""
    cap = limit_for(plan, key)
    if current < cap:
        return None
    return Response(
        {
            "success": False,
            "error": f"Your {plan or 'free'} plan allows {cap} {label}. "
                     f"Upgrade to add more.",
            "code": "limit_reached",
            "limit": cap,
            "current": current,
            "current_plan": plan or "free",
        },
        status=402,
    )
