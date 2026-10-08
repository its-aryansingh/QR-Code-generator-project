"""`/r/<code>`: send a scan of a dynamic QR code to its destination and count it.

Gates, in order: unknown or deleted -> 404, paused -> 404, not started -> 425,
expired or over its scan limit -> 410, password -> a small form. Every
outcome is recorded in `scan_events`; only real redirects by non-bots count
towards `total_scans` (and `unique_scans` once per visitor per day).

The full redirect service (rules, caching, edge headers) is plan phase P4;
this is the minimum the dashboard needs for scans to work end to end.
"""

import hashlib
import logging
import re
from html import escape

from django.contrib.auth.hashers import check_password
from django.db import connection, transaction
from django.db.models import F, Q
from django.http import HttpRequest, HttpResponse, HttpResponseRedirect
from django.utils import timezone
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods

from apps.analytics.models import ScanEvent
from qrit.settings.env import env

from .models import QRCode

logger = logging.getLogger(__name__)

BOT = re.compile(
    r"bot|crawl|spider|slurp|preview|facebookexternalhit|curl|wget|python-requests|headless", re.I
)
TABLET = re.compile(r"ipad|tablet|(android(?!.*mobile))", re.I)
MOBILE = re.compile(r"mobi|iphone|ipod|android", re.I)
DESKTOP = re.compile(r"windows|macintosh|x11|linux|cros", re.I)


def device_type(user_agent: str) -> str:
    if TABLET.search(user_agent):
        return "tablet"
    if MOBILE.search(user_agent):
        return "mobile"
    if DESKTOP.search(user_agent):
        return "desktop"
    return "other"


def client_ip(request: HttpRequest) -> str:
    forwarded = str(request.META.get("HTTP_X_FORWARDED_FOR") or "")
    if forwarded:
        return forwarded.split(",")[0].strip()
    return str(request.META.get("REMOTE_ADDR") or "")


def record_scan(request: HttpRequest, qr: QRCode, outcome: str) -> None:
    """Best effort: a failure here must never stop the redirect."""
    try:
        now = timezone.now()
        user_agent = request.META.get("HTTP_USER_AGENT", "")[:512]
        is_bot = bool(BOT.search(user_agent)) or not user_agent
        day = now.date()
        visitor_hash = hashlib.sha256(
            f"{env.SCAN_SALT_SECRET}|{day.isoformat()}|{client_ip(request)}|{user_agent}".encode()
        ).digest()
        counted = outcome == "redirect" and not is_bot
        with transaction.atomic():
            is_unique = False
            if counted:
                with connection.cursor() as cursor:
                    cursor.execute(
                        "INSERT INTO scan_visitors_daily (qr_code_id, day, visitor_hash, first_seen_at) "
                        "VALUES (%s, %s, %s, %s) ON CONFLICT DO NOTHING",
                        [qr.id, day, visitor_hash, now],
                    )
                    is_unique = cursor.rowcount == 1
            referrer = request.META.get("HTTP_REFERER", "")
            ScanEvent.objects.create(
                occurred_at=now,
                workspace_id=qr.workspace_id,
                qr_code_id=qr.id,
                version_id=qr.current_version_id,
                campaign_id=qr.campaign_id,
                domain_id=qr.domain_id or qr.workspace_id,
                outcome=outcome,
                method=request.method or "GET",
                is_bot=is_bot,
                bot_reason="user_agent" if is_bot else None,
                is_unique=is_unique,
                visitor_hash=visitor_hash,
                device_type=device_type(user_agent),
                language=(request.META.get("HTTP_ACCEPT_LANGUAGE", "").split(",")[0] or None),
                referrer_host=(referrer.split("/")[2][:255] if referrer.count("/") >= 2 else None),
            )
            if counted:
                QRCode.objects.filter(id=qr.id).update(
                    total_scans=F("total_scans") + 1,
                    unique_scans=F("unique_scans") + (1 if is_unique else 0),
                    last_scanned_at=now,
                )
    except Exception:  # noqa: BLE001 - analytics must not break redirects
        logger.exception("could not record scan for %s", qr.id)


def page(title: str, message: str, status: int, form: str = "") -> HttpResponse:
    html = f"""<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex">
<title>{escape(title)}</title>
<style>body{{margin:0;min-height:100vh;display:grid;place-items:center;background:#09090b;color:#e4e4e7;
font:16px/1.5 system-ui,sans-serif}}main{{max-width:22rem;padding:2rem;text-align:center}}
h1{{font-size:1.25rem;margin:0 0 .5rem}}p{{color:#a1a1aa;margin:0 0 1rem}}
input,button{{width:100%;box-sizing:border-box;padding:.75rem;border-radius:.75rem;font:inherit}}
input{{border:1px solid #3f3f46;background:#18181b;color:#fafafa;margin-bottom:.75rem}}
button{{border:0;background:#7c3aed;color:#fff;font-weight:600;cursor:pointer}}</style></head>
<body><main><h1>{escape(title)}</h1><p>{escape(message)}</p>{form}</main></body></html>"""
    response = HttpResponse(html, status=status)
    response["Cache-Control"] = "no-store"
    return response


PASSWORD_FORM = (
    '<form method="post"><input type="password" name="password" placeholder="Password" '
    'autocomplete="current-password" required autofocus><button type="submit">Continue</button></form>'
)


@csrf_exempt  # a public password gate: there is no session to protect
@require_http_methods(["GET", "HEAD", "POST"])
def scan(request: HttpRequest, code: str) -> HttpResponse:
    code = code.strip().upper()
    qr = (
        QRCode.objects.select_related("current_version")
        .filter(
            Q(short_code=code) | Q(legacy_short_code=code),
            mode="dynamic",
            deleted_at__isnull=True,
        )
        .first()
    )
    if qr is None:
        return page("QR code not found", "This code doesn't exist or has been deleted.", 404)

    now = timezone.now()
    if qr.status != "active":
        record_scan(request, qr, "paused")
        return page("QR code paused", "The owner has paused this QR code.", 404)
    if qr.starts_at and qr.starts_at > now:
        record_scan(request, qr, "not_started")
        return page("Not active yet", "This QR code isn't live yet. Try again later.", 425)
    if qr.expires_at and qr.expires_at <= now:
        record_scan(request, qr, "expired")
        return page("QR code expired", "This QR code is no longer active.", 410)
    if qr.scan_limit and qr.total_scans >= qr.scan_limit:
        record_scan(request, qr, "limit_reached")
        return page("QR code expired", "This QR code has reached its scan limit.", 410)

    if qr.password_hash:
        if request.method != "POST":
            return page("Password required", "Enter the password to continue.", 200, PASSWORD_FORM)
        if not check_password(request.POST.get("password", ""), qr.password_hash):
            record_scan(request, qr, "password_fail")
            return page(
                "Wrong password", "That password isn't right. Try again.", 401, PASSWORD_FORM
            )

    target = qr.current_version.destination_url if qr.current_version else None
    if not target:
        return page("QR code not found", "This code has no destination yet.", 404)
    record_scan(request, qr, "redirect")
    response = HttpResponseRedirect(target)
    response["Cache-Control"] = "no-store"
    response["Referrer-Policy"] = "no-referrer"
    return response
