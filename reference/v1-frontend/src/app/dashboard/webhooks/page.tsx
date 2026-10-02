"use client";

import { useState } from "react";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, ConfirmModal, CopyButton, EmptyState, ErrorState, Field, Grid,
  LoadingPanel, Modal, Panel, PageHeader, StatCard, StatusDot, TextInput, Toggle,
  formatNumber, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { Webhook, WebhookLog } from "@/types/enterprise";

export default function WebhooksPage() {
  return (
    <WorkspaceGate requiredRole="admin" requiredFeature="webhooks">
      <Webhooks />
    </WorkspaceGate>
  );
}

function Webhooks() {
  const { workspaceId } = useWorkspace();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Webhook | null>(null);
  const [url, setUrl] = useState("");
  const [description, setDescription] = useState("");
  const [events, setEvents] = useState<string[]>(["scan.created"]);
  const [newSecret, setNewSecret] = useState<{ url: string; secret: string } | null>(null);
  const [deleting, setDeleting] = useState<Webhook | null>(null);
  const [logsFor, setLogsFor] = useState<Webhook | null>(null);
  const [logs, setLogs] = useState<WebhookLog[]>([]);
  const [testResult, setTestResult] = useState<Record<string, string>>({});

  const list = useResource((id) => enterprise.listWebhooks(id), []);

  const close = () => {
    setFormOpen(false);
    setEditing(null);
    setUrl("");
    setDescription("");
    setEvents(["scan.created"]);
  };

  const save = useMutation(
    async () => {
      if (editing) {
        return enterprise.updateWebhook(workspaceId!, editing.id, {
          url: url.trim(), events, description,
        });
      }
      return enterprise.createWebhook(workspaceId!, {
        url: url.trim(), events, description: description || undefined,
      });
    },
    {
      onSuccess: (result) => {
        if (result && "secret" in result && result.secret) {
          setNewSecret({ url: result.url, secret: result.secret });
        }
        close();
        list.reload();
      },
    },
  );

  const remove = useMutation(
    (id: string) => enterprise.deleteWebhook(workspaceId!, id),
    { onSuccess: () => { setDeleting(null); list.reload(); } },
  );

  const toggleActive = useMutation(
    (webhook: Webhook) =>
      enterprise.updateWebhook(workspaceId!, webhook.id, { is_active: !webhook.is_active }),
    { onSuccess: () => list.reload() },
  );

  const test = useMutation(
    async (webhook: Webhook) => {
      const result = await enterprise.testWebhook(workspaceId!, webhook.id);
      setTestResult((current) => ({
        ...current,
        [webhook.id]: result.delivered
          ? `Delivered · HTTP ${result.status_code} in ${result.duration_ms}ms`
          : `Failed · ${result.error || `HTTP ${result.status_code}`}`,
      }));
      return result;
    },
    { onSuccess: () => list.reload() },
  );

  const openLogs = useMutation(
    async (webhook: Webhook) => {
      const result = await enterprise.webhookLogs(workspaceId!, webhook.id);
      setLogs(result);
      setLogsFor(webhook);
      return result;
    },
  );

  const openEdit = (webhook: Webhook) => {
    setEditing(webhook);
    setUrl(webhook.url);
    setDescription(webhook.description ?? "");
    setEvents(webhook.events);
    setFormOpen(true);
  };

  const webhooks = list.data?.webhooks ?? [];
  const catalogue = list.data?.events ?? [];
  const healthy = webhooks.filter((row) => row.is_active && row.fail_count === 0).length;

  return (
    <>
      <PageHeader
        title="Webhooks"
        description="Push scan events into your own systems in real time. Every delivery is signed so you can verify it came from us."
        actions={<Btn variant="primary" onClick={() => setFormOpen(true)}>+ Add endpoint</Btn>}
      />

      <Grid cols={3} className="mb-5">
        <StatCard label="Endpoints" value={formatNumber(webhooks.length)} />
        <StatCard label="Healthy" value={formatNumber(healthy)} />
        <StatCard
          label="Failing"
          value={formatNumber(webhooks.filter((row) => row.fail_count > 0).length)}
          hint={webhooks.some((row) => row.fail_count > 0) ? "Check the delivery log" : undefined}
        />
      </Grid>

      {newSecret && (
        <div className="mb-5 rounded-xl border border-amber-900/40 bg-amber-950/20 p-4">
          <p className="text-sm font-medium text-amber-200">Save your signing secret now</p>
          <p className="mt-0.5 text-xs text-amber-300/70">
            This is the only time it is shown. Use it to verify the{" "}
            <code className="text-amber-200">X-QRit-Signature</code> header on every delivery.
          </p>
          <div className="mt-2.5 flex items-center gap-2 rounded-lg border border-amber-900/40 bg-zinc-950 px-3 py-2">
            <code className="min-w-0 flex-1 truncate text-xs text-zinc-300">{newSecret.secret}</code>
            <CopyButton value={newSecret.secret} />
          </div>
          <button
            onClick={() => setNewSecret(null)}
            className="mt-2 text-xs text-amber-300/70 underline underline-offset-2 hover:text-amber-200"
          >
            I have saved it
          </button>
        </div>
      )}

      {list.error && <ErrorState message={list.error.message} onRetry={list.reload} />}
      {test.error && <div className="mb-4"><ErrorState message={test.error} /></div>}

      {list.loading && !list.data ? (
        <LoadingPanel rows={4} />
      ) : webhooks.length ? (
        <div className="space-y-3">
          {webhooks.map((webhook) => (
            <div key={webhook.id} className="rounded-xl border border-zinc-800/70 bg-zinc-900/40 p-4">
              <div className="flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-sm text-zinc-100">{webhook.url}</p>
                  {webhook.description && (
                    <p className="mt-0.5 truncate text-xs text-zinc-500">{webhook.description}</p>
                  )}
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {webhook.events.map((event) => (
                      <Badge key={event} tone="violet">{event}</Badge>
                    ))}
                  </div>
                </div>
                <div className="flex flex-col items-end gap-2">
                  <StatusDot active={webhook.is_active} />
                  {webhook.fail_count > 0 && (
                    <Badge tone="danger">{webhook.fail_count} consecutive failures</Badge>
                  )}
                </div>
              </div>

              {testResult[webhook.id] && (
                <p
                  className={
                    testResult[webhook.id].startsWith("Delivered")
                      ? "mt-3 text-xs text-emerald-400"
                      : "mt-3 text-xs text-red-400"
                  }
                >
                  {testResult[webhook.id]}
                </p>
              )}

              <div className="mt-3 flex flex-wrap items-center gap-1.5 border-t border-zinc-800/60 pt-3 text-xs text-zinc-600">
                <span>
                  Last delivery {webhook.last_triggered ? formatRelative(webhook.last_triggered) : "never"}
                  {typeof webhook.delivery_count === "number" && ` · ${formatNumber(webhook.delivery_count)} total`}
                </span>
                <div className="ml-auto flex flex-wrap gap-1.5">
                  <Btn size="sm" variant="outline" loading={test.busy} onClick={() => test.run(webhook)}>
                    Send test
                  </Btn>
                  <Btn size="sm" variant="ghost" onClick={() => openLogs.run(webhook)}>Deliveries</Btn>
                  <Btn size="sm" variant="ghost" onClick={() => toggleActive.run(webhook)}>
                    {webhook.is_active ? "Pause" : "Resume"}
                  </Btn>
                  <Btn size="sm" variant="ghost" onClick={() => openEdit(webhook)}>Edit</Btn>
                  <Btn
                    size="sm"
                    variant="ghost"
                    className="text-red-400 hover:bg-red-500/10"
                    onClick={() => setDeleting(webhook)}
                  >
                    Delete
                  </Btn>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyState
          icon={
            <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
            </svg>
          }
          title="No webhook endpoints yet"
          description="Add an endpoint to receive a signed POST every time one of your codes is scanned — useful for piping scans into a CRM, a data warehouse or Slack."
          action={<Btn variant="primary" onClick={() => setFormOpen(true)}>Add your first endpoint</Btn>}
        />
      )}

      <Panel title="Verifying deliveries" className="mt-5">
        <p className="text-sm text-zinc-400">
          Each request carries <code className="text-zinc-200">X-QRit-Signature: sha256=…</code>,
          an HMAC of the raw request body keyed with your signing secret. Recompute it and compare
          in constant time before trusting the payload.
        </p>
        <pre className="mt-3 overflow-x-auto rounded-lg border border-zinc-800 bg-zinc-950 p-3 text-xs text-zinc-400">
{`import hmac, hashlib

def verify(raw_body: bytes, header: str, secret: str) -> bool:
    expected = hmac.new(secret.encode(), raw_body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", header)`}
        </pre>
      </Panel>

      <Modal
        open={formOpen}
        onClose={close}
        title={editing ? "Edit endpoint" : "Add webhook endpoint"}
        description="We will POST a JSON payload to this URL when the selected events happen."
        width="lg"
        footer={
          <>
            <Btn variant="ghost" onClick={close}>Cancel</Btn>
            <Btn
              variant="primary"
              loading={save.busy}
              disabled={!url.startsWith("http") || events.length === 0}
              onClick={() => save.run()}
            >
              {editing ? "Save changes" : "Create endpoint"}
            </Btn>
          </>
        }
      >
        {save.error && <div className="mb-4"><ErrorState message={save.error} /></div>}
        <div className="space-y-4">
          <Field label="Endpoint URL" required hint="Must be reachable over the public internet.">
            <TextInput
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://hooks.yourcompany.com/qrit"
              autoFocus
            />
          </Field>
          <Field label="Description">
            <TextInput
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Pipes scans into the CRM"
            />
          </Field>
          <Field label="Events" required>
            <div className="space-y-2 rounded-lg border border-zinc-800 bg-zinc-950/60 p-3">
              {catalogue.map((event) => (
                <Toggle
                  key={event.id}
                  checked={events.includes(event.id)}
                  onChange={(next) =>
                    setEvents((current) =>
                      next ? [...current, event.id] : current.filter((id) => id !== event.id),
                    )
                  }
                  label={event.label}
                  description={event.description}
                />
              ))}
            </div>
          </Field>
        </div>
      </Modal>

      <Modal
        open={Boolean(logsFor)}
        onClose={() => setLogsFor(null)}
        title="Recent deliveries"
        description={logsFor?.url}
        width="lg"
      >
        {logs.length ? (
          <ul className="divide-y divide-zinc-800/60">
            {logs.map((log) => (
              <li key={log.id} className="py-2.5">
                <div className="flex items-center gap-2">
                  <Badge tone={log.success ? "success" : "danger"}>
                    {log.status_code || "ERR"}
                  </Badge>
                  <span className="text-sm text-zinc-300">{log.event}</span>
                  <span className="ml-auto text-xs text-zinc-600">
                    {log.duration_ms}ms · {formatRelative(log.created_at)}
                  </span>
                </div>
                {log.error && <p className="mt-1 text-xs text-red-400">{log.error}</p>}
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState title="No deliveries yet" description="Send a test to check the endpoint responds." />
        )}
      </Modal>

      <ConfirmModal
        open={Boolean(deleting)}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.run(deleting.id)}
        busy={remove.busy}
        title="Delete this endpoint?"
        message="Deliveries stop immediately and the signing secret is destroyed. You will need a new secret if you recreate it."
      />
    </>
  );
}
