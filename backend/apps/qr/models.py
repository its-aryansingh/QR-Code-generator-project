"""QR codes, versions, templates, files, and domains models.

Plan §6.3 & §7.7:
- domains
- folders
- tags
- campaigns
- templates
- files
- qr_codes
- short_code_tombstones
- qr_code_tags
- qr_versions
"""

from django.db import models
from django.utils import timezone

from apps.core.fields import CIText, FixedCharField
from apps.core.ids import uuid7


class Domain(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="domains",
    )
    hostname = CIText(unique=True)
    status = models.TextField(default="pending")
    verification_token = models.TextField()
    provider_hostname_id = models.TextField(null=True, blank=True)
    tls_status = models.TextField(default="pending")
    root_redirect_url = models.TextField(null=True, blank=True)
    not_found_url = models.TextField(null=True, blank=True)
    last_checked_at = models.DateTimeField(null=True, blank=True)
    verified_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "domains"
        indexes = [
            models.Index(fields=["workspace"], name="domains_workspace_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    status__in=["pending", "verifying", "active", "failed", "disabled"]
                ),
                name="domains_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(tls_status__in=["pending", "active", "failed"]),
                name="domains_tls_status_check",
            ),
        ]

    def __str__(self) -> str:
        return str(self.hostname)


class Folder(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="folders",
    )
    parent = models.ForeignKey(
        "self",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        db_column="parent_id",
        related_name="children",
    )
    name = models.TextField()
    position = models.IntegerField(default=0)
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "folders"
        constraints = [
            models.UniqueConstraint(
                fields=["workspace", "parent", "name"],
                nulls_distinct=False,
                name="folders_workspace_id_parent_id_name_key",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class Tag(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="tags",
    )
    name = CIText()
    color = models.TextField(default="#64748B")
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "tags"
        constraints = [
            models.UniqueConstraint(
                fields=["workspace", "name"], name="tags_workspace_id_name_key"
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.name}"


class Campaign(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="campaigns",
    )
    name = models.TextField()
    status = models.TextField(default="active")
    starts_at = models.DateTimeField(null=True, blank=True)
    ends_at = models.DateTimeField(null=True, blank=True)
    goal_scans = models.BigIntegerField(null=True, blank=True)
    utm = models.JSONField(default=dict)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_campaigns",
    )
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "campaigns"
        indexes = [
            models.Index(fields=["workspace", "status"], name="campaigns_ws_status_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(status__in=["draft", "active", "paused", "ended", "archived"]),
                name="campaigns_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(goal_scans__isnull=True) | models.Q(goal_scans__gt=0),
                name="campaigns_goal_scans_check",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class Template(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="templates",
    )
    name = models.TextField()
    design = models.JSONField()
    is_locked = models.BooleanField(default=False)
    is_default = models.BooleanField(default=False)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_templates",
    )
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "templates"
        constraints = [
            models.UniqueConstraint(
                fields=["workspace"],
                condition=models.Q(is_default=True),
                name="templates_one_default_uniq",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class File(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="files",
    )
    purpose = models.TextField()
    storage_key = models.TextField(unique=True)
    mime_type = models.TextField()
    size_bytes = models.BigIntegerField()
    sha256 = models.BinaryField()
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="uploaded_files",
    )
    created_at = models.DateTimeField(default=timezone.now)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "files"
        indexes = [
            models.Index(fields=["workspace", "purpose"], name="files_ws_purpose_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    purpose__in=["logo", "hosted_asset", "export", "bulk_input", "bulk_output"]
                ),
                name="files_purpose_check",
            ),
            models.CheckConstraint(
                condition=models.Q(size_bytes__gte=0),
                name="files_size_bytes_check",
            ),
        ]

    def __str__(self) -> str:
        return self.storage_key


class QRCode(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="qr_codes",
    )
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_qr_codes",
    )
    mode = models.TextField()
    content_type = models.TextField()
    name = models.TextField()
    domain = models.ForeignKey(
        Domain,
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="domain_id",
        related_name="qr_codes",
    )
    short_code = models.TextField(null=True, blank=True)
    legacy_short_code = models.TextField(null=True, blank=True)
    legacy_host = CIText(null=True, blank=True)
    v1_printed_payload = models.TextField(null=True, blank=True)
    needs_reprint = models.BooleanField(default=False)
    gs1_gtin = FixedCharField(max_length=14, null=True, blank=True)
    static_payload = models.TextField(null=True, blank=True)
    static_content = models.JSONField(null=True, blank=True)
    current_version = models.ForeignKey(
        "QRVersion",
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="current_version_id",
        related_name="current_for_qr_codes",
    )
    design = models.JSONField(default=dict)
    design_hash = models.BinaryField(null=True, blank=True)
    template = models.ForeignKey(
        Template,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="template_id",
        related_name="qr_codes",
    )
    folder = models.ForeignKey(
        Folder,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="folder_id",
        related_name="qr_codes",
    )
    campaign = models.ForeignKey(
        Campaign,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="campaign_id",
        related_name="qr_codes",
    )
    status = models.TextField(default="active")
    is_read_only = models.BooleanField(default=False)
    starts_at = models.DateTimeField(null=True, blank=True)
    expires_at = models.DateTimeField(null=True, blank=True)
    scan_limit = models.BigIntegerField(null=True, blank=True)
    password_hash = models.TextField(null=True, blank=True)
    fallback_url = models.TextField(null=True, blank=True)
    safety_status = models.TextField(default="pending")
    total_scans = models.BigIntegerField(default=0)
    unique_scans = models.BigIntegerField(default=0)
    last_scanned_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)
    archived_at = models.DateTimeField(null=True, blank=True)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "qr_codes"
        indexes = [
            models.Index(
                fields=["workspace", "created_at", "id"],
                condition=models.Q(deleted_at__isnull=True),
                name="qr_codes_ws_list_idx",
            ),
            models.Index(
                fields=["workspace", "folder"],
                condition=models.Q(deleted_at__isnull=True),
                name="qr_codes_ws_folder_idx",
            ),
            models.Index(
                fields=["workspace", "campaign"],
                condition=models.Q(deleted_at__isnull=True),
                name="qr_codes_ws_campaign_idx",
            ),
            models.Index(
                fields=["legacy_host", "legacy_short_code"],
                condition=models.Q(legacy_short_code__isnull=False),
                name="qr_codes_legacy_host_idx",
            ),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(mode__in=["static", "dynamic"]),
                name="qr_codes_mode_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    content_type__in=[
                        "url",
                        "text",
                        "email",
                        "phone",
                        "sms",
                        "whatsapp",
                        "wifi",
                        "vcard",
                        "event",
                        "upi",
                        "location",
                        "links_page",
                        "file",
                        "app_store",
                        "gs1",
                        "form",
                        "serial_batch",
                    ]
                ),
                name="qr_codes_content_type_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["active", "paused", "archived", "blocked"]),
                name="qr_codes_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(safety_status__in=["pending", "safe", "flagged", "blocked"]),
                name="qr_codes_safety_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(scan_limit__isnull=True) | models.Q(scan_limit__gt=0),
                name="qr_codes_scan_limit_check",
            ),
            models.CheckConstraint(
                condition=(
                    models.Q(
                        mode="dynamic",
                        short_code__isnull=False,
                        domain__isnull=False,
                        static_payload__isnull=True,
                    )
                    | models.Q(mode="static", short_code__isnull=True, static_payload__isnull=False)
                ),
                name="qr_dynamic_has_link",
            ),
            models.CheckConstraint(
                condition=models.Q(mode="dynamic")
                | ~models.Q(
                    content_type__in=[
                        "links_page",
                        "file",
                        "app_store",
                        "gs1",
                        "form",
                        "serial_batch",
                    ]
                ),
                name="qr_static_types",
            ),
            models.CheckConstraint(
                condition=models.Q(mode="static")
                | ~models.Q(content_type__in=["wifi", "upi", "text", "location"]),
                name="qr_dynamic_types",
            ),
            models.UniqueConstraint(
                fields=["domain", "short_code"],
                condition=models.Q(short_code__isnull=False),
                name="qr_codes_domain_code_uniq",
            ),
            models.UniqueConstraint(
                fields=["legacy_short_code"],
                condition=models.Q(legacy_short_code__isnull=False),
                name="qr_codes_legacy_code_uniq",
            ),
            models.UniqueConstraint(
                fields=["domain", "gs1_gtin"],
                condition=models.Q(gs1_gtin__isnull=False),
                name="qr_codes_domain_gtin_uniq",
            ),
        ]

    def __str__(self) -> str:
        return self.name


class ShortCodeTombstone(models.Model):
    pk = models.CompositePrimaryKey("domain", "short_code")
    domain = models.ForeignKey(
        Domain,
        on_delete=models.DO_NOTHING,
        db_column="domain_id",
        related_name="tombstones",
    )
    short_code = models.TextField()
    purged_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "short_code_tombstones"

    def __str__(self) -> str:
        return f"{self.domain_id}:{self.short_code}"


class QRCodeTag(models.Model):
    pk = models.CompositePrimaryKey("qr_code", "tag")
    qr_code = models.ForeignKey(
        QRCode,
        on_delete=models.CASCADE,
        db_column="qr_code_id",
        related_name="code_tags",
    )
    tag = models.ForeignKey(
        Tag,
        on_delete=models.CASCADE,
        db_column="tag_id",
        related_name="tagged_codes",
    )

    class Meta:
        db_table = "qr_code_tags"
        indexes = [
            models.Index(fields=["tag"], name="qr_code_tags_tag_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.qr_code_id}:{self.tag_id}"


class QRVersion(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    qr_code = models.ForeignKey(
        QRCode,
        on_delete=models.CASCADE,
        db_column="qr_code_id",
        related_name="versions",
    )
    version_no = models.IntegerField()
    destination_kind = models.TextField()
    destination_url = models.TextField(null=True, blank=True)
    hosted_page = models.JSONField(null=True, blank=True)
    rules = models.JSONField(default=list)
    utm = models.JSONField(default=dict)
    effective_at = models.DateTimeField(default=timezone.now)
    safety_status = models.TextField(default="pending")
    restored_from = models.ForeignKey(
        "self",
        null=True,
        blank=True,
        on_delete=models.DO_NOTHING,
        db_column="restored_from",
        related_name="restored_to",
    )
    change_note = models.TextField(null=True, blank=True)
    created_by = models.ForeignKey(
        "accounts.User",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="created_by",
        related_name="created_versions",
    )
    created_by_key = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)
    approval_status = models.TextField(
        default="not_required",
        choices=[
            ("not_required", "Not Required"),
            ("pending", "Pending"),
            ("approved", "Approved"),
            ("rejected", "Rejected"),
            ("cancelled", "Cancelled"),
        ],
    )
    approval_request = models.ForeignKey(
        "approvals.ApprovalRequest",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        db_column="approval_request_id",
        related_name="versions",
    )

    class Meta:
        db_table = "qr_versions"
        indexes = [
            models.Index(
                fields=["qr_code", "effective_at", "version_no"], name="qr_versions_effective_idx"
            ),
            models.Index(
                fields=["qr_code"],
                name="qr_versions_pending_idx",
                condition=models.Q(approval_status="pending"),
            ),
            models.Index(
                fields=["approval_request"],
                name="qr_versions_approval_req_idx",
                condition=models.Q(approval_request__isnull=False),
            ),
        ]
        constraints = [
            models.UniqueConstraint(
                fields=["qr_code", "version_no"], name="qr_versions_qr_code_id_version_no_key"
            ),
            models.CheckConstraint(
                condition=models.Q(version_no__gt=0),
                name="qr_versions_version_no_check",
            ),
            models.CheckConstraint(
                condition=models.Q(destination_kind__in=["url", "hosted_page"]),
                name="qr_versions_destination_kind_check",
            ),
            models.CheckConstraint(
                condition=models.Q(safety_status__in=["pending", "safe", "flagged", "blocked"]),
                name="qr_versions_safety_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    approval_status__in=[
                        "not_required",
                        "pending",
                        "approved",
                        "rejected",
                        "cancelled",
                    ]
                ),
                name="qr_versions_approval_status_check",
            ),
            models.CheckConstraint(
                condition=(
                    models.Q(
                        destination_kind="url",
                        destination_url__isnull=False,
                        hosted_page__isnull=True,
                    )
                    | models.Q(destination_kind="hosted_page", hosted_page__isnull=False)
                ),
                name="qr_version_destination",
            ),
            models.CheckConstraint(
                condition=~models.Q(approval_status__in=["pending", "approved", "rejected"])
                | models.Q(approval_request__isnull=False),
                name="qr_version_pending_has_request",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.qr_code_id}:v{self.version_no}"
