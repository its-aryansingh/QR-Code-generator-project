"use client";

import { useState, useEffect } from "react";
import { useAuthStore } from "@/lib/auth";
import { BulkUpload } from "@/components/dashboard/BulkUpload";

export default function BulkPage() {
  const { accessToken } = useAuthStore();
  const [workspaceId, setWorkspaceId] = useState<string>("");

  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  useEffect(() => {
    const stored = localStorage.getItem("qrit_active_workspace");
    if (stored) { setWorkspaceId(stored); return; }
    if (!accessToken) return;
    fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((data) => {
        if (data.success && data.data?.length) {
          setWorkspaceId(data.data[0].id);
          localStorage.setItem("qrit_active_workspace", data.data[0].id);
        }
      })
      .catch(() => {});
  }, [accessToken]);

  return (
    <div className="max-w-3xl space-y-8">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-bold text-zinc-100">Bulk QR Generation</h1>
        <p className="text-sm text-zinc-500 mt-1">
          Upload a CSV to generate up to 100 QR codes at once and download them as a ZIP archive.
        </p>
      </div>

      {/* Instructions */}
      <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-3">
        <h2 className="text-sm font-semibold text-zinc-300">CSV format</h2>
        <p className="text-xs text-zinc-500 leading-relaxed">
          Your file must include a header row. The <code className="text-violet-400">content</code> (or <code className="text-violet-400">url</code>) column is required.
          <code className="text-violet-400"> title</code> and <code className="text-violet-400">type</code> are optional — title defaults to the content value, type defaults to <code className="text-violet-400">url</code>.
        </p>
        <div className="rounded-lg bg-zinc-950 border border-zinc-800 px-4 py-3 text-xs font-mono text-zinc-400 overflow-x-auto">
          <span className="text-emerald-400">title</span>,<span className="text-emerald-400">content</span>,<span className="text-emerald-400">type</span>{"\n"}
          "Homepage","https://acme.com","url"{"\n"}
          "Office WiFi","WIFI:T:WPA;S:AcmeHQ;P:secret;;","wifi"{"\n"}
          "Jane Doe","BEGIN:VCARD\nVERSION:3.0\nFN:Jane Doe\nEND:VCARD","vcard"
        </div>

        <div className="grid sm:grid-cols-3 gap-3 pt-1">
          {[
            { label: "Supported types", value: "url · wifi · vcard · email · sms · text · location" },
            { label: "Max rows", value: "100 per request (Enterprise: 1 000 async)" },
            { label: "Max file size", value: "5 MB" },
          ].map((item) => (
            <div key={item.label} className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
              <p className="text-[10px] font-semibold text-zinc-600 uppercase tracking-wider mb-1">{item.label}</p>
              <p className="text-xs text-zinc-400">{item.value}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Upload component */}
      {workspaceId ? (
        <BulkUpload workspaceId={workspaceId} />
      ) : (
        <div className="flex flex-col items-center justify-center py-16 gap-4">
          <div className="w-10 h-10 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
          <p className="text-zinc-500 text-sm">Loading workspace…</p>
        </div>
      )}
    </div>
  );
}
