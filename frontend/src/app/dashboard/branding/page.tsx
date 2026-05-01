"use client";

import { useEffect, useState } from "react";
import { useAuthStore } from "@/lib/auth";

export default function BrandingPage() {
    const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";
    const [workspaceId, setWorkspaceId] = useState("");
    const [branding, setBranding] = useState({
        custom_domain: "",
        brand_color: "#8B5CF6",
        brand_logo: "",
        favicon_url: "",
        custom_css: "",
        remove_branding: false,
        custom_footer: "",
    });
    const [sso, setSSO] = useState({ sso_enabled: false, sso_provider: "", sso_config: {} as Record<string, string> });
    const [saving, setSaving] = useState(false);
    const [msg, setMsg] = useState("");

    useEffect(() => { loadData(); }, []);

    const loadData = async () => {
        const token = useAuthStore.getState().accessToken;
        let id = localStorage.getItem("qrit_active_workspace") || "";
        if (!id) {
            const wsRes = await fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${token}` } });
            const wsData = await wsRes.json();
            if (wsData.success && wsData.data?.length > 0) {
                id = wsData.data[0].id;
                localStorage.setItem("qrit_active_workspace", id);
            }
        }
        if (id) {
            setWorkspaceId(id);
            const [brandRes, ssoRes] = await Promise.all([
                fetch(`${api}/workspaces/${id}/branding`, { headers: { Authorization: `Bearer ${token}` } }),
                fetch(`${api}/workspaces/${id}/sso`, { headers: { Authorization: `Bearer ${token}` } }),
            ]);
            const brandData = await brandRes.json();
            const ssoData = await ssoRes.json();
            if (brandData.success) setBranding(brandData.data);
            if (ssoData.success) setSSO(ssoData.data);
        }
    };

    const saveBranding = async () => {
        setSaving(true);
        const token = useAuthStore.getState().accessToken;
        await fetch(`${api}/workspaces/${workspaceId}/branding`, {
            method: "PUT", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
            body: JSON.stringify(branding),
        });
        setMsg("Branding saved!"); setSaving(false);
        setTimeout(() => setMsg(""), 3000);
    };

    const saveSSOConfig = async () => {
        setSaving(true);
        const token = useAuthStore.getState().accessToken;
        await fetch(`${api}/workspaces/${workspaceId}/sso`, {
            method: "PUT", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
            body: JSON.stringify({ enabled: sso.sso_enabled, provider: sso.sso_provider, config: sso.sso_config }),
        });
        setMsg("SSO configuration saved!"); setSaving(false);
        setTimeout(() => setMsg(""), 3000);
    };

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-2xl font-bold text-zinc-100">White-Labeling & SSO</h1>
                <p className="text-sm text-zinc-500 mt-1">Customize branding and configure single sign-on</p>
            </div>

            {msg && (
                <div className="bg-emerald-500/10 border border-emerald-500/30 rounded-lg px-4 py-2 text-emerald-400 text-sm">
                    ✅ {msg}
                </div>
            )}

            {/* Branding Section */}
            <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6 space-y-5">
                <h2 className="text-lg font-semibold text-zinc-200 flex items-center gap-2">🎨 Brand Customization</h2>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                        <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Custom Domain</label>
                        <input value={branding.custom_domain} onChange={e => setBranding({ ...branding, custom_domain: e.target.value })}
                            placeholder="qr.yourdomain.com" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                    </div>
                    <div>
                        <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Brand Color</label>
                        <div className="flex gap-2 items-center">
                            <input type="color" value={branding.brand_color} onChange={e => setBranding({ ...branding, brand_color: e.target.value })}
                                className="w-10 h-10 rounded border border-zinc-700 cursor-pointer" />
                            <input value={branding.brand_color} onChange={e => setBranding({ ...branding, brand_color: e.target.value })}
                                className="flex-1 bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                        </div>
                    </div>
                    <div>
                        <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Brand Logo URL</label>
                        <input value={branding.brand_logo} onChange={e => setBranding({ ...branding, brand_logo: e.target.value })}
                            placeholder="https://..." className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                    </div>
                    <div>
                        <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Favicon URL</label>
                        <input value={branding.favicon_url} onChange={e => setBranding({ ...branding, favicon_url: e.target.value })}
                            placeholder="https://..." className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                    </div>
                </div>

                <div>
                    <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Custom Footer Text</label>
                    <input value={branding.custom_footer} onChange={e => setBranding({ ...branding, custom_footer: e.target.value })}
                        placeholder="© 2026 Your Company" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                </div>

                <div>
                    <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Custom CSS Override</label>
                    <textarea value={branding.custom_css} onChange={e => setBranding({ ...branding, custom_css: e.target.value })}
                        rows={4} placeholder=":root { --primary: #8B5CF6; }" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 font-mono focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                </div>

                <div className="flex items-center gap-3">
                    <button onClick={() => setBranding({ ...branding, remove_branding: !branding.remove_branding })}
                        className={`w-10 h-5 rounded-full transition-colors ${branding.remove_branding ? "bg-violet-600" : "bg-zinc-700"}`}>
                        <div className={`w-4 h-4 bg-white rounded-full transition-transform mx-0.5 ${branding.remove_branding ? "translate-x-5" : ""}`} />
                    </button>
                    <span className="text-sm text-zinc-300">Remove &quot;Powered by QRit&quot; branding</span>
                </div>

                {/* Live Preview */}
                <div className="border-t border-zinc-800/60 pt-4">
                    <label className="block text-xs text-zinc-500 mb-3 uppercase tracking-wider">Live Preview</label>
                    <div className="bg-zinc-950 rounded-lg p-6 border border-zinc-800/40" style={{ borderTopColor: branding.brand_color, borderTopWidth: "3px" }}>
                        <div className="flex items-center gap-3 mb-4">
                            {branding.brand_logo ? (
                                <img src={branding.brand_logo} alt="Logo" className="h-8 w-8 rounded object-cover" />
                            ) : (
                                <div className="h-8 w-8 rounded flex items-center justify-center text-white font-bold text-sm" style={{ backgroundColor: branding.brand_color }}>Q</div>
                            )}
                            <span className="text-zinc-200 font-semibold">Your QR Experience</span>
                        </div>
                        <div className="h-24 bg-zinc-900 rounded-lg flex items-center justify-center">
                            <span className="text-zinc-600 text-sm">QR Code Content Area</span>
                        </div>
                        {!branding.remove_branding && (
                            <p className="text-center text-zinc-600 text-xs mt-3">Powered by QRit</p>
                        )}
                        {branding.custom_footer && (
                            <p className="text-center text-zinc-500 text-xs mt-2">{branding.custom_footer}</p>
                        )}
                    </div>
                </div>

                <button onClick={saveBranding} disabled={saving}
                    className="px-5 py-2 bg-violet-600 hover:bg-violet-500 rounded-lg text-sm font-medium text-white transition-colors disabled:opacity-50">
                    {saving ? "Saving..." : "Save Branding"}
                </button>
            </div>

            {/* SSO Section */}
            <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-6 space-y-5">
                <h2 className="text-lg font-semibold text-zinc-200 flex items-center gap-2">🔐 Single Sign-On (SSO)</h2>

                <div className="flex items-center gap-3">
                    <button onClick={() => setSSO({ ...sso, sso_enabled: !sso.sso_enabled })}
                        className={`w-10 h-5 rounded-full transition-colors ${sso.sso_enabled ? "bg-emerald-600" : "bg-zinc-700"}`}>
                        <div className={`w-4 h-4 bg-white rounded-full transition-transform mx-0.5 ${sso.sso_enabled ? "translate-x-5" : ""}`} />
                    </button>
                    <span className="text-sm text-zinc-300">Enable SSO for workspace</span>
                </div>

                {sso.sso_enabled && (
                    <div className="space-y-4 animate-in slide-in-from-top-2">
                        <div>
                            <label className="block text-xs text-zinc-500 mb-1 uppercase tracking-wider">Provider</label>
                            <div className="flex gap-3">
                                {["saml", "oidc"].map((p) => (
                                    <button key={p} onClick={() => setSSO({ ...sso, sso_provider: p })}
                                        className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${sso.sso_provider === p ? "bg-violet-600 text-white" : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                                            }`}>
                                        {p.toUpperCase()}
                                    </button>
                                ))}
                            </div>
                        </div>

                        {sso.sso_provider === "saml" && (
                            <div className="space-y-3">
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">SSO URL (IdP Login URL)</label>
                                    <input value={sso.sso_config?.sso_url || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, sso_url: e.target.value } })}
                                        placeholder="https://idp.example.com/sso/saml" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">Entity ID</label>
                                    <input value={sso.sso_config?.entity_id || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, entity_id: e.target.value } })}
                                        placeholder="https://idp.example.com/entity" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">X.509 Certificate</label>
                                    <textarea value={sso.sso_config?.certificate || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, certificate: e.target.value } })}
                                        rows={3} placeholder="-----BEGIN CERTIFICATE-----" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 font-mono focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                            </div>
                        )}

                        {sso.sso_provider === "oidc" && (
                            <div className="space-y-3">
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">Issuer URL</label>
                                    <input value={sso.sso_config?.issuer || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, issuer: e.target.value } })}
                                        placeholder="https://accounts.google.com" className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">Client ID</label>
                                    <input value={sso.sso_config?.client_id || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, client_id: e.target.value } })}
                                        className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1">Client Secret</label>
                                    <input type="password" value={sso.sso_config?.client_secret || ""} onChange={e => setSSO({ ...sso, sso_config: { ...sso.sso_config, client_secret: e.target.value } })}
                                        className="w-full bg-zinc-800/60 border border-zinc-700/50 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-violet-500/50" />
                                </div>
                            </div>
                        )}

                        <button onClick={saveSSOConfig} disabled={saving}
                            className="px-5 py-2 bg-emerald-600 hover:bg-emerald-500 rounded-lg text-sm font-medium text-white transition-colors disabled:opacity-50">
                            {saving ? "Saving..." : "Save SSO Configuration"}
                        </button>
                    </div>
                )}
            </div>

            {/* Info Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-4">
                    <h3 className="text-sm font-semibold text-zinc-300 mb-2">🌐 Custom Domains</h3>
                    <p className="text-xs text-zinc-500 leading-relaxed">
                        Point your CNAME record to <code className="text-violet-400">qr.qrit.app</code> and enter your domain above.
                        SSL certificates are provisioned automatically.
                    </p>
                </div>
                <div className="bg-zinc-900/60 border border-zinc-800/60 rounded-xl p-4">
                    <h3 className="text-sm font-semibold text-zinc-300 mb-2">🔒 SSO Providers</h3>
                    <p className="text-xs text-zinc-500 leading-relaxed">
                        We support SAML 2.0 and OpenID Connect. Compatible with Okta, Azure AD, Google Workspace, OneLogin, and more.
                    </p>
                </div>
            </div>
        </div>
    );
}
