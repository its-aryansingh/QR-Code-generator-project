import uuid
import jwt as pyjwt
from django.conf import settings
from django.utils import timezone
from rest_framework.views import APIView
from rest_framework.response import Response
from api.models import User
from api.utils.auth import hash_password, check_password, sign_tokens, decode_token, require_auth, generate_api_key


class RegisterView(APIView):
    def post(self, request):
        email = request.data.get("email", "").strip()
        password = request.data.get("password", "")
        name = request.data.get("name", "")
        if not email or not password:
            return Response({"success": False, "error": "Email and password required"}, status=400)
        if len(password) < 8:
            return Response({"success": False, "error": "Password must be at least 8 characters"}, status=400)
        if User.objects.filter(email=email).exists():
            return Response({"success": False, "error": "Email already registered"}, status=409)

        user = User(
            id=uuid.uuid4(),
            email=email,
            password_hash=hash_password(password),
            name=name,
            api_key=generate_api_key(),
            api_calls_reset_at=timezone.now().date(),
            created_at=timezone.now(),
            updated_at=timezone.now(),
        )
        user.save(force_insert=True)
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": {"id": str(user.id), "email": user.email, "name": user.name, "plan": user.plan},
            **tokens,
        }}, status=201)


class LoginView(APIView):
    def post(self, request):
        email = request.data.get("email", "")
        password = request.data.get("password", "")
        try:
            user = User.objects.get(email=email)
        except User.DoesNotExist:
            return Response({"success": False, "error": "Invalid email or password"}, status=401)
        if not check_password(password, user.password_hash):
            return Response({"success": False, "error": "Invalid email or password"}, status=401)
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": {"id": str(user.id), "email": user.email, "name": user.name,
                     "plan": user.plan, "avatar_url": user.avatar_url},
            **tokens,
        }})


class RefreshView(APIView):
    def post(self, request):
        token = (
            request.data.get("refresh_token")
            or request.data.get("refreshToken")
            or ""
        )
        if not token:
            return Response({"success": False, "error": "Refresh token required"}, status=400)
        try:
            payload = decode_token(token)
            if payload.get("type") != "refresh":
                return Response({"success": False, "error": "Invalid token type"}, status=401)
            user = User.objects.get(id=payload["sub"])
        except (pyjwt.InvalidTokenError, User.DoesNotExist):
            return Response({"success": False, "error": "Invalid or expired refresh token"}, status=401)
        return Response({"success": True, "data": sign_tokens(str(user.id), user.email, user.plan or "free")})


class LogoutView(APIView):
    @require_auth
    def post(self, request):
        return Response({"success": True, "data": {"message": "Logged out"}})


class MeView(APIView):
    @require_auth
    def get(self, request):
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": {
            "id": str(user.id), "email": user.email, "name": user.name,
            "company": user.company, "avatar_url": user.avatar_url,
            "plan": user.plan, "plan_expires_at": user.plan_expires_at,
            "subscription_status": user.subscription_status, "created_at": user.created_at,
        }})

    @require_auth
    def put(self, request):
        User.objects.filter(id=request.auth_user["id"]).update(
            name=request.data.get("name", ""),
            company=request.data.get("company", ""),
            avatar_url=request.data.get("avatar_url", ""),
            updated_at=timezone.now(),
        )
        user = User.objects.get(id=request.auth_user["id"])
        return Response({"success": True, "data": {
            "id": str(user.id), "email": user.email, "name": user.name,
            "company": user.company, "avatar_url": user.avatar_url, "plan": user.plan,
        }})


class ChangePasswordView(APIView):
    @require_auth
    def post(self, request):
        current = request.data.get("current_password") or request.data.get("currentPassword") or ""
        new_pass = request.data.get("new_password") or request.data.get("newPassword") or ""
        if not current or not new_pass or len(new_pass) < 8:
            return Response({"success": False, "error": "Invalid password"}, status=400)
        try:
            user = User.objects.get(id=request.auth_user["id"])
        except User.DoesNotExist:
            return Response({"success": False, "error": "Not found"}, status=404)
        if not check_password(current, user.password_hash):
            return Response({"success": False, "error": "Current password is incorrect"}, status=400)
        User.objects.filter(pk=user.pk).update(
            password_hash=hash_password(new_pass), updated_at=timezone.now()
        )
        return Response({"success": True, "data": {"message": "Password updated"}})
