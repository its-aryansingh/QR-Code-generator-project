'use client';

import React, { useState, useMemo } from 'react';
import Link from 'next/link';
import { renderSvg, DEFAULT_DESIGN } from '@qrit/qr-render';
import { QrCode, Contact, Download, UserCheck } from 'lucide-react';

export default function VCardQRPage() {
  const [firstName, setFirstName] = useState('Aryan');
  const [lastName, setLastName] = useState('Singh');
  const [company, setCompany] = useState('QRit Inc.');
  const [title, setTitle] = useState('Founder & Lead Architect');
  const [phone, setPhone] = useState('+91 98765 43210');
  const [email, setEmail] = useState('aryan@qrit.io');
  const [website, setWebsite] = useState('https://qrit.io');

  // vCard 3.0 standard payload
  const payload = useMemo(() => {
    return [
      'BEGIN:VCARD',
      'VERSION:3.0',
      `N:${lastName};${firstName};;;`,
      `FN:${firstName} ${lastName}`.trim(),
      company ? `ORG:${company}` : '',
      title ? `TITLE:${title}` : '',
      phone ? `TEL;TYPE=CELL:${phone}` : '',
      email ? `EMAIL;TYPE=INTERNET:${email}` : '',
      website ? `URL:${website}` : '',
      'END:VCARD',
    ]
      .filter(Boolean)
      .join('\n');
  }, [firstName, lastName, company, title, phone, email, website]);

  const renderResult = useMemo(() => {
    try {
      return renderSvg({ payload: payload || 'BEGIN:VCARD\nVERSION:3.0\nFN:Aryan\nEND:VCARD', design: DEFAULT_DESIGN });
    } catch {
      return renderSvg({ payload: 'BEGIN:VCARD\nVERSION:3.0\nFN:Aryan\nEND:VCARD', design: DEFAULT_DESIGN });
    }
  }, [payload]);

  const handleDownloadSvg = () => {
    const blob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `vcard-${firstName}-${lastName}.svg`;
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
          <Contact className="w-3.5 h-3.5" />
          <span>vCard 3.0 Digital Contact</span>
        </div>
        <h1 className="text-3xl md:text-5xl font-extrabold tracking-tight text-[var(--text)]">
          Free vCard QR Code Generator
        </h1>
        <p className="text-sm md:text-base text-[var(--text-muted)] max-w-xl mx-auto">
          Share your contact card, phone number, and social links instantly. Anyone scanning saves your contact in one tap.
        </p>
      </section>

      <main className="max-w-5xl mx-auto px-4 py-6 w-full flex-1">
        <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">
          {/* Controls */}
          <div className="md:col-span-7 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 shadow-xs">
            <h2 className="text-sm font-bold text-[var(--text)]">Contact Information</h2>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">First Name</label>
                <input
                  type="text"
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  placeholder="Aryan"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Last Name</label>
                <input
                  type="text"
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  placeholder="Singh"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Company / Organization</label>
                <input
                  type="text"
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                  placeholder="Company Name"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Job Title</label>
                <input
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="e.g. Managing Director"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Phone Number</label>
                <input
                  type="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="+1 234 567 8900"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Email Address</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="hello@example.com"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Website URL</label>
              <input
                type="url"
                value={website}
                onChange={(e) => setWebsite(e.target.value)}
                placeholder="https://yourwebsite.com"
                className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
              />
            </div>
          </div>

          {/* Preview & Download */}
          <div className="md:col-span-5 bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4 text-center shadow-xs">
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">
              Digital Business Card Preview
            </h3>

            <div className="p-4 bg-white rounded-2xl border border-[var(--border)] inline-block mx-auto shadow-inner">
              <div
                className="w-48 h-48"
                dangerouslySetInnerHTML={{ __html: renderResult.svg }}
              />
            </div>

            <div className="p-3 bg-[var(--bg-subtle)] rounded-xl text-left space-y-1">
              <div className="font-bold text-xs text-[var(--text)]">{firstName} {lastName}</div>
              <div className="text-[11px] text-[var(--text-muted)]">{title} • {company}</div>
              <div className="text-[11px] text-[var(--accent)] font-mono">{phone}</div>
            </div>

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
        <p>© {new Date().getFullYear()} QRit. vCard Digital Contact Generator.</p>
      </footer>
    </div>
  );
}
