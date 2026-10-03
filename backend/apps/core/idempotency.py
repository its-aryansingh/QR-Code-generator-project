"""Idempotency-Key request deduplication and response replay.

Plan §7.1:
- Header `Idempotency-Key` (8–128 characters, else 400 `invalid_idempotency_key`).
- Safe methods (`GET`, `HEAD`, `OPTIONS`, `DELETE`) bypass.
- Scoped to `(workspace_id, principal, method, path)`.
- Reused key with different request body/path -> 422 `idempotency_key_reused`.
- In-flight request with same key -> 409 `request_in_flight`.
- Status >= 500 or 429 are purged so the client can retry.
- Replayed response returns original status, body, and header `Idempotent-Replayed: true`.
- TTL: 24 hours.
"""

import hashlib
import json
from collections.abc import Callable
from datetime import timedelta
from typing import Any
from uuid import UUID

from django.db import IntegrityError, transaction
from django.http import HttpRequest, HttpResponse, JsonResponse
from django.utils import timezone

HEADER_IDEMPOTENCY_KEY = "Idempotency-Key"
HEADER_REPLAYED = "Idempotent-Replayed"
KEY_RETENTION = timedelta(hours=24)


def get_principal_id(request: HttpRequest) -> str:
    """Extract a stable string identifier for the calling principal."""
    user = getattr(request, "user", None)
    if user and user.is_authenticated:
        return str(user.id)

    api_key_id = getattr(request, "api_key_id", None)
    if api_key_id:
        return f"key:{api_key_id}"

    return "anonymous"


def get_workspace_id(request: HttpRequest) -> UUID | None:
    """Extract the active workspace_id for the request, if any."""
    ws = getattr(request, "workspace", None)
    if ws is not None and hasattr(ws, "id"):
        return ws.id  # type: ignore[no-any-return]
    ws_id = getattr(request, "workspace_id", None)
    if ws_id is not None:
        try:
            return UUID(str(ws_id))
        except (ValueError, AttributeError):
            return None
    return None


class IdempotencyMiddleware:
    """Middleware enforcing Idempotency-Key semantics."""

    def __init__(self, get_response: Callable[[HttpRequest], HttpResponse]) -> None:
        self.get_response = get_response

    def __call__(self, request: HttpRequest) -> HttpResponse:
        if request.method in ("GET", "HEAD", "OPTIONS", "DELETE"):
            return self.get_response(request)

        raw_key = request.headers.get(HEADER_IDEMPOTENCY_KEY)
        if raw_key is None:
            raw_key = request.META.get("HTTP_IDEMPOTENCY_KEY")

        if not raw_key:
            return self.get_response(request)

        raw_key = raw_key.strip()
        if len(raw_key) < 8 or len(raw_key) > 128:
            return JsonResponse(
                {
                    "type": "https://qrit.io/errors/invalid_idempotency_key",
                    "title": "Invalid Idempotency-Key",
                    "status": 400,
                    "code": "invalid_idempotency_key",
                    "detail": "Idempotency-Key must be 8–128 characters",
                },
                status=400,
                content_type="application/problem+json",
            )

        ws_id = get_workspace_id(request)
        if ws_id is None:
            # If not in a workspace context, proceed without deduplication
            return self.get_response(request)

        principal = get_principal_id(request)
        key_seed = f"{principal}\x00{raw_key}".encode()
        key_hash = hashlib.sha256(key_seed).hexdigest()

        body_bytes = request.body or b""
        req_hash = hashlib.sha256(body_bytes).digest()

        from apps.core.models import IdempotencyKey

        now = timezone.now()

        existing = IdempotencyKey.objects.filter(workspace_id=ws_id, key=key_hash).first()
        if existing is not None:
            if now > existing.expires_at:
                existing.delete()
                existing = None
            else:
                if (
                    bytes(existing.request_hash) != req_hash
                    or existing.method != request.method
                    or existing.path != request.path
                ):
                    return JsonResponse(
                        {
                            "type": "https://qrit.io/errors/idempotency_key_reused",
                            "title": "Unprocessable Entity",
                            "status": 422,
                            "code": "idempotency_key_reused",
                            "detail": "this Idempotency-Key was already used with a different request",
                        },
                        status=422,
                        content_type="application/problem+json",
                    )

                if existing.status_code is None:
                    return JsonResponse(
                        {
                            "type": "https://qrit.io/errors/request_in_flight",
                            "title": "Conflict",
                            "status": 409,
                            "code": "request_in_flight",
                            "detail": "a request with this Idempotency-Key is still processing",
                        },
                        status=409,
                        content_type="application/problem+json",
                    )

                # Replay original response
                resp_content = ""
                if existing.response_body is not None:
                    if isinstance(existing.response_body, str):
                        resp_content = existing.response_body
                    else:
                        resp_content = json.dumps(existing.response_body)

                content_type = "application/json"
                if existing.status_code >= 400:
                    content_type = "application/problem+json"

                response = HttpResponse(
                    content=resp_content,
                    status=existing.status_code,
                    content_type=content_type,
                )
                response[HEADER_REPLAYED] = "true"
                return response

        # Reserve key
        try:
            with transaction.atomic():
                IdempotencyKey.objects.create(
                    workspace_id=ws_id,
                    key=key_hash,
                    method=request.method or "GET",
                    path=request.path,
                    request_hash=req_hash,
                    status_code=None,
                    response_body=None,
                    created_at=now,
                    expires_at=now + KEY_RETENTION,
                )
        except IntegrityError:
            return JsonResponse(
                {
                    "type": "https://qrit.io/errors/request_in_flight",
                    "title": "Conflict",
                    "status": 409,
                    "code": "request_in_flight",
                    "detail": "a request with this Idempotency-Key is still processing",
                },
                status=409,
                content_type="application/problem+json",
            )

        completed = False
        try:
            response = self.get_response(request)
            completed = True

            if response.status_code >= 500 or response.status_code == 429:
                IdempotencyKey.objects.filter(workspace_id=ws_id, key=key_hash).delete()
            else:
                stored_body: Any = None
                try:
                    stored_body = json.loads(response.content.decode("utf-8"))
                except Exception:
                    stored_body = response.content.decode("utf-8", errors="replace")

                IdempotencyKey.objects.filter(workspace_id=ws_id, key=key_hash).update(
                    status_code=response.status_code,
                    response_body=stored_body,
                )

            return response
        finally:
            if not completed:
                IdempotencyKey.objects.filter(workspace_id=ws_id, key=key_hash).delete()
