"""End-to-end tests for Google OAuth and GitHub OAuth flows."""

import json

import pytest
from django.test import Client

from apps.accounts.models import OAuthAccount, User
from apps.orgs.models import Organization
from apps.workspaces.models import Workspace


@pytest.mark.django_db
def test_google_oauth_new_user() -> None:
    client = Client()
    credential = "test-google:1001:alice.google@example.com:Alice Google"

    # 1. Sign in with Google (new user)
    resp = client.post(
        "/v1/auth/google",
        data=json.dumps({"credential": credential}),
        content_type="application/json",
    )
    assert resp.status_code == 200, resp.content
    data = resp.json()
    assert data["user"]["email"] == "alice.google@example.com"
    assert "token" in data
    assert "qrit_access" in client.cookies
    assert "qrit_refresh" in client.cookies
    assert "qrit_csrf" in client.cookies

    # Check database models
    user = User.objects.get(email="alice.google@example.com")
    assert user.name == "Alice Google"
    assert user.email_verified_at is not None

    oauth_acc = OAuthAccount.objects.get(user=user, provider="google")
    assert oauth_acc.provider_user_id == "1001"

    # User has default Organization and Workspace
    org = Organization.objects.filter(slug__contains="alice").first()
    assert org is not None

    ws = Workspace.objects.filter(owner=user).first()
    assert ws is not None

    # 2. Subsequent sign in with Google logs into same user
    c2 = Client()
    resp2 = c2.post(
        "/v1/auth/google",
        data=json.dumps({"id_token": credential}),  # test id_token field alias
        content_type="application/json",
    )
    assert resp2.status_code == 200
    assert resp2.json()["user"]["id"] == str(user.id)


@pytest.mark.django_db
def test_google_oauth_link_existing_user() -> None:
    # Existing user registered with password
    user = User.objects.create_user(
        email="bob@example.com",
        password="existing-password-123",
        name="Bob Original",
    )

    client = Client()
    credential = "test-google:1002:bob@example.com:Bob Google"
    resp = client.post(
        "/v1/auth/google",
        data=json.dumps({"credential": credential}),
        content_type="application/json",
    )
    assert resp.status_code == 200
    assert resp.json()["user"]["id"] == str(user.id)

    # OAuthAccount linked
    oauth = OAuthAccount.objects.get(provider="google", provider_user_id="1002")
    assert oauth.user_id == user.id


@pytest.mark.django_db
def test_github_oauth_new_user() -> None:
    client = Client()
    code = "test-github:2001:charlie.github@example.com:Charlie Dev"

    # 1. Sign in with GitHub (new user)
    resp = client.post(
        "/v1/auth/github",
        data=json.dumps({"code": code, "redirect_uri": "http://localhost:3000/auth/callback"}),
        content_type="application/json",
    )
    assert resp.status_code == 200, resp.content
    data = resp.json()
    assert data["user"]["email"] == "charlie.github@example.com"
    assert "token" in data
    assert "qrit_access" in client.cookies

    # Check database models
    user = User.objects.get(email="charlie.github@example.com")
    assert user.name == "Charlie Dev"
    assert user.email_verified_at is not None

    oauth_acc = OAuthAccount.objects.get(user=user, provider="github")
    assert oauth_acc.provider_user_id == "2001"

    # 2. Subsequent sign in with GitHub
    c2 = Client()
    resp2 = c2.post(
        "/v1/auth/github",
        data=json.dumps({"code": code}),
        content_type="application/json",
    )
    assert resp2.status_code == 200
    assert resp2.json()["user"]["id"] == str(user.id)


@pytest.mark.django_db
def test_github_oauth_link_existing_user() -> None:
    user = User.objects.create_user(
        email="dana@example.com",
        password="existing-password-456",
        name="Dana",
    )

    client = Client()
    code = "test-github:2002:dana@example.com:Dana Developer"
    resp = client.post(
        "/v1/auth/github",
        data=json.dumps({"code": code}),
        content_type="application/json",
    )
    assert resp.status_code == 200
    assert resp.json()["user"]["id"] == str(user.id)

    oauth = OAuthAccount.objects.get(provider="github", provider_user_id="2002")
    assert oauth.user_id == user.id
