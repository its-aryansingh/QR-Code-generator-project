"use client";

/**
 * The live QR "proof": the exact code that will be printed, rendered in the
 * browser as you type, on a white card with printer's crop marks.
 */
import { useEffect, useRef } from "react";
import type QRCodeStyling from "qr-code-styling";
import type { Options } from "qr-code-styling";

export type DotStyle = "square" | "rounded" | "dots" | "classy-rounded";
export type CornerStyle = "square" | "extra-rounded" | "dot";

export interface QrStyle {
  color: string;
  background: string; // hex or "transparent"
  dots: DotStyle;
  corners: CornerStyle;
  logo: string | null;
}

export const DEFAULT_STYLE: QrStyle = {
  color: "#111827",
  background: "#FFFFFF",
  dots: "square",
  corners: "square",
  logo: null,
};

export function qrOptions(data: string, style: QrStyle, size: number): Partial<Options> {
  return {
    width: size,
    height: size,
    type: "svg",
    data,
    margin: Math.round(size * 0.04),
    // A logo hides modules; the highest error correction keeps it scannable.
    qrOptions: { errorCorrectionLevel: style.logo ? "H" : "M" },
    dotsOptions: { type: style.dots, color: style.color },
    cornersSquareOptions: { type: style.corners, color: style.color },
    cornersDotOptions: { type: style.corners === "dot" ? "dot" : "square", color: style.color },
    backgroundOptions: { color: style.background === "transparent" ? "rgba(0,0,0,0)" : style.background },
    image: style.logo ?? undefined,
    imageOptions: { margin: Math.round(size * 0.01), imageSize: 0.32, hideBackgroundDots: true, crossOrigin: "anonymous" },
  };
}

async function load(): Promise<typeof QRCodeStyling> {
  return (await import("qr-code-styling")).default;
}

/** Save the code at print size. */
export async function downloadQr(data: string, style: QrStyle, name: string, extension: "png" | "svg") {
  const Styling = await load();
  const qr = new Styling(qrOptions(data, style, 1024));
  await qr.download({ name: name.replace(/[^\w-]+/g, "-").replace(/^-|-$/g, "") || "qr-code", extension });
}

interface ProofProps {
  data: string;
  style: QrStyle;
  /** Faded sample shown before there's anything to encode. */
  placeholder?: boolean;
  label: string;
  children?: React.ReactNode;
}

export function QrProof({ data, style, placeholder = false, label, children }: ProofProps) {
  const holder = useRef<HTMLDivElement>(null);
  const qr = useRef<QRCodeStyling | null>(null);

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      const Styling = await load();
      if (cancelled || !holder.current) return;
      const options = qrOptions(data, style, 320);
      if (!qr.current) {
        qr.current = new Styling(options);
        holder.current.innerHTML = "";
        qr.current.append(holder.current);
      } else {
        qr.current.update(options);
      }
    }, 90);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [data, style]);

  const transparent = style.background === "transparent";

  return (
    <figure className="flex flex-col items-center">
      <div className="relative p-5">
        {/* Crop marks: this is a proof of what gets printed. */}
        {(["left-0 top-0", "right-0 top-0 rotate-90", "right-0 bottom-0 rotate-180", "left-0 bottom-0 -rotate-90"] as const).map((pos) => (
          <span key={pos} aria-hidden className={`pointer-events-none absolute h-3.5 w-3.5 border-l border-t border-zinc-500 ${pos}`} />
        ))}
        <div
          className={`relative w-[240px] sm:w-[280px] aspect-square rounded-[6px] shadow-[0_1px_0_rgba(255,255,255,0.06),0_18px_40px_-18px_rgba(0,0,0,0.8)] transition-opacity duration-200 ${
            transparent ? "qr-checker" : "bg-white"
          } ${placeholder ? "opacity-25" : "opacity-100"}`}
        >
          <div ref={holder} role="img" aria-label={label} className="absolute inset-0 [&_svg]:h-full [&_svg]:w-full [&_canvas]:h-full [&_canvas]:w-full" />
        </div>
      </div>
      {children && <figcaption className="mt-1 w-full">{children}</figcaption>}
    </figure>
  );
}

/** A small live copy of the code, for the phone's sticky create bar. */
export function QrThumb({ data, style, faded }: { data: string; style: QrStyle; faded?: boolean }) {
  const holder = useRef<HTMLDivElement>(null);
  const qr = useRef<QRCodeStyling | null>(null);

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      const Styling = await load();
      if (cancelled || !holder.current) return;
      const options = qrOptions(data, style, 96);
      if (!qr.current) {
        qr.current = new Styling(options);
        holder.current.innerHTML = "";
        qr.current.append(holder.current);
      } else {
        qr.current.update(options);
      }
    }, 120);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [data, style]);

  return (
    <div
      aria-hidden
      className={`h-11 w-11 shrink-0 overflow-hidden rounded-md ${style.background === "transparent" ? "qr-checker" : "bg-white"} ${faded ? "opacity-30" : ""}`}
    >
      <div ref={holder} className="h-full w-full [&_svg]:h-full [&_svg]:w-full" />
    </div>
  );
}
