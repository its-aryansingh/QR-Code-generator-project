"""Campaign endpoints for the dashboard: list, create, view, edit, delete.

The dashboard speaks v1's campaign shape. v3 stores five statuses
(draft/active/paused/ended/archived); "scheduled" and "completed" are what an
active campaign is called before its start date and after its end date, and a
"completed" campaign saved from the dashboard is stored as "ended". The schema
has no description or colour column, so the colour is derived from the id and
the description is not stored.
"""

import hashlib
from datetime import datetime, timedelta
from typing import Any

from django.db import transaction
from django.db.models import Count, Sum
from django.utils import timezone
from django.utils.dateparse import parse_date, parse_datetime
from rest_framework.request import Request
from rest_framework.response import Response

from apps.analytics.models import ScanEvent
from apps.audit.record import changed_fields, record
from apps.core.errors import not_found, unprocessable
from apps.workspaces.access import (
    DashboardAPIView,
    body,
    iso,
    ok,
    uuid_or_404,
    workspace_for,
)
from apps.workspaces.entitlements import require_capacity, require_feature
from apps.workspaces.models import Workspace

from .models import Campaign, QRCode

UI_STATUSES = ("draft", "scheduled", "active", "paused", "completed", "archived")
TO_STORED = {"scheduled": "active", "completed": "ended"}
UTM_KEYS = ("source", "medium", "campaign", "term", "content")
PALETTE = ("#8B5CF6", "#06B6D4", "#10B981", "#F59E0B", "#EF4444", "#EC4899", "#3B82F6", "#84CC16")


def color_for(campaign: Campaign) -> str:
    return PALETTE[hashlib.sha256(campaign.id.bytes).digest()[0] % len(PALETTE)]


def ui_status(campaign: Campaign, now: datetime | None = None) -> str:
    now = now or timezone.now()
    if campaign.status == "ended":
        return "completed"
    if campaign.status == "active":
        if campaign.starts_at and campaign.starts_at > now:
            return "scheduled"
        if campaign.ends_at and campaign.ends_at < now:
            return "completed"
    return campaign.status


def campaign_stats(workspace: Workspace, ids: list[Any]) -> dict[Any, dict[str, int]]:
    stats: dict[Any, dict[str, int]] = {cid: {"qr_count": 0, "scan_count": 0} for cid in ids}
    for row in (
        QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True, campaign_id__in=ids)
        .values("campaign_id")
        .annotate(qr_count=Count("id"), scan_count=Sum("total_scans"))
    ):
        stats[row["campaign_id"]] = {
            "qr_count": row["qr_count"],
            "scan_count": row["scan_count"] or 0,
        }
    week_ago = timezone.now() - timedelta(days=7)
    for recent in (
        ScanEvent.objects.filter(
            workspace_id=workspace.id,
            campaign_id__in=ids,
            occurred_at__gte=week_ago,
            is_bot=False,
            outcome="redirect",
        )
        .values("campaign_id")
        .annotate(n=Count("event_id"))
    ):
        entry = stats.setdefault(recent["campaign_id"], {"qr_count": 0, "scan_count": 0})
        entry["recent_scans"] = recent["n"]
    return stats


def serialize(campaign: Campaign, stats: dict[str, int] | None = None) -> dict[str, Any]:
    stats = stats or {}
    goal = campaign.goal_scans or 0
    scans = stats.get("scan_count", 0)
    utm = campaign.utm if isinstance(campaign.utm, dict) else {}
    return {
        "id": str(campaign.id),
        "workspace_id": str(campaign.workspace_id),
        "name": campaign.name,
        "description": None,
        "status": ui_status(campaign),
        "color": color_for(campaign),
        "starts_at": iso(campaign.starts_at),
        "ends_at": iso(campaign.ends_at),
        "scan_goal": goal,
        "utm": {key: utm.get(key) for key in UTM_KEYS},
        "tags": [],
        "qr_count": stats.get("qr_count", 0),
        "scan_count": scans,
        "recent_scans": stats.get("recent_scans", 0),
        "goal_progress": round(min(scans / goal * 100, 100), 1) if goal else None,
        "created_at": iso(campaign.created_at),
        "updated_at": iso(campaign.updated_at),
    }


def _when(value: Any, field: str, end_of_day: bool = False) -> datetime | None:
    if value in (None, ""):
        return None
    text = str(value).strip()
    parsed = parse_datetime(text)
    if parsed is None:
        day = parse_date(text)
        if day is None:
            raise unprocessable(code="invalid_date", detail=f"{field} must be a date")
        parsed = datetime.combine(day, datetime.max.time() if end_of_day else datetime.min.time())
    if timezone.is_naive(parsed):
        parsed = timezone.make_aware(parsed)
    return parsed


def _goal(value: Any) -> int | None:
    if value in (None, ""):
        return None
    try:
        goal = int(value)
    except (TypeError, ValueError):
        raise unprocessable(
            code="invalid_scan_goal", detail="scan_goal must be a whole number"
        ) from None
    if goal < 0:
        raise unprocessable(code="invalid_scan_goal", detail="scan_goal can't be negative")
    return goal or None


def _apply(campaign: Campaign, data: dict[str, Any], partial: bool) -> None:
    if "name" in data or not partial:
        name = str(data.get("name") or "").strip()
        if not name:
            raise unprocessable(code="name_required", detail="campaign name is required")
        campaign.name = name[:200]
    if "status" in data or not partial:
        status = str(data.get("status") or "draft")
        if status not in UI_STATUSES:
            raise unprocessable(
                code="invalid_status", detail=f"status must be one of {', '.join(UI_STATUSES)}"
            )
        campaign.status = TO_STORED.get(status, status)
    if "starts_at" in data:
        campaign.starts_at = _when(data["starts_at"], "starts_at")
    if "ends_at" in data:
        campaign.ends_at = _when(data["ends_at"], "ends_at", end_of_day=True)
    if campaign.starts_at and campaign.ends_at and campaign.ends_at <= campaign.starts_at:
        raise unprocessable(
            code="invalid_dates", detail="the end date must be after the start date"
        )
    if "scan_goal" in data:
        campaign.goal_scans = _goal(data["scan_goal"])
    if "utm" in data:
        raw = data["utm"] if isinstance(data["utm"], dict) else {}
        campaign.utm = {
            key: str(raw[key]).strip()[:200] for key in UTM_KEYS if raw.get(key) not in (None, "")
        }


def _campaign_in(workspace: Workspace, campaign_id: str) -> Campaign:
    campaign = Campaign.objects.filter(
        workspace=workspace, id=uuid_or_404(campaign_id, "campaign")
    ).first()
    if campaign is None:
        raise not_found("campaign not found")
    return campaign


def _audit_view(campaign: Campaign) -> dict[str, Any]:
    return {
        "name": campaign.name,
        "status": campaign.status,
        "starts_at": iso(campaign.starts_at),
        "ends_at": iso(campaign.ends_at),
        "goal_scans": campaign.goal_scans,
        "utm": campaign.utm,
    }


class WorkspaceCampaignsView(DashboardAPIView):
    list_view = True

    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        rows = list(Campaign.objects.filter(workspace=workspace).order_by("-created_at"))
        wanted = request.query_params.get("status")
        search = (request.query_params.get("search") or "").strip().lower()
        stats = campaign_stats(workspace, [c.id for c in rows])
        out = [serialize(c, stats.get(c.id)) for c in rows]
        if wanted:
            out = [c for c in out if c["status"] == wanted]
        if search:
            out = [c for c in out if search in c["name"].lower()]
        return ok(out)

    def post(self, request: Request, ws: str) -> Response:
        user = self.user(request)
        workspace, _ = workspace_for(user, ws, min_role="editor")
        require_feature(workspace.plan_id, "campaigns")
        require_capacity(
            workspace.plan_id,
            "max_campaigns",
            Campaign.objects.filter(workspace=workspace).count(),
            "campaigns",
        )
        campaign = Campaign(workspace=workspace, created_by=user)
        _apply(campaign, body(request), partial=False)
        with transaction.atomic():
            campaign.save()
            record(request, workspace, "create", "campaign", campaign.id, {"name": campaign.name})
        return ok(serialize(campaign), status=201)


class WorkspaceCampaignDetailView(DashboardAPIView):
    def get(self, request: Request, ws: str, campaign_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        campaign = _campaign_in(workspace, campaign_id)
        data = serialize(campaign, campaign_stats(workspace, [campaign.id]).get(campaign.id))
        data["qr_codes"] = [
            {
                "id": str(qr.id),
                "title": qr.name,
                "qr_type": (qr.design or {}).get("qr_type") or qr.content_type,
                "short_code": qr.short_code,
                "scan_count": qr.total_scans,
                "is_active": qr.status == "active",
            }
            for qr in QRCode.objects.filter(
                workspace=workspace, campaign=campaign, deleted_at__isnull=True
            ).order_by("-total_scans")[:50]
        ]
        return ok(data)

    def put(self, request: Request, ws: str, campaign_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="editor")
        campaign = _campaign_in(workspace, campaign_id)
        before = _audit_view(campaign)
        _apply(campaign, body(request), partial=True)
        campaign.updated_at = timezone.now()
        after = _audit_view(campaign)
        with transaction.atomic():
            campaign.save()
            record(
                request,
                workspace,
                "update",
                "campaign",
                campaign.id,
                {"name": campaign.name, "fields": changed_fields(before, after)},
            )
        return ok(serialize(campaign, campaign_stats(workspace, [campaign.id]).get(campaign.id)))

    def delete(self, request: Request, ws: str, campaign_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="editor")
        campaign = _campaign_in(workspace, campaign_id)
        with transaction.atomic():
            record(request, workspace, "delete", "campaign", campaign.id, {"name": campaign.name})
            campaign.delete()
        return ok({"message": "Campaign deleted"})
