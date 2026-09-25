'use client';

import React from 'react';
import { AlertCircle } from 'lucide-react';

interface UsageMeterProps {
  label: string;
  current: number;
  limit: number; // -1 indicates unlimited
  unit?: string;
  helperText?: string;
}

export function UsageMeter({
  label,
  current,
  limit,
  unit = '',
  helperText,
}: UsageMeterProps) {
  const isUnlimited = limit === -1 || limit === Infinity;
  const percentage = isUnlimited ? 0 : Math.min(100, Math.round((current / Math.max(1, limit)) * 100));

  let statusColor = 'bg-[var(--accent)]';
  let badgeColor = 'text-[var(--text-muted)]';
  let isNearLimit = false;
  let isExceeded = false;

  if (!isUnlimited) {
    if (percentage >= 100) {
      statusColor = 'bg-[var(--danger)]';
      badgeColor = 'text-[var(--danger)] font-bold';
      isExceeded = true;
    } else if (percentage >= 80) {
      statusColor = 'bg-amber-500';
      badgeColor = 'text-amber-500 font-bold';
      isNearLimit = true;
    }
  }

  return (
    <div className="p-4 rounded-xl bg-[var(--surface)] border border-[var(--border)] space-y-2">
      <div className="flex items-center justify-between text-xs">
        <span className="font-semibold text-[var(--text)]">{label}</span>
        <div className="flex items-center gap-1.5">
          <span className={`font-mono text-xs ${badgeColor}`}>
            {current.toLocaleString()} {unit}
            {' / '}
            {isUnlimited ? '∞ Unlimited' : `${limit.toLocaleString()} ${unit}`}
          </span>
          {!isUnlimited && (
            <span className="text-[10px] text-[var(--text-muted)] font-mono">
              ({percentage}%)
            </span>
          )}
        </div>
      </div>

      <div className="w-full h-2 rounded-full bg-[var(--bg-subtle)] overflow-hidden">
        {isUnlimited ? (
          <div className="h-full bg-emerald-500/40 rounded-full w-full" />
        ) : (
          <div
            className={`h-full rounded-full transition-all duration-500 ${statusColor}`}
            style={{ width: `${percentage}%` }}
          />
        )}
      </div>

      <div className="flex items-center justify-between text-[11px] text-[var(--text-muted)]">
        <span>{helperText || (isUnlimited ? 'No hard limit on this plan' : `${percentage}% consumed`)}</span>
        {(isNearLimit || isExceeded) && (
          <span className="inline-flex items-center gap-1 text-[10px] text-amber-500 font-medium">
            <AlertCircle className="w-3 h-3" />
            {isExceeded ? 'Limit reached' : 'Approaching plan limit'}
          </span>
        )}
      </div>
    </div>
  );
}
