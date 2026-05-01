"use client";

import { useEffect, useState, useCallback } from "react";
import { useAuthStore } from "@/lib/auth";
import { ScanHeatmap } from "@/components/dashboard/ScanHeatmap";
import { Download, FileText } from "lucide-react";

interface AnalyticsData {
    summary: { total_qr_codes: number; total_scans: number };
    scans_by_date: { date: string; count: number }[];
    devices: { name: string; value: number }[];
    countries: { name: string; value: number }[];
    browsers: { name: string; value: number }[];
    referrers: { name: string; value: number }[];
    top_qr_codes: { id: string; title: string; scan_count: number; qr_type: string; content: string }[];
}

const COLORS = ["#8B5CF6", "#EC4899", "#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#6366F1", "#14B8A6"];

export default function AnalyticsDashboardPage() {
    const [data, setData] = useState<AnalyticsData | null>(null);
    const [days, setDays] = useState(30);
    const [loading, setLoading] = useState(true);
    const [workspaceId, setWorkspaceId] = useState("");
    const [exporting, setExporting] = useState(false);
    const [exportingPDF, setExportingPDF] = useState(false);
    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

    const loadAnalytics = useCallback(async () => {
        setLoading(true);
        const token = useAuthStore.getState().accessToken;
        try {
            const stored = localStorage.getItem("qrit_active_workspace");
            let wsId = stored;
            if (!wsId) {
                const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
                const wsData = await wsRes.json();
                if (wsData.success && wsData.data?.length > 0) wsId = wsData.data[0].id;
            }
            if (!wsId) return;
            setWorkspaceId(wsId);
            const res = await fetch(`${api}/workspaces/${wsId}/analytics?days=${days}`, {
                headers: { Authorization: `Bearer ${token}` },
            });
            const result = await res.json();
            if (result.success) setData(result.data);
        } catch {}
        finally { setLoading(false); }
    }, [days, api]);

    useEffect(() => { loadAnalytics(); }, [loadAnalytics]);

    const handleExportCSV = async () => {
        if (!workspaceId || exporting) return;
        setExporting(true);
        const token = useAuthStore.getState().accessToken;
        try {
            const res = await fetch(`${api}/workspaces/${workspaceId}/export/analytics?days=${days}`, {
                headers: { Authorization: `Bearer ${token}` },
            });
            const blob = await res.blob();
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = `analytics-${days}d.csv`;
            a.click();
            URL.revokeObjectURL(url);
        } catch {}
        finally { setExporting(false); }
    };

    const handleExportPDF = async () => {
        if (!data || exportingPDF) return;
        setExportingPDF(true);
        try {
            const { jsPDF } = await import("jspdf");
            const doc = new jsPDF({ orientation: "portrait", unit: "mm", format: "a4" });
            const pageW = doc.internal.pageSize.getWidth();
            const margin = 15;
            let y = margin;

            const addText = (text: string, x: number, fontSize: number, style: "normal" | "bold" = "normal", color: [number, number, number] = [220, 220, 220]) => {
                doc.setFontSize(fontSize);
                doc.setFont("helvetica", style);
                doc.setTextColor(...color);
                doc.text(text, x, y);
            };

            // Dark background header
            doc.setFillColor(15, 15, 20);
            doc.rect(0, 0, pageW, 297, "F");

            // Title
            addText("Analytics Report", margin, 22, "bold", [167, 139, 250]);
            y += 8;
            addText(`Last ${days} days  •  Generated ${new Date().toLocaleDateString()}`, margin, 9, "normal", [113, 113, 122]);
            y += 14;

            // Divider
            doc.setDrawColor(63, 63, 70);
            doc.line(margin, y, pageW - margin, y);
            y += 10;

            // Summary
            addText("SUMMARY", margin, 8, "bold", [113, 113, 122]);
            y += 7;
            const summaryItems = [
                ["Total QR Codes", data.summary.total_qr_codes.toLocaleString()],
                ["Total Scans", data.summary.total_scans.toLocaleString()],
                ["Countries Reached", String(data.countries?.length || 0)],
            ];
            const colW = (pageW - margin * 2) / 3;
            summaryItems.forEach(([label, val], i) => {
                const cx = margin + i * colW;
                doc.setFontSize(18);
                doc.setFont("helvetica", "bold");
                doc.setTextColor(245, 245, 245);
                doc.text(val, cx, y + 7);
                doc.setFontSize(8);
                doc.setFont("helvetica", "normal");
                doc.setTextColor(113, 113, 122);
                doc.text(label, cx, y + 14);
            });
            y += 24;

            doc.setDrawColor(63, 63, 70);
            doc.line(margin, y, pageW - margin, y);
            y += 10;

            // Top QR Codes
            if (data.top_qr_codes?.length > 0) {
                addText("TOP QR CODES", margin, 8, "bold", [113, 113, 122]);
                y += 7;
                data.top_qr_codes.slice(0, 10).forEach((qr, i) => {
                    doc.setFontSize(9);
                    doc.setFont("helvetica", "bold");
                    doc.setTextColor(245, 245, 245);
                    doc.text(`${i + 1}.`, margin, y);
                    doc.text(doc.splitTextToSize(qr.title || qr.content, 110)[0], margin + 10, y);
                    doc.setFont("helvetica", "normal");
                    doc.setTextColor(167, 139, 250);
                    doc.text(`${qr.scan_count.toLocaleString()} scans`, pageW - margin - 30, y);
                    doc.setTextColor(113, 113, 122);
                    doc.text(qr.qr_type, pageW - margin - 5, y, { align: "right" });
                    y += 7;
                    if (y > 270) { doc.addPage(); doc.setFillColor(15, 15, 20); doc.rect(0, 0, pageW, 297, "F"); y = margin; }
                });
                y += 5;
            }

            // Devices
            if (data.devices?.length > 0) {
                doc.setDrawColor(63, 63, 70);
                doc.line(margin, y, pageW - margin, y);
                y += 10;
                addText("DEVICES", margin, 8, "bold", [113, 113, 122]);
                y += 7;
                const total = data.devices.reduce((s, d) => s + d.value, 0);
                data.devices.slice(0, 6).forEach((d) => {
                    const pct = ((d.value / total) * 100).toFixed(1);
                    doc.setFontSize(9);
                    doc.setFont("helvetica", "normal");
                    doc.setTextColor(220, 220, 220);
                    doc.text(d.name, margin, y);
                    doc.setTextColor(113, 113, 122);
                    doc.text(`${d.value} (${pct}%)`, pageW - margin, y, { align: "right" });
                    y += 6;
                });
                y += 5;
            }

            // Countries
            if (data.countries?.length > 0) {
                if (y > 240) { doc.addPage(); doc.setFillColor(15, 15, 20); doc.rect(0, 0, pageW, 297, "F"); y = margin; }
                doc.setDrawColor(63, 63, 70);
                doc.line(margin, y, pageW - margin, y);
                y += 10;
                addText("COUNTRIES", margin, 8, "bold", [113, 113, 122]);
                y += 7;
                const total = data.countries.reduce((s, d) => s + d.value, 0);
                data.countries.slice(0, 10).forEach((d) => {
                    const pct = ((d.value / total) * 100).toFixed(1);
                    doc.setFontSize(9);
                    doc.setFont("helvetica", "normal");
                    doc.setTextColor(220, 220, 220);
                    doc.text(d.name, margin, y);
                    doc.setTextColor(113, 113, 122);
                    doc.text(`${d.value} (${pct}%)`, pageW - margin, y, { align: "right" });
                    y += 6;
                });
            }

            doc.save(`analytics-${days}d.pdf`);
        } catch (e) {
            console.error("PDF export failed:", e);
        } finally {
            setExportingPDF(false);
        }
    };

    const maxScan = data?.scans_by_date?.reduce((m, d) => Math.max(m, d.count), 0) || 1;
    const totalDevices = data?.devices?.reduce((s, d) => s + d.value, 0) || 1;
    const totalCountries = data?.countries?.reduce((s, d) => s + d.value, 0) || 1;
    const totalBrowsers = data?.browsers?.reduce((s, d) => s + d.value, 0) || 1;

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold text-zinc-100">Analytics</h1>
                    <p className="text-sm text-zinc-500 mt-1">Workspace scan analytics and performance metrics</p>
                </div>
                <div className="flex items-center gap-2">
                    {[7, 14, 30, 90].map((d) => (
                        <button
                            key={d}
                            onClick={() => setDays(d)}
                            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-all ${days === d
                                ? "bg-violet-600 text-white"
                                : "bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
                                }`}
                        >
                            {d}d
                        </button>
                    ))}
                    <button
                        onClick={handleExportCSV}
                        disabled={exporting || !workspaceId}
                        className="ml-2 flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-zinc-800 text-zinc-300 hover:bg-zinc-700 disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        <Download size={14} />
                        {exporting ? "Exporting…" : "CSV"}
                    </button>
                    <button
                        onClick={handleExportPDF}
                        disabled={exportingPDF || !data}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-zinc-800 text-zinc-300 hover:bg-zinc-700 disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        <FileText size={14} />
                        {exportingPDF ? "Generating…" : "PDF"}
                    </button>
                </div>
            </div>

            {loading ? (
                <div className="flex items-center justify-center py-24">
                    <div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
                </div>
            ) : data ? (
                <>
                    {/* Summary Cards */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                        <SummaryCard label="Total QR Codes" value={data.summary.total_qr_codes} icon="📱" color="violet" />
                        <SummaryCard label="Total Scans" value={data.summary.total_scans} icon="📊" color="blue" />
                        <SummaryCard label="Countries Reached" value={data.countries?.length || 0} icon="🌍" color="emerald" />
                        <SummaryCard label="Top Performers" value={data.top_qr_codes?.length || 0} icon="🏆" color="amber" />
                    </div>

                    {/* Scans Over Time Chart */}
                    <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
                        <h3 className="text-sm font-semibold text-zinc-300 mb-4">📈 Scans Over Time ({days} days)</h3>
                        {data.scans_by_date?.length > 0 ? (
                            <div className="flex items-end gap-1 h-48">
                                {data.scans_by_date.map((d, i) => (
                                    <div key={i} className="flex-1 flex flex-col items-center justify-end group relative">
                                        <div
                                            className="w-full bg-gradient-to-t from-violet-600 to-violet-400 rounded-t-sm transition-all hover:from-violet-500 hover:to-violet-300 min-h-[2px]"
                                            style={{ height: `${(d.count / maxScan) * 100}%` }}
                                        />
                                        <div className="absolute -top-8 bg-zinc-800 text-zinc-200 text-xs px-2 py-1 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-10">
                                            {d.count} scans • {new Date(d.date).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                                        </div>
                                        {i % Math.max(1, Math.floor(data.scans_by_date.length / 7)) === 0 && (
                                            <span className="text-[10px] text-zinc-600 mt-1">
                                                {new Date(d.date).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                                            </span>
                                        )}
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <EmptyState text="No scan data for this period" />
                        )}
                    </div>

                    {/* Scan Heatmap */}
                    {workspaceId && (
                        <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
                            <h3 className="text-sm font-semibold text-zinc-300 mb-4">🗓️ Scan Activity Heatmap</h3>
                            <ScanHeatmap workspaceId={workspaceId} />
                        </div>
                    )}

                    {/* Breakdown Charts Row */}
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        {/* Device Breakdown */}
                        <BreakdownCard title="🖥️ Devices" items={data.devices} total={totalDevices} />
                        {/* Country Breakdown */}
                        <BreakdownCard title="🌍 Countries" items={data.countries} total={totalCountries} />
                        {/* Browser Breakdown */}
                        <BreakdownCard title="🌐 Browsers" items={data.browsers} total={totalBrowsers} />
                    </div>

                    {/* Referrer Sources */}
                    {data.referrers?.length > 0 && (
                        <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
                            <h3 className="text-sm font-semibold text-zinc-300 mb-4">🔗 Traffic Sources</h3>
                            <div className="space-y-3">
                                {data.referrers.map((r, i) => {
                                    const totalRef = data.referrers.reduce((s, d) => s + d.value, 0);
                                    const pct = ((r.value / totalRef) * 100).toFixed(1);
                                    return (
                                        <div key={i} className="flex items-center gap-3">
                                            <span className="text-sm text-zinc-300 w-48 truncate">{r.name}</span>
                                            <div className="flex-1 h-2 bg-zinc-800 rounded-full overflow-hidden">
                                                <div
                                                    className="h-full rounded-full"
                                                    style={{
                                                        width: `${pct}%`,
                                                        backgroundColor: COLORS[i % COLORS.length],
                                                    }}
                                                />
                                            </div>
                                            <span className="text-xs text-zinc-500 w-20 text-right">
                                                {r.value} ({pct}%)
                                            </span>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    )}

                    {/* Top Performing QR Codes */}
                    <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
                        <h3 className="text-sm font-semibold text-zinc-300 mb-4">🏆 Top Performing QR Codes</h3>
                        {data.top_qr_codes?.length > 0 ? (
                            <div className="space-y-3">
                                {data.top_qr_codes.map((qr, i) => (
                                    <div key={qr.id} className="flex items-center gap-4 px-3 py-2 rounded-lg hover:bg-zinc-800/40 transition-colors">
                                        <span className="text-lg font-bold text-zinc-600 w-8 text-center">#{i + 1}</span>
                                        <div className="flex-1 min-w-0">
                                            <p className="text-sm text-zinc-200 font-medium truncate">{qr.title || qr.content}</p>
                                            <p className="text-xs text-zinc-500">{qr.qr_type}</p>
                                        </div>
                                        <div className="text-right">
                                            <p className="text-sm font-semibold text-violet-400">{qr.scan_count.toLocaleString()}</p>
                                            <p className="text-xs text-zinc-600">scans</p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <EmptyState text="No scans recorded yet" />
                        )}
                    </div>
                </>
            ) : (
                <EmptyState text="Create a workspace to view analytics" />
            )}
        </div>
    );
}

function SummaryCard({ label, value, icon, color }: { label: string; value: number; icon: string; color: string }) {
    const colorMap: Record<string, string> = {
        violet: "from-violet-600/20 to-violet-700/10 border-violet-500/20",
        blue: "from-blue-600/20 to-blue-700/10 border-blue-500/20",
        emerald: "from-emerald-600/20 to-emerald-700/10 border-emerald-500/20",
        amber: "from-amber-600/20 to-amber-700/10 border-amber-500/20",
    };

    return (
        <div className={`bg-gradient-to-br ${colorMap[color]} border rounded-xl p-4`}>
            <div className="flex items-center gap-2 mb-2">
                <span className="text-lg">{icon}</span>
                <span className="text-xs text-zinc-500 uppercase tracking-wider">{label}</span>
            </div>
            <p className="text-2xl font-bold text-zinc-100">{value.toLocaleString()}</p>
        </div>
    );
}

function BreakdownCard({ title, items, total }: { title: string; items: { name: string; value: number }[]; total: number }) {
    return (
        <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6">
            <h3 className="text-sm font-semibold text-zinc-300 mb-4">{title}</h3>
            {items?.length > 0 ? (
                <div className="space-y-3">
                    {items.slice(0, 6).map((item, i) => {
                        const pct = ((item.value / total) * 100).toFixed(1);
                        return (
                            <div key={i} className="flex items-center gap-2">
                                <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: COLORS[i % COLORS.length] }} />
                                <span className="text-sm text-zinc-300 flex-1 truncate">{item.name}</span>
                                <span className="text-xs text-zinc-500">{pct}%</span>
                            </div>
                        );
                    })}
                    {/* Donut visualization */}
                    <div className="flex justify-center pt-2">
                        <svg width="100" height="100" viewBox="0 0 100 100">
                            {items.slice(0, 6).reduce(
                                (acc, item, i) => {
                                    const pct = (item.value / total) * 100;
                                    const start = acc.offset;
                                    acc.offset += pct;
                                    acc.elements.push(
                                        <circle
                                            key={i}
                                            cx="50"
                                            cy="50"
                                            r="40"
                                            fill="none"
                                            stroke={COLORS[i % COLORS.length]}
                                            strokeWidth="12"
                                            strokeDasharray={`${pct * 2.51} ${251 - pct * 2.51}`}
                                            strokeDashoffset={-start * 2.51}
                                            className="transition-all duration-500"
                                        />
                                    );
                                    return acc;
                                },
                                { offset: 0, elements: [] as React.ReactNode[] }
                            ).elements}
                            <text x="50" y="52" textAnchor="middle" className="fill-zinc-300 text-xs font-bold" fontSize="14">
                                {total}
                            </text>
                        </svg>
                    </div>
                </div>
            ) : (
                <EmptyState text="No data yet" />
            )}
        </div>
    );
}

function EmptyState({ text }: { text: string }) {
    return (
        <div className="text-center py-12">
            <p className="text-zinc-600">{text}</p>
        </div>
    );
}
