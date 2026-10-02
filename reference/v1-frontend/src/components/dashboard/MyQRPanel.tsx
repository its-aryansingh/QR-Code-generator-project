import React from "react";
import { QRIcons } from "@/utils/qrConstants";

interface QRRecord {
    id: string;
    qr_type: string;
    title?: string;
    content: string;
    created_at: string;
    is_dynamic: boolean;
    scan_count: number;
    customization?: any;
}

interface MyQRPanelProps {
    myQRs: QRRecord[];
    loadingMyQRs: boolean;
    fetchMyQRs: () => void;
    openEditModal: (qr: QRRecord) => void;
    handleEditDesign: (qr: QRRecord) => void;
    handleDeleteQR: (id: string) => void;
    setActivePanel: (panel: "create" | "customize" | "track" | "myqrs") => void;
}

export const MyQRPanel: React.FC<MyQRPanelProps> = ({
    myQRs,
    loadingMyQRs,
    fetchMyQRs,
    openEditModal,
    handleEditDesign,
    handleDeleteQR,
    setActivePanel,
}) => {
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between mb-4">
                <div>
                    <h2 className="text-xl font-bold text-white">My QR Codes</h2>
                    <p className="text-sm text-slate-400">Manage your generated codes</p>
                </div>
                <button
                    onClick={fetchMyQRs}
                    className="p-2 rounded-lg bg-white/5 hover:bg-white/10 text-slate-400 hover:text-white transition-colors"
                    title="Refresh"
                >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                </button>
            </div>

            {loadingMyQRs ? (
                <div className="flex items-center justify-center h-64">
                    <div className="w-8 h-8 border-4 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
                </div>
            ) : myQRs.length === 0 ? (
                <div className="text-center py-12 rounded-2xl bg-white/5 border border-white/10 border-dashed">
                    <div className="w-16 h-16 bg-white/5 rounded-full flex items-center justify-center mx-auto mb-4">
                        <svg className="w-8 h-8 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                        </svg>
                    </div>
                    <h3 className="text-lg font-medium text-white mb-2">No QR Codes Yet</h3>
                    <p className="text-slate-400 mb-6 max-w-xs mx-auto">
                        Generate your first QR code to see it here.
                    </p>
                    <button
                        onClick={() => setActivePanel("create")}
                        className="px-6 py-2 bg-violet-600 hover:bg-violet-700 text-white rounded-lg transition-colors font-medium"
                    >
                        Create QR Code
                    </button>
                </div>
            ) : (
                <div className="grid gap-4">
                    {myQRs.map((qr) => (
                        <div key={qr.id} className="p-4 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 transition-all flex items-center justify-between group">
                            <div className="flex items-center gap-4">
                                <div className="w-12 h-12 bg-white/10 rounded-lg p-2 text-white flex items-center justify-center">
                                    {QRIcons[qr.qr_type] || QRIcons["url"]}
                                </div>
                                <div>
                                    <h4 className="text-white font-medium truncate max-w-[200px]">{qr.title || qr.content || "Untitled QR"}</h4>
                                    <div className="flex items-center gap-2 text-xs text-slate-400 mt-1">
                                        <span className="uppercase bg-white/10 px-1.5 py-0.5 rounded text-[10px]">{qr.qr_type}</span>
                                        <span>•</span>
                                        <span>{new Date(qr.created_at).toLocaleDateString()}</span>
                                        {qr.is_dynamic && <span className="text-emerald-400 bg-emerald-400/10 px-1.5 py-0.5 rounded text-[10px]">Dynamic</span>}
                                    </div>
                                </div>
                            </div>

                            <div className="flex items-center gap-3">
                                <div className="text-right mr-2 hidden sm:block">
                                    <div className="text-white font-bold">{qr.scan_count || 0}</div>
                                    <div className="text-[10px] text-slate-500 uppercase">Scans</div>
                                </div>
                                <button
                                    onClick={() => openEditModal(qr)}
                                    className="p-2 rounded-lg hover:bg-violet-500/20 text-slate-400 hover:text-violet-400 transition-colors"
                                    title="Edit"
                                >
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                    </svg>
                                </button>
                                <button
                                    onClick={() => handleEditDesign(qr)}
                                    className="p-2 rounded-lg hover:bg-violet-500/20 text-slate-400 hover:text-violet-400 transition-colors"
                                    title="Edit Design"
                                >
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                                    </svg>
                                </button>
                                <button
                                    onClick={() => handleDeleteQR(qr.id)}
                                    className="p-2 rounded-lg hover:bg-red-500/20 text-slate-400 hover:text-red-400 transition-colors"
                                    title="Delete"
                                >
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                    </svg>
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};
