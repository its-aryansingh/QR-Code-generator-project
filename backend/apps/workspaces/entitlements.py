"""Plan limits and feature flags, as the dashboard reads them.

One table drives both the API's limit checks and the flags the dashboard uses
to show a locked "upgrade" card instead of a failing request. The numbers are
v1's table (reference/v1-django/qrapp/settings.py PLAN_ENTITLEMENTS) mapped
onto v3's plan ids: v1 starter -> pro, v1 pro -> business.
"""

from typing import Any

from django.conf import settings

PLANS = ("free", "pro", "business", "enterprise")

_FEATURES_PRO = ["bulk", "webhooks", "campaigns"]
_FEATURES_BUSINESS = [
    *_FEATURES_PRO,
    "templates",
    "routing",
    "audit_log",
    "white_label",
    "custom_domain",
    "gs1",
    "exports",
]
_FEATURES_ENTERPRISE = [
    *_FEATURES_BUSINESS,
    "sso",
    "scim",
    "security_policy",
    "priority_support",
    "sla",
]

PLAN_ENTITLEMENTS: dict[str, dict[str, Any]] = {
    "free": {
        "max_workspaces": 1,
        "max_members": 1,
        "max_qr_codes": 50,
        "max_folders": 5,
        "max_campaigns": 1,
        "max_templates": 0,
        "max_api_keys": 1,
        "max_custom_domains": 0,
        "bulk_batch_size": 25,
        "analytics_retention_days": 30,
        "features": [],
    },
    "pro": {
        "max_workspaces": 2,
        "max_members": 3,
        "max_qr_codes": 500,
        "max_folders": 25,
        "max_campaigns": 10,
        "max_templates": 3,
        "max_api_keys": 2,
        "max_custom_domains": 0,
        "bulk_batch_size": 250,
        "analytics_retention_days": 90,
        "features": _FEATURES_PRO,
    },
    "business": {
        "max_workspaces": 10,
        "max_members": 25,
        "max_qr_codes": 10000,
        "max_folders": 200,
        "max_campaigns": 100,
        "max_templates": 25,
        "max_api_keys": 10,
        "max_custom_domains": 1,
        "bulk_batch_size": 2000,
        "analytics_retention_days": 365,
        "features": _FEATURES_BUSINESS,
    },
    "enterprise": {
        "max_workspaces": 100,
        "max_members": 1000,
        "max_qr_codes": 1000000,
        "max_folders": 5000,
        "max_campaigns": 5000,
        "max_templates": 500,
        "max_api_keys": 50,
        "max_custom_domains": 25,
        "bulk_batch_size": 10000,
        "analytics_retention_days": 1095,
        "features": _FEATURES_ENTERPRISE,
    },
}

API_DAILY_LIMITS = {"free": 50, "pro": 500, "business": 1000, "enterprise": 10000}

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


def _effective_plan(plan: str | None) -> str:
    """The plan whose limits apply: the top one while ALL_FEATURES_UNLOCKED is on."""
    return "enterprise" if settings.ALL_FEATURES_UNLOCKED else plan or "free"


def plan_entitlements(plan: str | None) -> dict[str, Any]:
    return PLAN_ENTITLEMENTS.get(_effective_plan(plan), PLAN_ENTITLEMENTS["free"])


def limit_for(plan: str | None, key: str) -> int:
    return int(plan_entitlements(plan).get(key, 0))


def min_plan_for(feature: str) -> str:
    for plan in PLANS:
        if feature in PLAN_ENTITLEMENTS[plan]["features"]:
            return plan
    return "enterprise"


def entitlements_payload(plan: str | None) -> dict[str, Any]:
    """Limits plus every feature flag, in the shape the v1 dashboard reads."""
    ent = plan_entitlements(plan)
    return {
        "plan": plan or "free",
        "limits": {k: v for k, v in ent.items() if k != "features"},
        "features": {
            name: {
                "enabled": name in ent["features"],
                "label": label,
                "min_plan": min_plan_for(name),
            }
            for name, label in FEATURE_LABELS.items()
        },
        "api_daily_limit": API_DAILY_LIMITS.get(_effective_plan(plan), API_DAILY_LIMITS["free"]),
    }
