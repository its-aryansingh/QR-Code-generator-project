from datetime import date
from django.conf import settings
from django.db import connection
from rest_framework.views import APIView
from rest_framework.response import Response
from api.utils.qr import generate_qr_base64, is_premium_type
from api.utils.rate_limit import check_free_tier_limit
from api.utils.ip import get_client_ip
from api.models import QRRecord, QRScan, FreeTierUsage

QR_TYPES = [
    {"id": "url", "name": "Website", "description": "Link to any website URL", "icon": "globe", "category": "links", "is_premium": False},
    {"id": "text", "name": "Text", "description": "Plain text message", "icon": "file-text", "category": "basic", "is_premium": False},
    {"id": "wifi", "name": "WiFi", "description": "Connect to a Wi-Fi network", "icon": "wifi", "category": "technical", "is_premium": False},
    {"id": "vcard", "name": "vCard", "description": "Share a digital business card", "icon": "user-plus", "category": "business", "is_premium": False},
    {"id": "email", "name": "Email", "description": "Send an email", "icon": "mail", "category": "basic", "is_premium": False},
    {"id": "sms", "name": "SMS", "description": "Send a text message", "icon": "message-square", "category": "basic", "is_premium": False},
    {"id": "phone", "name": "Phone", "description": "Make a phone call", "icon": "phone", "category": "basic", "is_premium": False},
    {"id": "pdf", "name": "PDF", "description": "Show a PDF document", "icon": "file", "category": "media", "is_premium": True},
    {"id": "images", "name": "Images", "description": "Share multiple images", "icon": "image", "category": "media", "is_premium": True},
    {"id": "video", "name": "Video", "description": "Show a video", "icon": "video", "category": "media", "is_premium": True},
    {"id": "mp3", "name": "MP3", "description": "Share an audio file", "icon": "music", "category": "media", "is_premium": True},
    {"id": "facebook", "name": "Facebook", "description": "Share your Facebook page", "icon": "facebook", "category": "social", "is_premium": False},
    {"id": "instagram", "name": "Instagram", "description": "Share your Instagram", "icon": "instagram", "category": "social", "is_premium": False},
    {"id": "whatsapp", "name": "WhatsApp", "description": "Get WhatsApp messages", "icon": "message-circle", "category": "social", "is_premium": False},
    {"id": "social", "name": "Social Media", "description": "Share your social channels", "icon": "share-2", "category": "social", "is_premium": False},
    {"id": "apps", "name": "Apps", "description": "Redirect to an app store", "icon": "smartphone", "category": "technical", "is_premium": False},
    {"id": "menu", "name": "Menu", "description": "Create a restaurant menu", "icon": "menu", "category": "business", "is_premium": True},
    {"id": "coupon", "name": "Coupon", "description": "Share a coupon", "icon": "tag", "category": "business", "is_premium": True},
    {"id": "business", "name": "Business", "description": "Share business information", "icon": "briefcase", "category": "business", "is_premium": True},
    {"id": "links", "name": "List of Links", "description": "Share multiple links", "icon": "link", "category": "links", "is_premium": False},
]


class PublicGenerateView(APIView):
    def post(self, request):
        content = request.data.get("content", "")
        qr_type = request.data.get("qr_type", "url")
        size = max(50, min(int(request.data.get("size", 200)), 1000))
        format = request.data.get("format", "png")

        if is_premium_type(qr_type):
            return Response({"success": False, "error": "Premium QR type. Please sign up for access."}, status=403)

        rl = check_free_tier_limit(request)
        if not rl["allowed"]:
            return Response({"success": False, "error": "Daily limit reached. Sign up for unlimited QR codes!",
                             "data": {"remaining": 0, "limit": settings.FREE_TIER_DAILY_LIMIT}}, status=429)

        try:
            qr_base64 = generate_qr_base64(content or " ", min(size, 256), format)
        except Exception as e:
            return Response({"success": False, "error": str(e)}, status=400)

        return Response({
            "success": True,
            "qr_base64": qr_base64,
            "qr_type": qr_type,
            "size": size,
            "format": format,
            "remaining": rl["remaining"],
            "limit": settings.FREE_TIER_DAILY_LIMIT,
        })


class PublicTypesView(APIView):
    def get(self, request):
        return Response({"success": True, "data": QR_TYPES})


class PublicQuotaView(APIView):
    def get(self, request):
        ip = get_client_ip(request)
        today = date.today()
        try:
            usage = FreeTierUsage.objects.filter(ip_address=ip, date=today).first()
            count = int(usage.qr_count) if usage else 0
        except Exception:
            count = 0
        limit = settings.FREE_TIER_DAILY_LIMIT
        return Response({"success": True, "remaining": max(0, limit - count), "limit": limit})


class PublicAnalyticsView(APIView):
    def get(self, request, code):
        try:
            qr = QRRecord.objects.get(short_code=code)
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "QR code not found"}, status=404)

        if not qr.is_dynamic:
            return Response({"success": False, "error": "Analytics only available for dynamic QR codes"}, status=400)

        from datetime import timedelta
        from django.utils import timezone
        since = timezone.now() - timedelta(days=7)

        total_scans = QRScan.objects.filter(qr_id=qr.id, scanned_at__gte=since).count()
        recent_scans = list(QRScan.objects.filter(qr_id=qr.id).order_by("-scanned_at")[:10].values(
            "scanned_at", "country_name", "device_type", "browser"
        ))
        return Response({"success": True, "data": {
            "id": str(qr.id),
            "total_scans": total_scans,
            "recent_scans": recent_scans,
        }})
