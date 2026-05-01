"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/auth";
import { QRSecurityPanel, SecurityOptions } from "@/components/dashboard/QRSecurityPanel";
import {
  ArrowLeft, Save, ExternalLink, ToggleLeft, ToggleRight,
  BarChart3, Globe, Smartphone, Monitor, AlertCircle, Check
} from "lucide-react";
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell
} from "recharts";

const inputCls = "w-full bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2.5 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500 transition-colors";
const labelCls = "block text-xs text-zinc-500 mb-1.5 font-medium";
const COLORS = ["#8b5cf6", "#06b6d4", "#f59e0b", "#ef4444", "#10b981", "#ec4899"];

interface QRRecord {
  id: string;
  title: string;
  content: string;
  qr_type: string;
  is_dynamic: boolean;
  is_active: boolean;
  redirect_url: string;
  short_code: string;
  scan_count: number;
  tags: string;
  password?: string;
  max_scans?: number;
  geo_restrictions?: string;
  expires_at?: string;
  customization?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

interface AnalyticsData {
  total_scans: number;
  unique_visitors: number;
  scans_by_date: { date: string; count: number }[];
  devices: { name: string; value: number }[];
  countries: { name: string; value: number }[];
  browsers: { name: string; value: number }[];
}

export default function EditQRPage() {
  const params = useParams();
  const router = useRouter();
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
  const qrId = params.id as string;

  const [record, setRecord] = useState<QRRecord | null>(null);
  const [analytics, setAnalytics] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  // Editable fields
  const [title, setTitle] = useState("");
  const [redirectUrl, setRedirectUrl] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [tags, setTags] = useState("");
  const [security, setSecurity] = useState<SecurityOptions>({
    password: "", expiresAt: "", maxScans: "", geoRestrictions: "",
  });

  // Load QR record
  useEffect(() => {
    if (!accessToken || !qrId) return;
    const load = async () => {
      try {
        const res = await fetch(`${api}/qr/${qrId}`, {
          headers: { Authorization: `Bearer ${accessToken}` },
        });
        const data = await res.json();
        if (data.success) {
          const r = data.data;
          setRecord(r);
          setTitle(r.title || "");
          setRedirectUrl(r.redirect_url || "");
          setIsActive(r.is_active);
          setTags(r.tags || "");
          setSecurity({
            password: "",
            expiresAt: r.expires_at ? r.expires_at.slice(0, 16) : "",
            maxScans: r.max_scans ? String(r.max_scans) : "",
            geoRestrictions: r.geo_restrictions || "",
          });
        }
      } catch {
        setError("Failed to load QR code");
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [accessToken, qrId, api]);

  // Load analytics
  useEffect(() => {
    if (!accessToken || !qrId) return;
    const loadAnalytics = async () => {
      try {
        const res = await fetch(`${api}/qr/${qrId}/analytics`, {
          headers: { Authorization: `Bearer ${accessToken}` },
        });
        const data = await res.json();
        if (data.success) setAnalytics(data.data);
      } catch { /* analytics are optional */ }
    };
    loadAnalytics();
  }, [accessToken, qrId, api]);

  const handleSave = useCallback(async () => {
    if (!accessToken || !qrId) return;
    setSaving(true);
    setError("");
    setSaved(false);

    const body: Record<string, unknown> = { title, is_active: isActive, tags };
    if (record?.is_dynamic && redirectUrl) body.redirect_url = redirectUrl;
    if (security.password) body.password = security.password;
    if (security.expiresAt) body.expires_at = new Date(security.expiresAt).toISOString();
    if (security.maxScans) body.max_scans = parseInt(security.maxScans);
    if (security.geoRestrictions) body.geo_restrictions = security.geoRestrictions;

    try {
      const res = await fetch(`${api}/qr/${qrId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.success) {
        setSaved(true);
        setTimeout(() => setSaved(false), 3000);
      } else {
        setError(data.error || "Failed to update");
      }
    } catch {
      setError("Network error");
    } finally {
      setSaving(false);
    }
  }, [accessToken, qrId, title, redirectUrl, isActive, tags, security, record, api]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-8 h-8 border-2 border-zinc-700 border-t-violet-500 rounded-full animate-spin" />
      </div>
    );
  }

  if (!record) {
    return (
      <div className="text-center py-16">
        <p className="text-zinc-400">QR code not found</p>
        <button onClick={() => router.back()} className="mt-4 text-violet-400 hover:text-violet-300 text-sm">← Go back</button>
      </div>
    );
  }

  return (
    <div className="max-w-4xl space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button onClick={() => router.back()} className="p-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-white transition-colors">
            <ArrowLeft size={16} />
          </button>
          <div>
            <h1 className="text-xl font-bold text-zinc-100">Edit QR Code</h1>
            <p className="text-xs text-zinc-500 mt-0.5 font-mono">{record.short_code ? `/r/${record.short_code}` : record.qr_type}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {saved && <span className="text-sm text-emerald-400 flex items-center gap-1"><Check size={14} /> Saved</span>}
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-5 py-2 bg-violet-600 hover:bg-violet-500 disabled:opacity-60 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {saving ? <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" /> : <Save size={14} />}
            {saving ? "Saving…" : "Save Changes"}
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-950/40 border border-red-800 rounded-lg text-sm text-red-400">
          <AlertCircle size={14} /> {error}
        </div>
      )}

      {/* Main content grid */}
      <div className="grid lg:grid-cols-3 gap-6">
        {/* Left column: edit fields */}
        <div className="lg:col-span-2 space-y-5">
          {/* Basic Info */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
            <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider">Basic Info</h2>
            <div>
              <label className={labelCls}>Title</label>
              <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="My QR Code" className={inputCls} />
            </div>

            {record.is_dynamic && (
              <div>
                <label className={labelCls}>Redirect URL</label>
                <div className="flex gap-2">
                  <input value={redirectUrl} onChange={(e) => setRedirectUrl(e.target.value)} placeholder="https://example.com" className={inputCls} />
                  {redirectUrl && (
                    <a href={redirectUrl} target="_blank" rel="noopener noreferrer"
                      className="shrink-0 p-2.5 rounded-lg bg-zinc-800 border border-zinc-700 text-zinc-400 hover:text-white transition-colors">
                      <ExternalLink size={16} />
                    </a>
                  )}
                </div>
              </div>
            )}

            <div>
              <label className={labelCls}>Tags (comma-separated)</label>
              <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="marketing, landing-page" className={inputCls} />
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-zinc-800">
              <div>
                <p className="text-sm text-zinc-300">Active Status</p>
                <p className="text-xs text-zinc-500 mt-0.5">Inactive QR codes won&apos;t redirect</p>
              </div>
              <button onClick={() => setIsActive(!isActive)}
                className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-all border ${isActive
                  ? "bg-emerald-900/30 border-emerald-700 text-emerald-400"
                  : "bg-zinc-800 border-zinc-700 text-zinc-400"
                }`}>
                {isActive ? <ToggleRight size={16} /> : <ToggleLeft size={16} />}
                {isActive ? "Active" : "Inactive"}
              </button>
            </div>
          </div>

          {/* Security */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
            <QRSecurityPanel value={security} onChange={setSecurity} />
          </div>

          {/* Scan Analytics Chart */}
          {analytics && analytics.scans_by_date && analytics.scans_by_date.length > 0 && (
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
                  <BarChart3 size={14} className="text-violet-400" /> Scan Analytics
                </h2>
                <span className="text-xs text-zinc-500">Last 30 days</span>
              </div>
              <div className="h-48">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={analytics.scans_by_date}>
                    <defs>
                      <linearGradient id="scanGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                    <XAxis dataKey="date" tick={{ fill: "#71717a", fontSize: 10 }} tickFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "short", day: "numeric" })} />
                    <YAxis tick={{ fill: "#71717a", fontSize: 10 }} />
                    <Tooltip
                      contentStyle={{ background: "#18181b", border: "1px solid #3f3f46", borderRadius: "8px", fontSize: 12 }}
                      labelStyle={{ color: "#a1a1aa" }}
                      itemStyle={{ color: "#8b5cf6" }}
                    />
                    <Area type="monotone" dataKey="count" stroke="#8b5cf6" fillOpacity={1} fill="url(#scanGradient)" strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          )}

          {/* Device & Country breakdown */}
          {analytics && (analytics.devices?.length > 0 || analytics.countries?.length > 0) && (
            <div className="grid sm:grid-cols-2 gap-5">
              {analytics.devices?.length > 0 && (
                <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
                  <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-3 flex items-center gap-1.5">
                    <Smartphone size={12} /> Devices
                  </h3>
                  <div className="h-32">
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie data={analytics.devices} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={50} strokeWidth={0}>
                          {analytics.devices.map((_, i) => <Cell key={i} fill={COLORS[i % COLORS.length]} />)}
                        </Pie>
                        <Tooltip contentStyle={{ background: "#18181b", border: "1px solid #3f3f46", borderRadius: "8px", fontSize: 11 }} />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {analytics.devices.map((d, i) => (
                      <span key={d.name} className="flex items-center gap-1 text-[10px] text-zinc-400">
                        <span className="w-2 h-2 rounded-full" style={{ backgroundColor: COLORS[i % COLORS.length] }} />
                        {d.name} ({d.value})
                      </span>
                    ))}
                  </div>
                </div>
              )}
              {analytics.countries?.length > 0 && (
                <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
                  <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-3 flex items-center gap-1.5">
                    <Globe size={12} /> Top Countries
                  </h3>
                  <div className="space-y-2">
                    {analytics.countries.slice(0, 6).map((c) => (
                      <div key={c.name} className="flex items-center justify-between text-xs">
                        <span className="text-zinc-300">{c.name}</span>
                        <span className="text-zinc-500">{c.value} scans</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Right column: info sidebar */}
        <div className="space-y-5">
          {/* Meta */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Details</h3>
            <InfoRow label="Type" value={record.qr_type} />
            <InfoRow label="Mode" value={record.is_dynamic ? "Dynamic" : "Static"} />
            <InfoRow label="Scans" value={String(record.scan_count)} />
            <InfoRow label="Created" value={new Date(record.created_at).toLocaleDateString()} />
            <InfoRow label="Updated" value={new Date(record.updated_at).toLocaleDateString()} />
            {record.short_code && <InfoRow label="Short Code" value={record.short_code} />}
          </div>

          {/* Quick Stats */}
          {analytics && (
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
              <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Performance</h3>
              <div className="grid grid-cols-2 gap-3">
                <StatCard label="Total Scans" value={analytics.total_scans ?? record.scan_count} />
                <StatCard label="Unique" value={analytics.unique_visitors ?? "-"} />
              </div>
              {analytics.browsers?.length > 0 && (
                <div>
                  <p className="text-[10px] text-zinc-500 uppercase tracking-wider mb-1.5 flex items-center gap-1"><Monitor size={10} /> Top Browser</p>
                  <p className="text-sm text-zinc-200">{analytics.browsers[0].name}</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-zinc-500">{label}</span>
      <span className="text-zinc-200 font-medium">{value}</span>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="bg-zinc-800/50 rounded-lg p-3 text-center">
      <p className="text-lg font-bold text-white">{typeof value === "number" ? value.toLocaleString() : value}</p>
      <p className="text-[10px] text-zinc-500 mt-0.5">{label}</p>
    </div>
  );
}
