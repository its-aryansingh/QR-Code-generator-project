"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";

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

export default function RegisterPage() {
    const router = useRouter();
    const setTokens = useAuthStore((state) => state.setTokens);
    const [isLoading, setIsLoading] = useState(false);
    const [isGoogleLoading, setIsGoogleLoading] = useState(false);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");

    const handleGoogleResponse = useCallback(async (response: { credential: string }) => {
        setIsGoogleLoading(true);
        try {
            const result = await api.googleLogin(response.credential);
            if (result.success && result.data) {
                setTokens(result.data);
                toast.success("Welcome!", { description: "Account created with Google" });
                router.push("/dashboard");
            } else {
                toast.error("Google sign-up failed", { description: result.error || "Could not authenticate" });
            }
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : "An error occurred";
            toast.error("Google sign-up failed", { description: message });
        } finally {
            setIsGoogleLoading(false);
        }
    }, [setTokens, router]);

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
            document.head.removeChild(script);
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

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (password !== confirmPassword) {
            toast.error("Passwords don't match");
            return;
        }

        setIsLoading(true);

        try {
            const response = await api.register({ email, password });

            if (response.success) {
                toast.success("Account created!", { description: "Please sign in to continue" });
                router.push("/login");
            } else {
                toast.error("Registration failed", { description: response.error || "Could not create account" });
            }
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : "An error occurred";
            toast.error("Registration failed", { description: message });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center p-4 bg-zinc-950">
            <Card className="w-full max-w-md relative z-10 bg-zinc-900 border-zinc-800">
                <CardHeader className="text-center">
                    <div className="mx-auto mb-4 w-12 h-12 rounded-xl bg-white flex items-center justify-center">
                        <svg className="w-7 h-7 text-zinc-950" fill="currentColor" viewBox="0 0 24 24">
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
                    <CardTitle className="text-2xl text-zinc-50">Create Account</CardTitle>
                    <CardDescription className="text-zinc-500">
                        Join QRit and start generating QR codes
                    </CardDescription>
                </CardHeader>

                {/* Google Sign-Up Button */}
                <div className="px-6 pb-2">
                    <button
                        type="button"
                        onClick={handleGoogleClick}
                        disabled={isGoogleLoading}
                        className="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-lg bg-white hover:bg-gray-50 text-gray-700 font-medium text-sm transition-all duration-300 shadow-lg shadow-black/10 hover:shadow-xl hover:shadow-black/20 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {isGoogleLoading ? (
                            <svg className="animate-spin h-5 w-5 text-gray-600" viewBox="0 0 24 24">
                                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                            </svg>
                        ) : (
                            <svg width="20" height="20" viewBox="0 0 24 24">
                                <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
                                <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
                                <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
                                <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
                            </svg>
                        )}
                        Continue with Google
                    </button>

                    {/* Divider */}
                    <div className="relative my-5">
                        <div className="absolute inset-0 flex items-center">
                            <div className="w-full border-t border-white/10" />
                        </div>
                        <div className="relative flex justify-center text-xs">
                            <span className="px-3 bg-zinc-900 text-zinc-600 uppercase tracking-wider">or</span>
                        </div>
                    </div>
                </div>

                <form onSubmit={handleSubmit}>
                    <CardContent className="space-y-4 pt-0">
                        <div className="space-y-2">
                            <Label htmlFor="email" className="text-zinc-400">Email</Label>
                            <Input
                                id="email"
                                type="email"
                                placeholder="you@example.com"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                required
                                className="bg-zinc-950 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-600 focus:ring-zinc-700"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="password" className="text-zinc-400">Password</Label>
                            <Input
                                id="password"
                                type="password"
                                placeholder="••••••••"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                required
                                minLength={8}
                                className="bg-zinc-950 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-600 focus:ring-zinc-700"
                            />
                            <p className="text-xs text-zinc-600">Must be at least 8 characters</p>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="confirmPassword" className="text-zinc-400">Confirm Password</Label>
                            <Input
                                id="confirmPassword"
                                type="password"
                                placeholder="••••••••"
                                value={confirmPassword}
                                onChange={(e) => setConfirmPassword(e.target.value)}
                                required
                                minLength={8}
                                className="bg-zinc-950 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-600 focus:ring-zinc-700"
                            />
                        </div>
                    </CardContent>
                    <CardFooter className="flex flex-col gap-4">
                        <Button
                            type="submit"
                            disabled={isLoading}
                            className="w-full bg-white hover:bg-zinc-200 text-zinc-900 font-semibold py-5 rounded-lg transition-all duration-200"
                        >
                            {isLoading ? (
                                <span className="flex items-center gap-2">
                                    <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                                    </svg>
                                    Creating account...
                                </span>
                            ) : (
                                "Create Account"
                            )}
                        </Button>
                        <p className="text-sm text-center text-zinc-500">
                            Already have an account?{" "}
                            <Link href="/login" className="text-violet-400 hover:text-violet-300 font-medium">
                                Sign in
                            </Link>
                        </p>
                    </CardFooter>
                </form>
            </Card>
        </div>
    );
}
