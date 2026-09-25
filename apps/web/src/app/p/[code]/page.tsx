'use client';

import React from 'react';
import {
  User,
  Phone,
  Mail,
  Globe,
  MapPin,
  Calendar,
  Download,
  ExternalLink,
  FileText,
} from 'lucide-react';

interface VCardPayload {
  first_name: string;
  last_name: string;
  org?: string;
  title?: string;
  phones?: { type: string; value: string }[];
  emails?: { type: string; value: string }[];
  website?: string;
  address?: { street?: string; city?: string; country?: string };
  note?: string;
}

interface LinksPagePayload {
  title: string;
  bio?: string;
  links: { label: string; url: string; icon?: string }[];
}

interface FilePayload {
  title: string;
  file_url: string;
  file_type?: string;
}

interface EventPayload {
  title: string;
  description?: string;
  location?: string;
  starts_at: string;
  ends_at: string;
}

interface HostedPageData {
  kind: 'vcard' | 'links_page' | 'file' | 'event';
  theme?: {
    accent?: string;
    background?: string;
  };
  vcard?: VCardPayload;
  links_page?: LinksPagePayload;
  file?: FilePayload;
  event?: EventPayload;
}

export default function HostedPage({
  params,
}: {
  params: Promise<{ code: string }>;
}) {
  const { code } = React.use(params);

  // Demo fallback hosted data
  const data: HostedPageData = {
    kind: 'vcard',
    theme: { accent: '#5B5BF7', background: '#FFFFFF' },
    vcard: {
      first_name: 'Aryan',
      last_name: 'Singh',
      org: 'QRit Technologies',
      title: 'Founder & Architect',
      phones: [{ type: 'cell', value: '+1 234 567 8900' }],
      emails: [{ type: 'work', value: 'aryan@example.com' }],
      website: 'https://qrit.io',
      address: { city: 'Bengaluru', country: 'India' },
      note: 'Connect with me on QRit',
    },
  };

  const handleDownloadVcf = () => {
    if (!data.vcard) return;
    const v = data.vcard;
    const crlf = '\r\n';
    const vcf = [
      'BEGIN:VCARD',
      'VERSION:3.0',
      `FN:${v.first_name} ${v.last_name}`,
      `N:${v.last_name};${v.first_name};;;`,
      v.org ? `ORG:${v.org}` : '',
      v.title ? `TITLE:${v.title}` : '',
      ...(v.phones || []).map((p) => `TEL;TYPE=${p.type.toUpperCase()}:${p.value}`),
      ...(v.emails || []).map((e) => `EMAIL;TYPE=${e.type.toUpperCase()}:${e.value}`),
      v.website ? `URL:${v.website}` : '',
      'END:VCARD',
    ]
      .filter(Boolean)
      .join(crlf);

    const blob = new Blob([vcf], { type: 'text/vcard;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${v.first_name}_${v.last_name}.vcf`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="min-h-screen bg-[var(--bg-subtle)] flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-[var(--surface)] rounded-3xl border border-[var(--border)] shadow-xl overflow-hidden">
        {/* Cover / Header Banner */}
        <div
          className="h-32 w-full flex items-end justify-center pb-4"
          style={{ backgroundColor: data.theme?.accent || 'var(--accent)' }}
        >
          <div className="w-20 h-20 -mb-10 rounded-full bg-[var(--surface)] p-1.5 shadow-md">
            <div className="w-full h-full rounded-full bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center font-bold text-2xl">
              {data.vcard ? data.vcard.first_name[0] : <User className="w-8 h-8" />}
            </div>
          </div>
        </div>

        {/* Content Details */}
        <div className="pt-14 pb-8 px-6 text-center space-y-6">
          {data.kind === 'vcard' && data.vcard && (
            <>
              <div>
                <h1 className="text-xl font-extrabold text-[var(--text)]">
                  {data.vcard.first_name} {data.vcard.last_name}
                </h1>
                {data.vcard.title && (
                  <p className="text-xs text-[var(--text-muted)] font-medium mt-0.5">
                    {data.vcard.title} {data.vcard.org ? `at ${data.vcard.org}` : ''}
                  </p>
                )}
              </div>

              {/* Action Buttons */}
              <div className="space-y-2">
                <button
                  onClick={handleDownloadVcf}
                  className="w-full py-3 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-bold text-xs flex items-center justify-center gap-2 shadow-sm hover:opacity-95 transition-opacity"
                >
                  <Download className="w-4 h-4" />
                  Save to Contacts (.vcf)
                </button>
              </div>

              {/* Contact list items */}
              <div className="text-left space-y-3 pt-2 border-t border-[var(--border)] text-xs">
                {data.vcard.phones?.map((p, i) => (
                  <a
                    key={i}
                    href={`tel:${p.value}`}
                    className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-subtle)] hover:bg-[var(--border)]/40 transition-colors"
                  >
                    <div className="w-8 h-8 rounded-lg bg-[var(--surface)] flex items-center justify-center text-[var(--accent)]">
                      <Phone className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-[10px] text-[var(--text-muted)] uppercase">{p.type}</div>
                      <div className="font-semibold text-[var(--text)]">{p.value}</div>
                    </div>
                  </a>
                ))}

                {data.vcard.emails?.map((e, i) => (
                  <a
                    key={i}
                    href={`mailto:${e.value}`}
                    className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-subtle)] hover:bg-[var(--border)]/40 transition-colors"
                  >
                    <div className="w-8 h-8 rounded-lg bg-[var(--surface)] flex items-center justify-center text-[var(--accent)]">
                      <Mail className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-[10px] text-[var(--text-muted)] uppercase">{e.type}</div>
                      <div className="font-semibold text-[var(--text)]">{e.value}</div>
                    </div>
                  </a>
                ))}

                {data.vcard.website && (
                  <a
                    href={data.vcard.website}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-subtle)] hover:bg-[var(--border)]/40 transition-colors"
                  >
                    <div className="w-8 h-8 rounded-lg bg-[var(--surface)] flex items-center justify-center text-[var(--accent)]">
                      <Globe className="w-4 h-4" />
                    </div>
                    <div className="flex-1 truncate">
                      <div className="text-[10px] text-[var(--text-muted)] uppercase">Website</div>
                      <div className="font-semibold text-[var(--text)] truncate">{data.vcard.website}</div>
                    </div>
                    <ExternalLink className="w-3.5 h-3.5 text-[var(--text-muted)]" />
                  </a>
                )}

                {data.vcard.address && (
                  <div className="flex items-center gap-3 p-2.5 rounded-xl bg-[var(--bg-subtle)]">
                    <div className="w-8 h-8 rounded-lg bg-[var(--surface)] flex items-center justify-center text-[var(--accent)]">
                      <MapPin className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-[10px] text-[var(--text-muted)] uppercase">Location</div>
                      <div className="font-semibold text-[var(--text)]">
                        {[data.vcard.address.city, data.vcard.address.country].filter(Boolean).join(', ')}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </>
          )}

          {data.kind === 'links_page' && data.links_page && (
            <div className="space-y-4">
              <h1 className="text-xl font-bold text-[var(--text)]">{data.links_page.title}</h1>
              {data.links_page.bio && <p className="text-xs text-[var(--text-muted)]">{data.links_page.bio}</p>}
              <div className="space-y-2">
                {data.links_page.links.map((link, idx) => (
                  <a
                    key={idx}
                    href={link.url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center justify-between p-3.5 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)] text-xs font-bold text-[var(--text)] hover:border-[var(--accent)] transition-colors"
                  >
                    <span>{link.label}</span>
                    <ExternalLink className="w-4 h-4 text-[var(--text-muted)]" />
                  </a>
                ))}
              </div>
            </div>
          )}

          {data.kind === 'file' && (
            <div className="space-y-4">
              <FileText className="w-12 h-12 text-[var(--accent)] mx-auto" />
              <h1 className="text-lg font-bold text-[var(--text)]">Download Attachment</h1>
              <a
                href="#"
                className="inline-flex items-center gap-2 px-6 py-3 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-bold text-xs shadow-sm hover:opacity-95"
              >
                <Download className="w-4 h-4" />
                Download Document
              </a>
            </div>
          )}

          {data.kind === 'event' && (
            <div className="space-y-4">
              <Calendar className="w-12 h-12 text-[var(--accent)] mx-auto" />
              <h1 className="text-lg font-bold text-[var(--text)]">Event Calendar Invitation</h1>
              <button className="inline-flex items-center gap-2 px-6 py-3 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-bold text-xs shadow-sm hover:opacity-95">
                <Download className="w-4 h-4" />
                Add to Calendar (.ics)
              </button>
            </div>
          )}
        </div>

        {/* Footer Brand */}
        <div className="py-3 bg-[var(--bg-subtle)] text-center text-[10px] text-[var(--text-muted)] border-t border-[var(--border)]">
          Powered by <strong className="text-[var(--text)]">QRit</strong>
        </div>
      </div>
    </div>
  );
}
