"""Retargeting pixels and consent analytics models.

Plan §6.2 & §6.3:
- pixels
- qr_code_pixels
- pixel_consent_daily
"""

from django.db import models
from django.db.models import Q
from django.utils.timezone import now as tz_now

from apps.core.ids import uuid7


class Pixel(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.ForeignKey(
        "workspaces.Workspace", on_delete=models.CASCADE, related_name="pixels"
    )
    provider = models.TextField(
        choices=[
            ("meta", "Meta"),
            ("google", "Google"),
            ("linkedin", "LinkedIn"),
            ("tiktok", "TikTok"),
        ]
    )
    external_id = models.TextField()
    name = models.TextField()
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "pixels"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(provider__in=["meta", "google", "linkedin", "tiktok"]),
                name="pixels_provider_check",
            ),
            models.CheckConstraint(
                condition=Q(external_id__regex=r"^[A-Za-z0-9_-]{3,40}$"),
                name="pixels_external_id_check",
            ),
            models.UniqueConstraint(
                fields=["workspace", "provider", "external_id"],
                name="pixels_workspace_id_provider_external_id_key",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.provider}:{self.external_id})"


class QRCodePixel(models.Model):
    pk = models.CompositePrimaryKey("qr_code", "pixel")
    qr_code = models.ForeignKey("qr.QRCode", on_delete=models.CASCADE, related_name="pixel_links")
    pixel = models.ForeignKey(Pixel, on_delete=models.CASCADE, related_name="code_links")

    class Meta:
        db_table = "qr_code_pixels"

    def __str__(self) -> str:
        return f"QR {self.qr_code_id} <-> Pixel {self.pixel_id}"


class PixelConsentDaily(models.Model):
    pk = models.CompositePrimaryKey("qr_code_id", "day")
    qr_code_id = models.UUIDField()
    day = models.DateField()
    shown = models.IntegerField(default=0)
    accepted = models.IntegerField(default=0)
    declined = models.IntegerField(default=0)
    auto_fired = models.IntegerField(default=0)

    class Meta:
        db_table = "pixel_consent_daily"

    def __str__(self) -> str:
        return f"Consent for {self.qr_code_id} on {self.day}"
