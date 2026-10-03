"""Alert rule and event models.

Plan §6.2 & §6.3:
- alert_rules
- alert_events
"""

from typing import Any

from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


def default_alert_channels() -> dict[str, Any]:
    return {"emails": [], "integration_ids": []}


class AlertRule(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="alert_rules"
    )
    name = models.TextField()
    kind = models.TextField(
        choices=[
            ("scan_spike", "Scan Spike"),
            ("scan_drop", "Scan Drop"),
            ("scan_threshold", "Scan Threshold"),
            ("no_scans", "No Scans"),
            ("destination_down", "Destination Down"),
            ("serial_anomaly", "Serial Anomaly"),
            ("security_event", "Security Event"),
            ("approval_pending", "Approval Pending"),
        ]
    )
    target_type = models.TextField(
        choices=[
            ("workspace", "Workspace"),
            ("qr_code", "QR Code"),
            ("campaign", "Campaign"),
            ("serial_batch", "Serial Batch"),
        ]
    )
    target_id = models.UUIDField(null=True, blank=True)
    params = models.JSONField(default=dict)
    channels = models.JSONField(default=default_alert_channels)
    cooldown_minutes = models.IntegerField(default=60)
    is_active = models.BooleanField(default=True)
    last_fired_at = models.DateTimeField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "alert_rules"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    kind__in=[
                        "scan_spike",
                        "scan_drop",
                        "scan_threshold",
                        "no_scans",
                        "destination_down",
                        "serial_anomaly",
                        "security_event",
                        "approval_pending",
                    ]
                ),
                name="alert_rules_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    target_type__in=["workspace", "qr_code", "campaign", "serial_batch"]
                ),
                name="alert_rules_target_type_check",
            ),
            models.CheckConstraint(
                condition=Q(cooldown_minutes__gte=5, cooldown_minutes__lte=10080),
                name="alert_rules_cooldown_minutes_check",
            ),
            models.CheckConstraint(
                condition=(Q(target_type="workspace") & Q(target_id__isnull=True))
                | (~Q(target_type="workspace") & Q(target_id__isnull=False)),
                name="alert_target",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.kind})"


class AlertEvent(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    rule = models.ForeignKey(AlertRule, on_delete=models.CASCADE, related_name="events")
    fired_at = models.DateTimeField(default=tz_now)
    payload = models.JSONField()
    resolved_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "alert_events"
        indexes = [
            models.Index(fields=["rule", "-fired_at"], name="alert_events_rule_idx"),
        ]

    def __str__(self) -> str:
        return f"Alert for {self.rule_id} at {self.fired_at}"
