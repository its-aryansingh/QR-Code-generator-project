'use client';

import React, { useState, useMemo } from 'react';
import Link from 'next/link';
import { renderSvg, DEFAULT_DESIGN } from '@qrit/qr-render';
import { QrCode, IndianRupee, Download, ShieldCheck, CheckCircle2 } from 'lucide-react';

export default function UPIQRPage() {
  const [vpa, setVpa] = useState('merchant@okhdfcbank');
  const [name, setName] = useState('My Business Store');
  const [amount, setAmount] = useState('499');
  const [note, setNote] = useState('Payment for Order #104');

  // NPCI spec: upi://pay?pa=<vpa>&pn=<name>&am=<amount>&cu=INR&tn=<note>
  const payload = useMemo(() => {
    const params = new URLSearchParams();
    if (vpa) params.set('pa', vpa.trim());
    if (name) params.set('pn', name.trim());
    if (amount && Number(amount) > 0) params.set('am', amount.trim());
    params.set('cu', 'INR');
    if (note) params.set('tn', note.trim());
    return `upi://pay?${params.toString()}`;
  }, [vpa, name, amount, note]);

  const renderResult = useMemo(() => {
    try {
      return renderSvg({ payload: payload || 'upi://pay?pa=test@upi&pn=Test&cu=INR', design: DEFAULT_DESIGN });
    } catch {
      return renderSvg({ payload: 'upi://pay?pa=test@upi&pn=Test&cu=INR', design: DEFAULT_DESIGN });
    }
  }, [payload]);

  const handleDownloadSvg = () => {
    const blob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `upi-qr-${vpa.split('@')[0] || 'pay'}.svg`;
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
        <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-600">
          <IndianRupee className="w-3.5 h-3.5" />
          <span>NPCI Specification Compliant</span>
        </div>
        <h1 className="text-3xl md:text-5xl font-extrabold tracking-tight text-[var(--text)]">
          Free UPI Payment QR Code Generator
        </h1>
        <p className="text-sm md:text-base text-[var(--text-muted)] max-w-xl mx-auto">
          Generate print-ready QR codes for PhonePe, Google Pay, Paytm, and BHIM UPI with zero transaction cut.
        </p>
      </section>

      <main className="max-w-5xl mx-auto px-4 py-6 w-full flex-1">
        <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">
          {/* Controls */}
          <div className="md:col-span-7 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 shadow-xs">
            <h2 className="text-sm font-bold text-[var(--text)]">Payment Details</h2>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                UPI ID / VPA
              </label>
              <input
                type="text"
                required
                value={vpa}
                onChange={(e) => setVpa(e.target.value)}
                placeholder="yourname@bank"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs font-mono text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Payee / Merchant Name
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Business or Store Name"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                  Preset Amount (₹ Optional)
                </label>
                <input
                  type="number"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  placeholder="Leave empty for any amount"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs font-mono text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                  Note / Reference
                </label>
                <input
                  type="text"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="e.g. Table 4 Bill"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
            </div>

            <div className="p-3 bg-[var(--bg-subtle)] rounded-xl text-[11px] text-[var(--text-muted)] flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-emerald-500 shrink-0" />
              <span>Payments go directly to your bank account. QRit takes zero commission.</span>
            </div>
          </div>

          {/* Preview & Download */}
          <div className="md:col-span-5 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 text-center shadow-xs">
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">
              Scannable Standee Preview
            </h3>

            <div className="p-4 bg-white rounded-2xl border border-[var(--border)] inline-block mx-auto shadow-inner">
              <div
                className="w-48 h-48"
                dangerouslySetInnerHTML={{ __html: renderResult.svg }}
              />
            </div>

            <p className="text-[11px] text-[var(--text-muted)] font-mono">
              {payload.substring(0, 45)}...
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
        <p>© {new Date().getFullYear()} QRit. NPCI UPI Standee Generator.</p>
      </footer>
    </div>
  );
}
