"use client";

import { useState } from "react";
import { ExternalLink, FileText, Inbox, Trash2 } from "lucide-react";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, ConfirmModal, DataTable, EmptyState, ErrorState, Field, Grid, LoadingPanel,
  Modal, PageHeader, Pagination, StatCard, Tabs, TextInput, Toggle, formatDate, formatNumber,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { Lead, LeadPage } from "@/types/enterprise";

interface PageForm {
  name: string;
  headline: string;
  subheadline: string;
  button_text: string;
  button_color: string;
  background_color: string;
  text_color: string;
  thank_you_message: string;
  requires_opt_in: boolean;
}

const EMPTY_PAGE: PageForm = {
  name: "",
  headline: "",
  subheadline: "",
  button_text: "Submit",
  button_color: "#8B5CF6",
  background_color: "#09090B",
  text_color: "#FAFAFA",
  thank_you_message: "Thank you for signing up!",
  requires_opt_in: false,
};

const DEFAULT_FIELDS = [
  { name: "email", type: "email", required: true, label: "Email address" },
  { name: "name", type: "text", required: false, label: "Full name" },
];

export default function LeadsPage() {
  return (
    <WorkspaceGate>
      <Leads />
    </WorkspaceGate>
  );
}

function Leads() {
  const { workspaceId, atLeast } = useWorkspace();
  const canEdit = atLeast("editor");
  const [tab, setTab] = useState<"pages" | "leads">("pages");
  const [leadPage, setLeadPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState<PageForm>(EMPTY_PAGE);
  const [deleting, setDeleting] = useState<LeadPage | null>(null);

  const pages = useResource((id) => enterprise.listLeadPages(id), []);
  const leads = useResource(
    (id) => enterprise.listLeads(id, { page: leadPage, limit: 25 }),
    [leadPage],
  );

  const create = useMutation(
    () =>
      enterprise.createLeadPage(workspaceId!, {
        ...form,
        name: form.name.trim(),
        form_fields: DEFAULT_FIELDS,
      }),
    {
      onSuccess: () => {
        setCreateOpen(false);
        setForm(EMPTY_PAGE);
        pages.reload();
      },
    },
  );

  const toggle = useMutation(
    (page: LeadPage) => enterprise.updateLeadPage(workspaceId!, page.id, { is_active: !page.is_active }),
    { onSuccess: () => pages.reload() },
  );

  const remove = useMutation(
    (id: string) => enterprise.deleteLeadPage(workspaceId!, id),
    { onSuccess: () => { setDeleting(null); pages.reload(); leads.reload(); } },
  );

  const rows = pages.data ?? [];
  const totalViews = rows.reduce((sum, page) => sum + page.views, 0);
  const totalLeads = rows.reduce((sum, page) => sum + page.lead_count, 0);
  const conversion = totalViews > 0 ? ((totalLeads / totalViews) * 100).toFixed(1) : "0";

  return (
    <>
      <PageHeader
        title="Lead capture"
        description="Landing pages that collect leads from your QR codes. Answers are stored encrypted."
        actions={
          canEdit && (
            <Btn variant="primary" onClick={() => setCreateOpen(true)} disabled={!pages.data}>
              + New page
            </Btn>
          )
        }
      />

      <Grid cols={3} className="mb-5">
        <StatCard label="Pages" value={formatNumber(rows.length)} />
        <StatCard label="Page views" value={formatNumber(totalViews)} />
        <StatCard label="Conversion" value={`${conversion}%`} hint={`${formatNumber(totalLeads)} ${totalLeads === 1 ? "lead" : "leads"}`} />
      </Grid>

      <Tabs
        active={tab}
        onChange={(id) => setTab(id as "pages" | "leads")}
        tabs={[
          { id: "pages", label: "Pages", count: rows.length },
          { id: "leads", label: "Leads", count: leads.data?.total },
        ]}
      />

      {tab === "pages" && (
        pages.error ? (
          <ErrorState message={pages.error.message} onRetry={pages.reload} />
        ) : pages.loading && !pages.data ? (
          <LoadingPanel rows={3} />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<FileText className="size-5" />}
            title="No lead pages yet"
            description="Create a page, then point a QR code at its link to start collecting leads."
            action={canEdit && <Btn variant="primary" onClick={() => setCreateOpen(true)}>+ New page</Btn>}
          />
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {rows.map((page) => (
              <article key={page.id} className="rounded-xl border border-zinc-800/70 bg-zinc-900/40 p-5">
                <div className="mb-3 flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="truncate text-sm font-semibold text-zinc-100">{page.name}</h3>
                    {page.headline && <p className="mt-0.5 truncate text-xs text-zinc-400">{page.headline}</p>}
                  </div>
                  <Badge tone={page.is_active ? "success" : "neutral"}>{page.is_active ? "Live" : "Paused"}</Badge>
                </div>
                <dl className="mb-4 grid grid-cols-3 gap-2 text-xs">
                  <div><dt className="text-zinc-400">Views</dt><dd className="font-medium tabular-nums text-zinc-100">{formatNumber(page.views)}</dd></div>
                  <div><dt className="text-zinc-400">Leads</dt><dd className="font-medium tabular-nums text-zinc-100">{formatNumber(page.lead_count)}</dd></div>
                  <div><dt className="text-zinc-400">Rate</dt><dd className="font-medium tabular-nums text-zinc-100">{page.conversion_rate}%</dd></div>
                </dl>
                <div className="flex flex-wrap items-center gap-2">
                  <a
                    href={`/p/${page.slug}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex min-w-0 flex-1 items-center gap-1 truncate text-xs text-violet-400 hover:text-violet-300"
                  >
                    <ExternalLink className="size-3.5 shrink-0" aria-hidden />
                    <span className="truncate">/p/{page.slug}</span>
                  </a>
                  {canEdit && (
                    <>
                      <Btn size="sm" variant="outline" onClick={() => toggle.run(page)} loading={toggle.busy}>
                        {page.is_active ? "Pause" : "Publish"}
                      </Btn>
                      <Btn size="sm" variant="ghost" onClick={() => setDeleting(page)} aria-label={`Delete ${page.name}`}>
                        <Trash2 className="size-4" aria-hidden />
                      </Btn>
                    </>
                  )}
                </div>
              </article>
            ))}
          </div>
        )
      )}

      {tab === "leads" && (
        leads.error ? (
          <ErrorState message={leads.error.message} onRetry={leads.reload} />
        ) : (
          <>
            <DataTable<Lead>
              loading={leads.loading && !leads.data}
              rows={leads.data?.items ?? []}
              empty={
                <EmptyState
                  icon={<Inbox className="size-5" />}
                  title="No leads captured yet"
                  description="Leads appear here as soon as someone submits one of your pages."
                />
              }
              columns={[
                { key: "email", header: "Email", cell: (row) => row.email || "—", className: "text-zinc-100" },
                { key: "page", header: "Page", cell: (row) => row.page_name || "—" },
                {
                  key: "opted", header: "Opted in",
                  cell: (row) => <Badge tone={row.opted_in ? "success" : "neutral"}>{row.opted_in ? "Yes" : "No"}</Badge>,
                },
                { key: "when", header: "Submitted", cell: (row) => formatDate(row.created_at, true) },
              ]}
            />
            {leads.data && (
              <Pagination page={leads.data.page} pages={leads.data.pages} total={leads.data.total} onPage={setLeadPage} />
            )}
          </>
        )
      )}

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="New lead page"
        description="Visitors see this page when they scan a code that points at its link."
        footer={
          <>
            <Btn variant="ghost" onClick={() => setCreateOpen(false)}>Cancel</Btn>
            <Btn variant="primary" onClick={() => create.run()} loading={create.busy} disabled={!form.name.trim()}>
              Create page
            </Btn>
          </>
        }
      >
        <div className="space-y-4">
          {create.error && <ErrorState message={create.error} />}
          <Field label="Page name" required>
            <TextInput value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Spring offer" />
          </Field>
          <Field label="Headline">
            <TextInput value={form.headline} onChange={(e) => setForm({ ...form, headline: e.target.value })} placeholder="Get exclusive updates" />
          </Field>
          <Field label="Subheadline">
            <TextInput value={form.subheadline} onChange={(e) => setForm({ ...form, subheadline: e.target.value })} placeholder="Sign up to receive the latest news" />
          </Field>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {(["button_color", "background_color", "text_color"] as const).map((key) => (
              <Field key={key} label={key === "button_color" ? "Button" : key === "background_color" ? "Background" : "Text"}>
                <input
                  type="color"
                  value={form[key]}
                  onChange={(e) => setForm({ ...form, [key]: e.target.value })}
                  aria-label={`${key.replace("_", " ")}`}
                  className="h-10 w-full cursor-pointer rounded-lg border border-zinc-700 bg-transparent"
                />
              </Field>
            ))}
          </div>
          <Toggle
            checked={form.requires_opt_in}
            onChange={(next) => setForm({ ...form, requires_opt_in: next })}
            label="Ask for marketing consent"
            description="Visitors must tick a consent box before submitting."
          />
        </div>
      </Modal>

      <ConfirmModal
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.run(deleting.id)}
        busy={remove.busy}
        title={`Delete “${deleting?.name ?? ""}”?`}
        message="The page stops working immediately and its captured leads are deleted with it."
      />
    </>
  );
}
