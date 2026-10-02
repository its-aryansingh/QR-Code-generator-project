import React from "react";

interface CustomizeQRPanelProps {
    designTab: string;
    setDesignTab: (tab: any) => void;
    selectedDotStyle: string;
    setSelectedDotStyle: (style: string) => void;
    selectedCornerStyle: string;
    setSelectedCornerStyle: (style: string) => void;
    selectedFrame: string;
    setSelectedFrame: (frame: string) => void;
    eyeInnerStyle: string;
    setEyeInnerStyle: (style: string) => void;
    quietZone: number;
    setQuietZone: (zone: number) => void;
    errorLevel: string;
    setErrorLevel: (level: string) => void;
    qrColor: string;
    setQrColor: (color: string) => void;
    bgColor: string;
    setBgColor: (color: string) => void;
    useGradient: boolean;
    setUseGradient: (use: boolean) => void;
    gradientStart: string;
    setGradientStart: (color: string) => void;
    gradientEnd: string;
    setGradientEnd: (color: string) => void;
    gradientType: "linear" | "radial";
    setGradientType: (type: "linear" | "radial") => void;
    gradientRotation: number;
    setGradientRotation: (rotation: number) => void;
    transparentBg: boolean;
    setTransparentBg: (transparent: boolean) => void;
    logoFile: string | null;
    setLogoFile: (file: string | null) => void;
    logoSize: number;
    setLogoSize: (size: number) => void;
    logoMargin: number;
    setLogoMargin: (margin: number) => void;
    bgImage: string | null;
    setBgImage: (image: string | null) => void;
}

export const CustomizeQRPanel: React.FC<CustomizeQRPanelProps> = ({
    designTab,
    setDesignTab,
    selectedDotStyle,
    setSelectedDotStyle,
    selectedCornerStyle,
    setSelectedCornerStyle,
    selectedFrame,
    setSelectedFrame,
    eyeInnerStyle,
    setEyeInnerStyle,
    quietZone,
    setQuietZone,
    errorLevel,
    setErrorLevel,
    qrColor,
    setQrColor,
    bgColor,
    setBgColor,
    useGradient,
    setUseGradient,
    gradientStart,
    setGradientStart,
    gradientEnd,
    setGradientEnd,
    gradientType,
    setGradientType,
    gradientRotation,
    setGradientRotation,
    transparentBg,
    setTransparentBg,
    logoFile,
    setLogoFile,
    logoSize,
    setLogoSize,
    logoMargin,
    setLogoMargin,
    bgImage,
    setBgImage,
}) => {
    return (
        <div className="space-y-5">
            {/* Design Tabs */}
            <div className="flex gap-1 p-1 rounded-lg bg-white/5">
                {[
                    { id: "templates", label: "Templates", icon: "✨" },
                    { id: "shape", label: "Shape", icon: "⬛" },
                    { id: "color", label: "Color", icon: "🎨" },
                    { id: "logo", label: "Logo", icon: "🏷️" },
                    { id: "background", label: "BG", icon: "🖼️" },
                ].map((tab) => (
                    <button
                        key={tab.id}
                        onClick={() => setDesignTab(tab.id)}
                        className={`flex-1 flex items-center justify-center gap-2 px-3 py-2.5 rounded-md text-xs font-medium transition-all ${designTab === tab.id
                            ? "bg-gradient-to-r from-violet-600 to-fuchsia-600 text-white"
                            : "text-zinc-500 hover:text-white hover:bg-white/5"
                            }`}
                    >
                        <span>{tab.icon}</span>
                        <span>{tab.label}</span>
                    </button>
                ))}
            </div>

            {/* Templates Panel */}
            {designTab === "templates" && (
                <div className="space-y-4">
                    <p className="text-sm text-zinc-500">Choose a pre-designed style</p>

                    {/* Classic */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Classic</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "classic", name: "B&W", dot: "square", corner: "square", color: "#000000", bg: "#FFFFFF", gradient: null },
                                { id: "mono-blue", name: "Mono Blue", dot: "square", corner: "square", color: "#1E3A5F", bg: "#F0F4F8", gradient: null },
                                { id: "dark-mode", name: "Dark Mode", dot: "square", corner: "square", color: "#E2E8F0", bg: "#0F172A", gradient: null },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg border border-white/20" style={{ backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Modern */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Modern</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "rounded-dots", name: "Rounded", dot: "dots", corner: "extra-rounded", color: "#2563EB", bg: "#FFFFFF", gradient: null },
                                { id: "gradient-pop", name: "Gradient", dot: "rounded", corner: "dot", color: "#8B5CF6", bg: "#FFFFFF", gradient: ["#4F46E5", "#EC4899"] },
                                { id: "diamond", name: "Diamond", dot: "diamond", corner: "square", color: "#0F172A", bg: "#F8FAFC", gradient: null },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg" style={tpl.gradient ? { background: `linear-gradient(135deg, ${tpl.gradient[0]}, ${tpl.gradient[1]})` } : { backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Nature */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Nature</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "forest", name: "Forest", dot: "classy", corner: "extra-rounded", color: "#166534", bg: "#F0FDF4", gradient: ["#166534", "#4ADE80"] },
                                { id: "ocean", name: "Ocean", dot: "dots", corner: "dot", color: "#0369A1", bg: "#F0F9FF", gradient: ["#0EA5E9", "#06B6D4"] },
                                { id: "sunset", name: "Sunset", dot: "rounded", corner: "extra-rounded", color: "#EA580C", bg: "#FFF7ED", gradient: ["#F97316", "#EF4444"] },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg" style={tpl.gradient ? { background: `linear-gradient(135deg, ${tpl.gradient[0]}, ${tpl.gradient[1]})` } : { backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Neon */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Neon</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "neon-pink", name: "Pink", dot: "dots", corner: "dot", color: "#EC4899", bg: "#0F0A1A", gradient: ["#EC4899", "#F472B6"] },
                                { id: "neon-cyan", name: "Cyan", dot: "rounded", corner: "extra-rounded", color: "#06B6D4", bg: "#0A0F1A", gradient: ["#06B6D4", "#22D3EE"] },
                                { id: "neon-purple", name: "Purple", dot: "extra-rounded", corner: "dot", color: "#A855F7", bg: "#0F0A1A", gradient: ["#7C3AED", "#C084FC"] },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg" style={tpl.gradient ? { background: `linear-gradient(135deg, ${tpl.gradient[0]}, ${tpl.gradient[1]})` } : { backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Business */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Business</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "corporate", name: "Corporate", dot: "square", corner: "square", color: "#1E293B", bg: "#FFFFFF", gradient: null },
                                { id: "minimal", name: "Minimal", dot: "dots", corner: "extra-rounded", color: "#64748B", bg: "#F8FAFC", gradient: null },
                                { id: "executive", name: "Executive", dot: "classy", corner: "square", color: "#1C1917", bg: "#FAFAF9", gradient: ["#1C1917", "#44403C"] },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg" style={tpl.gradient ? { background: `linear-gradient(135deg, ${tpl.gradient[0]}, ${tpl.gradient[1]})` } : { backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Social */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Social</p>
                        <div className="grid grid-cols-3 gap-2">
                            {[
                                { id: "instagram", name: "Instagram", dot: "rounded", corner: "extra-rounded", color: "#E4405F", bg: "#FFFFFF", gradient: ["#F58529", "#DD2A7B"] },
                                { id: "tiktok", name: "TikTok", dot: "dots", corner: "dot", color: "#000000", bg: "#FFFFFF", gradient: ["#00F2EA", "#FF0050"] },
                                { id: "youtube", name: "YouTube", dot: "square", corner: "square", color: "#FF0000", bg: "#FFFFFF", gradient: ["#FF0000", "#CC0000"] },
                            ].map((tpl) => (
                                <button
                                    key={tpl.id}
                                    onClick={() => {
                                        setSelectedDotStyle(tpl.dot); setSelectedCornerStyle(tpl.corner);
                                        setQrColor(tpl.color); setBgColor(tpl.bg);
                                        if (tpl.gradient) { setUseGradient(true); setGradientStart(tpl.gradient[0]); setGradientEnd(tpl.gradient[1]); }
                                        else setUseGradient(false);
                                    }}
                                    className="flex flex-col items-center gap-1.5 p-2.5 rounded-xl border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <div className="w-8 h-8 rounded-lg" style={tpl.gradient ? { background: `linear-gradient(135deg, ${tpl.gradient[0]}, ${tpl.gradient[1]})` } : { backgroundColor: tpl.color }} />
                                    <span className="text-[10px] text-zinc-500">{tpl.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {/* Frame Options */}
            {designTab === "frame" && (
                <div className="space-y-4">
                    <p className="text-sm text-zinc-500">Add a decorative frame around your QR code</p>
                    <div className="grid grid-cols-4 gap-3">
                        {[
                            { id: "none", label: "None", preview: "⊘" },
                            { id: "simple", label: "Simple", preview: "▢" },
                            { id: "rounded", label: "Rounded", preview: "◯" },
                            { id: "fancy", label: "Fancy", preview: "❋" },
                            { id: "scan-me", label: "Scan Me", preview: "📱" },
                            { id: "arrow", label: "Arrow", preview: "➤" },
                            { id: "badge", label: "Badge", preview: "🏅" },
                            { id: "dots", label: "Dots", preview: "⋯" },
                            { id: "phone", label: "Phone", preview: "📱" },
                            { id: "polaroid", label: "Polaroid", preview: "📷" },
                            { id: "ribbon", label: "Ribbon", preview: "🎀" },
                            { id: "sticker", label: "Sticker", preview: "🏷️" },
                            { id: "neon", label: "Neon", preview: "✨" },
                        ].map((frame) => (
                            <button
                                key={frame.id}
                                onClick={() => setSelectedFrame(frame.id)}
                                className={`flex flex-col items-center gap-2 p-3 rounded-xl border-2 transition-all ${selectedFrame === frame.id
                                    ? "border-violet-500 bg-violet-500/10"
                                    : "border-white/10 hover:border-white/20 bg-white/5"
                                    }`}
                            >
                                <span className="text-2xl">{frame.preview}</span>
                                <span className="text-[10px] text-zinc-500">{frame.label}</span>
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {/* Shape Options */}
            {designTab === "shape" && (
                <div className="space-y-5">
                    {/* Dot Style */}
                    <div>
                        <p className="text-sm text-zinc-400 mb-3">Body Pattern</p>
                        <div className="grid grid-cols-4 gap-2">
                            {[
                                { id: "square", label: "Square", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="2" y="2" width="16" height="16" fill="currentColor" /></svg> },
                                { id: "dots", label: "Dots", preview: <svg width="20" height="20" viewBox="0 0 20 20"><circle cx="10" cy="10" r="8" fill="currentColor" /></svg> },
                                { id: "rounded", label: "Rounded", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="2" y="2" width="16" height="16" rx="4" fill="currentColor" /></svg> },
                                { id: "classy", label: "Classy", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="2" y="2" width="16" height="16" rx="0" ry="8" fill="currentColor" /></svg> },
                                { id: "extra-rounded", label: "Pill", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="2" y="2" width="16" height="16" rx="8" fill="currentColor" /></svg> },
                                { id: "diamond", label: "Diamond", preview: <svg width="20" height="20" viewBox="0 0 20 20"><polygon points="10,1 19,10 10,19 1,10" fill="currentColor" /></svg> },
                                { id: "star", label: "Star", preview: <svg width="20" height="20" viewBox="0 0 20 20"><polygon points="10,1 12.5,7 19,7.5 14,12.5 15.5,19 10,15.5 4.5,19 6,12.5 1,7.5 7.5,7" fill="currentColor" /></svg> },
                                { id: "heart", label: "Heart", preview: <svg width="20" height="20" viewBox="0 0 20 20"><path d="M10 18s-7-5-7-10c0-3 2-5 5-5 1.5 0 2 1 2 1s.5-1 2-1c3 0 5 2 5 5 0 5-7 10-7 10z" fill="currentColor" /></svg> },
                                { id: "cross", label: "Cross", preview: <svg width="20" height="20" viewBox="0 0 20 20"><path d="M7 2h6v5h5v6h-5v5H7v-5H2V7h5z" fill="currentColor" /></svg> },
                                { id: "classy-rounded", label: "Classy R", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="2" y="2" width="16" height="16" rx="2" ry="8" fill="currentColor" /></svg> },
                                { id: "thin-rounded", label: "Thin", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="6" y="2" width="8" height="16" rx="4" fill="currentColor" /></svg> },
                                { id: "vertical-line", label: "Lines", preview: <svg width="20" height="20" viewBox="0 0 20 20"><rect x="7" y="1" width="6" height="18" fill="currentColor" /></svg> },
                            ].map((style) => (
                                <button
                                    key={style.id}
                                    onClick={() => setSelectedDotStyle(style.id)}
                                    className={`flex flex-col items-center gap-1.5 p-2 rounded-lg border transition-all ${selectedDotStyle === style.id
                                        ? "border-violet-500 bg-violet-500/10 text-violet-300"
                                        : "border-white/10 hover:border-white/20 text-zinc-500"
                                        }`}
                                >
                                    <span className="text-lg">{style.preview}</span>
                                    <span className="text-[8px]">{style.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Eye Outer Style */}
                    <div>
                        <p className="text-sm text-zinc-400 mb-3">Eye Frame (Outer)</p>
                        <div className="grid grid-cols-4 gap-2">
                            {[
                                { id: "square", label: "Square", preview: <svg width="22" height="22" viewBox="0 0 22 22"><rect x="1" y="1" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "dot", label: "Circle", preview: <svg width="22" height="22" viewBox="0 0 22 22"><circle cx="11" cy="11" r="9" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "extra-rounded", label: "Rounded", preview: <svg width="22" height="22" viewBox="0 0 22 22"><rect x="1" y="1" width="20" height="20" rx="6" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "none", label: "None", preview: <svg width="22" height="22" viewBox="0 0 22 22"><circle cx="11" cy="11" r="3" fill="currentColor" /></svg> },
                                { id: "leaf", label: "Leaf", preview: <svg width="22" height="22" viewBox="0 0 22 22"><rect x="1" y="1" width="20" height="20" rx="10" ry="2" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "rounded-sm", label: "Soft", preview: <svg width="22" height="22" viewBox="0 0 22 22"><rect x="1" y="1" width="20" height="20" rx="3" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "classy", label: "Classy", preview: <svg width="22" height="22" viewBox="0 0 22 22"><rect x="1" y="1" width="20" height="20" rx="0" ry="10" fill="none" stroke="currentColor" strokeWidth="3" /></svg> },
                                { id: "shield", label: "Shield", preview: <svg width="22" height="22" viewBox="0 0 22 22"><path d="M11 1L20 5v7c0 5-9 9-9 9S2 17 2 12V5z" fill="none" stroke="currentColor" strokeWidth="2" /></svg> },
                            ].map((style) => (
                                <button
                                    key={style.id}
                                    onClick={() => setSelectedCornerStyle(style.id)}
                                    className={`flex flex-col items-center gap-1.5 p-2 rounded-lg border transition-all ${selectedCornerStyle === style.id
                                        ? "border-violet-500 bg-violet-500/10 text-violet-300"
                                        : "border-white/10 hover:border-white/20 text-zinc-500"
                                        }`}
                                >
                                    <span>{style.preview}</span>
                                    <span className="text-[8px]">{style.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Eye Inner Style */}
                    <div>
                        <p className="text-sm text-zinc-400 mb-3">Eye Ball (Inner)</p>
                        <div className="grid grid-cols-5 gap-2">
                            {[
                                { id: "square", label: "Square", preview: <svg width="16" height="16" viewBox="0 0 16 16"><rect x="2" y="2" width="12" height="12" fill="currentColor" /></svg> },
                                { id: "dot", label: "Circle", preview: <svg width="16" height="16" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6" fill="currentColor" /></svg> },
                                { id: "rounded", label: "Rounded", preview: <svg width="16" height="16" viewBox="0 0 16 16"><rect x="2" y="2" width="12" height="12" rx="3" fill="currentColor" /></svg> },
                                { id: "diamond", label: "Diamond", preview: <svg width="16" height="16" viewBox="0 0 16 16"><polygon points="8,1 15,8 8,15 1,8" fill="currentColor" /></svg> },
                                { id: "star", label: "Star", preview: <svg width="16" height="16" viewBox="0 0 16 16"><polygon points="8,1 10,6 16,6 11,10 13,16 8,12 3,16 5,10 0,6 6,6" fill="currentColor" /></svg> },
                            ].map((style) => (
                                <button
                                    key={style.id}
                                    onClick={() => setEyeInnerStyle(style.id)}
                                    className={`flex flex-col items-center gap-1 p-2 rounded-lg border transition-all ${eyeInnerStyle === style.id
                                        ? "border-violet-500 bg-violet-500/10 text-violet-300"
                                        : "border-white/10 hover:border-white/20 text-zinc-500"
                                        }`}
                                >
                                    <span>{style.preview}</span>
                                    <span className="text-[7px]">{style.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Quiet Zone / Margin */}
                    <div>
                        <div className="flex justify-between mb-2">
                            <p className="text-sm text-zinc-400">Quiet Zone</p>
                            <span className="text-xs text-zinc-500">{quietZone}px</span>
                        </div>
                        <input
                            type="range"
                            min="0"
                            max="50"
                            value={quietZone}
                            onChange={(e) => setQuietZone(Number(e.target.value))}
                            className="w-full h-1.5 bg-white/10 rounded-full appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-violet-500"
                        />
                        <div className="flex justify-between text-[10px] text-zinc-600 mt-1">
                            <span>None</span>
                            <span>Standard</span>
                            <span>Wide</span>
                        </div>
                    </div>

                    {/* Error Correction Level */}
                    <div>
                        <p className="text-sm text-zinc-400 mb-3">Error Correction</p>
                        <div className="grid grid-cols-4 gap-2">
                            {[
                                { id: "L", label: "Low (7%)" },
                                { id: "M", label: "Med (15%)" },
                                { id: "Q", label: "Qtr (25%)" },
                                { id: "H", label: "High (30%)" },
                            ].map((level) => (
                                <button
                                    key={level.id}
                                    onClick={() => setErrorLevel(level.id)}
                                    className={`p-2 rounded-lg border text-center transition-all ${errorLevel === level.id
                                        ? "border-violet-500 bg-violet-500/10 text-white"
                                        : "border-white/10 hover:border-white/20 text-zinc-500"
                                        }`}
                                >
                                    <div className="text-lg font-bold">{level.id}</div>
                                    <div className="text-[8px]">{level.label.split(" ")[0]}</div>
                                </button>
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {/* Color Options */}
            {designTab === "color" && (
                <div className="space-y-5">
                    <div>
                        <div className="flex items-center justify-between mb-3">
                            <label className="text-sm text-zinc-400">QR Code Color</label>
                            <div className="flex items-center gap-2">
                                <span className={`text-xs ${!useGradient ? "text-white" : "text-zinc-600"}`}>Solid</span>
                                <button
                                    onClick={() => setUseGradient(!useGradient)}
                                    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none ${useGradient ? "bg-violet-600" : "bg-slate-600"}`}
                                >
                                    <span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${useGradient ? "translate-x-4.5" : "translate-x-1"}`} />
                                </button>
                                <span className={`text-xs ${useGradient ? "text-white" : "text-zinc-600"}`}>Gradient</span>
                            </div>
                        </div>

                        {!useGradient ? (
                            <>
                                <div className="flex items-center gap-3">
                                    <input
                                        type="color"
                                        value={qrColor}
                                        onChange={(e) => setQrColor(e.target.value)}
                                        className="w-14 h-14 rounded-xl cursor-pointer border-2 border-white/10"
                                    />
                                    <input
                                        type="text"
                                        value={qrColor}
                                        onChange={(e) => setQrColor(e.target.value)}
                                        className="flex-1 px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white text-sm font-mono"
                                    />
                                </div>
                                <div className="flex gap-2 mt-3 overflow-x-auto pb-2">
                                    {["#000000", "#1a1a2e", "#16213e", "#0f3460", "#533483", "#e94560", "#00b894", "#0984e3"].map((color) => (
                                        <button
                                            key={color}
                                            onClick={() => setQrColor(color)}
                                            className={`w-8 h-8 rounded-lg border-2 transition-transform hover:scale-110 flex-shrink-0 ${qrColor === color ? "border-violet-400 scale-110" : "border-transparent"
                                                }`}
                                            style={{ backgroundColor: color }}
                                        />
                                    ))}
                                </div>
                            </>
                        ) : (
                            <div className="space-y-4 animate-fadeIn">
                                <div className="grid grid-cols-2 gap-3">
                                    <div>
                                        <label className="block text-xs text-zinc-500 mb-1.5">Start Color</label>
                                        <div className="flex items-center gap-2">
                                            <input
                                                type="color"
                                                value={gradientStart}
                                                onChange={(e) => setGradientStart(e.target.value)}
                                                className="w-10 h-10 rounded-lg cursor-pointer border-2 border-white/10"
                                            />
                                            <input
                                                type="text"
                                                value={gradientStart}
                                                onChange={(e) => setGradientStart(e.target.value)}
                                                className="w-full px-2 py-2 bg-white/5 border border-white/10 rounded-lg text-white text-xs font-mono"
                                            />
                                        </div>
                                    </div>
                                    <div>
                                        <label className="block text-xs text-zinc-500 mb-1.5">End Color</label>
                                        <div className="flex items-center gap-2">
                                            <input
                                                type="color"
                                                value={gradientEnd}
                                                onChange={(e) => setGradientEnd(e.target.value)}
                                                className="w-10 h-10 rounded-lg cursor-pointer border-2 border-white/10"
                                            />
                                            <input
                                                type="text"
                                                value={gradientEnd}
                                                onChange={(e) => setGradientEnd(e.target.value)}
                                                className="w-full px-2 py-2 bg-white/5 border border-white/10 rounded-lg text-white text-xs font-mono"
                                            />
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <label className="block text-xs text-zinc-500 mb-1.5">Gradient Type</label>
                                    <div className="flex gap-2">
                                        <button
                                            onClick={() => setGradientType("linear")}
                                            className={`flex-1 py-1.5 rounded-lg text-xs border ${gradientType === "linear" ? "bg-violet-500/20 border-violet-500 text-white" : "bg-white/5 border-white/10 text-zinc-500"}`}
                                        >
                                            Linear
                                        </button>
                                        <button
                                            onClick={() => setGradientType("radial")}
                                            className={`flex-1 py-1.5 rounded-lg text-xs border ${gradientType === "radial" ? "bg-violet-500/20 border-violet-500 text-white" : "bg-white/5 border-white/10 text-zinc-500"}`}
                                        >
                                            Radial
                                        </button>
                                    </div>
                                </div>

                                {gradientType === "linear" && (
                                    <div>
                                        <div className="flex justify-between mb-1">
                                            <label className="text-xs text-zinc-500">Rotation</label>
                                            <span className="text-xs text-zinc-500">{gradientRotation}°</span>
                                        </div>
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            value={gradientRotation}
                                            onChange={(e) => setGradientRotation(Number(e.target.value))}
                                            className="w-full h-1 bg-white/10 rounded-full appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white"
                                        />
                                    </div>
                                )}
                            </div>
                        )}

                        {/* Gradient Presets */}
                        {useGradient && (
                            <div className="mt-3">
                                <label className="block text-xs text-zinc-500 mb-2">Quick Presets</label>
                                <div className="grid grid-cols-4 gap-2">
                                    {[
                                        { name: "Ocean", start: "#0EA5E9", end: "#06B6D4" },
                                        { name: "Sunset", start: "#F97316", end: "#EF4444" },
                                        { name: "Neon", start: "#A855F7", end: "#EC4899" },
                                        { name: "Forest", start: "#16A34A", end: "#4ADE80" },
                                        { name: "Berry", start: "#DB2777", end: "#9333EA" },
                                        { name: "Golden", start: "#F59E0B", end: "#EAB308" },
                                        { name: "Fire", start: "#DC2626", end: "#F97316" },
                                        { name: "Night", start: "#1E3A5F", end: "#7C3AED" },
                                    ].map((preset) => (
                                        <button
                                            key={preset.name}
                                            onClick={() => { setGradientStart(preset.start); setGradientEnd(preset.end); }}
                                            className="flex flex-col items-center gap-1 p-1.5 rounded-lg border border-white/10 hover:border-violet-500/50 transition-all"
                                        >
                                            <div className="w-full h-5 rounded" style={{ background: `linear-gradient(135deg, ${preset.start}, ${preset.end})` }} />
                                            <span className="text-[8px] text-zinc-500">{preset.name}</span>
                                        </button>
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>

                    <div>
                        <label className="block text-sm text-zinc-400 mb-3">Background Color</label>
                        <div className="flex items-center gap-3">
                            <input
                                type="color"
                                value={bgColor}
                                onChange={(e) => setBgColor(e.target.value)}
                                className="w-14 h-14 rounded-xl cursor-pointer border-2 border-white/10"
                            />
                            <input
                                type="text"
                                value={bgColor}
                                onChange={(e) => setBgColor(e.target.value)}
                                className="flex-1 px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white text-sm font-mono"
                            />
                        </div>
                        {/* Background Presets */}
                        <div className="flex gap-2 mt-3">
                            {["#FFFFFF", "#f8f9fa", "#e9ecef", "#ffeaa7", "#dfe6e9", "#fab1a0", "#81ecec", "#a29bfe"].map((color) => (
                                <button
                                    key={color}
                                    onClick={() => setBgColor(color)}
                                    className={`w-8 h-8 rounded-lg border-2 transition-transform hover:scale-110 ${bgColor === color ? "border-violet-400 scale-110" : "border-white/20"
                                        }`}
                                    style={{ backgroundColor: color }}
                                />
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {/* Logo Upload */}
            {designTab === "logo" && (
                <div className="space-y-4">
                    <p className="text-sm text-zinc-500">Add your logo to the center of the QR code</p>
                    <input
                        type="file"
                        accept="image/*"
                        onChange={(e) => {
                            const file = e.target.files?.[0];
                            if (file) {
                                const reader = new FileReader();
                                reader.onload = (ev) => setLogoFile(ev.target?.result as string);
                                reader.readAsDataURL(file);
                            }
                        }}
                        className="hidden"
                        id="logo-upload"
                    />
                    <label
                        htmlFor="logo-upload"
                        className="flex flex-col items-center justify-center p-6 rounded-xl border-2 border-dashed border-white/10 hover:border-violet-500/50 cursor-pointer transition-all hover:bg-violet-500/5"
                    >
                        {logoFile ? (
                            <>
                                <img src={logoFile} alt="Logo" className="w-16 h-16 rounded-xl object-contain bg-white p-2 mb-2" />
                                <p className="text-sm text-white">Logo uploaded</p>
                                <p className="text-xs text-zinc-600">Click to change</p>
                            </>
                        ) : (
                            <>
                                <div className="w-14 h-14 rounded-xl bg-white/5 flex items-center justify-center mb-2">
                                    <svg className="w-7 h-7 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                    </svg>
                                </div>
                                <p className="text-sm text-white">Drop logo here or click to upload</p>
                                <p className="text-xs text-zinc-600 mt-1">PNG, JPG, SVG up to 2MB</p>
                            </>
                        )}
                    </label>

                    {/* Logo Size Slider */}
                    {logoFile && (
                        <div className="space-y-3">
                            <div>
                                <div className="flex justify-between mb-1.5">
                                    <label className="text-xs text-zinc-500">Logo Size</label>
                                    <span className="text-xs text-zinc-500">{Math.round(logoSize * 100)}%</span>
                                </div>
                                <input
                                    type="range"
                                    min="0.1"
                                    max="0.4"
                                    step="0.01"
                                    value={logoSize}
                                    onChange={(e) => setLogoSize(Number(e.target.value))}
                                    className="w-full h-1.5 bg-white/10 rounded-full appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-violet-500"
                                />
                            </div>
                            <div>
                                <div className="flex justify-between mb-1.5">
                                    <label className="text-xs text-zinc-500">Logo Margin</label>
                                    <span className="text-xs text-zinc-500">{logoMargin}px</span>
                                </div>
                                <input
                                    type="range"
                                    min="0"
                                    max="20"
                                    value={logoMargin}
                                    onChange={(e) => setLogoMargin(Number(e.target.value))}
                                    className="w-full h-1.5 bg-white/10 rounded-full appearance-none cursor-pointer [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-violet-500"
                                />
                            </div>
                            <button
                                onClick={() => setLogoFile(null)}
                                className="w-full py-2 rounded-lg border border-red-500/30 text-red-400 text-sm hover:bg-red-500/10 transition-colors"
                            >
                                Remove Logo
                            </button>
                        </div>
                    )}

                    {/* Brand Logo Presets */}
                    <div>
                        <p className="text-xs text-zinc-600 uppercase tracking-wider mb-2">Quick Brand Logos</p>
                        <div className="grid grid-cols-5 gap-2">
                            {[
                                { name: "Apple", emoji: "🍎" },
                                { name: "Google", emoji: "🔍" },
                                { name: "Instagram", emoji: "📷" },
                                { name: "Facebook", emoji: "👤" },
                                { name: "YouTube", emoji: "▶️" },
                                { name: "TikTok", emoji: "🎵" },
                                { name: "Twitter", emoji: "🐦" },
                                { name: "WhatsApp", emoji: "💬" },
                                { name: "LinkedIn", emoji: "💼" },
                                { name: "Spotify", emoji: "🎶" },
                            ].map((brand) => (
                                <button
                                    key={brand.name}
                                    onClick={() => {
                                        // Create a canvas with the brand emoji as a quick logo
                                        const canvas = document.createElement('canvas');
                                        canvas.width = 128; canvas.height = 128;
                                        const ctx = canvas.getContext('2d');
                                        if (ctx) {
                                            ctx.fillStyle = '#ffffff';
                                            ctx.fillRect(0, 0, 128, 128);
                                            ctx.font = '80px serif';
                                            ctx.textAlign = 'center';
                                            ctx.textBaseline = 'middle';
                                            ctx.fillText(brand.emoji, 64, 64);
                                            setLogoFile(canvas.toDataURL());
                                        }
                                    }}
                                    className="flex flex-col items-center gap-1 p-2 rounded-lg border border-white/10 hover:border-violet-500/50 hover:bg-white/5 transition-all"
                                >
                                    <span className="text-xl">{brand.emoji}</span>
                                    <span className="text-[7px] text-zinc-500">{brand.name}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    <p className="text-xs text-slate-500 text-center">
                        💡 Use &quot;High&quot; error correction for best results with logos
                    </p>
                </div>
            )}

            {/* Background Options */}
            {designTab === "background" && (
                <div className="space-y-5">
                    <p className="text-sm text-slate-400">Customize your QR code background</p>

                    {/* Transparent Background */}
                    <div className="flex items-center justify-between p-3 rounded-xl bg-white/5 border border-white/10">
                        <div>
                            <p className="text-sm text-white">Transparent Background</p>
                            <p className="text-xs text-slate-500">Remove background color</p>
                        </div>
                        <button
                            onClick={() => setTransparentBg(!transparentBg)}
                            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none ${transparentBg ? "bg-violet-600" : "bg-slate-600"}`}
                        >
                            <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${transparentBg ? "translate-x-6" : "translate-x-1"}`} />
                        </button>
                    </div>

                    {/* Background Color (when not transparent) */}
                    {!transparentBg && (
                        <div>
                            <label className="block text-sm text-slate-300 mb-2">Background Color</label>
                            <div className="flex items-center gap-3">
                                <input
                                    type="color"
                                    value={bgColor}
                                    onChange={(e) => setBgColor(e.target.value)}
                                    className="w-12 h-12 rounded-xl cursor-pointer border-2 border-white/10"
                                />
                                <input
                                    type="text"
                                    value={bgColor}
                                    onChange={(e) => setBgColor(e.target.value)}
                                    className="flex-1 px-3 py-2.5 bg-white/5 border border-white/10 rounded-xl text-white text-sm font-mono"
                                />
                            </div>
                            <div className="flex gap-2 mt-2 overflow-x-auto pb-1">
                                {["#FFFFFF", "#f8f9fa", "#e9ecef", "#ffeaa7", "#dfe6e9", "#fab1a0", "#81ecec", "#a29bfe"].map((color) => (
                                    <button
                                        key={color}
                                        onClick={() => setBgColor(color)}
                                        className={`w-7 h-7 rounded-lg border-2 transition-transform hover:scale-110 flex-shrink-0 ${bgColor === color ? "border-violet-400 scale-110" : "border-white/20"}`}
                                        style={{ backgroundColor: color }}
                                    />
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Background Image Upload */}
                    <div>
                        <label className="block text-sm text-slate-300 mb-2">Background Image</label>
                        <input
                            type="file"
                            accept="image/*"
                            onChange={(e) => {
                                const file = e.target.files?.[0];
                                if (file) {
                                    const reader = new FileReader();
                                    reader.onload = (ev) => setBgImage(ev.target?.result as string);
                                    reader.readAsDataURL(file);
                                }
                            }}
                            className="hidden"
                            id="bg-image-upload"
                        />
                        <label
                            htmlFor="bg-image-upload"
                            className="flex flex-col items-center justify-center p-6 rounded-xl border-2 border-dashed border-white/10 hover:border-violet-500/50 cursor-pointer transition-all hover:bg-violet-500/5"
                        >
                            {bgImage ? (
                                <>
                                    <div className="w-full h-32 rounded-lg bg-cover bg-center mb-2" style={{ backgroundImage: `url(${bgImage})` }} />
                                    <p className="text-sm text-white">Image uploaded</p>
                                    <p className="text-xs text-slate-500">Click to change</p>
                                </>
                            ) : (
                                <>
                                    <div className="w-14 h-14 rounded-xl bg-white/5 flex items-center justify-center mb-2">
                                        <svg className="w-7 h-7 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                        </svg>
                                    </div>
                                    <p className="text-sm text-white">Upload Background</p>
                                </>
                            )}
                        </label>
                        {bgImage && (
                            <button
                                onClick={() => setBgImage(null)}
                                className="w-full mt-2 py-2 rounded-lg border border-red-500/30 text-red-400 text-sm hover:bg-red-500/10 transition-colors"
                            >
                                Remove Image
                            </button>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
};
