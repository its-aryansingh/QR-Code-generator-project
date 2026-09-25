import React from 'react';
import { AnalyticsFilter } from './types';
import { Calendar, Download } from 'lucide-react';

interface FilterBarProps {
  filter: AnalyticsFilter;
  onChange: (filter: AnalyticsFilter) => void;
  onExportCsv?: () => void;
}

export function FilterBar({ filter, onChange, onExportCsv }: FilterBarProps) {
  const ranges: { key: AnalyticsFilter['range']; label: string }[] = [
    { key: 'today', label: 'Today' },
    { key: '7d', label: 'Last 7 Days' },
    { key: '30d', label: 'Last 30 Days' },
    { key: '90d', label: 'Last 90 Days' },
    { key: '12m', label: 'Last 12 Months' },
  ];

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-[var(--surface)] p-3 rounded-2xl border border-[var(--border)] shadow-xs">
      <div className="flex flex-wrap items-center gap-1.5">
        <Calendar className="w-4 h-4 text-[var(--text-muted)] mr-1" />
        {ranges.map((r) => (
          <button
            key={r.key}
            onClick={() => onChange({ ...filter, range: r.key })}
            className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors ${
              filter.range === r.key
                ? 'bg-[var(--accent)] text-[var(--accent-fg)]'
                : 'bg-[var(--bg-subtle)] text-[var(--text-muted)] hover:text-[var(--text)]'
            }`}
          >
            {r.label}
          </button>
        ))}
      </div>

      <div className="flex items-center gap-2">
        {onExportCsv && (
          <button
            onClick={onExportCsv}
            className="px-3 py-1.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-semibold text-[var(--text)] hover:bg-[var(--border)]/40 transition-colors flex items-center gap-1.5"
          >
            <Download className="w-3.5 h-3.5" />
            Export CSV
          </button>
        )}
      </div>
    </div>
  );
}
