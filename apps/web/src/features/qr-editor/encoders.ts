const VPA_REGEX = /^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$/;

export function encodeUrl(raw: string): string {
  return raw.trim();
}

export function encodeText(text: string): string {
  if (text.length > 1000) {
    throw new Error('text exceeds 1000 characters');
  }
  return text;
}

export function encodeEmail(to: string, subject?: string, body?: string): string {
  const cleanTo = to.trim();
  const params = new URLSearchParams();
  if (subject) params.set('subject', subject);
  if (body) params.set('body', body);
  const qs = params.toString();
  return qs ? `mailto:${cleanTo}?${qs}` : `mailto:${cleanTo}`;
}

export function encodePhone(phone: string): string {
  return `tel:${phone.trim()}`;
}

export function encodeSms(phone: string, message?: string): string {
  return `SMSTO:${phone.trim()}:${message || ''}`;
}

export function encodeWhatsApp(phone: string, text?: string): string {
  const digits = phone.replace(/[^0-9]+/g, '');
  if (text) {
    return `https://wa.me/${digits}?text=${encodeURIComponent(text)}`;
  }
  return `https://wa.me/${digits}`;
}

export function escapeWiFi(s: string): string {
  let res = '';
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (['\\', ';', ',', ':', '"'].includes(c)) {
      res += '\\' + c;
    } else {
      res += c;
    }
  }
  return res;
}

export function encodeWiFi(ssid: string, password: string = '', authType: string = 'WPA', hidden: boolean = false): string {
  let auth = (authType || '').trim().toUpperCase();
  if (!auth || auth === 'NONE') {
    auth = 'nopass';
  }
  const hiddenStr = hidden ? 'true' : 'false';
  return `WIFI:T:${auth};S:${escapeWiFi(ssid)};P:${escapeWiFi(password)};H:${hiddenStr};;`;
}

export function escapeVCard(s: string): string {
  let res = '';
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (['\\', ';', ','].includes(c)) {
      res += '\\' + c;
    } else if (c === '\n') {
      res += '\\n';
    } else if (c === '\r') {
      // skip CR
    } else {
      res += c;
    }
  }
  return res;
}

export interface VCardData {
  firstName: string;
  lastName: string;
  company?: string;
  title?: string;
  phone?: string;
  email?: string;
  website?: string;
  street?: string;
  city?: string;
  state?: string;
  postalCode?: string;
  country?: string;
  note?: string;
}

export function encodeVCard(data: VCardData): string {
  const crlf = '\r\n';
  let s = 'BEGIN:VCARD' + crlf + 'VERSION:3.0' + crlf;

  const fullName = `${data.firstName || ''} ${data.lastName || ''}`.trim();
  if (fullName) {
    s += `FN:${escapeVCard(fullName)}${crlf}`;
  }

  s += `N:${escapeVCard(data.lastName || '')};${escapeVCard(data.firstName || '')};;;${crlf}`;

  if (data.company) s += `ORG:${escapeVCard(data.company)}${crlf}`;
  if (data.title) s += `TITLE:${escapeVCard(data.title)}${crlf}`;
  if (data.phone) s += `TEL;TYPE=CELL:${escapeVCard(data.phone)}${crlf}`;
  if (data.email) s += `EMAIL;TYPE=INTERNET:${escapeVCard(data.email)}${crlf}`;
  if (data.website) s += `URL:${escapeVCard(data.website)}${crlf}`;

  if (data.street || data.city || data.state || data.postalCode || data.country) {
    s += `ADR;TYPE=WORK:;;${escapeVCard(data.street || '')};${escapeVCard(data.city || '')};${escapeVCard(data.state || '')};${escapeVCard(data.postalCode || '')};${escapeVCard(data.country || '')}${crlf}`;
  }

  if (data.note) s += `NOTE:${escapeVCard(data.note)}${crlf}`;

  s += 'END:VCARD' + crlf;
  return s;
}

export interface EventData {
  title: string;
  description?: string;
  location?: string;
  start: Date;
  end: Date;
}

function formatUtcDate(d: Date): string {
  return d.toISOString().replace(/[-:]/g, '').split('.')[0] + 'Z';
}

export function encodeEvent(evt: EventData): string {
  const crlf = '\r\n';
  let s = 'BEGIN:VCALENDAR' + crlf + 'VERSION:2.0' + crlf + 'BEGIN:VEVENT' + crlf;
  if (evt.title) s += `SUMMARY:${escapeVCard(evt.title)}${crlf}`;
  if (evt.description) s += `DESCRIPTION:${escapeVCard(evt.description)}${crlf}`;
  if (evt.location) s += `LOCATION:${escapeVCard(evt.location)}${crlf}`;
  s += `DTSTART:${formatUtcDate(evt.start)}${crlf}`;
  s += `DTEND:${formatUtcDate(evt.end)}${crlf}`;
  s += 'END:VEVENT' + crlf + 'END:VCALENDAR' + crlf;
  return s;
}

export function encodeUpi(vpa: string, name?: string, amount?: number, note?: string): string {
  const cleanVpa = vpa.trim();
  if (!VPA_REGEX.test(cleanVpa)) {
    throw new Error('invalid UPI VPA address');
  }

  const params = new URLSearchParams();
  params.set('pa', cleanVpa);
  if (name) params.set('pn', name);
  if (amount !== undefined && amount > 0) {
    params.set('am', amount.toFixed(2));
  }
  params.set('cu', 'INR');
  if (note) params.set('tn', note);

  return `upi://pay?${params.toString()}`;
}

export function encodeLocation(lat: number, lng: number): string {
  if (lat < -90 || lat > 90 || lng < -180 || lng > 180) {
    throw new Error('invalid latitude or longitude');
  }
  return `geo:${lat.toFixed(6)},${lng.toFixed(6)}`;
}

export function validateAndPadGtin14(raw: string): string {
  let digits = raw.trim();
  if (!/^\d+$/.test(digits)) {
    throw new Error('invalid GS1 GTIN format or checksum');
  }

  switch (digits.length) {
    case 8:
    case 12:
    case 13:
      digits = digits.padStart(14, '0');
      break;
    case 14:
      break;
    default:
      throw new Error('invalid GS1 GTIN format or checksum');
  }

  let sum = 0;
  for (let i = 0; i < 13; i++) {
    const d = parseInt(digits[i], 10);
    const weight = i % 2 === 0 ? 3 : 1;
    sum += d * weight;
  }

  const checkDigit = (10 - (sum % 10)) % 10;
  if (checkDigit !== parseInt(digits[13], 10)) {
    throw new Error('invalid GS1 GTIN format or checksum');
  }

  return digits;
}

export function formatGs1Payload(host: string, gtin14: string): string {
  const cleanHost = host.replace(/\/+$/, '');
  return `HTTPS://${cleanHost.toUpperCase()}/01/${gtin14}`;
}
