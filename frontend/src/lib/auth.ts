import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { TokenPair } from '@/types';

interface AuthState {
    accessToken: string | null;
    refreshToken: string | null;
    isAuthenticated: boolean;
    setTokens: (tokens: TokenPair) => void;
    logout: () => void;
}

export const useAuthStore = create<AuthState>()(
    persist(
        (set) => ({
            accessToken: null,
            refreshToken: null,
            isAuthenticated: false,
            setTokens: (tokens: TokenPair) =>
                set({
                    accessToken: tokens.access_token,
                    refreshToken: tokens.refresh_token,
                    isAuthenticated: true,
                }),
            logout: () =>
                set({
                    accessToken: null,
                    refreshToken: null,
                    isAuthenticated: false,
                }),
        }),
        {
            name: 'qrapp-auth',
            partialize: (state) => ({
                accessToken: state.accessToken,
                refreshToken: state.refreshToken,
                isAuthenticated: state.isAuthenticated,
            }),
        }
    )
);
