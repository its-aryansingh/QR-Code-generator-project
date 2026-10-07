"""Security regressions for Google and GitHub sign-in.

Provider HTTP calls go through `apps.accounts.services.safe_client`, which these
tests replace with a fake so the real verification code paths run.
"""

import json
from datetime import UTC, datetime
from typing import Any

import pytest
from django.test import Client

from apps.accounts import services
from apps.accounts.models import OAuthAccount, Session, User
from qrit.settings import base as base_settings
from qrit.settings.env import env

GOOGLE_CLIENT_ID = "qrit-web.apps.googleusercontent.com"


class FakeResponse:
    def __init__(self, status_code: int, data: Any) -> None:
        self.status_code = status_code
        self._data = data

    def json(self) -> Any:
        return self._data


class FakeClient:
    """Answers provider calls by URL; records them."""

    def __init__(self, routes: dict[str, FakeResponse]) -> None:
        self.routes = routes
        self.calls: list[str] = []

    def get(self, url: str, **_: Any) -> FakeResponse:
        self.calls.append(url)
        return self.routes[url]

    def post(self, url: str, **_: Any) -> FakeResponse:
        self.calls.append(url)
        return self.routes[url]


def tokeninfo(**overrides: Any) -> dict[str, Any]:
    claims = {
        "iss": "https://accounts.google.com",
        "aud": GOOGLE_CLIENT_ID,
        "sub": "google-123",
        "email": "alice@example.com",
        "email_verified": "true",
        "name": "Alice",
    }
    claims.update(overrides)
    return claims


def google_client(claims: dict[str, Any], status_code: int = 200) -> FakeClient:
    return FakeClient(
        {"https://oauth2.googleapis.com/tokeninfo": FakeResponse(status_code, claims)}
    )


def github_client(emails: list[dict[str, Any]], public_email: str | None = None) -> FakeClient:
    return FakeClient(
        {
            "https://github.com/login/oauth/access_token": FakeResponse(
                200, {"access_token": "gho_x"}
            ),
            "https://api.github.com/user": FakeResponse(
                200, {"id": 4242, "login": "octo", "name": "Octo", "email": public_email}
            ),
            "https://api.github.com/user/emails": FakeResponse(200, emails),
        }
    )


def post(path: str, body: dict[str, Any]) -> Any:
    return Client().post(path, data=json.dumps(body), content_type="application/json")


@pytest.fixture
def providers(settings: Any) -> Any:
    settings.OAUTH_TEST_TOKENS = False  # behave like production
    settings.GOOGLE_CLIENT_ID = GOOGLE_CLIENT_ID
    settings.GITHUB_CLIENT_ID = "gh-client"
    settings.GITHUB_CLIENT_SECRET = "gh-secret"
    return settings


def test_fake_provider_tokens_are_off_unless_the_test_settings_enable_them() -> None:
    assert base_settings.OAUTH_TEST_TOKENS is False
    # APP_ENV must be a real setting; getattr(settings, "APP_ENV", "local")
    # used to report "local" in production.
    assert base_settings.APP_ENV == env.APP_ENV


@pytest.mark.django_db
def test_test_google_credential_is_rejected_in_production(providers: Any, monkeypatch: Any) -> None:
    victim = User.objects.create_user(email="victim@example.com", password="Victim-pass-123")
    fake = google_client({"error": "invalid_token"}, status_code=400)
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post("/api/v1/auth/google", {"credential": "test-google:1:victim@example.com:x"})

    assert resp.status_code == 401
    assert fake.calls == ["https://oauth2.googleapis.com/tokeninfo"]  # really asked Google
    assert not OAuthAccount.objects.filter(user=victim).exists()


@pytest.mark.django_db
def test_test_github_code_is_rejected_in_production(providers: Any, monkeypatch: Any) -> None:
    User.objects.create_user(email="victim@example.com", password="Victim-pass-123")
    fake = FakeClient(
        {
            "https://github.com/login/oauth/access_token": FakeResponse(
                200, {"error": "bad_verification_code"}
            ),
        }
    )
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post("/api/v1/auth/github", {"code": "test-github:1:victim@example.com:x"})

    assert resp.status_code == 401
    assert not OAuthAccount.objects.exists()


@pytest.mark.django_db
@pytest.mark.parametrize(
    ("overrides", "label"),
    [
        ({"aud": "another-app.apps.googleusercontent.com"}, "token minted for another app"),
        ({"iss": "https://evil.example"}, "wrong issuer"),
        ({"email_verified": "false"}, "unverified email"),
    ],
)
def test_google_credentials_that_fail_verification(
    providers: Any, monkeypatch: Any, overrides: dict[str, Any], label: str
) -> None:
    User.objects.create_user(email="alice@example.com", password="Alice-pass-123")
    monkeypatch.setattr(services, "safe_client", lambda **_: google_client(tokeninfo(**overrides)))

    resp = post("/api/v1/auth/google", {"credential": "eyJ.real.token"})

    assert resp.status_code == 401, label
    assert not OAuthAccount.objects.exists(), label


@pytest.mark.django_db
def test_valid_google_credential_signs_in(providers: Any, monkeypatch: Any) -> None:
    monkeypatch.setattr(services, "safe_client", lambda **_: google_client(tokeninfo()))

    resp = post("/api/v1/auth/google", {"credential": "eyJ.real.token"})

    assert resp.status_code == 200, resp.content
    assert resp.json()["user"]["email"] == "alice@example.com"
    assert OAuthAccount.objects.filter(provider="google", provider_user_id="google-123").exists()


@pytest.mark.django_db
def test_github_never_uses_an_unverified_email(providers: Any, monkeypatch: Any) -> None:
    victim = User.objects.create_user(email="victim@example.com", password="Victim-pass-123")
    # The attacker added the victim's address to their GitHub account without verifying it.
    fake = github_client(
        [{"email": "victim@example.com", "primary": True, "verified": False}],
        public_email="victim@example.com",
    )
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post("/api/v1/auth/github", {"code": "real-code"})

    assert resp.status_code == 401
    assert not OAuthAccount.objects.filter(user=victim).exists()


@pytest.mark.django_db
def test_github_prefers_the_primary_verified_email(providers: Any, monkeypatch: Any) -> None:
    fake = github_client(
        [
            {"email": "unverified@example.com", "primary": False, "verified": False},
            {"email": "old@example.com", "primary": False, "verified": True},
            {"email": "Octo@Example.com", "primary": True, "verified": True},
        ]
    )
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post("/api/v1/auth/github", {"code": "real-code"})

    assert resp.status_code == 200, resp.content
    assert resp.json()["user"]["email"] == "octo@example.com"


@pytest.mark.django_db
def test_unverified_account_loses_its_password_and_sessions_before_linking(
    providers: Any, monkeypatch: Any
) -> None:
    # Someone registered alice@example.com without owning it.
    squatter = User.objects.create_user(email="alice@example.com", password="Squatter-pass-123")
    assert squatter.email_verified_at is None
    old_session, *_ = services.start_session(user=squatter)
    monkeypatch.setattr(services, "safe_client", lambda **_: google_client(tokeninfo()))

    resp = post("/api/v1/auth/google", {"credential": "eyJ.real.token"})

    assert resp.status_code == 200, resp.content
    squatter.refresh_from_db()
    assert squatter.email_verified_at is not None
    assert not squatter.has_usable_password()
    old_session.refresh_from_db()
    assert old_session.revoked_at is not None
    assert Session.objects.filter(user=squatter, revoked_at__isnull=True).count() == 1


@pytest.mark.django_db
def test_verified_account_keeps_its_password_when_linking(providers: Any, monkeypatch: Any) -> None:
    owner = User.objects.create_user(email="alice@example.com", password="Owner-pass-123")
    owner.email_verified_at = datetime.now(UTC)
    owner.save(update_fields=["email_verified_at"])
    monkeypatch.setattr(services, "safe_client", lambda **_: google_client(tokeninfo()))

    resp = post("/api/v1/auth/google", {"credential": "eyJ.real.token"})

    assert resp.status_code == 200, resp.content
    owner.refresh_from_db()
    assert owner.check_password("Owner-pass-123")
