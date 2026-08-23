"""Audit trail.

The AuditLog table already existed but nothing ever wrote to it, so the audit
page could only ever render empty. Every mutating workspace endpoint now calls
`record`.
"""

import json
import logging
import uuid

from django.utils import timezone

from api.models import AuditLog
from api.utils.ip import get_client_ip

logger = logging.getLogger(__name__)

# action verbs kept stable so the UI can map them to icons and colours
CREATE = "create"
UPDATE = "update"
DELETE = "delete"
INVITE = "invite"
JOIN = "join"
REMOVE = "remove"
ROLE_CHANGE = "role_change"
LOGIN = "login"
EXPORT = "export"
REVOKE = "revoke"
TEST = "test"


def record(request, workspace_id, action, resource, resource_id=None, details=None):
    """Write one audit entry. Never raises -- auditing must not break a write."""
    try:
        user = getattr(request, "auth_user", None)
        if not user or not workspace_id:
            return
        AuditLog(
            id=uuid.uuid4(),
            workspace_id=workspace_id,
            user_id=user["id"],
            action=action,
            resource=resource,
            resource_id=resource_id,
            details=json.dumps(details, default=str) if details is not None else None,
            ip_address=get_client_ip(request),
            user_agent=(request.META.get("HTTP_USER_AGENT") or "")[:500],
            created_at=timezone.now(),
        ).save(force_insert=True)
    except Exception:
        logger.exception("audit write failed: %s %s", action, resource)
