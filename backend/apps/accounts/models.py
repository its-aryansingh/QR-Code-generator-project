"""Account, user, and session models.

Plan §6.3 & §7.2:
- users (AUTH_USER_MODEL)
- oauth_accounts
- sessions
- email_tokens
"""

from typing import Any, ClassVar

from django.contrib.auth.base_user import AbstractBaseUser, BaseUserManager
from django.db import models
from django.utils.timezone import now as tz_now

from apps.core.fields import CIText
from apps.core.ids import uuid7


class UserManager(BaseUserManager["User"]):
    def create_user(self, email: str, password: str | None = None, **extra_fields: Any) -> "User":
        if not email:
            raise ValueError("Email must be set")
        email = self.normalize_email(email)
        user = self.model(email=email, **extra_fields)
        if password:
            user.set_password(password)
        else:
            user.set_unusable_password()
        user.save(using=self._db)
        return user

    def create_superuser(
        self, email: str, password: str | None = None, **extra_fields: Any
    ) -> "User":
        extra_fields.setdefault("is_staff", True)
        return self.create_user(email, password, **extra_fields)


class User(AbstractBaseUser):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    email = CIText(unique=True)
    email_verified_at = models.DateTimeField(null=True, blank=True)
    password = models.TextField(db_column="password_hash", null=True, blank=True)  # type: ignore[assignment]
    name = models.TextField(default="")
    avatar_url = models.TextField(null=True, blank=True)
    locale = models.TextField(default="en")
    timezone = models.TextField(default="UTC")
    is_staff = models.BooleanField(default=False)
    last_login = models.DateTimeField(db_column="last_login_at", null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    updated_at = models.DateTimeField(default=tz_now)
    deleted_at = models.DateTimeField(null=True, blank=True)

    objects = UserManager()

    USERNAME_FIELD = "email"
    EMAIL_FIELD = "email"
    REQUIRED_FIELDS: ClassVar[list[str]] = []

    class Meta:
        db_table = "users"

    def __str__(self) -> str:
        return str(self.email)


class OAuthAccount(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    user = models.ForeignKey(
        User, on_delete=models.DB_CASCADE, db_column="user_id", related_name="oauth_accounts"
    )
    provider = models.TextField()
    provider_user_id = models.TextField()
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "oauth_accounts"
        constraints = [
            models.UniqueConstraint(
                fields=["provider", "provider_user_id"],
                name="oauth_accounts_provider_provider_user_id_key",
            ),
            models.CheckConstraint(
                condition=models.Q(provider__in=["google"]),
                name="oauth_accounts_provider_check",
            ),
        ]

    def __str__(self) -> str:
        return f"{self.provider}:{self.provider_user_id}"


class Session(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    user = models.ForeignKey(
        User, on_delete=models.DB_CASCADE, db_column="user_id", related_name="sessions"
    )
    family_id = models.UUIDField()
    refresh_token_hash = models.BinaryField(unique=True)
    user_agent = models.TextField(null=True, blank=True)
    ip_prefix = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)
    last_used_at = models.DateTimeField(default=tz_now)
    expires_at = models.DateTimeField()
    revoked_at = models.DateTimeField(null=True, blank=True)
    replaced_by = models.ForeignKey(
        "self",
        null=True,
        blank=True,
        on_delete=models.DB_SET_NULL,
        db_column="replaced_by",
        related_name="replaces",
    )

    class Meta:
        db_table = "sessions"
        indexes = [
            models.Index(
                fields=["user_id"],
                condition=models.Q(revoked_at__isnull=True),
                name="sessions_user_active_idx",
            ),
            models.Index(fields=["family_id"], name="sessions_family_idx"),
        ]

    def __str__(self) -> str:
        return f"session:{self.id}"


class EmailToken(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid7, editable=False)
    user = models.ForeignKey(
        User, on_delete=models.DB_CASCADE, db_column="user_id", related_name="email_tokens"
    )
    purpose = models.TextField()
    token_hash = models.BinaryField(unique=True)
    expires_at = models.DateTimeField()
    used_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(default=tz_now)

    class Meta:
        db_table = "email_tokens"
        constraints = [
            models.CheckConstraint(
                condition=models.Q(purpose__in=["verify_email", "reset_password", "magic_link"]),
                name="email_tokens_purpose_check",
            ),
        ]

    def __str__(self) -> str:
        return f"email_token:{self.purpose}:{self.id}"
