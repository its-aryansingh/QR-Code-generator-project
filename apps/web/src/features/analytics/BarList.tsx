import React from 'react';
import { BreakdownItem } from './types';

interface BarListProps {
  title: string;
  items: BreakdownItem[];
}

export function BarList({ title, items }: BarListProps) {
  const maxCount = Math.max(1, ...items.map((i) => i.count));

  return (
    <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs space-y-4">
      <h3 className="text-sm font-bold text-[var(--text)]">{title}</h3>

      {items.length === 0 ? (
        <div className="py-8 text-center text-xs text-[var(--text-muted)]">
          No data available for this range
        </div>
      ) : (
        <div className="space-y-3">
          {items.map((item) => {
            const widthPct = Math.max(4, Math.round((item.count / maxCount) * 100));
            return (
              <div key={item.key} className="space-y-1">
                <div className="flex items-center justify-between text-xs">
                  <span className="font-medium text-[var(--text)] truncate">{item.label || item.key}</span>
                  <span className="font-mono text-[var(--text-muted)] tabular-nums">
                    {item.count.toLocaleString()} ({item.percentage.toFixed(1)}%)
                  </span>
                </div>
                <div className="w-full h-2 rounded-full bg-[var(--bg-subtle)] border border-[var(--border)] overflow-hidden">
                  <div
                    className="h-full bg-[var(--accent)] rounded-full transition-all duration-300"
                    style={{ width: `${widthPct}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
