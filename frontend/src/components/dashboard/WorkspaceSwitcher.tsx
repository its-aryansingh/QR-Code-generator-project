"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/auth";

interface Workspace {
  id: string;
  name: string;
  slug: string;
  plan: string;
  qr_count?: number;
  scan_count?: number;
}

interface WorkspaceSwitcherProps {
  onWorkspaceChange?: (ws: Workspace) => void;
  compact?: boolean;
}

export function WorkspaceSwitcher({ onWorkspaceChange, compact = false }: WorkspaceSwitcherProps) {
  const { accessToken } = useAuthStore();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [active, setActive] = useState<Workspace | null>(null);
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [loading, setLoading] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  useEffect(() => {
    if (!accessToken) return;
    fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((data) => {
        if (data.success && data.data?.length) {
          setWorkspaces(data.data);
          const saved = localStorage.getItem("qrit_active_workspace");
          const found = saved ? data.data.find((w: Workspace) => w.id === saved) : null;
          const initial = found ?? data.data[0];
          setActive(initial);
          onWorkspaceChange?.(initial);
        }
      })
      .catch(() => {});
  }, [accessToken]);

  // Close on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const select = (ws: Workspace) => {
    setActive(ws);
    localStorage.setItem("qrit_active_workspace", ws.id);
    onWorkspaceChange?.(ws);
    setOpen(false);
  };

  const createWorkspace = async () => {
    if (!newName.trim()) return;
    setLoading(true);
    try {
      const res = await fetch(`${api}/workspaces`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
        body: JSON.stringify({ name: newName.trim() }),
      }).then((r) => r.json());

      if (res.success) {
        const ws = res.data as Workspace;
        setWorkspaces((prev) => [...prev, ws]);
        select(ws);
        setCreating(false);
        setNewName("");
      }
    } finally {
      setLoading(false);
    }
  };

  const planBadge = (plan: string) => {
    const map: Record<string, string> = {
      free: "bg-zinc-800 text-zinc-400",
      starter: "bg-blue-600/20 text-blue-400",
      pro: "bg-violet-600/20 text-violet-400",
      enterprise: "bg-amber-600/20 text-amber-400",
    };
    return map[plan] ?? "bg-zinc-800 text-zinc-400";
  };

  if (!active) return null;

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-800 hover:border-zinc-700 transition-colors text-left group"
      >
        <div className="w-7 h-7 rounded-md bg-violet-600/25 flex items-center justify-center text-violet-300 text-sm font-bold flex-shrink-0">
          {active.name.charAt(0).toUpperCase()}
        </div>
        {!compact && (
          <>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-zinc-200 truncate">{active.name}</p>
              <p className={`text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded inline-block mt-0.5 ${planBadge(active.plan)}`}>
                {active.plan}
              </p>
            </div>
            <svg
              className={`w-4 h-4 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`}
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
          </>
        )}
      </button>

      {open && (
        <div className="absolute top-full left-0 right-0 mt-1.5 bg-zinc-900 border border-zinc-800 rounded-xl shadow-2xl shadow-black/40 z-50 py-1.5 overflow-hidden">
          <p className="px-3 pb-1 pt-0.5 text-[10px] font-semibold tracking-widest text-zinc-600 uppercase">
            Your Workspaces
          </p>

          <div className="max-h-48 overflow-y-auto">
            {workspaces.map((ws) => (
              <button
                key={ws.id}
                onClick={() => select(ws)}
                className={`w-full flex items-center gap-2.5 px-3 py-2 hover:bg-zinc-800 transition-colors text-left ${active.id === ws.id ? "bg-zinc-800/60" : ""}`}
              >
                <div className="w-7 h-7 rounded-md bg-violet-600/20 flex items-center justify-center text-violet-300 text-sm font-bold flex-shrink-0">
                  {ws.name.charAt(0).toUpperCase()}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-zinc-200 truncate">{ws.name}</p>
                  {ws.qr_count !== undefined && (
                    <p className="text-xs text-zinc-600">{ws.qr_count} QR codes</p>
                  )}
                </div>
                {active.id === ws.id && (
                  <svg className="w-4 h-4 text-violet-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                  </svg>
                )}
              </button>
            ))}
          </div>

          <div className="border-t border-zinc-800/70 mt-1 pt-1">
            {creating ? (
              <div className="px-3 py-2 space-y-2">
                <input
                  autoFocus
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") createWorkspace(); if (e.key === "Escape") setCreating(false); }}
                  placeholder="Workspace name…"
                  className="w-full px-3 py-1.5 bg-zinc-950 border border-zinc-700 rounded-lg text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-600"
                />
                <div className="flex gap-2">
                  <button
                    onClick={createWorkspace}
                    disabled={loading || !newName.trim()}
                    className="flex-1 py-1.5 bg-violet-600 text-white text-xs font-semibold rounded-lg hover:bg-violet-500 disabled:opacity-50 transition-colors"
                  >
                    {loading ? "Creating…" : "Create"}
                  </button>
                  <button
                    onClick={() => { setCreating(false); setNewName(""); }}
                    className="px-3 py-1.5 bg-zinc-800 text-zinc-400 text-xs rounded-lg hover:bg-zinc-700 transition-colors"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <button
                onClick={() => setCreating(true)}
                className="w-full flex items-center gap-2.5 px-3 py-2 hover:bg-zinc-800 transition-colors text-left text-zinc-400 text-sm"
              >
                <div className="w-7 h-7 rounded-md border border-dashed border-zinc-700 flex items-center justify-center text-zinc-600">
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                </div>
                New Workspace
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
