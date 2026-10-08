"use client";

/**
 * Download, copy and share for one QR code. Everything is drawn in the
 * browser with the code's own style, so every format matches the preview.
 */
import { useState } from "react";
import { Check, Copy, Download, ImageIcon, Loader2, Mail, Share2 } from "lucide-react";

import type { QrStyle } from "@/components/create/qr-proof";
import {
  canCopyImage, copyQrImage, exportQr, nativeShare, shareTargets, type ExportFormat,
} from "@/lib/qr-export";

const FORMATS: { id: ExportFormat; label: string; note: string }[] = [
  { id: "png", label: "PNG", note: "Web, slides, documents" },
  { id: "svg", label: "SVG", note: "Sharp at any print size" },
  { id: "pdf", label: "PDF", note: "A4 sheet, ready to print" },
  { id: "jpeg", label: "JPG", note: "Smallest file" },
];

const SIZES = [
  { px: 512, label: "S" },
  { px: 1024, label: "M" },
  { px: 2048, label: "L" },
];

interface Props {
  /** What the printed code contains (the short link for tracked codes). */
  data: string;
  style: QrStyle;
  name: string;
  /** The short link, when the code has one. */
  link?: string | null;
}

export function QrShareActions({ data, style, name, link }: Props) {
  const [size, setSize] = useState(1024);
  const [busy, setBusy] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const flash = (id: string) => {
    setDone(id);
    window.setTimeout(() => setDone((d) => (d === id ? null : d)), 1600);
  };

  const run = async (id: string, task: () => Promise<void>) => {
    setBusy(id);
    setMessage(null);
    try {
      await task();
      flash(id);
    } catch {
      setMessage(
        id === "copy-image"
          ? "This browser can't copy images. Download the PNG instead."
          : "Something went wrong preparing the file. Try again.",
      );
    } finally {
      setBusy(null);
    }
  };

  const share = () =>
    run("share", async () => {
      const result = await nativeShare({ data, style, name, link });
      if (result === "unsupported") {
        setMessage(link ? "Use one of the apps below to share the link." : "Download the PNG and share it from your device.");
      }
    });

  const btn =
    "inline-flex items-center justify-center gap-1.5 rounded-xl border border-zinc-700 text-[14px] font-medium text-zinc-100 transition-colors hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 disabled:opacity-50";

  return (
    <div className="space-y-5">
      {/* Download */}
      <section aria-labelledby="qr-download">
        <div className="mb-2 flex items-center justify-between">
          <h3 id="qr-download" className="text-[13px] font-medium text-zinc-300">Download</h3>
          <div role="radiogroup" aria-label="Image size" className="inline-flex rounded-lg border border-zinc-800 p-0.5">
            {SIZES.map((s) => (
              <button
                key={s.px}
                type="button"
                role="radio"
                aria-checked={size === s.px}
                title={`${s.px} × ${s.px} pixels`}
                onClick={() => setSize(s.px)}
                className={`h-6 min-w-7 rounded-md px-1.5 text-[12px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 ${
                  size === s.px ? "bg-zinc-800 text-zinc-100" : "text-zinc-500 hover:text-zinc-300"
                }`}
              >
                {s.label}
              </button>
            ))}
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2">
          {FORMATS.map((f) => (
            <button
              key={f.id}
              type="button"
              disabled={busy !== null}
              onClick={() => run(f.id, () => exportQr(f.id, { data, style, size, name, link }))}
              className={`${f.id === "png" ? "border-white bg-white text-zinc-900 hover:bg-zinc-200" : ""} ${btn} h-auto flex-col items-start gap-0 px-3 py-2 text-left`}
            >
              <span className="inline-flex items-center gap-1.5 text-[14px] font-semibold">
                {busy === f.id ? <Loader2 size={14} className="animate-spin" /> : done === f.id ? <Check size={14} /> : <Download size={14} />}
                {f.label}
              </span>
              <span className={`text-[12px] font-normal ${f.id === "png" ? "text-zinc-600" : "text-zinc-500"}`}>{f.note}</span>
            </button>
          ))}
        </div>
      </section>

      {/* Copy */}
      <section aria-labelledby="qr-copy">
        <h3 id="qr-copy" className="mb-2 text-[13px] font-medium text-zinc-300">Copy</h3>
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            disabled={busy !== null || !canCopyImage()}
            title={canCopyImage() ? "Paste it straight into chats, docs or slides" : "Not supported in this browser"}
            onClick={() => run("copy-image", () => copyQrImage(data, style))}
            className={`${btn} h-10`}
          >
            {done === "copy-image" ? <Check size={15} /> : <ImageIcon size={15} />}
            {done === "copy-image" ? "Image copied" : "Copy image"}
          </button>
          <button
            type="button"
            disabled={busy !== null}
            onClick={() => run("copy-link", () => navigator.clipboard.writeText(link || data))}
            className={`${btn} h-10`}
          >
            {done === "copy-link" ? <Check size={15} /> : <Copy size={15} />}
            {done === "copy-link" ? "Copied" : link ? "Copy link" : "Copy content"}
          </button>
        </div>
      </section>

      {/* Share */}
      <section aria-labelledby="qr-share">
        <h3 id="qr-share" className="mb-2 text-[13px] font-medium text-zinc-300">Share</h3>
        <button type="button" disabled={busy !== null} onClick={share} className={`${btn} h-10 w-full`}>
          {busy === "share" ? <Loader2 size={15} className="animate-spin" /> : <Share2 size={15} />}
          Share from this device
        </button>
        {link && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {shareTargets(link, name).map((t) => (
              <a
                key={t.id}
                href={t.href}
                target={t.id === "email" ? undefined : "_blank"}
                rel="noreferrer"
                className="inline-flex h-8 items-center gap-1.5 rounded-full border border-zinc-800 px-3 text-[13px] text-zinc-300 transition-colors hover:border-zinc-600 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400"
              >
                {t.id === "email" && <Mail size={13} />}
                {t.label}
              </a>
            ))}
          </div>
        )}
      </section>

      {message && (
        <p role="status" className="text-[13px] text-zinc-400">
          {message}
        </p>
      )}
    </div>
  );
}
