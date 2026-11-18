"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";

/** Light / dark / system, applied as a class on <html> before first paint. */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
      storageKey="qrit-theme"
    >
      {children}
    </NextThemesProvider>
  );
}
