"""Trust and safety models.

Plan §6.2 & §7.24:
- abuse_reports
"""

from django.db import models
from django.utils import timezone

from apps.core.fields import CIText
from apps.core.ids import uuid7


class AbuseReport(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    qr_code = models.ForeignKey(
        "qr.QRCode",
        null=True,
        blank=True,
        on_delete=models.DB_SET_NULL,
        db_column="qr_code_id",
        related_name="abuse_reports",
    )
    hostname = CIText()
    short_code = models.TextField()
    reason = models.TextField()
    details = models.TextField(null=True, blank=True)
    reporter_email = CIText(null=True, blank=True)
    reporter_ip_prefix = models.TextField(null=True, blank=True)
    status = models.TextField(default="open")
    resolved_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="resolved_by",
        related_name="resolved_abuse_reports",
    )
    resolved_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "abuse_reports"
        indexes = [
            models.Index(
                fields=["created_at"],
                condition=models.Q(status="open"),
                name="abuse_reports_open_idx",
            ),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    reason__in=["phishing", "malware", "scam", "spam", "illegal", "other"]
                ),
                name="abuse_reports_reason_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["open", "actioned", "dismissed"]),
                name="abuse_reports_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.hostname}/{self.short_code}:{self.reason}"
