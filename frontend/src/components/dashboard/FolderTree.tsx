"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuthStore } from "@/lib/auth";
import { toast } from "sonner";

interface Folder {
  id: string;
  name: string;
  color: string;
  description?: string;
  parent_id?: string;
  children?: Folder[];
}

interface FolderTreeProps {
  workspaceId: string;
  selectedFolderId?: string;
  onSelectFolder: (folderId?: string) => void;
}

function buildTree(flat: Folder[]): Folder[] {
  const map = new Map<string, Folder>();
  flat.forEach((f) => map.set(f.id, { ...f, children: [] }));
  const roots: Folder[] = [];
  map.forEach((f) => {
    if (f.parent_id && map.has(f.parent_id)) {
      map.get(f.parent_id)!.children!.push(f);
    } else {
      roots.push(f);
    }
  });
  return roots;
}

function FolderNode({
  folder,
  depth,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  depth: number;
  selected?: string;
  onSelect: (id: string) => void;
  onRename: (id: string, name: string) => void;
  onDelete: (id: string) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const [renaming, setRenaming] = useState(false);
  const [tempName, setTempName] = useState(folder.name);
  const hasChildren = (folder.children?.length ?? 0) > 0;

  return (
    <div>
      <div
        className={`group flex items-center gap-1.5 px-2 py-1.5 rounded-lg cursor-pointer transition-colors ${selected === folder.id ? "bg-zinc-800 text-zinc-100" : "text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200"}`}
        style={{ paddingLeft: `${8 + depth * 16}px` }}
        onClick={() => onSelect(folder.id)}
      >
        {hasChildren && (
          <button
            onClick={(e) => { e.stopPropagation(); setExpanded(!expanded); }}
            className="w-4 h-4 flex items-center justify-center text-zinc-600 hover:text-zinc-400 flex-shrink-0"
          >
            <svg className={`w-3 h-3 transition-transform ${expanded ? "rotate-90" : ""}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        )}
        {!hasChildren && <span className="w-4 flex-shrink-0" />}

        <span
          className="w-3 h-3 rounded-sm flex-shrink-0"
          style={{ backgroundColor: folder.color || "#8B5CF6" }}
        />

        {renaming ? (
          <input
            autoFocus
            value={tempName}
            onChange={(e) => setTempName(e.target.value)}
            onBlur={() => { onRename(folder.id, tempName); setRenaming(false); }}
            onKeyDown={(e) => { if (e.key === "Enter") { onRename(folder.id, tempName); setRenaming(false); } if (e.key === "Escape") { setTempName(folder.name); setRenaming(false); } }}
            onClick={(e) => e.stopPropagation()}
            className="flex-1 bg-zinc-900 border border-zinc-700 rounded px-1.5 py-0.5 text-xs text-zinc-200 focus:outline-none focus:border-violet-500"
          />
        ) : (
          <span className="flex-1 text-sm truncate">{folder.name}</span>
        )}

        {/* Actions — show on hover */}
        <div className="hidden group-hover:flex items-center gap-0.5 ml-auto flex-shrink-0">
          <button
            onClick={(e) => { e.stopPropagation(); setRenaming(true); }}
            className="w-5 h-5 flex items-center justify-center text-zinc-600 hover:text-zinc-300"
            title="Rename"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(folder.id); }}
            className="w-5 h-5 flex items-center justify-center text-zinc-600 hover:text-red-400"
            title="Delete"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>

      {expanded && hasChildren && (
        <div>
          {folder.children!.map((child) => (
            <FolderNode
              key={child.id}
              folder={child}
              depth={depth + 1}
              selected={selected}
              onSelect={onSelect}
              onRename={onRename}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function FolderTree({ workspaceId, selectedFolderId, onSelectFolder }: FolderTreeProps) {
  const { accessToken } = useAuthStore();
  const [tree, setTree] = useState<Folder[]>([]);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newColor, setNewColor] = useState("#8B5CF6");
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const fetchFolders = useCallback(async () => {
    if (!workspaceId || !accessToken) return;
    const res = await fetch(`${api}/workspaces/${workspaceId}/folders`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    }).then((r) => r.json());
    if (res.success) setTree(buildTree(res.data ?? []));
  }, [workspaceId, accessToken]);

  useEffect(() => { fetchFolders(); }, [fetchFolders]);

  const createFolder = async () => {
    if (!newName.trim()) return;
    const res = await fetch(`${api}/workspaces/${workspaceId}/folders`, {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
      body: JSON.stringify({ name: newName.trim(), color: newColor }),
    }).then((r) => r.json());

    if (res.success) {
      fetchFolders();
      setCreating(false);
      setNewName("");
      toast.success("Folder created");
    }
  };

  const renameFolder = async (id: string, name: string) => {
    if (!name.trim()) return;
    await fetch(`${api}/workspaces/${workspaceId}/folders/${id}`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    fetchFolders();
  };

  const deleteFolder = async (id: string) => {
    if (!confirm("Delete this folder? QR codes inside will be moved to root.")) return;
    await fetch(`${api}/workspaces/${workspaceId}/folders/${id}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    if (selectedFolderId === id) onSelectFolder(undefined);
    fetchFolders();
    toast.success("Folder deleted");
  };

  const COLORS = ["#8B5CF6", "#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#EC4899", "#06B6D4"];

  return (
    <div className="bg-zinc-900 rounded-xl border border-zinc-800 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-zinc-800">
        <span className="text-xs font-semibold text-zinc-500 uppercase tracking-widest">Folders</span>
        <button
          onClick={() => setCreating(!creating)}
          className="w-6 h-6 flex items-center justify-center rounded text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
          title="New folder"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
      </div>

      {/* Create form */}
      {creating && (
        <div className="px-3 py-2.5 border-b border-zinc-800 space-y-2 bg-zinc-950/50">
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") createFolder(); if (e.key === "Escape") setCreating(false); }}
            placeholder="Folder name…"
            className="w-full px-2.5 py-1.5 bg-zinc-900 border border-zinc-700 rounded-lg text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500"
          />
          <div className="flex items-center gap-1.5">
            {COLORS.map((c) => (
              <button
                key={c}
                onClick={() => setNewColor(c)}
                className={`w-5 h-5 rounded-sm transition-transform ${newColor === c ? "scale-125 ring-2 ring-white/30" : ""}`}
                style={{ backgroundColor: c }}
              />
            ))}
          </div>
          <div className="flex gap-2">
            <button onClick={createFolder} disabled={!newName.trim()} className="flex-1 py-1 bg-violet-600 text-white text-xs font-semibold rounded-lg hover:bg-violet-500 disabled:opacity-50 transition-colors">
              Create
            </button>
            <button onClick={() => { setCreating(false); setNewName(""); }} className="px-3 py-1 bg-zinc-800 text-zinc-400 text-xs rounded-lg hover:bg-zinc-700 transition-colors">
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* All QR node */}
      <div className="py-1.5 px-2">
        <div
          onClick={() => onSelectFolder(undefined)}
          className={`flex items-center gap-2 px-2 py-1.5 rounded-lg cursor-pointer transition-colors text-sm ${!selectedFolderId ? "bg-zinc-800 text-zinc-100" : "text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200"}`}
        >
          <svg className="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
          </svg>
          All QR Codes
        </div>

        {/* Folder tree */}
        {tree.map((f) => (
          <FolderNode
            key={f.id}
            folder={f}
            depth={0}
            selected={selectedFolderId}
            onSelect={(id) => onSelectFolder(id)}
            onRename={renameFolder}
            onDelete={deleteFolder}
          />
        ))}

        {tree.length === 0 && !creating && (
          <p className="text-xs text-zinc-700 text-center py-4">No folders yet</p>
        )}
      </div>
    </div>
  );
}
