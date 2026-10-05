"""Password validation matching Go v2 auth and enterprise policies."""

from apps.core.errors import unprocessable

COMMON_PASSWORDS = {
    "1234567890",
    "password123",
    "qwertyuiop",
    "12345678901",
    "administrator",
    "changeme123",
    "welcome1234",
    "iloveyou123",
    "password1234",
    "123456789012",
}


def validate_password_strength(password: str) -> None:
    """Validate password length and check against common passwords denylist."""
    if len(password) < 10:
        raise unprocessable(
            code="weak_password",
            detail="password must be at least 10 characters long",
        )

    lower = password.strip().lower()
    if lower in COMMON_PASSWORDS:
        raise unprocessable(
            code="weak_password",
            detail="password is too common or easily guessed",
        )
