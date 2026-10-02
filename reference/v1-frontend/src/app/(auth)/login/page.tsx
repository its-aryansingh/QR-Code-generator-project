"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import axios from "axios";
import { toast } from "sonner";
import { ArrowRight } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";
import { SocialAuthButtons } from "@/components/auth/social-auth-buttons";
import { PasswordInput } from "@/components/auth/password-input";

const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "";

declare global {
    interface Window {
        google?: {
            accounts: {
                id: {
                    initialize: (config: Record<string, unknown>) => void;
                    prompt: () => void;
                };
            };
        };
    }
}

function LoginForm() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const setTokens = useAuthStore((state) => state.setTokens);
    const setReturnUrl = useAuthStore((state) => state.setReturnUrl);
    const returnUrl = useAuthStore((state) => state.returnUrl);
    const [isLoading, setIsLoading] = useState(false);
    const [isGoogleLoading, setIsGoogleLoading] = useState(false);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");

    // Capture returnUrl from query params
    useEffect(() => {
        const from = searchParams.get("returnUrl") || searchParams.get("from");
        if (from) {
            setReturnUrl(from);
        }
    }, [searchParams, setReturnUrl]);

    const navigateAfterAuth = useCallback(() => {
        const destination = returnUrl || "/dashboard";
        setReturnUrl(null);
        router.push(destination);
    }, [returnUrl, setReturnUrl, router]);

    const handleGoogleResponse = useCallback(async (response: { credential: string }) => {
        setIsGoogleLoading(true);
        try {
            const result = await api.googleLogin(response.credential);
            if (result.success && result.data) {
                setTokens(result.data);
                toast.success("Welcome!", { description: "Signed in with Google" });
                navigateAfterAuth();
            } else {
                toast.error("Google sign-in failed", { description: result.error || "Could not authenticate" });
            }
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : "An error occurred";
            toast.error("Google sign-in failed", { description: message });
        } finally {
            setIsGoogleLoading(false);
        }
    }, [setTokens, navigateAfterAuth]);

    useEffect(() => {
        if (!GOOGLE_CLIENT_ID || GOOGLE_CLIENT_ID.includes("YOUR_GOOGLE")) return;

        const script = document.createElement("script");
        script.src = "https://accounts.google.com/gsi/client";
        script.async = true;
        script.defer = true;
        script.onload = () => {
            window.google?.accounts.id.initialize({
                client_id: GOOGLE_CLIENT_ID,
                callback: handleGoogleResponse,
            });
        };
        document.head.appendChild(script);

        return () => {
            if (script.parentNode) {
                script.parentNode.removeChild(script);
            }
        };
    }, [handleGoogleResponse]);

    const handleGoogleClick = () => {
        if (!GOOGLE_CLIENT_ID || GOOGLE_CLIENT_ID.includes("YOUR_GOOGLE")) {
            toast.error("Google sign-in not configured", {
                description: "Set NEXT_PUBLIC_GOOGLE_CLIENT_ID in .env.local",
            });
            return;
        }
        window.google?.accounts.id.prompt();
    };

    const handleGithubClick = () => {
        toast.error("GitHub sign-in not configured", {
            description: "GitHub OAuth is coming soon",
        });
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);

        try {
            const response = await api.login({ email, password });

            if (response.success && response.data) {
                setTokens(response.data);
                toast.success("Welcome back!", { description: "Login successful" });
                navigateAfterAuth();
            } else {
                const error = response.error || "Invalid credentials";
                if (error.includes("locked")) {
                    toast.error("Account locked", { description: error, duration: 8000 });
                } else if (error.includes("Too many")) {
                    toast.error("Rate limited", { description: error, duration: 8000 });
                } else {
                    toast.error("Login failed", { description: error });
                }
            }
        } catch (error: unknown) {
            if (axios.isAxiosError(error)) {
                const data = error.response?.data;
                const message = data?.error || "An error occurred";
                if (error.response?.status === 423) {
                    toast.error("Account locked", { description: message, duration: 8000 });
                } else if (error.response?.status === 429) {
                    toast.error("Too many attempts", { description: message, duration: 8000 });
                } else {
                    toast.error("Login failed", { description: message });
                }
            } else {
                const message = error instanceof Error ? error.message : "An error occurred";
                toast.error("Login failed", { description: message });
            }
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <>
            {/* Mobile logo - only shown on small screens where brand panel is hidden */}
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
                <h1 className="text-3xl font-bold text-white mb-2">Welcome back</h1>
                <p className="text-zinc-400">
                    Sign in to access your QR dashboard
                </p>
            </div>

            {/* Social Auth Buttons */}
            <SocialAuthButtons
                onGoogleClick={handleGoogleClick}
                onGithubClick={handleGithubClick}
                isGoogleLoading={isGoogleLoading}
                mode="login"
            />

            {/* Email/Password Form */}
            <form onSubmit={handleSubmit} className="space-y-5">
                {/* Email */}
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
                        className="w-full h-11 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 px-4 text-sm outline-none transition-all duration-200 focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20"
                    />
                </div>

                {/* Password */}
                <div className="space-y-2">
                    <div className="flex items-center justify-between">
                        <label htmlFor="password" className="text-sm font-medium text-zinc-300">
                            Password
                        </label>
                        <Link
                            href="/forgot-password"
                            className="text-xs text-violet-400 hover:text-violet-300 transition-colors"
                        >
                            Forgot password?
                        </Link>
                    </div>
                    <PasswordInput
                        id="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        autoComplete="current-password"
                    />
                </div>

                {/* Submit Button */}
                <button
                    type="submit"
                    disabled={isLoading || !email || !password}
                    className="w-full h-11 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 hover:from-violet-500 hover:to-violet-400 text-white font-semibold text-sm transition-all duration-200 shadow-lg shadow-violet-500/20 hover:shadow-violet-500/30 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                    {isLoading ? (
                        <span className="flex items-center gap-2">
                            <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                            Signing in...
                        </span>
                    ) : (
                        <>
                            Sign In
                            <ArrowRight className="w-4 h-4" />
                        </>
                    )}
                </button>
            </form>

            {/* Footer */}
            <p className="mt-8 text-sm text-center text-zinc-500">
                Don&apos;t have an account?{" "}
                <Link href="/register" className="text-violet-400 hover:text-violet-300 font-medium transition-colors">
                    Create one
                </Link>
            </p>
        </>
    );
}

export default function LoginPage() {
    return (
        <Suspense fallback={
            <div className="flex items-center justify-center">
                <svg className="animate-spin h-6 w-6 text-violet-400" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
            </div>
        }>
            <LoginForm />
        </Suspense>
    );
}
