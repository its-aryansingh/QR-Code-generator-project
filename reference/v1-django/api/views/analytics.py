"""Per-user and per-QR analytics.

Rewritten off raw SQL: the previous queries used Postgres-only casts
(`%s::uuid`, `DATE(x)::text`) and raised a 500 on SQLite, so the dashboard was
unusable in local development.
"""

from datetime import timedelta

from django.db.models import Count, Q, Sum
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api.models import QRRecord, QRScan, RoutingRule
from api.utils.analytics import breakdown, delta, hourly_distribution, timeseries
from api.utils.auth import require_auth
from api.utils.rbac import accessible_workspace_ids


def _visible_qr(user_id):
    """Codes the user owns plus every code in a workspace they belong to."""
    return QRRecord.objects.filter(
        Q(user_id=user_id) | Q(workspace_id__in=accessible_workspace_ids(user_id))
    ).distinct()


class DashboardView(APIView):
    @require_auth
    def get(self, request):
        user_id = request.auth_user["id"]
        days = max(1, min(int(request.query_params.get("days", 30)), 365))
        since = timezone.now() - timedelta(days=days)

        qr_qs = _visible_qr(user_id)
        qr_ids = list(qr_qs.values_list("id", flat=True))
        scans = QRScan.objects.filter(qr_record_id__in=qr_ids, scanned_at__gte=since)
        previous = QRScan.objects.filter(
            qr_record_id__in=qr_ids,
            scanned_at__gte=since - timedelta(days=days),
            scanned_at__lt=since,
        )

        total_scans = scans.count()
        return Response(
            {
                "success": True,
                "data": {
                    "range_days": days,
                    "total_qrs": len(qr_ids),
                    "total_qr_codes": len(qr_ids),
                    "active_qrs": qr_qs.filter(is_active=True).count(),
                    "dynamic_qr_codes": qr_qs.filter(is_dynamic=True).count(),
                    "total_scans": total_scans,
                    "lifetime_scans": int(qr_qs.aggregate(n=Sum("scan_count"))["n"] or 0),
                    "unique_visitors": scans.values("ip_address").distinct().count(),
                    "scans_delta": delta(total_scans, previous.count()),
                    "scans_by_date": timeseries(scans, since, days),
                    "by_device": breakdown(scans, "device_type", limit=6, label="device"),
                    "by_country": breakdown(scans, "country_name", limit=8, label="country"),
                    "recent_scans": [
                        {
                            "id": str(row["id"]),
                            "scanned_at": row["scanned_at"].isoformat() if row["scanned_at"] else None,
                            "qr_id": str(row["qr_record_id"]),
                            "qr_title": row["qr_record__title"],
                            "qr_type": row["qr_record__qr_type"],
                            "country": row["country_name"],
                            "device": row["device_type"],
                            "browser": row["browser"],
                        }
                        for row in scans.order_by("-scanned_at")[:20].values(
                            "id", "scanned_at", "qr_record_id", "qr_record__title",
                            "qr_record__qr_type", "country_name", "device_type", "browser",
                        )
                    ],
                    "top_qr_codes": [
                        {
                            "id": str(row["id"]), "title": row["title"],
                            "qr_type": row["qr_type"], "short_code": row["short_code"],
                            "scan_count": int(row["scan_count"] or 0),
                        }
                        for row in qr_qs.order_by("-scan_count")[:5].values(
                            "id", "title", "qr_type", "short_code", "scan_count"
                        )
                    ],
                },
            }
        )


class QRAnalyticsView(APIView):
    @require_auth
    def get(self, request, qr_id):
        qr = _visible_qr(request.auth_user["id"]).filter(id=qr_id).first()
        if not qr:
            return Response({"success": False, "error": "Not found"}, status=404)

        days = max(1, min(int(request.query_params.get("days", 30)), 365))
        since = timezone.now() - timedelta(days=days)
        scans = QRScan.objects.filter(qr_record_id=qr.id, scanned_at__gte=since)
        previous = QRScan.objects.filter(
            qr_record_id=qr.id,
            scanned_at__gte=since - timedelta(days=days),
            scanned_at__lt=since,
        )
        total = scans.count()

        rules = RoutingRule.objects.filter(qr_record_id=qr.id).order_by("-hit_count")
        routing_performance = [
            {
                "id": str(r.id), "name": r.name, "condition": r.condition,
                "destination_url": r.destination_url, "hit_count": int(r.hit_count or 0),
                "share": round(int(r.hit_count or 0) / total * 100, 1) if total else 0.0,
            }
            for r in rules
        ]

        return Response(
            {
                "success": True,
                "data": {
                    "qr": {
                        "id": str(qr.id), "title": qr.title, "qr_type": qr.qr_type,
                        "content": qr.content, "short_code": qr.short_code,
                        "is_dynamic": qr.is_dynamic, "is_active": qr.is_active,
                        "scan_count": int(qr.scan_count or 0),
                        "created_at": qr.created_at.isoformat() if qr.created_at else None,
                    },
                    "range_days": days,
                    "analytics": {
                        "total_scans": total,
                        "lifetime_scans": int(qr.scan_count or 0),
                        "unique_visitors": scans.values("ip_address").distinct().count(),
                        "scans_delta": delta(total, previous.count()),
                        "scans_by_date": timeseries(scans, since, days),
                        "scans_by_hour": hourly_distribution(scans),
                        "scans_by_country": list(
                            scans.values("country_code", "country_name")
                            .annotate(count=Count("id")).order_by("-count")[:10]
                        ),
                        "scans_by_device": breakdown(scans, "device_type", 8, "device_type"),
                        "scans_by_browser": breakdown(scans, "browser", 10, "browser"),
                        "scans_by_os": breakdown(scans, "os", 10, "os"),
                        "scans_by_language": breakdown(scans, "language", 8, "language"),
                        "scans_by_referrer": breakdown(scans, "referrer", 10, "referrer"),
                        "top_cities": list(
                            scans.exclude(city__isnull=True).exclude(city="")
                            .values("city", "country_code")
                            .annotate(count=Count("id")).order_by("-count")[:10]
                        ),
                        "routing_performance": routing_performance,
                    },
                },
            }
        )


class QRScansView(APIView):
    @require_auth
    def get(self, request, qr_id):
        qr = _visible_qr(request.auth_user["id"]).filter(id=qr_id).first()
        if not qr:
            return Response({"success": False, "error": "Not found"}, status=404)

        limit = min(max(int(request.query_params.get("limit", 50)), 1), 200)
        page = max(int(request.query_params.get("page", 1)), 1)

        rows = QRScan.objects.filter(qr_record_id=qr.id).order_by("-scanned_at")
        total = rows.count()
        window = rows[(page - 1) * limit : page * limit]

        return Response(
            {
                "success": True,
                "data": {
                    "scans": [
                        {
                            "id": str(row["id"]),
                            "scanned_at": row["scanned_at"].isoformat() if row["scanned_at"] else None,
                            "ip_address": row["ip_address"],
                            "country_code": row["country_code"],
                            "country_name": row["country_name"],
                            "city": row["city"],
                            "region": row["region"],
                            "device_type": row["device_type"],
                            "os": row["os"],
                            "browser": row["browser"],
                            "referrer": row["referrer"],
                            "language": row["language"],
                        }
                        for row in window.values(
                            "id", "scanned_at", "ip_address", "country_code", "country_name",
                            "city", "region", "device_type", "os", "browser", "referrer", "language",
                        )
                    ],
                    "total": total, "page": page, "limit": limit,
                    "pages": (total + limit - 1) // limit,
                },
            }
        )
