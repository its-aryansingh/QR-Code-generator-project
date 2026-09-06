"use client";

import { useState } from "react";
import Link from "next/link";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, ConfirmModal, EmptyState, ErrorState, Field, Grid, Modal, Panel,
  PageHeader, SearchInput, Select, StatCard, TextArea, TextInput, formatDate,
  formatNumber,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import { cn } from "@/lib/utils";
import type { Campaign, CampaignStatus } from "@/types/enterprise";

const STATUSES: CampaignStatus[] = [
  "draft", "scheduled", "active", "paused", "completed", "archived",
];

const STATUS_TONE: Record<CampaignStatus, "neutral" | "info" | "success" | "warning" | "violet"> = {
  draft: "neutral",
  scheduled: "info",
  active: "success",
  paused: "warning",
  completed: "violet",
  archived: "neutral",
};

interface FormState {
  name: string;
  description: string;
  status: CampaignStatus;
  color: string;
  starts_at: string;
  ends_at: string;
  scan_goal: string;
  utm_source: string;
  utm_medium: string;
  utm_campaign: string;
}

const EMPTY_FORM: FormState = {
  name: "", description: "", status: "draft", color: "#8B5CF6",
  starts_at: "", ends_at: "", scan_goal: "",
  utm_source: "", utm_medium: "qr", utm_campaign: "",
};

export default function CampaignsPage() {
  return (
    <WorkspaceGate requiredFeature="campaigns">
      <Campaigns />
    </WorkspaceGate>
  );
}

function Campaigns() {
  const { workspaceId, atLeast } = useWorkspace();
  const canEdit = atLeast("editor");

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [editing, setEditing] = useState<Campaign | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [deleting, setDeleting] = useState<Campaign | null>(null);

  const list = useResource(
    (id) => enterprise.listCampaigns(id, { status: statusFilter || undefined }),
    [statusFilter],
  );

  const close = () => {
    setFormOpen(false);
    setEditing(null);
    setForm(EMPTY_FORM);
  };

  const save = useMutation(
    async () => {
      const payload = {
        name: form.name.trim(),
        description: form.description || undefined,
        status: form.status,
        color: form.color,
        starts_at: form.starts_at || null,
        ends_at: form.ends_at || null,
        scan_goal: Number(form.scan_goal) || 0,
        utm: {
          source: form.utm_source || null,
          medium: form.utm_medium || null,
          campaign: form.utm_campaign || null,
        },
      };
      return editing
        ? enterprise.updateCampaign(workspaceId!, editing.id, payload as Partial<Campaign>)
        : enterprise.createCampaign(workspaceId!, payload as Partial<Campaign> & { name: string });
    },
    { onSuccess: () => { close(); list.reload(); } },
  );

  const remove = useMutation(
    (id: string) => enterprise.deleteCampaign(workspaceId!, id),
    { onSuccess: () => { setDeleting(null); list.reload(); } },
  );

  const openCreate = () => {
    setEditing(null);
    setForm(EMPTY_FORM);
    setFormOpen(true);
  };

  const openEdit = (campaign: Campaign) => {
    setEditing(campaign);
    setForm({
      name: campaign.name,
      description: campaign.description ?? "",
      status: campaign.status,
      color: campaign.color,
      starts_at: campaign.starts_at?.slice(0, 10) ?? "",
      ends_at: campaign.ends_at?.slice(0, 10) ?? "",
      scan_goal: String(campaign.scan_goal || ""),
      utm_source: campaign.utm.source ?? "",
      utm_medium: campaign.utm.medium ?? "",
      utm_campaign: campaign.utm.campaign ?? "",
    });
    setFormOpen(true);
  };

  const campaigns = (list.data ?? []).filter((campaign) =>
    !search || campaign.name.toLowerCase().includes(search.toLowerCase()),
  );

  const totals = (list.data ?? []).reduce(
    (acc, campaign) => ({
      scans: acc.scans + campaign.scan_count,
      codes: acc.codes + campaign.qr_count,
      active: acc.active + (campaign.status === "active" ? 1 : 0),
    }),
    { scans: 0, codes: 0, active: 0 },
  );

  return (
    <>
      <PageHeader
        title="Campaigns"
        description="Group codes by initiative, inherit UTM tags automatically, and measure against a goal."
        actions={canEdit && <Btn variant="primary" onClick={openCreate}>+ New campaign</Btn>}
      />

      <Grid cols={4} className="mb-5">
        <StatCard label="Campaigns" value={formatNumber(list.data?.length ?? 0)} />
        <StatCard label="Active now" value={formatNumber(totals.active)} />
        <StatCard label="Codes assigned" value={formatNumber(totals.codes)} />
        <StatCard label="Scans attributed" value={formatNumber(totals.scans)} />
      </Grid>

      <div className="mb-4 flex flex-col gap-2 sm:flex-row">
        <SearchInput value={search} onChange={setSearch} placeholder="Search campaigns…" className="sm:max-w-xs sm:flex-1" />
        <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="sm:w-44">
          <option value="">Any status</option>
          {STATUSES.map((status) => (
            <option key={status} value={status} className="capitalize">{status}</option>
          ))}
        </Select>
      </div>

      {list.error && <ErrorState message={list.error.message} onRetry={list.reload} />}

      {list.loading && !list.data ? (
        <Grid cols={3}>
          {[0, 1, 2].map((index) => (
            <div key={index} className="h-44 animate-pulse rounded-xl bg-zinc-900/60" />
          ))}
        </Grid>
      ) : campaigns.length ? (
        <Grid cols={3}>
          {campaigns.map((campaign) => (
            <CampaignCard
              key={campaign.id}
              campaign={campaign}
              canEdit={canEdit}
              onEdit={() => openEdit(campaign)}
              onDelete={() => setDeleting(campaign)}
            />
          ))}
        </Grid>
      ) : (
        <EmptyState
          icon={
            <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
          }
          title={search || statusFilter ? "No campaigns match" : "No campaigns yet"}
          description="A campaign groups codes under one initiative and stamps its UTM parameters onto every destination, so your web analytics and your scan analytics agree."
          action={canEdit && !search ? <Btn variant="primary" onClick={openCreate}>Create a campaign</Btn> : undefined}
        />
      )}

      <Modal
        open={formOpen}
        onClose={close}
        title={editing ? `Edit ${editing.name}` : "New campaign"}
        description="UTM values are applied to every URL code you assign to this campaign."
        width="lg"
        footer={
          <>
            <Btn variant="ghost" onClick={close}>Cancel</Btn>
            <Btn variant="primary" loading={save.busy} disabled={!form.name.trim()} onClick={() => save.run()}>
              {editing ? "Save changes" : "Create campaign"}
            </Btn>
          </>
        }
      >
        {save.error && <div className="mb-4"><ErrorState message={save.error} /></div>}
        <div className="space-y-4">
          <Field label="Campaign name" required>
            <TextInput
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="Spring product launch"
              autoFocus
            />
          </Field>
          <Field label="Description">
            <TextArea
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder="What is this campaign for?"
            />
          </Field>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Field label="Status">
              <Select
                value={form.status}
                onChange={(e) => setForm({ ...form, status: e.target.value as CampaignStatus })}
              >
                {STATUSES.map((status) => (
                  <option key={status} value={status} className="capitalize">{status}</option>
                ))}
              </Select>
            </Field>
            <Field label="Starts">
              <TextInput
                type="date"
                value={form.starts_at}
                onChange={(e) => setForm({ ...form, starts_at: e.target.value })}
              />
            </Field>
            <Field label="Ends">
              <TextInput
                type="date"
                value={form.ends_at}
                onChange={(e) => setForm({ ...form, ends_at: e.target.value })}
              />
            </Field>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Scan goal" hint="Progress is tracked against this on the card.">
              <TextInput
                type="number"
                min={0}
                value={form.scan_goal}
                onChange={(e) => setForm({ ...form, scan_goal: e.target.value })}
                placeholder="10000"
              />
            </Field>
            <Field label="Colour">
              <div className="flex gap-2">
                <input
                  type="color"
                  value={form.color}
                  onChange={(e) => setForm({ ...form, color: e.target.value })}
                  className="h-9 w-12 cursor-pointer rounded-lg border border-zinc-800 bg-zinc-950"
                />
                <TextInput value={form.color} onChange={(e) => setForm({ ...form, color: e.target.value })} />
              </div>
            </Field>
          </div>
          <div className="rounded-lg border border-zinc-800 bg-zinc-950/60 p-4">
            <p className="mb-3 text-xs font-medium text-zinc-300">UTM parameters</p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Field label="Source">
                <TextInput
                  value={form.utm_source}
                  onChange={(e) => setForm({ ...form, utm_source: e.target.value })}
                  placeholder="packaging"
                />
              </Field>
              <Field label="Medium">
                <TextInput
                  value={form.utm_medium}
                  onChange={(e) => setForm({ ...form, utm_medium: e.target.value })}
                  placeholder="qr"
                />
              </Field>
              <Field label="Campaign">
                <TextInput
                  value={form.utm_campaign}
                  onChange={(e) => setForm({ ...form, utm_campaign: e.target.value })}
                  placeholder="spring-2026"
                />
              </Field>
            </div>
          </div>
        </div>
      </Modal>

      <ConfirmModal
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.run(deleting.id)}
        busy={remove.busy}
        title={`Delete ${deleting?.name}?`}
        message="The codes assigned to this campaign keep working — they are simply unlinked and stop being attributed to it."
      />
    </>
  );
}

function CampaignCard({
  campaign,
  canEdit,
  onEdit,
  onDelete,
}: {
  campaign: Campaign;
  canEdit: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const progress = campaign.goal_progress;

  return (
    <div className="flex flex-col rounded-xl border border-zinc-800/70 bg-zinc-900/40 p-4 transition-colors hover:border-zinc-700">
      <div className="flex items-start gap-2.5">
        <span
          className="mt-1 size-2.5 shrink-0 rounded-full"
          style={{ background: campaign.color }}
        />
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-zinc-100">{campaign.name}</p>
          {campaign.description && (
            <p className="mt-0.5 line-clamp-2 text-xs text-zinc-500">{campaign.description}</p>
          )}
        </div>
        <Badge tone={STATUS_TONE[campaign.status]} className="capitalize">{campaign.status}</Badge>
      </div>

      <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
        <div>
          <dt className="text-xs text-zinc-600">Codes</dt>
          <dd className="tabular-nums text-zinc-200">{formatNumber(campaign.qr_count)}</dd>
        </div>
        <div>
          <dt className="text-xs text-zinc-600">Scans</dt>
          <dd className="tabular-nums text-zinc-200">{formatNumber(campaign.scan_count)}</dd>
        </div>
      </dl>

      {typeof progress === "number" && (
        <div className="mt-3">
          <div className="mb-1 flex items-baseline justify-between text-xs">
            <span className="text-zinc-500">Goal</span>
            <span className="tabular-nums text-zinc-400">
              {formatNumber(campaign.scan_count)} / {formatNumber(campaign.scan_goal)}
            </span>
          </div>
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-800/70">
            <div
              className={cn("h-full rounded-full transition-[width] duration-500")}
              style={{ width: `${Math.max(1.5, progress)}%`, background: campaign.color }}
            />
          </div>
        </div>
      )}

      {(campaign.starts_at || campaign.ends_at) && (
        <p className="mt-3 text-xs text-zinc-600">
          {formatDate(campaign.starts_at)} → {formatDate(campaign.ends_at)}
        </p>
      )}

      {campaign.utm.source && (
        <p className="mt-2 truncate font-mono text-[11px] text-zinc-600">
          ?utm_source={campaign.utm.source}
          {campaign.utm.medium && `&utm_medium=${campaign.utm.medium}`}
        </p>
      )}

      <div className="mt-4 flex items-center gap-1 border-t border-zinc-800/60 pt-3">
        <Link href={`/dashboard/qr-codes?campaign_id=${campaign.id}`} className="flex-1">
          <Btn size="sm" variant="ghost" className="w-full">View codes</Btn>
        </Link>
        {canEdit && (
          <>
            <Btn size="sm" variant="ghost" onClick={onEdit}>Edit</Btn>
            <Btn size="sm" variant="ghost" className="text-red-400 hover:bg-red-500/10" onClick={onDelete}>
              Delete
            </Btn>
          </>
        )}
      </div>
    </div>
  );
}
