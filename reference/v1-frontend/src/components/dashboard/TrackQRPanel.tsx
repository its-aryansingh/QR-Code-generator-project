import React from "react";
import { AnalyticsDashboard } from "./AnalyticsDashboard";

interface TrackQRPanelProps {
    isDynamicQR: boolean;
    setIsDynamicQR: (value: boolean) => void;
    startedTrackingCode: string | null;
    trackingQrId: string | null;
    handleGenerateDynamicQR: () => void;
    loading: boolean;
    content: string;
    vcardData: any;
    wifiData: any;
    analyticsData: any;
    dateRange: string;
    setDateRange: (value: string) => void;
    accessToken: string | null;
    API_BASE_URL: string;
}

export const TrackQRPanel: React.FC<TrackQRPanelProps> = ({
    isDynamicQR,
    setIsDynamicQR,
    startedTrackingCode,
    trackingQrId,
    handleGenerateDynamicQR,
    loading,
    content,
    vcardData,
    wifiData,
    analyticsData,
    dateRange,
    setDateRange,
    accessToken,
    API_BASE_URL
}) => {
    console.log("TrackQRPanel rendered. analyticsData:", analyticsData);
    return (
        <div className="space-y-6">
            {!isDynamicQR ? (
                <div className="text-center py-12">
                    <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-violet-500/20 to-fuchsia-500/20 flex items-center justify-center">
                        <svg className="w-10 h-10 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                        </svg>
                    </div>
                    <h3 className="text-xl font-semibold mb-2">Enable Tracking</h3>
                    <p className="text-slate-400 mb-6 max-w-sm mx-auto">
                        Turn on "Dynamic QR Code" above to track scans, locations, and devices in real-time.
                    </p>
                    <button
                        onClick={() => setIsDynamicQR(true)}
                        className="inline-flex items-center gap-2 px-6 py-3 bg-gradient-to-r from-violet-600 to-fuchsia-600 rounded-xl text-sm font-semibold hover:shadow-lg hover:shadow-violet-500/25 transition-all"
                    >
                        Enable Dynamic QR
                    </button>
                </div>
            ) : !startedTrackingCode ? (
                <div className="text-center py-10">
                    <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mx-auto mb-4">
                        <span className="text-3xl">📊</span>
                    </div>
                    <h3 className="text-lg font-medium text-white mb-2">Start Tracking</h3>
                    <p className="text-sm text-slate-400 max-w-xs mx-auto mb-6">
                        Generate a Dynamic QR code to see real-time scan analytics here.
                    </p>
                    <button
                        onClick={handleGenerateDynamicQR}
                        disabled={loading || (!content && !vcardData.name && !wifiData.ssid)}
                        className="px-6 py-2 bg-violet-600 hover:bg-violet-700 text-white rounded-lg transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {loading ? "Generating..." : "Generate Dynamic QR"}
                    </button>
                </div>
            ) : (
                <>
                    {/* Live Status */}
                    <div className="flex items-center justify-between p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20">
                        <div className="flex items-center gap-3">
                            <div className="relative">
                                <div className="w-3 h-3 rounded-full bg-emerald-400"></div>
                                <div className="absolute inset-0 w-3 h-3 rounded-full bg-emerald-400 animate-ping"></div>
                            </div>
                            <span className="text-sm text-emerald-300">Live Tracking Active</span>
                        </div>
                        <span className="text-xs text-slate-400">
                            Code: <span className="font-mono text-white">{startedTrackingCode}</span>
                        </span>
                    </div>

                    {/* Dashboard Layout */}
                    <AnalyticsDashboard
                        analyticsData={analyticsData}
                        dateRange={dateRange}
                        setDateRange={setDateRange}
                        accessToken={accessToken}
                        API_BASE_URL={API_BASE_URL}
                        qrId={trackingQrId}
                    />
                </>
            )}
        </div>
    );
};
