"""Cursor-based pagination helper.

Plan §5.4 & §7.1:
- Cursors are opaque URL-safe base64 strings encoding `(created_at, id)`.
- Default limit: 50, maximum limit: 200.
- Responses follow `{data: [...], next_cursor: "..." | null}`.
- Offset pagination is never used.
"""

import base64
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

from django.db.models import Q, QuerySet
from django.utils import timezone


def encode_cursor(created_at: datetime, item_id: Any) -> str:
    """Encode created_at datetime and item id into an opaque URL-safe base64 string."""
    if timezone.is_naive(created_at):
        created_at = timezone.make_aware(created_at, UTC)
    else:
        created_at = created_at.astimezone(UTC)

    ts_iso = created_at.isoformat()
    raw = f"{ts_iso}|{item_id}".encode()
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def decode_cursor(cursor_str: str) -> tuple[datetime, str]:
    """Decode a cursor string back into (created_at, item_id)."""
    padding = "=" * ((4 - len(cursor_str) % 4) % 4)
    raw = base64.urlsafe_b64decode((cursor_str + padding).encode("ascii")).decode("utf-8")
    parts = raw.split("|", 1)
    if len(parts) != 2:
        raise ValueError("Invalid cursor format")
    dt = datetime.fromisoformat(parts[0])
    if timezone.is_naive(dt):
        dt = timezone.make_aware(dt, UTC)
    return dt, parts[1]


def paginate_queryset(
    queryset: QuerySet[Any],
    request: Any,
    default_limit: int = 50,
    max_limit: int = 200,
    ordering_field: str = "created_at",
    id_field: str = "id",
) -> tuple[list[Any], str | None]:
    """Paginate a queryset using cursor pagination.

    Fetches limit + 1 items to determine if a subsequent page exists.
    Assumes descending order by (ordering_field, id_field).
    """
    params = getattr(request, "query_params", getattr(request, "GET", {}))

    try:
        limit = int(params.get("limit", default_limit))
    except (TypeError, ValueError):
        limit = default_limit

    limit = max(1, min(limit, max_limit))

    cursor_param = params.get("cursor")
    if cursor_param:
        try:
            cursor_dt, cursor_id = decode_cursor(cursor_param)
            # Try to convert cursor_id to UUID if matching field is UUID
            try:
                parsed_id: Any = UUID(cursor_id)
            except (ValueError, AttributeError):
                parsed_id = cursor_id

            # Filter for rows strictly after cursor in descending order
            filter_q = Q(**{f"{ordering_field}__lt": cursor_dt}) | (
                Q(**{ordering_field: cursor_dt}) & Q(**{f"{id_field}__lt": parsed_id})
            )
            queryset = queryset.filter(filter_q)
        except Exception:
            pass  # Invalid cursor treated as start

    queryset = queryset.order_by(f"-{ordering_field}", f"-{id_field}")

    # Fetch limit + 1
    items = list(queryset[: limit + 1])

    next_cursor: str | None = None
    if len(items) > limit:
        items = items[:limit]
        last_item = items[-1]
        next_dt = getattr(last_item, ordering_field)
        next_id = getattr(last_item, id_field)
        next_cursor = encode_cursor(next_dt, next_id)

    return items, next_cursor
