"""Tests for envelope encryption, key rotation, platform secrets, and blind indexes.

Ported from reference/go-v2/internal/httpapi/enterprise_test.go::TestEnvelopeAndFlags.
"""

from uuid import uuid4

import pytest

from apps.core.crypto import Keyring
from apps.orgs.models import Organization


@pytest.mark.django_db
def test_envelope_and_flags_encryption() -> None:
    """Port of TestEnvelopeAndFlags (encryption & blind index portions)."""
    o1 = Organization.objects.create(name="Org 1", slug=f"org-1-{uuid4().hex[:6]}")
    o2 = Organization.objects.create(name="Org 2", slug=f"org-2-{uuid4().hex[:6]}")

    master_key = b"12345678901234567890123456789012"
    keyring = Keyring(master_key=master_key)

    plaintext = b"asha@example.com"

    # 1. Encrypt and decrypt roundtrip
    ct = keyring.encrypt(o1.id, plaintext)
    assert keyring.decrypt(o1.id, ct) == plaintext

    # 2. Reject foreign organization decryption
    with pytest.raises(ValueError, match="foreign ciphertext"):
        keyring.decrypt(o2.id, ct)

    # 3. Rotate key
    keyring.rotate(o1.id)
    ct2 = keyring.encrypt(o1.id, b"x")

    # Key ID changed
    assert ct2[:4] != ct[:4]

    # 4. Old ciphertext still decrypts after rotation
    assert keyring.decrypt(o1.id, ct) == plaintext

    # 5. Blind index normalization and org isolation
    bidx1 = keyring.blind_index(o1.id, " Asha@Example.com ")
    bidx2 = keyring.blind_index(o1.id, "asha@example.com")
    bidx_other_org = keyring.blind_index(o2.id, "asha@example.com")

    assert bidx1 == bidx2
    assert bidx1 != bidx_other_org

    # 6. Platform secret seal and open
    plat_ct = keyring.seal_platform(b"secret-totp-seed", "totp")
    assert keyring.open_platform(plat_ct, "totp") == b"secret-totp-seed"

    # Wrong label fails
    with pytest.raises(ValueError):
        keyring.open_platform(plat_ct, "other_label")
