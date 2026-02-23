import { Code, ConnectError, type Interceptor, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import * as Sentry from '@sentry/react';
import { sentryInterceptor } from './sentry-connect-interceptor';

// ============================================================
// CSRF Token Management
// ============================================================

/**
 * CSRF token stored in memory (never localStorage for security).
 * The token is received from the server on login/refresh and must be
 * sent in the X-CSRF-Token header for all mutating requests.
 */
let csrfToken: string | null = null;

/**
 * Sets the CSRF token received from the server.
 * Call this after login or session refresh.
 */
export function setCsrfToken(token: string): void {
  csrfToken = token;
}

/**
 * Clears the CSRF token.
 * Call this on logout.
 */
export function clearCsrfToken(): void {
  csrfToken = null;
}

/**
 * Gets the current CSRF token.
 * Returns null if no token is set.
 */
export function getCsrfToken(): string | null {
  return csrfToken;
}

/**
 * Generates a random trace ID for distributed tracing.
 * Format: 32 hex characters (128 bits) as per W3C Trace Context.
 */
function generateTraceId(): string {
  const array = new Uint8Array(16);
  safeGetRandomValues(array);
  return Array.from(array)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Generates a random span ID for distributed tracing.
 * Format: 16 hex characters (64 bits) as per W3C Trace Context.
 */
function generateSpanId(): string {
  const array = new Uint8Array(8);
  safeGetRandomValues(array);
  return Array.from(array)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Safe wrapper around crypto.getRandomValues with Math.random fallback.
 * Covers environments where crypto API is unavailable (older browsers, some test envs).
 */
function safeGetRandomValues(array: Uint8Array): void {
  try {
    crypto.getRandomValues(array);
  } catch {
    Sentry.captureMessage('crypto.getRandomValues unavailable', 'warning');
    for (let i = 0; i < array.length; i++) {
      array[i] = Math.floor(Math.random() * 256);
    }
  }
}

/**
 * Generates a traceparent header value following W3C Trace Context spec.
 * @see https://www.w3.org/TR/trace-context/
 */
function generateTraceparent(): string {
  const version = '00';
  const traceId = generateTraceId();
  const spanId = generateSpanId();
  const flags = '01'; // sampled
  return `${version}-${traceId}-${spanId}-${flags}`;
}

/**
 * Checks if a gRPC method is idempotent and safe to retry.
 * Idempotent methods: GET-like operations that don't modify state.
 */
function isIdempotentMethod(methodName: string): boolean {
  const idempotentPrefixes = ['List', 'Get', 'Check', 'Fetch', 'Search', 'Find', 'Count'];
  return idempotentPrefixes.some((prefix) => methodName.startsWith(prefix));
}

/**
 * Checks if an error code is retryable (transient failure).
 */
function isRetryableError(code: Code): boolean {
  return (
    code === Code.Unavailable ||
    code === Code.ResourceExhausted ||
    code === Code.Aborted ||
    code === Code.DeadlineExceeded
  );
}

/**
 * Calculates exponential backoff delay with jitter.
 * @param attempt - Current retry attempt (0-indexed)
 * @param baseDelayMs - Base delay in milliseconds (default: 100ms)
 * @param maxDelayMs - Maximum delay cap in milliseconds (default: 5000ms)
 */
function calculateBackoffDelay(attempt: number, baseDelayMs = 100, maxDelayMs = 5000): number {
  const exponentialDelay = baseDelayMs * 2 ** attempt;
  const cappedDelay = Math.min(exponentialDelay, maxDelayMs);
  // Add jitter: random value between 0 and delay
  const jitter = Math.random() * cappedDelay;
  return Math.floor(cappedDelay + jitter);
}

/**
 * Waits for specified milliseconds.
 */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Structured error information for UI handling.
 */
export interface ConnectErrorInfo {
  code: Code;
  message: string;
  userMessage: string;
  action: 'inline' | 'toast' | 'redirect' | 'retry' | 'none';
  redirectTo?: string;
  retryable: boolean;
}

/**
 * Maps gRPC error codes to user-friendly messages and UI actions.
 * Based on standard gRPC error code semantics.
 */
export function mapConnectError(error: ConnectError): ConnectErrorInfo {
  const code = error.code;
  const message = error.message;

  switch (code) {
    case Code.InvalidArgument:
      return {
        code,
        message,
        userMessage: 'Dados inválidos. Verifique os campos e tente novamente.',
        action: 'inline',
        retryable: false,
      };

    case Code.Unauthenticated:
      return {
        code,
        message,
        userMessage: 'Sessão expirada. Faça login novamente.',
        action: 'redirect',
        redirectTo: '/auth',
        retryable: false,
      };

    case Code.PermissionDenied:
      return {
        code,
        message,
        userMessage: 'Você não tem permissão para realizar esta ação.',
        action: 'toast',
        retryable: false,
      };

    case Code.NotFound:
      return {
        code,
        message,
        userMessage: 'Recurso não encontrado.',
        action: 'toast',
        retryable: false,
      };

    case Code.AlreadyExists:
      return {
        code,
        message,
        userMessage: 'Este item já existe.',
        action: 'inline',
        retryable: false,
      };

    case Code.ResourceExhausted:
      return {
        code,
        message,
        userMessage: 'Limite de requisições atingido. Aguarde um momento.',
        action: 'toast',
        retryable: true,
      };

    case Code.FailedPrecondition:
      return {
        code,
        message,
        userMessage: 'Operação não permitida no estado atual.',
        action: 'toast',
        retryable: false,
      };

    case Code.Aborted:
      return {
        code,
        message,
        userMessage: 'Operação cancelada. Tente novamente.',
        action: 'toast',
        retryable: true,
      };

    case Code.OutOfRange:
      return {
        code,
        message,
        userMessage: 'Valor fora do intervalo permitido.',
        action: 'inline',
        retryable: false,
      };

    case Code.Unimplemented:
      return {
        code,
        message,
        userMessage: 'Funcionalidade não disponível.',
        action: 'toast',
        retryable: false,
      };

    case Code.Internal:
      return {
        code,
        message,
        userMessage: 'Erro interno do servidor. Tente novamente mais tarde.',
        action: 'toast',
        retryable: true,
      };

    case Code.Unavailable:
      return {
        code,
        message,
        userMessage: 'Serviço temporariamente indisponível. Tentando reconectar...',
        action: 'retry',
        retryable: true,
      };

    case Code.DataLoss:
      return {
        code,
        message,
        userMessage: 'Erro de dados. Entre em contato com o suporte.',
        action: 'toast',
        retryable: false,
      };

    case Code.DeadlineExceeded:
      return {
        code,
        message,
        userMessage: 'A operação demorou muito. Tente novamente.',
        action: 'toast',
        retryable: true,
      };

    case Code.Canceled:
      return {
        code,
        message,
        userMessage: 'Operação cancelada.',
        action: 'none',
        retryable: false,
      };

    default:
      return {
        code,
        message,
        userMessage: 'Ocorreu um erro. Tente novamente.',
        action: 'toast',
        retryable: false,
      };
  }
}

/**
 * Helper to check if an error is a ConnectError.
 */
export function isConnectError(error: unknown): error is ConnectError {
  return error instanceof ConnectError;
}

/**
 * Extracts ConnectErrorInfo from any error.
 * Returns a generic error info for non-Connect errors.
 */
export function getErrorInfo(error: unknown): ConnectErrorInfo {
  if (isConnectError(error)) {
    return mapConnectError(error);
  }

  // Network errors (fetch failed, etc.)
  if (error instanceof Error) {
    if (error.name === 'AbortError') {
      return {
        code: Code.Canceled,
        message: error.message,
        userMessage: 'Operação cancelada.',
        action: 'none',
        retryable: false,
      };
    }

    // Generic network error
    return {
      code: Code.Unavailable,
      message: error.message,
      userMessage: 'Erro de conexão. Verifique sua internet e tente novamente.',
      action: 'toast',
      retryable: true,
    };
  }

  // Unknown error type
  return {
    code: Code.Unknown,
    message: String(error),
    userMessage: 'Ocorreu um erro inesperado.',
    action: 'toast',
    retryable: false,
  };
}

/**
 * CSRF interceptor: Adds CSRF token to mutating requests.
 * The token is stored in memory and received from the server.
 */
const csrfInterceptor: Interceptor = (next) => async (req) => {
  // Add CSRF token if available
  if (csrfToken) {
    req.header.set('X-CSRF-Token', csrfToken);
  }

  return next(req);
};

/**
 * Tracing interceptor: Adds W3C Trace Context headers for distributed tracing.
 * Enables correlation of requests across services.
 */
const tracingInterceptor: Interceptor = (next) => async (req) => {
  const traceparent = generateTraceparent();
  req.header.set('traceparent', traceparent);

  // Store trace ID for potential error logging
  const traceId = traceparent.split('-')[1];
  req.header.set('x-trace-id', traceId);

  return next(req);
};

/**
 * Retry interceptor: Implements exponential backoff for transient failures.
 * Only retries idempotent methods to avoid side effects.
 *
 * @param maxRetries - Maximum number of retry attempts (default: 3)
 */
function createRetryInterceptor(maxRetries = 3): Interceptor {
  return (next) => async (req) => {
    const methodName = req.method.name;
    const canRetry = isIdempotentMethod(methodName);

    let lastError: unknown;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        // Check if request was aborted
        if (req.signal?.aborted) {
          throw new ConnectError('Request aborted', Code.Canceled);
        }

        return await next(req);
      } catch (error) {
        lastError = error;

        // Don't retry if not idempotent
        if (!canRetry) {
          throw error;
        }

        // Don't retry if signal is aborted
        if (req.signal?.aborted) {
          throw error;
        }

        // Check if error is retryable
        if (isConnectError(error) && isRetryableError(error.code)) {
          // Don't retry if we've exhausted attempts
          if (attempt >= maxRetries) {
            throw error;
          }

          // Wait before retrying with exponential backoff
          const delay = calculateBackoffDelay(attempt);
          await sleep(delay);

          continue;
        }

        // Non-retryable error, throw immediately
        throw error;
      }
    }

    // Should not reach here, but throw last error just in case
    throw lastError;
  };
}

/**
 * Error normalization interceptor: Enriches errors with additional context.
 * Logs errors for debugging while keeping sensitive info server-side.
 */
const errorNormalizationInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (error) {
    // Log error with trace context for debugging
    const traceId = req.header.get('x-trace-id');

    if (isConnectError(error)) {
      // Log structured error info (without sensitive data)
      console.error('[ConnectRPC Error]', {
        method: req.method.name,
        service: req.service.typeName,
        code: Code[error.code],
        traceId,
      });
    } else {
      console.error('[ConnectRPC Network Error]', {
        method: req.method.name,
        service: req.service.typeName,
        error: error instanceof Error ? error.message : 'Unknown error',
        traceId,
      });
    }

    throw error;
  }
};

/**
 * Auth refresh interceptor: Automatically refreshes expired sessions.
 * On Unauthenticated error, attempts to call RefreshSession RPC once.
 * If refresh succeeds, retries the original request. If it fails, redirects to /auth.
 * Deduplicates concurrent refresh attempts to avoid race conditions.
 */
function createAuthRefreshInterceptor(): Interceptor {
  let refreshPromise: Promise<boolean> | null = null;

  // Bare transport for refresh calls only (no retry/refresh interceptors to avoid loops)
  const refreshTransport = createConnectTransport({
    baseUrl: import.meta.env.VITE_API_URL ? `${import.meta.env.VITE_API_URL}/connect` : '/connect',
    credentials: import.meta.env.VITE_API_URL ? 'include' : 'same-origin',
    interceptors: [csrfInterceptor],
  });

  return (next) => async (req) => {
    // Skip refresh logic for auth-related methods (avoid infinite loops)
    const skipMethods = [
      'RefreshSession',
      'Login',
      'Register',
      'Logout',
      'VerifyMagicLink',
      'GetCurrentUser',
      'ForgotPassword',
      'SendMagicLink',
      'VerifyEmail',
      'ResendVerificationEmail',
    ];
    if (skipMethods.includes(req.method.name)) {
      return next(req);
    }

    try {
      return await next(req);
    } catch (error) {
      if (!(error instanceof ConnectError) || error.code !== Code.Unauthenticated) {
        throw error;
      }

      // Deduplicate concurrent refresh attempts
      if (!refreshPromise) {
        refreshPromise = (async () => {
          try {
            // Dynamic import to avoid circular dependency
            const { AuthService } = await import('@/gen/auth/v1/auth_pb');
            const refreshClient = createClient(AuthService, refreshTransport);
            const response = await refreshClient.refreshSession({});

            // Update CSRF token if returned
            if (response.csrfToken) {
              setCsrfToken(response.csrfToken);
            }
            return true;
          } catch {
            return false;
          } finally {
            setTimeout(() => {
              refreshPromise = null;
            }, 1000);
          }
        })();
      }

      const refreshed = await refreshPromise;
      if (refreshed) {
        return await next(req);
      }

      // Refresh failed — redirect to login
      window.location.href = '/auth';
      throw error;
    }
  };
}

/**
 * ConnectRPC transport configuration with production-ready interceptors.
 *
 * Interceptor order (executed in sequence):
 * 1. CSRF: Adds CSRF token to requests
 * 2. Tracing: Adds traceparent header
 * 3. Retry: Handles transient failures with backoff
 * 4. Auth refresh: Handles expired sessions with automatic refresh
 * 5. Error normalization: Logs and enriches errors
 * 6. Sentry: Captures errors with gRPC context (must be LAST)
 *
 * IMPORTANT: Uses `credentials: 'include'` to automatically send
 * HTTP-only session cookies with every request.
 *
 * @example
 * ```typescript
 * import { transport } from '@/lib/connect-transport';
 * import { useTransport } from '@connectrpc/connect-query';
 *
 * // In component
 * const transport = useTransport();
 * const client = createClient(MyService, transport);
 * ```
 */
export const transport = createConnectTransport({
  // In development, use relative path to go through Vite proxy (solves cross-origin cookie issues)
  // In production, use the full API URL
  baseUrl: import.meta.env.VITE_API_URL ? `${import.meta.env.VITE_API_URL}/connect` : '/connect',
  // Send cookies with requests
  // Use 'same-origin' in development (with proxy) or 'include' in production (cross-origin)
  credentials: import.meta.env.VITE_API_URL ? 'include' : 'same-origin',
  interceptors: [
    csrfInterceptor,
    tracingInterceptor,
    createRetryInterceptor(3),
    createAuthRefreshInterceptor(),
    errorNormalizationInterceptor,
    sentryInterceptor,
  ],
});

/**
 * Creates a transport with custom configuration.
 * Useful for specific use cases like longer timeouts.
 *
 * @param options - Custom transport options
 */
export function createCustomTransport(options?: {
  maxRetries?: number;
  baseUrl?: string;
}) {
  const defaultBaseUrl = import.meta.env.VITE_API_URL
    ? `${import.meta.env.VITE_API_URL}/connect`
    : '/connect';
  const { maxRetries = 3, baseUrl = defaultBaseUrl } = options ?? {};

  return createConnectTransport({
    baseUrl,
    credentials: import.meta.env.VITE_API_URL ? 'include' : 'same-origin',
    interceptors: [
      csrfInterceptor,
      tracingInterceptor,
      createRetryInterceptor(maxRetries),
      createAuthRefreshInterceptor(),
      errorNormalizationInterceptor,
      sentryInterceptor,
    ],
  });
}
