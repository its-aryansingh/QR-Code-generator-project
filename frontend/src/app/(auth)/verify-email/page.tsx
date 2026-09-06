"use client";

import { useState, useEffect, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { CheckCircle2, XCircle, Mail } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";

function VerifyEmailContent() {
    const searchParams = useSearchParams();
    const token = searchParams.get("token") || "";
    const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

    const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
    const [error, setError] = useState("");
    const [resending, setResending] = useState(false);

    useEffect(() => {
        if (!token) {
            setStatus("error");
            setError("No verification token found.");
            return;
        }

        const verify = async () => {
            try {
                const response = await api.verifyEmail(token);
                if (response.success) {
                    setStatus("success");
                } else {
                    setStatus("error");
                    setError(response.error || "Verification failed");
                }
            } catch {
                setStatus("error");
                setError("Invalid or expired verification link.");
            }
        };

        verify();
    }, [token]);

    const handleResend = async () => {
        setResending(true);
        try {
            const response = await api.resendVerification();
            if (response.success) {
                toast.success("Verification email sent!", { description: "Check your inbox" });
            } else {
                toast.error("Failed to send", { description: response.error || "Please try again" });
            }
        } catch {
            toast.error("Failed to send", { description: "Please try again later" });
        } finally {
            setResending(false);
        }
    };

    if (status === "loading") {
        return (
            <div className="text-center">
                <div className="w-16 h-16 rounded-2xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center mx-auto mb-6">
                    <svg className="animate-spin h-8 w-8 text-violet-400" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                </div>
                <h1 className="text-3xl font-bold text-white mb-3">Verifying your email...</h1>
                <p className="text-zinc-400">Please wait a moment</p>
            </div>
        );
    }

    if (status === "success") {
        return (
            <div className="text-center">
                <div className="w-16 h-16 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mx-auto mb-6">
                    <CheckCircle2 className="w-8 h-8 text-emerald-400" />
                </div>
                <h1 className="text-3xl font-bold text-white mb-3">Email verified!</h1>
                <p className="text-zinc-400 mb-6">Your email address has been verified successfully.</p>
                <Link
                    href={isAuthenticated ? "/dashboard" : "/login"}
                    className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 hover:from-violet-500 hover:to-violet-400 text-white font-semibold text-sm transition-all duration-200 shadow-lg shadow-violet-500/20"
                >
                    {isAuthenticated ? "Go to Dashboard" : "Sign In"}
                </Link>
            </div>
        );
    }

    return (
        <div className="text-center">
            <div className="w-16 h-16 rounded-2xl bg-red-500/10 border border-red-500/20 flex items-center justify-center mx-auto mb-6">
                <XCircle className="w-8 h-8 text-red-400" />
            </div>
            <h1 className="text-3xl font-bold text-white mb-3">Verification failed</h1>
            <p className="text-zinc-400 mb-6">{error}</p>
            {isAuthenticated && (
                <button
                    onClick={handleResend}
                    disabled={resending}
                    className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-100 font-medium text-sm transition-all border border-zinc-700 disabled:opacity-50 mb-4"
                >
                    <Mail className="w-4 h-4" />
                    {resending ? "Sending..." : "Resend verification email"}
                </button>
            )}
            <div>
                <Link
                    href="/login"
                    className="text-violet-400 hover:text-violet-300 font-medium transition-colors text-sm"
                >
                    Back to sign in
                </Link>
            </div>
        </div>
    );
}

export default function VerifyEmailPage() {
    return (
        <Suspense fallback={
            <div className="flex items-center justify-center">
                <svg className="animate-spin h-6 w-6 text-violet-400" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
            </div>
        }>
            <VerifyEmailContent />
        </Suspense>
    );
}
