"""QR codes as the v1 dashboard creates, lists and edits them.

Mapping onto the v3 model:
- A *dynamic* code points at `/r/<short_code>` on the platform domain and
  its destination lives in a `QRVersion`; editing the destination adds a
  version, so the printed code never changes.
- A *static* code encodes its content directly (`static_payload`).
- v3 only allows dynamic codes whose destination is a web address (the
  schema forbids dynamic wifi/text/location/upi and needs a hosted page for
  vCard/event), so anything else is saved as static.
"""

import base64
import io
import re
import secrets
from datetime import datetime
from typing import Any
from urllib.parse import urlparse
from uuid import UUID

import segno
from django.conf import settings
from django.contrib.auth.hashers import make_password
from django.db import IntegrityError, transaction
from django.db.models import Max
from django.utils import timezone
from django.utils.dateparse import parse_datetime

from apps.accounts.models import User
from apps.core.errors import ApiError, unprocessable
from apps.workspaces.entitlements import limit_for
from apps.workspaces.models import Workspace

from .models import Domain, Folder, QRCode, QRCodeTag, QRVersion, Tag

# Crockford base32 without I, L, O, U (reference/go-v2/internal/shortcode).
SHORT_CODE_ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
SHORT_CODE_LENGTH = 7

# UI qr_type -> v3 content_type (the schema has no bitcoin type).
CONTENT_TYPES = {
    "url": "url",
    "text": "text",
    "email": "email",
    "phone": "phone",
    "sms": "sms",
    "whatsapp": "whatsapp",
    "wifi": "wifi",
    "vcard": "vcard",
    "event": "event",
    "upi": "upi",
    "location": "location",
    "bitcoin": "text",
}
DYNAMIC_TYPES = {"url", "whatsapp"}
# "https:", "mailto:", "tel:" ... but not "example.com:8080".
HAS_SCHEME = re.compile(r"^[a-z][a-z0-9+-]*:", re.IGNORECASE)
LOOKS_LIKE_HOST = re.compile(r"^(?:[a-z0-9-]+\.)+[a-z]{2,}$", re.IGNORECASE)
MAX_CONTENT_LENGTH = 4096


def short_code() -> str:
    return "".join(secrets.choice(SHORT_CODE_ALPHABET) for _ in range(SHORT_CODE_LENGTH))


def platform_host() -> str:
    """Host that serves /r/<code>: the web app, which forwards it to the API."""
    return urlparse(settings.APP_BASE_URL).netloc or "localhost"


def platform_domain() -> Domain:
    host = platform_host()
    domain = Domain.objects.filter(hostname=host).first()
    if domain is None:
        try:
            with transaction.atomic():
                domain = Domain.objects.create(
                    workspace=None,
                    hostname=host,
                    status="active",
                    tls_status="active",
                    verification_token=secrets.token_urlsafe(24),
                    verified_at=timezone.now(),
                )
        except IntegrityError:
            domain = Domain.objects.get(hostname=host)
    return domain


def short_url(code: str | None) -> str | None:
    if not code:
        return None
    return f"{settings.APP_BASE_URL}/r/{code}"


def is_web_url(value: str) -> bool:
    parsed = urlparse(value)
    return parsed.scheme in ("http", "https") and bool(parsed.netloc)


def with_scheme(value: str) -> str:
    """`example.com/menu` -> `https://example.com/menu`; anything else unchanged.

    People type addresses the way browsers let them; the scheme is only
    added to something that already looks like a host name.
    """
    if HAS_SCHEME.match(value) or any(ch.isspace() for ch in value):
        return value
    host = value.split("/")[0].split("?")[0].split(":")[0]
    if LOOKS_LIKE_HOST.match(host):
        return f"https://{value}"
    return value


def render_png_data_url(payload: str, size: int = 512, design: dict[str, Any] | None = None) -> str:
    """PNG of `payload` as a data: URL (the UI drops it straight into <img src>)."""
    design = design or {}
    dark = _color(design.get("foreground_color"), "#000000")
    background = design.get("background_color")
    light = None if background == "transparent" else _color(background, "#ffffff")
    qr = segno.make(payload, error="m", micro=False)
    # segno scales by whole modules; pick the scale that gets closest to `size`.
    modules = qr.symbol_size(border=2)[0]
    scale = max(1, round(size / modules))
    out = io.BytesIO()
    qr.save(out, kind="png", scale=scale, border=2, dark=dark, light=light)
    return "data:image/png;base64," + base64.b64encode(out.getvalue()).decode()


def _color(value: Any, default: str) -> str:
    if isinstance(value, str) and value.startswith("#") and len(value) in (4, 7):
        try:
            int(value[1:], 16)
            return value
        except ValueError:
            pass
    return default


def encoded_payload(qr: QRCode) -> str:
    """What the printed code contains."""
    if qr.mode == "dynamic":
        return short_url(qr.short_code) or ""
    return qr.static_payload or ""


def destination(qr: QRCode) -> str:
    if qr.mode == "dynamic":
        version = qr.current_version
        return (version.destination_url or "") if version else ""
    return qr.static_payload or ""


def is_expired(qr: QRCode, now: datetime | None = None) -> bool:
    now = now or timezone.now()
    if qr.expires_at and qr.expires_at <= now:
        return True
    return bool(qr.scan_limit and qr.total_scans >= qr.scan_limit)


def serialize(qr: QRCode, tags: list[str] | None = None) -> dict[str, Any]:
    design = qr.design if isinstance(qr.design, dict) else {}
    folder = qr.folder if qr.folder_id else None
    campaign = qr.campaign if qr.campaign_id else None
    content = destination(qr)
    return {
        "id": str(qr.id),
        "user_id": str(qr.created_by_id) if qr.created_by_id else None,
        "workspace_id": str(qr.workspace_id),
        "folder_id": str(qr.folder_id) if qr.folder_id else None,
        "folder_name": folder.name if folder else None,
        "campaign_id": str(qr.campaign_id) if qr.campaign_id else None,
        "campaign_name": campaign.name if campaign else None,
        "template_id": str(qr.template_id) if qr.template_id else None,
        "gs1": None,
        "title": qr.name,
        "content": content,
        "qr_type": design.get("qr_type") or qr.content_type,
        "size": int(design.get("size") or 512),
        "is_dynamic": qr.mode == "dynamic",
        "short_code": qr.short_code,
        "short_url": short_url(qr.short_code),
        "redirect_url": content if qr.mode == "dynamic" else None,
        "is_active": qr.status == "active",
        "is_expired": is_expired(qr),
        "has_password": bool(qr.password_hash),
        "scan_count": qr.total_scans,
        "unique_scans": qr.unique_scans,
        "last_scanned_at": qr.last_scanned_at.isoformat() if qr.last_scanned_at else None,
        "tags": tags if tags is not None else tag_names(qr),
        "metadata": {},
        "customization": design.get("customization") or {},
        "max_scans": qr.scan_limit,
        "expires_at": qr.expires_at.isoformat() if qr.expires_at else None,
        "geo_restrictions": None,
        "scheduled_at": qr.starts_at.isoformat() if qr.starts_at else None,
        "created_at": qr.created_at.isoformat(),
        "updated_at": qr.updated_at.isoformat(),
    }


def tag_names(qr: QRCode) -> list[str]:
    return sorted(
        str(name)
        for name in QRCodeTag.objects.filter(qr_code=qr).values_list("tag__name", flat=True)
    )


def _parse_when(value: Any, field: str) -> datetime | None:
    if value in (None, ""):
        return None
    parsed = parse_datetime(str(value))
    if parsed is None:
        raise unprocessable(code="invalid_date", detail=f"{field} must be an ISO 8601 date-time")
    if timezone.is_naive(parsed):
        parsed = timezone.make_aware(parsed)
    return parsed


def _parse_limit(value: Any) -> int | None:
    if value in (None, "", 0, "0"):
        return None
    try:
        limit = int(value)
    except (TypeError, ValueError):
        raise unprocessable(
            code="invalid_max_scans", detail="max_scans must be a whole number"
        ) from None
    if limit < 1:
        raise unprocessable(code="invalid_max_scans", detail="max_scans must be at least 1")
    return limit


def _clean_content(value: Any) -> str:
    content = str(value or "").strip()
    if not content:
        raise unprocessable(code="content_required", detail="content is required")
    if len(content) > MAX_CONTENT_LENGTH:
        raise unprocessable(code="content_too_long", detail="content is too long for a QR code")
    return content


def folder_in(workspace: Workspace, folder_id: Any) -> Folder | None:
    if not folder_id:
        return None
    try:
        return Folder.objects.get(id=UUID(str(folder_id)), workspace=workspace)
    except (ValueError, Folder.DoesNotExist):
        raise unprocessable(
            code="invalid_folder", detail="folder not found in this workspace"
        ) from None


def create(workspace: Workspace, user: User, body: dict[str, Any]) -> QRCode:
    """Create a QR code from the dashboard's create-page body."""
    in_use = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True).count()
    allowed = limit_for(workspace.plan_id, "max_qr_codes")
    if in_use >= allowed:
        raise ApiError(
            status=402,
            code="limit_reached",
            detail=f"your plan allows {allowed} QR codes; upgrade to create more",
            extra={"required_plan": "pro"},
        )

    qr_type = str(body.get("qr_type") or "url").lower()
    content_type = CONTENT_TYPES.get(qr_type, "text")
    content = _clean_content(body.get("content"))
    if content_type == "url":
        content = with_scheme(content)
    wants_dynamic = bool(body.get("is_dynamic", True))
    dynamic = wants_dynamic and content_type in DYNAMIC_TYPES and is_web_url(content)
    if content_type == "url" and not is_web_url(content):
        raise unprocessable(
            code="invalid_url", detail="enter a full web address starting with https://"
        )

    try:
        size = int(body.get("size") or 512)
    except (TypeError, ValueError):
        size = 512
    design: dict[str, Any] = {
        "qr_type": qr_type,
        "size": min(max(size, 50), 4096),
        "customization": body.get("customization")
        if isinstance(body.get("customization"), dict)
        else {},
    }
    password = str(body.get("password") or "")

    with transaction.atomic():
        qr = QRCode(
            workspace=workspace,
            created_by=user,
            mode="dynamic" if dynamic else "static",
            content_type=content_type,
            name=str(body.get("title") or "").strip()[:200] or "Untitled QR code",
            design=design,
            folder=folder_in(workspace, body.get("folder_id")),
            expires_at=_parse_when(body.get("expires_at"), "expires_at"),
            scan_limit=_parse_limit(body.get("max_scans")),
            password_hash=make_password(password) if password and dynamic else None,
            safety_status="safe",
        )
        if dynamic:
            qr.domain = platform_domain()
            qr.static_payload = None
            _save_with_unique_code(qr)
            version = QRVersion.objects.create(
                qr_code=qr,
                version_no=1,
                destination_kind="url",
                destination_url=content,
                safety_status="safe",
                created_by=user,
            )
            qr.current_version = version
            qr.save(update_fields=["current_version"])
        else:
            qr.static_payload = content
            qr.save()
    return qr


def _save_with_unique_code(qr: QRCode) -> None:
    for _ in range(8):
        qr.short_code = short_code()
        try:
            with transaction.atomic():
                qr.save()
            return
        except IntegrityError:
            if not QRCode.objects.filter(domain=qr.domain, short_code=qr.short_code).exists():
                raise
    raise ApiError(
        status=503, code="short_code_exhausted", detail="could not allocate a short link; try again"
    )


def update(qr: QRCode, user: User, body: dict[str, Any]) -> QRCode:
    """Apply the edit fields the dashboard sends; ignore the rest."""
    fields = ["updated_at"]
    if "title" in body:
        qr.name = str(body.get("title") or "").strip()[:200] or qr.name
        fields.append("name")
    new_destination = body.get("redirect_url") or body.get("content")
    if new_destination:
        content = _clean_content(new_destination)
        if qr.mode == "dynamic" or qr.content_type == "url":
            content = with_scheme(content)
        if qr.mode == "dynamic":
            if not is_web_url(content):
                raise unprocessable(
                    code="invalid_url", detail="enter a full web address starting with https://"
                )
            if content != destination(qr):
                last = QRVersion.objects.filter(qr_code=qr).aggregate(n=Max("version_no"))["n"] or 0
                qr.current_version = QRVersion.objects.create(
                    qr_code=qr,
                    version_no=last + 1,
                    destination_kind="url",
                    destination_url=content,
                    safety_status="safe",
                    created_by=user,
                )
                fields.append("current_version")
        else:
            qr.static_payload = content
            fields.append("static_payload")
    if "is_active" in body:
        qr.status = "active" if body.get("is_active") else "paused"
        fields.append("status")
    if "expires_at" in body:
        qr.expires_at = _parse_when(body.get("expires_at"), "expires_at")
        fields.append("expires_at")
    if "max_scans" in body:
        qr.scan_limit = _parse_limit(body.get("max_scans"))
        fields.append("scan_limit")
    if "password" in body and qr.mode == "dynamic":
        password = str(body.get("password") or "")
        qr.password_hash = make_password(password) if password else None
        fields.append("password_hash")
    if "tags" in body:
        set_tags(qr, body.get("tags"))
    qr.updated_at = timezone.now()
    qr.save(update_fields=fields)
    return qr


def parse_tags(raw: Any) -> list[str]:
    if isinstance(raw, str):
        raw = raw.split(",")
    if not isinstance(raw, list):
        return []
    return sorted({str(t).strip()[:50] for t in raw if str(t).strip()})


def set_tags(qr: QRCode, raw: Any, mode: str = "replace") -> None:
    names = parse_tags(raw)
    tags = [Tag.objects.get_or_create(workspace_id=qr.workspace_id, name=name)[0] for name in names]
    if mode == "replace":
        QRCodeTag.objects.filter(qr_code=qr).exclude(tag__in=tags).delete()
    if mode == "remove":
        QRCodeTag.objects.filter(qr_code=qr, tag__in=tags).delete()
        return
    existing = set(QRCodeTag.objects.filter(qr_code=qr).values_list("tag_id", flat=True))
    QRCodeTag.objects.bulk_create(
        [QRCodeTag(qr_code=qr, tag=tag) for tag in tags if tag.id not in existing]
    )
