'use client';

import React, { useState, useMemo } from 'react';
import Link from 'next/link';
import { renderSvg, DEFAULT_DESIGN } from '@qrit/qr-render';
import { encodeWiFi } from '@/features/qr-editor/encoders';
import { QrCode, Wifi, Download, Check, Sparkles, ArrowRight, ShieldCheck } from 'lucide-react';

export default function WiFiQRPage() {
  const [ssid, setSsid] = useState('Office-Guest-WiFi');
  const [password, setPassword] = useState('ConnectSecure2026');
  const [encryption, setEncryption] = useState<'WPA' | 'WEP' | 'nopass'>('WPA');
  const [hidden, setHidden] = useState(false);

  const payload = useMemo(() => {
    return encodeWiFi(ssid, password, encryption, hidden);
  }, [ssid, password, encryption, hidden]);

  const renderResult = useMemo(() => {
    try {
      return renderSvg({ payload: payload || 'WIFI:S:Guest;T:WPA;P:pass;;', design: DEFAULT_DESIGN });
    } catch {
      return renderSvg({ payload: 'WIFI:S:Guest;T:WPA;P:pass;;', design: DEFAULT_DESIGN });
    }
  }, [payload]);

  const handleDownloadSvg = () => {
    const blob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `wifi-qr-${ssid || 'network'}.svg`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="min-h-screen flex flex-col bg-[var(--bg)]">
      <header className="border-b border-[var(--border)] bg-[var(--surface)]/80 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-[var(--accent)] flex items-center justify-center text-white font-bold shadow-sm">
              <QrCode className="w-5 h-5" />
            </div>
            <span className="font-extrabold text-xl tracking-tight text-[var(--text)]">QRit</span>
          </Link>
          <div className="flex items-center gap-3">
            <Link
              href="/pricing"
              className="text-xs font-semibold text-[var(--text-muted)] hover:text-[var(--text)]"
            >
              Pricing
            </Link>
            <Link
              href="/register"
              className="text-xs font-semibold bg-[var(--accent)] text-[var(--accent-fg)] px-3 py-1.5 rounded-lg hover:opacity-95"
            >
              Sign Up
            </Link>
          </div>
        </div>
      </header>

      <section className="pt-12 pb-8 px-4 text-center max-w-4xl mx-auto space-y-3">
        <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-[var(--accent)]/10 text-[var(--accent)]">
          <Wifi className="w-3.5 h-3.5" />
          <span>Instant Wi-Fi Connection</span>
        </div>
        <h1 className="text-3xl md:text-5xl font-extrabold tracking-tight text-[var(--text)]">
          Free Wi-Fi QR Code Generator
        </h1>
        <p className="text-sm md:text-base text-[var(--text-muted)] max-w-xl mx-auto">
          Let guests, customers, and team members connect to your wireless network instantly with their smartphone camera.
        </p>
      </section>

      <main className="max-w-5xl mx-auto px-4 py-6 w-full flex-1">
        <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">
          {/* Controls */}
          <div className="md:col-span-7 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 shadow-xs">
            <h2 className="text-sm font-bold text-[var(--text)]">Wireless Network Details</h2>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Network Name (SSID)
              </label>
              <input
                type="text"
                value={ssid}
                onChange={(e) => setSsid(e.target.value)}
                placeholder="e.g. CoffeeShop_Guest"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Password
              </label>
              <input
                type="text"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Wi-Fi Password"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs font-mono text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Security Encryption
              </label>
              <div className="grid grid-cols-3 gap-2">
                {(['WPA', 'WEP', 'nopass'] as const).map((enc) => (
                  <button
                    key={enc}
                    type="button"
                    onClick={() => setEncryption(enc)}
                    className={`py-2 px-3 rounded-xl border text-xs font-semibold transition-all ${
                      encryption === enc
                        ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]'
                        : 'border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text-muted)]'
                    }`}
                  >
                    {enc === 'nopass' ? 'None (Open)' : enc}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2 pt-1">
              <input
                type="checkbox"
                id="hidden-net"
                checked={hidden}
                onChange={(e) => setHidden(e.target.checked)}
                className="rounded text-[var(--accent)]"
              />
              <label htmlFor="hidden-net" className="text-xs text-[var(--text-muted)] select-none">
                Hidden wireless network (SSID not broadcasting)
              </label>
            </div>
          </div>

          {/* Preview & Download */}
          <div className="md:col-span-5 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 text-center shadow-xs">
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">
              Scannable Preview
            </h3>

            <div className="p-4 bg-white rounded-2xl border border-[var(--border)] inline-block mx-auto shadow-inner">
              <div
                className="w-48 h-48"
                dangerouslySetInnerHTML={{ __html: renderResult.svg }}
              />
            </div>

            <p className="text-[11px] text-[var(--text-muted)]">
              Scan with iOS Camera or Android to automatically join network.
            </p>

            <button
              onClick={handleDownloadSvg}
              className="w-full py-2.5 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold flex items-center justify-center gap-2 hover:opacity-95 shadow-xs"
            >
              <Download className="w-4 h-4" />
              <span>Download Vector SVG</span>
            </button>
          </div>
        </div>
      </main>

      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit. Free Wireless QR Generator.</p>
      </footer>
    </div>
  );
}
