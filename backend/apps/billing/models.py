"""Billing, subscription, contract, and GST invoice models.

Plan §6.2, §6.3, §6.4 & §7.13:
- subscriptions
- billing_events
- contracts
- invoice_sequences
- invoices
"""

from decimal import Decimal

from django.db import models
from django.db.models import F, Q
from django.utils.timezone import now as tz_now

from apps.core.fields import FixedCharField
from apps.core.ids import uuid7


class Subscription(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey(
        "orgs.Organization",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        related_name="subscriptions",
    )
    workspace = models.ForeignKey(
        "workspaces.Workspace",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        db_column="workspace_id",
        related_name="subscriptions",
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
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)

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
            # Plan §6.4 partial unique index:
            models.UniqueConstraint(
                fields=["org"],
                condition=models.Q(status__in=["trialing", "active", "past_due", "paused"]),
                name="subscriptions_org_uniq",
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
    created_at = models.DateTimeField(default=tz_now)

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


class Contract(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey("orgs.Organization", on_delete=models.CASCADE, related_name="contracts")
    name = models.TextField()
    starts_on = models.DateField()
    ends_on = models.DateField()
    billing_interval = models.TextField(
        choices=[
            ("month", "Month"),
            ("quarter", "Quarter"),
            ("year", "Year"),
        ]
    )
    currency = FixedCharField(
        max_length=3,
        choices=[
            ("INR", "INR"),
            ("USD", "USD"),
            ("EUR", "EUR"),
            ("GBP", "GBP"),
        ],
    )
    amount_minor = models.BigIntegerField()
    seats = models.IntegerField()
    limits_override = models.JSONField(default=dict)
    sla_uptime = models.DecimalField(max_digits=5, decimal_places=3, default=Decimal("99.950"))
    po_number = models.TextField(null=True, blank=True)
    payment_terms_days = models.IntegerField(default=30)
    status = models.TextField(
        default="draft",
        choices=[
            ("draft", "Draft"),
            ("active", "Active"),
            ("expired", "Expired"),
            ("terminated", "Terminated"),
        ],
    )
    created_by = models.ForeignKey(
        "accounts.User",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        db_column="created_by",
        related_name="created_contracts",
    )
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "contracts"
        constraints = [
            models.CheckConstraint(
                condition=Q(ends_on__gt=F("starts_on")),
                name="contracts_ends_on_check",
            ),
            models.CheckConstraint(
                condition=models.Q(billing_interval__in=["month", "quarter", "year"]),
                name="contracts_billing_interval_check",
            ),
            models.CheckConstraint(
                condition=models.Q(currency__in=["INR", "USD", "EUR", "GBP"]),
                name="contracts_currency_check",
            ),
            models.CheckConstraint(
                condition=Q(amount_minor__gte=0),
                name="contracts_amount_minor_check",
            ),
            models.CheckConstraint(
                condition=Q(seats__gt=0),
                name="contracts_seats_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["draft", "active", "expired", "terminated"]),
                name="contracts_status_check",
            ),
            models.UniqueConstraint(
                fields=["org"],
                condition=Q(status="active"),
                name="contracts_one_active_uniq",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.name} ({self.org_id})"


class InvoiceSequence(models.Model):
    fy = models.TextField(primary_key=True)
    last_seq = models.IntegerField(default=0)

    class Meta:
        db_table = "invoice_sequences"
        constraints = [
            models.CheckConstraint(
                condition=Q(fy__regex=r"^[0-9]{4}-[0-9]{2}$"),
                name="invoice_sequences_fy_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.fy}: {self.last_seq}"


class Invoice(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    org = models.ForeignKey("orgs.Organization", on_delete=models.PROTECT, related_name="invoices")
    contract = models.ForeignKey(
        Contract,
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="invoices",
    )
    number = models.TextField(unique=True)
    issue_date = models.DateField()
    due_date = models.DateField()
    currency = FixedCharField(max_length=3)
    tax_mode = models.TextField(
        choices=[
            ("cgst_sgst", "CGST + SGST"),
            ("igst", "IGST"),
            ("export_lut", "Export under LUT"),
            ("reverse_charge", "Reverse Charge"),
            ("none", "None"),
        ]
    )
    place_of_supply = models.TextField()
    seller = models.JSONField()
    buyer = models.JSONField()
    lines = models.JSONField()
    subtotal_minor = models.BigIntegerField()
    cgst_minor = models.BigIntegerField(default=0)
    sgst_minor = models.BigIntegerField(default=0)
    igst_minor = models.BigIntegerField(default=0)
    total_minor = models.BigIntegerField()
    endorsement = models.TextField(null=True, blank=True)
    status = models.TextField(
        default="issued",
        choices=[
            ("issued", "Issued"),
            ("paid", "Paid"),
            ("void", "Void"),
        ],
    )
    pdf_file = models.ForeignKey(
        "qr.File",
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name="+",
    )
    paid_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    # 00007 additions:
    period_start = models.DateField(null=True, blank=True)
    period_end = models.DateField(null=True, blank=True)
    payment_link_id = models.TextField(null=True, blank=True)
    payment_link_url = models.TextField(null=True, blank=True)
    reminders_sent = models.IntegerField(default=0)
    last_reminder_at = models.DateTimeField(null=True, blank=True)
    void_reason = models.TextField(null=True, blank=True)
    paid_reference = models.TextField(null=True, blank=True)

    class Meta:
        db_table = "invoices"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(
                    tax_mode__in=["cgst_sgst", "igst", "export_lut", "reverse_charge", "none"]
                ),
                name="invoices_tax_mode_check",
            ),
            models.CheckConstraint(
                condition=models.Q(status__in=["issued", "paid", "void"]),
                name="invoices_status_check",
            ),
            models.CheckConstraint(
                condition=models.Q(
                    total_minor=F("subtotal_minor")
                    + F("cgst_minor")
                    + F("sgst_minor")
                    + F("igst_minor")
                ),
                name="invoice_totals",
            ),
            models.CheckConstraint(
                condition=~Q(tax_mode="export_lut")
                | (
                    Q(cgst_minor=0)
                    & Q(sgst_minor=0)
                    & Q(igst_minor=0)
                    & Q(endorsement__isnull=False)
                ),
                name="invoice_export_zero",
            ),
            models.UniqueConstraint(
                fields=["contract", "period_start"],
                condition=Q(contract__isnull=False, period_start__isnull=False) & ~Q(status="void"),
                name="invoices_contract_period_uniq",
            ),
        ]
        indexes = [
            models.Index(fields=["org", "-issue_date"], name="invoices_org_idx"),
            models.Index(
                fields=["due_date"],
                name="invoices_unpaid_idx",
                condition=Q(status="issued"),
            ),
        ]

    def __str__(self) -> str:
        return f"Invoice {self.number} ({self.status})"
