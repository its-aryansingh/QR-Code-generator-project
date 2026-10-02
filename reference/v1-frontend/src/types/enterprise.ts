export type Role = "owner" | "admin" | "editor" | "viewer";
export type Plan = "free" | "starter" | "pro" | "enterprise";

export const ROLE_RANK: Record<Role, number> = {
  viewer: 1,
  editor: 2,
  admin: 3,
  owner: 4,
};

export function roleAtLeast(role: Role | undefined, minimum: Role): boolean {
  if (!role) return false;
  return ROLE_RANK[role] >= ROLE_RANK[minimum];
}

export interface ApiEnvelope<T> {
  success: boolean;
  data?: T;
  error?: string;
  code?: string;
  [key: string]: unknown;
}

export interface UserSummary {
  id: string;
  email: string;
  name?: string | null;
  avatar_url?: string | null;
}

export interface Profile extends UserSummary {
  company?: string | null;
  plan: Plan;
  plan_expires_at?: string | null;
  subscription_status?: string | null;
  subscription_ends_at?: string | null;
  default_workspace_id?: string | null;
  created_at?: string | null;
}

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  description?: string | null;
  owner_id?: string | null;
  plan: Plan;
  role?: Role;
  logo_url?: string | null;
  brand_color: string;
  brand_logo?: string | null;
  favicon_url?: string | null;
  custom_css?: string | null;
  custom_footer?: string | null;
  custom_domain?: string | null;
  remove_branding: boolean;
  sso_enabled: boolean;
  qr_count?: number;
  scan_count?: number;
  member_count?: number;
  campaign_count?: number;
  folder_count?: number;
  entitlements?: Entitlements;
  created_at?: string | null;
  updated_at?: string | null;
}

export type FeatureName =
  | "bulk" | "webhooks" | "campaigns" | "templates" | "routing" | "audit_log"
  | "white_label" | "custom_domain" | "gs1" | "exports" | "sso" | "scim"
  | "security_policy" | "priority_support" | "sla";

export interface Entitlements {
  plan: Plan;
  role?: Role;
  limits: {
    max_workspaces: number;
    max_members: number;
    max_qr_codes: number;
    max_folders: number;
    max_campaigns: number;
    max_templates: number;
    max_api_keys: number;
    max_custom_domains: number;
    bulk_batch_size: number;
    analytics_retention_days: number;
  };
  features: Record<FeatureName, { enabled: boolean; label: string; min_plan: Plan }>;
  usage?: Record<string, number>;
  api_daily_limit: number;
}

export interface Member {
  id: string;
  role: Role;
  joined_at?: string | null;
  invited_by?: string | null;
  is_you: boolean;
  user: UserSummary;
}

export interface Invite {
  id: string;
  email: string;
  role: Role;
  status: "pending" | "accepted" | "revoked" | "expired" | "superseded";
  expires_at: string;
  created_at?: string | null;
  invited_by?: string | null;
  invite_url?: string | null;
}

export interface Folder {
  id: string;
  workspace_id: string;
  parent_id?: string | null;
  name: string;
  description?: string | null;
  color: string;
  icon?: string | null;
  sort_order: number;
  qr_count: number;
  scan_count: number;
  children: Folder[];
  created_at?: string | null;
}

export type CampaignStatus =
  | "draft" | "scheduled" | "active" | "paused" | "completed" | "archived";

export interface Campaign {
  id: string;
  workspace_id: string;
  name: string;
  description?: string | null;
  status: CampaignStatus;
  color: string;
  starts_at?: string | null;
  ends_at?: string | null;
  scan_goal: number;
  utm: {
    source?: string | null;
    medium?: string | null;
    campaign?: string | null;
    term?: string | null;
    content?: string | null;
  };
  tags: string[];
  qr_count: number;
  scan_count: number;
  recent_scans?: number;
  goal_progress?: number | null;
  qr_codes?: Array<{
    id: string;
    title?: string | null;
    qr_type: string;
    short_code?: string | null;
    scan_count: number;
    is_active: boolean;
  }>;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface QRTemplate {
  id: string;
  workspace_id: string;
  name: string;
  description?: string | null;
  design: Record<string, unknown>;
  preview_url?: string | null;
  is_default: boolean;
  is_locked: boolean;
  usage_count: number;
  created_at?: string | null;
  updated_at?: string | null;
}

export type RoutingCondition =
  | "country" | "device" | "os" | "language"
  | "time_of_day" | "date_range" | "weight" | "scan_count";

export interface RoutingRule {
  id: string;
  qr_id: string;
  name?: string | null;
  condition: RoutingCondition;
  operator: string;
  value?: string | null;
  destination_url: string;
  priority: number;
  is_active: boolean;
  hit_count: number;
  created_at?: string | null;
}

export interface WorkspaceQR {
  id: string;
  workspace_id?: string | null;
  folder_id?: string | null;
  folder_name?: string | null;
  campaign_id?: string | null;
  campaign_name?: string | null;
  template_id?: string | null;
  title?: string | null;
  content: string;
  qr_type: string;
  size: number;
  is_dynamic: boolean;
  short_code?: string | null;
  short_url?: string | null;
  redirect_url?: string | null;
  is_active: boolean;
  scan_count: number;
  tags: string[];
  metadata?: Record<string, unknown>;
  customization?: Record<string, unknown>;
  gs1?: Record<string, unknown> | null;
  max_scans?: number | null;
  expires_at?: string | null;
  scheduled_at?: string | null;
  qr_base64?: string;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface Paginated<T> {
  items: T[];
  total: number;
  page: number;
  limit: number;
  pages: number;
}

export interface QRListResponse extends Paginated<WorkspaceQR> {
  facets: {
    types: Array<{ qr_type: string; count: number }>;
    total_scans: number;
  };
}

export interface CountPoint {
  count: number;
  [key: string]: string | number | null;
}

export interface WorkspaceAnalytics {
  range_days: number;
  retention_days: number;
  generated_at: string;
  totals: {
    total_scans: number;
    unique_visitors: number;
    total_qr_codes: number;
    active_qr_codes: number;
    dynamic_qr_codes: number;
    scanned_qr_codes: number;
    lifetime_scans: number;
    avg_scans_per_qr: number;
    engagement_rate: number;
  };
  trend: { scans: number; previous_scans: number; unique_visitors: number };
  scans_by_date: Array<{ date: string; count: number }>;
  scans_by_hour: Array<{ hour: number; count: number }>;
  by_country: Array<{ country: string; count: number }>;
  by_city: Array<{ city: string; country_code: string; count: number }>;
  by_device: Array<{ device: string; count: number }>;
  by_os: Array<{ os: string; count: number }>;
  by_browser: Array<{ browser: string; count: number }>;
  by_language: Array<{ language: string; count: number }>;
  by_referrer: Array<{ referrer: string; count: number }>;
  by_type: Array<{ qr_type: string; count: number; scans: number }>;
  top_qr_codes: Array<{
    id: string; title?: string | null; qr_type: string;
    short_code?: string | null; scan_count: number; is_active: boolean;
  }>;
  recent_scans: Array<{
    id: string; scanned_at: string | null; qr_id: string; qr_title?: string | null;
    country?: string | null; country_code?: string | null; city?: string | null;
    device?: string | null; os?: string | null; browser?: string | null;
  }>;
}

export interface WorkspaceOverview {
  range_days: number;
  stats: {
    total_qr_codes: number;
    active_qr_codes: number;
    dynamic_qr_codes: number;
    total_scans: number;
    scans_delta: number;
    unique_visitors: number;
    active_campaigns: number;
    new_leads: number;
  };
  scans_by_date: Array<{ date: string; count: number }>;
  by_device: Array<{ device: string; count: number }>;
  top_qr_codes: Array<{
    id: string; title?: string | null; qr_type: string;
    short_code?: string | null; scan_count: number; is_active: boolean;
  }>;
  recent_qr_codes: Array<{
    id: string; title?: string | null; qr_type: string;
    short_code?: string | null; scan_count: number; created_at?: string | null;
  }>;
}

export interface AuditEntry {
  id: string;
  action: string;
  resource: string;
  resource_id?: string | null;
  details?: Record<string, unknown> | null;
  ip_address?: string | null;
  user_agent?: string | null;
  created_at?: string | null;
  user: UserSummary | null;
}

export interface AuditResponse extends Paginated<AuditEntry> {
  actions: string[];
  resources: string[];
}

export interface Webhook {
  id: string;
  workspace_id: string;
  url: string;
  events: string[];
  is_active: boolean;
  description?: string | null;
  last_triggered?: string | null;
  fail_count: number;
  delivery_count?: number;
  secret?: string;
  created_at?: string | null;
}

export interface WebhookEvent {
  id: string;
  label: string;
  description: string;
}

export interface WebhookLog {
  id: string;
  webhook_id: string;
  event: string;
  payload?: string | null;
  status_code: number;
  response?: string | null;
  success: boolean;
  duration_ms: number;
  error?: string | null;
  created_at?: string | null;
}

export interface ApiKey {
  id: string;
  workspace_id: string;
  name: string;
  prefix: string;
  masked: string;
  scopes: string[];
  last_used_at?: string | null;
  calls_today: number;
  total_calls: number;
  expires_at?: string | null;
  revoked_at?: string | null;
  is_active: boolean;
  key?: string;
  created_at?: string | null;
}

export interface CustomDomain {
  id: string;
  workspace_id: string;
  domain: string;
  status: "pending" | "verified" | "failed";
  is_primary: boolean;
  ssl_status: string;
  verification_method: string;
  dns_record: { type: string; host: string; value: string };
  cname_record: { type: string; host: string; value: string };
  verified_at?: string | null;
  last_checked_at?: string | null;
  created_at?: string | null;
}

export interface SsoConfig {
  workspace_id: string;
  is_enabled: boolean;
  provider: "saml" | "oidc";
  entity_id?: string | null;
  sso_url?: string | null;
  slo_url?: string | null;
  certificate?: boolean | string | null;
  metadata_url?: string | null;
  email_domains: string[];
  default_role: Role;
  enforce_sso: boolean;
  scim_enabled: boolean;
  has_scim_token: boolean;
  acs_url: string;
  sp_entity_id: string;
  available?: boolean;
  updated_at?: string | null;
}

export interface SecurityPolicy {
  workspace_id: string;
  allowed_domains: string[];
  blocked_domains: string[];
  require_https: boolean;
  require_approval: boolean;
  scan_alert_threshold: number;
  password_min_length: number;
  session_timeout_minutes: number;
  available?: boolean;
  updated_at?: string | null;
}

export interface Branding {
  workspace_id: string;
  brand_color: string;
  brand_logo?: string | null;
  logo_url?: string | null;
  favicon_url?: string | null;
  custom_css?: string | null;
  custom_footer?: string | null;
  remove_branding: boolean;
  custom_domain?: string | null;
  can_white_label: boolean;
}

export interface BulkJob {
  id: string;
  workspace_id: string;
  filename?: string | null;
  status: "pending" | "processing" | "completed" | "partial" | "failed";
  total_rows: number;
  success_count: number;
  error_count: number;
  errors: Array<{ row: number; content: string; error: string }>;
  options: Record<string, unknown>;
  created_at?: string | null;
  completed_at?: string | null;
}

export interface BulkResult {
  row: number;
  id: string;
  title?: string | null;
  content: string;
  short_code?: string | null;
  short_url?: string | null;
  qr_base64: string;
}

export interface LeadPage {
  id: string;
  workspace_id: string;
  qr_record_id?: string | null;
  name: string;
  slug: string;
  headline?: string | null;
  subheadline?: string | null;
  hero_image?: string | null;
  button_text: string;
  button_color: string;
  background_color: string;
  text_color: string;
  thank_you_message: string;
  redirect_url?: string | null;
  form_fields: Array<{ name: string; label: string; type: string; required?: boolean }>;
  is_active: boolean;
  requires_opt_in: boolean;
  privacy_policy?: string | null;
  consent_text?: string | null;
  views: number;
  submissions: number;
  lead_count: number;
  conversion_rate: number;
  created_at?: string | null;
}

export interface Lead {
  id: string;
  page_id: string;
  page_name?: string | null;
  email?: string | null;
  data: Record<string, unknown>;
  opted_in: boolean;
  source?: string | null;
  ip_address?: string | null;
  created_at?: string | null;
}
