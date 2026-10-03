"""Consent-aware lead capture and DSAR models.

Plan §6.2 & §6.3:
- forms
- form_submissions
- dsar_requests
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class Form(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="forms"
    )
    name = models.TextField()
    fields = models.JSONField()
    notice = models.JSONField()
    notice_version = models.IntegerField(default=1)
    double_opt_in = models.BooleanField(default=False)
    retention_days = models.IntegerField(default=180)
    notify_emails = ArrayField(CIText(), default=list)
    is_active = models.BooleanField(default=True)
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
    # Plan §6.4 deltas:
    legacy_slug = models.TextField(unique=True, null=True, blank=True)
    presentation = models.JSONField(default=dict)

    class Meta:
        db_table = "forms"
        constraints = [
            models.CheckConstraint(
                condition=Q(retention_days__gte=1, retention_days__lte=1095),
                name="forms_retention_days_check",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class FormSubmission(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    form = models.ForeignKey(Form, on_delete=models.CASCADE, related_name="submissions")
    workspace_id = models.UUIDField()
    qr_code = models.ForeignKey(
        "qr.QRCode",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="form_submissions",
    )
    key_id = models.IntegerField()
    data_ct = models.BinaryField()
    email_bidx = models.BinaryField(null=True, blank=True)
    consent = models.JSONField()
    status = models.TextField(
        default="confirmed",
        choices=[
            ("pending_confirmation", "Pending Confirmation"),
            ("confirmed", "Confirmed"),
            ("withdrawn", "Withdrawn"),
            ("erased", "Erased"),
        ],
    )
    delete_after = models.DateField()
    created_at = models.DateTimeField(default=tz_now)
    confirmed_at = models.DateTimeField(null=True, blank=True)
    withdrawn_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "form_submissions"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    status__in=["pending_confirmation", "confirmed", "withdrawn", "erased"]
                ),
                name="form_submissions_status_check",
            ),
        ]
        indexes = [
            models.Index(
                fields=["workspace_id", "-created_at"],
                name="form_submissions_ws_idx",
            ),
            models.Index(
                fields=["email_bidx"],
                name="form_submissions_bidx_idx",
                condition=Q(email_bidx__isnull=False),
            ),
            models.Index(
                fields=["delete_after"],
                name="form_submissions_purge_idx",
                condition=~Q(status="erased"),
            ),
        ]

    def __str__(self) -> str:
        return f"Submission {self.id} for form {self.form_id}"


class DSARRequest(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="dsar_requests"
    )
    kind = models.TextField(
        choices=[
            ("access", "Access"),
            ("erasure", "Erasure"),
            ("correction", "Correction"),
        ]
    )
    subject_bidx = models.BinaryField()
    status = models.TextField(
        default="open",
        choices=[
            ("open", "Open"),
            ("completed", "Completed"),
            ("rejected", "Rejected"),
        ],
    )
    due_at = models.DateTimeField()
    result_file = models.ForeignKey(
        "qr.File",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    handled_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        db_column="handled_by",
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)
    completed_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "dsar_requests"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(kind__in=["access", "erasure", "correction"]),
                name="dsar_requests_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["open", "completed", "rejected"]),
                name="dsar_requests_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"DSAR {self.kind} ({self.status}) for {self.org_id}"
