"""Command to wait for database migrations to be applied before worker/ingest starts."""

import time
from typing import Any

from django.core.management.base import BaseCommand
from django.db import connections
from django.db.migrations.executor import MigrationExecutor


class Command(BaseCommand):
    help = "Waits for all pending database migrations to be applied before proceeding."

    def add_arguments(self, parser: Any) -> None:
        parser.add_argument(
            "--timeout",
            type=int,
            default=60,
            help="Maximum seconds to wait for migrations (default: 60)",
        )
        parser.add_argument(
            "--interval",
            type=float,
            default=2.0,
            help="Seconds to wait between checks (default: 2.0)",
        )
        parser.add_argument(
            "--database",
            default="default",
            help="Database alias to check (default: default)",
        )

    def handle(self, *args: Any, **options: Any) -> None:
        timeout = options["timeout"]
        interval = options["interval"]
        db_alias = options["database"]
        start_time = time.monotonic()
        conn = connections[db_alias]

        self.stdout.write(f"Waiting for database migrations on alias '{db_alias}'...")

        while True:
            try:
                conn.ensure_connection()
                executor = MigrationExecutor(conn)
                targets = executor.loader.graph.leaf_nodes()
                plan = executor.migration_plan(targets)
                if not plan:
                    self.stdout.write(self.style.SUCCESS("All database migrations are applied."))
                    return
                self.stdout.write(f"{len(plan)} migrations still pending...")
            except Exception as e:
                self.stdout.write(f"Database connection or migration check status: {e}")

            if time.monotonic() - start_time >= timeout:
                raise TimeoutError(
                    f"Timed out after {timeout}s waiting for database migrations on '{db_alias}'."
                )

            time.sleep(interval)
