"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { useAuthStore } from "@/lib/auth";

export default function DashboardLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const router = useRouter();
    const pathname = usePathname();
    const { isAuthenticated, logout, accessToken } = useAuthStore();
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
    const [workspaces, setWorkspaces] = useState<any[]>([]);
    const [activeWorkspace, setActiveWorkspace] = useState<any>(null);
    const [showWsSwitcher, setShowWsSwitcher] = useState(false);
    const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);

    useEffect(() => {
        if (!isAuthenticated) {
            router.push("/login");
        }
    }, [isAuthenticated, router]);

    // Fetch workspaces
    useEffect(() => {
        if (isAuthenticated) {
            fetch(`${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081/api/v1'}/workspaces`, {
                headers: { Authorization: `Bearer ${accessToken}` },
            })
                .then(res => res.json())
                .then(data => {
                    if (data.success && data.data) {
                        setWorkspaces(data.data);
                        if (data.data.length > 0 && !activeWorkspace) {
                            setActiveWorkspace(data.data[0]);
                        }
                    }
                })
                .catch(() => { });
        }
    }, [isAuthenticated]);

    const handleLogout = () => {
        logout();
        router.push("/");
    };

    if (!isAuthenticated) {
        return null;
    }

    const navSections = [
        {
            title: "MAIN",
            items: [
                {
                    href: "/", label: "✦ Create QR",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4v16m8-8H4" /></svg>
                },
                {
                    href: "/dashboard", label: "Dashboard", exact: true,
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" /></svg>
                },
                {
                    href: "/dashboard/qr-codes", label: "QR Codes",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z" /></svg>
                },
                {
                    href: "/dashboard/campaigns", label: "Campaigns",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
                },
                {
                    href: "/dashboard/bulk", label: "Bulk Upload",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 13h6m-3-3v6m5 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                },
            ]
        },
        {
            title: "ANALYTICS",
            items: [
                {
                    href: "/dashboard/analytics", label: "Analytics",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" /></svg>
                },
            ]
        },
        {
            title: "ENTERPRISE",
            items: [
                {
                    href: "/dashboard/team", label: "Team",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
                },
                {
                    href: "/dashboard/webhooks", label: "Webhooks",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
                },
                {
                    href: "/dashboard/integrations", label: "Integrations",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
                },
                {
                    href: "/dashboard/audit-log", label: "Audit Log",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                },
                {
                    href: "/dashboard/api", label: "API",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>
                },
                {
                    href: "/dashboard/branding", label: "Branding",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" /></svg>
                },
                {
                    href: "/dashboard/leads", label: "Leads",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" /></svg>
                },
                {
                    href: "/dashboard/reports", label: "Reports",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 10v6m0 0l-3-3m3 3l3-3M3 17V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" /></svg>
                },
            ]
        },
        {
            title: "SETTINGS",
            items: [
                {
                    href: "/dashboard/profile", label: "Profile",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" /></svg>
                },
                {
                    href: "/dashboard/settings", label: "Settings",
                    icon: <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                },
            ]
        },
    ];

    const isActive = (href: string, exact?: boolean) => {
        if (exact) return pathname === href;
        return pathname.startsWith(href);
    };

    return (
        <div className="min-h-screen flex bg-zinc-950">
            {/* Mobile Sidebar Overlay */}
            {mobileSidebarOpen && (
                <div className="fixed inset-0 z-50 lg:hidden">
                    <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setMobileSidebarOpen(false)} />
                    <aside className="absolute left-0 top-0 bottom-0 w-72 bg-zinc-950 border-r border-zinc-800/60 flex flex-col shadow-2xl overflow-y-auto">
                        <div className="p-4 border-b border-zinc-800/60 flex items-center justify-between">
                            <div className="flex items-center gap-3">
                                <div className="w-8 h-8 rounded-lg bg-white flex items-center justify-center flex-shrink-0">
                                    <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                                        <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                                        <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                                        <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#18181b" />
                                        <rect x="14" y="14" width="4" height="4" rx="1" fill="#18181b" />
                                        <rect x="20" y="14" width="1" height="4" rx="0.5" fill="#18181b" />
                                        <rect x="14" y="20" width="4" height="1" rx="0.5" fill="#18181b" />
                                        <rect x="20" y="20" width="1" height="1" rx="0.5" fill="#18181b" />
                                    </svg>
                                </div>
                                <span className="text-lg font-bold text-white">QR<span className="text-zinc-500">it</span></span>
                            </div>
                            <button onClick={() => setMobileSidebarOpen(false)} className="w-9 h-9 flex items-center justify-center rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                            </button>
                        </div>
                        <nav className="flex-1 py-2 px-3 space-y-3">
                            {navSections.map((section) => (
                                <div key={section.title}>
                                    <p className="text-[10px] font-semibold tracking-widest text-zinc-600 uppercase mb-1 px-3">{section.title}</p>
                                    <div className="space-y-0.5">
                                        {section.items.map((item) => (
                                            <Link
                                                key={item.href}
                                                href={item.href}
                                                onClick={() => setMobileSidebarOpen(false)}
                                                className={`flex items-center gap-3 px-3 py-2.5 rounded-lg transition-all duration-150 group ${item.href === '/'
                                                    ? 'bg-violet-600/20 text-violet-300 hover:bg-violet-600/30 border border-violet-500/20'
                                                    : isActive(item.href, item.exact)
                                                        ? 'bg-zinc-800 text-white'
                                                        : 'text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900'
                                                    }`}
                                            >
                                                <span className={item.href === '/' ? 'text-violet-400' : isActive(item.href, item.exact) ? 'text-white' : 'text-zinc-500 group-hover:text-zinc-300'}>
                                                    {item.icon}
                                                </span>
                                                <span className="text-sm font-medium">{item.label}</span>
                                            </Link>
                                        ))}
                                    </div>
                                </div>
                            ))}
                        </nav>
                        <div className="p-3 border-t border-zinc-800/60">
                            <button onClick={() => { handleLogout(); setMobileSidebarOpen(false); }} className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-red-400 hover:bg-red-500/10 transition-colors text-sm font-medium">
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" /></svg>
                                Sign Out
                            </button>
                        </div>
                    </aside>
                </div>
            )}

            {/* Desktop Sidebar */}
            <aside className={`hidden lg:flex ${sidebarCollapsed ? 'w-16' : 'w-64'} border-r border-zinc-800/60 flex-col transition-all duration-200 sticky top-0 h-screen`}>
                {/* Logo & Workspace Switcher */}
                <div className="p-4 border-b border-zinc-800/60">
                    <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-white flex items-center justify-center flex-shrink-0">
                            <svg viewBox="0 0 24 24" className="w-5 h-5" fill="none">
                                <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                                <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#18181b" />
                                <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#18181b" />
                                <rect x="14" y="14" width="4" height="4" rx="1" fill="#18181b" />
                                <rect x="20" y="14" width="1" height="4" rx="0.5" fill="#18181b" />
                                <rect x="14" y="20" width="4" height="1" rx="0.5" fill="#18181b" />
                                <rect x="20" y="20" width="1" height="1" rx="0.5" fill="#18181b" />
                            </svg>
                        </div>
                        {!sidebarCollapsed && (
                            <div>
                                <span className="text-lg font-bold text-white">QR</span>
                                <span className="text-lg font-bold text-zinc-500">it</span>
                            </div>
                        )}
                    </div>

                    {/* Workspace Switcher */}
                    {!sidebarCollapsed && (
                        <div className="mt-3 relative">
                            <button
                                onClick={() => setShowWsSwitcher(!showWsSwitcher)}
                                className="w-full flex items-center gap-2 px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-800 hover:border-zinc-700 transition-colors text-left"
                            >
                                <div className="w-6 h-6 rounded bg-violet-600/20 flex items-center justify-center text-violet-400 text-xs font-bold">
                                    {activeWorkspace?.name?.charAt(0)?.toUpperCase() || 'W'}
                                </div>
                                <div className="flex-1 min-w-0">
                                    <p className="text-sm text-zinc-200 truncate">{activeWorkspace?.name || 'Select Workspace'}</p>
                                </div>
                                <svg className="w-4 h-4 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
                                </svg>
                            </button>

                            {showWsSwitcher && (
                                <div className="absolute top-full left-0 right-0 mt-1 bg-zinc-900 border border-zinc-800 rounded-lg shadow-xl z-50 py-1">
                                    {workspaces.map(ws => (
                                        <button
                                            key={ws.id}
                                            onClick={() => { setActiveWorkspace(ws); setShowWsSwitcher(false); }}
                                            className={`w-full flex items-center gap-2 px-3 py-2 hover:bg-zinc-800 transition-colors text-left ${activeWorkspace?.id === ws.id ? 'bg-zinc-800' : ''}`}
                                        >
                                            <div className="w-6 h-6 rounded bg-violet-600/20 flex items-center justify-center text-violet-400 text-xs font-bold">
                                                {ws.name?.charAt(0)?.toUpperCase()}
                                            </div>
                                            <span className="text-sm text-zinc-200 truncate">{ws.name}</span>
                                            {activeWorkspace?.id === ws.id && (
                                                <svg className="w-4 h-4 text-violet-400 ml-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                                </svg>
                                            )}
                                        </button>
                                    ))}
                                    <div className="border-t border-zinc-800 mt-1 pt-1">
                                        <Link href="/dashboard/settings" onClick={() => setShowWsSwitcher(false)}
                                            className="w-full flex items-center gap-2 px-3 py-2 hover:bg-zinc-800 transition-colors text-left text-zinc-400 text-sm">
                                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                                            </svg>
                                            New Workspace
                                        </Link>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>

                {/* Navigation */}
                <nav className="flex-1 overflow-y-auto py-2 px-3 space-y-3">
                    {navSections.map((section) => (
                        <div key={section.title}>
                            {!sidebarCollapsed && (
                                <p className="text-[10px] font-semibold tracking-widest text-zinc-600 uppercase mb-1 px-3">{section.title}</p>
                            )}
                            <div className="space-y-0.5">
                                {section.items.map((item) => (
                                    <Link
                                        key={item.href}
                                        href={item.href}
                                        className={`flex items-center gap-3 px-3 py-1.5 rounded-lg transition-all duration-150 group ${item.href === '/'
                                            ? 'bg-violet-600/20 text-violet-300 hover:bg-violet-600/30 hover:text-violet-200 border border-violet-500/20'
                                            : isActive(item.href, item.exact)
                                                ? 'bg-zinc-800 text-white'
                                                : 'text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900'
                                            }`}
                                    >
                                        <span className={
                                            item.href === '/'
                                                ? 'text-violet-400'
                                                : isActive(item.href, item.exact) ? 'text-white' : 'text-zinc-500 group-hover:text-zinc-300'
                                        }>
                                            {item.icon}
                                        </span>
                                        {!sidebarCollapsed && (
                                            <span className="text-sm font-medium">{item.label}</span>
                                        )}
                                    </Link>
                                ))}
                            </div>
                        </div>
                    ))}
                </nav>

                {/* Sidebar Footer */}
                <div className="p-3 border-t border-zinc-800/60">
                    <div className="flex items-center gap-3 px-3 py-2">
                        <div className="w-8 h-8 rounded-full bg-zinc-800 flex items-center justify-center text-zinc-400 text-sm font-medium">
                            U
                        </div>
                        {!sidebarCollapsed && (
                            <div className="flex-1 min-w-0">
                                <p className="text-sm text-zinc-200 truncate">My Account</p>
                                <p className="text-xs text-zinc-600 truncate">Enterprise</p>
                            </div>
                        )}
                        {!sidebarCollapsed && (
                            <button
                                onClick={handleLogout}
                                className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
                                title="Logout"
                            >
                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                                </svg>
                            </button>
                        )}
                    </div>
                    {/* Collapse Toggle */}
                    <button
                        onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                        className="w-full mt-1 flex items-center justify-center py-1 rounded-lg text-zinc-600 hover:text-zinc-400 hover:bg-zinc-900 transition-colors"
                    >
                        <svg className={`w-4 h-4 transition-transform ${sidebarCollapsed ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
                        </svg>
                    </button>
                </div>
            </aside>

            {/* Main Content */}
            <main className="flex-1 overflow-auto">
                {/* Top Bar */}
                <div className="sticky top-0 z-40 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-xl">
                    <div className="flex items-center justify-between px-3 sm:px-6 h-14">
                        <div className="flex items-center gap-2 sm:gap-2 text-sm">
                            {/* Mobile hamburger */}
                            <button
                                onClick={() => setMobileSidebarOpen(true)}
                                className="lg:hidden w-9 h-9 flex items-center justify-center rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors mr-1"
                                aria-label="Open navigation"
                            >
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                                </svg>
                            </button>
                            <span className="text-zinc-500">
                                {activeWorkspace?.name || 'Personal'}
                            </span>
                            <svg className="w-3 h-3 text-zinc-700" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                            </svg>
                            <span className="text-zinc-200">
                                {navSections.flatMap(s => s.items).find(i => isActive(i.href, i.exact))?.label || 'Dashboard'}
                            </span>
                        </div>
                        <div className="flex items-center gap-3">
                            <Link
                                href="/dashboard/create"
                                className="px-4 py-1.5 bg-white text-zinc-950 text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors"
                            >
                                + Create QR
                            </Link>
                        </div>
                    </div>
                </div>

                <div className="p-6">
                    {children}
                </div>
            </main>
        </div>
    );
}
