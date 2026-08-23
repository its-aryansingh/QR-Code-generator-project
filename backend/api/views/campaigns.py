"""Campaigns and brand templates.

Campaigns are the measurement unit competitors organise their dashboards
around: a named initiative with a window, a scan goal and UTM defaults that
every QR inside it inherits. Folders remain pure storage.
"""

import uuid
from datetime import timedelta
from urllib.parse import urlencode, urlparse, urlunparse, parse_qsl

from django.db.models import Count, Q, Sum
from django.utils import timezone
from rest_framework.response import Response
from rest_framework.views import APIView

from api import serializers as s
from api.models import Campaign, QRRecord, QRScan, QRTemplate
from api.utils import audit
from api.utils.analytics import timeseries
from api.utils.auth import require_auth
from api.utils.entitlements import feature_denied, limit_exceeded
from api.utils.rbac import require_workspace

VALID_STATUS = set(Campaign.STATUS_CHOICES)


def apply_utm(url, campaign):
    """Append the campaign's UTM defaults to `url` without clobbering
    parameters the author set explicitly."""
    if not url or not campaign:
        return url
    params = {
        "utm_source": campaign.utm_source,
        "utm_medium": campaign.utm_medium,
        "utm_campaign": campaign.utm_campaign,
        "utm_term": campaign.utm_term,
        "utm_content": campaign.utm_content,
    }
    params = {k: v for k, v in params.items() if v}
    if not params:
        return url
    try:
        parts = urlparse(url)
    except ValueError:
        return url
    existing = dict(parse_qsl(parts.query, keep_blank_values=True))
    for key, value in params.items():
        existing.setdefault(key, value)
    return urlunparse(parts._replace(query=urlencode(existing)))


def _campaign_stats(ws_id, campaign_ids, days=30):
    """QR and scan counts per campaign. Campaign membership is stored in the
    QR record's metadata so no schema change was needed on the legacy table."""
    if not campaign_ids:
        return {}
    since = timezone.now() - timedelta(days=days)
    stats = {str(cid): {"qr_count": 0, "scan_count": 0, "recent_scans": 0} for cid in campaign_ids}

    rows = QRRecord.objects.filter(workspace_id=ws_id).values("id", "metadata", "scan_count")
    qr_to_campaign = {}
    for row in rows:
        cid = (row["metadata"] or {}).get("campaign_id")
        if cid and str(cid) in stats:
            qr_to_campaign[row["id"]] = str(cid)
            stats[str(cid)]["qr_count"] += 1
            stats[str(cid)]["scan_count"] += int(row["scan_count"] or 0)

    if qr_to_campaign:
        recent = (
            QRScan.objects.filter(qr_record_id__in=list(qr_to_campaign), scanned_at__gte=since)
            .values("qr_record_id")
            .annotate(n=Count("id"))
        )
        for row in recent:
            cid = qr_to_campaign.get(row["qr_record_id"])
            if cid:
                stats[cid]["recent_scans"] += row["n"]
    return stats


class CampaignListView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        rows = Campaign.objects.filter(workspace_id=ws_id)
        if request.query_params.get("status"):
            rows = rows.filter(status=request.query_params["status"])
        if request.query_params.get("search"):
            term = request.query_params["search"]
            rows = rows.filter(Q(name__icontains=term) | Q(description__icontains=term))
        rows = list(rows.order_by("-created_at"))
        stats = _campaign_stats(ws_id, [r.id for r in rows])
        return Response(
            {"success": True, "data": [s.campaign(r, stats.get(str(r.id))) for r in rows]}
        )

    @require_auth
    @require_workspace("editor")
    def post(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "campaigns")
        if denial:
            return denial
        name = (request.data.get("name") or "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)

        current = Campaign.objects.filter(workspace_id=ws_id).count()
        denial = limit_exceeded(request.workspace.plan or "free", "max_campaigns", current, "campaigns")
        if denial:
            return denial

        status = request.data.get("status") or "draft"
        if status not in VALID_STATUS:
            return Response(
                {"success": False, "error": f"status must be one of {', '.join(sorted(VALID_STATUS))}"},
                status=400,
            )

        utm = request.data.get("utm") or {}
        now = timezone.now()
        row = Campaign(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            name=name,
            description=request.data.get("description"),
            status=status,
            color=request.data.get("color") or "#8B5CF6",
            starts_at=request.data.get("starts_at") or None,
            ends_at=request.data.get("ends_at") or None,
            scan_goal=int(request.data.get("scan_goal") or 0),
            utm_source=utm.get("source") or request.data.get("utm_source"),
            utm_medium=utm.get("medium") or request.data.get("utm_medium"),
            utm_campaign=utm.get("campaign") or request.data.get("utm_campaign"),
            utm_term=utm.get("term") or request.data.get("utm_term"),
            utm_content=utm.get("content") or request.data.get("utm_content"),
            tags=",".join(request.data.get("tags") or []) or None,
            created_by=request.auth_user["id"],
            created_at=now,
            updated_at=now,
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "campaign", row.id, {"name": name, "status": status})
        return Response({"success": True, "data": s.campaign(row, {"qr_count": 0, "scan_count": 0})}, status=201)


class CampaignDetailView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id, campaign_id):
        row = Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Campaign not found"}, status=404)
        stats = _campaign_stats(ws_id, [row.id])
        data = s.campaign(row, stats.get(str(row.id)))
        data["qr_codes"] = [
            {
                "id": str(q.id),
                "title": q.title,
                "qr_type": q.qr_type,
                "short_code": q.short_code,
                "scan_count": int(q.scan_count or 0),
                "is_active": q.is_active,
            }
            for q in QRRecord.objects.filter(workspace_id=ws_id).order_by("-scan_count")
            if (q.metadata or {}).get("campaign_id") == str(row.id)
        ]
        return Response({"success": True, "data": data})

    @require_auth
    @require_workspace("editor")
    def put(self, request, ws_id, campaign_id):
        row = Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Campaign not found"}, status=404)

        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("name", "description", "color", "starts_at", "ends_at"):
            if field in d:
                updates[field] = d[field] or None
        if "status" in d:
            if d["status"] not in VALID_STATUS:
                return Response({"success": False, "error": "Invalid status"}, status=400)
            updates["status"] = d["status"]
        if "scan_goal" in d:
            updates["scan_goal"] = int(d["scan_goal"] or 0)
        if "tags" in d:
            updates["tags"] = ",".join(d["tags"] or []) or None
        utm = d.get("utm") or {}
        for key, column in (
            ("source", "utm_source"), ("medium", "utm_medium"), ("campaign", "utm_campaign"),
            ("term", "utm_term"), ("content", "utm_content"),
        ):
            if key in utm:
                updates[column] = utm[key] or None

        Campaign.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "campaign", campaign_id,
                     {"fields": sorted(k for k in updates if k != "updated_at")})
        stats = _campaign_stats(ws_id, [row.id])
        return Response({"success": True, "data": s.campaign(row, stats.get(str(row.id)))})

    @require_auth
    @require_workspace("editor")
    def delete(self, request, ws_id, campaign_id):
        row = Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Campaign not found"}, status=404)
        # Unlink the QR codes; deleting a campaign must never break a live code.
        for qr in QRRecord.objects.filter(workspace_id=ws_id):
            meta = qr.metadata or {}
            if meta.get("campaign_id") == str(campaign_id):
                meta.pop("campaign_id", None)
                QRRecord.objects.filter(pk=qr.pk).update(metadata=meta)
        name = row.name
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "campaign", campaign_id, {"name": name})
        return Response({"success": True, "data": {"message": "Campaign deleted"}})


class CampaignAnalyticsView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id, campaign_id):
        row = Campaign.objects.filter(id=campaign_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Campaign not found"}, status=404)
        days = max(1, min(int(request.query_params.get("days", 30)), 365))
        since = timezone.now() - timedelta(days=days)

        qr_ids = [
            q["id"]
            for q in QRRecord.objects.filter(workspace_id=ws_id).values("id", "metadata")
            if (q["metadata"] or {}).get("campaign_id") == str(campaign_id)
        ]
        scans = QRScan.objects.filter(qr_record_id__in=qr_ids, scanned_at__gte=since)
        total = scans.count()
        goal = int(row.scan_goal or 0)

        return Response(
            {
                "success": True,
                "data": {
                    "campaign": s.campaign(row),
                    "range_days": days,
                    "total_scans": total,
                    "qr_count": len(qr_ids),
                    "goal": goal,
                    "goal_progress": round(min(100.0, total / goal * 100), 1) if goal else None,
                    "scans_by_date": timeseries(scans, since, days),
                    "by_country": list(
                        scans.values("country_code", "country_name")
                        .annotate(count=Count("id")).order_by("-count")[:10]
                    ),
                    "by_device": list(
                        scans.values("device_type").annotate(count=Count("id")).order_by("-count")
                    ),
                    "top_qr": [
                        {
                            "id": str(q.id), "title": q.title,
                            "short_code": q.short_code, "scan_count": int(q.scan_count or 0),
                        }
                        for q in QRRecord.objects.filter(id__in=qr_ids).order_by("-scan_count")[:10]
                    ],
                },
            }
        )


# ------------------------------------------------------------------ templates


class TemplateListView(APIView):
    @require_auth
    @require_workspace("viewer")
    def get(self, request, ws_id):
        rows = QRTemplate.objects.filter(workspace_id=ws_id).order_by("-is_default", "name")
        return Response({"success": True, "data": [s.template(r) for r in rows]})

    @require_auth
    @require_workspace("admin")
    def post(self, request, ws_id):
        denial = feature_denied(request.workspace.plan or "free", "templates")
        if denial:
            return denial
        name = (request.data.get("name") or "").strip()
        if not name:
            return Response({"success": False, "error": "name is required"}, status=400)
        current = QRTemplate.objects.filter(workspace_id=ws_id).count()
        denial = limit_exceeded(request.workspace.plan or "free", "max_templates", current, "templates")
        if denial:
            return denial

        is_default = bool(request.data.get("is_default"))
        if is_default:
            QRTemplate.objects.filter(workspace_id=ws_id).update(is_default=False)

        now = timezone.now()
        row = QRTemplate(
            id=uuid.uuid4(),
            workspace_id=ws_id,
            name=name,
            description=request.data.get("description"),
            design=request.data.get("design") or {},
            preview_url=request.data.get("preview_url"),
            is_default=is_default,
            is_locked=bool(request.data.get("is_locked")),
            created_by=request.auth_user["id"],
            created_at=now,
            updated_at=now,
        )
        row.save(force_insert=True)
        audit.record(request, ws_id, audit.CREATE, "template", row.id,
                     {"name": name, "locked": row.is_locked})
        return Response({"success": True, "data": s.template(row)}, status=201)


class TemplateDetailView(APIView):
    @require_auth
    @require_workspace("admin")
    def put(self, request, ws_id, template_id):
        row = QRTemplate.objects.filter(id=template_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Template not found"}, status=404)
        d = request.data
        updates = {"updated_at": timezone.now()}
        for field in ("name", "description", "preview_url"):
            if field in d:
                updates[field] = d[field]
        if "design" in d:
            updates["design"] = d["design"] or {}
        if "is_locked" in d:
            updates["is_locked"] = bool(d["is_locked"])
        if "is_default" in d:
            updates["is_default"] = bool(d["is_default"])
            if updates["is_default"]:
                QRTemplate.objects.filter(workspace_id=ws_id).exclude(pk=row.pk).update(is_default=False)

        QRTemplate.objects.filter(pk=row.pk).update(**updates)
        row.refresh_from_db()
        audit.record(request, ws_id, audit.UPDATE, "template", template_id, {"name": row.name})
        return Response({"success": True, "data": s.template(row)})

    @require_auth
    @require_workspace("admin")
    def delete(self, request, ws_id, template_id):
        row = QRTemplate.objects.filter(id=template_id, workspace_id=ws_id).first()
        if not row:
            return Response({"success": False, "error": "Template not found"}, status=404)
        name = row.name
        row.delete()
        audit.record(request, ws_id, audit.DELETE, "template", template_id, {"name": name})
        return Response({"success": True, "data": {"message": "Template deleted"}})
