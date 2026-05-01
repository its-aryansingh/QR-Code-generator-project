"use client";

import { useState, useRef, useCallback } from "react";
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

export function BulkUpload({ workspaceId, onComplete }: BulkUploadProps) {
  const { accessToken } = useAuthStore();
  const [stage, setStage] = useState<Stage>("idle");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<PreviewRow[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [progress, setProgress] = useState(0);
  const [successCount, setSuccessCount] = useState(0);
  const [isDragging, setIsDragging] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const loadPreview = useCallback(async (f: File) => {
    setFile(f);
    const form = new FormData();
    form.append("file", f);
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/bulk/preview`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}` },
        body: form,
      }).then((r) => r.json());

      if (res.success) {
        setPreview(res.preview ?? []);
        setTotalRows(res.total_rows ?? 0);
        setStage("preview");
      } else {
        toast.error(res.error || "Invalid CSV");
      }
    } catch {
      toast.error("Failed to parse CSV");
    }
  }, [workspaceId, accessToken, api]);

  const handleFile = (f: File | null) => {
    if (!f) return;
    if (!f.name.endsWith(".csv")) { toast.error("Please upload a .csv file"); return; }
    if (f.size > 5 * 1024 * 1024) { toast.error("File too large (max 5 MB)"); return; }
    loadPreview(f);
  };

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    handleFile(e.dataTransfer.files[0] ?? null);
  };

  const generate = async () => {
    if (!file) return;
    setStage("generating");
    setProgress(0);

    // Fake progress ticker while the request is in-flight
    const ticker = setInterval(() => setProgress((p) => Math.min(p + 3, 92)), 400);

    try {
      const form = new FormData();
      form.append("file", file);

      const res = await fetch(`${api}/workspaces/${workspaceId}/bulk`, {
        method: "POST",
        headers: { Authorization: `Bearer ${accessToken}` },
        body: form,
      });

      clearInterval(ticker);

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        toast.error(body.error || "Generation failed");
        setStage("preview");
        return;
      }

      // Parse response headers for counts
      const success = parseInt(res.headers.get("X-Bulk-Success") ?? "0", 10);
      const total = parseInt(res.headers.get("X-Bulk-Total") ?? String(totalRows), 10);

      setProgress(100);
      setSuccessCount(success);
      setStage("done");

      // Trigger ZIP download
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "qrit-bulk-qrcodes.zip";
      a.click();
      URL.revokeObjectURL(url);

      toast.success(`Generated ${success} / ${total} QR codes`);
      onComplete?.(success);
    } catch {
      clearInterval(ticker);
      toast.error("Network error — please retry");
      setStage("preview");
    }
  };

  const reset = () => {
    setStage("idle");
    setFile(null);
    setPreview([]);
    setTotalRows(0);
    setProgress(0);
    if (fileRef.current) fileRef.current.value = "";
  };

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
            <p className="text-sm text-zinc-500 mt-1">or click to browse — max 5 MB, 100 rows</p>
          </div>

          {/* Format hint */}
          <div className="mt-2 px-4 py-2.5 bg-zinc-900 border border-zinc-800 rounded-xl text-xs font-mono text-zinc-500 text-left w-full max-w-sm">
            <span className="text-zinc-400">title</span>,<span className="text-zinc-400">content</span>,<span className="text-zinc-400">type</span><br />
            "Product A","https://…","url"<br />
            "Trade WiFi","WIFI:T:WPA;…","wifi"<br />
            "Contact Card","BEGIN:VCARD…","vcard"
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
                <span className="ml-2 text-sm font-normal text-zinc-500">— {totalRows} row{totalRows !== 1 ? "s" : ""} detected</span>
              </p>
              {totalRows > 100 && (
                <p className="text-xs text-amber-400 mt-1">Capped at 100 rows per request.</p>
              )}
            </div>
            <button onClick={reset} className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors">
              Change file
            </button>
          </div>

          {/* Preview table */}
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
            {totalRows > 10 && (
              <p className="px-4 py-2.5 text-xs text-zinc-600 border-t border-zinc-800">
                Showing 10 of {Math.min(totalRows, 100)} rows that will be generated.
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
              Generate {Math.min(totalRows, 100)} QR Codes &amp; Download ZIP
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
          {/* Progress bar */}
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
            <p className="text-sm text-zinc-500">Your ZIP download should have started automatically.</p>
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
