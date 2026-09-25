'use client';

import React, { useState, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { renderSvg, DesignV1, DEFAULT_DESIGN, calculateScannability, ModuleShape } from '@qrit/qr-render';
import { api, ApiError } from '@/lib/api/client';
import { ArrowLeft, CheckCircle2, AlertTriangle, Loader2 } from 'lucide-react';

export default function NewQrPage({
  params,
}: {
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);
  const router = useRouter();

  const [name, setName] = useState('');
  const [destinationUrl, setDestinationUrl] = useState('https://');
  const [customCode, setCustomCode] = useState('');
  const [design, setDesign] = useState<DesignV1>(DEFAULT_DESIGN);

  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Live SVG render preview
  const renderResult = useMemo(() => {
    try {
      return renderSvg({
        payload: destinationUrl || 'https://example.com',
        design,
      });
    } catch {
      return renderSvg({ payload: 'https://example.com', design: DEFAULT_DESIGN });
    }
  }, [destinationUrl, design]);

  const scannability = useMemo(() => {
    return calculateScannability(design, renderResult.version);
  }, [design, renderResult.version]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setErrorMsg(null);

    try {
      const res: any = await api.post(`/v1/workspaces/${workspace}/qr`, {
        name,
        content_type: 'url',
        destination_url: destinationUrl,
        short_code: customCode || undefined,
        design,
      });

      const qrId = res?.id || res?.qr?.id;
      if (qrId) {
        router.push(`/w/${workspace}/qr/${qrId}`);
      } else {
        router.push(`/w/${workspace}/qr`);
      }
    } catch (err: any) {
      if (err instanceof ApiError) {
        setErrorMsg(err.problem.detail || err.problem.title);
      } else {
        setErrorMsg(err?.message || 'Failed to create QR code');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div className="flex items-center gap-3">
        <Link
          href={`/w/${workspace}/qr`}
          className="p-1.5 rounded-lg border border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </Link>
        <div>
          <h1 className="text-xl font-extrabold tracking-tight text-[var(--text)]">Create Dynamic QR Code</h1>
          <p className="text-xs text-[var(--text-muted)]">Configure destination and visual appearance</p>
        </div>
      </div>

      {errorMsg && (
        <div className="p-3 rounded-xl bg-[var(--danger)]/10 border border-[var(--danger)]/20 text-xs text-[var(--danger)]">
          {errorMsg}
        </div>
      )}

      <form onSubmit={handleSubmit} className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Left Form controls */}
        <div className="lg:col-span-7 space-y-6">
          <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Link Details</h2>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Campaign Name</label>
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Summer Brochure Launch"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Destination URL</label>
              <input
                type="url"
                required
                value={destinationUrl}
                onChange={(e) => setDestinationUrl(e.target.value)}
                placeholder="https://mycompany.com/promo"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)] font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Custom Shortcode (Optional)</label>
              <div className="flex items-center">
                <span className="px-3 py-2.5 bg-[var(--bg-subtle)] border border-r-0 border-[var(--border)] rounded-l-lg text-xs text-[var(--text-muted)] font-mono">
                  qr.example.com/
                </span>
                <input
                  type="text"
                  value={customCode}
                  onChange={(e) => setCustomCode(e.target.value)}
                  placeholder="promo2026"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-r-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)] font-mono"
                />
              </div>
            </div>
          </div>

          {/* Design Controls */}
          <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-6">
            <h2 className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Appearance</h2>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">Module Pattern</label>
              <div className="grid grid-cols-3 gap-2">
                {(['square', 'dots', 'rounded', 'extra-rounded', 'classy', 'classy-rounded'] as ModuleShape[]).map((shape) => (
                  <button
                    key={shape}
                    type="button"
                    onClick={() => setDesign({ ...design, modules: { ...design.modules, shape } })}
                    className={`px-3 py-2 text-xs font-medium rounded-lg border capitalize transition-all ${
                      design.modules.shape === shape
                        ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)] font-semibold'
                        : 'border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text)]'
                    }`}
                  >
                    {shape.replace('-', ' ')}
                  </button>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Pattern Color</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={design.modules.color}
                    onChange={(e) =>
                      setDesign({
                        ...design,
                        modules: { ...design.modules, color: e.target.value },
                        finder: {
                          ...design.finder,
                          outer_color: e.target.value,
                          inner_color: e.target.value,
                        },
                      })
                    }
                    className="w-8 h-8 rounded border border-[var(--border)] cursor-pointer"
                  />
                  <span className="text-xs font-mono text-[var(--text-muted)] uppercase">{design.modules.color}</span>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Background</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={design.background.color}
                    onChange={(e) =>
                      setDesign({
                        ...design,
                        background: { ...design.background, color: e.target.value },
                      })
                    }
                    className="w-8 h-8 rounded border border-[var(--border)] cursor-pointer"
                  />
                  <span className="text-xs font-mono text-[var(--text-muted)] uppercase">{design.background.color}</span>
                </div>
              </div>
            </div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-3 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-bold text-sm flex items-center justify-center gap-2 hover:opacity-95 transition-opacity disabled:opacity-60"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Create QR Code'}
          </button>
        </div>

        {/* Right Sticky Preview */}
        <div className="lg:col-span-5 sticky top-20 space-y-4">
          <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] flex flex-col items-center">
            <div
              className="w-64 h-64 flex items-center justify-center p-3 rounded-xl bg-white border border-[var(--border)] shadow-xs"
              dangerouslySetInnerHTML={{ __html: renderResult.svg }}
            />

            <div className="w-full mt-6 p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)]">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs font-semibold text-[var(--text-muted)]">Scannability</span>
                <div className="flex items-center gap-1.5">
                  {scannability.score >= 70 ? (
                    <CheckCircle2 className="w-4 h-4 text-[var(--success)]" />
                  ) : (
                    <AlertTriangle className="w-4 h-4 text-[var(--warning)]" />
                  )}
                  <span className="text-xs font-bold">{scannability.label} ({scannability.score}/100)</span>
                </div>
              </div>
              <div className="w-full h-2 rounded-full bg-[var(--border)] overflow-hidden">
                <div
                  className={`h-full ${scannability.score >= 70 ? 'bg-[var(--success)]' : 'bg-[var(--warning)]'}`}
                  style={{ width: `${scannability.score}%` }}
                />
              </div>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}
