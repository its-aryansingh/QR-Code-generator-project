"""Workspaces, members, invites, folders and the audit trail.

Every handler here is guarded by `require_workspace`, which replaced the
previous behaviour where any authenticated caller could read any workspace's
members, folders or audit log by guessing its UUID.
"""

import re
import secrets
import uuid
from datetime import timedelta

from django.db.models import Count, Q, Sum
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api import serializers as s
from api.models import (
    AuditLog,
    Campaign,
    Folder,
    QRRecord,
    User,
    Workspace,
    WorkspaceInvite,
    WorkspaceMember,
)
from api.utils import audit
from api.utils.auth import require_auth
from api.utils.entitlements import entitlements_payload, limit_exceeded
from api.utils.rbac import get_workspace_role, require_workspace
from django.conf import settings

ASSIGNABLE_ROLES = ("admin", "editor", "viewer")


def _slugify(name):
    slug = re.sub(r"[^a-z0-9-]", "", re.sub(r"\s+", "-", name.lower()))[:60] or "workspace"
    return f"{slug}-{secrets.token_hex(3)}"


def _counts_for(ws_ids):
    """Aggregate QR, scan, member and campaign counts for a set of workspaces."""
    qr = {
        row["workspace_id"]: row
        for row in QRRecord.objects.filter(workspace_id__in=ws_ids)
        .values("workspace_id")
        .annotate(qr_count=Count("id"), scan_count=Sum("scan_count"))
    }
    members = {
        row["workspace_id"]: row["n"]
        for row in WorkspaceMember.objects.filter(workspace_id__in=ws_ids)
        .values("workspace_id")
        .annotate(n=Count("id"))
    }
    campaigns = {
        row["workspace_id"]: row["n"]
        for row in Campaign.objects.filter(workspace_id__in=ws_ids)
        .values("workspace_id")
        .annotate(n=Count("id"))
    }
    folders = {
        row["workspace_id"]: row["n"]
        for row in Folder.objects.filter(workspace_id__in=ws_ids)
        .values("workspace_id")
        .annotate(n=Count("id"))
    }
    out = {}
    for ws_id in ws_ids:
        stats = qr.get(ws_id, {})
        out[ws_id] = {
            "qr_count": stats.get("qr_count", 0) or 0,
            "scan_count": int(stats.get("scan_count") or 0),
            "member_count": members.get(ws_id, 0),
            "campaign_count": campaigns.get(ws_id, 0),
            "folder_count": folders.get(ws_id, 0),
        }
    return out


class WorkspaceListView(APIView):
    @require_auth
    def get(self, request):
        user_id = request.auth_user["id"]
        workspaces = (
            Workspace.objects.filter(Q(owner_id=user_id) | Q(members__user_id=user_id))
            .distinct()
            .order_by("-created_at")
        )
        rows = list(workspaces)
        counts = _counts_for([w.id for w in rows])
        data = [
            s.workspace(w, role=get_workspace_role(user_id, w), counts=counts.get(w.id))
            for w in rows
        ]
        return Response({"success": True, "data": data})

    @require_auth
    def post(self, request):
        name = (request.data.get("name") or "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)

        user = User.objects.filter(id=request.auth_user["id"]).first()
        plan = (user.plan if user else "free") or "free"
        owned = Workspace.objects.filter(owner_id=request.auth_user["id"]).count()
        denial = limit_exceeded(plan, "max_workspaces", owned, "workspaces")
        if denial:
            return denial

        now = timezone.now()
        ws = Workspace(
            id=uuid.uuid4(),
            name=name,
            slug=_slugify(name),
            description=request.data.get("description"),
            owner_id=request.auth_user["id"],
            plan=plan,
            brand_color=request.data.get("brand_color") or "#8B5CF6",
            created_at=now,
            updated_at=now,
        )
        ws.save(force_insert=True)
        WorkspaceMember(
            id=uuid.uuid4(),
            workspace_id=ws.id,
            user_id=request.auth_user["id"],
            role="owner",
            joined_at=now,
        ).save(force_insert=True)

        if user and not user.default_workspace_id:
            User.objects.filter(pk=user.pk).update(default_workspace_id=ws.id)

        audit.record(request, ws.id, audit.CREATE, "workspace", ws.id, {"name": name})
        return Response(
            {"success": True, "data": s.workspace(ws, role="owner", counts=_counts_for([ws.id])[ws.id])},
            status=201,
        )


class WorkspaceDetailView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        ws = request.workspace
        data = s.workspace(ws, role=request.workspace_role, counts=_counts_for([ws.id])[ws.id])
        data["entitlements"] = entitlements_payload(ws.plan or "free")
        return Response({"success": True, "data": data})

    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id):
        allowed = [
            "name", "description", "brand_color", "brand_logo", "favicon_url",
            "custom_css", "remove_branding", "custom_footer", "logo_url",
        ]
        updates = {k: v for k, v in request.data.items() if k in allowed}
        if not updates:
            return Response({"success": False, "error": "No updatable fields supplied"}, status=400)
        if "remove_branding" in updates:
            updates["remove_branding"] = bool(updates["remove_branding"])
        updates["updated_at"] = timezone.now()

        Workspace.objects.filter(pk=request.workspace.pk).update(**updates)
        request.workspace.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "workspace", ws_id,
                     {"fields": sorted(k for k in updates if k != "updated_at")})
        return Response({"success": True, "data": s.workspace(request.workspace, role=request.workspace_role)})

    @require_auth
    @require_workspace("owner")
    def delete(self, request, ws_id):
        name = request.workspace.name
        # Detach QR codes rather than cascading them away -- a printed code
        # must keep resolving even after its workspace is dissolved.
        QRRecord.objects.filter(workspace_id=ws_id).update(workspace_id=None, folder_id=None)
        audit.record(request, ws_id, audit.DELETE, "workspace", ws_id, {"name": name})
        request.workspace.delete()
        return Response({"success": True, "data": {"message": "Workspace deleted"}})


class WorkspaceEntitlementsView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        ws = request.workspace
        counts = _counts_for([ws.id])[ws.id]
        payload = entitlements_payload(ws.plan or "free")
        payload["usage"] = {
            "qr_codes": counts["qr_count"],
            "members": counts["member_count"],
            "campaigns": counts["campaign_count"],
            "folders": counts["folder_count"],
        }
        payload["role"] = request.workspace_role
        return Response({"success": True, "data": payload})


# ------------------------------------------------------------------ members


class WorkspaceMembersView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        rows = (
            WorkspaceMember.objects.filter(workspace_id=ws_id)
            .select_related("user")
            .order_by("joined_at")
        )
        owner_id = request.workspace.owner_id
        me = request.auth_user["id"]
        return Response(
            {"success": True, "data": [s.member(r, me, owner_id) for r in rows]}
        )


class WorkspaceMemberDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id, member_id):
        role = request.data.get("role")
        if role not in ASSIGNABLE_ROLES:
            return Response(
                {"success": False, "error": f"role must be one of {', '.join(ASSIGNABLE_ROLES)}"},
                status=400,
            )
        row = WorkspaceMember.objects.filter(id=member_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Member not found"}, status=404)
        if str(row.user_id) == str(request.workspace.owner_id):
            return Response(
                {"success": False, "error": "The workspace owner's role cannot be changed"},
                status=400,
            )
        previous = row.role
        WorkspaceMember.objects.filter(pk=row.pk).update(role=role)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.ROLE_CHANGE, "member", row.id,
                     {"from": previous, "to": role, "user_id": str(row.user_id)})
        return Response(
            {"success": True, "data": s.member(row, request.auth_user["id"], request.workspace.owner_id)}
        )

    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, member_id):
        row = WorkspaceMember.objects.filter(id=member_id, workspace_id=ws_id).select_related("user").first()
        if not row:
            return Response({"success": False, "error": "Member not found"}, status=404)
        if str(row.user_id) == str(request.workspace.owner_id):
            return Response(
                {"success": False, "error": "The workspace owner cannot be removed"}, status=400
            )
        email = row.user.email if row.user else None
        row.delete()
        audit.record(request, ws_id, audit.REMOVE, "member", member_id, {"email": email})
        return Response({"success": True, "data": {"message": "Member removed"}})


# ------------------------------------------------------------------ invites


class WorkspaceInvitesView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        rows = WorkspaceInvite.objects.filter(workspace_id=ws_id).order_by("-created_at")
        base = settings.APP_BASE_URL
        return Response(
            {
                "success": True,
                "data": [
                    s.invite(r, f"{base}/invite/{r.token}" if r.status == "pending" else None)
                    for r in rows
                ],
            }
        )

    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id):
        email = (request.data.get("email") or "").strip().lower()
        role = request.data.get("role") or "viewer"
        if not email or "@" not in email:
            return Response({"success": False, "error": "A valid email is required"}, status=400)
        if role not in ASSIGNABLE_ROLES:
            return Response(
                {"success": False, "error": f"role must be one of {', '.join(ASSIGNABLE_ROLES)}"},
                status=400,
            )

        existing = User.objects.filter(email=email).values_list("id", flat=True).first()
        if existing and WorkspaceMember.objects.filter(workspace_id=ws_id, user_id=existing).exists():
            return Response(
                {"success": False, "error": "That person is already a member of this workspace"},
                status=409,
            )

        member_count = WorkspaceMember.objects.filter(workspace_id=ws_id).count()
        pending = WorkspaceInvite.objects.filter(workspace_id=ws_id, status="pending").count()
        denial = limit_exceeded(
            request.workspace.plan or "free", "max_members", member_count + pending, "seats"
        )
        if denial:
            return denial

        WorkspaceInvite.objects.filter(workspace_id=ws_id, email=email, status="pending").update(
            status="superseded"
        )

        token = secrets.token_urlsafe(32)
        row = WorkspaceInvite(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            email=email,
            role=role,
            token=token,
            invited_by=request.auth_user["id"],
            expires_at=timezone.now() + timedelta(days=7),
            created_at=timezone.now(),
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.INVITE, "invite", row.id, {"email": email, "role": role})
        return Response(
            {"success": True, "data": s.invite(row, f"{settings.APP_BASE_URL}/invite/{token}")},
            status=201,
        )


class WorkspaceInviteDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, invite_id):
        row = WorkspaceInvite.objects.filter(id=invite_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Invite not found"}, status=404)
        WorkspaceInvite.objects.filter(pk=row.pk).update(status="revoked")
        audit.record(request, ws_id, audit.REVOKE, "invite", invite_id, {"email": row.email})
        return Response({"success": True, "data": {"message": "Invite revoked"}})


class InvitePreviewView(APIView):
    """Unauthenticated: lets the invite landing page render before sign-in."""

    def get(self, request, token):
        row = WorkspaceInvite.objects.filter(token=token).first()
        if not row:
            return Response({"success": False, "error": "This invite link is not valid"}, status=404)
        expired = row.expires_at < timezone.now()
        ws = Workspace.objects.filter(id=row.workspace_id).first()
        inviter = User.objects.filter(id=row.invited_by).first()
        return Response(
            {
                "success": True,
                "data": {
                    "email": row.email,
                    "role": row.role,
                    "status": "expired" if expired else row.status,
                    "is_valid": row.status == "pending" and not expired,
                    "expires_at": row.expires_at.isoformat(),
                    "workspace": {
                        "id": str(ws.id),
                        "name": ws.name,
                        "brand_color": ws.brand_color,
                        "brand_logo": ws.brand_logo,
                    }
                    if ws
                    else None,
                    "invited_by": s.user_summary(inviter),
                },
            }
        )


class WorkspaceAcceptInviteView(APIView):
    @require_auth
    def post(self, request, token):
        row = WorkspaceInvite.objects.filter(token=token).first()
        if not row or row.status != "pending" or row.expires_at < timezone.now():
            return Response(
                {"success": False, "error": "This invite is no longer valid"}, status=400
            )

        user = User.objects.filter(id=request.auth_user["id"]).first()
        if user and user.email.lower() != row.email.lower():
            return Response(
                {
                    "success": False,
                    "error": f"This invite was sent to {row.email}. Sign in with that address to accept it.",
                    "code": "email_mismatch",
                },
                status=403,
            )

        existing = WorkspaceMember.objects.filter(
            workspace_id=row.workspace_id, user_id=request.auth_user["id"]
        ).first()
        if not existing:
            WorkspaceMember(
                id=uuid.uuid4(),
                workspace_id=row.workspace_id,
                user_id=request.auth_user["id"],
                role=row.role,
                invited_by=row.invited_by,
                joined_at=timezone.now(),
            ).save(force_insert=True)

        WorkspaceInvite.objects.filter(pk=row.pk).update(status="accepted")
        audit.record(request, row.workspace_id, audit.JOIN, "member", request.auth_user["id"],
                     {"role": row.role, "via": "invite"})
        ws = Workspace.objects.filter(id=row.workspace_id).first()
        return Response(
            {
                "success": True,
                "data": {
                    "message": "Joined workspace",
                    "workspace_id": str(row.workspace_id),
                    "role": row.role,
                    "workspace": s.workspace(ws, role=row.role) if ws else None,
                },
            }
        )


# ------------------------------------------------------------------ folders


class WorkspaceFoldersView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        rows = list(Folder.objects.filter(workspace_id=ws_id).order_by("sort_order", "name"))
        qr_counts = {
            str(r["folder_id"]): r["n"]
            for r in QRRecord.objects.filter(workspace_id=ws_id, folder_id__isnull=False)
            .values("folder_id")
            .annotate(n=Count("id"))
        }
        by_parent = {}
        for row in rows:
            by_parent.setdefault(str(row.parent_id) if row.parent_id else None, []).append(row)

        def build(parent_key):
            return [
                s.folder(
                    row,
                    qr_count=qr_counts.get(str(row.id), 0),
                    children=build(str(row.id)),
                )
                for row in by_parent.get(parent_key, [])
            ]

        return Response({"success": True, "data": build(None)})

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        name = (request.data.get("name") or "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)
        current = Folder.objects.filter(workspace_id=ws_id).count()
        denial = limit_exceeded(request.workspace.plan or "free", "max_folders", current, "folders")
        if denial:
            return denial

        now = timezone.now()
        row = Folder(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            name=name,
            description=request.data.get("description"),
            color=request.data.get("color") or "#8B5CF6",
            icon=request.data.get("icon"),
            parent_id=request.data.get("parent_id") or None,
            sort_order=int(request.data.get("sort_order") or 0),
            created_at=now,
            updated_at=now,
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "folder", row.id, {"name": name})
        return Response({"success": True, "data": s.folder(row)}, status=201)


class FolderDetailView(APIView):
    @require_auth
    @require_workspace("editor")
    def put(self, request, ws_id, folder_id):
        row = Folder.objects.filter(id=folder_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Folder not found"}, status=404)
        allowed = ["name", "description", "color", "icon", "sort_order", "parent_id"]
        updates = {k: v for k, v in request.data.items() if k in allowed}
        if str(updates.get("parent_id") or "") == str(folder_id):
            return Response({"success": False, "error": "A folder cannot be its own parent"}, status=400)
        updates["updated_at"] = timezone.now()
        Folder.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "folder", folder_id, {"name": row.name})
        return Response({"success": True, "data": s.folder(row)})

    @require_auth
    @require_workspace("editor")
    def delete(self, request, ws_id, folder_id):
        row = Folder.objects.filter(id=folder_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Folder not found"}, status=404)
        QRRecord.objects.filter(folder_id=folder_id).update(folder_id=None)
        Folder.objects.filter(parent_id=folder_id).update(parent_id=row.parent_id)
        name = row.name
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "folder", folder_id, {"name": name})
        return Response({"success": True, "data": {"message": "Folder deleted"}})


# ------------------------------------------------------------------ audit log


class WorkspaceAuditLogView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        q = request.query_params
        limit = min(max(int(q.get("limit", 50)), 1), 200)
        page = max(int(q.get("page", 1)), 1)

        rows = AuditLog.objects.filter(workspace_id=ws_id).select_related("user")
        if q.get("action"):
            rows = rows.filter(action=q["action"])
        if q.get("resource"):
            rows = rows.filter(resource=q["resource"])
        if q.get("user_id"):
            rows = rows.filter(user_id=q["user_id"])
        if q.get("days"):
            rows = rows.filter(created_at__gte=timezone.now() - timedelta(days=int(q["days"])))
        if q.get("search"):
            term = q["search"]
            rows = rows.filter(
                Q(action__icontains=term)
                | Q(resource__icontains=term)
                | Q(details__icontains=term)
                | Q(user__email__icontains=term)
            )

        total = rows.count()
        window = rows.order_by("-created_at")[(page - 1) * limit : page * limit]
        return Response(
            {
                "success": True,
                "data": {
                    "items": [s.audit_entry(r) for r in window],
                    "total": total,
                    "page": page,
                    "limit": limit,
                    "pages": (total + limit - 1) // limit,
                    "actions": sorted(
                        AuditLog.objects.filter(workspace_id=ws_id)
                        .values_list("action", flat=True)
                        .distinct()
                    ),
                    "resources": sorted(
                        AuditLog.objects.filter(workspace_id=ws_id)
                        .values_list("resource", flat=True)
                        .distinct()
                    ),
                },
            }
        )
