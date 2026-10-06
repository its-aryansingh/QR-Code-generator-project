import type { OAuthProvider } from "@/types";
import { OAUTH_URL } from "./config";

export const PROVIDER_LABELS: Record<OAuthProvider, string> = {
  google: "Google",
  github: "GitHub",
};

export function providerLabel(provider?: string | null): string {
  return (provider && PROVIDER_LABELS[provider as OAuthProvider]) || "your provider";
}

/** Mirrors the backend rule: only same-site relative paths. */
export function safeNext(value?: string | null, fallback = "/dashboard"): string {
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return fallback;
  if (value.length > 512 || /[\u0000-\u001f\u007f]/.test(value)) return fallback;
  if (value.startsWith("/auth/callback")) return fallback;
  return value;
}

/** Human message for each error code the backend can send back. */
export function oauthErrorMessage(code: string | null | undefined, provider?: string | null): string {
  const name = providerLabel(provider);
  switch (code) {
    case "access_denied":
      return `You cancelled signing in with ${name}.`;
    case "invalid_state":
    case "state_expired":
      return "Your sign-in session expired or was started in another browser tab. Please try again.";
    case "email_unverified":
      return provider === "github"
        ? "Your GitHub account has no verified email address. Verify one in GitHub's email settings, then try again."
        : `Your ${name} email address isn't verified yet. Verify it with ${name}, then try again.`;
    case "email_missing":
      return `${name} didn't share an email address with us, so we can't create your account.`;
    case "exchange_failed":
    case "provider_error":
    case "invalid_token":
      return `${name} couldn't complete the sign-in. Please try again.`;
    case "provider_unavailable":
      return `${name} couldn't be reached. Please try again in a moment.`;
    case "not_configured":
      return `Sign-in with ${name} isn't set up on this server yet.`;
    case "unknown_provider":
      return "That sign-in method isn't supported.";
    case "account_in_use":
      return `That ${name} account is already connected to a different QRit account.`;
    case "provider_already_linked":
      return `Your QRit account is already connected to a different ${name} account.`;
    case "link_expired":
      return "The connect request expired. Please try again.";
    case "invalid_code":
      return "This sign-in link has expired or was already used. Please sign in again.";
    default:
      return "Something went wrong while signing you in. Please try again.";
  }
}

/** Sign in or sign up: a full-page navigation to the backend's start endpoint. */
export function startOAuthSignIn(provider: OAuthProvider, next = "/dashboard") {
  window.location.assign(`${OAUTH_URL}/${provider}/start?next=${encodeURIComponent(safeNext(next))}`);
}

/**
 * Connect a provider to the signed-in account. The short-lived intent is sent
 * in a form POST so it never lands in the address bar, history or logs.
 */
export function submitOAuthLink(startUrl: string, intent: string, next: string) {
  const form = document.createElement("form");
  form.method = "POST";
  form.action = startUrl;
  form.style.display = "none";
  for (const [name, value] of Object.entries({ intent, next: safeNext(next) })) {
    const input = document.createElement("input");
    input.type = "hidden";
    input.name = name;
    input.value = value;
    form.appendChild(input);
  }
  document.body.appendChild(form);
  form.submit();
}
