"use client";

import { useState } from "react";
import { useAuthStore } from "@/lib/auth";

export default function ReportsPage() {
    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
    const [downloading, setDownloading] = useState<string | null>(null);
    const [days, setDays] = useState(30);

    const downloadReport = async (type: string) => {
        setDownloading(type);
        const token = useAuthStore.getState().accessToken;

        // Get first workspace
        const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
        const wsData = await wsRes.json();
        if (!wsData.success || !wsData.data?.length) {
            setDownloading(null);
            return;
        }
        const wsId = wsData.data[0].id;

        let url = "";
        let filename = "";
        switch (type) {
            case "analytics":
                url = `${api}/workspaces/${wsId}/export/analytics?days=${days}`;
                filename = `analytics_${days}d.csv`;
                break;
            case "qr-codes":
                url = `${api}/workspaces/${wsId}/export/qr-codes`;
                filename = "qr_codes.csv";
                break;
            case "leads":
                url = `${api}/workspaces/${wsId}/export/leads`;
                filename = "leads.csv";
                break;
            case "report":
                url = `${api}/workspaces/${wsId}/report?days=${days}`;
                filename = `report_${days}d.json`;
                break;
        }

        try {
            const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
            const blob = await res.blob();
            const a = document.createElement("a");
            a.href = URL.createObjectURL(blob);
            a.download = filename;
            a.click();
            URL.revokeObjectURL(a.href);
        } catch (err) {
            console.error("Download failed:", err);
        }
        setDownloading(null);
    };

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-2xl font-bold text-zinc-100">Reports & Export</h1>
                <p className="text-sm text-zinc-500 mt-1">Download analytics reports and export data</p>
            </div>

            {/* Period Selector */}
            <div className="flex items-center gap-3">
                <span className="text-sm text-zinc-400">Report Period:</span>
                <div className="flex gap-2">
                    {[7, 14, 30, 90].map((d) => (
                        <button key={d} onClick={() => setDays(d)}
                            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${days === d ? "bg-violet-600 text-white" : "bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800"
                                }`}>
                            {d} days
                        </button>
                    ))}
                </div>
            </div>

            {/* Export Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <ExportCard
                    title="Analytics Report (CSV)"
                    description="Scan data by date, device breakdowns, country distribution"
                    icon="📊"
                    color="violet"
                    downloading={downloading === "analytics"}
                    onDownload={() => downloadReport("analytics")}
                />
                <ExportCard
                    title="QR Codes Export (CSV)"
                    description="All QR codes with title, content, type, scan count, and status"
                    icon="📱"
                    color="blue"
                    downloading={downloading === "qr-codes"}
                    onDownload={() => downloadReport("qr-codes")}
                />
                <ExportCard
                    title="Leads Export (CSV)"
                    description="All captured leads with email, source, opt-in status, and timestamps"
                    icon="👥"
                    color="emerald"
                    downloading={downloading === "leads"}
                    onDownload={() => downloadReport("leads")}
                />
                <ExportCard
                    title="Full Performance Report (JSON)"
                    description="Comprehensive report with summary, breakdowns, and top performers"
                    icon="📋"
                    color="amber"
                    downloading={downloading === "report"}
                    onDownload={() => downloadReport("report")}
                />
            </div>

            {/* Scheduled Reports Info */}
            <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
                <h3 className="text-sm font-semibold text-zinc-300 mb-3">📅 Scheduled Reports</h3>
                <p className="text-sm text-zinc-500 mb-4">
                    Automatically receive performance reports in your inbox. Available on Pro and Enterprise plans.
                </p>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                    <div className="bg-zinc-800/40 rounded-lg p-4 border border-zinc-700/30">
                        <p className="text-sm font-medium text-zinc-300">Daily Digest</p>
                        <p className="text-xs text-zinc-500 mt-1">Key metrics, new scans, and alerts</p>
                    </div>
                    <div className="bg-zinc-800/40 rounded-lg p-4 border border-zinc-700/30">
                        <p className="text-sm font-medium text-zinc-300">Weekly Summary</p>
                        <p className="text-xs text-zinc-500 mt-1">Top performers, growth trends, team activity</p>
                    </div>
                    <div className="bg-zinc-800/40 rounded-lg p-4 border border-zinc-700/30">
                        <p className="text-sm font-medium text-zinc-300">Monthly Report</p>
                        <p className="text-xs text-zinc-500 mt-1">Full analytics with charts and recommendations</p>
                    </div>
                </div>
                <button className="mt-4 px-4 py-2 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700/50 rounded-lg text-sm text-zinc-300 transition-colors">
                    Configure Scheduled Reports →
                </button>
            </div>
        </div>
    );
}

function ExportCard({ title, description, icon, color, downloading, onDownload }: {
    title: string; description: string; icon: string; color: string; downloading: boolean; onDownload: () => void;
}) {
    const colorMap: Record<string, string> = {
        violet: "from-violet-600/10 to-violet-800/5 border-violet-500/20 hover:border-violet-500/40",
        blue: "from-blue-600/10 to-blue-800/5 border-blue-500/20 hover:border-blue-500/40",
        emerald: "from-emerald-600/10 to-emerald-800/5 border-emerald-500/20 hover:border-emerald-500/40",
        amber: "from-amber-600/10 to-amber-800/5 border-amber-500/20 hover:border-amber-500/40",
    };

    return (
        <div className={`bg-gradient-to-br ${colorMap[color]} border rounded-xl p-5 transition-all`}>
            <div className="flex items-start justify-between">
                <div>
                    <span className="text-2xl">{icon}</span>
                    <h3 className="text-sm font-semibold text-zinc-200 mt-2">{title}</h3>
                    <p className="text-xs text-zinc-500 mt-1">{description}</p>
                </div>
                <button onClick={onDownload} disabled={downloading}
                    className="px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 border border-zinc-700/50 rounded-lg text-sm text-zinc-300 transition-colors disabled:opacity-50 whitespace-nowrap">
                    {downloading ? (
                        <span className="flex items-center gap-1">
                            <span className="w-3 h-3 border border-zinc-400 border-t-transparent rounded-full animate-spin" />
                            Downloading...
                        </span>
                    ) : "⬇ Download"}
                </button>
            </div>
        </div>
    );
}
