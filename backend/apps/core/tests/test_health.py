"""Unit and integration tests for health endpoints."""

import json
from unittest.mock import patch

from django.test import RequestFactory, SimpleTestCase

from apps.core.views.health import health_alias_view, healthz_view, readyz_view


class HealthEndpointsTest(SimpleTestCase):
    def setUp(self):
        self.factory = RequestFactory()

    def _get_json(self, response):
        return json.loads(response.content.decode("utf-8"))

    def test_healthz_returns_ok(self):
        request = self.factory.get("/healthz")
        response = healthz_view(request)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(self._get_json(response), {"status": "ok"})

    @patch("apps.core.views.health.check_redis")
    @patch("apps.core.views.health.check_database")
    def test_readyz_healthy(self, mock_db, mock_redis):
        mock_db.return_value = (True, None)
        mock_redis.return_value = (True, None)

        request = self.factory.get("/readyz")
        response = readyz_view(request)
        self.assertEqual(response.status_code, 200)
        data = self._get_json(response)
        self.assertEqual(data["status"], "ready")
        self.assertEqual(data["database"], "ok")
        self.assertEqual(data["redis"], "ok")

    @patch("apps.core.views.health.check_redis")
    @patch("apps.core.views.health.check_database")
    def test_readyz_db_failure(self, mock_db, mock_redis):
        mock_db.return_value = (False, "DB connection refused")
        mock_redis.return_value = (True, None)

        request = self.factory.get("/readyz")
        response = readyz_view(request)
        self.assertEqual(response.status_code, 503)
        data = self._get_json(response)
        self.assertEqual(data["status"], "unavailable")
        self.assertIn("DB connection refused", data["database"])

    @patch("apps.core.views.health.check_redis")
    @patch("apps.core.views.health.check_database")
    def test_readyz_redis_failure(self, mock_db, mock_redis):
        mock_db.return_value = (True, None)
        mock_redis.return_value = (False, "Redis connection refused")

        request = self.factory.get("/readyz")
        response = readyz_view(request)
        self.assertEqual(response.status_code, 503)
        data = self._get_json(response)
        self.assertEqual(data["status"], "unavailable")
        self.assertIn("Redis connection refused", data["redis"])

    @patch("apps.core.views.health.check_redis")
    @patch("apps.core.views.health.check_database")
    def test_health_alias(self, mock_db, mock_redis):
        mock_db.return_value = (True, None)
        mock_redis.return_value = (True, None)

        request = self.factory.get("/health")
        response = health_alias_view(request)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(self._get_json(response)["status"], "ready")
