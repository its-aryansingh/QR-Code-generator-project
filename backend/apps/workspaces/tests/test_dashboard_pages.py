"""The dashboard pages from the 27 Oct 2026 QA report, end to end through the API.

Campaigns answered 405 to POST (BUG-01); Leads, Brand & Domains, Analytics,
Team, Webhooks and Audit log called routes the v3 API didn't have, and an
unknown path came back as Django's HTML 404 page (BUG-02..07).
"""

import base64
import hashlib
import hmac
import json
from typing import Any

import httpx
import pytest

from apps.audit.models import AuditLog
from apps.integrations.models import Webhook
from apps.leads.models import FormSubmission
from apps.qr.tests.test_dashboard_api import Api, scan


@pytest.fixture
def alice() -> Api:
    return Api("alice")


@pytest.fixture
def bob() -> Api:
    return Api("bob")


def ws(api: Api, path: str = "") -> str:
    return f"/workspaces/{api.workspace_id}{path}"


def error(api: Api, method: str, path: str, status: int, body: Any = None) -> dict[str, Any]:
    response = api.call(method, path, body)
    assert response.status_code == status, response.content
    payload: dict[str, Any] = response.json()
    assert payload["success"] is False
    assert payload["error"]
    return payload


@pytest.mark.django_db
def test_unknown_api_paths_answer_json_not_html(alice: Api) -> None:
    for path in ("/api/v1/workspaces/x/does-not-exist", "/v1/nothing/here"):
        response = alice.client.get(path)
        assert response.status_code == 404
        assert response["Content-Type"] == "application/problem+json"
        assert response.json()["success"] is False


@pytest.mark.django_db
def test_campaign_create_edit_delete(alice: Api) -> None:
    created = alice.ok(
        "post",
        ws(alice, "/campaigns"),
        {
            "name": "testing",
            "description": "spring push",
            "status": "draft",
            "color": "#ff0000",
            "starts_at": "2026-10-01",
            "ends_at": "2026-12-31",
            "scan_goal": 500,
            "utm": {"source": "flyer", "medium": "qr", "campaign": "spring-2026"},
        },
        201,
    )
    assert created["name"] == "testing"
    assert created["status"] == "draft"
    assert created["scan_goal"] == 500
    assert created["utm"]["medium"] == "qr"
    assert created["starts_at"].startswith("2026-10-01")
    assert created["color"].startswith("#")

    listing = alice.ok("get", ws(alice, "/campaigns"))
    assert [c["id"] for c in listing] == [created["id"]]

    updated = alice.ok(
        "put", ws(alice, f"/campaigns/{created['id']}"), {"status": "completed", "scan_goal": 0}
    )
    assert updated["status"] == "completed"
    assert updated["scan_goal"] == 0
    assert alice.ok("get", ws(alice, "/campaigns?status=completed"))[0]["id"] == created["id"]

    qr = alice.create()
    alice.ok("put", f"/qr/{qr['id']}", {"campaign_id": created["id"]})
    detail = alice.ok("get", ws(alice, f"/campaigns/{created['id']}"))
    assert detail["qr_count"] == 1
    assert [row["id"] for row in detail["qr_codes"]] == [qr["id"]]

    alice.ok("delete", ws(alice, f"/campaigns/{created['id']}"))
    assert alice.ok("get", ws(alice, "/campaigns")) == []
    assert alice.ok("get", f"/qr/{qr['id']}")["campaign_id"] is None

    actions = list(
        AuditLog.objects.filter(target_type="campaign")
        .order_by("id")
        .values_list("action", flat=True)
    )
    assert actions == ["create", "update", "delete"]


@pytest.mark.django_db
def test_campaign_validation(alice: Api) -> None:
    assert (
        error(alice, "post", ws(alice, "/campaigns"), 422, {"name": " "})["code"] == "name_required"
    )
    bad_dates = {"name": "x", "starts_at": "2026-12-01", "ends_at": "2026-11-01"}
    assert error(alice, "post", ws(alice, "/campaigns"), 422, bad_dates)["code"] == "invalid_dates"
    bad_status = {"name": "x", "status": "running"}
    assert (
        error(alice, "post", ws(alice, "/campaigns"), 422, bad_status)["code"] == "invalid_status"
    )
    error(alice, "put", ws(alice, "/campaigns/not-a-uuid"), 404, {"name": "x"})


@pytest.mark.django_db
def test_locked_features_answer_upgrade_required(alice: Api, settings: Any) -> None:
    settings.ALL_FEATURES_UNLOCKED = False
    locked = error(alice, "post", ws(alice, "/campaigns"), 402, {"name": "spring"})
    assert locked["code"] == "upgrade_required"
    assert locked["required_plan"] == "pro"
    hook = {"url": "https://hooks.example.com/qrit", "events": ["qr.created"]}
    assert error(alice, "post", ws(alice, "/webhooks"), 402, hook)["code"] == "upgrade_required"
    assert error(alice, "get", ws(alice, "/audit-logs"), 402)["code"] == "upgrade_required"
    assert alice.ok("get", ws(alice, "/webhooks"))["available"] is False


@pytest.mark.django_db
def test_team_members_invites_and_joining(alice: Api, bob: Api) -> None:
    members = alice.ok("get", ws(alice, "/members"))
    assert len(members) == 1
    assert members[0]["role"] == "owner"
    assert members[0]["is_you"] is True
    assert members[0]["user"]["email"] == alice.user.email

    invite = alice.ok(
        "post", ws(alice, "/invites"), {"email": bob.user.email.upper(), "role": "viewer"}, 201
    )
    assert invite["status"] == "pending"
    assert invite["role"] == "viewer"
    token = invite["invite_url"].rsplit("/", 1)[1]
    assert [i["id"] for i in alice.ok("get", ws(alice, "/invites"))] == [invite["id"]]
    assert alice.ok("get", ws(alice, "/entitlements"))["usage"]["members"] == 1

    preview = Api.__new__(Api)
    preview.client = bob.client.__class__()
    shown = preview.ok("get", f"/public/invite/{token}")
    assert shown["is_valid"] is True
    assert shown["workspace"]["id"] == alice.workspace_id

    assert error(alice, "post", f"/workspaces/invite/{token}/accept", 403)["code"] == (
        "invite_email_mismatch"
    )
    joined = bob.ok("post", f"/workspaces/invite/{token}/accept", {})
    assert joined["workspace_id"] == alice.workspace_id
    assert joined["role"] == "viewer"
    assert (
        error(bob, "post", f"/workspaces/invite/{token}/accept", 410)["code"] == "invite_accepted"
    )

    members = alice.ok("get", ws(alice, "/members"))
    assert [m["role"] for m in members] == ["owner", "viewer"]
    bob_id = str(bob.user.id)

    error(bob, "get", ws(alice, "/invites"), 403)
    promoted = alice.ok("put", ws(alice, f"/members/{bob_id}"), {"role": "editor"})
    assert promoted["role"] == "editor"
    error(alice, "put", ws(alice, f"/members/{alice.user.id}"), 403, {"role": "viewer"})

    alice.ok("delete", ws(alice, f"/members/{bob_id}"))
    assert len(alice.ok("get", ws(alice, "/members"))) == 1
    error(bob, "get", ws(alice, "/members"), 404)

    actions = list(
        AuditLog.objects.filter(workspace_id=alice.workspace_id)
        .order_by("id")
        .values_list("action", flat=True)
    )
    assert actions == ["invite", "join", "role_change", "remove"]


@pytest.mark.django_db
def test_revoked_invite_frees_the_seat(alice: Api) -> None:
    invite = alice.ok("post", ws(alice, "/invites"), {"email": "c@example.com"}, 201)
    alice.ok("delete", ws(alice, f"/invites/{invite['id']}"))
    assert alice.ok("get", ws(alice, "/invites"))[0]["status"] == "revoked"
    again = alice.ok("post", ws(alice, "/invites"), {"email": "c@example.com"}, 201)
    assert again["status"] == "pending"
    assert error(alice, "post", ws(alice, "/invites"), 422, {"email": "nope"})["code"] == (
        "invalid_email"
    )


@pytest.mark.django_db
def test_analytics_empty_then_after_scans(alice: Api) -> None:
    empty = alice.ok("get", ws(alice, "/analytics?days=7"))
    assert empty["range_days"] == 7
    assert empty["totals"]["total_scans"] == 0
    assert empty["totals"]["engagement_rate"] == 0.0
    assert len(empty["scans_by_date"]) == 8
    assert len(empty["scans_by_hour"]) == 24
    assert empty["recent_scans"] == []

    qr = alice.create()
    alice.create(title="Unscanned")
    assert scan(qr["short_code"]).status_code == 302
    assert scan(qr["short_code"]).status_code == 302

    data = alice.ok("get", ws(alice, "/analytics?days=30"))
    assert data["totals"]["total_scans"] == 2
    assert data["totals"]["unique_visitors"] == 1
    assert data["totals"]["total_qr_codes"] == 2
    assert data["totals"]["scanned_qr_codes"] == 1
    assert data["totals"]["engagement_rate"] == 50.0
    assert data["by_device"][0]["count"] == 2
    assert data["top_qr_codes"][0]["id"] == qr["id"]
    assert [row["qr_id"] for row in data["recent_scans"]] == [qr["id"], qr["id"]]
    assert sum(row["count"] for row in data["scans_by_date"]) == 2

    export = alice.call("get", ws(alice, "/export/analytics?days=30"))
    assert export.status_code == 200
    assert export["Content-Type"].startswith("text/csv")
    lines = export.content.decode().strip().splitlines()
    assert lines[0].startswith("scanned_at,qr_id,qr_title")
    assert len(lines) == 3


@pytest.mark.django_db
def test_audit_log_lists_filters_and_exports(alice: Api) -> None:
    qr = alice.create(title="=HYPERLINK(1)")
    alice.ok("post", ws(alice, "/campaigns"), {"name": "Launch"}, 201)
    alice.ok("delete", f"/qr/{qr['id']}")

    log = alice.ok("get", ws(alice, "/audit-logs"))
    assert log["total"] == 3
    assert [e["action"] for e in log["items"]] == ["delete", "create", "create"]
    assert log["items"][0]["user"]["email"] == alice.user.email
    assert log["actions"] == ["create", "delete"]
    assert log["resources"] == ["campaign", "qr_code"]

    assert alice.ok("get", ws(alice, "/audit-logs?resource=campaign"))["total"] == 1
    assert alice.ok("get", ws(alice, "/audit-logs?action=delete"))["total"] == 1
    assert alice.ok("get", ws(alice, "/audit-logs?search=launch"))["total"] == 1
    assert alice.ok("get", ws(alice, "/audit-logs?limit=1&page=2"))["pages"] == 3

    export = alice.call("get", ws(alice, "/export/audit"))
    assert export.status_code == 200
    body = export.content.decode()
    assert "'=HYPERLINK(1)" in body
    assert alice.ok("get", ws(alice, "/audit-logs?action=export"))["total"] == 1


@pytest.mark.django_db
def test_webhook_lifecycle_and_signed_test_delivery(
    alice: Api, monkeypatch: pytest.MonkeyPatch
) -> None:
    listing = alice.ok("get", ws(alice, "/webhooks"))
    assert listing["webhooks"] == []
    assert listing["available"] is True
    assert "scan.created" in {e["id"] for e in listing["events"]}

    created = alice.ok(
        "post",
        ws(alice, "/webhooks"),
        {"url": "https://hooks.example.com/qrit", "events": ["scan.created", "qr.created"]},
        201,
    )
    secret = created["secret"]
    assert secret.startswith("whsec_")
    stored = Webhook.objects.get(id=created["id"])
    assert secret.encode() not in bytes(stored.secret_ciphertext)
    assert "secret" not in alice.ok("get", ws(alice, "/webhooks"))["webhooks"][0]

    seen: list[httpx.Request] = []

    def endpoint(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, text="ok")

    monkeypatch.setattr(
        "apps.integrations.views.safe_client",
        lambda timeout: httpx.Client(transport=httpx.MockTransport(endpoint)),
    )
    result = alice.ok("post", ws(alice, f"/webhooks/{created['id']}/test"), {})
    assert result["delivered"] is True
    assert result["status_code"] == 200

    sent = seen[0]
    key = base64.b64decode(secret.removeprefix("whsec_"))
    signed = f"{sent.headers['webhook-id']}.{sent.headers['webhook-timestamp']}.".encode()
    expected = base64.b64encode(hmac.new(key, signed + sent.content, hashlib.sha256).digest())
    assert sent.headers["webhook-signature"] == "v1," + expected.decode()
    assert json.loads(sent.content)["type"] == "ping"

    logs = alice.ok("get", ws(alice, f"/webhooks/{created['id']}/logs"))
    assert [(row["event"], row["success"]) for row in logs] == [("ping", True)]

    paused = alice.ok("put", ws(alice, f"/webhooks/{created['id']}"), {"is_active": False})
    assert paused["is_active"] is False
    assert paused["delivery_count"] == 1
    bad = {"url": "ftp://nope", "events": ["qr.created"]}
    assert error(alice, "post", ws(alice, "/webhooks"), 422, bad)["code"] == "invalid_url"
    unknown = {"url": "https://x.example.com", "events": ["qr.exploded"]}
    assert error(alice, "post", ws(alice, "/webhooks"), 422, unknown)["code"] == "invalid_events"

    alice.ok("delete", ws(alice, f"/webhooks/{created['id']}"))
    assert alice.ok("get", ws(alice, "/webhooks"))["webhooks"] == []


@pytest.mark.django_db
def test_webhook_test_reports_unreachable_endpoint(
    alice: Api, monkeypatch: pytest.MonkeyPatch
) -> None:
    created = alice.ok(
        "post",
        ws(alice, "/webhooks"),
        {"url": "https://hooks.example.com/down", "events": ["qr.created"]},
        201,
    )

    def refuse(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused", request=request)

    monkeypatch.setattr(
        "apps.integrations.views.safe_client",
        lambda timeout: httpx.Client(transport=httpx.MockTransport(refuse)),
    )
    result = alice.ok("post", ws(alice, f"/webhooks/{created['id']}/test"), {})
    assert result["delivered"] is False
    assert "connection refused" in result["error"]
    assert alice.ok("get", ws(alice, f"/webhooks/{created['id']}/logs"))[0]["success"] is False


@pytest.mark.django_db
def test_lead_pages_capture_encrypted_leads(alice: Api) -> None:
    page = alice.ok(
        "post",
        ws(alice, "/pages"),
        {
            "name": "Spring Offer",
            "headline": "Get 20% off",
            "button_color": "#10B981",
            "requires_opt_in": True,
            "form_fields": json.dumps(
                [
                    {"name": "email", "type": "email", "required": True, "label": "Email"},
                    {"name": "name", "type": "text", "required": False, "label": "Name"},
                ]
            ),
        },
        201,
    )
    assert page["slug"] == "spring-offer"
    assert page["lead_count"] == 0

    visitor = Api.__new__(Api)
    visitor.client = alice.client.__class__()
    public = visitor.ok("get", "/pages/spring-offer")
    assert public["headline"] == "Get 20% off"
    assert json.loads(public["form_fields"])[0]["name"] == "email"

    missing = visitor.call("post", "/pages/spring-offer/submit", {"name": "Ann"})
    assert missing.status_code == 422
    assert missing.json()["error"] == "Email is required"
    no_consent = visitor.call("post", "/pages/spring-offer/submit", {"email": "ann@example.com"})
    assert no_consent.json()["code"] == "consent_required"
    visitor.ok(
        "post",
        "/pages/spring-offer/submit",
        {"email": "Ann@Example.com", "name": "Ann", "_opt_in": "true"},
        201,
    )

    row = FormSubmission.objects.get()
    assert b"ann@example.com" not in bytes(row.data_ct)

    leads = alice.ok("get", ws(alice, "/leads"))
    assert leads["total"] == 1
    lead = leads["items"][0]
    assert lead["email"] == "ann@example.com"
    assert lead["data"]["name"] == "Ann"
    assert lead["opted_in"] is True
    assert alice.ok("get", ws(alice, "/leads?search=ann@example.com"))["total"] == 1
    assert alice.ok("get", ws(alice, "/leads?search=bob@example.com"))["total"] == 0

    pages = alice.ok("get", ws(alice, "/pages"))
    assert pages[0]["views"] == 1
    assert pages[0]["lead_count"] == 1
    assert pages[0]["conversion_rate"] == 100.0

    alice.ok("put", ws(alice, f"/pages/{page['id']}"), {"is_active": False})
    assert visitor.call("get", "/pages/spring-offer").status_code == 404

    export = alice.call("get", ws(alice, "/export/leads")).content.decode()
    assert "ann@example.com" in export


@pytest.mark.django_db
def test_branding_and_sso_status(alice: Api) -> None:
    brand = alice.ok("get", ws(alice, "/branding"))
    assert brand["brand_color"] == "#8B5CF6"
    assert brand["custom_css"] is None

    saved = alice.ok(
        "put",
        ws(alice, "/branding"),
        {
            "brand_color": "#112233",
            "brand_logo": "https://cdn.example.com/logo.png",
            "custom_footer": "Acme Inc.",
        },
    )
    assert saved["brand_color"] == "#112233"
    assert saved["brand_logo"] == "https://cdn.example.com/logo.png"
    assert alice.ok("get", "/workspaces")[0]["brand_color"] == "#112233"

    assert error(alice, "put", ws(alice, "/branding"), 422, {"brand_color": "red"})["code"] == (
        "invalid_color"
    )
    css = {"custom_css": "body{display:none}"}
    assert error(alice, "put", ws(alice, "/branding"), 422, css)["code"] == (
        "custom_css_unsupported"
    )

    sso = alice.ok("get", ws(alice, "/sso"))
    assert sso["available"] is False
    assert sso["is_enabled"] is False
    assert error(alice, "put", ws(alice, "/sso"), 409, {"is_enabled": True})["code"] == (
        "sso_unavailable"
    )
