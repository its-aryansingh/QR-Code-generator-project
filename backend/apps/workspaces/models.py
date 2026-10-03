"""Workspace, member, invite, and policy models.

Plan §6.3 & §7.4:
- workspaces
- workspace_members
- invites
- workspace_policies
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class Workspace(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        db_column="org_id",
        related_name="workspaces",
    )
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
        on_delete=models.SET_NULL,
        db_column="default_domain_id",
        related_name="default_for_workspaces",
    )
    brand = models.JSONField(default=dict)
    settings = models.JSONField(default=dict)
    is_sandbox = models.BooleanField(default=False)
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
        indexes = [
            models.Index(fields=["org"], name="workspaces_org_idx"),
        ]

    def __str__(self) -> str:
        return self.name


class WorkspaceMember(models.Model):
    pk = models.CompositePrimaryKey("workspace", "user")
    workspace = models.ForeignKey(
        Workspace,
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="members",
    )
    user = models.ForeignKey(
        "accounts.User",
        on_delete=models.CASCADE,
        db_column="user_id",
        related_name="workspace_memberships",
    )
    role = models.TextField(
        choices=[
            ("owner", "Owner"),
            ("admin", "Admin"),
            ("editor", "Editor"),
            ("reviewer", "Reviewer"),
            ("analyst", "Analyst"),
            ("custom", "Custom"),
        ]
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "workspace_members"
        indexes = [
            models.Index(fields=["user"], name="workspace_members_user_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    role__in=["owner", "admin", "editor", "reviewer", "analyst", "custom"]
                ),
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
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="invites",
    )
    email = CIText()
    role = models.TextField(
        choices=[
            ("admin", "Admin"),
            ("editor", "Editor"),
            ("reviewer", "Reviewer"),
            ("analyst", "Analyst"),
        ]
    )
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
                condition=models.Q(role__in=["admin", "editor", "reviewer", "analyst"]),
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


class WorkspacePolicy(models.Model):
    workspace = models.OneToOneField(
        Workspace,
        on_delete=models.CASCADE,
        primary_key=True,
        related_name="policy",
        db_column="workspace_id",
    )
    allowed_destination_hosts = ArrayField(models.TextField(), default=list)
    blocked_destination_hosts = ArrayField(models.TextField(), default=list)
    require_https = models.BooleanField(default=True)
    approval_mode = models.TextField(
        default="off",
        choices=[
            ("off", "Off"),
            ("outside_allowlist", "Outside Allowlist"),
            ("all_destination_changes", "All Destination Changes"),
            ("all_changes", "All Changes"),
        ],
    )
    approvals_required = models.IntegerField(default=1)
    approval_expiry_hours = models.IntegerField(default=168)
    require_template = models.BooleanField(default=False)
    pixel_consent_mode = models.TextField(
        default="opt_in_all",
        choices=[
            ("opt_in_all", "Opt In All"),
            ("opt_in_where_required", "Opt In Where Required"),
        ],
    )
    disabled_features = ArrayField(models.TextField(), default=list)
    updated_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        db_column="updated_by",
        related_name="+",
    )
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "workspace_policies"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    approval_mode__in=[
                        "off",
                        "outside_allowlist",
                        "all_destination_changes",
                        "all_changes",
                    ]
                ),
                name="workspace_policies_approval_mode_check",
            ),
            models.CheckConstraint(
                condition=Q(approvals_required__gte=1, approvals_required__lte=3),
                name="workspace_policies_approvals_required_check",
            ),
            models.CheckConstraint(
                condition=Q(approval_expiry_hours__gte=1, approval_expiry_hours__lte=720),
                name="workspace_policies_approval_expiry_hours_check",
            ),
            models.CheckConstraint(
                condition=models.Q(pixel_consent_mode__in=["opt_in_all", "opt_in_where_required"]),
                name="workspace_policies_pixel_consent_mode_check",
            ),
        ]

    def __str__(self) -> str:
        return f"Policy for workspace {self.workspace_id}"
