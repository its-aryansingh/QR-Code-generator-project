"""Enterprise identity models: SSO, SCIM directories, and linked identities.

Plan §6.2 & §6.3:
- sso_connections
- user_identities
- scim_directories
- scim_users
"""

from typing import Any

from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


def default_attribute_mapping() -> dict[str, Any]:
    return {
        "email": "email",
        "first_name": "firstName",
        "last_name": "lastName",
        "groups": "groups",
    }


class SSOConnection(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="sso_connections"
    )
    protocol = models.TextField(
        choices=[
            ("saml", "SAML"),
            ("oidc", "OIDC"),
        ]
    )
    label = models.TextField()
    status = models.TextField(
        default="draft",
        choices=[
            ("draft", "Draft"),
            ("testing", "Testing"),
            ("active", "Active"),
            ("disabled", "Disabled"),
        ],
    )
    bridge_tenant = models.TextField(null=True, blank=True)
    oidc_issuer = models.TextField(null=True, blank=True)
    oidc_client_id = models.TextField(null=True, blank=True)
    oidc_client_secret_ct = models.BinaryField(null=True, blank=True)
    jit_provisioning = models.BooleanField(default=True)
    default_workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    default_role_key = models.TextField(default="analyst")
    attribute_mapping = models.JSONField(default=default_attribute_mapping)
    last_login_at = models.DateTimeField(null=True, blank=True)
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
    # 00005 additions:
    oidc_scopes = models.TextField(default="openid email profile")
    groups_claim = models.TextField(default="groups")
    test_passed_at = models.DateTimeField(null=True, blank=True)
    saml_metadata_url = models.TextField(null=True, blank=True)
    idp_kind = models.TextField(
        default="custom",
        choices=[
            ("okta", "Okta"),
            ("entra", "Entra ID"),
            ("google", "Google Workspace"),
            ("jumpcloud", "JumpCloud"),
            ("onelogin", "OneLogin"),
            ("ping", "Ping Identity"),
            ("keycloak", "Keycloak"),
            ("custom", "Custom"),
        ],
    )

    class Meta:
        db_table = "sso_connections"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(protocol__in=["saml", "oidc"]),
                name="sso_connections_protocol_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["draft", "testing", "active", "disabled"]),
                name="sso_connections_status_check",
            ),
            models.CheckConstraint(
                condition=~Q(protocol="oidc")
                | (Q(oidc_issuer__isnull=False) & Q(oidc_client_id__isnull=False)),
                name="sso_oidc_fields",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    idp_kind__in=[
                        "okta",
                        "entra",
                        "google",
                        "jumpcloud",
                        "onelogin",
                        "ping",
                        "keycloak",
                        "custom",
                    ]
                ),
                name="sso_connections_idp_kind_check",
            ),
        ]
        indexes = [
            models.Index(fields=["org"], name="sso_connections_org_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.label} ({self.protocol})"


class UserIdentity(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    user = models.ForeignKey("accounts.User", on_delete=models.CASCADE, related_name="identities")
    connection = models.ForeignKey(
        SSOConnection, on_delete=models.CASCADE, related_name="identities"
    )
    subject = models.TextField()
    email_at_login = CIText()
    raw_claims = models.JSONField(default=dict)
    last_login_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "user_identities"
        constraints = [
            models.UniqueConstraint(
                fields=["connection", "subject"],
                name="user_identities_connection_id_subject_key",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.subject} via {self.connection_id}"


class SCIMDirectory(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization", on_delete=models.CASCADE, related_name="scim_directories"
    )
    label = models.TextField()
    token_prefix = models.TextField(unique=True)
    token_hash = models.BinaryField(unique=True)
    status = models.TextField(
        default="active",
        choices=[
            ("active", "Active"),
            ("disabled", "Disabled"),
        ],
    )
    deprovision_action = models.TextField(
        default="suspend",
        choices=[
            ("suspend", "Suspend"),
            ("remove", "Remove"),
        ],
    )
    last_request_at = models.DateTimeField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        db_column="created_by",
        related_name="+",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "scim_directories"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "disabled"]),
                name="scim_directories_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(deprovision_action__in=["suspend", "remove"]),
                name="scim_directories_deprovision_action_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.label} ({self.org_id})"


class SCIMUser(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    directory = models.ForeignKey(SCIMDirectory, on_delete=models.CASCADE, related_name="users")
    user = models.ForeignKey(
        "accounts.User", on_delete=models.CASCADE, related_name="scim_profiles"
    )
    external_id = models.TextField(null=True, blank=True)
    user_name = models.TextField()
    active = models.BooleanField(default=True)
    resource = models.JSONField()
    version = models.IntegerField(default=1)
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "scim_users"
        constraints = [
            models.UniqueConstraint(
                fields=["directory", "user_name"],
                name="scim_users_directory_id_user_name_key",
            ),
            models.UniqueConstraint(
                fields=["directory", "user"],
                name="scim_users_directory_id_user_id_key",
            ),
            models.UniqueConstraint(
                fields=["directory", "external_id"],
                condition=Q(external_id__isnull=False),
                name="scim_users_external_uniq",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.user_name} in dir {self.directory_id}"
