"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Link from "next/link";
import { API_URL } from "@/lib/api";
import { useAuthStore } from "@/lib/auth";
import { CreateQRPanel } from "@/components/dashboard/CreateQRPanel";
import { MyQRPanel } from "@/components/dashboard/MyQRPanel";
import { CustomizeQRPanel } from "@/components/dashboard/CustomizeQRPanel";
import { qrTypes, QRIcons } from "@/utils/qrConstants";
import {
  CheckCircle2, BarChart3, Users, QrCode, ArrowRight,
  MonitorSmartphone, MapPin, Clock, ShieldCheck, ChevronDown, Sparkles, ChevronUp, Star, Quote
} from "lucide-react";

// builds the QR content string from the active form state

function buildContent(
  type: string,
  content: string,
  vcardData: any,
  emailData: any,
  smsData: any,
  wifiData: any,
  locationData: any,
  eventData: any,
  socialData: any,
  bitcoinData: any,
  upiData: any,
  whatsappData: any,
  mecardData: any,
  multiLinkData: any[]
): string {
  switch (type) {
    case "url":
    case "text":
    case "phone":
      return content || "";
    case "vcard": {
      const v = vcardData;
      if (!v.name) return "";
      return [
        "BEGIN:VCARD", "VERSION:3.0",
        `FN:${v.name}`,
        v.phone && `TEL:${v.phone}`,
        v.email && `EMAIL:${v.email}`,
        v.company && `ORG:${v.company}`,
        v.title && `TITLE:${v.title}`,
        v.url && `URL:${v.url}`,
        v.address && `ADR:;;${v.address};;;;`,
        "END:VCARD",
      ].filter(Boolean).join("\n");
    }
    case "email": {
      const e = emailData;
      if (!e.to) return "";
      const params = new URLSearchParams();
      if (e.subject) params.set("subject", e.subject);
      if (e.body) params.set("body", e.body);
      const qs = params.toString();
      return `mailto:${e.to}${qs ? `?${qs}` : ""}`;
    }
    case "sms":
      return smsData.phone ? `SMSTO:${smsData.phone}:${smsData.message || ""}` : "";
    case "wifi": {
      const w = wifiData;
      if (!w.ssid) return "";
      return `WIFI:T:${w.encryption || "WPA"};S:${w.ssid};P:${w.password || ""};;`;
    }
    case "location": {
      const l = locationData;
      if (!l.lat || !l.lng) return "";
      return `geo:${l.lat},${l.lng}${l.name ? `?q=${encodeURIComponent(l.name)}` : ""}`;
    }
    case "event": {
      const ev = eventData;
      if (!ev.title) return "";
      const fmt = (d: string) => d.replace(/[-:T]/g, "").slice(0, 15);
      return [
        "BEGIN:VEVENT",
        `SUMMARY:${ev.title}`,
        ev.startDate && `DTSTART:${fmt(ev.startDate)}`,
        ev.endDate && `DTEND:${fmt(ev.endDate)}`,
        ev.location && `LOCATION:${ev.location}`,
        ev.description && `DESCRIPTION:${ev.description}`,
        "END:VEVENT",
      ].filter(Boolean).join("\n");
    }
    case "social": {
      const s = socialData;
      if (!s.username) return "";
      const bases: Record<string, string> = {
        instagram: "https://instagram.com/",
        twitter: "https://twitter.com/",
        facebook: "https://facebook.com/",
        linkedin: "https://linkedin.com/in/",
        tiktok: "https://tiktok.com/@",
        youtube: "https://youtube.com/@",
      };
      return s.platform === "custom" ? s.username : `${bases[s.platform] || "https://"}${s.username}`;
    }
    case "bitcoin": {
      const b = bitcoinData;
      if (!b.address) return "";
      const params: string[] = [];
      if (b.amount) params.push(`amount=${b.amount}`);
      if (b.label) params.push(`label=${encodeURIComponent(b.label)}`);
      return `bitcoin:${b.address}${params.length ? `?${params.join("&")}` : ""}`;
    }
    case "upi": {
      const u = upiData;
      if (!u.upiId) return "";
      const params = [`pa=${u.upiId}`];
      if (u.name) params.push(`pn=${encodeURIComponent(u.name)}`);
      if (u.amount) params.push(`am=${u.amount}`);
      if (u.refId) params.push(`tr=${u.refId}`);
      if (u.note) params.push(`tn=${encodeURIComponent(u.note)}`);
      return `upi://pay?${params.join("&")}`;
    }
    case "whatsapp": {
      const wa = whatsappData;
      if (!wa.phone) return "";
      const num = wa.phone.replace(/\D/g, "");
      return `https://wa.me/${num}${wa.message ? `?text=${encodeURIComponent(wa.message)}` : ""}`;
    }
    case "mecard": {
      const mc = mecardData;
      if (!mc.name) return "";
      const parts = [`MECARD:N:${mc.name}`];
      (mc.phones || []).forEach((p: string) => p && parts.push(`TEL:${p}`));
      (mc.emails || []).forEach((e: string) => e && parts.push(`EMAIL:${e}`));
      if (mc.address) parts.push(`ADR:${mc.address}`);
      if (mc.url) parts.push(`URL:${mc.url}`);
      if (mc.note) parts.push(`NOTE:${mc.note}`);
      return parts.join(";") + ";;";
    }
    case "multilink": {
      const links = multiLinkData.filter((l: any) => l.url);
      if (!links.length) return "";
      return links.map((l: any) => l.url).join("\n");
    }
    default:
      return content || "";
  }
}

// Main page

type Panel = "create" | "myqrs";

function toQrDataUrl(raw: string | null | undefined): string | null {
  if (!raw || typeof raw !== "string") return null;
  const t = raw.trim();
  if (!t) return null;
  return t.startsWith("data:") ? t : `data:image/png;base64,${t}`;
}

export default function HomePage() {
  const { accessToken } = useAuthStore();
  const api = API_URL;

  // Panel / type state
  const [activePanel, setActivePanel] = useState<Panel>("create");
  const [selectedType, setSelectedType] = useState("url");
  const [isDynamicQR, setIsDynamicQR] = useState(false);
  const [title, setTitle] = useState("");

  // Per-type form state (mirrors CreateQRPanel props)
  const [content, setContent] = useState("https://example.com");
  const [vcardData, setVcardData] = useState({ name: "", phone: "", email: "", company: "", title: "", url: "", address: "" });
  const [emailData, setEmailData] = useState({ to: "", subject: "", body: "" });
  const [smsData, setSmsData] = useState({ phone: "", message: "" });
  const [wifiData, setWifiData] = useState({ ssid: "", password: "", encryption: "WPA" });
  const [locationData, setLocationData] = useState({ lat: "", lng: "", name: "" });
  const [eventData, setEventData] = useState({ title: "", location: "", startDate: "", endDate: "", description: "" });
  const [socialData, setSocialData] = useState({ platform: "instagram", username: "" });
  const [bitcoinData, setBitcoinData] = useState({ address: "", amount: "", label: "" });
  const [upiData, setUpiData] = useState({ upiId: "", name: "", amount: "", refId: "", note: "" });
  const [whatsappData, setWhatsappData] = useState({ phone: "", message: "" });
  const [mecardData, setMecardData] = useState({ name: "", phones: ["+91 "], emails: [""], address: "", url: "", note: "" });
  const [multiLinkData, setMultiLinkData] = useState([{ label: "", url: "" }]);

  // QR preview state
  const [qrImage, setQrImage] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Customization state
  const [showCustomization, setShowCustomization] = useState(false);
  const [designTab, setDesignTab] = useState("templates");
  const [selectedDotStyle, setSelectedDotStyle] = useState("square");
  const [selectedCornerStyle, setSelectedCornerStyle] = useState("square");
  const [selectedFrame, setSelectedFrame] = useState("none");
  const [eyeInnerStyle, setEyeInnerStyle] = useState("square");
  const [quietZone, setQuietZone] = useState(4);
  const [errorLevel, setErrorLevel] = useState("M");
  const [qrColor, setQrColor] = useState("#000000");
  const [bgColor, setBgColor] = useState("#ffffff");
  const [useGradient, setUseGradient] = useState(false);
  const [gradientStart, setGradientStart] = useState("#000000");
  const [gradientEnd, setGradientEnd] = useState("#000000");
  const [gradientType, setGradientType] = useState<"linear" | "radial">("linear");
  const [gradientRotation, setGradientRotation] = useState(0);
  const [transparentBg, setTransparentBg] = useState(false);
  const [logoFile, setLogoFile] = useState<string | null>(null);
  const [logoSize, setLogoSize] = useState(40);
  const [logoMargin, setLogoMargin] = useState(0);
  const [bgImage, setBgImage] = useState<string | null>(null);

  // Marketing sections state
  const [expandedFaq, setExpandedFaq] = useState<number | null>(null);

  const customizationOptions = {
    designTab, selectedDotStyle, selectedCornerStyle, selectedFrame, eyeInnerStyle, quietZone, errorLevel,
    qrColor, bgColor, useGradient, gradientStart, gradientEnd, gradientType, gradientRotation,
    transparentBg, logoFile, logoSize, logoMargin, bgImage
  };

  // My QRs state
  const [myQRs, setMyQRs] = useState<any[]>([]);
  const [loadingMyQRs, setLoadingMyQRs] = useState(false);

  // Build content string from current form state
  const currentContent = buildContent(
    selectedType, content,
    vcardData, emailData, smsData, wifiData, locationData,
    eventData, socialData, bitcoinData, upiData,
    whatsappData, mecardData, multiLinkData
  );

  // Auto-generate QR on content change (local preview first so UI always works; server may replace)
  const generateQR = useCallback(async (qrContent: string, custOptions: any) => {
    if (!qrContent) {
      setQrImage(null);
      return;
    }
    setGenerating(true);
    const previewSize = accessToken ? 300 : 220;
    const showLocalPreview = async () => {
      try {
        const { getLocalQrDataUrl } = await import("@/lib/localQrPreview");
        setQrImage(await getLocalQrDataUrl(qrContent, previewSize, custOptions));
      } catch {
        /* keep prior image; server may still succeed */
      }
    };
    try {
      await showLocalPreview();
      const endpoint = accessToken
        ? `${api}/qr/generate`
        : `${api}/public/generate`;
      const body = accessToken
        ? { title: title || qrContent.slice(0, 60), content: qrContent, qr_type: selectedType, size: 300, is_dynamic: isDynamicQR, customization: custOptions }
        : { content: qrContent, qr_type: selectedType, size: 200 };
      const res = await fetch(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify(body),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || !data.success) return;
      const serverImg = accessToken
        ? toQrDataUrl(data.data?.qr_base64)
        : toQrDataUrl(data.qr_base64);
      if (serverImg) setQrImage(serverImg);
    } catch {
      await showLocalPreview();
    } finally {
      setGenerating(false);
    }
  }, [accessToken, api, selectedType, isDynamicQR, title]);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const currentCustOptions = customizationOptions;
    debounceRef.current = setTimeout(() => generateQR(currentContent, currentCustOptions), 300);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [currentContent, generateQR, customizationOptions.designTab, customizationOptions.selectedDotStyle, customizationOptions.selectedCornerStyle, customizationOptions.selectedFrame, customizationOptions.eyeInnerStyle, customizationOptions.quietZone, customizationOptions.errorLevel, customizationOptions.qrColor, customizationOptions.bgColor, customizationOptions.useGradient, customizationOptions.gradientStart, customizationOptions.gradientEnd, customizationOptions.gradientType, customizationOptions.gradientRotation, customizationOptions.transparentBg, customizationOptions.logoFile, customizationOptions.logoSize, customizationOptions.logoMargin, customizationOptions.bgImage]);

  // Reset form when type changes
  const handleTypeSelect = (t: string) => {
    setSelectedType(t);
    setContent(t === "url" ? "https://example.com" : "");
    setQrImage(null);
    setSaveSuccess(false);
  };

  // Fetch saved QRs
  const fetchMyQRs = useCallback(async () => {
    if (!accessToken) return;
    setLoadingMyQRs(true);
    try {
      const res = await fetch(`${api}/qr/history?limit=50`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      const data = await res.json();
      if (data.success) setMyQRs(data.data?.records ?? data.data ?? []);
    } catch { /* silent */ } finally {
      setLoadingMyQRs(false);
    }
  }, [accessToken, api]);

  useEffect(() => {
    if (activePanel === "myqrs") fetchMyQRs();
  }, [activePanel, fetchMyQRs]);

  // Save QR to account
  const handleSave = async () => {
    if (!accessToken || !qrImage || !currentContent) return;
    setSaving(true);
    try {
      const res = await fetch(`${api}/qr/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({
          title: title || currentContent.slice(0, 60),
          content: currentContent,
          qr_type: selectedType,
          size: 512,
          is_dynamic: isDynamicQR,
        }),
      });
      const data = await res.json();
      if (data.success) { setSaveSuccess(true); setTimeout(() => setSaveSuccess(false), 3000); }
    } catch { /* silent */ } finally {
      setSaving(false);
    }
  };

  // Download QR as PNG
  const handleDownload = () => {
    if (!qrImage) return;
    const a = document.createElement("a");
    a.href = qrImage;
    a.download = `qrit-${selectedType}-qrcode.png`;
    a.click();
  };

  const handleDeleteQR = async (id: string) => {
    if (!accessToken) return;
    await fetch(`${api}/qr/${id}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    fetchMyQRs();
  };

  return (
    <div className="min-h-screen bg-[#0d0d1a] text-white flex flex-col">

      {/* ── Navigation ── */}
      <nav className="sticky top-0 z-50 bg-[#0d0d1a]/80 backdrop-blur-xl border-b border-white/10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-violet-500/30">
              <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                <rect x="3" y="3" width="7" height="7" rx="1.5" fill="white" />
                <rect x="14" y="3" width="7" height="7" rx="1.5" fill="white" />
                <rect x="3" y="14" width="7" height="7" rx="1.5" fill="white" />
                <rect x="14" y="14" width="4" height="4" rx="1" fill="white" />
                <rect x="20" y="14" width="1" height="4" rx="0.5" fill="white" />
                <rect x="14" y="20" width="4" height="1" rx="0.5" fill="white" />
              </svg>
            </div>
            <span className="text-lg font-bold text-white">QR<span className="text-violet-400">it</span></span>
          </div>

          <div className="flex items-center gap-3">
            {accessToken ? (
              <Link href="/dashboard" className="flex items-center gap-2 px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 border border-white/10 text-sm text-slate-300 hover:text-white transition-all">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                </svg>
                Dashboard
              </Link>
            ) : (
              <>
                <Link href="/login" className="text-sm text-slate-400 hover:text-white transition-colors px-3 py-2">
                  Sign In
                </Link>
                <Link href="/register" className="px-4 py-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-sm font-semibold rounded-xl transition-all shadow-lg shadow-violet-500/20">
                  Get Started Free
                </Link>
              </>
            )}
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <div className="text-center py-10 px-4 relative">
        <div className="pointer-events-none absolute inset-0 -z-10">
          <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[300px] bg-violet-600/10 rounded-full blur-[100px]" />
        </div>
        <h1 className="text-3xl sm:text-5xl font-extrabold tracking-tight mb-3">
          Generate{" "}
          <span className="bg-gradient-to-r from-violet-400 via-purple-400 to-indigo-400 bg-clip-text text-transparent">
            QR Codes
          </span>{" "}
          instantly
        </h1>
        <p className="text-slate-400 text-base sm:text-lg max-w-xl mx-auto">
          14 QR types. Live preview. Download in seconds. No sign-up needed.
        </p>
      </div>

      {/* ── Main tool ── */}
      <div className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 pb-12">

        {/* Panel tabs (for logged-in users) */}
        {accessToken && (
          <div className="flex gap-1 mb-6 p-1 bg-white/5 border border-white/10 rounded-xl w-fit">
            {(["create", "myqrs"] as Panel[]).map((p) => (
              <button
                key={p}
                onClick={() => setActivePanel(p)}
                className={`px-5 py-2 rounded-lg text-sm font-medium transition-all ${
                  activePanel === p
                    ? "bg-white/10 text-white shadow-inner"
                    : "text-slate-500 hover:text-slate-300"
                }`}
              >
                {p === "create" ? "Create" : "My QR Codes"}
              </button>
            ))}
          </div>
        )}

        {/* My QRs panel */}
        {activePanel === "myqrs" && accessToken ? (
          <div className="p-6 rounded-2xl bg-white/5 border border-white/10">
            <MyQRPanel
              myQRs={myQRs}
              loadingMyQRs={loadingMyQRs}
              fetchMyQRs={fetchMyQRs}
              openEditModal={() => {}}
              handleEditDesign={() => {}}
              handleDeleteQR={handleDeleteQR}
              setActivePanel={(p: any) => setActivePanel(p === "create" ? "create" : "myqrs")}
            />
          </div>
        ) : (
          <div className="grid lg:grid-cols-[1fr_340px] gap-6">

            {/* ── Left: type selector + form ── */}
            <div className="space-y-5">

              {/* QR type grid */}
              <div className="p-5 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm">
                <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-widest mb-4">
                  Select QR Type
                </h2>
                <div className="grid grid-cols-4 sm:grid-cols-7 gap-2">
                  {qrTypes.map((t) => (
                    <button
                      key={t.id}
                      onClick={() => handleTypeSelect(t.id)}
                      title={t.description}
                      className={`flex flex-col items-center gap-1.5 p-3 rounded-xl border transition-all group ${
                        selectedType === t.id
                          ? "bg-violet-600/20 border-violet-500/50 shadow-lg shadow-violet-500/10"
                          : "bg-white/3 border-white/8 hover:bg-white/8 hover:border-white/20"
                      }`}
                    >
                      <div className={`w-7 h-7 flex items-center justify-center transition-transform group-hover:scale-110 ${
                        selectedType === t.id ? "scale-110" : ""
                      }`}>
                        {QRIcons[t.id]}
                      </div>
                      <span className={`text-[10px] font-medium truncate w-full text-center leading-tight ${
                        selectedType === t.id ? "text-violet-300" : "text-slate-500"
                      }`}>
                        {t.name}
                      </span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Form */}
              <div className="p-5 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm">
                <div className="flex items-center justify-between mb-5">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-lg bg-white/10 flex items-center justify-center">
                      {QRIcons[selectedType]}
                    </div>
                    <div>
                      <h2 className="text-sm font-semibold text-white">
                        {qrTypes.find(t => t.id === selectedType)?.name}
                      </h2>
                      <p className="text-xs text-slate-500">
                        {qrTypes.find(t => t.id === selectedType)?.description}
                      </p>
                    </div>
                  </div>

                  <label className="flex items-center gap-2 cursor-pointer select-none">
                    <span className="text-xs text-slate-500">Dynamic</span>
                    <div
                      onClick={() => setIsDynamicQR(!isDynamicQR)}
                      className={`relative w-10 h-5 rounded-full border transition-all ${
                        isDynamicQR
                          ? "bg-violet-600/50 border-violet-500/50"
                          : "bg-white/10 border-white/20"
                      }`}
                    >
                      <div className={`absolute top-0.5 w-4 h-4 rounded-full transition-all ${
                        isDynamicQR ? "left-5 bg-violet-400" : "left-0.5 bg-slate-500"
                      }`} />
                    </div>
                  </label>
                </div>

                {/* Title field */}
                <div className="mb-4">
                  <label className="block text-xs text-slate-500 mb-1.5">Title (optional)</label>
                  <input
                    type="text"
                    value={title}
                    onChange={e => setTitle(e.target.value)}
                    placeholder="My QR Code"
                    className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder:text-slate-600 focus:outline-none focus:border-violet-500/50 focus:ring-2 focus:ring-violet-500/20 transition-all text-sm"
                  />
                </div>

                <CreateQRPanel
                  selectedType={selectedType}
                  setSelectedType={setSelectedType}
                  isDynamicQR={isDynamicQR}
                  setIsDynamicQR={setIsDynamicQR}
                  content={content}
                  setContent={setContent}
                  vcardData={vcardData}
                  setVcardData={setVcardData}
                  emailData={emailData}
                  setEmailData={setEmailData}
                  smsData={smsData}
                  setSmsData={setSmsData}
                  wifiData={wifiData}
                  setWifiData={setWifiData}
                  locationData={locationData}
                  setLocationData={setLocationData}
                  eventData={eventData}
                  setEventData={setEventData}
                  socialData={socialData}
                  setSocialData={setSocialData}
                  bitcoinData={bitcoinData}
                  setBitcoinData={setBitcoinData}
                  upiData={upiData}
                  setUpiData={setUpiData}
                  whatsappData={whatsappData}
                  setWhatsappData={setWhatsappData}
                  mecardData={mecardData}
                  setMecardData={setMecardData}
                  multiLinkData={multiLinkData}
                  setMultiLinkData={setMultiLinkData}
                />
              </div>

              {/* ✨ Customize Design (Free for everyone) */}
              <div className="p-5 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm">
                <button
                  onClick={() => setShowCustomization(!showCustomization)}
                  className="w-full flex items-center justify-between"
                >
                  <div className="flex items-center gap-2">
                    <Sparkles className="w-5 h-5 text-violet-400" />
                    <span className="font-semibold text-white">Customize Design</span>
                  </div>
                  {showCustomization ? <ChevronUp className="w-5 h-5 text-slate-500" /> : <ChevronDown className="w-5 h-5 text-slate-500" />}
                </button>

                {showCustomization && (
                  <div className="mt-5 pt-5 border-t border-white/5">
                    <CustomizeQRPanel
                      designTab={designTab} setDesignTab={setDesignTab}
                      selectedDotStyle={selectedDotStyle} setSelectedDotStyle={setSelectedDotStyle}
                      selectedCornerStyle={selectedCornerStyle} setSelectedCornerStyle={setSelectedCornerStyle}
                      selectedFrame={selectedFrame} setSelectedFrame={setSelectedFrame}
                      eyeInnerStyle={eyeInnerStyle} setEyeInnerStyle={setEyeInnerStyle}
                      quietZone={quietZone} setQuietZone={setQuietZone}
                      errorLevel={errorLevel} setErrorLevel={setErrorLevel}
                      qrColor={qrColor} setQrColor={setQrColor}
                      bgColor={bgColor} setBgColor={setBgColor}
                      useGradient={useGradient} setUseGradient={setUseGradient}
                      gradientStart={gradientStart} setGradientStart={setGradientStart}
                      gradientEnd={gradientEnd} setGradientEnd={setGradientEnd}
                      gradientType={gradientType} setGradientType={setGradientType}
                      gradientRotation={gradientRotation} setGradientRotation={setGradientRotation}
                      transparentBg={transparentBg} setTransparentBg={setTransparentBg}
                      logoFile={logoFile} setLogoFile={setLogoFile}
                      logoSize={logoSize} setLogoSize={setLogoSize}
                      logoMargin={logoMargin} setLogoMargin={setLogoMargin}
                      bgImage={bgImage} setBgImage={setBgImage}
                    />
                  </div>
                )}
              </div>
            </div>

            {/* ── Right: live preview ── */}
            <div className="lg:sticky lg:top-24 lg:self-start space-y-4">
              <div className="p-6 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm flex flex-col items-center gap-5">
                <h2 className="self-start text-xs font-semibold text-slate-500 uppercase tracking-widest">
                  Live Preview
                </h2>

                {/* QR display */}
                <div className="w-56 h-56 rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center overflow-hidden">
                  {generating ? (
                    <div className="flex flex-col items-center gap-3">
                      <div className="w-10 h-10 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
                      <span className="text-xs text-slate-600">Generating…</span>
                    </div>
                  ) : qrImage ? (
                    <img src={qrImage} alt="QR Code" className="w-full h-full object-contain p-3" />
                  ) : (
                    <div className="text-center px-4">
                      <svg className="w-16 h-16 mx-auto mb-3 text-slate-800" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M3 3h7v7H3V3zm1 1v5h5V4H4zm1 1h3v3H5V5zm9-2h7v7h-7V3zm1 1v5h5V4h-5zm1 1h3v3h-3V5zM3 14h7v7H3v-7zm1 1v5h5v-5H4zm1 1h3v3H5v-3zm9 0h2v2h-2v-2zm3-1h1v1h-1v-1zm1 2h1v1h-1v-1zm-4 2h1v3h-1v-3zm2 0h2v1h-2v-1zm1 1h1v2h-1v-2zm2-2h1v1h-1v-1zm0 2h1v2h-1v-2z" />
                      </svg>
                      <p className="text-xs text-slate-700 leading-relaxed">
                        Fill in the form<br />to generate your QR
                      </p>
                    </div>
                  )}
                </div>

                {/* Actions */}
                {qrImage && (
                  <div className="w-full space-y-2">
                    <button
                      onClick={handleDownload}
                      className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-white/10 hover:bg-white/15 border border-white/10 text-sm font-medium text-slate-200 hover:text-white transition-all"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                      </svg>
                      Download PNG
                    </button>

                    {accessToken ? (
                      <button
                        onClick={handleSave}
                        disabled={saving || saveSuccess}
                        className={`w-full flex items-center justify-center gap-2 py-3 rounded-xl border text-sm font-medium transition-all ${
                          saveSuccess
                            ? "bg-emerald-500/20 border-emerald-500/40 text-emerald-300"
                            : "bg-violet-600/20 hover:bg-violet-600/30 border-violet-500/30 text-violet-300 hover:text-violet-200"
                        }`}
                      >
                        {saveSuccess ? (
                          <>
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                            Saved!
                          </>
                        ) : saving ? (
                          <>
                            <div className="w-4 h-4 border-2 border-violet-400 border-t-transparent rounded-full animate-spin" />
                            Saving…
                          </>
                        ) : (
                          <>
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
                            </svg>
                            Save to My QRs
                          </>
                        )}
                      </button>
                    ) : (
                      <Link
                        href="/register"
                        className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-violet-600/20 hover:bg-violet-600/30 border border-violet-500/30 text-violet-300 hover:text-violet-200 text-sm font-medium transition-all"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                        </svg>
                        Sign up to save & track
                      </Link>
                    )}
                  </div>
                )}

                {/* Stats / info */}
                {!qrImage && (
                  <div className="w-full grid grid-cols-2 gap-3 pt-2 border-t border-white/5">
                    {[
                      { label: "QR Types", value: "14+" },
                      { label: "No watermark", value: "Free" },
                      { label: "Formats", value: "PNG" },
                      { label: "Account needed", value: "No" },
                    ].map(s => (
                      <div key={s.label} className="text-center">
                        <p className="text-sm font-bold text-white">{s.value}</p>
                        <p className="text-[10px] text-slate-600 uppercase tracking-wide">{s.label}</p>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Enterprise CTA */}
              {!accessToken && (
                <div className="p-4 rounded-2xl bg-gradient-to-br from-violet-600/10 to-indigo-600/5 border border-violet-500/20">
                  <p className="text-xs font-semibold text-violet-300 mb-1">Need more power?</p>
                  <p className="text-xs text-slate-500 mb-3 leading-relaxed">
                    Dynamic QRs, analytics, team workspaces, bulk generation, and webhooks.
                  </p>
                  <Link
                    href="/register"
                    className="block text-center py-2 px-4 bg-violet-600/30 hover:bg-violet-600/40 border border-violet-500/30 text-violet-300 hover:text-violet-200 text-xs font-semibold rounded-lg transition-colors"
                  >
                    Start free →
                  </Link>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* ── Trust Bar ── */}
      <section className="py-12 border-y border-white/5 bg-white/[0.02]">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 text-center">
          <p className="text-sm font-medium text-slate-400 mb-8 uppercase tracking-widest">
            Trusted by 10,000+ creators & businesses worldwide
          </p>
          <div className="flex flex-wrap justify-center gap-8 md:gap-16">
            {[
              { label: "QR Codes Generated", value: "500K+" },
              { label: "Countries", value: "190+" },
              { label: "Uptime", value: "99.9%" },
              { label: "Average Rating", value: "4.8★" },
            ].map(stat => (
              <div key={stat.label} className="text-center">
                <h4 className="text-2xl sm:text-3xl font-bold text-white mb-1">{stat.value}</h4>
                <p className="text-xs text-slate-500 uppercase tracking-wider">{stat.label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Features Section (Zig-Zag) ── */}
      <section className="py-24 overflow-hidden">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 space-y-24">
          <div className="text-center max-w-3xl mx-auto mb-16">
            <h2 className="text-3xl md:text-4xl font-bold mb-4">Everything you need to create perfect QR codes</h2>
            <p className="text-slate-400 text-lg">QRit gives you the tools to create, track, and manage all your QR codes in one place.</p>
          </div>

          <div className="grid md:grid-cols-2 gap-12 items-center">
            <div className="space-y-6">
              <div className="w-12 h-12 rounded-2xl bg-violet-500/20 flex items-center justify-center border border-violet-500/30">
                <CheckCircle2 className="w-6 h-6 text-violet-400" />
              </div>
              <h3 className="text-2xl font-bold">Dynamic QR Codes</h3>
              <p className="text-slate-400 text-lg leading-relaxed">
                Printed a QR code with a typo? Need to update the menu link? Dynamic QR codes let you edit the destination URL anytime without reprinting the code.
              </p>
              <ul className="space-y-3">
                {["Edit links instantly", "Never reprint a code", "Track analytics"].map(f => (
                  <li key={f} className="flex items-center gap-3 text-slate-300">
                    <CheckCircle2 className="w-5 h-5 text-violet-500" />
                    {f}
                  </li>
                ))}
              </ul>
            </div>
            <div className="relative">
              <div className="absolute inset-0 bg-gradient-to-tr from-violet-600/20 to-transparent blur-3xl rounded-full" />
              <div className="relative rounded-2xl border border-white/10 bg-white/5 p-8 shadow-2xl backdrop-blur-sm">
                <div className="space-y-4">
                  <div className="h-4 bg-white/10 rounded w-1/3" />
                  <div className="h-10 bg-white/5 rounded border border-white/10 flex items-center px-4 text-sm text-slate-400">https://old-link.com</div>
                  <div className="flex justify-center my-4">
                    <ArrowRight className="w-6 h-6 text-violet-400 rotate-90" />
                  </div>
                  <div className="h-10 bg-violet-500/20 rounded border border-violet-500/30 flex items-center px-4 text-sm text-white">https://new-link.com</div>
                </div>
              </div>
            </div>
          </div>

          <div className="grid md:grid-cols-2 gap-12 items-center">
            <div className="relative order-2 md:order-1">
              <div className="absolute inset-0 bg-gradient-to-tr from-cyan-600/20 to-transparent blur-3xl rounded-full" />
              <div className="relative rounded-2xl border border-white/10 bg-white/5 p-8 shadow-2xl backdrop-blur-sm">
                <div className="flex items-end gap-2 h-40">
                  {[40, 70, 45, 90, 65, 85, 100].map((h, i) => (
                    <div key={i} className="flex-1 bg-gradient-to-t from-violet-600 to-indigo-400 rounded-t-sm opacity-80 hover:opacity-100 transition-opacity" style={{ height: `${h}%` }} />
                  ))}
                </div>
                <div className="mt-6 flex justify-between text-xs text-slate-500">
                  <span>Mon</span><span>Tue</span><span>Wed</span><span>Thu</span><span>Fri</span><span>Sat</span><span>Sun</span>
                </div>
              </div>
            </div>
            <div className="space-y-6 order-1 md:order-2">
              <div className="w-12 h-12 rounded-2xl bg-cyan-500/20 flex items-center justify-center border border-cyan-500/30">
                <BarChart3 className="w-6 h-6 text-cyan-400" />
              </div>
              <h3 className="text-2xl font-bold">Advanced Analytics</h3>
              <p className="text-slate-400 text-lg leading-relaxed">
                Know exactly how your campaigns are performing. Track scans by time, location, and device to optimize your marketing efforts.
              </p>
              <ul className="space-y-3">
                {["Location tracking", "Device statistics", "Time-based metrics"].map(f => (
                  <li key={f} className="flex items-center gap-3 text-slate-300">
                    <CheckCircle2 className="w-5 h-5 text-cyan-500" />
                    {f}
                  </li>
                ))}
              </ul>
            </div>
          </div>

          <div className="grid md:grid-cols-2 gap-12 items-center">
            <div className="space-y-6">
              <div className="w-12 h-12 rounded-2xl bg-fuchsia-500/20 flex items-center justify-center border border-fuchsia-500/30">
                <Users className="w-6 h-6 text-fuchsia-400" />
              </div>
              <h3 className="text-2xl font-bold">Team Workspaces</h3>
              <p className="text-slate-400 text-lg leading-relaxed">
                Collaborate with your entire team. Create shared workspaces, manage permissions, and organize QR codes by project or department.
              </p>
              <ul className="space-y-3">
                {["Shared folders", "Role-based access", "Activity logs"].map(f => (
                  <li key={f} className="flex items-center gap-3 text-slate-300">
                    <CheckCircle2 className="w-5 h-5 text-fuchsia-500" />
                    {f}
                  </li>
                ))}
              </ul>
            </div>
            <div className="relative">
              <div className="absolute inset-0 bg-gradient-to-tr from-fuchsia-600/20 to-transparent blur-3xl rounded-full" />
              <div className="relative rounded-2xl border border-white/10 bg-white/5 p-8 shadow-2xl backdrop-blur-sm space-y-4">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="flex items-center gap-4 p-3 rounded-lg bg-white/5 border border-white/5">
                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-slate-700 to-slate-600 border-2 border-white/10 flex items-center justify-center">
                      <Users className="w-5 h-5 text-white/50" />
                    </div>
                    <div className="flex-1">
                      <div className="h-3 bg-white/20 rounded w-1/3 mb-2" />
                      <div className="h-2 bg-white/10 rounded w-1/4" />
                    </div>
                    <div className="px-3 py-1 rounded-full bg-white/10 text-xs text-slate-300">Editor</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── How It Works ── */}
      <section className="py-24 bg-white/[0.02] border-y border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold mb-4">How it works</h2>
            <p className="text-slate-400 text-lg">Create your perfect QR code in three simple steps.</p>
          </div>

          <div className="grid md:grid-cols-3 gap-8 relative">
            <div className="hidden md:block absolute top-12 left-1/6 right-1/6 h-0.5 bg-gradient-to-r from-violet-500/50 to-fuchsia-500/50 -z-10" />

            {[
              { step: 1, title: "Choose QR Type", desc: "Select from 14+ options including URL, WiFi, vCard, PDF, and more." },
              { step: 2, title: "Customize Design", desc: "Add your logo, choose colors, and select unique patterns to match your brand." },
              { step: 3, title: "Download & Track", desc: "Download in high quality PNG/SVG and track scans with our analytics dashboard." }
            ].map(item => (
              <div key={item.step} className="text-center relative">
                <div className="w-24 h-24 mx-auto rounded-full bg-[#0d0d1a] border-2 border-violet-500/30 flex items-center justify-center shadow-[0_0_30px_-5px_rgba(139,92,246,0.3)] mb-6 relative z-10">
                  <span className="text-4xl font-black text-transparent bg-clip-text bg-gradient-to-br from-violet-400 to-fuchsia-400">{item.step}</span>
                </div>
                <h3 className="text-xl font-bold mb-2">{item.title}</h3>
                <p className="text-slate-400">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Use Cases ── */}
      <section className="py-24">
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold mb-4">Built for every industry</h2>
            <p className="text-slate-400 text-lg">See how businesses are using QRit to connect offline to online.</p>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-3 gap-6">
            {[
              { icon: <MonitorSmartphone />, title: "Restaurants", desc: "Digital menus and ordering" },
              { icon: <MapPin />, title: "Real Estate", desc: "Virtual tours and listings" },
              { icon: <Clock />, title: "Events", desc: "Ticketing and schedules" },
              { icon: <CheckCircle2 />, title: "Retail", desc: "Product info and discounts" },
              { icon: <Users />, title: "Education", desc: "Resources and attendance" },
              { icon: <BarChart3 />, title: "Marketing", desc: "Campaign tracking" },
            ].map(uc => (
              <div key={uc.title} className="p-6 rounded-2xl bg-white/5 border border-white/10 hover:bg-white/10 transition-colors group">
                <div className="w-12 h-12 rounded-xl bg-violet-500/10 text-violet-400 flex items-center justify-center mb-4 group-hover:scale-110 transition-transform">
                  {uc.icon}
                </div>
                <h4 className="text-lg font-semibold mb-1">{uc.title}</h4>
                <p className="text-sm text-slate-400">{uc.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Testimonials ── */}
      <section className="py-24 bg-white/[0.02] border-y border-white/5">
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold mb-4">Loved by our users</h2>
            <p className="text-slate-400 text-lg">Don't just take our word for it.</p>
          </div>

          <div className="grid md:grid-cols-3 gap-6">
            {[
              { name: "Sarah Jenkins", role: "Marketing Director", quote: "QRit has completely transformed how we run our print campaigns. The analytics are incredibly detailed, and dynamic QRs have saved us thousands in reprint costs." },
              { name: "Marcus Chen", role: "Restaurant Owner", quote: "We switched to digital menus using QRit last year. The customization options mean our codes perfectly match our brand, and updating the menu is instant." },
              { name: "Elena Rodriguez", role: "Event Coordinator", quote: "The easiest QR code generator I've used. Creating vCards for networking events and tracking attendance has never been smoother. Highly recommended!" }
            ].map(t => (
              <div key={t.name} className="p-8 rounded-2xl bg-[#0d0d1a] border border-white/10 relative">
                <Quote className="absolute top-6 right-6 w-8 h-8 text-white/5" />
                <div className="flex gap-1 mb-4">
                  {[1,2,3,4,5].map(i => <Star key={i} className="w-4 h-4 fill-amber-400 text-amber-400" />)}
                </div>
                <p className="text-slate-300 italic mb-6 relative z-10">"{t.quote}"</p>
                <div>
                  <h4 className="font-semibold text-white">{t.name}</h4>
                  <p className="text-sm text-slate-500">{t.role}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── FAQ ── */}
      <section className="py-24">
        <div className="max-w-3xl mx-auto px-4 sm:px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold mb-4">Frequently Asked Questions</h2>
          </div>

          <div className="space-y-4">
            {[
              { q: "What is a dynamic QR code?", a: "A dynamic QR code allows you to edit the destination URL even after the code has been printed. This means you never have to reprint materials if a link changes." },
              { q: "Is QRit free to use?", a: "Yes! You can generate static QR codes and fully customize them for free without an account. We offer paid plans for dynamic QR codes and advanced analytics." },
              { q: "Do the free QR codes expire?", a: "No, static QR codes generated on QRit never expire and have no scan limits." },
              { q: "Can I use my own logo?", a: "Yes, you can easily upload your own logo to place in the center of the QR code using our customization tools." },
              { q: "What formats can I download my QR code in?", a: "Free users can download in high-quality PNG format. Premium users can also download in vector formats like SVG and PDF." },
              { q: "How do I track QR code scans?", a: "When you create a dynamic QR code with a Pro or Enterprise plan, you get access to a dashboard showing scan locations, devices, and times." },
            ].map((faq, i) => (
              <div key={i} className="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                <button
                  onClick={() => setExpandedFaq(expandedFaq === i ? null : i)}
                  className="w-full px-6 py-4 text-left flex items-center justify-between hover:bg-white/5 transition-colors"
                >
                  <span className="font-semibold text-white">{faq.q}</span>
                  {expandedFaq === i ? <ChevronUp className="w-5 h-5 text-slate-400" /> : <ChevronDown className="w-5 h-5 text-slate-400" />}
                </button>
                {expandedFaq === i && (
                  <div className="px-6 pb-4 text-slate-400">
                    {faq.a}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Final CTA ── */}
      <section className="py-24 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-transparent to-violet-900/20" />
        <div className="max-w-4xl mx-auto px-4 sm:px-6 relative z-10 text-center">
          <h2 className="text-4xl md:text-5xl font-extrabold mb-6">Ready to create your QR code?</h2>
          <p className="text-xl text-slate-400 mb-10">Join thousands of users generating beautiful, trackable QR codes today.</p>
          <div className="flex flex-col sm:flex-row justify-center gap-4">
            <Link href="/register" className="px-8 py-4 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white font-bold rounded-xl transition-all shadow-lg shadow-violet-500/25 flex items-center justify-center gap-2">
              Get Started Free <ArrowRight className="w-5 h-5" />
            </Link>
            <button onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })} className="px-8 py-4 bg-white/10 hover:bg-white/15 border border-white/10 text-white font-bold rounded-xl transition-all">
              Create QR Code Now
            </button>
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t border-white/10 bg-[#0a0a14] py-12 px-4 sm:px-6">
        <div className="max-w-7xl mx-auto">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8 mb-12">
            <div className="col-span-2 md:col-span-1">
              <div className="flex items-center gap-2 mb-4">
                <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center">
                  <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                    <rect x="3" y="3" width="7" height="7" rx="1.5" fill="white" />
                    <rect x="14" y="3" width="7" height="7" rx="1.5" fill="white" />
                    <rect x="3" y="14" width="7" height="7" rx="1.5" fill="white" />
                  </svg>
                </div>
                <span className="text-xl font-bold text-white">QR<span className="text-violet-400">it</span></span>
              </div>
              <p className="text-sm text-slate-500 mb-6">
                The most advanced QR code generator for creators, businesses, and everyone in between.
              </p>
            </div>

            <div>
              <h4 className="font-semibold text-white mb-4">Product</h4>
              <ul className="space-y-2 text-sm text-slate-500">
                <li><Link href="/" className="hover:text-violet-400 transition-colors">QR Generator</Link></li>
                <li><Link href="/pricing" className="hover:text-violet-400 transition-colors">Pricing</Link></li>
                <li><Link href="/dashboard" className="hover:text-violet-400 transition-colors">Dashboard</Link></li>
              </ul>
            </div>

            <div>
              <h4 className="font-semibold text-white mb-4">Resources</h4>
              <ul className="space-y-2 text-sm text-slate-500">
                <li><Link href="#" className="hover:text-violet-400 transition-colors">Help Center</Link></li>
                <li><Link href="#" className="hover:text-violet-400 transition-colors">API Documentation</Link></li>
                <li><Link href="#" className="hover:text-violet-400 transition-colors">Blog</Link></li>
              </ul>
            </div>

            <div>
              <h4 className="font-semibold text-white mb-4">Company</h4>
              <ul className="space-y-2 text-sm text-slate-500">
                <li><Link href="#" className="hover:text-violet-400 transition-colors">About Us</Link></li>
                <li><Link href="#" className="hover:text-violet-400 transition-colors">Contact</Link></li>
                <li><Link href="#" className="hover:text-violet-400 transition-colors">Privacy Policy</Link></li>
                <li><Link href="#" className="hover:text-violet-400 transition-colors">Terms of Service</Link></li>
              </ul>
            </div>
          </div>

          <div className="pt-8 border-t border-white/10 flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-xs text-slate-600">© 2026 QRit. All rights reserved.</p>
            <div className="flex items-center gap-4">
              <a href="#" className="text-slate-600 hover:text-white transition-colors">
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M24 4.557c-.883.392-1.832.656-2.828.775 1.017-.609 1.798-1.574 2.165-2.724-.951.564-2.005.974-3.127 1.195-.897-.957-2.178-1.555-3.594-1.555-3.179 0-5.515 2.966-4.797 6.045-4.091-.205-7.719-2.165-10.148-5.144-1.29 2.213-.669 5.108 1.523 6.574-.806-.026-1.566-.247-2.229-.616-.054 2.281 1.581 4.415 3.949 4.89-.693.188-1.452.232-2.224.084.626 1.956 2.444 3.379 4.6 3.419-2.07 1.623-4.678 2.348-7.29 2.04 2.179 1.397 4.768 2.212 7.548 2.212 9.142 0 14.307-7.721 13.995-14.646.962-.695 1.797-1.562 2.457-2.549z"/></svg>
              </a>
              <a href="#" className="text-slate-600 hover:text-white transition-colors">
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm3.98-10.842a1.44 1.44 0 11-2.88 0 1.44 1.44 0 012.88 0z"/></svg>
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
