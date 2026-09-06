"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";

import { enterprise } from "@/lib/enterprise";
import { useAuthStore } from "@/lib/auth";
import { useWorkspace, useWorkspaceStore } from "@/lib/workspace";
import { cn } from "@/lib/utils";
import { Avatar, Badge, RoleBadge } from "@/components/enterprise/ui";
import type { FeatureName, Role } from "@/types/enterprise";

interface NavItem {
  href: string;
  label: string;
  icon: React.ReactNode;
  exact?: boolean;
  feature?: FeatureName;
  role?: Role;
}

const icon = (path: string) => (
  <svg className="size-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.6} d={path} />
  </svg>
);

const NAV: Array<{ title: string; items: NavItem[] }> = [
  {
    title: "Workspace",
    items: [
      { href: "/dashboard", label: "Overview", exact: true, icon: icon("M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z") },
      { href: "/dashboard/qr-codes", label: "QR Codes", icon: icon("M4 4h6v6H4V4zm10 0h6v6h-6V4zM4 14h6v6H4v-6zm10 3h3m0 0h3m-3 0v3m0-6v0") },
      { href: "/dashboard/campaigns", label: "Campaigns", feature: "campaigns", icon: icon("M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z") },
      { href: "/dashboard/bulk", label: "Bulk Create", feature: "bulk", icon: icon("M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z") },
      { href: "/dashboard/templates", label: "Templates", feature: "templates", icon: icon("M4 5a1 1 0 011-1h14a1 1 0 011 1v3H4V5zm0 5h6v9H5a1 1 0 01-1-1v-8zm8 0h8v8a1 1 0 01-1 1h-7v-9z") },
    ],
  },
  {
    title: "Insights",
    items: [
      { href: "/dashboard/analytics", label: "Analytics", icon: icon("M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z") },
      { href: "/dashboard/leads", label: "Leads", icon: icon("M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z") },
      { href: "/dashboard/reports", label: "Reports", feature: "exports", icon: icon("M12 10v6m0 0l-3-3m3 3l3-3M3 17V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z") },
    ],
  },
  {
    title: "Administration",
    items: [
      { href: "/dashboard/team", label: "Team", icon: icon("M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z") },
      { href: "/dashboard/branding", label: "Brand & Domains", role: "admin", icon: icon("M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01") },
      { href: "/dashboard/security", label: "Security & SSO", role: "admin", icon: icon("M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z") },
      { href: "/dashboard/webhooks", label: "Webhooks", role: "admin", icon: icon("M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1") },
      { href: "/dashboard/api", label: "API Keys", role: "admin", icon: icon("M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4") },
      { href: "/dashboard/audit-log", label: "Audit Log", role: "admin", feature: "audit_log", icon: icon("M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z") },
      { href: "/dashboard/settings", label: "Settings", icon: icon("M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM15 12a3 3 0 11-6 0 3 3 0 016 0z") },
    ],
  },
];

const PLAN_TONE = {
  free: "neutral", starter: "info", pro: "violet", enterprise: "success",
} as const;

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  const { isAuthenticated, profile, setProfile, logout, hydrated } = useAuthStore();
  const { workspaces, workspace, plan, role, can, atLeast } = useWorkspace();
  const loadWorkspaces = useWorkspaceStore((s) => s.load);
  const setActive = useWorkspaceStore((s) => s.setActive);
  const resetWorkspaces = useWorkspaceStore((s) => s.reset);

  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [switcherOpen, setSwitcherOpen] = useState(false);
  const switcherRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (hydrated && !isAuthenticated) {
        router.replace(`/login?returnUrl=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [hydrated, isAuthenticated, router]);

  useEffect(() => {
    if (!isAuthenticated) return;
    void loadWorkspaces();
    if (!profile) {
      enterprise.me().then(setProfile).catch(() => undefined);
    }
  }, [isAuthenticated, loadWorkspaces, profile, setProfile]);

  useEffect(() => {
    setMobileOpen(false);
    setSwitcherOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!switcherOpen) return;
    const onClick = (event: MouseEvent) => {
      if (!switcherRef.current?.contains(event.target as Node)) setSwitcherOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [switcherOpen]);

  const handleLogout = () => {
    logout();
    resetWorkspaces();
    router.push("/");
  };

  const isActive = (item: NavItem) =>
    item.exact ? pathname === item.href : pathname.startsWith(item.href);

  /** A nav item the plan or role excludes stays visible but reads as locked,
   *  so people can see what the tier above them offers. */
  const lockState = (item: NavItem) => {
    if (item.role && !atLeast(item.role)) return "role" as const;
    if (item.feature && !can(item.feature)) return "plan" as const;
    return null;
  };

  const currentLabel = useMemo(() => {
    const all = NAV.flatMap((section) => section.items);
    const match = all
      .filter((item) => isActive(item))
      .sort((a, b) => b.href.length - a.href.length)[0];
    return match?.label ?? "Dashboard";
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  if (!hydrated || !isAuthenticated) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950">
        <div className="size-6 animate-spin rounded-full border-2 border-zinc-700 border-t-violet-500" />
      </div>
    );
  }

  const Sidebar = ({ mobile = false }: { mobile?: boolean }) => (
    <>
      <div className="border-b border-zinc-800/60 p-3">
        <div className="flex items-center gap-2.5 px-1">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-white">
            <svg viewBox="0 0 24 24" className="size-5">
              <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
              <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
              <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#18181b" />
              <rect x="14" y="14" width="4" height="4" rx="1" fill="#18181b" />
              <rect x="20" y="14" width="1" height="4" rx="0.5" fill="#18181b" />
              <rect x="14" y="20" width="4" height="1" rx="0.5" fill="#18181b" />
            </svg>
          </div>
          {(!collapsed || mobile) && (
            <span className="text-lg font-bold text-white">
              QR<span className="text-zinc-500">it</span>
            </span>
          )}
          {mobile && (
            <button
              onClick={() => setMobileOpen(false)}
              className="ml-auto rounded-lg p-2 text-zinc-400 hover:bg-zinc-800"
              aria-label="Close navigation"
            >
              <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>

        {(!collapsed || mobile) && (
          <div className="relative mt-3" ref={mobile ? undefined : switcherRef}>
            <button
              onClick={() => setSwitcherOpen((open) => !open)}
              className="flex w-full items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-900 px-2.5 py-2 text-left transition-colors hover:border-zinc-700"
            >
              <span
                className="flex size-6 shrink-0 items-center justify-center rounded text-xs font-bold text-white"
                style={{ background: workspace?.brand_color ?? "#8B5CF6" }}
              >
                {workspace?.name?.charAt(0).toUpperCase() ?? "W"}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm text-zinc-100">
                  {workspace?.name ?? "No workspace"}
                </span>
                <span className="block truncate text-[11px] capitalize text-zinc-500">
                  {plan} plan{role ? ` · ${role}` : ""}
                </span>
              </span>
              <svg className="size-4 shrink-0 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
              </svg>
            </button>

            {switcherOpen && (
              <div className="absolute inset-x-0 top-full z-50 mt-1 max-h-72 overflow-y-auto rounded-lg border border-zinc-800 bg-zinc-900 py-1 shadow-2xl">
                {workspaces.length === 0 && (
                  <p className="px-3 py-2 text-xs text-zinc-500">No workspaces yet</p>
                )}
                {workspaces.map((item) => (
                  <button
                    key={item.id}
                    onClick={() => {
                      setActive(item.id);
                      setSwitcherOpen(false);
                    }}
                    className={cn(
                      "flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-zinc-800",
                      workspace?.id === item.id && "bg-zinc-800/70",
                    )}
                  >
                    <span
                      className="flex size-6 shrink-0 items-center justify-center rounded text-xs font-bold text-white"
                      style={{ background: item.brand_color ?? "#8B5CF6" }}
                    >
                      {item.name.charAt(0).toUpperCase()}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm text-zinc-200">{item.name}</span>
                      <span className="block text-[11px] text-zinc-500">
                        {(item.qr_count ?? 0).toLocaleString()} codes ·{" "}
                        {(item.member_count ?? 0).toLocaleString()} members
                      </span>
                    </span>
                    {item.role && <RoleBadge role={item.role} />}
                  </button>
                ))}
                <div className="mt-1 border-t border-zinc-800 pt-1">
                  <Link
                    href="/dashboard/settings"
                    className="flex items-center gap-2 px-3 py-2 text-sm text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-200"
                  >
                    <svg className="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v12m6-6H6" />
                    </svg>
                    New workspace
                  </Link>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      <nav className="flex-1 space-y-4 overflow-y-auto px-2.5 py-3">
        {NAV.map((section) => (
          <div key={section.title}>
            {(!collapsed || mobile) && (
              <p className="mb-1 px-2.5 text-[10px] font-semibold uppercase tracking-widest text-zinc-600">
                {section.title}
              </p>
            )}
            <div className="space-y-0.5">
              {section.items.map((item) => {
                const locked = lockState(item);
                const active = isActive(item);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    title={collapsed && !mobile ? item.label : undefined}
                    className={cn(
                      "group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors",
                      active
                        ? "bg-zinc-800 font-medium text-white"
                        : "text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200",
                      locked && !active && "opacity-60",
                    )}
                  >
                    <span className={active ? "text-white" : "text-zinc-500 group-hover:text-zinc-300"}>
                      {item.icon}
                    </span>
                    {(!collapsed || mobile) && (
                      <>
                        <span className="truncate">{item.label}</span>
                        {locked && (
                          <svg
                            className="ml-auto size-3.5 shrink-0 text-zinc-600"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                          </svg>
                        )}
                      </>
                    )}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="border-t border-zinc-800/60 p-2.5">
        <Link
          href="/dashboard/profile"
          className="flex items-center gap-2.5 rounded-lg px-2 py-2 transition-colors hover:bg-zinc-900"
        >
          <Avatar name={profile?.name} email={profile?.email} url={profile?.avatar_url} size={30} />
          {(!collapsed || mobile) && (
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm text-zinc-200">
                {profile?.name || profile?.email || "My account"}
              </span>
              <span className="block truncate text-[11px] text-zinc-600">{profile?.email}</span>
            </span>
          )}
        </Link>
        <button
          onClick={handleLogout}
          className="mt-0.5 flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm text-zinc-500 transition-colors hover:bg-red-500/10 hover:text-red-400"
        >
          <svg className="size-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.6} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          {(!collapsed || mobile) && "Sign out"}
        </button>
        {!mobile && (
          <button
            onClick={() => setCollapsed((value) => !value)}
            className="mt-1 flex w-full items-center justify-center rounded-lg py-1 text-zinc-600 transition-colors hover:bg-zinc-900 hover:text-zinc-400"
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            <svg
              className={cn("size-4 transition-transform", collapsed && "rotate-180")}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            </svg>
          </button>
        )}
      </div>
    </>
  );

  return (
    <div className="flex min-h-screen bg-zinc-950">
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 flex w-72 flex-col overflow-y-auto border-r border-zinc-800/60 bg-zinc-950 shadow-2xl">
            <Sidebar mobile />
          </aside>
        </div>
      )}

      <aside
        className={cn(
          "sticky top-0 hidden h-screen flex-col border-r border-zinc-800/60 transition-[width] duration-200 lg:flex",
          collapsed ? "w-16" : "w-64",
        )}
      >
        <Sidebar />
      </aside>

      <main className="min-w-0 flex-1">
        <header className="sticky top-0 z-40 border-b border-zinc-800/60 bg-zinc-950/85 backdrop-blur-xl">
          <div className="flex h-14 items-center justify-between gap-3 px-3 sm:px-6">
            <div className="flex min-w-0 items-center gap-2 text-sm">
              <button
                onClick={() => setMobileOpen(true)}
                className="rounded-lg p-2 text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-200 lg:hidden"
                aria-label="Open navigation"
              >
                <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>
              <span className="hidden truncate text-zinc-500 sm:inline">
                {workspace?.name ?? "Personal"}
              </span>
              <svg className="hidden size-3 shrink-0 text-zinc-700 sm:block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
              <span className="truncate text-zinc-200">{currentLabel}</span>
            </div>

            <div className="flex shrink-0 items-center gap-2">
              <Badge tone={PLAN_TONE[plan as keyof typeof PLAN_TONE] ?? "neutral"} className="hidden capitalize sm:inline-flex">
                {plan}
              </Badge>
              {atLeast("editor") && (
                <Link
                  href="/dashboard/create"
                  className="rounded-lg bg-white px-3.5 py-1.5 text-sm font-medium text-zinc-950 transition-colors hover:bg-zinc-200"
                >
                  + Create
                </Link>
              )}
            </div>
          </div>
        </header>

        <div className="p-4 sm:p-6">{children}</div>
      </main>
    </div>
  );
}
