"""Audit rows for changes made from the dashboard.

Call `record` inside the same transaction as the change it describes, so a
rolled-back change never leaves an audit row behind. Rows are written unsealed;
`audit_seal()` chains them later. The client address is stored as a prefix and
the user agent is never stored (plan §12).
"""

from typing import Any
from uuid import UUID

from rest_framework.request import Request

from apps.accounts.models import User
from apps.core.net.client_ip import ip_prefix
from apps.workspaces.models import Workspace

from .models import AuditLog


def record(
    request: Request | None,
    workspace: Workspace,
    action: str,
    target_type: str,
    target_id: UUID | str | None = None,
    changes: dict[str, Any] | None = None,
    actor: User | None = None,
) -> AuditLog:
    user = actor
    if user is None and request is not None and isinstance(request.user, User):
        user = request.user
    return AuditLog.objects.create(
        org_id=workspace.org_id,
        workspace_id=workspace.id,
        actor_type="user" if user is not None else "system",
        actor_id=user.id if user is not None else None,
        action=action,
        target_type=target_type,
        target_id=UUID(str(target_id)) if target_id else None,
        changes=changes or {},
        ip_prefix=ip_prefix(request) if request is not None else None,
        request_id=(request.META.get("HTTP_X_REQUEST_ID") or None) if request is not None else None,
    )


def changed_fields(before: dict[str, Any], after: dict[str, Any]) -> list[str]:
    return sorted(k for k in after if before.get(k) != after.get(k))
