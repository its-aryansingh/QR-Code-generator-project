"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/auth";
import { QRSecurityPanel, SecurityOptions } from "@/components/dashboard/QRSecurityPanel";
import { ArrowLeft, Save, Trash2, QrCode, AlertCircle, ExternalLink } from "lucide-react";

interface QRRecord {
  id: string;
  title: string;
  content: string;
  qr_type: string;
  is_dynamic: boolean;
  is_active: boolean;
  short_code: string;
  scan_count: number;
  redirect_url?: string;
  expires_at?: string;
  max_scans?: number;
  geo_restrictions?: string;
  created_at: string;
  workspace_id?: string;
  folder_id?: string;
}

export default function QREditPage() {
  const params = useParams();
  const router = useRouter();
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
  const appBase = process.env.NEXT_PUBLIC_APP_URL || "http://localhost:8081";

  const id = params?.id as string;

  const [record, setRecord] = useState<QRRecord | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [preview, setPreview] = useState<string | null>(null);

  // Form state
  const [title, setTitle] = useState("");
  const [redirectUrl, setRedirectUrl] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [security, setSecurity] = useState<SecurityOptions>({
    password: "",
    expiresAt: "",
    maxScans: "",
    geoRestrictions: "",
  });

  useEffect(() => {
    if (!id || !accessToken) return;
    fetch(`${api}/qr/${id}`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((d) => {
        if (d.success) {
          const r: QRRecord = d.data;
          setRecord(r);
          setTitle(r.title || "");
          setRedirectUrl(r.redirect_url || r.content || "");
          setIsActive(r.is_active);
          setSecurity({
            password: "",
            expiresAt: r.expires_at ? new Date(r.expires_at).toISOString().slice(0, 16) : "",
            maxScans: r.max_scans != null ? String(r.max_scans) : "",
            geoRestrictions: r.geo_restrictions || "",
          });
        } else {
          setError("QR code not found.");
        }
      })
      .catch(() => setError("Failed to load QR code."))
      .finally(() => setLoading(false));
  }, [id, accessToken, api]);

  const loadPreview = useCallback(async () => {
    if (!record || !accessToken) return;
    const content = record.is_dynamic ? (redirectUrl || record.content) : record.content;
    try {
      const res = await fetch(`${api}/qr/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({ title, content, qr_type: record.qr_type, size: 300, is_dynamic: false }),
      });
      const data = await res.json();
      if (data.success) setPreview(data.data.qr_base64);
    } catch {}
  }, [record, redirectUrl, title, accessToken, api]);

  useEffect(() => {
    if (record) loadPreview();
  }, [record?.id]);

  const handleSave = async () => {
    if (!record) return;
    setSaving(true);
    setError("");
    const body: Record<string, unknown> = {
      title,
      is_active: isActive,
    };
    if (record.is_dynamic) body.redirect_url = redirectUrl;
    if (security.expiresAt) body.expires_at = new Date(security.expiresAt).toISOString();
    if (security.maxScans) body.max_scans = parseInt(security.maxScans);
    if (security.geoRestrictions) body.geo_restrictions = security.geoRestrictions;

    try {
      const res = await fetch(`${api}/qr/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.success) {
        setSaved(true);
        setTimeout(() => setSaved(false), 3000);
        loadPreview();
      } else {
        setError(data.error ?? "Save failed");
      }
    } catch {
      setError("Network error");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm("Delete this QR code? This cannot be undone.")) return;
    setDeleting(true);
    try {
      await fetch(`${api}/qr/${id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      router.push("/dashboard/qr-codes");
    } catch {
      setError("Delete failed");
      setDeleting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error && !record) {
    return (
      <div className="max-w-xl">
        <div className="flex items-center gap-2 p-4 bg-red-950/40 border border-red-800 rounded-xl text-red-400">
          <AlertCircle size={16} /> {error}
        </div>
      </div>
    );
  }

  if (!record) return null;

  const shortUrl = `${appBase}/r/${record.short_code}`;

  return (
    <div className="max-w-3xl space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={() => router.back()}
            className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800 rounded-lg"
          >
            <ArrowLeft size={18} />
          </button>
          <div>
            <h1 className="text-xl font-bold text-zinc-100">{record.title || "Untitled QR"}</h1>
            <p className="text-xs text-zinc-500 mt-0.5">
              {record.qr_type.toUpperCase()} · {record.scan_count} scans · created {new Date(record.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleDelete}
            disabled={deleting}
            className="flex items-center gap-1.5 px-3 py-2 text-sm text-red-400 border border-red-800/50 rounded-lg hover:bg-red-950/30 disabled:opacity-50"
          >
            <Trash2 size={14} /> Delete
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 text-sm bg-violet-600 hover:bg-violet-500 disabled:opacity-50 text-white font-medium rounded-lg"
          >
            {saving ? <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" /> : <Save size={14} />}
            {saved ? "Saved!" : saving ? "Saving…" : "Save Changes"}
          </button>
        </div>
      </div>

      <div className="grid sm:grid-cols-5 gap-5">
        {/* Left: form */}
        <div className="sm:col-span-3 space-y-5">
          {/* Basic info */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
            <h2 className="text-sm font-semibold text-zinc-300">Details</h2>

            <div>
              <label className="block text-xs text-zinc-500 mb-1.5">Title</label>
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full bg-zinc-950 border border-zinc-700 rounded-lg px-3 py-2.5 text-sm text-zinc-200 focus:outline-none focus:border-violet-500"
              />
            </div>

            {record.is_dynamic && (
              <div>
                <label className="block text-xs text-zinc-500 mb-1.5">Redirect URL</label>
                <input
                  value={redirectUrl}
                  onChange={(e) => setRedirectUrl(e.target.value)}
                  type="url"
                  className="w-full bg-zinc-950 border border-zinc-700 rounded-lg px-3 py-2.5 text-sm text-zinc-200 focus:outline-none focus:border-violet-500"
                />
              </div>
            )}

            {record.is_dynamic && (
              <div className="flex items-center gap-3 p-3 bg-zinc-950 border border-zinc-800 rounded-lg">
                <QrCode size={14} className="text-violet-400 shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-zinc-500 mb-0.5">Short URL</p>
                  <a href={shortUrl} target="_blank" className="text-sm font-mono text-violet-400 hover:text-violet-300 flex items-center gap-1 truncate">
                    {shortUrl} <ExternalLink size={11} />
                  </a>
                </div>
              </div>
            )}

            <label className="flex items-center gap-3 cursor-pointer">
              <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} className="w-4 h-4 rounded accent-violet-500" />
              <span className="text-sm text-zinc-300">Active (scans redirect / count)</span>
            </label>
          </div>

          {/* Security */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
            <QRSecurityPanel value={security} onChange={setSecurity} />
          </div>
        </div>

        {/* Right: preview */}
        <div className="sm:col-span-2 space-y-4">
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
            <h2 className="text-sm font-semibold text-zinc-300">Preview</h2>
            {preview ? (
              <img src={preview} alt="QR code preview" className="w-full rounded-lg border border-zinc-800" />
            ) : (
              <div className="aspect-square bg-zinc-950 rounded-lg border border-zinc-800 flex items-center justify-center">
                <div className="w-6 h-6 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
              </div>
            )}
            {preview && (
              <a
                href={preview}
                download={`${record.title || "qr-code"}.png`}
                className="block text-center text-xs text-zinc-500 hover:text-zinc-300 py-1.5 border border-zinc-800 rounded-lg hover:bg-zinc-800"
              >
                Download PNG
              </a>
            )}
          </div>

          {/* Stats */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
            <h2 className="text-sm font-semibold text-zinc-300">Stats</h2>
            <div className="space-y-2">
              <Row label="Total scans" value={record.scan_count.toLocaleString()} />
              <Row label="Type" value={record.qr_type.toUpperCase()} />
              <Row label="Mode" value={record.is_dynamic ? "Dynamic" : "Static"} />
              <Row label="Status" value={record.is_active ? "Active" : "Inactive"} color={record.is_active ? "text-emerald-400" : "text-zinc-500"} />
            </div>
          </div>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-950/40 border border-red-800 rounded-xl text-sm text-red-400">
          <AlertCircle size={14} /> {error}
        </div>
      )}
    </div>
  );
}

function Row({ label, value, color = "text-zinc-200" }: { label: string; value: string; color?: string }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-zinc-500">{label}</span>
      <span className={color}>{value}</span>
    </div>
  );
}
