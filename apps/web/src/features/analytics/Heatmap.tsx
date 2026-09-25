import React from 'react';
import { HeatmapCell } from './types';

interface HeatmapProps {
  cells: HeatmapCell[];
}

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

export function Heatmap({ cells }: HeatmapProps) {
  const maxVal = Math.max(1, ...cells.map((c) => c.count));

  // Map into 2D array [day][hour]
  const grid: number[][] = Array.from({ length: 7 }, () => Array(24).fill(0));
  for (const c of cells) {
    if (c.day_of_week >= 0 && c.day_of_week < 7 && c.hour >= 0 && c.hour < 24) {
      grid[c.day_of_week][c.hour] = c.count;
    }
  }

  const getOpacity = (count: number) => {
    if (count === 0) return 0.05;
    return Math.max(0.15, Math.min(1.0, count / maxVal));
  };

  return (
    <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs space-y-4">
      <div>
        <h3 className="text-sm font-bold text-[var(--text)]">Scan Activity Heatmap</h3>
        <p className="text-xs text-[var(--text-muted)]">Frequency by day of week and local hour of day</p>
      </div>

      <div className="overflow-x-auto">
        <div className="min-w-[600px] space-y-1 text-xs">
          {/* Hour headers */}
          <div className="flex items-center gap-1 pl-10 text-[10px] text-[var(--text-muted)] font-mono">
            {Array.from({ length: 24 }).map((_, h) => (
              <div key={h} className="flex-1 text-center">
                {h % 3 === 0 ? h : ''}
              </div>
            ))}
          </div>

          {/* Grid rows */}
          {DAYS.map((dayName, d) => (
            <div key={dayName} className="flex items-center gap-1">
              <span className="w-9 text-[11px] font-semibold text-[var(--text-muted)]">{dayName}</span>
              <div className="flex-1 flex items-center gap-1">
                {grid[d].map((cnt, h) => (
                  <div
                    key={h}
                    title={`${dayName} ${h}:00 - ${cnt} scans`}
                    className="flex-1 h-5 rounded-xs transition-opacity hover:opacity-100 cursor-pointer"
                    style={{
                      backgroundColor: 'var(--accent)',
                      opacity: getOpacity(cnt),
                    }}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
