"""Workspace-scoped QR codes, bulk operations and bulk generation jobs.

The old QR endpoints filtered on `user_id`, so two people in the same
workspace could not see each other's codes -- which defeats the point of a
shared workspace. Everything here is scoped to the workspace and gated by
role instead.
"""

import csv
import io
import json
import uuid
from datetime import timedelta

from django.db.models import Count, Q, Sum
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api.models import BulkJob, Campaign, Folder, QRRecord, QRTemplate, RoutingRule
from api.utils import audit, gs1
from api.utils.auth import require_auth
from api.utils.entitlements import feature_denied, limit_exceeded, limit_for
from api.utils.qr import generate_qr_base64, generate_short_code
from api.utils.rbac import require_workspace
from api.views.campaigns import apply_utm
from api.views.governance import check_destination
from api.views.qr import _serialize_qr
from django.conf import settings

SORT_FIELDS = {
    "created": "-created_at",
    "created_asc": "created_at",
    "scans": "-scan_count",
    "scans_asc": "scan_count",
    "title": "title",
    "updated": "-updated_at",
}


def _short_url(qr, workspace):
    if not qr.short_code:
        return None
    domain = workspace.custom_domain if workspace and workspace.custom_domain else None
    base = f"https://{domain}" if domain else settings.SHORT_LINK_BASE_URL
    return f"{base}/r/{qr.short_code}"


def _decorate(qr, workspace, folder_names=None, campaign_names=None):
    data = _serialize_qr(qr)
    data["short_url"] = _short_url(qr, workspace)
    meta = qr.metadata or {}
    campaign_id = meta.get("campaign_id")
    data["campaign_id"] = campaign_id
    data["campaign_name"] = (campaign_names or {}).get(campaign_id)
    data["folder_name"] = (folder_names or {}).get(str(qr.folder_id)) if qr.folder_id else None
    data["template_id"] = meta.get("template_id")
    data["gs1"] = meta.get("gs1")
    data["tags"] = [t.strip() for t in (qr.tags or "").split(",") if t.strip()]
    return data


class WorkspaceQRListView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        q = request.query_params
        limit = min(max(int(q.get("limit", 25)), 1), 100)
        page = max(int(q.get("page", 1)), 1)

        rows = QRRecord.objects.filter(workspace_id=ws_id)
        if q.get("folder_id"):
            rows = rows.filter(folder_id=q["folder_id"])
        if q.get("type"):
            rows = rows.filter(qr_type=q["type"])
        if q.get("status") == "active":
            rows = rows.filter(is_active=True)
        elif q.get("status") == "inactive":
            rows = rows.filter(is_active=False)
        elif q.get("status") == "expired":
            rows = rows.filter(expires_at__lt=timezone.now())
        if q.get("dynamic") == "true":
            rows = rows.filter(is_dynamic=True)
        elif q.get("dynamic") == "false":
            rows = rows.filter(is_dynamic=False)
        if q.get("tag"):
            rows = rows.filter(tags__icontains=q["tag"])
        if q.get("search"):
            term = q["search"]
            rows = rows.filter(
                Q(title__icontains=term) | Q(content__icontains=term) | Q(short_code__icontains=term)
            )
        if q.get("campaign_id"):
            # campaign membership lives in metadata; filter in Python so the
            # query stays portable across SQLite and Postgres JSON support
            wanted = q["campaign_id"]
            ids = [
                row["id"]
                for row in rows.values("id", "metadata")
                if (row["metadata"] or {}).get("campaign_id") == wanted
            ]
            rows = rows.filter(id__in=ids)

        total = rows.count()
        order = SORT_FIELDS.get(q.get("sort", "created"), "-created_at")
        window = list(rows.order_by(order)[(page - 1) * limit : page * limit])

        folder_names = {
            str(f["id"]): f["name"] for f in Folder.objects.filter(workspace_id=ws_id).values("id", "name")
        }
        campaign_names = {
            str(c["id"]): c["name"] for c in Campaign.objects.filter(workspace_id=ws_id).values("id", "name")
        }

        return Response(
            {
                "success": True,
                "data": {
                    "items": [_decorate(r, request.workspace, folder_names, campaign_names) for r in window],
                    "total": total,
                    "page": page,
                    "limit": limit,
                    "pages": (total + limit - 1) // limit,
                    "facets": {
                        "types": list(
                            QRRecord.objects.filter(workspace_id=ws_id)
                            .values("qr_type").annotate(count=Count("id")).order_by("-count")
                        ),
                        "total_scans": int(
                            QRRecord.objects.filter(workspace_id=ws_id)
                            .aggregate(n=Sum("scan_count"))["n"] or 0
                        ),
                    },
                },
            }
        )

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        d = dict(request.data)
        plan = request.workspace.plan or "free"

        current = QRRecord.objects.filter(workspace_id=ws_id).count()
        denial = limit_exceeded(plan, "max_qr_codes", current, "QR codes")
        if denial:
            return denial

        record, error, status = _create_workspace_qr(request, ws_id, d, plan)
        if error:
            return Response({"success": False, "error": error}, status=status)

        audit.record(request, ws_id, audit.CREATE, "qr", record.id,
                     {"title": record.title, "type": record.qr_type})
        data = _decorate(record, request.workspace)
        data["qr_base64"] = generate_qr_base64(record.content, int(record.size or 256),
                                               d.get("format", "png"))
        return Response({"success": True, "data": data}, status=201)


def _create_workspace_qr(request, ws_id, d, plan):
    """Shared create path for single and bulk creation.

    Returns (record, error_message, http_status).
    """
    qr_type = d.get("qr_type") or "url"
    content = (d.get("content") or "").strip()

    # GS1 Digital Link codes are composed from structured attributes rather
    # than a raw URL, so build the URI before anything else validates content.
    gs1_meta = None
    if qr_type == "gs1":
        denial = feature_denied(plan, "gs1")
        if denial is not None:
            return None, "GS1 Digital Link requires the pro plan or higher", 402
        attributes = d.get("gs1") or {}
        uri, errors = gs1.build_digital_link(
            attributes.get("domain") or request.workspace.custom_domain, attributes
        )
        if errors:
            return None, "; ".join(errors), 400
        content = uri
        gs1_meta = {**attributes, "uri": uri}

    if not content:
        return None, "content is required", 400

    # Campaign UTMs are stamped onto the destination at creation time so the
    # printed code and the analytics agree forever.
    campaign = None
    campaign_id = d.get("campaign_id")
    if campaign_id:
        campaign = Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).first()
        if not campaign:
            return None, "Campaign not found in this workspace", 404
        if qr_type in ("url", "gs1"):
            content = apply_utm(content, campaign)

    policy_error = check_destination(ws_id, content)
    if policy_error:
        return None, policy_error, 403

    if d.get("folder_id"):
        if not Folder.objects.filter(id=d["folder_id"], workspace_id=ws_id).exists():
            return None, "Folder not found in this workspace", 404

    customization = d.get("customization") or {}
    template_id = d.get("template_id")
    if template_id:
        template = QRTemplate.objects.filter(id=template_id, workspace_id=ws_id).first()
        if not template:
            return None, "Template not found in this workspace", 404
        # A locked template wins over anything the caller sent; that is the
        # guarantee brand teams are buying.
        customization = {**customization, **(template.design or {})} if template.is_locked \
            else {**(template.design or {}), **customization}
        QRTemplate.objects.filter(pk=template.pk).update(usage_count=template.usage_count + 1)

    is_dynamic = bool(d.get("is_dynamic", qr_type == "gs1"))
    metadata = dict(d.get("metadata") or {})
    if campaign_id:
        metadata["campaign_id"] = str(campaign_id)
    if template_id:
        metadata["template_id"] = str(template_id)
    if gs1_meta:
        metadata["gs1"] = gs1_meta

    tags = d.get("tags")
    if isinstance(tags, list):
        tags = ",".join(str(t).strip() for t in tags if str(t).strip())

    now = timezone.now()
    record = QRRecord(
        id=uuid.uuid4(),
        user_id=request.auth_user["id"],
        workspace_id=ws_id,
        folder_id=d.get("folder_id") or None,
        tags=tags or None,
        title=d.get("title") or None,
        content=content,
        qr_type=qr_type,
        size=max(50, min(int(d.get("size") or 512), 4096)),
        is_dynamic=is_dynamic,
        short_code=generate_short_code() if is_dynamic else None,
        redirect_url=content if is_dynamic else None,
        metadata=metadata,
        customization=customization,
        password=d.get("password") or None,
        max_scans=d.get("max_scans") or None,
        expires_at=d.get("expires_at") or None,
        scheduled_at=d.get("scheduled_at") or None,
        geo_restrictions=d.get("geo_restrictions") or None,
        created_at=now,
        updated_at=now,
    )
    record.save(force_insert=True)
    return record, None, 201


class WorkspaceQRBulkActionView(APIView):
    """Apply one action to many QR codes: the table's multi-select toolbar."""

    ACTIONS = ("move", "tag", "untag", "activate", "deactivate", "assign_campaign",
               "clear_campaign", "delete")

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        action = request.data.get("action")
        ids = request.data.get("qr_ids") or []
        if action not in self.ACTIONS:
            return Response(
                {"success": False, "error": f"action must be one of {', '.join(self.ACTIONS)}"},
                status=400,
            )
        if not ids:
            return Response({"success": False, "error": "qr_ids is required"}, status=400)

        rows = QRRecord.objects.filter(workspace_id=ws_id, id__in=ids)
        matched = rows.count()
        if not matched:
            return Response({"success": False, "error": "No matching QR codes"}, status=404)

        now = timezone.now()
        if action == "move":
            folder_id = request.data.get("folder_id") or None
            if folder_id and not Folder.objects.filter(id=folder_id, workspace_id=ws_id).exists():
                return Response({"success": False, "error": "Folder not found"}, status=404)
            rows.update(folder_id=folder_id, updated_at=now)

        elif action in ("tag", "untag"):
            incoming = request.data.get("tags") or []
            if isinstance(incoming, str):
                incoming = [incoming]
            incoming = {t.strip() for t in incoming if t.strip()}
            if not incoming:
                return Response({"success": False, "error": "tags is required"}, status=400)
            for row in rows:
                existing = {t.strip() for t in (row.tags or "").split(",") if t.strip()}
                merged = existing | incoming if action == "tag" else existing - incoming
                QRRecord.objects.filter(pk=row.pk).update(
                    tags=",".join(sorted(merged)) or None, updated_at=now
                )

        elif action in ("activate", "deactivate"):
            rows.update(is_active=action == "activate", updated_at=now)

        elif action == "assign_campaign":
            campaign_id = request.data.get("campaign_id")
            if not Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).exists():
                return Response({"success": False, "error": "Campaign not found"}, status=404)
            for row in rows:
                meta = dict(row.metadata or {})
                meta["campaign_id"] = str(campaign_id)
                QRRecord.objects.filter(pk=row.pk).update(metadata=meta, updated_at=now)

        elif action == "clear_campaign":
            for row in rows:
                meta = dict(row.metadata or {})
                meta.pop("campaign_id", None)
                QRRecord.objects.filter(pk=row.pk).update(metadata=meta, updated_at=now)

        elif action == "delete":
            if request.workspace_role not in ("admin", "owner"):
                return Response(
                    {"success": False, "error": "Only admins can bulk delete QR codes",
                     "code": "insufficient_role"},
                    status=403,
                )
            rows.delete()

        audit.record(request, ws_id, action, "qr_bulk", None, {"count": matched, "action": action})
        return Response({"success": True, "data": {"action": action, "affected": matched}})


# ------------------------------------------------------------------ bulk jobs


BULK_COLUMNS = ["content", "title", "qr_type", "folder", "campaign", "tags", "is_dynamic"]


class BulkJobListView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        from api import serializers as s

        rows = BulkJob.objects.filter(workspace_id=ws_id).order_by("-created_at")[:50]
        return Response(
            {
                "success": True,
                "data": {
                    "jobs": [s.bulk_job(r) for r in rows],
                    "columns": BULK_COLUMNS,
                    "max_rows": limit_for(request.workspace.plan or "free", "bulk_batch_size"),
                    "template_csv": ",".join(BULK_COLUMNS) + "\n"
                                    "https://acme.com/product-a,Product A,url,Packaging,Spring Launch,print;retail,true",
                },
            }
        )

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        """Generate many QR codes from CSV text or a JSON row array."""
        from api import serializers as s

        plan = request.workspace.plan or "free"
        denial = feature_denied(plan, "bulk")
        if denial:
            return denial

        rows, parse_error = _parse_bulk_input(request.data)
        if parse_error:
            return Response({"success": False, "error": parse_error}, status=400)

        max_rows = limit_for(plan, "bulk_batch_size")
        if len(rows) > max_rows:
            return Response(
                {
                    "success": False,
                    "error": f"Your {plan} plan allows {max_rows} rows per batch; "
                             f"this file has {len(rows)}.",
                    "code": "limit_reached",
                    "limit": max_rows,
                },
                status=402,
            )

        existing = QRRecord.objects.filter(workspace_id=ws_id).count()
        cap = limit_for(plan, "max_qr_codes")
        if existing + len(rows) > cap:
            return Response(
                {
                    "success": False,
                    "error": f"This batch would take you to {existing + len(rows)} QR codes; "
                             f"your {plan} plan allows {cap}.",
                    "code": "limit_reached",
                    "limit": cap,
                },
                status=402,
            )

        now = timezone.now()
        job = BulkJob(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            created_by=request.auth_user["id"],
            filename=request.data.get("filename") or "bulk-upload.csv",
            status="processing",
            total_rows=len(rows),
            options={
                "size": int(request.data.get("size") or 512),
                "format": request.data.get("format") or "png",
                "template_id": request.data.get("template_id"),
                "folder_id": request.data.get("folder_id"),
                "campaign_id": request.data.get("campaign_id"),
                "is_dynamic": bool(request.data.get("is_dynamic", True)),
            },
            created_at=now,
        )
        job.save(force_insert=True)

        folders = {
            f["name"].lower(): str(f["id"])
            for f in Folder.objects.filter(workspace_id=ws_id).values("id", "name")
        }
        campaigns = {
            c["name"].lower(): str(c["id"])
            for c in Campaign.objects.filter(workspace_id=ws_id).values("id", "name")
        }

        results, errors = [], []
        for index, row in enumerate(rows, start=1):
            payload = _bulk_row_to_payload(row, job.options, folders, campaigns)
            record, error, _status = _create_workspace_qr(request, ws_id, payload, plan)
            if error:
                errors.append({"row": index, "content": row.get("content", ""), "error": error})
                continue
            results.append(
                {
                    "row": index,
                    "id": str(record.id),
                    "title": record.title,
                    "content": record.content,
                    "short_code": record.short_code,
                    "short_url": _short_url(record, request.workspace),
                    "qr_base64": generate_qr_base64(
                        record.content, job.options["size"], job.options["format"]
                    ),
                }
            )

        BulkJob.objects.filter(pk=job.pk).update(
            status="completed" if not errors else ("partial" if results else "failed"),
            success_count=len(results),
            error_count=len(errors),
            errors=json.dumps(errors[:200]),
            completed_at=timezone.now(),
        )
        job.refresh_from_db()
        audit.record(request, ws_id, audit.CREATE, "bulk_job", job.id,
                     {"total": len(rows), "created": len(results), "failed": len(errors)})

        return Response(
            {"success": True, "data": {"job": s.bulk_job(job), "results": results, "errors": errors}},
            status=201,
        )


class BulkJobDetailView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id, job_id):
        from api import serializers as s

        row = BulkJob.objects.filter(id=job_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Job not found"}, status=404)
        return Response({"success": True, "data": s.bulk_job(row)})


def _parse_bulk_input(payload):
    """Accept either `csv` text or a `rows` array. Returns (rows, error)."""
    if payload.get("rows"):
        rows = payload["rows"]
        if not isinstance(rows, list):
            return None, "rows must be an array"
        return [r for r in rows if isinstance(r, dict)], None

    text = payload.get("csv")
    if not text:
        return None, "Provide either csv text or a rows array"
    try:
        reader = csv.DictReader(io.StringIO(text.strip()))
        rows = [
            {(k or "").strip().lower(): (v or "").strip() for k, v in row.items() if k}
            for row in reader
        ]
    except csv.Error as exc:
        return None, f"Could not parse the CSV: {exc}"
    if not rows:
        return None, "The CSV had no data rows"
    if not any(row.get("content") for row in rows):
        return None, "The CSV needs a 'content' column"
    return rows, None


def _bulk_row_to_payload(row, options, folders, campaigns):
    """Map one spreadsheet row onto the QR creation payload."""
    folder_name = (row.get("folder") or "").strip().lower()
    campaign_name = (row.get("campaign") or "").strip().lower()
    raw_dynamic = str(row.get("is_dynamic", "")).strip().lower()
    tags = row.get("tags") or ""
    return {
        "content": row.get("content") or row.get("url") or "",
        "title": row.get("title") or None,
        "qr_type": row.get("qr_type") or row.get("type") or "url",
        "folder_id": row.get("folder_id") or folders.get(folder_name) or options.get("folder_id"),
        "campaign_id": row.get("campaign_id") or campaigns.get(campaign_name) or options.get("campaign_id"),
        "tags": [t.strip() for t in tags.replace(";", ",").split(",") if t.strip()],
        "is_dynamic": raw_dynamic in ("true", "1", "yes", "y")
        if raw_dynamic else options.get("is_dynamic", True),
        "size": options.get("size", 512),
        "template_id": options.get("template_id"),
    }


# ------------------------------------------------------------------ routing


class RoutingRuleListView(APIView):
    """Smart routing rules for one dynamic QR code."""

    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id, qr_id):
        from api import serializers as s

        qr = QRRecord.objects.filter(id=qr_id, workspace_id=ws_id).first()
        if not qr:
            return Response({"success": False, "error": "QR code not found"}, status=404)
        rows = RoutingRule.objects.filter(qr_record_id=qr_id)
        return Response(
            {
                "success": True,
                "data": {
                    "rules": [s.routing_rule(r) for r in rows],
                    "default_destination": qr.redirect_url or qr.content,
                    "conditions": RoutingRule.CONDITIONS,
                    "available": feature_denied(request.workspace.plan or "free", "routing") is None,
                },
            }
        )

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id, qr_id):
        from api import serializers as s

        denial = feature_denied(request.workspace.plan or "free", "routing")
        if denial:
            return denial
        qr = QRRecord.objects.filter(id=qr_id, workspace_id=ws_id).first()
        if not qr:
            return Response({"success": False, "error": "QR code not found"}, status=404)
        if not qr.is_dynamic:
            return Response(
                {"success": False, "error": "Only dynamic QR codes can be routed"}, status=400
            )

        condition = request.data.get("condition")
        if condition not in RoutingRule.CONDITIONS:
            return Response(
                {"success": False,
                 "error": f"condition must be one of {', '.join(RoutingRule.CONDITIONS)}"},
                status=400,
            )
        destination = (request.data.get("destination_url") or "").strip()
        if not destination:
            return Response({"success": False, "error": "destination_url is required"}, status=400)
        policy_error = check_destination(ws_id, destination)
        if policy_error:
            return Response({"success": False, "error": policy_error}, status=403)

        value = request.data.get("value")
        if isinstance(value, list):
            value = ",".join(str(v).strip() for v in value)
        if condition == "weight":
            try:
                weight = int(value)
            except (TypeError, ValueError):
                return Response(
                    {"success": False, "error": "A weight rule needs a percentage between 1 and 100"},
                    status=400,
                )
            if not 1 <= weight <= 100:
                return Response(
                    {"success": False, "error": "Weight must be between 1 and 100"}, status=400
                )
            value = str(weight)

        row = RoutingRule(
            id=uuid.uuid4(),
            qr_record_id=qr_id,
            name=request.data.get("name") or None,
            condition=condition,
            operator=request.data.get("operator") or "in",
            value=value,
            destination_url=destination,
            priority=int(request.data.get("priority") or 0),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "routing_rule", row.id,
                     {"qr_id": str(qr_id), "condition": condition})
        return Response({"success": True, "data": s.routing_rule(row)}, status=201)


class RoutingRuleDetailView(APIView):
    @require_auth
    @require_workspace("editor")
    def put(self, request, ws_id, qr_id, rule_id):
        from api import serializers as s

        row = RoutingRule.objects.filter(id=rule_id, qr_record_id=qr_id).first()
        if not row or not QRRecord.objects.filter(id=qr_id, workspace_id=ws_id).exists():
            return Response({"success": False, "error": "Rule not found"}, status=404)
        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("name", "operator"):
            if field in d:
                updates[field] = d[field]
        if "destination_url" in d:
            policy_error = check_destination(ws_id, d["destination_url"])
            if policy_error:
                return Response({"success": False, "error": policy_error}, status=403)
            updates["destination_url"] = d["destination_url"]
        if "value" in d:
            value = d["value"]
            updates["value"] = ",".join(str(v) for v in value) if isinstance(value, list) else value
        if "priority" in d:
            updates["priority"] = int(d["priority"] or 0)
        if "is_active" in d:
            updates["is_active"] = bool(d["is_active"])

        RoutingRule.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "routing_rule", rule_id, {"qr_id": str(qr_id)})
        return Response({"success": True, "data": s.routing_rule(row)})

    @require_auth
    @require_workspace("editor")
    def delete(self, request, ws_id, qr_id, rule_id):
        row = RoutingRule.objects.filter(id=rule_id, qr_record_id=qr_id).first()
        if not row or not QRRecord.objects.filter(id=qr_id, workspace_id=ws_id).exists():
            return Response({"success": False, "error": "Rule not found"}, status=404)
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "routing_rule", rule_id, {"qr_id": str(qr_id)})
        return Response({"success": True, "data": {"message": "Rule deleted"}})
