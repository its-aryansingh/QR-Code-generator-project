"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/auth";
import { QRSecurityPanel, SecurityOptions } from "@/components/dashboard/QRSecurityPanel";
import { CustomizeQRPanel } from "@/components/dashboard/CustomizeQRPanel";
import {
  Link, Wifi, User, Mail, MessageSquare, MapPin, Calendar,
  FileText, Bitcoin, Smartphone, QrCode, ChevronRight, Check,
  Folder, AlertCircle
} from "lucide-react";

const QR_TYPES = [
  { id: "url",      label: "URL",        icon: Link,          desc: "Website or landing page" },
  { id: "wifi",     label: "Wi-Fi",      icon: Wifi,          desc: "Network credentials" },
  { id: "vcard",    label: "vCard",      icon: User,          desc: "Contact card" },
  { id: "email",    label: "Email",      icon: Mail,          desc: "Pre-filled email" },
  { id: "sms",      label: "SMS",        icon: MessageSquare, desc: "Pre-filled text message" },
  { id: "location", label: "Location",   icon: MapPin,        desc: "GPS coordinates" },
  { id: "event",    label: "Event",      icon: Calendar,      desc: "Calendar event" },
  { id: "text",     label: "Plain Text", icon: FileText,      desc: "Any text content" },
  { id: "bitcoin",  label: "Bitcoin",    icon: Bitcoin,       desc: "Crypto payment address" },
  { id: "upi",      label: "UPI",        icon: Smartphone,    desc: "India UPI payment" },
  { id: "whatsapp", label: "WhatsApp",   icon: MessageSquare, desc: "WhatsApp chat link" },
];

const STEPS = ["Type", "Content", "Security", "Customize", "Review"];

interface FolderItem { id: string; name: string; parentId?: string }
interface WorkspaceItem { id: string; name: string }

const emptyContent: Record<string, Record<string, string>> = {
  url:      { url: "" },
  wifi:     { ssid: "", password: "", encryption: "WPA" },
  vcard:    { firstName: "", lastName: "", org: "", phone: "", email: "", website: "" },
  email:    { to: "", subject: "", body: "" },
  sms:      { to: "", message: "" },
  location: { lat: "", lng: "", label: "" },
  event:    { title: "", start: "", end: "", location: "" },
  text:     { text: "" },
  bitcoin:  { address: "", amount: "" },
  upi:      { vpa: "", name: "", amount: "" },
  whatsapp: { phone: "", message: "" },
};

function buildContent(type: string, fields: Record<string, string>): string {
  switch (type) {
    case "url":      return fields.url;
    case "wifi":     return `WIFI:T:${fields.encryption};S:${fields.ssid};P:${fields.password};;`;
    case "vcard":    return `BEGIN:VCARD\nVERSION:3.0\nFN:${fields.firstName} ${fields.lastName}\nORG:${fields.org}\nTEL:${fields.phone}\nEMAIL:${fields.email}\nURL:${fields.website}\nEND:VCARD`;
    case "email":    return `mailto:${fields.to}?subject=${encodeURIComponent(fields.subject)}&body=${encodeURIComponent(fields.body)}`;
    case "sms":      return `SMSTO:${fields.to}:${fields.message}`;
    case "location": return `geo:${fields.lat},${fields.lng}${fields.label ? `?q=${encodeURIComponent(fields.label)}` : ""}`;
    case "event":    return `BEGIN:VEVENT\nSUMMARY:${fields.title}\nDTSTART:${fields.start.replace(/[-:T]/g, "").slice(0, 15)}\nDTEND:${fields.end.replace(/[-:T]/g, "").slice(0, 15)}\nLOCATION:${fields.location}\nEND:VEVENT`;
    case "text":     return fields.text;
    case "bitcoin":  return `bitcoin:${fields.address}${fields.amount ? `?amount=${fields.amount}` : ""}`;
    case "upi":      return `upi://pay?pa=${fields.vpa}&pn=${encodeURIComponent(fields.name)}&am=${fields.amount}`;
    case "whatsapp": return `https://wa.me/${fields.phone.replace(/\D/g, "")}${fields.message ? `?text=${encodeURIComponent(fields.message)}` : ""}`;
    default:         return "";
  }
}

const inputCls = "w-full bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2.5 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500";
const labelCls = "block text-xs text-zinc-500 mb-1.5";

function ContentFields({ type, fields, onChange }: { type: string; fields: Record<string, string>; onChange: (k: string, v: string) => void }) {
  const f = (key: string, label: string, placeholder = "", inputType = "text") => (
    <div key={key}>
      <label className={labelCls}>{label}</label>
      <input type={inputType} value={fields[key] ?? ""} onChange={(e) => onChange(key, e.target.value)} placeholder={placeholder} className={inputCls} />
    </div>
  );
  const ta = (key: string, label: string, placeholder = "") => (
    <div key={key}>
      <label className={labelCls}>{label}</label>
      <textarea value={fields[key] ?? ""} onChange={(e) => onChange(key, e.target.value)} placeholder={placeholder} rows={3} className={inputCls + " resize-none"} />
    </div>
  );

  switch (type) {
    case "url":      return <>{f("url", "Website URL", "https://example.com", "url")}</>;
    case "wifi":     return <>{f("ssid", "Network Name (SSID)", "MyNetwork")}{f("password", "Password", "••••••••", "password")}<div><label className={labelCls}>Encryption</label><select value={fields.encryption} onChange={(e) => onChange("encryption", e.target.value)} className={inputCls}><option value="WPA">WPA/WPA2</option><option value="WEP">WEP</option><option value="nopass">None</option></select></div></>;
    case "vcard":    return <div className="grid sm:grid-cols-2 gap-3">{f("firstName","First Name","John")}{f("lastName","Last Name","Doe")}{f("org","Organization","Acme Inc.")}{f("phone","Phone","+1 555 0123","tel")}{f("email","Email","john@acme.com","email")}{f("website","Website","https://acme.com","url")}</div>;
    case "email":    return <>{f("to","To Email","recipient@example.com","email")}{f("subject","Subject","Hello")}{ta("body","Body","Your message here…")}</>;
    case "sms":      return <>{f("to","Phone Number","+1 555 0123","tel")}{ta("message","Message","Your message here…")}</>;
    case "location": return <div className="grid sm:grid-cols-2 gap-3">{f("lat","Latitude","37.7749","number")}{f("lng","Longitude","-122.4194","number")}<div className="sm:col-span-2">{f("label","Label (optional)","Acme HQ")}</div></div>;
    case "event":    return <>{f("title","Event Title","Team Meeting")}<div className="grid sm:grid-cols-2 gap-3">{f("start","Start","","datetime-local")}{f("end","End","","datetime-local")}</div>{f("location","Location (optional)","Conference Room A")}</>;
    case "text":     return <>{ta("text","Plain Text","Enter any text…")}</>;
    case "bitcoin":  return <>{f("address","Bitcoin Address","1A1zP1eP5QGefi2DMPTfTL5SLmv7Divf")}{f("amount","Amount (BTC, optional)","0.001","number")}</>;
    case "upi":      return <>{f("vpa","UPI VPA","name@upi")}{f("name","Payee Name","John Doe")}{f("amount","Amount (₹, optional)","100","number")}</>;
    case "whatsapp": return <>{f("phone","Phone (with country code)","+1 555 0123","tel")}{ta("message","Pre-filled Message (optional)","Hi there!")}</>;
    default:         return null;
  }
}

export default function CreatePage() {
  const router = useRouter();
  const { accessToken } = useAuthStore();
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [step, setStep] = useState(0);
  const [qrType, setQrType] = useState("url");
  const [title, setTitle] = useState("");
  const [isDynamic, setIsDynamic] = useState(true);
  const [fields, setFields] = useState<Record<string, string>>(emptyContent["url"]);
  const [security, setSecurity] = useState<SecurityOptions>({ password: "", expiresAt: "", maxScans: "", geoRestrictions: "" });
  const [workspaceId, setWorkspaceId] = useState("");
  const [folderId, setFolderId] = useState("");
  const [workspaces, setWorkspaces] = useState<WorkspaceItem[]>([]);
  const [folders, setFolders] = useState<FolderItem[]>([]);
  const [preview, setPreview] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  // Customization state
  const [designTab, setDesignTab] = useState("templates");
  const [selectedDotStyle, setSelectedDotStyle] = useState("square");
  const [selectedCornerStyle, setSelectedCornerStyle] = useState("square");
  const [selectedFrame, setSelectedFrame] = useState("none");
  const [eyeInnerStyle, setEyeInnerStyle] = useState("square");
  const [quietZone, setQuietZone] = useState(20);
  const [errorLevel, setErrorLevel] = useState("M");
  const [qrColor, setQrColor] = useState("#000000");
  const [bgColor, setBgColor] = useState("#FFFFFF");
  const [useGradient, setUseGradient] = useState(false);
  const [gradientStart, setGradientStart] = useState("#4F46E5");
  const [gradientEnd, setGradientEnd] = useState("#EC4899");
  const [gradientType, setGradientType] = useState<"linear" | "radial">("linear");
  const [gradientRotation, setGradientRotation] = useState(135);
  const [transparentBg, setTransparentBg] = useState(false);
  const [logoFile, setLogoFile] = useState<string | null>(null);
  const [logoSize, setLogoSize] = useState(0.2);
  const [logoMargin, setLogoMargin] = useState(5);
  const [bgImage, setBgImage] = useState<string | null>(null);

  useEffect(() => {
    if (!accessToken) return;
    fetch(`${api}/workspaces`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((d) => {
        if (d.success) {
          setWorkspaces(d.data ?? []);
          const stored = localStorage.getItem("qrit_active_workspace");
          const initial = stored ?? d.data?.[0]?.id ?? "";
          setWorkspaceId(initial);
        }
      })
      .catch(() => {});
  }, [accessToken]);

  useEffect(() => {
    if (!workspaceId || !accessToken) return;
    fetch(`${api}/workspaces/${workspaceId}/folders`, { headers: { Authorization: `Bearer ${accessToken}` } })
      .then((r) => r.json())
      .then((d) => { if (d.success) setFolders(d.data ?? []); })
      .catch(() => {});
  }, [workspaceId, accessToken]);

  const handleTypeSelect = (t: string) => {
    setQrType(t);
    setFields({ ...emptyContent[t] });
    setStep(1);
  };

  const handleFieldChange = (k: string, v: string) => setFields((prev) => ({ ...prev, [k]: v }));

  const generatePreview = useCallback(async () => {
    if (!accessToken) return;
    const content = buildContent(qrType, fields);
    if (!content) return;
    try {
      const res = await fetch(`${api}/qr/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({ title: title || content.slice(0, 40), content, qr_type: qrType, size: 300, is_dynamic: false }),
      });
      const data = await res.json();
      if (data.success) setPreview(data.data.qr_base64);
    } catch {}
  }, [accessToken, qrType, fields, title]);

  useEffect(() => {
    if (step === 4) generatePreview();
  }, [step, generatePreview]);

  const handleSubmit = async () => {
    setSubmitting(true);
    setError("");
    const content = buildContent(qrType, fields);
    const body: Record<string, unknown> = {
      title: title || content.slice(0, 60),
      content,
      qr_type: qrType,
      size: 512,
      is_dynamic: isDynamic,
      workspace_id: workspaceId || undefined,
      folder_id: folderId || undefined,
    };
    if (security.password) body.password = security.password;
    if (security.expiresAt) body.expires_at = new Date(security.expiresAt).toISOString();
    if (security.maxScans) body.max_scans = parseInt(security.maxScans);
    if (security.geoRestrictions) body.geo_restrictions = security.geoRestrictions;

    // Add customization
    const customization: Record<string, unknown> = {
      foreground_color: qrColor,
      background_color: transparentBg ? "transparent" : bgColor,
      body_style: selectedDotStyle,
      corner_style: selectedCornerStyle,
    };
    if (useGradient) {
      customization.gradient = { type: gradientType, start_color: gradientStart, end_color: gradientEnd, rotation: gradientRotation };
    }
    if (selectedFrame !== "none") {
      customization.frame = { style: selectedFrame };
    }
    if (logoFile) {
      customization.logo = { url: logoFile, size: logoSize };
    }
    body.customization = customization;

    try {
      const res = await fetch(`${api}/qr/generate`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.success) {
        router.push("/dashboard/qr-codes");
      } else {
        setError(data.error ?? "Failed to create QR code");
      }
    } catch {
      setError("Network error. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-3xl space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-zinc-100">Create QR Code</h1>
        <p className="text-sm text-zinc-500 mt-1">Generate a QR code with enterprise security and tracking.</p>
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-1">
        {STEPS.map((s, i) => (
          <div key={s} className="flex items-center gap-1">
            <button
              onClick={() => i < step && setStep(i)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                i === step ? "bg-violet-600 text-white" :
                i < step ? "bg-zinc-800 text-zinc-300 hover:bg-zinc-700 cursor-pointer" :
                "bg-zinc-900 text-zinc-600 cursor-default"
              }`}
            >
              {i < step ? <Check size={11} /> : <span>{i + 1}</span>}
              {s}
            </button>
            {i < STEPS.length - 1 && <ChevronRight size={14} className="text-zinc-700" />}
          </div>
        ))}
      </div>

      {/* Step 0: Type selector */}
      {step === 0 && (
        <div className="grid sm:grid-cols-2 gap-2">
          {QR_TYPES.map(({ id, label, icon: Icon, desc }) => (
            <button
              key={id}
              onClick={() => handleTypeSelect(id)}
              className="flex items-center gap-3 p-4 bg-zinc-900 border border-zinc-800 rounded-xl text-left hover:border-violet-600 hover:bg-zinc-800/60 transition-all group"
            >
              <div className="w-9 h-9 rounded-lg bg-zinc-800 group-hover:bg-violet-900/40 flex items-center justify-center shrink-0">
                <Icon size={18} className="text-zinc-400 group-hover:text-violet-400" />
              </div>
              <div>
                <p className="text-sm font-medium text-zinc-200">{label}</p>
                <p className="text-xs text-zinc-500">{desc}</p>
              </div>
            </button>
          ))}
        </div>
      )}

      {/* Step 1: Content */}
      {step === 1 && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-2 mb-2">
            <QrCode size={16} className="text-violet-400" />
            <span className="text-sm font-semibold text-zinc-300">
              {QR_TYPES.find((t) => t.id === qrType)?.label} Content
            </span>
          </div>

          <div>
            <label className={labelCls}>Title (optional)</label>
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="My QR Code" className={inputCls} />
          </div>

          <ContentFields type={qrType} fields={fields} onChange={handleFieldChange} />

          <div className="flex items-center gap-3 pt-2">
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={isDynamic} onChange={(e) => setIsDynamic(e.target.checked)} className="w-4 h-4 rounded accent-violet-500" />
              <span className="text-sm text-zinc-300">Dynamic QR (editable after creation)</span>
            </label>
          </div>

          {/* Workspace / folder */}
          <div className="grid sm:grid-cols-2 gap-3 pt-1 border-t border-zinc-800">
            <div>
              <label className={labelCls + " flex items-center gap-1.5"}><Folder size={11} /> Workspace</label>
              <select value={workspaceId} onChange={(e) => setWorkspaceId(e.target.value)} className={inputCls}>
                <option value="">— No workspace —</option>
                {workspaces.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}
              </select>
            </div>
            <div>
              <label className={labelCls + " flex items-center gap-1.5"}><Folder size={11} /> Folder (optional)</label>
              <select value={folderId} onChange={(e) => setFolderId(e.target.value)} className={inputCls} disabled={!folders.length}>
                <option value="">— Root —</option>
                {folders.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
              </select>
            </div>
          </div>

          <div className="flex justify-between pt-2">
            <button onClick={() => setStep(0)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-200">← Back</button>
            <button
              onClick={() => setStep(2)}
              disabled={!buildContent(qrType, fields)}
              className="px-5 py-2 bg-violet-600 hover:bg-violet-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg"
            >
              Next: Security →
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Security */}
      {step === 2 && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-5">
          <QRSecurityPanel value={security} onChange={setSecurity} />
          <div className="flex justify-between pt-2 border-t border-zinc-800">
            <button onClick={() => setStep(1)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-200">← Back</button>
            <button
              onClick={() => setStep(3)}
              className="px-5 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg"
            >
              Next: Customize →
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Customize */}
      {step === 3 && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-5">
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
          <div className="flex justify-between pt-2 border-t border-zinc-800">
            <button onClick={() => setStep(2)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-200">← Back</button>
            <button
              onClick={() => setStep(4)}
              className="px-5 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm font-medium rounded-lg"
            >
              Review →
            </button>
          </div>
        </div>
      )}

      {/* Step 4: Review */}
      {step === 4 && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 space-y-5">
          <div className="flex gap-6">
            {/* Preview */}
            <div className="shrink-0">
              {preview ? (
                <img src={preview} alt="QR preview" className="w-36 h-36 rounded-lg border border-zinc-700" />
              ) : (
                <div className="w-36 h-36 rounded-lg border border-zinc-700 bg-zinc-800 flex items-center justify-center">
                  <div className="w-6 h-6 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
                </div>
              )}
            </div>

            {/* Summary */}
            <div className="flex-1 space-y-2 text-sm">
              <Row label="Type" value={QR_TYPES.find((t) => t.id === qrType)?.label ?? qrType} />
              <Row label="Title" value={title || "(auto)"} />
              <Row label="Mode" value={isDynamic ? "Dynamic" : "Static"} />
              <Row label="Workspace" value={workspaces.find((w) => w.id === workspaceId)?.name ?? "—"} />
              {security.password && <Row label="Password" value="Protected ✓" />}
              {security.expiresAt && <Row label="Expires" value={new Date(security.expiresAt).toLocaleString()} />}
              {security.maxScans && <Row label="Max scans" value={security.maxScans} />}
              {security.geoRestrictions && <Row label="Geo" value={security.geoRestrictions} />}
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-950/40 border border-red-800 rounded-lg text-sm text-red-400">
              <AlertCircle size={14} /> {error}
            </div>
          )}

          <div className="flex justify-between pt-2 border-t border-zinc-800">
            <button onClick={() => setStep(3)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-200">← Back</button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="px-6 py-2 bg-violet-600 hover:bg-violet-500 disabled:opacity-60 text-white text-sm font-semibold rounded-lg flex items-center gap-2"
            >
              {submitting && <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />}
              {submitting ? "Creating…" : "Create QR Code"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-zinc-500 w-24 shrink-0">{label}</span>
      <span className="text-zinc-200">{value}</span>
    </div>
  );
}
