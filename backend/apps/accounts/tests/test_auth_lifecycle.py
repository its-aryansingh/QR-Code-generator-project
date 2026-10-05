"""Port of Go v2 TestAuthLifecycle (foundation_test.go:13-99).

Verifies the complete authentication lifecycle:
- registration, duplicate 409
- /me inspection and PATCH updates
- CSRF double-submit protection
- email verification link and single-use 410
- login, invalid password, account lockout
- logout session revocation
- refresh token rotation and reuse detection (family revocation)
- password forgot/reset and session revocation
"""

import json

import pytest
from django.test import Client

from apps.accounts.models import EmailToken, User


@pytest.mark.django_db
def test_auth_lifecycle() -> None:
    client = Client()
    email = "alice@example.com"
    password = "correct horse battery"

    # 1. Register Alice
    resp = client.post(
        "/v1/auth/register",
        data=json.dumps({"email": email, "password": password, "name": "alice"}),
        content_type="application/json",
    )
    assert resp.status_code == 201, resp.content
    data = resp.json()
    assert data["user"]["email"] == email
    assert data["user"]["has_password"] is True
    assert "token" in data
    _access_token = data["token"]

    # Verify no password_hash leaked in body
    assert "password_hash" not in str(resp.content)

    # 2. GET /v1/me
    resp = client.get("/v1/me")
    assert resp.status_code == 200, resp.content
    me_data = resp.json()
    assert me_data["user"]["email"] == email
    assert me_data["user"]["has_password"] is True
    assert "workspaces" in me_data
    assert len(me_data["workspaces"]) >= 1

    # 3. Duplicate email registration -> 409 Conflict
    c2 = Client()
    resp = c2.post(
        "/v1/auth/register",
        data=json.dumps({"email": email, "password": "correct horse battery"}),
        content_type="application/json",
    )
    assert resp.status_code == 409, resp.content

    # 4. CSRF: cookie-authenticated mutation without matching X-CSRF-Token is refused with 403
    csrf_token = client.cookies["qrit_csrf"].value
    resp = client.patch(
        "/v1/me",
        data=json.dumps({"name": "A"}),
        content_type="application/json",
        HTTP_X_CSRF_TOKEN="wrong-csrf-token",
    )
    assert resp.status_code == 403, resp.content

    # Mutation with correct CSRF token succeeds
    resp = client.patch(
        "/v1/me",
        data=json.dumps({"name": "Alice", "timezone": "Asia/Kolkata"}),
        content_type="application/json",
        HTTP_X_CSRF_TOKEN=csrf_token,
    )
    assert resp.status_code == 200, resp.content
    assert resp.json()["name"] == "Alice"
    assert resp.json()["timezone"] == "Asia/Kolkata"

    # Invalid timezone -> 422
    resp = client.patch(
        "/v1/me",
        data=json.dumps({"timezone": "Mars/Base"}),
        content_type="application/json",
        HTTP_X_CSRF_TOKEN=csrf_token,
    )
    assert resp.status_code == 422, resp.content

    # 5. Email verification via token
    user = User.objects.get(email=email)
    tok_row = EmailToken.objects.filter(user=user, purpose="verify_email").first()
    assert tok_row is not None

    # Get a fresh token from services for verify-email test
    from apps.accounts.tokens import generate_random_token

    plain_vtok, vtok_hash = generate_random_token(32)
    tok_row.token_hash = vtok_hash
    tok_row.save(update_fields=["token_hash"])

    unauth_client = Client()
    resp = unauth_client.post(
        "/v1/auth/verify-email",
        data=json.dumps({"token": plain_vtok}),
        content_type="application/json",
    )
    assert resp.status_code == 200, resp.content
    assert resp.json()["verified"] is True

    # Re-use of verification token -> 410 Gone
    resp = unauth_client.post(
        "/v1/auth/verify-email",
        data=json.dumps({"token": plain_vtok}),
        content_type="application/json",
    )
    assert resp.status_code == 410, resp.content

    # 6. Login, wrong password, correct password
    lc = Client()
    resp = lc.post(
        "/v1/auth/login",
        data=json.dumps({"email": email, "password": "wrong password"}),
        content_type="application/json",
    )
    assert resp.status_code == 401, resp.content

    resp = lc.post(
        "/v1/auth/login",
        data=json.dumps({"email": email, "password": password}),
        content_type="application/json",
    )
    assert resp.status_code == 200, resp.content
    login_access = resp.json()["token"]

    # 7. Logout revokes the session: old token stops working immediately
    bearer_client = Client()
    resp = bearer_client.get(
        "/v1/me",
        HTTP_AUTHORIZATION=f"Bearer {login_access}",
    )
    assert resp.status_code == 200, resp.content

    # Sign out via cookie-authenticated client
    resp = lc.post("/v1/auth/logout")
    assert resp.status_code == 204

    # Now bearer token fails with 401
    resp = bearer_client.get(
        "/v1/me",
        HTTP_AUTHORIZATION=f"Bearer {login_access}",
    )
    assert resp.status_code == 401

    # 8. Refresh rotation and reuse detection
    rc = Client()
    resp = rc.post(
        "/v1/auth/login",
        data=json.dumps({"email": email, "password": password}),
        content_type="application/json",
    )
    assert resp.status_code == 200
    old_refresh = rc.cookies["qrit_refresh"].value

    # First refresh succeeds
    resp = rc.post("/v1/auth/refresh")
    assert resp.status_code == 200, resp.content
    new_refresh = rc.cookies["qrit_refresh"].value
    assert new_refresh != old_refresh

    # Replay of old refresh token triggers reuse detection -> 401
    replay_client = Client()
    replay_client.cookies["qrit_refresh"] = old_refresh
    resp = replay_client.post("/v1/auth/refresh")
    assert resp.status_code == 401, resp.content
    assert "reuse" in resp.json()["detail"].lower()

    # The new refresh token is also revoked because whole family was revoked!
    resp = rc.post("/v1/auth/refresh")
    assert resp.status_code == 401, resp.content

    # 9. Account lockout after 5 failed login attempts
    lock_client = Client()
    for _ in range(5):
        resp = lock_client.post(
            "/v1/auth/login",
            data=json.dumps({"email": email, "password": "wrong password!!"}),
            content_type="application/json",
        )
        assert resp.status_code == 401

    # 6th attempt should be locked out with 429
    resp = lock_client.post(
        "/v1/auth/login",
        data=json.dumps({"email": email, "password": password}),
        content_type="application/json",
    )
    assert resp.status_code == 429, resp.content

    # 10. Forgot/reset password
    fc = Client()
    # Forgot password always returns 202 Accepted
    resp = fc.post(
        "/v1/auth/password/forgot",
        data=json.dumps({"email": email}),
        content_type="application/json",
    )
    assert resp.status_code == 202, resp.content

    resp = fc.post(
        "/v1/auth/password/forgot",
        data=json.dumps({"email": "nobody@example.com"}),
        content_type="application/json",
    )
    assert resp.status_code == 202, resp.content

    # Generate test reset token
    reset_plain, reset_hash = generate_random_token(32)
    EmailToken.objects.filter(user=user, purpose="reset_password").update(token_hash=reset_hash)

    # Short password -> 422
    resp = fc.post(
        "/v1/auth/password/reset",
        data=json.dumps({"token": reset_plain, "password": "short"}),
        content_type="application/json",
    )
    assert resp.status_code == 422, resp.content

    # Valid password -> 204
    resp = fc.post(
        "/v1/auth/password/reset",
        data=json.dumps({"token": reset_plain, "password": "a brand new passphrase"}),
        content_type="application/json",
    )
    assert resp.status_code == 204

    # Old login succeeds with new password (and lockout is cleared)
    resp = fc.post(
        "/v1/auth/login",
        data=json.dumps({"email": email, "password": "a brand new passphrase"}),
        content_type="application/json",
    )
    assert resp.status_code == 200, resp.content
