"""Team endpoints for the dashboard: members, roles, invites, and accepting an invite.

The dashboard knows four roles (owner/admin/editor/viewer); "viewer" is stored
as v3's read-only "analyst". Invite links carry a random token; only its
SHA-256 is stored, so the link is shown once, when the invite is created.
Seats are members plus pending invites, so the counts on the Team page always
add up to what the seat limit checks.
"""

import hashlib
import secrets
from datetime import timedelta
from typing import Any

from django.conf import settings
from django.core.exceptions import ValidationError
from django.core.validators import validate_email
from django.db import transaction
from django.db.models import Q, QuerySet
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.accounts.models import User
from apps.audit.record import record
from apps.core.errors import ApiError, conflict, forbidden, not_found, unprocessable

from .access import (
    ROLE_RANK,
    DashboardAPIView,
    PublicDashboardAPIView,
    body,
    iso,
    ok,
    require_role,
    ui_role,
    user_summary,
    uuid_or_404,
    workspace_for,
)
from .entitlements import require_capacity
from .models import Invite, Workspace, WorkspaceMember
from .views import serialize_workspace

INVITE_TTL = timedelta(days=7)
TO_STORED = {"admin": "admin", "editor": "editor", "viewer": "analyst"}


def _stored_role(value: Any) -> str:
    role = TO_STORED.get(str(value or ""))
    if role is None:
        raise unprocessable(code="invalid_role", detail="role must be admin, editor or viewer")
    return role


def _token_hash(token: str) -> bytes:
    return hashlib.sha256(token.encode()).digest()


def _pending(workspace: Workspace) -> QuerySet[Invite]:
    return Invite.objects.filter(
        workspace=workspace,
        accepted_at__isnull=True,
        revoked_at__isnull=True,
        expires_at__gt=timezone.now(),
    )


def seats_used(workspace: Workspace) -> int:
    return WorkspaceMember.objects.filter(workspace=workspace).count() + _pending(workspace).count()


def invite_status(invite: Invite) -> str:
    if invite.accepted_at:
        return "accepted"
    if invite.revoked_at:
        return "revoked"
    if invite.expires_at <= timezone.now():
        return "expired"
    return "pending"


def serialize_invite(invite: Invite, token: str | None = None) -> dict[str, Any]:
    return {
        "id": str(invite.id),
        "email": str(invite.email),
        "role": ui_role(invite.role),
        "status": invite_status(invite),
        "expires_at": iso(invite.expires_at),
        "created_at": iso(invite.created_at),
        "invited_by": str(invite.invited_by_id),
        "invite_url": f"{settings.APP_BASE_URL}/invite/{token}" if token else None,
    }


def _members(workspace: Workspace, viewer: User) -> list[dict[str, Any]]:
    rows = list(
        WorkspaceMember.objects.filter(workspace=workspace)
        .select_related("user")
        .order_by("created_at")
    )
    if not any(row.user_id == workspace.owner_id for row in rows):
        rows.insert(
            0,
            WorkspaceMember(
                workspace=workspace,
                user=workspace.owner,
                role="owner",
                created_at=workspace.created_at,
            ),
        )
    rows.sort(key=lambda row: (-ROLE_RANK.get(row.role, 0), row.created_at))
    return [
        {
            "id": str(row.user_id),
            "role": "owner" if row.user_id == workspace.owner_id else ui_role(row.role),
            "joined_at": iso(row.created_at),
            "invited_by": None,
            "is_you": row.user_id == viewer.id,
            "user": user_summary(row.user),
        }
        for row in rows
    ]


class WorkspaceMembersView(DashboardAPIView):
    list_view = True

    def get(self, request: Request, ws: str) -> Response:
        user = self.user(request)
        workspace, _ = workspace_for(user, ws)
        return ok(_members(workspace, user))


def _member_in(workspace: Workspace, member_id: str) -> WorkspaceMember:
    uid = uuid_or_404(member_id, "member")
    row = WorkspaceMember.objects.filter(workspace=workspace, user_id=uid).select_related("user")
    member = row.first()
    if member is None:
        raise not_found("member not found")
    return member


class WorkspaceMemberDetailView(DashboardAPIView):
    def put(self, request: Request, ws: str, member_id: str) -> Response:
        actor = self.user(request)
        workspace, actor_role = workspace_for(actor, ws, min_role="admin")
        member = _member_in(workspace, member_id)
        new_role = _stored_role(body(request).get("role"))
        if member.user_id == workspace.owner_id or member.role == "owner":
            raise forbidden(code="owner_role_fixed", detail="the owner's role can't be changed")
        if member.user_id == actor.id:
            raise forbidden(code="own_role_fixed", detail="you can't change your own role")
        if "admin" in (new_role, member.role):
            require_role(actor_role, "owner")
        previous = member.role
        if previous != new_role:
            with transaction.atomic():
                WorkspaceMember.objects.filter(workspace=workspace, user_id=member.user_id).update(
                    role=new_role
                )
                record(
                    request,
                    workspace,
                    "role_change",
                    "member",
                    member.user_id,
                    {
                        "email": str(member.user.email),
                        "from": ui_role(previous),
                        "to": ui_role(new_role),
                    },
                )
        member.role = new_role
        return ok(next(m for m in _members(workspace, actor) if m["id"] == str(member.user_id)))

    def delete(self, request: Request, ws: str, member_id: str) -> Response:
        actor = self.user(request)
        leaving = str(member_id) == str(actor.id)
        workspace, actor_role = workspace_for(actor, ws, min_role="analyst" if leaving else "admin")
        member = _member_in(workspace, member_id)
        if member.user_id == workspace.owner_id or member.role == "owner":
            raise forbidden(code="owner_cannot_leave", detail="the owner can't be removed")
        if member.role == "admin" and not leaving:
            require_role(actor_role, "owner")
        with transaction.atomic():
            record(
                request,
                workspace,
                "remove",
                "member",
                member.user_id,
                {"email": str(member.user.email), "role": ui_role(member.role)},
            )
            WorkspaceMember.objects.filter(workspace=workspace, user_id=member.user_id).delete()
        return ok({"message": "Member removed"})


class WorkspaceInvitesView(DashboardAPIView):
    list_view = True

    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        recent = timezone.now() - timedelta(days=30)
        rows = Invite.objects.filter(workspace=workspace).filter(
            Q(accepted_at__isnull=True, revoked_at__isnull=True) | Q(created_at__gte=recent)
        )
        return ok([serialize_invite(invite) for invite in rows.order_by("-created_at")[:200]])

    def post(self, request: Request, ws: str) -> Response:
        actor = self.user(request)
        workspace, actor_role = workspace_for(actor, ws, min_role="admin")
        data = body(request)
        email = str(data.get("email") or "").strip().lower()
        try:
            validate_email(email)
        except ValidationError:
            raise unprocessable(
                code="invalid_email", detail="enter a valid email address"
            ) from None
        role = _stored_role(data.get("role") or "editor")
        if role == "admin":
            require_role(actor_role, "owner")
        if WorkspaceMember.objects.filter(workspace=workspace, user__email__iexact=email).exists():
            raise conflict(code="already_member", detail=f"{email} is already in this workspace")
        open_invites = Invite.objects.filter(
            workspace=workspace,
            email__iexact=email,
            accepted_at__isnull=True,
            revoked_at__isnull=True,
        )
        replacing = open_invites.filter(expires_at__gt=timezone.now()).count()
        require_capacity(
            workspace.plan_id, "max_members", seats_used(workspace) - replacing, "seats"
        )
        token = secrets.token_urlsafe(32)
        with transaction.atomic():
            open_invites.update(revoked_at=timezone.now())
            invite = Invite.objects.create(
                workspace=workspace,
                email=email,
                role=role,
                token_hash=_token_hash(token),
                invited_by=actor,
                expires_at=timezone.now() + INVITE_TTL,
            )
            record(
                request,
                workspace,
                "invite",
                "invite",
                invite.id,
                {"email": email, "role": ui_role(role)},
            )
        return ok(serialize_invite(invite, token), status=201)


class WorkspaceInviteDetailView(DashboardAPIView):
    def delete(self, request: Request, ws: str, invite_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="admin")
        invite = Invite.objects.filter(
            workspace=workspace, id=uuid_or_404(invite_id, "invite")
        ).first()
        if invite is None:
            raise not_found("invite not found")
        if invite_status(invite) == "pending":
            with transaction.atomic():
                invite.revoked_at = timezone.now()
                invite.save(update_fields=["revoked_at"])
                record(
                    request, workspace, "revoke", "invite", invite.id, {"email": str(invite.email)}
                )
        return ok({"message": "Invite revoked"})


def _invite_by_token(token: str) -> Invite:
    invite = (
        Invite.objects.select_related("workspace", "invited_by")
        .filter(token_hash=_token_hash(token), workspace__deleted_at__isnull=True)
        .first()
    )
    if invite is None:
        raise not_found("this invite link isn't valid")
    return invite


class InvitePreviewView(PublicDashboardAPIView):
    def get(self, request: Request, token: str) -> Response:
        invite = _invite_by_token(token)
        status = invite_status(invite)
        brand = invite.workspace.brand if isinstance(invite.workspace.brand, dict) else {}
        return ok(
            {
                "email": str(invite.email),
                "role": ui_role(invite.role),
                "status": status,
                "is_valid": status == "pending",
                "expires_at": iso(invite.expires_at),
                "workspace": {
                    "id": str(invite.workspace_id),
                    "name": invite.workspace.name,
                    "brand_color": brand.get("color") or "#8B5CF6",
                    "brand_logo": brand.get("logo_url"),
                },
                "invited_by": {
                    "name": invite.invited_by.name or None,
                    "email": str(invite.invited_by.email),
                },
            }
        )


class InviteAcceptView(DashboardAPIView):
    def post(self, request: Request, token: str) -> Response:
        user = self.user(request)
        invite = _invite_by_token(token)
        status = invite_status(invite)
        if status != "pending":
            raise ApiError(status=410, code=f"invite_{status}", detail=f"this invite is {status}")
        if str(invite.email).lower() != str(user.email).lower():
            raise forbidden(
                code="invite_email_mismatch",
                detail=f"this invite was sent to {invite.email}; sign in with that address to accept it",
            )
        workspace = invite.workspace
        with transaction.atomic():
            if not WorkspaceMember.objects.filter(workspace=workspace, user=user).exists():
                WorkspaceMember.objects.create(workspace=workspace, user=user, role=invite.role)
            invite.accepted_at = timezone.now()
            invite.save(update_fields=["accepted_at"])
            record(request, workspace, "join", "member", user.id, {"role": ui_role(invite.role)})
        return ok(
            {
                "message": f"You joined {workspace.name}",
                "workspace_id": str(workspace.id),
                "role": ui_role(invite.role),
                "workspace": serialize_workspace(workspace, invite.role),
            }
        )
