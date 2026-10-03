"""Tests for feature flags evaluation and org-level overrides.

Ported from reference/go-v2/internal/httpapi/enterprise_test.go::TestEnvelopeAndFlags.
"""

from uuid import uuid4

import pytest

from apps.core.flags import clear_cache, enabled, set_flag
from apps.orgs.models import Organization


@pytest.mark.django_db
def test_envelope_and_flags_feature_flags() -> None:
    """Port of TestEnvelopeAndFlags (feature flag portion)."""
    clear_cache()
    o1 = Organization.objects.create(name="Org 1", slug=f"org-flags-1-{uuid4().hex[:6]}")
    o2 = Organization.objects.create(name="Org 2", slug=f"org-flags-2-{uuid4().hex[:6]}")

    flag_key = f"test.flag.{uuid4().hex[:8]}"

    # Global default: False
    set_flag(flag_key, org_id=None, flag_enabled=False)
    # Org 1 override: True
    set_flag(flag_key, org_id=o1.id, flag_enabled=True)

    # o1 has flag True, o2 has global default False
    assert enabled(flag_key, o1.id) is True
    assert enabled(flag_key, o2.id) is False

    # Unknown flag defaults to False
    assert enabled("completely.unknown.flag", o1.id) is False
