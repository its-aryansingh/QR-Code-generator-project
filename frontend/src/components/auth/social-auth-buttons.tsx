"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { startOAuthSignIn } from "@/lib/oauth";
import type { OAuthProvider, OAuthProviders } from "@/types";

interface SocialAuthButtonsProps {
  mode?: "login" | "register";
  /** Where to land after signing in (same-site path). */
  next?: string;
}

function LoadingSpinner({ className = "" }: { className?: string }) {
  return (
    <svg className={`animate-spin h-5 w-5 ${className}`} viewBox="0 0 24 24">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  );
}

export function GoogleIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
    </svg>
  );
}

export function GithubIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" className="text-white">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  );
}

export function SocialAuthButtons({ mode = "login", next = "/dashboard" }: SocialAuthButtonsProps) {
  const actionText = mode === "login" ? "Sign in" : "Sign up";
  const [providers, setProviders] = useState<OAuthProviders | null>(null);
  const [pending, setPending] = useState<OAuthProvider | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .oauthProviders()
      .then((res) => {
        if (!cancelled) setProviders(res.success && res.data ? res.data : { google: false, github: false });
      })
      .catch(() => {
        if (!cancelled) setProviders({ google: false, github: false });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Coming back to this page with the browser's Back button restores it from
  // the bfcache with the spinner still showing; reset it.
  useEffect(() => {
    const reset = (event: PageTransitionEvent) => {
      if (event.persisted) setPending(null);
    };
    window.addEventListener("pageshow", reset);
    return () => window.removeEventListener("pageshow", reset);
  }, []);

  const go = (provider: OAuthProvider) => {
    setPending(provider);
    startOAuthSignIn(provider, next);
  };

  // Until we know which providers the server supports, reserve the space so
  // the form below does not jump.
  if (providers === null) {
    return <div className="mb-6 h-[124px] animate-pulse rounded-xl bg-zinc-900/60" aria-hidden />;
  }
  if (!providers.google && !providers.github) {
    return null;
  }

  return (
    <div className="space-y-3">
      {providers.google && (
        <button
          type="button"
          onClick={() => go("google")}
          disabled={pending !== null}
          className="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-xl bg-white hover:bg-gray-50 text-gray-700 font-medium text-sm transition-all duration-200 shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {pending === "google" ? <LoadingSpinner className="text-gray-600" /> : <GoogleIcon />}
          {actionText} with Google
        </button>
      )}

      {providers.github && (
        <button
          type="button"
          onClick={() => go("github")}
          disabled={pending !== null}
          className="w-full flex items-center justify-center gap-3 px-4 py-3 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-100 font-medium text-sm transition-all duration-200 border border-zinc-700 hover:border-zinc-600 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {pending === "github" ? <LoadingSpinner className="text-zinc-300" /> : <GithubIcon />}
          {actionText} with GitHub
        </button>
      )}

      {/* Divider */}
      <div className="relative my-6">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-zinc-800" />
        </div>
        <div className="relative flex justify-center text-xs">
          <span className="px-4 bg-zinc-950 text-zinc-500 uppercase tracking-widest font-medium">
            or continue with email
          </span>
        </div>
      </div>
    </div>
  );
}
