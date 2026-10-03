"""Workspace, member, and invite models.

Plan §6.3 & §7.4:
- workspaces
- workspace_members
- invites
"""

from django.db import models
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class Workspace(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    name = models.TextField()
    slug = CIText(unique=True)
    owner = models.ForeignKey(
        "accounts.User",
        on_delete=models.DO_NOTHING,
        db_column="owner_id",
        related_name="owned_workspaces",
    )
    plan_id = models.TextField(default="free")
    timezone = models.TextField(default="UTC")
    default_domain = models.ForeignKey(
        "qr.Domain",
        null=True,
        blank=True,
        on_delete=models.DB_SET_NULL,
        db_column="default_domain_id",
        related_name="default_for_workspaces",
    )
    brand = models.JSONField(default=dict)
    settings = models.JSONField(default=dict)
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "workspaces"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(plan_id__in=["free", "pro", "business", "enterprise"]),
                name="workspaces_plan_id_check",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class WorkspaceMember(models.Model):
    pk = models.CompositePrimaryKey("workspace", "user")
    workspace = models.ForeignKey(
        Workspace,
        on_delete=models.DB_CASCADE,
        db_column="workspace_id",
        related_name="members",
    )
    user = models.ForeignKey(
        "accounts.User",
        on_delete=models.DB_CASCADE,
        db_column="user_id",
        related_name="workspace_memberships",
    )
    role = models.TextField()
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "workspace_members"
        indexes = [
            models.Index(fields=["user"], name="workspace_members_user_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(role__in=["owner", "admin", "editor", "analyst"]),
                name="workspace_members_role_check",
            ),
            models.UniqueConstraint(
                fields=["workspace"],
                condition=models.Q(role="owner"),
                name="workspace_one_owner_uniq",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.user_id}:{self.role}"


class Invite(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        Workspace,
        on_delete=models.DB_CASCADE,
        db_column="workspace_id",
        related_name="invites",
    )
    email = CIText()
    role = models.TextField()
    token_hash = models.BinaryField(unique=True)
    invited_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.DO_NOTHING,
        db_column="invited_by",
        related_name="sent_invites",
    )
    expires_at = models.DateTimeField()
    accepted_at = models.DateTimeField(null=True, blank=True)
    revoked_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "invites"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(role__in=["admin", "editor", "analyst"]),
                name="invites_role_check",
            ),
            models.UniqueConstraint(
                fields=["workspace", "email"],
                condition=models.Q(accepted_at__isnull=True, revoked_at__isnull=True),
                name="invites_pending_uniq",
            ),
        ]

    def __str__(self) -> str:
        return f"invite:{self.workspace_id}:{self.email}"
