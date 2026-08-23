"""Workspace-wide analytics and CSV exports.

The dashboard has always called `/workspaces/{id}/analytics`; nothing ever
served it, which is why the Analytics page rendered permanently empty.
"""

import csv
from datetime import timedelta

from django.db.models import Count, Q, Sum
from django.http import HttpResponse
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api.models import Campaign, Lead, LeadCapturePage, QRRecord, QRScan
from api.utils import audit
from api.utils.analytics import breakdown, delta, hourly_distribution, percentage, timeseries
from api.utils.auth import require_auth
from api.utils.entitlements import feature_denied, limit_for
from api.utils.rbac import require_workspace


def _clamp_days(request, plan):
    """Analytics retention is a plan entitlement, so a free workspace asking
    for 365 days silently gets its 30-day window rather than an error."""
    requested = max(1, min(int(request.query_params.get("days", 30)), 1095))
    return min(requested, limit_for(plan, "analytics_retention_days"))


class WorkspaceAnalyticsView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        plan = request.workspace.plan or "free"
        days = _clamp_days(request, plan)
        now = timezone.now()
        since = now - timedelta(days=days)
        prev_since = since - timedelta(days=days)

        qr_qs = QRRecord.objects.filter(workspace_id=ws_id)
        qr_ids = list(qr_qs.values_list("id", flat=True))

        scans = QRScan.objects.filter(qr_record_id__in=qr_ids, scanned_at__gte=since)
        prev_scans = QRScan.objects.filter(
            qr_record_id__in=qr_ids, scanned_at__gte=prev_since, scanned_at__lt=since
        )

        total_scans = scans.count()
        prev_total = prev_scans.count()
        unique_visitors = scans.values("ip_address").distinct().count()
        total_qrs = len(qr_ids)
        active_qrs = qr_qs.filter(is_active=True).count()
        dynamic_qrs = qr_qs.filter(is_dynamic=True).count()
        scanned_qrs = scans.values("qr_record_id").distinct().count()

        top_qr = list(
            qr_qs.order_by("-scan_count")[:10].values(
                "id", "title", "qr_type", "short_code", "scan_count", "is_active"
            )
        )
        for row in top_qr:
            row["id"] = str(row["id"])

        countries = breakdown(scans, "country_name", limit=15, label="country")
        cities = list(
            scans.exclude(city__isnull=True)
            .exclude(city="")
            .values("city", "country_code")
            .annotate(count=Count("id"))
            .order_by("-count")[:15]
        )

        return Response(
            {
                "success": True,
                "data": {
                    "range_days": days,
                    "retention_days": limit_for(plan, "analytics_retention_days"),
                    "generated_at": now.isoformat(),
                    "totals": {
                        "total_scans": total_scans,
                        "unique_visitors": unique_visitors,
                        "total_qr_codes": total_qrs,
                        "active_qr_codes": active_qrs,
                        "dynamic_qr_codes": dynamic_qrs,
                        "scanned_qr_codes": scanned_qrs,
                        "lifetime_scans": int(qr_qs.aggregate(n=Sum("scan_count"))["n"] or 0),
                        "avg_scans_per_qr": round(total_scans / total_qrs, 1) if total_qrs else 0.0,
                        "engagement_rate": percentage(scanned_qrs, total_qrs),
                    },
                    "trend": {
                        "scans": delta(total_scans, prev_total),
                        "previous_scans": prev_total,
                        "unique_visitors": delta(
                            unique_visitors, prev_scans.values("ip_address").distinct().count()
                        ),
                    },
                    "scans_by_date": timeseries(scans, since, days),
                    "scans_by_hour": hourly_distribution(scans),
                    "by_country": countries,
                    "by_city": cities,
                    "by_device": breakdown(scans, "device_type", limit=8, label="device"),
                    "by_os": breakdown(scans, "os", limit=8, label="os"),
                    "by_browser": breakdown(scans, "browser", limit=8, label="browser"),
                    "by_language": breakdown(scans, "language", limit=8, label="language"),
                    "by_referrer": breakdown(scans, "referrer", limit=10, label="referrer"),
                    "by_type": [
                        {"qr_type": row["qr_type"] or "url", "count": row["n"],
                         "scans": int(row["scans"] or 0)}
                        for row in qr_qs.values("qr_type")
                        .annotate(n=Count("id"), scans=Sum("scan_count"))
                        .order_by("-n")
                    ],
                    "top_qr_codes": top_qr,
                    "recent_scans": [
                        {
                            "id": str(row["id"]),
                            "scanned_at": row["scanned_at"].isoformat() if row["scanned_at"] else None,
                            "qr_id": str(row["qr_record_id"]),
                            "qr_title": row["qr_record__title"],
                            "country": row["country_name"],
                            "country_code": row["country_code"],
                            "city": row["city"],
                            "device": row["device_type"],
                            "os": row["os"],
                            "browser": row["browser"],
                        }
                        for row in scans.order_by("-scanned_at")[:25].values(
                            "id", "scanned_at", "qr_record_id", "qr_record__title",
                            "country_name", "country_code", "city", "device_type", "os", "browser",
                        )
                    ],
                },
            }
        )


class WorkspaceOverviewView(APIView):
    """Compact payload for the dashboard landing page."""

    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        plan = request.workspace.plan or "free"
        days = _clamp_days(request, plan)
        now = timezone.now()
        since = now - timedelta(days=days)

        qr_qs = QRRecord.objects.filter(workspace_id=ws_id)
        qr_ids = list(qr_qs.values_list("id", flat=True))
        scans = QRScan.objects.filter(qr_record_id__in=qr_ids, scanned_at__gte=since)
        prev = QRScan.objects.filter(
            qr_record_id__in=qr_ids,
            scanned_at__gte=since - timedelta(days=days),
            scanned_at__lt=since,
        )

        total_scans = scans.count()
        active_campaigns = Campaign.objects.filter(
            workspace_id=ws_id, status__in=["active", "scheduled"]
        ).count()
        page_ids = list(
            LeadCapturePage.objects.filter(workspace_id=ws_id).values_list("id", flat=True)
        )
        leads = Lead.objects.filter(page_id__in=page_ids, created_at__gte=since).count()

        return Response(
            {
                "success": True,
                "data": {
                    "range_days": days,
                    "stats": {
                        "total_qr_codes": len(qr_ids),
                        "active_qr_codes": qr_qs.filter(is_active=True).count(),
                        "dynamic_qr_codes": qr_qs.filter(is_dynamic=True).count(),
                        "total_scans": total_scans,
                        "scans_delta": delta(total_scans, prev.count()),
                        "unique_visitors": scans.values("ip_address").distinct().count(),
                        "active_campaigns": active_campaigns,
                        "new_leads": leads,
                    },
                    "scans_by_date": timeseries(scans, since, days),
                    "by_device": breakdown(scans, "device_type", limit=5, label="device"),
                    "top_qr_codes": [
                        {
                            "id": str(row["id"]), "title": row["title"],
                            "qr_type": row["qr_type"], "short_code": row["short_code"],
                            "scan_count": int(row["scan_count"] or 0), "is_active": row["is_active"],
                        }
                        for row in qr_qs.order_by("-scan_count")[:5].values(
                            "id", "title", "qr_type", "short_code", "scan_count", "is_active"
                        )
                    ],
                    "recent_qr_codes": [
                        {
                            "id": str(row["id"]), "title": row["title"],
                            "qr_type": row["qr_type"], "short_code": row["short_code"],
                            "scan_count": int(row["scan_count"] or 0),
                            "created_at": row["created_at"].isoformat() if row["created_at"] else None,
                        }
                        for row in qr_qs.order_by("-created_at")[:5].values(
                            "id", "title", "qr_type", "short_code", "scan_count", "created_at"
                        )
                    ],
                },
            }
        )


# ------------------------------------------------------------------ exports


def _csv_response(filename, header, rows):
    response = HttpResponse(content_type="text/csv")
    response["Content-Disposition"] = f'attachment; filename="{filename}"'
    writer = csv.writer(response)
    writer.writerow(header)
    for row in rows:
        writer.writerow(row)
    return response


class WorkspaceExportView(APIView):
    """`kind` is one of analytics | qr | leads | campaigns | audit."""

    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id, kind):
        plan = request.workspace.plan or "free"
        denial = feature_denied(plan, "exports")
        if denial:
            return denial

        days = _clamp_days(request, plan)
        since = timezone.now() - timedelta(days=days)
        stamp = timezone.now().strftime("%Y-%m-%d")
        qr_ids = list(QRRecord.objects.filter(workspace_id=ws_id).values_list("id", flat=True))
        audit.record(request, ws_id, audit.EXPORT, kind, None, {"days": days})

        if kind == "analytics":
            scans = QRScan.objects.filter(
                qr_record_id__in=qr_ids, scanned_at__gte=since
            ).order_by("-scanned_at")
            if request.query_params.get("qr_id"):
                scans = scans.filter(qr_record_id=request.query_params["qr_id"])
            return _csv_response(
                f"analytics-{stamp}.csv",
                ["Scan ID", "Scanned At", "QR ID", "QR Title", "Country", "Country Code",
                 "City", "Region", "Device", "OS", "Browser", "Language", "Referrer"],
                (
                    [
                        row["id"],
                        row["scanned_at"].isoformat() if row["scanned_at"] else "",
                        row["qr_record_id"], row["qr_record__title"],
                        row["country_name"], row["country_code"], row["city"], row["region"],
                        row["device_type"], row["os"], row["browser"], row["language"],
                        row["referrer"],
                    ]
                    for row in scans.values(
                        "id", "scanned_at", "qr_record_id", "qr_record__title",
                        "country_name", "country_code", "city", "region",
                        "device_type", "os", "browser", "language", "referrer",
                    )
                ),
            )

        if kind == "qr":
            rows = QRRecord.objects.filter(workspace_id=ws_id).order_by("-created_at")
            return _csv_response(
                f"qr-codes-{stamp}.csv",
                ["ID", "Title", "Type", "Content", "Short Code", "Scans", "Dynamic",
                 "Active", "Folder ID", "Tags", "Created At", "Expires At"],
                (
                    [
                        r.id, r.title, r.qr_type, r.content, r.short_code,
                        r.scan_count, r.is_dynamic, r.is_active, r.folder_id, r.tags,
                        r.created_at.isoformat() if r.created_at else "",
                        r.expires_at.isoformat() if r.expires_at else "",
                    ]
                    for r in rows
                ),
            )

        if kind == "leads":
            page_ids = list(
                LeadCapturePage.objects.filter(workspace_id=ws_id).values_list("id", flat=True)
            )
            rows = Lead.objects.filter(page_id__in=page_ids).order_by("-created_at")
            if request.query_params.get("page_id"):
                rows = rows.filter(page_id=request.query_params["page_id"])
            return _csv_response(
                f"leads-{stamp}.csv",
                ["ID", "Page ID", "Email", "Data", "Opted In", "Source", "IP Address", "Created At"],
                (
                    [
                        r.id, r.page_id, r.email, r.data, r.opted_in, r.source, r.ip_address,
                        r.created_at.isoformat() if r.created_at else "",
                    ]
                    for r in rows
                ),
            )

        if kind == "campaigns":
            rows = Campaign.objects.filter(workspace_id=ws_id).order_by("-created_at")
            return _csv_response(
                f"campaigns-{stamp}.csv",
                ["ID", "Name", "Status", "Starts At", "Ends At", "Scan Goal",
                 "UTM Source", "UTM Medium", "UTM Campaign", "Created At"],
                (
                    [
                        r.id, r.name, r.status,
                        r.starts_at.isoformat() if r.starts_at else "",
                        r.ends_at.isoformat() if r.ends_at else "",
                        r.scan_goal, r.utm_source, r.utm_medium, r.utm_campaign,
                        r.created_at.isoformat() if r.created_at else "",
                    ]
                    for r in rows
                ),
            )

        if kind == "audit":
            from api.models import AuditLog

            rows = (
                AuditLog.objects.filter(workspace_id=ws_id, created_at__gte=since)
                .select_related("user")
                .order_by("-created_at")
            )
            return _csv_response(
                f"audit-log-{stamp}.csv",
                ["ID", "Timestamp", "User", "Email", "Action", "Resource",
                 "Resource ID", "Details", "IP Address"],
                (
                    [
                        r.id,
                        r.created_at.isoformat() if r.created_at else "",
                        r.user.name if r.user else "",
                        r.user.email if r.user else "",
                        r.action, r.resource, r.resource_id, r.details, r.ip_address,
                    ]
                    for r in rows
                ),
            )

        return Response(
            {"success": False, "error": "Unknown export type",
             "valid": ["analytics", "qr", "leads", "campaigns", "audit"]},
            status=400,
        )
