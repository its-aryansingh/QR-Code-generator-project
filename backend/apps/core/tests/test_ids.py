"""Tests for apps.core.ids (UUIDv7 generation)."""

import time
import uuid

from apps.core.ids import uuid7


def test_uuid7_version_and_variant() -> None:
    u = uuid7()
    assert isinstance(u, uuid.UUID)
    assert u.version == 7
    # Variant should be RFC 4122 / RFC 9562 (specified as integer 2 or variant attribute)
    assert (u.int >> 62) & 0b11 == 0b10


def test_uuid7_time_ordering() -> None:
    u1 = uuid7()
    time.sleep(0.002)
    u2 = uuid7()
    # Newer UUIDv7 should be strictly greater than older UUIDv7
    assert u2.int > u1.int
    assert str(u2) > str(u1)


def test_uuid7_uniqueness() -> None:
    generated = {uuid7() for _ in range(1000)}
    assert len(generated) == 1000
