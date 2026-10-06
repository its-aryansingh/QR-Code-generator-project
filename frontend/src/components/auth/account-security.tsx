"use client";

import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import { toast } from "sonner";
import { Check, Eye, EyeOff, Link2, Shield, Unlink } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";
import { PROVIDER_LABELS, submitOAuthLink } from "@/lib/oauth";
import { GithubIcon, GoogleIcon } from "@/components/auth/social-auth-buttons";
import type { ConnectedAccounts, OAuthProvider } from "@/types";

const inputCls =
  "w-full bg-zinc-800 border border-zinc-700 rounded-lg px-4 py-2.5 text-sm text-white placeholder:text-zinc-600 focus:outline-none focus:border-violet-500 transition-colors";
const labelCls = "text-sm text-zinc-400 block mb-1.5 font-medium";
const PROVIDERS: OAuthProvider[] = ["google", "github"];

function errorText(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    const message = err.response?.data?.error;
    if (typeof message === "string" && message) return message;
  }
  return fallback;
}

function PasswordField({
  id, label, value, onChange, autoComplete, placeholder,
}: {
  id: string; label: string; value: string; onChange: (v: string) => void;
  autoComplete: string; placeholder?: string;
}) {
  const [show, setShow] = useState(false);
  return (
    <div>
      <label htmlFor={id} className={labelCls}>{label}</label>
      <div className="relative">
        <input
          id={id}
          type={show ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete={autoComplete}
          placeholder={placeholder}
          className={inputCls + " pr-10"}
        />
        <button
          type="button"
          onClick={() => setShow(!show)}
          aria-label={show ? "Hide password" : "Show password"}
          className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300"
        >
          {show ? <EyeOff size={16} /> : <Eye size={16} />}
        </button>
      </div>
    </div>
  );
}

/** Password (change or first-time set) and connected Google/GitHub accounts. */
export function AccountSecurity() {
  const setTokens = useAuthStore((s) => s.setTokens);
  const [data, setData] = useState<ConnectedAccounts | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [savingPassword, setSavingPassword] = useState(false);
  const [passwordSaved, setPasswordSaved] = useState(false);
  const [busy, setBusy] = useState<OAuthProvider | null>(null);

  const load = useCallback(async () => {
    try {
      const res = await api.oauthAccounts();
      if (res.success && res.data) setData(res.data);
      else setLoadError(true);
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const hasPassword = data?.has_password ?? true;

  const savePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingPassword(true);
    setPasswordSaved(false);
    try {
      const res = await api.changePassword(next, hasPassword ? current : undefined);
      if (res.success && res.data) {
        // The backend signs out every other session and hands this one new tokens.
        setTokens(res.data);
        setCurrent("");
        setNext("");
        setPasswordSaved(true);
        setTimeout(() => setPasswordSaved(false), 3000);
        toast.success(hasPassword ? "Password updated" : "Password set", {
          description: hasPassword
            ? "Other sessions have been signed out."
            : "You can now also sign in with your email and password.",
        });
        await load();
      } else {
        toast.error("Couldn't update password", { description: res.error });
      }
    } catch (err) {
      toast.error("Couldn't update password", { description: errorText(err, "Please try again.") });
    } finally {
      setSavingPassword(false);
    }
  };

  const connect = async (provider: OAuthProvider) => {
    setBusy(provider);
    try {
      const res = await api.oauthLink(provider);
      if (res.success && res.data) {
        submitOAuthLink(res.data.start_url, res.data.intent, "/dashboard/profile");
        return; // the page is navigating away
      }
      toast.error(`Couldn't connect ${PROVIDER_LABELS[provider]}`, { description: res.error });
    } catch (err) {
      toast.error(`Couldn't connect ${PROVIDER_LABELS[provider]}`, { description: errorText(err, "Please try again.") });
    }
    setBusy(null);
  };

  const disconnect = async (provider: OAuthProvider) => {
    if (!confirm(`Disconnect ${PROVIDER_LABELS[provider]}? You won't be able to sign in with it until you connect it again.`)) return;
    setBusy(provider);
    try {
      const res = await api.oauthUnlink(provider);
      if (res.success && res.data) {
        setData(res.data);
        toast.success(`${PROVIDER_LABELS[provider]} disconnected`);
      }
    } catch (err) {
      toast.error(`Couldn't disconnect ${PROVIDER_LABELS[provider]}`, { description: errorText(err, "Please try again.") });
    } finally {
      setBusy(null);
    }
  };

  const connected = new Map((data?.accounts ?? []).map((a) => [a.provider, a]));
  const visibleProviders = PROVIDERS.filter((p) => data?.providers[p] || connected.has(p));

  return (
    <>
      {/* Password */}
      <form onSubmit={savePassword} className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
        <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
          <Shield size={14} className="text-violet-400" /> {hasPassword ? "Password" : "Set a password"}
        </h2>
        {!hasPassword && (
          <p className="text-sm text-zinc-500">
            You sign in with {data?.accounts.map((a) => PROVIDER_LABELS[a.provider]).join(" or ") || "a connected account"}.
            Add a password to also sign in with your email address.
          </p>
        )}
        {hasPassword && (
          <PasswordField id="current-password" label="Current password" value={current} onChange={setCurrent}
            autoComplete="current-password" />
        )}
        <PasswordField id="new-password" label="New password" value={next} onChange={setNext}
          autoComplete="new-password" placeholder="8+ characters with upper, lower, number and symbol" />
        <div className="flex items-center gap-3">
          <button
            type="submit"
            disabled={savingPassword || !next || (hasPassword && !current)}
            className="px-5 py-2 bg-zinc-800 text-zinc-300 text-sm font-medium rounded-lg hover:bg-zinc-700 transition-colors disabled:opacity-50 border border-zinc-700"
          >
            {savingPassword ? "Saving…" : hasPassword ? "Update password" : "Set password"}
          </button>
          {passwordSaved && (
            <span className="text-sm text-emerald-400 flex items-center gap-1"><Check size={14} /> Saved</span>
          )}
        </div>
      </form>

      {/* Connected accounts */}
      <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
        <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
          <Link2 size={14} className="text-violet-400" /> Connected accounts
        </h2>
        {loadError && <p className="text-sm text-red-400">Couldn&apos;t load connected accounts.</p>}
        {data && visibleProviders.length === 0 && (
          <p className="text-sm text-zinc-500">Google and GitHub sign-in aren&apos;t enabled on this server.</p>
        )}
        <ul className="divide-y divide-zinc-800">
          {visibleProviders.map((provider) => {
            const account = connected.get(provider);
            return (
              <li key={provider} className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
                <div className="flex items-center gap-3 min-w-0">
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-zinc-800 border border-zinc-700">
                    {provider === "google" ? <GoogleIcon /> : <GithubIcon />}
                  </span>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-white">{PROVIDER_LABELS[provider]}</p>
                    <p className="text-xs text-zinc-500 truncate">
                      {account ? account.email || "Connected" : "Not connected"}
                    </p>
                  </div>
                </div>
                {account ? (
                  <button
                    type="button"
                    onClick={() => disconnect(provider)}
                    disabled={busy !== null}
                    className="shrink-0 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border border-zinc-700 text-zinc-400 hover:text-red-300 hover:border-red-800 transition-colors disabled:opacity-50"
                  >
                    <Unlink size={12} /> Disconnect
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => connect(provider)}
                    disabled={busy !== null || !data?.providers[provider]}
                    className="shrink-0 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-white text-zinc-950 hover:bg-zinc-200 transition-colors disabled:opacity-50"
                  >
                    <Link2 size={12} /> {busy === provider ? "Redirecting…" : "Connect"}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      </div>
    </>
  );
}
