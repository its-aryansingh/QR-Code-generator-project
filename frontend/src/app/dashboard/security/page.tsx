"use client";

import Link from "next/link";
import { ArrowRight, ShieldCheck } from "lucide-react";

export default function SecurityPage() {
  return (
    <div className="max-w-3xl space-y-6">
      <div>
        <div className="flex items-center gap-3">
          <ShieldCheck className="size-6 text-emerald-400" />
          <h1 className="text-2xl font-bold text-zinc-100">Security & SSO</h1>
        </div>
        <p className="mt-1 text-sm text-zinc-500">Manage workspace security and single sign-on settings.</p>
      </div>
      <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-6">
        <h2 className="font-semibold text-zinc-200">Single sign-on configuration</h2>
        <p className="mt-2 text-sm text-zinc-500">Configure SAML or OIDC providers from the workspace branding and SSO settings.</p>
        <Link href="/dashboard/branding" className="mt-5 inline-flex items-center gap-2 rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500">
          Open SSO settings <ArrowRight className="size-4" />
        </Link>
      </div>
    </div>
  );
}
