"""Standardized error responses adhering to RFC 7807 (application/problem+json).

Plan §5.3, §7.1 and Appendix B define the error codes and response structure.
"""

from typing import Any

from django.core.exceptions import PermissionDenied
from django.http import Http404, JsonResponse
from rest_framework.exceptions import (
    APIException,
    AuthenticationFailed,
    NotAuthenticated,
    Throttled,
    ValidationError,
)
from rest_framework.views import exception_handler as drf_default_exception_handler

# Stable error codes catalog (Plan Appendix B)
CODES = {
    "allowlist_required",
    "already_decided",
    "already_member",
    "already_owner",
    "api_key_not_allowed",
    "api_keys_cannot_approve",
    "approval_not_pending",
    "approval_requires_user",
    "billing_hold",
    "binding_exists",
    "break_glass_required",
    "code_blocked",
    "comment_required",
    "conflict",
    "design_required",
    "destination_required",
    "destination_unsafe",
    "discovery_failed",
    "domain_claimed",
    "domain_exists",
    "domain_unverified",
    "duplicate_rule_id",
    "dynamic_only_type",
    "email_exists",
    "email_mismatch",
    "email_required",
    "empty_body",
    "folder_cycle",
    "folder_exists",
    "folder_not_empty",
    "forbidden",
    "granularity_too_fine",
    "group_exists",
    "group_managed_externally",
    "gtin_exists",
    "hosted_page_required",
    "hosted_page_too_large",
    "idempotency_key_reused",
    "internal_error",
    "invalid_credentials",
    "invalid_input",
    "invalid_token",
    "invite_domain_not_allowed",
    "invite_exists",
    "ip_not_allowed",
    "limit_reached",
    "metadata_required",
    "mfa_code_required",
    "mfa_not_enabled",
    "mfa_required",
    "mfa_required_by_org",
    "mfa_verification_required",
    "not_a_member",
    "not_found",
    "not_org_member",
    "not_scheduled",
    "oidc_fields_required",
    "org_access_suspended",
    "owner_cannot_leave",
    "owner_immutable",
    "owner_required",
    "payload_too_large",
    "payment_required",
    "permissions_required",
    "privilege_escalation",
    "public_domain",
    "rate_limited",
    "read_only_over_limit",
    "reason_required",
    "request_in_flight",
    "reserved_key",
    "role_exists",
    "role_in_use",
    "role_required",
    "saml_metadata_rejected",
    "seat_limit_reached",
    "secret_required",
    "self_approval",
    "self_change",
    "self_role_change",
    "session_expired",
    "session_required",
    "slug_exists",
    "sso_account_exists",
    "sso_enforced",
    "sso_not_ready",
    "sso_reauth_required",
    "sso_required",
    "sso_user_not_provisioned",
    "static_content_required",
    "static_immutable",
    "static_no_lifecycle",
    "static_only_type",
    "step_up_required",
    "system_role",
    "tag_exists",
    "template_locked",
    "template_required",
    "test_required",
    "too_deep",
    "too_many_break_glass",
    "too_many_codes",
    "too_many_hosts",
    "too_many_ranges",
    "too_many_rules",
    "too_many_streams",
    "txt_record_not_found",
    "unauthorized",
    "unknown_permission",
    "unprocessable",
    "upgrade_required",
    "use_membership",
    "validation_failed",
    "weak_password",
    "wildcard_not_allowed",
    "workspace_required",
    "would_lock_out",
}


class ApiError(Exception):
    """Domain exception representing an API error mapped to problem+json."""

    def __init__(
        self,
        status: int,
        code: str,
        detail: str,
        *,
        instance: str | None = None,
        errors: Any | None = None,
        extra: dict[str, Any] | None = None,
    ):
        super().__init__(detail)
        self.status = status
        self.code = code
        self.detail = detail
        self.instance = instance
        self.errors = errors
        self.extra = extra or {}


# Convenience factory helpers per plan §5.3
def not_found(detail: str = "Resource not found", code: str = "not_found") -> ApiError:
    return ApiError(status=404, code=code, detail=detail)


def forbidden(code: str = "forbidden", detail: str = "Access denied") -> ApiError:
    return ApiError(status=403, code=code, detail=detail)


def unprocessable(
    code: str = "unprocessable",
    detail: str = "Validation failed",
    errors: Any | None = None,
) -> ApiError:
    return ApiError(status=422, code=code, detail=detail, errors=errors)


def payment_required(
    code: str = "upgrade_required",
    detail: str = "Feature requires a plan upgrade",
    required_plan: str = "pro",
) -> ApiError:
    return ApiError(status=402, code=code, detail=detail, extra={"required_plan": required_plan})


def conflict(code: str = "conflict", detail: str = "Conflict with existing state") -> ApiError:
    return ApiError(status=409, code=code, detail=detail)


def rate_limited(retry_after: int = 60, detail: str = "Rate limit exceeded") -> ApiError:
    return ApiError(
        status=429, code="rate_limited", detail=detail, extra={"retry_after": retry_after}
    )


def problem_json_response(
    status: int,
    code: str,
    detail: str,
    instance: str | None = None,
    errors: Any | None = None,
    extra: dict[str, Any] | None = None,
) -> JsonResponse:
    """Format and return an RFC 7807 problem+json response."""
    payload: dict[str, Any] = {
        "type": f"urn:qrit:error:{code}",
        "title": code.replace("_", " ").title(),
        "status": status,
        "detail": detail,
        "code": code,
    }
    if instance:
        payload["instance"] = instance
    if errors is not None:
        payload["errors"] = errors
    if extra:
        payload.update(extra)

    response = JsonResponse(payload, status=status)
    response["Content-Type"] = "application/problem+json"
    if extra and "retry_after" in extra:
        response["Retry-After"] = str(extra["retry_after"])
    return response


def exception_handler(exc: Exception, context: dict[str, Any]) -> JsonResponse | None:
    """DRF exception handler converting all API errors to RFC 7807 problem+json."""
    request = context.get("request")
    instance = request.path if request else None

    if isinstance(exc, ApiError):
        return problem_json_response(
            status=exc.status,
            code=exc.code,
            detail=exc.detail,
            instance=exc.instance or instance,
            errors=exc.errors,
            extra=exc.extra,
        )

    if isinstance(exc, ValidationError):
        return problem_json_response(
            status=422,
            code="validation_failed",
            detail="The request failed validation checks.",
            instance=instance,
            errors=exc.detail,
        )

    if isinstance(exc, Http404):
        return problem_json_response(
            status=404,
            code="not_found",
            detail=str(exc) or "Resource not found",
            instance=instance,
        )

    if isinstance(exc, (PermissionDenied,)):
        return problem_json_response(
            status=403,
            code="forbidden",
            detail=str(exc) or "Permission denied",
            instance=instance,
        )

    if isinstance(exc, (NotAuthenticated, AuthenticationFailed)):
        return problem_json_response(
            status=401,
            code="unauthorized",
            detail=str(exc.detail)
            if hasattr(exc, "detail")
            else "Authentication credentials were not provided or are invalid.",
            instance=instance,
        )

    if isinstance(exc, Throttled):
        wait_val = getattr(exc, "wait", None)
        retry_secs = int(wait_val) if wait_val is not None else 60
        return problem_json_response(
            status=429,
            code="rate_limited",
            detail=f"Request was throttled. Expected available in {retry_secs} seconds.",
            instance=instance,
            extra={"retry_after": retry_secs},
        )

    if isinstance(exc, APIException):
        code = getattr(exc, "default_code", "api_error")
        return problem_json_response(
            status=exc.status_code,
            code=code,
            detail=str(exc.detail),
            instance=instance,
        )

    # Let unhandled exceptions fall through or return a 500 problem
    drf_response = drf_default_exception_handler(exc, context)
    if drf_response is not None:
        return problem_json_response(
            status=drf_response.status_code,
            code="api_error",
            detail=str(drf_response.data),
            instance=instance,
        )

    return None
