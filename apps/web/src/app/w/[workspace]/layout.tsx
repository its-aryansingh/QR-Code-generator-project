'use client';

import React from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  QrCode,
  BarChart3,
  Globe,
  Settings,
  Plus,
  LogOut,
  Building,
} from 'lucide-react';
import { api } from '@/lib/api/client';

export default function WorkspaceLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);
  const pathname = usePathname();
  const router = useRouter();

  const handleLogout = async () => {
    try {
      await api.post('/v1/auth/logout');
    } catch {
      // ignore
    }
    router.push('/login');
  };

  const navItems = [
    { label: 'QR Codes', href: `/w/${workspace}/qr`, icon: QrCode },
    { label: 'Analytics', href: `/w/${workspace}/analytics`, icon: BarChart3 },
    { label: 'Domains', href: `/w/${workspace}/domains`, icon: Globe },
    { label: 'Settings', href: `/w/${workspace}/settings`, icon: Settings },
  ];

  return (
    <div className="min-h-screen flex bg-[var(--bg)]">
      {/* Sidebar */}
      <aside className="w-64 border-r border-[var(--border)] bg-[var(--surface)] flex flex-col shrink-0">
        <div className="p-4 border-b border-[var(--border)]">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-[var(--accent)] flex items-center justify-center text-white font-bold">
              <QrCode className="w-5 h-5" />
            </div>
            <div className="overflow-hidden">
              <div className="font-bold text-sm tracking-tight text-[var(--text)]">QRit</div>
              <div className="text-[11px] text-[var(--text-muted)] truncate flex items-center gap-1">
                <Building className="w-3 h-3" />
                {workspace}
              </div>
            </div>
          </div>
        </div>

        <div className="p-3">
          <Link
            href={`/w/${workspace}/qr/new`}
            className="w-full py-2 px-3 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg font-semibold text-xs flex items-center justify-center gap-1.5 shadow-xs hover:opacity-95 transition-opacity"
          >
            <Plus className="w-4 h-4" />
            New QR Code
          </Link>
        </div>

        <nav className="flex-1 px-3 py-2 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-2.5 px-3 py-2 rounded-lg text-xs font-medium transition-colors ${
                  isActive
                    ? 'bg-[var(--accent)]/10 text-[var(--accent)] font-semibold'
                    : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-[var(--bg-subtle)]'
                }`}
              >
                <Icon className="w-4 h-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="p-3 border-t border-[var(--border)]">
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-2 px-3 py-2 text-xs font-medium text-[var(--text-muted)] hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded-lg transition-colors"
          >
            <LogOut className="w-4 h-4" />
            Sign Out
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="h-14 border-b border-[var(--border)] bg-[var(--surface)] px-6 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <span>Workspace</span>
            <span>/</span>
            <span className="font-semibold text-[var(--text)] capitalize">{workspace}</span>
          </div>
          <div className="flex items-center gap-3">
            <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold bg-green-500/10 text-green-600">
              API Live
            </span>
          </div>
        </header>

        <main className="flex-1 p-6 overflow-y-auto">
          {children}
        </main>
      </div>
    </div>
  );
}
