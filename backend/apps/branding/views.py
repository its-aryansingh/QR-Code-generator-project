"""Workspace branding and the SSO status card on the Brand & Domains page.

Branding lives in `workspaces.brand`. Custom CSS is not accepted: QRit never
renders customer CSS (plan §12). Single sign-on is configured per organisation
and its sign-in flow isn't served by this API yet, so the SSO endpoint reports
the organisation's connection read-only with `available: false` rather than
accepting settings that would not take effect.
"""

import re
from typing import Any

from django.conf import settings
from django.db import transaction
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.audit.record import changed_fields, record
from apps.core.errors import ApiError, unprocessable
from apps.identity.models import SSOConnection
from apps.workspaces.access import DashboardAPIView, body, iso, ok, ui_role, workspace_for
from apps.workspaces.entitlements import plan_entitlements, require_feature
from apps.workspaces.models import Workspace

HEX = re.compile(r"^#[0-9a-fA-F]{6}$")


def _brand(workspace: Workspace) -> dict[str, Any]:
    return workspace.brand if isinstance(workspace.brand, dict) else {}


def serialize(workspace: Workspace) -> dict[str, Any]:
    brand = _brand(workspace)
    return {
        "workspace_id": str(workspace.id),
        "brand_color": brand.get("color") or "#8B5CF6",
        "brand_logo": brand.get("logo_url"),
        "logo_url": brand.get("logo_url"),
        "favicon_url": brand.get("favicon_url"),
        "custom_css": None,
        "custom_footer": brand.get("footer"),
        "remove_branding": bool(brand.get("remove_branding")),
        "custom_domain": None,
        "can_white_label": "white_label" in plan_entitlements(workspace.plan_id)["features"],
    }


def _https_or_none(value: Any, field: str) -> str | None:
    url = str(value or "").strip()
    if not url:
        return None
    if not url.startswith("https://") or len(url) > 2048:
        raise unprocessable(code="invalid_url", detail=f"{field} must be an https:// address")
    return url


class WorkspaceBrandingView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        return ok(serialize(workspace))

    def put(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        data = body(request)
        before = dict(_brand(workspace))
        brand = dict(before)
        if "brand_color" in data:
            color = str(data["brand_color"] or "")
            if not HEX.match(color):
                raise unprocessable(code="invalid_color", detail="brand_color must be #RRGGBB")
            brand["color"] = color
        for key, field in (("brand_logo", "logo_url"), ("logo_url", "logo_url")):
            if key in data:
                brand[field] = _https_or_none(data[key], key)
        if "favicon_url" in data:
            brand["favicon_url"] = _https_or_none(data["favicon_url"], "favicon_url")
        if "custom_footer" in data:
            brand["footer"] = str(data["custom_footer"] or "").strip()[:300] or None
        if "remove_branding" in data:
            wanted = bool(data["remove_branding"])
            if wanted:
                require_feature(workspace.plan_id, "white_label")
            brand["remove_branding"] = wanted
        if (data.get("custom_css") or "").strip():
            raise unprocessable(
                code="custom_css_unsupported",
                detail="custom CSS isn't supported; use the brand colour, logo and footer",
            )
        fields = changed_fields(before, brand)
        if fields:
            with transaction.atomic():
                workspace.brand = brand
                workspace.updated_at = timezone.now()
                workspace.save(update_fields=["brand", "updated_at"])
                record(request, workspace, "update", "branding", workspace.id, {"fields": fields})
        return ok(serialize(workspace))


def _sso(workspace: Workspace) -> dict[str, Any]:
    connection = (
        SSOConnection.objects.filter(org_id=workspace.org_id).order_by("-updated_at").first()
        if workspace.org_id
        else None
    )
    base = settings.API_PUBLIC_URL
    return {
        "workspace_id": str(workspace.id),
        "is_enabled": bool(connection and connection.status == "active"),
        "provider": connection.protocol if connection else "saml",
        "entity_id": None,
        "sso_url": None,
        "slo_url": None,
        "certificate": None,
        "metadata_url": connection.saml_metadata_url if connection else None,
        "issuer": connection.oidc_issuer if connection else None,
        "client_id": connection.oidc_client_id if connection else None,
        "email_domains": [],
        "default_role": ui_role(connection.default_role_key) if connection else "viewer",
        "enforce_sso": False,
        "scim_enabled": False,
        "has_scim_token": False,
        "acs_url": f"{base}/v1/sso/saml/acs" if base else "",
        "sp_entity_id": f"{base}/v1/sso/saml/metadata" if base else "",
        "available": False,
        "updated_at": iso(connection.updated_at) if connection else None,
    }


class WorkspaceSsoView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        return ok(_sso(workspace))

    def put(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        require_feature(workspace.plan_id, "sso")
        raise ApiError(
            status=409,
            code="sso_unavailable",
            detail="single sign-on setup isn't available on this server yet",
        )
