"""OpenAPI description of the accounts authentication scheme (drf-spectacular)."""

from typing import Any

from drf_spectacular.extensions import OpenApiAuthenticationExtension


# drf-spectacular registers extensions in an untyped __init_subclass__.
class SessionAuthenticationScheme(OpenApiAuthenticationExtension):  # type: ignore[no-untyped-call]
    """`SessionAuthentication` accepts the access JWT as a Bearer token or in the
    `qrit_access` cookie (cookie callers must also send `X-CSRF-Token`)."""

    target_class = "apps.accounts.authentication.SessionAuthentication"
    name = "bearerAuth"

    def get_security_definition(self, auto_schema: Any) -> dict[str, Any]:
        return {
            "type": "http",
            "scheme": "bearer",
            "bearerFormat": "JWT",
            "description": (
                "Ed25519 access JWT in `Authorization: Bearer`, or the `qrit_access` "
                "cookie with a matching `X-CSRF-Token` header on unsafe methods."
            ),
        }
