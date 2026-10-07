"""Every setting the code reads through django.conf.settings must exist.

`getattr(settings, "NAME", default)` silently returns the default when NAME was
never defined in qrit.settings. That hid several production bugs at once:
APP_ENV always read "local" (fake OAuth credentials accepted, cookies never
Secure), each worker generated its own JWT signing key, and rate limiting
never reached Redis.
"""

import re
from pathlib import Path

import pytest
from django.conf import settings
from django.core.exceptions import ImproperlyConfigured

from apps.accounts.tokens import TokenManager

BACKEND = Path(__file__).resolve().parent.parent
LOOKUP = re.compile(r"""getattr\(\s*settings\s*,\s*["']([A-Z][A-Z0-9_]*)["']""")


def settings_read_by_the_code() -> dict[str, list[str]]:
    found: dict[str, list[str]] = {}
    for path in sorted((BACKEND / "apps").rglob("*.py")):
        if "tests" in path.parts or "migrations" in path.parts:
            continue
        for name in LOOKUP.findall(path.read_text()):
            found.setdefault(name, []).append(str(path.relative_to(BACKEND)))
    return found


def test_every_setting_read_through_getattr_is_defined() -> None:
    lookups = settings_read_by_the_code()
    assert lookups, "the scan found no getattr(settings, ...) calls; is the pattern stale?"
    missing = {name: files for name, files in lookups.items() if not hasattr(settings, name)}
    assert not missing, f"read via getattr(settings, ...) but never defined: {missing}"


def test_tokens_from_one_worker_verify_on_another() -> None:
    """Two processes built from the same settings must share the signing key."""
    worker_a, worker_b = TokenManager(), TokenManager()
    token = worker_a.create_access_token(
        user_id="01a113aa-ab33-71ca-bfb8-e386b48aa1f4",
        session_id="01a113aa-ac49-7291-989f-1a660b948365",
    )
    assert worker_b.verify_access_token(token)["sub"] == "01a113aa-ab33-71ca-bfb8-e386b48aa1f4"


@pytest.mark.parametrize("raw", ["", "not-a-key"])
def test_production_refuses_to_invent_a_signing_key(settings, raw: str) -> None:  # type: ignore[no-untyped-def]
    settings.APP_ENV = "production"
    settings.JWT_ED25519_PRIVATE_KEY = raw
    with pytest.raises(ImproperlyConfigured):
        TokenManager()
