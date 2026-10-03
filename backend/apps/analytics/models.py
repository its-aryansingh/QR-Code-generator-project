"""Analytics models.

Plan §6.1 rule 3 & §7.10:
- scan_events (partitioned by range on occurred_at; managed = False, created via RunSQL)
- scan_visitors_daily
- scan_stats_15m
- scan_stats_daily_dim
"""

from django.db import models

from apps.core.fields import FixedCharField
from apps.core.ids import uuid7


class ScanEvent(models.Model):
    """Raw scan events partitioned monthly by occurred_at.

    Created via RunSQL in migrations (managed = False).
    """

    pk = models.CompositePrimaryKey("event_id", "occurred_at")
    event_id = models.UUIDField(default=uuid7, editable=False)
    occurred_at = models.DateTimeField()
    workspace_id = models.UUIDField()
    qr_code_id = models.UUIDField()
    version_id = models.UUIDField(null=True, blank=True)
    campaign_id = models.UUIDField(null=True, blank=True)
    domain_id = models.UUIDField()
    rule_id = models.TextField(null=True, blank=True)
    outcome = models.TextField()
    method = models.TextField(default="GET")
    is_bot = models.BooleanField(default=False)
    bot_reason = models.TextField(null=True, blank=True)
    is_duplicate = models.BooleanField(default=False)
    is_unique = models.BooleanField(default=False)
    visitor_hash = models.BinaryField()
    device_type = models.TextField(null=True, blank=True)
    os = models.TextField(null=True, blank=True)
    os_version = models.TextField(null=True, blank=True)
    browser = models.TextField(null=True, blank=True)
    browser_version = models.TextField(null=True, blank=True)
    country = FixedCharField(max_length=2, null=True, blank=True)
    region = models.TextField(null=True, blank=True)
    city = models.TextField(null=True, blank=True)
    language = models.TextField(null=True, blank=True)
    referrer_host = models.TextField(null=True, blank=True)
    utm_source = models.TextField(null=True, blank=True)
    utm_medium = models.TextField(null=True, blank=True)
    utm_campaign = models.TextField(null=True, blank=True)

    class Meta:
        managed = False
        db_table = "scan_events"


class ScanVisitorDaily(models.Model):
    pk = models.CompositePrimaryKey("qr_code_id", "day", "visitor_hash")
    qr_code_id = models.UUIDField()
    day = models.DateField()
    visitor_hash = models.BinaryField()
    first_seen_at = models.DateTimeField()

    class Meta:
        db_table = "scan_visitors_daily"

    def __str__(self) -> str:
        return f"{self.qr_code_id}:{self.day}"


class ScanStat15m(models.Model):
    pk = models.CompositePrimaryKey("qr_code_id", "bucket_start")
    qr_code_id = models.UUIDField()
    bucket_start = models.DateTimeField()
    workspace_id = models.UUIDField()
    campaign_id = models.UUIDField(null=True, blank=True)
    scans = models.IntegerField(default=0)
    unique_scans = models.IntegerField(default=0)
    bot_hits = models.IntegerField(default=0)
    blocked_hits = models.IntegerField(default=0)

    class Meta:
        db_table = "scan_stats_15m"
        indexes = [
            models.Index(fields=["workspace_id", "bucket_start"], name="scan_stats_15m_ws_idx"),
            models.Index(
                fields=["campaign_id", "bucket_start"],
                condition=models.Q(campaign_id__isnull=False),
                name="scan_stats_15m_campaign_idx",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.qr_code_id}:{self.bucket_start}"


class ScanStatDailyDim(models.Model):
    pk = models.CompositePrimaryKey("qr_code_id", "day", "dimension", "key")
    qr_code_id = models.UUIDField()
    day = models.DateField()
    dimension = models.TextField()
    key = models.TextField()
    workspace_id = models.UUIDField()
    campaign_id = models.UUIDField(null=True, blank=True)
    scans = models.IntegerField(default=0)
    unique_scans = models.IntegerField(default=0)

    class Meta:
        db_table = "scan_stats_daily_dim"
        indexes = [
            models.Index(fields=["workspace_id", "dimension", "day"], name="scan_stats_dim_ws_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    dimension__in=[
                        "country",
                        "region",
                        "city",
                        "device",
                        "os",
                        "browser",
                        "language",
                        "referrer",
                        "rule",
                        "version",
                        "utm_source",
                    ]
                ),
                name="scan_stats_daily_dim_dimension_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.qr_code_id}:{self.day}:{self.dimension}:{self.key}"
