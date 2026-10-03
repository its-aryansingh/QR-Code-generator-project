"""Core models: idempotency keys, feature flags, and org data keys.

Plan §6.2 & §7.1:
- idempotency_keys
- feature_flags
- org_data_keys
"""

from django.db import models
from django.utils.timezone import now as tz_now


class IdempotencyKey(models.Model):
    pk = models.CompositePrimaryKey("workspace_id", "key")
    workspace_id = models.UUIDField()
    key = models.TextField()
    method = models.TextField()
    path = models.TextField()
    request_hash = models.BinaryField()
    status_code = models.IntegerField(null=True, blank=True)
    response_body = models.JSONField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    expires_at = models.DateTimeField()

    class Meta:
        db_table = "idempotency_keys"
        indexes = [
            models.Index(fields=["expires_at"], name="idempotency_expiry_idx"),
        ]
        constraints = [
            models.CheckConstraint(
                condition=models.Q(key__regex=r"^.{8,128}$"),
                name="idempotency_keys_key_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.workspace_id}:{self.key}"


class FeatureFlag(models.Model):
    key = models.TextField()
    org = models.ForeignKey(
        "orgs.Organization",
        on_delete=models.CASCADE,
        null=True,
        blank=True,
        related_name="feature_flags",
    )
    enabled = models.BooleanField()
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
        db_table = "feature_flags"
        constraints = [
            models.UniqueConstraint(
                fields=["key", "org"],
                nulls_distinct=False,
                name="feature_flags_key_org_id_key",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.key}={self.enabled} (org: {self.org_id})"


class OrgDataKey(models.Model):
    pk = models.CompositePrimaryKey("org_id", "key_id")
    org = models.ForeignKey("orgs.Organization", on_delete=models.CASCADE, related_name="data_keys")
    key_id = models.IntegerField()
    dek_ct = models.BinaryField()
    created_at = models.DateTimeField(default=tz_now)
    retired_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "org_data_keys"

    def __str__(self) -> str:
        return f"OrgDataKey key_{self.key_id} for org {self.org_id}"
