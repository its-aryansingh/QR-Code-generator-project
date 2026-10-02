#!/usr/bin/env python
import os
import sys

if __name__ == "__main__":
    os.environ.setdefault("DJANGO_SETTINGS_MODULE", "qrapp.settings")

    # Automatically expand literal $PORT or ${PORT} strings passed by platforms without shell expansion
    port = os.environ.get("PORT", "8084")
    sys.argv = [arg.replace("$PORT", port).replace("${PORT}", port) for arg in sys.argv]

    from django.core.management import execute_from_command_line
    execute_from_command_line(sys.argv)
