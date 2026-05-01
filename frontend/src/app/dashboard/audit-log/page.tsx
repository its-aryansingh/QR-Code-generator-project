"use client";

import { useEffect, useState, useCallback } from "react";
import { useAuthStore } from "@/lib/auth";
import { Download, Filter } from "lucide-react";

interface AuditEntry {
  id: string;
  action: string;
  resource: string;
  details?: string;
  ip_address?: string;
  created_at: string;
  user?: { email: string; name?: string };
}

const ACTION_COLORS: Record<string, string> = {
  create:        "text-emerald-400 bg-emerald-500/10",
  update:        "text-blue-400 bg-blue-500/10",
  delete:        "text-red-400 bg-red-500/10",
  invite:        "text-violet-400 bg-violet-500/10",
  join:          "text-amber-400 bg-amber-500/10",
  leave:         "text-zinc-400 bg-zinc-700/30",
  export:        "text-cyan-400 bg-cyan-500/10",
  bulk_generate: "text-yellow-400 bg-yellow-500/10",
};

const ACTION_ICONS: Record<string, string> = {
  create: "🟢", update: "🔵", delete: "🔴", invite: "💌",
  join: "👋", leave: "👤", export: "📤", bulk_generate: "⚡",
};

const ACTIONS = ["create", "update", "delete", "invite", "join", "leave", "export", "bulk_generate"];
const RESOURCES = ["qr_code", "workspace", "member", "folder", "webhook", "lead_page", "api_key"];

export default function AuditLogPage() {
  const [logs, setLogs]             = useState<AuditEntry[]>([]);
  const [total, setTotal]           = useState(0);
  const [loading, setLoading]       = useState(true);
  const [exporting, setExporting]   = useState(false);
  const [offset, setOffset]         = useState(0);
  const [workspaceId, setWorkspaceId] = useState("");
  const [filterAction, setFilterAction]   = useState("");
  const [filterResource, setFilterResource] = useState("");
  const [filterFrom, setFilterFrom] = useState("");
  const [filterTo, setFilterTo]     = useState("");
  const limit = 20;

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const loadLogs = useCallback(async () => {
    setLoading(true);
    const token = useAuthStore.getState().accessToken;
    try {
      let wsId = workspaceId;
      if (!wsId) {
        const stored = localStorage.getItem("qrit_active_workspace");
        if (stored) { wsId = stored; setWorkspaceId(stored); }
        else {
          const r = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
          const d = await r.json();
          if (d.success && d.data?.length) { wsId = d.data[0].id; setWorkspaceId(wsId); }
        }
      }
      if (!wsId) return;

      const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      if (filterAction)   params.set("action", filterAction);
      if (filterResource) params.set("resource", filterResource);
      if (filterFrom)     params.set("from", new Date(filterFrom).toISOString());
      if (filterTo)       params.set("to", new Date(filterTo + "T23:59:59").toISOString());

      const res = await fetch(`${api}/workspaces/${wsId}/audit-logs?${params}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success) {
        setLogs(data.data || []);
        setTotal(data.total || 0);
      }
    } catch {}
    finally { setLoading(false); }
  }, [offset, filterAction, filterResource, filterFrom, filterTo, workspaceId, api]);

  useEffect(() => { loadLogs(); }, [loadLogs]);

  const handleFilterChange = () => { setOffset(0); };

  const handleExportCSV = async () => {
    if (!workspaceId || exporting) return;
    setExporting(true);
    const token = useAuthStore.getState().accessToken;
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/export/analytics`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url; a.download = "audit-log.csv"; a.click();
      URL.revokeObjectURL(url);
    } catch {}
    finally { setExporting(false); }
  };

  const selectCls = "bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-300 focus:outline-none focus:border-violet-500";
  const inputCls  = "bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-300 focus:outline-none focus:border-violet-500 [color-scheme:dark]";

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white">Audit Log</h1>
          <p className="text-zinc-500 text-sm mt-1">Complete workspace activity history for compliance and security</p>
        </div>
        <button
          onClick={handleExportCSV}
          disabled={exporting || !workspaceId}
          className="flex items-center gap-1.5 px-3 py-2 text-sm bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg hover:bg-zinc-700 disabled:opacity-40"
        >
          <Download size={14} />
          {exporting ? "Exporting…" : "Export CSV"}
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 p-4 bg-zinc-900 border border-zinc-800 rounded-xl">
        <div className="flex items-center gap-2 text-xs text-zinc-500 mr-1">
          <Filter size={12} /> Filters
        </div>
        <select value={filterAction} onChange={(e) => { setFilterAction(e.target.value); handleFilterChange(); }} className={selectCls}>
          <option value="">All actions</option>
          {ACTIONS.map((a) => <option key={a} value={a}>{a}</option>)}
        </select>
        <select value={filterResource} onChange={(e) => { setFilterResource(e.target.value); handleFilterChange(); }} className={selectCls}>
          <option value="">All resources</option>
          {RESOURCES.map((r) => <option key={r} value={r}>{r.replace("_", " ")}</option>)}
        </select>
        <input type="date" value={filterFrom} onChange={(e) => { setFilterFrom(e.target.value); handleFilterChange(); }} className={inputCls} placeholder="From" />
        <input type="date" value={filterTo} onChange={(e) => { setFilterTo(e.target.value); handleFilterChange(); }} className={inputCls} placeholder="To" />
        {(filterAction || filterResource || filterFrom || filterTo) && (
          <button
            onClick={() => { setFilterAction(""); setFilterResource(""); setFilterFrom(""); setFilterTo(""); setOffset(0); }}
            className="text-xs text-zinc-500 hover:text-zinc-300 px-2 py-1 rounded border border-zinc-700 bg-zinc-800"
          >
            Clear
          </button>
        )}
      </div>

      {/* Table */}
      <div className="bg-zinc-900/30 border border-zinc-800/60 rounded-xl overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
          </div>
        ) : logs.length === 0 ? (
          <div className="text-center py-16">
            <p className="text-zinc-500">No audit entries found</p>
            {(filterAction || filterResource || filterFrom || filterTo) && (
              <p className="text-xs text-zinc-600 mt-1">Try clearing filters</p>
            )}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-zinc-800/60">
                    {["Action", "User", "Resource", "Details", "IP", "Time"].map((h) => (
                      <th key={h} className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => (
                    <tr key={log.id} className="border-b border-zinc-800/30 hover:bg-zinc-800/20 transition-colors">
                      <td className="px-5 py-3">
                        <span className={`text-xs px-2.5 py-1 rounded-full ${ACTION_COLORS[log.action] ?? ACTION_COLORS.update}`}>
                          {ACTION_ICONS[log.action] ?? "📋"} {log.action}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-sm text-zinc-300">{log.user?.name || log.user?.email || "System"}</td>
                      <td className="px-5 py-3">
                        <span className="text-xs px-2 py-0.5 rounded bg-zinc-800 text-zinc-400">{log.resource}</span>
                      </td>
                      <td className="px-5 py-3 text-sm text-zinc-500 max-w-xs truncate" title={log.details}>{log.details || "—"}</td>
                      <td className="px-5 py-3 text-xs text-zinc-600 font-mono">{log.ip_address || "—"}</td>
                      <td className="px-5 py-3 text-xs text-zinc-500 whitespace-nowrap">{new Date(log.created_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {total > limit && (
              <div className="flex items-center justify-between px-5 py-3 border-t border-zinc-800/60">
                <p className="text-xs text-zinc-500">
                  {offset + 1}–{Math.min(offset + limit, total)} of {total}
                </p>
                <div className="flex gap-2">
                  <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}
                    className="px-3 py-1.5 text-xs bg-zinc-800 text-zinc-400 rounded-lg hover:text-white disabled:opacity-30">
                    Previous
                  </button>
                  <button disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}
                    className="px-3 py-1.5 text-xs bg-zinc-800 text-zinc-400 rounded-lg hover:text-white disabled:opacity-30">
                    Next
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
