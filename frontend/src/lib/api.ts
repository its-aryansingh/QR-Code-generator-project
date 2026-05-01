import axios, { AxiosInstance, AxiosError } from 'axios';
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

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081/api/v1';
const BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8081';

class ApiClient {
    private client: AxiosInstance;

    constructor() {
        this.client = axios.create({
            baseURL: API_URL,
            headers: {
                'Content-Type': 'application/json',
            },
        });

        // Request interceptor to add auth token
        this.client.interceptors.request.use((config) => {
            const token = useAuthStore.getState().accessToken;
            if (token) {
                config.headers.Authorization = `Bearer ${token}`;
            }
            return config;
        });

        // Response interceptor to handle token refresh
        this.client.interceptors.response.use(
            (response) => response,
            async (error: AxiosError) => {
                const originalRequest = error.config;

                if (error.response?.status === 401 && originalRequest) {
                    const refreshToken = useAuthStore.getState().refreshToken;

                    if (refreshToken) {
                        try {
                            const response = await axios.post<ApiResponse<TokenPair>>(
                                `${API_URL}/auth/refresh`,
                                { refresh_token: refreshToken }
                            );

                            if (response.data.success && response.data.data) {
                                useAuthStore.getState().setTokens(response.data.data);
                                originalRequest.headers.Authorization = `Bearer ${response.data.data.access_token}`;
                                return this.client(originalRequest);
                            }
                        } catch {
                            useAuthStore.getState().logout();
                        }
                    } else {
                        useAuthStore.getState().logout();
                    }
                }

                return Promise.reject(error);
            }
        );
    }

    // ==================== Auth Endpoints ====================

    async register(data: RegisterRequest): Promise<ApiResponse<User>> {
        const response = await this.client.post<ApiResponse<User>>('/auth/register', data);
        return response.data;
    }

    async login(data: LoginRequest): Promise<ApiResponse<TokenPair>> {
        const response = await this.client.post<ApiResponse<TokenPair>>('/auth/login', data);
        return response.data;
    }

    async googleLogin(idToken: string): Promise<ApiResponse<TokenPair>> {
        const response = await this.client.post<ApiResponse<TokenPair>>('/auth/google', { id_token: idToken });
        return response.data;
    }

    async refresh(refreshToken: string): Promise<ApiResponse<TokenPair>> {
        const response = await this.client.post<ApiResponse<TokenPair>>('/auth/refresh', {
            refresh_token: refreshToken,
        });
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
