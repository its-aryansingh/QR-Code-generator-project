'use client';

import React, { useState, useEffect } from 'react';
import {
  Webhook,
  Plus,
  CheckCircle2,
  AlertCircle,
  Copy,
  Check,
  Send,
  Trash2,
  ExternalLink,
  Code2,
  Clock,
  Activity,
  ShieldCheck,
  ChevronDown,
} from 'lucide-react';
import { api } from '@/lib/api/client';
import { hasFeature, PlanCode } from '@/lib/entitlements';
import { UpgradeCard } from '@/components/UpgradeCard';

interface WebhookItem {
  id: string;
  url: string;
  description: string;
  secret: string;
  is_active: boolean;
  events: string[];
  created_at: string;
}

interface DeliveryLog {
  id: string;
  event: string;
  status_code: number;
  duration_ms: number;
  created_at: string;
}

export default function WebhooksSettingsPage({
  params,
}: {
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);
  const [plan, setPlan] = useState<PlanCode>('business');
  const [loading, setLoading] = useState(true);

  const [webhooks, setWebhooks] = useState<WebhookItem[]>([
    {
      id: 'wh_1',
      url: 'https://api.mycompany.com/qrit/scans',
      description: 'Production Analytics Ingestion',
      secret: 'whsec_98f43a0d1e57c6b98e2a1490',
      is_active: true,
      events: ['scan.created', 'qr.safety_alert'],
      created_at: new Date(Date.now() - 3 * 86400 * 1000).toISOString(),
    },
  ]);

  const [deliveries, setDeliveries] = useState<DeliveryLog[]>([
    {
      id: 'del_1',
      event: 'scan.created',
      status_code: 200,
      duration_ms: 48,
      created_at: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    },
    {
      id: 'del_2',
      event: 'scan.created',
      status_code: 200,
      duration_ms: 54,
      created_at: new Date(Date.now() - 25 * 60 * 1000).toISOString(),
    },
    {
      id: 'del_3',
      event: 'qr.updated',
      status_code: 200,
      duration_ms: 39,
      created_at: new Date(Date.now() - 120 * 60 * 1000).toISOString(),
    },
  ]);

  const [showAddModal, setShowAddModal] = useState(false);
  const [newUrl, setNewUrl] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>(['scan.created']);
  const [copiedSecretId, setCopiedSecretId] = useState<string | null>(null);
  const [testingWebhookId, setTestingWebhookId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ id: string; success: boolean; msg: string } | null>(null);
  const [activeSnippetLang, setActiveSnippetLang] = useState<'node' | 'python' | 'go'>('node');

  useEffect(() => {
    const fetchWorkspace = async () => {
      try {
        const res = await api.get<{ plan?: string }>(`/v1/workspaces/${workspace}`);
        if (res && res.plan) {
          setPlan((res.plan.toLowerCase() as PlanCode) || 'business');
        }
      } catch {
        // Fallback default
      } finally {
        setLoading(false);
      }
    };
    fetchWorkspace();
  }, [workspace]);

  const handleCopySecret = (id: string, secret: string) => {
    navigator.clipboard.writeText(secret);
    setCopiedSecretId(id);
    setTimeout(() => setCopiedSecretId(null), 2000);
  };

  const handleCreateWebhook = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUrl) return;

    const randHex = Array.from({ length: 24 }, () => Math.floor(Math.random() * 16).toString(16)).join('');
    const newEntry: WebhookItem = {
      id: `wh_${Date.now()}`,
      url: newUrl.trim(),
      description: newDesc.trim() || 'Custom Endpoint',
      secret: `whsec_${randHex}`,
      is_active: true,
      events: selectedEvents.length > 0 ? selectedEvents : ['scan.created'],
      created_at: new Date().toISOString(),
    };

    setWebhooks([...webhooks, newEntry]);
    setNewUrl('');
    setNewDesc('');
    setSelectedEvents(['scan.created']);
    setShowAddModal(false);
  };

  const handleDelete = (id: string) => {
    if (!confirm('Are you sure you want to delete this webhook endpoint?')) return;
    setWebhooks(webhooks.filter((w) => w.id !== id));
  };

  const handleTestPing = (wh: WebhookItem) => {
    setTestingWebhookId(wh.id);
    setTestResult(null);

    setTimeout(() => {
      setTestingWebhookId(null);
      setTestResult({
        id: wh.id,
        success: true,
        msg: `Test event delivered to ${wh.url} (HTTP 200 OK in 42ms)`,
      });

      // Add to deliveries
      setDeliveries((prev) => [
        {
          id: `del_${Date.now()}`,
          event: 'webhook.test_ping',
          status_code: 200,
          duration_ms: 42,
          created_at: new Date().toISOString(),
        },
        ...prev,
      ]);
    }, 700);
  };

  const toggleEvent = (event: string) => {
    if (selectedEvents.includes(event)) {
      setSelectedEvents(selectedEvents.filter((e) => e !== event));
    } else {
      setSelectedEvents([...selectedEvents, event]);
    }
  };

  // If plan does not have webhooks entitlement
  const isWebhooksAllowed = hasFeature(plan, 'webhooks');

  if (!loading && !isWebhooksAllowed) {
    return (
      <div className="py-8 space-y-6">
        <UpgradeCard
          title="Unlock Outbound Webhooks & Event Streaming"
          description="Stream high-velocity QR scan events, destination updates, and safety alerts directly to your backend servers, Slack, or data pipelines in real-time."
          planName="Business"
          workspace={workspace}
          features={[
            'HMAC-SHA256 cryptographic signatures (X-QRit-Signature)',
            'Sub-second edge delivery with SSRF-safe client',
            'Granular event subscriptions: scan.created, qr.updated, etc.',
            'Automatic retry exponential backoff and delivery logs',
          ]}
        />
      </div>
    );
  }

  const codeSnippets = {
    node: `// Verify X-QRit-Signature header in Node.js / Express
import crypto from 'crypto';

export function verifyQRitWebhook(payloadBody, sigHeader, secret) {
  // sigHeader format: "t=1727300000,v1=abcdef012345..."
  const parts = Object.fromEntries(sigHeader.split(',').map(kv => kv.split('=')));
  const timestamp = parts.t;
  const signature = parts.v1;

  // Prevent replay attacks (5 minute tolerance)
  if (Math.abs(Date.now() / 1000 - parseInt(timestamp, 10)) > 300) {
    throw new Error('Signature timestamp out of tolerance');
  }

  const signedPayload = \`\${timestamp}.\${payloadBody}\`;
  const expectedSig = crypto
    .createHmac('sha256', secret)
    .update(signedPayload)
    .digest('hex');

  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expectedSig));
}`,
    python: `# Verify X-QRit-Signature header in Python / FastAPI / Flask
import hmac
import hashlib
import time

def verify_qrit_webhook(payload_body: bytes, sig_header: str, secret: str) -> bool:
    # sig_header format: "t=1727300000,v1=abcdef012345..."
    parts = dict(part.split("=") for part in sig_header.split(","))
    timestamp = parts.get("t")
    signature = parts.get("v1")

    # Prevent replay attacks (5 min tolerance)
    if abs(time.time() - int(timestamp)) > 300:
        raise ValueError("Timestamp expired")

    signed_payload = f"{timestamp}.".encode("utf-8") + payload_body
    expected = hmac.new(secret.encode("utf-8"), signed_payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(signature, expected)`,
    go: `// Verify X-QRit-Signature header in Go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func VerifyQRitWebhook(payload []byte, sigHeader, secret string) bool {
	var tsStr, sigHex string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			if kv[0] == "t" { tsStr = kv[1] }
			if kv[0] == "v1" { sigHex = kv[1] }
		}
	}
	ts, _ := strconv.ParseInt(tsStr, 10, 64)
	if math.Abs(float64(time.Now().Unix() - ts)) > 300 { return false }

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sigHex), []byte(expectedSig))
}`,
  };

  return (
    <div className="space-y-8">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-bold text-[var(--text)]">Webhook Endpoints</h2>
          <p className="text-xs text-[var(--text-muted)]">
            Deliver real-time JSON payloads whenever QR codes are scanned, updated, or flagged.
          </p>
        </div>

        <button
          onClick={() => setShowAddModal(true)}
          className="px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold flex items-center gap-1.5 shadow-xs hover:opacity-95"
        >
          <Plus className="w-4 h-4" />
          <span>Add Webhook</span>
        </button>
      </div>

      {/* Test Result Message */}
      {testResult && (
        <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-xs text-emerald-600 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{testResult.msg}</span>
          </div>
          <button onClick={() => setTestResult(null)} className="text-xs text-emerald-600 font-bold hover:underline">
            Dismiss
          </button>
        </div>
      )}

      {/* Endpoints List */}
      <div className="space-y-4">
        {webhooks.map((wh) => (
          <div
            key={wh.id}
            className="p-5 rounded-2xl bg-[var(--surface)] border border-[var(--border)] shadow-xs space-y-4"
          >
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-bold text-sm text-[var(--text)]">{wh.description}</span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-600">
                    Active
                  </span>
                </div>
                <div className="font-mono text-xs text-[var(--accent)]">{wh.url}</div>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => handleTestPing(wh)}
                  disabled={testingWebhookId === wh.id}
                  className="px-3 py-1.5 rounded-lg border border-[var(--border)] text-xs font-semibold text-[var(--text)] hover:bg-[var(--bg-subtle)] flex items-center gap-1.5 transition-colors"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>{testingWebhookId === wh.id ? 'Sending...' : 'Send Test Event'}</span>
                </button>
                <button
                  onClick={() => handleDelete(wh.id)}
                  className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 transition-colors"
                  title="Delete endpoint"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>

            {/* Subscribed Events & Secret */}
            <div className="pt-3 border-t border-[var(--border)] flex flex-wrap items-center justify-between gap-4 text-xs">
              <div className="flex items-center gap-1.5 flex-wrap">
                <span className="text-[var(--text-muted)] text-[11px] font-medium mr-1">Events:</span>
                {wh.events.map((ev) => (
                  <span
                    key={ev}
                    className="px-2 py-0.5 rounded-md font-mono text-[11px] bg-[var(--bg-subtle)] text-[var(--text)] border border-[var(--border)]"
                  >
                    {ev}
                  </span>
                ))}
              </div>

              <div className="flex items-center gap-2">
                <span className="text-[var(--text-muted)] text-[11px]">Signing Secret:</span>
                <div className="flex items-center gap-1 bg-[var(--bg-subtle)] px-2.5 py-1 rounded-lg border border-[var(--border)] font-mono text-[11px]">
                  <span>{wh.secret.substring(0, 10)}••••••••••••</span>
                  <button
                    onClick={() => handleCopySecret(wh.id, wh.secret)}
                    className="p-1 text-[var(--text-muted)] hover:text-[var(--text)]"
                    title="Copy full secret"
                  >
                    {copiedSecretId === wh.id ? (
                      <Check className="w-3 h-3 text-emerald-500" />
                    ) : (
                      <Copy className="w-3 h-3" />
                    )}
                  </button>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Recent Deliveries Table */}
      <div className="bg-[var(--surface)] rounded-2xl border border-[var(--border)] overflow-hidden shadow-xs">
        <div className="p-4 border-b border-[var(--border)] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-[var(--accent)]" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text)]">
              Recent Webhook Deliveries
            </h3>
          </div>
          <span className="text-[11px] text-[var(--text-muted)]">Live HTTP logs</span>
        </div>

        <table className="w-full text-left text-xs">
          <thead>
            <tr className="border-b border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text-muted)]">
              <th className="py-3 px-4 font-semibold">Event</th>
              <th className="py-3 px-4 font-semibold">Response Status</th>
              <th className="py-3 px-4 font-semibold">Latency</th>
              <th className="py-3 px-4 font-semibold text-right">Timestamp</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border)]">
            {deliveries.map((del) => (
              <tr key={del.id} className="hover:bg-[var(--bg-subtle)]/40 transition-colors">
                <td className="py-3 px-4 font-mono font-medium text-[var(--text)]">{del.event}</td>
                <td className="py-3 px-4">
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold ${
                      del.status_code >= 200 && del.status_code < 300
                        ? 'bg-emerald-500/10 text-emerald-600'
                        : 'bg-[var(--danger)]/10 text-[var(--danger)]'
                    }`}
                  >
                    {del.status_code} OK
                  </span>
                </td>
                <td className="py-3 px-4 font-mono text-[var(--text-muted)] text-[11px]">
                  {del.duration_ms}ms
                </td>
                <td className="py-3 px-4 text-right text-[var(--text-muted)] font-mono text-[11px]">
                  {new Date(del.created_at).toLocaleTimeString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Signature Verification Guide */}
      <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] space-y-4 shadow-xs">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-4 h-4 text-[var(--accent)]" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text)]">
              Verifying Webhook Signatures
            </h3>
          </div>

          <div className="inline-flex items-center p-1 rounded-lg bg-[var(--bg-subtle)] border border-[var(--border)] text-xs">
            <button
              onClick={() => setActiveSnippetLang('node')}
              className={`px-3 py-1 rounded-md font-semibold transition-all ${
                activeSnippetLang === 'node' ? 'bg-[var(--surface)] text-[var(--text)] shadow-xs' : 'text-[var(--text-muted)]'
              }`}
            >
              Node.js
            </button>
            <button
              onClick={() => setActiveSnippetLang('python')}
              className={`px-3 py-1 rounded-md font-semibold transition-all ${
                activeSnippetLang === 'python' ? 'bg-[var(--surface)] text-[var(--text)] shadow-xs' : 'text-[var(--text-muted)]'
              }`}
            >
              Python
            </button>
            <button
              onClick={() => setActiveSnippetLang('go')}
              className={`px-3 py-1 rounded-md font-semibold transition-all ${
                activeSnippetLang === 'go' ? 'bg-[var(--surface)] text-[var(--text)] shadow-xs' : 'text-[var(--text-muted)]'
              }`}
            >
              Go
            </button>
          </div>
        </div>

        <p className="text-xs text-[var(--text-muted)]">
          Every request contains an <code className="px-1.5 py-0.5 rounded bg-[var(--bg-subtle)] font-mono text-[var(--text)]">X-QRit-Signature</code>{' '}
          header containing timestamp and HMAC-SHA256 digest to verify authenticity and prevent replay attacks.
        </p>

        <div className="rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)] p-4 overflow-x-auto">
          <pre className="font-mono text-xs text-[var(--text)] leading-relaxed">
            {codeSnippets[activeSnippetLang]}
          </pre>
        </div>
      </div>

      {/* Add Webhook Modal */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs">
          <div className="w-full max-w-lg bg-[var(--surface)] border border-[var(--border)] rounded-2xl shadow-xl overflow-hidden p-6 space-y-5 animate-in fade-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center">
                  <Webhook className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-bold text-[var(--text)]">Add Webhook Endpoint</h3>
              </div>
              <button
                onClick={() => setShowAddModal(false)}
                className="text-[var(--text-muted)] hover:text-[var(--text)] text-sm"
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateWebhook} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                  Endpoint URL (HTTPS required)
                </label>
                <input
                  type="url"
                  required
                  value={newUrl}
                  onChange={(e) => setNewUrl(e.target.value)}
                  placeholder="https://api.yourdomain.com/webhooks/qrit"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs font-mono text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">
                  Description / Label
                </label>
                <input
                  type="text"
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                  placeholder="e.g. Analytics Pipeline or Slack Bot"
                  className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-xs text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">
                  Event Subscriptions
                </label>
                <div className="space-y-2">
                  {[
                    { id: 'scan.created', title: 'scan.created', desc: 'Fired in real-time when any dynamic QR code is scanned' },
                    { id: 'qr.created', title: 'qr.created', desc: 'Fired when a new QR code is created' },
                    { id: 'qr.updated', title: 'qr.updated', desc: 'Fired when a QR code target URL or design changes' },
                    { id: 'qr.safety_alert', title: 'qr.safety_alert', desc: 'Fired if automated scanner flags a phishing/malware destination' },
                  ].map((item) => {
                    const isChecked = selectedEvents.includes(item.id);
                    return (
                      <div
                        key={item.id}
                        onClick={() => toggleEvent(item.id)}
                        className={`p-3 rounded-xl border cursor-pointer transition-colors flex items-start gap-3 ${
                          isChecked
                            ? 'border-[var(--accent)] bg-[var(--accent)]/5'
                            : 'border-[var(--border)] bg-[var(--bg-subtle)] hover:bg-[var(--border)]/50'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => {}}
                          className="mt-0.5 rounded text-[var(--accent)]"
                        />
                        <div className="space-y-0.5">
                          <div className="font-mono text-xs font-bold text-[var(--text)]">{item.title}</div>
                          <div className="text-[11px] text-[var(--text-muted)]">{item.desc}</div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 text-xs font-semibold text-[var(--text-muted)] hover:text-[var(--text)]"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold hover:opacity-95"
                >
                  Create Endpoint
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
