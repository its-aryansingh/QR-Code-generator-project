"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";
import Link from "next/link";

interface DashboardStats {
    totalQR: number;
    totalScans: number;
    activeQR: number;
    dynamicQR: number;
}

export default function DashboardOverview() {
    const [stats, setStats] = useState<DashboardStats>({ totalQR: 0, totalScans: 0, activeQR: 0, dynamicQR: 0 });
    const [recentQR, setRecentQR] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const token = useAuthStore.getState().accessToken;
        const api = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081/api/v1';

        // Fetch stats
        Promise.all([
            fetch(`${api}/analytics/summary`, { headers: { Authorization: `Bearer ${token}` } }).then(r => r.json()),
            fetch(`${api}/qr/history?limit=5`, { headers: { Authorization: `Bearer ${token}` } }).then(r => r.json()),
        ]).then(([summaryRes, historyRes]) => {
            if (summaryRes.success) {
                setStats({
                    totalQR: summaryRes.data?.total_qr_codes || 0,
                    totalScans: summaryRes.data?.total_scans || 0,
                    activeQR: summaryRes.data?.active_qr_codes || 0,
                    dynamicQR: summaryRes.data?.dynamic_qr_codes || 0,
                });
            }
            if (historyRes.success) {
                setRecentQR(historyRes.data?.records || []);
            }
        }).finally(() => setLoading(false));
    }, []);

    const statCards = [
        { label: "Total QR Codes", value: stats.totalQR, icon: "📊", color: "from-violet-500/10 to-violet-600/5", border: "border-violet-500/20" },
        { label: "Total Scans", value: stats.totalScans, icon: "📱", color: "from-emerald-500/10 to-emerald-600/5", border: "border-emerald-500/20" },
        { label: "Active QR Codes", value: stats.activeQR, icon: "✅", color: "from-blue-500/10 to-blue-600/5", border: "border-blue-500/20" },
        { label: "Dynamic QR Codes", value: stats.dynamicQR, icon: "🔗", color: "from-amber-500/10 to-amber-600/5", border: "border-amber-500/20" },
    ];

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="w-8 h-8 border-2 border-zinc-700 border-t-white rounded-full animate-spin" />
            </div>
        );
    }

    return (
        <div className="space-y-8">
            {/* Welcome Header */}
            <div>
                <h1 className="text-2xl font-bold text-white">Dashboard</h1>
                <p className="text-zinc-500 mt-1">Overview of your QR code performance and activity.</p>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                {statCards.map((card) => (
                    <div key={card.label}
                        className={`bg-gradient-to-br ${card.color} border ${card.border} rounded-xl p-5 transition-all hover:scale-[1.02]`}
                    >
                        <div className="flex items-center justify-between">
                            <span className="text-2xl">{card.icon}</span>
                        </div>
                        <p className="text-3xl font-bold text-white mt-3">{card.value.toLocaleString()}</p>
                        <p className="text-sm text-zinc-400 mt-1">{card.label}</p>
                    </div>
                ))}
            </div>

            {/* Quick Actions */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <Link href="/dashboard/qr-codes"
                    className="group flex items-center gap-4 p-5 bg-zinc-900/50 border border-zinc-800/60 rounded-xl hover:border-zinc-700 transition-all">
                    <div className="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center text-violet-400 group-hover:bg-violet-500/20 transition-colors">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4v16m8-8H4" /></svg>
                    </div>
                    <div>
                        <p className="text-sm font-medium text-white">Create QR Code</p>
                        <p className="text-xs text-zinc-500">Generate new QR codes</p>
                    </div>
                </Link>
                <Link href="/dashboard/campaigns"
                    className="group flex items-center gap-4 p-5 bg-zinc-900/50 border border-zinc-800/60 rounded-xl hover:border-zinc-700 transition-all">
                    <div className="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-400 group-hover:bg-blue-500/20 transition-colors">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
                    </div>
                    <div>
                        <p className="text-sm font-medium text-white">Campaigns</p>
                        <p className="text-xs text-zinc-500">Organize into folders</p>
                    </div>
                </Link>
                <Link href="/dashboard/team"
                    className="group flex items-center gap-4 p-5 bg-zinc-900/50 border border-zinc-800/60 rounded-xl hover:border-zinc-700 transition-all">
                    <div className="w-10 h-10 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-400 group-hover:bg-emerald-500/20 transition-colors">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
                    </div>
                    <div>
                        <p className="text-sm font-medium text-white">Manage Team</p>
                        <p className="text-xs text-zinc-500">Invite members & set roles</p>
                    </div>
                </Link>
            </div>

            {/* Recent QR Codes */}
            <div>
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold text-white">Recent QR Codes</h2>
                    <Link href="/dashboard/qr-codes" className="text-sm text-zinc-400 hover:text-white transition-colors">
                        View All →
                    </Link>
                </div>
                <div className="bg-zinc-900/30 border border-zinc-800/60 rounded-xl overflow-hidden">
                    <table className="w-full">
                        <thead>
                            <tr className="border-b border-zinc-800/60">
                                <th className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">Title</th>
                                <th className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">Type</th>
                                <th className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">Scans</th>
                                <th className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">Status</th>
                                <th className="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-5 py-3">Created</th>
                            </tr>
                        </thead>
                        <tbody>
                            {recentQR.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="text-center py-10 text-zinc-500">
                                        <p>No QR codes yet.</p>
                                        <Link href="/dashboard/qr-codes" className="text-violet-400 hover:text-violet-300 text-sm mt-2 inline-block">
                                            Create your first QR code →
                                        </Link>
                                    </td>
                                </tr>
                            ) : (
                                recentQR.map((qr: any) => (
                                    <tr key={qr.id} className="border-b border-zinc-800/30 hover:bg-zinc-800/20 transition-colors">
                                        <td className="px-5 py-3">
                                            <p className="text-sm text-white font-medium">{qr.title || 'Untitled'}</p>
                                            <p className="text-xs text-zinc-500 truncate max-w-xs">{qr.content}</p>
                                        </td>
                                        <td className="px-5 py-3">
                                            <span className="text-xs px-2 py-1 rounded-full bg-zinc-800 text-zinc-300">{qr.qr_type}</span>
                                        </td>
                                        <td className="px-5 py-3 text-sm text-zinc-300">{qr.scan_count || 0}</td>
                                        <td className="px-5 py-3">
                                            <span className={`text-xs px-2 py-1 rounded-full ${qr.is_active ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'}`}>
                                                {qr.is_active ? 'Active' : 'Inactive'}
                                            </span>
                                        </td>
                                        <td className="px-5 py-3 text-sm text-zinc-500">
                                            {new Date(qr.created_at).toLocaleDateString()}
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
}
