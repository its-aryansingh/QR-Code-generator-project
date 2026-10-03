"""Scheduled report models.

Plan §6.2 & §6.3:
- report_schedules
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class ReportSchedule(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="report_schedules"
    )
    name = models.TextField()
    frequency = models.TextField(
        choices=[
            ("daily", "Daily"),
            ("weekly", "Weekly"),
            ("monthly", "Monthly"),
        ]
    )
    weekday = models.IntegerField(null=True, blank=True)
    month_day = models.IntegerField(null=True, blank=True)
    hour_local = models.IntegerField(default=9)
    timezone = models.TextField()
    filters = models.JSONField(default=dict)
    format = models.TextField(
        default="pdf",
        choices=[
            ("pdf", "PDF"),
            ("csv", "CSV"),
            ("both", "Both"),
        ],
    )
    recipients = ArrayField(CIText())
    is_active = models.BooleanField(default=True)
    next_run_at = models.DateTimeField()
    last_run_at = models.DateTimeField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "report_schedules"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(frequency__in=["daily", "weekly", "monthly"]),
                name="report_schedules_frequency_check",
            ),
            models.CheckConstraint(
                condition=Q(weekday__isnull=True) | Q(weekday__gte=1, weekday__lte=7),
                name="report_schedules_weekday_check",
            ),
            models.CheckConstraint(
                condition=Q(month_day__isnull=True) | Q(month_day__gte=1, month_day__lte=28),
                name="report_schedules_month_day_check",
            ),
            models.CheckConstraint(
                condition=Q(hour_local__gte=0, hour_local__lte=23),
                name="report_schedules_hour_local_check",
            ),
            models.CheckConstraint(
                condition=models.Q(format__in=["pdf", "csv", "both"]),
                name="report_schedules_format_check",
            ),
        ]
        indexes = [
            models.Index(
                fields=["next_run_at"],
                name="report_schedules_due_idx",
                condition=Q(is_active=True),
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.frequency})"
