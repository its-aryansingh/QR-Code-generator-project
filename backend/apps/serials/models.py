"""Product authentication and serial code models.

Plan §6.2 & §6.3:
- serial_batches
- serial_codes
"""

from typing import Any

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import FixedCharField
from apps.core.ids import uuid7


def default_serial_rules() -> dict[str, Any]:
    return {
        "max_scans": 20,
        "multi_country_window_hours": 24,
        "velocity_per_hour": 10,
    }


class SerialBatch(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="serial_batches"
    )
    qr_code = models.OneToOneField(
        "qr.QRCode", on_delete=models.PROTECT, related_name="serial_batch"
    )
    name = models.TextField()
    gtin = FixedCharField(max_length=14, null=True, blank=True)
    quantity = models.IntegerField()
    rules = models.JSONField(default=default_serial_rules)
    verify_page = models.JSONField(default=dict)
    status = models.TextField(
        default="generating",
        choices=[
            ("generating", "Generating"),
            ("ready", "Ready"),
            ("void", "Void"),
        ],
    )
    generated = models.IntegerField(default=0)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "serial_batches"
        constraints = [
            models.CheckConstraint(
                condition=Q(gtin__isnull=True) | Q(gtin__regex=r"^[0-9]{14}$"),
                name="serial_batches_gtin_check",
            ),
            models.CheckConstraint(
                condition=Q(quantity__gte=1, quantity__lte=10000000),
                name="serial_batches_quantity_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["generating", "ready", "void"]),
                name="serial_batches_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.quantity} units)"


class SerialCode(models.Model):
    serial = models.CharField(max_length=12, primary_key=True)
    batch = models.ForeignKey(SerialBatch, on_delete=models.CASCADE, related_name="codes")
    status = models.TextField(
        default="active",
        choices=[
            ("active", "Active"),
            ("flagged", "Flagged"),
            ("void", "Void"),
        ],
    )
    scan_count = models.IntegerField(default=0)
    first_scan_at = models.DateTimeField(null=True, blank=True)
    first_country = FixedCharField(max_length=2, null=True, blank=True)
    first_city = models.TextField(null=True, blank=True)
    last_scan_at = models.DateTimeField(null=True, blank=True)
    countries = ArrayField(FixedCharField(max_length=2), default=list)
    flagged_reason = models.TextField(null=True, blank=True)

    class Meta:
        db_table = "serial_codes"
        constraints = [
            models.CheckConstraint(
                condition=Q(serial__regex=r"^[0-9A-HJKMNP-TV-Z]{12}$"),
                name="serial_codes_serial_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "flagged", "void"]),
                name="serial_codes_status_check",
            ),
        ]
        indexes = [
            models.Index(fields=["batch", "status"], name="serial_codes_batch_idx"),
        ]

    def __str__(self) -> str:
        return self.serial
