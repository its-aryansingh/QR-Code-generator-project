"""Dashboard routes for workspaces and their QR codes (mounted under /v1/ and /api/v1/)."""

from django.urls import path

from apps.qr import views as qr_views

from . import views

urlpatterns = [
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
        views.WorkspaceCampaignsView.as_view(),
        name="workspace-campaigns",
    ),
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
