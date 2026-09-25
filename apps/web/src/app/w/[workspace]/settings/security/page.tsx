'use client';

import React, { useState } from 'react';
import {
  Shield,
  KeyRound,
  Smartphone,
  CheckCircle2,
  AlertTriangle,
  Copy,
  Check,
  Download,
  Laptop,
  LogOut,
  RefreshCw,
  Lock,
} from 'lucide-react';
import { renderSvg, DEFAULT_DESIGN } from '@qrit/qr-render';

interface SessionItem {
  id: string;
  device: string;
  browser: string;
  ip: string;
  location: string;
  last_active: string;
  is_current: boolean;
}

export default function SecuritySettingsPage({
  params,
}: {
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);

  // 2FA state
  const [is2FAEnabled, setIs2FAEnabled] = useState(false);
  const [showSetup2FA, setShowSetup2FA] = useState(false);
  const [totpSecret, setTotpSecret] = useState('JBSWY3DPEHPK3PXP4MZT');
  const [confirmCode, setConfirmCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [copiedCodes, setCopiedCodes] = useState(false);
  const [step2FA, setStep2FA] = useState<'scan' | 'verify' | 'recovery'>('scan');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Sessions state
  const [sessions, setSessions] = useState<SessionItem[]>([
    {
      id: 'sess_curr',
      device: 'Windows PC',
      browser: 'Chrome 128.0',
      ip: '103.21.244.18',
      location: 'New Delhi, India',
      last_active: 'Active now',
      is_current: true,
    },
    {
      id: 'sess_phone',
      device: 'Apple iPhone 15 Pro',
      browser: 'Mobile Safari 17.4',
      ip: '103.21.244.52',
      location: 'New Delhi, India',
      last_active: '4 hours ago',
      is_current: false,
    },
  ]);

  // SVG QR Code for Authenticator setup
  const totpUri = `otpauth://totp/QRit:${workspace}@user?secret=${totpSecret}&issuer=QRit&digits=6&period=30`;
  const qrSvg = React.useMemo(() => {
    try {
      const res = renderSvg({
        payload: totpUri,
        design: DEFAULT_DESIGN,
      });
      return res.svg;
    } catch {
      return '';
    }
  }, [totpUri]);

  const handleStart2FA = () => {
    setErrorMessage(null);
    setStep2FA('scan');
    setShowSetup2FA(true);
  };

  const handleVerify2FACode = (e: React.FormEvent) => {
    e.preventDefault();
    if (confirmCode.length !== 6) {
      setErrorMessage('Please enter a 6-digit verification code.');
      return;
    }

    // Generate 10 sample recovery backup codes
    const codes = [
      'A7B2-C9D4', 'E5F6-G1H8', 'J3K4-L7M9', 'N2P3-Q4R5',
      'S6T7-U8V9', 'W1X2-Y3Z4', '2B3C-4D5F', '6G7H-8J9K',
      '2L3M-4N5P', '6Q7R-8S9T',
    ];
    setRecoveryCodes(codes);
    setStep2FA('recovery');
  };

  const handleComplete2FA = () => {
    setIs2FAEnabled(true);
    setShowSetup2FA(false);
    setConfirmCode('');
  };

  const handleDisable2FA = () => {
    if (confirm('Are you sure you want to disable Two-Factor Authentication? Your account will be less secure.')) {
      setIs2FAEnabled(false);
      setRecoveryCodes([]);
    }
  };

  const handleCopyRecoveryCodes = () => {
    navigator.clipboard.writeText(recoveryCodes.join('\n'));
    setCopiedCodes(true);
    setTimeout(() => setCopiedCodes(false), 2000);
  };

  const handleRevokeSession = (sessionId: string) => {
    setSessions(sessions.filter((s) => s.id !== sessionId));
  };

  const handleRevokeOtherSessions = () => {
    if (confirm('Sign out of all other active sessions across other devices?')) {
      setSessions(sessions.filter((s) => s.is_current));
    }
  };

  return (
    <div className="space-y-8">
      {/* 2FA Security Card */}
      <div className="p-6 rounded-2xl bg-[var(--surface)] border border-[var(--border)] shadow-xs space-y-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <KeyRound className="w-4 h-4 text-[var(--accent)]" />
              <h3 className="text-sm font-bold text-[var(--text)]">Two-Factor Authentication (2FA)</h3>
              {is2FAEnabled ? (
                <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-600">
                  Enabled
                </span>
              ) : (
                <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-600">
                  Disabled
                </span>
              )}
            </div>
            <p className="text-xs text-[var(--text-muted)]">
              Protect your account from unauthorized access by requiring a time-based code (TOTP) on login.
            </p>
          </div>

          <div>
            {is2FAEnabled ? (
              <button
                onClick={handleDisable2FA}
                className="px-4 py-2 border border-[var(--danger)]/30 text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded-xl text-xs font-semibold transition-colors"
              >
                Disable 2FA
              </button>
            ) : (
              <button
                onClick={handleStart2FA}
                className="px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-semibold hover:opacity-95 shadow-xs transition-opacity"
              >
                Enable 2FA
              </button>
            )}
          </div>
        </div>

        {/* 2FA Setup Flow Modal */}
        {showSetup2FA && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs">
            <div className="w-full max-w-md bg-[var(--surface)] border border-[var(--border)] rounded-2xl shadow-xl overflow-hidden p-6 space-y-5 animate-in fade-in zoom-in-95 duration-150">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-lg bg-[var(--accent)]/10 text-[var(--accent)] flex items-center justify-center">
                    <Smartphone className="w-4 h-4" />
                  </div>
                  <h3 className="text-sm font-bold text-[var(--text)]">Set up Authenticator App</h3>
                </div>
                <button
                  onClick={() => setShowSetup2FA(false)}
                  className="text-[var(--text-muted)] hover:text-[var(--text)] text-sm"
                >
                  ✕
                </button>
              </div>

              {step2FA === 'scan' && (
                <div className="space-y-4">
                  <p className="text-xs text-[var(--text-muted)]">
                    1. Scan this QR code using your preferred authenticator app (Google Authenticator, 1Password, Authy).
                  </p>

                  <div className="p-4 bg-white rounded-2xl border border-[var(--border)] flex items-center justify-center mx-auto max-w-[220px]">
                    <div
                      className="w-44 h-44"
                      dangerouslySetInnerHTML={{ __html: qrSvg }}
                    />
                  </div>

                  <div className="p-2.5 rounded-xl bg-[var(--bg-subtle)] text-center space-y-1">
                    <div className="text-[10px] uppercase font-bold text-[var(--text-muted)]">Manual Entry Key</div>
                    <code className="text-xs font-mono font-bold text-[var(--text)] tracking-wider">
                      {totpSecret}
                    </code>
                  </div>

                  <div className="flex justify-end pt-2">
                    <button
                      onClick={() => setStep2FA('verify')}
                      className="w-full py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold hover:opacity-95"
                    >
                      Next: Verify Code
                    </button>
                  </div>
                </div>
              )}

              {step2FA === 'verify' && (
                <form onSubmit={handleVerify2FACode} className="space-y-4">
                  <p className="text-xs text-[var(--text-muted)]">
                    2. Enter the 6-digit code shown in your authenticator app to confirm setup.
                  </p>

                  {errorMessage && (
                    <div className="p-2.5 rounded-xl bg-[var(--danger)]/10 text-[var(--danger)] text-xs flex items-center gap-2">
                      <AlertTriangle className="w-4 h-4 shrink-0" />
                      <span>{errorMessage}</span>
                    </div>
                  )}

                  <div>
                    <input
                      type="text"
                      maxLength={6}
                      autoFocus
                      required
                      value={confirmCode}
                      onChange={(e) => setConfirmCode(e.target.value.replace(/\D/g, ''))}
                      placeholder="123456"
                      className="w-full text-center tracking-widest text-2xl font-mono py-3 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-xl text-[var(--text)] focus:outline-hidden focus:border-[var(--accent)]"
                    />
                  </div>

                  <div className="flex items-center justify-between pt-2">
                    <button
                      type="button"
                      onClick={() => setStep2FA('scan')}
                      className="text-xs text-[var(--text-muted)] hover:text-[var(--text)]"
                    >
                      Back
                    </button>
                    <button
                      type="submit"
                      className="px-5 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold hover:opacity-95"
                    >
                      Verify and Activate
                    </button>
                  </div>
                </form>
              )}

              {step2FA === 'recovery' && (
                <div className="space-y-4">
                  <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-xs text-amber-600 flex items-start gap-2">
                    <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
                    <div>
                      <strong>Save your backup recovery codes!</strong>
                      <p className="text-[11px] mt-0.5">
                        Each code can be used once if you lose access to your mobile authenticator.
                      </p>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2 p-3 bg-[var(--bg-subtle)] rounded-xl font-mono text-xs text-[var(--text)] text-center border border-[var(--border)]">
                    {recoveryCodes.map((code, idx) => (
                      <div key={idx} className="p-1 rounded bg-[var(--surface)]">
                        {code}
                      </div>
                    ))}
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      onClick={handleCopyRecoveryCodes}
                      className="flex-1 py-2 px-3 border border-[var(--border)] rounded-xl text-xs font-semibold flex items-center justify-center gap-1.5 hover:bg-[var(--bg-subtle)]"
                    >
                      {copiedCodes ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>{copiedCodes ? 'Copied Codes' : 'Copy All'}</span>
                    </button>
                    <button
                      onClick={handleComplete2FA}
                      className="flex-1 py-2 px-3 bg-[var(--accent)] text-[var(--accent-fg)] rounded-xl text-xs font-bold hover:opacity-95"
                    >
                      Done
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Active Sessions List */}
      <div className="bg-[var(--surface)] rounded-2xl border border-[var(--border)] overflow-hidden shadow-xs">
        <div className="p-4 border-b border-[var(--border)] flex items-center justify-between">
          <div className="space-y-0.5">
            <h3 className="text-xs font-bold uppercase tracking-wider text-[var(--text)]">Active Sessions</h3>
            <p className="text-[11px] text-[var(--text-muted)]">
              Devices and browsers currently logged into your account.
            </p>
          </div>

          <button
            onClick={handleRevokeOtherSessions}
            className="px-3 py-1.5 text-xs font-semibold text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded-lg transition-colors flex items-center gap-1.5"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span>Sign out other devices</span>
          </button>
        </div>

        <div className="divide-y divide-[var(--border)]">
          {sessions.map((sess) => (
            <div key={sess.id} className="p-4 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3.5">
                <div className="w-9 h-9 rounded-xl bg-[var(--bg-subtle)] border border-[var(--border)] flex items-center justify-center text-[var(--text-muted)]">
                  {sess.device.includes('iPhone') ? <Smartphone className="w-4 h-4" /> : <Laptop className="w-4 h-4" />}
                </div>
                <div className="space-y-0.5">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-xs text-[var(--text)]">{sess.device}</span>
                    {sess.is_current && (
                      <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-emerald-500/15 text-emerald-500">
                        This device
                      </span>
                    )}
                  </div>
                  <div className="text-[11px] text-[var(--text-muted)] flex items-center gap-2">
                    <span>{sess.browser}</span>
                    <span>•</span>
                    <span>{sess.location}</span>
                    <span>•</span>
                    <span className="font-mono">{sess.ip}</span>
                  </div>
                </div>
              </div>

              {!sess.is_current && (
                <button
                  onClick={() => handleRevokeSession(sess.id)}
                  className="px-3 py-1 text-xs text-[var(--text-muted)] hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded-lg transition-colors"
                >
                  Revoke
                </button>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
