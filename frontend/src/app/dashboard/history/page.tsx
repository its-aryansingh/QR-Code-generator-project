"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/auth";

interface QRRecord {
  id: string;
  title: string;
  content: string;
  qr_type: string;
  is_dynamic: boolean;
  is_active: boolean;
  scan_count: number;
  created_at: string;
  short_code?: string;
}

const TYPE_LABELS: Record<string, string> = {
  url: "URL", wifi: "WiFi", vcard: "vCard", email: "Email",
  sms: "SMS", phone: "Phone", text: "Text", location: "Location",
  social: "Social", upi: "UPI", whatsapp: "WhatsApp", mecard: "MeCard",
  multilink: "MultiLink", event: "Event", bitcoin: "Bitcoin",
};

export default function HistoryPage() {
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [records, setRecords] = useState<QRRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState("");
  const limit = 20;

  const fetchHistory = useCallback(async () => {
    if (!accessToken) return;
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: String(limit), offset: String(page * limit) });
      if (search) params.set("search", search);
      const res = await fetch(`${api}/qr/history?${params}`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      const data = await res.json();
      if (data.success) {
        setRecords(data.data?.records ?? data.data ?? []);
        setTotal(data.total ?? data.data?.total ?? 0);
      }
    } finally {
      setLoading(false);
    }
  }, [accessToken, api, page, search]);

  useEffect(() => { fetchHistory(); }, [fetchHistory]);
  useEffect(() => { setPage(0); }, [search]);

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-zinc-100">QR Code History</h1>
          <p className="text-sm text-zinc-500 mt-1">All QR codes you've generated</p>
        </div>
        <Link
          href="/dashboard/create"
          className="flex items-center gap-1.5 px-4 py-2 bg-white text-zinc-950 text-sm font-semibold rounded-xl hover:bg-zinc-100 transition-colors"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
          New QR Code
        </Link>
      </div>

      {/* Search */}
      <div className="relative max-w-sm">
        <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search QR codes…"
          className="w-full pl-9 pr-4 py-2 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
        />
      </div>

      {/* Table */}
      <div className="bg-zinc-900 rounded-xl border border-zinc-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-800 bg-zinc-950/50">
                <th className="px-4 py-3 text-left font-semibold text-zinc-400">Title / Content</th>
                <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden sm:table-cell">Type</th>
                <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden md:table-cell">Mode</th>
                <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden sm:table-cell">Scans</th>
                <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden lg:table-cell">Created</th>
                <th className="px-4 py-3 text-left font-semibold text-zinc-400">Status</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-zinc-800/60 animate-pulse">
                    <td colSpan={6} className="px-4 py-3">
                      <div className="h-4 bg-zinc-800 rounded w-full" />
                    </td>
                  </tr>
                ))
              ) : records.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-16 text-center">
                    <p className="text-zinc-600">{search ? "No QR codes match your search." : "No QR codes yet."}</p>
                    {!search && (
                      <Link href="/dashboard/create" className="text-violet-400 hover:text-violet-300 text-sm mt-2 inline-block">
                        Create your first QR code →
                      </Link>
                    )}
                  </td>
                </tr>
              ) : (
                records.map((rec) => (
                  <tr
                    key={rec.id}
                    className="border-b border-zinc-800/60 hover:bg-zinc-800/30 transition-colors"
                  >
                    <td className="px-4 py-3">
                      <Link href={`/dashboard/qr-codes/${rec.id}`} className="group">
                        <div className="font-medium text-zinc-200 group-hover:text-violet-400 truncate max-w-[220px] transition-colors" title={rec.title || rec.content}>
                          {rec.title || rec.content?.substring(0, 50)}
                        </div>
                        {rec.short_code && (
                          <p className="text-xs text-zinc-600 font-mono">/r/{rec.short_code}</p>
                        )}
                      </Link>
                    </td>
                    <td className="px-4 py-3 hidden sm:table-cell">
                      <span className="px-2 py-0.5 bg-zinc-800 text-zinc-400 rounded text-xs font-medium">
                        {TYPE_LABELS[rec.qr_type] ?? rec.qr_type}
                      </span>
                    </td>
                    <td className="px-4 py-3 hidden md:table-cell">
                      {rec.is_dynamic ? (
                        <span className="text-xs text-violet-400 font-semibold">Dynamic</span>
                      ) : (
                        <span className="text-xs text-zinc-600">Static</span>
                      )}
                    </td>
                    <td className="px-4 py-3 hidden sm:table-cell">
                      <span className="font-mono text-zinc-300 text-xs">{rec.scan_count.toLocaleString()}</span>
                    </td>
                    <td className="px-4 py-3 hidden lg:table-cell text-zinc-500 text-xs">
                      {new Date(rec.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block w-2 h-2 rounded-full ${rec.is_active ? "bg-emerald-400" : "bg-zinc-600"}`}
                        title={rec.is_active ? "Active" : "Inactive"}
                      />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-zinc-800">
            <p className="text-xs text-zinc-500">
              Showing {page * limit + 1}–{Math.min((page + 1) * limit, total)} of {total}
            </p>
            <div className="flex gap-1">
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="px-3 py-1.5 text-xs rounded-lg border border-zinc-700 text-zinc-400 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                Previous
              </button>
              <button
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
                className="px-3 py-1.5 text-xs rounded-lg border border-zinc-700 text-zinc-400 hover:bg-zinc-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
