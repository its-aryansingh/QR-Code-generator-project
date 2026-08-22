import uuid
import os
import secrets
from datetime import timedelta
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import Workspace, WorkspaceMember, WorkspaceInvite, Folder, AuditLog, User, QRRecord
from api.utils.auth import require_auth


def _slugify(name):
    import re
    slug = re.sub(r"[^a-z0-9-]", "", re.sub(r"\s+", "-", name.lower()))[:60]
    return f"{slug}-{secrets.token_hex(3)}"


def _ws_data(ws):
    return {
        "id": str(ws.id), "name": ws.name, "slug": ws.slug,
        "description": ws.description, "owner_id": str(ws.owner_id) if ws.owner_id else None,
        "brand_color": ws.brand_color, "brand_logo": ws.brand_logo,
        "favicon_url": ws.favicon_url, "custom_domain": ws.custom_domain,
        "remove_branding": ws.remove_branding,
        "created_at": ws.created_at.isoformat() if ws.created_at else None,
        "updated_at": ws.updated_at.isoformat() if ws.updated_at else None,
    }


class WorkspaceListView(APIView):
    @require_auth
    def get(self, request):
        user_id = request.auth_user["id"]
        workspaces = Workspace.objects.filter(
            owner_id=user_id
        ) | Workspace.objects.filter(
            members__user_id=user_id
        )
        workspaces = workspaces.distinct().order_by("-created_at")
        result = []
        for ws in workspaces:
            d = _ws_data(ws)
            d["member_count"] = WorkspaceMember.objects.filter(workspace_id=ws.id).count()
            d["qr_count"] = QRRecord.objects.filter(workspace_id=ws.id).count()
            result.append(d)
        return Response({"success": True, "data": result})

    @require_auth
    def post(self, request):
        name = request.data.get("name", "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)
        ws = Workspace(
            id=uuid.uuid4(),
            name=name,
            slug=_slugify(name),
            description=request.data.get("description"),
            owner_id=request.auth_user["id"],
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        ws.save(force_insert=True)
        WorkspaceMember(
            id=uuid.uuid4(),
            workspace_id=ws.id,
            user_id=request.auth_user["id"],
            role="owner",
            joined_at=timezone.now(),
        ).save(force_insert=True)
        return Response({"success": True, "data": _ws_data(ws)}, status=201)


class WorkspaceDetailView(APIView):
    def _get_ws(self, ws_id, user_id):
        try:
            ws = Workspace.objects.get(id=ws_id)
        except Workspace.DoesNotExist:
            return None
        if str(ws.owner_id) != user_id and not WorkspaceMember.objects.filter(workspace_id=ws_id, user_id=user_id).exists():
            return None
        return ws

    @require_auth
    def get(self, request, ws_id):
        ws = self._get_ws(ws_id, request.auth_user["id"])
        if not ws:
            return Response({"success": False, "error": "Not found"}, status=404)
        d = _ws_data(ws)
        members = list(WorkspaceMember.objects.filter(workspace_id=ws.id).select_related("user").values(
            "id", "role", "joined_at", "user__id", "user__email", "user__name", "user__avatar_url"
        ))
        d["workspace_members"] = members
        d["qr_count"] = QRRecord.objects.filter(workspace_id=ws.id).count()
        d["folder_count"] = Folder.objects.filter(workspace_id=ws.id).count()
        return Response({"success": True, "data": d})

    @require_auth
    def put(self, request, ws_id):
        try:
            ws = Workspace.objects.get(id=ws_id, owner_id=request.auth_user["id"])
        except Workspace.DoesNotExist:
            return Response({"success": False, "error": "Only the workspace owner can update it"}, status=403)
        allowed = ["name", "description", "brand_color", "brand_logo", "favicon_url",
                   "custom_css", "remove_branding", "custom_footer", "custom_domain"]
        updates = {k: v for k, v in request.data.items() if k in allowed}
        updates["updated_at"] = timezone.now()
        Workspace.objects.filter(pk=ws.pk).update(**updates)
        ws.refresh_from_db()
        return Response({"success": True, "data": _ws_data(ws)})

    @require_auth
    def delete(self, request, ws_id):
        try:
            ws = Workspace.objects.get(id=ws_id, owner_id=request.auth_user["id"])
        except Workspace.DoesNotExist:
            return Response({"success": False, "error": "Only the workspace owner can delete it"}, status=403)
        ws.delete()
        return Response({"success": True, "data": {"message": "Workspace deleted"}})


class WorkspaceInviteView(APIView):
    @require_auth
    def post(self, request, ws_id):
        try:
            ws = Workspace.objects.get(id=ws_id, owner_id=request.auth_user["id"])
        except Workspace.DoesNotExist:
            return Response({"success": False, "error": "Only workspace owner can invite members"}, status=403)
        email = request.data.get("email", "")
        role = request.data.get("role", "viewer")
        if not email or role not in ("admin", "editor", "viewer"):
            return Response({"success": False, "error": "Valid email and role required"}, status=400)
        token = secrets.token_hex(32)
        invite = WorkspaceInvite(
            id=uuid.uuid4(),
            workspace_id=ws.id,
            email=email,
            role=role,
            token=token,
            invited_by=request.auth_user["id"],
            expires_at=timezone.now() + timedelta(days=7),
        )
        invite.save(force_insert=True)
        app_base_url = os.environ.get("APP_BASE_URL", "http://localhost:3000")
        return Response({"success": True, "data": {
            "invite": {"id": str(invite.id), "email": email, "role": role, "token": token},
            "invite_url": f"{app_base_url}/invite/{token}",
        }}, status=201)


class WorkspaceAcceptInviteView(APIView):
    @require_auth
    def post(self, request, token):
        try:
            invite = WorkspaceInvite.objects.get(token=token)
        except WorkspaceInvite.DoesNotExist:
            return Response({"success": False, "error": "Invalid or expired invite"}, status=400)
        if invite.status != "pending" or invite.expires_at < timezone.now():
            return Response({"success": False, "error": "Invalid or expired invite"}, status=400)
        WorkspaceMember(
            id=uuid.uuid4(),
            workspace_id=invite.workspace_id,
            user_id=request.auth_user["id"],
            role=invite.role,
            invited_by=invite.invited_by,
            joined_at=timezone.now(),
        ).save(force_insert=True)
        WorkspaceInvite.objects.filter(pk=invite.pk).update(status="accepted")
        return Response({"success": True, "data": {"message": "Joined workspace", "workspace_id": str(invite.workspace_id)}})


class WorkspaceMembersView(APIView):
    @require_auth
    def get(self, request, ws_id):
        members = list(WorkspaceMember.objects.filter(workspace_id=ws_id).order_by("joined_at").values(
            "id", "role", "joined_at", "user__id", "user__email", "user__name", "user__avatar_url"
        ))
        return Response({"success": True, "data": members})


class WorkspaceMemberDetailView(APIView):
    @require_auth
    def put(self, request, ws_id, member_id):
        try:
            Workspace.objects.get(id=ws_id, owner_id=request.auth_user["id"])
        except Workspace.DoesNotExist:
            return Response({"success": False, "error": "Forbidden"}, status=403)
        role = request.data.get("role")
        if role not in ("admin", "editor", "viewer"):
            return Response({"success": False, "error": "Invalid role"}, status=400)
        WorkspaceMember.objects.filter(id=member_id).update(role=role)
        return Response({"success": True, "data": {"id": member_id, "role": role}})

    @require_auth
    def delete(self, request, ws_id, member_id):
        try:
            Workspace.objects.get(id=ws_id, owner_id=request.auth_user["id"])
        except Workspace.DoesNotExist:
            return Response({"success": False, "error": "Forbidden"}, status=403)
        WorkspaceMember.objects.filter(id=member_id).delete()
        return Response({"success": True, "data": {"message": "Member removed"}})


class WorkspaceFoldersView(APIView):
    @require_auth
    def get(self, request, ws_id):
        folders = Folder.objects.filter(workspace_id=ws_id, parent_id__isnull=True).order_by("sort_order", "name")
        result = []
        for f in folders:
            result.append({
                "id": str(f.id), "name": f.name, "description": f.description,
                "color": f.color, "icon": f.icon, "sort_order": f.sort_order,
                "qr_count": QRRecord.objects.filter(folder_id=f.id).count(),
                "children": list(Folder.objects.filter(parent_id=f.id).values("id", "name", "color", "icon")),
            })
        return Response({"success": True, "data": result})

    @require_auth
    def post(self, request, ws_id):
        name = request.data.get("name", "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)
        folder = Folder(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            name=name,
            description=request.data.get("description"),
            color=request.data.get("color", "#8B5CF6"),
            icon=request.data.get("icon"),
            parent_id=request.data.get("parent_id"),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        folder.save(force_insert=True)
        return Response({"success": True, "data": {
            "id": str(folder.id), "name": folder.name, "workspace_id": ws_id,
        }}, status=201)


class FolderDetailView(APIView):
    @require_auth
    def put(self, request, ws_id, folder_id):
        try:
            folder = Folder.objects.get(id=folder_id, workspace_id=ws_id)
        except Folder.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        allowed = ["name", "description", "color", "icon", "sort_order"]
        updates = {k: v for k, v in request.data.items() if k in allowed}
        updates["updated_at"] = timezone.now()
        Folder.objects.filter(pk=folder.pk).update(**updates)
        folder.refresh_from_db()
        return Response({"success": True, "data": {
            "id": str(folder.id), "name": folder.name, "color": folder.color,
        }})

    @require_auth
    def delete(self, request, ws_id, folder_id):
        try:
            folder = Folder.objects.get(id=folder_id, workspace_id=ws_id)
        except Folder.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        QRRecord.objects.filter(folder_id=folder_id).update(folder_id=None)
        folder.delete()
        return Response({"success": True, "data": {"message": "Folder deleted"}})


class WorkspaceAuditLogView(APIView):
    @require_auth
    def get(self, request, ws_id):
        limit = min(int(request.query_params.get("limit", 50)), 200)
        logs = list(AuditLog.objects.filter(workspace_id=ws_id).order_by("-created_at")[:limit].values(
            "id", "action", "resource", "resource_id", "details",
            "created_at", "user__id", "user__email", "user__name",
        ))
        return Response({"success": True, "data": logs})
