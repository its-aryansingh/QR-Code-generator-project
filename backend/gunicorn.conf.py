"""Production Gunicorn configuration for QRit backend."""
import multiprocessing
import os

bind = os.environ.get("GUNICORN_BIND", f"0.0.0.0:{os.environ.get('PORT', '8084')}")

# Worker concurrency: configurable via WEB_CONCURRENCY, default calculated or 3
default_workers = min(4, max(2, multiprocessing.cpu_count() * 2 + 1))
workers = int(os.environ.get("WEB_CONCURRENCY", default_workers))
threads = int(os.environ.get("GUNICORN_THREADS", 2))
worker_class = "gthread"

timeout = int(os.environ.get("GUNICORN_TIMEOUT", 120))
keepalive = int(os.environ.get("GUNICORN_KEEPALIVE", 5))

# Recycle workers after processing requests to prevent memory leaks
max_requests = 1000
max_requests_jitter = 50

# Log to stdout and stderr for container/platform log collectors
accesslog = "-"
errorlog = "-"
loglevel = os.environ.get("LOG_LEVEL", "info").lower()
capture_output = True
