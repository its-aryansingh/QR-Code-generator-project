"""Tests for SSRF guard in HTTP safe_client.

Ported from reference/go-v2/internal/sso/sso_test.go::TestSSRFGuard.
"""

import pytest

from apps.core.http import safe_client


def test_ssrf_guard() -> None:
    """Port of TestSSRFGuard: loopback is refused when allow_private=False."""
    # Loopback IP
    client = safe_client(allow_private=False)
    with pytest.raises(Exception) as exc_info:
        client.get("http://127.0.0.1:8000/healthz")
    assert "refusing" in str(exc_info.value)

    # Link-local cloud metadata IP
    with pytest.raises(Exception) as exc_info:
        client.get("http://169.254.169.254/latest/meta-data")
    assert "refusing" in str(exc_info.value)
