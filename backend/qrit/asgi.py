"""ASGI config for QRit."""

import os

from django.core.asgi import get_asgi_application

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "qrit.settings.api")

application = get_asgi_application()
