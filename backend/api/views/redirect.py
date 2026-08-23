"""Dynamic QR resolution.

This is the hot path -- a person is standing in front of a printed code with
their phone out. Everything expensive (scan write, geo enrichment, webhook
dispatch) happens after the redirect has already been returned.
"""

import hashlib
import hmac
import json
import logging
import threading
import urllib.error
import urllib.request
import uuid
from datetime import datetime

from django.db.models import F
from django.http import (
    HttpResponse,
    HttpResponseGone,
    HttpResponseNotFound,
    HttpResponseRedirect,
)
from django.utils import timezone
from django.views import View

from api.models import QRRecord, QRScan, RoutingRule, Webhook, WebhookLog
from api.utils import geo, routing
from api.utils.ip import get_client_ip

logger = logging.getLogger(__name__)

PASSWORD_FORM = """<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Protected link</title>
<style>
  :root {{ color-scheme: light dark; }}
  body {{ margin:0; min-height:100vh; display:grid; place-items:center;
         font:16px/1.5 system-ui,-apple-system,Segoe UI,sans-serif;
         background:#09090b; color:#fafafa; padding:24px; }}
  .card {{ width:100%; max-width:360px; background:#18181b; border:1px solid #27272a;
          border-radius:16px; padding:28px; }}
  h1 {{ font-size:18px; margin:0 0 6px; }}
  p {{ margin:0 0 20px; color:#a1a1aa; font-size:14px; }}
  input {{ width:100%; box-sizing:border-box; padding:11px 13px; border-radius:9px;
          border:1px solid #3f3f46; background:#09090b; color:#fafafa; font-size:15px; }}
  button {{ width:100%; margin-top:12px; padding:11px; border:0; border-radius:9px;
           background:#8b5cf6; color:#fff; font-size:15px; font-weight:600; cursor:pointer; }}
  .err {{ color:#f87171; font-size:13px; margin:10px 0 0; }}
</style></head>
<body><form class="card" method="post">
  <h1>This code is password protected</h1>
  <p>Enter the password to continue.</p>
  <input type="password" name="password" placeholder="Password" autofocus required>
  <button type="submit">Continue</button>
  {error}
</form></body></html>"""


def _dispatch_webhooks(qr, scan, rule):
    if not qr.workspace_id:
        return
    event = "scan.created"
    webhooks = [
        wh
        for wh in Webhook.objects.filter(workspace_id=qr.workspace_id, is_active=True)
        if event in (wh.events or "").split(",")
    ]
    if not webhooks:
        return

    payload = json.dumps(
        {
            "event": event,
            "timestamp": datetime.utcnow().isoformat(),
            "data": {
                "qr_id": str(qr.id),
                "qr_title": qr.title,
                "workspace_id": str(qr.workspace_id),
                "short_code": qr.short_code,
                "scan_id": str(scan.id),
                "destination": rule.destination_url if rule else (qr.redirect_url or qr.content),
                "matched_rule": str(rule.id) if rule else None,
                "device_type": scan.device_type,
                "browser": scan.browser,
                "os": scan.os,
                "country": scan.country_code,
                "city": scan.city,
                "scanned_at": scan.scanned_at.isoformat() if scan.scanned_at else None,
            },
        }
    ).encode()

    for wh in webhooks:
        signature = hmac.new((wh.secret or "").encode(), payload, hashlib.sha256).hexdigest()
        started = timezone.now()
        status_code, body, ok, error = 0, "", False, ""
        try:
            request = urllib.request.Request(
                wh.url,
                data=payload,
                headers={
                    "Content-Type": "application/json",
                    "X-QRit-Event": event,
                    "X-QRit-Signature": f"sha256={signature}",
                },
                method="POST",
            )
            with urllib.request.urlopen(request, timeout=5) as response:
                status_code = response.status
                body = response.read(4096).decode("utf-8", "replace")
                ok = 200 <= status_code < 300
        except urllib.error.HTTPError as exc:
            status_code, error = exc.code, str(exc)
        except Exception as exc:
            error = str(exc)

        duration = int((timezone.now() - started).total_seconds() * 1000)
        try:
            WebhookLog(
                id=uuid.uuid4(), webhook_id=wh.id, event=event, payload=payload.decode(),
                status_code=status_code, response=body, success=ok, duration=duration,
                error=error, created_at=timezone.now(),
            ).save(force_insert=True)
            if ok:
                Webhook.objects.filter(pk=wh.pk).update(
                    last_triggered=timezone.now(), fail_count=0
                )
            else:
                Webhook.objects.filter(pk=wh.pk).update(fail_count=F("fail_count") + 1)
        except Exception:
            logger.exception("webhook log write failed")


def _parse_user_agent(user_agent):
    try:
        from ua_parser import user_agent_parser

        parsed = user_agent_parser.Parse(user_agent or "")
        family = (parsed["device"]["family"] or "").lower()
        if "tablet" in family or "ipad" in family:
            device_type = "tablet"
        elif "spider" in family or "bot" in family:
            device_type = "bot"
        elif parsed["device"]["brand"] is not None:
            device_type = "mobile"
        else:
            device_type = "desktop"
        return {
            "device_type": device_type,
            "os": parsed["os"]["family"],
            "os_version": parsed["os"]["major"],
            "browser": parsed["user_agent"]["family"],
            "browser_version": parsed["user_agent"]["major"],
        }
    except Exception:
        return {"device_type": "desktop", "os": None, "os_version": None,
                "browser": None, "browser_version": None}


def _record_scan(qr, context, rule):
    """Persist the scan off the request thread."""

    def run():
        try:
            scan = QRScan(
                id=uuid.uuid4(),
                qr_record_id=qr.id,
                ip_address=context["ip"],
                user_agent=context["user_agent"],
                referrer=context["referrer"],
                language=context["language"],
                scanned_at=context["now"],
                device_type=context["device_type"],
                os=context["os"],
                os_version=context["os_version"],
                browser=context["browser"],
                browser_version=context["browser_version"],
                country_code=context["country_code"],
                country_name=context["country_name"],
                city=context["city"],
                region=context["region"],
                latitude=context["latitude"],
                longitude=context["longitude"],
            )
            scan.save(force_insert=True)
            QRRecord.objects.filter(id=qr.id).update(scan_count=F("scan_count") + 1)
            if rule is not None:
                RoutingRule.objects.filter(id=rule.id).update(hit_count=F("hit_count") + 1)
            _dispatch_webhooks(qr, scan, rule)
        except Exception:
            logger.exception("scan record failed for qr %s", qr.id)

    threading.Thread(target=run, daemon=True).start()


def _build_context(request, qr):
    ip = get_client_ip(request)
    user_agent = request.META.get("HTTP_USER_AGENT")
    return {
        "now": timezone.now(),
        "ip": ip,
        "user_agent": user_agent,
        "referrer": request.META.get("HTTP_REFERER"),
        "language": (request.META.get("HTTP_ACCEPT_LANGUAGE") or "").split(",")[0],
        "scan_count": int(qr.scan_count or 0),
        **_parse_user_agent(user_agent),
        **geo.resolve(request, ip),
    }


def _geo_blocked(qr, context):
    """`geo_restrictions` is `allow:US,CA` or `block:RU,CN`."""
    raw = (qr.geo_restrictions or "").strip()
    if not raw:
        return False
    mode, _, codes = raw.partition(":")
    allowed = {c.strip().upper() for c in codes.split(",") if c.strip()}
    if not allowed:
        return False
    country = (context.get("country_code") or "").upper()
    if mode.lower() == "block":
        return country in allowed
    # allow-list: an unknown country fails open so a CDN without geo headers
    # does not silently break every scan
    return bool(country) and country not in allowed


class DynamicRedirectView(View):
    def get(self, request, code):
        return self._resolve(request, code, submitted_password=None)

    def post(self, request, code):
        return self._resolve(request, code, submitted_password=request.POST.get("password", ""))

    def _resolve(self, request, code, submitted_password):
        qr = QRRecord.objects.filter(short_code=code).first()
        if qr is None or not qr.is_active:
            return HttpResponseNotFound("QR code not found")

        now = timezone.now()
        if qr.scheduled_at and qr.scheduled_at > now:
            return HttpResponse("This QR code is not active yet", status=425)
        if qr.expires_at and qr.expires_at < now:
            return HttpResponseGone("QR code expired")
        if qr.max_scans and int(qr.scan_count or 0) >= int(qr.max_scans):
            return HttpResponseGone("QR code scan limit reached")

        if qr.password:
            if submitted_password is None:
                return HttpResponse(PASSWORD_FORM.format(error=""))
            if submitted_password != qr.password:
                return HttpResponse(
                    PASSWORD_FORM.format(error='<p class="err">Incorrect password</p>'), status=401
                )

        context = _build_context(request, qr)
        if _geo_blocked(qr, context):
            return HttpResponse("This QR code is not available in your region", status=451)

        rules = list(RoutingRule.objects.filter(qr_record_id=qr.id, is_active=True))
        destination, matched = routing.resolve_destination(qr, rules, context)

        _record_scan(qr, context, matched)

        if not destination:
            return HttpResponseNotFound("This QR code has no destination set")
        return HttpResponseRedirect(destination)
