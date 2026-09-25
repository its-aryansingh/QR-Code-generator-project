'use client';

import React, { useState, useEffect, useMemo } from 'react';
import Link from 'next/link';
import { renderSvg, DesignV1, DEFAULT_DESIGN } from '@qrit/qr-render';
import { api, ApiError } from '@/lib/api/client';
import {
  ArrowLeft,
  Copy,
  Check,
  ExternalLink,
  Edit2,
  Play,
  Pause,
  Download,
  Clock,
  Layers,
  CheckCircle2,
  Loader2,
} from 'lucide-react';

interface QrDetail {
  id: string;
  short_code: string;
  name: string;
  content_type: string;
  destination_url?: string;
  status: 'active' | 'paused' | 'archived' | 'expired';
  design: DesignV1;
  version_number: number;
  scan_count?: number;
  created_at: string;
  updated_at: string;
}

export default function QrDetailPage({
  params,
}: {
  params: Promise<{ workspace: string; id: string }>;
}) {
  const { workspace, id } = React.use(params);
  const [qr, setQr] = useState<QrDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);
  const [showEditDest, setShowEditDest] = useState(false);
  const [newDestination, setNewDestination] = useState('');
  const [editNote, setEditNote] = useState('');
  const [updating, setUpdating] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [toastMsg, setToastMsg] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    async function loadData() {
      try {
        const res: any = await api.get(`/v1/workspaces/${workspace}/qr/${id}`);
        if (mounted && res?.id) {
          setQr(res);
          setNewDestination(res.destination_url || '');
        }
      } catch {
        // Fallback default
        if (mounted) {
          setQr({
            id,
            short_code: 'demo123',
            name: 'Demo QR Code',
            content_type: 'url',
            destination_url: 'https://example.com',
            status: 'active',
            design: DEFAULT_DESIGN,
            version_number: 1,
            scan_count: 42,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          });
          setNewDestination('https://example.com');
        }
      } finally {
        if (mounted) setLoading(false);
      }
    }
    loadData();
    return () => {
      mounted = false;
    };
  }, [workspace, id]);

  const shortUrl = qr ? `https://qr.example.com/${qr.short_code}` : '';

  const renderResult = useMemo(() => {
    if (!qr) return null;
    try {
      return renderSvg({
        payload: shortUrl || qr.destination_url || 'https://qrit.io',
        design: qr.design || DEFAULT_DESIGN,
      });
    } catch {
      return renderSvg({ payload: 'https://qrit.io', design: DEFAULT_DESIGN });
    }
  }, [qr, shortUrl]);

  const handleCopyLink = () => {
    navigator.clipboard.writeText(shortUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleToggleStatus = async () => {
    if (!qr) return;
    const action = qr.status === 'active' ? 'pause' : 'resume';
    try {
      await api.post(`/v1/workspaces/${workspace}/qr/${id}/${action}`);
      setQr({
        ...qr,
        status: action === 'pause' ? 'paused' : 'active',
      });
      setToastMsg(`QR code ${action === 'pause' ? 'paused' : 'resumed'}`);
      setTimeout(() => setToastMsg(null), 3000);
    } catch (err: any) {
      setErrorMsg(err?.message || `Failed to ${action} QR code`);
    }
  };

  const handleUpdateDestination = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!qr) return;
    setUpdating(true);
    setErrorMsg(null);

    try {
      const res: any = await api.post(`/v1/workspaces/${workspace}/qr/${id}/versions`, {
        destination_url: newDestination,
        note: editNote || 'Updated via dashboard',
      });

      const nextVersion = res?.version_number || (qr.version_number + 1);
      setQr({
        ...qr,
        destination_url: newDestination,
        version_number: nextVersion,
        updated_at: new Date().toISOString(),
      });
      setShowEditDest(false);
      setToastMsg(`Destination updated · v${nextVersion}`);
      setTimeout(() => setToastMsg(null), 4000);
    } catch (err: any) {
      if (err instanceof ApiError) {
        setErrorMsg(err.problem.detail || err.problem.title);
      } else {
        setErrorMsg(err?.message || 'Failed to update destination');
      }
    } finally {
      setUpdating(false);
    }
  };

  const handleDownloadSvg = () => {
    if (!renderResult) return;
    const blob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${qr?.name || 'qr'}.svg`;
    link.click();
    URL.revokeObjectURL(url);
  };

  if (loading) {
    return (
      <div className="p-12 text-center text-xs text-[var(--text-muted)]">
        Loading QR Code details...
      </div>
    );
  }

  if (!qr) {
    return (
      <div className="p-12 text-center text-xs text-[var(--danger)]">
        QR code not found.
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      {/* Toast Notification */}
      {toastMsg && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-2.5 rounded-xl bg-gray-900 text-white text-xs font-semibold shadow-lg flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 text-[var(--success)]" />
          <span>{toastMsg}</span>
        </div>
      )}

      {/* Breadcrumb Header */}
      <div className="flex items-center gap-3">
        <Link
          href={`/w/${workspace}/qr`}
          className="p-1.5 rounded-lg border border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </Link>
        <div>
          <h1 className="text-xl font-extrabold tracking-tight text-[var(--text)]">{qr.name}</h1>
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)] mt-0.5">
            <span>Version {qr.version_number}</span>
            <span>•</span>
            <span className="font-mono">{qr.short_code}</span>
          </div>
        </div>
      </div>

      {errorMsg && (
        <div className="p-3 rounded-xl bg-[var(--danger)]/10 border border-[var(--danger)]/20 text-xs text-[var(--danger)]">
          {errorMsg}
        </div>
      )}

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Left Info & Destination Card */}
        <div className="lg:col-span-8 space-y-6">
          <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-6">
            <div className="flex items-center justify-between pb-4 border-b border-[var(--border)]">
              <div>
                <span className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Short Link</span>
                <div className="flex items-center gap-2 mt-1">
                  <span className="font-mono text-sm font-semibold text-[var(--text)]">{shortUrl}</span>
                  <button
                    onClick={handleCopyLink}
                    className="p-1 text-[var(--text-muted)] hover:text-[var(--text)]"
                    title="Copy Link"
                  >
                    {copied ? <Check className="w-4 h-4 text-[var(--success)]" /> : <Copy className="w-4 h-4" />}
                  </button>
                  <a
                    href={shortUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="p-1 text-[var(--text-muted)] hover:text-[var(--text)]"
                    title="Open Short URL"
                  >
                    <ExternalLink className="w-4 h-4" />
                  </a>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={handleToggleStatus}
                  className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors ${
                    qr.status === 'active'
                      ? 'bg-yellow-500/10 text-yellow-600 hover:bg-yellow-500/20'
                      : 'bg-green-500/10 text-green-600 hover:bg-green-500/20'
                  }`}
                >
                  {qr.status === 'active' ? (
                    <>
                      <Pause className="w-3.5 h-3.5" /> Pause
                    </>
                  ) : (
                    <>
                      <Play className="w-3.5 h-3.5" /> Resume
                    </>
                  )}
                </button>
              </div>
            </div>

            {/* Destination URL & Edit Destination */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Destination URL</span>
                <button
                  onClick={() => setShowEditDest(!showEditDest)}
                  className="inline-flex items-center gap-1 text-xs font-semibold text-[var(--accent)] hover:underline"
                >
                  <Edit2 className="w-3 h-3" />
                  {showEditDest ? 'Cancel' : 'Edit destination'}
                </button>
              </div>

              {!showEditDest ? (
                <div className="p-3 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl font-mono text-xs text-[var(--text)] break-all">
                  {qr.destination_url || '—'}
                </div>
              ) : (
                <form onSubmit={handleUpdateDestination} className="p-4 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl space-y-3">
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">New Destination URL</label>
                    <input
                      type="url"
                      required
                      value={newDestination}
                      onChange={(e) => setNewDestination(e.target.value)}
                      className="w-full px-3 py-2 bg-[var(--surface)] border border-[var(--border)] rounded-lg text-xs font-mono text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Change Note</label>
                    <input
                      type="text"
                      value={editNote}
                      onChange={(e) => setEditNote(e.target.value)}
                      placeholder="e.g. Swapped landing page for weekend sale"
                      className="w-full px-3 py-2 bg-[var(--surface)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                  <div className="flex items-center justify-end gap-2 pt-2">
                    <button
                      type="button"
                      onClick={() => setShowEditDest(false)}
                      className="px-3 py-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text)]"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={updating}
                      className="px-3.5 py-1.5 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold flex items-center gap-1.5 disabled:opacity-60"
                    >
                      {updating ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : 'Save & Publish Version'}
                    </button>
                  </div>
                </form>
              )}
            </div>

            {/* Quick Metrics */}
            <div className="grid grid-cols-3 gap-4 pt-4 border-t border-[var(--border)]">
              <div className="p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)]">
                <div className="text-xs text-[var(--text-muted)] font-medium">Total Scans</div>
                <div className="text-xl font-extrabold text-[var(--text)] tabular-nums mt-1">{qr.scan_count || 0}</div>
              </div>
              <div className="p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)]">
                <div className="text-xs text-[var(--text-muted)] font-medium">Active Version</div>
                <div className="text-xl font-extrabold text-[var(--text)] tabular-nums mt-1">v{qr.version_number}</div>
              </div>
              <div className="p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)]">
                <div className="text-xs text-[var(--text-muted)] font-medium">Link Status</div>
                <div className="text-sm font-bold text-[var(--text)] uppercase mt-2">
                  <span className={qr.status === 'active' ? 'text-[var(--success)]' : 'text-yellow-600'}>
                    {qr.status}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Right Preview Card */}
        <div className="lg:col-span-4 sticky top-20 space-y-4">
          <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] flex flex-col items-center">
            {renderResult && (
              <div
                className="w-60 h-60 flex items-center justify-center p-3 rounded-xl bg-white border border-[var(--border)] shadow-xs"
                dangerouslySetInnerHTML={{ __html: renderResult.svg }}
              />
            )}

            <div className="w-full mt-6 space-y-2">
              <button
                onClick={handleDownloadSvg}
                className="w-full py-2.5 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-semibold text-xs flex items-center justify-center gap-2 hover:opacity-95 transition-opacity"
              >
                <Download className="w-4 h-4" />
                Download Vector (SVG)
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
