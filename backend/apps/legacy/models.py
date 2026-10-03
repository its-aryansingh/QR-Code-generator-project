"""Legacy v1-to-v3 migration and bookkeeping models.

Plan §6.4 & V1_TO_V3_MIGRATION.md:
- legacy_id_map
- legacy_import_runs
- legacy_email_log
"""

from django.db import models
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class LegacyIdMap(models.Model):
    pk = models.CompositePrimaryKey("entity", "v1_id")
    entity = models.TextField()
    v1_id = models.UUIDField()
    v3_id = models.TextField()
    imported_at = models.DateTimeField(default=tz_now)
    checksum = models.TextField(null=True, blank=True)

    class Meta:
        db_table = "legacy_id_map"

    def __str__(self) -> str:
        return f"{self.entity}:{self.v1_id} -> {self.v3_id}"


class LegacyImportRun(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    started_at = models.DateTimeField(default=tz_now)
    finished_at = models.DateTimeField(null=True, blank=True)
    status = models.TextField(default="running")
    stats = models.JSONField(default=dict)
    error = models.TextField(null=True, blank=True)

    class Meta:
        db_table = "legacy_import_runs"

    def __str__(self) -> str:
        return f"Import run {self.id} ({self.status})"


class LegacyEmailLog(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    user_id = models.UUIDField()
    email = CIText()
    email_type = models.TextField()
    sent_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "legacy_email_log"

    def __str__(self) -> str:
        return f"Email {self.email_type} to {self.email}"
