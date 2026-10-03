"""Audit log models.

Plan §6.3 & §7.12:
- audit_logs (append-only ledger)
"""

from django.db import models
from django.utils import timezone


class AuditLog(models.Model):
    id = models.BigAutoField(primary_key=True)
    workspace_id = models.UUIDField(null=True, blank=True)
    actor_type = models.TextField()
    actor_id = models.UUIDField(null=True, blank=True)
    action = models.TextField()
    target_type = models.TextField()
    target_id = models.UUIDField(null=True, blank=True)
    changes = models.JSONField(default=dict)
    ip_prefix = models.TextField(null=True, blank=True)
    user_agent = models.TextField(null=True, blank=True)
    request_id = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "audit_logs"
        indexes = [
            models.Index(fields=["workspace_id", "-created_at"], name="audit_logs_ws_time_idx"),
            models.Index(fields=["target_type", "target_id"], name="audit_logs_target_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(actor_type__in=["user", "api_key", "system", "staff"]),
                name="audit_logs_actor_type_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.action}:{self.id}"
