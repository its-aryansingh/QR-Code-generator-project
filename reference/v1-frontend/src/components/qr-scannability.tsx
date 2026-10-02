import React, { useMemo } from 'react';
import { AlertCircle, CheckCircle2, Info, AlertTriangle } from 'lucide-react';

interface ScannabilityProps {
  fgColor: string;
  bgColor: string;
  logoSize: number;
  quietZone: number;
  errorLevel: string;
}

function hexToRgb(hex: string) {
  const shorthandRegex = /^#?([a-f\d])([a-f\d])([a-f\d])$/i;
  const fullHex = hex.replace(shorthandRegex, (m, r, g, b) => r + r + g + g + b + b);
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(fullHex);
  return result ? {
    r: parseInt(result[1], 16),
    g: parseInt(result[2], 16),
    b: parseInt(result[3], 16)
  } : { r: 0, g: 0, b: 0 };
}

function getLuminance(r: number, g: number, b: number) {
  const [rs, gs, bs] = [r / 255, g / 255, b / 255].map(c => {
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs;
}

export function getContrastRatio(hex1: string, hex2: string): number {
  const rgb1 = hexToRgb(hex1);
  const rgb2 = hexToRgb(hex2);
  const lum1 = getLuminance(rgb1.r, rgb1.g, rgb1.b);
  const lum2 = getLuminance(rgb2.r, rgb2.g, rgb2.b);
  const brightest = Math.max(lum1, lum2);
  const darkest = Math.min(lum1, lum2);
  return (brightest + 0.05) / (darkest + 0.05);
}

export const QRScannability: React.FC<ScannabilityProps> = ({ fgColor, bgColor, logoSize, quietZone, errorLevel }) => {
  const warnings = useMemo(() => {
    const issues = [];

    const ratio = getContrastRatio(fgColor, bgColor);
    if (ratio < 3) {
      issues.push({ type: 'error', text: 'Low contrast — this QR may not scan on some devices' });
    }

    const fgLum = getLuminance(hexToRgb(fgColor).r, hexToRgb(fgColor).g, hexToRgb(fgColor).b);
    const bgLum = getLuminance(hexToRgb(bgColor).r, hexToRgb(bgColor).g, hexToRgb(bgColor).b);
    if (bgLum < fgLum) {
      issues.push({ type: 'warning', text: 'Inverted colors — light QR on dark background may not scan reliably' });
    }

    if (logoSize > 25) {
      issues.push({ type: 'warning', text: 'Logo covers too much area — reduce size for reliable scanning' });
    }

    if (logoSize > 0 && errorLevel === 'L') {
      issues.push({ type: 'tip', text: 'Tip: Use High error correction when adding a logo' });
    }

    if (quietZone < 10) {
      issues.push({ type: 'tip', text: 'A larger quiet zone improves scannability' });
    }

    return issues;
  }, [fgColor, bgColor, logoSize, quietZone, errorLevel]);

  if (warnings.length === 0) {
    return (
      <div className="flex items-center gap-2 text-sm text-emerald-400 mt-2 bg-emerald-500/10 p-2 rounded-lg border border-emerald-500/20">
        <CheckCircle2 className="w-4 h-4" />
        <span>Excellent scannability</span>
      </div>
    );
  }

  return (
    <div className="mt-4 space-y-2">
      {warnings.map((w, i) => {
        if (w.type === 'error') {
          return (
            <div key={i} className="flex items-start gap-2 text-sm text-rose-400 bg-rose-500/10 p-2.5 rounded-lg border border-rose-500/20">
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
              <span>⚠️ {w.text.replace('⚠️ ', '')}</span>
            </div>
          );
        }
        if (w.type === 'warning') {
          return (
            <div key={i} className="flex items-start gap-2 text-sm text-amber-400 bg-amber-500/10 p-2.5 rounded-lg border border-amber-500/20">
              <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
              <span>⚠️ {w.text.replace('⚠️ ', '')}</span>
            </div>
          );
        }
        return (
          <div key={i} className="flex items-start gap-2 text-sm text-cyan-400 bg-cyan-500/10 p-2.5 rounded-lg border border-cyan-500/20">
            <Info className="w-4 h-4 mt-0.5 shrink-0" />
            <span>💡 {w.text.replace('💡 ', '')}</span>
          </div>
        );
      })}
    </div>
  );
};
