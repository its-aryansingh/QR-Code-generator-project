"""Staff support access grants and session models.

Plan §6.2 & §6.3:
- support_access_grants
- support_sessions
"""

from datetime import timedelta

from django.db import models
from django.db.models import F
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class SupportAccessGrant(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="support_grants"
    )
    granted_by = models.ForeignKey(
        "accounts.User", on_delete=models.PROTECT, related_name="granted_support_access"
    )
    scope = models.TextField(
        default="read",
        choices=[
            ("read", "Read"),
            ("read_write", "Read/Write"),
        ],
    )
    reason = models.TextField()
    expires_at = models.DateTimeField()
    revoked_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "support_access_grants"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(scope__in=["read", "read_write"]),
                name="support_access_grants_scope_check",
            ),
            models.CheckConstraint(
                condition=models.Q(expires_at__lte=F("created_at") + timedelta(days=7)),
                name="support_grant_max_7d",
            ),
        ]
        indexes = [
            models.Index(fields=["org", "-created_at"], name="support_access_grants_org_idx"),
        ]

    def __str__(self) -> str:
        return f"Support grant {self.id} for org {self.org_id} ({self.scope})"


class SupportSession(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    grant = models.ForeignKey(SupportAccessGrant, on_delete=models.CASCADE, related_name="sessions")
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="support_sessions"
    )
    staff_user = models.ForeignKey(
        "accounts.User", on_delete=models.PROTECT, related_name="support_sessions"
    )
    reason = models.TextField()
    started_at = models.DateTimeField(default=tz_now)
    expires_at = models.DateTimeField()
    ended_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "support_sessions"
        indexes = [
            models.Index(fields=["org", "-started_at"], name="support_sessions_org_idx"),
        ]

    def __str__(self) -> str:
        return f"Staff {self.staff_user_id} in org {self.org_id}"
