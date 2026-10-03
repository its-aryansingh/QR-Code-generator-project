"""Developer platform models (API keys, usage, and bulk jobs).

Plan §6.2, §6.3, §6.4 & §7.21:
- api_keys
- api_usage_daily
- jobs
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.utils.timezone import now as tz_now

from apps.core.fields import CIDRField
from apps.core.ids import uuid7


def default_scopes() -> list[str]:
    return ["qr:read", "qr:write", "analytics:read"]


class ApiKey(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="api_keys",
    )
    name = models.TextField()
    prefix = models.TextField(unique=True)
    key_hash = models.BinaryField(unique=True)
    scopes = ArrayField(models.TextField(), default=default_scopes)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_api_keys",
    )
    last_used_at = models.DateTimeField(null=True, blank=True)
    expires_at = models.DateTimeField(null=True, blank=True)
    revoked_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    # Enterprise additions:
    environment = models.TextField(
        default="live",
        choices=[
            ("live", "Live"),
            ("test", "Test"),
        ],
    )
    ip_allowlist = ArrayField(CIDRField(), default=list)
    # Plan §6.4 deltas:
    legacy_bcrypt_hash = models.TextField(null=True, blank=True)
    legacy_kind = models.TextField(
        null=True,
        blank=True,
        choices=[
            ("v1_user", "v1 User"),
            ("v1_workspace", "v1 Workspace"),
        ],
    )

    class Meta:
        db_table = "api_keys"
        indexes = [
            models.Index(
                fields=["workspace"],
                condition=models.Q(revoked_at__isnull=True),
                name="api_keys_ws_idx",
            ),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(environment__in=["live", "test"]),
                name="api_keys_environment_check",
            ),
            models.CheckConstraint(
                condition=models.Q(legacy_kind__isnull=True)
                | models.Q(legacy_kind__in=["v1_user", "v1_workspace"]),
                name="api_keys_legacy_kind_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.prefix})"


class ApiUsageDaily(models.Model):
    pk = models.CompositePrimaryKey("api_key_id", "day")
    api_key_id = models.UUIDField()
    day = models.DateField()
    requests = models.IntegerField(default=0)
    errors = models.IntegerField(default=0)
    throttled = models.IntegerField(default=0)

    class Meta:
        db_table = "api_usage_daily"

    def __str__(self) -> str:
        return f"Usage for key {self.api_key_id} on {self.day}"


class Job(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="jobs",
    )
    kind = models.TextField()
    status = models.TextField(default="queued")
    input_file = models.ForeignKey(
        "qr.File",
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="input_file_id",
        related_name="input_for_jobs",
    )
    output_file = models.ForeignKey(
        "qr.File",
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="output_file_id",
        related_name="output_for_jobs",
    )
    params = models.JSONField(default=dict)
    total = models.IntegerField(default=0)
    processed = models.IntegerField(default=0)
    failed = models.IntegerField(default=0)
    errors = models.JSONField(default=list)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_jobs",
    )
    created_at = models.DateTimeField(default=tz_now)
    started_at = models.DateTimeField(null=True, blank=True)
    finished_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "jobs"
        indexes = [
            models.Index(fields=["workspace", "-created_at"], name="jobs_ws_time_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    kind__in=[
                        "bulk_create",
                        "bulk_update",
                        "bulk_download",
                        "export_scans",
                        "export_qr_codes",
                        "print_sheet",
                        "serial_generate",
                        "serial_export",
                        "gs1_import",
                        "warehouse_export",
                        "audit_export",
                        "dsar_export",
                        "lead_export",
                        "report_run",
                    ]
                ),
                name="jobs_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    status__in=["queued", "running", "succeeded", "failed", "canceled"]
                ),
                name="jobs_status_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.kind}:{self.id}"
