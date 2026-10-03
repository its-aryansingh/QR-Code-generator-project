"""Feature flags evaluation and management.

Plan §7.1:
- feature_flags(key, org_id NULL=global, enabled)
- org value overrides global default
- cached 60s
- `core.flags.enabled(key, org_id)`
"""

import threading
import time
from uuid import UUID

from django.db import connection

_CACHE_TTL_SECONDS = 60.0
_CACHE_LOCK = threading.RLock()
_CACHE: dict[str, tuple[float, dict[str, bool]]] = {}


def clear_cache() -> None:
    """Clear in-memory feature flags cache."""
    with _CACHE_LOCK:
        _CACHE.clear()


def all_flags(org_id: str | UUID | None = None) -> dict[str, bool]:
    """Return all effective flag values for the organisation (or globally if org_id is None)."""
    org_str = str(org_id) if org_id is not None else ""
    now = time.monotonic()

    with _CACHE_LOCK:
        if org_str in _CACHE:
            ts, cached_map = _CACHE[org_str]
            if now - ts < _CACHE_TTL_SECONDS:
                return cached_map

    flags_map: dict[str, bool] = {}
    with connection.cursor() as cursor:
        if org_str:
            cursor.execute(
                """
                SELECT DISTINCT ON (key) key, enabled
                FROM feature_flags
                WHERE org_id IS NULL OR org_id = %s
                ORDER BY key, org_id NULLS LAST
                """,
                [org_str],
            )
        else:
            cursor.execute(
                """
                SELECT key, enabled
                FROM feature_flags
                WHERE org_id IS NULL
                ORDER BY key
                """
            )

        for row in cursor.fetchall():
            flags_map[row[0]] = bool(row[1])

    with _CACHE_LOCK:
        _CACHE[org_str] = (now, flags_map)

    return flags_map


def enabled(key: str, org_id: str | UUID | None = None) -> bool:
    """Check if a feature flag is enabled for an organisation (or globally)."""
    m = all_flags(org_id)
    return m.get(key, False)


def set_flag(
    key: str,
    org_id: str | UUID | None,
    flag_enabled: bool,
    updated_by_id: str | UUID | None = None,
) -> None:
    """Upsert a feature flag (org_id None = global default)."""
    org_str = str(org_id) if org_id is not None else None
    user_str = str(updated_by_id) if updated_by_id is not None else None

    with connection.cursor() as cursor:
        cursor.execute(
            """
            INSERT INTO feature_flags (key, org_id, enabled, updated_by, updated_at)
            VALUES (%s, %s, %s, %s, now())
            ON CONFLICT (key, org_id) DO UPDATE
            SET enabled = EXCLUDED.enabled,
                updated_by = EXCLUDED.updated_by,
                updated_at = now()
            """,
            [key, org_str, flag_enabled, user_str],
        )

    clear_cache()
