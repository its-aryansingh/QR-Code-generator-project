export interface User {
  id: string;
  email: string;
  name?: string;
  company?: string;
  plan: string;
  created_at: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
}

export interface QRType {
  id: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  is_premium: boolean;
}

export interface QRCustomization {
  foreground_color?: string;
  background_color?: string;
  corner_style?: 'square' | 'rounded' | 'dots';
  body_style?: 'square' | 'rounded' | 'dots';
  logo?: {
    url: string;
    size: number;
  };
  frame?: {
    style: 'none' | 'bottom-text' | 'top-text';
    text: string;
    color: string;
  };
}

export interface QRRecord {
  id: string;
  user_id: string;
  title?: string;
  content: string;
  qr_type: string;
  size: number;
  short_code?: string;
  is_dynamic: boolean;
  redirect_url?: string;
  customization?: QRCustomization;
  is_active: boolean;
  scan_count: number;
  created_at: string;
}

export interface QRGenerateResponse {
  id: string;
  title?: string;
  content: string;
  qr_type: string;
  size: number;
  short_code?: string;
  is_dynamic: boolean;
  qr_base64: string;
  created_at: string;
}

export interface QRHistoryResponse {
  records: QRRecord[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface PublicGenerateResponse {
  success: boolean;
  qr_base64: string;
  content: string;
  qr_type: string;
  size: number;
  remaining: number;
  limit: number;
  short_code?: string;
  is_dynamic?: boolean;
  error?: string;
}

export interface ScanAnalytics {
  total_scans: number;
  unique_scans: number;
  scans_by_date: { date: string; count: number }[];
  scans_by_country: { country_code: string; country_name: string; count: number }[];
  scans_by_device: { device_type: string; count: number }[];
  scans_by_browser: { browser: string; count: number }[];
  scans_by_os: { os: string; count: number }[];
  scans_by_referrer: { referrer: string; count: number }[];
  top_cities: { city: string; country_code: string; count: number }[];
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  code?: string;
}

// ==================== OAuth (Google / GitHub) ====================

export type OAuthProvider = 'google' | 'github';

export type OAuthProviders = Record<OAuthProvider, boolean>;

export interface OAuthUser {
  id: string;
  email: string;
  name?: string | null;
  plan?: string | null;
  avatar_url?: string | null;
  email_verified: boolean;
  has_password: boolean;
}

export interface OAuthExchangeResult extends TokenPair {
  user: OAuthUser;
  next: string;
  provider: OAuthProvider;
  is_new_user: boolean;
  password_reset: boolean;
}

export interface OAuthLinkStart {
  start_url: string;
  intent: string;
}

export interface ConnectedAccount {
  provider: OAuthProvider;
  email?: string | null;
  connected_at: string;
  last_login_at?: string | null;
}

export interface ConnectedAccounts {
  has_password: boolean;
  providers: OAuthProviders;
  accounts: ConnectedAccount[];
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  name?: string;
}

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  password: string;
}

export interface GenerateQRRequest {
  title?: string;
  content?: string;
  qr_type: string;
  size?: number;
  is_dynamic?: boolean;
  metadata?: Record<string, unknown>;
  customization?: QRCustomization;
}

export interface PublicGenerateRequest {
  content: string;
  qr_type: string;
  size?: number;
  is_dynamic?: boolean;
  metadata?: Record<string, unknown>;
  customization?: {
    foreground_color?: string;
    background_color?: string;
  };
}

// Metadata types for different QR types
export interface URLMetadata {
  url: string;
}

export interface WiFiMetadata {
  ssid: string;
  password?: string;
  encryption: 'WPA' | 'WEP' | 'nopass';
  hidden?: boolean;
}

export interface VCardMetadata {
  first_name: string;
  last_name: string;
  email?: string;
  phone?: string;
  mobile?: string;
  company?: string;
  job_title?: string;
  website?: string;
  address?: string;
  city?: string;
  state?: string;
  country?: string;
  postal_code?: string;
  note?: string;
}

export interface EmailMetadata {
  to: string;
  subject?: string;
  body?: string;
}

export interface SMSMetadata {
  phone: string;
  message?: string;
}

export interface SocialMetadata {
  platform: string;
  username?: string;
  url?: string;
  phone?: string;
  message?: string;
}
