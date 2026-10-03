"""Approval workflow models: approval requests and decisions.

Plan §6.2 & §6.3:
- approval_requests
- approval_decisions
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class ApprovalRequest(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="approval_requests"
    )
    kind = models.TextField(
        choices=[
            ("create", "Create"),
            ("destination", "Destination"),
            ("rules", "Rules"),
            ("bulk_update", "Bulk Update"),
        ]
    )
    qr_code = models.ForeignKey(
        "qr.QRCode",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="approval_requests",
    )
    job = models.ForeignKey(
        "developer.Job",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="approval_requests",
    )
    reasons = ArrayField(models.TextField())
    requested_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.PROTECT,
        db_column="requested_by",
        related_name="submitted_approval_requests",
    )
    required_approvals = models.IntegerField()
    status = models.TextField(
        default="pending",
        choices=[
            ("pending", "Pending"),
            ("approved", "Approved"),
            ("rejected", "Rejected"),
            ("cancelled", "Cancelled"),
            ("expired", "Expired"),
        ],
    )
    note = models.TextField(null=True, blank=True)
    override_reason = models.TextField(null=True, blank=True)
    expires_at = models.DateTimeField()
    decided_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "approval_requests"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(kind__in=["create", "destination", "rules", "bulk_update"]),
                name="approval_requests_kind_check",
            ),
            models.CheckConstraint(
                condition=Q(required_approvals__gte=1, required_approvals__lte=3),
                name="approval_requests_required_approvals_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    status__in=["pending", "approved", "rejected", "cancelled", "expired"]
                ),
                name="approval_requests_status_check",
            ),
            models.CheckConstraint(
                condition=(
                    Q(kind="bulk_update", job__isnull=False, qr_code__isnull=True)
                    | (~Q(kind="bulk_update") & Q(qr_code__isnull=False, job__isnull=True))
                ),
                name="approval_subject",
            ),
        ]
        indexes = [
            models.Index(
                fields=["workspace", "created_at"],
                name="approval_requests_ws_pend_idx",
                condition=Q(status="pending"),
            ),
            models.Index(
                fields=["expires_at"],
                name="approval_req_pend_exp_idx",
                condition=Q(status="pending"),
            ),
            models.Index(
                fields=["requested_by", "-created_at"],
                name="approval_req_requester_idx",
            ),
        ]

    def __str__(self) -> str:
        return f"Approval request {self.id} ({self.kind}, status={self.status})"


class ApprovalDecision(models.Model):
    pk = models.CompositePrimaryKey("request_id", "approver_id")
    request = models.ForeignKey(ApprovalRequest, on_delete=models.CASCADE, related_name="decisions")
    approver = models.ForeignKey(
        "accounts.User", on_delete=models.PROTECT, related_name="approval_decisions"
    )
    decision = models.TextField(
        choices=[
            ("approve", "Approve"),
            ("reject", "Reject"),
        ]
    )
    comment = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "approval_decisions"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(decision__in=["approve", "reject"]),
                name="approval_decisions_decision_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.decision} on {self.request_id} by {self.approver_id}"
