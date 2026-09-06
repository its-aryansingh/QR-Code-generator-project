"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { ArrowLeft, ArrowRight, CheckCircle2, XCircle } from "lucide-react";
import { api } from "@/lib/api";
import { PasswordInput } from "@/components/auth/password-input";
import { PasswordStrength, getPasswordScore } from "@/components/auth/password-strength";

function ResetPasswordForm() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const token = searchParams.get("token") || "";

    const [isLoading, setIsLoading] = useState(false);
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [success, setSuccess] = useState(false);
    const [error, setError] = useState("");

    const passwordScore = getPasswordScore(password);
    const isPasswordStrong = passwordScore >= 4;
    const passwordsMatch = password === confirmPassword;

    useEffect(() => {
        if (!token) {
            setError("No reset token found. Please request a new password reset link.");
        }
    }, [token]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!isPasswordStrong) {
            toast.error("Password too weak", { description: "Please meet at least 4 of the 5 requirements" });
            return;
        }
        if (!passwordsMatch) {
            toast.error("Passwords don't match");
            return;
        }

        setIsLoading(true);
        try {
            const response = await api.resetPassword(token, password);
            if (response.success) {
                setSuccess(true);
                toast.success("Password reset!", { description: "Redirecting to login..." });
                setTimeout(() => router.push("/login"), 3000);
            } else {
                setError(response.error || "Failed to reset password");
            }
        } catch (err: unknown) {
            const message = err && typeof err === "object" && "response" in err
                ? (err as { response?: { data?: { error?: string } } }).response?.data?.error || "Failed to reset password"
                : "An error occurred";
            setError(message);
        } finally {
            setIsLoading(false);
        }
    };

    if (success) {
        return (
            <>
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
                <div className="text-center">
                    <div className="w-16 h-16 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mx-auto mb-6">
                        <CheckCircle2 className="w-8 h-8 text-emerald-400" />
                    </div>
                    <h1 className="text-3xl font-bold text-white mb-3">Password reset!</h1>
                    <p className="text-zinc-400 mb-6">
                        Your password has been changed successfully. Redirecting you to sign in...
                    </p>
                    <Link
                        href="/login"
                        className="text-violet-400 hover:text-violet-300 font-medium transition-colors text-sm"
                    >
                        Go to sign in now
                    </Link>
                </div>
            </>
        );
    }

    if (error && !token) {
        return (
            <>
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
                <div className="text-center">
                    <div className="w-16 h-16 rounded-2xl bg-red-500/10 border border-red-500/20 flex items-center justify-center mx-auto mb-6">
                        <XCircle className="w-8 h-8 text-red-400" />
                    </div>
                    <h1 className="text-3xl font-bold text-white mb-3">Invalid reset link</h1>
                    <p className="text-zinc-400 mb-6">{error}</p>
                    <Link
                        href="/forgot-password"
                        className="text-violet-400 hover:text-violet-300 font-medium transition-colors text-sm"
                    >
                        Request a new reset link
                    </Link>
                </div>
            </>
        );
    }

    return (
        <>
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

            <div className="mb-8">
                <h1 className="text-3xl font-bold text-white mb-2">Set new password</h1>
                <p className="text-zinc-400">
                    Your new password must be different from previously used passwords
                </p>
            </div>

            {error && (
                <div className="mb-6 p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
                    {error}
                </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-5">
                <div className="space-y-2">
                    <label htmlFor="password" className="text-sm font-medium text-zinc-300">
                        New password
                    </label>
                    <PasswordInput
                        id="password"
                        value={password}
                        onChange={(e) => { setPassword(e.target.value); setError(""); }}
                        autoComplete="new-password"
                    />
                    <PasswordStrength password={password} />
                </div>

                <div className="space-y-2">
                    <label htmlFor="confirmPassword" className="text-sm font-medium text-zinc-300">
                        Confirm password
                    </label>
                    <PasswordInput
                        id="confirmPassword"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        autoComplete="new-password"
                    />
                    {confirmPassword && !passwordsMatch && (
                        <p className="text-xs text-red-400 mt-1">Passwords do not match</p>
                    )}
                </div>

                <button
                    type="submit"
                    disabled={isLoading || !isPasswordStrong || !passwordsMatch || !confirmPassword}
                    className="w-full h-11 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 hover:from-violet-500 hover:to-violet-400 text-white font-semibold text-sm transition-all duration-200 shadow-lg shadow-violet-500/20 hover:shadow-violet-500/30 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                    {isLoading ? (
                        <span className="flex items-center gap-2">
                            <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            Resetting...
                        </span>
                    ) : (
                        <>
                            Reset Password
                            <ArrowRight className="w-4 h-4" />
                        </>
                    )}
                </button>
            </form>

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

export default function ResetPasswordPage() {
    return (
        <Suspense fallback={
            <div className="flex items-center justify-center">
                <svg className="animate-spin h-6 w-6 text-violet-400" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
            </div>
        }>
            <ResetPasswordForm />
        </Suspense>
    );
}
