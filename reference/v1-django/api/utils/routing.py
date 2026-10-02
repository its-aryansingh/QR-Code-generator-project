"""Smart-routing evaluation.

One printed QR code, many destinations, chosen at scan time. This is the
capability every serious competitor leads with -- geo-fencing, device
targeting, scheduled windows and percentage split tests.

Rules are evaluated highest-priority first; the first match wins. Weighted
rules are evaluated as a stable split so a given visitor keeps landing in the
same bucket, which is what makes an A/B test measurable.
"""

import hashlib
from datetime import datetime, time

from django.utils import timezone


def _values(rule):
    return [v.strip().lower() for v in (rule.value or "").split(",") if v.strip()]


def _matches_membership(rule, candidate):
    """Handle `in` / `not_in` for a single candidate value."""
    if candidate is None:
        return False
    options = _values(rule)
    hit = str(candidate).strip().lower() in options
    return not hit if rule.operator == "not_in" else hit


def _parse_clock(raw):
    try:
        hour, _, minute = raw.partition(":")
        return time(int(hour), int(minute or 0))
    except (ValueError, AttributeError):
        return None


def _matches_time_of_day(rule, now):
    """value is `HH:MM-HH:MM` in UTC; windows may wrap past midnight."""
    start_raw, _, end_raw = (rule.value or "").partition("-")
    start, end = _parse_clock(start_raw), _parse_clock(end_raw)
    if not start or not end:
        return False
    current = now.time()
    inside = start <= current <= end if start <= end else (current >= start or current <= end)
    return not inside if rule.operator == "not_in" else inside


def _matches_date_range(rule, now):
    """value is `YYYY-MM-DD..YYYY-MM-DD`; either side may be blank."""
    start_raw, _, end_raw = (rule.value or "").partition("..")
    today = now.date()
    try:
        if start_raw.strip() and today < datetime.strptime(start_raw.strip(), "%Y-%m-%d").date():
            return False
        if end_raw.strip() and today > datetime.strptime(end_raw.strip(), "%Y-%m-%d").date():
            return False
    except ValueError:
        return False
    return True


def _matches_weight(rule, context):
    """Deterministic split on a per-visitor bucket.

    Hashing the visitor rather than calling random() means the same phone
    keeps seeing the same variant, so the test measures the variant instead
    of measuring coin flips.
    """
    try:
        threshold = int(rule.value)
    except (TypeError, ValueError):
        return False
    seed = f"{rule.qr_record_id}:{context.get('ip') or ''}:{context.get('user_agent') or ''}"
    bucket = int(hashlib.sha256(seed.encode()).hexdigest()[:8], 16) % 100
    return bucket < threshold


def _matches_scan_count(rule, context):
    """value is a threshold; operator `gt` (default) or `lt`."""
    try:
        threshold = int(rule.value)
    except (TypeError, ValueError):
        return False
    count = int(context.get("scan_count") or 0)
    return count < threshold if rule.operator == "lt" else count >= threshold


def matches(rule, context):
    condition = rule.condition
    now = context.get("now") or timezone.now()

    if condition == "country":
        return _matches_membership(rule, context.get("country_code"))
    if condition == "device":
        return _matches_membership(rule, context.get("device_type"))
    if condition == "os":
        return _matches_membership(rule, context.get("os"))
    if condition == "language":
        language = (context.get("language") or "").split("-")[0]
        return _matches_membership(rule, language)
    if condition == "time_of_day":
        return _matches_time_of_day(rule, now)
    if condition == "date_range":
        return _matches_date_range(rule, now)
    if condition == "weight":
        return _matches_weight(rule, context)
    if condition == "scan_count":
        return _matches_scan_count(rule, context)
    return False


def resolve_destination(qr, rules, context):
    """Return (destination_url, matched_rule_or_None)."""
    default = qr.redirect_url or qr.content
    for rule in rules:
        if not rule.is_active:
            continue
        try:
            if matches(rule, context):
                return rule.destination_url, rule
        except Exception:
            # A malformed rule must never take a printed code offline.
            continue
    return default, None


def describe_condition(rule):
    """Plain-English summary for the dashboard rule list."""
    values = rule.value or ""
    negated = rule.operator == "not_in"
    if rule.condition == "country":
        return f"Country is {'not ' if negated else ''}{values}"
    if rule.condition == "device":
        return f"Device is {'not ' if negated else ''}{values}"
    if rule.condition == "os":
        return f"OS is {'not ' if negated else ''}{values}"
    if rule.condition == "language":
        return f"Language is {'not ' if negated else ''}{values}"
    if rule.condition == "time_of_day":
        return f"Between {values} UTC"
    if rule.condition == "date_range":
        return f"Between {values.replace('..', ' and ')}"
    if rule.condition == "weight":
        return f"{values}% of visitors"
    if rule.condition == "scan_count":
        return f"After {values} scans" if rule.operator != "lt" else f"Before {values} scans"
    return rule.condition
