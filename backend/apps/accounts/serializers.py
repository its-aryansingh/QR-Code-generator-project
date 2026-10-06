"""DRF Serializers for accounts, auth, sessions, and OAuth flows."""

from typing import Any

from rest_framework import serializers

from apps.workspaces.models import Workspace

from .models import User


class RegisterRequestSerializer(serializers.Serializer[Any]):
    email = serializers.EmailField(required=True)
    password = serializers.CharField(required=True, write_only=True)
    name = serializers.CharField(required=False, allow_blank=True, default="")


class LoginRequestSerializer(serializers.Serializer[Any]):
    email = serializers.EmailField(required=True)
    password = serializers.CharField(required=True, write_only=True)


class GoogleAuthRequestSerializer(serializers.Serializer[Any]):
    credential = serializers.CharField(required=False, allow_blank=True, default="")
    id_token = serializers.CharField(required=False, allow_blank=True, default="")

    def validate(self, attrs: dict[str, Any]) -> dict[str, Any]:
        token = attrs.get("credential") or attrs.get("id_token")
        if not token:
            raise serializers.ValidationError("Either credential or id_token is required")
        attrs["token"] = token
        return attrs


class GitHubAuthRequestSerializer(serializers.Serializer[Any]):
    code = serializers.CharField(required=True)
    redirect_uri = serializers.CharField(required=False, allow_null=True, default=None)


class RefreshTokenRequestSerializer(serializers.Serializer[Any]):
    refresh_token = serializers.CharField(required=False, allow_blank=True, default="")


class VerifyEmailRequestSerializer(serializers.Serializer[Any]):
    token = serializers.CharField(required=True)


class ForgotPasswordRequestSerializer(serializers.Serializer[Any]):
    email = serializers.EmailField(required=True)


class ResetPasswordRequestSerializer(serializers.Serializer[Any]):
    token = serializers.CharField(required=True)
    password = serializers.CharField(required=True, write_only=True)


class ChangePasswordRequestSerializer(serializers.Serializer[Any]):
    current_password = serializers.CharField(
        required=False, allow_blank=True, default="", write_only=True
    )
    new_password = serializers.CharField(required=True, write_only=True)


class UpdateMeRequestSerializer(serializers.Serializer[Any]):
    name = serializers.CharField(required=False, max_length=120)
    locale = serializers.CharField(required=False, max_length=10)
    timezone = serializers.CharField(required=False, max_length=50)


class UserDTOSerializer(serializers.ModelSerializer[User]):
    has_password = serializers.SerializerMethodField()
    email_verified = serializers.SerializerMethodField()

    class Meta:
        model = User
        fields = [
            "id",
            "email",
            "name",
            "avatar_url",
            "locale",
            "timezone",
            "is_staff",
            "has_password",
            "email_verified",
            "created_at",
            "updated_at",
        ]

    def get_has_password(self, obj: User) -> bool:
        return bool(obj.password)

    def get_email_verified(self, obj: User) -> bool:
        return obj.email_verified_at is not None


class WorkspaceDTOSerializer(serializers.ModelSerializer[Workspace]):
    role = serializers.CharField(read_only=True, default="owner")

    class Meta:
        model = Workspace
        fields = [
            "id",
            "name",
            "slug",
            "owner_id",
            "plan_id",
            "timezone",
            "role",
            "is_sandbox",
            "created_at",
            "updated_at",
        ]


# Response shapes, used for the OpenAPI schema.


class AuthResponseSerializer(serializers.Serializer[Any]):
    user = UserDTOSerializer()
    token = serializers.CharField(help_text="Same value as access_token.")
    access_token = serializers.CharField()
    refresh_token = serializers.CharField()
    mfa_required = serializers.BooleanField()


class RegisterResponseSerializer(serializers.Serializer[Any]):
    user = UserDTOSerializer()
    workspace = WorkspaceDTOSerializer()
    token = serializers.CharField(help_text="Same value as access_token.")
    access_token = serializers.CharField()
    refresh_token = serializers.CharField()


class RefreshResponseSerializer(serializers.Serializer[Any]):
    token = serializers.CharField(help_text="Same value as access_token.")
    access_token = serializers.CharField()
    refresh_token = serializers.CharField()


class MembershipOrgSerializer(serializers.Serializer[Any]):
    id = serializers.UUIDField()
    name = serializers.CharField()
    slug = serializers.CharField()
    role = serializers.CharField()


class MeResponseSerializer(serializers.Serializer[Any]):
    user = UserDTOSerializer()
    workspaces = WorkspaceDTOSerializer(many=True)
    organizations = MembershipOrgSerializer(many=True)


class SessionDTOSerializer(serializers.Serializer[Any]):
    id = serializers.UUIDField()
    user_agent = serializers.CharField(allow_null=True)
    ip_prefix = serializers.CharField(allow_null=True)
    created_at = serializers.DateTimeField()
    last_used_at = serializers.DateTimeField()
    current = serializers.BooleanField()
