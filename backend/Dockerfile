FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PORT=8084

WORKDIR /app

# Install system dependencies including curl for healthcheck
RUN apt-get update && \
    apt-get install -y --no-install-recommends libpq-dev gcc curl && \
    rm -rf /var/lib/apt/lists/*

# Create non-root system user
RUN groupadd -r appuser && useradd -r -g appuser -d /app appuser

# Install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY . .

# Make entrypoint executable
RUN chmod +x entrypoint.sh

# Collect static assets into staticfiles/
RUN python manage.py collectstatic --noinput

# Transfer ownership to non-root user
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:${PORT}/health?shallow=1 || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["gunicorn", "qrapp.wsgi:application", "--config", "gunicorn.conf.py", "--bind", "0.0.0.0:8084"]
