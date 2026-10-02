"use client";

import { useEffect, useMemo, useState } from "react";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Avatar, Badge, Btn, EmptyState, ErrorState, LoadingPanel, Pagination, Panel,
  PageHeader, SearchInput, Select, formatDate, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { AuditEntry } from "@/types/enterprise";

type Tone = "success" | "info" | "warning" | "danger" | "violet" | "neutral";

const ACTION_TONE: Record<string, Tone> = {
  create: "success",
  update: "info",
  delete: "danger",
  remove: "danger",
  revoke: "danger",
  invite: "violet",
  join: "success",
  role_change: "warning",
  export: "info",
  test: "neutral",
  login: "neutral",
};

/** Turns a row into a sentence, so the log reads as a story rather than a
 *  table of enum values. */
function describe(entry: AuditEntry) {
  const who = entry.user?.name || entry.user?.email || "Someone";
  const details = entry.details ?? {};
  const name = typeof details.name === "string" ? details.name : undefined;
  const resource = entry.resource.replace(/_/g, " ");

  switch (entry.action) {
    case "create":
      return `${who} created ${resource}${name ? ` “${name}”` : ""}`;
    case "update": {
      const fields = Array.isArray(details.fields) ? details.fields.join(", ") : null;
      return `${who} updated ${resource}${fields ? ` (${fields})` : ""}`;
    }
    case "delete":
      return `${who} deleted ${resource}${name ? ` “${name}”` : ""}`;
    case "invite":
      return `${who} invited ${details.email ?? "someone"} as ${details.role ?? "a member"}`;
    case "join":
      return `${who} joined the workspace as ${details.role ?? "a member"}`;
    case "remove":
      return `${who} removed ${details.email ?? "a member"}`;
    case "revoke":
      return `${who} revoked ${resource}${details.email ? ` for ${details.email}` : ""}`;
    case "role_change":
      return `${who} changed a member's role from ${details.from} to ${details.to}`;
    case "export":
      return `${who} exported ${resource} data`;
    case "test":
      return `${who} sent a test ${resource} delivery`;
    default:
      return `${who} performed ${entry.action} on ${resource}`;
  }
}

export default function AuditLogPage() {
  return (
    <WorkspaceGate requiredRole="admin" requiredFeature="audit_log">
      <AuditLog />
    </WorkspaceGate>
  );
}

function AuditLog() {
  const { workspaceId, can } = useWorkspace();
  const [search, setSearch] = useState("");
  const [debounced, setDebounced] = useState("");
  const [action, setAction] = useState("");
  const [resource, setResource] = useState("");
  const [days, setDays] = useState("");
  const [page, setPage] = useState(1);

  useEffect(() => {
    const timer = setTimeout(() => { setDebounced(search); setPage(1); }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  const query = useMemo(
    () => ({
      page,
      limit: 50,
      search: debounced || undefined,
      action: action || undefined,
      resource: resource || undefined,
      days: days || undefined,
    }),
    [page, debounced, action, resource, days],
  );

  const { data, loading, error, reload } = useResource(
    (id) => enterprise.auditLog(id, query),
    [query],
  );

  const exportCsv = useMutation(() => enterprise.exportCsv(workspaceId!, "audit", { days: days || 365 }));

  const filtered = [debounced, action, resource, days].filter(Boolean).length;

  return (
    <>
      <PageHeader
        title="Audit log"
        description="An immutable record of every change made in this workspace — who did what, from where, and when."
        actions={
          can("exports") && (
            <Btn variant="outline" loading={exportCsv.busy} onClick={() => exportCsv.run()}>
              Export CSV
            </Btn>
          )
        }
      />

      {error && <ErrorState message={error.message} onRetry={reload} />}
      {exportCsv.error && <div className="mb-4"><ErrorState message={exportCsv.error} /></div>}

      <Panel bodyClassName="p-4">
        <div className="mb-4 flex flex-col gap-2 lg:flex-row">
          <SearchInput
            value={search}
            onChange={setSearch}
            placeholder="Search by action, resource or person…"
            className="lg:max-w-sm lg:flex-1"
          />
          <div className="flex flex-wrap gap-2">
            <Select value={action} onChange={(e) => { setAction(e.target.value); setPage(1); }} className="w-auto min-w-32">
              <option value="">Any action</option>
              {(data?.actions ?? []).map((value) => (
                <option key={value} value={value} className="capitalize">
                  {value.replace(/_/g, " ")}
                </option>
              ))}
            </Select>
            <Select value={resource} onChange={(e) => { setResource(e.target.value); setPage(1); }} className="w-auto min-w-32">
              <option value="">Any resource</option>
              {(data?.resources ?? []).map((value) => (
                <option key={value} value={value} className="capitalize">
                  {value.replace(/_/g, " ")}
                </option>
              ))}
            </Select>
            <Select value={days} onChange={(e) => { setDays(e.target.value); setPage(1); }} className="w-auto min-w-32">
              <option value="">All time</option>
              <option value="1">Last 24 hours</option>
              <option value="7">Last 7 days</option>
              <option value="30">Last 30 days</option>
              <option value="90">Last 90 days</option>
            </Select>
            {filtered > 0 && (
              <Btn
                variant="ghost"
                onClick={() => {
                  setSearch(""); setAction(""); setResource(""); setDays(""); setPage(1);
                }}
              >
                Clear ({filtered})
              </Btn>
            )}
          </div>
        </div>

        {loading && !data ? (
          <LoadingPanel rows={8} />
        ) : data && data.items.length ? (
          <>
            <ol className="relative space-y-0 border-l border-zinc-800/70 pl-0">
              {data.items.map((entry) => (
                <li key={entry.id} className="relative flex gap-3 py-3 pl-5">
                  <span
                    className="absolute -left-[5px] top-[22px] size-2.5 rounded-full border-2 border-zinc-950"
                    style={{
                      background:
                        ACTION_TONE[entry.action] === "danger" ? "#e66767"
                        : ACTION_TONE[entry.action] === "success" ? "#199e70"
                        : ACTION_TONE[entry.action] === "warning" ? "#c98500"
                        : "#52525b",
                    }}
                  />
                  <Avatar
                    name={entry.user?.name}
                    email={entry.user?.email}
                    url={entry.user?.avatar_url}
                    size={30}
                  />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm text-zinc-200">{describe(entry)}</p>
                    <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-zinc-600">
                      <Badge tone={ACTION_TONE[entry.action] ?? "neutral"}>
                        {entry.action.replace(/_/g, " ")}
                      </Badge>
                      <span title={formatDate(entry.created_at, true)}>
                        {formatRelative(entry.created_at)}
                      </span>
                      {entry.ip_address && <span>· {entry.ip_address}</span>}
                      {entry.resource_id && (
                        <span className="font-mono">· {String(entry.resource_id).slice(0, 8)}</span>
                      )}
                    </p>
                  </div>
                </li>
              ))}
            </ol>
            <Pagination page={data.page} pages={data.pages} total={data.total} onPage={setPage} />
          </>
        ) : (
          <EmptyState
            icon={
              <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            }
            title={filtered ? "No entries match these filters" : "No activity recorded yet"}
            description={
              filtered
                ? "Try clearing a filter or widening the date range."
                : "Every create, update, delete, invite and export in this workspace is recorded here as it happens."
            }
          />
        )}
      </Panel>
    </>
  );
}
