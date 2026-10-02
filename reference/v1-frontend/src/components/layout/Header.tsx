"use client";

import Link from "next/link";
import { useAuthStore } from "@/lib/auth";
import { useRouter } from "next/navigation";
import { useState, useRef, useEffect } from "react";

interface HeaderProps {
    user?: any;
    onOpenSettings?: () => void;
}

export const Header: React.FC<HeaderProps> = ({ user, onOpenSettings }) => {
    const { isAuthenticated, logout } = useAuthStore();
    const router = useRouter();
    const [isMenuOpen, setIsMenuOpen] = useState(false);
    const [isMobileNavOpen, setIsMobileNavOpen] = useState(false);
    const menuRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                setIsMenuOpen(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    // Lock body scroll when mobile nav is open
    useEffect(() => {
        if (isMobileNavOpen) {
            document.body.style.overflow = "hidden";
        } else {
            document.body.style.overflow = "";
        }
        return () => { document.body.style.overflow = ""; };
    }, [isMobileNavOpen]);

    const handleLogout = () => {
        logout();
        setIsMenuOpen(false);
        setIsMobileNavOpen(false);
        router.push("/");
    };

    const closeMobileNav = () => setIsMobileNavOpen(false);

    return (
        <>
            <header className="relative z-50 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-xl sticky top-0">
                <div className="container mx-auto px-4 sm:px-6 py-3 sm:py-4">
                    <div className="flex items-center justify-between">
                        {/* Logo */}
                        <Link href="/" className="flex items-center gap-2.5 group" onClick={closeMobileNav}>
                            <div className="w-8 h-8 rounded-lg bg-white flex items-center justify-center transition-transform duration-200 group-hover:scale-105">
                                <svg className="w-5 h-5 text-zinc-950" fill="currentColor" viewBox="0 0 24 24">
                                    <rect x="2" y="2" width="6" height="6" rx="0.5" />
                                    <rect x="4" y="4" width="2" height="2" fill="white" />
                                    <rect x="16" y="2" width="6" height="6" rx="0.5" />
                                    <rect x="18" y="4" width="2" height="2" fill="white" />
                                    <rect x="2" y="16" width="6" height="6" rx="0.5" />
                                    <rect x="4" y="18" width="2" height="2" fill="white" />
                                    <rect x="10" y="2" width="2" height="2" />
                                    <rect x="10" y="6" width="2" height="2" />
                                    <rect x="10" y="10" width="4" height="2" />
                                    <rect x="2" y="10" width="2" height="4" />
                                    <rect x="16" y="10" width="2" height="2" />
                                    <rect x="20" y="10" width="2" height="2" />
                                    <rect x="16" y="16" width="2" height="2" />
                                    <rect x="20" y="16" width="2" height="6" />
                                    <rect x="16" y="20" width="2" height="2" />
                                </svg>
                            </div>
                            <span className="text-lg font-bold tracking-tight text-zinc-50">QR<span className="text-zinc-400">it</span></span>
                        </Link>

                        {/* Desktop Nav */}
                        <nav className="hidden md:flex items-center gap-8">
                            <Link href="/" className="text-zinc-50 font-medium text-sm transition-colors">Generator</Link>
                            <Link href="/pricing" className="text-zinc-500 font-medium text-sm hover:text-zinc-300 transition-colors">Pricing</Link>
                            {isAuthenticated && (
                                <Link href="/dashboard" className="text-zinc-500 font-medium text-sm hover:text-zinc-300 transition-colors">Dashboard</Link>
                            )}
                        </nav>

                        {/* Desktop Auth */}
                        <div className="hidden md:flex items-center gap-4">
                            {isAuthenticated ? (
                                <div className="relative" ref={menuRef}>
                                    <button
                                        onClick={() => setIsMenuOpen(!isMenuOpen)}
                                        className="flex items-center gap-3 focus:outline-none group"
                                    >
                                        {user ? (
                                            <>
                                                <div className="text-right hidden sm:block">
                                                    <p className="text-sm font-medium text-zinc-200 group-hover:text-white transition-colors">{user.name || "User"}</p>
                                                    <p className="text-xs text-zinc-500 capitalize">{user.plan} Plan</p>
                                                </div>
                                                <div className="w-9 h-9 rounded-full bg-violet-600 flex items-center justify-center overflow-hidden ring-2 ring-zinc-800">
                                                    {user.avatar_url ? (
                                                        <img src={user.avatar_url} alt={user.name} className="w-full h-full object-cover" />
                                                    ) : (
                                                        <span className="text-white font-semibold text-sm">
                                                            {(user.name || user.email || "U")[0].toUpperCase()}
                                                        </span>
                                                    )}
                                                </div>
                                            </>
                                        ) : (
                                            <div className="w-9 h-9 rounded-full bg-violet-600 flex items-center justify-center ring-2 ring-zinc-800">
                                                <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                                </svg>
                                            </div>
                                        )}
                                    </button>

                                    {isMenuOpen && (
                                        <div className="absolute right-0 mt-3 w-48 bg-zinc-900 border border-zinc-800 rounded-xl shadow-2xl shadow-black/50 overflow-hidden py-1">
                                            <Link
                                                href="/dashboard"
                                                onClick={() => setIsMenuOpen(false)}
                                                className="w-full px-4 py-2.5 text-left text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200 flex items-center gap-2.5 transition-colors"
                                            >
                                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" /></svg>
                                                Dashboard
                                            </Link>
                                            {onOpenSettings && (
                                                <button
                                                    onClick={() => { onOpenSettings(); setIsMenuOpen(false); }}
                                                    className="w-full px-4 py-2.5 text-left text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200 flex items-center gap-2.5 transition-colors"
                                                >
                                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                                                    Settings
                                                </button>
                                            )}
                                            <div className="h-px bg-zinc-800 my-1"></div>
                                            <button
                                                onClick={handleLogout}
                                                className="w-full px-4 py-2.5 text-left text-sm text-red-400 hover:bg-red-500/10 hover:text-red-300 flex items-center gap-2.5 transition-colors"
                                            >
                                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" /></svg>
                                                Sign Out
                                            </button>
                                        </div>
                                    )}
                                </div>
                            ) : (
                                <>
                                    <Link href="/login" className="text-zinc-400 hover:text-zinc-200 text-sm font-medium transition-colors">
                                        Sign in
                                    </Link>
                                    <Link
                                        href="/register"
                                        className="px-5 py-2 bg-white text-zinc-900 rounded-lg text-sm font-semibold hover:bg-zinc-200 transition-all duration-200"
                                    >
                                        Get Started
                                    </Link>
                                </>
                            )}
                        </div>

                        {/* Mobile Right: Avatar + Hamburger */}
                        <div className="flex md:hidden items-center gap-3">
                            {isAuthenticated && user && (
                                <div className="w-8 h-8 rounded-full bg-violet-600 flex items-center justify-center overflow-hidden ring-2 ring-zinc-800">
                                    {user.avatar_url ? (
                                        <img src={user.avatar_url} alt={user.name} className="w-full h-full object-cover" />
                                    ) : (
                                        <span className="text-white font-semibold text-xs">
                                            {(user.name || user.email || "U")[0].toUpperCase()}
                                        </span>
                                    )}
                                </div>
                            )}
                            <button
                                onClick={() => setIsMobileNavOpen(true)}
                                className="w-10 h-10 flex items-center justify-center rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
                                aria-label="Open navigation menu"
                            >
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                                </svg>
                            </button>
                        </div>
                    </div>
                </div>
            </header>

            {/* Mobile Nav Drawer */}
            {isMobileNavOpen && (
                <div className="fixed inset-0 z-[100] md:hidden">
                    {/* Backdrop */}
                    <div
                        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
                        onClick={closeMobileNav}
                    />
                    {/* Drawer */}
                    <div className="absolute right-0 top-0 bottom-0 w-72 bg-zinc-950 border-l border-zinc-800 flex flex-col shadow-2xl">
                        {/* Drawer Header */}
                        <div className="flex items-center justify-between p-4 border-b border-zinc-800">
                            <span className="text-lg font-bold text-zinc-50">QR<span className="text-zinc-400">it</span></span>
                            <button
                                onClick={closeMobileNav}
                                className="w-9 h-9 flex items-center justify-center rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
                                aria-label="Close menu"
                            >
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>

                        {/* Drawer Nav Links */}
                        <nav className="flex-1 p-4 space-y-1">
                            <Link
                                href="/"
                                onClick={closeMobileNav}
                                className="flex items-center gap-3 px-4 py-3 rounded-xl text-zinc-200 hover:bg-zinc-800 transition-colors font-medium"
                            >
                                <svg className="w-5 h-5 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z" />
                                </svg>
                                QR Generator
                            </Link>
                            <Link
                                href="/pricing"
                                onClick={closeMobileNav}
                                className="flex items-center gap-3 px-4 py-3 rounded-xl text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200 transition-colors font-medium"
                            >
                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z" />
                                </svg>
                                Pricing
                            </Link>
                            {isAuthenticated && (
                                <Link
                                    href="/dashboard"
                                    onClick={closeMobileNav}
                                    className="flex items-center gap-3 px-4 py-3 rounded-xl text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200 transition-colors font-medium"
                                >
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
                                    </svg>
                                    Dashboard
                                </Link>
                            )}
                            {isAuthenticated && onOpenSettings && (
                                <button
                                    onClick={() => { onOpenSettings(); closeMobileNav(); }}
                                    className="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200 transition-colors font-medium"
                                >
                                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                                    </svg>
                                    Settings
                                </button>
                            )}
                        </nav>

                        {/* Drawer Footer */}
                        <div className="p-4 border-t border-zinc-800 space-y-2">
                            {isAuthenticated ? (
                                <>
                                    {user && (
                                        <div className="flex items-center gap-3 px-4 py-3 rounded-xl bg-zinc-900">
                                            <div className="w-8 h-8 rounded-full bg-violet-600 flex items-center justify-center text-white font-semibold text-sm flex-shrink-0">
                                                {(user.name || user.email || "U")[0].toUpperCase()}
                                            </div>
                                            <div className="min-w-0">
                                                <p className="text-sm font-medium text-zinc-200 truncate">{user.name || "User"}</p>
                                                <p className="text-xs text-zinc-500 capitalize">{user.plan} Plan</p>
                                            </div>
                                        </div>
                                    )}
                                    <button
                                        onClick={handleLogout}
                                        className="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-red-400 hover:bg-red-500/10 transition-colors font-medium"
                                    >
                                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                                        </svg>
                                        Sign Out
                                    </button>
                                </>
                            ) : (
                                <>
                                    <Link
                                        href="/login"
                                        onClick={closeMobileNav}
                                        className="flex items-center justify-center w-full py-3 rounded-xl border border-zinc-700 text-zinc-300 hover:bg-zinc-800 transition-colors font-medium text-sm"
                                    >
                                        Sign In
                                    </Link>
                                    <Link
                                        href="/register"
                                        onClick={closeMobileNav}
                                        className="flex items-center justify-center w-full py-3 rounded-xl bg-white text-zinc-900 hover:bg-zinc-200 transition-colors font-semibold text-sm"
                                    >
                                        Get Started Free
                                    </Link>
                                </>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </>
    );
};
