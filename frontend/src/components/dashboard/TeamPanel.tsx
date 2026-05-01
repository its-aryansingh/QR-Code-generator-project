"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuthStore } from "@/lib/auth";
import { toast } from "sonner";

interface Member {
  id: string;
  user_id: string;
  role: string;
  joined_at: string;
  user?: { name: string; email: string };
}

interface Invite {
  id: string;
  email: string;
  role: string;
  status: string;
  expires_at: string;
  created_at: string;
}

interface TeamPanelProps {
  workspaceId: string;
  currentUserRole?: string;
}

const ROLE_COLORS: Record<string, string> = {
  owner: "bg-amber-600/20 text-amber-400 border-amber-500/30",
  admin: "bg-violet-600/20 text-violet-400 border-violet-500/30",
  editor: "bg-blue-600/20 text-blue-400 border-blue-500/30",
  viewer: "bg-zinc-700 text-zinc-400 border-zinc-600",
};

const ROLES = ["admin", "editor", "viewer"];

export function TeamPanel({ workspaceId, currentUserRole = "viewer" }: TeamPanelProps) {
  const { accessToken } = useAuthStore();
  const [members, setMembers] = useState<Member[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [loading, setLoading] = useState(true);
  const [showInvite, setShowInvite] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("editor");
  const [sending, setSending] = useState(false);

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
  const canManage = currentUserRole === "owner" || currentUserRole === "admin";

  const fetchTeam = useCallback(async () => {
    if (!workspaceId || !accessToken) return;
    setLoading(true);
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/members`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      }).then((r) => r.json());
      if (res.success) setMembers(res.data ?? []);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, accessToken]);

  useEffect(() => { fetchTeam(); }, [fetchTeam]);

  const sendInvite = async () => {
    if (!inviteEmail.trim()) return;
    setSending(true);
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/members/invite`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
        body: JSON.stringify({ email: inviteEmail.trim(), role: inviteRole }),
      }).then((r) => r.json());

      if (res.success) {
        toast.success(`Invite sent to ${inviteEmail}`);
        setInviteEmail("");
        setShowInvite(false);
        fetchTeam();
      } else {
        toast.error(res.error || "Failed to send invite");
      }
    } finally {
      setSending(false);
    }
  };

  const updateRole = async (memberId: string, role: string) => {
    const res = await fetch(`${api}/workspaces/${workspaceId}/members/${memberId}/role`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
      body: JSON.stringify({ role }),
    }).then((r) => r.json());

    if (res.success) {
      toast.success("Role updated");
      fetchTeam();
    } else {
      toast.error(res.error || "Failed to update role");
    }
  };

  const removeMember = async (memberId: string) => {
    if (!confirm("Remove this member from the workspace?")) return;
    const res = await fetch(`${api}/workspaces/${workspaceId}/members/${memberId}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    }).then((r) => r.json());

    if (res.success) {
      toast.success("Member removed");
      fetchTeam();
    } else {
      toast.error(res.error || "Failed to remove member");
    }
  };

  const formatDate = (s: string) => new Date(s).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-zinc-100">Team Members</h2>
          <p className="text-sm text-zinc-500 mt-0.5">{members.length} member{members.length !== 1 ? "s" : ""}</p>
        </div>
        {canManage && (
          <button
            onClick={() => setShowInvite(!showInvite)}
            className="flex items-center gap-2 px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold rounded-xl transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            Invite Member
          </button>
        )}
      </div>

      {/* Invite form */}
      {showInvite && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
          <h3 className="font-semibold text-zinc-200 text-sm">Invite by Email</h3>
          <div className="flex flex-col sm:flex-row gap-3">
            <input
              autoFocus
              type="email"
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") sendInvite(); }}
              placeholder="colleague@company.com"
              className="flex-1 px-4 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-600"
            />
            <select
              value={inviteRole}
              onChange={(e) => setInviteRole(e.target.value)}
              className="px-3 py-2.5 bg-zinc-950 border border-zinc-800 rounded-xl text-sm text-zinc-300 focus:outline-none focus:border-violet-600"
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>{r.charAt(0).toUpperCase() + r.slice(1)}</option>
              ))}
            </select>
            <button
              onClick={sendInvite}
              disabled={sending || !inviteEmail.trim()}
              className="px-5 py-2.5 bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold rounded-xl transition-colors disabled:opacity-50"
            >
              {sending ? "Sending…" : "Send Invite"}
            </button>
          </div>
          <p className="text-xs text-zinc-600">The recipient will receive an invite link valid for 7 days.</p>
        </div>
      )}

      {/* Role legend */}
      <div className="flex flex-wrap gap-2 text-xs">
        {Object.entries(ROLE_COLORS).map(([role, cls]) => (
          <span key={role} className={`px-2.5 py-1 rounded-full border font-semibold ${cls}`}>
            {role.charAt(0).toUpperCase() + role.slice(1)}
          </span>
        ))}
        <span className="text-zinc-600 self-center">Owner › Admin › Editor › Viewer</span>
      </div>

      {/* Members table */}
      <div className="bg-zinc-900 rounded-xl border border-zinc-800 overflow-hidden">
        {loading ? (
          <div className="p-8 text-center text-zinc-600 text-sm animate-pulse">Loading members…</div>
        ) : members.length === 0 ? (
          <div className="p-8 text-center text-zinc-600 text-sm">No members yet.</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-800 bg-zinc-950/50">
                <th className="px-5 py-3 text-left font-semibold text-zinc-400">Member</th>
                <th className="px-5 py-3 text-left font-semibold text-zinc-400 hidden sm:table-cell">Role</th>
                <th className="px-5 py-3 text-left font-semibold text-zinc-400 hidden md:table-cell">Joined</th>
                {canManage && <th className="px-5 py-3 text-left font-semibold text-zinc-400">Actions</th>}
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.id} className="border-b border-zinc-800/60 hover:bg-zinc-800/20 transition-colors">
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-zinc-800 flex items-center justify-center text-zinc-400 font-semibold text-sm flex-shrink-0">
                        {(m.user?.name ?? m.user?.email ?? "?").charAt(0).toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <p className="font-medium text-zinc-200 truncate">{m.user?.name ?? "—"}</p>
                        <p className="text-xs text-zinc-500 truncate">{m.user?.email ?? m.user_id}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-3.5 hidden sm:table-cell">
                    {canManage && m.role !== "owner" ? (
                      <select
                        value={m.role}
                        onChange={(e) => updateRole(m.id, e.target.value)}
                        className={`px-2.5 py-1 rounded-full border text-xs font-semibold bg-transparent cursor-pointer focus:outline-none ${ROLE_COLORS[m.role] ?? ""}`}
                      >
                        {ROLES.map((r) => <option key={r} value={r}>{r.charAt(0).toUpperCase() + r.slice(1)}</option>)}
                      </select>
                    ) : (
                      <span className={`px-2.5 py-1 rounded-full border text-xs font-semibold ${ROLE_COLORS[m.role] ?? ""}`}>
                        {m.role.charAt(0).toUpperCase() + m.role.slice(1)}
                      </span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-xs text-zinc-500 hidden md:table-cell">
                    {formatDate(m.joined_at)}
                  </td>
                  {canManage && (
                    <td className="px-5 py-3.5">
                      {m.role !== "owner" && (
                        <button
                          onClick={() => removeMember(m.id)}
                          className="p-1.5 rounded-lg text-zinc-600 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                          title="Remove member"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
