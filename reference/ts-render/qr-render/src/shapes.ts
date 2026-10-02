export interface Neighbours {
  top: boolean;
  bottom: boolean;
  left: boolean;
  right: boolean;
}

export type ModuleShape = 'square' | 'dots' | 'rounded' | 'extra-rounded' | 'classy' | 'classy-rounded';

export function renderModulePath(
  x: number,
  y: number,
  size: number,
  shape: ModuleShape,
  n: Neighbours
): string {
  switch (shape) {
    case 'dots': {
      const r = size * 0.44;
      const cx = x + size / 2;
      const cy = y + size / 2;
      return `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx} ${cy + r} A ${r} ${r} 0 1 1 ${cx} ${cy - r} Z`;
    }

    case 'rounded': {
      const r = size * 0.25;
      return `M ${x + r} ${y} L ${x + size - r} ${y} Q ${x + size} ${y} ${x + size} ${y + r} L ${x + size} ${y + size - r} Q ${x + size} ${y + size} ${x + size - r} ${y + size} L ${x + r} ${y + size} Q ${x} ${y + size} ${x} ${y + size - r} L ${x} ${y + r} Q ${x} ${y} ${x + r} ${y} Z`;
    }

    case 'extra-rounded': {
      const r = size * 0.5;
      return `M ${x + r} ${y} L ${x + size - r} ${y} A ${r} ${r} 0 0 1 ${x + size} ${y + r} L ${x + size} ${y + size - r} A ${r} ${r} 0 0 1 ${x + size - r} ${y + size} L ${x + r} ${y + size} A ${r} ${r} 0 0 1 ${x} ${y + size - r} L ${x} ${y + r} A ${r} ${r} 0 0 1 ${x + r} ${y} Z`;
    }

    case 'classy': {
      // Rounded corners only where there is no adjacent neighbour
      const r = size * 0.4;
      const rTL = !n.top && !n.left ? r : 0;
      const rTR = !n.top && !n.right ? r : 0;
      const rBR = !n.bottom && !n.right ? r : 0;
      const rBL = !n.bottom && !n.left ? r : 0;

      return `M ${x + rTL} ${y} ` +
        `L ${x + size - rTR} ${y} ` +
        (rTR ? `Q ${x + size} ${y} ${x + size} ${y + rTR} ` : '') +
        `L ${x + size} ${y + size - rBR} ` +
        (rBR ? `Q ${x + size} ${y + size} ${x + size - rBR} ${y + size} ` : '') +
        `L ${x + rBL} ${y + size} ` +
        (rBL ? `Q ${x} ${y + size} ${x} ${y + size - rBL} ` : '') +
        `L ${x} ${y + rTL} ` +
        (rTL ? `Q ${x} ${y} ${x + rTL} ${y} ` : '') +
        'Z';
    }

    case 'classy-rounded': {
      const r = size * 0.48;
      const rTL = !n.top && !n.left ? r : 0;
      const rTR = !n.top && !n.right ? r : 0;
      const rBR = !n.bottom && !n.right ? r : 0;
      const rBL = !n.bottom && !n.left ? r : 0;

      return `M ${x + rTL} ${y} ` +
        `L ${x + size - rTR} ${y} ` +
        (rTR ? `A ${rTR} ${rTR} 0 0 1 ${x + size} ${y + rTR} ` : '') +
        `L ${x + size} ${y + size - rBR} ` +
        (rBR ? `A ${rBR} ${rBR} 0 0 1 ${x + size - rBR} ${y + size} ` : '') +
        `L ${x + rBL} ${y + size} ` +
        (rBL ? `A ${rBL} ${rBL} 0 0 1 ${x} ${y + size - rBL} ` : '') +
        `L ${x} ${y + rTL} ` +
        (rTL ? `A ${rTL} ${rTL} 0 0 1 ${x + rTL} ${y} ` : '') +
        'Z';
    }

    case 'square':
    default:
      return `M ${x} ${y} h ${size} v ${size} h -${size} Z`;
  }
}
