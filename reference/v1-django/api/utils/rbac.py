"""Workspace role-based access control.

Every workspace-scoped endpoint runs through `require_workspace`, which
resolves the caller's role once and attaches it to the request. Before this
existed, any authenticated user could read any workspace's members, folders
and audit log just by knowing its UUID.

Role hierarchy (each role inherits everything below it):

    owner   full control, billing, delete workspace, transfer ownership
    admin   members, roles, invites, webhooks, API keys, branding, SSO
    editor  create/edit/delete QR codes, campaigns, folders, templates
    viewer  read-only: dashboards, analytics, exports
"""

from functools import wraps

from rest_framework.response import Response

from api.models import Workspace, WorkspaceMember

ROLE_RANK = {"viewer": 1, "editor": 2, "admin": 3, "owner": 4}
ROLES = list(ROLE_RANK)


def role_at_least(role, minimum):
    return ROLE_RANK.get(role or "", 0) >= ROLE_RANK.get(minimum, 99)


def get_workspace_role(user_id, workspace):
    """Return the caller's effective role in `workspace`, or None."""
    if workspace is None:
        return None
    if str(workspace.owner_id) == str(user_id):
        return "owner"
    member = WorkspaceMember.objects.filter(
        workspace_id=workspace.id, user_id=user_id
    ).only("role").first()
    if not member:
        return None
    # A membership row that claims ownership without owning the workspace is
    # downgraded; the workspaces.owner_id column is the single source of truth.
    return "admin" if member.role == "owner" else member.role


def resolve_workspace(user_id, ws_id):
    """Return (workspace, role). Either may be None when access is denied."""
    workspace = Workspace.objects.filter(id=ws_id).first()
    if workspace is None:
        return None, None
    return workspace, get_workspace_role(user_id, workspace)


def require_workspace(minimum="viewer"):
    """Guard a view method that accepts a `ws_id` argument.

    Stacks below `require_auth`, which must run first to populate
    `request.auth_user`:

        @require_auth
        @require_workspace("admin")
        def post(self, request, ws_id): ...

    On success the request carries `workspace`, `workspace_role` and
    `workspace_plan`.
    """

    def decorator(view_func):
        @wraps(view_func)
        def wrapper(self, request, *args, **kwargs):
            ws_id = kwargs.get("ws_id") or (args[0] if args else None)
            user = getattr(request, "auth_user", None)
            if not user:
                return Response({"success": False, "error": "Unauthorized"}, status=401)

            workspace, role = resolve_workspace(user["id"], ws_id)
            if workspace is None or role is None:
                # Same response for "missing" and "forbidden" so workspace IDs
                # cannot be enumerated.
                return Response(
                    {"success": False, "error": "Workspace not found"}, status=404
                )
            if not role_at_least(role, minimum):
                return Response(
                    {
                        "success": False,
                        "error": f"This action requires the {minimum} role or higher",
                        "code": "insufficient_role",
                        "your_role": role,
                        "required_role": minimum,
                    },
                    status=403,
                )

            request.workspace = workspace
            request.workspace_role = role
            request.workspace_plan = workspace.plan or "free"
            return view_func(self, request, *args, **kwargs)

        return wrapper

    return decorator


def accessible_workspace_ids(user_id):
    """Every workspace the user can read, owned or joined."""
    owned = Workspace.objects.filter(owner_id=user_id).values_list("id", flat=True)
    joined = WorkspaceMember.objects.filter(user_id=user_id).values_list(
        "workspace_id", flat=True
    )
    return list({*owned, *joined})
