"""Tests for password strength validation."""

import pytest

from apps.accounts.passwords import validate_password_strength
from apps.core.errors import ApiError


def test_password_too_short() -> None:
    with pytest.raises(ApiError) as exc_info:
        validate_password_strength("short")
    assert exc_info.value.code == "weak_password"
    assert "at least 10 characters" in exc_info.value.detail


def test_password_common_breached() -> None:
    with pytest.raises(ApiError) as exc_info:
        validate_password_strength("password123")
    assert exc_info.value.code == "weak_password"
    assert "too common" in exc_info.value.detail


def test_password_valid() -> None:
    # Strong passphrase
    validate_password_strength("correct horse battery staple")
