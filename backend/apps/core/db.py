"""Database workspace scoping and session variable utilities.

Plan §6.2 & §7.1:
- workspace_scope context manager pins the transaction to a workspace
- sets PostgreSQL `app.workspace_id` session setting
- handles nested contexts safely
"""

from collections.abc import Iterator
from contextlib import contextmanager
from uuid import UUID

from django.db import connection, transaction


@contextmanager
def workspace_scope(ws_id: str | UUID | None) -> Iterator[None]:
    """Context manager that sets PostgreSQL `app.workspace_id` setting inside an atomic transaction.

    If `ws_id` is None, `app.workspace_id` is cleared / set to empty string so bypass/org-level queries work.
    Supports nesting by saving and restoring the previous workspace ID.
    """
    ws_str = str(ws_id) if ws_id is not None else ""

    with transaction.atomic():
        with connection.cursor() as cursor:
            cursor.execute("SELECT current_setting('app.workspace_id', true)")
            row = cursor.fetchone()
            prev_ws = row[0] if row and row[0] else ""

            cursor.execute("SELECT set_config('app.workspace_id', %s, true)", [ws_str])
        try:
            yield
        finally:
            with connection.cursor() as cursor:
                cursor.execute("SELECT set_config('app.workspace_id', %s, true)", [prev_ws])


def current_workspace_id() -> str | None:
    """Return the currently active workspace_id from PostgreSQL session variable, or None."""
    with connection.cursor() as cursor:
        cursor.execute("SELECT current_setting('app.workspace_id', true)")
        row = cursor.fetchone()
        if row and row[0]:
            return str(row[0])
    return None
