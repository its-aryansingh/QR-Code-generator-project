"""Gunicorn configuration for QRit API service."""

import os

bind = f"[::]:{os.environ.get('PORT', '8080')}"
workers = int(os.environ.get("WEB_CONCURRENCY", "2"))
worker_class = "uvicorn_worker.UvicornWorker"
graceful_timeout = 30
timeout = 60
keepalive = 5

# Leave forwarded_allow_ips at default localhost so uvicorn doesn't rewrite
# client addresses from spoofable X-Forwarded-For (client IP comes from core.net)
forwarded_allow_ips = "127.0.0.1"

# Access logs disabled: structlog logs application requests
accesslog = None
