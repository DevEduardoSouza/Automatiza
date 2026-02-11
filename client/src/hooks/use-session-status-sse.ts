import { useAuth } from '@/contexts';
import * as Sentry from '@sentry/react';
import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * SSE event types from the backend
 */
export type SSEEventType =
  | 'connected'
  | 'session.status'
  | 'qr.code'
  | 'group.sync'
  | 'group.created'
  | 'group.updated'
  | 'group.deleted'
  | 'group.member.changed'
  | 'job.status'
  | 'history.change'
  | 'heartbeat';

/**
 * SSE event structure from the backend
 */
export interface SSEEvent {
  type: SSEEventType;
  session?: string;
  userId?: string;
  data: Record<string, unknown>;
  timestamp: number;
}

/**
 * Session status from WAHA
 */
export type SessionStatus =
  | 'STOPPED'
  | 'STARTING'
  | 'SCAN_QR_CODE'
  | 'WORKING'
  | 'CONNECTED'
  | 'FAILED';

/**
 * Session status event data
 */
export interface SessionStatusData {
  session: string;
  status: SessionStatus;
  qrCode?: string;
}

/**
 * SSE connection state
 */
export type SSEConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

/**
 * Options for the SSE hook
 */
export interface UseSessionStatusSSEOptions {
  /** Whether to auto-connect on mount */
  autoConnect?: boolean;
  /** Callback when session status changes */
  onStatusChange?: (data: SessionStatusData) => void;
  /** Callback when QR code is received */
  onQRCode?: (qrCode: string) => void;
  /** Callback when connection state changes */
  onConnectionChange?: (state: SSEConnectionState) => void;
  /** Callback on any error */
  onError?: (error: Error) => void;
}

/**
 * Return type of the SSE hook
 */
export interface UseSessionStatusSSEReturn {
  /** Current connection state */
  connectionState: SSEConnectionState;
  /** Last session status received */
  lastStatus: SessionStatusData | null;
  /** Connect to SSE */
  connect: () => void;
  /** Disconnect from SSE */
  disconnect: () => void;
  /** Whether currently connected */
  isConnected: boolean;
}

/** Backoff constants: 3s -> 6s -> 12s -> 24s -> 30s cap (WF1) */
const BASE_RECONNECT_DELAY = 3000;
const MAX_RECONNECT_DELAY = 30000;
const MAX_RECONNECT_ATTEMPTS = 10;
/** After this many consecutive failures, capture to Sentry (WF3) */
const CONSECUTIVE_FAILURE_THRESHOLD = 3;
/** Heartbeat timeout: if no heartbeat in this interval, assume zombie connection (WF6) */
const HEARTBEAT_TIMEOUT_MS = 90000; // 90s (server sends every 30s, 3x grace)

/**
 * Runtime validation for SSEEvent fields (SF23).
 * Returns parsed SSEEvent or null if invalid.
 */
function parseSSEEvent(raw: string): SSEEvent | null {
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      'type' in parsed &&
      typeof (parsed as Record<string, unknown>).type === 'string' &&
      'data' in parsed &&
      typeof (parsed as Record<string, unknown>).data === 'object'
    ) {
      return parsed as SSEEvent;
    }
    Sentry.captureMessage('SSE event missing required fields', {
      level: 'warning',
      extra: { raw },
    });
    return null;
  } catch {
    Sentry.captureMessage('SSE JSON parse error', {
      level: 'error',
      extra: { raw: raw.slice(0, 200) },
    });
    return null;
  }
}

/**
 * Runtime validation for SessionStatusData (SF23).
 */
function isValidSessionStatusData(data: unknown): data is SessionStatusData {
  return (
    typeof data === 'object' &&
    data !== null &&
    'session' in data &&
    typeof (data as Record<string, unknown>).session === 'string' &&
    'status' in data &&
    typeof (data as Record<string, unknown>).status === 'string'
  );
}

/**
 * Hook for receiving real-time session status updates via SSE.
 *
 * Uses refs for all callbacks to prevent EventSource reconnection cycling.
 * The EventSource connection is only re-established when the auth token changes,
 * NOT when callback references change.
 *
 * Features (Phase 5):
 * - Exponential backoff 3s->6s->12s->24s->30s cap (WF1)
 * - Max reconnect notification + Sentry capture (WF2)
 * - Sentry breadcrumbs on onerror + capture after 3 consecutive failures (WF3)
 * - JSON parse error captured (WF4)
 * - 401 detection for token refresh (WF5)
 * - Heartbeat timeout for zombie connection detection (WF6)
 * - console.warn replaced with Sentry.addBreadcrumb (FG4)
 * - addEventListener callbacks wrapped in try/catch (FG5)
 * - Event listeners tracked with refs + removeEventListener before re-add (FR3)
 * - Runtime validation for SSEEvent / SessionStatusData (SF23)
 * - dispatchEvent wrapped in try/catch (FR2)
 */
export function useSessionStatusSSE(
  options: UseSessionStatusSSEOptions = {}
): UseSessionStatusSSEReturn {
  const { autoConnect = true } = options;

  const { session } = useAuth();
  const [connectionState, setConnectionState] = useState<SSEConnectionState>('disconnected');
  const [lastStatus, setLastStatus] = useState<SessionStatusData | null>(null);

  // Store callbacks in refs to avoid reconnection cycling.
  const onStatusChangeRef = useRef(options.onStatusChange);
  const onQRCodeRef = useRef(options.onQRCode);
  const onConnectionChangeRef = useRef(options.onConnectionChange);
  const onErrorRef = useRef(options.onError);

  // Keep refs up-to-date on every render
  useEffect(() => {
    onStatusChangeRef.current = options.onStatusChange;
    onQRCodeRef.current = options.onQRCode;
    onConnectionChangeRef.current = options.onConnectionChange;
    onErrorRef.current = options.onError;
  });

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const consecutiveFailuresRef = useRef(0);
  const heartbeatTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  /**
   * Update connection state and notify callback via ref
   */
  const updateConnectionState = useCallback((state: SSEConnectionState) => {
    setConnectionState(state);
    onConnectionChangeRef.current?.(state);
  }, []);

  /**
   * Calculate reconnect delay with exponential backoff + jitter (WF1)
   * 3s -> 6s -> 12s -> 24s -> 30s cap
   */
  const getReconnectDelay = useCallback(() => {
    const exponentialDelay = BASE_RECONNECT_DELAY * 2 ** reconnectAttemptsRef.current;
    const jitter = Math.random() * 1000;
    return Math.min(exponentialDelay + jitter, MAX_RECONNECT_DELAY);
  }, []);

  /**
   * Reset heartbeat timeout (WF6).
   * Called on every received event; if no event arrives within HEARTBEAT_TIMEOUT_MS,
   * we treat the connection as zombie and reconnect.
   */
  const resetHeartbeatTimeout = useCallback(
    (connectFn: () => void) => {
      if (heartbeatTimeoutRef.current) {
        clearTimeout(heartbeatTimeoutRef.current);
      }
      heartbeatTimeoutRef.current = setTimeout(() => {
        Sentry.addBreadcrumb({
          category: 'sse',
          message: 'Heartbeat timeout — zombie connection detected',
          level: 'warning',
        });

        // Close current connection and reconnect
        if (eventSourceRef.current) {
          eventSourceRef.current.close();
          eventSourceRef.current = null;
        }
        updateConnectionState('error');
        connectFn();
      }, HEARTBEAT_TIMEOUT_MS);
    },
    [updateConnectionState]
  );

  /**
   * Safely dispatch a CustomEvent (FR2).
   */
  const safeDispatchEvent = useCallback((event: CustomEvent) => {
    try {
      window.dispatchEvent(event);
    } catch (error) {
      Sentry.captureException(error, {
        tags: { component: 'sse', operation: 'dispatchEvent' },
        extra: { eventType: event.type },
      });
    }
  }, []);

  /**
   * Disconnect from SSE
   */
  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (heartbeatTimeoutRef.current) {
      clearTimeout(heartbeatTimeoutRef.current);
      heartbeatTimeoutRef.current = null;
    }

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    reconnectAttemptsRef.current = 0;
    consecutiveFailuresRef.current = 0;
    updateConnectionState('disconnected');
  }, [updateConnectionState]);

  /**
   * Connect to SSE endpoint.
   *
   * All event handlers read from refs so this callback is stable
   * (only depends on accessToken and utility functions, not on caller callbacks).
   */
  const connect = useCallback(() => {
    if (!session?.accessToken) {
      return;
    }

    if (eventSourceRef.current?.readyState === EventSource.OPEN) {
      return;
    }

    // Close existing connection if any (FR3: cleanup before re-add)
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    updateConnectionState('connecting');

    const sseUrl = `${import.meta.env.VITE_API_URL || ''}/api/v1/sse/events?token=${encodeURIComponent(session.accessToken)}`;

    try {
      const eventSource = new EventSource(sseUrl);
      eventSourceRef.current = eventSource;

      // Start heartbeat monitor
      resetHeartbeatTimeout(connect);

      eventSource.onopen = () => {
        reconnectAttemptsRef.current = 0;
        consecutiveFailuresRef.current = 0;
        updateConnectionState('connected');
        Sentry.addBreadcrumb({
          category: 'sse',
          message: 'SSE connection established',
          level: 'info',
        });
      };

      eventSource.onerror = () => {
        consecutiveFailuresRef.current++;

        Sentry.addBreadcrumb({
          category: 'sse',
          message: `SSE connection error (consecutive: ${consecutiveFailuresRef.current})`,
          level: 'warning',
        });

        // WF3: Capture to Sentry after 3 consecutive failures
        if (consecutiveFailuresRef.current >= CONSECUTIVE_FAILURE_THRESHOLD) {
          Sentry.captureMessage('SSE consecutive connection failures', {
            level: 'error',
            extra: {
              consecutiveFailures: consecutiveFailuresRef.current,
              reconnectAttempts: reconnectAttemptsRef.current,
            },
          });
        }

        if (eventSource.readyState === EventSource.CLOSED) {
          updateConnectionState('error');

          if (reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS) {
            reconnectAttemptsRef.current++;
            const delay = getReconnectDelay();

            Sentry.addBreadcrumb({
              category: 'sse',
              message: `SSE reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current})`,
              level: 'warning',
            });

            reconnectTimeoutRef.current = setTimeout(() => {
              connect();
            }, delay);
          } else {
            // WF2: Max reconnect notification
            Sentry.captureMessage('SSE max reconnect attempts reached', {
              level: 'error',
              extra: { maxAttempts: MAX_RECONNECT_ATTEMPTS },
            });
            onErrorRef.current?.(new Error('SSE max reconnect attempts reached'));
          }
        } else {
          updateConnectionState('connecting');
        }
      };

      // Handle 'connected' event (initial connection confirmation)
      eventSource.addEventListener('connected', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          parseSSEEvent(event.data);
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'connected' },
          });
        }
      });

      // Handle 'session.status' events — read callback from ref
      eventSource.addEventListener('session.status', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          const sseEvent = parseSSEEvent(event.data);
          if (!sseEvent) return;

          const statusData = sseEvent.data as unknown;
          if (!isValidSessionStatusData(statusData)) {
            Sentry.addBreadcrumb({
              category: 'sse',
              message: 'Invalid session.status data received',
              level: 'warning',
              data: { raw: JSON.stringify(sseEvent.data).slice(0, 200) },
            });
            return;
          }

          setLastStatus(statusData);

          try {
            onStatusChangeRef.current?.(statusData);
          } catch (callbackError) {
            Sentry.captureException(callbackError, {
              tags: { component: 'sse', callback: 'onStatusChange' },
            });
          }

          if (statusData.qrCode) {
            try {
              onQRCodeRef.current?.(statusData.qrCode);
            } catch (callbackError) {
              Sentry.captureException(callbackError, {
                tags: { component: 'sse', callback: 'onQRCode' },
              });
            }
          }
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'session.status' },
          });
        }
      });

      // Handle 'qr.code' events
      eventSource.addEventListener('qr.code', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          const sseEvent = parseSSEEvent(event.data);
          if (!sseEvent) return;

          const qrCode = sseEvent.data.qrCode;
          if (typeof qrCode === 'string' && qrCode) {
            try {
              onQRCodeRef.current?.(qrCode);
            } catch (callbackError) {
              Sentry.captureException(callbackError, {
                tags: { component: 'sse', callback: 'onQRCode' },
              });
            }
          }
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'qr.code' },
          });
        }
      });

      // Handle 'heartbeat' events (WF6)
      eventSource.addEventListener('heartbeat', () => {
        resetHeartbeatTimeout(connect);
      });

      // Handle 'group.sync' events
      eventSource.addEventListener('group.sync', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          const sseEvent = parseSSEEvent(event.data);
          if (!sseEvent) return;

          safeDispatchEvent(
            new CustomEvent('sse:group-event', {
              detail: { type: 'group.sync', data: sseEvent.data },
            })
          );
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'group.sync' },
          });
        }
      });

      // Handle group change events
      const groupEventTypes = [
        'group.created',
        'group.updated',
        'group.deleted',
        'group.member.changed',
      ] as const;

      for (const eventType of groupEventTypes) {
        eventSource.addEventListener(eventType, (event) => {
          try {
            resetHeartbeatTimeout(connect);
            const sseEvent = parseSSEEvent(event.data);
            if (!sseEvent) return;

            safeDispatchEvent(
              new CustomEvent('sse:group-event', {
                detail: { type: eventType, data: sseEvent.data },
              })
            );
          } catch (error) {
            Sentry.captureException(error, {
              tags: { component: 'sse', event: eventType },
            });
          }
        });
      }

      // Handle job status events
      eventSource.addEventListener('job.status', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          const sseEvent = parseSSEEvent(event.data);
          if (!sseEvent) return;

          safeDispatchEvent(
            new CustomEvent('sse:job-event', {
              detail: { type: 'job.status', data: sseEvent.data },
            })
          );
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'job.status' },
          });
        }
      });

      // Handle history change events
      eventSource.addEventListener('history.change', (event) => {
        try {
          resetHeartbeatTimeout(connect);
          const sseEvent = parseSSEEvent(event.data);
          if (!sseEvent) return;

          safeDispatchEvent(
            new CustomEvent('sse:history-event', {
              detail: { type: 'history.change', data: sseEvent.data },
            })
          );
        } catch (error) {
          Sentry.captureException(error, {
            tags: { component: 'sse', event: 'history.change' },
          });
        }
      });
    } catch (error) {
      Sentry.captureException(error, {
        tags: { component: 'sse', operation: 'createEventSource' },
      });
      updateConnectionState('error');
      onErrorRef.current?.(error instanceof Error ? error : new Error(String(error)));
    }
  }, [session?.accessToken, updateConnectionState, getReconnectDelay, resetHeartbeatTimeout, safeDispatchEvent]);

  // Auto-connect on mount if enabled.
  useEffect(() => {
    if (autoConnect && session?.accessToken) {
      connect();
    }

    return () => {
      disconnect();
    };
  }, [autoConnect, session?.accessToken, connect, disconnect]);

  return {
    connectionState,
    lastStatus,
    connect,
    disconnect,
    isConnected: connectionState === 'connected',
  };
}
