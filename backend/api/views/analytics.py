from datetime import timedelta
from django.utils import timezone
from django.db import connection
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import QRRecord, QRScan
from api.utils.auth import require_auth


class DashboardView(APIView):
    @require_auth
    def get(self, request):
        user_id = request.auth_user["id"]
        days = max(1, int(request.query_params.get("days", 30)))
        since = timezone.now() - timedelta(days=days)

        total_qrs = QRRecord.objects.filter(user_id=user_id).count()
        active_qrs = QRRecord.objects.filter(user_id=user_id, is_active=True).count()
        total_scans = QRScan.objects.filter(qr_record__user_id=user_id, scanned_at__gte=since).count()

        recent_scans = list(QRScan.objects.filter(
            qr_record__user_id=user_id, scanned_at__gte=since
        ).order_by("-scanned_at")[:20].values(
            "scanned_at", "country_name", "device_type", "browser",
            "qr_record_id", "qr_record__title", "qr_record__qr_type",
        ))

        with connection.cursor() as cur:
            cur.execute("""
                SELECT DATE(s.scanned_at)::text AS date, COUNT(*) AS count
                FROM qr_scans s JOIN qr_records r ON s.qr_id = r.id
                WHERE r.user_id = %s::uuid AND s.scanned_at >= %s
                GROUP BY DATE(s.scanned_at) ORDER BY DATE(s.scanned_at)
            """, [user_id, since])
            scans_by_date = [{"date": row[0], "count": row[1]} for row in cur.fetchall()]

        return Response({"success": True, "data": {
            "total_qrs": total_qrs,
            "total_scans": total_scans,
            "active_qrs": active_qrs,
            "scans_by_date": scans_by_date,
            "recent_scans": recent_scans,
        }})


class QRAnalyticsView(APIView):
    @require_auth
    def get(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        days = max(1, int(request.query_params.get("days", 30)))
        since = timezone.now() - timedelta(days=days)
        qid = str(qr.id)

        total_scans = QRScan.objects.filter(qr_record_id=qid, scanned_at__gte=since).count()

        with connection.cursor() as cur:
            cur.execute("""
                SELECT DATE(scanned_at)::text, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY DATE(scanned_at) ORDER BY DATE(scanned_at)
            """, [qid, since])
            by_date = [{"date": r[0], "count": r[1]} for r in cur.fetchall()]

            cur.execute("""
                SELECT country_code, country_name, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY country_code, country_name ORDER BY 3 DESC LIMIT 10
            """, [qid, since])
            by_country = [{"country_code": r[0], "country_name": r[1], "count": r[2]} for r in cur.fetchall()]

            cur.execute("""
                SELECT device_type, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY device_type ORDER BY 2 DESC
            """, [qid, since])
            by_device = [{"device_type": r[0], "count": r[1]} for r in cur.fetchall()]

            cur.execute("""
                SELECT browser, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY browser ORDER BY 2 DESC LIMIT 10
            """, [qid, since])
            by_browser = [{"browser": r[0], "count": r[1]} for r in cur.fetchall()]

            cur.execute("""
                SELECT os, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY os ORDER BY 2 DESC LIMIT 10
            """, [qid, since])
            by_os = [{"os": r[0], "count": r[1]} for r in cur.fetchall()]

            cur.execute("""
                SELECT city, country_code, COUNT(*) FROM qr_scans
                WHERE qr_id=%s::uuid AND scanned_at>=%s GROUP BY city, country_code ORDER BY 3 DESC LIMIT 10
            """, [qid, since])
            top_cities = [{"city": r[0], "country_code": r[1], "count": r[2]} for r in cur.fetchall()]

        return Response({"success": True, "data": {
            "qr": {
                "id": str(qr.id), "title": qr.title, "qr_type": qr.qr_type,
                "content": qr.content, "is_dynamic": qr.is_dynamic, "is_active": qr.is_active,
            },
            "analytics": {
                "total_scans": total_scans,
                "scans_by_date": by_date,
                "scans_by_country": by_country,
                "scans_by_device": by_device,
                "scans_by_browser": by_browser,
                "scans_by_os": by_os,
                "top_cities": top_cities,
            },
        }})


class QRScansView(APIView):
    @require_auth
    def get(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        limit = min(int(request.query_params.get("limit", 50)), 200)
        page = max(int(request.query_params.get("page", 1)), 1)

        qs = QRScan.objects.filter(qr_record_id=qr.id).order_by("-scanned_at")
        total = qs.count()
        scans = list(qs[(page - 1) * limit: (page - 1) * limit + limit].values())
        return Response({"success": True, "data": {
            "scans": scans, "total": total, "page": page, "limit": limit,
            "pages": (total + limit - 1) // limit,
        }})
