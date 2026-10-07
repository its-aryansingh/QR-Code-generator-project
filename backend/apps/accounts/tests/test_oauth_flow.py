"""Redirect flow for Google and GitHub: start -> provider -> /callback page -> POST code."""

import base64
import hashlib
import json
from typing import Any
from urllib.parse import parse_qs, urlsplit

import pytest
from django.core import signing
from django.test import Client

from apps.accounts import oauth_flow, services
from apps.accounts.models import OAuthAccount

APP = "https://app.example.com"
GOOGLE_CLIENT_ID = "qrit-web.apps.googleusercontent.com"


class FakeResponse:
    def __init__(self, status_code: int, data: Any) -> None:
        self.status_code = status_code
        self._data = data

    def json(self) -> Any:
        return self._data


class FakeProvider:
    """Stands in for safe_client(); answers by URL and records what was sent."""

    def __init__(self, routes: dict[str, Any]) -> None:
        self.routes = routes
        self.calls: list[tuple[str, str, dict[str, Any]]] = []

    def get(self, url: str, params: dict[str, Any] | None = None, **_: Any) -> FakeResponse:
        self.calls.append(("GET", url, params or {}))
        return self._answer(url, params or {})

    def post(self, url: str, data: dict[str, Any] | None = None, **_: Any) -> FakeResponse:
        self.calls.append(("POST", url, data or {}))
        return self._answer(url, data or {})

    def _answer(self, url: str, sent: dict[str, Any]) -> FakeResponse:
        route = self.routes[url]
        answer: FakeResponse = route(sent) if callable(route) else route
        return answer


def query(url: str) -> dict[str, str]:
    return {k: v[0] for k, v in parse_qs(urlsplit(url).query).items()}


def challenge_of(verifier: str) -> str:
    digest = hashlib.sha256(verifier.encode()).digest()
    return base64.urlsafe_b64encode(digest).rstrip(b"=").decode()


@pytest.fixture
def providers(settings: Any) -> Any:
    settings.OAUTH_TEST_TOKENS = False
    settings.APP_BASE_URL = APP
    settings.GOOGLE_CLIENT_ID = GOOGLE_CLIENT_ID
    settings.GOOGLE_CLIENT_SECRET = "google-secret"
    settings.GITHUB_CLIENT_ID = "gh-client"
    settings.GITHUB_CLIENT_SECRET = "gh-secret"
    return settings


def start(client: Client, provider: str, next_path: str = "/dashboard/qr") -> dict[str, str]:
    resp = client.get(f"/api/v1/auth/{provider}/start", {"next": next_path})
    assert resp.status_code == 302, resp.content
    return query(resp["Location"])


def github_routes(settings: Any, check: Any = None) -> dict[str, Any]:
    def token(sent: dict[str, Any]) -> FakeResponse:
        if check:
            check(sent)
        return FakeResponse(200, {"access_token": "gho_x"})

    return {
        settings.GITHUB_TOKEN_URL: token,
        f"{settings.GITHUB_API_URL}/user": FakeResponse(
            200, {"id": 77, "login": "octo", "name": "Octo"}
        ),
        f"{settings.GITHUB_API_URL}/user/emails": FakeResponse(
            200, [{"email": "octo@example.com", "primary": True, "verified": True}]
        ),
    }


def post(client: Client, path: str, body: dict[str, Any]) -> Any:
    return client.post(path, data=json.dumps(body), content_type="application/json")


def test_providers_endpoint_reports_what_is_configured(providers: Any) -> None:
    assert Client().get("/api/v1/auth/oauth/providers").json() == {"google": True, "github": True}
    providers.GITHUB_CLIENT_SECRET = ""
    assert Client().get("/api/v1/auth/oauth/providers").json() == {"google": True, "github": False}


def test_github_start_redirects_with_state_pkce_and_a_locked_down_cookie(providers: Any) -> None:
    client = Client()
    resp = client.get("/api/v1/auth/github/start", {"next": "/dashboard/qr"})

    assert resp["Location"].startswith(providers.GITHUB_AUTHORIZE_URL + "?")
    assert resp["Cache-Control"] == "no-store"
    params = query(resp["Location"])
    assert params["client_id"] == "gh-client"
    assert params["redirect_uri"] == f"{APP}/callback/github"
    assert params["scope"] == "read:user user:email"
    assert params["code_challenge_method"] == "S256"

    cookie = resp.cookies[oauth_flow.STATE_COOKIE]
    assert cookie["httponly"] and cookie["samesite"] == "Lax"
    assert cookie["path"] == "/api/v1/auth/"
    stored = signing.loads(cookie.value, salt="qrit.accounts.oauth.state")
    assert stored["s"] == params["state"]
    assert params["code_challenge"] == challenge_of(stored["v"])
    assert stored["next"] == "/dashboard/qr"


def test_google_start_asks_for_openid_with_a_nonce(providers: Any) -> None:
    params = start(Client(), "google")
    assert params["scope"] == "openid email profile"
    assert params["response_type"] == "code"
    assert params["redirect_uri"] == f"{APP}/callback/google"
    assert params["nonce"]


def test_start_for_unknown_or_unconfigured_providers(providers: Any) -> None:
    assert Client().get("/api/v1/auth/twitter/start")["Location"] == f"{APP}/login"
    providers.GOOGLE_CLIENT_SECRET = ""
    resp = Client().get("/api/v1/auth/google/start")
    assert resp["Location"] == f"{APP}/callback/google?error=not_configured"


@pytest.mark.parametrize("bad", ["//evil.example", "https://evil.example", "/\\evil.example"])
def test_next_cannot_leave_the_site(providers: Any, bad: str) -> None:
    resp = Client().get("/api/v1/auth/github/start", {"next": bad})
    stored = signing.loads(
        resp.cookies[oauth_flow.STATE_COOKIE].value, salt="qrit.accounts.oauth.state"
    )
    assert stored["next"] == "/dashboard"


@pytest.mark.django_db
def test_github_redirect_flow_end_to_end(providers: Any, monkeypatch: Any) -> None:
    client = Client()
    resp = client.get("/api/v1/auth/github/start", {"next": "/dashboard/qr"})
    params = query(resp["Location"])
    stored = signing.loads(
        resp.cookies[oauth_flow.STATE_COOKIE].value, salt="qrit.accounts.oauth.state"
    )

    def check(sent: dict[str, Any]) -> None:
        assert sent["code_verifier"] == stored["v"]
        assert challenge_of(sent["code_verifier"]) == params["code_challenge"]
        assert sent["redirect_uri"] == f"{APP}/callback/github"
        assert sent["client_secret"] == "gh-secret"

    fake = FakeProvider(github_routes(providers, check))
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    done = post(
        client,
        "/api/v1/auth/github",
        {"code": "gh-code", "state": params["state"], "redirect_uri": "https://evil.example/cb"},
    )

    assert done.status_code == 200, done.content
    body = done.json()
    assert body["user"]["email"] == "octo@example.com"
    assert body["user"]["has_password"] is False  # OAuth-only account
    assert body["next"] == "/dashboard/qr"
    assert body["access_token"]
    assert done.cookies[oauth_flow.STATE_COOKIE].value == ""  # cleared
    assert OAuthAccount.objects.filter(provider="github", provider_user_id="77").exists()


@pytest.mark.django_db
def test_github_callback_without_this_browsers_cookie_is_refused(
    providers: Any, monkeypatch: Any
) -> None:
    """Login CSRF: a callback link carrying someone else's code, opened in another browser."""
    params = start(Client(), "github")
    fake = FakeProvider(github_routes(providers))
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post(
        Client(), "/api/v1/auth/github", {"code": "attacker-code", "state": params["state"]}
    )

    assert resp.status_code == 400
    assert resp.json()["code"] == "invalid_state"
    assert fake.calls == []  # the code was never exchanged


@pytest.mark.django_db
@pytest.mark.parametrize("state", ["", "wrong-state"])
def test_github_state_must_match(providers: Any, monkeypatch: Any, state: str) -> None:
    client = Client()
    start(client, "github")
    fake = FakeProvider(github_routes(providers))
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post(client, "/api/v1/auth/github", {"code": "gh-code", "state": state})

    assert resp.status_code == 400
    assert fake.calls == []


@pytest.mark.django_db
def test_state_for_one_provider_does_not_work_for_the_other(
    providers: Any, monkeypatch: Any
) -> None:
    client = Client()
    params = start(client, "github")
    monkeypatch.setattr(services, "safe_client", lambda **_: FakeProvider({}))

    resp = post(client, "/api/v1/auth/google", {"code": "c", "state": params["state"]})

    assert resp.status_code == 400
    assert resp.json()["code"] == "invalid_state"


def google_routes(settings: Any, nonce: str, check: Any = None) -> dict[str, Any]:
    def token(sent: dict[str, Any]) -> FakeResponse:
        if check:
            check(sent)
        return FakeResponse(200, {"id_token": "header.payload.signature"})

    claims = {
        "iss": "https://accounts.google.com",
        "aud": GOOGLE_CLIENT_ID,
        "sub": "g-55",
        "email": "alice@example.com",
        "email_verified": "true",
        "name": "Alice",
        "nonce": nonce,
    }
    return {
        settings.GOOGLE_TOKEN_URL: token,
        settings.GOOGLE_TOKENINFO_URL: FakeResponse(200, claims),
    }


@pytest.mark.django_db
def test_google_redirect_flow_end_to_end(providers: Any, monkeypatch: Any) -> None:
    client = Client()
    resp = client.get("/api/v1/auth/google/start", {"next": "/dashboard/analytics"})
    params = query(resp["Location"])
    stored = signing.loads(
        resp.cookies[oauth_flow.STATE_COOKIE].value, salt="qrit.accounts.oauth.state"
    )

    def check(sent: dict[str, Any]) -> None:
        assert sent["grant_type"] == "authorization_code"
        assert sent["client_secret"] == "google-secret"
        assert sent["redirect_uri"] == f"{APP}/callback/google"
        assert challenge_of(sent["code_verifier"]) == params["code_challenge"]

    fake = FakeProvider(google_routes(providers, nonce=stored["n"], check=check))
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    done = post(client, "/api/v1/auth/google", {"code": "g-code", "state": params["state"]})

    assert done.status_code == 200, done.content
    assert done.json()["user"]["email"] == "alice@example.com"
    assert done.json()["next"] == "/dashboard/analytics"


@pytest.mark.django_db
def test_google_token_with_another_sessions_nonce_is_refused(
    providers: Any, monkeypatch: Any
) -> None:
    client = Client()
    params = start(client, "google")
    fake = FakeProvider(google_routes(providers, nonce="some-other-nonce"))
    monkeypatch.setattr(services, "safe_client", lambda **_: fake)

    resp = post(client, "/api/v1/auth/google", {"code": "g-code", "state": params["state"]})

    assert resp.status_code == 401
    assert not OAuthAccount.objects.exists()


def test_railway_domains_are_always_allowed_hosts() -> None:
    from qrit.settings.env import Settings

    configured = Settings(
        ALLOWED_HOSTS="api.example.com",
        RAILWAY_PRIVATE_DOMAIN="qr-backend.railway.internal",
        RAILWAY_PUBLIC_DOMAIN="qr-api-production.up.railway.app",
    )
    assert configured.allowed_hosts_list == [
        "api.example.com",
        "healthcheck.railway.app",
        "qr-backend.railway.internal",
        "qr-api-production.up.railway.app",
    ]
    assert Settings(
        ALLOWED_HOSTS="*", RAILWAY_PRIVATE_DOMAIN="x.railway.internal"
    ).allowed_hosts_list == ["*"]
