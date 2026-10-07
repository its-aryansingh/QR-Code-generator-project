"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import axios from "axios";
import { toast } from "sonner";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";
import {
  callbackErrorMessage,
  exchangeErrorMessage,
  isProvider,
  providerLabel,
  safeNext,
} from "@/lib/oauth";

type Status = { kind: "working" } | { kind: "error"; title: string; message: string };

/**
 * Google and GitHub send the browser here (the redirect URI registered with
 * them is `<site>/callback/google` or `<site>/callback/github`). The page hands
 * `code` and `state` to the backend, which checks `state` against the cookie
 * set when this browser started the sign-in, then returns the session tokens.
 */
function OAuthCallback() {
  const router = useRouter();
  const params = useSearchParams();
  const route = useParams<{ provider: string }>();
  const setTokens = useAuthStore((s) => s.setTokens);
  const setProfile = useAuthStore((s) => s.setProfile);
  const setReturnUrl = useAuthStore((s) => s.setReturnUrl);

  // Read the query once: the effect below wipes it from the address bar.
  const [query] = useState(() => ({
    provider: route?.provider ?? "",
    code: params.get("code"),
    state: params.get("state") ?? "",
    error: params.get("error"),
  }));
  const [status, setStatus] = useState<Status>(() => {
    if (!isProvider(query.provider)) {
      return { kind: "error", title: "We couldn't sign you in", message: "That sign-in method isn't supported." };
    }
    if (query.error) {
      return {
        kind: "error",
        title: query.error === "access_denied" ? "Sign-in cancelled" : "We couldn't sign you in",
        message: callbackErrorMessage(query.error, query.provider),
      };
    }
    if (!query.code) {
      return {
        kind: "error",
        title: "We couldn't sign you in",
        message: `${providerLabel(query.provider)} didn't send a sign-in code. Please try again.`,
      };
    }
    return { kind: "working" };
  });
  const started = useRef(false);

  useEffect(() => {
    // An authorization code is single-use; never send it twice (React strict
    // mode runs effects twice in development).
    if (started.current) return;
    started.current = true;

    // Keep the code out of history and out of any Referer header.
    window.history.replaceState(null, "", window.location.pathname);

    const { provider, code, state, error } = query;
    if (!isProvider(provider) || error || !code) return;

    api
      .oauthComplete(provider, code, state)
      .then((res) => {
        if (!res.success || !res.data) {
          throw new Error(res.error || "Sign-in failed");
        }
        setProfile(null); // the dashboard reloads it for the account that just signed in
        setTokens(res.data);
        setReturnUrl(null);
        toast.success("Welcome!", { description: `Signed in with ${providerLabel(provider)}.` });
        router.replace(safeNext(res.data.next));
      })
      .catch((err: unknown) => {
        const body = axios.isAxiosError(err) ? err.response?.data : undefined;
        const code = typeof body?.code === "string" ? body.code : undefined;
        const detail = typeof body?.error === "string" ? body.error : undefined;
        setStatus({
          kind: "error",
          title: "We couldn't sign you in",
          message: exchangeErrorMessage(
            axios.isAxiosError(err) && err.response?.status === 502 ? "provider_unavailable" : code,
            detail,
            provider,
          ),
        });
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

  return (
    <div className="space-y-6" role="alert">
      <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-red-900/60 bg-red-950/40">
        <AlertCircle className="h-6 w-6 text-red-400" />
      </div>
      <div>
        <h1 className="mb-2 text-2xl font-bold text-white">{status.title}</h1>
        <p className="text-zinc-400">{status.message}</p>
      </div>
      <Link
        href="/login"
        className="inline-flex h-11 items-center gap-2 rounded-xl bg-gradient-to-r from-violet-600 to-violet-500 px-5 text-sm font-semibold text-white shadow-lg shadow-violet-500/20 transition-all hover:from-violet-500 hover:to-violet-400"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to sign in
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
