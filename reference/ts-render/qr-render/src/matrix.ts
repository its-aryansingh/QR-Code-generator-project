import QRCode from 'qrcode';
import { DesignV1 } from './design';

export interface QRMatrix {
  size: number;
  data: boolean[][];
  version: number;
  ecc: 'L' | 'M' | 'Q' | 'H';
}

export function isFinderModule(row: number, col: number, size: number): boolean {
  // Top-left 7x7
  if (row < 7 && col < 7) return true;
  // Top-right 7x7
  if (row < 7 && col >= size - 7) return true;
  // Bottom-left 7x7
  if (row >= size - 7 && col < 7) return true;
  return false;
}

export interface LogoBox {
  startRow: number;
  endRow: number;
  startCol: number;
  endCol: number;
}

export function calculateLogoBox(size: number, design: DesignV1): LogoBox | null {
  if (!design.logo || !design.logo.clear_modules) return null;

  const ratio = Math.max(0.10, Math.min(0.30, design.logo.size_ratio));
  const logoModules = Math.floor(size * ratio);
  const padding = design.logo.padding || 1;

  const totalBox = logoModules + padding * 2;
  const start = Math.floor((size - totalBox) / 2);
  const end = start + totalBox;

  return {
    startRow: start,
    endRow: end,
    startCol: start,
    endCol: end,
  };
}

export function isLogoModule(row: number, col: number, box: LogoBox | null): boolean {
  if (!box) return false;
  return row >= box.startRow && row < box.endRow && col >= box.startCol && col < box.endCol;
}

export function generateMatrix(payload: string, eccLevel: 'auto' | 'L' | 'M' | 'Q' | 'H'): QRMatrix {
  let ecc: 'L' | 'M' | 'Q' | 'H' = 'M';
  if (eccLevel !== 'auto') {
    ecc = eccLevel;
  }

  const qr = QRCode.create(payload, {
    errorCorrectionLevel: ecc,
  });

  const size = qr.modules.size;
  const data: boolean[][] = [];

  for (let r = 0; r < size; r++) {
    const row: boolean[] = [];
    for (let c = 0; c < size; c++) {
      row.push(Boolean(qr.modules.get(r, c)));
    }
    data.push(row);
  }

  return {
    size,
    data,
    version: qr.version,
    ecc,
  };
}
