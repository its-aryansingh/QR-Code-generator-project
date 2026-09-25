import React from 'react';
import Link from 'next/link';
import { QrCode, ArrowLeft, Shield } from 'lucide-react';

export const metadata = {
  title: 'Terms of Service - QRit',
  description: 'Terms of Service and Acceptable Use Policy for the QRit QR Code platform.',
};

export default function TermsPage() {
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
          <Link
            href="/"
            className="text-xs font-semibold text-[var(--text-muted)] hover:text-[var(--text)] flex items-center gap-1.5"
          >
            <ArrowLeft className="w-3.5 h-3.5" />
            <span>Back to Home</span>
          </Link>
        </div>
      </header>

      <main className="max-w-3xl mx-auto px-4 py-12 flex-1 space-y-8">
        <div className="space-y-2">
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-[var(--accent)]/10 text-[var(--accent)]">
            <Shield className="w-3.5 h-3.5" />
            <span>Legal Agreement</span>
          </div>
          <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-[var(--text)]">
            Terms of Service
          </h1>
          <p className="text-xs text-[var(--text-muted)]">
            Last updated: September 2026 • Effective immediately
          </p>
        </div>

        <div className="space-y-6 text-xs text-[var(--text-muted)] leading-relaxed">
          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">1. Acceptance of Terms</h2>
            <p>
              By accessing or using QRit (&quot;the Service&quot;), provided by QRit Inc., you agree to be bound by these
              Terms of Service. If you disagree with any part of these terms, you may not access or use the platform.
            </p>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">2. Acceptable Use Policy</h2>
            <p>
              You agree not to misuse the Service. Specifically, you agree not to create QR codes, short links, or hosted
              pages that:
            </p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Facilitate phishing, credential harvesting, or identity theft.</li>
              <li>Distribute trojans, ransomware, viruses, or any malicious software.</li>
              <li>Circumvent security controls or conduct denial-of-service attacks.</li>
              <li>Violate applicable national or international laws and privacy regulations.</li>
              <li>Infringe upon intellectual property, trademark, or proprietary rights.</li>
            </ul>
            <p>
              We reserve the right to immediately block or suspend any QR code or account flagged for abusive behavior.
            </p>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">3. Subscription, Downgrades, and Code Preservation</h2>
            <p>
              Paid subscription plans (Pro, Business, Enterprise) provide increased resource limits, team seats, and
              integrations.
            </p>
            <p>
              <strong>The Anti-Hostage Guarantee:</strong> If you cancel or downgrade your subscription, existing dynamic
              QR codes will <em>never</em> be deleted or redirected to an advertising paywall. Codes exceeding the lower
              tier limits will transition into read-only mode (destination cannot be altered until upgraded or archived).
            </p>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">4. Disclaimers and Limitation of Liability</h2>
            <p>
              The Service is provided &quot;as is&quot; and &quot;as available&quot;. QRit disclaims all warranties, whether express or
              implied. Under no circumstances shall QRit be liable for indirect, incidental, special, or consequential
              damages resulting from the use or inability to use the Service.
            </p>
          </section>
        </div>
      </main>

      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit. All rights reserved.</p>
      </footer>
    </div>
  );
}
