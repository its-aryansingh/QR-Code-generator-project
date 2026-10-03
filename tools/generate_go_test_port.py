import os
import re

mapping = {
    "reference/go-v2/internal/abuse/disposable_test.go": ("P2", "apps/trust/tests/test_disposable.py"),
    "reference/go-v2/internal/abuse/velocity_test.go": ("P2", "apps/trust/tests/test_velocity.py"),
    "reference/go-v2/internal/approval/approval_test.go": ("P6", "apps/approvals/tests/test_domain.py"),
    "reference/go-v2/internal/audit/audit_test.go": ("P6", "apps/audit/tests/test_domain.py"),
    "reference/go-v2/internal/auditstream/auditstream_test.go": ("P6", "apps/audit/tests/test_streams.py"),
    "reference/go-v2/internal/auth/auth_test.go": ("P2", "apps/accounts/tests/test_auth.py"),
    "reference/go-v2/internal/auth/totp_test.go": ("P2", "apps/accounts/tests/test_totp.py"),
    "reference/go-v2/internal/authz/authz_test.go": ("P2", "apps/access/tests/test_authz.py"),
    "reference/go-v2/internal/billing/billing_test.go": ("P8", "apps/billing/tests/test_providers.py"),
    "reference/go-v2/internal/domains/domains_test.go": ("P2", "apps/qr/tests/test_domains.py"),
    "reference/go-v2/internal/email/email_test.go": ("P2", "apps/core/tests/test_email.py"),
    "reference/go-v2/internal/email/templates_test.go": ("P2", "apps/core/tests/test_templates.py"),
    "reference/go-v2/internal/entitlements/entitlements_test.go": ("P2", "apps/billing/tests/test_entitlements.py"),
    "reference/go-v2/internal/hosted/hosted_test.go": ("P4", "apps/qr/tests/test_hosted.py"),
    "reference/go-v2/internal/httpapi/enterprise_test.go": ("P3/P6", "tests/test_enterprise_api.py"),
    "reference/go-v2/internal/httpapi/foundation_test.go": ("P2", "tests/test_foundation_api.py"),
    "reference/go-v2/internal/httpapi/governance_test.go": ("P6", "tests/test_governance_api.py"),
    "reference/go-v2/internal/httpapi/identity_test.go": ("P7", "tests/test_identity_api.py"),
    "reference/go-v2/internal/httpapi/pipeline_test.go": ("P5", "tests/test_pipeline_api.py"),
    "reference/go-v2/internal/httpapi/scim_unit_test.go": ("P7", "apps/identity/tests/test_scim_unit.py"),
    "reference/go-v2/internal/idempotency/idempotency_test.go": ("P1", "apps/core/tests/test_idempotency.py"),
    "reference/go-v2/internal/ingest/ingest_test.go": ("P5", "apps/analytics/tests/test_ingest.py"),
    "reference/go-v2/internal/invoicing/tax_test.go": ("P8", "apps/billing/tests/test_tax.py"),
    "reference/go-v2/internal/netutil/clientip_test.go": ("P1", "apps/core/tests/test_net.py"),
    "reference/go-v2/internal/qr/content_test.go": ("P4", "apps/qr/tests/test_content.py"),
    "reference/go-v2/internal/qr/qr_test.go": ("P4", "apps/qr/tests/test_design.py"),
    "reference/go-v2/internal/rbac/rbac_test.go": ("P2", "apps/access/tests/test_rbac.py"),
    "reference/go-v2/internal/realtime/realtime_test.go": ("P5", "apps/analytics/tests/test_realtime.py"),
    "reference/go-v2/internal/resolve/resolve_test.go": ("P5", "apps/redirect/tests/test_resolve.py"),
    "reference/go-v2/internal/routing/engine_test.go": ("P5", "apps/redirect/tests/test_routing.py"),
    "reference/go-v2/internal/scan/botdetect_test.go": ("P5", "apps/analytics/tests/test_botdetect.py"),
    "reference/go-v2/internal/scan/scan_test.go": ("P5", "apps/analytics/tests/test_scan.py"),
    "reference/go-v2/internal/scan/uaparse_test.go": ("P5", "apps/analytics/tests/test_uaparse.py"),
    "reference/go-v2/internal/shortcode/shortcode_test.go": ("P4", "apps/qr/tests/test_shortcode.py"),
    "reference/go-v2/internal/sso/sso_test.go": ("P7", "apps/identity/tests/test_sso.py"),
    "reference/go-v2/internal/stdwebhook/stdwebhook_test.go": ("P9", "apps/integrations/tests/test_stdwebhook.py"),
    "reference/go-v2/internal/urlsafety/rescan_test.go": ("P4", "apps/trust/tests/test_rescan.py"),
    "reference/go-v2/internal/urlsafety/urlsafety_test.go": ("P4", "apps/trust/tests/test_urlsafety.py"),
    "reference/go-v2/internal/urlsafety/webrisk_test.go": ("P4", "apps/trust/tests/test_webrisk.py"),
    "reference/go-v2/internal/version/scheduler_test.go": ("P4", "apps/qr/tests/test_scheduler.py"),
    "reference/go-v2/internal/version/version_test.go": ("P4", "apps/qr/tests/test_version.py"),
    "reference/go-v2/internal/webhooks/webhooks_test.go": ("P9", "apps/integrations/tests/test_webhooks.py"),
    "reference/go-v2/internal/workspace/workspace_test.go": ("P2", "apps/workspaces/tests/test_workspace.py"),
}

root = "reference/go-v2"
all_tests = []
for dirpath, _, filenames in sorted(os.walk(root)):
    for f in sorted(filenames):
        if f.endswith("_test.go"):
            rel_path = os.path.join(dirpath, f).replace(os.sep, "/")
            with open(rel_path, "r", encoding="utf-8") as fp:
                for line in fp:
                    m = re.match(r"^func (Test\w+)\(", line)
                    if m:
                        test_name = m.group(1)
                        all_tests.append((rel_path, test_name))

def to_snake(name: str) -> str:
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", name)
    return re.sub(r"([a-z\d])([A-Z])", r"\1_\2", s).lower()

rows = []
for fpath, tname in all_tests:
    phase, target_base = mapping.get(fpath, ("P2", "tests/test_go.py"))
    py_test = to_snake(tname)
    target = f"{target_base}::{py_test}"
    rows.append(f"| `{fpath}` | `{tname}` | `{target}` | {phase} | todo | |")

header = f"""# GO_TEST_PORT.md — Go v2 to Python/pytest Test Port Tracking

This file tracks the porting of all 101 Go tests from `reference/go-v2/**/*_test.go` (44 files) into pytest suites on branch `v3`.
Plan §7 & Rule 3.4 / 3.6: Every test assertion from Go must be preserved.

Total Go tests: {len(rows)}

| Go file | Go test | pytest target | phase | status | notes |
|---|---|---|---|---|---|"""

content = header + "\n" + "\n".join(rows) + "\n"
with open("docs/v3/GO_TEST_PORT.md", "w", encoding="utf-8", newline="\n") as fp:
    fp.write(content)
print(f"Done writing {len(rows)} tests to docs/v3/GO_TEST_PORT.md")
