import { renderSvg, DesignV1, ScanWarning } from '@qrit/qr-render';
import { Resvg } from '@resvg/resvg-js';
import PDFDocument from 'pdfkit';
// @ts-ignore
import SVGtoPDF from 'svg-to-pdfkit';

export interface RenderRequest {
  payload: string;
  design?: Partial<DesignV1>;
  format: 'svg' | 'png' | 'pdf';
  sizePx?: number;
}

export interface RenderResult {
  contentType: string;
  data: string | Buffer;
  warnings: ScanWarning[];
  version: number;
  ecc: string;
}

export async function processRender(req: RenderRequest): Promise<RenderResult> {
  const sizePx = req.sizePx || 512;
  const rendered = renderSvg({
    payload: req.payload,
    design: req.design,
    sizePx: req.format === 'svg' ? req.sizePx : undefined,
  });

  if (req.format === 'svg') {
    return {
      contentType: 'image/svg+xml',
      data: rendered.svg,
      warnings: rendered.warnings,
      version: rendered.version,
      ecc: rendered.ecc,
    };
  }

  if (req.format === 'png') {
    const resvg = new Resvg(rendered.svg, {
      fitTo: { mode: 'width', value: sizePx },
    });
    const pngData = resvg.render();
    const pngBuffer = pngData.asPng();

    return {
      contentType: 'image/png',
      data: pngBuffer,
      warnings: rendered.warnings,
      version: rendered.version,
      ecc: rendered.ecc,
    };
  }

  if (req.format === 'pdf') {
    const pdfBuffer = await new Promise<Buffer>((resolve, reject) => {
      const doc = new PDFDocument({
        autoFirstPage: false,
      });

      const chunks: Buffer[] = [];
      doc.on('data', (chunk) => chunks.push(chunk));
      doc.on('end', () => resolve(Buffer.concat(chunks)));
      doc.on('error', (err) => reject(err));

      doc.addPage({
        size: [sizePx, sizePx],
        margin: 0,
      });

      const svgFn = typeof SVGtoPDF === 'function' ? SVGtoPDF : (SVGtoPDF as any).default;
      if (typeof svgFn === 'function') {
        svgFn(doc, rendered.svg, 0, 0, { width: sizePx, height: sizePx });
      }
      doc.end();
    });

    return {
      contentType: 'application/pdf',
      data: pdfBuffer,
      warnings: rendered.warnings,
      version: rendered.version,
      ecc: rendered.ecc,
    };
  }

  throw new Error(`Unsupported format: ${req.format}`);
}
