"""The workspace audit log as the dashboard reads it: filters, search, pages, CSV."""

from datetime import timedelta
from typing import Any

from django.db.models import Q, QuerySet, TextField
from django.db.models.functions import Cast
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.accounts.models import User
from apps.workspaces.access import (
    DashboardAPIView,
    iso,
    ok,
    page_params,
    paginated,
    user_summary,
    workspace_for,
)
from apps.workspaces.entitlements import require_feature
from apps.workspaces.models import Workspace

from .models import AuditLog


def _filtered(workspace: Workspace, request: Request) -> QuerySet[AuditLog]:
    params = request.query_params
    rows = AuditLog.objects.filter(workspace_id=workspace.id)
    if params.get("action"):
        rows = rows.filter(action=params["action"])
    if params.get("resource"):
        rows = rows.filter(target_type=params["resource"])
    try:
        days = int(params.get("days") or 0)
    except ValueError:
        days = 0
    if days > 0:
        rows = rows.filter(created_at__gte=timezone.now() - timedelta(days=days))
    search = (params.get("search") or "").strip()
    if search:
        people = User.objects.filter(Q(email__icontains=search) | Q(name__icontains=search))
        rows = rows.annotate(changes_text=Cast("changes", TextField())).filter(
            Q(action__icontains=search)
            | Q(target_type__icontains=search)
            | Q(changes_text__icontains=search)
            | Q(actor_id__in=people.values("id"))
        )
    return rows.order_by("-created_at", "-id")


def _entries(rows: list[AuditLog]) -> list[dict[str, Any]]:
    users = {u.id: u for u in User.objects.filter(id__in={r.actor_id for r in rows if r.actor_id})}
    return [
        {
            "id": str(row.id),
            "action": row.action,
            "resource": row.target_type,
            "resource_id": str(row.target_id) if row.target_id else None,
            "details": row.changes or {},
            "ip_address": row.ip_prefix,
            "user_agent": None,
            "created_at": iso(row.created_at),
            "user": user_summary(users.get(row.actor_id)) if row.actor_id else None,
        }
        for row in rows
    ]


class WorkspaceAuditLogView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        require_feature(workspace.plan_id, "audit_log")
        page, limit = page_params(request, default_limit=50)
        rows = _filtered(workspace, request)
        total = rows.count()
        everything = AuditLog.objects.filter(workspace_id=workspace.id)
        return ok(
            {
                **paginated(
                    _entries(list(rows[(page - 1) * limit : page * limit])), total, page, limit
                ),
                "actions": list(
                    everything.order_by("action").values_list("action", flat=True).distinct()
                ),
                "resources": list(
                    everything.order_by("target_type")
                    .values_list("target_type", flat=True)
                    .distinct()
                ),
            }
        )


def export_rows(workspace: Workspace, request: Request) -> tuple[list[str], list[list[Any]]]:
    header = [
        "created_at",
        "user_email",
        "action",
        "resource",
        "resource_id",
        "details",
        "ip_prefix",
    ]
    entries = _entries(list(_filtered(workspace, request)[:50000]))
    return header, [
        [
            e["created_at"],
            (e["user"] or {}).get("email", "system"),
            e["action"],
            e["resource"],
            e["resource_id"],
            e["details"],
            e["ip_address"],
        ]
        for e in entries
    ]
