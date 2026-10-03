"""Tests for cursor-based pagination helper.

Plan §5.4 & §7.1:
- Cursors are opaque URL-safe base64 strings encoding (created_at, id).
- Response shape {data, next_cursor}.
"""

from datetime import UTC, datetime
from uuid import uuid4

from apps.core.pagination import decode_cursor, encode_cursor


def test_cursor_encoding_and_decoding() -> None:
    dt = datetime(2026, 10, 3, 14, 30, 0, tzinfo=UTC)
    item_id = str(uuid4())

    cursor = encode_cursor(dt, item_id)
    assert isinstance(cursor, str)
    assert len(cursor) > 0

    decoded_dt, decoded_id = decode_cursor(cursor)
    assert decoded_dt == dt
    assert decoded_id == item_id
