"""Settings and middleware behaviour that decides whether a Railway deploy works."""

import json
import os
import subprocess
import sys
from pathlib import Path

from django.test import Client, SimpleTestCase, TestCase, override_settings

import qrapp.settings as project_settings

BACKEND_DIR = Path(__file__).resolve().parents[2]

_PRINT_SETTINGS = (
    "import json, qrapp.settings as s; print(json.dumps({"
    "'hosts': s.ALLOWED_HOSTS, 'cors': s.CORS_ALLOWED_ORIGINS, 'csrf': s.CSRF_TRUSTED_ORIGINS,"
    "'callback': s.OAUTH_CALLBACK_BASE_URL, 'exempt': s.SECURE_REDIRECT_EXEMPT,"
    "'ssl_redirect': getattr(s, 'SECURE_SSL_REDIRECT', False),"
    "'email': [s.EMAIL_BACKEND, s.QRIT_EMAIL_BACKEND]}))"
)


def load_settings(**env):
    """Import the settings module in a clean process with the given env."""
    clean = {k: v for k, v in os.environ.items() if not k.startswith(("RAILWAY_", "OAUTH_", "APP_BASE_URL",
                                                                       "ALLOWED_HOSTS", "CORS_", "CSRF_"))}
    clean.update({"DJANGO_SECRET_KEY": "x", "JWT_SECRET": "j" * 64, "DEBUG": "False"}, **env)
    out = subprocess.run([sys.executable, "-c", _PRINT_SETTINGS], cwd=BACKEND_DIR, env=clean,
                         capture_output=True, text=True, check=True)
    return json.loads(out.stdout.strip().splitlines()[-1])


class RailwaySettingsTests(SimpleTestCase):
    def test_railway_hosts_are_always_allowed_when_hosts_are_restricted(self):
        s = load_settings(ALLOWED_HOSTS="api.example.com",
                          RAILWAY_PUBLIC_DOMAIN="qr-backend-production.up.railway.app",
                          RAILWAY_PRIVATE_DOMAIN="qr-backend.railway.internal")
        for host in ("api.example.com", "healthcheck.railway.app", "qr-backend-production.up.railway.app",
                     "qr-backend.railway.internal", "localhost"):
            self.assertIn(host, s["hosts"])

    def test_wildcard_hosts_are_left_alone(self):
        self.assertEqual(load_settings()["hosts"], ["*"])

    def test_frontend_origin_is_allowed_for_cors_and_csrf(self):
        s = load_settings(APP_BASE_URL="https://qr-frontend-production.up.railway.app/")
        self.assertIn("https://qr-frontend-production.up.railway.app", s["cors"])
        self.assertIn("https://qr-frontend-production.up.railway.app", s["csrf"])

    def test_oauth_callback_base_defaults_to_the_frontend_and_can_be_overridden(self):
        self.assertEqual(load_settings(APP_BASE_URL="https://app.example.com/")["callback"], "https://app.example.com")
        s = load_settings(APP_BASE_URL="https://app.example.com", OAUTH_CALLBACK_BASE_URL="https://api.example.com/")
        self.assertEqual(s["callback"], "https://api.example.com")

    def test_app_email_switch_does_not_clobber_djangos_email_backend(self):
        s = load_settings(EMAIL_BACKEND="resend")
        self.assertEqual(s["email"], ["django.core.mail.backends.console.EmailBackend", "resend"])

    def test_production_redirects_to_https_but_exempts_the_health_check(self):
        s = load_settings()
        self.assertTrue(s["ssl_redirect"])
        self.assertIn(r"^health/?$", s["exempt"])


class RailwayRequestTests(TestCase):
    @override_settings(SECURE_SSL_REDIRECT=True, ALLOWED_HOSTS=["healthcheck.railway.app", "api.example.com"])
    def test_plain_http_health_check_is_served_not_redirected(self):
        client = Client()  # fresh handler so SecurityMiddleware reads the overridden settings
        health = client.get("/health", HTTP_HOST="healthcheck.railway.app")
        self.assertEqual(health.status_code, 200)
        self.assertEqual(health.json()["status"], "ok")
        api = client.get("/api/v1/auth/oauth/providers", HTTP_HOST="api.example.com")
        self.assertEqual(api.status_code, 301)
        self.assertTrue(api["Location"].startswith("https://"))
        proxied = client.get("/api/v1/auth/oauth/providers", HTTP_HOST="api.example.com",
                             HTTP_X_FORWARDED_PROTO="https")
        self.assertEqual(proxied.status_code, 200)

    def test_disallowed_host_is_a_clean_400_not_a_crash(self):
        # Django mails admins about DisallowedHost when DEBUG is off; that
        # handler used to blow up on EMAIL_BACKEND="console" and return 500.
        # The test runner swaps in its own mail backend, so pin the project's.
        with override_settings(ALLOWED_HOSTS=["api.example.com"], DEBUG=False,
                               EMAIL_BACKEND=project_settings.EMAIL_BACKEND):
            resp = Client().get("/health", HTTP_HOST="evil.example")
        self.assertEqual(resp.status_code, 400)

    @override_settings(CORS_ALLOWED_ORIGINS=["https://app.example.com"])
    def test_cors_preflight_from_the_frontend_is_answered(self):
        resp = Client().options(
            "/api/v1/auth/me",
            HTTP_ORIGIN="https://app.example.com",
            HTTP_ACCESS_CONTROL_REQUEST_METHOD="GET",
            HTTP_ACCESS_CONTROL_REQUEST_HEADERS="authorization",
        )
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp["Access-Control-Allow-Origin"], "https://app.example.com")
