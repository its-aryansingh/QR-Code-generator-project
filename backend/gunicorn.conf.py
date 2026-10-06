"""Production Gunicorn configuration for QRit backend."""
import multiprocessing
import os
import socket


def _default_bind() -> str:
    """Listen on every interface, IPv6 included when the host has it.

    Railway injects PORT and reaches services over IPv6 on its private network,
    so "[::]" (dual-stack: IPv4 is accepted too) is required there. Hosts and
    containers with IPv6 disabled cannot even create an IPv6 socket, so fall
    back to IPv4 instead of crashing at boot.
    """
    port = os.environ.get("PORT", "8084")
    try:
        socket.socket(socket.AF_INET6, socket.SOCK_STREAM).close()
    except OSError:
        return f"0.0.0.0:{port}"
    return f"[::]:{port}"


bind = os.environ.get("GUNICORN_BIND") or _default_bind()

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
