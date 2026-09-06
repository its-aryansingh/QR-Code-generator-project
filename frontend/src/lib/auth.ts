import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Profile } from "@/types/enterprise";

export interface TokenResponse {
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
}

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  tokenExpiresAt: number | null;
  returnUrl: string | null;
  profile: Profile | null;
  hydrated: boolean;
  setTokens: (tokens: TokenResponse) => void;
  setProfile: (profile: Profile | null) => void;
  setReturnUrl: (url: string | null) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
      tokenExpiresAt: null,
      returnUrl: null,
      profile: null,
      hydrated: false,

      setTokens: (tokens) => {
        const access = tokens.access_token ?? null;
        const refresh = tokens.refresh_token ?? null;
        const expiresAt = tokens.expires_in
          ? Date.now() + tokens.expires_in * 1000
          : null;
        set({
          accessToken: access,
          refreshToken: refresh,
          isAuthenticated: Boolean(access),
          tokenExpiresAt: expiresAt,
        });
      },

      setProfile: (profile) => set({ profile }),

      setReturnUrl: (url) => set({ returnUrl: url }),

      logout: () =>
        set({
          accessToken: null,
          refreshToken: null,
          isAuthenticated: false,
          tokenExpiresAt: null,
          returnUrl: null,
          profile: null,
        }),
    }),
    {
      name: "qrapp-auth",
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        isAuthenticated: state.isAuthenticated,
        tokenExpiresAt: state.tokenExpiresAt,
        profile: state.profile,
      }),
      onRehydrateStorage: () => (state) => {
        if (!state) return;
        const invalid = (value: string | null) =>
          !value || value === "undefined" || value === "null";
        if (invalid(state.accessToken)) {
          state.accessToken = null;
          state.refreshToken = null;
          state.isAuthenticated = false;
          state.tokenExpiresAt = null;
          state.profile = null;
        }
        state.hydrated = true;
      },
    },
  ),
);
