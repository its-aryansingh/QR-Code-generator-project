"""Enterprise governance: branding, SSO, security policy, custom domains
and workspace-scoped API keys.

These are the four things that show up on every enterprise procurement
checklist and none of them had an endpoint before.
"""

import secrets
import uuid
from urllib.parse import urlparse

import bcrypt
from django.conf import settings
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api import serializers as s
from api.models import ApiKey, CustomDomain, SecurityPolicy, SsoConfig, Workspace
from api.utils import audit
from api.utils.auth import require_auth
from api.utils.entitlements import feature_denied, limit_exceeded
from api.utils.rbac import require_workspace

VALID_SCOPES = [
    "qr:read", "qr:write", "qr:delete",
    "analytics:read", "campaigns:read", "campaigns:write",
    "leads:read", "webhooks:write",
]


# ------------------------------------------------------------------ branding


class BrandingView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        ws = request.workspace
        primary = CustomDomain.objects.filter(
            workspace_id=ws_id, is_primary=True, status="verified"
        ).first()
        return Response(
            {
                "success": True,
                "data": {
                    "workspace_id": str(ws.id),
                    "brand_color": ws.brand_color or "#8B5CF6",
                    "brand_logo": ws.brand_logo,
                    "logo_url": ws.logo_url,
                    "favicon_url": ws.favicon_url,
                    "custom_css": ws.custom_css,
                    "custom_footer": ws.custom_footer,
                    "remove_branding": bool(ws.remove_branding),
                    "custom_domain": primary.domain if primary else ws.custom_domain,
                    "can_white_label": feature_denied(ws.plan or "free", "white_label") is None,
                },
            }
        )

    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id):
        plan = request.workspace.plan or "free"
        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("brand_color", "brand_logo", "logo_url", "favicon_url",
                      "custom_css", "custom_footer"):
            if field in d:
                updates[field] = d[field] or None

        # Removing QRit branding and injecting custom CSS are white-label
        # capabilities, so they are gated even though the columns always exist.
        white_label_fields = {"remove_branding", "custom_css", "custom_footer"} & set(d)
        if white_label_fields:
            denial = feature_denied(plan, "white_label")
            if denial:
                return denial
        if "remove_branding" in d:
            updates["remove_branding"] = bool(d["remove_branding"])

        Workspace.objects.filter(pk=request.workspace.pk).update(**updates)
        request.workspace.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "branding", ws_id,
                     {"fields": sorted(k for k in updates if k != "updated_at")})
        return self.get(request, ws_id=ws_id)


# ------------------------------------------------------------------ SSO


class SsoView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        row = SsoConfig.objects.filter(workspace_id=ws_id).first()
        data = s.sso_config(row, ws_id)
        data["available"] = feature_denied(request.workspace.plan or "free", "sso") is None
        return Response({"success": True, "data": data})

    @require_auth
    @require_workspace("owner")
    def put(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "sso")
        if denial:
            return denial

        d = request.data
        now = timezone.now()
        row = SsoConfig.objects.filter(workspace_id=ws_id).first()
        if row is None:
            row = SsoConfig(id=uuid.uuid4(), workspace_id=ws_id, created_at=now, updated_at=now)
            row.save(force_insert=True)

        updates = {"updated_at": now}
        for field in ("entity_id", "sso_url", "slo_url", "certificate", "metadata_url"):
            if field in d:
                updates[field] = d[field] or None
        if "provider" in d:
            if d["provider"] not in ("saml", "oidc"):
                return Response({"success": False, "error": "provider must be saml or oidc"}, status=400)
            updates["provider"] = d["provider"]
        if "email_domains" in d:
            domains = d["email_domains"]
            if isinstance(domains, str):
                domains = [domains]
            updates["email_domains"] = ",".join(x.strip().lower().lstrip("@") for x in domains if x.strip()) or None
        if "default_role" in d:
            if d["default_role"] not in ("admin", "editor", "viewer"):
                return Response({"success": False, "error": "Invalid default_role"}, status=400)
            updates["default_role"] = d["default_role"]
        for flag in ("is_enabled", "enforce_sso", "scim_enabled"):
            if flag in d:
                updates[flag] = bool(d[flag])

        if updates.get("is_enabled"):
            merged = {**{f.name: getattr(row, f.name) for f in SsoConfig._meta.fields}, **updates}
            missing = [f for f in ("entity_id", "sso_url", "certificate") if not merged.get(f)]
            if missing:
                return Response(
                    {"success": False,
                     "error": f"Cannot enable SSO until these are set: {', '.join(missing)}"},
                    status=400,
                )

        SsoConfig.objects.filter(pk=row.pk).update(**updates)
        Workspace.objects.filter(pk=request.workspace.pk).update(
            sso_enabled=bool(updates.get("is_enabled", row.is_enabled)),
            sso_provider=updates.get("provider", row.provider),
        )
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "sso", ws_id,
                     {"enabled": row.is_enabled, "provider": row.provider})
        return Response({"success": True, "data": s.sso_config(row, ws_id)})


class ScimTokenView(APIView):
    @require_auth
    @require_workspace("owner")
    def post(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "scim")
        if denial:
            return denial
        row = SsoConfig.objects.filter(workspace_id=ws_id).first()
        if row is None:
            return Response({"success": False, "error": "Configure SSO before enabling SCIM"}, status=400)
        token = f"scim_{secrets.token_urlsafe(32)}"
        SsoConfig.objects.filter(pk=row.pk).update(
            scim_token_hash=bcrypt.hashpw(token.encode(), bcrypt.gensalt(rounds=12)).decode(),
            scim_enabled=True,
            updated_at=timezone.now(),
        )
        audit.record(request, ws_id, audit.CREATE, "scim_token", ws_id, None)
        return Response(
            {"success": True, "data": {"token": token, "shown_once": True}}, status=201
        )


# ------------------------------------------------------------------ security


class SecurityPolicyView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        row = SecurityPolicy.objects.filter(workspace_id=ws_id).first()
        data = s.security_policy(row, ws_id)
        data["available"] = feature_denied(request.workspace.plan or "free", "security_policy") is None
        return Response({"success": True, "data": data})

    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "security_policy")
        if denial:
            return denial
        d = request.data
        now = timezone.now()
        row = SecurityPolicy.objects.filter(workspace_id=ws_id).first()
        if row is None:
            row = SecurityPolicy(id=uuid.uuid4(), workspace_id=ws_id, created_at=now, updated_at=now)
            row.save(force_insert=True)

        updates = {"updated_at": now}
        for field in ("allowed_domains", "blocked_domains"):
            if field in d:
                value = d[field]
                if isinstance(value, str):
                    value = [value]
                updates[field] = ",".join(x.strip().lower() for x in (value or []) if x.strip()) or None
        for flag in ("require_https", "require_approval"):
            if flag in d:
                updates[flag] = bool(d[flag])
        for number in ("scan_alert_threshold", "password_min_length", "session_timeout_minutes"):
            if number in d:
                updates[number] = max(0, int(d[number] or 0))

        SecurityPolicy.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "security_policy", ws_id,
                     {"fields": sorted(k for k in updates if k != "updated_at")})
        return Response({"success": True, "data": s.security_policy(row, ws_id)})


def check_destination(workspace_id, url):
    """Validate a redirect target against the workspace security policy.

    Returns an error string, or None when the URL is allowed. This is the
    control that stops an editor pointing a printed code at an attacker's host.
    """
    if not url:
        return None
    policy = SecurityPolicy.objects.filter(workspace_id=workspace_id).first()
    if policy is None:
        return None
    try:
        parts = urlparse(url)
    except ValueError:
        return "Destination is not a valid URL"
    if not parts.scheme:
        return None  # non-URL payloads (wifi, vcard, text) are not policed here
    if policy.require_https and parts.scheme == "http":
        return "This workspace requires HTTPS destinations"

    host = (parts.hostname or "").lower()
    blocked = [x.strip() for x in (policy.blocked_domains or "").split(",") if x.strip()]
    if any(host == b or host.endswith(f".{b}") for b in blocked):
        return f"{host} is on this workspace's blocked domain list"

    allowed = [x.strip() for x in (policy.allowed_domains or "").split(",") if x.strip()]
    if allowed and not any(host == a or host.endswith(f".{a}") for a in allowed):
        return f"{host} is not on this workspace's allowed domain list"
    return None


# ------------------------------------------------------------------ domains


class CustomDomainListView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        rows = CustomDomain.objects.filter(workspace_id=ws_id).order_by("-is_primary", "domain")
        return Response({"success": True, "data": [s.custom_domain(r) for r in rows]})

    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id):
        plan = request.workspace.plan or "free"
        denial = feature_denied(plan, "custom_domain")
        if denial:
            return denial
        domain = (request.data.get("domain") or "").strip().lower()
        domain = domain.replace("https://", "").replace("http://", "").strip("/")
        if not domain or "." not in domain or " " in domain:
            return Response({"success": False, "error": "Enter a valid domain, e.g. links.acme.com"}, status=400)
        if CustomDomain.objects.filter(domain=domain).exists():
            return Response({"success": False, "error": "That domain is already registered"}, status=409)

        current = CustomDomain.objects.filter(workspace_id=ws_id).count()
        denial = limit_exceeded(plan, "max_custom_domains", current, "custom domains")
        if denial:
            return denial

        row = CustomDomain(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            domain=domain,
            verification_token=f"qrit-verify={secrets.token_hex(16)}",
            is_primary=current == 0,
            created_at=timezone.now(),
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "domain", row.id, {"domain": domain})
        return Response({"success": True, "data": s.custom_domain(row)}, status=201)


class CustomDomainDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id, domain_id):
        """Verify the DNS TXT record."""
        row = CustomDomain.objects.filter(id=domain_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Domain not found"}, status=404)

        verified, detail = _verify_dns_txt(row.domain, row.verification_token)
        updates = {"last_checked_at": timezone.now()}
        if verified:
            updates.update(status="verified", ssl_status="active", verified_at=timezone.now())
            Workspace.objects.filter(pk=request.workspace.pk).update(custom_domain=row.domain)
        else:
            updates["status"] = "failed"
        CustomDomain.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "domain", domain_id,
                     {"domain": row.domain, "verified": verified})
        return Response(
            {"success": True, "data": {**s.custom_domain(row), "verified": verified, "detail": detail}}
        )

    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, domain_id):
        row = CustomDomain.objects.filter(id=domain_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Domain not found"}, status=404)
        domain, was_primary = row.domain, row.is_primary
        row.delete()
        if was_primary:
            replacement = CustomDomain.objects.filter(workspace_id=ws_id, status="verified").first()
            if replacement:
                CustomDomain.objects.filter(pk=replacement.pk).update(is_primary=True)
            Workspace.objects.filter(pk=request.workspace.pk).update(
                custom_domain=replacement.domain if replacement else None
            )
        audit.record(request, ws_id, audit.DELETE, "domain", domain_id, {"domain": domain})
        return Response({"success": True, "data": {"message": "Domain removed"}})


def _verify_dns_txt(domain, expected_token):
    """Confirm ownership of `domain`.

    Two methods, tried in order. The HTTP method needs no extra dependency, so
    verification works on a stock deployment; the DNS method is used when
    dnspython is installed and is the one enterprise DNS teams expect.
    """
    dns_ok, dns_detail = _check_dns(domain, expected_token)
    if dns_ok:
        return True, dns_detail
    http_ok, http_detail = _check_http_file(domain, expected_token)
    if http_ok:
        return True, http_detail
    return False, f"{dns_detail} {http_detail}".strip()


def _check_dns(domain, expected_token):
    host = f"_qrit-verify.{domain}"
    try:
        import dns.resolver  # type: ignore
    except ImportError:
        return False, "DNS check skipped (dnspython not installed)."
    try:
        for record in dns.resolver.resolve(host, "TXT", lifetime=5):
            text = b"".join(record.strings).decode() if hasattr(record, "strings") else str(record)
            if expected_token in text:
                return True, f"Verified via TXT record on {host}."
        return False, f"TXT record on {host} did not contain the token."
    except Exception as exc:
        return False, f"Could not read TXT records for {host} ({exc})."


def _check_http_file(domain, expected_token):
    """Look for the token at https://<domain>/.well-known/qrit-verification.txt"""
    import urllib.error
    import urllib.request

    url = f"https://{domain}/.well-known/qrit-verification.txt"
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "QRit-DomainVerifier/1.0"})
        with urllib.request.urlopen(request, timeout=8) as response:
            body = response.read(4096).decode("utf-8", "replace")
        if expected_token in body:
            return True, f"Verified via {url}."
        return False, f"{url} did not contain the token."
    except Exception as exc:
        return False, f"Could not fetch {url} ({exc})."


# ------------------------------------------------------------------ API keys


class ApiKeyListView(APIView):
    @require_auth
    @require_workspace("admin")
    def get(self, request, ws_id):
        rows = ApiKey.objects.filter(workspace_id=ws_id).order_by("-created_at")
        return Response(
            {
                "success": True,
                "data": {
                    "keys": [s.api_key(r) for r in rows],
                    "valid_scopes": VALID_SCOPES,
                    "base_url": f"{settings.API_BASE_URL}/api/v1",
                },
            }
        )

    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id):
        plan = request.workspace.plan or "free"
        name = (request.data.get("name") or "").strip() or "Untitled key"
        scopes = request.data.get("scopes") or ["qr:read", "qr:write", "analytics:read"]
        invalid = [x for x in scopes if x not in VALID_SCOPES]
        if invalid:
            return Response(
                {"success": False, "error": f"Unknown scopes: {', '.join(invalid)}",
                 "valid_scopes": VALID_SCOPES},
                status=400,
            )

        live = ApiKey.objects.filter(workspace_id=ws_id, revoked_at__isnull=True).count()
        denial = limit_exceeded(plan, "max_api_keys", live, "API keys")
        if denial:
            return denial

        prefix = f"qk_{secrets.token_hex(4)}"
        secret = secrets.token_urlsafe(32)
        plaintext = f"{prefix}.{secret}"

        row = ApiKey(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            name=name,
            prefix=prefix,
            key_hash=bcrypt.hashpw(secret.encode(), bcrypt.gensalt(rounds=12)).decode(),
            scopes=",".join(scopes),
            calls_reset_at=timezone.now().date(),
            expires_at=request.data.get("expires_at") or None,
            created_by=request.auth_user["id"],
            created_at=timezone.now(),
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "api_key", row.id,
                     {"name": name, "scopes": scopes})
        # The plaintext is returned exactly once and never stored.
        return Response({"success": True, "data": s.api_key(row, plaintext)}, status=201)


class ApiKeyDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, key_id):
        row = ApiKey.objects.filter(id=key_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "API key not found"}, status=404)
        ApiKey.objects.filter(pk=row.pk).update(revoked_at=timezone.now())
        audit.record(request, ws_id, audit.REVOKE, "api_key", key_id, {"name": row.name})
        return Response({"success": True, "data": {"message": "API key revoked"}})
