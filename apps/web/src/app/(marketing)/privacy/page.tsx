import React from 'react';
import Link from 'next/link';
import { QrCode, ArrowLeft, Lock } from 'lucide-react';

export const metadata = {
  title: 'Privacy Policy - QRit',
  description: 'Privacy Policy and data protection commitments for QRit.',
};

export default function PrivacyPage() {
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
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-600">
            <Lock className="w-3.5 h-3.5" />
            <span>Privacy First</span>
          </div>
          <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-[var(--text)]">
            Privacy Policy
          </h1>
          <p className="text-xs text-[var(--text-muted)]">
            Last updated: September 2026 • GDPR & DPDP Compliant
          </p>
        </div>

        <div className="space-y-6 text-xs text-[var(--text-muted)] leading-relaxed">
          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">1. Our Core Privacy Philosophy</h2>
            <p>
              At QRit, we believe analytics should measure interest without compromising individual user dignity.
              We do not track individual users across the web, we do not sell personal data to advertisers, and we never
              store raw IP addresses in scan analytics databases.
            </p>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">2. Information Collected During QR Scans</h2>
            <p>
              When an end-user scans a dynamic QR code routed through our service, our edge servers process the request to
              instantly redirect the visitor. We collect:
            </p>
            <ul className="list-disc pl-5 space-y-1">
              <li><strong>Anonymized IP:</strong> Raw IP addresses are used solely for geo-lookup (country/city level) and immediately discarded. To count daily unique visitors, a cryptographic one-way hash of the IP with a daily rotating salt is used.</li>
              <li><strong>User-Agent:</strong> Parsed to identify device category (mobile, desktop, tablet), operating system (iOS, Android, Windows), and browser type.</li>
              <li><strong>Referrer and Timestamp:</strong> Used for aggregate campaign tracking and hourly distribution graphs.</li>
            </ul>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">3. Account & Workspace Data</h2>
            <p>
              When you register for an account, we collect your email address, display name, and hashed credentials.
              Payment information is handled securely by certified Level 1 PCI-DSS compliant providers (Stripe and Razorpay).
              We never store your raw credit card numbers or UPI MPINs.
            </p>
          </section>

          <section className="space-y-2">
            <h2 className="text-base font-bold text-[var(--text)]">4. Data Retention and Deletion</h2>
            <p>
              Scan analytics data is retained according to your workspace plan limits (30 days on Free up to 3 years on
              Enterprise). You may request full account and workspace data erasure at any time via your workspace settings
              or by contacting privacy@qrit.io.
            </p>
          </section>
        </div>
      </main>

      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit. Privacy Preserved.</p>
      </footer>
    </div>
  );
}
