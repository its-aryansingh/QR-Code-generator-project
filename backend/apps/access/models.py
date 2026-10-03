"""Authorization and access models: roles, bindings, groups.

Plan §6.2 & §6.3:
- roles (+ system roles seeded in migration)
- role_bindings
- groups
- group_members
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class Role(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="custom_roles",
    )
    key = models.TextField()
    name = models.TextField()
    description = models.TextField(default="")
    permissions = ArrayField(models.TextField())
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "roles"
        constraints = [
            models.CheckConstraint(
                condition=Q(key__regex=r"^[a-z][a-z0-9_]{1,40}$"),
                name="roles_key_check",
            ),
            models.UniqueConstraint(
                fields=["org", "key"],
                nulls_distinct=False,
                name="roles_org_id_key_key",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.key})"


class RoleBinding(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey("orgs.Organization", on_delete=models.CASCADE, related_name="+")
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="role_bindings"
    )
    principal_type = models.TextField(
        choices=[
            ("user", "User"),
            ("group", "Group"),
        ]
    )
    principal_id = models.UUIDField()
    role = models.ForeignKey(Role, on_delete=models.PROTECT, related_name="bindings")
    scope_type = models.TextField(
        default="workspace",
        choices=[
            ("workspace", "Workspace"),
            ("folder", "Folder"),
        ],
    )
    folder = models.ForeignKey(
        "qr.Folder",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="role_bindings",
    )
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "role_bindings"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(principal_type__in=["user", "group"]),
                name="role_bindings_principal_type_check",
            ),
            models.CheckConstraint(
                condition=models.Q(scope_type__in=["workspace", "folder"]),
                name="role_bindings_scope_type_check",
            ),
            models.CheckConstraint(
                condition=(Q(scope_type="folder") & Q(folder__isnull=False))
                | (Q(scope_type="workspace") & Q(folder__isnull=True)),
                name="binding_scope",
            ),
            models.UniqueConstraint(
                fields=["workspace", "principal_type", "principal_id", "role", "folder"],
                nulls_distinct=False,
                name="role_bindings_workspace_id_principal_type_principal_id_role_key",
            ),
        ]
        indexes = [
            models.Index(
                fields=["principal_type", "principal_id"], name="role_bindings_principal_idx"
            ),
            models.Index(fields=["workspace"], name="role_bindings_ws_idx"),
        ]

    def __str__(self) -> str:
        return (
            f"{self.principal_type}:{self.principal_id} -> {self.role.key} in {self.workspace_id}"
        )


class Group(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey("orgs.Organization", on_delete=models.CASCADE, related_name="groups")
    display_name = models.TextField()
    source = models.TextField(
        default="manual",
        choices=[
            ("manual", "Manual"),
            ("scim", "SCIM"),
            ("sso", "SSO"),
        ],
    )
    directory = models.ForeignKey(
        "identity.SCIMDirectory",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="groups",
    )
    external_id = models.TextField(null=True, blank=True)
    version = models.IntegerField(default=1)
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "groups"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(source__in=["manual", "scim", "sso"]),
                name="groups_source_check",
            ),
            models.UniqueConstraint(
                fields=["org", "display_name"],
                name="groups_org_id_display_name_key",
            ),
            models.CheckConstraint(
                condition=(Q(source="scim") & Q(directory__isnull=False))
                | (~Q(source="scim") & Q(directory__isnull=True)),
                name="group_scim_dir",
            ),
        ]
        indexes = [
            models.Index(fields=["org", "source"], name="groups_org_source_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.display_name} ({self.org_id})"


class GroupMember(models.Model):
    pk = models.CompositePrimaryKey("group_id", "user_id")
    group = models.ForeignKey(Group, on_delete=models.CASCADE, related_name="members")
    user = models.ForeignKey(
        "accounts.User", on_delete=models.CASCADE, related_name="group_memberships"
    )
    added_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "group_members"
        indexes = [
            models.Index(fields=["user"], name="group_members_user_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.user_id} in group {self.group_id}"
