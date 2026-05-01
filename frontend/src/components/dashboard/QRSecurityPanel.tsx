"use client";

import { useState } from "react";
import { Lock, Clock, Hash, Globe } from "lucide-react";

export interface SecurityOptions {
  password: string;
  expiresAt: string;       // ISO date string or ""
  maxScans: string;        // numeric string or ""
  geoRestrictions: string; // comma-separated country codes or ""
}

interface Props {
  value: SecurityOptions;
  onChange: (v: SecurityOptions) => void;
}

const GEO_SUGGESTIONS = [
  { code: "US", label: "United States" },
  { code: "GB", label: "United Kingdom" },
  { code: "IN", label: "India" },
  { code: "DE", label: "Germany" },
  { code: "CA", label: "Canada" },
  { code: "AU", label: "Australia" },
];

export function QRSecurityPanel({ value, onChange }: Props) {
  const [geoInput, setGeoInput] = useState("");

  const set = (k: keyof SecurityOptions, v: string) => onChange({ ...value, [k]: v });

  const addGeo = (code: string) => {
    const codes = value.geoRestrictions
      ? value.geoRestrictions.split(",").map((c) => c.trim())
      : [];
    if (!codes.includes(code)) {
      set("geoRestrictions", [...codes, code].join(","));
    }
    setGeoInput("");
  };

  const removeGeo = (code: string) => {
    const codes = value.geoRestrictions
      .split(",")
      .map((c) => c.trim())
      .filter((c) => c !== code);
    set("geoRestrictions", codes.join(","));
  };

  const activeCodes = value.geoRestrictions
    ? value.geoRestrictions.split(",").map((c) => c.trim()).filter(Boolean)
    : [];

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
        <Lock size={14} className="text-violet-400" />
        Security &amp; Access Control
      </h3>

      <div className="grid sm:grid-cols-2 gap-3">
        {/* Password */}
        <div>
          <label className="block text-xs text-zinc-500 mb-1.5 flex items-center gap-1.5">
            <Lock size={11} /> Password Protection
          </label>
          <input
            type="password"
            value={value.password}
            onChange={(e) => set("password", e.target.value)}
            placeholder="Leave blank for no password"
            className="w-full bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500"
          />
        </div>

        {/* Expiry */}
        <div>
          <label className="block text-xs text-zinc-500 mb-1.5 flex items-center gap-1.5">
            <Clock size={11} /> Expiry Date
          </label>
          <input
            type="datetime-local"
            value={value.expiresAt}
            onChange={(e) => set("expiresAt", e.target.value)}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:border-violet-500 [color-scheme:dark]"
          />
        </div>

        {/* Max scans */}
        <div>
          <label className="block text-xs text-zinc-500 mb-1.5 flex items-center gap-1.5">
            <Hash size={11} /> Max Scans
          </label>
          <input
            type="number"
            min={1}
            value={value.maxScans}
            onChange={(e) => set("maxScans", e.target.value)}
            placeholder="Unlimited"
            className="w-full bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500"
          />
        </div>

        {/* Geo restrictions */}
        <div>
          <label className="block text-xs text-zinc-500 mb-1.5 flex items-center gap-1.5">
            <Globe size={11} /> Allowed Countries
          </label>
          <div className="flex gap-1.5">
            <input
              type="text"
              value={geoInput}
              onChange={(e) => setGeoInput(e.target.value.toUpperCase())}
              onKeyDown={(e) => {
                if (e.key === "Enter" && geoInput.trim()) {
                  e.preventDefault();
                  addGeo(geoInput.trim());
                }
              }}
              placeholder="e.g. US, IN"
              maxLength={2}
              className="flex-1 bg-zinc-900 border border-zinc-700 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:border-violet-500"
            />
            <button
              type="button"
              onClick={() => geoInput.trim() && addGeo(geoInput.trim())}
              className="px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-300 hover:bg-zinc-700"
            >
              Add
            </button>
          </div>
          {/* Quick-add suggestions */}
          <div className="flex flex-wrap gap-1 mt-2">
            {GEO_SUGGESTIONS.filter((s) => !activeCodes.includes(s.code)).map((s) => (
              <button
                key={s.code}
                type="button"
                onClick={() => addGeo(s.code)}
                className="text-[10px] px-2 py-0.5 bg-zinc-800 border border-zinc-700 rounded text-zinc-400 hover:text-zinc-200 hover:border-zinc-500"
              >
                +{s.code}
              </button>
            ))}
          </div>
          {/* Active codes */}
          {activeCodes.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-2">
              {activeCodes.map((code) => (
                <span
                  key={code}
                  className="inline-flex items-center gap-1 text-[10px] px-2 py-0.5 bg-violet-900/40 border border-violet-700 rounded text-violet-300"
                >
                  {code}
                  <button
                    type="button"
                    onClick={() => removeGeo(code)}
                    className="hover:text-white"
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
