"""White-label branding and custom domain models for organizations.

Plan §6.2 & §6.3:
- org_branding
"""

from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText


class OrgBranding(models.Model):
    org = models.OneToOneField(
        "orgs.Organization",
        on_delete=models.CASCADE,
        primary_key=True,
        related_name="branding",
        db_column="org_id",
    )
    app_hostname = CIText(unique=True, null=True, blank=True)
    app_hostname_status = models.TextField(
        default="none",
        choices=[
            ("none", "None"),
            ("pending", "Pending"),
            ("active", "Active"),
            ("failed", "Failed"),
        ],
    )
    provider_hostname_id = models.TextField(null=True, blank=True)
    email_domain = CIText(unique=True, null=True, blank=True)
    email_domain_status = models.TextField(
        default="none",
        choices=[
            ("none", "None"),
            ("pending", "Pending"),
            ("active", "Active"),
            ("failed", "Failed"),
        ],
    )
    email_provider_id = models.TextField(null=True, blank=True)
    product_name = models.TextField(null=True, blank=True)
    logo_file = models.ForeignKey(
        "qr.File",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    favicon_file = models.ForeignKey(
        "qr.File",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    primary_color = models.TextField(null=True, blank=True)
    support_url = models.TextField(null=True, blank=True)
    hide_platform_brand = models.BooleanField(default=False)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "org_branding"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(app_hostname_status__in=["none", "pending", "active", "failed"]),
                name="org_branding_app_hostname_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(email_domain_status__in=["none", "pending", "active", "failed"]),
                name="org_branding_email_domain_status_check",
            ),
            models.CheckConstraint(
                condition=Q(primary_color__isnull=True)
                | Q(primary_color__regex=r"^#[0-9A-Fa-f]{6}$"),
                name="org_branding_primary_color_check",
            ),
        ]

    def __str__(self) -> str:
        return f"Branding for {self.org_id}"
