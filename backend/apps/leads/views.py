"""Lead pages and captured leads for the dashboard, plus the public page at /p/<slug>.

A dashboard "lead page" is a v3 form: the fields and consent notice live in
their columns, the page's look (headline, colours, button) in
`forms.presentation`, and its public address in `forms.legacy_slug`, which is
where v1's /p/<slug> links already resolve. Submissions are encrypted with the
org's key and indexed by a blind index of the email, never stored in clear.
"""

import json
import re
from datetime import timedelta
from typing import Any

from django.core.exceptions import ValidationError
from django.core.validators import validate_email
from django.db import connection, transaction
from django.db.models import Count, Q, QuerySet
from django.utils import timezone
from django.utils.text import slugify
from rest_framework.request import Request
from rest_framework.response import Response

from apps.audit.record import changed_fields, record
from apps.core.crypto import get_keyring
from apps.core.errors import not_found, rate_limited, unprocessable
from apps.core.net.client_ip import ip_key
from apps.core.ratelimit import allow
from apps.workspaces.access import (
    DashboardAPIView,
    PublicDashboardAPIView,
    body,
    iso,
    ok,
    page_params,
    paginated,
    uuid_or_404,
    workspace_for,
)
from apps.workspaces.models import Workspace

from .models import Form, FormSubmission

MAX_FIELDS = 25
MAX_VALUE_LENGTH = 2000
FIELD_TYPES = {"text", "email", "phone", "tel", "select", "checkbox", "textarea", "date", "number"}
LOOK_DEFAULTS: dict[str, Any] = {
    "headline": "",
    "subheadline": "",
    "hero_image": None,
    "button_text": "Submit",
    "button_color": "#8B5CF6",
    "background_color": "#09090B",
    "text_color": "#FAFAFA",
    "thank_you_message": "Thank you!",
    "redirect_url": None,
    "requires_opt_in": False,
}
HEX = re.compile(r"^#[0-9a-fA-F]{6}$")
DEFAULT_FIELDS = [
    {"name": "email", "label": "Email address", "type": "email", "required": True},
    {"name": "name", "label": "Full name", "type": "text", "required": False},
]


def _look(form: Form) -> dict[str, Any]:
    stored = form.presentation if isinstance(form.presentation, dict) else {}
    return {**LOOK_DEFAULTS, **{k: v for k, v in stored.items() if k in LOOK_DEFAULTS}}


def _views(form: Form) -> int:
    stored = form.presentation if isinstance(form.presentation, dict) else {}
    return int(stored.get("views") or 0)


def _notice(form: Form) -> dict[str, Any]:
    return form.notice if isinstance(form.notice, dict) else {}


def serialize_page(form: Form, lead_count: int = 0) -> dict[str, Any]:
    look, notice, views = _look(form), _notice(form), _views(form)
    return {
        "id": str(form.id),
        "workspace_id": str(form.workspace_id),
        "qr_record_id": None,
        "name": form.name,
        "slug": form.legacy_slug,
        **look,
        "form_fields": form.fields if isinstance(form.fields, list) else [],
        "is_active": form.is_active,
        "privacy_policy": notice.get("privacy_url"),
        "consent_text": notice.get("text"),
        "views": views,
        "submissions": lead_count,
        "lead_count": lead_count,
        "conversion_rate": round(lead_count / views * 100, 1) if views else 0.0,
        "created_at": iso(form.created_at),
    }


def _clean_fields(raw: Any) -> list[dict[str, Any]]:
    if isinstance(raw, str):
        try:
            raw = json.loads(raw or "[]")
        except json.JSONDecodeError:
            raise unprocessable(
                code="invalid_fields", detail="form_fields isn't valid JSON"
            ) from None
    if not isinstance(raw, list) or not raw:
        raise unprocessable(code="invalid_fields", detail="add at least one form field")
    if len(raw) > MAX_FIELDS:
        raise unprocessable(
            code="invalid_fields", detail=f"a form can have at most {MAX_FIELDS} fields"
        )
    fields, seen = [], set()
    for item in raw:
        if not isinstance(item, dict):
            raise unprocessable(code="invalid_fields", detail="each form field must be an object")
        name = re.sub(r"[^a-z0-9_]", "_", str(item.get("name") or "").strip().lower())[:40]
        kind = str(item.get("type") or "text")
        if not name or name.startswith("_") or name in seen or kind not in FIELD_TYPES:
            raise unprocessable(
                code="invalid_fields",
                detail="each field needs a unique name and a known type",
            )
        seen.add(name)
        field: dict[str, Any] = {
            "name": name,
            "label": str(item.get("label") or name)[:120],
            "type": kind,
            "required": bool(item.get("required")),
        }
        if isinstance(item.get("options"), list):
            field["options"] = [str(o)[:120] for o in item["options"][:50]]
        fields.append(field)
    return fields


def _unique_slug(name: str, current: Form | None = None) -> str:
    base = slugify(name)[:50] or "page"
    slug, n = base, 1
    taken = Form.objects.filter(legacy_slug__startswith=base)
    if current is not None:
        taken = taken.exclude(id=current.id)
    used = set(taken.values_list("legacy_slug", flat=True))
    while slug in used:
        n += 1
        slug = f"{base}-{n}"
    return slug


def _apply(form: Form, data: dict[str, Any], partial: bool) -> None:
    if "name" in data or not partial:
        name = str(data.get("name") or "").strip()
        if not name:
            raise unprocessable(code="name_required", detail="page name is required")
        form.name = name[:120]
    if "form_fields" in data or not partial:
        form.fields = _clean_fields(data.get("form_fields") or DEFAULT_FIELDS)
    look = _look(form) if form.presentation else dict(LOOK_DEFAULTS)
    for key in LOOK_DEFAULTS:
        if key not in data:
            continue
        value = data[key]
        if key.endswith("_color"):
            if not HEX.match(str(value or "")):
                raise unprocessable(code="invalid_color", detail=f"{key} must be a #RRGGBB colour")
            look[key] = str(value)
        elif key == "requires_opt_in":
            look[key] = bool(value)
        elif key in ("redirect_url", "hero_image"):
            url = str(value or "").strip()
            if url and not url.startswith("https://"):
                raise unprocessable(code="invalid_url", detail=f"{key} must be an https:// address")
            look[key] = url or None
        else:
            look[key] = str(value or "")[:500]
    form.presentation = {**look, "views": _views(form)}
    if "consent_text" in data or "privacy_policy" in data or not partial:
        notice = _notice(form)
        form.notice = {
            "text": str(data.get("consent_text", notice.get("text")) or "")[:1000] or None,
            "privacy_url": str(data.get("privacy_policy", notice.get("privacy_url")) or "")[:500]
            or None,
        }
    if "is_active" in data:
        form.is_active = bool(data["is_active"])


def _lead_counts(workspace: Workspace) -> dict[Any, int]:
    return {
        row["form_id"]: row["n"]
        for row in FormSubmission.objects.filter(workspace_id=workspace.id)
        .exclude(status="erased")
        .values("form_id")
        .annotate(n=Count("id"))
    }


def _form_in(workspace: Workspace, page_id: str) -> Form:
    form = Form.objects.filter(workspace=workspace, id=uuid_or_404(page_id, "page")).first()
    if form is None:
        raise not_found("page not found")
    return form


class WorkspaceLeadPagesView(DashboardAPIView):
    list_view = True

    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        counts = _lead_counts(workspace)
        forms = Form.objects.filter(workspace=workspace).order_by("-created_at")
        return ok([serialize_page(form, counts.get(form.id, 0)) for form in forms])

    def post(self, request: Request, ws: str) -> Response:
        user = self.user(request)
        workspace, _ = workspace_for(user, ws, min_role="editor")
        form = Form(workspace=workspace, created_by=user, presentation={})
        _apply(form, body(request), partial=False)
        form.legacy_slug = _unique_slug(form.name)
        with transaction.atomic():
            form.save()
            record(request, workspace, "create", "lead_page", form.id, {"name": form.name})
        return ok(serialize_page(form), status=201)


class WorkspaceLeadPageDetailView(DashboardAPIView):
    def get(self, request: Request, ws: str, page_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        form = _form_in(workspace, page_id)
        return ok(serialize_page(form, _lead_counts(workspace).get(form.id, 0)))

    def put(self, request: Request, ws: str, page_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="editor")
        form = _form_in(workspace, page_id)
        before = serialize_page(form)
        _apply(form, body(request), partial=True)
        form.updated_at = timezone.now()
        after = serialize_page(form)
        with transaction.atomic():
            form.save()
            record(
                request,
                workspace,
                "update",
                "lead_page",
                form.id,
                {"name": form.name, "fields": changed_fields(before, after)},
            )
        return ok(serialize_page(form, _lead_counts(workspace).get(form.id, 0)))

    def delete(self, request: Request, ws: str, page_id: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws, min_role="editor")
        form = _form_in(workspace, page_id)
        with transaction.atomic():
            record(request, workspace, "delete", "lead_page", form.id, {"name": form.name})
            form.delete()
        return ok({"message": "Page deleted"})


def _seal(workspace: Workspace, form: Form, data: dict[str, Any]) -> tuple[int, bytes]:
    keyring = get_keyring()
    raw = json.dumps(data, separators=(",", ":")).encode()
    if workspace.org_id:
        ct = keyring.encrypt(workspace.org_id, raw)
        return int.from_bytes(ct[:4], "big"), ct
    return 0, keyring.seal_platform(raw, f"lead:{form.id}")


def _unseal(workspace: Workspace, row: FormSubmission) -> dict[str, Any]:
    keyring = get_keyring()
    try:
        if workspace.org_id:
            raw = keyring.decrypt(workspace.org_id, row.data_ct)
        else:
            raw = keyring.open_platform(row.data_ct, f"lead:{row.form_id}")
        data = json.loads(raw)
    except (ValueError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


def _leads(workspace: Workspace, request: Request) -> QuerySet[FormSubmission]:
    rows = FormSubmission.objects.filter(workspace_id=workspace.id).exclude(status="erased")
    params = request.query_params
    if params.get("page_id"):
        rows = rows.filter(form_id=uuid_or_404(params["page_id"], "page"))
    search = (params.get("search") or "").strip().lower()
    if search and "@" in search:
        keyring = get_keyring()
        rows = rows.filter(
            Q(email_bidx=keyring.blind_index(workspace.org_id, search)) if workspace.org_id else Q()
        )
    return rows.select_related("form").order_by("-created_at")


def _lead(workspace: Workspace, row: FormSubmission) -> dict[str, Any]:
    data = _unseal(workspace, row)
    consent = row.consent if isinstance(row.consent, dict) else {}
    return {
        "id": str(row.id),
        "page_id": str(row.form_id),
        "page_name": row.form.name,
        "email": data.get("email"),
        "data": data,
        "opted_in": bool(consent.get("opted_in")),
        "source": "qr" if row.qr_code_id else "link",
        "ip_address": None,
        "created_at": iso(row.created_at),
    }


class WorkspaceLeadsView(DashboardAPIView):
    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        page, limit = page_params(request, default_limit=50)
        rows = _leads(workspace, request)
        total = rows.count()
        return ok(
            paginated(
                [_lead(workspace, row) for row in rows[(page - 1) * limit : page * limit]],
                total,
                page,
                limit,
            )
        )


def export_rows(workspace: Workspace) -> tuple[list[str], list[list[Any]]]:
    rows = (
        FormSubmission.objects.filter(workspace_id=workspace.id)
        .exclude(status="erased")
        .select_related("form")
        .order_by("-created_at")[:50000]
    )
    leads = [_lead(workspace, row) for row in rows]
    keys = sorted({k for lead in leads for k in lead["data"]} - {"email"})
    header = ["created_at", "page", "email", "opted_in", *keys]
    return header, [
        [lead["created_at"], lead["page_name"], lead["email"], lead["opted_in"]]
        + [lead["data"].get(k) for k in keys]
        for lead in leads
    ]


def _public_form(slug: str) -> Form:
    form = (
        Form.objects.select_related("workspace")
        .filter(legacy_slug=slug, is_active=True, workspace__deleted_at__isnull=True)
        .first()
    )
    if form is None:
        raise not_found("this page doesn't exist or is no longer active")
    return form


class PublicLeadPageView(PublicDashboardAPIView):
    def get(self, request: Request, slug: str) -> Response:
        form = _public_form(slug)
        with connection.cursor() as cursor:
            cursor.execute(
                "UPDATE forms SET presentation = jsonb_set(presentation, '{views}', "
                "to_jsonb(coalesce((presentation->>'views')::int, 0) + 1)) WHERE id = %s",
                [form.id],
            )
        look, notice = _look(form), _notice(form)
        return ok(
            {
                "id": str(form.id),
                "name": form.name,
                "slug": form.legacy_slug,
                **{k: ("" if v is None else v) for k, v in look.items()},
                "form_fields": json.dumps(form.fields if isinstance(form.fields, list) else []),
                "consent_text": notice.get("text"),
                "privacy_policy": notice.get("privacy_url"),
                "is_active": form.is_active,
            }
        )


class PublicLeadSubmitView(PublicDashboardAPIView):
    def post(self, request: Request, slug: str) -> Response:
        allowed, _, retry_after = allow(f"rl:lead:{slug}:{ip_key(request)}", 10, 3600)
        if not allowed:
            raise rate_limited(retry_after)
        form = _public_form(slug)
        workspace = form.workspace
        data_in = body(request)
        look = _look(form)
        values: dict[str, Any] = {}
        for field in form.fields if isinstance(form.fields, list) else []:
            value = str(data_in.get(field["name"]) or "").strip()[:MAX_VALUE_LENGTH]
            if field.get("required") and not value:
                raise unprocessable(code="field_required", detail=f"{field['label']} is required")
            if value and field["type"] == "email":
                try:
                    validate_email(value)
                except ValidationError:
                    raise unprocessable(
                        code="invalid_email", detail=f"{field['label']} must be an email address"
                    ) from None
                value = value.lower()
            if value:
                values[field["name"]] = value
        opted_in = str(data_in.get("_opt_in") or "").lower() == "true"
        if look["requires_opt_in"] and not opted_in:
            raise unprocessable(code="consent_required", detail="please agree to continue")
        if not values:
            raise unprocessable(code="empty_submission", detail="fill in the form first")
        key_id, data_ct = _seal(workspace, form, values)
        email = values.get("email")
        FormSubmission.objects.create(
            form=form,
            workspace_id=workspace.id,
            key_id=key_id,
            data_ct=data_ct,
            email_bidx=(
                get_keyring().blind_index(workspace.org_id, email)
                if email and workspace.org_id
                else None
            ),
            consent={
                "opted_in": opted_in,
                "notice_version": form.notice_version,
                "text": _notice(form).get("text"),
                "at": timezone.now().isoformat(),
            },
            status="pending_confirmation" if form.double_opt_in else "confirmed",
            delete_after=(timezone.now() + timedelta(days=form.retention_days)).date(),
        )
        return ok({"message": look["thank_you_message"] or "Thank you!"}, status=201)
