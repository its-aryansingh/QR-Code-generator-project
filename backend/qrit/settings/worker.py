"""Settings for the background worker and scan ingest consumer."""

from .base import *  # noqa: F403
from .base import _parse_db_url
from .env import env

# Worker uses unpooled connection for LISTEN/NOTIFY and advisory locks
unpooled_url = env.DATABASE_UNPOOLED_URL or env.DATABASE_URL
DATABASES = {"default": _parse_db_url(unpooled_url, min_pool=1, max_pool=env.WORKER_CONCURRENCY)}
# Procrastinate worker doesn't use pooling
DATABASES["default"]["CONN_MAX_AGE"] = 0
if "OPTIONS" in DATABASES["default"] and "pool" in DATABASES["default"]["OPTIONS"]:
    del DATABASES["default"]["OPTIONS"]["pool"]
