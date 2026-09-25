import { DesignV1, validateDesign } from './design';

function sortKeys(obj: any): any {
  if (Array.isArray(obj)) {
    return obj.map(sortKeys);
  }
  if (obj !== null && typeof obj === 'object') {
    const sorted: Record<string, any> = {};
    for (const key of Object.keys(obj).sort()) {
      sorted[key] = sortKeys(obj[key]);
    }
    return sorted;
  }
  return obj;
}

export function canonicalizeDesign(design: DesignV1): DesignV1 {
  const check = validateDesign(design);
  if (!check.valid) {
    throw new Error(check.error);
  }

  const copy = JSON.parse(JSON.stringify(design)) as DesignV1;
  copy.modules.color = copy.modules.color.toLowerCase();
  copy.finder.outer_color = copy.finder.outer_color.toLowerCase();
  copy.finder.inner_color = copy.finder.inner_color.toLowerCase();
  copy.background.color = copy.background.color.toLowerCase();

  if (copy.modules.gradient) {
    for (const stop of copy.modules.gradient.stops) {
      stop.color = stop.color.toLowerCase();
    }
  }

  if (copy.frame) {
    copy.frame.text_color = copy.frame.text_color.toLowerCase();
    copy.frame.color = copy.frame.color.toLowerCase();
  }

  return sortKeys(copy);
}

export function canonicalJson(design: DesignV1): string {
  const canonical = canonicalizeDesign(design);
  return JSON.stringify(canonical);
}
