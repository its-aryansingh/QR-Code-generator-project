import uuid
import re
import secrets
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import LeadCapturePage, Lead
from api.utils.auth import require_auth
from api.utils.ip import get_client_ip


def _slugify(name):
    slug = re.sub(r"[^a-z0-9-]", "", re.sub(r"\s+", "-", name.lower()))
    return f"{slug}-{secrets.token_hex(3)}"


class LeadPagesView(APIView):
    @require_auth
    def get(self, request):
        workspace_id = request.query_params.get("workspace_id")
        if not workspace_id:
            return Response({"success": False, "error": "workspace_id required"}, status=400)
        pages = list(LeadCapturePage.objects.filter(workspace_id=workspace_id).order_by("-created_at").values(
            "id", "name", "slug", "headline", "is_active", "views", "submissions", "created_at"
        ))
        return Response({"success": True, "data": pages})

    @require_auth
    def post(self, request):
        d = request.data
        name = d.get("name", "").strip()
        workspace_id = d.get("workspace_id")
        if not name or not workspace_id:
            return Response({"success": False, "error": "name and workspace_id are required"}, status=400)

        page = LeadCapturePage(
            id=uuid.uuid4(),
            workspace_id=workspace_id,
            name=name,
            slug=_slugify(name),
            headline=d.get("headline"),
            subheadline=d.get("subheadline"),
            button_text=d.get("button_text", "Submit"),
            button_color=d.get("button_color", "#8B5CF6"),
            background_color=d.get("background_color", "#09090B"),
            text_color=d.get("text_color", "#FAFAFA"),
            thank_you_message=d.get("thank_you_message", "Thank you for signing up!"),
            redirect_url=d.get("redirect_url"),
            form_fields=d.get("form_fields", "[]"),
            requires_opt_in=bool(d.get("requires_opt_in", False)),
            privacy_policy=d.get("privacy_policy_url"),
            consent_text=d.get("consent_text"),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        page.save(force_insert=True)
        return Response({"success": True, "data": {
            "id": str(page.id), "name": page.name, "slug": page.slug,
            "workspace_id": str(workspace_id),
        }}, status=201)


class LeadPageDetailView(APIView):
    @require_auth
    def get(self, request, page_id):
        try:
            page = LeadCapturePage.objects.get(id=page_id)
        except LeadCapturePage.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": {
            "id": str(page.id), "name": page.name, "slug": page.slug,
            "headline": page.headline, "subheadline": page.subheadline,
            "button_text": page.button_text, "button_color": page.button_color,
            "background_color": page.background_color, "text_color": page.text_color,
            "thank_you_message": page.thank_you_message, "redirect_url": page.redirect_url,
            "form_fields": page.form_fields, "requires_opt_in": page.requires_opt_in,
            "privacy_policy": page.privacy_policy, "consent_text": page.consent_text,
            "is_active": page.is_active, "views": page.views, "submissions": page.submissions,
        }})

    @require_auth
    def put(self, request, page_id):
        try:
            page = LeadCapturePage.objects.get(id=page_id)
        except LeadCapturePage.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        allowed = ["name", "headline", "subheadline", "button_text", "button_color",
                   "background_color", "text_color", "thank_you_message", "redirect_url",
                   "form_fields", "requires_opt_in", "is_active"]
        updates = {k: v for k, v in request.data.items() if k in allowed}
        updates["updated_at"] = timezone.now()
        LeadCapturePage.objects.filter(pk=page.pk).update(**updates)
        page.refresh_from_db()
        return Response({"success": True, "data": {"id": str(page.id), "name": page.name}})

    @require_auth
    def delete(self, request, page_id):
        LeadCapturePage.objects.filter(id=page_id).delete()
        return Response({"success": True, "data": {"message": "Page deleted"}})


class LeadSubmissionsView(APIView):
    @require_auth
    def get(self, request, page_id):
        limit = min(int(request.query_params.get("limit", 50)), 500)
        page_num = max(int(request.query_params.get("page", 1)), 1)
        qs = Lead.objects.filter(page_id=page_id).order_by("-created_at")
        total = qs.count()
        leads = list(qs[(page_num - 1) * limit: (page_num - 1) * limit + limit].values())
        return Response({"success": True, "data": {
            "leads": leads, "total": total, "page": page_num, "limit": limit,
            "pages": (total + limit - 1) // limit,
        }})


class PublicLeadPageView(APIView):
    def get(self, request, slug):
        try:
            page = LeadCapturePage.objects.get(slug=slug)
        except LeadCapturePage.DoesNotExist:
            return Response({"success": False, "error": "Page not found"}, status=404)
        if not page.is_active:
            return Response({"success": False, "error": "Page not found"}, status=404)
        LeadCapturePage.objects.filter(pk=page.pk).update(views=page.views + 1)
        return Response({"success": True, "data": {
            "id": str(page.id), "name": page.name, "slug": page.slug,
            "headline": page.headline, "subheadline": page.subheadline,
            "button_text": page.button_text, "button_color": page.button_color,
            "background_color": page.background_color, "text_color": page.text_color,
            "form_fields": page.form_fields, "requires_opt_in": page.requires_opt_in,
            "privacy_policy": page.privacy_policy, "consent_text": page.consent_text,
            "is_active": page.is_active,
        }})

    def post(self, request, slug):
        try:
            page = LeadCapturePage.objects.get(slug=slug)
        except LeadCapturePage.DoesNotExist:
            return Response({"success": False, "error": "Page not found"}, status=404)
        if not page.is_active:
            return Response({"success": False, "error": "Page not found"}, status=404)
        import json
        lead = Lead(
            id=uuid.uuid4(),
            page_id=page.id,
            data=json.dumps(request.data),
            email=request.data.get("email"),
            ip_address=get_client_ip(request),
            user_agent=request.META.get("HTTP_USER_AGENT"),
            opted_in=request.data.get("opt_in") is True,
            created_at=timezone.now(),
        )
        lead.save(force_insert=True)
        LeadCapturePage.objects.filter(pk=page.pk).update(submissions=page.submissions + 1)
        return Response({"success": True, "data": {
            "message": page.thank_you_message,
            "redirect_url": page.redirect_url,
            "lead_id": str(lead.id),
        }}, status=201)
