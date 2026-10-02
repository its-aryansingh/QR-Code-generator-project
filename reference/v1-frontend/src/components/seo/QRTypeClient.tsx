"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { API_URL } from "@/lib/api";
import { CreateQRPanel } from "@/components/dashboard/CreateQRPanel";
import { qrTypes, QRIcons } from "@/utils/qrConstants";
import Link from "next/link";

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
      return `mailto:${e.to}${qs ? "?" + qs : ""}`;
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
      return `geo:${l.lat},${l.lng}${l.name ? "?q=" + encodeURIComponent(l.name) : ""}`;
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
      return `bitcoin:${b.address}${params.length ? "?" + params.join("&") : ""}`;
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
      return `https://wa.me/${num}${wa.message ? "?text=" + encodeURIComponent(wa.message) : ""}`;
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

function toQrDataUrl(raw: string | null | undefined): string | null {
  if (!raw || typeof raw !== "string") return null;
  const t = raw.trim();
  if (!t) return null;
  return t.startsWith("data:") ? t : `data:image/png;base64,${t}`;
}

export function QRTypeClient({ type }: { type: string }) {
  const [isDynamicQR, setIsDynamicQR] = useState(false);
  const [content, setContent] = useState(type === "url" ? "https://example.com" : "");
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

  const [qrImage, setQrImage] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const currentContent = buildContent(
    type, content, vcardData, emailData, smsData, wifiData, locationData,
    eventData, socialData, bitcoinData, upiData, whatsappData, mecardData, multiLinkData
  );

  const generateQR = useCallback(async (qrContent: string) => {
    if (!qrContent) {
      setQrImage(null);
      return;
    }
    setGenerating(true);
    const showLocalPreview = async () => {
      try {
        const { getLocalQrDataUrl } = await import("@/lib/localQrPreview");
        setQrImage(await getLocalQrDataUrl(qrContent, 220));
      } catch {
        // ignore
      }
    };
    try {
      await showLocalPreview();
      const res = await fetch(`${API_URL}/public/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: qrContent, qr_type: type, size: 200 }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok && data.success && data.qr_base64) {
        const serverImg = toQrDataUrl(data.qr_base64);
        if (serverImg) setQrImage(serverImg);
      }
    } catch {
      await showLocalPreview();
    } finally {
      setGenerating(false);
    }
  }, [type]);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => generateQR(currentContent), 300);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [currentContent, generateQR]);

  const handleDownload = () => {
    if (!qrImage) return;
    const a = document.createElement("a");
    a.href = qrImage;
    a.download = `qrit-${type}-qrcode.png`;
    a.click();
  };

  const typeInfo = qrTypes.find(t => t.id === type);

  return (
    <div className="grid lg:grid-cols-[1fr_340px] gap-6 max-w-5xl mx-auto my-8 relative z-10 text-left">
      <div className="space-y-5">
        <div className="p-5 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm">
          <div className="flex items-center justify-between mb-5">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-white/10 flex items-center justify-center">
                {QRIcons[type]}
              </div>
              <div>
                <h2 className="text-sm font-semibold text-white">
                  {typeInfo?.name}
                </h2>
                <p className="text-xs text-slate-500">
                  {typeInfo?.description}
                </p>
              </div>
            </div>
          </div>
          <CreateQRPanel
            selectedType={type}
            setSelectedType={() => {}}
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
      </div>

      <div className="lg:sticky lg:top-24 lg:self-start space-y-4">
        <div className="p-6 rounded-2xl bg-white/5 border border-white/10 backdrop-blur-sm flex flex-col items-center gap-5">
          <h2 className="self-start text-xs font-semibold text-slate-500 uppercase tracking-widest">
            Live Preview
          </h2>
          <div className="w-56 h-56 rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center overflow-hidden">
            {generating ? (
              <div className="flex flex-col items-center gap-3">
                <div className="w-10 h-10 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
                <span className="text-xs text-slate-600">Generating…</span>
              </div>
            ) : qrImage ? (
              <img src={qrImage} alt={`${typeInfo?.name} QR Code`} className="w-full h-full object-contain p-3" />
            ) : (
              <div className="text-center px-4">
                <svg className="w-16 h-16 mx-auto mb-3 text-slate-800" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M3 3h7v7H3V3zm1 1v5h5V4H4zm1 1h3v3H5V5zm9-2h7v7h-7V3zm1 1v5h5V4h-5zm1 1h3v3h-3V5zM3 14h7v7H3v-7zm1 1v5h5v-5H4zm1 1h3v3H5v-3zm9 0h2v2h-2v-2zm3-1h1v1h-1v-1zm1 2h1v1h-1v-1zm-4 2h1v3h-1v-3zm2 0h2v1h-2v-1zm1 1h1v2h-1v-2zm2-2h1v1h-1v-1zm0 2h1v2h-1v-2z" />
                </svg>
                <p className="text-xs text-slate-700 leading-relaxed">Fill in the form to generate</p>
              </div>
            )}
          </div>
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
            </div>
          )}
        </div>
        <div className="p-4 rounded-2xl bg-gradient-to-br from-violet-600/10 to-indigo-600/5 border border-violet-500/20 text-center">
            <p className="text-sm font-semibold text-violet-300 mb-1">Want to track scans?</p>
            <p className="text-xs text-slate-500 mb-3">Create an account for dynamic QRs & analytics.</p>
            <Link
            href="/register"
            className="inline-block py-2 px-4 bg-violet-600/30 hover:bg-violet-600/40 border border-violet-500/30 text-violet-300 hover:text-violet-200 text-xs font-semibold rounded-lg transition-colors"
            >
            Get Started Free →
            </Link>
        </div>
      </div>
    </div>
  );
}
