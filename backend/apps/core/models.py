"""Core models (idempotency keys).

Plan §6.2 & §7.1:
- idempotency_keys
"""

from django.db import models
from django.utils import timezone


class IdempotencyKey(models.Model):
    pk = models.CompositePrimaryKey("workspace_id", "key")
    workspace_id = models.UUIDField()
    key = models.TextField()
    method = models.TextField()
    path = models.TextField()
    request_hash = models.BinaryField()
    status_code = models.IntegerField(null=True, blank=True)
    response_body = models.JSONField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)
    expires_at = models.DateTimeField()

    class Meta:
        db_table = "idempotency_keys"
        indexes = [
            models.Index(fields=["expires_at"], name="idempotency_expiry_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(key__regex=r"^.{8,128}$"),
                name="idempotency_keys_key_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.key}"
