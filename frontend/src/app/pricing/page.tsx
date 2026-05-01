"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/auth";

const plans = [
    {
        name: "Free",
        price: "$0",
        period: "forever",
        description: "Perfect for personal use",
        features: [
            "10 QR codes per day",
            "Static QR codes only",
            "Basic types (URL, Text, WiFi)",
            "PNG download",
            "Color customization",
            "No account required",
        ],
        notIncluded: [
            "Dynamic QR codes",
            "Analytics tracking",
            "Logo/branding",
            "SVG/PDF export",
            "API access",
        ],
        cta: "Get Started Free",
        href: "/",
        popular: false,
    },
    {
        name: "Starter",
        price: "$9",
        period: "per month",
        description: "Great for small businesses",
        features: [
            "Unlimited static QR codes",
            "10 dynamic QR codes",
            "All QR types",
            "PNG & SVG download",
            "Color customization",
            "Logo overlay",
            "Basic analytics",
            "Email support",
        ],
        notIncluded: [
            "Bulk generation",
            "API access",
            "Advanced analytics",
        ],
        cta: "Start Free Trial",
        href: "/register?plan=starter",
        popular: false,
    },
    {
        name: "Pro",
        price: "$29",
        period: "per month",
        description: "Best for growing teams",
        features: [
            "Unlimited static QR codes",
            "100 dynamic QR codes",
            "All QR types",
            "PNG, SVG & PDF download",
            "Full customization",
            "Logo overlay & frames",
            "Advanced analytics",
            "Location tracking",
            "Device breakdown",
            "API access (1,000/day)",
            "Priority support",
        ],
        notIncluded: [],
        cta: "Start Free Trial",
        href: "/register?plan=pro",
        popular: true,
    },
    {
        name: "Enterprise",
        price: "Custom",
        period: "contact us",
        description: "For large organizations",
        features: [
            "Everything in Pro",
            "Unlimited dynamic QR codes",
            "Custom domain redirects",
            "API access (10,000+/day)",
            "Bulk generation",
            "SSO & team management",
            "Dedicated account manager",
            "SLA guarantee",
            "Custom integrations",
            "White-label options",
        ],
        notIncluded: [],
        cta: "Contact Sales",
        href: "/contact?plan=enterprise",
        popular: false,
    },
];

const faqs = [
    {
        q: "What's the difference between static and dynamic QR codes?",
        a: "Static QR codes embed the destination URL directly - they can't be changed after creation. Dynamic QR codes redirect through our servers, allowing you to update the destination without reprinting the QR code."
    },
    {
        q: "Can I upgrade or downgrade my plan?",
        a: "Yes! You can change your plan at any time. When upgrading, you'll be prorated for the remainder of your billing cycle. Downgrades take effect at the next billing date."
    },
    {
        q: "Do you offer refunds?",
        a: "We offer a 14-day money-back guarantee for all paid plans. If you're not satisfied, contact us for a full refund."
    },
    {
        q: "What payment methods do you accept?",
        a: "We accept all major credit cards (Visa, Mastercard, American Express) and PayPal. Enterprise customers can pay via invoice."
    },
    {
        q: "Is there a limit on QR code scans?",
        a: "No! There's no limit on how many times your QR codes can be scanned, regardless of your plan."
    },
];

export default function PricingPage() {
    const [billingPeriod, setBillingPeriod] = useState<"monthly" | "yearly">("monthly");
    const [expandedFaq, setExpandedFaq] = useState<number | null>(null);
    const [mobileNavOpen, setMobileNavOpen] = useState(false);
    const [loadingPlan, setLoadingPlan] = useState<string | null>(null);
    const router = useRouter();
    const { isAuthenticated, accessToken } = useAuthStore();
    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

    const handleCheckout = async (planName: string, href: string) => {
        if (planName === "Free" || planName === "Enterprise") {
            router.push(href);
            return;
        }

        if (!isAuthenticated) {
            router.push(href);
            return;
        }

        setLoadingPlan(planName);
        try {
            const res = await fetch(`${api}/payments/checkout`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${accessToken}`,
                },
                body: JSON.stringify({
                    plan: planName.toLowerCase(),
                    billing_period: billingPeriod,
                }),
            });
            const data = await res.json();
            if (data.success && data.checkout_url) {
                window.location.href = data.checkout_url;
            } else {
                router.push(href); // fallback
            }
        } catch {
            router.push(href); // fallback
        } finally {
            setLoadingPlan(null);
        }
    };

    const getPrice = (plan: typeof plans[0]) => {
        if (plan.price === "$0" || plan.price === "Custom") return plan.price;

        const monthlyPrice = parseInt(plan.price.replace("$", ""));
        if (billingPeriod === "yearly") {
            const yearlyPrice = Math.round(monthlyPrice * 12 * 0.8); // 20% discount
            return `$${yearlyPrice}`;
        }
        return plan.price;
    };

    const getPeriod = (plan: typeof plans[0]) => {
        if (plan.period === "forever" || plan.period === "contact us") return plan.period;
        return billingPeriod === "yearly" ? "per year" : "per month";
    };

    return (
        <div className="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
            {/* Header */}
            <header className="border-b border-white/10 bg-slate-900/50 backdrop-blur-xl sticky top-0 z-50">
                <div className="container mx-auto px-4 py-4 flex items-center justify-between">
                    <Link href="/" className="text-2xl font-bold bg-gradient-to-r from-cyan-400 to-purple-500 bg-clip-text text-transparent">
                        QRit
                    </Link>
                    <nav className="hidden md:flex items-center gap-6">
                        <Link href="/" className="text-slate-300 hover:text-white transition-colors">Generator</Link>
                        <Link href="/pricing" className="text-white font-medium">Pricing</Link>
                        <Link href="/login" className="text-slate-300 hover:text-white transition-colors">Sign In</Link>
                        <Link href="/register">
                            <Button className="bg-gradient-to-r from-cyan-500 to-purple-500 hover:opacity-90">
                                Get Started
                            </Button>
                        </Link>
                    </nav>
                    {/* Mobile hamburger */}
                    <button
                        onClick={() => setMobileNavOpen(!mobileNavOpen)}
                        className="md:hidden w-10 h-10 flex items-center justify-center rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                        aria-label="Toggle navigation"
                    >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                        </svg>
                    </button>
                </div>
                {/* Mobile Nav Dropdown */}
                {mobileNavOpen && (
                    <div className="md:hidden border-t border-white/10 bg-slate-900 px-4 py-3 space-y-1">
                        <Link href="/" onClick={() => setMobileNavOpen(false)} className="block px-4 py-3 rounded-xl text-slate-300 hover:bg-slate-800 hover:text-white transition-colors">Generator</Link>
                        <Link href="/pricing" onClick={() => setMobileNavOpen(false)} className="block px-4 py-3 rounded-xl text-white font-medium bg-slate-800/50">Pricing</Link>
                        <Link href="/login" onClick={() => setMobileNavOpen(false)} className="block px-4 py-3 rounded-xl text-slate-300 hover:bg-slate-800 hover:text-white transition-colors">Sign In</Link>
                        <Link href="/register" onClick={() => setMobileNavOpen(false)} className="block px-4 py-3 rounded-xl bg-gradient-to-r from-cyan-500 to-purple-500 text-white font-semibold text-center">Get Started</Link>
                    </div>
                )}
            </header>

            <main className="container mx-auto px-4 py-16">
                {/* Hero */}
                <div className="text-center mb-16">
                    <h1 className="text-4xl md:text-5xl font-bold text-white mb-4">
                        Simple, Transparent Pricing
                    </h1>
                    <p className="text-xl text-slate-400 max-w-2xl mx-auto mb-8">
                        Start free, upgrade when you need more. No hidden fees, cancel anytime.
                    </p>

                    {/* Billing Toggle */}
                    <div className="inline-flex items-center gap-4 p-1 bg-slate-800 rounded-full">
                        <button
                            onClick={() => setBillingPeriod("monthly")}
                            className={cn(
                                "px-6 py-2 rounded-full text-sm font-medium transition-all",
                                billingPeriod === "monthly"
                                    ? "bg-gradient-to-r from-cyan-500 to-purple-500 text-white"
                                    : "text-slate-400 hover:text-white"
                            )}
                        >
                            Monthly
                        </button>
                        <button
                            onClick={() => setBillingPeriod("yearly")}
                            className={cn(
                                "px-6 py-2 rounded-full text-sm font-medium transition-all flex items-center gap-2",
                                billingPeriod === "yearly"
                                    ? "bg-gradient-to-r from-cyan-500 to-purple-500 text-white"
                                    : "text-slate-400 hover:text-white"
                            )}
                        >
                            Yearly
                            <span className="px-2 py-0.5 bg-green-500/20 text-green-400 text-xs rounded-full">
                                Save 20%
                            </span>
                        </button>
                    </div>
                </div>

                {/* Plans Grid */}
                <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6 mb-20">
                    {plans.map((plan) => (
                        <div
                            key={plan.name}
                            className={cn(
                                "relative rounded-2xl p-6 transition-all",
                                plan.popular
                                    ? "bg-gradient-to-b from-purple-900/50 to-slate-800/50 border-2 border-purple-500 scale-105"
                                    : "bg-slate-800/50 border border-white/10"
                            )}
                        >
                            {plan.popular && (
                                <div className="absolute -top-4 left-1/2 -translate-x-1/2">
                                    <span className="px-4 py-1 bg-gradient-to-r from-purple-500 to-pink-500 text-white text-sm font-semibold rounded-full shadow-lg">
                                        Most Popular
                                    </span>
                                </div>
                            )}

                            <div className="mb-6">
                                <h3 className="text-xl font-semibold text-white mb-1">{plan.name}</h3>
                                <p className="text-slate-400 text-sm">{plan.description}</p>
                            </div>

                            <div className="mb-6">
                                <div className="flex items-baseline gap-1">
                                    <span className="text-4xl font-bold text-white">{getPrice(plan)}</span>
                                    <span className="text-slate-400">/{getPeriod(plan)}</span>
                                </div>
                            </div>

                            <Button
                                onClick={() => handleCheckout(plan.name, plan.href)}
                                disabled={loadingPlan === plan.name}
                                className={cn(
                                    "w-full mb-6",
                                    plan.popular
                                        ? "bg-gradient-to-r from-purple-500 to-pink-500 hover:opacity-90"
                                        : plan.name === "Free"
                                            ? "bg-slate-700 hover:bg-slate-600"
                                            : "bg-gradient-to-r from-cyan-500 to-purple-500 hover:opacity-90"
                                )}
                            >
                                {loadingPlan === plan.name ? "Redirecting..." : plan.cta}
                            </Button>

                            <ul className="space-y-3">
                                {plan.features.map((feature) => (
                                    <li key={feature} className="flex items-start gap-2 text-sm text-slate-300">
                                        <svg className="w-5 h-5 text-green-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                                        </svg>
                                        {feature}
                                    </li>
                                ))}
                                {plan.notIncluded.map((feature) => (
                                    <li key={feature} className="flex items-start gap-2 text-sm text-slate-500">
                                        <svg className="w-5 h-5 text-slate-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                        </svg>
                                        {feature}
                                    </li>
                                ))}
                            </ul>
                        </div>
                    ))}
                </div>

                {/* Feature Comparison */}
                <div className="mb-20">
                    <h2 className="text-2xl sm:text-3xl font-bold text-white text-center mb-8">Compare Features</h2>
                    <div className="overflow-x-auto -mx-4 px-4">
                        <table className="w-full">
                            <thead>
                                <tr className="border-b border-white/10">
                                    <th className="text-left py-4 px-4 text-slate-400 font-medium">Feature</th>
                                    <th className="text-center py-4 px-4 text-slate-400 font-medium">Free</th>
                                    <th className="text-center py-4 px-4 text-slate-400 font-medium">Starter</th>
                                    <th className="text-center py-4 px-4 text-slate-400 font-medium bg-purple-900/20">Pro</th>
                                    <th className="text-center py-4 px-4 text-slate-400 font-medium">Enterprise</th>
                                </tr>
                            </thead>
                            <tbody>
                                {[
                                    { feature: "Static QR codes", free: "10/day", starter: "Unlimited", pro: "Unlimited", enterprise: "Unlimited" },
                                    { feature: "Dynamic QR codes", free: "—", starter: "10", pro: "100", enterprise: "Unlimited" },
                                    { feature: "QR types", free: "Basic", starter: "All", pro: "All", enterprise: "All" },
                                    { feature: "Color customization", free: "✓", starter: "✓", pro: "✓", enterprise: "✓" },
                                    { feature: "Logo overlay", free: "—", starter: "✓", pro: "✓", enterprise: "✓" },
                                    { feature: "SVG export", free: "—", starter: "✓", pro: "✓", enterprise: "✓" },
                                    { feature: "PDF export", free: "—", starter: "—", pro: "✓", enterprise: "✓" },
                                    { feature: "Analytics", free: "—", starter: "Basic", pro: "Advanced", enterprise: "Advanced" },
                                    { feature: "API access", free: "—", starter: "—", pro: "1,000/day", enterprise: "10,000+/day" },
                                    { feature: "Team members", free: "1", starter: "1", pro: "5", enterprise: "Unlimited" },
                                    { feature: "Support", free: "Community", starter: "Email", pro: "Priority", enterprise: "Dedicated" },
                                ].map((row) => (
                                    <tr key={row.feature} className="border-b border-white/5 hover:bg-white/5">
                                        <td className="py-4 px-4 text-white">{row.feature}</td>
                                        <td className="text-center py-4 px-4 text-slate-400">{row.free}</td>
                                        <td className="text-center py-4 px-4 text-slate-400">{row.starter}</td>
                                        <td className="text-center py-4 px-4 text-white bg-purple-900/10">{row.pro}</td>
                                        <td className="text-center py-4 px-4 text-slate-400">{row.enterprise}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>

                {/* FAQ */}
                <div className="max-w-3xl mx-auto">
                    <h2 className="text-3xl font-bold text-white text-center mb-8">Frequently Asked Questions</h2>
                    <div className="space-y-4">
                        {faqs.map((faq, i) => (
                            <div key={i} className="bg-slate-800/50 rounded-xl border border-white/10 overflow-hidden">
                                <button
                                    onClick={() => setExpandedFaq(expandedFaq === i ? null : i)}
                                    className="w-full px-6 py-4 text-left flex items-center justify-between"
                                >
                                    <span className="font-medium text-white">{faq.q}</span>
                                    <svg
                                        className={cn(
                                            "w-5 h-5 text-slate-400 transition-transform",
                                            expandedFaq === i && "rotate-180"
                                        )}
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                    >
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
                                    </svg>
                                </button>
                                {expandedFaq === i && (
                                    <div className="px-6 pb-4 text-slate-400">
                                        {faq.a}
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>
                </div>

                {/* CTA */}
                <div className="mt-20 text-center">
                    <div className="bg-gradient-to-r from-purple-900/50 to-cyan-900/50 rounded-3xl p-6 sm:p-12 border border-purple-500/30">
                        <h2 className="text-3xl font-bold text-white mb-4">Ready to get started?</h2>
                        <p className="text-slate-300 mb-8 max-w-xl mx-auto">
                            Join thousands of businesses using QRit to connect with their customers.
                        </p>
                        <div className="flex flex-wrap justify-center gap-4">
                            <Link href="/">
                                <Button size="lg" className="bg-white text-purple-900 hover:bg-slate-100 px-8">
                                    Try Free
                                </Button>
                            </Link>
                            <Link href="/contact">
                                <Button size="lg" variant="outline" className="border-white/30 text-white hover:bg-white/10 px-8">
                                    Contact Sales
                                </Button>
                            </Link>
                        </div>
                    </div>
                </div>
            </main>

            {/* Footer */}
            <footer className="border-t border-white/10 bg-slate-900/50 mt-20">
                <div className="container mx-auto px-4 py-12">
                    <div className="flex flex-col md:flex-row justify-between items-center gap-4">
                        <span className="text-xl font-bold bg-gradient-to-r from-cyan-400 to-purple-500 bg-clip-text text-transparent">
                            QRit
                        </span>
                        <div className="flex gap-6 text-slate-400 text-sm">
                            <Link href="/privacy" className="hover:text-white">Privacy</Link>
                            <Link href="/terms" className="hover:text-white">Terms</Link>
                            <Link href="/contact" className="hover:text-white">Contact</Link>
                        </div>
                        <div className="text-slate-500 text-sm">
                            © 2026 QRit. All rights reserved.
                        </div>
                    </div>
                </div>
            </footer>
        </div>
    );
}
