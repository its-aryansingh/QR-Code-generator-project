"""Workspace analytics and CSV exports for the dashboard.

Counts are human scans that reached a destination (`outcome = "redirect"`,
not bots), the same definition the overview page uses, so the two pages agree.
Hours are UTC, as the page labels them.
"""

from datetime import date, timedelta
from typing import Any

from django.db import transaction
from django.db.models import Count, QuerySet
from django.db.models.functions import ExtractHour, TruncDate
from django.http import HttpResponse
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.audit.record import record
from apps.audit.views import export_rows as audit_rows
from apps.core.errors import not_found
from apps.leads.views import export_rows as lead_rows
from apps.qr import dashboard
from apps.qr.campaigns import campaign_stats
from apps.qr.campaigns import serialize as serialize_campaign
from apps.qr.models import Campaign, QRCode
from apps.workspaces.access import DashboardAPIView, iso, ok, workspace_for
from apps.workspaces.entitlements import limit_for, require_feature
from apps.workspaces.export import csv_response
from apps.workspaces.models import Workspace
from apps.workspaces.views import percent_delta, range_days

from .models import ScanEvent

TOP = 10


def human_scans(workspace: Workspace) -> QuerySet[ScanEvent]:
    return ScanEvent.objects.filter(workspace_id=workspace.id, is_bot=False, outcome="redirect")


def _ranked(rows: QuerySet[ScanEvent], field: str, label: str) -> list[dict[str, Any]]:
    return [
        {label: row[field] or "Unknown", "count": row["n"]}
        for row in rows.values(field).annotate(n=Count("event_id")).order_by("-n", field)[:TOP]
    ]


def _qr_type(qr: QRCode) -> str:
    return str((qr.design or {}).get("qr_type") or qr.content_type)


def workspace_analytics(workspace: Workspace, days: int) -> dict[str, Any]:
    now = timezone.now()
    since = now - timedelta(days=days)
    before = since - timedelta(days=days)
    scans = human_scans(workspace)
    window = scans.filter(occurred_at__gte=since)
    previous = scans.filter(occurred_at__gte=before, occurred_at__lt=since)

    total = window.count()
    previous_total = previous.count()
    visitors = window.values("visitor_hash").distinct().count()
    previous_visitors = previous.values("visitor_hash").distinct().count()

    per_day = {
        row["day"]: row["n"]
        for row in window.annotate(day=TruncDate("occurred_at"))
        .values("day")
        .annotate(n=Count("event_id"))
    }
    first: date = since.date()
    per_hour = {
        row["hour"]: row["n"]
        for row in window.annotate(hour=ExtractHour("occurred_at"))
        .values("hour")
        .annotate(n=Count("event_id"))
    }

    codes = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True)
    code_list = list(codes.only("id", "name", "design", "content_type", "total_scans", "status"))
    titles = {qr.id: qr.name for qr in code_list}
    scanned_ids = set(window.values_list("qr_code_id", flat=True).distinct())
    window_by_code = {
        row["qr_code_id"]: row["n"]
        for row in window.values("qr_code_id").annotate(n=Count("event_id"))
    }
    by_type: dict[str, dict[str, Any]] = {}
    for qr in code_list:
        entry = by_type.setdefault(_qr_type(qr), {"qr_type": _qr_type(qr), "count": 0, "scans": 0})
        entry["count"] += 1
        entry["scans"] += window_by_code.get(qr.id, 0)
    total_codes = len(code_list)
    scanned_codes = sum(1 for qr in code_list if qr.id in scanned_ids)

    return {
        "range_days": days,
        "retention_days": limit_for(workspace.plan_id, "analytics_retention_days"),
        "generated_at": now.isoformat(),
        "totals": {
            "total_scans": total,
            "unique_visitors": visitors,
            "total_qr_codes": total_codes,
            "active_qr_codes": sum(1 for qr in code_list if qr.status == "active"),
            "dynamic_qr_codes": codes.filter(mode="dynamic").count(),
            "scanned_qr_codes": scanned_codes,
            "lifetime_scans": sum(qr.total_scans for qr in code_list),
            "avg_scans_per_qr": round(total / total_codes, 1) if total_codes else 0.0,
            "engagement_rate": round(scanned_codes / total_codes * 100, 1) if total_codes else 0.0,
        },
        "trend": {
            "scans": percent_delta(total, previous_total),
            "previous_scans": previous_total,
            "unique_visitors": percent_delta(visitors, previous_visitors),
        },
        "scans_by_date": [
            {
                "date": (first + timedelta(days=i)).isoformat(),
                "count": per_day.get(first + timedelta(days=i), 0),
            }
            for i in range(days + 1)
        ],
        "scans_by_hour": [{"hour": h, "count": per_hour.get(h, 0)} for h in range(24)],
        "by_country": _ranked(window, "country", "country"),
        "by_city": [
            {"city": row["city"], "country_code": row["country"] or "", "count": row["n"]}
            for row in window.exclude(city__isnull=True)
            .exclude(city="")
            .values("city", "country")
            .annotate(n=Count("event_id"))
            .order_by("-n", "city")[:TOP]
        ],
        "by_device": _ranked(window, "device_type", "device"),
        "by_os": _ranked(window, "os", "os"),
        "by_browser": _ranked(window, "browser", "browser"),
        "by_language": _ranked(window.exclude(language__isnull=True), "language", "language"),
        "by_referrer": _ranked(
            window.exclude(referrer_host__isnull=True), "referrer_host", "referrer"
        ),
        "by_type": sorted(by_type.values(), key=lambda row: (-row["scans"], -row["count"])),
        "top_qr_codes": [
            {
                "id": str(qr.id),
                "title": qr.name,
                "qr_type": _qr_type(qr),
                "short_code": qr.short_code,
                "scan_count": qr.total_scans,
                "is_active": qr.status == "active",
            }
            for qr in codes.order_by("-total_scans", "-created_at")[:TOP]
        ],
        "recent_scans": [
            {
                "id": str(event.event_id),
                "scanned_at": iso(event.occurred_at),
                "qr_id": str(event.qr_code_id),
                "qr_title": titles.get(event.qr_code_id),
                "country": event.country,
                "country_code": event.country,
                "city": event.city,
                "device": event.device_type,
                "os": event.os,
                "browser": event.browser,
            }
            for event in window.order_by("-occurred_at")[:20]
        ],
    }


class WorkspaceAnalyticsView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        return ok(workspace_analytics(workspace, range_days(request, workspace.plan_id)))


def _analytics_rows(workspace: Workspace, days: int) -> tuple[list[str], list[list[Any]]]:
    since = timezone.now() - timedelta(days=days)
    rows = human_scans(workspace).filter(occurred_at__gte=since).order_by("-occurred_at")[:50000]
    titles = dict(QRCode.objects.filter(workspace=workspace).values_list("id", "name"))
    header = [
        "scanned_at", "qr_id", "qr_title", "country", "region", "city",
        "device", "os", "browser", "language", "referrer", "utm_source",
        "utm_medium", "utm_campaign",
    ]  # fmt: skip
    return header, [
        [
            iso(e.occurred_at),
            str(e.qr_code_id),
            titles.get(e.qr_code_id, ""),
            e.country,
            e.region,
            e.city,
            e.device_type,
            e.os,
            e.browser,
            e.language,
            e.referrer_host,
            e.utm_source,
            e.utm_medium,
            e.utm_campaign,
        ]  # fmt: skip
        for e in rows
    ]


def _qr_rows(workspace: Workspace) -> tuple[list[str], list[list[Any]]]:
    header = ["id", "title", "type", "mode", "status", "short_url", "destination",
              "scans", "folder", "campaign", "created_at"]  # fmt: skip
    rows = (
        QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True)
        .select_related("folder", "campaign", "current_version")
        .order_by("-created_at")
    )
    return header, [
        [
            str(qr.id),
            qr.name,
            _qr_type(qr),
            qr.mode,
            qr.status,
            dashboard.short_url(qr.short_code) or "",
            dashboard.destination(qr),
            qr.total_scans,
            qr.folder.name if qr.folder else "",
            qr.campaign.name if qr.campaign else "",
            iso(qr.created_at),
        ]  # fmt: skip
        for qr in rows
    ]


def _campaign_rows(workspace: Workspace) -> tuple[list[str], list[list[Any]]]:
    header = ["id", "name", "status", "starts_at", "ends_at", "scan_goal", "qr_codes",
              "scans", "utm_source", "utm_medium", "utm_campaign"]  # fmt: skip
    rows = list(Campaign.objects.filter(workspace=workspace).order_by("-created_at"))
    stats = campaign_stats(workspace, [c.id for c in rows])
    out = []
    for campaign in rows:
        data = serialize_campaign(campaign, stats.get(campaign.id))
        out.append(
            [
                data["id"],
                data["name"],
                data["status"],
                data["starts_at"],
                data["ends_at"],
                data["scan_goal"] or "",
                data["qr_count"],
                data["scan_count"],
                data["utm"]["source"],
                data["utm"]["medium"],
                data["utm"]["campaign"],
            ]  # fmt: skip
        )
    return header, out


class WorkspaceExportView(DashboardAPIView):
    def get(self, request: Request, ws: str, kind: str) -> HttpResponse:
        user = self.user(request)
        workspace, _ = workspace_for(user, ws, min_role="admin" if kind == "audit" else "analyst")
        require_feature(workspace.plan_id, "audit_log" if kind == "audit" else "exports")
        days = range_days(request, workspace.plan_id)
        if kind == "analytics":
            header, rows = _analytics_rows(workspace, days)
        elif kind == "qr":
            header, rows = _qr_rows(workspace)
        elif kind == "campaigns":
            header, rows = _campaign_rows(workspace)
        elif kind == "audit":
            header, rows = audit_rows(workspace, request)
        elif kind == "leads":
            header, rows = lead_rows(workspace)
        else:
            raise not_found("unknown export")
        with transaction.atomic():
            record(request, workspace, "export", kind, None, {"rows": len(rows)})
        return csv_response(f"{kind}-{timezone.now().date().isoformat()}.csv", header, rows)
