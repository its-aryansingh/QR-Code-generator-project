"""Integrations and webhooks models.

Plan §6.2, §6.3, §6.4 & §7.14:
- webhooks
- webhook_deliveries
- integrations
- integration_deliveries
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class Webhook(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
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
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_webhooks",
    )
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)
    # Plan §6.4 delta:
    signature_scheme = models.TextField(
        default="standard",
        choices=[
            ("standard", "Standard Webhooks"),
            ("legacy_v1", "Legacy v1 HMAC-SHA256"),
            ("both", "Both"),
        ],
    )

    class Meta:
        db_table = "webhooks"
        indexes = [
            models.Index(
                fields=["workspace"],
                condition=models.Q(is_active=True),
                name="webhooks_ws_active_idx",
            ),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(signature_scheme__in=["standard", "legacy_v1", "both"]),
                name="webhooks_signature_scheme_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.url}"


class WebhookDelivery(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    webhook = models.ForeignKey(
        Webhook,
        on_delete=models.CASCADE,
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
    created_at = models.DateTimeField(default=tz_now)

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


class Integration(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="integrations"
    )
    provider = models.TextField(
        choices=[
            ("slack", "Slack"),
            ("teams", "Microsoft Teams"),
            ("zapier", "Zapier"),
            ("make", "Make"),
            ("hubspot", "HubSpot"),
            ("salesforce", "Salesforce"),
            ("google_sheets", "Google Sheets"),
            ("ga4", "Google Analytics 4"),
            ("warehouse_s3", "Amazon S3 Warehouse"),
            ("warehouse_gcs", "Google Cloud Storage"),
            ("warehouse_bigquery", "Google BigQuery"),
        ]
    )
    name = models.TextField()
    status = models.TextField(
        default="active",
        choices=[
            ("active", "Active"),
            ("error", "Error"),
            ("disabled", "Disabled"),
        ],
    )
    config = models.JSONField(default=dict)
    credentials_ct = models.BinaryField(null=True, blank=True)
    events = ArrayField(models.TextField(), default=list)
    last_success_at = models.DateTimeField(null=True, blank=True)
    last_error = models.TextField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        db_column="created_by",
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "integrations"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    provider__in=[
                        "slack",
                        "teams",
                        "zapier",
                        "make",
                        "hubspot",
                        "salesforce",
                        "google_sheets",
                        "ga4",
                        "warehouse_s3",
                        "warehouse_gcs",
                        "warehouse_bigquery",
                    ]
                ),
                name="integrations_provider_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "error", "disabled"]),
                name="integrations_status_check",
            ),
        ]
        indexes = [
            models.Index(
                fields=["workspace"],
                name="integrations_ws_idx",
                condition=Q(status="active"),
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.provider})"


class IntegrationDelivery(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    integration = models.ForeignKey(
        Integration, on_delete=models.CASCADE, related_name="deliveries"
    )
    event_id = models.UUIDField()
    event_type = models.TextField()
    status = models.TextField(
        default="pending",
        choices=[
            ("pending", "Pending"),
            ("succeeded", "Succeeded"),
            ("failed", "Failed"),
            ("dead", "Dead"),
        ],
    )
    attempts = models.IntegerField(default=0)
    last_error = models.TextField(null=True, blank=True)
    next_attempt_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    delivered_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "integration_deliveries"
        constraints = [
            models.UniqueConstraint(
                fields=["integration", "event_id"],
                name="integration_deliveries_integration_id_event_id_key",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["pending", "succeeded", "failed", "dead"]),
                name="integration_deliveries_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.integration_id}:{self.event_id}:{self.status}"
