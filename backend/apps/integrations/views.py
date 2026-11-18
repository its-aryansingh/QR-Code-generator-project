"""Webhook endpoints for the dashboard: list, create, edit, delete, test, delivery log.

Secrets are `whsec_` Standard Webhooks secrets, encrypted with the org's key
and shown once, when the webhook is created. Test deliveries go out through
the SSRF-safe client and are recorded in `webhook_deliveries` like any other.
"""

import base64
import hashlib
import hmac
import json
import secrets
import time
from typing import Any
from urllib.parse import urlsplit
from uuid import UUID

import httpx
from django.conf import settings
from django.db import transaction
from django.db.models import Count, Max
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.audit.record import changed_fields, record
from apps.core.crypto import get_keyring
from apps.core.errors import not_found, unprocessable
from apps.core.http import safe_client
from apps.core.ids import uuid7
from apps.workspaces.access import (
    DashboardAPIView,
    body,
    iso,
    ok,
    uuid_or_404,
    workspace_for,
)
from apps.workspaces.entitlements import plan_entitlements, require_feature
from apps.workspaces.models import Workspace

from .models import Webhook, WebhookDelivery

EVENTS = [
    ("qr.created", "QR code created", "A QR code was created in this workspace."),
    ("qr.updated", "QR code updated", "A QR code's settings or destination changed."),
    ("qr.deleted", "QR code deleted", "A QR code was deleted."),
    ("qr.version.activated", "Version activated", "A scheduled or approved version went live."),
    ("qr.safety.blocked", "Destination blocked", "A destination failed the safety check."),
    ("scan.created", "Scan", "Someone scanned a code (batched, up to 100 scans per delivery)."),
    ("lead.created", "Lead captured", "A lead form was submitted."),
    ("lead.withdrawn", "Lead withdrawn", "A lead withdrew consent."),
    ("approval.requested", "Approval requested", "A change is waiting for review."),
    ("approval.decided", "Approval decided", "A reviewer approved or rejected a change."),
    ("member.added", "Member added", "Someone joined the workspace."),
    ("member.removed", "Member removed", "Someone left or was removed from the workspace."),
]
EVENT_IDS = {event_id for event_id, _, _ in EVENTS}
TIMEOUT_SECONDS = 10.0
RESPONSE_PREVIEW_BYTES = 2048


def new_secret() -> str:
    return "whsec_" + base64.b64encode(secrets.token_bytes(24)).decode()


def sign(secret: str, msg_id: str, timestamp: int, payload: bytes) -> str:
    key = base64.b64decode(secret.removeprefix("whsec_"))
    mac = hmac.new(key, f"{msg_id}.{timestamp}.".encode() + payload, hashlib.sha256)
    return "v1," + base64.b64encode(mac.digest()).decode()


def _seal(workspace: Workspace, webhook_id: UUID, secret: str) -> bytes:
    keyring = get_keyring()
    if workspace.org_id:
        return keyring.encrypt(workspace.org_id, secret.encode())
    return keyring.seal_platform(secret.encode(), f"webhook:{webhook_id}")


def _open(webhook: Webhook, workspace: Workspace) -> str:
    keyring = get_keyring()
    if workspace.org_id:
        return keyring.decrypt(workspace.org_id, webhook.secret_ciphertext).decode()
    return keyring.open_platform(webhook.secret_ciphertext, f"webhook:{webhook.id}").decode()


def _clean_url(value: Any) -> str:
    url = str(value or "").strip()
    parts = urlsplit(url)
    allowed = ("https",) if settings.APP_ENV not in ("local", "test") else ("https", "http")
    if parts.scheme not in allowed or not parts.hostname:
        raise unprocessable(code="invalid_url", detail="the endpoint must be a full https:// URL")
    if len(url) > 2048:
        raise unprocessable(code="invalid_url", detail="the endpoint URL is too long")
    return url


def _clean_events(value: Any) -> list[str]:
    events = [str(e) for e in value] if isinstance(value, list) else []
    unknown = sorted(set(events) - EVENT_IDS)
    if unknown:
        raise unprocessable(code="invalid_events", detail=f"unknown events: {', '.join(unknown)}")
    if not events:
        raise unprocessable(code="events_required", detail="pick at least one event")
    return sorted(set(events))


def serialize(webhook: Webhook, stats: dict[str, Any] | None = None) -> dict[str, Any]:
    stats = stats or {}
    return {
        "id": str(webhook.id),
        "workspace_id": str(webhook.workspace_id),
        "url": webhook.url,
        "events": list(webhook.events),
        "is_active": webhook.is_active,
        "description": None,
        "last_triggered": iso(stats.get("last")),
        "fail_count": webhook.consecutive_failures,
        "delivery_count": stats.get("n", 0),
        "disabled_reason": webhook.disabled_reason,
        "signature_scheme": webhook.signature_scheme,
        "created_at": iso(webhook.created_at),
    }


def _stats(ids: list[UUID]) -> dict[UUID, dict[str, Any]]:
    return {
        row["webhook_id"]: {"n": row["n"], "last": row["last"]}
        for row in WebhookDelivery.objects.filter(webhook_id__in=ids)
        .values("webhook_id")
        .annotate(n=Count("id"), last=Max("created_at"))
    }


def _webhook_in(workspace: Workspace, webhook_id: str) -> Webhook:
    webhook = Webhook.objects.filter(
        workspace=workspace, id=uuid_or_404(webhook_id, "webhook")
    ).first()
    if webhook is None:
        raise not_found("webhook not found")
    return webhook


def _admin_workspace(view: DashboardAPIView, request: Request, ws: str) -> Workspace:
    workspace, _ = workspace_for(view.user(request), ws, min_role="admin")
    require_feature(workspace.plan_id, "webhooks")
    return workspace


class WorkspaceWebhookListView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        rows = list(Webhook.objects.filter(workspace=workspace).order_by("-created_at"))
        stats = _stats([w.id for w in rows])
        return ok(
            {
                "webhooks": [serialize(w, stats.get(w.id)) for w in rows],
                "events": [{"id": i, "label": label, "description": d} for i, label, d in EVENTS],
                "available": "webhooks" in plan_entitlements(workspace.plan_id)["features"],
            }
        )

    def post(self, request: Request, ws: str) -> Response:
        user = self.user(request)
        workspace = _admin_workspace(self, request, ws)
        data = body(request)
        url, events = _clean_url(data.get("url")), _clean_events(data.get("events"))
        secret = new_secret()
        webhook = Webhook(id=uuid7(), workspace=workspace, url=url, events=events, created_by=user)
        webhook.secret_ciphertext = _seal(workspace, webhook.id, secret)
        with transaction.atomic():
            webhook.save(force_insert=True)
            record(
                request, workspace, "create", "webhook", webhook.id, {"url": url, "events": events}
            )
        return ok({**serialize(webhook), "secret": secret}, status=201)


class WorkspaceWebhookDetailView(DashboardAPIView):
    def put(self, request: Request, ws: str, wh_id: str) -> Response:
        workspace = _admin_workspace(self, request, ws)
        webhook = _webhook_in(workspace, wh_id)
        data = body(request)
        before = {
            "url": webhook.url,
            "events": list(webhook.events),
            "is_active": webhook.is_active,
        }
        if "url" in data:
            webhook.url = _clean_url(data["url"])
        if "events" in data:
            webhook.events = _clean_events(data["events"])
        if "is_active" in data:
            webhook.is_active = bool(data["is_active"])
            if webhook.is_active:
                webhook.consecutive_failures = 0
                webhook.disabled_reason = None
        after = {"url": webhook.url, "events": list(webhook.events), "is_active": webhook.is_active}
        webhook.updated_at = timezone.now()
        with transaction.atomic():
            webhook.save()
            record(
                request,
                workspace,
                "update",
                "webhook",
                webhook.id,
                {"url": webhook.url, "fields": changed_fields(before, after)},
            )
        return ok(serialize(webhook, _stats([webhook.id]).get(webhook.id)))

    patch = put

    def delete(self, request: Request, ws: str, wh_id: str) -> Response:
        workspace = _admin_workspace(self, request, ws)
        webhook = _webhook_in(workspace, wh_id)
        with transaction.atomic():
            record(request, workspace, "delete", "webhook", webhook.id, {"url": webhook.url})
            webhook.delete()
        return ok({"message": "Webhook deleted"})


def deliver_test(workspace: Workspace, webhook: Webhook) -> dict[str, Any]:
    event_id = uuid7()
    payload = {
        "id": str(event_id),
        "type": "ping",
        "created_at": timezone.now().isoformat(),
        "workspace_id": str(workspace.id),
        "data": {"message": "Test delivery from QRit", "webhook_id": str(webhook.id)},
    }
    raw = json.dumps(payload, separators=(",", ":")).encode()
    timestamp = int(time.time())
    headers = {
        "content-type": "application/json",
        "user-agent": "QRit-Webhooks/3",
        "webhook-id": str(event_id),
        "webhook-timestamp": str(timestamp),
        "webhook-signature": sign(_open(webhook, workspace), str(event_id), timestamp, raw),
    }
    started = time.monotonic()
    status_code, error, preview = 0, "", ""
    try:
        with safe_client(timeout=TIMEOUT_SECONDS) as client:
            response = client.post(webhook.url, content=raw, headers=headers)
        status_code = response.status_code
        preview = response.content[:RESPONSE_PREVIEW_BYTES].decode("utf-8", "replace")
        if not 200 <= status_code < 300:
            error = f"endpoint answered HTTP {status_code}"
    except httpx.TimeoutException:
        error = f"no response within {int(TIMEOUT_SECONDS)} seconds"
    except (httpx.HTTPError, ValueError, OSError) as exc:
        error = f"could not connect: {exc}"[:300]
    duration_ms = int((time.monotonic() - started) * 1000)
    delivered = not error
    WebhookDelivery.objects.create(
        webhook=webhook,
        event_id=event_id,
        event_type="ping",
        payload=payload,
        status="succeeded" if delivered else "failed",
        attempts=1,
        last_status_code=status_code or None,
        last_error=error or None,
        delivered_at=timezone.now() if delivered else None,
    )
    return {
        "delivered": delivered,
        "status_code": status_code,
        "duration_ms": duration_ms,
        "error": error,
        "response_preview": preview,
    }


class WorkspaceWebhookTestView(DashboardAPIView):
    def post(self, request: Request, ws: str, wh_id: str) -> Response:
        workspace = _admin_workspace(self, request, ws)
        webhook = _webhook_in(workspace, wh_id)
        result = deliver_test(workspace, webhook)
        with transaction.atomic():
            record(
                request,
                workspace,
                "test",
                "webhook",
                webhook.id,
                {"url": webhook.url, "delivered": result["delivered"]},
            )
        return ok(result)


class WorkspaceWebhookLogsView(DashboardAPIView):
    def get(self, request: Request, ws: str, wh_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        webhook = _webhook_in(workspace, wh_id)
        rows = WebhookDelivery.objects.filter(webhook=webhook).order_by("-created_at")[:50]
        return ok(
            [
                {
                    "id": str(row.id),
                    "webhook_id": str(webhook.id),
                    "event": row.event_type,
                    "payload": json.dumps(row.payload),
                    "status_code": row.last_status_code or 0,
                    "response": None,
                    "success": row.status == "succeeded",
                    "duration_ms": 0,
                    "error": row.last_error,
                    "created_at": iso(row.created_at),
                }
                for row in rows
            ]
        )
