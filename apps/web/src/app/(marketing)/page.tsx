'use client';

import React, { useState, useMemo } from 'react';
import Link from 'next/link';
import { renderSvg, DesignV1, DEFAULT_DESIGN, calculateScannability, ModuleShape } from '@qrit/qr-render';
import { encodeUrl, encodeText, encodeEmail, encodePhone, encodeSms, encodeWhatsApp, encodeWiFi } from '@/features/qr-editor/encoders';
import { QrCode, Download, Sparkles, CheckCircle2, AlertTriangle, ArrowRight, Shield, Layers, Zap } from 'lucide-react';

type ContentType = 'url' | 'text' | 'wifi' | 'email' | 'phone' | 'sms' | 'whatsapp';

export default function MarketingPage() {
  const [contentType, setContentType] = useState<ContentType>('url');
  const [urlVal, setUrlVal] = useState('https://qrit.io');
  const [textVal, setTextVal] = useState('Hello from QRit!');
  const [wifiSsid, setWifiSsid] = useState('MyNetwork');
  const [wifiPass, setWifiPass] = useState('SuperSecretPass');
  const [emailTo, setEmailTo] = useState('hello@example.com');
  const [phoneVal, setPhoneVal] = useState('+1234567890');
  const [smsPhone, setSmsPhone] = useState('+1234567890');
  const [smsMsg, setSmsMsg] = useState('Hi there!');
  const [waPhone, setWaPhone] = useState('1234567890');
  const [waMsg, setWaMsg] = useState('Hello!');

  const [design, setDesign] = useState<DesignV1>(DEFAULT_DESIGN);
  const [qrMode, setQrMode] = useState<'static' | 'dynamic'>('static');

  // Compute payload based on active content type
  const payload = useMemo(() => {
    try {
      switch (contentType) {
        case 'url': return encodeUrl(urlVal);
        case 'text': return encodeText(textVal);
        case 'wifi': return encodeWiFi(wifiSsid, wifiPass);
        case 'email': return encodeEmail(emailTo);
        case 'phone': return encodePhone(phoneVal);
        case 'sms': return encodeSms(smsPhone, smsMsg);
        case 'whatsapp': return encodeWhatsApp(waPhone, waMsg);
        default: return urlVal;
      }
    } catch {
      return urlVal;
    }
  }, [contentType, urlVal, textVal, wifiSsid, wifiPass, emailTo, phoneVal, smsPhone, smsMsg, waPhone, waMsg]);

  // Synchronous SVG rendering
  const renderResult = useMemo(() => {
    try {
      return renderSvg({ payload: payload || 'https://qrit.io', design });
    } catch {
      return renderSvg({ payload: 'https://qrit.io', design: DEFAULT_DESIGN });
    }
  }, [payload, design]);

  const scannability = useMemo(() => {
    return calculateScannability(design, renderResult.version);
  }, [design, renderResult.version]);

  const handleDownloadSvg = () => {
    const blob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `qrit-${Date.now()}.svg`;
    link.click();
    URL.revokeObjectURL(url);
  };

  const handleDownloadPng = (size: number) => {
    const img = new Image();
    const svgBlob = new Blob([renderResult.svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(svgBlob);

    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.fillStyle = design.background.transparent ? 'transparent' : design.background.color;
        ctx.fillRect(0, 0, size, size);
        ctx.drawImage(img, 0, 0, size, size);
        const pngUrl = canvas.toDataURL('image/png');
        const link = document.createElement('a');
        link.href = pngUrl;
        link.download = `qrit-${size}x${size}-${Date.now()}.png`;
        link.click();
      }
      URL.revokeObjectURL(url);
    };
    img.src = url;
  };

  return (
    <div className="min-h-screen flex flex-col bg-[var(--bg)]">
      {/* Top Navbar */}
      <header className="border-b border-[var(--border)] bg-[var(--surface)]/80 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-[var(--accent)] flex items-center justify-center text-white font-bold shadow-sm">
              <QrCode className="w-5 h-5" />
            </div>
            <span className="font-extrabold text-xl tracking-tight text-[var(--text)]">QRit</span>
          </div>

          <div className="flex items-center gap-4">
            <Link
              href="/pricing"
              className="text-sm font-medium text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
            >
              Pricing
            </Link>
            <Link
              href="/login"
              className="text-sm font-medium text-[var(--text-muted)] hover:text-[var(--text)] transition-colors"
            >
              Sign In
            </Link>
            <Link
              href="/register"
              className="text-sm font-semibold bg-[var(--accent)] text-[var(--accent-fg)] px-4 py-2 rounded-lg hover:opacity-95 transition-opacity"
            >
              Get Started Free
            </Link>
          </div>
        </div>
      </header>

      {/* Hero Header */}
      <section className="pt-12 pb-8 px-4 text-center max-w-4xl mx-auto">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold bg-[var(--accent)]/10 text-[var(--accent)] mb-4">
          <Sparkles className="w-3.5 h-3.5" />
          <span>Next-Generation Dynamic QR Platform</span>
        </div>
        <h1 className="text-4xl md:text-6xl font-extrabold tracking-tight text-[var(--text)] mb-4">
          Create trackable, branded QR codes in seconds
        </h1>
        <p className="text-lg md:text-xl text-[var(--text-muted)] max-w-2xl mx-auto">
          High-reliability dynamic redirects, vector rendering, real-time analytics, and guaranteed scannability.
        </p>
      </section>

      {/* Main Generator Section */}
      <main className="max-w-7xl mx-auto px-4 py-6 w-full flex-1">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
          {/* Controls Column */}
          <div className="lg:col-span-7 space-y-6">
            {/* Mode & Type Selection */}
            <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs">
              <div className="flex items-center justify-between mb-6 pb-4 border-b border-[var(--border)]">
                <span className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Code Type</span>
                <div className="inline-flex p-1 bg-[var(--bg-subtle)] rounded-lg text-xs font-medium border border-[var(--border)]">
                  <button
                    onClick={() => setQrMode('static')}
                    className={`px-3 py-1 rounded-md transition-all ${qrMode === 'static' ? 'bg-[var(--surface)] text-[var(--text)] shadow-xs font-semibold' : 'text-[var(--text-muted)]'}`}
                  >
                    Static (Free)
                  </button>
                  <button
                    onClick={() => setQrMode('dynamic')}
                    className={`px-3 py-1 rounded-md transition-all flex items-center gap-1.5 ${qrMode === 'dynamic' ? 'bg-[var(--surface)] text-[var(--text)] shadow-xs font-semibold' : 'text-[var(--text-muted)]'}`}
                  >
                    Dynamic
                    <span className="bg-[var(--accent)] text-white text-[10px] px-1.5 py-0.2 rounded font-bold">PRO</span>
                  </button>
                </div>
              </div>

              {/* Type Tabs */}
              <div className="flex flex-wrap gap-2 mb-6">
                {(['url', 'text', 'wifi', 'email', 'phone', 'sms', 'whatsapp'] as ContentType[]).map((type) => (
                  <button
                    key={type}
                    onClick={() => setContentType(type)}
                    className={`px-3.5 py-2 text-xs font-semibold rounded-lg uppercase tracking-wider transition-colors ${
                      contentType === type
                        ? 'bg-[var(--accent)] text-[var(--accent-fg)]'
                        : 'bg-[var(--bg-subtle)] text-[var(--text-muted)] hover:text-[var(--text)]'
                    }`}
                  >
                    {type}
                  </button>
                ))}
              </div>

              {/* Form Input fields based on type */}
              <div className="space-y-4">
                {contentType === 'url' && (
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Destination URL</label>
                    <input
                      type="url"
                      value={urlVal}
                      onChange={(e) => setUrlVal(e.target.value)}
                      placeholder="https://example.com"
                      className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                )}

                {contentType === 'text' && (
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Plain Text (max 1000 chars)</label>
                    <textarea
                      value={textVal}
                      onChange={(e) => setTextVal(e.target.value)}
                      rows={3}
                      className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                )}

                {contentType === 'wifi' && (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Network Name (SSID)</label>
                      <input
                        type="text"
                        value={wifiSsid}
                        onChange={(e) => setWifiSsid(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Password</label>
                      <input
                        type="text"
                        value={wifiPass}
                        onChange={(e) => setWifiPass(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                  </div>
                )}

                {contentType === 'email' && (
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Recipient Email</label>
                    <input
                      type="email"
                      value={emailTo}
                      onChange={(e) => setEmailTo(e.target.value)}
                      className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                )}

                {contentType === 'phone' && (
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Phone Number (with country code)</label>
                    <input
                      type="tel"
                      value={phoneVal}
                      onChange={(e) => setPhoneVal(e.target.value)}
                      className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>
                )}

                {contentType === 'sms' && (
                  <div className="space-y-4">
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Phone Number</label>
                      <input
                        type="tel"
                        value={smsPhone}
                        onChange={(e) => setSmsPhone(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Message</label>
                      <input
                        type="text"
                        value={smsMsg}
                        onChange={(e) => setSmsMsg(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                  </div>
                )}

                {contentType === 'whatsapp' && (
                  <div className="space-y-4">
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">WhatsApp Number (digits only)</label>
                      <input
                        type="text"
                        value={waPhone}
                        onChange={(e) => setWaPhone(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Prefilled Message</label>
                      <input
                        type="text"
                        value={waMsg}
                        onChange={(e) => setWaMsg(e.target.value)}
                        className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-sm text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Customization Options */}
            <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs space-y-6">
              <span className="text-xs font-bold uppercase tracking-wider text-[var(--text-muted)]">Design & Styling</span>

              {/* Module Shape */}
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">Module Pattern</label>
                <div className="grid grid-cols-3 gap-2">
                  {(['square', 'dots', 'rounded', 'extra-rounded', 'classy', 'classy-rounded'] as ModuleShape[]).map((shape) => (
                    <button
                      key={shape}
                      onClick={() => setDesign({ ...design, modules: { ...design.modules, shape } })}
                      className={`px-3 py-2 text-xs font-medium rounded-lg border capitalize transition-all ${
                        design.modules.shape === shape
                          ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)] font-semibold'
                          : 'border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text)]'
                      }`}
                    >
                      {shape.replace('-', ' ')}
                    </button>
                  ))}
                </div>
              </div>

              {/* Finder Eye Shapes */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">Corner Ring</label>
                  <select
                    value={design.finder.outer_shape}
                    onChange={(e) =>
                      setDesign({
                        ...design,
                        finder: { ...design.finder, outer_shape: e.target.value as any },
                      })
                    }
                    className="w-full px-3 py-2 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)] focus:outline-hidden"
                  >
                    <option value="square">Square</option>
                    <option value="rounded">Rounded</option>
                    <option value="circle">Circle</option>
                    <option value="leaf">Leaf</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">Corner Eye</label>
                  <select
                    value={design.finder.inner_shape}
                    onChange={(e) =>
                      setDesign({
                        ...design,
                        finder: { ...design.finder, inner_shape: e.target.value as any },
                      })
                    }
                    className="w-full px-3 py-2 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs text-[var(--text)] focus:outline-hidden"
                  >
                    <option value="square">Square</option>
                    <option value="rounded">Rounded</option>
                    <option value="circle">Circle</option>
                    <option value="dot">Dot</option>
                  </select>
                </div>
              </div>

              {/* Color pickers */}
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Pattern Color</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="color"
                      value={design.modules.color}
                      onChange={(e) =>
                        setDesign({
                          ...design,
                          modules: { ...design.modules, color: e.target.value },
                          finder: {
                            ...design.finder,
                            outer_color: e.target.value,
                            inner_color: e.target.value,
                          },
                        })
                      }
                      className="w-8 h-8 rounded border border-[var(--border)] cursor-pointer"
                    />
                    <span className="text-xs font-mono text-[var(--text-muted)] uppercase">{design.modules.color}</span>
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Background</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="color"
                      value={design.background.color}
                      onChange={(e) =>
                        setDesign({
                          ...design,
                          background: { ...design.background, color: e.target.value },
                        })
                      }
                      className="w-8 h-8 rounded border border-[var(--border)] cursor-pointer"
                    />
                    <span className="text-xs font-mono text-[var(--text-muted)] uppercase">{design.background.color}</span>
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">Quiet Margin</label>
                  <select
                    value={design.quiet_zone}
                    onChange={(e) => setDesign({ ...design, quiet_zone: parseInt(e.target.value, 10) })}
                    className="w-full px-2 py-1.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded text-xs text-[var(--text)]"
                  >
                    <option value="1">1 (Compact)</option>
                    <option value="2">2 (Narrow)</option>
                    <option value="4">4 (Standard)</option>
                    <option value="6">6 (Spacious)</option>
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* Sticky Preview & Scannability Column */}
          <div className="lg:col-span-5 sticky top-24 space-y-6">
            <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs flex flex-col items-center">
              {/* QR Code SVG Preview */}
              <div
                className="w-72 h-72 md:w-80 md:h-80 flex items-center justify-center p-4 rounded-xl bg-white shadow-xs border border-[var(--border)]"
                dangerouslySetInnerHTML={{ __html: renderResult.svg }}
              />

              {/* Scannability Meter */}
              <div className="w-full mt-6 p-4 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)]">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-semibold text-[var(--text-muted)]">Scannability Score</span>
                  <div className="flex items-center gap-1.5">
                    {scannability.score >= 70 ? (
                      <CheckCircle2 className="w-4 h-4 text-[var(--success)]" />
                    ) : (
                      <AlertTriangle className="w-4 h-4 text-[var(--warning)]" />
                    )}
                    <span
                      className={`text-xs font-bold ${
                        scannability.score >= 85
                          ? 'text-[var(--success)]'
                          : scannability.score >= 70
                          ? 'text-blue-500'
                          : scannability.score >= 50
                          ? 'text-[var(--warning)]'
                          : 'text-[var(--danger)]'
                      }`}
                    >
                      {scannability.label} ({scannability.score}/100)
                    </span>
                  </div>
                </div>

                {/* Progress bar */}
                <div className="w-full h-2 rounded-full bg-[var(--border)] overflow-hidden">
                  <div
                    className={`h-full transition-all duration-300 ${
                      scannability.score >= 70
                        ? 'bg-[var(--success)]'
                        : scannability.score >= 50
                        ? 'bg-[var(--warning)]'
                        : 'bg-[var(--danger)]'
                    }`}
                    style={{ width: `${Math.max(5, scannability.score)}%` }}
                  />
                </div>

                {/* Warnings list if any */}
                {scannability.warnings.length > 0 && (
                  <div className="mt-3 space-y-1">
                    {scannability.warnings.map((w, idx) => (
                      <p key={idx} className="text-[11px] text-[var(--danger)] flex items-start gap-1">
                        <span>•</span> {w.message}
                      </p>
                    ))}
                  </div>
                )}
              </div>

              {/* Export Buttons */}
              <div className="w-full mt-6 space-y-2.5">
                <button
                  onClick={handleDownloadSvg}
                  className="w-full py-2.5 px-4 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl font-semibold text-sm flex items-center justify-center gap-2 hover:opacity-95 transition-opacity"
                >
                  <Download className="w-4 h-4" />
                  Download Vector (SVG)
                </button>

                <div className="grid grid-cols-2 gap-2">
                  <button
                    onClick={() => handleDownloadPng(1024)}
                    className="py-2 px-3 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-semibold text-[var(--text)] hover:bg-[var(--border)]/40 transition-colors flex items-center justify-center gap-1.5"
                  >
                    <Download className="w-3.5 h-3.5" />
                    PNG 1024px
                  </button>
                  <button
                    onClick={() => handleDownloadPng(2048)}
                    className="py-2 px-3 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-semibold text-[var(--text)] hover:bg-[var(--border)]/40 transition-colors flex items-center justify-center gap-1.5"
                  >
                    <Download className="w-3.5 h-3.5" />
                    PNG 2048px (HD)
                  </button>
                </div>
              </div>

              {qrMode === 'dynamic' && (
                <div className="w-full mt-4 p-3 rounded-lg bg-[var(--accent)]/10 border border-[var(--accent)]/20 text-center">
                  <p className="text-xs font-medium text-[var(--accent)]">
                    Create an account to activate scan analytics and dynamic destination editing.
                  </p>
                  <Link
                    href="/register"
                    className="mt-2 inline-flex items-center gap-1 text-xs font-bold text-[var(--accent)] hover:underline"
                  >
                    Sign up now <ArrowRight className="w-3 h-3" />
                  </Link>
                </div>
              )}
            </div>
          </div>
        </div>
      </main>

      {/* Feature Value Props Section */}
      <section className="border-t border-[var(--border)] bg-[var(--bg-subtle)] py-16 px-4">
        <div className="max-w-6xl mx-auto grid grid-cols-1 md:grid-cols-3 gap-8">
          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)]">
            <div className="w-10 h-10 rounded-xl bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center mb-4">
              <Zap className="w-5 h-5" />
            </div>
            <h3 className="text-base font-bold text-[var(--text)] mb-2">High-Reliability Redirects</h3>
            <p className="text-sm text-[var(--text-muted)]">
              Sub-50ms p99 response times with geo-distributed edge caching and zero-downtime destination changes.
            </p>
          </div>

          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)]">
            <div className="w-10 h-10 rounded-xl bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center mb-4">
              <Shield className="w-5 h-5" />
            </div>
            <h3 className="text-base font-bold text-[var(--text)] mb-2">Automated URL Safety</h3>
            <p className="text-sm text-[var(--text-muted)]">
              Real-time phishing, malware, and abuse detection protects your audience and brand integrity.
            </p>
          </div>

          <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)]">
            <div className="w-10 h-10 rounded-xl bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center mb-4">
              <Layers className="w-5 h-5" />
            </div>
            <h3 className="text-base font-bold text-[var(--text)] mb-2">Enterprise Analytics</h3>
            <p className="text-sm text-[var(--text-muted)]">
              Privacy-preserving daily visitor counts, bot-filtered scans, device, OS, and country breakdowns.
            </p>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-[var(--border)] py-8 px-4 text-center text-xs text-[var(--text-muted)]">
        <p>© {new Date().getFullYear()} QRit. Modern QR Code Infrastructure.</p>
      </footer>
    </div>
  );
}
