"use client";

import { QrCode, BarChart3, Palette, Shield, Zap } from "lucide-react";

const features = [
  {
    icon: QrCode,
    title: "Dynamic QR Codes",
    description: "Edit destinations anytime without reprinting",
  },
  {
    icon: BarChart3,
    title: "Real-time Analytics",
    description: "Track scans, locations, devices & more",
  },
  {
    icon: Palette,
    title: "Custom Branding",
    description: "Colors, logos, frames — make it yours",
  },
  {
    icon: Zap,
    title: "Instant Generation",
    description: "URL, WiFi, vCard, UPI & 10+ types",
  },
];

export function AuthBrandPanel() {
  return (
    <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden flex-col justify-between p-12 bg-gradient-to-br from-violet-950/80 via-zinc-950 to-zinc-950">
      {/* Background decorative elements */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {/* Gradient orbs */}
        <div className="absolute -top-24 -left-24 w-96 h-96 bg-violet-600/10 rounded-full blur-3xl" />
        <div className="absolute bottom-0 right-0 w-80 h-80 bg-violet-500/5 rounded-full blur-3xl" />

        {/* Dot grid pattern */}
        <div className="absolute inset-0 dot-grid opacity-40" />

        {/* Decorative QR code illustration */}
        <div className="absolute bottom-12 right-12 opacity-[0.06]">
          <svg width="280" height="280" viewBox="0 0 280 280" fill="currentColor" className="text-white">
            {/* Top-left finder pattern */}
            <rect x="10" y="10" width="70" height="70" rx="4" />
            <rect x="20" y="20" width="50" height="50" rx="2" fill="#09090b" />
            <rect x="30" y="30" width="30" height="30" rx="1" />
            {/* Top-right finder pattern */}
            <rect x="200" y="10" width="70" height="70" rx="4" />
            <rect x="210" y="20" width="50" height="50" rx="2" fill="#09090b" />
            <rect x="220" y="30" width="30" height="30" rx="1" />
            {/* Bottom-left finder pattern */}
            <rect x="10" y="200" width="70" height="70" rx="4" />
            <rect x="20" y="210" width="50" height="50" rx="2" fill="#09090b" />
            <rect x="30" y="220" width="30" height="30" rx="1" />
            {/* Data modules */}
            <rect x="100" y="10" width="15" height="15" rx="2" />
            <rect x="120" y="30" width="15" height="15" rx="2" />
            <rect x="100" y="50" width="15" height="15" rx="2" />
            <rect x="140" y="10" width="15" height="15" rx="2" />
            <rect x="100" y="100" width="20" height="20" rx="3" />
            <rect x="130" y="100" width="20" height="20" rx="3" />
            <rect x="160" y="100" width="20" height="20" rx="3" />
            <rect x="100" y="130" width="20" height="20" rx="3" />
            <rect x="160" y="130" width="20" height="20" rx="3" />
            <rect x="200" y="100" width="15" height="15" rx="2" />
            <rect x="230" y="120" width="15" height="15" rx="2" />
            <rect x="250" y="100" width="15" height="15" rx="2" />
            <rect x="100" y="200" width="15" height="15" rx="2" />
            <rect x="120" y="220" width="15" height="15" rx="2" />
            <rect x="140" y="240" width="15" height="15" rx="2" />
            <rect x="200" y="200" width="20" height="20" rx="3" />
            <rect x="230" y="200" width="15" height="15" rx="2" />
            <rect x="250" y="230" width="15" height="15" rx="2" />
            <rect x="200" y="250" width="15" height="15" rx="2" />
          </svg>
        </div>
      </div>

      {/* Content */}
      <div className="relative z-10">
        {/* Logo */}
        <div className="flex items-center gap-3 mb-16">
          <div className="w-10 h-10 rounded-xl bg-white flex items-center justify-center">
            <svg className="w-6 h-6 text-zinc-950" fill="currentColor" viewBox="0 0 24 24">
              <rect x="2" y="2" width="6" height="6" rx="0.5" />
              <rect x="4" y="4" width="2" height="2" fill="white" />
              <rect x="16" y="2" width="6" height="6" rx="0.5" />
              <rect x="18" y="4" width="2" height="2" fill="white" />
              <rect x="2" y="16" width="6" height="6" rx="0.5" />
              <rect x="4" y="18" width="2" height="2" fill="white" />
              <rect x="10" y="2" width="2" height="2" />
              <rect x="10" y="10" width="4" height="2" />
              <rect x="16" y="16" width="2" height="2" />
              <rect x="20" y="16" width="2" height="6" />
            </svg>
          </div>
          <span className="text-xl font-bold text-white tracking-tight">QRit</span>
        </div>

        {/* Headline */}
        <h1 className="text-4xl xl:text-5xl font-bold text-white leading-tight mb-4">
          Create stunning
          <br />
          QR codes that
          <br />
          <span className="bg-gradient-to-r from-violet-400 to-violet-300 bg-clip-text text-transparent">
            get scanned
          </span>
        </h1>
        <p className="text-zinc-400 text-lg mb-12 max-w-md">
          The all-in-one platform for generating, customizing, and tracking QR codes.
        </p>

        {/* Feature list */}
        <div className="space-y-5">
          {features.map((feature) => (
            <div key={feature.title} className="flex items-start gap-4 group">
              <div className="w-9 h-9 rounded-lg bg-violet-500/10 border border-violet-500/20 flex items-center justify-center flex-shrink-0 group-hover:bg-violet-500/20 transition-colors duration-300">
                <feature.icon className="w-4.5 h-4.5 text-violet-400" />
              </div>
              <div>
                <p className="text-white font-medium text-sm">{feature.title}</p>
                <p className="text-zinc-500 text-sm">{feature.description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Trust signals at bottom */}
      <div className="relative z-10 mt-12">
        <div className="flex items-center gap-2 mb-3">
          <Shield className="w-4 h-4 text-violet-400" />
          <span className="text-xs text-zinc-500 uppercase tracking-wider font-medium">Trusted & Secure</span>
        </div>
        <div className="flex items-center gap-6 text-zinc-600 text-xs">
          <span>SSL Encrypted</span>
          <span className="w-1 h-1 rounded-full bg-zinc-700" />
          <span>No credit card required</span>
          <span className="w-1 h-1 rounded-full bg-zinc-700" />
          <span>Free forever plan</span>
        </div>
      </div>
    </div>
  );
}
