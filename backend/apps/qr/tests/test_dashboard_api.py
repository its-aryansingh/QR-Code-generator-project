"""The dashboard's workspace and QR endpoints, and scanning through /r/<code>.

Users are created through the real sign-up service, so each test starts from
what a new account actually has: one workspace on the free plan.
"""

import json
import secrets
from datetime import timedelta
from typing import Any

import pytest
from django.test import Client
from django.utils import timezone

from apps.accounts.services import register
from apps.analytics.models import ScanEvent
from apps.qr.models import QRCode, QRVersion

PHONE = (
    "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 "
    "(KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
)


class Api:
    """A signed-in browser: bearer token, no cookies (so no CSRF dance)."""

    def __init__(self, name: str) -> None:
        user, workspace, _session, access, _refresh, _csrf = register(
            email=f"{name}-{secrets.token_hex(3)}@example.com",
            password="correct horse battery",
            name=name,
            ip_key=secrets.token_hex(8),
        )
        self.user = user
        self.workspace_id = str(workspace.id)
        self.client = Client(HTTP_AUTHORIZATION=f"Bearer {access}")

    def call(self, method: str, path: str, body: dict[str, Any] | None = None) -> Any:
        response = getattr(self.client, method)(
            f"/api/v1{path}",
            data=json.dumps(body) if body is not None else None,
            content_type="application/json",
        )
        return response

    def ok(
        self, method: str, path: str, body: dict[str, Any] | None = None, status: int = 200
    ) -> Any:
        response = self.call(method, path, body)
        assert response.status_code == status, response.content
        payload = response.json()
        assert payload["success"] is True
        return payload["data"]

    def create(self, **body: Any) -> Any:
        defaults = {
            "title": "Menu",
            "qr_type": "url",
            "content": "https://example.com/menu",
            "is_dynamic": True,
        }
        return self.ok(
            "post", "/qr/generate", {**defaults, "workspace_id": self.workspace_id, **body}, 201
        )


def scan(code: str, user_agent: str = PHONE, method: str = "get", **extra: Any) -> Any:
    client = Client(HTTP_USER_AGENT=user_agent, HTTP_X_FORWARDED_FOR="203.0.113.7")
    return getattr(client, method)(f"/r/{code}", **extra)


@pytest.fixture
def alice() -> Api:
    return Api("alice")


@pytest.mark.django_db
def test_new_account_lands_on_a_working_dashboard(alice: Api) -> None:
    workspaces = alice.ok("get", "/workspaces")
    assert [w["id"] for w in workspaces] == [alice.workspace_id]
    assert workspaces[0]["role"] == "owner"
    assert workspaces[0]["plan"] == "free"
    assert workspaces[0]["qr_count"] == 0

    ent = alice.ok("get", f"/workspaces/{alice.workspace_id}/entitlements")
    assert ent["role"] == "owner"
    assert ent["plan"] == "free"
    assert ent["usage"]["qr_codes"] == 0
    # ALL_FEATURES_UNLOCKED (the default): every feature, the top limits.
    assert all(feature["enabled"] for feature in ent["features"].values())
    assert ent["limits"]["max_qr_codes"] == 1000000
    assert ent["api_daily_limit"] == 10000


@pytest.mark.django_db
def test_plan_table_applies_when_features_are_locked(alice: Api, settings: Any) -> None:
    settings.ALL_FEATURES_UNLOCKED = False
    ent = alice.ok("get", f"/workspaces/{alice.workspace_id}/entitlements")
    assert ent["limits"]["max_qr_codes"] == 50
    assert ent["features"]["bulk"] == {
        "enabled": False,
        "label": "Bulk generation",
        "min_plan": "pro",
    }
    assert ent["api_daily_limit"] == 50

    overview = alice.ok("get", f"/workspaces/{alice.workspace_id}/overview?days=7")
    assert overview["stats"]["total_scans"] == 0
    assert len(overview["scans_by_date"]) == 8
    for key in ("by_device", "top_qr_codes", "recent_qr_codes"):
        assert overview[key] == []

    assert alice.ok("get", f"/workspaces/{alice.workspace_id}/folders") == []
    listing = alice.ok("get", f"/workspaces/{alice.workspace_id}/qr")
    assert listing == {
        "items": [],
        "total": 0,
        "page": 1,
        "limit": 25,
        "pages": 0,
        "facets": {"types": [], "total_scans": 0},
    }


@pytest.mark.django_db
def test_create_scan_edit_pause_end_to_end(alice: Api) -> None:
    qr = alice.create()
    assert qr["is_dynamic"] is True
    assert qr["qr_base64"].startswith("data:image/png;base64,")
    code = qr["short_code"]
    assert len(code) == 7
    assert qr["short_url"].endswith(f"/r/{code}")
    assert qr["content"] == qr["redirect_url"] == "https://example.com/menu"

    listing = alice.ok("get", f"/workspaces/{alice.workspace_id}/qr")
    assert [item["id"] for item in listing["items"]] == [qr["id"]]
    assert listing["facets"]["types"] == [{"qr_type": "url", "count": 1}]

    # Two scans by the same phone, one by a bot.
    first = scan(code)
    assert first.status_code == 302
    assert first["Location"] == "https://example.com/menu"
    assert scan(code.lower()).status_code == 302
    assert scan(code, user_agent="Googlebot/2.1").status_code == 302

    detail = alice.ok("get", f"/workspaces/{alice.workspace_id}/qr/{qr['id']}")
    assert detail["scan_count"] == 2
    assert detail["unique_scans"] == 1
    assert ScanEvent.objects.filter(qr_code_id=qr["id"], is_bot=True).count() == 1

    overview = alice.ok("get", f"/workspaces/{alice.workspace_id}/overview")
    assert overview["stats"]["total_scans"] == 2
    assert overview["stats"]["unique_visitors"] == 1
    assert sum(day["count"] for day in overview["scans_by_date"]) == 2
    assert overview["by_device"] == [{"device": "mobile", "count": 2}]
    assert overview["top_qr_codes"][0]["scan_count"] == 2

    # Changing the destination keeps the printed code and adds a version.
    edited = alice.ok(
        "put",
        f"/workspaces/{alice.workspace_id}/qr/{qr['id']}",
        {"title": "Dinner menu", "redirect_url": "https://example.com/dinner"},
    )
    assert edited["title"] == "Dinner menu"
    assert edited["short_code"] == code
    assert QRVersion.objects.filter(qr_code_id=qr["id"]).count() == 2
    assert scan(code)["Location"] == "https://example.com/dinner"

    # Pausing stops redirects; resuming restores them.
    paused = alice.ok("put", f"/qr/{qr['id']}/toggle", {})
    assert paused["is_active"] is False
    assert scan(code).status_code == 404
    alice.ok("put", f"/qr/{qr['id']}/toggle", {})
    assert scan(code).status_code == 302


@pytest.mark.django_db
def test_detail_page_works_before_a_workspace_is_picked(alice: Api) -> None:
    """The page sends localStorage's workspace id, which is "null" on a fresh login."""
    qr = alice.create()
    data = alice.ok("get", f"/workspaces/null/qr/{qr['id']}")
    assert data["id"] == qr["id"]
    assert data["qr_base64"].startswith("data:image/png;base64,")
    alice.ok("put", f"/workspaces/null/qr/{qr['id']}", {"is_active": False})
    assert QRCode.objects.get(id=qr["id"]).status == "paused"


@pytest.mark.django_db
def test_non_web_content_is_saved_as_static(alice: Api) -> None:
    wifi = alice.create(qr_type="wifi", content="WIFI:T:WPA;S:Cafe;P:secret;;", title="Wi-Fi")
    assert wifi["is_dynamic"] is False
    assert wifi["short_code"] is None
    assert wifi["content"] == "WIFI:T:WPA;S:Cafe;P:secret;;"
    static_url = alice.create(is_dynamic=False)
    assert static_url["is_dynamic"] is False


@pytest.mark.django_db
def test_preview_renders_without_saving(alice: Api) -> None:
    preview = alice.ok("post", "/qr/generate", {"content": "https://example.com", "size": 300})
    assert preview["qr_base64"].startswith("data:image/png;base64,")
    assert QRCode.objects.count() == 0


@pytest.mark.django_db
def test_validation_errors_carry_the_v1_error_fields(alice: Api) -> None:
    response = alice.call(
        "post",
        "/qr/generate",
        {"workspace_id": alice.workspace_id, "qr_type": "url", "content": "not a url"},
    )
    assert response.status_code == 422
    body = response.json()
    assert body["success"] is False
    assert body["code"] == "invalid_url"
    assert "https://" in body["error"]


@pytest.mark.django_db
def test_other_users_cannot_see_or_change_a_workspace(alice: Api) -> None:
    bob = Api("bob")
    qr = alice.create()
    assert bob.call("get", f"/workspaces/{alice.workspace_id}/qr").status_code == 404
    assert bob.call("get", f"/workspaces/{alice.workspace_id}/overview").status_code == 404
    assert bob.call("get", f"/workspaces/null/qr/{qr['id']}").status_code == 404
    assert bob.call("get", f"/workspaces/{bob.workspace_id}/qr/{qr['id']}").status_code == 404
    assert bob.call("put", f"/qr/{qr['id']}/toggle", {}).status_code == 404
    assert (
        bob.call(
            "post", "/qr/generate", {"workspace_id": alice.workspace_id, "content": "https://x.y"}
        ).status_code
        == 404
    )
    assert [w["id"] for w in bob.ok("get", "/workspaces")] == [bob.workspace_id]
    assert Client().get("/api/v1/workspaces").status_code == 401


@pytest.mark.django_db
def test_plan_limits(alice: Api, monkeypatch: pytest.MonkeyPatch, settings: Any) -> None:
    from apps.workspaces import entitlements

    settings.ALL_FEATURES_UNLOCKED = False
    monkeypatch.setitem(entitlements.PLAN_ENTITLEMENTS["free"], "max_qr_codes", 1)
    alice.create()
    response = alice.call(
        "post",
        "/qr/generate",
        {"workspace_id": alice.workspace_id, "content": "https://example.com/2"},
    )
    assert response.status_code == 402
    assert response.json()["code"] == "limit_reached"

    # Free plan: one workspace.
    response = alice.call("post", "/workspaces", {"name": "Second"})
    assert response.status_code == 402

    # Unlocked, the same account can add a second workspace.
    settings.ALL_FEATURES_UNLOCKED = True
    created = alice.ok("post", "/workspaces", {"name": "Second"}, 201)
    assert created["role"] == "owner"


@pytest.mark.django_db
def test_scan_gates(alice: Api) -> None:
    locked = alice.create(password="open sesame")
    code = locked["short_code"]
    assert scan(code).status_code == 200  # the password form
    assert scan(code, method="post", data={"password": "wrong"}).status_code == 401
    right = scan(code, method="post", data={"password": "open sesame"})
    assert right.status_code == 302

    expired = alice.create(expires_at=(timezone.now() - timedelta(minutes=1)).isoformat())
    assert scan(expired["short_code"]).status_code == 410

    once = alice.create(max_scans=1)
    assert scan(once["short_code"]).status_code == 302
    assert scan(once["short_code"]).status_code == 410

    assert scan("ZZZZZZZ").status_code == 404


@pytest.mark.django_db
def test_folders_and_bulk_actions(alice: Api) -> None:
    folder = alice.ok("post", f"/workspaces/{alice.workspace_id}/folders", {"name": "Menus"}, 201)
    one, two = alice.create(), alice.create(title="Second")
    base = f"/workspaces/{alice.workspace_id}/qr"

    moved = alice.ok(
        "post",
        f"{base}/bulk-action",
        {"action": "move", "qr_ids": [one["id"]], "folder_id": folder["id"]},
    )
    assert moved == {"action": "move", "affected": 1}
    assert alice.ok("get", f"/workspaces/{alice.workspace_id}/folders")[0]["qr_count"] == 1
    assert [i["id"] for i in alice.ok("get", f"{base}?folder_id={folder['id']}")["items"]] == [
        one["id"]
    ]

    alice.ok(
        "post",
        f"{base}/bulk-action",
        {"action": "tag", "qr_ids": [two["id"]], "tags": ["print", "lunch"]},
    )
    assert alice.ok("get", f"{base}?tag=print")["items"][0]["tags"] == ["lunch", "print"]

    alice.ok(
        "post", f"{base}/bulk-action", {"action": "deactivate", "qr_ids": [one["id"], two["id"]]}
    )
    assert alice.ok("get", f"{base}?status=inactive")["total"] == 2

    deleted = alice.ok("post", f"{base}/bulk-action", {"action": "delete", "qr_ids": [one["id"]]})
    assert deleted["affected"] == 1
    assert alice.ok("get", base)["total"] == 1
    assert scan(one["short_code"]).status_code == 404


@pytest.mark.django_db
def test_addresses_without_https_are_accepted(alice: Api) -> None:
    """People type `www.example.com`; the create wizard sends it as is."""
    qr = alice.create(content="www.example.com/menu")
    assert qr["is_dynamic"] is True
    assert qr["content"] == "https://www.example.com/menu"
    assert scan(qr["short_code"])["Location"] == "https://www.example.com/menu"

    edited = alice.ok("put", f"/qr/{qr['id']}", {"redirect_url": "example.org"})
    assert edited["content"] == "https://example.org"

    response = alice.call(
        "post", "/qr/generate", {"workspace_id": alice.workspace_id, "content": "not a url"}
    )
    assert response.status_code == 422
