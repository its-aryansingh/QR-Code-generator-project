"use client";

import Link from "next/link";

const FEATURES = [
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
      </svg>
    ),
    title: "Workspace & Team Management",
    desc: "Invite members, assign roles (Owner, Admin, Editor, Viewer), and keep every team working inside their own scoped workspace — no code mixing, no permission accidents.",
    badge: "Phase 1",
    gradient: "from-violet-500/15 to-indigo-500/10",
    accent: "text-violet-400",
    border: "border-violet-500/20",
  },
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
    ),
    title: "Bulk QR Generation",
    desc: "Upload a CSV. Get a ZIP. Generate up to 100 QR codes in one request — URL, vCard, WiFi, and more. Enterprise plan goes up to 1 000 rows with async processing.",
    badge: "Phase 1",
    gradient: "from-emerald-500/15 to-teal-500/10",
    accent: "text-emerald-400",
    border: "border-emerald-500/20",
  },
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
      </svg>
    ),
    title: "Folders & Campaigns",
    desc: "Nest folders inside workspaces to mirror your campaign structure. Filter, bulk-move, and search across thousands of codes with zero friction.",
    badge: "Phase 1",
    gradient: "from-amber-500/15 to-orange-500/10",
    accent: "text-amber-400",
    border: "border-amber-500/20",
  },
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
      </svg>
    ),
    title: "Webhooks & Integrations",
    desc: "Trigger events on scan, create, or update. Built-in retry logic keeps your Zapier, HubSpot, and Slack pipelines running even when endpoints hiccup.",
    badge: "Phase 2",
    gradient: "from-sky-500/15 to-cyan-500/10",
    accent: "text-sky-400",
    border: "border-sky-500/20",
  },
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
      </svg>
    ),
    title: "White-Label Landing Pages",
    desc: "Serve QR experiences from your own domain (qr.yourbrand.com) with your colors, logo, and custom CSS. Completely remove QRit branding on Enterprise.",
    badge: "Phase 3",
    gradient: "from-pink-500/15 to-rose-500/10",
    accent: "text-pink-400",
    border: "border-pink-500/20",
  },
  {
    icon: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
      </svg>
    ),
    title: "Audit Logs & Compliance",
    desc: "Every CRUD operation is stamped with actor, timestamp, and IP address. Export logs to CSV for SOC 2 and ISO 27001 evidence packages.",
    badge: "Phase 3",
    gradient: "from-fuchsia-500/15 to-purple-500/10",
    accent: "text-fuchsia-400",
    border: "border-fuchsia-500/20",
  },
];

const STATS = [
  { value: "14+", label: "QR Code Types" },
  { value: "100", label: "Bulk per Request" },
  { value: "4", label: "Role Levels" },
  { value: "99.9%", label: "Uptime SLA" },
];

const PLANS = [
  {
    name: "Starter",
    price: "$29",
    period: "/mo",
    desc: "For growing teams getting started with dynamic QR.",
    features: ["5 workspace members", "500 QR codes", "Basic analytics", "API access", "Email support"],
    cta: "Start Free Trial",
    highlight: false,
  },
  {
    name: "Pro",
    price: "$79",
    period: "/mo",
    desc: "Full enterprise feature set for scaling campaigns.",
    features: ["25 workspace members", "5 000 QR codes", "Advanced analytics + CSV export", "Webhooks & integrations", "Password-protected QR", "Priority support"],
    cta: "Start Free Trial",
    highlight: true,
  },
  {
    name: "Enterprise",
    price: "Custom",
    period: "",
    desc: "Unlimited scale with white-label, SSO, and SLA.",
    features: ["Unlimited members & workspaces", "Unlimited QR codes", "Bulk up to 1 000/request", "White-label & custom domain", "SAML / OIDC SSO", "Audit logs & compliance", "Dedicated CSM"],
    cta: "Contact Sales",
    highlight: false,
  },
];

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">

      {/* ── NAV ── */}
      <nav className="sticky top-0 z-50 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-xl">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          {/* Logo */}
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-white flex items-center justify-center">
              <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#18181b" />
                <rect x="14" y="14" width="4" height="4" rx="1" fill="#18181b" />
                <rect x="20" y="14" width="1" height="4" rx="0.5" fill="#18181b" />
                <rect x="14" y="20" width="4" height="1" rx="0.5" fill="#18181b" />
                <rect x="20" y="20" width="1" height="1" rx="0.5" fill="#18181b" />
              </svg>
            </div>
            <span className="text-lg font-bold">QR<span className="text-zinc-500">it</span></span>
            <span className="hidden sm:inline text-xs font-semibold tracking-widest text-zinc-600 uppercase border border-zinc-800 rounded px-2 py-0.5 ml-1">Enterprise</span>
          </div>

          {/* Links */}
          <div className="hidden md:flex items-center gap-8 text-sm text-zinc-400">
            <a href="#features" className="hover:text-zinc-100 transition-colors">Features</a>
            <a href="#pricing" className="hover:text-zinc-100 transition-colors">Pricing</a>
            <Link href="/dashboard/api" className="hover:text-zinc-100 transition-colors">API Docs</Link>
          </div>

          {/* Auth CTAs */}
          <div className="flex items-center gap-3">
            <Link href="/login" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors hidden sm:block">
              Sign In
            </Link>
            <Link
              href="/register"
              className="px-4 py-2 bg-white text-zinc-950 text-sm font-semibold rounded-lg hover:bg-zinc-100 transition-colors"
            >
              Start Free Trial
            </Link>
          </div>
        </div>
      </nav>

      {/* ── HERO ── */}
      <section className="relative overflow-hidden py-24 sm:py-32">
        {/* Ambient blobs */}
        <div className="pointer-events-none absolute inset-0 -z-10">
          <div className="absolute -top-32 left-1/2 -translate-x-1/2 w-[800px] h-[500px] bg-violet-600/10 rounded-full blur-[120px]" />
          <div className="absolute top-40 right-0 w-[400px] h-[400px] bg-indigo-600/8 rounded-full blur-[100px]" />
        </div>

        <div className="max-w-4xl mx-auto px-6 text-center">
          {/* Eyebrow */}
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-violet-500/10 border border-violet-500/20 text-violet-300 text-xs font-semibold tracking-wider uppercase mb-6">
            <span className="w-1.5 h-1.5 rounded-full bg-violet-400 animate-pulse" />
            Enterprise QR Management Platform
          </div>

          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight leading-[1.05] mb-6">
            The QR platform your{" "}
            <span className="bg-gradient-to-r from-violet-400 to-indigo-400 bg-clip-text text-transparent">
              enterprise team
            </span>{" "}
            actually wants to use
          </h1>

          <p className="text-lg sm:text-xl text-zinc-400 max-w-2xl mx-auto leading-relaxed mb-10">
            Workspaces, role-based access, bulk CSV generation, webhook integrations,
            audit logs, and white-labeling — everything Beaconstac and Scanova charge
            a fortune for, built for modern engineering teams.
          </p>

          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link
              href="/register"
              className="inline-flex items-center justify-center gap-2 px-8 py-3.5 bg-gradient-to-r from-violet-600 to-indigo-600 text-white font-semibold rounded-xl hover:from-violet-500 hover:to-indigo-500 transition-all shadow-xl shadow-violet-500/20"
            >
              Start Free — No Credit Card
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
              </svg>
            </Link>
            <Link
              href="/dashboard"
              className="inline-flex items-center justify-center gap-2 px-8 py-3.5 border border-zinc-700 text-zinc-300 font-semibold rounded-xl hover:border-zinc-600 hover:text-zinc-100 hover:bg-zinc-900 transition-all"
            >
              View Dashboard Demo
            </Link>
          </div>

          {/* Trust logos row */}
          <p className="mt-12 text-xs text-zinc-600 uppercase tracking-widest mb-4">Trusted by teams at</p>
          <div className="flex items-center justify-center gap-8 opacity-30 grayscale">
            {["Acme Corp", "Globex", "Initech", "Umbrella", "Hooli"].map((name) => (
              <span key={name} className="text-sm font-bold text-zinc-300 tracking-wide">{name}</span>
            ))}
          </div>
        </div>
      </section>

      {/* ── STATS ── */}
      <section className="border-y border-zinc-800/60 py-10">
        <div className="max-w-4xl mx-auto px-6">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-8 text-center">
            {STATS.map((s) => (
              <div key={s.label}>
                <p className="text-3xl font-extrabold text-zinc-100 mb-1">{s.value}</p>
                <p className="text-sm text-zinc-500">{s.label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── FEATURES ── */}
      <section id="features" className="py-24 sm:py-32">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold mb-4">
              Built for enterprise from day one
            </h2>
            <p className="text-zinc-400 max-w-xl mx-auto text-lg">
              Every feature your B2B clients demand — shipped across three deliberate phases.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {FEATURES.map((f) => (
              <div
                key={f.title}
                className={`group relative p-7 rounded-2xl bg-gradient-to-br ${f.gradient} border ${f.border} hover:border-opacity-60 transition-all duration-300`}
              >
                <div className={`w-11 h-11 rounded-xl bg-zinc-900/80 flex items-center justify-center mb-5 ${f.accent} group-hover:scale-110 transition-transform duration-300`}>
                  {f.icon}
                </div>
                <div className="flex items-start justify-between mb-2">
                  <h3 className="font-semibold text-zinc-100 text-[15px]">{f.title}</h3>
                  <span className={`text-[10px] font-semibold px-2 py-0.5 rounded-full border ${f.border} ${f.accent} ml-3 flex-shrink-0`}>
                    {f.badge}
                  </span>
                </div>
                <p className="text-zinc-400 text-sm leading-relaxed">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── WORKFLOW CALLOUT ── */}
      <section className="py-20 border-y border-zinc-800/60">
        <div className="max-w-5xl mx-auto px-6">
          <div className="grid lg:grid-cols-2 gap-12 items-center">
            <div>
              <h2 className="text-3xl font-bold mb-5">One CSV. 100 QR codes. One click.</h2>
              <p className="text-zinc-400 leading-relaxed mb-6">
                Stop generating QR codes one at a time. Upload a spreadsheet with your
                campaign URLs, select the workspace folder, and QRit streams a ZIP archive
                back — each PNG named after the row title.
              </p>
              <ul className="space-y-3 text-sm text-zinc-300">
                {[
                  "Auto-detects title, content, and type columns",
                  "Concurrent generation with 5-worker pool",
                  "summary.txt included in ZIP with error report",
                  "Up to 1 000 rows on Enterprise plan",
                ].map((item) => (
                  <li key={item} className="flex items-center gap-3">
                    <div className="w-5 h-5 rounded-full bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center flex-shrink-0">
                      <svg className="w-3 h-3 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                      </svg>
                    </div>
                    {item}
                  </li>
                ))}
              </ul>
              <Link href="/dashboard" className="inline-flex items-center gap-2 mt-8 px-6 py-3 bg-emerald-600/20 border border-emerald-500/30 text-emerald-300 font-semibold rounded-xl hover:bg-emerald-600/30 transition-colors text-sm">
                Try Bulk Upload
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                </svg>
              </Link>
            </div>

            {/* Code preview */}
            <div className="rounded-2xl bg-zinc-900 border border-zinc-800 overflow-hidden">
              <div className="px-5 py-3 border-b border-zinc-800 flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-red-500/60" />
                <div className="w-3 h-3 rounded-full bg-amber-500/60" />
                <div className="w-3 h-3 rounded-full bg-emerald-500/60" />
                <span className="ml-3 text-xs text-zinc-600">campaign.csv</span>
              </div>
              <pre className="p-5 text-xs text-zinc-300 font-mono leading-relaxed overflow-x-auto">
{`title,content,type
"Product Launch","https://acme.com/launch","url"
"Trade Show WiFi","WIFI:T:WPA;S:AcmeExpo;P:secret;;","wifi"
"Sales Contact","BEGIN:VCARD\\nFN:Jane Doe\\n...","vcard"
"Support Page","https://acme.com/support","url"
... 96 more rows`}
              </pre>
              <div className="px-5 py-3 border-t border-zinc-800 flex items-center justify-between">
                <span className="text-xs text-zinc-600">100 rows detected</span>
                <span className="text-xs text-emerald-400 font-semibold">→ qrit-bulk-qrcodes.zip</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── PRICING ── */}
      <section id="pricing" className="py-24 sm:py-32">
        <div className="max-w-6xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold mb-4">Transparent pricing</h2>
            <p className="text-zinc-400 max-w-lg mx-auto">
              Start free. Upgrade when your team grows. No hidden per-scan fees.
            </p>
          </div>

          <div className="grid md:grid-cols-3 gap-6">
            {PLANS.map((plan) => (
              <div
                key={plan.name}
                className={`relative rounded-2xl p-8 flex flex-col ${plan.highlight
                  ? "bg-gradient-to-b from-violet-600/20 to-indigo-600/10 border border-violet-500/40 shadow-2xl shadow-violet-500/10"
                  : "bg-zinc-900 border border-zinc-800"
                  }`}
              >
                {plan.highlight && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                    <span className="px-3 py-1 bg-violet-600 text-white text-xs font-bold rounded-full tracking-wider uppercase">
                      Most Popular
                    </span>
                  </div>
                )}
                <div className="mb-6">
                  <p className="text-sm font-semibold text-zinc-400 mb-1">{plan.name}</p>
                  <div className="flex items-end gap-1 mb-2">
                    <span className="text-4xl font-extrabold text-zinc-100">{plan.price}</span>
                    {plan.period && <span className="text-zinc-500 mb-1">{plan.period}</span>}
                  </div>
                  <p className="text-sm text-zinc-500">{plan.desc}</p>
                </div>

                <ul className="space-y-2.5 mb-8 flex-1">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-center gap-2.5 text-sm text-zinc-300">
                      <svg className="w-4 h-4 text-violet-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                      </svg>
                      {f}
                    </li>
                  ))}
                </ul>

                <Link
                  href={plan.name === "Enterprise" ? "mailto:sales@qrit.io" : "/register"}
                  className={`block text-center py-3 rounded-xl font-semibold text-sm transition-all ${plan.highlight
                    ? "bg-gradient-to-r from-violet-600 to-indigo-600 text-white hover:from-violet-500 hover:to-indigo-500 shadow-lg shadow-violet-500/20"
                    : "bg-zinc-800 text-zinc-200 hover:bg-zinc-700 border border-zinc-700"
                    }`}
                >
                  {plan.cta}
                </Link>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── FINAL CTA ── */}
      <section className="py-20 border-t border-zinc-800/60">
        <div className="max-w-3xl mx-auto px-6 text-center">
          <h2 className="text-3xl sm:text-4xl font-bold mb-5">
            Ready to replace your legacy QR tool?
          </h2>
          <p className="text-zinc-400 mb-8 text-lg">
            14-day free trial. Full enterprise feature access. Cancel anytime.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link
              href="/register"
              className="inline-flex items-center justify-center gap-2 px-8 py-3.5 bg-white text-zinc-950 font-bold rounded-xl hover:bg-zinc-100 transition-colors shadow-xl"
            >
              Create your workspace
            </Link>
            <Link
              href="/pricing"
              className="inline-flex items-center justify-center gap-2 px-8 py-3.5 border border-zinc-700 text-zinc-400 font-semibold rounded-xl hover:border-zinc-600 hover:text-zinc-200 hover:bg-zinc-900 transition-all"
            >
              Compare plans
            </Link>
          </div>
        </div>
      </section>

      {/* ── FOOTER ── */}
      <footer className="border-t border-zinc-800/60 py-12">
        <div className="max-w-7xl mx-auto px-6">
          <div className="flex flex-col md:flex-row items-center justify-between gap-6">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-md bg-white flex items-center justify-center">
                <svg viewBox="0 0 24 24" className="w-4 h-4" fill="none">
                  <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                  <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                  <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#18181b" />
                  <rect x="14" y="14" width="4" height="4" rx="1" fill="#18181b" />
                </svg>
              </div>
              <span className="font-bold text-zinc-200">QR<span className="text-zinc-500">it</span></span>
              <span className="text-xs text-zinc-700 ml-1">Enterprise</span>
            </div>
            <div className="flex items-center gap-8 text-sm text-zinc-500">
              <Link href="/pricing" className="hover:text-zinc-300 transition-colors">Pricing</Link>
              <Link href="/dashboard/api" className="hover:text-zinc-300 transition-colors">API</Link>
              <a href="#features" className="hover:text-zinc-300 transition-colors">Features</a>
              <Link href="/login" className="hover:text-zinc-300 transition-colors">Sign In</Link>
            </div>
            <p className="text-sm text-zinc-600">© 2025 QRit. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
