/**
 * SERVIÇO DE API - ENDPOINTS PÚBLICOS
 *
 * Este arquivo contém apenas serviços para endpoints públicos que não usam ConnectRPC.
 * Todos os outros serviços foram migrados para hooks ConnectRPC em /src/hooks/use-*-connect.ts
 *
 * Migração:
 * - wahaService            → useWhatsAppConnect
 * - loadBalancerService   → useLoadBalancerConnect
 * - adminService          → useAdminConnect
 * - automationsService    → useAutomationsConnect
 * - contactsService       → useContactsConnect
 * - shopeeService         → useShopeeConnect
 * - mercadoLivreService   → useMercadoLivreConnect
 * - aiToolsService        → useAiToolsConnect
 * - imagesService         → useImagesConnect
 * - campaignsService      → useCampaignsConnect
 * - jobsService           → useJobsConnect
 * - webhooksService       → useWebhooksConnect
 * - groupsService         → REMOVIDO (código morto)
 * - historyService        → useHistoryConnect
 * - settingsService       → useSettingsConnect
 */

import axios, { type AxiosInstance, type AxiosError, type AxiosRequestConfig } from 'axios';

/**
 * Interface for API error with additional properties
 */
interface ApiError extends Error {
  status?: number;
  details?: Record<string, unknown>;
}

/**
 * Interface for backend error response
 */
interface BackendErrorResponse {
  success?: boolean;
  error?: string;
  message?: string;
  details?: Record<string, unknown>;
}

/**
 * Interface for health check response
 */
interface HealthCheckResponse {
  status: string;
  timestamp: string;
}

/**
 * Configuração da API
 */
const API_BASE_URL = import.meta.env.VITE_API_URL || '';
const API_VERSION = 'v1';

/**
 * Cliente HTTP configurado para endpoints públicos (sem autenticação)
 */
class ApiService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: `${API_BASE_URL}/api/${API_VERSION}`,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Interceptor para tratamento de erros
    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        // Timeout / network instability: surface a friendly message to the UI
        if (error.code === 'ECONNABORTED') {
          const e: ApiError = new Error(
            'A conexão demorou para responder. Aguarde alguns segundos e tente novamente.'
          );
          e.status = 504;
          e.details = { code: 'ECONNABORTED' };
          return Promise.reject(e);
        }

        // Normalize backend errors so UI toasts show the friendly message
        const data = error.response?.data as BackendErrorResponse | undefined;
        const backendMessage =
          (data && typeof data.error === 'string' && data.error) ||
          (data && typeof data.message === 'string' && data.message) ||
          undefined;

        if (backendMessage) {
          const e: ApiError = new Error(backendMessage);
          e.status = error.response?.status;
          e.details = data?.details;
          return Promise.reject(e);
        }

        return Promise.reject(error);
      }
    );
  }

  /**
   * GET request
   */
  async get<T>(endpoint: string, params?: Record<string, unknown>): Promise<T> {
    const response = await this.client.get<T>(endpoint, { params });
    return response.data;
  }

  /**
   * POST request
   */
  async post<T>(endpoint: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.post<T>(endpoint, data, config);
    return response.data;
  }

  /**
   * Health check
   */
  async healthCheck(): Promise<HealthCheckResponse> {
    return this.get('/health');
  }
}

// Exportar instância singleton
export const api = new ApiService();

/**
 * ===============================================
 * REDIRECT (endpoint público - sem autenticação)
 * ===============================================
 */
/** FSQ13: Runtime type guard for redirect response */
interface RedirectResponse {
  url?: string;
  redirect_url?: string;
  [key: string]: unknown;
}

function isRedirectResponse(data: unknown): data is RedirectResponse {
  return typeof data === 'object' && data !== null;
}

export const redirectService = {
  /**
   * Processa link de redirecionamento por slug (uso público /r/:slug)
   * Este endpoint é público e não requer autenticação.
   */
  async processRedirectBySlug(slug: string): Promise<RedirectResponse> {
    const response: unknown = await api.post('/redirect', { slug });
    const data = (response as { data?: unknown })?.data ?? response;
    if (!isRedirectResponse(data)) {
      throw new Error('Invalid redirect response format');
    }
    return data;
  },
};

export default api;
