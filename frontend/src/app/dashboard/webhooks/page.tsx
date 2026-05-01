"use client";

import { useEffect, useState, useCallback } from "react";
import { useAuthStore } from "@/lib/auth";
import { ChevronDown, ChevronUp, Trash2, Zap, Check, X } from "lucide-react";

interface WebhookData {
  id: string;
  url: string;
  events: string;
  is_active: boolean;
  fail_count: number;
  last_triggered?: string;
  description?: string;
  created_at: string;
}

interface WebhookLog {
  id: string;
  event: string;
  status_code: number;
  success: boolean;
  attempt: number;
  response_body?: string;
  created_at: string;
}

const EVENT_OPTIONS = [
  { value: "qr.created", label: "QR Created" },
  { value: "qr.updated", label: "QR Updated" },
  { value: "qr.deleted", label: "QR Deleted" },
  { value: "qr.scanned", label: "QR Scanned" },
  { value: "bulk.completed", label: "Bulk Generation Done" },
  { value: "member.joined", label: "Member Joined" },
];

export default function WebhooksPage() {
  const [webhooks, setWebhooks] = useState<WebhookData[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newUrl, setNewUrl] = useState("");
  const [newEvents, setNewEvents] = useState<string[]>(["qr.scanned"]);
  const [newDesc, setNewDesc] = useState("");
  const [loading, setLoading] = useState(true);
  const [workspaceId, setWorkspaceId] = useState<string | null>(null);
  const [expandedLogs, setExpandedLogs] = useState<Record<string, boolean>>({});
  const [logs, setLogs] = useState<Record<string, WebhookLog[]>>({});
  const [testing, setTesting] = useState<Record<string, boolean>>({});
  const [testResult, setTestResult] = useState<Record<string, "ok" | "fail" | null>>({});
  const [editingWebhook, setEditingWebhook] = useState<WebhookData | null>(null);
  const [editUrl, setEditUrl] = useState("");
  const [editEvents, setEditEvents] = useState<string[]>([]);
  const [editDesc, setEditDesc] = useState("");
  const [editActive, setEditActive] = useState(true);

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const loadWebhooks = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    try {
      const stored = localStorage.getItem("qrit_active_workspace");
      let wsId = stored;
      if (!wsId) {
        const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
        const wsData = await wsRes.json();
        if (wsData.success && wsData.data?.length > 0) wsId = wsData.data[0].id;
      }
      if (!wsId) return;
      setWorkspaceId(wsId);
      const res = await fetch(`${api}/workspaces/${wsId}/webhooks`, { headers: { Authorization: `Bearer ${token}` } });
      const data = await res.json();
      if (data.success) setWebhooks(data.data || []);
    } catch {}
    finally { setLoading(false); }
  }, [api]);

  useEffect(() => { loadWebhooks(); }, [loadWebhooks]);

  const handleCreate = async () => {
    if (!workspaceId || !newUrl) return;
    const token = useAuthStore.getState().accessToken;
    const res = await fetch(`${api}/workspaces/${workspaceId}/webhooks`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      body: JSON.stringify({ url: newUrl, events: newEvents, description: newDesc }),
    });
    const data = await res.json();
    if (data.success) {
      setWebhooks([...webhooks, data.data]);
      setNewUrl(""); setNewEvents(["qr.scanned"]); setNewDesc("");
      setShowCreate(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!workspaceId) return;
    const token = useAuthStore.getState().accessToken;
    await fetch(`${api}/workspaces/${workspaceId}/webhooks/${id}`, {
      method: "DELETE", headers: { Authorization: `Bearer ${token}` },
    });
    setWebhooks(webhooks.filter((w) => w.id !== id));
  };

  const loadLogs = async (webhookId: string) => {
    if (!workspaceId) return;
    const token = useAuthStore.getState().accessToken;
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/webhooks/${webhookId}/logs`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      if (data.success) setLogs((prev) => ({ ...prev, [webhookId]: data.data || [] }));
    } catch {}
  };

  const toggleLogs = (id: string) => {
    const next = !expandedLogs[id];
    setExpandedLogs((prev) => ({ ...prev, [id]: next }));
    if (next && !logs[id]) loadLogs(id);
  };

  const handleTest = async (wh: WebhookData) => {
    if (!workspaceId || testing[wh.id]) return;
    setTesting((p) => ({ ...p, [wh.id]: true }));
    setTestResult((p) => ({ ...p, [wh.id]: null }));
    const token = useAuthStore.getState().accessToken;
    try {
      // Fire a synthetic qr.scanned event via the webhook service trigger endpoint
      const res = await fetch(`${api}/workspaces/${workspaceId}/webhooks/${wh.id}/test`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      });
      setTestResult((p) => ({ ...p, [wh.id]: res.ok ? "ok" : "fail" }));
      // Refresh logs after test
      setTimeout(() => loadLogs(wh.id), 1500);
    } catch {
      setTestResult((p) => ({ ...p, [wh.id]: "fail" }));
    } finally {
      setTesting((p) => ({ ...p, [wh.id]: false }));
      setTimeout(() => setTestResult((p) => ({ ...p, [wh.id]: null })), 4000);
    }
  };

  const toggleEvent = (event: string) =>
    setNewEvents((prev) =>
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]
    );

  const openEdit = (wh: WebhookData) => {
    setEditingWebhook(wh);
    setEditUrl(wh.url);
    setEditEvents(wh.events.split(",").map((e) => e.trim()));
    setEditDesc(wh.description || "");
    setEditActive(wh.is_active);
  };

  const toggleEditEvent = (event: string) =>
    setEditEvents((prev) =>
      prev.includes(event) ? prev.filter((e) => e !== event) : [...prev, event]
    );

  const handleUpdate = async () => {
    if (!workspaceId || !editingWebhook) return;
    const token = useAuthStore.getState().accessToken;
    const res = await fetch(`${api}/workspaces/${workspaceId}/webhooks/${editingWebhook.id}`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      body: JSON.stringify({ url: editUrl, events: editEvents, description: editDesc, is_active: editActive }),
    });
    const data = await res.json();
    if (data.success) {
      setWebhooks(webhooks.map((w) => (w.id === editingWebhook.id ? data.data : w)));
      setEditingWebhook(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-8 h-8 border-2 border-zinc-700 border-t-white rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Webhooks</h1>
          <p className="text-zinc-500 mt-1">Get real-time notifications when events happen in your workspace</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-white text-zinc-950 text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors"
        >
          + Add Webhook
        </button>
      </div>

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-6 w-full max-w-lg">
            <h3 className="text-lg font-semibold text-white mb-4">New Webhook</h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-zinc-400 block mb-1">Endpoint URL</label>
                <input
                  type="url" value={newUrl} onChange={(e) => setNewUrl(e.target.value)}
                  className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                  placeholder="https://your-app.com/webhook"
                />
              </div>
              <div>
                <label className="text-sm text-zinc-400 block mb-2">Events</label>
                <div className="grid grid-cols-2 gap-2">
                  {EVENT_OPTIONS.map((ev) => (
                    <button key={ev.value} onClick={() => toggleEvent(ev.value)}
                      className={`text-left px-3 py-2 rounded-lg text-sm transition-all border ${newEvents.includes(ev.value)
                        ? "bg-violet-500/10 border-violet-500/30 text-violet-300"
                        : "bg-zinc-800/50 border-zinc-700/50 text-zinc-400 hover:border-zinc-600"
                      }`}>
                      {ev.label}
                    </button>
                  ))}
                </div>
              </div>
              <div>
                <label className="text-sm text-zinc-400 block mb-1">Description (optional)</label>
                <input type="text" value={newDesc} onChange={(e) => setNewDesc(e.target.value)}
                  className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                  placeholder="Slack notification for scans"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button onClick={() => setShowCreate(false)}
                  className="flex-1 px-4 py-2.5 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 transition-colors">
                  Cancel
                </button>
                <button onClick={handleCreate}
                  className="flex-1 px-4 py-2.5 bg-white text-zinc-950 font-medium rounded-lg hover:bg-zinc-200 transition-colors">
                  Create Webhook
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Webhooks List */}
      {webhooks.length === 0 ? (
        <div className="text-center py-16 bg-zinc-900/30 border border-zinc-800/60 rounded-xl">
          <p className="text-zinc-400 mb-1">No webhooks configured</p>
          <p className="text-zinc-600 text-sm">Add webhooks to get real-time event notifications</p>
        </div>
      ) : (
        <div className="space-y-3">
          {webhooks.map((wh) => (
            <div key={wh.id} className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl overflow-hidden hover:border-zinc-700 transition-all">
              {/* Webhook header */}
              <div className="p-5">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className={`w-2 h-2 rounded-full shrink-0 ${wh.is_active ? "bg-emerald-400" : "bg-zinc-600"}`} />
                      <code className="text-sm text-zinc-200 font-mono truncate">{wh.url}</code>
                    </div>
                    {wh.description && <p className="text-xs text-zinc-500 mt-1 ml-4">{wh.description}</p>}
                    <div className="flex flex-wrap gap-1.5 mt-3 ml-4">
                      {wh.events.split(",").map((e) => (
                        <span key={e} className="text-[10px] px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-400 border border-zinc-700/50">
                          {e.trim()}
                        </span>
                      ))}
                    </div>
                    {wh.last_triggered && (
                      <p className="text-[10px] text-zinc-600 mt-2 ml-4">
                        Last triggered: {new Date(wh.last_triggered).toLocaleString()}
                      </p>
                    )}
                  </div>

                  {/* Actions */}
                  <div className="flex items-center gap-2 ml-4 shrink-0">
                    {wh.fail_count > 0 && (
                      <span className="text-xs text-amber-400 bg-amber-500/10 px-2 py-1 rounded">
                        {wh.fail_count} failures
                      </span>
                    )}

                    {/* Test button */}
                    <button
                      onClick={() => handleTest(wh)}
                      disabled={testing[wh.id]}
                      title="Send test event"
                      className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all border ${
                        testResult[wh.id] === "ok"
                          ? "bg-emerald-900/30 border-emerald-700 text-emerald-400"
                          : testResult[wh.id] === "fail"
                          ? "bg-red-900/30 border-red-700 text-red-400"
                          : "bg-zinc-800 border-zinc-700 text-zinc-300 hover:bg-zinc-700"
                      } disabled:opacity-50`}
                    >
                      {testing[wh.id] ? (
                        <div className="w-3 h-3 border border-zinc-400 border-t-transparent rounded-full animate-spin" />
                      ) : testResult[wh.id] === "ok" ? (
                        <Check size={12} />
                      ) : testResult[wh.id] === "fail" ? (
                        <X size={12} />
                      ) : (
                        <Zap size={12} />
                      )}
                      {testing[wh.id] ? "Testing…" : testResult[wh.id] === "ok" ? "Sent" : testResult[wh.id] === "fail" ? "Failed" : "Test"}
                    </button>

                    {/* Logs toggle */}
                    <button
                      onClick={() => toggleLogs(wh.id)}
                      title="View delivery logs"
                      className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs bg-zinc-800 border border-zinc-700 text-zinc-300 hover:bg-zinc-700"
                    >
                      {expandedLogs[wh.id] ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                      Logs
                    </button>

                    {/* Edit */}
                    <button
                      onClick={() => openEdit(wh)}
                      className="p-1.5 text-zinc-600 hover:text-white transition-colors"
                      title="Edit webhook"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                    </button>

                    {/* Delete */}
                    <button
                      onClick={() => handleDelete(wh.id)}
                      className="p-1.5 text-zinc-600 hover:text-red-400 transition-colors"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                </div>
              </div>

              {/* Log viewer */}
              {expandedLogs[wh.id] && (
                <div className="border-t border-zinc-800 bg-zinc-950/60">
                  {!logs[wh.id] ? (
                    <div className="flex items-center justify-center py-6">
                      <div className="w-5 h-5 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
                    </div>
                  ) : logs[wh.id].length === 0 ? (
                    <p className="text-center text-zinc-600 text-xs py-6">No delivery logs yet</p>
                  ) : (
                    <div className="divide-y divide-zinc-800/60">
                      {logs[wh.id].slice(0, 10).map((log) => (
                        <div key={log.id} className="flex items-center gap-3 px-5 py-3 text-xs">
                          <span className={`w-2 h-2 rounded-full shrink-0 ${log.success ? "bg-emerald-400" : "bg-red-400"}`} />
                          <span className="text-zinc-400 font-mono w-24 shrink-0">{log.event}</span>
                          <span className={`font-mono w-10 shrink-0 ${log.success ? "text-emerald-400" : "text-red-400"}`}>
                            {log.status_code}
                          </span>
                          <span className="text-zinc-600">attempt {log.attempt}</span>
                          <span className="ml-auto text-zinc-600 shrink-0">
                            {new Date(log.created_at).toLocaleTimeString()}
                          </span>
                          {log.response_body && (
                            <span className="text-zinc-700 truncate max-w-[120px]" title={log.response_body}>
                              {log.response_body.slice(0, 40)}
                            </span>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
      {/* Edit Modal */}
      {editingWebhook && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-6 w-full max-w-lg">
            <h3 className="text-lg font-semibold text-white mb-4">Edit Webhook</h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-zinc-400 block mb-1">Endpoint URL</label>
                <input
                  type="url" value={editUrl} onChange={(e) => setEditUrl(e.target.value)}
                  className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                />
              </div>
              <div>
                <label className="text-sm text-zinc-400 block mb-2">Events</label>
                <div className="grid grid-cols-2 gap-2">
                  {EVENT_OPTIONS.map((ev) => (
                    <button key={ev.value} onClick={() => toggleEditEvent(ev.value)}
                      className={`text-left px-3 py-2 rounded-lg text-sm transition-all border ${editEvents.includes(ev.value)
                        ? "bg-violet-500/10 border-violet-500/30 text-violet-300"
                        : "bg-zinc-800/50 border-zinc-700/50 text-zinc-400 hover:border-zinc-600"
                      }`}>
                      {ev.label}
                    </button>
                  ))}
                </div>
              </div>
              <div>
                <label className="text-sm text-zinc-400 block mb-1">Description</label>
                <input type="text" value={editDesc} onChange={(e) => setEditDesc(e.target.value)}
                  className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                />
              </div>
              <div className="flex items-center justify-between p-3 bg-zinc-800/50 border border-zinc-700/50 rounded-lg">
                <div>
                  <p className="text-sm text-zinc-300 font-medium">Active Status</p>
                  <p className="text-xs text-zinc-500">Toggle to pause/resume this webhook</p>
                </div>
                <button
                  onClick={() => setEditActive(!editActive)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none ${editActive ? "bg-emerald-500" : "bg-zinc-600"}`}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${editActive ? "translate-x-6" : "translate-x-1"}`} />
                </button>
              </div>
              <div className="flex gap-3 pt-2">
                <button onClick={() => setEditingWebhook(null)}
                  className="flex-1 px-4 py-2.5 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 transition-colors">
                  Cancel
                </button>
                <button onClick={handleUpdate}
                  className="flex-1 px-4 py-2.5 bg-violet-600 hover:bg-violet-500 text-white font-medium rounded-lg transition-colors">
                  Save Changes
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
