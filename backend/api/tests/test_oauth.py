"""Google and GitHub sign-in, end to end through the real views.

Provider HTTP calls and Google's JWKS are replaced with in-process fakes; the
ID tokens are real RS256 JWTs signed with a key generated for this module, so
signature, audience, issuer, expiry and nonce checks all run for real.
"""

import base64
import hashlib
import time
from datetime import timedelta
from types import SimpleNamespace
from unittest import mock
from urllib.parse import parse_qs, urlsplit

import jwt
from cryptography.hazmat.primitives.asymmetric import rsa
from django.test import TestCase, override_settings
from django.utils import timezone

from api.models import AuthHandoffCode, OAuthAccount, RefreshToken, User
from api.utils import oauth
from api.utils.auth import hash_password, sign_tokens

_KEY = rsa.generate_private_key(public_exponent=65537, key_size=2048)
_OTHER_KEY = rsa.generate_private_key(public_exponent=65537, key_size=2048)

APP = "https://app.example.com"
GH_TOKEN = "https://github.com/login/oauth/access_token"
GH_USER = "https://api.github.com/user"
GH_EMAILS = "https://api.github.com/user/emails"
GOOGLE_TOKEN = "https://oauth2.googleapis.com/token"
STRONG_PASSWORD = "Str0ng!Passw0rd"


class FakeJWKClient:
    def get_signing_key_from_jwt(self, token):
        jwt.get_unverified_header(token)  # garbage raises DecodeError, as in production
        return SimpleNamespace(key=_KEY.public_key())


class FakeResponse:
    def __init__(self, status_code, data):
        self.status_code = status_code
        self._data = data

    def json(self):
        return self._data


class FakeSession:
    """Answers provider calls by (method, url); records what was sent."""

    def __init__(self, routes):
        self.routes = routes
        self.calls = []

    def post(self, url, data=None, headers=None, timeout=None):
        self.calls.append(("POST", url, data, headers))
        return self._reply(("POST", url), data)

    def get(self, url, headers=None, timeout=None):
        self.calls.append(("GET", url, None, headers))
        return self._reply(("GET", url), None)

    def _reply(self, key, data):
        handler = self.routes[key]
        return handler(data) if callable(handler) else handler


def google_id_token(*, sub="g-123", email="alice@example.com", verified=True, nonce=None,
                    aud="google-client", iss="https://accounts.google.com", exp_in=600,
                    key=_KEY, name="Alice Doe"):
    now = int(time.time())
    claims = {
        "sub": sub, "email": email, "email_verified": verified, "aud": aud, "iss": iss,
        "iat": now, "exp": now + exp_in, "name": name, "picture": "https://img.example/alice.png",
    }
    if nonce is not None:
        claims["nonce"] = nonce
    return jwt.encode(claims, key, algorithm="RS256", headers={"kid": "test-key"})


def github_session(*, uid=42, login="octocat", name="Octo Cat", emails=None, token_body=None):
    if emails is None:
        emails = [
            {"email": "octo-old@example.com", "primary": False, "verified": True},
            {"email": "Octo@Example.com", "primary": True, "verified": True},
        ]
    return FakeSession({
        ("POST", GH_TOKEN): FakeResponse(200, token_body or {"access_token": "gho_test", "token_type": "bearer"}),
        ("GET", GH_USER): FakeResponse(200, {"id": uid, "login": login, "name": name,
                                             "avatar_url": "https://avatars.example/octo.png"}),
        ("GET", GH_EMAILS): FakeResponse(200, emails),
    })


def google_session(**token_kwargs):
    return FakeSession({
        ("POST", GOOGLE_TOKEN): lambda data: FakeResponse(
            200, {"access_token": "ya29.test", "id_token": google_id_token(**token_kwargs)}
        ),
    })


def query(url):
    return {k: v[0] for k, v in parse_qs(urlsplit(url).query).items()}


@override_settings(
    APP_BASE_URL=APP,
    OAUTH_CALLBACK_BASE_URL=APP,
    GOOGLE_CLIENT_ID="google-client",
    GOOGLE_CLIENT_SECRET="google-secret",
    GITHUB_CLIENT_ID="gh-client",
    GITHUB_CLIENT_SECRET="gh-secret",
    SECURE_SSL_REDIRECT=False,
)
class OAuthFlowTestCase(TestCase):
    # ------------------------------------------------------------ helpers
    def start(self, provider, next_path="/dashboard/qr", **extra):
        resp = self.client.get(f"/api/v1/auth/oauth/{provider}/start", {"next": next_path, **extra}, secure=True)
        self.assertEqual(resp.status_code, 302)
        return resp, query(resp["Location"])

    def callback(self, provider, session, params):
        with mock.patch.object(oauth, "_http", return_value=session), \
                mock.patch.object(oauth, "_jwk_client", return_value=FakeJWKClient()):
            resp = self.client.get(f"/api/v1/auth/oauth/{provider}/callback", params, secure=True)
        self.assertEqual(resp.status_code, 302)
        self.assertTrue(resp["Location"].startswith(f"{APP}/auth/callback"), resp["Location"])
        return resp, query(resp["Location"])

    def exchange(self, code):
        return self.client.post("/api/v1/auth/oauth/exchange", {"code": code}, content_type="application/json")

    def sign_in(self, provider, session_factory, next_path="/dashboard/qr"):
        _, params = self.start(provider, next_path)
        session = session_factory(params)
        resp, result = self.callback(provider, session, {"code": "provider-code", "state": params["state"]})
        return resp, result, session

    def github_sign_in(self, **kwargs):
        return self.sign_in("github", lambda _params: github_session(**kwargs))

    def google_sign_in(self, **kwargs):
        def factory(params):
            kwargs.setdefault("nonce", params["nonce"])
            return google_session(**kwargs)
        return self.sign_in("google", factory)

    def auth(self, token):
        return {"HTTP_AUTHORIZATION": f"Bearer {token}"}

    def make_user(self, email, *, password=STRONG_PASSWORD, verified=True):
        now = timezone.now()
        return User.objects.create(
            email=email, password_hash=hash_password(password), name="Existing",
            email_verified=verified, created_at=now, updated_at=now,
        )

    # ------------------------------------------------------------ discovery
    def test_providers_endpoint_reports_configured_providers(self):
        resp = self.client.get("/api/v1/auth/oauth/providers")
        self.assertEqual(resp.json()["data"], {"google": True, "github": True})
        with self.settings(GITHUB_CLIENT_SECRET=""):
            resp = self.client.get("/api/v1/auth/oauth/providers")
        self.assertEqual(resp.json()["data"], {"google": True, "github": False})

    # ------------------------------------------------------------ start
    def test_github_start_redirects_with_pkce_and_a_locked_down_state_cookie(self):
        resp, params = self.start("github")
        self.assertTrue(resp["Location"].startswith("https://github.com/login/oauth/authorize?"))
        self.assertEqual(params["client_id"], "gh-client")
        self.assertEqual(params["redirect_uri"], f"{APP}/api/v1/auth/oauth/github/callback")
        self.assertEqual(params["scope"], "read:user user:email")
        self.assertEqual(params["code_challenge_method"], "S256")
        self.assertEqual(resp["Cache-Control"], "no-store")

        cookie = resp.cookies[oauth.STATE_COOKIE]
        self.assertTrue(cookie["httponly"])
        self.assertTrue(cookie["secure"])
        self.assertEqual(cookie["samesite"], "Lax")
        self.assertEqual(cookie["path"], "/api/v1/auth/oauth/")

        state = oauth.load_state(cookie.value)
        self.assertEqual(state["s"], params["state"])
        self.assertEqual(state["next"], "/dashboard/qr")
        expected = base64.urlsafe_b64encode(hashlib.sha256(state["v"].encode()).digest()).rstrip(b"=").decode()
        self.assertEqual(params["code_challenge"], expected)

    def test_google_start_requests_openid_with_nonce(self):
        resp, params = self.start("google")
        self.assertTrue(resp["Location"].startswith("https://accounts.google.com/o/oauth2/v2/auth?"))
        self.assertEqual(params["scope"], "openid email profile")
        self.assertEqual(params["response_type"], "code")
        self.assertEqual(params["prompt"], "select_account")
        state = oauth.load_state(resp.cookies[oauth.STATE_COOKIE].value)
        self.assertEqual(state["n"], params["nonce"])

    def test_start_for_unconfigured_or_unknown_provider_returns_to_frontend(self):
        with self.settings(GOOGLE_CLIENT_ID=""):
            resp = self.client.get("/api/v1/auth/oauth/google/start")
        self.assertEqual(query(resp["Location"])["error"], "not_configured")
        resp = self.client.get("/api/v1/auth/oauth/twitter/start")
        self.assertEqual(query(resp["Location"])["error"], "unknown_provider")

    def test_offsite_next_values_are_replaced(self):
        for bad in ("//evil.example", "https://evil.example/x", "/\\evil.example", "/ok\nSet-Cookie:x", "x" * 600):
            resp = self.client.get("/api/v1/auth/oauth/github/start", {"next": bad})
            self.assertEqual(oauth.load_state(resp.cookies[oauth.STATE_COOKIE].value)["next"], "/dashboard", bad)
        self.assertEqual(oauth.safe_next("/dashboard/qr?tab=2"), "/dashboard/qr?tab=2")

    # ------------------------------------------------------------ GitHub
    def test_github_sign_up_end_to_end(self):
        resp, result, session = self.github_sign_in()
        self.assertNotIn("error", result)
        self.assertEqual(result["provider"], "github")
        self.assertNotIn("token", resp["Location"].replace("/auth/callback", ""))
        self.assertEqual(resp.cookies[oauth.STATE_COOKIE].value, "")  # state cookie cleared

        token_call = session.calls[0]
        self.assertEqual(token_call[1], GH_TOKEN)
        self.assertEqual(token_call[2]["client_secret"], "gh-secret")
        self.assertEqual(token_call[2]["redirect_uri"], f"{APP}/api/v1/auth/oauth/github/callback")
        self.assertEqual(len(token_call[2]["code_verifier"]), 64)

        data = self.exchange(result["code"]).json()["data"]
        self.assertEqual(data["next"], "/dashboard/qr")
        self.assertTrue(data["is_new_user"])
        self.assertEqual(data["user"]["email"], "octo@example.com")  # primary verified, normalised
        self.assertFalse(data["user"]["has_password"])
        self.assertTrue(data["access_token"] and data["refresh_token"])

        user = User.objects.get(email="octo@example.com")
        self.assertTrue(user.email_verified)
        self.assertFalse(user.has_usable_password)
        self.assertEqual(user.name, "Octo Cat")
        self.assertTrue(user.api_key.startswith("ak_"))
        self.assertTrue(OAuthAccount.objects.filter(user=user, provider="github", provider_user_id="42").exists())

        me = self.client.get("/api/v1/auth/me", **self.auth(data["access_token"])).json()["data"]
        self.assertEqual(me["connected_accounts"], ["github"])
        self.assertFalse(me["has_password"])

        again = self.exchange(result["code"])
        self.assertEqual(again.status_code, 400)
        self.assertEqual(again.json()["code"], "invalid_code")

    def test_handoff_code_expires(self):
        _, result, _ = self.github_sign_in()
        AuthHandoffCode.objects.update(expires_at=timezone.now() - timedelta(seconds=1))
        self.assertEqual(self.exchange(result["code"]).status_code, 400)
        self.assertEqual(self.exchange("not-a-code").status_code, 400)
        self.assertEqual(self.exchange(None).status_code, 400)

    def test_returning_user_is_matched_by_provider_id_not_email(self):
        self.github_sign_in()
        _, result, _ = self.github_sign_in(emails=[{"email": "renamed@example.com", "primary": True, "verified": True}])
        data = self.exchange(result["code"]).json()["data"]
        self.assertFalse(data["is_new_user"])
        self.assertEqual(data["user"]["email"], "octo@example.com")
        self.assertEqual(User.objects.count(), 1)
        self.assertEqual(OAuthAccount.objects.get().email, "renamed@example.com")

    def test_unverified_or_missing_github_email_is_refused(self):
        _, result, _ = self.github_sign_in(emails=[{"email": "x@example.com", "primary": True, "verified": False}])
        self.assertEqual(result["error"], "email_unverified")
        _, result, _ = self.github_sign_in(emails=[])
        self.assertEqual(result["error"], "email_missing")
        self.assertEqual(User.objects.count(), 0)

    def test_state_mismatch_never_exchanges_the_code(self):
        _, params = self.start("github")
        session = github_session()
        _, result = self.callback("github", session, {"code": "c", "state": params["state"] + "x"})
        self.assertEqual(result["error"], "invalid_state")
        self.assertEqual(session.calls, [])

    def test_missing_state_cookie_is_refused(self):
        _, params = self.start("github")
        self.client.cookies.clear()
        session = github_session()
        _, result = self.callback("github", session, {"code": "c", "state": params["state"]})
        self.assertEqual(result["error"], "invalid_state")
        self.assertEqual(session.calls, [])

    def test_state_issued_for_one_provider_is_refused_by_the_other(self):
        _, params = self.start("github")
        _, result = self.callback("google", google_session(), {"code": "c", "state": params["state"]})
        self.assertEqual(result["error"], "invalid_state")

    def test_tampered_state_cookie_is_refused(self):
        _, params = self.start("github")
        self.client.cookies[oauth.STATE_COOKIE] = self.client.cookies[oauth.STATE_COOKIE].value[:-2] + "xx"
        _, result = self.callback("github", github_session(), {"code": "c", "state": params["state"]})
        self.assertEqual(result["error"], "invalid_state")

    def test_user_cancelling_consent_returns_a_readable_error(self):
        _, params = self.start("github", "/dashboard/analytics")
        _, result = self.callback("github", github_session(), {"error": "access_denied", "state": params["state"]})
        self.assertEqual(result["error"], "access_denied")
        self.assertEqual(result["next"], "/dashboard/analytics")

    def test_provider_rejecting_the_code(self):
        _, result, _ = self.github_sign_in(token_body={"error": "bad_verification_code"})
        self.assertEqual(result["error"], "exchange_failed")

    # ------------------------------------------------------------ linking rules
    def test_existing_verified_password_account_is_linked_and_keeps_its_password(self):
        user = self.make_user("octo@example.com")
        _, result, _ = self.github_sign_in()
        data = self.exchange(result["code"]).json()["data"]
        self.assertEqual(data["user"]["id"], str(user.id))
        self.assertFalse(data["is_new_user"])
        self.assertFalse(data["password_reset"])
        self.assertTrue(data["user"]["has_password"])
        login = self.client.post("/api/v1/auth/login", {"email": "octo@example.com", "password": STRONG_PASSWORD},
                                 content_type="application/json")
        self.assertEqual(login.status_code, 200)

    def test_unverified_account_loses_its_unproven_password_and_sessions(self):
        squatter = self.make_user("octo@example.com", verified=False)
        sign_tokens(str(squatter.id), squatter.email, "free")
        _, result, _ = self.github_sign_in()
        data = self.exchange(result["code"]).json()["data"]
        self.assertTrue(data["password_reset"])
        self.assertEqual(data["user"]["id"], str(squatter.id))

        squatter.refresh_from_db()
        self.assertTrue(squatter.email_verified)
        self.assertFalse(squatter.has_usable_password)
        # Only the fresh session from the exchange is still live.
        self.assertEqual(RefreshToken.objects.filter(user=squatter, revoked=False).count(), 1)
        login = self.client.post("/api/v1/auth/login", {"email": "octo@example.com", "password": STRONG_PASSWORD},
                                 content_type="application/json")
        self.assertEqual(login.status_code, 401)

    def test_same_verified_email_on_google_and_github_is_one_account(self):
        _, g, _ = self.google_sign_in(email="octo@example.com")
        google_user = self.exchange(g["code"]).json()["data"]["user"]
        _, h, _ = self.github_sign_in()
        github_user = self.exchange(h["code"]).json()["data"]["user"]
        self.assertEqual(google_user["id"], github_user["id"])
        self.assertEqual(OAuthAccount.objects.filter(user_id=google_user["id"]).count(), 2)

    def test_second_account_of_same_provider_is_not_silently_swapped_in(self):
        self.github_sign_in()
        _, result, _ = self.github_sign_in(uid=99)
        self.assertEqual(result["error"], "provider_already_linked")

    # ------------------------------------------------------------ Google
    def test_google_sign_up_end_to_end(self):
        _, result, session = self.google_sign_in()
        sent = session.calls[0][2]
        self.assertEqual(sent["grant_type"], "authorization_code")
        self.assertEqual(sent["client_secret"], "google-secret")
        self.assertEqual(sent["redirect_uri"], f"{APP}/api/v1/auth/oauth/google/callback")
        self.assertIn("code_verifier", sent)

        data = self.exchange(result["code"]).json()["data"]
        self.assertTrue(data["is_new_user"])
        self.assertEqual(data["provider"], "google")
        self.assertEqual(data["user"]["email"], "alice@example.com")
        self.assertEqual(data["user"]["name"], "Alice Doe")
        self.assertEqual(data["user"]["avatar_url"], "https://img.example/alice.png")
        self.assertTrue(OAuthAccount.objects.filter(provider="google", provider_user_id="g-123").exists())

    def test_google_tokens_that_fail_verification_are_refused(self):
        cases = {
            "wrong nonce": {"nonce": "attacker-nonce"},
            "wrong audience": {"aud": "someone-elses-client"},
            "wrong issuer": {"iss": "https://evil.example"},
            "expired": {"exp_in": -3600},
            "wrong key": {"key": _OTHER_KEY},
        }
        for label, kwargs in cases.items():
            _, result, _ = self.google_sign_in(**kwargs)
            self.assertEqual(result.get("error"), "invalid_token", label)
        self.assertEqual(User.objects.count(), 0)

    def test_google_unverified_email_is_refused(self):
        _, result, _ = self.google_sign_in(verified=False)
        self.assertEqual(result["error"], "email_unverified")

    def test_google_identity_services_credential_endpoint(self):
        with mock.patch.object(oauth, "_jwk_client", return_value=FakeJWKClient()):
            ok = self.client.post("/api/v1/auth/google", {"credential": google_id_token()},
                                  content_type="application/json")
            bad = self.client.post("/api/v1/auth/google", {"credential": google_id_token(aud="other")},
                                   content_type="application/json")
            garbage = self.client.post("/api/v1/auth/google", {"credential": "not-a-jwt"},
                                       content_type="application/json")
        self.assertEqual(ok.status_code, 200, ok.content)
        self.assertTrue(ok.json()["data"]["is_new_user"])
        self.assertEqual(bad.status_code, 401)
        self.assertEqual(garbage.status_code, 401)
        with self.settings(GOOGLE_CLIENT_ID=""):
            off = self.client.post("/api/v1/auth/google", {"credential": "x"}, content_type="application/json")
        self.assertEqual(off.status_code, 501)

    # ------------------------------------------------------------ connect from profile
    def _link(self, user, provider, session, intent_provider=None):
        tokens = sign_tokens(str(user.id), user.email, "free")
        resp = self.client.post(f"/api/v1/auth/oauth/{intent_provider or provider}/link", **self.auth(tokens["access_token"]))
        self.assertEqual(resp.status_code, 200, resp.content)
        link = resp.json()["data"]
        start = self.client.post(f"/api/v1/auth/oauth/{provider}/start",
                                 {"intent": link["intent"], "next": "/dashboard/settings"}, secure=True,
                                 HTTP_ORIGIN=APP, HTTP_SEC_FETCH_SITE="same-origin")
        self.assertEqual(start.status_code, 302)
        params = query(start["Location"])
        if "error" in params:
            return params
        if provider == "google":
            session = google_session(nonce=params["nonce"])
        _, result = self.callback(provider, session, {"code": "c", "state": params["state"]})
        return result

    def test_connecting_github_from_the_profile(self):
        user = self.make_user("someone@example.com")
        result = self._link(user, "github", github_session(emails=[{"email": "different@example.com",
                                                                     "primary": True, "verified": True}]))
        self.assertEqual(result["linked"], "github")
        self.assertEqual(result["next"], "/dashboard/settings")
        self.assertTrue(OAuthAccount.objects.filter(user=user, provider="github").exists())
        self.assertEqual(User.objects.count(), 1)

    def test_link_intent_is_bound_to_its_provider_and_cannot_be_forged(self):
        user = self.make_user("someone@example.com")
        result = self._link(user, "github", github_session(), intent_provider="google")
        self.assertEqual(result["error"], "invalid_state")
        forged = self.client.post("/api/v1/auth/oauth/github/start", {"intent": "forged"}, HTTP_ORIGIN=APP)
        self.assertEqual(query(forged["Location"])["error"], "invalid_state")
        self.assertFalse(OAuthAccount.objects.exists())

    def test_link_start_must_be_posted_from_our_own_site(self):
        """Another site auto-submitting its own valid intent must not get a state cookie."""
        attacker = self.make_user("attacker@example.com")
        token = sign_tokens(str(attacker.id), attacker.email, "free")["access_token"]
        intent = self.client.post("/api/v1/auth/oauth/github/link", **self.auth(token)).json()["data"]["intent"]
        attempts = {
            "cross-site origin": {"HTTP_ORIGIN": "https://evil.example", "HTTP_SEC_FETCH_SITE": "cross-site"},
            "origin only": {"HTTP_ORIGIN": "https://evil.example"},
            "fetch metadata only": {"HTTP_SEC_FETCH_SITE": "cross-site"},
            "no headers at all": {},
        }
        for label, headers in attempts.items():
            resp = self.client.post("/api/v1/auth/oauth/github/start", {"intent": intent}, **headers)
            self.assertEqual(query(resp["Location"]).get("error"), "invalid_state", label)
            self.assertNotIn(oauth.STATE_COOKIE, {k for k, v in resp.cookies.items() if v.value}, label)
        ok = self.client.post("/api/v1/auth/oauth/github/start", {"intent": intent},
                              HTTP_ORIGIN=APP, HTTP_SEC_FETCH_SITE="same-origin")
        self.assertTrue(ok["Location"].startswith("https://github.com/login/oauth/authorize"))

    def test_link_requires_a_signed_in_user(self):
        self.assertEqual(self.client.post("/api/v1/auth/oauth/github/link").status_code, 401)

    def test_identity_already_used_by_another_account(self):
        self.github_sign_in()
        other = self.make_user("other@example.com")
        result = self._link(other, "github", github_session())
        self.assertEqual(result["error"], "account_in_use")

    # ------------------------------------------------------------ password + unlink
    def test_oauth_only_user_must_set_a_password_before_unlinking_last_method(self):
        _, result, _ = self.github_sign_in()
        token = self.exchange(result["code"]).json()["data"]["access_token"]

        blocked = self.client.delete("/api/v1/auth/oauth/accounts/github", **self.auth(token))
        self.assertEqual(blocked.status_code, 409)
        self.assertEqual(blocked.json()["code"], "last_login_method")

        set_pw = self.client.post("/api/v1/auth/change-password", {"new_password": STRONG_PASSWORD},
                                  content_type="application/json", **self.auth(token))
        self.assertEqual(set_pw.status_code, 200, set_pw.content)
        token = set_pw.json()["data"]["access_token"]

        accounts = self.client.get("/api/v1/auth/oauth/accounts", **self.auth(token)).json()["data"]
        self.assertTrue(accounts["has_password"])
        self.assertEqual([a["provider"] for a in accounts["accounts"]], ["github"])

        removed = self.client.delete("/api/v1/auth/oauth/accounts/github", **self.auth(token))
        self.assertEqual(removed.status_code, 200)
        self.assertEqual(removed.json()["data"]["accounts"], [])
        login = self.client.post("/api/v1/auth/login", {"email": "octo@example.com", "password": STRONG_PASSWORD},
                                 content_type="application/json")
        self.assertEqual(login.status_code, 200)

    def test_password_users_still_need_their_current_password(self):
        user = self.make_user("pw@example.com")
        token = sign_tokens(str(user.id), user.email, "free")["access_token"]
        resp = self.client.post("/api/v1/auth/change-password", {"new_password": "An0ther!Passw0rd"},
                                content_type="application/json", **self.auth(token))
        self.assertEqual(resp.status_code, 400)
