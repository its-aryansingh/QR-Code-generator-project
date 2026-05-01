"use client";

import { useState, useEffect } from "react";
import { useAuthStore } from "@/lib/auth";
import { Check, Copy, ExternalLink, Zap, Plus } from "lucide-react";

interface Webhook {
  id: string;
  url: string;
  events: string[];
  is_active: boolean;
}

interface Integration {
  id: string;
  name: string;
  description: string;
  logo: React.ReactNode;
  accentColor: string;
  badgeColor: string;
  events: string[];
  setupSteps: string[];
  docsUrl: string;
  webhookUrlHint: string;
  eventExample: string;
}

const INTEGRATIONS: Integration[] = [
  {
    id: "zapier",
    name: "Zapier",
    description: "Automate QR scan events into 6,000+ apps. Trigger Zaps when a QR code is scanned, created, or deleted — no code required.",
    logo: (
      <svg viewBox="0 0 48 48" className="w-8 h-8" fill="none">
        <circle cx="24" cy="24" r="24" fill="#FF4A00" />
        <path d="M24 10l2.8 8.6h9l-7.3 5.3 2.8 8.6L24 27.2l-7.3 5.3 2.8-8.6L12.2 18.6h9L24 10z" fill="white" />
      </svg>
    ),
    accentColor: "border-orange-500/30 bg-orange-500/5",
    badgeColor: "text-orange-400 bg-orange-500/10",
    events: ["qr.scanned", "qr.created", "qr.deleted"],
    setupSteps: [
      "Go to zapier.com and create a new Zap",
      "Choose 'Webhooks by Zapier' as your trigger",
      "Select 'Catch Hook' as the trigger event",
      "Copy the webhook URL Zapier gives you",
      "Paste it below and select which events to send",
      "Test the connection, then publish your Zap",
    ],
    docsUrl: "https://zapier.com/apps/webhook/integrations",
    webhookUrlHint: "https://hooks.zapier.com/hooks/catch/...",
    eventExample: '{"event":"qr.scanned","data":{"qr_id":"...","scan_count":42,"country":"US"}}',
  },
  {
    id: "slack",
    name: "Slack",
    description: "Get real-time Slack notifications for QR activity. Post to any channel when QR codes are scanned or workspace events occur.",
    logo: (
      <svg viewBox="0 0 48 48" className="w-8 h-8" fill="none">
        <circle cx="24" cy="24" r="24" fill="#4A154B" />
        <path d="M18 14a2 2 0 012-2 2 2 0 012 2v8a2 2 0 01-2 2 2 2 0 01-2-2V14zM14 20a2 2 0 010-4h4v4h-4zM30 14a2 2 0 014 0v8a2 2 0 01-4 0V14zM30 24h4a2 2 0 010 4h-4v-4zM18 34a2 2 0 01-2-2 2 2 0 012-2h8a2 2 0 012 2 2 2 0 01-2 2H18zM24 34v4a2 2 0 01-4 0v-4h4zM30 20a2 2 0 012-2h4v4h-4a2 2 0 01-2-2zM14 28a2 2 0 010-4 2 2 0 012 2v2h-2z" fill="white" />
      </svg>
    ),
    accentColor: "border-purple-500/30 bg-purple-500/5",
    badgeColor: "text-purple-400 bg-purple-500/10",
    events: ["qr.scanned", "qr.created", "qr.updated", "qr.deleted"],
    setupSteps: [
      "Go to api.slack.com/apps and create a new app",
      "Enable 'Incoming Webhooks' in the Features section",
      "Click 'Add New Webhook to Workspace'",
      "Select the channel you want to post to",
      "Copy the generated webhook URL",
      "Paste it below and select which events to forward",
    ],
    docsUrl: "https://api.slack.com/messaging/webhooks",
    webhookUrlHint: "https://hooks.slack.com/services/T.../B.../...",
    eventExample: '{"event":"qr.scanned","data":{"title":"My QR","scan_count":1}}',
  },
  {
    id: "hubspot",
    name: "HubSpot",
    description: "Sync QR scan data into HubSpot CRM. Update contact records, trigger workflows, and attribute marketing campaigns to QR scans.",
    logo: (
      <svg viewBox="0 0 48 48" className="w-8 h-8" fill="none">
        <circle cx="24" cy="24" r="24" fill="#FF7A59" />
        <circle cx="30" cy="18" r="5" fill="white" />
        <circle cx="30" cy="18" r="2.5" fill="#FF7A59" />
        <path d="M25.5 20.5l-9 5" stroke="white" strokeWidth="2.5" strokeLinecap="round" />
        <circle cx="14" cy="27" r="4" fill="white" />
        <circle cx="14" cy="27" r="2" fill="#FF7A59" />
        <path d="M17.5 29.5l5 6" stroke="white" strokeWidth="2.5" strokeLinecap="round" />
        <circle cx="24" cy="37" r="4" fill="white" />
        <circle cx="24" cy="37" r="2" fill="#FF7A59" />
      </svg>
    ),
    accentColor: "border-rose-500/30 bg-rose-500/5",
    badgeColor: "text-rose-400 bg-rose-500/10",
    events: ["qr.scanned", "lead.captured"],
    setupSteps: [
      "In HubSpot, go to Settings → Integrations → Private Apps",
      "Create a new private app with 'crm.objects.contacts.write' scope",
      "OR use HubSpot's Webhooks app in the Marketplace",
      "Set the target URL to your HubSpot workflow webhook",
      "Paste the URL below and select 'qr.scanned' events",
      "Map the payload fields to HubSpot contact properties",
    ],
    docsUrl: "https://developers.hubspot.com/docs/api/webhooks",
    webhookUrlHint: "https://api.hsforms.com/submissions/v3/integration/...",
    eventExample: '{"event":"qr.scanned","data":{"qr_id":"...","country":"US","device":"mobile","referrer":"direct"}}',
  },
  {
    id: "make",
    name: "Make (Integromat)",
    description: "Build powerful automation scenarios with Make. Connect QR scan events to thousands of apps with advanced data transformation.",
    logo: (
      <svg viewBox="0 0 48 48" className="w-8 h-8" fill="none">
        <circle cx="24" cy="24" r="24" fill="#6D00CC" />
        <path d="M12 24c0-6.627 5.373-12 12-12s12 5.373 12 12-5.373 12-12 12" stroke="white" strokeWidth="3" strokeLinecap="round" />
        <circle cx="24" cy="24" r="4" fill="white" />
      </svg>
    ),
    accentColor: "border-violet-500/30 bg-violet-500/5",
    badgeColor: "text-violet-400 bg-violet-500/10",
    events: ["qr.scanned", "qr.created", "qr.deleted", "lead.captured"],
    setupSteps: [
      "In Make, create a new scenario",
      "Add a 'Webhooks' module as the trigger",
      "Choose 'Custom webhook' and create a new one",
      "Copy the generated webhook URL",
      "Paste it below and select events to send",
      "Add downstream modules to process QR data",
    ],
    docsUrl: "https://www.make.com/en/help/tools/webhooks",
    webhookUrlHint: "https://hook.eu1.make.com/...",
    eventExample: '{"event":"qr.scanned","data":{"qr_id":"...","scan_count":1}}',
  },
];

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); }}
      className="p-1.5 rounded text-zinc-500 hover:text-zinc-300 hover:bg-zinc-700 transition-colors"
    >
      {copied ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
    </button>
  );
}

function IntegrationCard({ integration, workspaceId, existingWebhooks }: {
  integration: Integration;
  workspaceId: string;
  existingWebhooks: Webhook[];
}) {
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
  const [expanded, setExpanded] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  const connected = existingWebhooks.some((w) =>
    integration.webhookUrlHint.includes(w.url.split("/")[2] ?? "NOMATCH") ||
    integration.events.some((e) => w.events?.includes(e))
  );

  const handleCreate = async () => {
    if (!webhookUrl.trim() || !workspaceId) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/webhooks`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({
          url: webhookUrl.trim(),
          events: integration.events,
          secret: crypto.randomUUID().replace(/-/g, ""),
        }),
      });
      const data = await res.json();
      if (data.success) {
        setSaved(true);
        setWebhookUrl("");
        setTimeout(() => setSaved(false), 3000);
      } else {
        setError(data.error ?? "Failed to create webhook");
      }
    } catch {
      setError("Network error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className={`border rounded-xl overflow-hidden transition-all ${integration.accentColor}`}>
      {/* Card header */}
      <div className="p-5">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center flex-shrink-0">
              {integration.logo}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-zinc-100">{integration.name}</h3>
                {connected && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/15 border border-emerald-500/30 text-emerald-400">
                    Connected
                  </span>
                )}
              </div>
              <p className="text-sm text-zinc-400 mt-0.5 leading-relaxed">{integration.description}</p>
            </div>
          </div>
          <a
            href={integration.docsUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="shrink-0 p-2 text-zinc-600 hover:text-zinc-400 transition-colors"
            title="Docs"
          >
            <ExternalLink size={14} />
          </a>
        </div>

        {/* Events */}
        <div className="flex flex-wrap gap-1.5 mt-4">
          {integration.events.map((e) => (
            <span key={e} className={`text-[11px] px-2 py-0.5 rounded-full font-mono ${integration.badgeColor}`}>
              {e}
            </span>
          ))}
        </div>

        <button
          onClick={() => setExpanded((v) => !v)}
          className="mt-4 w-full flex items-center justify-center gap-1.5 py-2 text-xs text-zinc-400 hover:text-zinc-200 border border-zinc-700/50 rounded-lg hover:bg-zinc-800/40 transition-colors"
        >
          <Zap size={12} />
          {expanded ? "Hide setup" : "Setup guide"}
        </button>
      </div>

      {/* Expanded setup */}
      {expanded && (
        <div className="border-t border-zinc-800/60 bg-zinc-950/60 p-5 space-y-5">
          {/* Steps */}
          <div>
            <h4 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-3">Setup Steps</h4>
            <ol className="space-y-2">
              {integration.setupSteps.map((step, i) => (
                <li key={i} className="flex items-start gap-3 text-sm text-zinc-400">
                  <span className="flex-shrink-0 w-5 h-5 rounded-full bg-zinc-800 border border-zinc-700 text-xs font-bold text-zinc-500 flex items-center justify-center mt-0.5">
                    {i + 1}
                  </span>
                  {step}
                </li>
              ))}
            </ol>
          </div>

          {/* Payload example */}
          <div>
            <h4 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-2">Example Payload</h4>
            <div className="relative">
              <pre className="bg-zinc-900 border border-zinc-800 rounded-lg p-3 text-xs font-mono text-zinc-400 overflow-x-auto whitespace-pre-wrap">
                {integration.eventExample}
              </pre>
              <div className="absolute top-2 right-2">
                <CopyButton text={integration.eventExample} />
              </div>
            </div>
          </div>

          {/* Connect form */}
          <div>
            <h4 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-2">Connect Webhook</h4>
            <div className="flex gap-2">
              <input
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
                placeholder={integration.webhookUrlHint}
                className="flex-1 bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500 font-mono"
              />
              <button
                onClick={handleCreate}
                disabled={saving || !webhookUrl.trim() || !workspaceId}
                className="flex items-center gap-1.5 px-4 py-2 bg-violet-600 hover:bg-violet-500 disabled:opacity-40 text-white text-sm font-medium rounded-lg transition-colors"
              >
                {saving ? (
                  <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                ) : saved ? (
                  <Check size={14} />
                ) : (
                  <Plus size={14} />
                )}
                {saved ? "Connected!" : saving ? "Saving…" : "Connect"}
              </button>
            </div>
            {error && <p className="text-xs text-red-400 mt-1.5">{error}</p>}
            {saved && (
              <p className="text-xs text-emerald-400 mt-1.5">
                Webhook created — events will now be forwarded to {integration.name}.
              </p>
            )}
            <p className="text-xs text-zinc-600 mt-1.5">
              A unique HMAC-SHA256 signing secret will be generated automatically. View it in the{" "}
              <a href="/dashboard/webhooks" className="text-violet-400 hover:text-violet-300">Webhooks page</a>.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

export default function IntegrationsPage() {
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
  const [workspaceId, setWorkspaceId] = useState("");
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);

  useEffect(() => {
    const stored = localStorage.getItem("qrit_active_workspace");
    const wsId = stored || "";
    if (!wsId) {
      fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${accessToken}` } })
        .then((r) => r.json())
        .then((d) => {
          if (d.success && d.data?.length) {
            setWorkspaceId(d.data[0].id);
            fetchWebhooks(d.data[0].id);
          }
        })
        .catch(() => {});
    } else {
      setWorkspaceId(wsId);
      fetchWebhooks(wsId);
    }
  }, [accessToken]);

  const fetchWebhooks = (wsId: string) => {
    fetch(`${api}/workspaces/${wsId}/webhooks`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then((r) => r.json())
      .then((d) => { if (d.success) setWebhooks(d.data || []); })
      .catch(() => {});
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-bold text-zinc-100">Integrations</h1>
          <p className="text-sm text-zinc-500 mt-1">
            Connect QRit to your favorite tools via webhooks — no code required.
          </p>
        </div>
        <a
          href="/dashboard/webhooks"
          className="flex items-center gap-1.5 px-3 py-2 text-sm bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg hover:bg-zinc-700 transition-colors"
        >
          <Zap size={14} />
          All Webhooks
        </a>
      </div>

      {/* How it works banner */}
      <div className="flex items-start gap-4 p-4 bg-violet-600/10 border border-violet-500/20 rounded-xl">
        <div className="w-8 h-8 rounded-lg bg-violet-600/20 flex items-center justify-center flex-shrink-0">
          <Zap size={16} className="text-violet-400" />
        </div>
        <div>
          <p className="text-sm font-medium text-zinc-200">How integrations work</p>
          <p className="text-sm text-zinc-400 mt-0.5">
            QRit sends a signed HTTP POST request to your webhook URL whenever a selected event occurs.
            Each request includes an <code className="text-violet-400 text-xs">X-QRit-Signature</code> header (HMAC-SHA256)
            so you can verify authenticity. Events fire in real-time with sub-second latency.
          </p>
        </div>
      </div>

      {/* Integration cards */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {INTEGRATIONS.map((integration) => (
          <IntegrationCard
            key={integration.id}
            integration={integration}
            workspaceId={workspaceId}
            existingWebhooks={webhooks}
          />
        ))}
      </div>

      {/* Custom webhook CTA */}
      <div className="p-5 bg-zinc-900/60 border border-zinc-800/60 rounded-xl flex items-center justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-zinc-200">Need a custom integration?</p>
          <p className="text-sm text-zinc-500 mt-0.5">
            Create a generic webhook on the Webhooks page and subscribe to any combination of events.
          </p>
        </div>
        <a
          href="/dashboard/webhooks"
          className="flex-shrink-0 flex items-center gap-1.5 px-4 py-2 bg-zinc-800 border border-zinc-700 text-zinc-300 text-sm rounded-lg hover:bg-zinc-700 transition-colors"
        >
          <Plus size={14} /> Create Webhook
        </a>
      </div>

      {/* Event reference */}
      <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl overflow-hidden">
        <div className="px-5 py-4 border-b border-zinc-800/60">
          <h2 className="text-sm font-semibold text-zinc-300">Available Events</h2>
        </div>
        <div className="divide-y divide-zinc-800/40">
          {[
            { event: "qr.created", desc: "Fired when a new QR code is created in any workspace." },
            { event: "qr.updated", desc: "Fired when a QR code's title, redirect URL, or settings change." },
            { event: "qr.deleted", desc: "Fired when a QR code is permanently deleted." },
            { event: "qr.scanned", desc: "Fired on every scan, includes device, country, referrer, and timestamp." },
            { event: "lead.captured", desc: "Fired when a visitor submits a lead capture form." },
            { event: "webhook.test", desc: "Fired manually via the 'Test' button — for endpoint verification." },
          ].map(({ event, desc }) => (
            <div key={event} className="flex items-start gap-4 px-5 py-3">
              <code className="text-xs font-mono text-violet-400 bg-violet-500/10 px-2 py-0.5 rounded mt-0.5 flex-shrink-0">
                {event}
              </code>
              <p className="text-sm text-zinc-400">{desc}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
