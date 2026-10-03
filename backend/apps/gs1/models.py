"""GS1 Digital Link conformant resolver models.

Plan §6.2 & §6.3:
- gs1_items
- gs1_links
"""

from django.contrib.postgres.fields import ArrayField
from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.fields import FixedCharField
from apps.core.ids import uuid7


class GS1Item(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="gs1_items"
    )
    domain = models.ForeignKey("qr.Domain", on_delete=models.PROTECT, related_name="gs1_items")
    qr_code = models.ForeignKey(
        "qr.QRCode",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="gs1_items",
    )
    gtin = FixedCharField(max_length=14)
    cpv = models.TextField(null=True, blank=True)
    lot = models.TextField(null=True, blank=True)
    serial = models.TextField(null=True, blank=True)
    title = models.TextField()
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "gs1_items"
        constraints = [
            models.CheckConstraint(
                condition=Q(gtin__regex=r"^[0-9]{14}$"),
                name="gs1_items_gtin_check",
            ),
            models.UniqueConstraint(
                fields=["domain", "gtin", "cpv", "lot", "serial"],
                nulls_distinct=False,
                name="gs1_items_domain_id_gtin_cpv_lot_serial_key",
            ),
        ]
        indexes = [
            models.Index(fields=["domain", "gtin"], name="gs1_items_lookup_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.title} (GTIN: {self.gtin})"


class GS1Link(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    item = models.ForeignKey(GS1Item, on_delete=models.CASCADE, related_name="links")
    link_type = models.TextField()
    href = models.TextField()
    title = models.TextField()
    hreflang = ArrayField(models.TextField(), default=list)
    media_type = models.TextField(null=True, blank=True)
    is_default = models.BooleanField(default=False)
    position = models.IntegerField(default=0)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "gs1_links"
        constraints = [
            models.CheckConstraint(
                condition=Q(link_type__regex=r"^gs1:[A-Za-z]+$"),
                name="gs1_links_link_type_check",
            ),
            models.UniqueConstraint(
                fields=["item"],
                name="gs1_links_one_default_uniq",
                condition=Q(is_default=True),
            ),
        ]
        indexes = [
            models.Index(fields=["item", "link_type"], name="gs1_links_item_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.link_type} -> {self.href}"
