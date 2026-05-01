"use client";

import { useState, useEffect } from "react";
import { useAuthStore } from "@/lib/auth";
import {
  User, Mail, Building, Key, CreditCard, Shield,
  Save, Check, AlertCircle, Eye, EyeOff, Crown
} from "lucide-react";

const inputCls = "w-full bg-zinc-800 border border-zinc-700 rounded-lg px-4 py-2.5 text-sm text-white placeholder:text-zinc-600 focus:outline-none focus:border-violet-500 transition-colors";
const labelCls = "text-sm text-zinc-400 block mb-1.5 font-medium";

interface UserProfile {
  id: string;
  email: string;
  name: string;
  company: string;
  avatar_url: string;
  plan: string;
  subscription_status: string;
  subscription_ends_at?: string;
  plan_expires_at?: string;
  api_key?: string;
  api_calls_today: number;
  created_at: string;
}

const PLAN_COLORS: Record<string, string> = {
  free: "text-zinc-400 bg-zinc-800 border-zinc-700",
  starter: "text-cyan-400 bg-cyan-900/30 border-cyan-700",
  pro: "text-violet-400 bg-violet-900/30 border-violet-700",
  enterprise: "text-amber-400 bg-amber-900/30 border-amber-700",
};

export default function ProfilePage() {
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  // Editable fields
  const [name, setName] = useState("");
  const [company, setCompany] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");

  // Password change
  const [newPassword, setNewPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [passwordSaved, setPasswordSaved] = useState(false);

  // API key
  const [showApiKey, setShowApiKey] = useState(false);

  useEffect(() => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;
    fetch(`${api}/user/profile`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.json())
      .then((data) => {
        if (data.success) {
          const p = data.data;
          setProfile(p);
          setName(p.name || "");
          setCompany(p.company || "");
          setAvatarUrl(p.avatar_url || "");
        }
      })
      .catch(() => setError("Failed to load profile"))
      .finally(() => setLoading(false));
  }, [api]);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setSaved(false);
    const token = useAuthStore.getState().accessToken;
    try {
      const res = await fetch(`${api}/user/profile`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ name, company, avatar_url: avatarUrl }),
      });
      const data = await res.json();
      if (data.success) {
        setSaved(true);
        setTimeout(() => setSaved(false), 3000);
      } else {
        setError(data.error || "Failed to save");
      }
    } catch {
      setError("Network error");
    } finally {
      setSaving(false);
    }
  };

  const handlePasswordChange = async () => {
    if (!newPassword || newPassword.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    setSaving(true);
    setError("");
    const token = useAuthStore.getState().accessToken;
    try {
      const res = await fetch(`${api}/user/profile`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ password: newPassword }),
      });
      const data = await res.json();
      if (data.success) {
        setPasswordSaved(true);
        setNewPassword("");
        setTimeout(() => setPasswordSaved(false), 3000);
      } else {
        setError(data.error || "Failed to update password");
      }
    } catch {
      setError("Network error");
    } finally {
      setSaving(false);
    }
  };

  const handleCheckout = async (plan: string) => {
    const token = useAuthStore.getState().accessToken;
    try {
      const res = await fetch(`${api}/payments/checkout`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ plan, billing_period: "monthly" }),
      });
      const data = await res.json();
      if (data.success && data.checkout_url) {
        window.location.href = data.checkout_url;
      }
    } catch { /* Stripe may not be configured */ }
  };

  const handleCancelSubscription = async () => {
    if (!confirm("Cancel your subscription? You'll retain access until the end of the billing period.")) return;
    const token = useAuthStore.getState().accessToken;
    try {
      await fetch(`${api}/payments/cancel`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      window.location.reload();
    } catch { /* ignore */ }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-8 h-8 border-2 border-zinc-700 border-t-violet-500 rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-2xl space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-white">Profile</h1>
        <p className="text-zinc-500 mt-1">Manage your account settings and subscription</p>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-950/40 border border-red-800 rounded-lg text-sm text-red-400">
          <AlertCircle size={14} /> {error}
        </div>
      )}

      {/* Profile Info */}
      <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-5">
        <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
          <User size={14} className="text-violet-400" /> Account
        </h2>

        <div className="flex items-center gap-4">
          <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-violet-600 to-cyan-600 flex items-center justify-center text-2xl font-bold text-white shrink-0">
            {name ? name[0].toUpperCase() : profile?.email?.[0]?.toUpperCase() || "?"}
          </div>
          <div>
            <p className="text-white font-semibold">{name || profile?.email}</p>
            <p className="text-sm text-zinc-500">{profile?.email}</p>
            <p className="text-xs text-zinc-600 mt-1">Joined {profile?.created_at ? new Date(profile.created_at).toLocaleDateString() : ""}</p>
          </div>
        </div>

        <div className="grid sm:grid-cols-2 gap-4">
          <div>
            <label className={labelCls}>
              <span className="flex items-center gap-1.5"><User size={12} /> Display Name</span>
            </label>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" className={inputCls} />
          </div>
          <div>
            <label className={labelCls}>
              <span className="flex items-center gap-1.5"><Building size={12} /> Company</span>
            </label>
            <input value={company} onChange={(e) => setCompany(e.target.value)} placeholder="Acme Inc." className={inputCls} />
          </div>
        </div>

        <div>
          <label className={labelCls}>
            <span className="flex items-center gap-1.5"><Mail size={12} /> Email</span>
          </label>
          <input value={profile?.email || ""} disabled className={inputCls + " opacity-50 cursor-not-allowed"} />
        </div>

        <div>
          <label className={labelCls}>Avatar URL</label>
          <input value={avatarUrl} onChange={(e) => setAvatarUrl(e.target.value)} placeholder="https://example.com/avatar.png" className={inputCls} />
        </div>

        <div className="flex items-center gap-3">
          <button onClick={handleSave} disabled={saving}
            className="px-5 py-2 bg-white text-zinc-950 text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors disabled:opacity-50 flex items-center gap-2">
            {saving ? <div className="w-4 h-4 border-2 border-zinc-950 border-t-transparent rounded-full animate-spin" /> : <Save size={14} />}
            Save Profile
          </button>
          {saved && <span className="text-sm text-emerald-400 flex items-center gap-1"><Check size={14} /> Saved</span>}
        </div>
      </div>

      {/* Password */}
      <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
        <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
          <Shield size={14} className="text-violet-400" /> Security
        </h2>
        <div>
          <label className={labelCls}>New Password</label>
          <div className="relative">
            <input
              type={showPassword ? "text" : "password"}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Minimum 8 characters"
              className={inputCls + " pr-10"}
            />
            <button onClick={() => setShowPassword(!showPassword)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300">
              {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={handlePasswordChange} disabled={!newPassword || saving}
            className="px-5 py-2 bg-zinc-800 text-zinc-300 text-sm font-medium rounded-lg hover:bg-zinc-700 transition-colors disabled:opacity-50 border border-zinc-700">
            Update Password
          </button>
          {passwordSaved && <span className="text-sm text-emerald-400 flex items-center gap-1"><Check size={14} /> Updated</span>}
        </div>
      </div>

      {/* Subscription */}
      <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
        <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
          <CreditCard size={14} className="text-violet-400" /> Subscription
        </h2>
        <div className="flex items-center gap-3">
          <span className={`px-3 py-1 rounded-lg text-sm font-medium border capitalize ${PLAN_COLORS[profile?.plan || "free"]}`}>
            <Crown size={12} className="inline mr-1" />
            {profile?.plan || "free"}
          </span>
          {profile?.subscription_status && profile.subscription_status !== "" && (
            <span className={`text-xs px-2 py-0.5 rounded-full ${profile.subscription_status === "active"
              ? "bg-emerald-900/30 text-emerald-400 border border-emerald-700"
              : "bg-amber-900/30 text-amber-400 border border-amber-700"
            }`}>
              {profile.subscription_status}
            </span>
          )}
        </div>

        {profile?.subscription_ends_at && (
          <p className="text-sm text-zinc-500">
            {profile.subscription_status === "canceling" ? "Access until: " : "Renews: "}
            <span className="text-zinc-300">{new Date(profile.subscription_ends_at).toLocaleDateString()}</span>
          </p>
        )}

        <div className="flex flex-wrap gap-3 pt-2">
          {profile?.plan === "free" && (
            <>
              <button onClick={() => handleCheckout("starter")}
                className="px-4 py-2 bg-gradient-to-r from-cyan-600 to-violet-600 text-white text-sm font-medium rounded-lg hover:opacity-90 transition-opacity">
                Upgrade to Starter — $9/mo
              </button>
              <button onClick={() => handleCheckout("pro")}
                className="px-4 py-2 bg-gradient-to-r from-violet-600 to-pink-600 text-white text-sm font-medium rounded-lg hover:opacity-90 transition-opacity">
                Upgrade to Pro — $29/mo
              </button>
            </>
          )}
          {profile?.plan === "starter" && (
            <button onClick={() => handleCheckout("pro")}
              className="px-4 py-2 bg-gradient-to-r from-violet-600 to-pink-600 text-white text-sm font-medium rounded-lg hover:opacity-90 transition-opacity">
              Upgrade to Pro — $29/mo
            </button>
          )}
          {profile?.plan !== "free" && profile?.subscription_status !== "canceling" && (
            <button onClick={handleCancelSubscription}
              className="px-4 py-2 bg-zinc-800 text-zinc-400 text-sm rounded-lg hover:bg-zinc-700 border border-zinc-700 transition-colors">
              Cancel Subscription
            </button>
          )}
        </div>
      </div>

      {/* API Key */}
      {profile?.api_key && (
        <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
          <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider flex items-center gap-2">
            <Key size={14} className="text-violet-400" /> API Access
          </h2>
          <div>
            <label className={labelCls}>API Key</label>
            <div className="flex gap-2">
              <input
                value={showApiKey ? profile.api_key : "•".repeat(40)}
                readOnly
                className={inputCls + " font-mono text-xs"}
              />
              <button onClick={() => setShowApiKey(!showApiKey)}
                className="shrink-0 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-400 hover:text-white transition-colors text-sm">
                {showApiKey ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
          <p className="text-xs text-zinc-500">
            API calls today: <span className="text-zinc-300 font-medium">{profile.api_calls_today}</span>
          </p>
        </div>
      )}
    </div>
  );
}
