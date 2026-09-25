export type PlanCode = 'free' | 'pro' | 'business' | 'enterprise';

export interface PlanLimits {
  active_dynamic_qrs: number;
  scans_per_month: number;
  retention_days: number;
  custom_domains: number;
  team_seats: number;
}

export interface PlanFeatures {
  custom_domain: boolean;
  remove_branding: boolean;
  bulk_create: boolean;
  password_protection: boolean;
  expiry_scheduling: boolean;
  scan_limits: boolean;
  advanced_analytics: boolean;
  rules_engine: boolean;
  webhooks: boolean;
  api_access: boolean;
  sso: boolean;
}

export interface PlanConfig {
  code: PlanCode;
  name: string;
  priceMonthlyUSD: number;
  priceYearlyUSD: number;
  priceMonthlyINR: number;
  priceYearlyINR: number;
  limits: PlanLimits;
  features: PlanFeatures;
}

export const PLANS: Record<PlanCode, PlanConfig> = {
  free: {
    code: 'free',
    name: 'Free',
    priceMonthlyUSD: 0,
    priceYearlyUSD: 0,
    priceMonthlyINR: 0,
    priceYearlyINR: 0,
    limits: {
      active_dynamic_qrs: 10,
      scans_per_month: 1000,
      retention_days: 30,
      custom_domains: 0,
      team_seats: 1,
    },
    features: {
      custom_domain: false,
      remove_branding: false,
      bulk_create: false,
      password_protection: false,
      expiry_scheduling: false,
      scan_limits: false,
      advanced_analytics: false,
      rules_engine: false,
      webhooks: false,
      api_access: false,
      sso: false,
    },
  },
  pro: {
    code: 'pro',
    name: 'Pro',
    priceMonthlyUSD: 12,
    priceYearlyUSD: 99,
    priceMonthlyINR: 999,
    priceYearlyINR: 7999,
    limits: {
      active_dynamic_qrs: 100,
      scans_per_month: 25000,
      retention_days: 365,
      custom_domains: 1,
      team_seats: 3,
    },
    features: {
      custom_domain: true,
      remove_branding: true,
      bulk_create: true,
      password_protection: true,
      expiry_scheduling: true,
      scan_limits: true,
      advanced_analytics: true,
      rules_engine: false,
      webhooks: false,
      api_access: false,
      sso: false,
    },
  },
  business: {
    code: 'business',
    name: 'Business',
    priceMonthlyUSD: 39,
    priceYearlyUSD: 349,
    priceMonthlyINR: 2999,
    priceYearlyINR: 27999,
    limits: {
      active_dynamic_qrs: 1000,
      scans_per_month: 100000,
      retention_days: 730,
      custom_domains: 5,
      team_seats: 10,
    },
    features: {
      custom_domain: true,
      remove_branding: true,
      bulk_create: true,
      password_protection: true,
      expiry_scheduling: true,
      scan_limits: true,
      advanced_analytics: true,
      rules_engine: true,
      webhooks: true,
      api_access: true,
      sso: false,
    },
  },
  enterprise: {
    code: 'enterprise',
    name: 'Enterprise',
    priceMonthlyUSD: 149,
    priceYearlyUSD: 1299,
    priceMonthlyINR: 11999,
    priceYearlyINR: 99999,
    limits: {
      active_dynamic_qrs: -1,
      scans_per_month: -1,
      retention_days: 1095,
      custom_domains: -1,
      team_seats: -1,
    },
    features: {
      custom_domain: true,
      remove_branding: true,
      bulk_create: true,
      password_protection: true,
      expiry_scheduling: true,
      scan_limits: true,
      advanced_analytics: true,
      rules_engine: true,
      webhooks: true,
      api_access: true,
      sso: true,
    },
  },
};

export function hasFeature(plan: PlanCode, feature: keyof PlanFeatures): boolean {
  const p = PLANS[plan] || PLANS.free;
  return Boolean(p.features[feature]);
}

export function getLimit(plan: PlanCode, limit: keyof PlanLimits): number {
  const p = PLANS[plan] || PLANS.free;
  return p.limits[limit];
}
