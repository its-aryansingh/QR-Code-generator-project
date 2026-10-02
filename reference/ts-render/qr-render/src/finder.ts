import { FinderConfig } from './design';

export interface FinderPosition {
  row: number;
  col: number;
}

export function getFinderPositions(size: number): FinderPosition[] {
  return [
    { row: 0, col: 0 },
    { row: 0, col: size - 7 },
    { row: size - 7, col: 0 },
  ];
}

function roundedRectPath(x: number, y: number, w: number, h: number, r: number): string {
  return `M ${x + r} ${y} ` +
    `L ${x + w - r} ${y} Q ${x + w} ${y} ${x + w} ${y + r} ` +
    `L ${x + w} ${y + h - r} Q ${x + w} ${y + h} ${x + w - r} ${y + h} ` +
    `L ${x + r} ${y + h} Q ${x} ${y + h} ${x} ${y + h - r} ` +
    `L ${x} ${y + r} Q ${x} ${y} ${x + r} ${y} Z`;
}

function leafRectPath(x: number, y: number, w: number, h: number, r: number): string {
  // Leaf: top-left and bottom-right corners rounded, top-right and bottom-left sharp
  return `M ${x + r} ${y} ` +
    `L ${x + w} ${y} ` +
    `L ${x + w} ${y + h - r} Q ${x + w} ${y + h} ${x + w - r} ${y + h} ` +
    `L ${x} ${y + h} ` +
    `L ${x} ${y + r} Q ${x} ${y} ${x + r} ${y} Z`;
}

function circlePath(cx: number, cy: number, r: number): string {
  return `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx} ${cy + r} A ${r} ${r} 0 1 1 ${cx} ${cy - r} Z`;
}

export function renderFinderOuter(
  x: number,
  y: number,
  size: number,
  shape: FinderConfig['outer_shape']
): string {
  const w = 7 * size;
  const h = 7 * size;
  const innerOffset = size;
  const innerW = 5 * size;
  const innerH = 5 * size;

  switch (shape) {
    case 'circle': {
      const cx = x + w / 2;
      const cy = y + h / 2;
      const outerR = w / 2;
      const innerR = innerW / 2;
      return `${circlePath(cx, cy, outerR)} ${circlePath(cx, cy, innerR)}`;
    }

    case 'rounded': {
      const outerR = 2 * size;
      const innerR = 1 * size;
      const outer = roundedRectPath(x, y, w, h, outerR);
      const inner = roundedRectPath(x + innerOffset, y + innerOffset, innerW, innerH, innerR);
      return `${outer} ${inner}`;
    }

    case 'leaf': {
      const outerR = 3 * size;
      const innerR = 1.5 * size;
      const outer = leafRectPath(x, y, w, h, outerR);
      const inner = leafRectPath(x + innerOffset, y + innerOffset, innerW, innerH, innerR);
      return `${outer} ${inner}`;
    }

    case 'square':
    default: {
      const outer = `M ${x} ${y} h ${w} v ${h} h -${w} Z`;
      const inner = `M ${x + innerOffset} ${y + innerOffset} h ${innerW} v ${innerH} h -${innerW} Z`;
      return `${outer} ${inner}`;
    }
  }
}

export function renderFinderInner(
  x: number,
  y: number,
  size: number,
  shape: FinderConfig['inner_shape']
): string {
  const ix = x + 2 * size;
  const iy = y + 2 * size;
  const iw = 3 * size;
  const ih = 3 * size;

  switch (shape) {
    case 'circle': {
      const cx = ix + iw / 2;
      const cy = iy + ih / 2;
      return circlePath(cx, cy, iw / 2);
    }

    case 'dot': {
      const cx = ix + iw / 2;
      const cy = iy + ih / 2;
      return circlePath(cx, cy, iw * 0.4);
    }

    case 'rounded': {
      return roundedRectPath(ix, iy, iw, ih, size);
    }

    case 'square':
    default: {
      return `M ${ix} ${iy} h ${iw} v ${ih} h -${iw} Z`;
    }
  }
}
