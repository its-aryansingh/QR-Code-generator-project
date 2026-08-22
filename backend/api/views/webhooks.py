import uuid
import secrets
import hmac
import hashlib
import json
import threading
import urllib.request
import urllib.error
from datetime import datetime
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import Webhook, WebhookLog
from api.utils.auth import require_auth


class WebhookListView(APIView):
    @require_auth
    def get(self, request):
        workspace_id = request.query_params.get("workspace_id")
        if not workspace_id:
            return Response({"success": False, "error": "workspace_id required"}, status=400)
        webhooks = list(Webhook.objects.filter(workspace_id=workspace_id).order_by("-created_at").values(
            "id", "url", "events", "is_active", "description", "last_triggered", "fail_count", "created_at"
        ))
        return Response({"success": True, "data": webhooks})

    @require_auth
    def post(self, request):
        url = request.data.get("url", "")
        events = request.data.get("events", [])
        workspace_id = request.data.get("workspace_id", "")
        if not url or not events or not workspace_id:
            return Response({"success": False, "error": "url, events, and workspace_id are required"}, status=400)
        secret = secrets.token_hex(32)
        wh = Webhook(
            id=uuid.uuid4(),
            workspace_id=workspace_id,
            url=url,
            secret=secret,
            events=",".join(events) if isinstance(events, list) else events,
            description=request.data.get("description"),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        wh.save(force_insert=True)
        d = {
            "id": str(wh.id), "url": wh.url, "events": wh.events,
            "description": wh.description, "is_active": wh.is_active,
            "secret": secret,
        }
        return Response({"success": True, "data": d}, status=201)


class WebhookDetailView(APIView):
    @require_auth
    def put(self, request, wh_id):
        try:
            wh = Webhook.objects.get(id=wh_id)
        except Webhook.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        updates = {"updated_at": timezone.now()}
        if "url" in request.data:
            updates["url"] = request.data["url"]
        if "events" in request.data:
            ev = request.data["events"]
            updates["events"] = ",".join(ev) if isinstance(ev, list) else ev
        if "description" in request.data:
            updates["description"] = request.data["description"]
        if "is_active" in request.data:
            updates["is_active"] = bool(request.data["is_active"])
        Webhook.objects.filter(pk=wh.pk).update(**updates)
        wh.refresh_from_db()
        return Response({"success": True, "data": {"id": str(wh.id), "url": wh.url, "events": wh.events, "is_active": wh.is_active}})

    @require_auth
    def delete(self, request, wh_id):
        Webhook.objects.filter(id=wh_id).delete()
        return Response({"success": True, "data": {"message": "Webhook deleted"}})


class WebhookTestView(APIView):
    @require_auth
    def post(self, request, wh_id):
        try:
            wh = Webhook.objects.get(id=wh_id)
        except Webhook.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        payload = json.dumps({"event": "test", "timestamp": datetime.utcnow().isoformat(), "data": {}}).encode()
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
            with urllib.request.urlopen(req, timeout=10) as resp:
                status_code = resp.status
                response_body = resp.read().decode()
                success = 200 <= status_code < 300
        except urllib.error.HTTPError as e:
            status_code = e.code
            error = str(e)
        except Exception as e:
            error = str(e)

        duration = int((timezone.now() - start).total_seconds() * 1000)
        WebhookLog(
            id=uuid.uuid4(),
            webhook_id=wh.id,
            event="test",
            payload=payload.decode(),
            status_code=status_code,
            response=response_body,
            success=success,
            duration=duration,
            error=error,
            created_at=timezone.now(),
        ).save(force_insert=True)

        return Response({"success": True, "data": {
            "success": success, "status_code": status_code,
            "duration_ms": duration, "error": error,
        }})


class WebhookLogsView(APIView):
    @require_auth
    def get(self, request, wh_id):
        limit = min(int(request.query_params.get("limit", 50)), 200)
        logs = list(WebhookLog.objects.filter(webhook_id=wh_id).order_by("-created_at")[:limit].values())
        return Response({"success": True, "data": logs})
