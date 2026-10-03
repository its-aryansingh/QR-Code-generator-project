"""Sliding-window rate limiter using Redis sorted sets (with memory fallback).

Plan §7.1:
- Redis sliding-window/GCRA limiter: `allow(key, limit, window) -> (ok, remaining, retry_after)`
- fail-closed for auth endpoints, fail-open with logging for API keys
- in-memory fallback for testing / offline environments
"""

import math
import os
import threading
import time
from collections.abc import Sequence
from datetime import timedelta
from typing import Any

import redis
import structlog
from django.conf import settings

logger = structlog.get_logger(__name__)

_REDIS_CLIENT: redis.Redis | None = None
_REDIS_CLIENT_LOCK = threading.Lock()


def get_redis_client() -> redis.Redis | None:
    """Return a Redis client instance from settings.REDIS_URL, or None if unavailable."""
    global _REDIS_CLIENT
    if _REDIS_CLIENT is None:
        with _REDIS_CLIENT_LOCK:
            if _REDIS_CLIENT is None:
                url = getattr(settings, "REDIS_URL", "")
                if url:
                    try:
                        client: redis.Redis = redis.from_url(
                            url,
                            socket_timeout=1.0,
                            socket_connect_timeout=1.0,
                        )
                        client.ping()
                        _REDIS_CLIENT = client
                    except Exception as err:
                        logger.warning("redis_connection_failed", error=str(err))
                        return None
    return _REDIS_CLIENT


class InMemoryRateLimiter:
    """In-memory sliding window limiter for tests and local fallback."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._buckets: dict[str, list[float]] = {}

    def allow(self, key: str, limit: int, window_seconds: float) -> tuple[bool, int, int]:
        now = time.time()
        cutoff = now - window_seconds

        with self._lock:
            timestamps = self._buckets.get(key, [])
            valid = [t for t in timestamps if t > cutoff]

            if len(valid) >= limit:
                oldest = valid[0]
                retry_after = max(1, math.ceil(oldest + window_seconds - now))
                self._buckets[key] = valid
                return False, 0, retry_after

            valid.append(now)
            self._buckets[key] = valid
            remaining = max(0, limit - len(valid))
            return True, remaining, 0

    def clear(self) -> None:
        with self._lock:
            self._buckets.clear()


_MEMORY_LIMITER = InMemoryRateLimiter()


def allow(
    key: str,
    limit: int,
    window: int | float | timedelta,
    fail_closed: bool = False,
) -> tuple[bool, int, int]:
    """Check if the given key is within the rate limit.

    Args:
        key: The rate limit bucket identifier (e.g. `rl:login:ip:192.0.2.1`).
        limit: Maximum number of allowed calls within the window.
        window: Duration of the sliding window in seconds or timedelta.
        fail_closed: If True and Redis fails, reject the request (auth).
                     If False and Redis fails, allow the request (API keys).

    Returns:
        tuple (allowed: bool, remaining: int, retry_after_seconds: int)
    """
    if isinstance(window, timedelta):
        window_seconds = window.total_seconds()
    else:
        window_seconds = float(window)

    client = get_redis_client()
    if client is None:
        if getattr(settings, "APP_ENV", "local") in ("local", "test"):
            return _MEMORY_LIMITER.allow(key, limit, window_seconds)
        if fail_closed:
            return False, 0, int(window_seconds)
        logger.warning("ratelimit_fail_open_redis_unavailable", key=key)
        return True, limit, 0

    redis_key = f"velocity:{key}"
    now = time.time()
    now_ms = int(now * 1000)
    cutoff_ms = int((now - window_seconds) * 1000)

    try:
        pipe = client.pipeline()
        pipe.zremrangebyscore(redis_key, "-inf", cutoff_ms)
        pipe.zcard(redis_key)
        pipe.zrange(redis_key, 0, 0, withscores=True)
        pipe.expire(redis_key, int(window_seconds * 2) + 1)
        results: Sequence[Any] = pipe.execute()

        count = int(results[1])
        oldest_list = results[2]

        if count >= limit:
            retry_after = int(window_seconds)
            if oldest_list:
                _, oldest_score = oldest_list[0]
                retry_after = max(
                    1, math.ceil((float(oldest_score) + window_seconds * 1000 - now_ms) / 1000)
                )
            return False, 0, retry_after

        # Add current scan / request
        member = f"{now_ms}-{os.urandom(4).hex()}"
        client.zadd(redis_key, {member: now_ms})
        remaining = max(0, limit - (count + 1))
        return True, remaining, 0

    except Exception as err:
        logger.warning("ratelimit_error", error=str(err), key=key)
        if fail_closed:
            return False, 0, int(window_seconds)
        return True, limit, 0
