'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api/client';
import {
  Plus,
  Search,
  ExternalLink,
  Copy,
  Check,
  QrCode,
  ArrowUpDown,
  MoreVertical,
} from 'lucide-react';

interface QrItem {
  id: string;
  short_code: string;
  name: string;
  content_type: string;
  destination_url?: string;
  status: 'active' | 'paused' | 'archived' | 'expired';
  scan_count?: number;
  created_at: string;
}

export default function QrListPage({
  params,
}: {
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);
  const [items, setItems] = useState<QrItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    async function loadData() {
      try {
        const res: any = await api.get(`/v1/workspaces/${workspace}/qr`);
        if (mounted && Array.isArray(res?.items)) {
          setItems(res.items);
        }
      } catch {
        // Fallback or demo empty list
        if (mounted) setItems([]);
      } finally {
        if (mounted) setLoading(false);
      }
    }
    loadData();
    return () => {
      mounted = false;
    };
  }, [workspace]);

  const handleCopy = (code: string) => {
    const fullUrl = `https://qr.example.com/${code}`;
    navigator.clipboard.writeText(fullUrl);
    setCopiedCode(code);
    setTimeout(() => setCopiedCode(null), 2000);
  };

  const filtered = items.filter(
    (i) =>
      i.name.toLowerCase().includes(search.toLowerCase()) ||
      i.short_code.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold tracking-tight text-[var(--text)]">QR Codes</h1>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">
            Manage, organize, and monitor performance of your dynamic links
          </p>
        </div>
        <Link
          href={`/w/${workspace}/qr/new`}
          className="inline-flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg font-semibold text-xs shadow-xs hover:opacity-95 transition-opacity"
        >
          <Plus className="w-4 h-4" />
          Create QR Code
        </Link>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex items-center justify-between gap-4 bg-[var(--surface)] p-3 rounded-xl border border-[var(--border)]">
        <div className="relative flex-1 max-w-md">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or code..."
            className="w-full pl-9 pr-3.5 py-1.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
          />
        </div>
        <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
          <span className="font-semibold text-[var(--text)]">{filtered.length}</span> codes
        </div>
      </div>

      {/* Table / Empty State */}
      {loading ? (
        <div className="bg-[var(--surface)] p-12 text-center rounded-2xl border border-[var(--border)] text-xs text-[var(--text-muted)]">
          Loading QR codes...
        </div>
      ) : filtered.length === 0 ? (
        <div className="bg-[var(--surface)] p-16 text-center rounded-2xl border border-[var(--border)] space-y-4">
          <div className="w-12 h-12 rounded-2xl bg-[var(--accent)]/10 text-[var(--accent)] mx-auto flex items-center justify-center">
            <QrCode className="w-6 h-6" />
          </div>
          <div className="max-w-xs mx-auto">
            <h3 className="text-sm font-bold text-[var(--text)]">No QR codes found</h3>
            <p className="text-xs text-[var(--text-muted)] mt-1">
              {search ? 'Try adjusting your search query.' : 'Create your first dynamic QR code to start tracking scans and engagement.'}
            </p>
          </div>
          {!search && (
            <Link
              href={`/w/${workspace}/qr/new`}
              className="inline-flex items-center gap-1.5 px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg font-semibold text-xs hover:opacity-95 transition-opacity"
            >
              <Plus className="w-4 h-4" />
              Create QR Code
            </Link>
          )}
        </div>
      ) : (
        <div className="bg-[var(--surface)] rounded-2xl border border-[var(--border)] overflow-hidden shadow-xs">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-[var(--bg-subtle)] border-b border-[var(--border)] text-[var(--text-muted)] font-semibold uppercase tracking-wider">
                <tr>
                  <th className="py-3 px-4">Name & Code</th>
                  <th className="py-3 px-4">Destination</th>
                  <th className="py-3 px-4">Scans</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
                {filtered.map((item) => (
                  <tr key={item.id} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-3">
                        <div className="w-9 h-9 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border)] flex items-center justify-center text-[var(--accent)]">
                          <QrCode className="w-5 h-5" />
                        </div>
                        <div>
                          <Link
                            href={`/w/${workspace}/qr/${item.id}`}
                            className="font-bold text-[var(--text)] hover:text-[var(--accent)] transition-colors"
                          >
                            {item.name}
                          </Link>
                          <div className="flex items-center gap-1.5 text-[11px] text-[var(--text-muted)] mt-0.5">
                            <span className="font-mono">qr.example.com/{item.short_code}</span>
                            <button
                              onClick={() => handleCopy(item.short_code)}
                              className="hover:text-[var(--text)]"
                              title="Copy Short Link"
                            >
                              {copiedCode === item.short_code ? (
                                <Check className="w-3 h-3 text-[var(--success)]" />
                              ) : (
                                <Copy className="w-3 h-3" />
                              )}
                            </button>
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="py-3 px-4">
                      <div className="max-w-xs truncate text-[var(--text-muted)] font-mono text-[11px]">
                        {item.destination_url || '—'}
                      </div>
                    </td>
                    <td className="py-3 px-4">
                      <span className="font-semibold text-[var(--text)] tabular-nums">
                        {item.scan_count || 0}
                      </span>
                    </td>
                    <td className="py-3 px-4">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider ${
                          item.status === 'active'
                            ? 'bg-green-500/10 text-green-600'
                            : item.status === 'paused'
                            ? 'bg-yellow-500/10 text-yellow-600'
                            : 'bg-red-500/10 text-red-600'
                        }`}
                      >
                        {item.status}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-right">
                      <Link
                        href={`/w/${workspace}/qr/${item.id}`}
                        className="p-1 text-[var(--text-muted)] hover:text-[var(--text)] inline-flex rounded"
                      >
                        <ExternalLink className="w-4 h-4" />
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
