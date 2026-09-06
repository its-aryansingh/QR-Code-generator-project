"use client";

import Link from "next/link";
import { ArrowRight, LayoutTemplate } from "lucide-react";

export default function TemplatesPage() {
  return (
    <div className="max-w-3xl space-y-6">
      <div>
        <div className="flex items-center gap-3">
          <LayoutTemplate className="size-6 text-violet-400" />
          <h1 className="text-2xl font-bold text-zinc-100">Templates</h1>
        </div>
        <p className="mt-1 text-sm text-zinc-500">Start from a ready-made QR code format.</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        {["URL", "vCard", "WiFi", "Event"].map((template) => (
          <Link
            key={template}
            href="/dashboard/create"
            className="group rounded-xl border border-zinc-800 bg-zinc-900/60 p-5 transition-colors hover:border-violet-500/50"
          >
            <h2 className="font-semibold text-zinc-200">{template} QR code</h2>
            <p className="mt-1 text-sm text-zinc-500">Create a {template} code from this template.</p>
            <span className="mt-4 flex items-center gap-1 text-sm text-violet-400">
              Use template <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
