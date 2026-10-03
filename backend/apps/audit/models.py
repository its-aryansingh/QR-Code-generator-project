"""Audit log, anchor, and stream models.

Plan §6.3 & §7.12:
- audit_logs (append-only hash chain)
- audit_anchors (daily WORM head hashes)
- audit_streams (SIEM exports)
"""

from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class AuditLog(models.Model):
    id = models.BigAutoField(primary_key=True)
    org_id = models.UUIDField(null=True, blank=True)
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
    created_at = models.DateTimeField(default=tz_now)
    # Tamper-evident chain additions:
    seq = models.BigIntegerField(null=True, blank=True)
    prev_hash = models.BinaryField(null=True, blank=True)
    hash = models.BinaryField(null=True, blank=True)
    sealed_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "audit_logs"
        indexes = [
            models.Index(fields=["workspace_id", "-created_at"], name="audit_logs_ws_time_idx"),
            models.Index(fields=["target_type", "target_id"], name="audit_logs_target_idx"),
            models.Index(
                fields=["org_id", "id"],
                name="audit_logs_unsealed_idx",
                condition=Q(hash__isnull=True),
            ),
            models.Index(
                fields=["org_id", "-created_at"],
                name="audit_logs_org_time_idx",
                condition=Q(org_id__isnull=False),
            ),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(actor_type__in=["user", "api_key", "system", "staff"]),
                name="audit_logs_actor_type_check",
            ),
            models.UniqueConstraint(
                fields=["org_id", "seq"],
                condition=Q(seq__isnull=False),
                name="audit_logs_org_seq_uniq",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.action}:{self.id}"


class AuditAnchor(models.Model):
    pk = models.CompositePrimaryKey("org_id", "day")
    org_id = models.UUIDField()
    day = models.DateField()
    last_seq = models.BigIntegerField()
    head_hash = models.BinaryField()
    object_key = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "audit_anchors"

    def __str__(self) -> str:
        return f"Anchor for org {self.org_id} on {self.day}"


class AuditStream(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="audit_streams"
    )
    kind = models.TextField(
        choices=[
            ("webhook", "Webhook"),
            ("splunk_hec", "Splunk HEC"),
            ("datadog", "Datadog"),
            ("s3", "S3"),
        ]
    )
    label = models.TextField(default="")
    config = models.JSONField()
    secret_ct = models.BinaryField(null=True, blank=True)
    cursor_seq = models.BigIntegerField(default=0)
    status = models.TextField(
        default="active",
        choices=[
            ("active", "Active"),
            ("paused", "Paused"),
            ("error", "Error"),
        ],
    )
    last_error = models.TextField(null=True, blank=True)
    failing_since = models.DateTimeField(null=True, blank=True)
    last_delivered_at = models.DateTimeField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "audit_streams"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(kind__in=["webhook", "splunk_hec", "datadog", "s3"]),
                name="audit_streams_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "paused", "error"]),
                name="audit_streams_status_check",
            ),
        ]
        indexes = [
            models.Index(fields=["org"], name="audit_streams_org_idx"),
        ]

    def __str__(self) -> str:
        return f"AuditStream {self.label or self.kind} for {self.org_id}"
