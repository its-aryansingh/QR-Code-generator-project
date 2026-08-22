from django.urls import path
from api.views.auth import (
    RegisterView, LoginView, RefreshView, LogoutView, MeView, ChangePasswordView,
)
from api.views.qr import (
    GenerateView, ApiGenerateView, HistoryView, QRDetailView, QRToggleView, QRDownloadView,
)
from api.views.public import PublicGenerateView, PublicTypesView, PublicQuotaView, PublicAnalyticsView
from api.views.analytics import DashboardView, QRAnalyticsView, QRScansView
from api.views.workspaces import (
    WorkspaceListView, WorkspaceDetailView, WorkspaceInviteView,
    WorkspaceAcceptInviteView, WorkspaceMembersView, WorkspaceMemberDetailView,
    WorkspaceFoldersView, FolderDetailView, WorkspaceAuditLogView,
)
from api.views.billing import CheckoutView, SubscriptionView, CancelView, PortalView, WebhookView
from api.views.webhooks import WebhookListView, WebhookDetailView, WebhookTestView, WebhookLogsView
from api.views.leads import (
    LeadPagesView, LeadPageDetailView, LeadSubmissionsView, PublicLeadPageView,
)
from api.views.reports import QRReportView, AnalyticsReportView, LeadsReportView
from api.views.apikey import ApiKeyView, ApiKeyRegenerateView, ApiKeyUsageView
from api.views.bulk import BulkGenerateView

urlpatterns = [
    # Auth
    path("auth/register", RegisterView.as_view()),
    path("auth/login", LoginView.as_view()),
    path("auth/refresh", RefreshView.as_view()),
    path("auth/logout", LogoutView.as_view()),
    path("auth/me", MeView.as_view()),
    path("auth/change-password", ChangePasswordView.as_view()),

    # QR
    path("qr/generate", GenerateView.as_view()),
    path("qr/api/generate", ApiGenerateView.as_view()),
    path("qr/history", HistoryView.as_view()),
    path("qr/<uuid:qr_id>", QRDetailView.as_view()),
    path("qr/<uuid:qr_id>/toggle", QRToggleView.as_view()),
    path("qr/<uuid:qr_id>/download", QRDownloadView.as_view()),

    # Bulk
    path("bulk/generate", BulkGenerateView.as_view()),

    # Public
    path("public/generate", PublicGenerateView.as_view()),
    path("public/types", PublicTypesView.as_view()),
    path("public/quota", PublicQuotaView.as_view()),
    path("public/analytics/<str:code>", PublicAnalyticsView.as_view()),
    path("public/leads/<slug:slug>", PublicLeadPageView.as_view()),

    # Analytics
    path("analytics/dashboard", DashboardView.as_view()),
    path("analytics/qr/<uuid:qr_id>", QRAnalyticsView.as_view()),
    path("analytics/qr/<uuid:qr_id>/scans", QRScansView.as_view()),

    # Workspaces
    path("workspaces", WorkspaceListView.as_view()),
    path("workspaces/<uuid:ws_id>", WorkspaceDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/invite", WorkspaceInviteView.as_view()),
    path("workspaces/invite/<str:token>/accept", WorkspaceAcceptInviteView.as_view()),
    path("workspaces/<uuid:ws_id>/members", WorkspaceMembersView.as_view()),
    path("workspaces/<uuid:ws_id>/members/<uuid:member_id>", WorkspaceMemberDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/folders", WorkspaceFoldersView.as_view()),
    path("workspaces/<uuid:ws_id>/folders/<uuid:folder_id>", FolderDetailView.as_view()),
    path("workspaces/<uuid:ws_id>/audit-log", WorkspaceAuditLogView.as_view()),

    # Billing
    path("billing/checkout", CheckoutView.as_view()),
    path("billing/subscription", SubscriptionView.as_view()),
    path("billing/cancel", CancelView.as_view()),
    path("billing/portal", PortalView.as_view()),
    path("billing/webhook", WebhookView.as_view()),

    # Webhooks
    path("webhooks", WebhookListView.as_view()),
    path("webhooks/<uuid:wh_id>", WebhookDetailView.as_view()),
    path("webhooks/<uuid:wh_id>/test", WebhookTestView.as_view()),
    path("webhooks/<uuid:wh_id>/logs", WebhookLogsView.as_view()),

    # Leads
    path("leads/pages", LeadPagesView.as_view()),
    path("leads/pages/<uuid:page_id>", LeadPageDetailView.as_view()),
    path("leads/pages/<uuid:page_id>/submissions", LeadSubmissionsView.as_view()),

    # Reports
    path("reports/qr", QRReportView.as_view()),
    path("reports/analytics", AnalyticsReportView.as_view()),
    path("reports/leads", LeadsReportView.as_view()),

    # API Key
    path("apikey", ApiKeyView.as_view()),
    path("apikey/regenerate", ApiKeyRegenerateView.as_view()),
    path("apikey/usage", ApiKeyUsageView.as_view()),
]
