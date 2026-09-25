'use client';

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { api, ApiError } from '@/lib/api/client';
import { QrCode, ArrowRight, AlertCircle, Loader2 } from 'lucide-react';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setErrorMsg(null);

    try {
      const res: any = await api.post('/v1/auth/login', { email, password });
      const ws = res?.user?.workspaces?.[0]?.slug || 'default';
      router.push(`/w/${ws}/qr`);
    } catch (err: any) {
      if (err instanceof ApiError) {
        setErrorMsg(err.problem.detail || err.problem.title);
      } else {
        setErrorMsg(err?.message || 'Login failed');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-[var(--bg-subtle)]">
      <div className="max-w-md w-full bg-[var(--surface)] p-8 rounded-2xl border border-[var(--border)] shadow-sm">
        <div className="text-center mb-8">
          <Link href="/" className="inline-flex items-center gap-2 mb-4">
            <div className="w-10 h-10 rounded-xl bg-[var(--accent)] flex items-center justify-center text-white font-bold">
              <QrCode className="w-6 h-6" />
            </div>
            <span className="font-extrabold text-2xl tracking-tight text-[var(--text)]">QRit</span>
          </Link>
          <h1 className="text-xl font-bold text-[var(--text)]">Sign in to your account</h1>
          <p className="text-xs text-[var(--text-muted)] mt-1">Manage your dynamic QR codes and analytics</p>
        </div>

        {errorMsg && (
          <div className="mb-6 p-3.5 rounded-xl bg-[var(--danger)]/10 border border-[var(--danger)]/20 flex items-start gap-2.5 text-xs text-[var(--danger)]">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Email address</label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="name@company.com"
              className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="block text-xs font-semibold text-[var(--text-muted)]">Password</label>
            </div>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••••••"
              className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg font-semibold text-sm flex items-center justify-center gap-2 hover:opacity-95 transition-opacity disabled:opacity-60"
          >
            {loading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <>
                Sign In
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </button>
        </form>

        <div className="mt-8 pt-6 border-t border-[var(--border)] text-center text-xs text-[var(--text-muted)]">
          Don&apos;t have an account?{' '}
          <Link href="/register" className="font-semibold text-[var(--accent)] hover:underline">
            Create account
          </Link>
        </div>
      </div>
    </div>
  );
}
