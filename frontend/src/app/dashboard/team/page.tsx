"use client";

import { useState, useEffect } from "react";
import { useAuthStore } from "@/lib/auth";
import { TeamPanel } from "@/components/dashboard/TeamPanel";

export default function TeamPage() {
  const { accessToken } = useAuthStore();
  const [workspaceId, setWorkspaceId] = useState<string>("");
  const [currentUserRole, setCurrentUserRole] = useState("viewer");

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  useEffect(() => {
    const stored = localStorage.getItem("qrit_active_workspace");
    if (stored) { setWorkspaceId(stored); return; }
    if (!accessToken) return;
    fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((data) => {
        if (data.success && data.data?.length) {
          setWorkspaceId(data.data[0].id);
          localStorage.setItem("qrit_active_workspace", data.data[0].id);
        }
      })
      .catch(() => {});
  }, [accessToken]);

  // Fetch current user's role in active workspace
  useEffect(() => {
    if (!workspaceId || !accessToken) return;
    fetch(`${api}/workspaces/${workspaceId}/members`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((data) => {
        if (data.success) {
          // Compare user_id with JWT subject — use profile endpoint as proxy
          fetch(`${api}/user/profile`, { headers: { Authorization: `Bearer ${accessToken}` } })
            .then((r) => r.json())
            .then((profile) => {
              const me = (data.data ?? []).find((m: any) => m.user_id === profile.data?.id);
              if (me) setCurrentUserRole(me.role);
            });
        }
      })
      .catch(() => {});
  }, [workspaceId, accessToken]);

  if (!workspaceId) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-4">
        <div className="w-10 h-10 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
        <p className="text-zinc-500 text-sm">Loading workspace…</p>
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      <TeamPanel workspaceId={workspaceId} currentUserRole={currentUserRole} />
    </div>
  );
}
