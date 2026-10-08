"""Workspace endpoints for the dashboard: list, create, detail, plan, overview, folders."""

from datetime import date, timedelta
from typing import Any

from django.db import transaction
from django.db.models import Count, Q, Sum
from django.db.models.functions import TruncDate
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.access.models import RoleBinding
from apps.accounts.services import _unique_slug
from apps.analytics.models import ScanEvent
from apps.core.errors import ApiError, unprocessable
from apps.qr.dashboard import folder_in
from apps.qr.models import Campaign, Folder, QRCode

from .access import DashboardAPIView, ok, ui_role, workspace_for
from .entitlements import entitlements_payload, limit_for
from .models import Workspace, WorkspaceMember

OWNER_ROLE_ID = "00000000-0000-7000-8000-00000000f001"


def _counts(ws: Workspace) -> dict[str, int]:
    qr = QRCode.objects.filter(workspace=ws, deleted_at__isnull=True).aggregate(
        qr_count=Count("id"), scan_count=Sum("total_scans")
    )
    return {
        "qr_count": qr["qr_count"] or 0,
        "scan_count": qr["scan_count"] or 0,
        "member_count": WorkspaceMember.objects.filter(workspace=ws).count(),
        "campaign_count": Campaign.objects.filter(workspace=ws).count(),
        "folder_count": Folder.objects.filter(workspace=ws).count(),
    }


def serialize_workspace(ws: Workspace, role: str) -> dict[str, Any]:
    brand = ws.brand if isinstance(ws.brand, dict) else {}
    settings_ = ws.settings if isinstance(ws.settings, dict) else {}
    return {
        "id": str(ws.id),
        "name": ws.name,
        "slug": str(ws.slug),
        "description": settings_.get("description"),
        "owner_id": str(ws.owner_id),
        "plan": ws.plan_id,
        "role": ui_role(role),
        "brand_color": brand.get("color") or "#8B5CF6",
        "logo_url": brand.get("logo_url"),
        "brand_logo": brand.get("logo_url"),
        "favicon_url": None,
        "custom_css": None,
        "custom_footer": None,
        "custom_domain": None,
        "remove_branding": False,
        "sso_enabled": False,
        **_counts(ws),
        "created_at": ws.created_at.isoformat(),
        "updated_at": ws.updated_at.isoformat(),
    }


def user_workspaces(user: Any) -> list[tuple[Workspace, str]]:
    roles = dict(WorkspaceMember.objects.filter(user=user).values_list("workspace_id", "role"))
    rows = (
        Workspace.objects.filter(Q(owner=user) | Q(id__in=list(roles)), deleted_at__isnull=True)
        .distinct()
        .order_by("-created_at")
    )
    return [(ws, "owner" if ws.owner_id == user.id else roles.get(ws.id, "analyst")) for ws in rows]


class WorkspaceListView(DashboardAPIView):
    list_view = True

    def get(self, request: Request) -> Response:
        user = self.user(request)
        return ok([serialize_workspace(ws, role) for ws, role in user_workspaces(user)])

    def post(self, request: Request) -> Response:
        user = self.user(request)
        body = request.data if isinstance(request.data, dict) else {}
        name = str(body.get("name") or "").strip()
        if not name:
            raise unprocessable(code="name_required", detail="workspace name is required")
        owned = list(Workspace.objects.filter(owner=user, deleted_at__isnull=True))
        plan = max((ws.plan_id for ws in owned), key=_plan_rank, default="free")
        allowed = limit_for(plan, "max_workspaces")
        if len(owned) >= allowed:
            raise ApiError(
                status=402,
                code="limit_reached",
                detail=f"your plan allows {allowed} workspace{'s' if allowed != 1 else ''}",
                extra={"required_plan": "pro"},
            )
        org = owned[0].org if owned else None
        brand_color = str(body.get("brand_color") or "").strip()
        with transaction.atomic():
            ws = Workspace.objects.create(
                org=org,
                name=name[:120],
                slug=_unique_slug(name, Workspace),
                owner=user,
                plan_id=plan,
                brand={"color": brand_color} if brand_color.startswith("#") else {},
                settings={"description": str(body.get("description") or "")[:500] or None},
            )
            WorkspaceMember.objects.create(workspace=ws, user=user, role="owner")
            if org is not None:
                RoleBinding.objects.create(
                    org=org,
                    workspace=ws,
                    principal_type="user",
                    principal_id=user.id,
                    role_id=OWNER_ROLE_ID,
                    scope_type="workspace",
                    created_by=user,
                )
        return ok(serialize_workspace(ws, "owner"), status=201)


def _plan_rank(plan: str) -> int:
    return (
        ("free", "pro", "business", "enterprise").index(plan)
        if plan in ("free", "pro", "business", "enterprise")
        else 0
    )


class WorkspaceDetailView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, role = workspace_for(self.user(request), ws)
        data = serialize_workspace(workspace, role)
        data["entitlements"] = {**entitlements_payload(workspace.plan_id), "role": ui_role(role)}
        return ok(data)


class WorkspaceEntitlementsView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, role = workspace_for(self.user(request), ws)
        counts = _counts(workspace)
        return ok(
            {
                **entitlements_payload(workspace.plan_id),
                "role": ui_role(role),
                "usage": {
                    "qr_codes": counts["qr_count"],
                    "members": counts["member_count"],
                    "campaigns": counts["campaign_count"],
                    "folders": counts["folder_count"],
                },
            }
        )


def _days(request: Request, plan: str) -> int:
    try:
        days = int(request.query_params.get("days") or 30)
    except ValueError:
        days = 30
    return max(1, min(days, limit_for(plan, "analytics_retention_days") or 30))


def _delta(current: int, previous: int) -> float:
    if previous == 0:
        return 100.0 if current else 0.0
    return round((current - previous) / previous * 100, 1)


class WorkspaceOverviewView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        days = _days(request, workspace.plan_id)
        now = timezone.now()
        since = now - timedelta(days=days)
        before = since - timedelta(days=days)

        scans = ScanEvent.objects.filter(
            workspace_id=workspace.id, is_bot=False, outcome="redirect"
        )
        window = scans.filter(occurred_at__gte=since)
        total = window.count()
        previous = scans.filter(occurred_at__gte=before, occurred_at__lt=since).count()

        per_day = {
            row["day"]: row["n"]
            for row in window.annotate(day=TruncDate("occurred_at"))
            .values("day")
            .annotate(n=Count("event_id"))
        }
        first: date = since.date()
        scans_by_date = [
            {
                "date": (first + timedelta(days=i)).isoformat(),
                "count": per_day.get(first + timedelta(days=i), 0),
            }
            for i in range(days + 1)
        ]
        by_device = [
            {"device": row["device_type"] or "Unknown", "count": row["n"]}
            for row in window.values("device_type").annotate(n=Count("event_id")).order_by("-n")[:5]
        ]

        codes = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True)

        def brief(qr: QRCode) -> dict[str, Any]:
            return {
                "id": str(qr.id),
                "title": qr.name,
                "qr_type": (qr.design or {}).get("qr_type") or qr.content_type,
                "short_code": qr.short_code,
                "scan_count": qr.total_scans,
                "is_active": qr.status == "active",
                "created_at": qr.created_at.isoformat(),
            }

        return ok(
            {
                "range_days": days,
                "stats": {
                    "total_qr_codes": codes.count(),
                    "active_qr_codes": codes.filter(status="active").count(),
                    "dynamic_qr_codes": codes.filter(mode="dynamic").count(),
                    "total_scans": total,
                    "scans_delta": _delta(total, previous),
                    "unique_visitors": window.values("visitor_hash").distinct().count(),
                    "active_campaigns": Campaign.objects.filter(
                        workspace=workspace, status__in=["active", "scheduled"]
                    ).count(),
                    "new_leads": 0,
                },
                "scans_by_date": scans_by_date,
                "by_device": by_device,
                "top_qr_codes": [
                    brief(qr) for qr in codes.order_by("-total_scans", "-created_at")[:5]
                ],
                "recent_qr_codes": [brief(qr) for qr in codes.order_by("-created_at")[:5]],
            }
        )


class WorkspaceFoldersView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        folders = list(Folder.objects.filter(workspace=workspace).order_by("position", "name"))
        stats = {
            row["folder_id"]: row
            for row in QRCode.objects.filter(
                workspace=workspace, deleted_at__isnull=True, folder__isnull=False
            )
            .values("folder_id")
            .annotate(qr_count=Count("id"), scan_count=Sum("total_scans"))
        }
        nodes: dict[Any, dict[str, Any]] = {}
        for folder in folders:
            row: dict[str, Any] = dict(stats.get(folder.id) or {})
            nodes[folder.id] = {
                "id": str(folder.id),
                "workspace_id": str(workspace.id),
                "parent_id": str(folder.parent_id) if folder.parent_id else None,
                "name": folder.name,
                "description": None,
                "color": "#8B5CF6",
                "icon": None,
                "sort_order": folder.position,
                "qr_count": row.get("qr_count") or 0,
                "scan_count": row.get("scan_count") or 0,
                "children": [],
                "created_at": folder.created_at.isoformat(),
            }
        roots: list[dict[str, Any]] = []
        for folder in folders:
            node = nodes[folder.id]
            parent = nodes.get(folder.parent_id) if folder.parent_id else None
            (parent["children"] if parent else roots).append(node)
        return ok(roots)

    def post(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="editor")
        body = request.data if isinstance(request.data, dict) else {}
        name = str(body.get("name") or "").strip()[:100]
        if not name:
            raise unprocessable(code="name_required", detail="folder name is required")
        allowed = limit_for(workspace.plan_id, "max_folders")
        if Folder.objects.filter(workspace=workspace).count() >= allowed:
            raise ApiError(
                status=402,
                code="limit_reached",
                detail=f"your plan allows {allowed} folders",
                extra={"required_plan": "pro"},
            )
        parent = folder_in(workspace, body.get("parent_id"))
        if Folder.objects.filter(workspace=workspace, parent=parent, name=name).exists():
            raise ApiError(
                status=409,
                code="folder_exists",
                detail="a folder with this name already exists here",
            )
        folder = Folder.objects.create(workspace=workspace, parent=parent, name=name)
        return ok(
            {
                "id": str(folder.id),
                "workspace_id": str(workspace.id),
                "parent_id": str(parent.id) if parent else None,
                "name": folder.name,
                "description": None,
                "color": "#8B5CF6",
                "icon": None,
                "sort_order": folder.position,
                "qr_count": 0,
                "scan_count": 0,
                "children": [],
                "created_at": folder.created_at.isoformat(),
            },
            status=201,
        )


class WorkspaceCampaignsView(DashboardAPIView):
    """Read-only list so pages that fetch campaigns don't fail."""

    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        return ok(
            [
                {
                    "id": str(c.id),
                    "workspace_id": str(workspace.id),
                    "name": c.name,
                    "status": c.status,
                    "starts_at": c.starts_at.isoformat() if c.starts_at else None,
                    "ends_at": c.ends_at.isoformat() if c.ends_at else None,
                    "goal_scans": c.goal_scans,
                    "utm": c.utm,
                    "created_at": c.created_at.isoformat(),
                }
                for c in Campaign.objects.filter(workspace=workspace).order_by("-created_at")
            ]
        )
