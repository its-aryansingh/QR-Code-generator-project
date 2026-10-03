"""Billing and subscription models.

Plan §6.2 & §7.13:
- subscriptions
- billing_events
"""

from django.db import models
from django.utils import timezone

from apps.core.ids import uuid7


class Subscription(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    workspace = models.OneToOneField(
        "workspaces.Workspace",
        on_delete=models.DB_CASCADE,
        db_column="workspace_id",
        related_name="subscription",
    )
    provider = models.TextField()
    provider_customer_id = models.TextField()
    provider_subscription_id = models.TextField(unique=True)
    plan_id = models.TextField()
    billing_interval = models.TextField()
    status = models.TextField()
    seats = models.IntegerField(default=1)
    current_period_start = models.DateTimeField(null=True, blank=True)
    current_period_end = models.DateTimeField(null=True, blank=True)
    cancel_at_period_end = models.BooleanField(default=False)
    canceled_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "subscriptions"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(provider__in=["stripe", "razorpay"]),
                name="subscriptions_provider_check",
            ),
            models.CheckConstraint(
                condition=models.Q(plan_id__in=["pro", "business", "enterprise"]),
                name="subscriptions_plan_id_check",
            ),
            models.CheckConstraint(
                condition=models.Q(billing_interval__in=["month", "year"]),
                name="subscriptions_billing_interval_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    status__in=[
                        "trialing",
                        "active",
                        "past_due",
                        "paused",
                        "canceled",
                        "incomplete",
                    ]
                ),
                name="subscriptions_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(seats__gt=0),
                name="subscriptions_seats_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.provider}:{self.provider_subscription_id}"


class BillingEvent(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    provider = models.TextField()
    provider_event_id = models.TextField()
    type = models.TextField()
    payload = models.JSONField()
    processed_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "billing_events"
        constraints = [
            models.UniqueConstraint(
                fields=["provider", "provider_event_id"],
                name="billing_events_provider_provider_event_id_key",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.provider}:{self.provider_event_id}"
