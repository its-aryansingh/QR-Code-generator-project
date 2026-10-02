"use client";

import { useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { ArrowLeft, ArrowRight, Mail } from "lucide-react";
import { api } from "@/lib/api";

export default function ForgotPasswordPage() {
    const [isLoading, setIsLoading] = useState(false);
    const [email, setEmail] = useState("");
    const [submitted, setSubmitted] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);

        try {
            const response = await api.forgotPassword(email);
            if (response.success) {
                setSubmitted(true);
            } else {
                toast.error("Request failed", { description: response.error || "Please try again" });
            }
        } catch {
            // Always show success to prevent email enumeration (matching backend behavior)
            setSubmitted(true);
        } finally {
            setIsLoading(false);
        }
    };

    if (submitted) {
        return (
            <>
                {/* Mobile logo */}
                <div className="lg:hidden flex items-center gap-3 mb-8">
                    <div className="w-10 h-10 rounded-xl bg-white flex items-center justify-center">
                        <svg className="w-6 h-6 text-zinc-950" fill="currentColor" viewBox="0 0 24 24">
                            <rect x="2" y="2" width="6" height="6" rx="0.5" />
                            <rect x="4" y="4" width="2" height="2" fill="white" />
                            <rect x="16" y="2" width="6" height="6" rx="0.5" />
                            <rect x="18" y="4" width="2" height="2" fill="white" />
                            <rect x="2" y="16" width="6" height="6" rx="0.5" />
                            <rect x="4" y="18" width="2" height="2" fill="white" />
                            <rect x="10" y="2" width="2" height="2" />
                            <rect x="10" y="10" width="4" height="2" />
                            <rect x="16" y="16" width="2" height="2" />
                            <rect x="20" y="16" width="2" height="6" />
                        </svg>
                    </div>
                    <span className="text-xl font-bold text-white tracking-tight">QRit</span>
                </div>

                {/* Success State */}
                <div className="text-center">
                    <div className="w-16 h-16 rounded-2xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center mx-auto mb-6">
                        <Mail className="w-8 h-8 text-violet-400" />
                    </div>
                    <h1 className="text-3xl font-bold text-white mb-3">Check your email</h1>
                    <p className="text-zinc-400 mb-2">
                        We sent a password reset link to
                    </p>
                    <p className="text-white font-medium mb-6">{email}</p>
                    <p className="text-zinc-500 text-sm mb-8">
                        Didn't receive the email? Check your spam folder, or{" "}
                        <button
                            onClick={() => setSubmitted(false)}
                            className="text-violet-400 hover:text-violet-300 transition-colors"
                        >
                            try another email address
                        </button>
                    </p>
                </div>

                <Link
                    href="/login"
                    className="flex items-center justify-center gap-2 text-sm text-zinc-400 hover:text-zinc-300 transition-colors"
                >
                    <ArrowLeft className="w-4 h-4" />
                    Back to sign in
                </Link>
            </>
        );
    }

    return (
        <>
            {/* Mobile logo */}
            <div className="lg:hidden flex items-center gap-3 mb-8">
                <div className="w-10 h-10 rounded-xl bg-white flex items-center justify-center">
                    <svg className="w-6 h-6 text-zinc-950" fill="currentColor" viewBox="0 0 24 24">
                        <rect x="2" y="2" width="6" height="6" rx="0.5" />
                        <rect x="4" y="4" width="2" height="2" fill="white" />
                        <rect x="16" y="2" width="6" height="6" rx="0.5" />
                        <rect x="18" y="4" width="2" height="2" fill="white" />
                        <rect x="2" y="16" width="6" height="6" rx="0.5" />
                        <rect x="4" y="18" width="2" height="2" fill="white" />
                        <rect x="10" y="2" width="2" height="2" />
                        <rect x="10" y="10" width="4" height="2" />
                        <rect x="16" y="16" width="2" height="2" />
                        <rect x="20" y="16" width="2" height="6" />
                    </svg>
                </div>
                <span className="text-xl font-bold text-white tracking-tight">QRit</span>
            </div>

            {/* Header */}
            <div className="mb-8">
                <h1 className="text-3xl font-bold text-white mb-2">Forgot password?</h1>
                <p className="text-zinc-400">
                    No worries, we'll send you a reset link
                </p>
            </div>

            {/* Form */}
            <form onSubmit={handleSubmit} className="space-y-5">
                <div className="space-y-2">
                    <label htmlFor="email" className="text-sm font-medium text-zinc-300">
                        Email address
                    </label>
                    <input
                        id="email"
                        type="email"
                        placeholder="you@example.com"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        required
                        autoComplete="email"
                        autoFocus
                        className="w-full h-11 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 px-4 text-sm outline-none transition-all duration-200 focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20"
                    />
                </div>

                <button
                    type="submit"
                    disabled={isLoading || !email}
                    className="w-full h-11 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 hover:from-violet-500 hover:to-violet-400 text-white font-semibold text-sm transition-all duration-200 shadow-lg shadow-violet-500/20 hover:shadow-violet-500/30 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                    {isLoading ? (
                        <span className="flex items-center gap-2">
                            <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            Sending...
                        </span>
                    ) : (
                        <>
                            Send Reset Link
                            <ArrowRight className="w-4 h-4" />
                        </>
                    )}
                </button>
            </form>

            {/* Footer */}
            <p className="mt-8 text-sm text-center">
                <Link
                    href="/login"
                    className="flex items-center justify-center gap-2 text-zinc-400 hover:text-zinc-300 transition-colors"
                >
                    <ArrowLeft className="w-4 h-4" />
                    Back to sign in
                </Link>
            </p>
        </>
    );
}
