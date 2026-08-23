import uuid
from django.db import models


class User(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    email = models.CharField(max_length=255, unique=True)
    password_hash = models.CharField(max_length=255)
    name = models.CharField(max_length=255, blank=True, null=True)
    company = models.CharField(max_length=255, blank=True, null=True)
    avatar_url = models.TextField(blank=True, null=True)
    plan = models.CharField(max_length=20, default="free", blank=True, null=True)
    plan_expires_at = models.DateTimeField(null=True, blank=True)
    subscription_status = models.CharField(max_length=20, default="", blank=True, null=True)
    subscription_ends_at = models.DateTimeField(null=True, blank=True)
    api_key = models.CharField(max_length=64, unique=True, null=True, blank=True)
    api_calls_today = models.BigIntegerField(default=0, null=True)
    api_calls_reset_at = models.DateField(null=True, blank=True)
    stripe_customer_id = models.CharField(max_length=255, null=True, blank=True)
    stripe_subscription_id = models.CharField(max_length=255, null=True, blank=True)
    default_workspace_id = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "users"


class QRRecord(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(User, on_delete=models.CASCADE, null=True, blank=True,
                             db_column="user_id", related_name="qr_records")
    workspace_id = models.UUIDField(null=True, blank=True)
    folder_id = models.UUIDField(null=True, blank=True)
    tags = models.TextField(blank=True, null=True)
    title = models.CharField(max_length=255, blank=True, null=True)
    content = models.TextField()
    qr_type = models.CharField(max_length=50, default="url", null=True)
    qr_type_id = models.CharField(max_length=50, blank=True, null=True)
    size = models.BigIntegerField(default=256, null=True)
    short_code = models.CharField(max_length=20, unique=True, null=True, blank=True)
    is_dynamic = models.BooleanField(default=False, null=True)
    redirect_url = models.TextField(blank=True, null=True)
    metadata = models.JSONField(default=dict, null=True)
    customization = models.JSONField(default=dict, null=True)
    password = models.CharField(max_length=255, blank=True, null=True)
    max_scans = models.BigIntegerField(null=True, blank=True)
    scheduled_at = models.DateTimeField(null=True, blank=True)
    geo_restrictions = models.TextField(blank=True, null=True)
    is_active = models.BooleanField(default=True, null=True)
    expires_at = models.DateTimeField(null=True, blank=True)
    scan_count = models.BigIntegerField(default=0, null=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "qr_records"


class QRScan(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    qr_record = models.ForeignKey(QRRecord, on_delete=models.CASCADE,
                                  db_column="qr_id", related_name="scans")
    scanned_at = models.DateTimeField(null=True, blank=True)
    ip_address = models.CharField(max_length=45, blank=True, null=True)
    country_code = models.CharField(max_length=2, blank=True, null=True)
    country_name = models.CharField(max_length=100, blank=True, null=True)
    city = models.CharField(max_length=100, blank=True, null=True)
    region = models.CharField(max_length=100, blank=True, null=True)
    latitude = models.DecimalField(max_digits=9, decimal_places=6, null=True, blank=True)
    longitude = models.DecimalField(max_digits=9, decimal_places=6, null=True, blank=True)
    device_type = models.CharField(max_length=20, blank=True, null=True)
    os = models.CharField(max_length=50, blank=True, null=True)
    os_version = models.CharField(max_length=20, blank=True, null=True)
    browser = models.CharField(max_length=50, blank=True, null=True)
    browser_version = models.CharField(max_length=20, blank=True, null=True)
    user_agent = models.TextField(blank=True, null=True)
    referrer = models.TextField(blank=True, null=True)
    language = models.CharField(max_length=10, blank=True, null=True)

    class Meta:
        managed = True
        db_table = "qr_scans"


class QRFile(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    qr_id = models.UUIDField(null=True, blank=True)
    user_id = models.UUIDField()
    filename = models.CharField(max_length=255)
    original_filename = models.CharField(max_length=255, blank=True, null=True)
    file_type = models.CharField(max_length=50)
    mime_type = models.CharField(max_length=100, blank=True, null=True)
    file_size = models.BigIntegerField(default=0, null=True)
    s3_bucket = models.CharField(max_length=100, blank=True, null=True)
    s3_key = models.TextField()
    s3_url = models.TextField(blank=True, null=True)
    is_processed = models.BooleanField(default=False, null=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "qr_files"


class FreeTierUsage(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    ip_address = models.CharField(max_length=45)
    session_id = models.CharField(max_length=64, blank=True, null=True)
    qr_count = models.BigIntegerField(default=0, null=True)
    date = models.DateField(null=True)
    first_used = models.DateTimeField(null=True, blank=True)
    last_used = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        # Keep the table name used by the deployed pre-Django backend.
        db_table = "free_tier_usages"
        unique_together = [["ip_address", "date"]]


class Workspace(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    name = models.CharField(max_length=255)
    slug = models.CharField(max_length=255, unique=True)
    description = models.TextField(blank=True, null=True)
    logo_url = models.TextField(blank=True, null=True)
    owner = models.ForeignKey(User, on_delete=models.CASCADE, db_column="owner_id", related_name="workspaces")
    plan = models.CharField(max_length=20, default="free", null=True)
    max_members = models.BigIntegerField(default=1, null=True)
    max_qr_codes = models.BigIntegerField(default=50, null=True)
    max_folders = models.BigIntegerField(default=5, null=True)
    custom_domain = models.CharField(max_length=255, blank=True, null=True)
    brand_color = models.CharField(max_length=7, default="#8B5CF6", null=True)
    brand_logo = models.TextField(blank=True, null=True)
    favicon_url = models.TextField(blank=True, null=True)
    custom_css = models.TextField(blank=True, null=True)
    remove_branding = models.BooleanField(default=False, null=True)
    custom_footer = models.TextField(blank=True, null=True)
    sso_enabled = models.BooleanField(default=False, null=True)
    sso_provider = models.CharField(max_length=20, blank=True, null=True)
    sso_config = models.TextField(blank=True, null=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "workspaces"


class WorkspaceMember(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey(Workspace, on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="members")
    user = models.ForeignKey(User, on_delete=models.CASCADE,
                             db_column="user_id", related_name="workspace_memberships")
    role = models.CharField(max_length=20, default="viewer")
    invited_by = models.UUIDField(null=True, blank=True)
    joined_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "workspace_members"
        unique_together = [["workspace", "user"]]


class WorkspaceInvite(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey(Workspace, on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="invites")
    email = models.CharField(max_length=255)
    role = models.CharField(max_length=20, default="viewer")
    token = models.CharField(max_length=64, unique=True)
    invited_by = models.UUIDField()
    status = models.CharField(max_length=20, default="pending")
    expires_at = models.DateTimeField()
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "workspace_invites"


class Folder(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey(Workspace, on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="folders")
    parent_id = models.UUIDField(null=True, blank=True)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, null=True)
    color = models.CharField(max_length=7, default="#8B5CF6")
    icon = models.CharField(max_length=50, blank=True, null=True)
    sort_order = models.IntegerField(default=0)
    qr_count = models.IntegerField(default=0)
    scan_count = models.IntegerField(default=0)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "folders"


class AuditLog(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace_id = models.UUIDField()
    user = models.ForeignKey(User, on_delete=models.CASCADE,
                             db_column="user_id", related_name="audit_logs")
    action = models.CharField(max_length=50)
    resource = models.CharField(max_length=50)
    resource_id = models.UUIDField(null=True, blank=True)
    details = models.TextField(blank=True, null=True)
    ip_address = models.CharField(max_length=45, blank=True, null=True)
    user_agent = models.TextField(blank=True, null=True)
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "audit_logs"


class Webhook(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace_id = models.UUIDField()
    url = models.TextField()
    secret = models.CharField(max_length=64, blank=True, null=True)
    events = models.TextField()
    is_active = models.BooleanField(default=True)
    description = models.TextField(blank=True, null=True)
    last_triggered = models.DateTimeField(null=True, blank=True)
    fail_count = models.IntegerField(default=0)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "webhooks"


class WebhookLog(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    webhook = models.ForeignKey(Webhook, on_delete=models.CASCADE,
                                db_column="webhook_id", related_name="logs")
    event = models.CharField(max_length=50)
    payload = models.TextField(blank=True, null=True)
    status_code = models.IntegerField(default=0)
    response = models.TextField(blank=True, null=True)
    success = models.BooleanField(default=False)
    duration = models.IntegerField(default=0)
    error = models.TextField(blank=True, null=True)
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "webhook_logs"


class LeadCapturePage(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace_id = models.UUIDField()
    qr_record_id = models.UUIDField(null=True, blank=True)
    name = models.CharField(max_length=255)
    slug = models.CharField(max_length=255, unique=True)
    headline = models.CharField(max_length=500, blank=True, null=True)
    subheadline = models.TextField(blank=True, null=True)
    hero_image = models.TextField(blank=True, null=True)
    button_text = models.CharField(max_length=100, default="Submit")
    button_color = models.CharField(max_length=7, default="#8B5CF6")
    background_color = models.CharField(max_length=7, default="#09090B")
    text_color = models.CharField(max_length=7, default="#FAFAFA")
    thank_you_message = models.TextField(default="Thank you for signing up!")
    redirect_url = models.TextField(blank=True, null=True)
    form_fields = models.TextField(default="[]")
    is_active = models.BooleanField(default=True)
    requires_opt_in = models.BooleanField(default=False)
    privacy_policy = models.TextField(blank=True, null=True)
    consent_text = models.TextField(blank=True, null=True)
    views = models.IntegerField(default=0)
    submissions = models.IntegerField(default=0)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "lead_capture_pages"


class Lead(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    page = models.ForeignKey(LeadCapturePage, on_delete=models.CASCADE,
                             db_column="page_id", related_name="leads")
    data = models.TextField()
    email = models.CharField(max_length=255, blank=True, null=True)
    ip_address = models.CharField(max_length=45, blank=True, null=True)
    user_agent = models.TextField(blank=True, null=True)
    source = models.CharField(max_length=255, blank=True, null=True)
    opted_in = models.BooleanField(default=False)
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "leads"


# ====================================================================
# Enterprise layer
# ====================================================================


class Campaign(models.Model):
    """A named marketing initiative that QR codes roll up into.

    Distinct from Folder: folders are storage, campaigns are measurement.
    A QR lives in one folder and is measured against one campaign.
    """

    STATUS_CHOICES = ["draft", "scheduled", "active", "paused", "completed", "archived"]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey("Workspace", on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="campaigns")
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, null=True)
    status = models.CharField(max_length=20, default="draft")
    color = models.CharField(max_length=7, default="#8B5CF6")
    starts_at = models.DateTimeField(null=True, blank=True)
    ends_at = models.DateTimeField(null=True, blank=True)
    scan_goal = models.BigIntegerField(default=0)
    utm_source = models.CharField(max_length=100, blank=True, null=True)
    utm_medium = models.CharField(max_length=100, blank=True, null=True)
    utm_campaign = models.CharField(max_length=100, blank=True, null=True)
    utm_term = models.CharField(max_length=100, blank=True, null=True)
    utm_content = models.CharField(max_length=100, blank=True, null=True)
    tags = models.TextField(blank=True, null=True)
    created_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "campaigns"
        indexes = [models.Index(fields=["workspace", "status"])]


class QRTemplate(models.Model):
    """Brand-locked design preset.

    When is_locked is set, editors must use the design verbatim. That is how
    brand teams keep ten thousand printed codes on-brand.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey("Workspace", on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="templates")
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, null=True)
    design = models.JSONField(default=dict)
    preview_url = models.TextField(blank=True, null=True)
    is_default = models.BooleanField(default=False)
    is_locked = models.BooleanField(default=False)
    usage_count = models.IntegerField(default=0)
    created_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "qr_templates"


class RoutingRule(models.Model):
    """Smart-routing condition evaluated at scan time, highest priority first.

    Weighted rules implement A/B split testing; every other condition is a
    deterministic match against the incoming request.
    """

    CONDITIONS = ["country", "device", "os", "language", "time_of_day",
                  "date_range", "weight", "scan_count"]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    qr_record = models.ForeignKey("QRRecord", on_delete=models.CASCADE,
                                  db_column="qr_id", related_name="routing_rules")
    name = models.CharField(max_length=255, blank=True, null=True)
    condition = models.CharField(max_length=30)
    operator = models.CharField(max_length=20, default="in")
    value = models.TextField(blank=True, null=True)
    destination_url = models.TextField()
    priority = models.IntegerField(default=0)
    is_active = models.BooleanField(default=True)
    hit_count = models.BigIntegerField(default=0)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "routing_rules"
        ordering = ["-priority", "created_at"]


class ApiKey(models.Model):
    """Workspace-scoped API credential.

    Only the hash is stored; the plaintext is returned once at creation.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey("Workspace", on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="api_keys")
    name = models.CharField(max_length=255)
    prefix = models.CharField(max_length=16, db_index=True)
    key_hash = models.CharField(max_length=255)
    scopes = models.TextField(default="qr:read,qr:write,analytics:read")
    last_used_at = models.DateTimeField(null=True, blank=True)
    calls_today = models.BigIntegerField(default=0)
    calls_reset_at = models.DateField(null=True, blank=True)
    total_calls = models.BigIntegerField(default=0)
    expires_at = models.DateTimeField(null=True, blank=True)
    revoked_at = models.DateTimeField(null=True, blank=True)
    created_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "workspace_api_keys"


class CustomDomain(models.Model):
    """Branded short-link domain.

    A dedicated domain with isolated reputation is the primary defence that
    keeps a quishing takedown on a shared domain from killing your codes.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey("Workspace", on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="domains")
    domain = models.CharField(max_length=255, unique=True)
    status = models.CharField(max_length=20, default="pending")
    verification_token = models.CharField(max_length=64)
    verification_method = models.CharField(max_length=20, default="dns_txt")
    is_primary = models.BooleanField(default=False)
    ssl_status = models.CharField(max_length=20, default="pending")
    last_checked_at = models.DateTimeField(null=True, blank=True)
    verified_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "custom_domains"


class SsoConfig(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.OneToOneField("Workspace", on_delete=models.CASCADE,
                                     db_column="workspace_id", related_name="sso")
    is_enabled = models.BooleanField(default=False)
    provider = models.CharField(max_length=20, default="saml")
    entity_id = models.CharField(max_length=500, blank=True, null=True)
    sso_url = models.TextField(blank=True, null=True)
    slo_url = models.TextField(blank=True, null=True)
    certificate = models.TextField(blank=True, null=True)
    metadata_url = models.TextField(blank=True, null=True)
    email_domains = models.TextField(blank=True, null=True)
    default_role = models.CharField(max_length=20, default="viewer")
    enforce_sso = models.BooleanField(default=False)
    scim_enabled = models.BooleanField(default=False)
    scim_token_hash = models.CharField(max_length=255, blank=True, null=True)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "sso_configs"


class SecurityPolicy(models.Model):
    """Per-workspace guardrails.

    allowed_domains stops an editor from pointing a printed code at an
    off-brand or attacker-controlled host.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.OneToOneField("Workspace", on_delete=models.CASCADE,
                                     db_column="workspace_id", related_name="security_policy")
    allowed_domains = models.TextField(blank=True, null=True)
    blocked_domains = models.TextField(blank=True, null=True)
    require_https = models.BooleanField(default=True)
    require_approval = models.BooleanField(default=False)
    scan_alert_threshold = models.IntegerField(default=0)
    password_min_length = models.IntegerField(default=8)
    session_timeout_minutes = models.IntegerField(default=0)
    created_at = models.DateTimeField(null=True, blank=True)
    updated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "security_policies"


class BulkJob(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    workspace = models.ForeignKey("Workspace", on_delete=models.CASCADE,
                                  db_column="workspace_id", related_name="bulk_jobs")
    created_by = models.UUIDField(null=True, blank=True)
    filename = models.CharField(max_length=255, blank=True, null=True)
    status = models.CharField(max_length=20, default="pending")
    total_rows = models.IntegerField(default=0)
    success_count = models.IntegerField(default=0)
    error_count = models.IntegerField(default=0)
    errors = models.TextField(blank=True, null=True)
    options = models.JSONField(default=dict)
    created_at = models.DateTimeField(null=True, blank=True)
    completed_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        managed = True
        db_table = "bulk_jobs"
