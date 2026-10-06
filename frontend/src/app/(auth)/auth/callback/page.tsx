"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import axios from "axios";
import { toast } from "sonner";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";
import { oauthErrorMessage, providerLabel, safeNext } from "@/lib/oauth";

type Status =
  | { kind: "working" }
  | { kind: "error"; code: string; provider: string | null; next: string };

/**
 * Landing page after Google/GitHub. The backend redirects here with either
 *   ?code=<one-time handoff code>&provider=…   (signed in: swap it for tokens)
 *   ?linked=<provider>&next=…                  (account connected from settings)
 *   ?error=<code>&provider=…&next=…            (show what went wrong)
 */
function OAuthCallback() {
  const router = useRouter();
  const params = useSearchParams();
  const setTokens = useAuthStore((s) => s.setTokens);
  const setProfile = useAuthStore((s) => s.setProfile);
  const setReturnUrl = useAuthStore((s) => s.setReturnUrl);
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  // Read the query once: the effect below wipes it from the address bar.
  const [query] = useState(() => ({
    code: params.get("code"),
    error: params.get("error"),
    linked: params.get("linked"),
    provider: params.get("provider"),
    next: safeNext(params.get("next")),
  }));
  const [status, setStatus] = useState<Status>(() => {
    if (query.error) return { kind: "error", code: query.error, provider: query.provider, next: query.next };
    if (!query.code && !query.linked) {
      return { kind: "error", code: "invalid_code", provider: query.provider, next: query.next };
    }
    return { kind: "working" };
  });
  const started = useRef(false);

  useEffect(() => {
    // The handoff code is single-use; never redeem it twice (React strict mode
    // runs effects twice in development).
    if (started.current) return;
    started.current = true;

    // Keep the code out of history and out of any Referer header.
    window.history.replaceState(null, "", "/auth/callback");

    const { code, linked, provider, next } = query;
    if (linked) {
      toast.success(`${providerLabel(linked)} connected`, {
        description: `You can now sign in with ${providerLabel(linked)}.`,
      });
      router.replace(safeNext(next, "/dashboard/profile"));
      return;
    }
    if (!code || query.error) return;

    api
      .oauthExchange(code)
      .then((res) => {
        if (!res.success || !res.data) {
          setStatus({ kind: "error", code: res.code || "invalid_code", provider, next });
          return;
        }
        const data = res.data;
        setProfile(null); // the dashboard reloads it for the account that just signed in
        setTokens(data);
        setReturnUrl(null);
        const name = providerLabel(data.provider);
        if (data.password_reset) {
          toast.success(`Signed in with ${name}`, {
            description:
              "This email had an unverified account with a password nobody had confirmed. That password was removed for your safety; you can set a new one in your profile.",
            duration: 10000,
          });
        } else if (data.is_new_user) {
          toast.success("Welcome to QRit!", { description: `Your account was created with ${name}.` });
        } else {
          toast.success("Welcome back!", { description: `Signed in with ${name}.` });
        }
        router.replace(safeNext(data.next));
      })
      .catch((err: unknown) => {
        const code =
          axios.isAxiosError(err) && typeof err.response?.data?.code === "string"
            ? err.response.data.code
            : axios.isAxiosError(err) && err.response?.status === 502
              ? "provider_unavailable"
              : "invalid_code";
        setStatus({ kind: "error", code, provider, next });
      });
  }, [query, router, setProfile, setReturnUrl, setTokens]);

  if (status.kind === "working") {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-16 text-center" role="status" aria-live="polite">
        <svg className="h-8 w-8 animate-spin text-violet-400" viewBox="0 0 24 24" aria-hidden>
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        <p className="text-sm text-zinc-400">Finishing sign-in…</p>
      </div>
    );
  }

  // A signed-in user who was connecting an account goes back to where they were.
  const backHref = isAuthenticated ? status.next : "/login";
  const backLabel = isAuthenticated ? "Back to QRit" : "Back to sign in";

  return (
    <div className="space-y-6" role="alert">
      <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-red-900/60 bg-red-950/40">
        <AlertCircle className="h-6 w-6 text-red-400" />
      </div>
      <div>
        <h1 className="mb-2 text-2xl font-bold text-white">
          {status.code === "access_denied" ? "Sign-in cancelled" : "We couldn't sign you in"}
        </h1>
        <p className="text-zinc-400">{oauthErrorMessage(status.code, status.provider)}</p>
      </div>
      <Link
        href={backHref}
        className="inline-flex h-11 items-center gap-2 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 px-5 text-sm font-semibold text-white shadow-lg shadow-violet-500/20 transition-all hover:from-violet-500 hover:to-violet-400"
      >
        <ArrowLeft className="h-4 w-4" />
        {backLabel}
      </Link>
    </div>
  );
}

export default function OAuthCallbackPage() {
  return (
    <Suspense fallback={null}>
      <OAuthCallback />
    </Suspense>
  );
}
