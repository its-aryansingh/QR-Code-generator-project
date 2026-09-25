export interface AnalyticsSummary {
  total_scans: number;
  unique_scans: number;
  bot_hits: number;
  blocked_hits: number;
  scans_delta_pct: number;
  uniques_delta_pct: number;
  top_country?: string;
  top_device?: string;
}

export interface TimeSeriesPoint {
  timestamp: string;
  scans: number;
  unique_scans: number;
}

export interface BreakdownItem {
  key: string;
  label: string;
  count: number;
  percentage: number;
}

export interface HeatmapCell {
  day_of_week: number; // 0 = Sun .. 6 = Sat
  hour: number;        // 0 .. 23
  count: number;
}

export interface AnalyticsFilter {
  range: 'today' | '7d' | '30d' | '90d' | '12m';
  qr_id?: string;
  campaign_id?: string;
  country?: string;
  device?: string;
}
