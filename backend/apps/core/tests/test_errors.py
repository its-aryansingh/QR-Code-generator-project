"""Unit tests for RFC 7807 problem+json error formatting and exception handler."""

import json

from django.core.exceptions import PermissionDenied
from django.http import Http404
from django.test import RequestFactory, SimpleTestCase
from rest_framework.exceptions import NotAuthenticated, Throttled, ValidationError

from apps.core.errors import (
    CODES,
    conflict,
    exception_handler,
    forbidden,
    not_found,
    payment_required,
    problem_json_response,
    rate_limited,
    unprocessable,
)


class ErrorHandlingTest(SimpleTestCase):
    def setUp(self):
        self.factory = RequestFactory()

    def _get_json(self, response):
        return json.loads(response.content.decode("utf-8"))

    def test_problem_json_response_headers_and_shape(self):
        resp = problem_json_response(
            status=404,
            code="not_found",
            detail="The requested item was not found.",
            instance="/v1/qr-codes/123",
        )
        self.assertEqual(resp.status_code, 404)
        self.assertEqual(resp["Content-Type"], "application/problem+json")
        data = self._get_json(resp)
        self.assertEqual(data["type"], "urn:qrit:error:not_found")
        self.assertEqual(data["title"], "Not Found")
        self.assertEqual(data["status"], 404)
        self.assertEqual(data["code"], "not_found")
        self.assertEqual(data["detail"], "The requested item was not found.")
        self.assertEqual(data["instance"], "/v1/qr-codes/123")

    def test_api_error_factories(self):
        err = not_found("QR code not found")
        self.assertEqual(err.status, 404)
        self.assertEqual(err.code, "not_found")

        err = forbidden("ip_not_allowed", "IP not in allowlist")
        self.assertEqual(err.status, 403)
        self.assertEqual(err.code, "ip_not_allowed")

        err = unprocessable("invalid_rules", "Rules syntax error", errors={"rules": "Invalid"})
        self.assertEqual(err.status, 422)
        self.assertEqual(err.code, "invalid_rules")
        self.assertEqual(err.errors, {"rules": "Invalid"})

        err = payment_required(required_plan="business")
        self.assertEqual(err.status, 402)
        self.assertEqual(err.extra["required_plan"], "business")

        err = conflict("slug_exists", "Slug already in use")
        self.assertEqual(err.status, 409)
        self.assertEqual(err.code, "slug_exists")

        err = rate_limited(retry_after=30)
        self.assertEqual(err.status, 429)
        self.assertEqual(err.code, "rate_limited")
        self.assertEqual(err.extra["retry_after"], 30)

    def test_exception_handler_with_api_error(self):
        req = self.factory.post("/v1/qr-codes")
        err = unprocessable(errors={"name": ["This field is required."]})
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 422)
        self.assertEqual(resp["Content-Type"], "application/problem+json")
        data = self._get_json(resp)
        self.assertEqual(data["code"], "unprocessable")
        self.assertEqual(data["errors"], {"name": ["This field is required."]})

    def test_exception_handler_with_validation_error(self):
        req = self.factory.post("/v1/qr-codes")
        err = ValidationError({"destination_url": ["Enter a valid URL."]})
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 422)
        data = self._get_json(resp)
        self.assertEqual(data["code"], "validation_failed")
        self.assertEqual(data["errors"], {"destination_url": ["Enter a valid URL."]})

    def test_exception_handler_with_http404(self):
        req = self.factory.get("/v1/workspaces/ws-1")
        err = Http404("Workspace does not exist")
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 404)
        data = self._get_json(resp)
        self.assertEqual(data["code"], "not_found")
        self.assertEqual(data["detail"], "Workspace does not exist")

    def test_exception_handler_with_permission_denied(self):
        req = self.factory.delete("/v1/orgs/org-1")
        err = PermissionDenied("Owner permissions required")
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 403)
        data = self._get_json(resp)
        self.assertEqual(data["code"], "forbidden")

    def test_exception_handler_with_not_authenticated(self):
        req = self.factory.get("/v1/user")
        err = NotAuthenticated("Authentication credentials were not provided.")
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 401)
        data = self._get_json(resp)
        self.assertEqual(data["code"], "unauthorized")

    def test_exception_handler_with_throttled(self):
        req = self.factory.get("/v1/codes")
        err = Throttled(wait=45)
        resp = exception_handler(err, {"request": req})
        assert resp is not None
        self.assertEqual(resp.status_code, 429)
        self.assertEqual(resp["Retry-After"], "45")
        data = self._get_json(resp)
        self.assertEqual(data["code"], "rate_limited")

    def test_stable_codes_catalog(self):
        for code in [
            "allowlist_required",
            "billing_hold",
            "destination_unsafe",
            "idempotency_key_reused",
            "mfa_required",
            "rate_limited",
            "seat_limit_reached",
            "sso_enforced",
            "step_up_required",
        ]:
            self.assertIn(code, CODES)
