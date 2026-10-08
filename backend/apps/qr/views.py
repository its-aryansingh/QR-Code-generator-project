"""QR code endpoints for the dashboard: list, create, view, edit, pause, delete, bulk."""

from typing import Any
from uuid import UUID

from django.db.models import Count, Prefetch, Q, QuerySet, Sum
from django.utils import timezone
from rest_framework.request import Request
from rest_framework.response import Response

from apps.core.errors import not_found, unprocessable
from apps.workspaces.access import (
    DashboardAPIView,
    member_role,
    ok,
    require_role,
    workspace_for,
)
from apps.workspaces.models import Workspace

from . import dashboard
from .models import QRCode, QRCodeTag

SORTS = {
    "created": "-created_at",
    "created_asc": "created_at",
    "scans": "-total_scans",
    "scans_asc": "total_scans",
    "title": "name",
    "updated": "-updated_at",
}


def _body(request: Request) -> dict[str, Any]:
    return request.data if isinstance(request.data, dict) else {}


def _with_tags(rows: QuerySet[QRCode]) -> list[dict[str, Any]]:
    rows = rows.select_related("folder", "campaign", "current_version").prefetch_related(
        Prefetch("code_tags", queryset=QRCodeTag.objects.select_related("tag"))
    )
    return [
        dashboard.serialize(qr, sorted(str(link.tag.name) for link in qr.code_tags.all()))
        for qr in rows
    ]


def _qr_for(
    user: Any, qr_id: str | UUID, ws: str | None = None, min_role: str = "analyst"
) -> QRCode:
    """The QR code if the user can see its workspace.

    The dashboard's detail page sends the workspace id from localStorage,
    which is `null` until the user has switched workspace once; the QR id is
    enough to find its workspace, so a non-UUID workspace is ignored.
    """
    try:
        qr = (
            QRCode.objects.select_related("workspace", "folder", "campaign", "current_version")
            .filter(id=UUID(str(qr_id)), deleted_at__isnull=True)
            .first()
        )
    except ValueError:
        qr = None
    if qr is None:
        raise not_found("QR code not found")
    if ws:
        try:
            if UUID(ws) != qr.workspace_id:
                raise not_found("QR code not found")
        except ValueError:
            pass
    role = member_role(user, qr.workspace)
    if role is None or qr.workspace.deleted_at is not None:
        raise not_found("QR code not found")
    require_role(role, min_role)
    return qr


def _filtered(workspace: Workspace, params: Any) -> QuerySet[QRCode]:
    rows = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True)
    search = (params.get("search") or "").strip()
    if search:
        rows = rows.filter(
            Q(name__icontains=search)
            | Q(short_code__icontains=search)
            | Q(static_payload__icontains=search)
            | Q(current_version__destination_url__icontains=search)
        )
    if params.get("folder_id"):
        try:
            rows = rows.filter(folder_id=UUID(params["folder_id"]))
        except ValueError:
            rows = rows.none()
    if params.get("campaign_id"):
        try:
            rows = rows.filter(campaign_id=UUID(params["campaign_id"]))
        except ValueError:
            rows = rows.none()
    if params.get("type"):
        qr_type = params["type"]
        rows = rows.filter(Q(design__qr_type=qr_type) | Q(content_type=qr_type))
    status = params.get("status")
    now = timezone.now()
    expired = Q(expires_at__lte=now)
    if status == "active":
        rows = rows.filter(status="active").exclude(expired)
    elif status == "inactive":
        rows = rows.exclude(status="active")
    elif status == "expired":
        rows = rows.filter(expired)
    dynamic = params.get("dynamic")
    if dynamic in ("true", "1"):
        rows = rows.filter(mode="dynamic")
    elif dynamic in ("false", "0"):
        rows = rows.filter(mode="static")
    if params.get("tag"):
        rows = rows.filter(code_tags__tag__name=params["tag"])
    return rows


class WorkspaceQRListView(DashboardAPIView):
    list_view = True

    def get(self, request: Request, ws: str) -> Response:
        workspace, _ = workspace_for(self.user(request), ws)
        params = request.query_params
        try:
            page = max(1, int(params.get("page") or 1))
            limit = min(100, max(1, int(params.get("limit") or 25)))
        except ValueError:
            raise unprocessable(
                code="invalid_page", detail="page and limit must be numbers"
            ) from None
        rows = _filtered(workspace, params).order_by(
            SORTS.get(params.get("sort") or "", "-created_at"), "-id"
        )
        total = rows.count()
        everything = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True)
        types = [
            {"qr_type": row["design__qr_type"] or row["content_type"], "count": row["n"]}
            for row in everything.values("design__qr_type", "content_type")
            .annotate(n=Count("id"))
            .order_by("-n")
        ]
        return ok(
            {
                "items": _with_tags(rows[(page - 1) * limit : page * limit]),
                "total": total,
                "page": page,
                "limit": limit,
                "pages": (total + limit - 1) // limit,
                "facets": {
                    "types": types,
                    "total_scans": everything.aggregate(n=Sum("total_scans"))["n"] or 0,
                },
            }
        )

    def post(self, request: Request, ws: str) -> Response:
        user = self.user(request)
        workspace, _ = workspace_for(user, ws, min_role="editor")
        qr = dashboard.create(workspace, user, _body(request))
        return ok(_created(qr), status=201)


def _created(qr: QRCode) -> dict[str, Any]:
    data = dashboard.serialize(qr, [])
    data["qr_base64"] = dashboard.render_png_data_url(
        dashboard.encoded_payload(qr), data["size"], data["customization"]
    )
    return data


def _detail(user: Any, qr_id: str, ws: str | None) -> Response:
    qr = _qr_for(user, qr_id, ws)
    data = dashboard.serialize(qr)
    data["qr_base64"] = dashboard.render_png_data_url(
        dashboard.encoded_payload(qr), 300, data["customization"]
    )
    return ok(data)


def _edit(request: Request, user: Any, qr_id: str, ws: str | None) -> Response:
    qr = _qr_for(user, qr_id, ws, min_role="editor")
    return ok(dashboard.serialize(dashboard.update(qr, user, _body(request))))


def _delete(user: Any, qr_id: str, ws: str | None) -> Response:
    qr = _qr_for(user, qr_id, ws, min_role="editor")
    qr.deleted_at = timezone.now()
    qr.save(update_fields=["deleted_at"])
    return ok({"message": "QR code deleted"})


class WorkspaceQRDetailView(DashboardAPIView):
    def get(self, request: Request, ws: str, qr_id: str) -> Response:
        return _detail(self.user(request), qr_id, ws)

    def put(self, request: Request, ws: str, qr_id: str) -> Response:
        return _edit(request, self.user(request), qr_id, ws)

    def delete(self, request: Request, ws: str, qr_id: str) -> Response:
        return _delete(self.user(request), qr_id, ws)


class QRDetailView(DashboardAPIView):
    """The same, addressed by QR id alone (`/qr/<id>`)."""

    def get(self, request: Request, qr_id: str) -> Response:
        return _detail(self.user(request), qr_id, None)

    def put(self, request: Request, qr_id: str) -> Response:
        return _edit(request, self.user(request), qr_id, None)

    def delete(self, request: Request, qr_id: str) -> Response:
        return _delete(self.user(request), qr_id, None)


class QRToggleView(DashboardAPIView):
    def put(self, request: Request, qr_id: str) -> Response:
        qr = _qr_for(self.user(request), qr_id, min_role="editor")
        qr.status = "paused" if qr.status == "active" else "active"
        qr.updated_at = timezone.now()
        qr.save(update_fields=["status", "updated_at"])
        return ok(dashboard.serialize(qr))


class QRGenerateView(DashboardAPIView):
    """The create page's endpoint.

    With `workspace_id` it creates the code. Without one it only renders a
    preview and saves nothing (v1 saved a record for every preview).
    """

    def post(self, request: Request) -> Response:
        user = self.user(request)
        body = _body(request)
        if body.get("workspace_id"):
            workspace, _ = workspace_for(user, body["workspace_id"], min_role="editor")
            qr = dashboard.create(workspace, user, body)
            return ok(_created(qr), status=201)
        content = str(body.get("content") or "").strip()
        if not content:
            raise unprocessable(code="content_required", detail="content is required")
        if len(content) > dashboard.MAX_CONTENT_LENGTH:
            raise unprocessable(code="content_too_long", detail="content is too long for a QR code")
        try:
            size = min(max(int(body.get("size") or 300), 50), 1024)
        except (TypeError, ValueError):
            size = 300
        customization = (
            body.get("customization") if isinstance(body.get("customization"), dict) else {}
        )
        return ok(
            {
                "content": content,
                "qr_type": body.get("qr_type") or "url",
                "size": size,
                "qr_base64": dashboard.render_png_data_url(content, size, customization),
            }
        )


BULK_ACTIONS = {"activate", "deactivate", "move", "tag", "untag", "delete"}


class WorkspaceQRBulkView(DashboardAPIView):
    def post(self, request: Request, ws: str) -> Response:
        body = _body(request)
        action = str(body.get("action") or "")
        if action not in BULK_ACTIONS:
            raise unprocessable(
                code="invalid_action",
                detail=f"action must be one of {', '.join(sorted(BULK_ACTIONS))}",
            )
        workspace, _ = workspace_for(
            self.user(request), ws, min_role="admin" if action == "delete" else "editor"
        )
        ids = []
        for raw in body.get("qr_ids") or []:
            try:
                ids.append(UUID(str(raw)))
            except ValueError:
                continue
        rows = QRCode.objects.filter(workspace=workspace, deleted_at__isnull=True, id__in=ids)
        now = timezone.now()
        if action in ("activate", "deactivate"):
            affected = rows.update(
                status="active" if action == "activate" else "paused", updated_at=now
            )
        elif action == "move":
            folder = dashboard.folder_in(workspace, body.get("folder_id"))
            affected = rows.update(folder=folder, updated_at=now)
        elif action in ("tag", "untag"):
            affected = 0
            for qr in rows:
                dashboard.set_tags(
                    qr, body.get("tags"), mode="add" if action == "tag" else "remove"
                )
                affected += 1
        else:
            affected = rows.update(deleted_at=now)
        return ok({"action": action, "affected": affected})
