"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";

interface LeadPage {
    id: string;
    name: string;
    slug: string;
    headline: string;
    views: number;
    submissions: number;
    is_active: boolean;
    created_at: string;
    button_color: string;
}

interface Lead {
    id: string;
    email: string;
    data: string;
    source: string;
    opted_in: boolean;
    created_at: string;
}

export default function LeadCapturePage() {
    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
    const [workspaceId, setWorkspaceId] = useState("");
    const [pages, setPages] = useState<LeadPage[]>([]);
    const [leads, setLeads] = useState<Lead[]>([]);
    const [showCreate, setShowCreate] = useState(false);
    const [activeTab, setActiveTab] = useState<"pages" | "leads">("pages");
    const [loading, setLoading] = useState(true);
    const [newPage, setNewPage] = useState({
        name: "",
        headline: "",
        subheadline: "",
        button_text: "Submit",
        button_color: "#8B5CF6",
        background_color: "#09090B",
        text_color: "#FAFAFA",
        thank_you_message: "Thank you for signing up!",
        form_fields: '[{"name":"email","type":"email","required":true,"label":"Email Address"},{"name":"name","type":"text","required":false,"label":"Full Name"}]',
    });

    useEffect(() => { loadData(); }, []);

    const loadData = async () => {
        setLoading(true);
        const token = useAuthStore.getState().accessToken;
        const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
        const wsData = await wsRes.json();
        if (wsData.success && wsData.data?.length > 0) {
            const id = wsData.data[0].id;
            setWorkspaceId(id);
            const [pagesRes, leadsRes] = await Promise.all([
                fetch(`${api}/workspaces/${id}/pages`, { headers: { Authorization: `Bearer ${token}` } }),
                fetch(`${api}/workspaces/${id}/leads`, { headers: { Authorization: `Bearer ${token}` } }),
            ]);
            const pagesData = await pagesRes.json();
            const leadsData = await leadsRes.json();
            if (pagesData.success) setPages(pagesData.data || []);
            if (leadsData.success) setLeads(leadsData.data || []);
        }
        setLoading(false);
    };

    const createPage = async () => {
        const token = useAuthStore.getState().accessToken;
        const res = await fetch(`${api}/workspaces/${workspaceId}/pages`, {
            method: "POST", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
            body: JSON.stringify(newPage),
        });
        const data = await res.json();
        if (data.success) {
            setPages([data.data, ...pages]);
            setShowCreate(false);
            setNewPage({ ...newPage, name: "", headline: "", subheadline: "" });
        }
    };

    const deletePage = async (id: string) => {
        const token = useAuthStore.getState().accessToken;
        await fetch(`${api}/workspaces/${workspaceId}/pages/${id}`, {
            method: "DELETE", headers: { Authorization: `Bearer ${token}` },
        });
        setPages(pages.filter((p) => p.id !== id));
    };

    const totalViews = pages.reduce((s, p) => s + p.views, 0);
    const totalSubmissions = pages.reduce((s, p) => s + p.submissions, 0);
    const avgConversion = totalViews > 0 ? ((totalSubmissions / totalViews) * 100).toFixed(1) : "0";

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold text-zinc-100">Lead Capture</h1>
                    <p className="text-sm text-zinc-500 mt-1">Create landing pages to capture leads from QR codes</p>
                </div>
                <button onClick={() => setShowCreate(true)} className="px-4 py-2 bg-violet-600 hover:bg-violet-500 rounded-lg text-sm font-medium text-white transition-colors">
                    + New Page
                </button>
            </div>

            {/* Summary */}
            <div className="grid grid-cols-3 gap-4">
                <StatCard label="Total Pages" value={pages.length} icon="📄" />
                <StatCard label="Total Views" value={totalViews} icon="👁️" />
                <StatCard label="Conversion Rate" value={`${avgConversion}%`} icon="🎯" />
            </div>

            {/* Tabs */}
            <div className="flex gap-1 bg-zinc-900/60 p-1 rounded-lg w-fit">
                {(["pages", "leads"] as const).map((tab) => (
                    <button key={tab} onClick={() => setActiveTab(tab)}
                        className={`px-4 py-1.5 rounded-md text-sm font-medium transition-all capitalize ${activeTab === tab ? "bg-violet-600 text-white" : "text-zinc-400 hover:text-zinc-200"
                            }`}>
                        {tab}
                    </button>
                ))}
            </div>

            {loading ? (
                <div className="flex justify-center py-16"><div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" /></div>
            ) : activeTab === "pages" ? (
                /* Pages Grid */
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {pages.map((page) => (
                        <div key={page.id} className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-5 hover:border-zinc-700/80 transition-colors group">
                            <div className="flex items-start justify-between mb-3">
                                <div>
                                    <h3 className="text-sm font-semibold text-zinc-200">{page.name}</h3>
                                    <p className="text-xs text-zinc-500 mt-0.5">{page.headline}</p>
                                </div>
                                <span className={`text-xs px-2 py-0.5 rounded-full ${page.is_active ? "bg-emerald-500/20 text-emerald-400" : "bg-zinc-700/40 text-zinc-500"}`}>
                                    {page.is_active ? "Live" : "Draft"}
                                </span>
                            </div>
                            <div className="flex items-center gap-4 text-xs text-zinc-500 mb-4">
                                <span>👁️ {page.views} views</span>
                                <span>📝 {page.submissions} leads</span>
                                <span>🎯 {page.views > 0 ? ((page.submissions / page.views) * 100).toFixed(1) : 0}%</span>
                            </div>
                            <div className="flex items-center gap-2">
                                <a href={`/p/${page.slug}`} target="_blank" rel="noopener"
                                    className="text-xs text-violet-400 hover:text-violet-300 truncate flex-1">
                                    /p/{page.slug}
                                </a>
                                <button onClick={() => deletePage(page.id)} className="text-xs text-red-400 hover:text-red-300 opacity-0 group-hover:opacity-100 transition-opacity">
                                    Delete
                                </button>
                            </div>
                        </div>
                    ))}
                    {pages.length === 0 && (
                        <div className="col-span-full text-center py-16 text-zinc-600">
                            <p className="text-4xl mb-2">📄</p>
                            <p>No lead capture pages yet. Create one to start capturing leads!</p>
                        </div>
                    )}
                </div>
            ) : (
                /* Leads Table */
                <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl overflow-hidden">
                    <table className="w-full">
                        <thead>
                            <tr className="border-b border-zinc-800/60">
                                <th className="text-left text-xs text-zinc-500 px-4 py-3 font-medium">Email</th>
                                <th className="text-left text-xs text-zinc-500 px-4 py-3 font-medium">Source</th>
                                <th className="text-left text-xs text-zinc-500 px-4 py-3 font-medium">Opted In</th>
                                <th className="text-left text-xs text-zinc-500 px-4 py-3 font-medium">Submitted</th>
                            </tr>
                        </thead>
                        <tbody>
                            {leads.map((lead) => (
                                <tr key={lead.id} className="border-b border-zinc-800/30 hover:bg-zinc-800/20 transition-colors">
                                    <td className="px-4 py-3 text-sm text-zinc-200">{lead.email || "—"}</td>
                                    <td className="px-4 py-3 text-sm text-zinc-400">{lead.source || "Direct"}</td>
                                    <td className="px-4 py-3">
                                        <span className={`text-xs px-2 py-0.5 rounded-full ${lead.opted_in ? "bg-emerald-500/20 text-emerald-400" : "bg-zinc-700/40 text-zinc-500"}`}>
                                            {lead.opted_in ? "Yes" : "No"}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3 text-xs text-zinc-500">{new Date(lead.created_at).toLocaleDateString()}</td>
                                </tr>
                            ))}
                            {leads.length === 0 && (
                                <tr><td colSpan={4} className="text-center py-12 text-zinc-600">No leads captured yet</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Create Modal */}
            {showCreate && (
                <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" onClick={() => setShowCreate(false)}>
                    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 w-full max-w-lg space-y-4" onClick={e => e.stopPropagation()}>
                        <h2 className="text-lg font-semibold text-zinc-200">Create Lead Capture Page</h2>

                        <div>
                            <label className="block text-xs text-zinc-500 mb-1">Page Name</label>
                            <input value={newPage.name} onChange={e => setNewPage({ ...newPage, name: e.target.value })}
                                className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                        </div>
                        <div>
                            <label className="block text-xs text-zinc-500 mb-1">Headline</label>
                            <input value={newPage.headline} onChange={e => setNewPage({ ...newPage, headline: e.target.value })}
                                placeholder="Get exclusive updates!" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                        </div>
                        <div>
                            <label className="block text-xs text-zinc-500 mb-1">Subheadline</label>
                            <input value={newPage.subheadline} onChange={e => setNewPage({ ...newPage, subheadline: e.target.value })}
                                placeholder="Sign up to receive the latest news" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                        </div>
                        <div className="grid grid-cols-3 gap-3">
                            <div>
                                <label className="block text-xs text-zinc-500 mb-1">Button Color</label>
                                <input type="color" value={newPage.button_color} onChange={e => setNewPage({ ...newPage, button_color: e.target.value })}
                                    className="w-full h-9 rounded border border-zinc-700 cursor-pointer" />
                            </div>
                            <div>
                                <label className="block text-xs text-zinc-500 mb-1">Background</label>
                                <input type="color" value={newPage.background_color} onChange={e => setNewPage({ ...newPage, background_color: e.target.value })}
                                    className="w-full h-9 rounded border border-zinc-700 cursor-pointer" />
                            </div>
                            <div>
                                <label className="block text-xs text-zinc-500 mb-1">Text Color</label>
                                <input type="color" value={newPage.text_color} onChange={e => setNewPage({ ...newPage, text_color: e.target.value })}
                                    className="w-full h-9 rounded border border-zinc-700 cursor-pointer" />
                            </div>
                        </div>

                        <div className="flex justify-end gap-3 pt-2">
                            <button onClick={() => setShowCreate(false)} className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-sm text-zinc-300">Cancel</button>
                            <button onClick={createPage} disabled={!newPage.name}
                                className="px-4 py-2 bg-violet-600 hover:bg-violet-500 rounded-lg text-sm font-medium text-white disabled:opacity-50">Create Page</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

function StatCard({ label, value, icon }: { label: string; value: number | string; icon: string }) {
    return (
        <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-4">
            <div className="flex items-center gap-2 mb-1">
                <span>{icon}</span>
                <span className="text-xs text-zinc-500 uppercase tracking-wider">{label}</span>
            </div>
            <p className="text-2xl font-bold text-zinc-100">{typeof value === "number" ? value.toLocaleString() : value}</p>
        </div>
    );
}
