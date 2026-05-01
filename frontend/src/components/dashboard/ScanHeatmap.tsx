"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";

const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const HOURS = Array.from({ length: 24 }, (_, i) => i);

interface HeatmapCell {
  day: number;   // 0=Sun … 6=Sat
  hour: number;  // 0-23
  count: number;
}

interface Props {
  workspaceId: string;
}

function colorForValue(value: number, max: number): string {
  if (max === 0 || value === 0) return "bg-zinc-800";
  const ratio = value / max;
  if (ratio < 0.2) return "bg-violet-900/40";
  if (ratio < 0.4) return "bg-violet-800/60";
  if (ratio < 0.6) return "bg-violet-700";
  if (ratio < 0.8) return "bg-violet-600";
  return "bg-violet-500";
}

export function ScanHeatmap({ workspaceId }: Props) {
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [cells, setCells] = useState<HeatmapCell[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!workspaceId || !accessToken) return;
    setLoading(true);
    fetch(`${api}/workspaces/${workspaceId}/analytics/heatmap`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then((r) => r.json())
      .then((d) => {
        if (d.success && Array.isArray(d.data)) setCells(d.data);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [workspaceId, accessToken]);

  // Build lookup: grid[day][hour] = count
  const grid: number[][] = Array.from({ length: 7 }, () => Array(24).fill(0));
  let max = 0;
  for (const c of cells) {
    if (c.day >= 0 && c.day < 7 && c.hour >= 0 && c.hour < 24) {
      grid[c.day][c.hour] = c.count;
      if (c.count > max) max = c.count;
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="w-6 h-6 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="overflow-x-auto">
        <div className="min-w-max">
          {/* Hour labels */}
          <div className="flex items-center mb-1 ml-8">
            {HOURS.map((h) => (
              <div key={h} className="w-5 text-center text-[9px] text-zinc-600">
                {h % 6 === 0 ? `${h}h` : ""}
              </div>
            ))}
          </div>

          {/* Grid rows */}
          {DAYS.map((day, di) => (
            <div key={day} className="flex items-center mb-0.5">
              <span className="w-8 text-[10px] text-zinc-500 shrink-0">{day}</span>
              {HOURS.map((h) => {
                const count = grid[di][h];
                return (
                  <div
                    key={h}
                    title={`${day} ${h}:00 — ${count} scan${count !== 1 ? "s" : ""}`}
                    className={`w-5 h-5 rounded-sm mx-px transition-colors ${colorForValue(count, max)}`}
                  />
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Legend */}
      <div className="flex items-center gap-2 text-[10px] text-zinc-500">
        <span>Less</span>
        {["bg-zinc-800", "bg-violet-900/40", "bg-violet-800/60", "bg-violet-700", "bg-violet-600", "bg-violet-500"].map((cls) => (
          <div key={cls} className={`w-3.5 h-3.5 rounded-sm ${cls}`} />
        ))}
        <span>More</span>
      </div>
    </div>
  );
}
