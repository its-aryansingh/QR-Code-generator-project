"use client";

/**
 * Analytics charts.
 *
 * Colour choices follow the data's job rather than taste:
 *  - Scan volume over time is ONE series measuring magnitude, so it uses one
 *    hue (the brand violet, 4.7:1 on the dashboard surface) and needs no legend.
 *  - Categorical breakdowns draw from a fixed, validated hue order. The set
 *    below passes lightness-band, chroma-floor, CVD-separation (worst adjacent
 *    ΔE 8.4 protan), normal-vision (worst 19.3) and 3:1 contrast against the
 *    #09090b surface. Hues are assigned in fixed slot order and never cycled.
 *
 * Every chart also exposes its numbers as text, so identity is never carried
 * by colour alone.
 */
import * as React from "react";
import {
  Area, AreaChart, Bar, BarChart, CartesianGrid, Cell,
  ResponsiveContainer, Tooltip, XAxis, YAxis,
} from "recharts";

import { cn } from "@/lib/utils";

/** Single-hue series colour for magnitude-over-time. */
export const SERIES_HUE = "#8B5CF6";

/** Fixed categorical order — assign by slot, never cycle past the end. */
export const CATEGORICAL = [
  "#3987e5", "#d95926", "#199e70", "#c98500",
  "#d55181", "#008300", "#9085e9", "#e66767",
] as const;

const AXIS = "#52525b";
const GRID = "#27272a";

export function categoricalColor(index: number) {
  // Past the palette, everything folds into one neutral rather than inventing
  // a hue that no longer clears the separation checks.
  return index < CATEGORICAL.length ? CATEGORICAL[index] : "#71717a";
}

function TooltipShell({
  label,
  rows,
}: {
  label?: React.ReactNode;
  rows: Array<{ color?: string; name: string; value: React.ReactNode }>;
}) {
  return (
    <div className="rounded-lg border border-zinc-700 bg-zinc-900/95 px-3 py-2 shadow-xl backdrop-blur">
      {label && <p className="mb-1 text-xs font-medium text-zinc-300">{label}</p>}
      {rows.map((row) => (
        <div key={row.name} className="flex items-center gap-2 text-xs">
          {row.color && (
            <span className="size-2 shrink-0 rounded-sm" style={{ background: row.color }} />
          )}
          <span className="text-zinc-400">{row.name}</span>
          <span className="ml-auto font-medium tabular-nums text-zinc-100">{row.value}</span>
        </div>
      ))}
    </div>
  );
}

function shortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

/* ------------------------------------------------------- scans over time */

export function TrendChart({
  data,
  height = 260,
  label = "Scans",
}: {
  data: Array<{ date: string; count: number }>;
  height?: number;
  label?: string;
}) {
  const total = data.reduce((sum, point) => sum + point.count, 0);

  if (!data.length || total === 0) {
    return (
      <div
        className="flex flex-col items-center justify-center rounded-lg border border-dashed border-zinc-800 text-center"
        style={{ height }}
      >
        <p className="text-sm text-zinc-400">No scans in this period yet</p>
        <p className="mt-1 text-xs text-zinc-600">
          Scans appear here within moments of someone using one of your codes.
        </p>
      </div>
    );
  }

  // Label only the peak; a number on every point is noise.
  const peak = data.reduce((best, point) => (point.count > best.count ? point : best), data[0]);

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 12, right: 8, left: -18, bottom: 0 }}>
          <defs>
            <linearGradient id="scanFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={SERIES_HUE} stopOpacity={0.28} />
              <stop offset="100%" stopColor={SERIES_HUE} stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={GRID} strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="date"
            tickFormatter={shortDate}
            tick={{ fill: AXIS, fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            minTickGap={28}
          />
          <YAxis
            tick={{ fill: AXIS, fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            allowDecimals={false}
            width={44}
          />
          <Tooltip
            cursor={{ stroke: "#52525b", strokeWidth: 1, strokeDasharray: "4 4" }}
            content={({ active, payload }) =>
              active && payload?.length ? (
                <TooltipShell
                  label={new Date(String(payload[0].payload.date)).toLocaleDateString(undefined, {
                    weekday: "short", month: "short", day: "numeric",
                  })}
                  rows={[{
                    color: SERIES_HUE,
                    name: label,
                    value: Number(payload[0].value).toLocaleString(),
                  }]}
                />
              ) : null
            }
          />
          <Area
            type="monotone"
            dataKey="count"
            stroke={SERIES_HUE}
            strokeWidth={2}
            fill="url(#scanFill)"
            activeDot={{ r: 4, strokeWidth: 2, stroke: "#09090b", fill: SERIES_HUE }}
            dot={false}
          />
        </AreaChart>
      </ResponsiveContainer>
      <p className="mt-1 text-xs text-zinc-600">
        {total.toLocaleString()} {label.toLowerCase()} · peak {peak.count.toLocaleString()} on{" "}
        {shortDate(peak.date)}
      </p>
    </div>
  );
}

/* ------------------------------------------------------- hour histogram */

export function HourChart({ data }: { data: Array<{ hour: number; count: number }> }) {
  const total = data.reduce((sum, point) => sum + point.count, 0);
  if (!total) {
    return <p className="py-8 text-center text-sm text-zinc-600">No scans to profile yet</p>;
  }
  const busiest = data.reduce((best, point) => (point.count > best.count ? point : best), data[0]);

  return (
    <div>
      <div style={{ height: 180 }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 4, right: 4, left: -22, bottom: 0 }}>
            <CartesianGrid stroke={GRID} strokeDasharray="3 3" vertical={false} />
            <XAxis
              dataKey="hour"
              tick={{ fill: AXIS, fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              interval={2}
              tickFormatter={(hour: number) => `${hour}h`}
            />
            <YAxis
              tick={{ fill: AXIS, fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              allowDecimals={false}
              width={40}
            />
            <Tooltip
              cursor={{ fill: "#ffffff08" }}
              content={({ active, payload }) =>
                active && payload?.length ? (
                  <TooltipShell
                    label={`${payload[0].payload.hour}:00 – ${payload[0].payload.hour}:59 UTC`}
                    rows={[{
                      color: SERIES_HUE,
                      name: "Scans",
                      value: Number(payload[0].value).toLocaleString(),
                    }]}
                  />
                ) : null
              }
            />
            {/* 4px rounded data-end, square against the baseline */}
            <Bar dataKey="count" fill={SERIES_HUE} radius={[4, 4, 0, 0]} maxBarSize={18} />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <p className="mt-1 text-xs text-zinc-600">
        Busiest hour {busiest.hour}:00 UTC with {busiest.count.toLocaleString()} scans
      </p>
    </div>
  );
}

/* ------------------------------------------------------- ranked bars */

/**
 * Magnitude comparison across a handful of named things. Plain markup rather
 * than a pie: length is read accurately, angle is not, and every row carries
 * its own label and value.
 */
export function RankedBars({
  data,
  colorize,
  emptyMessage = "Nothing recorded yet",
  max: explicitMax,
  unit = "scans",
}: {
  data: Array<{ label: string; count: number; hint?: string }>;
  colorize?: boolean;
  emptyMessage?: string;
  max?: number;
  unit?: string;
}) {
  const rows = data.filter((row) => row.count > 0);
  if (!rows.length) {
    return <p className="py-6 text-center text-sm text-zinc-600">{emptyMessage}</p>;
  }
  const total = rows.reduce((sum, row) => sum + row.count, 0);
  const max = explicitMax ?? Math.max(...rows.map((row) => row.count));

  return (
    <ul className="space-y-2.5">
      {rows.map((row, index) => {
        const share = total ? Math.round((row.count / total) * 100) : 0;
        const color = colorize ? categoricalColor(index) : SERIES_HUE;
        return (
          <li key={`${row.label}-${index}`}>
            <div className="mb-1 flex items-baseline justify-between gap-3 text-xs">
              <span className="flex min-w-0 items-center gap-2">
                {colorize && (
                  <span className="size-2 shrink-0 rounded-sm" style={{ background: color }} />
                )}
                <span className="truncate text-zinc-300">{row.label}</span>
                {row.hint && <span className="shrink-0 text-zinc-600">{row.hint}</span>}
              </span>
              <span className="shrink-0 tabular-nums text-zinc-400">
                {row.count.toLocaleString()}
                <span className="ml-1.5 text-zinc-600">{share}%</span>
              </span>
            </div>
            {/* 4px rounded data-end; the track supplies the 2px surface gap */}
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-800/70">
              <div
                className="h-full rounded-full transition-[width] duration-500"
                style={{
                  width: `${Math.max(2, (row.count / max) * 100)}%`,
                  background: color,
                }}
              />
            </div>
          </li>
        );
      })}
      <li className="pt-1 text-xs text-zinc-600">
        {total.toLocaleString()} {unit} across {rows.length}{" "}
        {rows.length === 1 ? "entry" : "entries"}
      </li>
    </ul>
  );
}

/* ------------------------------------------------------- share bar */

/** A single stacked bar for composition, with a 2px surface gap between
 *  segments and a full legend beneath. */
export function ShareBar({
  data,
  emptyMessage = "No data yet",
}: {
  data: Array<{ label: string; count: number }>;
  emptyMessage?: string;
}) {
  const rows = data.filter((row) => row.count > 0);
  const total = rows.reduce((sum, row) => sum + row.count, 0);
  if (!total) return <p className="py-6 text-center text-sm text-zinc-600">{emptyMessage}</p>;

  return (
    <div>
      <div className="flex h-2.5 w-full gap-0.5 overflow-hidden rounded-full">
        {rows.map((row, index) => (
          <div
            key={row.label}
            className="h-full first:rounded-l-full last:rounded-r-full"
            style={{
              width: `${(row.count / total) * 100}%`,
              background: categoricalColor(index),
            }}
            title={`${row.label}: ${row.count.toLocaleString()}`}
          />
        ))}
      </div>
      <ul className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1.5 sm:grid-cols-3">
        {rows.map((row, index) => (
          <li key={row.label} className="flex items-center gap-2 text-xs">
            <span
              className="size-2 shrink-0 rounded-sm"
              style={{ background: categoricalColor(index) }}
            />
            <span className="truncate capitalize text-zinc-400">{row.label}</span>
            <span className="ml-auto shrink-0 tabular-nums text-zinc-300">
              {Math.round((row.count / total) * 100)}%
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/* ------------------------------------------------------- sparkline */

export function Sparkline({
  data,
  className,
}: {
  data: Array<{ count: number }>;
  className?: string;
}) {
  if (data.length < 2) return null;
  const values = data.map((point) => point.count);
  const max = Math.max(...values, 1);
  const points = values
    .map((value, index) => `${(index / (values.length - 1)) * 100},${28 - (value / max) * 26}`)
    .join(" ");

  return (
    <svg
      viewBox="0 0 100 28"
      preserveAspectRatio="none"
      className={cn("h-7 w-full", className)}
      aria-hidden="true"
    >
      <polyline
        points={points}
        fill="none"
        stroke={SERIES_HUE}
        strokeWidth={2}
        vectorEffect="non-scaling-stroke"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
