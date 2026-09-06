"use client";

import { useState } from "react";
import Link from "next/link";

import { HourChart, RankedBars, ShareBar, TrendChart } from "@/components/enterprise/charts";
import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, EmptyState, ErrorState, Grid, LoadingPanel, Panel, PageHeader,
  Select, StatCard, Tabs, formatNumber, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";

const RANGES = [7, 14, 30, 90, 180, 365];

export default function AnalyticsPage() {
  return (
    <WorkspaceGate>
      <Analytics />
    </WorkspaceGate>
  );
}

function Analytics() {
  const [days, setDays] = useState(30);
  const [tab, setTab] = useState("audience");
  const { workspaceId, can, limit } = useWorkspace();

  const { data, loading, error, reload } = useResource(
    (id) => enterprise.analytics(id, days),
    [days],
  );

  const exportCsv = useMutation(() => enterprise.exportCsv(workspaceId!, "analytics", { days }));
  const retention = limit("analytics_retention_days");

  return (
    <>
      <PageHeader
        title="Analytics"
        description="Scan performance across every code in this workspace."
        actions={
          <>
            <Select value={days} onChange={(e) => setDays(Number(e.target.value))} className="w-40">
              {RANGES.filter((value) => value <= retention || value === 7).map((value) => (
                <option key={value} value={value}>Last {value} days</option>
              ))}
            </Select>
            {can("exports") && (
              <Btn variant="outline" loading={exportCsv.busy} onClick={() => exportCsv.run()}>
                Export CSV
              </Btn>
            )}
          </>
        }
      />

      {error && <ErrorState message={error.message} onRetry={reload} />}
      {exportCsv.error && <div className="mb-4"><ErrorState message={exportCsv.error} /></div>}

      {data && data.range_days < days && (
        <div className="mb-4 rounded-lg border border-amber-900/40 bg-amber-950/20 px-4 py-2.5 text-sm text-amber-200">
          Your plan retains {retention} days of scan history, so this view is showing{" "}
          {data.range_days} days.{" "}
          <Link href="/pricing" className="underline underline-offset-2">See longer retention</Link>
        </div>
      )}

      {loading && !data ? (
        <LoadingPanel rows={10} />
      ) : data ? (
        <div className="space-y-5">
          <Grid cols={4}>
            <StatCard
              label="Total scans"
              value={formatNumber(data.totals.total_scans)}
              delta={data.trend.scans}
              hint={`vs ${formatNumber(data.trend.previous_scans)} previous`}
            />
            <StatCard
              label="Unique visitors"
              value={formatNumber(data.totals.unique_visitors)}
              delta={data.trend.unique_visitors}
            />
            <StatCard
              label="Avg scans / code"
              value={data.totals.avg_scans_per_qr.toFixed(1)}
              hint={`${formatNumber(data.totals.total_qr_codes)} codes`}
            />
            <StatCard
              label="Codes scanned"
              value={`${data.totals.engagement_rate}%`}
              hint={`${formatNumber(data.totals.scanned_qr_codes)} of ${formatNumber(data.totals.total_qr_codes)} got at least one scan`}
            />
          </Grid>

          <Panel title="Scan volume" description={`Daily scans over the last ${data.range_days} days`}>
            <TrendChart data={data.scans_by_date} height={300} />
          </Panel>

          <Tabs
            active={tab}
            onChange={setTab}
            tabs={[
              { id: "audience", label: "Audience" },
              { id: "geography", label: "Geography" },
              { id: "content", label: "Content" },
              { id: "activity", label: "Live activity", count: data.recent_scans.length },
            ]}
          />

          {tab === "audience" && (
            <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
              <Panel title="Devices" description="What people scan with">
                <ShareBar
                  data={data.by_device.map((row) => ({ label: row.device, count: row.count }))}
                />
              </Panel>
              <Panel title="Operating systems">
                <RankedBars
                  data={data.by_os.map((row) => ({ label: row.os, count: row.count }))}
                  colorize
                />
              </Panel>
              <Panel title="Browsers">
                <RankedBars
                  data={data.by_browser.map((row) => ({ label: row.browser, count: row.count }))}
                />
              </Panel>
              <Panel title="Time of day" description="Scans by hour, UTC">
                <HourChart data={data.scans_by_hour} />
              </Panel>
              <Panel title="Languages" className="lg:col-span-2">
                <RankedBars
                  data={data.by_language.map((row) => ({ label: row.language, count: row.count }))}
                  emptyMessage="No language data recorded yet"
                />
              </Panel>
            </div>
          )}

          {tab === "geography" && (
            <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
              <Panel title="Countries" description="Where scans come from">
                <RankedBars
                  data={data.by_country.map((row) => ({ label: row.country, count: row.count }))}
                  colorize
                  emptyMessage="No location data yet — geography is resolved from your CDN's headers"
                />
              </Panel>
              <Panel title="Cities">
                <RankedBars
                  data={data.by_city.map((row) => ({
                    label: row.city,
                    count: row.count,
                    hint: row.country_code ?? undefined,
                  }))}
                  emptyMessage="No city data yet"
                />
              </Panel>
              <Panel title="Referrers" description="Where the scan traffic arrived from" className="lg:col-span-2">
                <RankedBars
                  data={data.by_referrer.map((row) => ({ label: row.referrer, count: row.count }))}
                  emptyMessage="Most camera-app scans send no referrer, so this is often empty"
                />
              </Panel>
            </div>
          )}

          {tab === "content" && (
            <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
              <Panel title="Top performing codes">
                {data.top_qr_codes.length ? (
                  <ul className="divide-y divide-zinc-800/60">
                    {data.top_qr_codes.map((row, index) => (
                      <li key={row.id}>
                        <Link
                          href={`/dashboard/qr-codes/${row.id}`}
                          className="flex items-center gap-3 py-2.5 transition-colors hover:text-white"
                        >
                          <span className="w-5 shrink-0 text-xs tabular-nums text-zinc-600">
                            {index + 1}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm text-zinc-200">
                              {row.title || row.short_code || "Untitled"}
                            </span>
                            <span className="block text-xs text-zinc-600">{row.qr_type}</span>
                          </span>
                          {!row.is_active && <Badge tone="warning">paused</Badge>}
                          <span className="shrink-0 tabular-nums text-sm text-zinc-300">
                            {formatNumber(row.scan_count)}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <EmptyState title="No scans yet" description="Top performers appear once your codes start getting scanned." />
                )}
              </Panel>
              <Panel title="By QR type" description="Codes created and scans earned per type">
                <RankedBars
                  data={data.by_type.map((row) => ({
                    label: row.qr_type,
                    count: row.scans,
                    hint: `${row.count} code${row.count === 1 ? "" : "s"}`,
                  }))}
                  colorize
                  emptyMessage="No codes created yet"
                />
              </Panel>
            </div>
          )}

          {tab === "activity" && (
            <Panel title="Recent scans" description="The last 25 scans across this workspace">
              {data.recent_scans.length ? (
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[560px] text-left text-sm">
                    <thead>
                      <tr className="border-b border-zinc-800/70">
                        {["When", "QR code", "Location", "Device"].map((header) => (
                          <th key={header} className="px-2 py-2 text-xs font-medium uppercase tracking-wide text-zinc-500">
                            {header}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {data.recent_scans.map((scan) => (
                        <tr key={scan.id} className="border-b border-zinc-800/40 last:border-0">
                          <td className="px-2 py-2.5 text-xs text-zinc-500">
                            {formatRelative(scan.scanned_at)}
                          </td>
                          <td className="max-w-[220px] px-2 py-2.5">
                            <Link href={`/dashboard/qr-codes/${scan.qr_id}`} className="block truncate text-zinc-200 hover:text-white">
                              {scan.qr_title || "Untitled"}
                            </Link>
                          </td>
                          <td className="px-2 py-2.5 text-zinc-400">
                            {[scan.city, scan.country].filter(Boolean).join(", ") || "Unknown"}
                          </td>
                          <td className="px-2 py-2.5 text-zinc-400">
                            <span className="capitalize">{scan.device || "unknown"}</span>
                            {scan.os && <span className="text-zinc-600"> · {scan.os}</span>}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <EmptyState
                  title="No scans in this period"
                  description="Scans show up here within moments of someone using one of your codes."
                />
              )}
            </Panel>
          )}
        </div>
      ) : null}
    </>
  );
}
