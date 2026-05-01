"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/auth";
import Link from "next/link";
import { Users, Shield, Clock, Check, AlertCircle, LogIn } from "lucide-react";

interface InviteInfo {
  workspace_name: string;
  workspace_logo?: string;
  role: string;
  invited_by: string;
  email: string;
  expires_at: string;
}

export default function InviteAcceptPage() {
  const params = useParams();
  const router = useRouter();
  const { accessToken, isAuthenticated } = useAuthStore();
  const token = params.token as string;
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState("");
  const [accepted, setAccepted] = useState(false);

  // Fetch invite info (public endpoint, no auth)
  useEffect(() => {
    const fetchInvite = async () => {
      try {
        const res = await fetch(`${api}/invites/${token}/info`);
        const data = await res.json();
        if (data.success) {
          setInvite(data.data);
        } else {
          setError(data.error || "Invalid or expired invite");
        }
      } catch {
        setError("Failed to load invite details");
      } finally {
        setLoading(false);
      }
    };
    fetchInvite();
  }, [token, api]);

  // Auto-accept if user just logged in and had a pending invite
  useEffect(() => {
    if (isAuthenticated && invite && !accepted && !accepting) {
      const pendingToken = localStorage.getItem("qrit_pending_invite");
      if (pendingToken === token) {
        localStorage.removeItem("qrit_pending_invite");
        handleAccept();
      }
    }
  }, [isAuthenticated, invite, accepted, accepting, token]);

  const handleAccept = async () => {
    if (!accessToken) {
      // Store token and redirect to login
      localStorage.setItem("qrit_pending_invite", token);
      router.push(`/login?redirect=/invite/${token}`);
      return;
    }

    setAccepting(true);
    setError("");

    try {
      const res = await fetch(`${api}/invites/${token}/accept`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      const data = await res.json();
      if (data.success) {
        setAccepted(true);
        // Redirect to workspace after 2s
        setTimeout(() => {
          if (data.workspace_id) {
            localStorage.setItem("qrit_active_workspace", data.workspace_id);
          }
          router.push("/dashboard");
        }, 2000);
      } else {
        setError(data.error || "Failed to accept invite");
      }
    } catch {
      setError("Network error. Please try again.");
    } finally {
      setAccepting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="w-10 h-10 border-2 border-zinc-700 border-t-violet-500 rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <Link href="/" className="text-2xl font-bold bg-gradient-to-r from-cyan-400 to-violet-500 bg-clip-text text-transparent">
            QRit
          </Link>
        </div>

        <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-8 shadow-xl">
          {/* Success state */}
          {accepted && (
            <div className="text-center space-y-4">
              <div className="w-16 h-16 rounded-full bg-emerald-900/30 border border-emerald-700 flex items-center justify-center mx-auto">
                <Check size={32} className="text-emerald-400" />
              </div>
              <h2 className="text-xl font-bold text-white">Welcome aboard!</h2>
              <p className="text-sm text-zinc-400">
                You&apos;ve joined <span className="text-white font-medium">{invite?.workspace_name}</span>
              </p>
              <p className="text-xs text-zinc-500">Redirecting to dashboard…</p>
            </div>
          )}

          {/* Error state (no invite found) */}
          {error && !invite && (
            <div className="text-center space-y-4">
              <div className="w-16 h-16 rounded-full bg-red-900/30 border border-red-700 flex items-center justify-center mx-auto">
                <AlertCircle size={32} className="text-red-400" />
              </div>
              <h2 className="text-xl font-bold text-white">Invalid Invite</h2>
              <p className="text-sm text-zinc-400">{error}</p>
              <Link href="/" className="inline-block px-6 py-2.5 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 text-sm transition-colors">
                Go Home
              </Link>
            </div>
          )}

          {/* Invite details */}
          {invite && !accepted && (
            <div className="space-y-6">
              <div className="text-center">
                {invite.workspace_logo ? (
                  <img src={invite.workspace_logo} alt="" className="w-16 h-16 rounded-xl mx-auto mb-4 border border-zinc-700" />
                ) : (
                  <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-violet-600 to-cyan-600 flex items-center justify-center mx-auto mb-4">
                    <Users size={28} className="text-white" />
                  </div>
                )}
                <h2 className="text-xl font-bold text-white">You&apos;re invited!</h2>
                <p className="text-sm text-zinc-400 mt-1">
                  <span className="text-zinc-200 font-medium">{invite.invited_by}</span> invited you to join
                </p>
              </div>

              {/* Workspace card */}
              <div className="bg-zinc-800/50 border border-zinc-700/50 rounded-xl p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-zinc-400">Workspace</span>
                  <span className="text-sm text-white font-semibold">{invite.workspace_name}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-zinc-400 flex items-center gap-1.5">
                    <Shield size={12} /> Role
                  </span>
                  <span className="text-sm text-violet-400 capitalize font-medium">{invite.role}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-zinc-400 flex items-center gap-1.5">
                    <Clock size={12} /> Expires
                  </span>
                  <span className="text-sm text-zinc-300">{new Date(invite.expires_at).toLocaleDateString()}</span>
                </div>
              </div>

              {error && (
                <div className="flex items-center gap-2 p-3 bg-red-950/40 border border-red-800 rounded-lg text-sm text-red-400">
                  <AlertCircle size={14} /> {error}
                </div>
              )}

              {/* Actions */}
              {isAuthenticated ? (
                <button
                  onClick={handleAccept}
                  disabled={accepting}
                  className="w-full py-3 bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 disabled:opacity-60 text-white font-semibold rounded-xl flex items-center justify-center gap-2 transition-all"
                >
                  {accepting ? (
                    <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  ) : (
                    <Check size={18} />
                  )}
                  {accepting ? "Joining…" : "Accept & Join Workspace"}
                </button>
              ) : (
                <div className="space-y-3">
                  <button
                    onClick={handleAccept}
                    className="w-full py-3 bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-white font-semibold rounded-xl flex items-center justify-center gap-2 transition-all"
                  >
                    <LogIn size={18} />
                    Sign in to Accept
                  </button>
                  <p className="text-center text-xs text-zinc-500">
                    Don&apos;t have an account?{" "}
                    <Link href={`/register?redirect=/invite/${token}`} className="text-violet-400 hover:text-violet-300">
                      Create one
                    </Link>
                  </p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
