"""Integrations and webhooks models.

Plan §6.2 & §7.14:
- webhooks
- webhook_deliveries
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.utils import timezone

from apps.core.ids import uuid7


class Webhook(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.DB_CASCADE,
        db_column="workspace_id",
        related_name="webhooks",
    )
    url = models.TextField()
    secret_ciphertext = models.BinaryField()
    events = ArrayField(models.TextField())
    is_active = models.BooleanField(default=True)
    consecutive_failures = models.IntegerField(default=0)
    disabled_reason = models.TextField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.DB_SET_NULL,
        db_column="created_by",
        related_name="created_webhooks",
    )
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "webhooks"
        indexes = [
            models.Index(
                fields=["workspace"],
                condition=models.Q(is_active=True),
                name="webhooks_ws_active_idx",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.url}"


class WebhookDelivery(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    webhook = models.ForeignKey(
        Webhook,
        on_delete=models.DB_CASCADE,
        db_column="webhook_id",
        related_name="deliveries",
    )
    event_id = models.UUIDField()
    event_type = models.TextField()
    payload = models.JSONField()
    status = models.TextField(default="pending")
    attempts = models.IntegerField(default=0)
    last_status_code = models.IntegerField(null=True, blank=True)
    last_error = models.TextField(null=True, blank=True)
    next_attempt_at = models.DateTimeField(null=True, blank=True)
    delivered_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "webhook_deliveries"
        indexes = [
            models.Index(fields=["webhook", "-created_at"], name="webhook_deliveries_webhook_idx"),
        ]
        constraints = [
            models.UniqueConstraint(
                fields=["webhook", "event_id"], name="webhook_deliveries_webhook_id_event_id_key"
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["pending", "succeeded", "failed", "dead"]),
                name="webhook_deliveries_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.webhook_id}:{self.event_id}:{self.status}"
