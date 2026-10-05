"""Tests for Idempotency-Key request deduplication and replay.

Ported from reference/go-v2/internal/idempotency/idempotency_test.go:
- TestIdempotencyKeyLength -> test_idempotency_key_length
- TestSafeMethodsBypass -> test_safe_methods_bypass
"""

import json
from typing import Any
from uuid import uuid4

import pytest
from django.http import HttpResponse, JsonResponse
from django.test import RequestFactory

from apps.core.idempotency import IdempotencyMiddleware


def test_idempotency_key_length() -> None:
    """Port of TestIdempotencyKeyLength: key > 128 chars rejected with 400."""
    rf = RequestFactory()
    long_key = "a" * 129
    req = rf.post(
        "/v1/test",
        data=json.dumps({"data": 1}),
        content_type="application/json",
        HTTP_IDEMPOTENCY_KEY=long_key,
    )

    middleware = IdempotencyMiddleware(lambda r: HttpResponse(status=200))
    resp = middleware(req)

    assert resp.status_code == 400
    data = json.loads(resp.content)
    assert data["code"] == "invalid_idempotency_key"


def test_safe_methods_bypass() -> None:
    """Port of TestSafeMethodsBypass: safe methods bypass idempotency checks."""
    rf = RequestFactory()
    methods = ["get", "head", "options", "delete"]

    for m in methods:
        factory_method = getattr(rf, m)
        req = factory_method("/v1/test", HTTP_IDEMPOTENCY_KEY="short")  # Invalid key length (< 8)
        middleware = IdempotencyMiddleware(lambda r: HttpResponse(status=200))
        resp = middleware(req)
        assert resp.status_code == 200, f"method {m} should bypass idempotency"


@pytest.mark.django_db
def test_idempotency_replay_and_conflict() -> None:
    """Verify replay returns cached response and conflicting body is rejected."""
    ws_id = uuid4()
    rf = RequestFactory()

    call_count = 0

    def test_view(request: Any) -> HttpResponse:
        nonlocal call_count
        call_count += 1
        return JsonResponse({"message": "success", "call_count": call_count}, status=201)

    middleware = IdempotencyMiddleware(test_view)

    # First call
    req1 = rf.post(
        "/v1/workspaces/codes",
        data=json.dumps({"name": "Test Code"}),
        content_type="application/json",
        HTTP_IDEMPOTENCY_KEY="my-idempotency-key-12345",
    )
    req1.workspace_id = ws_id
    resp1 = middleware(req1)

    assert resp1.status_code == 201
    assert call_count == 1
    data1 = json.loads(resp1.content)
    assert data1["call_count"] == 1
    assert resp1.get("Idempotent-Replayed") is None

    # Replay call with exact same body and path
    req2 = rf.post(
        "/v1/workspaces/codes",
        data=json.dumps({"name": "Test Code"}),
        content_type="application/json",
        HTTP_IDEMPOTENCY_KEY="my-idempotency-key-12345",
    )
    req2.workspace_id = ws_id
    resp2 = middleware(req2)

    assert resp2.status_code == 201
    assert call_count == 1  # Not executed again!
    data2 = json.loads(resp2.content)
    assert data2["call_count"] == 1
    assert resp2.get("Idempotent-Replayed") == "true"

    # Conflicting call with same key but different body -> 422
    req3 = rf.post(
        "/v1/workspaces/codes",
        data=json.dumps({"name": "Different Name"}),
        content_type="application/json",
        HTTP_IDEMPOTENCY_KEY="my-idempotency-key-12345",
    )
    req3.workspace_id = ws_id
    resp3 = middleware(req3)

    assert resp3.status_code == 422
    data3 = json.loads(resp3.content)
    assert data3["code"] == "idempotency_key_reused"
