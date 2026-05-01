"use client";

import { cn } from "@/lib/utils";

export interface QRTypeCardData {
    id: string;
    name: string;
    description: string;
    icon: string;
    category: string;
    isPremium: boolean;
}

const qrTypes: QRTypeCardData[] = [
    { id: "url", name: "Website", description: "Link to any website URL", icon: "globe", category: "links", isPremium: false },
    { id: "text", name: "Text", description: "Plain text message", icon: "file-text", category: "basic", isPremium: false },
    { id: "wifi", name: "WiFi", description: "Connect to a Wi-Fi network", icon: "wifi", category: "technical", isPremium: false },
    { id: "vcard", name: "vCard", description: "Share a digital business card", icon: "user-plus", category: "business", isPremium: false },
    { id: "email", name: "Email", description: "Send an email", icon: "mail", category: "basic", isPremium: false },
    { id: "sms", name: "SMS", description: "Send a text message", icon: "message-square", category: "basic", isPremium: false },
    { id: "phone", name: "Phone", description: "Make a phone call", icon: "phone", category: "basic", isPremium: false },
    { id: "pdf", name: "PDF", description: "Show a PDF document", icon: "file", category: "media", isPremium: true },
    { id: "images", name: "Images", description: "Share multiple images", icon: "image", category: "media", isPremium: true },
    { id: "video", name: "Video", description: "Show a video", icon: "video", category: "media", isPremium: true },
    { id: "mp3", name: "MP3", description: "Share an audio file", icon: "music", category: "media", isPremium: true },
    { id: "facebook", name: "Facebook", description: "Share your Facebook page", icon: "facebook", category: "social", isPremium: false },
    { id: "instagram", name: "Instagram", description: "Share your Instagram", icon: "instagram", category: "social", isPremium: false },
    { id: "whatsapp", name: "WhatsApp", description: "Get WhatsApp messages", icon: "message-circle", category: "social", isPremium: false },
    { id: "social", name: "Social Media", description: "Share your social channels", icon: "share-2", category: "social", isPremium: false },
    { id: "apps", name: "Apps", description: "Redirect to an app store", icon: "smartphone", category: "technical", isPremium: false },
    { id: "menu", name: "Menu", description: "Create a restaurant menu", icon: "menu", category: "business", isPremium: true },
    { id: "coupon", name: "Coupon", description: "Share a coupon", icon: "tag", category: "business", isPremium: true },
    { id: "business", name: "Business", description: "Share information about your business", icon: "briefcase", category: "business", isPremium: true },
    { id: "mecard", name: "meCard", description: "Simple contact card format", icon: "user-plus", category: "business", isPremium: false },
    { id: "links", name: "List of Links", description: "Share multiple links", icon: "link", category: "links", isPremium: false },
];

// Icon mapping using inline SVGs
const IconComponent = ({ icon, className }: { icon: string; className?: string }) => {
    const icons: Record<string, React.ReactNode> = {
        "globe": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="10" />
                <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
            </svg>
        ),
        "file-text": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
                <line x1="16" y1="13" x2="8" y2="13" />
                <line x1="16" y1="17" x2="8" y2="17" />
                <polyline points="10 9 9 9 8 9" />
            </svg>
        ),
        "wifi": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M5 12.55a11 11 0 0 1 14.08 0" />
                <path d="M1.42 9a16 16 0 0 1 21.16 0" />
                <path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
                <line x1="12" y1="20" x2="12.01" y2="20" />
            </svg>
        ),
        "user-plus": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                <circle cx="8.5" cy="7" r="4" />
                <line x1="20" y1="8" x2="20" y2="14" />
                <line x1="23" y1="11" x2="17" y2="11" />
            </svg>
        ),
        "mail": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                <polyline points="22,6 12,13 2,6" />
            </svg>
        ),
        "message-square": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
        ),
        "phone": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z" />
            </svg>
        ),
        "file": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
            </svg>
        ),
        "image": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                <circle cx="8.5" cy="8.5" r="1.5" />
                <polyline points="21 15 16 10 5 21" />
            </svg>
        ),
        "video": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18" />
                <line x1="7" y1="2" x2="7" y2="22" />
                <line x1="17" y1="2" x2="17" y2="22" />
                <line x1="2" y1="12" x2="22" y2="12" />
                <line x1="2" y1="7" x2="7" y2="7" />
                <line x1="2" y1="17" x2="7" y2="17" />
                <line x1="17" y1="17" x2="22" y2="17" />
                <line x1="17" y1="7" x2="22" y2="7" />
            </svg>
        ),
        "music": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M9 18V5l12-2v13" />
                <circle cx="6" cy="18" r="3" />
                <circle cx="18" cy="16" r="3" />
            </svg>
        ),
        "facebook": (
            <svg className={className} fill="currentColor" viewBox="0 0 24 24">
                <path d="M18 2h-3a5 5 0 0 0-5 5v3H7v4h3v8h4v-8h3l1-4h-4V7a1 1 0 0 1 1-1h3z" />
            </svg>
        ),
        "instagram": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="2" y="2" width="20" height="20" rx="5" ry="5" />
                <path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z" />
                <line x1="17.5" y1="6.5" x2="17.51" y2="6.5" />
            </svg>
        ),
        "message-circle": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
            </svg>
        ),
        "share-2": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <circle cx="18" cy="5" r="3" />
                <circle cx="6" cy="12" r="3" />
                <circle cx="18" cy="19" r="3" />
                <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
                <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
            </svg>
        ),
        "smartphone": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="5" y="2" width="14" height="20" rx="2" ry="2" />
                <line x1="12" y1="18" x2="12.01" y2="18" />
            </svg>
        ),
        "menu": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <line x1="3" y1="12" x2="21" y2="12" />
                <line x1="3" y1="6" x2="21" y2="6" />
                <line x1="3" y1="18" x2="21" y2="18" />
            </svg>
        ),
        "tag": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z" />
                <line x1="7" y1="7" x2="7.01" y2="7" />
            </svg>
        ),
        "briefcase": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="2" y="7" width="20" height="14" rx="2" ry="2" />
                <path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16" />
            </svg>
        ),
        "link": (
            <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
            </svg>
        ),
    };

    return icons[icon] || icons["link"];
};

interface QRTypeGridProps {
    selectedType: string | null;
    onSelectType: (type: QRTypeCardData) => void;
    showPremiumTypes?: boolean;
}

export function QRTypeGrid({ selectedType, onSelectType, showPremiumTypes = true }: QRTypeGridProps) {
    const filteredTypes = showPremiumTypes
        ? qrTypes
        : qrTypes.filter(t => !t.isPremium);

    return (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
            {filteredTypes.map((type) => (
                <button
                    key={type.id}
                    onClick={() => onSelectType(type)}
                    className={cn(
                        "relative flex flex-col items-center justify-center p-4 rounded-xl border transition-all duration-200",
                        "hover:scale-[1.02] active:scale-[0.98]",
                        selectedType === type.id
                            ? "bg-gradient-to-br from-cyan-500/20 to-purple-500/20 border-purple-500/50 shadow-lg shadow-purple-500/10"
                            : "bg-slate-800/50 border-white/10 hover:border-white/20 hover:bg-slate-800",
                        type.isPremium && "ring-1 ring-amber-500/30"
                    )}
                >
                    {type.isPremium && (
                        <span className="absolute -top-2 -right-2 px-1.5 py-0.5 text-[10px] font-bold rounded-full bg-gradient-to-r from-amber-500 to-orange-500 text-white">
                            PRO
                        </span>
                    )}
                    <div className={cn(
                        "w-10 h-10 rounded-lg flex items-center justify-center mb-2",
                        selectedType === type.id
                            ? "bg-gradient-to-br from-cyan-500 to-purple-500"
                            : "bg-slate-700/50"
                    )}>
                        <IconComponent
                            icon={type.icon}
                            className={cn(
                                "w-5 h-5 stroke-2",
                                selectedType === type.id ? "text-white" : "text-slate-300"
                            )}
                        />
                    </div>
                    <span className={cn(
                        "text-sm font-medium text-center",
                        selectedType === type.id ? "text-white" : "text-slate-300"
                    )}>
                        {type.name}
                    </span>
                    <span className="text-[10px] text-slate-500 text-center mt-0.5 line-clamp-1">
                        {type.description}
                    </span>
                </button>
            ))}
        </div>
    );
}

export { qrTypes };
