import React from 'react';
import { ArrowUpRight, ArrowDownRight, Minus } from 'lucide-react';

interface KpiTileProps {
  label: string;
  value: number | string;
  deltaPct?: number;
  subtitle?: string;
  icon?: React.ReactNode;
}

export function KpiTile({ label, value, deltaPct, subtitle, icon }: KpiTileProps) {
  const isPositive = deltaPct !== undefined && deltaPct > 0;
  const isNegative = deltaPct !== undefined && deltaPct < 0;

  return (
    <div className="bg-[var(--surface)] p-5 rounded-2xl border border-[var(--border)] shadow-xs">
      <div className="flex items-center justify-between text-xs text-[var(--text-muted)] mb-2 font-medium">
        <span>{label}</span>
        {icon && <span className="text-[var(--text-muted)]">{icon}</span>}
      </div>

      <div className="text-2xl font-extrabold tracking-tight text-[var(--text)] tabular-nums">
        {typeof value === 'number' ? value.toLocaleString() : value}
      </div>

      {(deltaPct !== undefined || subtitle) && (
        <div className="flex items-center gap-1.5 mt-2 text-xs">
          {deltaPct !== undefined && (
            <span
              className={`inline-flex items-center font-bold text-[11px] px-1.5 py-0.5 rounded ${
                isPositive
                  ? 'bg-green-500/10 text-green-600'
                  : isNegative
                  ? 'bg-red-500/10 text-red-600'
                  : 'bg-gray-500/10 text-gray-600'
              }`}
            >
              {isPositive ? (
                <ArrowUpRight className="w-3 h-3 mr-0.5" />
              ) : isNegative ? (
                <ArrowDownRight className="w-3 h-3 mr-0.5" />
              ) : (
                <Minus className="w-3 h-3 mr-0.5" />
              )}
              {Math.abs(deltaPct).toFixed(1)}%
            </span>
          )}
          {subtitle && <span className="text-[11px] text-[var(--text-muted)] truncate">{subtitle}</span>}
        </div>
      )}
    </div>
  );
}
