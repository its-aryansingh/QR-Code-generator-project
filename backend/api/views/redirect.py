import uuid
import threading
from django.utils import timezone
from django.http import HttpResponse, HttpResponseRedirect, HttpResponseNotFound, HttpResponseGone
from django.views import View
from api.models import QRRecord, QRScan
from api.utils.ip import get_client_ip


import json
import hmac
import hashlib
import urllib.request
import urllib.error
from datetime import datetime
from api.models import Webhook, WebhookLog

def _dispatch_webhooks(qr, scan):
    if not qr.workspace_id:
        return
    webhooks = Webhook.objects.filter(workspace_id=qr.workspace_id, is_active=True)
    if not webhooks.exists():
        return

    event = "scan.created"
    data = {
        "qr_id": str(qr.id),
        "short_code": qr.short_code,
        "scan_id": str(scan.id),
        "ip_address": scan.ip_address,
        "device_type": scan.device_type,
        "browser": scan.browser,
        "os": scan.os,
        "country": scan.country_code,
        "scanned_at": scan.scanned_at.isoformat() if scan.scanned_at else None,
    }

    payload = json.dumps({
        "event": event,
        "timestamp": datetime.utcnow().isoformat(),
        "data": data,
    }).encode()

    for wh in webhooks:
        if event not in (wh.events or "").split(","):
            continue

        sig = hmac.new((wh.secret or "").encode(), payload, hashlib.sha256).hexdigest()

        start = timezone.now()
        status_code, response_body, success, error = 0, "", False, ""
        try:
            req = urllib.request.Request(
                wh.url,
                data=payload,
                headers={"Content-Type": "application/json", "X-QRit-Signature": sig},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=5) as resp:
                status_code = resp.status
                response_body = resp.read().decode()
                success = 200 <= status_code < 300
        except urllib.error.HTTPError as e:
            status_code = e.code
            error = str(e)
        except Exception as e:
            error = str(e)

        duration = int((timezone.now() - start).total_seconds() * 1000)
        try:
            WebhookLog(
                id=uuid.uuid4(),
                webhook_id=wh.id,
                event=event,
                payload=payload.decode(),
                status_code=status_code,
                response=response_body,
                success=success,
                duration=duration,
                error=error,
                created_at=timezone.now(),
            ).save(force_insert=True)
        except Exception:
            pass

def _record_scan(qr, ip, user_agent, referrer, language):
    def run():
        try:
            from ua_parser import user_agent_parser
            ua = user_agent_parser.Parse(user_agent or "")
            device_family = ua["device"]["family"].lower()
            if device_family in ("tablet",):
                device_type = "tablet"
            elif device_family in ("spider",):
                device_type = "bot"
            elif ua["device"]["brand"] is not None:
                device_type = "mobile"
            else:
                device_type = "desktop"
            os_name = ua["os"]["family"]
            os_version = ua["os"]["major"]
            browser_name = ua["user_agent"]["family"]
            browser_version = ua["user_agent"]["major"]
        except Exception:
            device_type, os_name, os_version, browser_name, browser_version = "desktop", None, None, None, None

        try:
            scan = QRScan(
                id=uuid.uuid4(),
                qr_record_id=qr.id,
                ip_address=ip,
                device_type=device_type,
                os=os_name,
                os_version=os_version,
                browser=browser_name,
                browser_version=browser_version,
                user_agent=user_agent,
                referrer=referrer,
                language=language,
                scanned_at=timezone.now(),
            )
            scan.save(force_insert=True)
            from django.db.models import F
            QRRecord.objects.filter(id=qr.id).update(scan_count=F("scan_count") + 1)

            _dispatch_webhooks(qr, scan)
        except Exception:
            pass

    threading.Thread(target=run, daemon=True).start()


class DynamicRedirectView(View):
    def get(self, request, code):
        try:
            qr = QRRecord.objects.get(short_code=code)
        except QRRecord.DoesNotExist:
            return HttpResponseNotFound("QR code not found")

        if not qr.is_active:
            return HttpResponseNotFound("QR code not found")
        if qr.expires_at and qr.expires_at < timezone.now():
            return HttpResponseGone("QR code expired")
        if qr.max_scans and int(qr.scan_count or 0) >= int(qr.max_scans):
            return HttpResponseGone("QR code scan limit reached")

        _record_scan(
            qr=qr,
            ip=get_client_ip(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
            referrer=request.META.get("HTTP_REFERER"),
            language=(request.META.get("HTTP_ACCEPT_LANGUAGE") or "").split(",")[0],
        )

        return HttpResponseRedirect(qr.redirect_url or qr.content)
