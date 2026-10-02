import csv
from datetime import timedelta
from django.utils import timezone
from django.http import HttpResponse
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import QRRecord, QRScan, Lead
from api.utils.auth import require_auth


def _esc(v):
    return str(v or "")


class QRReportView(APIView):
    @require_auth
    def get(self, request):
        qrs = QRRecord.objects.filter(user_id=request.auth_user["id"]).order_by("-created_at").values(
            "id", "title", "qr_type", "content", "scan_count",
            "is_dynamic", "is_active", "created_at", "expires_at"
        )
        response = HttpResponse(content_type="text/csv")
        response["Content-Disposition"] = 'attachment; filename="qr-codes.csv"'
        w = csv.writer(response)
        w.writerow(["ID", "Title", "Type", "Content", "Scans", "Dynamic", "Active", "Created At", "Expires At"])
        for q in qrs:
            w.writerow([
                q["id"], q["title"], q["qr_type"], q["content"], q["scan_count"],
                q["is_dynamic"], q["is_active"],
                q["created_at"].isoformat() if q["created_at"] else "",
                q["expires_at"].isoformat() if q["expires_at"] else "",
            ])
        return response


class AnalyticsReportView(APIView):
    @require_auth
    def get(self, request):
        qr_id = request.query_params.get("qr_id")
        days = max(1, int(request.query_params.get("days", 30)))
        since = timezone.now() - timedelta(days=days)

        if qr_id:
            qs = QRScan.objects.filter(qr_record_id=qr_id, scanned_at__gte=since)
        else:
            qs = QRScan.objects.filter(qr_record__user_id=request.auth_user["id"], scanned_at__gte=since)

        scans = qs.order_by("-scanned_at").select_related("qr_record").values(
            "id", "scanned_at", "country_name", "country_code", "city",
            "device_type", "os", "browser", "referrer",
            "qr_record__id", "qr_record__title",
        )

        response = HttpResponse(content_type="text/csv")
        response["Content-Disposition"] = 'attachment; filename="analytics.csv"'
        w = csv.writer(response)
        w.writerow(["Scan ID", "Scanned At", "QR ID", "QR Title", "Country", "Country Code",
                    "City", "Device", "OS", "Browser", "Referrer"])
        for s in scans:
            w.writerow([
                s["id"],
                s["scanned_at"].isoformat() if s["scanned_at"] else "",
                s["qr_record__id"], s["qr_record__title"],
                s["country_name"], s["country_code"], s["city"],
                s["device_type"], s["os"], s["browser"], s["referrer"],
            ])
        return response


class LeadsReportView(APIView):
    @require_auth
    def get(self, request):
        page_id = request.query_params.get("page_id")
        if not page_id:
            return Response({"success": False, "error": "page_id required"}, status=400)
        leads = Lead.objects.filter(page_id=page_id).order_by("-created_at").values(
            "id", "email", "data", "opted_in", "created_at", "ip_address"
        )
        response = HttpResponse(content_type="text/csv")
        response["Content-Disposition"] = 'attachment; filename="leads.csv"'
        w = csv.writer(response)
        w.writerow(["ID", "Email", "Data", "Opted In", "IP Address", "Created At"])
        for l in leads:
            w.writerow([
                l["id"], l["email"], l["data"], l["opted_in"], l["ip_address"],
                l["created_at"].isoformat() if l["created_at"] else "",
            ])
        return response
