import { GradientConfig } from './design';

function simpleHash(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = (hash << 5) - hash + str.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash).toString(16).slice(0, 8);
}

export interface GradientDef {
  id: string;
  svg: string;
}

export function renderGradientDef(config: GradientConfig | null | undefined, prefix: string = 'mod'): GradientDef | null {
  if (!config || !config.stops || config.stops.length === 0) {
    return null;
  }

  const hashKey = `${prefix}-${config.type}-${config.rotation}-${config.stops.map(s => `${s.offset}:${s.color}`).join('-')}`;
  const id = `g-${prefix}-${simpleHash(hashKey)}`;

  const stopElements = config.stops
    .map(stop => {
      const offsetPercent = Math.max(0, Math.min(100, Math.round(stop.offset * 100)));
      return `<stop offset="${offsetPercent}%" stop-color="${stop.color}" />`;
    })
    .join('');

  if (config.type === 'radial') {
    const svg = `<radialGradient id="${id}" cx="50%" cy="50%" r="50%" fx="50%" fy="50%">${stopElements}</radialGradient>`;
    return { id, svg };
  }

  // Linear gradient: rotation in degrees (0 = left-to-right, 90 = top-to-bottom)
  const rad = ((config.rotation % 360) * Math.PI) / 180;
  const x1 = Math.round(50 - 50 * Math.cos(rad));
  const y1 = Math.round(50 - 50 * Math.sin(rad));
  const x2 = Math.round(50 + 50 * Math.cos(rad));
  const y2 = Math.round(50 + 50 * Math.sin(rad));

  const svg = `<linearGradient id="${id}" x1="${x1}%" y1="${y1}%" x2="${x2}%" y2="${y2}%">${stopElements}</linearGradient>`;
  return { id, svg };
}
