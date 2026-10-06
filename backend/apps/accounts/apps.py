from django.apps import AppConfig


class AccountsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "apps.accounts"

    def ready(self) -> None:
        # Registers the OpenAPI description of SessionAuthentication.
        from . import schema  # noqa: F401
