'use client';

import React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { CreditCard, Users, Webhook } from 'lucide-react';

export default function SettingsLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ workspace: string }>;
}) {
  const { workspace } = React.use(params);
  const pathname = usePathname();

  const tabs = [
    { label: 'Billing & Plans', href: `/w/${workspace}/settings/billing`, icon: CreditCard },
    { label: 'Team Members', href: `/w/${workspace}/settings/team`, icon: Users },
    { label: 'Webhooks & API', href: `/w/${workspace}/settings/webhooks`, icon: Webhook },
  ];

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-extrabold tracking-tight text-[var(--text)]">Workspace Settings</h1>
        <p className="text-xs text-[var(--text-muted)] mt-0.5">
          Manage your subscription plans, team permissions, and developer webhooks.
        </p>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-[var(--border)]">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = pathname.startsWith(tab.href);
          return (
            <Link
              key={tab.href}
              href={tab.href}
              className={`flex items-center gap-2 px-4 py-2.5 text-xs font-semibold border-b-2 -mb-px transition-colors ${
                isActive
                  ? 'border-[var(--accent)] text-[var(--accent)]'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text)] hover:border-[var(--border)]'
              }`}
            >
              <Icon className="w-4 h-4" />
              <span>{tab.label}</span>
            </Link>
          );
        })}
      </div>

      <div>{children}</div>
    </div>
  );
}
