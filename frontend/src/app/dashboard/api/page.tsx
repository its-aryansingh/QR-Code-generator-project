"use client";

import { useState, useEffect } from "react";
import { useAuthStore } from "@/lib/auth";
import { Copy, Check, Eye, EyeOff, RefreshCw, Lock, Terminal } from "lucide-react";

interface APIKeyInfo {
  api_key: string;
  plan: string;
  calls_today: number;
  calls_limit: number;
  can_access: boolean;
}

export default function APIKeyPage() {
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [keyInfo, setKeyInfo] = useState<APIKeyInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [showKey, setShowKey] = useState(false);
  const [copied, setCopied] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const [regenError, setRegenError] = useState("");

  useEffect(() => {
    loadAPIKey();
  }, [accessToken]);

  const loadAPIKey = async () => {
    if (!accessToken) return;
    setLoading(true);
    try {
      const res = await fetch(`${api}/api/key`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      const data = await res.json();
      if (data.success) setKeyInfo(data.data);
    } catch {}
    finally { setLoading(false); }
  };

  const copyToClipboard = () => {
    if (keyInfo?.api_key) {
      navigator.clipboard.writeText(keyInfo.api_key);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const regenerateKey = async () => {
    if (!confirm("This will invalidate your current API key immediately. Continue?")) return;
    setRegenerating(true);
    setRegenError("");
    try {
      const res = await fetch(`${api}/api/key/regenerate`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      const data = await res.json();
      if (data.success) {
        setKeyInfo((prev) => prev ? { ...prev, api_key: data.data.api_key } : prev);
        setShowKey(true);
      } else {
        setRegenError(data.error ?? "Failed to regenerate key");
      }
    } catch {
      setRegenError("Network error");
    } finally {
      setRegenerating(false);
    }
  };

  const maskKey = (key: string) =>
    showKey ? key : key.slice(0, 8) + "•".repeat(16) + key.slice(-4);

  const baseUrl = process.env.NEXT_PUBLIC_APP_URL || "http://localhost:8081";

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-3xl space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-zinc-100">API Access</h1>
        <p className="text-sm text-zinc-500 mt-1">Manage your API key and integrate QRit into your applications.</p>
      </div>

      {!keyInfo?.can_access ? (
        /* Upgrade prompt */
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-8 text-center">
          <Lock className="w-10 h-10 text-zinc-600 mx-auto mb-4" />
          <h2 className="text-lg font-semibold text-zinc-200 mb-2">API Access Locked</h2>
          <p className="text-sm text-zinc-500 mb-6">API access is available on Pro and Enterprise plans.</p>
          <a
            href="/dashboard/settings"
            className="inline-flex items-center px-5 py-2.5 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg transition-colors"
          >
            Upgrade Plan
          </a>
        </div>
      ) : (
        <div className="space-y-5">
          {/* API Key card */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-zinc-300">Your API Key</h2>
              <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-900/40 border border-emerald-700 text-emerald-400">
                Active
              </span>
            </div>

            <div className="flex gap-2">
              <code className="flex-1 bg-zinc-950 border border-zinc-800 rounded-lg px-3 py-2.5 text-sm font-mono text-zinc-300 overflow-hidden text-ellipsis">
                {keyInfo?.api_key ? maskKey(keyInfo.api_key) : "No key generated"}
              </code>
              <button
                onClick={() => setShowKey(!showKey)}
                className="px-3 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-400 hover:text-zinc-200"
                title={showKey ? "Hide" : "Show"}
              >
                {showKey ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
              <button
                onClick={copyToClipboard}
                className="px-3 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-400 hover:text-zinc-200"
                title="Copy"
              >
                {copied ? <Check size={15} className="text-emerald-400" /> : <Copy size={15} />}
              </button>
            </div>

            <div className="flex items-center justify-between pt-1">
              <p className="text-xs text-zinc-600">Never share your key in public repos or client-side code.</p>
              <button
                onClick={regenerateKey}
                disabled={regenerating}
                className="flex items-center gap-1.5 text-xs text-red-400 hover:text-red-300 disabled:opacity-50"
              >
                <RefreshCw size={12} className={regenerating ? "animate-spin" : ""} />
                {regenerating ? "Regenerating…" : "Regenerate"}
              </button>
            </div>
            {regenError && <p className="text-xs text-red-400">{regenError}</p>}
          </div>

          {/* Usage stats */}
          <div className="grid sm:grid-cols-3 gap-3">
            <StatCard label="API Calls Today" value={`${keyInfo.calls_today} / ${keyInfo.calls_limit === 0 ? "∞" : keyInfo.calls_limit}`} sub="Resets at midnight UTC" />
            <StatCard label="Plan" value={keyInfo.plan.charAt(0).toUpperCase() + keyInfo.plan.slice(1)} sub="API access included" />
            <StatCard label="Auth Method" value="API Key" sub="X-API-Key header" />
          </div>

          {/* Usage bar */}
          {keyInfo.calls_limit > 0 && (
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-4 space-y-2">
              <div className="flex justify-between text-xs text-zinc-500">
                <span>Daily usage</span>
                <span>{Math.round((keyInfo.calls_today / keyInfo.calls_limit) * 100)}%</span>
              </div>
              <div className="h-1.5 bg-zinc-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-violet-500 rounded-full transition-all"
                  style={{ width: `${Math.min(100, (keyInfo.calls_today / keyInfo.calls_limit) * 100)}%` }}
                />
              </div>
            </div>
          )}

          {/* Quick start */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
            <h2 className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
              <Terminal size={14} className="text-violet-400" /> Quick Start
            </h2>

            <CodeBlock label="Generate a QR Code" code={`curl -X POST ${baseUrl}/api/v1/api/generate \\
  -H "X-API-Key: ${keyInfo.api_key}" \\
  -H "Content-Type: application/json" \\
  -d '{"content": "https://example.com", "qr_type": "url", "size": 512}'`} />

            <CodeBlock label="Bulk Generate (Pro/Enterprise)" code={`curl -X POST ${baseUrl}/api/v1/api/bulk \\
  -H "X-API-Key: ${keyInfo.api_key}" \\
  -H "Content-Type: application/json" \\
  -d '{"items": [
  {"content": "https://example.com", "qr_type": "url"},
  {"content": "https://docs.example.com", "qr_type": "url"}
]}'`} />

            <CodeBlock label="CSV Bulk Upload (Workspace)" code={`curl -X POST ${baseUrl}/api/v1/workspaces/YOUR_WORKSPACE_ID/bulk \\
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \\
  -F "file=@codes.csv"
# Returns a ZIP archive of all generated QR codes`} />
          </div>

          {/* Authentication docs */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
            <h2 className="text-sm font-semibold text-zinc-300">Authentication</h2>
            <p className="text-sm text-zinc-500">Pass your API key in the <code className="text-violet-400 text-xs">X-API-Key</code> request header. Rate limits apply per plan:</p>
            <div className="grid sm:grid-cols-3 gap-3 text-xs">
              {[
                { plan: "Free", limit: "No API access" },
                { plan: "Pro", limit: "1,000 calls/day" },
                { plan: "Enterprise", limit: "10,000 calls/day" },
              ].map((row) => (
                <div key={row.plan} className="bg-zinc-950 border border-zinc-800 rounded-lg p-3">
                  <p className="font-semibold text-zinc-300 mb-1">{row.plan}</p>
                  <p className="text-zinc-500">{row.limit}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-4">
      <p className="text-[10px] uppercase tracking-wider text-zinc-600 mb-1">{label}</p>
      <p className="text-lg font-bold text-zinc-100">{value}</p>
      <p className="text-xs text-zinc-600 mt-0.5">{sub}</p>
    </div>
  );
}

function CodeBlock({ label, code }: { label: string; code: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div>
      <div className="flex items-center justify-between mb-1.5">
        <p className="text-xs text-zinc-500">{label}</p>
        <button
          onClick={() => { navigator.clipboard.writeText(code); setCopied(true); setTimeout(() => setCopied(false), 2000); }}
          className="text-xs text-zinc-500 hover:text-zinc-300 flex items-center gap-1"
        >
          {copied ? <><Check size={11} className="text-emerald-400" /> Copied</> : <><Copy size={11} /> Copy</>}
        </button>
      </div>
      <pre className="bg-zinc-950 border border-zinc-800 rounded-lg p-3 overflow-x-auto text-xs font-mono text-zinc-400 whitespace-pre-wrap">
        {code}
      </pre>
    </div>
  );
}
