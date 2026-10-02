import uuid
import json
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import QRRecord
from api.utils.auth import require_auth, require_api_key
from api.utils.qr import generate_qr_base64, generate_short_code, is_premium_type


def _serialize_qr(qr):
    return {
        "id": str(qr.id),
        "user_id": str(qr.user_id) if qr.user_id else None,
        "workspace_id": str(qr.workspace_id) if qr.workspace_id else None,
        "folder_id": str(qr.folder_id) if qr.folder_id else None,
        "title": qr.title,
        "content": qr.content,
        "qr_type": qr.qr_type,
        "size": int(qr.size) if qr.size else 256,
        "is_dynamic": qr.is_dynamic,
        "short_code": qr.short_code,
        "redirect_url": qr.redirect_url,
        "is_active": qr.is_active,
        "scan_count": qr.scan_count,
        "tags": qr.tags,
        "metadata": qr.metadata,
        "customization": qr.customization,
        "max_scans": int(qr.max_scans) if qr.max_scans else None,
        "expires_at": qr.expires_at.isoformat() if qr.expires_at else None,
        "geo_restrictions": qr.geo_restrictions,
        "scheduled_at": qr.scheduled_at.isoformat() if qr.scheduled_at else None,
        "created_at": qr.created_at.isoformat() if qr.created_at else None,
        "updated_at": qr.updated_at.isoformat() if qr.updated_at else None,
    }


def _create_qr(user_id, data):
    content = data.get("content", "").strip()
    if not content:
        return None, "content is required"
    qr_type = data.get("qr_type", "url")
    size = max(50, min(int(data.get("size", 256)), 2048))
    is_dynamic = bool(data.get("is_dynamic", False))
    short_code = generate_short_code() if is_dynamic else None
    format = data.get("format", "png")

    qr_base64 = generate_qr_base64(content, size, format)

    qr = QRRecord(
        id=uuid.uuid4(),
        user_id=user_id,
        workspace_id=data.get("workspace_id"),
        folder_id=data.get("folder_id"),
        tags=data.get("tags"),
        title=data.get("title"),
        content=content,
        qr_type=qr_type,
        size=size,
        is_dynamic=is_dynamic,
        short_code=short_code,
        redirect_url=content if is_dynamic else None,
        metadata=data.get("metadata", {}),
        customization=data.get("customization", {}),
        max_scans=data.get("max_scans"),
        expires_at=data.get("expires_at"),
        geo_restrictions=data.get("geo_restrictions"),
        scheduled_at=data.get("scheduled_at"),
        created_at=timezone.now(),
        updated_at=timezone.now(),
    )
    qr.save(force_insert=True)
    result = _serialize_qr(qr)
    result["qr_base64"] = qr_base64
    return result, None


class GenerateView(APIView):
    @require_auth
    def post(self, request):
        d = request.data
        if not d.get("content"):
            return Response({"success": False, "error": "content is required"}, status=400)
        if is_premium_type(d.get("qr_type", "url")) and request.auth_user["plan"] == "free":
            return Response({"success": False, "error": "Upgrade your plan to use premium QR types."}, status=403)
        result, err = _create_qr(request.auth_user["id"], d)
        if err:
            return Response({"success": False, "error": err}, status=400)
        return Response({"success": True, "data": result}, status=201)


class ApiGenerateView(APIView):
    @require_api_key
    def post(self, request):
        d = request.data
        if not d.get("content"):
            return Response({"success": False, "error": "content is required"}, status=400)
        result, err = _create_qr(request.auth_user["id"], d)
        if err:
            return Response({"success": False, "error": err}, status=400)
        return Response({"success": True, "data": result}, status=201)


class HistoryView(APIView):
    @require_auth
    def get(self, request):
        limit = min(int(request.query_params.get("limit", 20)), 100)
        page = max(int(request.query_params.get("page", 1)), 1)
        qr_type = request.query_params.get("type")
        search = request.query_params.get("search")
        workspace_id = request.query_params.get("workspace_id")
        folder_id = request.query_params.get("folder_id")

        qs = QRRecord.objects.filter(user_id=request.auth_user["id"])
        if workspace_id:
            qs = qs.filter(workspace_id=workspace_id)
        if folder_id:
            qs = qs.filter(folder_id=folder_id)
        if qr_type:
            qs = qs.filter(qr_type=qr_type)
        if search:
            from django.db.models import Q
            qs = qs.filter(Q(title__icontains=search) | Q(content__icontains=search))

        total = qs.count()
        items = qs.order_by("-created_at")[(page - 1) * limit: (page - 1) * limit + limit]
        return Response({"success": True, "data": {
            "items": [_serialize_qr(q) for q in items],
            "total": total,
            "page": page,
            "limit": limit,
            "pages": (total + limit - 1) // limit,
        }})


class QRDetailView(APIView):
    @require_auth
    def get(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": _serialize_qr(qr)})

    @require_auth
    def put(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)

        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("title", "content", "redirect_url", "tags", "geo_restrictions"):
            if field in d:
                updates[field] = d[field]
        for field in ("is_active",):
            if field in d:
                updates[field] = bool(d[field])
        for field in ("metadata", "customization"):
            if field in d:
                updates[field] = d[field]
        if "folder_id" in d:
            updates["folder_id"] = d["folder_id"]
        if "max_scans" in d:
            updates["max_scans"] = d["max_scans"]
        if "expires_at" in d:
            updates["expires_at"] = d["expires_at"]

        QRRecord.objects.filter(pk=qr.pk).update(**updates)
        qr.refresh_from_db()
        return Response({"success": True, "data": _serialize_qr(qr)})

    @require_auth
    def delete(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        qr.delete()
        return Response({"success": True, "data": {"message": "QR code deleted"}})


class QRToggleView(APIView):
    @require_auth
    def put(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        QRRecord.objects.filter(pk=qr.pk).update(is_active=not qr.is_active, updated_at=timezone.now())
        qr.refresh_from_db()
        return Response({"success": True, "data": _serialize_qr(qr)})


class QRDownloadView(APIView):
    @require_auth
    def get(self, request, qr_id):
        try:
            qr = QRRecord.objects.get(id=qr_id, user_id=request.auth_user["id"])
        except QRRecord.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        size = max(50, min(int(request.query_params.get("size", int(qr.size or 256))), 2048))
        format = request.query_params.get("format", "png")
        b64 = generate_qr_base64(qr.content, size, format)
        return Response({"success": True, "data": {
            "qr_base64": b64,
            "filename": f"{qr.title or str(qr.id)}.{format}",
        }})
