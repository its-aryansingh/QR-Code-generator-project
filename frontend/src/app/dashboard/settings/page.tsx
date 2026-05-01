"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";

export default function SettingsPage() {
    const [workspace, setWorkspace] = useState<any>(null);
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [showCreateWs, setShowCreateWs] = useState(false);
    const [newWsName, setNewWsName] = useState("");
    const [saving, setSaving] = useState(false);
    const [saved, setSaved] = useState(false);
    const [deleting, setDeleting] = useState(false);

    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

    useEffect(() => {
        const token = useAuthStore.getState().accessToken;
        fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } })
            .then(r => r.json())
            .then(data => {
                if (data.success && data.data?.length > 0) {
                    setWorkspace(data.data[0]);
                    setName(data.data[0].name);
                    setDescription(data.data[0].description || "");
                }
            });
    }, []);

    const handleSave = async () => {
        if (!workspace) return;
        setSaving(true);
        const token = useAuthStore.getState().accessToken;
        await fetch(`${api}/workspaces/${workspace.id}`, {
            method: "PUT",
            headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
            body: JSON.stringify({ name, description }),
        });
        setSaving(false);
        setSaved(true);
        setTimeout(() => setSaved(false), 2000);
    };

    const handleDeleteWorkspace = async () => {
        if (!workspace) return;
        if (!confirm(`Permanently delete "${workspace.name}"? This cannot be undone.`)) return;
        setDeleting(true);
        const token = useAuthStore.getState().accessToken;
        try {
            await fetch(`${api}/workspaces/${workspace.id}`, {
                method: "DELETE",
                headers: { Authorization: `Bearer ${token}` },
            });
            localStorage.removeItem("qrit_active_workspace");
            window.location.href = "/dashboard";
        } catch {
            setDeleting(false);
        }
    };

    const handleCreateWorkspace = async () => {
        if (!newWsName) return;
        const token = useAuthStore.getState().accessToken;
        const res = await fetch(`${api}/workspaces`, {
            method: "POST",
            headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
            body: JSON.stringify({ name: newWsName }),
        });
        const data = await res.json();
        if (data.success) {
            setShowCreateWs(false);
            setNewWsName("");
            window.location.reload();
        }
    };

    return (
        <div className="space-y-8 max-w-2xl">
            <div>
                <h1 className="text-2xl font-bold text-white">Workspace Settings</h1>
                <p className="text-zinc-500 mt-1">Configure your workspace preferences</p>
            </div>

            {/* Workspace Info */}
            <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-5">
                <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider">General</h2>
                <div>
                    <label className="text-sm text-zinc-400 block mb-1">Workspace Name</label>
                    <input type="text" value={name} onChange={e => setName(e.target.value)}
                        className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white focus:outline-none focus:border-zinc-600" />
                </div>
                <div>
                    <label className="text-sm text-zinc-400 block mb-1">Description</label>
                    <textarea value={description} onChange={e => setDescription(e.target.value)}
                        className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white focus:outline-none focus:border-zinc-600 resize-none"
                        rows={3} placeholder="What is this workspace for?" />
                </div>
                <div className="flex items-center gap-3">
                    <button onClick={handleSave} disabled={saving}
                        className="px-5 py-2 bg-white text-zinc-950 text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors disabled:opacity-50">
                        {saving ? 'Saving...' : 'Save Changes'}
                    </button>
                    {saved && <span className="text-sm text-emerald-400">✓ Saved</span>}
                </div>
            </div>

            {/* Plan Info */}
            <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
                <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider">Plan & Limits</h2>
                <div className="grid grid-cols-3 gap-4">
                    <div className="bg-zinc-800/50 rounded-lg p-4 text-center">
                        <p className="text-2xl font-bold text-white">{workspace?.max_qr_codes || 50}</p>
                        <p className="text-xs text-zinc-500 mt-1">Max QR Codes</p>
                    </div>
                    <div className="bg-zinc-800/50 rounded-lg p-4 text-center">
                        <p className="text-2xl font-bold text-white">{workspace?.max_members || 1}</p>
                        <p className="text-xs text-zinc-500 mt-1">Max Members</p>
                    </div>
                    <div className="bg-zinc-800/50 rounded-lg p-4 text-center">
                        <p className="text-2xl font-bold text-white">{workspace?.max_folders || 5}</p>
                        <p className="text-xs text-zinc-500 mt-1">Max Folders</p>
                    </div>
                </div>
                <p className="text-xs text-zinc-500">
                    Current plan: <span className="text-zinc-300 font-medium capitalize">{workspace?.plan || 'free'}</span>
                </p>
            </div>

            {/* Create New Workspace */}
            <div className="bg-zinc-900/50 border border-zinc-800/60 rounded-xl p-6 space-y-4">
                <h2 className="text-sm font-semibold text-zinc-300 uppercase tracking-wider">Create New Workspace</h2>
                <p className="text-sm text-zinc-500">Create additional workspaces for different teams or projects.</p>
                {showCreateWs ? (
                    <div className="space-y-3">
                        <input type="text" value={newWsName} onChange={e => setNewWsName(e.target.value)}
                            className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-zinc-600"
                            placeholder="New workspace name" />
                        <div className="flex gap-2">
                            <button onClick={() => setShowCreateWs(false)}
                                className="px-4 py-2 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 text-sm transition-colors">Cancel</button>
                            <button onClick={handleCreateWorkspace}
                                className="px-4 py-2 bg-white text-zinc-950 font-medium rounded-lg hover:bg-zinc-200 text-sm transition-colors">Create</button>
                        </div>
                    </div>
                ) : (
                    <button onClick={() => setShowCreateWs(true)}
                        className="px-4 py-2 bg-zinc-800 text-zinc-300 rounded-lg hover:bg-zinc-700 text-sm transition-colors border border-zinc-700">
                        + New Workspace
                    </button>
                )}
            </div>

            {/* Danger Zone */}
            {workspace && (
                <div className="bg-red-500/5 border border-red-500/10 rounded-xl p-6 space-y-4">
                    <h2 className="text-sm font-semibold text-red-400 uppercase tracking-wider">Danger Zone</h2>
                    <p className="text-sm text-zinc-500">Permanently delete this workspace and all its data including QR codes, campaigns, and analytics.</p>
                    <button
                        onClick={handleDeleteWorkspace}
                        disabled={deleting}
                        className="px-4 py-2 bg-red-500/10 text-red-400 border border-red-500/20 rounded-lg text-sm hover:bg-red-500/20 transition-colors disabled:opacity-50"
                    >
                        {deleting ? "Deleting…" : "Delete Workspace"}
                    </button>
                </div>
            )}
        </div>
    );
}
