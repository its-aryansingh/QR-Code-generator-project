"""Envelope encryption, platform secret sealing, and blind indexing.

Plan §6.3 & §7.1:
- Keyring: per-organisation envelope encryption using AES-256-GCM.
- DEKs stored in `org_data_keys`, wrapped with KEK derived from APP_ENCRYPTION_KEY.
- Ciphertext format: key_id (4 bytes BE) || nonce (12 bytes) || AES-256-GCM ciphertext + tag.
- AAD = org UUID bytes.
- Rotation retires active key and generates new DEK; old ciphertexts still decrypt.
- BlindIndex: HMAC-SHA256 keyed with HKDF-SHA256(APP_ENCRYPTION_KEY, salt=org UUID bytes, info="qrit-bidx-v1").
- SealPlatform / OpenPlatform for platform-level secrets with AAD = "platform:" + label.
"""

import base64
import hmac
import os
import threading
from uuid import UUID

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.kdf.hkdf import HKDF
from django.conf import settings
from django.db import connection, transaction
from django.utils import timezone


def parse_master_key(raw: str | bytes) -> bytes:
    """Parse a 32-byte master encryption key from string (base64, hex, or raw) or bytes."""
    if isinstance(raw, bytes):
        if len(raw) == 32:
            return raw
        raise ValueError(f"Master encryption key bytes must be 32 bytes, got {len(raw)}")

    raw_str = raw.strip()
    if len(raw_str) == 32:
        return raw_str.encode("utf-8")

    # Try base64
    try:
        decoded = base64.b64decode(raw_str)
        if len(decoded) == 32:
            return decoded
    except Exception:
        pass

    # Try hex
    try:
        decoded = bytes.fromhex(raw_str)
        if len(decoded) == 32:
            return decoded
    except Exception:
        pass

    raw_bytes = raw_str.encode("utf-8")
    if len(raw_bytes) == 32:
        return raw_bytes

    raise ValueError("Master encryption key must be 32 bytes (raw, base64, or hex)")


class Keyring:
    """Manages envelope encryption keys and operations."""

    def __init__(self, master_key: bytes | None = None) -> None:
        if master_key is None:
            raw_key = getattr(settings, "APP_ENCRYPTION_KEY", "")
            master_key = parse_master_key(raw_key)
        elif len(master_key) != 32:
            raise ValueError("Master key must be exactly 32 bytes")

        self.master_key = master_key
        # KEK derived with info="qrit/kek/v1"
        self.kek = HKDF(
            algorithm=hashes.SHA256(),
            length=32,
            salt=None,
            info=b"qrit/kek/v1",
        ).derive(self.master_key)

        self._lock = threading.RLock()
        self._deks: dict[str, bytes] = {}  # f"{org_id}:{key_id}" -> dek
        self._active_key_ids: dict[str, int] = {}  # str(org_id) -> key_id

    def _org_info(self, org_id: str | UUID) -> tuple[str, bytes, UUID]:
        if isinstance(org_id, UUID):
            return str(org_id), org_id.bytes, org_id
        u = UUID(str(org_id))
        return str(u), u.bytes, u

    def ensure(self, org_id: str | UUID) -> int:
        """Ensure an active DEK exists for the organisation and return its key_id."""
        org_str, org_bytes, org_uuid = self._org_info(org_id)

        with self._lock:
            if org_str in self._active_key_ids:
                return self._active_key_ids[org_str]

        from apps.core.models import OrgDataKey

        with transaction.atomic():
            # Acquire transaction advisory lock on org
            with connection.cursor() as cursor:
                cursor.execute(
                    "SELECT pg_advisory_xact_lock(hashtextextended('org_dek:' || %s, 0))",
                    [org_str],
                )

            active_row = (
                OrgDataKey.objects.filter(org_id=org_uuid, retired_at__isnull=True)
                .order_by("-key_id")
                .first()
            )
            if active_row is not None:
                kid = active_row.key_id
                # Unwrap DEK
                dek = self._unwrap(active_row.dek_ct, org_bytes)
                with self._lock:
                    self._active_key_ids[org_str] = kid
                    self._deks[f"{org_str}:{kid}"] = dek
                return kid

            # Create new DEK
            dek = os.urandom(32)
            wrapped = self._wrap(dek, org_bytes)

            last_row = OrgDataKey.objects.filter(org_id=org_uuid).order_by("-key_id").first()
            new_key_id = (last_row.key_id + 1) if last_row is not None else 1

            OrgDataKey.objects.create(
                org_id=org_uuid,
                key_id=new_key_id,
                dek_ct=wrapped,
            )

            with self._lock:
                self._active_key_ids[org_str] = new_key_id
                self._deks[f"{org_str}:{new_key_id}"] = dek

            return new_key_id

    def _wrap(self, dek: bytes, org_bytes: bytes) -> bytes:
        nonce = os.urandom(12)
        ct = AESGCM(self.kek).encrypt(nonce, dek, org_bytes)
        return nonce + ct

    def _unwrap(self, wrapped: bytes | memoryview, org_bytes: bytes) -> bytes:
        raw = bytes(wrapped)
        if len(raw) < 28:
            raise ValueError("Invalid wrapped key length")
        nonce = raw[:12]
        ct = raw[12:]
        return AESGCM(self.kek).decrypt(nonce, ct, org_bytes)

    def _get_dek(self, org_id: str | UUID, key_id: int) -> bytes:
        org_str, org_bytes, org_uuid = self._org_info(org_id)
        cache_key = f"{org_str}:{key_id}"

        with self._lock:
            if cache_key in self._deks:
                return self._deks[cache_key]

        from apps.core.models import OrgDataKey

        row = OrgDataKey.objects.filter(org_id=org_uuid, key_id=key_id).first()
        if row is None:
            raise ValueError(f"No key_id {key_id} for org {org_str}")

        dek = self._unwrap(row.dek_ct, org_bytes)
        with self._lock:
            self._deks[cache_key] = dek
        return dek

    def encrypt(self, org_id: str | UUID, plaintext: bytes) -> bytes:
        """Encrypt plaintext for an organisation using its active DEK.

        Returns: key_id (4 bytes BE) || nonce (12 bytes) || ciphertext + tag.
        """
        _org_str, org_bytes, _ = self._org_info(org_id)
        key_id = self.ensure(org_id)
        dek = self._get_dek(org_id, key_id)

        nonce = os.urandom(12)
        body = AESGCM(dek).encrypt(nonce, plaintext, org_bytes)
        return key_id.to_bytes(4, "big") + nonce + body

    def decrypt(self, org_id: str | UUID, ct: bytes | memoryview) -> bytes:
        """Decrypt ciphertext previously sealed for the specified organisation."""
        _org_str, org_bytes, _ = self._org_info(org_id)
        raw = bytes(ct)
        if len(raw) < 32:  # 4 key_id + 12 nonce + 16 tag minimum
            raise ValueError("envelope: malformed or foreign ciphertext")

        key_id = int.from_bytes(raw[:4], "big")
        nonce = raw[4:16]
        cipher_body = raw[16:]

        try:
            dek = self._get_dek(org_id, key_id)
            return AESGCM(dek).decrypt(nonce, cipher_body, org_bytes)
        except (InvalidTag, ValueError) as err:
            raise ValueError("envelope: malformed or foreign ciphertext") from err

    def rotate(self, org_id: str | UUID) -> int:
        """Retire the current active key for an org and generate a fresh DEK."""
        org_str, _, org_uuid = self._org_info(org_id)
        from apps.core.models import OrgDataKey

        with transaction.atomic():
            OrgDataKey.objects.filter(org_id=org_uuid, retired_at__isnull=True).update(
                retired_at=timezone.now()
            )
            with self._lock:
                self._active_key_ids.pop(org_str, None)

            return self.ensure(org_id)

    def blind_index(self, org_id: str | UUID, value: str) -> bytes:
        """Compute an org-specific keyed blind index for exact-match searches."""
        _, org_bytes, _ = self._org_info(org_id)
        hkdf = HKDF(
            algorithm=hashes.SHA256(),
            length=32,
            salt=org_bytes,
            info=b"qrit-bidx-v1",
        )
        bidx_key = hkdf.derive(self.master_key)
        normalized = value.strip().lower().encode("utf-8")
        return hmac.new(bidx_key, normalized, "sha256").digest()

    def seal_platform(self, plaintext: bytes, label: str) -> bytes:
        """Encrypt platform-level secrets not belonging to any organisation."""
        aad = f"platform:{label}".encode()
        nonce = os.urandom(12)
        ct = AESGCM(self.master_key).encrypt(nonce, plaintext, aad)
        return nonce + ct

    def open_platform(self, ct: bytes | memoryview, label: str) -> bytes:
        """Decrypt platform-level secrets."""
        aad = f"platform:{label}".encode()
        raw = bytes(ct)
        if len(raw) < 28:  # 12 nonce + 16 tag minimum
            raise ValueError("envelope: malformed platform ciphertext")
        nonce = raw[:12]
        cipher_body = raw[12:]
        try:
            return AESGCM(self.master_key).decrypt(nonce, cipher_body, aad)
        except InvalidTag as err:
            raise ValueError("envelope: malformed platform ciphertext") from err


_GLOBAL_KEYRING: Keyring | None = None
_GLOBAL_KEYRING_LOCK = threading.Lock()


def get_keyring() -> Keyring:
    """Return the global Keyring singleton."""
    global _GLOBAL_KEYRING
    if _GLOBAL_KEYRING is None:
        with _GLOBAL_KEYRING_LOCK:
            if _GLOBAL_KEYRING is None:
                _GLOBAL_KEYRING = Keyring()
    return _GLOBAL_KEYRING
