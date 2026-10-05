"use client";

import { useEffect, useState, useRef, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";

function GitHubCallbackContent() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const setTokens = useAuthStore((state) => state.setTokens);
    const returnUrl = useAuthStore((state) => state.returnUrl);
    const setReturnUrl = useAuthStore((state) => state.setReturnUrl);
    const [statusMessage, setStatusMessage] = useState("Authenticating with GitHub...");
    const hasExecuted = useRef(false);

    useEffect(() => {
        if (hasExecuted.current) return;
        hasExecuted.current = true;

        const code = searchParams.get("code");
        const state = searchParams.get("state");
        const error = searchParams.get("error");
        const errorDescription = searchParams.get("error_description");

        if (error) {
            toast.error("GitHub authentication canceled or failed", {
                description: errorDescription || error,
            });
            router.push("/login");
            return;
        }

        if (!code) {
            toast.error("No authorization code provided", {
                description: "GitHub did not return a valid code",
            });
            router.push("/login");
            return;
        }

        // Verify CSRF state token if present
        const savedState = sessionStorage.getItem("github_oauth_state");
        if (savedState && state && savedState !== state) {
            toast.error("Security verification failed", {
                description: "OAuth state mismatch. Please try signing in again.",
            });
            router.push("/login");
            return;
        }
        sessionStorage.removeItem("github_oauth_state");

        const authCode: string = code;
        const redirectUri = `${window.location.origin}/callback/github`;

        async function exchangeCode() {
            try {
                setStatusMessage("Exchanging code with QRit backend...");
                const response = await api.githubLogin(authCode, redirectUri);

                if (response.success && response.data) {
                    setTokens(response.data);
                    toast.success("Welcome!", {
                        description: "Successfully signed in with GitHub",
                    });
                    const destination = returnUrl || "/dashboard";
                    setReturnUrl(null);
                    router.push(destination);
                } else {
                    const msg = response.error || "Authentication failed";
                    toast.error("GitHub sign-in failed", { description: msg });
                    router.push("/login");
                }
            } catch (err: unknown) {
                const message = err instanceof Error ? err.message : "An unexpected error occurred";
                toast.error("GitHub sign-in failed", { description: message });
                router.push("/login");
            }
        }

        exchangeCode();
    }, [searchParams, router, setTokens, returnUrl, setReturnUrl]);

    return (
        <div className="min-h-screen flex items-center justify-center bg-zinc-950 px-4">
            <div className="max-w-md w-full text-center space-y-6 p-8 rounded-2xl bg-zinc-900/60 border border-zinc-800/80 backdrop-blur-xl shadow-2xl">
                <div className="flex justify-center">
                    <div className="w-14 h-14 rounded-2xl bg-zinc-800 flex items-center justify-center animate-pulse">
                        <svg className="w-7 h-7 text-white" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                        </svg>
                    </div>
                </div>
                <div className="space-y-2">
                    <h2 className="text-xl font-semibold text-zinc-100">Connecting to GitHub</h2>
                    <p className="text-sm text-zinc-400">{statusMessage}</p>
                </div>
                <div className="flex justify-center">
                    <svg className="animate-spin h-6 w-6 text-zinc-500" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                </div>
            </div>
        </div>
    );
}

export default function GitHubCallbackPage() {
    return (
        <Suspense
            fallback={
                <div className="min-h-screen flex items-center justify-center bg-zinc-950">
                    <div className="w-8 h-8 border-2 border-zinc-700 border-t-zinc-200 rounded-full animate-spin" />
                </div>
            }
        >
            <GitHubCallbackContent />
        </Suspense>
    );
}
