'use client';

import React from 'react';
import Link from 'next/link';
import { Sparkles, ArrowRight, CheckCircle2 } from 'lucide-react';

interface UpgradeCardProps {
  title: string;
  description: string;
  planName?: string;
  workspace: string;
  features?: string[];
  compact?: boolean;
}

export function UpgradeCard({
  title,
  description,
  planName = 'Business',
  workspace,
  features,
  compact = false,
}: UpgradeCardProps) {
  if (compact) {
    return (
      <div className="p-4 rounded-xl border border-[var(--accent)]/30 bg-[var(--accent)]/5 flex items-center justify-between gap-4">
        <div className="space-y-0.5">
          <div className="flex items-center gap-1.5 text-xs font-bold text-[var(--text)]">
            <Sparkles className="w-3.5 h-3.5 text-[var(--accent)]" />
            <span>{title}</span>
          </div>
          <p className="text-[11px] text-[var(--text-muted)]">{description}</p>
        </div>
        <Link
          href={`/w/${workspace}/settings/billing`}
          className="shrink-0 px-3 py-1.5 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold hover:opacity-95 transition-opacity inline-flex items-center gap-1"
        >
          Upgrade to {planName}
          <ArrowRight className="w-3 h-3" />
        </Link>
      </div>
    );
  }

  return (
    <div className="p-8 rounded-2xl border border-[var(--accent)]/30 bg-gradient-to-b from-[var(--surface)] to-[var(--bg-subtle)] text-center max-w-lg mx-auto space-y-5 shadow-sm">
      <div className="w-12 h-12 rounded-2xl bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center mx-auto shadow-inner">
        <Sparkles className="w-6 h-6" />
      </div>

      <div className="space-y-1.5">
        <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-[var(--accent)]/15 text-[var(--accent)]">
          {planName} Plan Feature
        </div>
        <h3 className="text-lg font-extrabold tracking-tight text-[var(--text)]">{title}</h3>
        <p className="text-xs text-[var(--text-muted)] max-w-md mx-auto leading-relaxed">
          {description}
        </p>
      </div>

      {features && features.length > 0 && (
        <div className="py-2 border-y border-[var(--border)] max-w-xs mx-auto space-y-2 text-left">
          {features.map((feat, idx) => (
            <div key={idx} className="flex items-center gap-2 text-xs text-[var(--text)]">
              <CheckCircle2 className="w-3.5 h-3.5 text-[var(--accent)] shrink-0" />
              <span>{feat}</span>
            </div>
          ))}
        </div>
      )}

      <div>
        <Link
          href={`/w/${workspace}/settings/billing`}
          className="inline-flex items-center gap-2 px-5 py-2.5 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold hover:opacity-95 shadow-sm transition-all"
        >
          <span>Upgrade to {planName}</span>
          <ArrowRight className="w-4 h-4" />
        </Link>
        <p className="text-[10px] text-[var(--text-muted)] mt-2">
          Instant activation • Cancel anytime • Codes keep working
        </p>
      </div>
    </div>
  );
}
