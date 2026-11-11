"use client";

import { useSyncExternalStore } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";

import { cn } from "@/lib/utils";

const OPTIONS = [
  { value: "light", label: "Light", Icon: Sun },
  { value: "dark", label: "Dark", Icon: Moon },
  { value: "system", label: "System", Icon: Monitor },
] as const;

const subscribe = () => () => undefined;

/** False during server render and hydration, so the saved theme never causes a mismatch. */
function useMounted() {
  return useSyncExternalStore(subscribe, () => true, () => false);
}

export function ThemeToggle({
  variant = "segmented",
  className,
}: {
  variant?: "segmented" | "cycle";
  className?: string;
}) {
  const { theme, setTheme } = useTheme();
  const mounted = useMounted();
  const current = mounted ? theme ?? "system" : undefined;

  if (variant === "cycle") {
    const index = OPTIONS.findIndex((option) => option.value === current);
    const active = OPTIONS[index < 0 ? 2 : index];
    const next = OPTIONS[(Math.max(index, 0) + 1) % OPTIONS.length];
    return (
      <button
        type="button"
        onClick={() => setTheme(next.value)}
        className={cn(
          "flex size-11 items-center justify-center rounded-lg text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-100",
          className,
        )}
        aria-label={`Theme: ${active.label}. Switch to ${next.label}`}
        title={`Theme: ${active.label}`}
      >
        <active.Icon className="size-5" aria-hidden />
      </button>
    );
  }

  return (
    <div
      role="radiogroup"
      aria-label="Theme"
      className={cn("inline-flex items-center gap-0.5 rounded-lg border border-zinc-700 p-0.5", className)}
    >
      {OPTIONS.map(({ value, label, Icon }) => {
        const selected = current === value;
        return (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={label}
            title={label}
            onClick={() => setTheme(value)}
            className={cn(
              "flex size-8 items-center justify-center rounded-md transition-colors",
              selected
                ? "bg-zinc-800 text-zinc-50"
                : "text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-100",
            )}
          >
            <Icon className="size-4" aria-hidden />
          </button>
        );
      })}
    </div>
  );
}
