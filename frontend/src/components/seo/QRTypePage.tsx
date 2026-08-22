import React from 'react';
import Link from 'next/link';
import { QRTypeClient } from './QRTypeClient';
import { qrTypes, QRIcons } from '@/utils/qrConstants';

interface FAQ {
  question: string;
  answer: string;
}

interface UseCase {
  title: string;
  desc: string;
}

interface Step {
  title: string;
  desc: string;
}

export interface QRTypePageProps {
  type: string;
  heroH1: string;
  heroSubtitle: string;
  howToSteps: Step[];
  useCases: UseCase[];
  faqs: FAQ[];
}

export function QRTypePage({
  type,
  heroH1,
  heroSubtitle,
  howToSteps,
  useCases,
  faqs
}: QRTypePageProps) {
  return (
    <div className="min-h-screen bg-[#0d0d1a] text-white">
      {/* Navigation - simplified for landing page */}
      <nav className="sticky top-0 z-50 bg-[#0d0d1a]/80 backdrop-blur-xl border-b border-white/10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-violet-500/30">
              <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                <rect x="3" y="3" width="7" height="7" rx="1.5" fill="white" />
                <rect x="14" y="3" width="7" height="7" rx="1.5" fill="white" />
                <rect x="3" y="14" width="7" height="7" rx="1.5" fill="white" />
                <rect x="14" y="14" width="4" height="4" rx="1" fill="white" />
                <rect x="20" y="14" width="1" height="4" rx="0.5" fill="white" />
                <rect x="14" y="20" width="4" height="1" rx="0.5" fill="white" />
              </svg>
            </div>
            <span className="text-lg font-bold text-white">QR<span className="text-violet-400">it</span></span>
          </Link>
          <div className="flex items-center gap-3">
            <Link href="/login" className="text-sm text-slate-400 hover:text-white transition-colors px-3 py-2">
              Sign In
            </Link>
            <Link href="/register" className="px-4 py-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-sm font-semibold rounded-xl transition-all shadow-lg shadow-violet-500/20">
              Get Started Free
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <section className="text-center py-16 px-4 relative overflow-hidden border-b border-white/5">
        <div className="pointer-events-none absolute inset-0 -z-10">
          <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[300px] bg-violet-600/10 rounded-full blur-[100px]" />
        </div>
        <div className="max-w-3xl mx-auto space-y-4">
          <h1 className="text-4xl sm:text-5xl md:text-6xl font-extrabold tracking-tight text-white leading-tight">
            {heroH1}
          </h1>
          <p className="text-slate-400 text-base sm:text-lg max-w-xl mx-auto">
            {heroSubtitle}
          </p>
        </div>
      </section>

      {/* Generator Section */}
      <section className="px-4 sm:px-6 -mt-8">
        <QRTypeClient type={type} />
      </section>

      {/* How to Create Section */}
      <section className="py-20 px-4 max-w-7xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold mb-4">How to create a {type} QR Code</h2>
          <p className="text-slate-400">Follow these 3 simple steps to generate your QR code instantly.</p>
        </div>
        <div className="grid md:grid-cols-3 gap-8">
          {howToSteps.map((step, idx) => (
            <div key={idx} className="bg-white/5 border border-white/10 rounded-2xl p-6 text-center">
              <div className="w-12 h-12 rounded-full bg-violet-600/20 text-violet-400 flex items-center justify-center text-xl font-bold mx-auto mb-4 border border-violet-500/30">
                {idx + 1}
              </div>
              <h3 className="text-lg font-semibold mb-2">{step.title}</h3>
              <p className="text-sm text-slate-400">{step.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Use Cases Section */}
      <section className="py-20 px-4 bg-white/[0.02] border-y border-white/5">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold mb-4">Where to use it</h2>
            <p className="text-slate-400">Discover popular use cases for this QR type.</p>
          </div>
          <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {useCases.map((uc, idx) => (
              <div key={idx} className="bg-white/5 border border-white/10 rounded-2xl p-6 hover:border-violet-500/30 transition-colors">
                <h3 className="text-lg font-semibold text-violet-300 mb-2">{uc.title}</h3>
                <p className="text-sm text-slate-400">{uc.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* FAQ Section */}
      <section className="py-20 px-4 max-w-3xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold mb-4">Frequently Asked Questions</h2>
        </div>
        <div className="space-y-4">
          {faqs.map((faq, idx) => (
            <details key={idx} className="group bg-white/5 border border-white/10 rounded-xl overflow-hidden">
              <summary className="flex items-center justify-between p-5 font-semibold cursor-pointer select-none">
                {faq.question}
                <span className="text-violet-400 group-open:-rotate-180 transition-transform duration-300">▼</span>
              </summary>
              <div className="px-5 pb-5 text-sm text-slate-400 leading-relaxed border-t border-white/10 pt-4">
                {faq.answer}
              </div>
            </details>
          ))}
        </div>
      </section>

      {/* Cross-links / Explore Other QR Types */}
      <section className="py-20 px-4 max-w-7xl mx-auto border-t border-white/10">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold mb-4">Explore Other QR Types</h2>
          <p className="text-slate-400">QRit supports 14+ QR code types for all your needs.</p>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-7 gap-4">
          {qrTypes.filter(t => t.id !== type).map(t => (
            <Link key={t.id} href={`/${t.id}-qr-code`} className="flex flex-col items-center gap-2 p-4 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 hover:border-violet-500/30 transition-all text-center group">
              <div className="w-8 h-8 flex items-center justify-center group-hover:scale-110 transition-transform text-violet-400">
                {QRIcons[t.id]}
              </div>
              <span className="text-xs font-medium text-slate-300">{t.name}</span>
            </Link>
          ))}
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-24 px-4 bg-gradient-to-b from-transparent to-violet-900/20 text-center border-t border-white/5">
        <h2 className="text-3xl md:text-4xl font-bold mb-6">Ready to manage your QR codes?</h2>
        <p className="text-slate-400 mb-8 max-w-2xl mx-auto">Create an account to track scans, edit destinations, and organize your QR codes in workspaces.</p>
        <Link href="/register" className="inline-block px-8 py-4 bg-violet-600 hover:bg-violet-500 text-white font-bold rounded-xl shadow-lg shadow-violet-500/30 transition-all">
          Create Free Account
        </Link>
      </section>

      {/* Footer */}
      <footer className="border-t border-white/8 py-8 px-4">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <span className="text-sm font-bold text-slate-300">QR<span className="text-violet-400">it</span></span>
          </div>
          <div className="flex items-center gap-6 text-xs text-slate-600">
            <Link href="/" className="hover:text-slate-400 transition-colors">Home</Link>
            <Link href="/login" className="hover:text-slate-400 transition-colors">Sign In</Link>
            <Link href="/register" className="hover:text-slate-400 transition-colors">Register</Link>
          </div>
          <p className="text-xs text-slate-700">© 2025 QRit</p>
        </div>
      </footer>
    </div>
  );
}
