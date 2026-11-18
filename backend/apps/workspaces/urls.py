"""Dashboard routes for workspaces and their QR codes (mounted under /v1/ and /api/v1/)."""

from django.urls import path

from apps.analytics import views as analytics_views
from apps.audit import views as audit_views
from apps.branding import views as branding_views
from apps.integrations import views as webhook_views
from apps.leads import views as lead_views
from apps.qr import campaigns
from apps.qr import views as qr_views

from . import team, views

urlpatterns = [
    path(
        "workspaces/invite/<str:token>/accept",
        team.InviteAcceptView.as_view(),
        name="workspace-invite-accept",
    ),
    path("public/invite/<str:token>", team.InvitePreviewView.as_view(), name="invite-preview"),
    path("pages/<slug:slug>", lead_views.PublicLeadPageView.as_view(), name="lead-page-public"),
    path(
        "pages/<slug:slug>/submit",
        lead_views.PublicLeadSubmitView.as_view(),
        name="lead-page-submit",
    ),
    path("workspaces", views.WorkspaceListView.as_view(), name="workspace-list"),
    path("workspaces/<str:ws>", views.WorkspaceDetailView.as_view(), name="workspace-detail"),
    path(
        "workspaces/<str:ws>/entitlements",
        views.WorkspaceEntitlementsView.as_view(),
        name="workspace-entitlements",
    ),
    path(
        "workspaces/<str:ws>/overview",
        views.WorkspaceOverviewView.as_view(),
        name="workspace-overview",
    ),
    path(
        "workspaces/<str:ws>/folders",
        views.WorkspaceFoldersView.as_view(),
        name="workspace-folders",
    ),
    path(
        "workspaces/<str:ws>/campaigns",
        campaigns.WorkspaceCampaignsView.as_view(),
        name="workspace-campaigns",
    ),
    path(
        "workspaces/<str:ws>/campaigns/<str:campaign_id>",
        campaigns.WorkspaceCampaignDetailView.as_view(),
        name="workspace-campaign-detail",
    ),
    path("workspaces/<str:ws>/members", team.WorkspaceMembersView.as_view(), name="ws-members"),
    path(
        "workspaces/<str:ws>/members/<str:member_id>",
        team.WorkspaceMemberDetailView.as_view(),
        name="ws-member-detail",
    ),
    path("workspaces/<str:ws>/invites", team.WorkspaceInvitesView.as_view(), name="ws-invites"),
    path(
        "workspaces/<str:ws>/invites/<str:invite_id>",
        team.WorkspaceInviteDetailView.as_view(),
        name="ws-invite-detail",
    ),
    path(
        "workspaces/<str:ws>/analytics",
        analytics_views.WorkspaceAnalyticsView.as_view(),
        name="ws-analytics",
    ),
    path(
        "workspaces/<str:ws>/export/<str:kind>",
        analytics_views.WorkspaceExportView.as_view(),
        name="ws-export",
    ),
    path(
        "workspaces/<str:ws>/audit-logs",
        audit_views.WorkspaceAuditLogView.as_view(),
        name="ws-audit-logs",
    ),
    path(
        "workspaces/<str:ws>/webhooks",
        webhook_views.WorkspaceWebhookListView.as_view(),
        name="ws-webhooks",
    ),
    path(
        "workspaces/<str:ws>/webhooks/<str:wh_id>",
        webhook_views.WorkspaceWebhookDetailView.as_view(),
        name="ws-webhook-detail",
    ),
    path(
        "workspaces/<str:ws>/webhooks/<str:wh_id>/test",
        webhook_views.WorkspaceWebhookTestView.as_view(),
        name="ws-webhook-test",
    ),
    path(
        "workspaces/<str:ws>/webhooks/<str:wh_id>/logs",
        webhook_views.WorkspaceWebhookLogsView.as_view(),
        name="ws-webhook-logs",
    ),
    path("workspaces/<str:ws>/pages", lead_views.WorkspaceLeadPagesView.as_view(), name="ws-pages"),
    path(
        "workspaces/<str:ws>/pages/<str:page_id>",
        lead_views.WorkspaceLeadPageDetailView.as_view(),
        name="ws-page-detail",
    ),
    path("workspaces/<str:ws>/leads", lead_views.WorkspaceLeadsView.as_view(), name="ws-leads"),
    path(
        "workspaces/<str:ws>/branding",
        branding_views.WorkspaceBrandingView.as_view(),
        name="ws-branding",
    ),
    path("workspaces/<str:ws>/sso", branding_views.WorkspaceSsoView.as_view(), name="ws-sso"),
    path(
        "workspaces/<str:ws>/qr", qr_views.WorkspaceQRListView.as_view(), name="workspace-qr-list"
    ),
    path(
        "workspaces/<str:ws>/qr/bulk-action",
        qr_views.WorkspaceQRBulkView.as_view(),
        name="workspace-qr-bulk",
    ),
    path(
        "workspaces/<str:ws>/qr/<str:qr_id>",
        qr_views.WorkspaceQRDetailView.as_view(),
        name="workspace-qr-detail",
    ),
    path("qr/generate", qr_views.QRGenerateView.as_view(), name="qr-generate"),
    path("qr/<str:qr_id>", qr_views.QRDetailView.as_view(), name="qr-detail"),
    path("qr/<str:qr_id>/toggle", qr_views.QRToggleView.as_view(), name="qr-toggle"),
]
