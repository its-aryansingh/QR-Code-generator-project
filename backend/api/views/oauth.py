"""HTTP endpoints for Google and GitHub sign-in. See `api.utils.oauth` for the flow."""

import hmac
import logging
import secrets
from urllib.parse import urlsplit

from django.conf import settings
from django.http import HttpResponseRedirect
from django.utils import timezone
from django.views import View
from rest_framework.response import Response
from rest_framework.views import APIView

from api.models import OAuthAccount, User
from api.utils import oauth
from api.utils.auth import require_auth, sign_tokens
from api.utils.ip import get_client_ip

logger = logging.getLogger(__name__)

CALLBACK_PAGE = "/auth/callback"


def user_payload(user: User) -> dict:
    return {
        "id": str(user.id),
        "email": user.email,
        "name": user.name,
        "plan": user.plan,
        "avatar_url": user.avatar_url,
        "email_verified": user.email_verified,
        "has_password": user.has_usable_password,
    }


def _origin(url: str) -> str:
    parts = urlsplit(url or "")
    return f"{parts.scheme}://{parts.netloc}" if parts.scheme and parts.netloc else ""


def _is_same_site_post(request) -> bool:
    """A browser form POST must come from our own frontend.

    Modern browsers send `Sec-Fetch-Site` and `Origin` on every form POST;
    either one naming another site is refused, and so is a POST with neither.
    """
    fetch_site = request.headers.get("Sec-Fetch-Site")
    if fetch_site and fetch_site != "same-origin":
        return False
    origin = request.headers.get("Origin")
    if origin:
        allowed = {_origin(settings.APP_BASE_URL), _origin(settings.OAUTH_CALLBACK_BASE_URL)}
        return origin.rstrip("/") in allowed
    return fetch_site == "same-origin"


def _redirect(url: str) -> HttpResponseRedirect:
    resp = HttpResponseRedirect(url)
    resp["Cache-Control"] = "no-store"
    resp["Referrer-Policy"] = "no-referrer"
    return resp


def _to_frontend(**params) -> HttpResponseRedirect:
    return _redirect(oauth.frontend_url(CALLBACK_PAGE, **params))


class OAuthProvidersView(APIView):
    """Which sign-in buttons the frontend should show."""

    def get(self, request):
        return Response({"success": True, "data": oauth.enabled_providers()})


class OAuthStartView(View):
    """Send the browser to Google/GitHub.

    GET  /auth/oauth/<provider>/start?next=/dashboard          -> sign in / sign up
    POST /auth/oauth/<provider>/start  (form: intent, next)     -> connect to the
         signed-in account; a form POST keeps the intent out of URLs and logs.
    """

    def get(self, request, provider):
        return self._start(request, provider, request.GET.get("next"), None)

    def post(self, request, provider):
        next_path = oauth.safe_next(request.POST.get("next"))
        if not _is_same_site_post(request):
            # Without this, another site could auto-submit its own valid intent
            # from the victim's browser and attach the victim's Google/GitHub
            # identity to the attacker's QRit account.
            logger.warning("Rejected cross-site OAuth link start (Origin=%s, Sec-Fetch-Site=%s)",
                           request.headers.get("Origin"), request.headers.get("Sec-Fetch-Site"))
            return _to_frontend(error="invalid_state", provider=provider, next=next_path)
        return self._start(request, provider, next_path, request.POST.get("intent"))

    def _start(self, request, provider, next_value, intent):
        next_path = oauth.safe_next(next_value)
        if provider not in oauth.PROVIDERS:
            return _to_frontend(error="unknown_provider", next=next_path)
        if not oauth.provider_enabled(provider):
            return _to_frontend(error="not_configured", provider=provider, next=next_path)

        link_user_id = None
        if intent:
            try:
                link_user_id = oauth.read_link_intent(intent, provider)
            except oauth.OAuthError as exc:
                return _to_frontend(error=exc.code, provider=provider, next=next_path)

        state = secrets.token_urlsafe(24)
        verifier, challenge = oauth.new_pkce_pair()
        nonce = secrets.token_urlsafe(24) if provider == "google" else None
        cookie = oauth.dump_state({
            "p": provider, "s": state, "v": verifier, "n": nonce,
            "next": next_path, "link": link_user_id,
        })

        resp = _redirect(oauth.authorize_url(
            provider, state=state, code_challenge=challenge, nonce=nonce,
        ))
        resp.set_cookie(
            oauth.STATE_COOKIE,
            cookie,
            max_age=settings.OAUTH_STATE_MAX_AGE_SECONDS,
            path=oauth.STATE_COOKIE_PATH,
            secure=request.is_secure(),
            httponly=True,
            samesite="Lax",
        )
        return resp


class OAuthCallbackView(View):
    """Provider redirects here; we finish sign-in and bounce to the frontend."""

    def get(self, request, provider):
        known = provider in oauth.PROVIDERS
        next_path = oauth.DEFAULT_NEXT
        state = None
        raw_cookie = request.COOKIES.get(oauth.STATE_COOKIE)
        try:
            if not known:
                raise oauth.OAuthError("unknown_provider")
            if raw_cookie:
                state = oauth.load_state(raw_cookie)
                next_path = oauth.safe_next(state.get("next"))

            provider_error = request.GET.get("error")
            if provider_error:
                raise oauth.OAuthError(
                    "access_denied" if provider_error == "access_denied" else "provider_error",
                    provider_error,
                )
            if state is None:
                # Cookie missing: expired, a different browser, or the flow was
                # never started here. Never exchange a code we did not ask for.
                raise oauth.OAuthError("invalid_state")
            returned_state = request.GET.get("state") or ""
            if state.get("p") != provider or not hmac.compare_digest(returned_state, state.get("s") or ""):
                raise oauth.OAuthError("invalid_state")
            code = request.GET.get("code")
            if not code:
                raise oauth.OAuthError("provider_error", "no code")

            profile = oauth.exchange_code(provider, code, state["v"], state.get("n"))

            if state.get("link"):
                oauth.link_identity(state["link"], profile)
                resp = _to_frontend(linked=provider, next=next_path)
            else:
                resp = self._sign_in(request, provider, profile, next_path)
        except oauth.OAuthError as exc:
            if exc.code not in ("access_denied",):
                logger.info("OAuth %s callback failed: %s (%s)", provider, exc.code, exc.detail)
            resp = _to_frontend(
                error=exc.code, provider=provider if known else None, next=next_path,
            )
        except Exception:
            logger.exception("OAuth %s callback crashed", provider)
            resp = _to_frontend(error="server_error", provider=provider if known else None, next=next_path)

        resp.delete_cookie(oauth.STATE_COOKIE, path=oauth.STATE_COOKIE_PATH, samesite="Lax")
        return resp

    @staticmethod
    def _sign_in(request, provider, profile, next_path):
        from api.views.auth import _record_login_attempt

        result = oauth.resolve_sign_in(profile)
        ip = get_client_ip(request)
        User.objects.filter(pk=result.user.pk).update(
            last_login_at=timezone.now(), last_login_ip=ip[:45],
        )
        _record_login_attempt(
            result.user.email, ip, True,
            reason=f"oauth_{provider}",
            user_agent=request.headers.get("User-Agent", ""),
        )
        code = oauth.issue_handoff(result, provider, next_path)
        return _to_frontend(code=code, provider=provider)


class OAuthExchangeView(APIView):
    """Swap the one-time handoff code for an access/refresh token pair."""

    def post(self, request):
        try:
            handoff = oauth.redeem_handoff(request.data.get("code"))
        except oauth.OAuthError as exc:
            return Response(
                {"success": False, "error": "This sign-in link has expired. Please try again.",
                 "code": exc.code},
                status=400,
            )
        user = handoff.user
        tokens = sign_tokens(str(user.id), user.email, user.plan or "free")
        return Response({"success": True, "data": {
            "user": user_payload(user),
            **tokens,
            "next": handoff.next_path,
            "provider": handoff.provider,
            "is_new_user": handoff.is_new_user,
            "password_reset": handoff.password_reset,
        }})


class OAuthLinkView(APIView):
    """Start connecting a provider to the signed-in account."""

    @require_auth
    def post(self, request, provider):
        if provider not in oauth.PROVIDERS:
            return Response({"success": False, "error": "Unknown provider", "code": "unknown_provider"}, status=404)
        if not oauth.provider_enabled(provider):
            return Response(
                {"success": False,
                 "error": f"{oauth.PROVIDER_LABELS[provider]} sign-in is not configured on this deployment.",
                 "code": "not_configured"},
                status=501,
            )
        return Response({"success": True, "data": {
            "start_url": f"/api/v1/auth/oauth/{provider}/start",
            "intent": oauth.make_link_intent(request.auth_user["id"], provider),
        }})


def _accounts_payload(user: User) -> dict:
    rows = OAuthAccount.objects.filter(user=user).order_by("created_at")
    return {
        "has_password": user.has_usable_password,
        "providers": oauth.enabled_providers(),
        "accounts": [
            {
                "provider": row.provider,
                "email": row.email,
                "connected_at": row.created_at,
                "last_login_at": row.last_login_at,
            }
            for row in rows
        ],
    }


class OAuthAccountsView(APIView):
    @require_auth
    def get(self, request):
        user = User.objects.filter(pk=request.auth_user["id"]).first()
        if not user:
            return Response({"success": False, "error": "Not found"}, status=404)
        return Response({"success": True, "data": _accounts_payload(user)})


class OAuthAccountDetailView(APIView):
    @require_auth
    def delete(self, request, provider):
        user = User.objects.filter(pk=request.auth_user["id"]).first()
        if not user:
            return Response({"success": False, "error": "Not found"}, status=404)
        account = OAuthAccount.objects.filter(user=user, provider=provider).first()
        if not account:
            return Response({"success": False, "error": "Not connected", "code": "not_connected"}, status=404)
        others = OAuthAccount.objects.filter(user=user).exclude(pk=account.pk).count()
        if not user.has_usable_password and others == 0:
            return Response(
                {"success": False,
                 "error": "Set a password first so you can still sign in after disconnecting.",
                 "code": "last_login_method"},
                status=409,
            )
        account.delete()
        return Response({"success": True, "data": _accounts_payload(user)})
