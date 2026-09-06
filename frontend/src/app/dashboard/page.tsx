"use client";

import { useState } from "react";
import Link from "next/link";

import { RankedBars, ShareBar, TrendChart } from "@/components/enterprise/charts";
import { useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, EmptyState, ErrorState, Grid, LoadingPanel, Panel, PageHeader,
  Select, StatCard, StatusDot, formatNumber, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";

const RANGES = [7, 14, 30, 90, 365];

export default function DashboardPage() {
  return (
    <WorkspaceGate>
      <Overview />
    </WorkspaceGate>
  );
}

function Overview() {
  const [days, setDays] = useState(30);
  const { workspace, plan, limit, atLeast } = useWorkspace();
  const { data, loading, error, reload } = useResource(
    (id) => enterprise.overview(id, days),
    [days],
  );

  const qrCap = limit("max_qr_codes");
  const usedShare = data && qrCap ? Math.min(100, (data.stats.total_qr_codes / qrCap) * 100) : 0;

  return (
    <>
      <PageHeader
        title={workspace?.name ?? "Overview"}
        description="Everything happening across this workspace right now."
        actions={
          <>
            <Select value={days} onChange={(e) => setDays(Number(e.target.value))} className="w-36">
              {RANGES.map((value) => (
                <option key={value} value={value}>Last {value} days</option>
              ))}
            </Select>
            <Link href="/dashboard/analytics">
              <Btn variant="outline">Full analytics</Btn>
            </Link>
          </>
        }
      />

      {error && <ErrorState message={error.message} onRetry={reload} />}

      {loading && !data ? (
        <LoadingPanel rows={8} />
      ) : data ? (
        <div className="space-y-5">
          <Grid cols={4}>
            <StatCard
              label="Scans"
              value={formatNumber(data.stats.total_scans)}
              delta={data.stats.scans_delta}
              hint={`last ${days} days`}
            />
            <StatCard
              label="Unique visitors"
              value={formatNumber(data.stats.unique_visitors)}
              hint="distinct devices"
            />
            <StatCard
              label="QR codes"
              value={formatNumber(data.stats.total_qr_codes)}
              hint={`${formatNumber(data.stats.active_qr_codes)} active · ${formatNumber(data.stats.dynamic_qr_codes)} dynamic`}
            />
            <StatCard
              label="Active campaigns"
              value={formatNumber(data.stats.active_campaigns)}
              hint={`${formatNumber(data.stats.new_leads)} new leads`}
              href="/dashboard/campaigns"
            />
          </Grid>

          <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
            <Panel
              title="Scan volume"
              description={`Daily scans over the last ${days} days`}
              className="lg:col-span-2"
            >
              <TrendChart data={data.scans_by_date} />
            </Panel>

            <Panel title="Devices" description="What people scan with">
              <ShareBar
                data={data.by_device.map((row) => ({
                  label: row.device || "Unknown",
                  count: row.count,
                }))}
                emptyMessage="No scans recorded yet"
              />
            </Panel>
          </div>

          <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <Panel
              title="Top performers"
              description="Most scanned codes, all time"
              actions={
                <Link href="/dashboard/qr-codes" className="text-xs text-zinc-400 hover:text-zinc-200">
                  View all
                </Link>
              }
            >
              {data.top_qr_codes.length ? (
                <RankedBars
                  data={data.top_qr_codes.map((row) => ({
                    label: row.title || row.short_code || "Untitled",
                    count: row.scan_count,
                    hint: row.qr_type,
                  }))}
                />
              ) : (
                <EmptyState
                  title="No scans yet"
                  description="Once your codes get scanned, your best performers show up here."
                />
              )}
            </Panel>

            <Panel
              title="Recently created"
              description="The newest codes in this workspace"
              actions={
                atLeast("editor") ? (
                  <Link href="/dashboard/create" className="text-xs text-zinc-400 hover:text-zinc-200">
                    New code
                  </Link>
                ) : undefined
              }
            >
              {data.recent_qr_codes.length ? (
                <ul className="divide-y divide-zinc-800/60">
                  {data.recent_qr_codes.map((row) => (
                    <li key={row.id}>
                      <Link
                        href={`/dashboard/qr-codes/${row.id}`}
                        className="flex items-center gap-3 py-2.5 transition-colors hover:text-white"
                      >
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm text-zinc-200">
                            {row.title || "Untitled"}
                          </span>
                          <span className="block truncate text-xs text-zinc-600">
                            {row.qr_type} · {formatRelative(row.created_at)}
                          </span>
                        </span>
                        <Badge tone="neutral">{formatNumber(row.scan_count)} scans</Badge>
                      </Link>
                    </li>
                  ))}
                </ul>
              ) : (
                <EmptyState
                  title="No QR codes yet"
                  description="Create your first code to start tracking scans."
                  action={
                    atLeast("editor") ? (
                      <Link href="/dashboard/create">
                        <Btn variant="primary">Create a QR code</Btn>
                      </Link>
                    ) : undefined
                  }
                />
              )}
            </Panel>
          </div>

          <Panel title="Plan usage" description={`You are on the ${plan} plan`}>
            <div className="space-y-3">
              <div>
                <div className="mb-1.5 flex items-baseline justify-between text-xs">
                  <span className="text-zinc-400">QR codes</span>
                  <span className="tabular-nums text-zinc-300">
                    {formatNumber(data.stats.total_qr_codes)} of {formatNumber(qrCap)}
                  </span>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-800/70">
                  <div
                    className="h-full rounded-full bg-violet-500 transition-[width] duration-500"
                    style={{ width: `${Math.max(1.5, usedShare)}%` }}
                  />
                </div>
              </div>
              <div className="flex items-center justify-between pt-1">
                <StatusDot active label={`Workspace ${workspace?.slug ?? ""}`} />
                <Link href="/pricing" className="text-xs text-violet-400 hover:text-violet-300">
                  Compare plans
                </Link>
              </div>
            </div>
          </Panel>
        </div>
      ) : null}
    </>
  );
}
