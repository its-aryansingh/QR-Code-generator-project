"""UUID and ID utilities.

Plan §5.4 & §6.1: UUIDv7 for new database rows (time-ordered, index locality).
RFC 9562 compliant implementation for Python <3.14 with fallback to uuid.uuid7 in Python 3.14+.
"""

import os
import time
import uuid


def uuid7() -> uuid.UUID:
    """Generate a time-ordered UUIDv7 per RFC 9562.

    Layout (128 bits):
    - unix_ts_ms (48 bits): milliseconds since Unix epoch
    - ver (4 bits): 0b0111 (version 7)
    - rand_a (12 bits): pseudo-random bits
    - var (2 bits): 0b10 (RFC 4122 / 9562 variant)
    - rand_b (62 bits): pseudo-random bits
    """
    uuid7_fn = getattr(uuid, "uuid7", None)
    if uuid7_fn is not None:
        val: uuid.UUID = uuid7_fn()
        return val

    # 48-bit timestamp in milliseconds
    ms = time.time_ns() // 1_000_000

    # 10 random bytes for rand_a (12 bits) and rand_b (62 bits)
    rand_bytes = os.urandom(10)
    rand_a = int.from_bytes(rand_bytes[:2], "big") & 0x0FFF
    rand_b = int.from_bytes(rand_bytes[2:], "big") & 0x3FFFFFFFFFFFFFFF

    int_val = (ms << 80) | (0x7 << 76) | (rand_a << 64) | (0x2 << 62) | rand_b
    return uuid.UUID(int=int_val)
