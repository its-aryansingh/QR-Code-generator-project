import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios';
import { useAuthStore } from './auth';
import type {
    ApiResponse,
    TokenPair,
    User,
    LoginRequest,
    RegisterRequest,
    GenerateQRRequest,
    QRGenerateResponse,
    QRHistoryResponse,
    QRType,
    PublicGenerateRequest,
    PublicGenerateResponse,
    ScanAnalytics,
    QRRecord,
} from '@/types';

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8084/api/v1";
export const BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8084";

class ApiClient {
    private client: AxiosInstance;
    private isRefreshing = false;
    private refreshPromise: Promise<boolean> | null = null;

    constructor() {
        this.client = axios.create({
            baseURL: API_URL,
            headers: {
                'Content-Type': 'application/json',
            },
        });

        // Request interceptor: add auth token + proactive refresh
        this.client.interceptors.request.use(async (config) => {
            const state = useAuthStore.getState();

            // Proactive refresh: if token expires within 2 minutes, refresh first
            if (
                state.accessToken &&
                state.tokenExpiresAt &&
                state.tokenExpiresAt - Date.now() < 2 * 60 * 1000 &&
                state.refreshToken &&
                !config.url?.includes('/auth/refresh')
            ) {
                await this.refreshTokens();
            }

            const token = useAuthStore.getState().accessToken;
            if (token) {
                config.headers.Authorization = `Bearer ${token}`;
            }
            return config;
        });

        // Response interceptor: handle 401 with queued refresh
        this.client.interceptors.response.use(
            (response) => response,
            async (error: AxiosError) => {
                const originalRequest = error.config as InternalAxiosRequestConfig & { _retried?: boolean };

                if (
                    error.response?.status === 401 &&
                    originalRequest &&
                    !originalRequest._retried &&
                    !originalRequest.url?.includes('/auth/refresh') &&
                    !originalRequest.url?.includes('/auth/login')
                ) {
                    originalRequest._retried = true;

                    const refreshed = await this.refreshTokens();
                    if (refreshed) {
                        const token = useAuthStore.getState().accessToken;
                        originalRequest.headers.Authorization = `Bearer ${token}`;
                        return this.client(originalRequest);
                    } else {
                        useAuthStore.getState().logout();
                    }
                }

                return Promise.reject(error);
            }
        );
    }

    /**
     * Refresh tokens with deduplication — if a refresh is already in-flight,
     * all callers wait for the same promise instead of firing parallel requests.
     */
    private async refreshTokens(): Promise<boolean> {
        if (this.refreshPromise) {
            return this.refreshPromise;
        }

        this.refreshPromise = (async () => {
            const refreshToken = useAuthStore.getState().refreshToken;
            if (!refreshToken) return false;

            try {
                const response = await axios.post<ApiResponse<TokenPair>>(
                    `${API_URL}/auth/refresh`,
                    { refresh_token: refreshToken }
                );

                if (response.data.success && response.data.data) {
                    useAuthStore.getState().setTokens(response.data.data);
                    return true;
                }
                return false;
            } catch {
                return false;
            }
        })();

        try {
            return await this.refreshPromise;
        } finally {
            this.refreshPromise = null;
        }
    }

    // ==================== Auth Endpoints ====================

    async register(data: RegisterRequest): Promise<ApiResponse<TokenPair & { user: User }>> {
        const response = await this.client.post<ApiResponse<TokenPair & { user: User }>>('/auth/register', data);
        return response.data;
    }

    async login(data: LoginRequest): Promise<ApiResponse<TokenPair & { user: User }>> {
        const response = await this.client.post<ApiResponse<TokenPair & { user: User }>>('/auth/login', data);
        return response.data;
    }

    async googleLogin(idToken: string): Promise<ApiResponse<TokenPair & { user: User }>> {
        const response = await this.client.post<ApiResponse<TokenPair & { user: User }>>('/auth/google', { id_token: idToken });
        return response.data;
    }

    async refresh(refreshToken: string): Promise<ApiResponse<TokenPair>> {
        const response = await this.client.post<ApiResponse<TokenPair>>('/auth/refresh', {
            refresh_token: refreshToken,
        });
        return response.data;
    }

    async logout(refreshToken?: string, allDevices = false): Promise<ApiResponse<{ message: string }>> {
        const response = await this.client.post<ApiResponse<{ message: string }>>('/auth/logout', {
            refresh_token: refreshToken,
            all_devices: allDevices,
        });
        return response.data;
    }

    async forgotPassword(email: string): Promise<ApiResponse<{ message: string }>> {
        const response = await this.client.post<ApiResponse<{ message: string }>>('/auth/forgot-password', { email });
        return response.data;
    }

    async resetPassword(token: string, password: string): Promise<ApiResponse<{ message: string }>> {
        const response = await this.client.post<ApiResponse<{ message: string }>>('/auth/reset-password', { token, password });
        return response.data;
    }

    async verifyEmail(token: string): Promise<ApiResponse<{ message: string }>> {
        const response = await this.client.post<ApiResponse<{ message: string }>>('/auth/verify-email', { token });
        return response.data;
    }

    async resendVerification(): Promise<ApiResponse<{ message: string }>> {
        const response = await this.client.post<ApiResponse<{ message: string }>>('/auth/resend-verification');
        return response.data;
    }

    // ==================== Public Endpoints (No Auth) ====================

    async getQRTypes(): Promise<ApiResponse<QRType[]>> {
        const response = await this.client.get<ApiResponse<QRType[]>>('/public/types');
        return response.data;
    }

    async getQuota(): Promise<{ remaining: number; limit: number }> {
        const response = await this.client.get<{ success: boolean; remaining: number; limit: number }>('/public/quota');
        return { remaining: response.data.remaining, limit: response.data.limit };
    }

    async publicGenerate(data: PublicGenerateRequest): Promise<PublicGenerateResponse> {
        const response = await this.client.post<PublicGenerateResponse>('/public/generate', data);
        return response.data;
    }

    // ==================== Protected QR Endpoints ====================

    async generateQR(data: GenerateQRRequest): Promise<ApiResponse<QRGenerateResponse>> {
        const response = await this.client.post<ApiResponse<QRGenerateResponse>>('/qr/generate', data);
        return response.data;
    }

    async getQRHistory(page = 1, pageSize = 20): Promise<ApiResponse<QRHistoryResponse>> {
        const response = await this.client.get<ApiResponse<QRHistoryResponse>>(
            `/qr/history?page=${page}&page_size=${pageSize}`
        );
        return response.data;
    }

    async getQRById(id: string): Promise<ApiResponse<QRRecord>> {
        const response = await this.client.get<ApiResponse<QRRecord>>(`/qr/${id}`);
        return response.data;
    }

    async updateQR(id: string, data: { title?: string; redirect_url?: string; is_active?: boolean }): Promise<ApiResponse<QRRecord>> {
        const response = await this.client.put<ApiResponse<QRRecord>>(`/qr/${id}`, data);
        return response.data;
    }

    // ==================== Analytics Endpoints ====================

    async getQRAnalytics(id: string, days = 30): Promise<ApiResponse<{ qr_id: string; title: string; analytics: ScanAnalytics }>> {
        const response = await this.client.get(`/qr/${id}/analytics?days=${days}`);
        return response.data;
    }

    async getUserAnalyticsSummary(): Promise<ApiResponse<{ total_qr_codes: number; dynamic_qr_codes: number; total_scans: number }>> {
        const response = await this.client.get('/analytics/summary');
        return response.data;
    }

    // ==================== Public Analytics ====================

    async getPublicAnalytics(shortCode: string): Promise<ApiResponse<{ analytics: ScanAnalytics; recent_scans: any[] }>> {
        const response = await this.client.get(`/public/analytics/${shortCode}`);
        return response.data;
    }

    // ==================== Utility ====================

    getRedirectURL(shortCode: string): string {
        return `${BASE_URL}/r/${shortCode}`;
    }
}

export const api = new ApiClient();
