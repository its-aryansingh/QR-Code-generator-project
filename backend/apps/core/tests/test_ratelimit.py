"""Tests for sliding-window rate limiter.

Plan §7.1:
- Sliding-window enforcement
- Retry-after computation
"""

import time

from apps.core.ratelimit import allow


def test_sliding_window_limiter() -> None:
    key = f"test-rl-{time.time()}"
    limit = 3
    window = 1.0  # 1 second

    # First 3 allowed
    ok, rem, _ = allow(key, limit, window)
    assert ok is True
    assert rem == 2

    ok, rem, _ = allow(key, limit, window)
    assert ok is True
    assert rem == 1

    ok, rem, _ = allow(key, limit, window)
    assert ok is True
    assert rem == 0

    # 4th rejected
    ok, rem, retry_after = allow(key, limit, window)
    assert ok is False
    assert rem == 0
    assert retry_after >= 1
