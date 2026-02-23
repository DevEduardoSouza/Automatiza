import * as Sentry from '@sentry/react';

// ============================================================
// PII Scrubbing Patterns
// ============================================================

const EMAIL_REGEX = /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g;
const PHONE_REGEX = /(\+?\d{1,3}[-.\s]?)?\(?\d{2,3}\)?[-.\s]?\d{4,5}[-.\s]?\d{4}/g;
const TOKEN_PARAM_REGEX =
  /([?&])(token|key|secret|password|credential|auth|api_key|apikey|access_token|refresh_token)=[^&]*/gi;

function stripSensitiveParams(url: string): string {
  return url.replace(TOKEN_PARAM_REGEX, '$1$2=[REDACTED]');
}

function scrubPII(text: string): string {
  return text.replace(EMAIL_REGEX, '[EMAIL_REDACTED]').replace(PHONE_REGEX, '[PHONE_REDACTED]');
}

// ============================================================
// Safe localStorage (private browsing fallback)
// ============================================================

const memoryStorage = new Map<string, string>();

/**
 * Safe localStorage wrapper with in-memory fallback.
 * Prevents crashes in private browsing mode (Safari/Firefox).
 */
export const safeLocalStorage = {
  getItem(key: string): string | null {
    try {
      return localStorage.getItem(key);
    } catch {
      return memoryStorage.get(key) ?? null;
    }
  },
  setItem(key: string, value: string): void {
    try {
      localStorage.setItem(key, value);
    } catch {
      memoryStorage.set(key, value);
    }
  },
  removeItem(key: string): void {
    try {
      localStorage.removeItem(key);
    } catch {
      memoryStorage.delete(key);
    }
  },
};

// ============================================================
// Safe Clipboard
// ============================================================

/**
 * Safe clipboard write with fallback for HTTP contexts.
 * Tries navigator.clipboard.writeText, falls back to execCommand('copy').
 * Returns true if copy succeeded, false otherwise.
 */
export async function safeClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    try {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      const success = document.execCommand('copy');
      document.body.removeChild(textarea);
      if (!success) {
        throw new Error('execCommand copy failed');
      }
      return true;
    } catch {
      Sentry.captureMessage('clipboard unavailable', 'warning');
      return false;
    }
  }
}

// ============================================================
// Sentry Helpers
// ============================================================

/**
 * Captures a hook error with structured context.
 * Use in catch blocks inside React hooks.
 */
export function captureHookError(
  error: unknown,
  hookName: string,
  operation: string,
  extra?: Record<string, unknown>
): void {
  Sentry.captureException(error, {
    tags: {
      hook: hookName,
      operation,
    },
    extra,
  });
}

/**
 * Sets Sentry user context for error tracking.
 * Pass null to clear user on logout.
 */
export function setSentryUser(user: { id: string; email: string; role?: string } | null): void {
  if (user) {
    Sentry.setUser({ id: user.id, email: user.email });
    if (user.role) {
      Sentry.setTag('user_role', user.role);
    }
  } else {
    Sentry.setUser(null);
  }
}

// ============================================================
// Init
// ============================================================

/**
 * Initializes Sentry for the React frontend.
 * Must be called BEFORE React root creation (FG1).
 */
export function initSentry(): void {
  const dsn = import.meta.env.VITE_SENTRY_DSN as string | undefined;
  const isDevMode = import.meta.env.VITE_SENTRY_DEV_MODE === 'true';
  const isProd = import.meta.env.PROD;

  // FG2: Safety net for unhandled rejections BEFORE Sentry.init
  window.addEventListener('unhandledrejection', (event) => {
    if (!Sentry.getClient()) {
      console.error('[Sentry:pre-init] Unhandled rejection:', event.reason);
    }
  });

  // Guard: warn if DSN is missing in production
  if (!dsn && isProd) {
    console.warn('[Sentry] VITE_SENTRY_DSN not set in production. Sentry is disabled.');
    return;
  }

  // FG15: Dev mode console logger without DSN
  if (!dsn && isDevMode) {
    console.info('[Sentry] Dev mode enabled without DSN. Errors will be logged to console.');
    return;
  }

  // Skip init if no DSN in development
  if (!dsn) {
    return;
  }

  Sentry.init({
    dsn,
    tunnel: isProd ? '/api/v1/sentry-tunnel' : undefined,
    environment: (import.meta.env.VITE_SENTRY_ENVIRONMENT as string) || import.meta.env.MODE,
    release: import.meta.env.VITE_SENTRY_RELEASE as string | undefined,
    integrations: [
      Sentry.browserTracingIntegration(),
      Sentry.replayIntegration({
        maskAllText: true,
        blockAllMedia: true,
      }),
    ],
    tracesSampleRate: isProd ? 0.1 : 1.0,
    replaysSessionSampleRate: 0.1,
    replaysOnErrorSampleRate: 1.0,

    denyUrls: [/extensions\//i, /^chrome:\/\//i, /^moz-extension:\/\//i],

    ignoreErrors: [
      'ResizeObserver loop',
      'ResizeObserver loop completed with undelivered notifications',
      'Non-Error promise rejection captured',
      'AbortError',
      'The user aborted a request',
      'signal is aborted without reason',
      /Failed to fetch/i,
      /Load failed/i,
      /NetworkError when attempting/i,
      /ChunkLoadError/i,
    ],

    // T4.5: PII redaction in beforeSend
    beforeSend(event) {
      // Strip sensitive query params from request URL
      if (event.request?.url) {
        event.request.url = stripSensitiveParams(event.request.url);
      }

      // Scrub PII from exception messages
      if (event.exception?.values) {
        for (const exception of event.exception.values) {
          if (exception.value) {
            exception.value = scrubPII(stripSensitiveParams(exception.value));
          }
        }
      }

      // Scrub PII from breadcrumbs + redact form data
      if (event.breadcrumbs) {
        for (const breadcrumb of event.breadcrumbs) {
          if (breadcrumb.message) {
            breadcrumb.message = scrubPII(stripSensitiveParams(breadcrumb.message));
          }
          if (breadcrumb.data && breadcrumb.category === 'ui.input') {
            breadcrumb.data = undefined;
          }
        }
      }

      return event;
    },

    // FSQ18: Scrub SSE token URLs from breadcrumbs
    beforeBreadcrumb(breadcrumb) {
      if (breadcrumb.data?.url && typeof breadcrumb.data.url === 'string') {
        breadcrumb.data.url = stripSensitiveParams(breadcrumb.data.url);
      }
      if (breadcrumb.message) {
        breadcrumb.message = stripSensitiveParams(breadcrumb.message);
      }
      return breadcrumb;
    },
  });
}
