"""Pluggable email backend for transactional auth emails.

In DEBUG / console mode every email is printed to stdout so local
development never requires a third-party service. Switch to
``EMAIL_BACKEND=resend`` and set ``RESEND_API_KEY`` for production.
"""

import logging
from django.conf import settings

logger = logging.getLogger(__name__)


def _send_console(to: str, subject: str, html: str):
    """Dev-mode: dump the email to the console."""
    logger.info(
        "\n" + "=" * 60 + "\n"
        f"  TO: {to}\n  SUBJECT: {subject}\n"
        + "=" * 60 + "\n"
        f"{html}\n"
        + "=" * 60
    )


def _send_resend(to: str, subject: str, html: str):
    """Production: deliver via Resend API."""
    try:
        import resend
        resend.api_key = settings.RESEND_API_KEY
        resend.Emails.send({
            "from": settings.EMAIL_FROM,
            "to": [to],
            "subject": subject,
            "html": html,
        })
    except Exception:
        logger.exception("Failed to send email via Resend to %s", to)
        raise


def send_email(to: str, subject: str, html: str):
    backend = getattr(settings, "EMAIL_BACKEND", "console")
    if backend == "resend":
        _send_resend(to, subject, html)
    else:
        _send_console(to, subject, html)


def send_password_reset_email(email: str, name: str, token: str):
    reset_url = f"{settings.APP_BASE_URL}/reset-password?token={token}"
    html = f"""
    <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 40px 20px;">
        <div style="text-align: center; margin-bottom: 32px;">
            <h1 style="color: #18181b; font-size: 24px; margin: 0;">QRit</h1>
        </div>
        <h2 style="color: #18181b; font-size: 20px;">Reset your password</h2>
        <p style="color: #52525b; font-size: 15px; line-height: 1.6;">
            Hi{' ' + name if name else ''},<br><br>
            We received a request to reset your password. Click the button below
            to choose a new one. This link expires in 1 hour.
        </p>
        <div style="text-align: center; margin: 32px 0;">
            <a href="{reset_url}"
               style="display: inline-block; padding: 12px 32px; background: #7c3aed;
                      color: white; text-decoration: none; border-radius: 8px;
                      font-weight: 600; font-size: 15px;">
                Reset Password
            </a>
        </div>
        <p style="color: #a1a1aa; font-size: 13px; line-height: 1.5;">
            If you didn&rsquo;t request this, you can safely ignore this email.
            Your password will remain unchanged.<br><br>
            Can&rsquo;t click the button? Copy this link:<br>
            <a href="{reset_url}" style="color: #7c3aed; word-break: break-all;">{reset_url}</a>
        </p>
    </div>
    """
    send_email(email, "Reset your QRit password", html)


def send_verification_email(email: str, name: str, token: str):
    verify_url = f"{settings.APP_BASE_URL}/verify-email?token={token}"
    html = f"""
    <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 40px 20px;">
        <div style="text-align: center; margin-bottom: 32px;">
            <h1 style="color: #18181b; font-size: 24px; margin: 0;">QRit</h1>
        </div>
        <h2 style="color: #18181b; font-size: 20px;">Verify your email</h2>
        <p style="color: #52525b; font-size: 15px; line-height: 1.6;">
            Hi{' ' + name if name else ''},<br><br>
            Welcome to QRit! Please verify your email address by clicking the
            button below.
        </p>
        <div style="text-align: center; margin: 32px 0;">
            <a href="{verify_url}"
               style="display: inline-block; padding: 12px 32px; background: #7c3aed;
                      color: white; text-decoration: none; border-radius: 8px;
                      font-weight: 600; font-size: 15px;">
                Verify Email
            </a>
        </div>
        <p style="color: #a1a1aa; font-size: 13px; line-height: 1.5;">
            This link expires in 24 hours.<br><br>
            Can&rsquo;t click the button? Copy this link:<br>
            <a href="{verify_url}" style="color: #7c3aed; word-break: break-all;">{verify_url}</a>
        </p>
    </div>
    """
    send_email(email, "Verify your QRit email", html)
