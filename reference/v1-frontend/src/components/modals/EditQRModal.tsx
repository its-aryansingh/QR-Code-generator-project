import React, { useState, useEffect } from "react";

interface EditQRModalProps {
    isOpen: boolean;
    onClose: () => void;
    qr: any;
    onUpdate: (data: { title: string; redirectURL?: string }) => void;
    loading?: boolean;
}

export const EditQRModal: React.FC<EditQRModalProps> = ({
    isOpen,
    onClose,
    qr,
    onUpdate,
    loading = false,
}) => {
    const [formData, setFormData] = useState({
        title: "",
        redirectURL: "",
    });

    useEffect(() => {
        if (qr) {
            setFormData({
                title: qr.title || "",
                redirectURL: qr.redirect_url || (qr.is_dynamic ? qr.content : ""),
            });
        }
    }, [qr]);

    if (!isOpen || !qr) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
            <div className="bg-[#1e1e2d] border border-white/10 rounded-2xl w-full max-w-md p-6 relative">
                <button
                    onClick={onClose}
                    className="absolute top-4 right-4 text-slate-400 hover:text-white"
                >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>

                <h3 className="text-xl font-bold text-white mb-6">Edit QR Code</h3>

                <div className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-slate-400 mb-1">Title</label>
                        <input
                            type="text"
                            value={formData.title}
                            onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                            className="w-full bg-black/20 border border-white/10 rounded-xl px-4 py-3 text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-violet-500/50"
                            placeholder="e.g., Marketing Campaign Q1"
                        />
                    </div>

                    {qr.is_dynamic && (
                        <div>
                            <label className="block text-sm font-medium text-slate-400 mb-1">Destination URL</label>
                            <input
                                type="url"
                                value={formData.redirectURL}
                                onChange={(e) => setFormData({ ...formData, redirectURL: e.target.value })}
                                className="w-full bg-black/20 border border-white/10 rounded-xl px-4 py-3 text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-violet-500/50"
                                placeholder="https://example.com/new-dest"
                            />
                            <p className="text-xs text-slate-500 mt-2">
                                Updates happen instantly. No need to reprint the QR code.
                            </p>
                        </div>
                    )}

                    {!qr.is_dynamic && (
                        <div className="p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-lg text-yellow-200 text-sm">
                            This is a static QR code. You can only edit the title. Content cannot be changed after generation.
                        </div>
                    )}

                    <div className="flex gap-3 mt-6">
                        <button
                            onClick={onClose}
                            className="flex-1 px-4 py-3 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 font-medium transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            onClick={() => onUpdate(formData)}
                            disabled={loading}
                            className="flex-1 px-4 py-3 rounded-xl bg-violet-600 hover:bg-violet-700 text-white font-medium transition-colors disabled:opacity-50"
                        >
                            {loading ? "Saving..." : "Save Changes"}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};
