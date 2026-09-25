import { FrameConfig } from './design';

export interface FrameLayout {
  totalWidth: number;
  totalHeight: number;
  qrX: number;
  qrY: number;
  frameSvg: string;
}

function escapeXml(unsafe: string): string {
  return unsafe.replace(/[<>&'"]/g, (c) => {
    switch (c) {
      case '<': return '&lt;';
      case '>': return '&gt;';
      case '&': return '&amp;';
      case '\'': return '&apos;';
      case '"': return '&quot;';
      default: return c;
    }
  });
}

export function computeFrameLayout(
  frame: FrameConfig | null | undefined,
  qrSize: number,
  qrBgColor: string = '#ffffff'
): FrameLayout {
  if (!frame || !frame.text) {
    return {
      totalWidth: qrSize,
      totalHeight: qrSize,
      qrX: 0,
      qrY: 0,
      frameSvg: '',
    };
  }

  const margin = Math.max(16, Math.round(qrSize * 0.08));
  const bannerHeight = Math.max(36, Math.round(qrSize * 0.16));
  const fontSize = Math.max(12, Math.round(bannerHeight * 0.42));
  const escapedText = escapeXml(frame.text);
  const fontFam = 'Geist, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';

  switch (frame.style) {
    case 'banner-top': {
      const totalWidth = qrSize + 2 * margin;
      const totalHeight = qrSize + 2 * margin + bannerHeight;
      const qrX = margin;
      const qrY = margin + bannerHeight;

      const frameSvg = [
        `<rect x="0" y="0" width="${totalWidth}" height="${totalHeight}" rx="16" fill="${frame.color}" />`,
        `<rect x="${qrX - 4}" y="${qrY - 4}" width="${qrSize + 8}" height="${qrSize + 8}" rx="8" fill="${qrBgColor}" />`,
        `<text x="${totalWidth / 2}" y="${margin + bannerHeight / 2 + fontSize * 0.35}" text-anchor="middle" font-family="${fontFam}" font-weight="600" font-size="${fontSize}" fill="${frame.text_color}">${escapedText}</text>`,
      ].join('');

      return { totalWidth, totalHeight, qrX, qrY, frameSvg };
    }

    case 'rounded-box': {
      const totalWidth = qrSize + 2 * margin;
      const totalHeight = qrSize + 2 * margin + bannerHeight;
      const qrX = margin;
      const qrY = margin;
      const strokeW = Math.max(4, Math.round(margin * 0.3));

      const pillHeight = Math.max(28, fontSize + 12);
      const pillWidth = Math.min(totalWidth - 20, Math.max(100, frame.text.length * fontSize * 0.7 + 32));
      const pillX = (totalWidth - pillWidth) / 2;
      const pillY = margin + qrSize + (bannerHeight - pillHeight) / 2;

      const frameSvg = [
        `<rect x="${strokeW / 2}" y="${strokeW / 2}" width="${totalWidth - strokeW}" height="${totalHeight - strokeW}" rx="16" fill="${qrBgColor}" stroke="${frame.color}" stroke-width="${strokeW}" />`,
        `<rect x="${pillX}" y="${pillY}" width="${pillWidth}" height="${pillHeight}" rx="${pillHeight / 2}" fill="${frame.color}" />`,
        `<text x="${totalWidth / 2}" y="${pillY + pillHeight / 2 + fontSize * 0.35}" text-anchor="middle" font-family="${fontFam}" font-weight="600" font-size="${fontSize}" fill="${frame.text_color}">${escapedText}</text>`,
      ].join('');

      return { totalWidth, totalHeight, qrX, qrY, frameSvg };
    }

    case 'speech': {
      const totalWidth = qrSize + 2 * margin;
      const pointerH = 12;
      const totalHeight = qrSize + 2 * margin + bannerHeight + pointerH;
      const qrX = margin;
      const qrY = margin;

      const cx = totalWidth / 2;
      const py = totalHeight - pointerH;

      const frameSvg = [
        `<rect x="0" y="0" width="${totalWidth}" height="${totalHeight - pointerH}" rx="16" fill="${frame.color}" />`,
        `<polygon points="${cx - 10},${py} ${cx},${totalHeight} ${cx + 10},${py}" fill="${frame.color}" />`,
        `<rect x="${qrX - 4}" y="${qrY - 4}" width="${qrSize + 8}" height="${qrSize + 8}" rx="8" fill="${qrBgColor}" />`,
        `<text x="${cx}" y="${margin + qrSize + bannerHeight / 2 + fontSize * 0.35}" text-anchor="middle" font-family="${fontFam}" font-weight="600" font-size="${fontSize}" fill="${frame.text_color}">${escapedText}</text>`,
      ].join('');

      return { totalWidth, totalHeight, qrX, qrY, frameSvg };
    }

    case 'banner-bottom':
    default: {
      const totalWidth = qrSize + 2 * margin;
      const totalHeight = qrSize + 2 * margin + bannerHeight;
      const qrX = margin;
      const qrY = margin;

      const frameSvg = [
        `<rect x="0" y="0" width="${totalWidth}" height="${totalHeight}" rx="16" fill="${frame.color}" />`,
        `<rect x="${qrX - 4}" y="${qrY - 4}" width="${qrSize + 8}" height="${qrSize + 8}" rx="8" fill="${qrBgColor}" />`,
        `<text x="${totalWidth / 2}" y="${margin + qrSize + bannerHeight / 2 + fontSize * 0.35}" text-anchor="middle" font-family="${fontFam}" font-weight="600" font-size="${fontSize}" fill="${frame.text_color}">${escapedText}</text>`,
      ].join('');

      return { totalWidth, totalHeight, qrX, qrY, frameSvg };
    }
  }
}
