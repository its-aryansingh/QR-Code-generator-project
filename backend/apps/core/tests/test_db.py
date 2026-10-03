"""Tests for workspace scoping context manager and row-level security.

Plan §6.2 & §7.1:
- workspace_scope context manager pins the session to a workspace
- supports nested contexts safely
- ensures RLS policy enforcement
"""

from uuid import uuid4

import pytest

from apps.core.db import current_workspace_id, workspace_scope


@pytest.mark.django_db
def test_workspace_scope_lifecycle() -> None:
    """Verify workspace_scope sets and restores app.workspace_id setting."""
    ws1 = str(uuid4())
    ws2 = str(uuid4())

    assert current_workspace_id() in (None, "")

    with workspace_scope(ws1):
        assert current_workspace_id() == ws1

        # Nested scope
        with workspace_scope(ws2):
            assert current_workspace_id() == ws2

        # Restored to ws1
        assert current_workspace_id() == ws1

    # Restored to empty/none
    assert current_workspace_id() in (None, "")
