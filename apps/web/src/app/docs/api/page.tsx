'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import {
  QrCode,
  ArrowLeft,
  Copy,
  Check,
  Code2,
  Terminal,
  Shield,
  Key,
  ExternalLink,
} from 'lucide-react';

export default function ApiDocsPage() {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const handleCopy = (code: string, idx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedIndex(idx);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const curlCreateQR = `curl -X POST https://api.qrit.io/v1/workspaces/ws-main/qr-codes \\
  -H "Authorization: Bearer qrit_live_sec_abcdef012345" \\
  -H "Content-Type: application/json" \\
  -d '{
    "mode": "dynamic",
    "name": "Summer Campaign Standee",
    "content_type": "url",
    "destination_url": "https://brand.com/summer-sale",
    "design": {
      "matrix": { "shape": "rounded" },
      "dots": { "color": "#0F172A" }
    }
  }'`;

  const curlGetAnalytics = `curl -X GET "https://api.qrit.io/v1/workspaces/ws-main/analytics?from=2026-09-01&to=2026-09-26" \\
  -H "Authorization: Bearer qrit_live_sec_abcdef012345"`;

  return (
    <div className="min-h-screen flex flex-col bg-[var(--bg)]">
      <header className="border-b border-[var(--border)] bg-[var(--surface)]/80 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-[var(--accent)] flex items-center justify-center text-white font-bold shadow-sm">
              <QrCode className="w-5 h-5" />
            </div>
            <span className="font-extrabold text-xl tracking-tight text-[var(--text)]">QRit API Docs</span>
          </Link>
          <div className="flex items-center gap-4">
            <Link
              href="/pricing"
              className="text-xs font-semibold text-[var(--text-muted)] hover:text-[var(--text)]"
            >
              Pricing
            </Link>
            <Link
              href="/login"
              className="text-xs font-semibold bg-[var(--accent)] text-[var(--accent-fg)] px-3 py-1.5 rounded-lg hover:opacity-95"
            >
              Get API Keys
            </Link>
          </div>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-4 py-12 flex-1 space-y-10">
        <div className="space-y-3">
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-[var(--accent)]/10 text-[var(--accent)]">
            <Terminal className="w-3.5 h-3.5" />
            <span>OpenAPI 3.1 & REST API</span>
          </div>
          <h1 className="text-3xl md:text-5xl font-extrabold tracking-tight text-[var(--text)]">
            QRit Developer Platform
          </h1>
          <p className="text-sm md:text-base text-[var(--text-muted)] max-w-2xl">
            Integrate dynamic QR generation, programmatic redirect routing, and real-time scan analytics into your applications.
          </p>
        </div>

        {/* Authentication Card */}
        <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] space-y-3 shadow-xs">
          <div className="flex items-center gap-2">
            <Key className="w-4 h-4 text-[var(--accent)]" />
            <h2 className="text-sm font-bold text-[var(--text)]">Authentication</h2>
          </div>
          <p className="text-xs text-[var(--text-muted)] leading-relaxed">
            All API requests must include your secret API key in the <code className="px-1.5 py-0.5 rounded bg-[var(--bg-subtle)] font-mono text-[var(--text)]">Authorization</code> header:
          </p>
          <div className="p-3 bg-[var(--bg-subtle)] rounded-xl font-mono text-xs text-[var(--text)] border border-[var(--border)]">
            Authorization: Bearer qrit_live_sec_your_api_key_here
          </div>
        </div>

        {/* Code Example 1: Create QR */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold text-[var(--text)]">Create Dynamic QR Code</h3>
              <p className="text-xs text-[var(--text-muted)] font-mono">POST /v1/workspaces/:ws/qr-codes</p>
            </div>
            <button
              onClick={() => handleCopy(curlCreateQR, 1)}
              className="px-3 py-1.5 rounded-lg border border-[var(--border)] text-xs font-semibold flex items-center gap-1.5 hover:bg-[var(--bg-subtle)] text-[var(--text)]"
            >
              {copiedIndex === 1 ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copiedIndex === 1 ? 'Copied' : 'Copy cURL'}</span>
            </button>
          </div>

          <div className="p-4 rounded-2xl bg-[var(--bg-subtle)] border border-[var(--border)] overflow-x-auto">
            <pre className="font-mono text-xs text-[var(--text)] leading-relaxed">
              {curlCreateQR}
            </pre>
          </div>
        </div>

        {/* Code Example 2: Query Analytics */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold text-[var(--text)]">Query Scan Analytics</h3>
              <p className="text-xs text-[var(--text-muted)] font-mono">GET /v1/workspaces/:ws/analytics</p>
            </div>
            <button
              onClick={() => handleCopy(curlGetAnalytics, 2)}
              className="px-3 py-1.5 rounded-lg border border-[var(--border)] text-xs font-semibold flex items-center gap-1.5 hover:bg-[var(--bg-subtle)] text-[var(--text)]"
            >
              {copiedIndex === 2 ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copiedIndex === 2 ? 'Copied' : 'Copy cURL'}</span>
            </button>
          </div>

          <div className="p-4 rounded-2xl bg-[var(--bg-subtle)] border border-[var(--border)] overflow-x-auto">
            <pre className="font-mono text-xs text-[var(--text)] leading-relaxed">
              {curlGetAnalytics}
            </pre>
          </div>
        </div>

        {/* Rate Limits */}
        <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] space-y-3 shadow-xs">
          <h2 className="text-sm font-bold text-[var(--text)]">Concurrency & Rate Limits</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
            <div className="p-3 bg-[var(--bg-subtle)] rounded-xl space-y-1">
              <div className="font-bold text-[var(--text)]">Free Plan</div>
              <div className="text-[var(--text-muted)] font-mono">100 requests / minute</div>
            </div>
            <div className="p-3 bg-[var(--bg-subtle)] rounded-xl space-y-1">
              <div className="font-bold text-[var(--accent)]">Pro Plan</div>
              <div className="text-[var(--text-muted)] font-mono">500 requests / minute</div>
            </div>
            <div className="p-3 bg-[var(--bg-subtle)] rounded-xl space-y-1">
              <div className="font-bold text-emerald-600">Business / Enterprise</div>
              <div className="text-[var(--text-muted)] font-mono">5,000+ requests / minute</div>
            </div>
          </div>
        </div>
      </main>

      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit Developer Platform.</p>
      </footer>
    </div>
  );
}
