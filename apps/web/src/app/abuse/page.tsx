'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { ShieldAlert, CheckCircle2, ArrowRight } from 'lucide-react';

export default function AbuseReportPage() {
  const [shortCodeOrUrl, setShortCodeOrUrl] = useState('');
  const [reporterEmail, setReporterEmail] = useState('');
  const [reason, setReason] = useState('phishing');
  const [details, setDetails] = useState('');
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitted(true);
  };

  return (
    <div className="min-h-screen bg-[var(--bg-subtle)] flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-[var(--surface)] p-8 rounded-3xl border border-[var(--border)] shadow-xl">
        <div className="text-center mb-6">
          <div className="w-12 h-12 rounded-2xl bg-[var(--danger)]/10 text-[var(--danger)] flex items-center justify-center mx-auto mb-3">
            <ShieldAlert className="w-6 h-6" />
          </div>
          <h1 className="text-xl font-extrabold text-[var(--text)]">Report Malicious QR Code</h1>
          <p className="text-xs text-[var(--text-muted)] mt-1">
            Help protect users from phishing, malware, scams, or abuse.
          </p>
        </div>

        {submitted ? (
          <div className="text-center py-8 space-y-3">
            <CheckCircle2 className="w-10 h-10 text-[var(--success)] mx-auto" />
            <h3 className="text-sm font-bold text-[var(--text)]">Report Received</h3>
            <p className="text-xs text-[var(--text-muted)]">
              Our trust & safety team has received your report and the link is undergoing automated reputation scanning.
            </p>
            <Link
              href="/"
              className="inline-flex items-center gap-1 text-xs font-semibold text-[var(--accent)] hover:underline pt-4"
            >
              Return to QRit <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Short URL or Code
              </label>
              <input
                type="text"
                required
                value={shortCodeOrUrl}
                onChange={(e) => setShortCodeOrUrl(e.target.value)}
                placeholder="qr.example.com/xyz123"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-mono text-[var(--text)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Reason for report
              </label>
              <select
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="w-full px-3 py-2 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)]"
              >
                <option value="phishing">Phishing / Credential theft</option>
                <option value="malware">Malware / Suspicious download</option>
                <option value="scam">Fraud / Financial scam</option>
                <option value="impersonation">Brand impersonation</option>
                <option value="other">Other abuse</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Your Email (Optional, for updates)
              </label>
              <input
                type="email"
                value={reporterEmail}
                onChange={(e) => setReporterEmail(e.target.value)}
                placeholder="reporter@example.com"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                Additional Details
              </label>
              <textarea
                value={details}
                onChange={(e) => setDetails(e.target.value)}
                rows={3}
                placeholder="Where was this QR code displayed? What occurred upon scanning?"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)]"
              />
            </div>

            <button
              type="submit"
              className="w-full py-2.5 px-4 bg-[var(--danger)] text-white rounded-lg font-semibold text-xs shadow-xs hover:opacity-95 transition-opacity"
            >
              Submit Report
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
