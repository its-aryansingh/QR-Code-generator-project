'use client';

import React, { useState } from 'react';
import { TimeSeriesPoint } from './types';
import { Table, BarChart2 } from 'lucide-react';

interface TimeSeriesChartProps {
  data: TimeSeriesPoint[];
  title?: string;
}

export function TimeSeriesChart({ data, title = 'Scan Activity' }: TimeSeriesChartProps) {
  const [viewMode, setViewMode] = useState<'chart' | 'table'>('chart');

  const maxVal = Math.max(1, ...data.map((d) => Math.max(d.scans, d.unique_scans)));
  const width = 600;
  const height = 180;
  const padding = 20;

  const pointsScans = data
    .map((d, i) => {
      const x = padding + (i / Math.max(1, data.length - 1)) * (width - 2 * padding);
      const y = height - padding - (d.scans / maxVal) * (height - 2 * padding);
      return `${x},${y}`;
    })
    .join(' ');

  const pointsUniques = data
    .map((d, i) => {
      const x = padding + (i / Math.max(1, data.length - 1)) * (width - 2 * padding);
      const y = height - padding - (d.unique_scans / maxVal) * (height - 2 * padding);
      return `${x},${y}`;
    })
    .join(' ');

  return (
    <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-bold text-[var(--text)]">{title}</h3>
          <p className="text-xs text-[var(--text-muted)]">Scans vs Unique Visitors over time</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-4 text-xs">
            <span className="flex items-center gap-1.5 text-xs text-[var(--text)]">
              <span className="w-2.5 h-2.5 rounded-full bg-[var(--accent)]" />
              Total Scans
            </span>
            <span className="flex items-center gap-1.5 text-xs text-[var(--text)]">
              <span className="w-2.5 h-2.5 rounded-full bg-teal-500" />
              Uniques
            </span>
          </div>
          <button
            onClick={() => setViewMode(viewMode === 'chart' ? 'table' : 'chart')}
            className="p-1.5 text-xs font-semibold text-[var(--text-muted)] hover:text-[var(--text)] rounded-lg border border-[var(--border)] flex items-center gap-1 transition-colors"
          >
            {viewMode === 'chart' ? <Table className="w-3.5 h-3.5" /> : <BarChart2 className="w-3.5 h-3.5" />}
            {viewMode === 'chart' ? 'View as table' : 'View chart'}
          </button>
        </div>
      </div>

      {viewMode === 'chart' ? (
        <div className="w-full overflow-hidden">
          <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-48">
            {/* Grid lines */}
            <line x1={padding} y1={height - padding} x2={width - padding} y2={height - padding} stroke="var(--border)" strokeWidth="1" />
            <line x1={padding} y1={height / 2} x2={width - padding} y2={height / 2} stroke="var(--border)" strokeWidth="1" strokeDasharray="3 3" />
            <line x1={padding} y1={padding} x2={width - padding} y2={padding} stroke="var(--border)" strokeWidth="1" strokeDasharray="3 3" />

            {/* Total Scans Polyline */}
            {data.length > 1 && (
              <polyline
                fill="none"
                stroke="var(--accent)"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                points={pointsScans}
              />
            )}

            {/* Unique Scans Polyline */}
            {data.length > 1 && (
              <polyline
                fill="none"
                stroke="#14b8a6"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeDasharray="4 2"
                points={pointsUniques}
              />
            )}
          </svg>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-[var(--bg-subtle)] text-[var(--text-muted)] font-semibold border-b border-[var(--border)]">
              <tr>
                <th className="py-2 px-3">Date</th>
                <th className="py-2 px-3 text-right">Scans</th>
                <th className="py-2 px-3 text-right">Unique Visitors</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border)]">
              {data.map((d, i) => (
                <tr key={i} className="hover:bg-[var(--bg-subtle)]/50">
                  <td className="py-2 px-3">{new Date(d.timestamp).toLocaleDateString()}</td>
                  <td className="py-2 px-3 text-right font-mono font-medium">{d.scans.toLocaleString()}</td>
                  <td className="py-2 px-3 text-right font-mono font-medium text-teal-600">{d.unique_scans.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
