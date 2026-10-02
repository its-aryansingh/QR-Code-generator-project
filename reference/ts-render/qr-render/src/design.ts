export interface GradientStop {
  offset: number;
  color: string;
}

export interface GradientConfig {
  type: 'linear' | 'radial';
  rotation: number; // 0-359
  stops: GradientStop[];
}

export interface ModulesConfig {
  shape: 'square' | 'dots' | 'rounded' | 'extra-rounded' | 'classy' | 'classy-rounded';
  color: string;
  gradient?: GradientConfig | null;
}

export interface FinderConfig {
  outer_shape: 'square' | 'rounded' | 'circle' | 'leaf';
  outer_color: string;
  inner_shape: 'square' | 'rounded' | 'circle' | 'dot';
  inner_color: string;
}

export interface BackgroundConfig {
  color: string;
  transparent: boolean;
}

export interface LogoConfig {
  file_id?: string;
  url?: string;
  size_ratio: number; // 0.10 - 0.30
  padding: number;    // 0 - 4
  clear_modules: boolean;
  shape: 'square' | 'circle';
}

export interface FrameConfig {
  style: 'banner-bottom' | 'banner-top' | 'rounded-box' | 'speech';
  text: string;       // <= 24 chars
  text_color: string;
  color: string;
}

export interface DesignV1 {
  v: 1;
  ecc: 'auto' | 'L' | 'M' | 'Q' | 'H';
  quiet_zone: number; // 0-10
  modules: ModulesConfig;
  finder: FinderConfig;
  background: BackgroundConfig;
  logo?: LogoConfig | null;
  frame?: FrameConfig | null;
}

export const DEFAULT_DESIGN: DesignV1 = {
  v: 1,
  ecc: 'auto',
  quiet_zone: 4,
  modules: {
    shape: 'square',
    color: '#111111',
    gradient: null,
  },
  finder: {
    outer_shape: 'square',
    outer_color: '#111111',
    inner_shape: 'square',
    inner_color: '#111111',
  },
  background: {
    color: '#ffffff',
    transparent: false,
  },
  logo: null,
  frame: null,
};

const HEX_COLOR_REGEX = /^#[0-9a-fA-F]{6}$/;

export function validateDesign(d: any): { valid: boolean; error?: string } {
  if (!d || typeof d !== 'object') return { valid: false, error: 'design must be an object' };
  if (d.v !== 1) return { valid: false, error: 'design version must be 1' };

  const ecc = String(d.ecc || '').toUpperCase();
  if (!['AUTO', 'L', 'M', 'Q', 'H'].includes(ecc)) {
    return { valid: false, error: 'invalid ecc level' };
  }

  if (typeof d.quiet_zone !== 'number' || d.quiet_zone < 0 || d.quiet_zone > 10) {
    return { valid: false, error: 'quiet_zone must be between 0 and 10' };
  }

  if (!d.modules || typeof d.modules !== 'object') return { valid: false, error: 'missing modules configuration' };
  if (!HEX_COLOR_REGEX.test(d.modules.color)) return { valid: false, error: 'modules.color must be valid hex #RRGGBB' };

  if (!d.finder || typeof d.finder !== 'object') return { valid: false, error: 'missing finder configuration' };
  if (!HEX_COLOR_REGEX.test(d.finder.outer_color)) return { valid: false, error: 'finder.outer_color must be valid hex' };
  if (!HEX_COLOR_REGEX.test(d.finder.inner_color)) return { valid: false, error: 'finder.inner_color must be valid hex' };

  if (!d.background || typeof d.background !== 'object') return { valid: false, error: 'missing background configuration' };
  if (!HEX_COLOR_REGEX.test(d.background.color)) return { valid: false, error: 'background.color must be valid hex' };

  if (d.logo) {
    if (typeof d.logo.size_ratio !== 'number' || d.logo.size_ratio < 0.1 || d.logo.size_ratio > 0.3) {
      return { valid: false, error: 'logo.size_ratio must be between 0.10 and 0.30' };
    }
  }

  if (d.frame) {
    if (typeof d.frame.text !== 'string' || d.frame.text.length > 24) {
      return { valid: false, error: 'frame.text must be <= 24 characters' };
    }
    if (!HEX_COLOR_REGEX.test(d.frame.text_color)) return { valid: false, error: 'frame.text_color must be valid hex' };
    if (!HEX_COLOR_REGEX.test(d.frame.color)) return { valid: false, error: 'frame.color must be valid hex' };
  }

  return { valid: true };
}
