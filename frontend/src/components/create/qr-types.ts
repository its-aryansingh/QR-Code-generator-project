/**
 * What each kind of QR code asks for, and the text it encodes.
 *
 * The encoded strings follow what phone cameras understand: `WIFI:` for
 * networks, vCard 3.0 for contacts, `mailto:`/`SMSTO:`/`geo:` and so on.
 */
import {
  Bitcoin, CalendarDays, Contact, Link2, Mail, MapPin, MessageCircle,
  MessageSquareText, Smartphone, Type, Wifi, type LucideIcon,
} from "lucide-react";

export type QrTypeId =
  | "url" | "wifi" | "vcard" | "text" | "email" | "whatsapp"
  | "sms" | "location" | "event" | "upi" | "bitcoin";

export interface FieldDef {
  key: string;
  label: string;
  placeholder?: string;
  input?: "text" | "url" | "email" | "tel" | "number" | "password" | "datetime-local";
  multiline?: boolean;
  options?: { value: string; label: string }[];
  /** Spans the full row in the two-column grid. */
  wide?: boolean;
  autoComplete?: string;
}

export interface QrTypeDef {
  id: QrTypeId;
  label: string;
  icon: LucideIcon;
  fields: FieldDef[];
  /** Shown under the fields when it helps. */
  hint?: string;
}

export const QR_TYPES: QrTypeDef[] = [
  {
    id: "url",
    label: "Link",
    icon: Link2,
    fields: [{ key: "url", label: "Where should it go?", placeholder: "example.com/menu", input: "url", wide: true }],
  },
  {
    id: "wifi",
    label: "Wi-Fi",
    icon: Wifi,
    hint: "Phones join the network when they scan it, no typing needed.",
    fields: [
      { key: "ssid", label: "Network name", placeholder: "Cafe Guest" },
      { key: "password", label: "Password", input: "password", autoComplete: "off" },
      {
        key: "encryption",
        label: "Security",
        options: [
          { value: "WPA", label: "WPA / WPA2 / WPA3" },
          { value: "WEP", label: "WEP" },
          { value: "nopass", label: "No password" },
        ],
      },
    ],
  },
  {
    id: "vcard",
    label: "Contact",
    icon: Contact,
    hint: "Scanning offers to save this person to the phone's contacts.",
    fields: [
      { key: "firstName", label: "First name", autoComplete: "off" },
      { key: "lastName", label: "Last name", autoComplete: "off" },
      { key: "phone", label: "Phone", input: "tel", placeholder: "+91 98765 43210" },
      { key: "email", label: "Email", input: "email" },
      { key: "org", label: "Company" },
      { key: "website", label: "Website", input: "url", placeholder: "example.com" },
    ],
  },
  {
    id: "text",
    label: "Text",
    icon: Type,
    fields: [{ key: "text", label: "Text", multiline: true, wide: true, placeholder: "Anything up to a few hundred characters" }],
  },
  {
    id: "email",
    label: "Email",
    icon: Mail,
    fields: [
      { key: "to", label: "Send to", input: "email", placeholder: "hello@example.com", wide: true },
      { key: "subject", label: "Subject", wide: true },
      { key: "body", label: "Message", multiline: true, wide: true },
    ],
  },
  {
    id: "whatsapp",
    label: "WhatsApp",
    icon: MessageCircle,
    fields: [
      { key: "phone", label: "Phone number with country code", input: "tel", placeholder: "+91 98765 43210", wide: true },
      { key: "message", label: "Opening message", multiline: true, wide: true, placeholder: "Hi! I'd like to book a table." },
    ],
  },
  {
    id: "sms",
    label: "SMS",
    icon: MessageSquareText,
    fields: [
      { key: "to", label: "Phone number", input: "tel", placeholder: "+91 98765 43210", wide: true },
      { key: "message", label: "Message", multiline: true, wide: true },
    ],
  },
  {
    id: "location",
    label: "Location",
    icon: MapPin,
    hint: "Opens the spot in the phone's maps app.",
    fields: [
      { key: "lat", label: "Latitude", input: "number", placeholder: "28.6139" },
      { key: "lng", label: "Longitude", input: "number", placeholder: "77.2090" },
      { key: "label", label: "Place name", wide: true, placeholder: "India Gate" },
    ],
  },
  {
    id: "event",
    label: "Event",
    icon: CalendarDays,
    hint: "Scanning offers to add it to the phone's calendar.",
    fields: [
      { key: "title", label: "Event name", wide: true },
      { key: "start", label: "Starts", input: "datetime-local" },
      { key: "end", label: "Ends", input: "datetime-local" },
      { key: "location", label: "Where", wide: true },
    ],
  },
  {
    id: "upi",
    label: "UPI",
    icon: Smartphone,
    hint: "Opens any UPI app ready to pay.",
    fields: [
      { key: "vpa", label: "UPI ID", placeholder: "name@okbank" },
      { key: "name", label: "Payee name" },
      { key: "amount", label: "Amount in ₹ (optional)", input: "number" },
    ],
  },
  {
    id: "bitcoin",
    label: "Bitcoin",
    icon: Bitcoin,
    fields: [
      { key: "address", label: "Wallet address", wide: true },
      { key: "amount", label: "Amount in BTC (optional)", input: "number" },
    ],
  },
];

export const TYPE_BY_ID = Object.fromEntries(QR_TYPES.map((t) => [t.id, t])) as Record<QrTypeId, QrTypeDef>;

/** Only web addresses can be tracked and re-pointed after printing. */
export const TRACKABLE: ReadonlySet<QrTypeId> = new Set(["url", "whatsapp"]);

const HAS_SCHEME = /^[a-z][a-z0-9+-]*:/i;
const LOOKS_LIKE_HOST = /^(?:[a-z0-9-]+\.)+[a-z]{2,}$/i;

/** `example.com/menu` -> `https://example.com/menu` (same rule as the API). */
export function withScheme(value: string): string {
  const v = value.trim();
  if (!v || HAS_SCHEME.test(v) || /\s/.test(v)) return v;
  const host = v.split("/")[0].split("?")[0].split(":")[0];
  return LOOKS_LIKE_HOST.test(host) ? `https://${v}` : v;
}

function isWebUrl(value: string): boolean {
  try {
    const u = new URL(value);
    return (u.protocol === "https:" || u.protocol === "http:") && u.hostname.includes(".");
  } catch {
    return false;
  }
}

const vcardEscape = (s: string) => s.replace(/\\/g, "\\\\").replace(/([,;])/g, "\\$1").replace(/\n/g, "\\n");
const wifiEscape = (s: string) => s.replace(/([\\;,:"])/g, "\\$1");
const icsDate = (local: string) => (local ? local.replace(/[-:]/g, "").slice(0, 13) + "00" : "");

export type Fields = Record<string, string>;

export interface BuildResult {
  content: string;
  /** First problem to fix, if any, and the field it belongs to. */
  error?: { field: string; message: string };
}

export function buildContent(type: QrTypeId, f: Fields): BuildResult {
  const v = (k: string) => (f[k] ?? "").trim();
  switch (type) {
    case "url": {
      const url = withScheme(v("url"));
      if (!url) return { content: "", error: { field: "url", message: "Enter the address people should land on." } };
      if (!isWebUrl(url)) return { content: url, error: { field: "url", message: "That doesn't look like a web address. Try something like example.com." } };
      return { content: url };
    }
    case "wifi": {
      if (!v("ssid")) return { content: "", error: { field: "ssid", message: "Enter the network name." } };
      const enc = f.encryption || "WPA";
      const pass = enc === "nopass" ? "" : wifiEscape(f.password ?? "");
      if (enc !== "nopass" && !pass) return { content: "", error: { field: "password", message: "Enter the Wi-Fi password, or choose No password." } };
      return { content: `WIFI:T:${enc};S:${wifiEscape(v("ssid"))};P:${pass};;` };
    }
    case "vcard": {
      if (!v("firstName") && !v("lastName") && !v("org"))
        return { content: "", error: { field: "firstName", message: "Add at least a name or a company." } };
      const lines = [
        "BEGIN:VCARD",
        "VERSION:3.0",
        `N:${vcardEscape(v("lastName"))};${vcardEscape(v("firstName"))};;;`,
        `FN:${vcardEscape([v("firstName"), v("lastName")].filter(Boolean).join(" ") || v("org"))}`,
        v("org") && `ORG:${vcardEscape(v("org"))}`,
        v("phone") && `TEL;TYPE=CELL:${v("phone")}`,
        v("email") && `EMAIL:${v("email")}`,
        v("website") && `URL:${withScheme(v("website"))}`,
        "END:VCARD",
      ];
      return { content: lines.filter(Boolean).join("\n") };
    }
    case "text":
      return v("text") ? { content: v("text") } : { content: "", error: { field: "text", message: "Type the text to encode." } };
    case "email": {
      if (!v("to")) return { content: "", error: { field: "to", message: "Enter who the email goes to." } };
      const q = new URLSearchParams();
      if (v("subject")) q.set("subject", v("subject"));
      if (v("body")) q.set("body", v("body"));
      const qs = q.toString().replace(/\+/g, "%20");
      return { content: `mailto:${v("to")}${qs ? `?${qs}` : ""}` };
    }
    case "whatsapp": {
      const digits = v("phone").replace(/\D/g, "");
      if (digits.length < 8) return { content: "", error: { field: "phone", message: "Enter the full number, including the country code." } };
      return { content: `https://wa.me/${digits}${v("message") ? `?text=${encodeURIComponent(v("message"))}` : ""}` };
    }
    case "sms":
      if (!v("to")) return { content: "", error: { field: "to", message: "Enter the phone number." } };
      return { content: `SMSTO:${v("to")}:${v("message")}` };
    case "location": {
      const lat = Number(v("lat"));
      const lng = Number(v("lng"));
      if (!v("lat") || !v("lng") || Number.isNaN(lat) || Number.isNaN(lng) || Math.abs(lat) > 90 || Math.abs(lng) > 180)
        return { content: "", error: { field: "lat", message: "Enter a latitude between -90 and 90 and a longitude between -180 and 180." } };
      return { content: `geo:${lat},${lng}${v("label") ? `?q=${encodeURIComponent(v("label"))}` : ""}` };
    }
    case "event": {
      if (!v("title")) return { content: "", error: { field: "title", message: "Give the event a name." } };
      if (!v("start")) return { content: "", error: { field: "start", message: "Choose when it starts." } };
      const lines = [
        "BEGIN:VCALENDAR",
        "VERSION:2.0",
        "BEGIN:VEVENT",
        `SUMMARY:${v("title")}`,
        `DTSTART:${icsDate(v("start"))}`,
        v("end") && `DTEND:${icsDate(v("end"))}`,
        v("location") && `LOCATION:${v("location")}`,
        "END:VEVENT",
        "END:VCALENDAR",
      ];
      return { content: lines.filter(Boolean).join("\n") };
    }
    case "upi": {
      if (!/^[\w.-]+@[\w.-]+$/.test(v("vpa"))) return { content: "", error: { field: "vpa", message: "Enter a UPI ID like name@okbank." } };
      const q = new URLSearchParams({ pa: v("vpa") });
      if (v("name")) q.set("pn", v("name"));
      if (v("amount")) q.set("am", v("amount"));
      q.set("cu", "INR");
      return { content: `upi://pay?${q.toString()}` };
    }
    case "bitcoin":
      if (!v("address")) return { content: "", error: { field: "address", message: "Enter the wallet address." } };
      return { content: `bitcoin:${v("address")}${v("amount") ? `?amount=${v("amount")}` : ""}` };
  }
}

/** A sensible name when the user leaves it blank. */
export function suggestName(type: QrTypeId, f: Fields): string {
  const v = (k: string) => (f[k] ?? "").trim();
  switch (type) {
    case "url": {
      try {
        const u = new URL(withScheme(v("url")));
        const path = u.pathname.replace(/\/$/, "");
        return (u.hostname.replace(/^www\./, "") + path).slice(0, 60);
      } catch {
        return "";
      }
    }
    case "wifi": return v("ssid") ? `Wi-Fi ${v("ssid")}` : "";
    case "vcard": return [v("firstName"), v("lastName")].filter(Boolean).join(" ") || v("org");
    case "text": return v("text").split("\n")[0].slice(0, 40);
    case "email": return v("to") ? `Email ${v("to")}` : "";
    case "whatsapp": return v("phone") ? `WhatsApp ${v("phone")}` : "";
    case "sms": return v("to") ? `SMS ${v("to")}` : "";
    case "location": return v("label") || (v("lat") && v("lng") ? `${v("lat")}, ${v("lng")}` : "");
    case "event": return v("title");
    case "upi": return v("vpa") ? `UPI ${v("vpa")}` : "";
    case "bitcoin": return v("address") ? `Bitcoin ${v("address").slice(0, 10)}…` : "";
  }
}

export const DEFAULT_FIELDS: Record<QrTypeId, Fields> = {
  url: { url: "" },
  wifi: { ssid: "", password: "", encryption: "WPA" },
  vcard: { firstName: "", lastName: "", phone: "", email: "", org: "", website: "" },
  text: { text: "" },
  email: { to: "", subject: "", body: "" },
  whatsapp: { phone: "", message: "" },
  sms: { to: "", message: "" },
  location: { lat: "", lng: "", label: "" },
  event: { title: "", start: "", end: "", location: "" },
  upi: { vpa: "", name: "", amount: "" },
  bitcoin: { address: "", amount: "" },
};
