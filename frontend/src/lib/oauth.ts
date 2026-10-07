import type { OAuthProvider } from "@/types";
import { OAUTH_URL } from "./config";

export const PROVIDER_LABELS: Record<OAuthProvider, string> = {
  google: "Google",
  github: "GitHub",
};

export function isProvider(value: string | null | undefined): value is OAuthProvider {
  return value === "google" || value === "github";
}

export function providerLabel(provider?: string | null): string {
  return isProvider(provider) ? PROVIDER_LABELS[provider] : "your provider";
}

/** Mirrors the backend rule: only same-site relative paths. */
export function safeNext(value?: string | null, fallback = "/dashboard"): string {
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return fallback;
  if (value.length > 512 || /[\u0000-\u001f\u007f]/.test(value)) return fallback;
  if (value.startsWith("/callback/")) return fallback;
  return value;
}

/** Message for an error the provider or the start step put in the callback URL. */
export function callbackErrorMessage(code: string, provider?: string | null): string {
  const name = providerLabel(provider);
  switch (code) {
    case "access_denied":
      return `You cancelled signing in with ${name}.`;
    case "not_configured":
      return `Sign-in with ${name} isn't set up on this server yet.`;
    default:
      return `${name} couldn't complete the sign-in. Please try again.`;
  }
}

/** Message for an error code returned by POST /auth/<provider>. */
export function exchangeErrorMessage(code: string | undefined, detail: string | undefined, provider: string): string {
  const name = providerLabel(provider);
  switch (code) {
    case "invalid_state":
      return "Your sign-in expired or was started in another browser tab. Please try again.";
    case "provider_unavailable":
      return `${name} couldn't be reached. Please try again in a moment.`;
    case "internal_error":
      return `Sign-in with ${name} isn't set up on this server yet.`;
    default:
      return detail || `${name} couldn't complete the sign-in. Please try again.`;
  }
}

/** Full-page navigation to the backend, which redirects to the provider. */
export function startOAuthSignIn(provider: OAuthProvider, next = "/dashboard") {
  window.location.assign(`${OAUTH_URL}/${provider}/start?next=${encodeURIComponent(safeNext(next))}`);
}
