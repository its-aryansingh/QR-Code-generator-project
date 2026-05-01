"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";

interface Folder {
    id: string;
    name: string;
    description: string;
    color: string;
    qr_count: number;
    scan_count: number;
    created_at: string;
}

export default function CampaignsPage() {
    const [folders, setFolders] = useState<Folder[]>([]);
    const [showCreate, setShowCreate] = useState(false);
    const [newName, setNewName] = useState("");
    const [newDescription, setNewDescription] = useState("");
    const [newColor, setNewColor] = useState("#8B5CF6");
    const [loading, setLoading] = useState(true);
    const [workspaceId, setWorkspaceId] = useState<string | null>(null);

    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
    const colors = ["#8B5CF6", "#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#EC4899", "#06B6D4", "#6366F1"];

    useEffect(() => {
        loadFolders();
    }, []);

    const loadFolders = async () => {
        const token = useAuthStore.getState().accessToken;
        let wsId = localStorage.getItem("qrit_active_workspace") || "";
        if (!wsId) {
            const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
            const wsData = await wsRes.json();
            if (wsData.success && wsData.data?.length > 0) {
                wsId = wsData.data[0].id;
                localStorage.setItem("qrit_active_workspace", wsId);
            }
        }
        if (!wsId) { setLoading(false); return; }
        setWorkspaceId(wsId);
        const fRes = await fetch(`${api}/workspaces/${wsId}/folders`, {
            headers: { Authorization: `Bearer ${token}` },
        });
        const fData = await fRes.json();
        if (fData.success) setFolders(fData.data || []);
        setLoading(false);
    };

    const handleCreate = async () => {
        if (!workspaceId || !newName) return;
        const token = useAuthStore.getState().accessToken;
        const res = await fetch(`${api}/workspaces/${workspaceId}/folders`, {
            method: "POST",
            headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
            body: JSON.stringify({ name: newName, description: newDescription, color: newColor }),
        });
        const data = await res.json();
        if (data.success) {
            setFolders([...folders, data.data]);
            setNewName("");
            setNewDescription("");
            setShowCreate(false);
        }
    };

    const handleDelete = async (id: string) => {
        if (!workspaceId) return;
        const token = useAuthStore.getState().accessToken;
        await fetch(`${api}/workspaces/${workspaceId}/folders/${id}`, {
            method: "DELETE",
            headers: { Authorization: `Bearer ${token}` },
        });
        setFolders(folders.filter(f => f.id !== id));
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="w-8 h-8 border-2 border-zinc-700 border-t-white rounded-full animate-spin" />
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold text-white">Campaigns & Folders</h1>
                    <p className="text-zinc-500 mt-1">Organize your QR codes into campaigns for better management</p>
                </div>
                <button
                    onClick={() => setShowCreate(true)}
                    className="px-4 py-2 bg-white text-zinc-950 text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors"
                >
                    + New Campaign
                </button>
            </div>

            {/* Create Modal */}
            {showCreate && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
                    <div className="bg-zinc-900 border border-zinc-800 rounded-2xl p-6 w-full max-w-md">
                        <h3 className="text-lg font-semibold text-white mb-4">New Campaign</h3>
                        <div className="space-y-4">
                            <div>
                                <label className="text-sm text-zinc-400 block mb-1">Name</label>
                                <input
                                    type="text"
                                    value={newName}
                                    onChange={e => setNewName(e.target.value)}
                                    className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                                    placeholder="Q1 Marketing Campaign"
                                />
                            </div>
                            <div>
                                <label className="text-sm text-zinc-400 block mb-1">Description</label>
                                <textarea
                                    value={newDescription}
                                    onChange={e => setNewDescription(e.target.value)}
                                    className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600 resize-none"
                                    rows={3}
                                    placeholder="Campaign description..."
                                />
                            </div>
                            <div>
                                <label className="text-sm text-zinc-400 block mb-1">Color</label>
                                <div className="flex gap-2">
                                    {colors.map(c => (
                                        <button
                                            key={c}
                                            onClick={() => setNewColor(c)}
                                            className={`w-8 h-8 rounded-full transition-all ${newColor === c ? 'ring-2 ring-white ring-offset-2 ring-offset-zinc-900 scale-110' : 'hover:scale-105'}`}
                                            style={{ backgroundColor: c }}
                                        />
                                    ))}
                                </div>
                            </div>
                            <div className="flex gap-3 pt-2">
                                <button onClick={() => setShowCreate(false)}
                                    className="flex-1 px-4 py-2.5 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 transition-colors">
                                    Cancel
                                </button>
                                <button onClick={handleCreate}
                                    className="flex-1 px-4 py-2.5 bg-white text-zinc-950 font-medium rounded-lg hover:bg-zinc-200 transition-colors">
                                    Create
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Folders Grid */}
            {folders.length === 0 ? (
                <div className="text-center py-16 bg-zinc-900/30 border border-zinc-800/60 rounded-xl">
                    <svg className="w-12 h-12 mx-auto text-zinc-700 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                    </svg>
                    <p className="text-zinc-400 mb-1">No campaigns yet</p>
                    <p className="text-zinc-600 text-sm">Create your first campaign to organize QR codes</p>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {folders.map(folder => (
                        <div key={folder.id} className="group bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-5 hover:border-zinc-700 transition-all">
                            <div className="flex items-start justify-between">
                                <div className="flex items-center gap-3">
                                    <div className="w-10 h-10 rounded-lg flex items-center justify-center" style={{ backgroundColor: folder.color + '20' }}>
                                        <svg className="w-5 h-5" style={{ color: folder.color }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                                        </svg>
                                    </div>
                                    <div>
                                        <h3 className="text-sm font-medium text-white">{folder.name}</h3>
                                        {folder.description && (
                                            <p className="text-xs text-zinc-500 mt-0.5 line-clamp-1">{folder.description}</p>
                                        )}
                                    </div>
                                </div>
                                <button
                                    onClick={() => handleDelete(folder.id)}
                                    className="opacity-0 group-hover:opacity-100 p-1 text-zinc-600 hover:text-red-400 transition-all"
                                >
                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                    </svg>
                                </button>
                            </div>
                            <div className="flex items-center gap-4 mt-4 pt-4 border-t border-zinc-800/60">
                                <div>
                                    <p className="text-lg font-semibold text-white">{folder.qr_count}</p>
                                    <p className="text-xs text-zinc-500">QR Codes</p>
                                </div>
                                <div>
                                    <p className="text-lg font-semibold text-white">{folder.scan_count}</p>
                                    <p className="text-xs text-zinc-500">Total Scans</p>
                                </div>
                                <p className="text-xs text-zinc-600 ml-auto">{new Date(folder.created_at).toLocaleDateString()}</p>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
