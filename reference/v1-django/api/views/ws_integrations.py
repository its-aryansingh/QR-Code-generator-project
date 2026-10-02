"""Workspace-scoped webhooks and lead capture.

The dashboard has always addressed these as
`/workspaces/{id}/webhooks` and `/workspaces/{id}/pages`, while the API only
exposed flat `/webhooks` and `/leads/pages` routes that took the workspace as
a query parameter and never checked membership.
"""

import hashlib
import hmac
import json
import re
import secrets
import uuid
from datetime import datetime, timedelta

from django.db.models import Count
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api import serializers as s
from api.models import Lead, LeadCapturePage, QRRecord, Webhook, WebhookLog
from api.utils import audit
from api.utils.auth import require_auth
from api.utils.entitlements import feature_denied
from api.utils.rbac import require_workspace

WEBHOOK_EVENTS = [
    {"id": "scan.created", "label": "QR code scanned",
     "description": "Fires on every scan, after routing has resolved."},
    {"id": "qr.created", "label": "QR code created",
     "description": "A new code was added to the workspace."},
    {"id": "qr.updated", "label": "QR code updated",
     "description": "A code's destination or settings changed."},
    {"id": "qr.deleted", "label": "QR code deleted",
     "description": "A code was removed."},
    {"id": "lead.captured", "label": "Lead captured",
     "description": "Someone submitted a lead capture form."},
    {"id": "campaign.completed", "label": "Campaign completed",
     "description": "A campaign reached its end date."},
    {"id": "member.joined", "label": "Member joined",
     "description": "Someone accepted a workspace invite."},
]


def _slugify(name):
    slug = re.sub(r"[^a-z0-9-]", "", re.sub(r"\s+", "-", (name or "").lower()))[:60] or "page"
    return f"{slug}-{secrets.token_hex(3)}"


class WorkspaceWebhookListView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        rows = Webhook.objects.filter(workspace_id=ws_id).order_by("-created_at")
        delivery = {
            row["webhook_id"]: row
            for row in WebhookLog.objects.filter(webhook__workspace_id=ws_id)
            .values("webhook_id")
            .annotate(total=Count("id"))
        }
        data = []
        for row in rows:
            item = s.webhook(row)
            item["delivery_count"] = delivery.get(row.id, {}).get("total", 0)
            data.append(item)
        return Response(
            {
                "success": True,
                "data": {
                    "webhooks": data,
                    "events": WEBHOOK_EVENTS,
                    "available": feature_denied(request.workspace.plan or "free", "webhooks") is None,
                },
            }
        )

    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "webhooks")
        if denial:
            return denial

        url = (request.data.get("url") or "").strip()
        events = request.data.get("events") or []
        if not url.startswith(("http://", "https://")):
            return Response({"success": False, "error": "Enter a valid http(s) endpoint URL"}, status=400)
        if isinstance(events, str):
            events = [events]
        valid = {e["id"] for e in WEBHOOK_EVENTS}
        unknown = [e for e in events if e not in valid]
        if not events:
            return Response({"success": False, "error": "Select at least one event"}, status=400)
        if unknown:
            return Response(
                {"success": False, "error": f"Unknown events: {', '.join(unknown)}",
                 "valid_events": sorted(valid)},
                status=400,
            )

        secret = f"whsec_{secrets.token_hex(24)}"
        now = timezone.now()
        row = Webhook(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            url=url,
            secret=secret,
            events=",".join(events),
            description=request.data.get("description"),
            is_active=bool(request.data.get("is_active", True)),
            created_at=now,
            updated_at=now,
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "webhook", row.id, {"url": url, "events": events})
        # The signing secret is returned once at creation.
        return Response({"success": True, "data": s.webhook(row, secret)}, status=201)


class WorkspaceWebhookDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id, wh_id):
        row = Webhook.objects.filter(id=wh_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Webhook not found"}, status=404)
        d = request.data
        updates = {"updated_at": timezone.now()}
        if "url" in d:
            if not str(d["url"]).startswith(("http://", "https://")):
                return Response({"success": False, "error": "Enter a valid http(s) endpoint URL"}, status=400)
            updates["url"] = d["url"]
        if "events" in d:
            events = d["events"]
            if isinstance(events, str):
                events = [events]
            valid = {e["id"] for e in WEBHOOK_EVENTS}
            unknown = [e for e in events if e not in valid]
            if unknown:
                return Response({"success": False, "error": f"Unknown events: {', '.join(unknown)}"}, status=400)
            updates["events"] = ",".join(events)
        if "description" in d:
            updates["description"] = d["description"]
        if "is_active" in d:
            updates["is_active"] = bool(d["is_active"])

        Webhook.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "webhook", wh_id, {"url": row.url})
        return Response({"success": True, "data": s.webhook(row)})

    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, wh_id):
        row = Webhook.objects.filter(id=wh_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Webhook not found"}, status=404)
        url = row.url
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "webhook", wh_id, {"url": url})
        return Response({"success": True, "data": {"message": "Webhook deleted"}})


class WorkspaceWebhookTestView(APIView):
    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id, wh_id):
        import urllib.error
        import urllib.request

        row = Webhook.objects.filter(id=wh_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Webhook not found"}, status=404)

        payload = json.dumps(
            {
                "event": "ping",
                "timestamp": datetime.utcnow().isoformat(),
                "data": {
                    "workspace_id": str(ws_id),
                    "message": "This is a test delivery from QRit.",
                },
            }
        ).encode()
        signature = hmac.new((row.secret or "").encode(), payload, hashlib.sha256).hexdigest()

        started = timezone.now()
        status_code, body, ok, error = 0, "", False, ""
        try:
            http_request = urllib.request.Request(
                row.url,
                data=payload,
                headers={
                    "Content-Type": "application/json",
                    "X-QRit-Event": "ping",
                    "X-QRit-Signature": f"sha256={signature}",
                },
                method="POST",
            )
            with urllib.request.urlopen(http_request, timeout=10) as response:
                status_code = response.status
                body = response.read(4096).decode("utf-8", "replace")
                ok = 200 <= status_code < 300
        except urllib.error.HTTPError as exc:
            status_code, error = exc.code, str(exc)
        except Exception as exc:
            error = str(exc)

        duration = int((timezone.now() - started).total_seconds() * 1000)
        WebhookLog(
            id=uuid.uuid4(), webhook_id=row.id, event="ping", payload=payload.decode(),
            status_code=status_code, response=body, success=ok, duration=duration,
            error=error, created_at=timezone.now(),
        ).save(force_insert=True)
        if ok:
            Webhook.objects.filter(pk=row.pk).update(last_triggered=timezone.now(), fail_count=0)

        audit.record(request, ws_id, audit.TEST, "webhook", wh_id, {"success": ok, "status": status_code})
        return Response(
            {
                "success": True,
                "data": {
                    "delivered": ok, "status_code": status_code,
                    "duration_ms": duration, "error": error,
                    "response_preview": body[:500],
                },
            }
        )


class WorkspaceWebhookLogsView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id, wh_id):
        if not Webhook.objects.filter(id=wh_id, workspace_id=ws_id).exists():
            return Response({"success": False, "error": "Webhook not found"}, status=404)
        limit = min(max(int(request.query_params.get("limit", 50)), 1), 200)
        rows = WebhookLog.objects.filter(webhook_id=wh_id).order_by("-created_at")[:limit]
        return Response({"success": True, "data": [s.webhook_log(r) for r in rows]})


# ------------------------------------------------------------------ leads


def _page_payload(row, lead_count=0):
    try:
        fields = json.loads(row.form_fields or "[]")
    except (ValueError, TypeError):
        fields = []
    return {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "qr_record_id": str(row.qr_record_id) if row.qr_record_id else None,
        "name": row.name,
        "slug": row.slug,
        "headline": row.headline,
        "subheadline": row.subheadline,
        "hero_image": row.hero_image,
        "button_text": row.button_text,
        "button_color": row.button_color,
        "background_color": row.background_color,
        "text_color": row.text_color,
        "thank_you_message": row.thank_you_message,
        "redirect_url": row.redirect_url,
        "form_fields": fields,
        "is_active": row.is_active,
        "requires_opt_in": row.requires_opt_in,
        "privacy_policy": row.privacy_policy,
        "consent_text": row.consent_text,
        "views": row.views,
        "submissions": row.submissions,
        "lead_count": lead_count,
        "conversion_rate": round(row.submissions / row.views * 100, 1) if row.views else 0.0,
        "created_at": row.created_at.isoformat() if row.created_at else None,
    }


class WorkspaceLeadPagesView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        rows = LeadCapturePage.objects.filter(workspace_id=ws_id).order_by("-created_at")
        counts = {
            row["page_id"]: row["n"]
            for row in Lead.objects.filter(page__workspace_id=ws_id)
            .values("page_id")
            .annotate(n=Count("id"))
        }
        return Response(
            {"success": True, "data": [_page_payload(r, counts.get(r.id, 0)) for r in rows]}
        )

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        d = request.data
        name = (d.get("name") or "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)

        fields = d.get("form_fields")
        if isinstance(fields, list):
            fields = json.dumps(fields)
        elif not fields:
            fields = json.dumps(
                [
                    {"name": "name", "label": "Full name", "type": "text", "required": True},
                    {"name": "email", "label": "Email", "type": "email", "required": True},
                ]
            )

        now = timezone.now()
        row = LeadCapturePage(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            qr_record_id=d.get("qr_record_id") or None,
            name=name,
            slug=_slugify(name),
            headline=d.get("headline"),
            subheadline=d.get("subheadline"),
            hero_image=d.get("hero_image"),
            button_text=d.get("button_text") or "Submit",
            button_color=d.get("button_color") or "#8B5CF6",
            background_color=d.get("background_color") or "#09090B",
            text_color=d.get("text_color") or "#FAFAFA",
            thank_you_message=d.get("thank_you_message") or "Thank you for signing up!",
            redirect_url=d.get("redirect_url"),
            form_fields=fields,
            requires_opt_in=bool(d.get("requires_opt_in")),
            privacy_policy=d.get("privacy_policy") or d.get("privacy_policy_url"),
            consent_text=d.get("consent_text"),
            created_at=now,
            updated_at=now,
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "lead_page", row.id, {"name": name})
        return Response({"success": True, "data": _page_payload(row)}, status=201)


class WorkspaceLeadPageDetailView(APIView):
    @require_auth
    @require_workspace("editor")
    def put(self, request, ws_id, page_id):
        row = LeadCapturePage.objects.filter(id=page_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Page not found"}, status=404)
        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("name", "headline", "subheadline", "hero_image", "button_text",
                      "button_color", "background_color", "text_color",
                      "thank_you_message", "redirect_url", "privacy_policy", "consent_text"):
            if field in d:
                updates[field] = d[field]
        if "form_fields" in d:
            updates["form_fields"] = (
                json.dumps(d["form_fields"]) if isinstance(d["form_fields"], list) else d["form_fields"]
            )
        for flag in ("is_active", "requires_opt_in"):
            if flag in d:
                updates[flag] = bool(d[flag])

        LeadCapturePage.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "lead_page", page_id, {"name": row.name})
        return Response({"success": True, "data": _page_payload(row)})

    @require_auth
    @require_workspace("editor")
    def delete(self, request, ws_id, page_id):
        row = LeadCapturePage.objects.filter(id=page_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Page not found"}, status=404)
        name = row.name
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "lead_page", page_id, {"name": name})
        return Response({"success": True, "data": {"message": "Page deleted"}})


class WorkspaceLeadsView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        limit = min(max(int(request.query_params.get("limit", 50)), 1), 200)
        page = max(int(request.query_params.get("page", 1)), 1)

        rows = Lead.objects.filter(page__workspace_id=ws_id).select_related("page")
        if request.query_params.get("page_id"):
            rows = rows.filter(page_id=request.query_params["page_id"])
        if request.query_params.get("days"):
            rows = rows.filter(
                created_at__gte=timezone.now() - timedelta(days=int(request.query_params["days"]))
            )
        if request.query_params.get("search"):
            rows = rows.filter(email__icontains=request.query_params["search"])

        total = rows.count()
        window = rows.order_by("-created_at")[(page - 1) * limit : page * limit]

        items = []
        for row in window:
            try:
                data = json.loads(row.data or "{}")
            except (ValueError, TypeError):
                data = {}
            items.append(
                {
                    "id": str(row.id),
                    "page_id": str(row.page_id),
                    "page_name": row.page.name if row.page else None,
                    "email": row.email,
                    "data": data,
                    "opted_in": row.opted_in,
                    "source": row.source,
                    "ip_address": row.ip_address,
                    "created_at": row.created_at.isoformat() if row.created_at else None,
                }
            )

        return Response(
            {
                "success": True,
                "data": {
                    "items": items, "total": total, "page": page, "limit": limit,
                    "pages": (total + limit - 1) // limit,
                },
            }
        )
