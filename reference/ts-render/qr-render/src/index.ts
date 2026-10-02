import { DesignV1, DEFAULT_DESIGN } from './design';
import { generateMatrix, isFinderModule, calculateLogoBox, isLogoModule } from './matrix';
import { renderModulePath } from './shapes';
import { getFinderPositions, renderFinderOuter, renderFinderInner } from './finder';
import { renderGradientDef } from './gradient';
import { renderLogo } from './logo';
import { computeFrameLayout } from './frame';
import { calculateScannability, ScanWarning } from './warnings';

export * from './design';
export * from './canonical';
export * from './contrast';
export * from './warnings';
export * from './matrix';
export * from './shapes';
export * from './finder';
export * from './gradient';
export * from './logo';
export * from './frame';

export interface RenderSvgInput {
  payload: string;
  design?: Partial<DesignV1>;
  sizePx?: number;
}

export interface RenderSvgResult {
  svg: string;
  version: number;
  ecc: 'L' | 'M' | 'Q' | 'H';
  modules: number;
  warnings: ScanWarning[];
}

export function renderSvg(input: RenderSvgInput): RenderSvgResult {
  const design: DesignV1 = {
    ...DEFAULT_DESIGN,
    ...(input.design || {}),
    modules: {
      ...DEFAULT_DESIGN.modules,
      ...(input.design?.modules || {}),
    },
    finder: {
      ...DEFAULT_DESIGN.finder,
      ...(input.design?.finder || {}),
    },
    background: {
      ...DEFAULT_DESIGN.background,
      ...(input.design?.background || {}),
    },
  };

  const matrix = generateMatrix(input.payload, design.ecc);
  const qz = Math.max(0, Math.min(10, design.quiet_zone));
  const moduleScale = 10;
  const qrGridModules = matrix.size + 2 * qz;
  const qrSize = qrGridModules * moduleScale;

  const layout = computeFrameLayout(design.frame, qrSize, design.background.color);
  const { totalWidth, totalHeight, qrX, qrY } = layout;

  const defs: string[] = [];

  // Gradient
  let moduleFill = design.modules.color;
  if (design.modules.gradient) {
    const gradDef = renderGradientDef(design.modules.gradient, 'mod');
    if (gradDef) {
      defs.push(gradDef.svg);
      moduleFill = `url(#${gradDef.id})`;
    }
  }

  // Logo box check
  const logoBox = calculateLogoBox(matrix.size, design);

  // Modules path
  const modulesPathParts: string[] = [];
  for (let r = 0; r < matrix.size; r++) {
    for (let c = 0; c < matrix.size; c++) {
      if (isFinderModule(r, c, matrix.size)) continue;
      if (isLogoModule(r, c, logoBox)) continue;
      if (!matrix.data[r][c]) continue;

      const top = r > 0 && matrix.data[r - 1][c] && !isFinderModule(r - 1, c, matrix.size) && !isLogoModule(r - 1, c, logoBox);
      const bottom = r < matrix.size - 1 && matrix.data[r + 1][c] && !isFinderModule(r + 1, c, matrix.size) && !isLogoModule(r + 1, c, logoBox);
      const left = c > 0 && matrix.data[r][c - 1] && !isFinderModule(r, c - 1, matrix.size) && !isLogoModule(r, c - 1, logoBox);
      const right = c < matrix.size - 1 && matrix.data[r][c + 1] && !isFinderModule(r, c + 1, matrix.size) && !isLogoModule(r, c + 1, logoBox);

      const mx = qrX + (c + qz) * moduleScale;
      const my = qrY + (r + qz) * moduleScale;

      modulesPathParts.push(
        renderModulePath(mx, my, moduleScale, design.modules.shape, { top, bottom, left, right })
      );
    }
  }

  // Finder paths
  const finderPositions = getFinderPositions(matrix.size);
  const finderOuterParts: string[] = [];
  const finderInnerParts: string[] = [];

  for (const pos of finderPositions) {
    const fx = qrX + (pos.col + qz) * moduleScale;
    const fy = qrY + (pos.row + qz) * moduleScale;

    finderOuterParts.push(
      renderFinderOuter(fx, fy, moduleScale, design.finder.outer_shape)
    );
    finderInnerParts.push(
      renderFinderInner(fx, fy, moduleScale, design.finder.inner_shape)
    );
  }

  // Logo rendering
  let logoSvg = '';
  if (design.logo && design.logo.url) {
    const logoResult = renderLogo(design.logo, qrSize, design.background.color);
    if (logoResult) {
      if (logoResult.defs) defs.push(logoResult.defs);
      logoSvg = `<g transform="translate(${qrX}, ${qrY})">${logoResult.svg}</g>`;
    }
  }

  // Scannability warnings
  const scannability = calculateScannability(design, matrix.version);

  // SVG Assembly
  const defsBlock = defs.length > 0 ? `<defs>${defs.join('')}</defs>` : '';
  const bgRect = !design.background.transparent
    ? `<rect x="${qrX}" y="${qrY}" width="${qrSize}" height="${qrSize}" fill="${design.background.color}" />`
    : '';

  const sizeAttrs = input.sizePx
    ? `width="${input.sizePx}" height="${Math.round(input.sizePx * (totalHeight / totalWidth))}" `
    : '';

  const svg = [
    `<svg xmlns="http://www.w3.org/2000/svg" ${sizeAttrs}viewBox="0 0 ${totalWidth} ${totalHeight}">`,
    defsBlock,
    layout.frameSvg,
    bgRect,
    `<path d="${modulesPathParts.join(' ')}" fill="${moduleFill}" />`,
    `<path fill-rule="evenodd" d="${finderOuterParts.join(' ')}" fill="${design.finder.outer_color}" />`,
    `<path d="${finderInnerParts.join(' ')}" fill="${design.finder.inner_color}" />`,
    logoSvg,
    '</svg>',
  ].join('');

  return {
    svg,
    version: matrix.version,
    ecc: matrix.ecc,
    modules: matrix.size,
    warnings: scannability.warnings,
  };
}
