'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import {
  QrCode,
  Check,
  X,
  HelpCircle,
  Sparkles,
  ArrowRight,
  ShieldCheck,
  Zap,
  Globe2,
  ChevronDown,
} from 'lucide-react';
import { PLANS, PlanCode } from '@/lib/entitlements';

export default function PricingPage() {
  const [interval, setInterval] = useState<'monthly' | 'yearly'>('yearly');
  const [currency, setCurrency] = useState<'USD' | 'INR'>('USD');
  const [openFaq, setOpenFaq] = useState<number | null>(0);

  // Auto-detect India locale based on timezone or language
  useEffect(() => {
    try {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
      const lang = navigator.language || '';
      if (tz.includes('Kolkata') || tz.includes('Calcutta') || lang.includes('IN') || lang.startsWith('hi')) {
        setCurrency('INR');
      }
    } catch {
      // Default to USD
    }
  }, []);

  const faqs = [
    {
      q: 'What happens to my QR codes if I cancel my subscription?',
      a: 'They keep working! Unlike other QR generators that hijack your links or display humiliating cancellation screens to your customers, QRit never deactivates your dynamic QR codes. If you cancel or downgrade, all existing codes continue redirecting forever. Over-limit codes simply transition to read-only mode, meaning you cannot change their target URL until you upgrade or archive unused codes.',
    },
    {
      q: 'Can I pay in Indian Rupees (INR) with UPI?',
      a: 'Yes, absolutely. We offer full domestic INR pricing supporting UPI Autopay, Net Banking, and Indian RuPay/Visa/MasterCard Debit and Credit cards via Razorpay. Global customers can pay via Stripe with any major currency and credit card.',
    },
    {
      q: 'What is the difference between static and dynamic QR codes?',
      a: 'Static QR codes encode the destination directly into the barcode pattern. Once printed, they can never be changed, and their scans cannot be counted or analyzed. Dynamic QR codes route through our edge redirect engine (sub-50ms latency), allowing you to edit the destination anytime, capture real-time scan analytics, schedule expiry, and run device or geo-routing rules without reprinting.',
    },
    {
      q: 'Can I use my own custom domain for short links?',
      a: 'Yes. Pro plans include 1 custom domain, Business includes 5 custom domains, and Enterprise includes unlimited custom domains. You simply point a CNAME to our edge proxy, and we automatically issue and renew zero-configuration TLS/SSL certificates.',
    },
    {
      q: 'Do you offer an API for programmatic QR creation and analytics?',
      a: 'Yes. Business and Enterprise plans come with full REST API keys, OpenAPI documentation, and outbound HMAC-SHA256 signed webhooks so your servers can receive scan events in real-time.',
    },
    {
      q: 'Can I switch between monthly and annual billing?',
      a: 'Yes, you can upgrade, downgrade, or switch billing frequency at any time from your workspace settings. Prorated credits are automatically calculated and applied to your account.',
    },
  ];

  const formatPrice = (planKey: PlanCode) => {
    const p = PLANS[planKey];
    if (currency === 'INR') {
      if (p.priceMonthlyINR === 0) return '₹0';
      if (interval === 'yearly') {
        const perMonth = Math.round(p.priceYearlyINR / 12);
        return `₹${perMonth.toLocaleString('en-IN')}`;
      }
      return `₹${p.priceMonthlyINR.toLocaleString('en-IN')}`;
    } else {
      if (p.priceMonthlyUSD === 0) return '$0';
      if (interval === 'yearly') {
        const perMonth = Math.round(p.priceYearlyUSD / 12);
        return `$${perMonth}`;
      }
      return `$${p.priceMonthlyUSD}`;
    }
  };

  const getBilledAnnualText = (planKey: PlanCode) => {
    const p = PLANS[planKey];
    if (interval !== 'yearly' || p.priceYearlyUSD === 0) return null;
    if (currency === 'INR') {
      return `billed ₹${p.priceYearlyINR.toLocaleString('en-IN')} annually`;
    }
    return `billed $${p.priceYearlyUSD} annually`;
  };

  // Structured data for SEO FAQ schema
  const faqSchema = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: faqs.map((f) => ({
      '@type': 'Question',
      name: f.q,
      acceptedAnswer: {
        '@type': 'Answer',
        text: f.a,
      },
    })),
  };

  return (
    <div className="min-h-screen flex flex-col bg-[var(--bg)]">
      {/* JSON-LD FAQ */}
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(faqSchema) }}
      />

      {/* Top Navbar */}
      <header className="border-b border-[var(--border)] bg-[var(--surface)]/80 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-[var(--accent)] flex items-center justify-center text-white font-bold shadow-sm">
              <QrCode className="w-5 h-5" />
            </div>
            <span className="font-extrabold text-xl tracking-tight text-[var(--text)]">QRit</span>
          </Link>

          <div className="flex items-center gap-4">
            <Link
              href="/login"
              className="text-sm font-medium text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
            >
              Sign In
            </Link>
            <Link
              href="/register"
              className="text-sm font-semibold bg-[var(--accent)] text-[var(--accent-fg)] px-4 py-2 rounded-lg hover:opacity-95 transition-opacity"
            >
              Get Started Free
            </Link>
          </div>
        </div>
      </header>

      {/* Hero Section */}
      <section className="pt-16 pb-12 px-4 text-center max-w-4xl mx-auto space-y-4">
        <div className="inline-flex items-center gap-2 px-3.5 py-1 rounded-full text-xs font-bold bg-[var(--accent)]/10 text-[var(--accent)]">
          <Sparkles className="w-3.5 h-3.5" />
          <span>Transparent, Developer-Grade Infrastructure</span>
        </div>
        <h1 className="text-4xl md:text-5xl font-extrabold tracking-tight text-[var(--text)]">
          Simple pricing. No hostage redirects.
        </h1>
        <p className="text-base md:text-lg text-[var(--text-muted)] max-w-2xl mx-auto leading-relaxed">
          Create custom, dynamic QR codes with real-time analytics. Your printed codes keep working even if you cancel.
        </p>

        {/* Toggles Container */}
        <div className="pt-6 flex flex-wrap items-center justify-center gap-6">
          {/* Billing Interval Toggle */}
          <div className="inline-flex items-center p-1 rounded-xl bg-[var(--surface)] border border-[var(--border)] shadow-xs">
            <button
              onClick={() => setInterval('monthly')}
              className={`px-4 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                interval === 'monthly'
                  ? 'bg-[var(--accent)] text-[var(--accent-fg)] shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)]'
              }`}
            >
              Monthly
            </button>
            <button
              onClick={() => setInterval('yearly')}
              className={`px-4 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-all ${
                interval === 'yearly'
                  ? 'bg-[var(--accent)] text-[var(--accent-fg)] shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)]'
              }`}
            >
              <span>Annual</span>
              <span className="text-[10px] font-extrabold px-1.5 py-0.2 rounded-full bg-emerald-500/20 text-emerald-400">
                Save 20%
              </span>
            </button>
          </div>

          {/* Currency Switcher */}
          <div className="inline-flex items-center p-1 rounded-xl bg-[var(--surface)] border border-[var(--border)] shadow-xs">
            <button
              onClick={() => setCurrency('USD')}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                currency === 'USD'
                  ? 'bg-[var(--bg-subtle)] text-[var(--text)] font-bold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)]'
              }`}
            >
              USD ($)
            </button>
            <button
              onClick={() => setCurrency('INR')}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                currency === 'INR'
                  ? 'bg-[var(--bg-subtle)] text-[var(--text)] font-bold'
                  : 'text-[var(--text-muted)] hover:text-[var(--text)]'
              }`}
            >
              INR (₹ UPI)
            </button>
          </div>
        </div>
      </section>

      {/* Pricing Cards Grid */}
      <section className="max-w-7xl mx-auto px-4 pb-20 w-full">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 items-stretch">
          {/* FREE PLAN */}
          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] flex flex-col justify-between shadow-xs">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-bold text-[var(--text)]">Free</span>
              </div>
              <p className="text-xs text-[var(--text-muted)] min-h-[36px]">
                For hobbyists and quick personal projects.
              </p>
              <div>
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-extrabold text-[var(--text)]">{formatPrice('free')}</span>
                  <span className="text-xs text-[var(--text-muted)]">/month</span>
                </div>
                <div className="text-[11px] text-[var(--text-muted)] mt-1">Free forever</div>
              </div>

              <div className="pt-4 border-t border-[var(--border)] space-y-2.5 text-xs text-[var(--text)]">
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>10</strong> Dynamic QR codes</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>1,000</strong> scans/month</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>30-day analytics retention</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Vector SVG & PNG exports</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Zero code deactivations</span>
                </div>
              </div>
            </div>

            <div className="pt-6">
              <Link
                href="/register"
                className="w-full py-2.5 px-4 rounded-xl text-xs font-bold text-center block bg-[var(--bg-subtle)] text-[var(--text)] hover:bg-[var(--border)] transition-colors"
              >
                Get Started Free
              </Link>
            </div>
          </div>

          {/* PRO PLAN */}
          <div className="p-6 rounded-2xl bg-[var(--surface)] border-2 border-[var(--accent)] flex flex-col justify-between shadow-md relative">
            <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-0.5 rounded-full bg-[var(--accent)] text-[var(--accent-fg)] text-[10px] font-extrabold tracking-wider uppercase shadow-xs">
              Most Popular
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-bold text-[var(--accent)]">Pro</span>
              </div>
              <p className="text-xs text-[var(--text-muted)] min-h-[36px]">
                For creators, small businesses, and marketing campaigns.
              </p>
              <div>
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-extrabold text-[var(--text)]">{formatPrice('pro')}</span>
                  <span className="text-xs text-[var(--text-muted)]">/month</span>
                </div>
                <div className="text-[11px] text-[var(--accent)] font-medium mt-1">
                  {getBilledAnnualText('pro') || 'Billed monthly'}
                </div>
              </div>

              <div className="pt-4 border-t border-[var(--border)] space-y-2.5 text-xs text-[var(--text)]">
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>100</strong> Dynamic QR codes</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>25,000</strong> scans/month</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>1 Custom domain</strong> (SSL included)</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>1-year analytics history</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Password & schedule protection</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Up to 3 team members</span>
                </div>
              </div>
            </div>

            <div className="pt-6">
              <Link
                href="/register?plan=pro"
                className="w-full py-2.5 px-4 rounded-xl text-xs font-bold text-center block bg-[var(--accent)] text-[var(--accent-fg)] hover:opacity-95 shadow-xs transition-opacity"
              >
                Start Free Pro Trial
              </Link>
            </div>
          </div>

          {/* BUSINESS PLAN */}
          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] flex flex-col justify-between shadow-xs">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-bold text-[var(--text)]">Business</span>
              </div>
              <p className="text-xs text-[var(--text-muted)] min-h-[36px]">
                For high-volume brands, agencies, and programmatic platforms.
              </p>
              <div>
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-extrabold text-[var(--text)]">{formatPrice('business')}</span>
                  <span className="text-xs text-[var(--text-muted)]">/month</span>
                </div>
                <div className="text-[11px] text-[var(--text-muted)] mt-1">
                  {getBilledAnnualText('business') || 'Billed monthly'}
                </div>
              </div>

              <div className="pt-4 border-t border-[var(--border)] space-y-2.5 text-xs text-[var(--text)]">
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>1,000</strong> Dynamic QR codes</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>100,000</strong> scans/month</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>5 Custom domains</strong></span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>Geo & Device Routing Rules</strong></span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>Webhooks & REST API Access</strong></span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Up to 10 team seats</span>
                </div>
              </div>
            </div>

            <div className="pt-6">
              <Link
                href="/register?plan=business"
                className="w-full py-2.5 px-4 rounded-xl text-xs font-bold text-center block bg-[var(--text)] text-[var(--bg)] hover:opacity-90 transition-opacity"
              >
                Start Business Trial
              </Link>
            </div>
          </div>

          {/* ENTERPRISE PLAN */}
          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] flex flex-col justify-between shadow-xs">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-bold text-[var(--text)]">Enterprise</span>
              </div>
              <p className="text-xs text-[var(--text-muted)] min-h-[36px]">
                Custom SLAs, millions of scans, SSO, and dedicated infrastructure.
              </p>
              <div>
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-extrabold text-[var(--text)]">{formatPrice('enterprise')}</span>
                  <span className="text-xs text-[var(--text-muted)]">/month</span>
                </div>
                <div className="text-[11px] text-[var(--text-muted)] mt-1">
                  {getBilledAnnualText('enterprise') || 'Billed monthly'}
                </div>
              </div>

              <div className="pt-4 border-t border-[var(--border)] space-y-2.5 text-xs text-[var(--text)]">
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>Unlimited</strong> Dynamic QR codes</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span><strong>Unlimited</strong> scans/month</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Unlimited custom domains</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>SAML / SSO & Role-based audit logs</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>99.99% uptime SLA</span>
                </div>
                <div className="flex items-center gap-2">
                  <Check className="w-4 h-4 text-emerald-500 shrink-0" />
                  <span>Dedicated Slack channel & support</span>
                </div>
              </div>
            </div>

            <div className="pt-6">
              <Link
                href="/register?plan=enterprise"
                className="w-full py-2.5 px-4 rounded-xl text-xs font-bold text-center block bg-[var(--bg-subtle)] text-[var(--text)] hover:bg-[var(--border)] transition-colors"
              >
                Contact Sales
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* Feature Comparison Matrix Table */}
      <section className="border-t border-[var(--border)] bg-[var(--bg-subtle)] py-20 px-4">
        <div className="max-w-6xl mx-auto space-y-8">
          <div className="text-center space-y-2">
            <h2 className="text-2xl md:text-3xl font-extrabold tracking-tight text-[var(--text)]">
              Compare All Features
            </h2>
            <p className="text-xs md:text-sm text-[var(--text-muted)]">
              Detailed breakdown of limits and capabilities across all plans.
            </p>
          </div>

          <div className="bg-[var(--surface)] rounded-2xl border border-[var(--border)] overflow-x-auto shadow-xs">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-[var(--border)] bg-[var(--bg-subtle)]">
                  <th className="py-4 px-6 font-bold text-[var(--text)] w-1/3">Feature</th>
                  <th className="py-4 px-4 font-bold text-[var(--text)] text-center">Free</th>
                  <th className="py-4 px-4 font-bold text-[var(--accent)] text-center">Pro</th>
                  <th className="py-4 px-4 font-bold text-[var(--text)] text-center">Business</th>
                  <th className="py-4 px-4 font-bold text-[var(--text)] text-center">Enterprise</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
                {/* Capacities */}
                <tr className="bg-[var(--bg-subtle)]/50">
                  <td colSpan={5} className="py-2.5 px-6 font-bold uppercase tracking-wider text-[10px] text-[var(--text-muted)]">
                    Capacity & Limits
                  </td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Active Dynamic QRs</td>
                  <td className="py-3 px-4 text-center font-mono">10</td>
                  <td className="py-3 px-4 text-center font-mono font-bold text-[var(--accent)]">100</td>
                  <td className="py-3 px-4 text-center font-mono">1,000</td>
                  <td className="py-3 px-4 text-center font-mono">Unlimited</td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Monthly Scan Volume</td>
                  <td className="py-3 px-4 text-center font-mono">1,000</td>
                  <td className="py-3 px-4 text-center font-mono font-bold text-[var(--accent)]">25,000</td>
                  <td className="py-3 px-4 text-center font-mono">100,000</td>
                  <td className="py-3 px-4 text-center font-mono">Unlimited</td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Analytics Retention</td>
                  <td className="py-3 px-4 text-center">30 Days</td>
                  <td className="py-3 px-4 text-center font-semibold text-[var(--accent)]">1 Year</td>
                  <td className="py-3 px-4 text-center">2 Years</td>
                  <td className="py-3 px-4 text-center">3 Years</td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Custom Short Domains</td>
                  <td className="py-3 px-4 text-center text-[var(--text-muted)]">—</td>
                  <td className="py-3 px-4 text-center font-semibold text-[var(--accent)]">1 Domain</td>
                  <td className="py-3 px-4 text-center">5 Domains</td>
                  <td className="py-3 px-4 text-center">Unlimited</td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Team Member Seats</td>
                  <td className="py-3 px-4 text-center font-mono">1</td>
                  <td className="py-3 px-4 text-center font-mono font-bold text-[var(--accent)]">3</td>
                  <td className="py-3 px-4 text-center font-mono">10</td>
                  <td className="py-3 px-4 text-center font-mono">Unlimited</td>
                </tr>

                {/* Advanced Features */}
                <tr className="bg-[var(--bg-subtle)]/50">
                  <td colSpan={5} className="py-2.5 px-6 font-bold uppercase tracking-wider text-[10px] text-[var(--text-muted)]">
                    Features & Security
                  </td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">No Code Deactivation Guarantee</td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">High-Res Vector Exports (SVG & PDF)</td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Password Protection & Expiry Dates</td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Advanced Rules Engine (Geo, OS, Language)</td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">Real-time Outbound Webhooks</td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">REST API Access & Personal API Keys</td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
                <tr>
                  <td className="py-3 px-6 text-[var(--text)] font-medium">SAML / SSO & Security Auditing</td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><X className="w-4 h-4 text-[var(--text-muted)] mx-auto opacity-40" /></td>
                  <td className="py-3 px-4 text-center"><Check className="w-4 h-4 text-emerald-500 mx-auto" /></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* Anti-Trap Guarantee Banner */}
      <section className="py-12 px-4 max-w-4xl mx-auto w-full">
        <div className="p-8 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-center space-y-3">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/20 text-emerald-600 flex items-center justify-center mx-auto">
            <ShieldCheck className="w-6 h-6" />
          </div>
          <h3 className="text-lg font-bold text-[var(--text)]">The QRit Anti-Lock-in Pledge</h3>
          <p className="text-xs text-[var(--text-muted)] max-w-xl mx-auto leading-relaxed">
            Other platforms disable your codes or redirect your traffic to aggressive upsell pages when you cancel.
            At QRit, your QR codes will <strong>never stop redirecting</strong>. If you downgrade, your excess codes simply become read-only.
          </p>
        </div>
      </section>

      {/* FAQ Section */}
      <section className="py-16 px-4 max-w-3xl mx-auto w-full space-y-8">
        <div className="text-center space-y-2">
          <h2 className="text-2xl md:text-3xl font-extrabold tracking-tight text-[var(--text)]">
            Frequently Asked Questions
          </h2>
          <p className="text-xs md:text-sm text-[var(--text-muted)]">
            Everything you need to know about plans, billing, and redirect reliability.
          </p>
        </div>

        <div className="space-y-3">
          {faqs.map((faq, idx) => {
            const isOpen = openFaq === idx;
            return (
              <div
                key={idx}
                className="rounded-xl border border-[var(--border)] bg-[var(--surface)] overflow-hidden transition-all"
              >
                <button
                  onClick={() => setOpenFaq(isOpen ? null : idx)}
                  className="w-full p-4 text-left flex items-center justify-between gap-4 font-semibold text-xs text-[var(--text)] hover:bg-[var(--bg-subtle)] transition-colors"
                >
                  <span>{faq.q}</span>
                  <ChevronDown
                    className={`w-4 h-4 text-[var(--text-muted)] shrink-0 transition-transform duration-200 ${
                      isOpen ? 'rotate-180 text-[var(--accent)]' : ''
                    }`}
                  />
                </button>
                {isOpen && (
                  <div className="px-4 pb-4 pt-1 text-xs text-[var(--text-muted)] leading-relaxed border-t border-[var(--border)]/50">
                    {faq.a}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>

      {/* CTA Bottom Banner */}
      <section className="border-t border-[var(--border)] bg-[var(--surface)] py-16 px-4 text-center">
        <div className="max-w-2xl mx-auto space-y-4">
          <h2 className="text-2xl md:text-3xl font-extrabold tracking-tight text-[var(--text)]">
            Ready to upgrade your QR infrastructure?
          </h2>
          <p className="text-xs md:text-sm text-[var(--text-muted)]">
            Join thousands of modern creators, marketers, and developers building with QRit.
          </p>
          <div className="pt-2">
            <Link
              href="/register"
              className="inline-flex items-center gap-2 px-6 py-3 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-bold text-xs shadow-sm hover:opacity-95 transition-opacity"
            >
              <span>Get Started in 30 Seconds</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit. All rights reserved.</p>
      </footer>
    </div>
  );
}
