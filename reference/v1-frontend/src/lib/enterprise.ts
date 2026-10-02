/**
 * The enterprise API client.
 *
 * Every dashboard page previously inlined its own `fetch` with its own
 * hardcoded API host -- three different ports across thirty files -- and its
 * own ad-hoc error handling. This is the single place that knows how to talk
 * to the backend.
 */
import { useAuthStore } from "./auth";
import type {
  ApiEnvelope, ApiKey, AuditResponse, Branding, BulkJob, BulkResult, Campaign,
  CustomDomain, Entitlements, Folder, Invite, Lead, LeadPage, Member, Paginated,
  Profile, QRListResponse, QRTemplate, Role, RoutingRule, SecurityPolicy,
  SsoConfig, Webhook, WebhookEvent, WebhookLog, Workspace, WorkspaceAnalytics,
  WorkspaceOverview, WorkspaceQR,
} from "@/types/enterprise";

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://127.0.0.1:8084/api/v1";
export const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") || API_URL.replace(/\/api\/v1$/, "");

/** Errors carry the backend's machine-readable code so the UI can react to
 *  `upgrade_required` and `insufficient_role` rather than just printing text. */
export class ApiError extends Error {
  status: number;
  code?: string;
  payload: Record<string, unknown>;

  constructor(message: string, status: number, payload: Record<string, unknown> = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = typeof payload.code === "string" ? payload.code : undefined;
    this.payload = payload;
  }

  get isUpgradeRequired() {
    return this.status === 402 || this.code === "upgrade_required" || this.code === "limit_reached";
  }
  get isForbidden() {
    return this.status === 403;
  }
  get requiredPlan() {
    return typeof this.payload.required_plan === "string" ? this.payload.required_plan : undefined;
  }
  get requiredRole() {
    return typeof this.payload.required_role === "string" ? this.payload.required_role : undefined;
  }
}

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

interface RequestOptions {
  method?: Method;
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined | null>;
  signal?: AbortSignal;
  /** Skip the 401 refresh dance (used by the refresh call itself). */
  raw?: boolean;
}

let refreshInFlight: Promise<boolean> | null = null;

/** Refresh once even if ten requests 401 at the same moment. */
async function refreshTokens(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    const refreshToken = useAuthStore.getState().refreshToken;
    if (!refreshToken) return false;
    try {
      const response = await fetch(`${API_URL}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
      const payload = await response.json();
      if (!response.ok || !payload?.success || !payload?.data?.access_token) return false;
      useAuthStore.getState().setTokens(payload.data);
      return true;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

function buildUrl(path: string, query?: RequestOptions["query"]) {
  const url = `${API_URL}${path.startsWith("/") ? path : `/${path}`}`;
  if (!query) return url;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== "") params.set(key, String(value));
  }
  const qs = params.toString();
  return qs ? `${url}?${qs}` : url;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, query, signal, raw } = options;

  const send = async (): Promise<Response> => {
    const token = useAuthStore.getState().accessToken;
    return fetch(buildUrl(path, query), {
      method,
      signal,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
  };

  let response = await send();

  if (response.status === 401 && !raw) {
    const refreshed = await refreshTokens();
    if (refreshed) {
      response = await send();
    } else {
      useAuthStore.getState().logout();
      throw new ApiError("Your session has expired. Please sign in again.", 401, {
        code: "session_expired",
      });
    }
  }

  let payload: ApiEnvelope<T> | null = null;
  const text = await response.text();
  if (text) {
    try {
      payload = JSON.parse(text) as ApiEnvelope<T>;
    } catch {
      payload = null;
    }
  }

  if (!response.ok || payload?.success === false) {
    throw new ApiError(
      payload?.error || `Request failed (${response.status})`,
      response.status,
      (payload as Record<string, unknown>) || {},
    );
  }
  return (payload?.data ?? (payload as unknown)) as T;
}

/** CSV exports come back as a file, not an envelope. */
async function download(path: string, query: RequestOptions["query"], filename: string) {
  const token = useAuthStore.getState().accessToken;
  const response = await fetch(buildUrl(path, query), {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!response.ok) {
    let message = `Export failed (${response.status})`;
    try {
      const payload = await response.json();
      message = payload?.error || message;
      throw new ApiError(message, response.status, payload);
    } catch (error) {
      if (error instanceof ApiError) throw error;
      throw new ApiError(message, response.status);
    }
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

const ws = (id: string) => `/workspaces/${id}`;

export const enterprise = {
  // ---------------------------------------------------------------- account
  me: () => request<Profile>("/auth/me"),
  updateProfile: (body: { name?: string; company?: string; avatar_url?: string }) =>
    request<Profile>("/auth/me", { method: "PUT", body }),
  changePassword: (current_password: string, new_password: string) =>
    request<{ message: string }>("/auth/change-password", {
      method: "POST",
      body: { current_password, new_password },
    }),

  // ---------------------------------------------------------------- workspaces
  listWorkspaces: () => request<Workspace[]>("/workspaces"),
  createWorkspace: (body: { name: string; description?: string; brand_color?: string }) =>
    request<Workspace>("/workspaces", { method: "POST", body }),
  getWorkspace: (id: string) => request<Workspace>(ws(id)),
  updateWorkspace: (id: string, body: Partial<Workspace>) =>
    request<Workspace>(ws(id), { method: "PUT", body }),
  deleteWorkspace: (id: string) =>
    request<{ message: string }>(ws(id), { method: "DELETE" }),
  entitlements: (id: string) => request<Entitlements>(`${ws(id)}/entitlements`),

  // ---------------------------------------------------------------- team
  listMembers: (id: string) => request<Member[]>(`${ws(id)}/members`),
  updateMemberRole: (id: string, memberId: string, role: Role) =>
    request<Member>(`${ws(id)}/members/${memberId}`, { method: "PUT", body: { role } }),
  removeMember: (id: string, memberId: string) =>
    request<{ message: string }>(`${ws(id)}/members/${memberId}`, { method: "DELETE" }),
  listInvites: (id: string) => request<Invite[]>(`${ws(id)}/invites`),
  createInvite: (id: string, email: string, role: Role) =>
    request<Invite>(`${ws(id)}/invites`, { method: "POST", body: { email, role } }),
  revokeInvite: (id: string, inviteId: string) =>
    request<{ message: string }>(`${ws(id)}/invites/${inviteId}`, { method: "DELETE" }),
  previewInvite: (token: string) =>
    request<{
      email: string; role: Role; status: string; is_valid: boolean; expires_at: string;
      workspace: { id: string; name: string; brand_color: string; brand_logo?: string | null } | null;
      invited_by: { name?: string | null; email: string } | null;
    }>(`/public/invite/${token}`),
  acceptInvite: (token: string) =>
    request<{ message: string; workspace_id: string; role: Role; workspace: Workspace }>(
      `/workspaces/invite/${token}/accept`, { method: "POST", body: {} },
    ),

  // ---------------------------------------------------------------- folders
  listFolders: (id: string) => request<Folder[]>(`${ws(id)}/folders`),
  createFolder: (id: string, body: { name: string; color?: string; parent_id?: string | null; description?: string }) =>
    request<Folder>(`${ws(id)}/folders`, { method: "POST", body }),
  updateFolder: (id: string, folderId: string, body: Partial<Folder>) =>
    request<Folder>(`${ws(id)}/folders/${folderId}`, { method: "PUT", body }),
  deleteFolder: (id: string, folderId: string) =>
    request<{ message: string }>(`${ws(id)}/folders/${folderId}`, { method: "DELETE" }),

  // ---------------------------------------------------------------- QR codes
  listQR: (id: string, query?: Record<string, string | number | undefined>) =>
    request<QRListResponse>(`${ws(id)}/qr`, { query }),
  createQR: (id: string, body: Record<string, unknown>) =>
    request<WorkspaceQR>(`${ws(id)}/qr`, { method: "POST", body }),
  getQR: (qrId: string) => request<WorkspaceQR>(`/qr/${qrId}`),
  updateQR: (qrId: string, body: Record<string, unknown>) =>
    request<WorkspaceQR>(`/qr/${qrId}`, { method: "PUT", body }),
  deleteQR: (qrId: string) => request<{ message: string }>(`/qr/${qrId}`, { method: "DELETE" }),
  toggleQR: (qrId: string) => request<WorkspaceQR>(`/qr/${qrId}/toggle`, { method: "PUT", body: {} }),
  downloadQR: (qrId: string, query?: { size?: number; format?: string }) =>
    request<{ qr_base64: string; filename: string }>(`/qr/${qrId}/download`, { query }),
  bulkAction: (
    id: string,
    body: {
      action: "move" | "tag" | "untag" | "activate" | "deactivate"
      | "assign_campaign" | "clear_campaign" | "delete";
      qr_ids: string[];
      folder_id?: string | null;
      campaign_id?: string;
      tags?: string[];
    },
  ) => request<{ action: string; affected: number }>(`${ws(id)}/qr/bulk-action`, { method: "POST", body }),

  // ---------------------------------------------------------------- routing
  listRoutingRules: (id: string, qrId: string) =>
    request<{
      rules: RoutingRule[]; default_destination: string;
      conditions: string[]; available: boolean;
    }>(`${ws(id)}/qr/${qrId}/routing`),
  createRoutingRule: (id: string, qrId: string, body: Partial<RoutingRule> & { destination_url: string }) =>
    request<RoutingRule>(`${ws(id)}/qr/${qrId}/routing`, { method: "POST", body }),
  updateRoutingRule: (id: string, qrId: string, ruleId: string, body: Partial<RoutingRule>) =>
    request<RoutingRule>(`${ws(id)}/qr/${qrId}/routing/${ruleId}`, { method: "PUT", body }),
  deleteRoutingRule: (id: string, qrId: string, ruleId: string) =>
    request<{ message: string }>(`${ws(id)}/qr/${qrId}/routing/${ruleId}`, { method: "DELETE" }),

  // ---------------------------------------------------------------- bulk
  listBulkJobs: (id: string) =>
    request<{ jobs: BulkJob[]; columns: string[]; max_rows: number; template_csv: string }>(
      `${ws(id)}/bulk`,
    ),
  runBulk: (id: string, body: Record<string, unknown>) =>
    request<{ job: BulkJob; results: BulkResult[]; errors: BulkJob["errors"] }>(`${ws(id)}/bulk`, {
      method: "POST", body,
    }),
  getBulkJob: (id: string, jobId: string) => request<BulkJob>(`${ws(id)}/bulk/${jobId}`),

  // ---------------------------------------------------------------- campaigns
  listCampaigns: (id: string, query?: Record<string, string | undefined>) =>
    request<Campaign[]>(`${ws(id)}/campaigns`, { query }),
  createCampaign: (id: string, body: Partial<Campaign> & { name: string }) =>
    request<Campaign>(`${ws(id)}/campaigns`, { method: "POST", body }),
  getCampaign: (id: string, campaignId: string) =>
    request<Campaign>(`${ws(id)}/campaigns/${campaignId}`),
  updateCampaign: (id: string, campaignId: string, body: Partial<Campaign>) =>
    request<Campaign>(`${ws(id)}/campaigns/${campaignId}`, { method: "PUT", body }),
  deleteCampaign: (id: string, campaignId: string) =>
    request<{ message: string }>(`${ws(id)}/campaigns/${campaignId}`, { method: "DELETE" }),
  campaignAnalytics: (id: string, campaignId: string, days = 30) =>
    request<Record<string, unknown>>(`${ws(id)}/campaigns/${campaignId}/analytics`, {
      query: { days },
    }),

  // ---------------------------------------------------------------- templates
  listTemplates: (id: string) => request<QRTemplate[]>(`${ws(id)}/templates`),
  createTemplate: (id: string, body: Partial<QRTemplate> & { name: string }) =>
    request<QRTemplate>(`${ws(id)}/templates`, { method: "POST", body }),
  updateTemplate: (id: string, templateId: string, body: Partial<QRTemplate>) =>
    request<QRTemplate>(`${ws(id)}/templates/${templateId}`, { method: "PUT", body }),
  deleteTemplate: (id: string, templateId: string) =>
    request<{ message: string }>(`${ws(id)}/templates/${templateId}`, { method: "DELETE" }),

  // ---------------------------------------------------------------- analytics
  analytics: (id: string, days = 30) =>
    request<WorkspaceAnalytics>(`${ws(id)}/analytics`, { query: { days } }),
  overview: (id: string, days = 30) =>
    request<WorkspaceOverview>(`${ws(id)}/overview`, { query: { days } }),
  qrAnalytics: (qrId: string, days = 30) =>
    request<Record<string, unknown>>(`/analytics/qr/${qrId}`, { query: { days } }),
  exportCsv: (
    id: string,
    kind: "analytics" | "qr" | "leads" | "campaigns" | "audit",
    query?: Record<string, string | number | undefined>,
  ) => download(`${ws(id)}/export/${kind}`, query, `${kind}-${new Date().toISOString().slice(0, 10)}.csv`),

  // ---------------------------------------------------------------- audit
  auditLog: (id: string, query?: Record<string, string | number | undefined>) =>
    request<AuditResponse>(`${ws(id)}/audit-logs`, { query }),

  // ---------------------------------------------------------------- webhooks
  listWebhooks: (id: string) =>
    request<{ webhooks: Webhook[]; events: WebhookEvent[]; available: boolean }>(`${ws(id)}/webhooks`),
  createWebhook: (id: string, body: { url: string; events: string[]; description?: string }) =>
    request<Webhook>(`${ws(id)}/webhooks`, { method: "POST", body }),
  updateWebhook: (id: string, webhookId: string, body: Partial<Webhook>) =>
    request<Webhook>(`${ws(id)}/webhooks/${webhookId}`, { method: "PUT", body }),
  deleteWebhook: (id: string, webhookId: string) =>
    request<{ message: string }>(`${ws(id)}/webhooks/${webhookId}`, { method: "DELETE" }),
  testWebhook: (id: string, webhookId: string) =>
    request<{
      delivered: boolean; status_code: number; duration_ms: number;
      error: string; response_preview: string;
    }>(`${ws(id)}/webhooks/${webhookId}/test`, { method: "POST", body: {} }),
  webhookLogs: (id: string, webhookId: string) =>
    request<WebhookLog[]>(`${ws(id)}/webhooks/${webhookId}/logs`),

  // ---------------------------------------------------------------- governance
  getBranding: (id: string) => request<Branding>(`${ws(id)}/branding`),
  updateBranding: (id: string, body: Partial<Branding>) =>
    request<Branding>(`${ws(id)}/branding`, { method: "PUT", body }),
  getSso: (id: string) => request<SsoConfig>(`${ws(id)}/sso`),
  updateSso: (id: string, body: Record<string, unknown>) =>
    request<SsoConfig>(`${ws(id)}/sso`, { method: "PUT", body }),
  createScimToken: (id: string) =>
    request<{ token: string; shown_once: boolean }>(`${ws(id)}/sso/scim-token`, {
      method: "POST", body: {},
    }),
  getSecurityPolicy: (id: string) => request<SecurityPolicy>(`${ws(id)}/security-policy`),
  updateSecurityPolicy: (id: string, body: Partial<SecurityPolicy>) =>
    request<SecurityPolicy>(`${ws(id)}/security-policy`, { method: "PUT", body }),

  listDomains: (id: string) => request<CustomDomain[]>(`${ws(id)}/domains`),
  addDomain: (id: string, domain: string) =>
    request<CustomDomain>(`${ws(id)}/domains`, { method: "POST", body: { domain } }),
  verifyDomain: (id: string, domainId: string) =>
    request<CustomDomain & { verified: boolean; detail: string }>(`${ws(id)}/domains/${domainId}`, {
      method: "POST", body: {},
    }),
  removeDomain: (id: string, domainId: string) =>
    request<{ message: string }>(`${ws(id)}/domains/${domainId}`, { method: "DELETE" }),

  listApiKeys: (id: string) =>
    request<{ keys: ApiKey[]; valid_scopes: string[]; base_url: string }>(`${ws(id)}/api-keys`),
  createApiKey: (id: string, body: { name: string; scopes: string[]; expires_at?: string }) =>
    request<ApiKey>(`${ws(id)}/api-keys`, { method: "POST", body }),
  revokeApiKey: (id: string, keyId: string) =>
    request<{ message: string }>(`${ws(id)}/api-keys/${keyId}`, { method: "DELETE" }),

  // ---------------------------------------------------------------- leads
  listLeadPages: (id: string) => request<LeadPage[]>(`${ws(id)}/pages`),
  createLeadPage: (id: string, body: Partial<LeadPage> & { name: string }) =>
    request<LeadPage>(`${ws(id)}/pages`, { method: "POST", body }),
  updateLeadPage: (id: string, pageId: string, body: Partial<LeadPage>) =>
    request<LeadPage>(`${ws(id)}/pages/${pageId}`, { method: "PUT", body }),
  deleteLeadPage: (id: string, pageId: string) =>
    request<{ message: string }>(`${ws(id)}/pages/${pageId}`, { method: "DELETE" }),
  listLeads: (id: string, query?: Record<string, string | number | undefined>) =>
    request<Paginated<Lead>>(`${ws(id)}/leads`, { query }),

  // ---------------------------------------------------------------- billing
  checkout: (plan: string) =>
    request<{ url?: string; checkout_url?: string }>("/billing/checkout", {
      method: "POST", body: { plan },
    }),
  subscription: () => request<Record<string, unknown>>("/billing/subscription"),
  cancelSubscription: () =>
    request<{ message: string }>("/billing/cancel", { method: "POST", body: {} }),
};

export { request as apiRequest };
