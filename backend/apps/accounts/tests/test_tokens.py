"""Tests for Ed25519 access tokens and opaque token hashing."""

from uuid import uuid4

import pytest

from apps.accounts.tokens import (
    TokenManager,
    generate_random_token,
    hash_token,
)
from apps.core.errors import ApiError


def test_hash_token_deterministic() -> None:
    h1 = hash_token("test-secret-token")
    h2 = hash_token("test-secret-token")
    assert h1 == h2
    assert len(h1) == 32


def test_generate_random_token() -> None:
    plain1, hash1 = generate_random_token(32)
    plain2, hash2 = generate_random_token(32)
    assert plain1 != plain2
    assert hash_token(plain1) == hash1
    assert len(plain1) == 64  # 32 bytes in hex
    assert len(hash1) == 32


def test_ed25519_jwt_lifecycle() -> None:
    tm = TokenManager()
    user_id = uuid4()
    session_id = uuid4()

    token = tm.create_access_token(user_id=user_id, session_id=session_id)
    assert isinstance(token, str)

    claims = tm.verify_access_token(token)
    assert claims["sub"] == str(user_id)
    assert claims["sid"] == str(session_id)
    assert claims["iss"] == "qrit"
    assert "exp" in claims
    assert "iat" in claims


def test_ed25519_jwt_expired() -> None:
    tm = TokenManager()
    user_id = uuid4()
    session_id = uuid4()

    # Create token with ttl = -1 second
    token = tm.create_access_token(user_id=user_id, session_id=session_id, ttl_seconds=-1)

    with pytest.raises(ApiError) as exc_info:
        tm.verify_access_token(token)
    assert exc_info.value.code == "session_expired"


def test_ed25519_jwt_invalid_signature() -> None:
    tm1 = TokenManager()
    tm2 = TokenManager()  # different key
    user_id = uuid4()
    session_id = uuid4()

    token = tm1.create_access_token(user_id=user_id, session_id=session_id)

    with pytest.raises(ApiError) as exc_info:
        tm2.verify_access_token(token)
    assert exc_info.value.code == "invalid_token"
