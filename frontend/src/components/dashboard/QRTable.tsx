"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/auth";
import { toast } from "sonner";

interface QRRecord {
  id: string;
  title: string;
  content: string;
  qr_type: string;
  short_code?: string;
  is_dynamic: boolean;
  is_active: boolean;
  scan_count: number;
  created_at: string;
  folder_id?: string;
}

interface QRTableProps {
  workspaceId: string;
  folderId?: string;
  onSelect?: (ids: string[]) => void;
}

const TYPE_LABELS: Record<string, string> = {
  url: "URL", wifi: "WiFi", vcard: "vCard", email: "Email",
  sms: "SMS", phone: "Phone", text: "Text", location: "Location",
  social: "Social", upi: "UPI", whatsapp: "WhatsApp", mecard: "MeCard",
  multilink: "MultiLink", event: "Event",
};

export function QRTable({ workspaceId, folderId, onSelect }: QRTableProps) {
  const { accessToken } = useAuthStore();
  const [records, setRecords] = useState<QRRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sortBy, setSortBy] = useState<"created_at" | "scan_count" | "title">("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const limit = 20;
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const fetchRecords = useCallback(async () => {
    if (!workspaceId || !accessToken) return;
    setLoading(true);
    try {
      const params = new URLSearchParams({
        limit: String(limit),
        offset: String(page * limit),
      });
      if (search) params.set("search", search);
      if (folderId) params.set("folder_id", folderId);

      const res = await fetch(
        `${api}/workspaces/${workspaceId}/qr?${params}`,
        { headers: { Authorization: `Bearer ${accessToken}` } }
      ).then((r) => r.json());

      if (res.success) {
        setRecords(res.data ?? []);
        setTotal(res.total ?? 0);
      }
    } finally {
      setLoading(false);
    }
  }, [workspaceId, accessToken, page, search, folderId]);

  useEffect(() => { fetchRecords(); }, [fetchRecords]);

  // Debounce search
  useEffect(() => {
    setPage(0);
  }, [search, folderId]);

  const sortedRecords = [...records].sort((a, b) => {
    let av: number | string = a[sortBy] as any;
    let bv: number | string = b[sortBy] as any;
    if (sortBy === "created_at") { av = new Date(av as string).getTime(); bv = new Date(bv as string).getTime(); }
    if (av < bv) return sortDir === "asc" ? -1 : 1;
    if (av > bv) return sortDir === "asc" ? 1 : -1;
    return 0;
  });

  const toggleSort = (col: typeof sortBy) => {
    if (sortBy === col) setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    else { setSortBy(col); setSortDir("desc"); }
  };

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      onSelect?.(Array.from(next));
      return next;
    });
  };

  const toggleAll = () => {
    if (selected.size === records.length) {
      setSelected(new Set());
      onSelect?.([]);
    } else {
      const all = new Set(records.map((r) => r.id));
      setSelected(all);
      onSelect?.(Array.from(all));
    }
  };

  const deleteSelected = async () => {
    if (!confirm(`Delete ${selected.size} QR code(s)?`)) return;
    try {
      await fetch(`${api}/workspaces/${workspaceId}/qr/bulk-delete`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
        body: JSON.stringify({ qr_ids: Array.from(selected) }),
      });
      toast.success("Deleted successfully");
      setSelected(new Set());
      fetchRecords();
    } catch {
      toast.error("Delete failed");
    }
  };

  const totalPages = Math.ceil(total / limit);

  const SortIcon = ({ col }: { col: string }) =>
    sortBy === col ? (
      <svg className="w-3 h-3 inline ml-1 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d={sortDir === "asc" ? "M5 15l7-7 7 7" : "M19 9l-7 7-7-7"} />
      </svg>
    ) : (
      <svg className="w-3 h-3 inline ml-1 text-zinc-700 group-hover:text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4M8 15l4 4 4-4" />
      </svg>
    );

  return (
    <div className="bg-zinc-900 rounded-xl border border-zinc-800 overflow-hidden">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-zinc-800">
        <div className="relative flex-1 min-w-[200px]">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search QR codes…"
            className="w-full pl-9 pr-4 py-2 bg-zinc-950 border border-zinc-800 rounded-lg text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
          />
        </div>

        <span className="text-sm text-zinc-500">{total} total</span>

        {selected.size > 0 && (
          <button
            onClick={deleteSelected}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-red-600/15 border border-red-500/30 text-red-400 text-xs font-semibold rounded-lg hover:bg-red-600/25 transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            Delete {selected.size}
          </button>
        )}

        <button
          onClick={fetchRecords}
          className="p-2 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
          title="Refresh"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-zinc-800 bg-zinc-950/50">
              <th className="w-10 px-4 py-3 text-left">
                <input
                  type="checkbox"
                  checked={records.length > 0 && selected.size === records.length}
                  onChange={toggleAll}
                  className="w-4 h-4 rounded border-zinc-700 bg-zinc-900 accent-violet-600"
                />
              </th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400 cursor-pointer group" onClick={() => toggleSort("title")}>
                Title <SortIcon col="title" />
              </th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden md:table-cell">Type</th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400 hidden lg:table-cell">Dynamic</th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400 cursor-pointer group hidden sm:table-cell" onClick={() => toggleSort("scan_count")}>
                Scans <SortIcon col="scan_count" />
              </th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400 cursor-pointer group hidden lg:table-cell" onClick={() => toggleSort("created_at")}>
                Created <SortIcon col="created_at" />
              </th>
              <th className="px-4 py-3 text-left font-semibold text-zinc-400">Status</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="border-b border-zinc-800/60 animate-pulse">
                  <td colSpan={7} className="px-4 py-3">
                    <div className="h-4 bg-zinc-800 rounded w-full" />
                  </td>
                </tr>
              ))
            ) : sortedRecords.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-12 text-center text-zinc-600 text-sm">
                  {search ? "No QR codes match your search." : "No QR codes in this workspace yet."}
                </td>
              </tr>
            ) : (
              sortedRecords.map((rec) => (
                <tr
                  key={rec.id}
                  className={`border-b border-zinc-800/60 hover:bg-zinc-800/30 transition-colors ${selected.has(rec.id) ? "bg-violet-600/5" : ""}`}
                >
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={selected.has(rec.id)}
                      onChange={() => toggleSelect(rec.id)}
                      className="w-4 h-4 rounded border-zinc-700 bg-zinc-900 accent-violet-600"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <Link href={`/dashboard/qr-codes/${rec.id}`} className="group">
                      <div className="font-medium text-zinc-200 group-hover:text-violet-400 truncate max-w-[180px] transition-colors" title={rec.title || rec.content}>
                        {rec.title || rec.content?.substring(0, 40)}
                      </div>
                      {rec.short_code && (
                        <p className="text-xs text-zinc-600 font-mono">/r/{rec.short_code}</p>
                      )}
                    </Link>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <span className="px-2 py-0.5 bg-zinc-800 text-zinc-400 rounded text-xs font-medium">
                      {TYPE_LABELS[rec.qr_type] ?? rec.qr_type}
                    </span>
                  </td>
                  <td className="px-4 py-3 hidden lg:table-cell">
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
                    <span className={`inline-block w-2 h-2 rounded-full ${rec.is_active ? "bg-emerald-400" : "bg-zinc-600"}`} title={rec.is_active ? "Active" : "Inactive"} />
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
  );
}
