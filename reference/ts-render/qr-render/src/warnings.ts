import { DesignV1 } from './design';
import { calculateContrastRatio, isLighter } from './contrast';

export interface ScanWarning {
  code: string;
  message: string;
  severity: 'info' | 'warning' | 'error';
  blocked: boolean;
}

export interface ScannabilityResult {
  score: number;
  label: 'Excellent' | 'Good' | 'Risky' | "Won't scan";
  contrastRatio: number;
  warnings: ScanWarning[];
  isBlocked: boolean;
}

export function calculateScannability(design: DesignV1, version: number = 4): ScannabilityResult {
  let score = 100;
  const warnings: ScanWarning[] = [];
  let isBlocked = false;

  const fg = design.modules.color;
  const bg = design.background.color;
  const contrast = calculateContrastRatio(fg, bg);

  // 1. Contrast checks
  if (contrast < 2.0) {
    score -= 60;
    isBlocked = true;
    warnings.push({
      code: 'contrast_too_low',
      message: 'Contrast is too low for camera sensors to decode.',
      severity: 'error',
      blocked: true,
    });
  } else if (contrast < 4.0) {
    score -= 35;
    warnings.push({
      code: 'low_contrast',
      message: 'Low contrast may cause scanning failures in dim lighting.',
      severity: 'warning',
      blocked: false,
    });
  } else if (contrast < 7.0) {
    score -= 10;
  }

  // 2. Inversion check
  if (isLighter(fg, bg)) {
    score -= 20;
    warnings.push({
      code: 'inverted',
      message: 'Inverted colors (light on dark) may not scan on older scanner apps.',
      severity: 'warning',
      blocked: false,
    });
  }

  // 3. Quiet zone
  if (design.quiet_zone < 2) {
    score -= 30;
    warnings.push({
      code: 'quiet_zone_small',
      message: 'Quiet zone margin is very small.',
      severity: 'warning',
      blocked: false,
    });
  } else if (design.quiet_zone < 4) {
    score -= 10;
  }

  // 4. Logo size
  if (design.logo) {
    if (design.logo.size_ratio > 0.30) {
      score -= 40;
      isBlocked = true;
      warnings.push({
        code: 'logo_too_large',
        message: 'Logo exceeds 30% of QR code area and blocks scanning.',
        severity: 'error',
        blocked: true,
      });
    } else if (design.logo.size_ratio > 0.25) {
      score -= 15;
      warnings.push({
        code: 'logo_large',
        message: 'Logo is large; ensure high error correction level is used.',
        severity: 'warning',
        blocked: false,
      });
    }
  }

  // 5. Version density
  if (version > 10) {
    score -= 10;
    warnings.push({
      code: 'dense',
      message: 'High module density; print at a larger size for reliable scanning.',
      severity: 'info',
      blocked: false,
    });
  }

  score = Math.max(0, Math.min(100, score));

  let label: 'Excellent' | 'Good' | 'Risky' | "Won't scan" = 'Excellent';
  if (score < 50 || isBlocked) {
    label = "Won't scan";
  } else if (score < 70) {
    label = 'Risky';
  } else if (score < 85) {
    label = 'Good';
  }

  return {
    score,
    label,
    contrastRatio: Math.round(contrast * 10) / 10,
    warnings,
    isBlocked,
  };
}
