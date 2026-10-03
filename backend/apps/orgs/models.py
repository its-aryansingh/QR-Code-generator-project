"""Organization models matching 00004_enterprise.sql, 00005, 00007 + Plan §6.4.

Plan §6.2 & §6.3:
- organizations
- org_members
- org_domains
- org_security_policies
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import F, Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIDRField, CIText, FixedCharField
from apps.core.ids import uuid7


class Organization(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    name = models.TextField()
    slug = CIText(unique=True)
    kind = models.TextField(
        default="standard",
        choices=[
            ("personal", "Personal"),
            ("standard", "Standard"),
            ("enterprise", "Enterprise"),
            ("agency", "Agency"),
        ],
    )
    parent_org = models.ForeignKey(
        "self",
        on_delete=models.PROTECT,
        null=True,
        blank=True,
        db_column="parent_org_id",
        related_name="client_orgs",
    )
    plan_id = models.TextField(
        default="free",
        choices=[
            ("free", "Free"),
            ("pro", "Pro"),
            ("business", "Business"),
            ("enterprise", "Enterprise"),
        ],
    )
    data_region = models.TextField(
        default="in",
        choices=[
            ("in", "India"),
            ("eu", "Europe"),
            ("us", "United States"),
        ],
    )
    legal_name = models.TextField(null=True, blank=True)
    billing_email = CIText(null=True, blank=True)
    gstin = models.TextField(null=True, blank=True)
    tax_country = FixedCharField(max_length=2, default="IN")
    billing_address = models.JSONField(default=dict)
    settings = models.JSONField(default=dict)
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)
    deleted_at = models.DateTimeField(null=True, blank=True)
    billing_hold_since = models.DateTimeField(null=True, blank=True)
    grandfathered_limits = models.JSONField(default=dict)

    class Meta:
        db_table = "organizations"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(kind__in=["personal", "standard", "enterprise", "agency"]),
                name="organizations_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(plan_id__in=["free", "pro", "business", "enterprise"]),
                name="organizations_plan_id_check",
            ),
            models.CheckConstraint(
                condition=models.Q(data_region__in=["in", "eu", "us"]),
                name="organizations_data_region_check",
            ),
            models.CheckConstraint(
                condition=Q(gstin__isnull=True)
                | Q(gstin__regex=r"^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$"),
                name="organizations_gstin_check",
            ),
            models.CheckConstraint(
                condition=Q(parent_org__isnull=True) | ~Q(parent_org=F("id")),
                name="org_no_self_parent",
            ),
        ]
        indexes = [
            models.Index(
                fields=["parent_org"],
                name="organizations_parent_idx",
                condition=Q(parent_org__isnull=False),
            ),
        ]

    def __str__(self) -> str:
        return self.name


class OrgMember(models.Model):
    pk = models.CompositePrimaryKey("org_id", "user_id")
    org = models.ForeignKey(Organization, on_delete=models.CASCADE, related_name="members")
    user = models.ForeignKey(
        "accounts.User", on_delete=models.CASCADE, related_name="org_memberships"
    )
    org_role = models.TextField(
        default="member",
        choices=[
            ("org_owner", "Org Owner"),
            ("org_admin", "Org Admin"),
            ("billing_admin", "Billing Admin"),
            ("member", "Member"),
        ],
    )
    status = models.TextField(
        default="active",
        choices=[
            ("active", "Active"),
            ("suspended", "Suspended"),
            ("deprovisioned", "Deprovisioned"),
        ],
    )
    source = models.TextField(
        default="invite",
        choices=[
            ("invite", "Invite"),
            ("sso_jit", "SSO JIT"),
            ("scim", "SCIM"),
            ("creator", "Creator"),
        ],
    )
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "org_members"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    org_role__in=["org_owner", "org_admin", "billing_admin", "member"]
                ),
                name="org_members_org_role_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "suspended", "deprovisioned"]),
                name="org_members_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(source__in=["invite", "sso_jit", "scim", "creator"]),
                name="org_members_source_check",
            ),
            models.UniqueConstraint(
                fields=["org"],
                name="org_one_owner_uniq",
                condition=Q(org_role="org_owner"),
            ),
        ]
        indexes = [
            models.Index(fields=["user"], name="org_members_user_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.user_id} in {self.org_id} ({self.org_role})"


class OrgDomain(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(Organization, on_delete=models.CASCADE, related_name="domains")
    domain = CIText()
    verification_token = models.TextField()
    verified_at = models.DateTimeField(null=True, blank=True)
    auto_join = models.BooleanField(default=False)
    auto_join_workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    last_checked_at = models.DateTimeField(null=True, blank=True)
    check_attempts = models.IntegerField(default=0)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "org_domains"
        constraints = [
            models.UniqueConstraint(
                fields=["org", "domain"],
                name="org_domains_org_domain_key",
            ),
            models.UniqueConstraint(
                fields=["domain"],
                name="org_domains_verified_uniq",
                condition=Q(verified_at__isnull=False),
            ),
        ]

    def __str__(self) -> str:
        return f"{self.domain} ({self.org_id})"


def default_allowed_mfa() -> list[str]:
    return ["totp", "webauthn"]


class OrgSecurityPolicy(models.Model):
    org = models.OneToOneField(
        Organization,
        on_delete=models.CASCADE,
        primary_key=True,
        related_name="security_policy",
        db_column="org_id",
    )
    enforce_sso = models.BooleanField(default=False)
    sso_break_glass_user_ids = ArrayField(models.UUIDField(), default=list)
    require_mfa = models.BooleanField(default=False)
    allowed_mfa_kinds = ArrayField(models.TextField(), default=default_allowed_mfa)
    session_idle_minutes = models.IntegerField(default=0)
    session_max_hours = models.IntegerField(default=0)
    dashboard_ip_allowlist = ArrayField(CIDRField(), default=list)
    api_ip_allowlist = ArrayField(CIDRField(), default=list)
    password_min_length = models.IntegerField(default=10)
    invite_email_domains = ArrayField(CIText(), default=list)
    api_key_max_days = models.IntegerField(default=0)
    export_permission = models.TextField(
        default="role",
        choices=[
            ("role", "Role"),
            ("admins_only", "Admins Only"),
            ("disabled", "Disabled"),
        ],
    )
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
        db_table = "org_security_policies"
        constraints = [
            models.CheckConstraint(
                condition=Q(session_idle_minutes=0)
                | Q(session_idle_minutes__gte=5, session_idle_minutes__lte=10080),
                name="org_security_policies_session_idle_minutes_check",
            ),
            models.CheckConstraint(
                condition=Q(session_max_hours=0)
                | Q(session_max_hours__gte=1, session_max_hours__lte=2160),
                name="org_security_policies_session_max_hours_check",
            ),
            models.CheckConstraint(
                condition=Q(password_min_length__gte=10, password_min_length__lte=128),
                name="org_security_policies_password_min_length_check",
            ),
            models.CheckConstraint(
                condition=Q(api_key_max_days__gte=0),
                name="org_security_policies_api_key_max_days_check",
            ),
            models.CheckConstraint(
                condition=models.Q(export_permission__in=["role", "admins_only", "disabled"]),
                name="org_security_policies_export_permission_check",
            ),
        ]

    def __str__(self) -> str:
        return f"Security policy for {self.org_id}"
