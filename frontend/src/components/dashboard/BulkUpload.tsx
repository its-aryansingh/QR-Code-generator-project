"use client";

import { useState, useRef, useCallback } from "react";
import JSZip from "jszip";
import { useAuthStore } from "@/lib/auth";
import { toast } from "sonner";

interface PreviewRow {
  row: number;
  title: string;
  content: string;
  type: string;
}

interface BulkUploadProps {
  workspaceId: string;
  onComplete?: (successCount: number) => void;
}

type Stage = "idle" | "preview" | "generating" | "done";

function parseCSV(text: string): PreviewRow[] {
  const lines = text.split(/\r?\n/).filter((l) => l.trim());
  if (lines.length < 2) return [];

  const header = lines[0].split(",").map((h) => h.replace(/^"|"$/g, "").trim().toLowerCase());
  const contentIdx = header.indexOf("content") !== -1 ? header.indexOf("content") : header.indexOf("url");
  const titleIdx = header.indexOf("title");
  const typeIdx = header.indexOf("type");

  if (contentIdx === -1) return [];

  return lines.slice(1).map((line, i) => {
    // simple CSV parse — handles quoted fields
    const cols: string[] = [];
    let cur = "";
    let inQuote = false;
    for (let ci = 0; ci < line.length; ci++) {
      const ch = line[ci];
      if (ch === '"') { inQuote = !inQuote; continue; }
      if (ch === "," && !inQuote) { cols.push(cur); cur = ""; continue; }
      cur += ch;
    }
    cols.push(cur);

    const content = cols[contentIdx]?.trim() ?? "";
    const title = titleIdx !== -1 ? cols[titleIdx]?.trim() : "";
    const type = typeIdx !== -1 ? cols[typeIdx]?.trim() : "url";

    return {
      row: i + 1,
      title: title || content.substring(0, 40),
      content,
      type: type || "url",
    };
  }).filter((r) => r.content);
}

async function buildZip(items: { filename: string; b64: string }[]): Promise<Blob> {
  const zip = new JSZip();
  for (const item of items) {
    const binary = atob(item.b64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    zip.file(item.filename, bytes);
  }
  return zip.generateAsync({ type: "blob" });
}

export function BulkUpload({ workspaceId, onComplete }: BulkUploadProps) {
  const { accessToken } = useAuthStore();
  const [stage, setStage] = useState<Stage>("idle");
  const [file, setFile] = useState<File | null>(null);
  const [rows, setRows] = useState<PreviewRow[]>([]);
  const [progress, setProgress] = useState(0);
  const [successCount, setSuccessCount] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8084/api/v1";

  const handleFile = useCallback((f: File | null) => {
    if (!f) return;
    if (!f.name.endsWith(".csv")) { toast.error("Please upload a .csv file"); return; }
    if (f.size > 5 * 1024 * 1024) { toast.error("File too large (max 5 MB)"); return; }

    const reader = new FileReader();
    reader.onload = (e) => {
      const text = e.target?.result as string;
      const parsed = parseCSV(text);
      if (parsed.length === 0) {
        toast.error("No valid rows found. Make sure the file has a 'content' or 'url' column.");
        return;
      }
      setFile(f);
      setRows(parsed.slice(0, 100));
      setStage("preview");
    };
    reader.readAsText(f);
  }, []);

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    handleFile(e.dataTransfer.files[0] ?? null);
  };

  const generate = async () => {
    if (rows.length === 0) return;
    setStage("generating");
    setProgress(0);

    const ticker = setInterval(() => setProgress((p) => Math.min(p + 4, 88)), 350);

    try {
      const items = rows.map((r) => ({ content: r.content, title: r.title, type: r.type, size: 512 }));

      const res = await fetch(`${api}/bulk/generate`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({ items }),
      });

      clearInterval(ticker);

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        toast.error(body.error || "Generation failed");
        setStage("preview");
        return;
      }

      const data = await res.json();
      const results: { content: string; qr_base64: string; success: boolean }[] = data.results ?? [];
      const succeeded = results.filter((r) => r.success);

      setProgress(100);
      setSuccessCount(succeeded.length);
      setStage("done");

      // Build ZIP from base64 strings
      const zipItems = succeeded.map((r, i) => ({
        filename: `qr-${i + 1}-${rows[i]?.title?.replace(/[^a-z0-9]/gi, "_").substring(0, 30) || i + 1}.png`,
        b64: r.qr_base64,
      }));

      try {
        const zipBlob = await buildZip(zipItems);
        const url = URL.createObjectURL(zipBlob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "qrit-bulk-qrcodes.zip";
        a.click();
        URL.revokeObjectURL(url);
      } catch {
        // jszip not available — download PNGs individually
        for (const item of zipItems) {
          const binary = atob(item.b64);
          const bytes = new Uint8Array(binary.length);
          for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
          const blob = new Blob([bytes], { type: "image/png" });
          const url = URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = url;
          a.download = item.filename;
          a.click();
          URL.revokeObjectURL(url);
        }
      }

      toast.success(`Generated ${succeeded.length} / ${rows.length} QR codes`);
      onComplete?.(succeeded.length);
    } catch {
      clearInterval(ticker);
      toast.error("Network error — please retry");
      setStage("preview");
    }
  };

  const reset = () => {
    setStage("idle");
    setFile(null);
    setRows([]);
    setProgress(0);
    if (fileRef.current) fileRef.current.value = "";
  };

  const preview = rows.slice(0, 10);

  return (
    <div className="space-y-5">
      {/* Drop zone */}
      {stage === "idle" && (
        <div
          onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
          onDragLeave={() => setIsDragging(false)}
          onDrop={onDrop}
          onClick={() => fileRef.current?.click()}
          className={`relative flex flex-col items-center justify-center gap-4 p-12 rounded-2xl border-2 border-dashed cursor-pointer transition-all duration-200 ${isDragging ? "border-violet-500 bg-violet-500/5" : "border-zinc-700 hover:border-zinc-600 bg-zinc-900/50"}`}
        >
          <input
            ref={fileRef}
            type="file"
            accept=".csv"
            className="hidden"
            onChange={(e) => handleFile(e.target.files?.[0] ?? null)}
          />
          <div className="w-14 h-14 rounded-2xl bg-zinc-800 flex items-center justify-center">
            <svg className="w-7 h-7 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </div>
          <div className="text-center">
            <p className="font-semibold text-zinc-200">Drop your CSV here</p>
            <p className="text-sm text-zinc-500 mt-1">or <span className="text-violet-400 underline underline-offset-2">click to browse</span> — max 5 MB, 100 rows</p>
          </div>

          <div className="mt-2 px-4 py-2.5 bg-zinc-900 border border-zinc-800 rounded-xl text-xs font-mono text-zinc-500 text-left w-full max-w-sm">
            <span className="text-zinc-400">title</span>,<span className="text-zinc-400">content</span>,<span className="text-zinc-400">type</span><br />
            "Product A","https://example.com","url"<br />
            "Trade WiFi","WIFI:T:WPA;S:Net;P:pass;;","wifi"<br />
            "Contact","BEGIN:VCARD…","vcard"
          </div>
        </div>
      )}

      {/* Preview */}
      {stage === "preview" && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-semibold text-zinc-200">
                {file?.name}
                <span className="ml-2 text-sm font-normal text-zinc-500">— {rows.length} row{rows.length !== 1 ? "s" : ""} ready</span>
              </p>
              {rows.length === 100 && (
                <p className="text-xs text-amber-400 mt-1">Capped at 100 rows per request.</p>
              )}
            </div>
            <button onClick={reset} className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors">
              Change file
            </button>
          </div>

          <div className="bg-zinc-900 rounded-xl border border-zinc-800 overflow-hidden">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-zinc-800 bg-zinc-950/50">
                  <th className="px-4 py-2.5 text-left font-semibold text-zinc-500">#</th>
                  <th className="px-4 py-2.5 text-left font-semibold text-zinc-500">Title</th>
                  <th className="px-4 py-2.5 text-left font-semibold text-zinc-500 hidden sm:table-cell">Content</th>
                  <th className="px-4 py-2.5 text-left font-semibold text-zinc-500">Type</th>
                </tr>
              </thead>
              <tbody>
                {preview.map((r) => (
                  <tr key={r.row} className="border-b border-zinc-800/60">
                    <td className="px-4 py-2 text-zinc-600">{r.row}</td>
                    <td className="px-4 py-2 text-zinc-300 max-w-[120px] truncate">{r.title}</td>
                    <td className="px-4 py-2 text-zinc-500 max-w-[180px] truncate hidden sm:table-cell">{r.content}</td>
                    <td className="px-4 py-2">
                      <span className="px-1.5 py-0.5 bg-zinc-800 text-zinc-400 rounded font-mono">{r.type}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {rows.length > 10 && (
              <p className="px-4 py-2.5 text-xs text-zinc-600 border-t border-zinc-800">
                Showing 10 of {rows.length} rows that will be generated.
              </p>
            )}
          </div>

          <div className="flex gap-3">
            <button
              onClick={generate}
              className="flex items-center gap-2 px-6 py-2.5 bg-gradient-to-r from-violet-600 to-indigo-600 text-white font-semibold rounded-xl hover:from-violet-500 hover:to-indigo-500 transition-all shadow-lg shadow-violet-500/20 text-sm"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              Generate {rows.length} QR Codes &amp; Download ZIP
            </button>
            <button onClick={reset} className="px-5 py-2.5 border border-zinc-700 text-zinc-400 font-semibold rounded-xl hover:bg-zinc-800 hover:text-zinc-200 transition-all text-sm">
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Generating */}
      {stage === "generating" && (
        <div className="flex flex-col items-center gap-6 py-12">
          <div className="relative w-16 h-16">
            <div className="absolute inset-0 rounded-full border-4 border-zinc-800" />
            <div className="absolute inset-0 rounded-full border-4 border-violet-500 border-t-transparent animate-spin" />
          </div>
          <div className="text-center">
            <p className="font-semibold text-zinc-200 mb-1">Generating QR codes…</p>
            <p className="text-sm text-zinc-500">Do not close this page</p>
          </div>
          <div className="w-full max-w-sm">
            <div className="flex justify-between text-xs text-zinc-500 mb-1.5">
              <span>Progress</span>
              <span>{progress}%</span>
            </div>
            <div className="h-2 bg-zinc-800 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-violet-600 to-indigo-600 rounded-full transition-all duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      )}

      {/* Done */}
      {stage === "done" && (
        <div className="flex flex-col items-center gap-5 py-12 text-center">
          <div className="w-16 h-16 rounded-full bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center">
            <svg className="w-8 h-8 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <div>
            <p className="text-xl font-bold text-zinc-100 mb-1">{successCount} QR codes generated</p>
            <p className="text-sm text-zinc-500">Your download should have started automatically.</p>
          </div>
          <button
            onClick={reset}
            className="px-6 py-2.5 bg-zinc-800 border border-zinc-700 text-zinc-300 font-semibold rounded-xl hover:bg-zinc-700 transition-colors text-sm"
          >
            Upload Another CSV
          </button>
        </div>
      )}
    </div>
  );
}
