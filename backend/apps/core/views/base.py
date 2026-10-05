"""Base APIView classes for control-plane endpoints.

Plan §5.2:
- PublicAPIView: unauthenticated endpoints (register, login, verify, public info).
- AuthenticatedAPIView: requires valid Session or Bearer JWT or API Key.
- WorkspaceScopedAPIView: resolves {ws} by UUID or slug, enters workspace_scope.
- OrgScopedAPIView: resolves {org} by UUID or slug.
- StaffAPIView: staff-only operations.
"""

from typing import Any
from uuid import UUID

from django.http import HttpRequest, HttpResponse
from rest_framework.permissions import IsAuthenticated
from rest_framework.request import Request
from rest_framework.views import APIView

from apps.accounts.authentication import SessionAuthentication
from apps.core.db import workspace_scope
from apps.core.errors import forbidden, not_found
from apps.workspaces.models import Workspace


class PublicAPIView(APIView):
    """Unauthenticated public APIView."""

    authentication_classes: list[Any] = []
    permission_classes: list[Any] = []
    required_permission: dict[str, str] = {}


class AuthenticatedAPIView(APIView):
    """Authenticated APIView requiring active user session or API key."""

    authentication_classes = [SessionAuthentication]
    permission_classes = [IsAuthenticated]
    required_permission: dict[str, str] = {}

    def initial(self, request: Request, *args: Any, **kwargs: Any) -> None:
        super().initial(request, *args, **kwargs)
        if not request.user or not request.user.is_authenticated:
            raise forbidden(code="unauthorized", detail="Authentication credentials required")


class WorkspaceScopedAPIView(AuthenticatedAPIView):
    """Workspace-scoped APIView wrapping handlers in apps.core.db.workspace_scope."""

    workspace: Workspace | None = None

    def initial(self, request: Request, *args: Any, **kwargs: Any) -> None:
        super().initial(request, *args, **kwargs)
        ws_param = kwargs.get("ws") or kwargs.get("ws_id") or kwargs.get("workspace_id")
        if not ws_param:
            return

        try:
            ws_uuid = UUID(str(ws_param))
            ws = Workspace.objects.filter(id=ws_uuid, deleted_at__isnull=True).first()
        except ValueError:
            ws = Workspace.objects.filter(slug=str(ws_param), deleted_at__isnull=True).first()

        if ws is None:
            raise not_found("workspace not found")

        self.workspace = ws

    def dispatch(self, request: HttpRequest, *args: Any, **kwargs: Any) -> HttpResponse:
        ws_id = self.workspace.id if self.workspace is not None else None
        with workspace_scope(ws_id):
            return super().dispatch(request, *args, **kwargs)


class OrgScopedAPIView(AuthenticatedAPIView):
    """Organization-scoped APIView."""

    pass


class StaffAPIView(AuthenticatedAPIView):
    """Staff-only APIView enforcing user.is_staff flag."""

    def initial(self, request: Request, *args: Any, **kwargs: Any) -> None:
        super().initial(request, *args, **kwargs)
        if not request.user.is_staff:
            raise forbidden(code="forbidden", detail="staff access required")
