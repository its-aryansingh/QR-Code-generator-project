'use client';

import React, { useState, useEffect } from 'react';
import { api } from '@/lib/api/client';
import {
  AnalyticsSummary,
  TimeSeriesPoint,
  BreakdownItem,
  HeatmapCell,
  AnalyticsFilter,
} from '@/features/analytics/types';
import { KpiTile } from '@/features/analytics/KpiTile';
import { TimeSeriesChart } from '@/features/analytics/TimeSeriesChart';
import { BarList } from '@/features/analytics/BarList';
import { Heatmap } from '@/features/analytics/Heatmap';
import { FilterBar } from '@/features/analytics/FilterBar';
import { LiveBadge } from '@/features/analytics/LiveBadge';
import { QrCode, Users, ShieldAlert, Ban } from 'lucide-react';

export default function AnalyticsPage({
  params,
}: {
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);

  const [filter, setFilter] = useState<AnalyticsFilter>({
    range: '7d',
  });

  const [summary, setSummary] = useState<AnalyticsSummary>({
    total_scans: 1240,
    unique_scans: 980,
    bot_hits: 154,
    blocked_hits: 12,
    scans_delta_pct: 14.2,
    uniques_delta_pct: 8.5,
    top_country: 'United States',
    top_device: 'Mobile (84%)',
  });

  const [timeseries, setTimeseries] = useState<TimeSeriesPoint[]>([]);
  const [countries, setCountries] = useState<BreakdownItem[]>([
    { key: 'US', label: 'United States', count: 620, percentage: 50.0 },
    { key: 'IN', label: 'India', count: 310, percentage: 25.0 },
    { key: 'GB', label: 'United Kingdom', count: 180, percentage: 14.5 },
    { key: 'DE', label: 'Germany', count: 80, percentage: 6.5 },
    { key: 'CA', label: 'Canada', count: 50, percentage: 4.0 },
  ]);

  const [devices, setDevices] = useState<BreakdownItem[]>([
    { key: 'mobile', label: 'Mobile (iOS & Android)', count: 1040, percentage: 83.9 },
    { key: 'desktop', label: 'Desktop', count: 160, percentage: 12.9 },
    { key: 'tablet', label: 'Tablet', count: 40, percentage: 3.2 },
  ]);

  const [osList, setOsList] = useState<BreakdownItem[]>([
    { key: 'iOS', label: 'Apple iOS', count: 680, percentage: 54.8 },
    { key: 'Android', label: 'Google Android', count: 360, percentage: 29.0 },
    { key: 'macOS', label: 'macOS', count: 110, percentage: 8.9 },
    { key: 'Windows', label: 'Windows', count: 90, percentage: 7.3 },
  ]);

  const [heatmapCells, setHeatmapCells] = useState<HeatmapCell[]>([]);

  useEffect(() => {
    // Generate sample 7-day trend
    const pts: TimeSeriesPoint[] = [];
    const now = new Date();
    for (let i = 6; i >= 0; i--) {
      const d = new Date(now);
      d.setDate(d.getDate() - i);
      pts.push({
        timestamp: d.toISOString(),
        scans: Math.floor(120 + Math.random() * 150),
        unique_scans: Math.floor(80 + Math.random() * 100),
      });
    }
    setTimeseries(pts);

    // Generate sample heatmap
    const cells: HeatmapCell[] = [];
    for (let day = 0; day < 7; day++) {
      for (let hr = 0; hr < 24; hr++) {
        const count = (hr >= 9 && hr <= 21) ? Math.floor(Math.random() * 40) : Math.floor(Math.random() * 8);
        cells.push({ day_of_week: day, hour: hr, count });
      }
    }
    setHeatmapCells(cells);
  }, [filter]);

  const handleExportCsv = () => {
    const csvContent =
      'data:text/csv;charset=utf-8,Date,Total Scans,Unique Visitors\n' +
      timeseries
        .map((t) => `${new Date(t.timestamp).toLocaleDateString()},${t.scans},${t.unique_scans}`)
        .join('\n');
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', `qrit-analytics-${filter.range}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="space-y-6">
      {/* Header with Live Badge */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold tracking-tight text-[var(--text)]">Analytics</h1>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">
            Real-time engagement, bot filtration, and audience demographics
          </p>
        </div>
        <LiveBadge workspace={workspace} />
      </div>

      {/* Filter Bar */}
      <FilterBar filter={filter} onChange={setFilter} onExportCsv={handleExportCsv} />

      {/* KPI Tiles Row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiTile
          label="Total Scans"
          value={summary.total_scans}
          deltaPct={summary.scans_delta_pct}
          subtitle="vs previous period"
          icon={<QrCode className="w-4 h-4" />}
        />
        <KpiTile
          label="Unique Visitors"
          value={summary.unique_scans}
          deltaPct={summary.uniques_delta_pct}
          subtitle="privacy-hashed daily"
          icon={<Users className="w-4 h-4" />}
        />
        <KpiTile
          label="Automated Hits Filtered"
          value={summary.bot_hits}
          subtitle="previews & scrapers ignored"
          icon={<ShieldAlert className="w-4 h-4" />}
        />
        <KpiTile
          label="Blocked / Expired Hits"
          value={summary.blocked_hits}
          subtitle="limits or rules matched"
          icon={<Ban className="w-4 h-4" />}
        />
      </div>

      {/* Time Series Chart */}
      <TimeSeriesChart data={timeseries} title="Scan Volume & Reach" />

      {/* Breakdown Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <BarList title="Top Countries" items={countries} />
        <BarList title="Device Types" items={devices} />
        <BarList title="Operating Systems" items={osList} />
      </div>

      {/* Heatmap Grid */}
      <Heatmap cells={heatmapCells} />

      {/* Footnote */}
      <div className="p-4 rounded-xl bg-[var(--surface)] border border-[var(--border)] text-xs text-[var(--text-muted)] flex items-center justify-between">
        <span>
          Data Quality Note: All scans exclude automated link previews (Slack, WhatsApp, bot crawlers). Visitor hashes are rotated daily to guarantee GDPR/CCPA privacy compliance.
        </span>
        <span className="font-mono text-[11px]">UTC / Workspace TZ</span>
      </div>
    </div>
  );
}
