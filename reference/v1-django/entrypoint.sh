#!/bin/sh
set -e

# Run database migrations if requested (useful for single-container & compose setups)
if [ "$RUN_MIGRATIONS" = "true" ] || [ "$RUN_MIGRATIONS" = "1" ]; then
    echo "Running database migrations..."
    python manage.py migrate --noinput
fi

exec "$@"
