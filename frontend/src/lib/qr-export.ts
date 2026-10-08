/**
 * Turning a QR code into files, clipboard images and shares, all in the
 * browser with the code's own style.
 */
import type { QrStyle } from "@/components/create/qr-proof";
import { qrOptions } from "@/components/create/qr-proof";

export type RasterFormat = "png" | "jpeg" | "webp";
export type ExportFormat = RasterFormat | "svg" | "pdf";

function safeName(name: string): string {
  return name.replace(/[^\w-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 60) || "qr-code";
}

/** JPEG and PDF have no transparency; give them white paper. */
function opaque(style: QrStyle): QrStyle {
  return style.background === "transparent" ? { ...style, background: "#FFFFFF" } : style;
}

export async function qrBlob(data: string, style: QrStyle, size: number, format: RasterFormat | "svg"): Promise<Blob> {
  const Styling = (await import("qr-code-styling")).default;
  const options = qrOptions(data, format === "jpeg" ? opaque(style) : style, size);
  // Raster formats render through a canvas so they come out at full size.
  if (format !== "svg") options.type = "canvas";
  const qr = new Styling(options);
  const raw = await qr.getRawData(format);
  if (!(raw instanceof Blob)) throw new Error("Could not draw the QR code");
  return format === "svg" ? new Blob([await raw.text()], { type: "image/svg+xml" }) : raw;
}

function save(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 2000);
}

function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error ?? new Error("read failed"));
    reader.readAsDataURL(blob);
  });
}

/** An A4 print sheet: the code 9 cm wide, its name and link underneath. */
async function pdfBlob(data: string, style: QrStyle, title: string, link?: string | null): Promise<Blob> {
  const [{ jsPDF }, png] = await Promise.all([import("jspdf"), qrBlob(data, opaque(style), 1200, "png")]);
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  const size = 90;
  const x = (210 - size) / 2;
  const y = 48;
  doc.setProperties({ title, creator: "QRit" });
  // FAST = deflate the image; without it the page is several megabytes.
  doc.addImage(await blobToDataUrl(png), "PNG", x, y, size, size, undefined, "FAST");
  doc.setFont("helvetica", "bold");
  doc.setFontSize(16);
  doc.setTextColor(17, 24, 39);
  doc.text(title, 105, y + size + 14, { align: "center", maxWidth: 170 });
  if (link) {
    doc.setFont("helvetica", "normal");
    doc.setFontSize(11);
    doc.setTextColor(82, 82, 91);
    doc.text(link.replace(/^https?:\/\//, ""), 105, y + size + 22, { align: "center", maxWidth: 170 });
  }
  return doc.output("blob");
}

export async function exportQr(
  format: ExportFormat,
  opts: { data: string; style: QrStyle; size: number; name: string; link?: string | null },
) {
  const base = safeName(opts.name);
  if (format === "pdf") {
    save(await pdfBlob(opts.data, opts.style, opts.name, opts.link), `${base}.pdf`);
    return;
  }
  const blob = await qrBlob(opts.data, opts.style, opts.size, format);
  save(blob, `${base}.${format === "jpeg" ? "jpg" : format}`);
}

export function canCopyImage(): boolean {
  return typeof window !== "undefined" && "ClipboardItem" in window && !!navigator.clipboard?.write;
}

export async function copyQrImage(data: string, style: QrStyle) {
  // Safari wants the ClipboardItem built synchronously from a promise.
  const item = new ClipboardItem({ "image/png": qrBlob(data, style, 1024, "png") });
  await navigator.clipboard.write([item]);
}

export async function nativeShare(opts: { data: string; style: QrStyle; name: string; link?: string | null }): Promise<"shared" | "cancelled" | "unsupported"> {
  if (typeof navigator === "undefined" || !navigator.share) return "unsupported";
  try {
    const png = await qrBlob(opts.data, opaque(opts.style), 1024, "png");
    const file = new File([png], `${safeName(opts.name)}.png`, { type: "image/png" });
    const payload: ShareData = { title: opts.name, text: opts.link ? `${opts.name}: ${opts.link}` : opts.name };
    if (navigator.canShare?.({ files: [file] })) payload.files = [file];
    else if (opts.link) payload.url = opts.link;
    await navigator.share(payload);
    return "shared";
  } catch (error) {
    return error instanceof DOMException && error.name === "AbortError" ? "cancelled" : "unsupported";
  }
}

/** Share targets that take a link. */
export function shareTargets(link: string, title: string) {
  const u = encodeURIComponent(link);
  const t = encodeURIComponent(title);
  return [
    { id: "whatsapp", label: "WhatsApp", href: `https://wa.me/?text=${encodeURIComponent(`${title} ${link}`)}` },
    { id: "telegram", label: "Telegram", href: `https://t.me/share/url?url=${u}&text=${t}` },
    { id: "x", label: "X", href: `https://x.com/intent/post?text=${t}&url=${u}` },
    { id: "linkedin", label: "LinkedIn", href: `https://www.linkedin.com/sharing/share-offsite/?url=${u}` },
    { id: "facebook", label: "Facebook", href: `https://www.facebook.com/sharer/sharer.php?u=${u}` },
    { id: "email", label: "Email", href: `mailto:?subject=${t}&body=${encodeURIComponent(`${title}\n${link}`)}` },
  ] as const;
}
