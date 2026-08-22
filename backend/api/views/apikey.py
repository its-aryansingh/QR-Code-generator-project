import uuid
from django.conf import settings
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import User
from api.utils.auth import require_auth, generate_api_key


class ApiKeyView(APIView):
    @require_auth
    def get(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        plan = user.plan or "free"
        limit = settings.PLAN_API_LIMITS.get(plan, 50)
        return Response({"success": True, "data": {
            "api_key": user.api_key,
            "calls_today": int(user.api_calls_today or 0),
            "daily_limit": limit,
            "reset_at": user.api_calls_reset_at.isoformat() if user.api_calls_reset_at else None,
            "plan": plan,
        }})


class ApiKeyRegenerateView(APIView):
    @require_auth
    def post(self, request):
        new_key = generate_api_key()
        User.objects.filter(id=request.auth_user["id"]).update(
            api_key=new_key,
            api_calls_today=0,
            api_calls_reset_at=timezone.now().date(),
            updated_at=timezone.now(),
        )
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": {"api_key": user.api_key, "plan": user.plan}})


class ApiKeyUsageView(APIView):
    @require_auth
    def get(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        plan = user.plan or "free"
        limit = settings.PLAN_API_LIMITS.get(plan, 50)
        calls = int(user.api_calls_today or 0)
        return Response({"success": True, "data": {
            "calls_today": calls,
            "daily_limit": limit,
            "remaining": max(0, limit - calls),
            "reset_at": user.api_calls_reset_at.isoformat() if user.api_calls_reset_at else None,
        }})
