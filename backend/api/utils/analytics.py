"""Shared analytics aggregation.

Everything here goes through the ORM rather than raw SQL. The previous
dashboard query used Postgres-only casts (`%s::uuid`, `DATE(x)::text`) and
returned a 500 on SQLite, so the dashboard was unusable in development.
"""

from datetime import timedelta

from django.db.models import Count
from django.db.models.functions import TruncDate, TruncHour


def timeseries(scan_qs, since, days, key="date"):
    """Daily scan counts with zero-filled gaps.

    A sparse series makes a chart lie -- three scans on three separate days
    render as a flat line unless the empty days are present.
    """
    rows = (
        scan_qs.annotate(bucket=TruncDate("scanned_at"))
        .values("bucket")
        .annotate(count=Count("id"))
        .order_by("bucket")
    )
    counts = {}
    for row in rows:
        bucket = row["bucket"]
        if bucket is None:
            continue
        counts[bucket.isoformat() if hasattr(bucket, "isoformat") else str(bucket)] = row["count"]

    start = since.date() if hasattr(since, "date") else since
    return [
        {key: (start + timedelta(days=offset)).isoformat(),
         "count": counts.get((start + timedelta(days=offset)).isoformat(), 0)}
        for offset in range(days + 1)
    ]


def hourly_distribution(scan_qs):
    """Scans bucketed by hour of day, zero-filled across all 24 hours."""
    rows = (
        scan_qs.annotate(bucket=TruncHour("scanned_at"))
        .values("bucket")
        .annotate(count=Count("id"))
    )
    hours = {h: 0 for h in range(24)}
    for row in rows:
        if row["bucket"] is not None:
            hours[row["bucket"].hour] += row["count"]
    return [{"hour": h, "count": hours[h]} for h in range(24)]


def breakdown(scan_qs, field, limit=10, label=None):
    """Top-N grouping on a single column, with nulls normalised to Unknown."""
    rows = (
        scan_qs.values(field)
        .annotate(count=Count("id"))
        .order_by("-count")[:limit]
    )
    name = label or field
    return [{name: row[field] or "Unknown", "count": row["count"]} for row in rows]


def percentage(part, whole):
    return round(part / whole * 100, 1) if whole else 0.0


def delta(current, previous):
    """Percentage change, guarding the divide-by-zero the UI would show as NaN."""
    if not previous:
        return 100.0 if current else 0.0
    return round((current - previous) / previous * 100, 1)
