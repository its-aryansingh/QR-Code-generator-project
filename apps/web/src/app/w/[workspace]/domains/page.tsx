'use client';

import React, { useState } from 'react';
import { Globe, Plus, CheckCircle2, Clock, Copy, Check } from 'lucide-react';

interface DomainItem {
  id: string;
  domain: string;
  status: 'active' | 'pending_verification' | 'failed';
  token: string;
  created_at: string;
}

export default function DomainsPage() {
  const [domains, setDomains] = useState<DomainItem[]>([
    {
      id: 'dom_1',
      domain: 'qr.mybrand.com',
      status: 'active',
      token: 'qrit_token_abc123',
      created_at: '2026-09-20',
    },
  ]);

  const [showAdd, setShowAdd] = useState(false);
  const [newDomain, setNewDomain] = useState('');
  const [copiedToken, setCopiedToken] = useState(false);

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain) return;
    const item: DomainItem = {
      id: `dom_${Date.now()}`,
      domain: newDomain.trim().toLowerCase(),
      status: 'pending_verification',
      token: `qrit_verify_${Math.random().toString(36).substring(2, 10)}`,
      created_at: new Date().toISOString().split('T')[0],
    };
    setDomains([...domains, item]);
    setNewDomain('');
    setShowAdd(false);
  };

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-extrabold tracking-tight text-[var(--text)]">Custom Domains</h1>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">
            Brand your short links with your own custom domain (e.g. go.brand.com)
          </p>
        </div>
        <button
          onClick={() => setShowAdd(true)}
          className="px-4 py-2 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold flex items-center gap-1.5 shadow-xs hover:opacity-95"
        >
          <Plus className="w-4 h-4" />
          Add Domain
        </button>
      </div>

      {showAdd && (
        <form onSubmit={handleAdd} className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] space-y-4">
          <h3 className="text-sm font-bold text-[var(--text)]">Add Custom Hostname</h3>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Hostname</label>
            <input
              type="text"
              required
              value={newDomain}
              onChange={(e) => setNewDomain(e.target.value)}
              placeholder="e.g. go.mycompany.com"
              className="w-full px-3.5 py-2.5 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-mono text-[var(--text)]"
            />
          </div>
          <div className="p-3 bg-[var(--bg-subtle)] rounded-xl text-xs space-y-2 text-[var(--text-muted)]">
            <p className="font-semibold text-[var(--text)]">Required DNS configuration:</p>
            <p>1. CNAME: <span className="font-mono text-[var(--text)]">domains.qr.example.com</span></p>
            <p>2. TXT: <span className="font-mono text-[var(--text)]">_qrit-challenge.{newDomain || '<hostname>'}</span></p>
          </div>
          <div className="flex items-center justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setShowAdd(false)}
              className="px-3 py-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text)]"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-1.5 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold"
            >
              Add Hostname
            </button>
          </div>
        </form>
      )}

      <div className="bg-[var(--surface)] rounded-2xl border border-[var(--border)] overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-[var(--bg-subtle)] text-[var(--text-muted)] font-semibold border-b border-[var(--border)] uppercase">
            <tr>
              <th className="py-3 px-4">Domain</th>
              <th className="py-3 px-4">Status</th>
              <th className="py-3 px-4">Verification Token</th>
              <th className="py-3 px-4">Created</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border)]">
            {domains.map((dom) => (
              <tr key={dom.id} className="hover:bg-[var(--bg-subtle)]/50">
                <td className="py-3 px-4 font-mono font-bold text-[var(--text)]">{dom.domain}</td>
                <td className="py-3 px-4">
                  {dom.status === 'active' ? (
                    <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-green-600 bg-green-500/10 px-2 py-0.5 rounded-full">
                      <CheckCircle2 className="w-3.5 h-3.5" />
                      Active
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-yellow-600 bg-yellow-500/10 px-2 py-0.5 rounded-full">
                      <Clock className="w-3.5 h-3.5" />
                      Pending DNS
                    </span>
                  )}
                </td>
                <td className="py-3 px-4 font-mono text-[11px] text-[var(--text-muted)]">
                  {dom.token}
                </td>
                <td className="py-3 px-4 text-[var(--text-muted)]">{dom.created_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
