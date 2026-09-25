import { LogoConfig } from './design';

export interface LogoRenderResult {
  svg: string;
  defs?: string;
}

export function renderLogo(
  logo: LogoConfig | null | undefined,
  qrSize: number, // total QR size in pixels or units
  bgColor: string = '#ffffff'
): LogoRenderResult | null {
  if (!logo || !logo.url) {
    return null;
  }

  const ratio = Math.max(0.10, Math.min(0.30, logo.size_ratio));
  const logoSize = qrSize * ratio;
  const padding = (logo.padding || 0) * (qrSize / 50); // Scale padding relative to size
  const totalBoxSize = logoSize + padding * 2;

  const boxX = (qrSize - totalBoxSize) / 2;
  const boxY = (qrSize - totalBoxSize) / 2;

  const imgX = (qrSize - logoSize) / 2;
  const imgY = (qrSize - logoSize) / 2;

  const clipId = `logo-clip-${Math.round(boxX)}-${Math.round(boxY)}`;

  let defs = '';
  let clipAttr = '';
  let bgShape = '';

  if (logo.shape === 'circle') {
    const cx = qrSize / 2;
    const cy = qrSize / 2;
    const r = totalBoxSize / 2;
    const imgR = logoSize / 2;

    defs = `<clipPath id="${clipId}"><circle cx="${cx}" cy="${cy}" r="${imgR}" /></clipPath>`;
    clipAttr = `clip-path="url(#${clipId})"`;
    bgShape = `<circle cx="${cx}" cy="${cy}" r="${r}" fill="${bgColor}" />`;
  } else {
    const r = 4;
    defs = `<clipPath id="${clipId}"><rect x="${imgX}" y="${imgY}" width="${logoSize}" height="${logoSize}" rx="${r}" ry="${r}" /></clipPath>`;
    clipAttr = `clip-path="url(#${clipId})"`;
    bgShape = `<rect x="${boxX}" y="${boxY}" width="${totalBoxSize}" height="${totalBoxSize}" rx="${r}" ry="${r}" fill="${bgColor}" />`;
  }

  const svg = [
    bgShape,
    `<image href="${logo.url}" x="${imgX}" y="${imgY}" width="${logoSize}" height="${logoSize}" preserveAspectRatio="xMidYMid meet" ${clipAttr} />`,
  ].join('');

  return { svg, defs };
}
