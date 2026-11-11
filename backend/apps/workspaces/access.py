"""Workspace membership checks and the response shape the v1 dashboard expects.

The dashboard in `frontend/` was written for v1. Some of its pages call the
API with a bare `fetch` and only act when the body is `{"success": true,
"data": ...}`, so the endpoints that serve those pages answer in that
envelope, and their errors carry `success: false` and `error` next to the
usual problem+json fields. The rest of the v3 API keeps bare JSON.
"""

from typing import Any
from uuid import UUID

from drf_spectacular.openapi import AutoSchema
from drf_spectacular.types import OpenApiTypes
from drf_spectacular.utils import inline_serializer
from rest_framework import serializers
from rest_framework.request import Request
from rest_framework.response import Response

from apps.accounts.models import User
from apps.core.errors import ApiError, forbidden, not_found
from apps.core.views.base import AuthenticatedAPIView, PublicAPIView

from .models import Workspace, WorkspaceMember

# v3 roles, weakest first. The v1 UI knows owner/admin/editor/viewer.
ROLE_RANK = {"custom": 0, "analyst": 0, "reviewer": 0, "editor": 1, "admin": 2, "owner": 3}
UI_ROLE = {"owner": "owner", "admin": "admin", "editor": "editor"}


def ui_role(role: str) -> str:
    return UI_ROLE.get(role, "viewer")


def member_role(user: User, workspace: Workspace) -> str | None:
    if workspace.owner_id == user.id:
        return "owner"
    row = WorkspaceMember.objects.filter(workspace=workspace, user=user).first()
    return row.role if row else None


def workspace_for(
    user: User, ws_id: str | UUID, min_role: str = "analyst"
) -> tuple[Workspace, str]:
    """The workspace if `user` belongs to it with at least `min_role`.

    A workspace the user can't see answers 404, not 403, so ids can't be probed.
    """
    try:
        ws_uuid = UUID(str(ws_id))
    except ValueError:
        raise not_found("workspace not found") from None
    ws = Workspace.objects.filter(id=ws_uuid, deleted_at__isnull=True).first()
    role = member_role(user, ws) if ws else None
    if ws is None or role is None:
        raise not_found("workspace not found")
    require_role(role, min_role)
    return ws, role


def require_role(role: str, min_role: str) -> None:
    if ROLE_RANK.get(role, 0) < ROLE_RANK[min_role]:
        needed = ui_role(min_role) if min_role in UI_ROLE else "viewer"
        raise ApiError(
            status=403,
            code="insufficient_role",
            detail=f"this action needs the {needed} role or higher",
            extra={"required_role": needed},
        )


def ok(data: Any, status: int = 200) -> Response:
    return Response({"success": True, "data": data}, status=status)


def body(request: Request) -> dict[str, Any]:
    return request.data if isinstance(request.data, dict) else {}


def page_params(request: Request, default_limit: int = 25) -> tuple[int, int]:
    try:
        page = max(1, int(request.query_params.get("page") or 1))
        limit = min(100, max(1, int(request.query_params.get("limit") or default_limit)))
    except ValueError:
        raise ApiError(
            status=422, code="invalid_page", detail="page and limit must be numbers"
        ) from None
    return page, limit


def paginated(items: list[Any], total: int, page: int, limit: int) -> dict[str, Any]:
    return {
        "items": items,
        "total": total,
        "page": page,
        "limit": limit,
        "pages": (total + limit - 1) // limit,
    }


def user_summary(user: User | None) -> dict[str, Any] | None:
    if user is None:
        return None
    return {
        "id": str(user.id),
        "email": str(user.email),
        "name": user.name or None,
        "avatar_url": user.avatar_url,
    }


def iso(value: Any) -> str | None:
    return value.isoformat() if value else None


def uuid_or_404(value: Any, what: str) -> UUID:
    try:
        return UUID(str(value))
    except ValueError:
        raise not_found(f"{what} not found") from None


ENVELOPE = inline_serializer(
    "DashboardEnvelope",
    {"success": serializers.BooleanField(), "data": serializers.JSONField()},
)


class DashboardSchema(AutoSchema):
    """OpenAPI for the dashboard endpoints: free-form JSON in, the envelope out."""

    def get_operation_id(self) -> str:
        operation_id: str = super().get_operation_id()
        if self.method == "GET" and getattr(self.view, "list_view", False):
            operation_id = operation_id.removesuffix("_retrieve") + "_list"
        return operation_id

    def get_request_serializer(self) -> Any:
        return OpenApiTypes.OBJECT if self.method in ("POST", "PUT", "PATCH") else None

    def get_response_serializers(self) -> Any:
        return ENVELOPE


class DashboardAPIView(AuthenticatedAPIView):
    """Authenticated endpoint serving the v1 dashboard (envelope responses)."""

    schema = DashboardSchema()
    list_view = False

    def handle_exception(self, exc: Exception) -> Response:
        if isinstance(exc, ApiError):
            exc.extra = {**exc.extra, "success": False, "error": exc.detail}
        return super().handle_exception(exc)

    def user(self, request: Request) -> User:
        user = request.user
        if not isinstance(user, User):
            raise forbidden(code="unauthorized", detail="Authentication credentials required")
        return user


class PublicDashboardAPIView(PublicAPIView):
    """Unauthenticated endpoint the v1 pages call (invite preview, public lead page)."""

    schema = DashboardSchema()

    def handle_exception(self, exc: Exception) -> Response:
        if isinstance(exc, ApiError):
            exc.extra = {**exc.extra, "success": False, "error": exc.detail}
        return super().handle_exception(exc)
