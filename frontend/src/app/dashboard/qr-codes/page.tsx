"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useAuthStore } from "@/lib/auth";
import { QRTable } from "@/components/dashboard/QRTable";
import { FolderTree } from "@/components/dashboard/FolderTree";
import { toast } from "sonner";

interface Folder {
  id: string;
  name: string;
  color: string;
}

export default function QRCodesPage() {
  const { accessToken } = useAuthStore();
  const [workspaceId, setWorkspaceId] = useState<string>("");
  const [selectedFolderId, setSelectedFolderId] = useState<string | undefined>();
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [showMoveModal, setShowMoveModal] = useState(false);
  const [moving, setMoving] = useState(false);
  const [tableKey, setTableKey] = useState(0);

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

  const loadFolders = useCallback(async () => {
    if (!workspaceId || !accessToken) return;
    const res = await fetch(`${api}/workspaces/${workspaceId}/folders`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    const data = await res.json();
    if (data.success) setFolders(data.data || []);
  }, [workspaceId, accessToken, api]);

  useEffect(() => { loadFolders(); }, [loadFolders]);

  const handleMoveToFolder = async (folderId: string | null) => {
    if (!workspaceId || selectedIds.length === 0) return;
    setMoving(true);
    try {
      const res = await fetch(`${api}/workspaces/${workspaceId}/qr/bulk-move`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({ qr_ids: selectedIds, folder_id: folderId }),
      });
      const data = await res.json();
      if (data.success) {
        toast.success(`Moved ${selectedIds.length} QR code(s)`);
        setSelectedIds([]);
        setShowMoveModal(false);
        setTableKey((k) => k + 1);
      } else {
        toast.error(data.error ?? "Move failed");
      }
    } catch {
      toast.error("Network error");
    } finally {
      setMoving(false);
    }
  };

  if (!workspaceId) {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-4">
        <div className="w-10 h-10 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
        <p className="text-zinc-500 text-sm">Loading workspace…</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-zinc-100">QR Codes</h1>
          <p className="text-sm text-zinc-500 mt-0.5">Manage all QR codes in this workspace</p>
        </div>
        <div className="flex items-center gap-2">
          <Link
            href="/dashboard/bulk"
            className="flex items-center gap-2 px-4 py-2 border border-zinc-700 text-zinc-300 text-sm font-semibold rounded-xl hover:bg-zinc-800 hover:text-zinc-100 transition-all"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            Bulk Upload
          </Link>
          <Link
            href="/dashboard/create"
            className="flex items-center gap-2 px-4 py-2 bg-white text-zinc-950 text-sm font-semibold rounded-xl hover:bg-zinc-100 transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            New QR Code
          </Link>
        </div>
      </div>

      <div className="flex gap-5">
        <div className="w-52 flex-shrink-0 hidden lg:block">
          <FolderTree
            workspaceId={workspaceId}
            selectedFolderId={selectedFolderId}
            onSelectFolder={setSelectedFolderId}
          />
        </div>

        <div className="flex-1 min-w-0 space-y-3">
          <div className="lg:hidden">
            <FolderTree
              workspaceId={workspaceId}
              selectedFolderId={selectedFolderId}
              onSelectFolder={setSelectedFolderId}
            />
          </div>

          <QRTable
            key={tableKey}
            workspaceId={workspaceId}
            folderId={selectedFolderId}
            onSelect={setSelectedIds}
          />
        </div>
      </div>

      {/* Floating selection bar */}
      {selectedIds.length > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50">
          <div className="flex items-center gap-3 px-5 py-3 bg-zinc-900 border border-zinc-700 rounded-2xl shadow-2xl shadow-black/50 backdrop-blur-xl">
            <span className="text-sm font-semibold text-zinc-300">{selectedIds.length} selected</span>
            <div className="h-4 w-px bg-zinc-700" />
            <button
              onClick={() => setShowMoveModal(true)}
              className="text-sm text-violet-400 font-medium hover:text-violet-300 transition-colors"
            >
              Move to folder
            </button>
          </div>
        </div>
      )}

      {/* Move to folder modal */}
      {showMoveModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" onClick={() => setShowMoveModal(false)}>
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 w-full max-w-sm space-y-4" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-sm font-semibold text-zinc-200">Move {selectedIds.length} QR code(s) to…</h3>
            <div className="space-y-1 max-h-64 overflow-y-auto">
              <button
                onClick={() => handleMoveToFolder(null)}
                disabled={moving}
                className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-zinc-800 text-left transition-colors"
              >
                <div className="w-5 h-5 rounded bg-zinc-700 flex items-center justify-center">
                  <svg className="w-3 h-3 text-zinc-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                  </svg>
                </div>
                <span className="text-sm text-zinc-400">No folder (root)</span>
              </button>
              {folders.map((f) => (
                <button
                  key={f.id}
                  onClick={() => handleMoveToFolder(f.id)}
                  disabled={moving}
                  className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-zinc-800 text-left transition-colors"
                >
                  <div className="w-5 h-5 rounded flex items-center justify-center" style={{ backgroundColor: f.color + "30" }}>
                    <svg className="w-3 h-3" style={{ color: f.color }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                    </svg>
                  </div>
                  <span className="text-sm text-zinc-200">{f.name}</span>
                </button>
              ))}
              {folders.length === 0 && (
                <p className="text-xs text-zinc-600 px-3 py-2">No folders yet. Create one in Campaigns.</p>
              )}
            </div>
            <button
              onClick={() => setShowMoveModal(false)}
              className="w-full py-2 text-sm text-zinc-500 hover:text-zinc-300 border border-zinc-800 rounded-lg transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
