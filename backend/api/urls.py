"""API route table.

Everything enterprise is addressed as `/workspaces/{ws_id}/...` so the
workspace is always in the path and `require_workspace` can enforce
membership before a handler runs. The legacy aliases at the bottom keep the
pre-Django paths working.
"""

from django.urls import path

from api.views import compat
from api.views.analytics import DashboardView, QRAnalyticsView, QRScansView
from api.views.apikey import ApiKeyRegenerateView, ApiKeyUsageView, ApiKeyView
from api.views.auth import (
    ChangePasswordView, LoginView, LogoutView, MeView, RefreshView, RegisterView,
)
from api.views.billing import (
    CancelView, CheckoutView, PortalView, SubscriptionView, WebhookView,
)
from api.views.bulk import BulkGenerateView
from api.views.campaigns import (
    CampaignAnalyticsView, CampaignDetailView, CampaignListView,
    TemplateDetailView, TemplateListView,
)
from api.views.governance import (
    ApiKeyDetailView, ApiKeyListView, BrandingView, CustomDomainDetailView,
    CustomDomainListView, ScimTokenView, SecurityPolicyView, SsoView,
)
from api.views.leads import (
    LeadPageDetailView, LeadPagesView, LeadSubmissionsView, PublicLeadPageView,
)
from api.views.public import (
    PublicAnalyticsView, PublicGenerateView, PublicQuotaView, PublicTypesView,
)
from api.views.qr import (
    ApiGenerateView, GenerateView, HistoryView, QRDetailView, QRDownloadView, QRToggleView,
)
from api.views.reports import AnalyticsReportView, LeadsReportView, QRReportView
from api.views.webhooks import (
    WebhookDetailView, WebhookListView, WebhookLogsView, WebhookTestView,
)
from api.views.workspaces import (
    FolderDetailView, InvitePreviewView, WorkspaceAcceptInviteView,
    WorkspaceAuditLogView, WorkspaceDetailView, WorkspaceEntitlementsView,
    WorkspaceFoldersView, WorkspaceInviteDetailView, WorkspaceInvitesView,
    WorkspaceListView, WorkspaceMemberDetailView, WorkspaceMembersView,
)
from api.views.ws_analytics import (
    WorkspaceAnalyticsView, WorkspaceExportView, WorkspaceOverviewView,
)
from api.views.ws_integrations import (
    WorkspaceLeadPageDetailView, WorkspaceLeadPagesView, WorkspaceLeadsView,
    WorkspaceWebhookDetailView, WorkspaceWebhookListView, WorkspaceWebhookLogsView,
    WorkspaceWebhookTestView,
)
from api.views.ws_qr import (
    BulkJobDetailView, BulkJobListView, RoutingRuleDetailView, RoutingRuleListView,
    WorkspaceQRBulkActionView, WorkspaceQRListView,
)

urlpatterns = [
    # ---------------------------------------------------------------- auth
    path("auth/register", RegisterView.as_view()),
    path("auth/login", LoginView.as_view()),
    path("auth/refresh", RefreshView.as_view()),
    path("auth/logout", LogoutView.as_view()),
    path("auth/me", MeView.as_view()),
    path("auth/change-password", ChangePasswordView.as_view()),
    path("auth/google", compat.GoogleAuthView.as_view()),

    # ---------------------------------------------------------------- QR
    path("qr/generate", GenerateView.as_view()),
    path("qr/api/generate", ApiGenerateView.as_view()),
    path("qr/history", HistoryView.as_view()),
    path("qr/<uuid:qr_id>", QRDetailView.as_view()),
    path("qr/<uuid:qr_id>/toggle", QRToggleView.as_view()),
    path("qr/<uuid:qr_id>/download", QRDownloadView.as_view()),
    path("qr/<uuid:qr_id>/analytics", QRAnalyticsView.as_view()),
    path("bulk/generate", BulkGenerateView.as_view()),

    # ---------------------------------------------------------------- public
    path("public/generate", PublicGenerateView.as_view()),
    path("public/types", PublicTypesView.as_view()),
    path("public/quota", PublicQuotaView.as_view()),
    path("public/analytics/<str:code>", PublicAnalyticsView.as_view()),
    path("public/leads/<slug:slug>", PublicLeadPageView.as_view()),
    path("public/invite/<str:token>", InvitePreviewView.as_view()),

    # ---------------------------------------------------------------- analytics
    path("analytics/dashboard", DashboardView.as_view()),
    path("analytics/qr/<uuid:qr_id>", QRAnalyticsView.as_view()),
    path("analytics/qr/<uuid:qr_id>/scans", QRScansView.as_view()),

    # ================================================================
    # Enterprise: everything below is workspace-scoped and role-guarded
    # ================================================================
    path("workspaces", WorkspaceListView.as_view()),
    path("workspaces/<uuid:ws_id>", WorkspaceDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/entitlements", WorkspaceEntitlementsView.as_view()),

    # members & invites
    path("workspaces/<uuid:ws_id>/members", WorkspaceMembersView.as_view()),
    path("workspaces/<uuid:ws_id>/members/<uuid:member_id>", WorkspaceMemberDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/invites", WorkspaceInvitesView.as_view()),
    path("workspaces/<uuid:ws_id>/invites/<uuid:invite_id>", WorkspaceInviteDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/invite", WorkspaceInvitesView.as_view()),  # legacy singular
    path("workspaces/invite/<str:token>/accept", WorkspaceAcceptInviteView.as_view()),

    # folders
    path("workspaces/<uuid:ws_id>/folders", WorkspaceFoldersView.as_view()),
    path("workspaces/<uuid:ws_id>/folders/<uuid:folder_id>", FolderDetailView.as_view()),

    # QR codes
    path("workspaces/<uuid:ws_id>/qr", WorkspaceQRListView.as_view()),
    path("workspaces/<uuid:ws_id>/qr/bulk-action", WorkspaceQRBulkActionView.as_view()),
    path("workspaces/<uuid:ws_id>/qr/<uuid:qr_id>/routing", RoutingRuleListView.as_view()),
    path("workspaces/<uuid:ws_id>/qr/<uuid:qr_id>/routing/<uuid:rule_id>",
         RoutingRuleDetailView.as_view()),

    # bulk generation
    path("workspaces/<uuid:ws_id>/bulk", BulkJobListView.as_view()),
    path("workspaces/<uuid:ws_id>/bulk/<uuid:job_id>", BulkJobDetailView.as_view()),

    # campaigns & templates
    path("workspaces/<uuid:ws_id>/campaigns", CampaignListView.as_view()),
    path("workspaces/<uuid:ws_id>/campaigns/<uuid:campaign_id>", CampaignDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/campaigns/<uuid:campaign_id>/analytics",
         CampaignAnalyticsView.as_view()),
    path("workspaces/<uuid:ws_id>/templates", TemplateListView.as_view()),
    path("workspaces/<uuid:ws_id>/templates/<uuid:template_id>", TemplateDetailView.as_view()),

    # analytics & exports
    path("workspaces/<uuid:ws_id>/analytics", WorkspaceAnalyticsView.as_view()),
    path("workspaces/<uuid:ws_id>/overview", WorkspaceOverviewView.as_view()),
    path("workspaces/<uuid:ws_id>/export/<str:kind>", WorkspaceExportView.as_view()),

    # governance
    path("workspaces/<uuid:ws_id>/branding", BrandingView.as_view()),
    path("workspaces/<uuid:ws_id>/sso", SsoView.as_view()),
    path("workspaces/<uuid:ws_id>/sso/scim-token", ScimTokenView.as_view()),
    path("workspaces/<uuid:ws_id>/security-policy", SecurityPolicyView.as_view()),
    path("workspaces/<uuid:ws_id>/domains", CustomDomainListView.as_view()),
    path("workspaces/<uuid:ws_id>/domains/<uuid:domain_id>", CustomDomainDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/api-keys", ApiKeyListView.as_view()),
    path("workspaces/<uuid:ws_id>/api-keys/<uuid:key_id>", ApiKeyDetailView.as_view()),

    # audit
    path("workspaces/<uuid:ws_id>/audit-logs", WorkspaceAuditLogView.as_view()),
    path("workspaces/<uuid:ws_id>/audit-log", WorkspaceAuditLogView.as_view()),  # legacy singular

    # webhooks
    path("workspaces/<uuid:ws_id>/webhooks", WorkspaceWebhookListView.as_view()),
    path("workspaces/<uuid:ws_id>/webhooks/<uuid:wh_id>", WorkspaceWebhookDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/webhooks/<uuid:wh_id>/test", WorkspaceWebhookTestView.as_view()),
    path("workspaces/<uuid:ws_id>/webhooks/<uuid:wh_id>/logs", WorkspaceWebhookLogsView.as_view()),

    # leads
    path("workspaces/<uuid:ws_id>/pages", WorkspaceLeadPagesView.as_view()),
    path("workspaces/<uuid:ws_id>/pages/<uuid:page_id>", WorkspaceLeadPageDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/leads", WorkspaceLeadsView.as_view()),

    # ---------------------------------------------------------------- billing
    path("billing/checkout", CheckoutView.as_view()),
    path("billing/subscription", SubscriptionView.as_view()),
    path("billing/cancel", CancelView.as_view()),
    path("billing/portal", PortalView.as_view()),
    path("billing/webhook", WebhookView.as_view()),

    # ---------------------------------------------------------------- flat legacy
    path("webhooks", WebhookListView.as_view()),
    path("webhooks/<uuid:wh_id>", WebhookDetailView.as_view()),
    path("webhooks/<uuid:wh_id>/test", WebhookTestView.as_view()),
    path("webhooks/<uuid:wh_id>/logs", WebhookLogsView.as_view()),
    path("leads/pages", LeadPagesView.as_view()),
    path("leads/pages/<uuid:page_id>", LeadPageDetailView.as_view()),
    path("leads/pages/<uuid:page_id>/submissions", LeadSubmissionsView.as_view()),
    path("reports/qr", QRReportView.as_view()),
    path("reports/analytics", AnalyticsReportView.as_view()),
    path("reports/leads", LeadsReportView.as_view()),
    path("apikey", ApiKeyView.as_view()),
    path("apikey/regenerate", ApiKeyRegenerateView.as_view()),
    path("apikey/usage", ApiKeyUsageView.as_view()),

    # Paths the dashboard was already calling before this rewrite.
    path("user/profile", compat.UserProfileView.as_view()),
    path("analytics/summary", compat.AnalyticsSummaryView.as_view()),
    path("api/key", compat.LegacyApiKeyView.as_view()),
    path("api/key/regenerate", compat.LegacyApiKeyRegenerateView.as_view()),
    path("payments/checkout", compat.PaymentsCheckoutView.as_view()),
    path("payments/cancel", compat.PaymentsCancelView.as_view()),
    path("payments/subscription", compat.PaymentsSubscriptionView.as_view()),
    path("payments/portal", compat.PaymentsPortalView.as_view()),
]
