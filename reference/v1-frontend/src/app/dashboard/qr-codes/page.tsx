"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, Column, ConfirmModal, CopyButton, DataTable, EmptyState, ErrorState,
  Grid, Modal, Pagination, Panel, PageHeader, SearchInput, Select, StatCard,
  StatusDot, TextInput, formatNumber, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { Campaign, Folder, WorkspaceQR } from "@/types/enterprise";

const SORTS = [
  { value: "created", label: "Newest first" },
  { value: "created_asc", label: "Oldest first" },
  { value: "scans", label: "Most scanned" },
  { value: "scans_asc", label: "Least scanned" },
  { value: "title", label: "Title A–Z" },
];

export default function QRCodesPage() {
  return (
    <WorkspaceGate>
      <QRCodes />
    </WorkspaceGate>
  );
}

/** Flattens the folder tree into indented options for the filter select. */
function flattenFolders(folders: Folder[], depth = 0): Array<{ id: string; label: string }> {
  return folders.flatMap((folder) => [
    { id: folder.id, label: `${"— ".repeat(depth)}${folder.name}` },
    ...flattenFolders(folder.children ?? [], depth + 1),
  ]);
}

function QRCodes() {
  const { workspaceId, atLeast, can } = useWorkspace();
  const canEdit = atLeast("editor");
  const canDelete = atLeast("admin");

  const [search, setSearch] = useState("");
  const [debounced, setDebounced] = useState("");
  const [folderId, setFolderId] = useState("");
  const [campaignId, setCampaignId] = useState("");
  const [type, setType] = useState("");
  const [status, setStatus] = useState("");
  const [sort, setSort] = useState("created");
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string[]>([]);
  const [moveOpen, setMoveOpen] = useState(false);
  const [tagOpen, setTagOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [tagValue, setTagValue] = useState("");
  const [moveTarget, setMoveTarget] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(search);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  const query = useMemo(
    () => ({
      page,
      limit: 25,
      sort,
      search: debounced || undefined,
      folder_id: folderId || undefined,
      campaign_id: campaignId || undefined,
      type: type || undefined,
      status: status || undefined,
    }),
    [page, sort, debounced, folderId, campaignId, type, status],
  );

  const list = useResource((id) => enterprise.listQR(id, query), [query]);
  const folders = useResource((id) => enterprise.listFolders(id), []);
  const campaigns = useResource(
    (id) => (can("campaigns") ? enterprise.listCampaigns(id) : Promise.resolve([] as Campaign[])),
    [],
  );

  const afterAction = () => {
    setSelected([]);
    setMoveOpen(false);
    setTagOpen(false);
    setDeleteOpen(false);
    setTagValue("");
    list.reload();
  };

  const bulk = useMutation(
    (body: Parameters<typeof enterprise.bulkAction>[1]) =>
      enterprise.bulkAction(workspaceId!, body),
    { onSuccess: afterAction },
  );

  const toggle = useMutation((qrId: string) => enterprise.toggleQR(qrId), {
    onSuccess: () => list.reload(),
  });

  const folderOptions = flattenFolders(folders.data ?? []);
  const rows = list.data?.items ?? [];

  const columns: Column<WorkspaceQR>[] = [
    {
      key: "title",
      header: "QR code",
      cell: (row) => (
        <div className="min-w-0">
          <p className="truncate font-medium text-zinc-100">{row.title || "Untitled"}</p>
          <p className="truncate text-xs text-zinc-600">{row.content}</p>
        </div>
      ),
      className: "max-w-[280px]",
    },
    {
      key: "type",
      header: "Type",
      cell: (row) => (
        <div className="flex flex-wrap items-center gap-1">
          <Badge tone="neutral">{row.qr_type}</Badge>
          {row.is_dynamic && <Badge tone="violet">dynamic</Badge>}
          {row.qr_type === "gs1" && <Badge tone="info">GS1</Badge>}
        </div>
      ),
    },
    {
      key: "location",
      header: "Folder / campaign",
      cell: (row) => (
        <div className="text-xs">
          <p className="text-zinc-400">{row.folder_name || "—"}</p>
          {row.campaign_name && <p className="text-violet-400">{row.campaign_name}</p>}
        </div>
      ),
    },
    {
      key: "scans",
      header: "Scans",
      cell: (row) => (
        <span className="tabular-nums text-zinc-200">{formatNumber(row.scan_count)}</span>
      ),
    },
    {
      key: "link",
      header: "Short link",
      cell: (row) =>
        row.short_url ? (
          <div className="flex items-center gap-1">
            <code className="truncate text-xs text-zinc-400">{row.short_code}</code>
            <CopyButton value={row.short_url} label="" />
          </div>
        ) : (
          <span className="text-xs text-zinc-600">static</span>
        ),
    },
    {
      key: "status",
      header: "Status",
      cell: (row) => <StatusDot active={row.is_active} />,
    },
    {
      key: "created",
      header: "Created",
      cell: (row) => <span className="text-xs text-zinc-500">{formatRelative(row.created_at)}</span>,
    },
    {
      key: "actions",
      header: "",
      cell: (row) => (
        <div className="flex items-center justify-end gap-1">
          <Link href={`/dashboard/qr-codes/${row.id}`}>
            <Btn size="sm" variant="ghost">Open</Btn>
          </Link>
          {canEdit && (
            <Btn
              size="sm"
              variant="ghost"
              onClick={() => toggle.run(row.id)}
              title={row.is_active ? "Pause this code" : "Reactivate this code"}
            >
              {row.is_active ? "Pause" : "Resume"}
            </Btn>
          )}
        </div>
      ),
      headerClassName: "text-right",
      className: "text-right",
    },
  ];

  const activeFilters = [folderId, campaignId, type, status, debounced].filter(Boolean).length;

  return (
    <>
      <PageHeader
        title="QR codes"
        description="Every code in this workspace, visible to the whole team."
        actions={
          canEdit && (
            <Link href="/dashboard/create">
              <Btn variant="primary">+ New QR code</Btn>
            </Link>
          )
        }
      />

      {list.data && (
        <Grid cols={4} className="mb-5">
          <StatCard label="Total codes" value={formatNumber(list.data.total)} />
          <StatCard label="Lifetime scans" value={formatNumber(list.data.facets.total_scans)} />
          <StatCard label="Folders" value={formatNumber(folderOptions.length)} href="/dashboard/campaigns" />
          <StatCard
            label="Types in use"
            value={formatNumber(list.data.facets.types.length)}
            hint={list.data.facets.types.slice(0, 3).map((t) => t.qr_type).join(", ")}
          />
        </Grid>
      )}

      <Panel bodyClassName="p-4">
        <div className="mb-4 flex flex-col gap-2 lg:flex-row lg:items-center">
          <SearchInput
            value={search}
            onChange={setSearch}
            placeholder="Search by title, destination or short code…"
            className="lg:max-w-sm lg:flex-1"
          />
          <div className="flex flex-wrap gap-2">
            <Select
              value={folderId}
              onChange={(e) => { setFolderId(e.target.value); setPage(1); }}
              className="w-auto min-w-36"
            >
              <option value="">All folders</option>
              {folderOptions.map((folder) => (
                <option key={folder.id} value={folder.id}>{folder.label}</option>
              ))}
            </Select>
            {can("campaigns") && (
              <Select
                value={campaignId}
                onChange={(e) => { setCampaignId(e.target.value); setPage(1); }}
                className="w-auto min-w-36"
              >
                <option value="">All campaigns</option>
                {(campaigns.data ?? []).map((campaign) => (
                  <option key={campaign.id} value={campaign.id}>{campaign.name}</option>
                ))}
              </Select>
            )}
            <Select
              value={type}
              onChange={(e) => { setType(e.target.value); setPage(1); }}
              className="w-auto min-w-28"
            >
              <option value="">All types</option>
              {(list.data?.facets.types ?? []).map((row) => (
                <option key={row.qr_type} value={row.qr_type}>
                  {row.qr_type} ({row.count})
                </option>
              ))}
            </Select>
            <Select
              value={status}
              onChange={(e) => { setStatus(e.target.value); setPage(1); }}
              className="w-auto min-w-28"
            >
              <option value="">Any status</option>
              <option value="active">Active</option>
              <option value="inactive">Paused</option>
              <option value="expired">Expired</option>
            </Select>
            <Select value={sort} onChange={(e) => setSort(e.target.value)} className="w-auto min-w-36">
              {SORTS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>
            {activeFilters > 0 && (
              <Btn
                variant="ghost"
                onClick={() => {
                  setSearch(""); setFolderId(""); setCampaignId("");
                  setType(""); setStatus(""); setPage(1);
                }}
              >
                Clear ({activeFilters})
              </Btn>
            )}
          </div>
        </div>

        {selected.length > 0 && canEdit && (
          <div className="mb-3 flex flex-wrap items-center gap-2 rounded-lg border border-violet-800/40 bg-violet-950/20 px-3 py-2">
            <span className="text-sm text-violet-200">
              {selected.length} selected
            </span>
            <div className="ml-auto flex flex-wrap gap-1.5">
              <Btn size="sm" variant="outline" onClick={() => setMoveOpen(true)}>Move</Btn>
              <Btn size="sm" variant="outline" onClick={() => setTagOpen(true)}>Tag</Btn>
              <Btn
                size="sm"
                variant="outline"
                onClick={() => bulk.run({ action: "activate", qr_ids: selected })}
              >
                Activate
              </Btn>
              <Btn
                size="sm"
                variant="outline"
                onClick={() => bulk.run({ action: "deactivate", qr_ids: selected })}
              >
                Pause
              </Btn>
              {canDelete && (
                <Btn size="sm" variant="danger" onClick={() => setDeleteOpen(true)}>Delete</Btn>
              )}
              <Btn size="sm" variant="ghost" onClick={() => setSelected([])}>Clear</Btn>
            </div>
          </div>
        )}

        {bulk.error && <div className="mb-3"><ErrorState message={bulk.error} /></div>}
        {list.error && <ErrorState message={list.error.message} onRetry={list.reload} />}

        <DataTable
          columns={columns}
          rows={rows}
          loading={list.loading && !list.data}
          selectable={canEdit}
          selected={selected}
          onSelectedChange={setSelected}
          empty={
            <EmptyState
              title={activeFilters ? "No codes match these filters" : "No QR codes yet"}
              description={
                activeFilters
                  ? "Try widening the filters or clearing the search."
                  : "Create your first code, or import a batch from a spreadsheet."
              }
              action={
                canEdit && !activeFilters ? (
                  <div className="flex gap-2">
                    <Link href="/dashboard/create"><Btn variant="primary">Create a code</Btn></Link>
                    <Link href="/dashboard/bulk"><Btn variant="outline">Bulk import</Btn></Link>
                  </div>
                ) : undefined
              }
            />
          }
        />

        {list.data && (
          <Pagination
            page={list.data.page}
            pages={list.data.pages}
            total={list.data.total}
            onPage={setPage}
          />
        )}
      </Panel>

      <Modal
        open={moveOpen}
        onClose={() => setMoveOpen(false)}
        title={`Move ${selected.length} code${selected.length === 1 ? "" : "s"}`}
        description="Pick the folder these codes should live in."
        footer={
          <>
            <Btn variant="ghost" onClick={() => setMoveOpen(false)}>Cancel</Btn>
            <Btn
              variant="primary"
              loading={bulk.busy}
              onClick={() => bulk.run({ action: "move", qr_ids: selected, folder_id: moveTarget || null })}
            >
              Move
            </Btn>
          </>
        }
      >
        <Select value={moveTarget} onChange={(e) => setMoveTarget(e.target.value)}>
          <option value="">No folder (top level)</option>
          {folderOptions.map((folder) => (
            <option key={folder.id} value={folder.id}>{folder.label}</option>
          ))}
        </Select>
      </Modal>

      <Modal
        open={tagOpen}
        onClose={() => setTagOpen(false)}
        title={`Tag ${selected.length} code${selected.length === 1 ? "" : "s"}`}
        description="Comma-separated. Existing tags are kept."
        footer={
          <>
            <Btn variant="ghost" onClick={() => setTagOpen(false)}>Cancel</Btn>
            <Btn
              variant="primary"
              loading={bulk.busy}
              disabled={!tagValue.trim()}
              onClick={() =>
                bulk.run({
                  action: "tag",
                  qr_ids: selected,
                  tags: tagValue.split(",").map((tag) => tag.trim()).filter(Boolean),
                })
              }
            >
              Apply tags
            </Btn>
          </>
        }
      >
        <TextInput
          value={tagValue}
          onChange={(e) => setTagValue(e.target.value)}
          placeholder="print, retail, q1-2026"
          autoFocus
        />
      </Modal>

      <ConfirmModal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        onConfirm={() => bulk.run({ action: "delete", qr_ids: selected })}
        busy={bulk.busy}
        title={`Delete ${selected.length} code${selected.length === 1 ? "" : "s"}?`}
        message="Anyone who scans these codes will hit a dead link. Printed material using them will stop working. This cannot be undone."
        confirmLabel="Delete permanently"
      />
    </>
  );
}
