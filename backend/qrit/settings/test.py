"""Settings module for running tests."""

from .base import *  # noqa: F403
from .base import _parse_db_url
from .env import env

SECRET_KEY = "test-only-secret-key-at-least-50-characters-long-safe-to-commit"
DEBUG = False

# Fast password hashing for unit tests
PASSWORD_HASHERS = [
    "django.contrib.auth.hashers.MD5PasswordHasher",
]

# Ensure default database uses test configuration
DATABASES = {
    "default": _parse_db_url(env.DATABASE_URL, min_pool=1, max_pool=2),
}

# In-memory email backend for testing
EMAIL_BACKEND = "django.core.mail.backends.locmem.EmailBackend"

# Lets the OAuth tests use "test-google:..." / "test-github:..." credentials
# instead of calling Google and GitHub. Never enabled outside this module.
OAUTH_TEST_TOKENS = True
