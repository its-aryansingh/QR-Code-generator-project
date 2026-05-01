import React from "react";
import axios from "axios";
import { toast } from "sonner";
import { AnalyticsChart } from "@/components/analytics/AnalyticsChart";
import { DeviceBreakdown } from "@/components/analytics/DeviceBreakdown";
import { LocationTable } from "@/components/analytics/LocationTable";
import { ReferrerTable } from "@/components/analytics/ReferrerTable";

interface AnalyticsDashboardProps {
    analyticsData: any;
    dateRange: string;
    setDateRange: (range: string) => void;
    accessToken: string | null;
    API_BASE_URL: string;
    qrId: string | null;
}

export const AnalyticsDashboard: React.FC<AnalyticsDashboardProps> = ({
    analyticsData,
    dateRange,
    setDateRange,
    accessToken,
    API_BASE_URL,
    qrId,
}) => {
    console.log("AnalyticsDashboard rendered. analyticsData:", analyticsData);
    console.log("AnalyticsDashboard scans:", analyticsData?.analytics?.total_scans);
    return (
        <div className="space-y-6">
            {/* Dashboard Header */}
            <div className="flex items-center justify-between mb-2">
                <div>
                    <h2 className="text-xl font-bold text-white">Analytics Overview</h2>
                    <p className="text-sm text-slate-400">Real-time performance metrics</p>
                </div>
                <div className="flex gap-2">
                    <select
                        value={dateRange}
                        onChange={(e) => setDateRange(e.target.value)}
                        className="bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-300 focus:outline-none focus:border-violet-500"
                    >
                        <option value="7">Last 7 Days</option>
                        <option value="30">Last 30 Days</option>
                        <option value="90">Last 90 Days</option>
                    </select>
                    <button
                        onClick={async () => {
                            if (!qrId) {
                                toast.error("Export unavailable", { description: "QR ID not found. Please refresh." });
                                return;
                            }

                            if (!accessToken) {
                                toast.error("Please login to export data", { description: "Export is a premium feature." });
                                return;
                            }

                            try {
                                const response = await axios.get(
                                    `${API_BASE_URL}/api/v1/qr/${qrId}/analytics/export`,
                                    {
                                        headers: { Authorization: `Bearer ${accessToken}` },
                                        responseType: 'blob',
                                    }
                                );

                                const url = window.URL.createObjectURL(new Blob([response.data]));
                                const link = document.createElement('a');
                                link.href = url;
                                link.setAttribute('download', `qr_analytics_${qrId}.csv`);
                                document.body.appendChild(link);
                                link.click();
                                link.remove();
                                toast.success("Analytics exported successfully");
                            } catch (error) {
                                console.error("Export failed:", error);
                                toast.error("Failed to export. ensure you are logged in.");
                            }
                        }}
                        className="flex items-center gap-2 px-4 py-2 bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 rounded-lg border border-indigo-500/20 transition-all text-sm font-medium"
                    >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                        Export CSV
                    </button>
                </div>
            </div>

            {/* Key Metrics Row */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 flex flex-col justify-between relative overflow-hidden group">
                    <div className="absolute inset-0 bg-gradient-to-br from-violet-600/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                    <div>
                        <p className="text-sm text-slate-400 font-medium">Total Scans</p>
                        <h3 className="text-3xl font-bold text-white mt-1">{analyticsData?.analytics?.total_scans || 0}</h3>
                    </div>
                    <div className="mt-4 flex items-center text-xs text-emerald-400">
                        <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" /></svg>
                        <span>All time</span>
                    </div>
                </div>

                <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 flex flex-col justify-between relative overflow-hidden group">
                    <div className="absolute inset-0 bg-gradient-to-br from-emerald-600/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                    <div>
                        <p className="text-sm text-slate-400 font-medium">Unique Users</p>
                        <h3 className="text-3xl font-bold text-white mt-1">{analyticsData?.analytics?.unique_scans || 0}</h3>
                    </div>
                    <div className="mt-4 flex items-center text-xs text-blue-400">
                        <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" /></svg>
                        <span>Distinct IPs</span>
                    </div>
                </div>

                <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 flex flex-col justify-between relative overflow-hidden group">
                    <div className="absolute inset-0 bg-gradient-to-br from-cyan-600/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                    <div>
                        <p className="text-sm text-slate-400 font-medium">Conversion Rate</p>
                        <h3 className="text-3xl font-bold text-white mt-1">
                            {analyticsData?.analytics?.total_scans > 0
                                ? Math.round((analyticsData.analytics.unique_scans / analyticsData.analytics.total_scans) * 100)
                                : 0}%
                        </h3>
                    </div>
                    <div className="mt-4 flex items-center text-xs text-slate-400">
                        <span>Unique / Total Ratio</span>
                    </div>
                </div>
            </div>

            {/* Charts Row */}
            <div className="grid grid-cols-1 gap-6">
                <AnalyticsChart data={analyticsData?.analytics?.scans_by_date || []} />
            </div>

            {/* Breakdown Row */}
            <DeviceBreakdown
                deviceData={analyticsData?.analytics?.scans_by_device}
                osData={analyticsData?.analytics?.scans_by_os}
                browserData={analyticsData?.analytics?.scans_by_browser}
            />

            {/* Location & Referrers Row */}
            <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 h-[400px]">
                <div className="xl:col-span-1">
                    <ReferrerTable referrerData={analyticsData?.analytics?.scans_by_referrer} />
                </div>
                <div className="xl:col-span-2">
                    <LocationTable
                        cityData={analyticsData?.analytics?.top_cities}
                        countryData={analyticsData?.analytics?.scans_by_country}
                    />
                </div>
            </div>

            {/* Recent Scans Feed */}
            <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 flex flex-col h-[400px]">
                <div className="flex items-center justify-between mb-4">
                    <h3 className="text-md font-semibold text-white flex items-center gap-2">
                        <span>Zap</span> Live Feed
                    </h3>
                    <div className="flex items-center gap-2 text-xs text-emerald-400 animate-pulse">
                        <div className="w-1.5 h-1.5 rounded-full bg-emerald-400"></div>
                        Real-time
                    </div>
                </div>

                <div className="flex-1 overflow-y-auto space-y-3 custom-scrollbar pr-2">
                    {analyticsData?.recent_scans && analyticsData.recent_scans.length > 0 ? (
                        analyticsData.recent_scans.map((scan: any, i: number) => (
                            <div key={i} className="flex items-center justify-between p-3 rounded-lg bg-white/5 border border-white/5 text-xs hover:bg-white/10 transition-colors">
                                <div className="flex items-center gap-3">
                                    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-slate-700 to-slate-800 flex items-center justify-center text-lg shadow-inner">
                                        {scan.device_type === "mobile" ? "📱" : (scan.device_type === "tablet" ? "iPad" : "💻")}
                                    </div>
                                    <div>
                                        <p className="text-white font-medium flex items-center gap-1.5">
                                            {scan.city ? `${scan.city}, ${scan.country_code}` : "Unknown Location"}
                                        </p>
                                        <p className="text-slate-500 font-mono text-[10px]">
                                            {scan.os} • {scan.browser}
                                        </p>
                                    </div>
                                </div>
                                <div className="text-right">
                                    <div className="text-slate-400 font-mono">
                                        {new Date(scan.scanned_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                    </div>
                                    <div className="text-[10px] text-slate-600">
                                        {new Date(scan.scanned_at).toLocaleDateString()}
                                    </div>
                                </div>
                            </div>
                        ))
                    ) : (
                        <div className="h-full flex flex-col items-center justify-center text-slate-500 gap-2">
                            <div className="w-12 h-12 rounded-full bg-white/5 flex items-center justify-center text-2xl">📡</div>
                            <p>Waiting for first scan...</p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};
