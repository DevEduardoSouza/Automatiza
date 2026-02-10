import { useAuth } from '@/contexts';
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

/**
 * Hook for receiving real-time session status updates via SSE.
 *
 * Uses refs for all callbacks to prevent EventSource reconnection cycling.
 * The EventSource connection is only re-established when the auth token changes,
 * NOT when callback references change.
 */
export function useSessionStatusSSE(
  options: UseSessionStatusSSEOptions = {}
): UseSessionStatusSSEReturn {
  const { autoConnect = true } = options;

  const { session } = useAuth();
  const [connectionState, setConnectionState] = useState<SSEConnectionState>('disconnected');
  const [lastStatus, setLastStatus] = useState<SessionStatusData | null>(null);

  // Store callbacks in refs to avoid reconnection cycling.
  // This is the standard React pattern for event handlers that shouldn't
  // cause effect re-execution when they change.
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
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const maxReconnectAttempts = 5;
  const baseReconnectDelay = 1000;

  /**
   * Update connection state and notify callback via ref
   */
  const updateConnectionState = useCallback((state: SSEConnectionState) => {
    setConnectionState(state);
    onConnectionChangeRef.current?.(state);
  }, []);

  /**
   * Calculate reconnect delay with exponential backoff + jitter
   */
  const getReconnectDelay = useCallback(() => {
    const exponentialDelay = baseReconnectDelay * 2 ** reconnectAttemptsRef.current;
    const jitter = Math.random() * 1000;
    return Math.min(exponentialDelay + jitter, 30000);
  }, []);

  /**
   * Disconnect from SSE
   */
  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    reconnectAttemptsRef.current = 0;
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

    // Close existing connection if any
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    updateConnectionState('connecting');

    const sseUrl = `${import.meta.env.VITE_API_URL || ''}/api/v1/sse/events?token=${encodeURIComponent(session.accessToken)}`;

    try {
      const eventSource = new EventSource(sseUrl);
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        reconnectAttemptsRef.current = 0;
        updateConnectionState('connected');
      };

      eventSource.onerror = (event) => {
        console.warn('[SSE] Connection error:', event);

        if (eventSource.readyState === EventSource.CLOSED) {
          updateConnectionState('error');

          if (reconnectAttemptsRef.current < maxReconnectAttempts) {
            reconnectAttemptsRef.current++;
            const delay = getReconnectDelay();
            console.warn(
              `[SSE] Reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current})`
            );

            reconnectTimeoutRef.current = setTimeout(() => {
              connect();
            }, delay);
          } else {
            console.warn('[SSE] Max reconnect attempts reached');
            onErrorRef.current?.(new Error('SSE max reconnect attempts reached'));
          }
        } else {
          updateConnectionState('connecting');
        }
      };

      // Handle 'connected' event (initial connection confirmation)
      eventSource.addEventListener('connected', (event) => {
        try {
          const data = JSON.parse(event.data) as SSEEvent;
          console.log('[SSE] Connected:', data);
        } catch (e) {
          console.warn('[SSE] Failed to parse connected event:', e);
        }
      });

      // Handle 'session.status' events — read callback from ref
      eventSource.addEventListener('session.status', (event) => {
        try {
          const sseEvent = JSON.parse(event.data) as SSEEvent;
          const statusData = sseEvent.data as unknown as SessionStatusData;

          console.log('[SSE] Session status:', statusData);

          setLastStatus(statusData);
          onStatusChangeRef.current?.(statusData);

          if (statusData.qrCode) {
            onQRCodeRef.current?.(statusData.qrCode);
          }
        } catch (e) {
          console.warn('[SSE] Failed to parse session.status event:', e);
        }
      });

      // Handle 'qr.code' events
      eventSource.addEventListener('qr.code', (event) => {
        try {
          const sseEvent = JSON.parse(event.data) as SSEEvent;
          const qrCode = sseEvent.data.qrCode as string;

          if (qrCode) {
            console.log('[SSE] QR code received');
            onQRCodeRef.current?.(qrCode);
          }
        } catch (e) {
          console.warn('[SSE] Failed to parse qr.code event:', e);
        }
      });

      // Handle 'heartbeat' events
      eventSource.addEventListener('heartbeat', () => {
        // Connection alive
      });

      // Handle 'group.sync' events
      eventSource.addEventListener('group.sync', (event) => {
        try {
          const sseEvent = JSON.parse(event.data) as SSEEvent;
          console.log('[SSE] Group sync:', sseEvent.data);
          window.dispatchEvent(
            new CustomEvent('sse:group-event', {
              detail: { type: 'group.sync', data: sseEvent.data },
            })
          );
        } catch (e) {
          console.warn('[SSE] Failed to parse group.sync event:', e);
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
            const sseEvent = JSON.parse(event.data) as SSEEvent;
            window.dispatchEvent(
              new CustomEvent('sse:group-event', {
                detail: { type: eventType, data: sseEvent.data },
              })
            );
          } catch (e) {
            console.warn(`[SSE] Failed to parse ${eventType} event:`, e);
          }
        });
      }

      // Handle job status events
      eventSource.addEventListener('job.status', (event) => {
        try {
          const sseEvent = JSON.parse(event.data) as SSEEvent;
          window.dispatchEvent(
            new CustomEvent('sse:job-event', {
              detail: { type: 'job.status', data: sseEvent.data },
            })
          );
        } catch (e) {
          console.warn('[SSE] Failed to parse job.status event:', e);
        }
      });

      // Handle history change events
      eventSource.addEventListener('history.change', (event) => {
        try {
          const sseEvent = JSON.parse(event.data) as SSEEvent;
          window.dispatchEvent(
            new CustomEvent('sse:history-event', {
              detail: { type: 'history.change', data: sseEvent.data },
            })
          );
        } catch (e) {
          console.warn('[SSE] Failed to parse history.change event:', e);
        }
      });
    } catch (error) {
      console.error('[SSE] Failed to create EventSource:', error);
      updateConnectionState('error');
      onErrorRef.current?.(error instanceof Error ? error : new Error(String(error)));
    }
  }, [session?.accessToken, updateConnectionState, getReconnectDelay]);

  // Auto-connect on mount if enabled.
  // IMPORTANT: `connect` no longer depends on callback refs, so this effect
  // only re-runs when the auth token changes (not on every render).
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
