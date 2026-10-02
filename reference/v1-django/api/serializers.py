"""Response shapes.

Plain functions rather than DRF serializers: the models are legacy-mapped and
several fields need computed values, so an explicit dict keeps the wire format
obvious and stable for the dashboard.
"""

from django.conf import settings


def _iso(value):
    return value.isoformat() if value else None


def _csv_list(value):
    if not value:
        return []
    if isinstance(value, list):
        return value
    return [part.strip() for part in str(value).split(",") if part.strip()]


# ------------------------------------------------------------------ user


def user_summary(user):
    if user is None:
        return None
    return {
        "id": str(user.id),
        "email": user.email,
        "name": user.name,
        "avatar_url": user.avatar_url,
    }


def user_detail(user):
    return {
        "id": str(user.id),
        "email": user.email,
        "name": user.name,
        "company": user.company,
        "avatar_url": user.avatar_url,
        "plan": user.plan or "free",
        "plan_expires_at": _iso(user.plan_expires_at),
        "subscription_status": user.subscription_status,
        "subscription_ends_at": _iso(user.subscription_ends_at),
        "default_workspace_id": str(user.default_workspace_id) if user.default_workspace_id else None,
        "created_at": _iso(user.created_at),
    }


# ------------------------------------------------------------------ workspace


def workspace(ws, role=None, counts=None):
    data = {
        "id": str(ws.id),
        "name": ws.name,
        "slug": ws.slug,
        "description": ws.description,
        "owner_id": str(ws.owner_id) if ws.owner_id else None,
        "plan": ws.plan or "free",
        "logo_url": ws.logo_url,
        "brand_color": ws.brand_color or "#8B5CF6",
        "brand_logo": ws.brand_logo,
        "favicon_url": ws.favicon_url,
        "custom_css": ws.custom_css,
        "custom_footer": ws.custom_footer,
        "custom_domain": ws.custom_domain,
        "remove_branding": bool(ws.remove_branding),
        "sso_enabled": bool(ws.sso_enabled),
        "created_at": _iso(ws.created_at),
        "updated_at": _iso(ws.updated_at),
    }
    if role is not None:
        data["role"] = role
    if counts:
        data.update(counts)
    return data


def member(row, current_user_id=None, owner_id=None):
    """`row` is a WorkspaceMember with `user` selected."""
    is_owner = owner_id is not None and str(row.user_id) == str(owner_id)
    return {
        "id": str(row.id),
        "role": "owner" if is_owner else row.role,
        "joined_at": _iso(row.joined_at),
        "invited_by": str(row.invited_by) if row.invited_by else None,
        "is_you": str(row.user_id) == str(current_user_id) if current_user_id else False,
        "user": user_summary(row.user),
    }


def invite(row, invite_url=None):
    data = {
        "id": str(row.id),
        "email": row.email,
        "role": row.role,
        "status": row.status,
        "expires_at": _iso(row.expires_at),
        "created_at": _iso(row.created_at),
        "invited_by": str(row.invited_by) if row.invited_by else None,
    }
    if invite_url:
        data["invite_url"] = invite_url
    return data


def folder(row, qr_count=0, children=None):
    return {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "parent_id": str(row.parent_id) if row.parent_id else None,
        "name": row.name,
        "description": row.description,
        "color": row.color,
        "icon": row.icon,
        "sort_order": row.sort_order,
        "qr_count": qr_count,
        "scan_count": row.scan_count,
        "children": children or [],
        "created_at": _iso(row.created_at),
    }


def audit_entry(row):
    import json

    details = None
    if row.details:
        try:
            details = json.loads(row.details)
        except (ValueError, TypeError):
            details = {"raw": row.details}
    return {
        "id": str(row.id),
        "action": row.action,
        "resource": row.resource,
        "resource_id": str(row.resource_id) if row.resource_id else None,
        "details": details,
        "ip_address": row.ip_address,
        "user_agent": row.user_agent,
        "created_at": _iso(row.created_at),
        "user": user_summary(row.user),
    }


# ------------------------------------------------------------------ campaigns


def campaign(row, stats=None):
    data = {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "name": row.name,
        "description": row.description,
        "status": row.status,
        "color": row.color,
        "starts_at": _iso(row.starts_at),
        "ends_at": _iso(row.ends_at),
        "scan_goal": int(row.scan_goal or 0),
        "utm": {
            "source": row.utm_source,
            "medium": row.utm_medium,
            "campaign": row.utm_campaign,
            "term": row.utm_term,
            "content": row.utm_content,
        },
        "tags": _csv_list(row.tags),
        "created_at": _iso(row.created_at),
        "updated_at": _iso(row.updated_at),
        "qr_count": 0,
        "scan_count": 0,
    }
    if stats:
        data.update(stats)
        goal = data["scan_goal"]
        data["goal_progress"] = round(min(100.0, data["scan_count"] / goal * 100), 1) if goal else None
    return data


def template(row):
    return {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "name": row.name,
        "description": row.description,
        "design": row.design or {},
        "preview_url": row.preview_url,
        "is_default": row.is_default,
        "is_locked": row.is_locked,
        "usage_count": row.usage_count,
        "created_at": _iso(row.created_at),
        "updated_at": _iso(row.updated_at),
    }


def routing_rule(row):
    return {
        "id": str(row.id),
        "qr_id": str(row.qr_record_id),
        "name": row.name,
        "condition": row.condition,
        "operator": row.operator,
        "value": row.value,
        "destination_url": row.destination_url,
        "priority": row.priority,
        "is_active": row.is_active,
        "hit_count": int(row.hit_count or 0),
        "created_at": _iso(row.created_at),
    }


# ------------------------------------------------------------------ governance


def api_key(row, plaintext=None):
    data = {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "name": row.name,
        "prefix": row.prefix,
        "masked": f"{row.prefix}{'*' * 24}",
        "scopes": _csv_list(row.scopes),
        "last_used_at": _iso(row.last_used_at),
        "calls_today": int(row.calls_today or 0),
        "total_calls": int(row.total_calls or 0),
        "expires_at": _iso(row.expires_at),
        "revoked_at": _iso(row.revoked_at),
        "is_active": row.revoked_at is None,
        "created_at": _iso(row.created_at),
    }
    if plaintext:
        data["key"] = plaintext
    return data


def custom_domain(row):
    return {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "domain": row.domain,
        "status": row.status,
        "is_primary": row.is_primary,
        "ssl_status": row.ssl_status,
        "verification_method": row.verification_method,
        "dns_record": {
            "type": "TXT",
            "host": f"_qrit-verify.{row.domain}",
            "value": row.verification_token,
        },
        "cname_record": {
            "type": "CNAME",
            "host": row.domain,
            "value": "cname.qrit.app",
        },
        "verified_at": _iso(row.verified_at),
        "last_checked_at": _iso(row.last_checked_at),
        "created_at": _iso(row.created_at),
    }


def sso_config(row, workspace_id):
    if row is None:
        return {
            "workspace_id": str(workspace_id),
            "is_enabled": False,
            "provider": "saml",
            "entity_id": None,
            "sso_url": None,
            "slo_url": None,
            "certificate": None,
            "metadata_url": None,
            "email_domains": [],
            "default_role": "viewer",
            "enforce_sso": False,
            "scim_enabled": False,
            "has_scim_token": False,
            "acs_url": f"{settings.API_BASE_URL}/api/v1/auth/sso/{workspace_id}/acs",
            "sp_entity_id": f"{settings.API_BASE_URL}/api/v1/auth/sso/{workspace_id}/metadata",
        }
    return {
        "workspace_id": str(row.workspace_id),
        "is_enabled": row.is_enabled,
        "provider": row.provider,
        "entity_id": row.entity_id,
        "sso_url": row.sso_url,
        "slo_url": row.slo_url,
        # certificate is write-only; expose presence, not content
        "certificate": bool(row.certificate),
        "metadata_url": row.metadata_url,
        "email_domains": _csv_list(row.email_domains),
        "default_role": row.default_role,
        "enforce_sso": row.enforce_sso,
        "scim_enabled": row.scim_enabled,
        "has_scim_token": bool(row.scim_token_hash),
        "acs_url": f"{settings.API_BASE_URL}/api/v1/auth/sso/{row.workspace_id}/acs",
        "sp_entity_id": f"{settings.API_BASE_URL}/api/v1/auth/sso/{row.workspace_id}/metadata",
        "updated_at": _iso(row.updated_at),
    }


def security_policy(row, workspace_id):
    if row is None:
        return {
            "workspace_id": str(workspace_id),
            "allowed_domains": [],
            "blocked_domains": [],
            "require_https": True,
            "require_approval": False,
            "scan_alert_threshold": 0,
            "password_min_length": 8,
            "session_timeout_minutes": 0,
        }
    return {
        "workspace_id": str(row.workspace_id),
        "allowed_domains": _csv_list(row.allowed_domains),
        "blocked_domains": _csv_list(row.blocked_domains),
        "require_https": row.require_https,
        "require_approval": row.require_approval,
        "scan_alert_threshold": row.scan_alert_threshold,
        "password_min_length": row.password_min_length,
        "session_timeout_minutes": row.session_timeout_minutes,
        "updated_at": _iso(row.updated_at),
    }


def webhook(row, secret=None):
    data = {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "url": row.url,
        "events": _csv_list(row.events),
        "is_active": row.is_active,
        "description": row.description,
        "last_triggered": _iso(row.last_triggered),
        "fail_count": row.fail_count,
        "created_at": _iso(row.created_at),
    }
    if secret:
        data["secret"] = secret
    return data


def webhook_log(row):
    return {
        "id": str(row.id),
        "webhook_id": str(row.webhook_id),
        "event": row.event,
        "payload": row.payload,
        "status_code": row.status_code,
        "response": (row.response or "")[:2000],
        "success": row.success,
        "duration_ms": row.duration,
        "error": row.error,
        "created_at": _iso(row.created_at),
    }


def bulk_job(row):
    import json

    errors = []
    if row.errors:
        try:
            errors = json.loads(row.errors)
        except (ValueError, TypeError):
            errors = []
    return {
        "id": str(row.id),
        "workspace_id": str(row.workspace_id),
        "filename": row.filename,
        "status": row.status,
        "total_rows": row.total_rows,
        "success_count": row.success_count,
        "error_count": row.error_count,
        "errors": errors,
        "options": row.options or {},
        "created_at": _iso(row.created_at),
        "completed_at": _iso(row.completed_at),
    }
