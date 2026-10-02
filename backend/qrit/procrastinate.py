"""Procrastinate app configuration and queue definitions."""

from procrastinate.contrib.django import app

QUEUE_DEFAULT = "default"
QUEUE_CRITICAL = "critical"
QUEUE_BULK = "bulk"
QUEUE_INTEGRATIONS = "integrations"
QUEUE_REPORTS = "reports"

QUEUES = [
    QUEUE_DEFAULT,
    QUEUE_CRITICAL,
    QUEUE_BULK,
    QUEUE_INTEGRATIONS,
    QUEUE_REPORTS,
]

__all__ = [
    "app",
    "QUEUE_DEFAULT",
    "QUEUE_CRITICAL",
    "QUEUE_BULK",
    "QUEUE_INTEGRATIONS",
    "QUEUE_REPORTS",
    "QUEUES",
]
