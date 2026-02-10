import type { InstanceStatus } from '@/entities/whatsapp-instance';
import { hasRealInstanceName } from '@/entities/whatsapp-instance';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useSessionStatusSSE } from '@/hooks/use-session-status-sse';
import { useToast } from '@/hooks/use-toast';
import { useWhatsAppConnect } from '@/hooks/use-whatsapp-connect';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface UseWhatsAppReturn {
  instanceName: string;
  instanceId: string | null;
  connecting: boolean;
  connected: boolean;
  verifying: boolean;
  disconnecting: boolean;
  qrCode: string | null;
  qrCountdown: number;
  checkingStatus: boolean;
  importing: boolean;
  importProgress: number;
  importedGroups: string[];
  autoSync: boolean;
  lastSync: Date | null;
  cleaning: boolean;
  setAutoSync: (value: boolean) => void;
  handleConnect: () => Promise<void>;
  handleImportGroups: (isSilent?: boolean) => Promise<void>;
  handleCleanOldInstances: () => Promise<void>;
  checkStatus: () => Promise<void>;
  disconnectInstance: () => Promise<void>;
  refreshQrCode: () => Promise<void>;
}

/**
 * Hook for WhatsApp connection page logic.
 *
 * Detection strategy (dual-path):
 *   1. SSE (instant) — backend pushes session.status events via Redis Pub/Sub bridge
 *   2. Polling (fallback) — CheckStatus RPC every 3s while connecting, 30s while connected
 *
 * Both paths can independently detect connection/disconnection and update state.
 */
export function useWhatsApp(): UseWhatsAppReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { createInstance, deleteInstance, checkStatus: checkStatusConnect } = useWhatsAppConnect();

  const {
    latestInstance,
    upsertInstanceStatus,
    cleanupInvalidInstances,
    deleteDisconnectedInstances,
    deleteInstance: deleteInstanceFromDb,
    refetch: refetchInstances,
  } = useInstancesConnect();

  const {
    syncGroups: syncGroupsConnect,
    getSyncStatus,
    refetch: refetchGroups,
  } = useGroupsConnect();

  const [instanceName, setInstanceName] = useState('');
  const [instanceId, setInstanceId] = useState<string | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [qrCountdown, setQrCountdown] = useState(30);
  const [checkingStatus, setCheckingStatus] = useState(false);
  const countdownRef = useRef<NodeJS.Timeout | null>(null);
  const refreshQrCodeRef = useRef<() => Promise<void>>(() => Promise.resolve());
  const countdownRefreshRef = useRef<() => Promise<void>>(() => Promise.resolve());
  const [importing, setImporting] = useState(false);
  const [importProgress, setImportProgress] = useState(0);
  const [importedGroups, setImportedGroups] = useState<string[]>([]);
  const [autoSync, setAutoSync] = useState(() => {
    const saved = localStorage.getItem('whatsapp-auto-sync');
    return saved === 'true';
  });
  const [lastSync, setLastSync] = useState<Date | null>(null);
  const [cleaning, setCleaning] = useState(false);

  // Track whether a QR refresh is already in progress to avoid overlapping calls
  const refreshingQrRef = useRef(false);

  // Keep values in refs so async callbacks and effects always read the latest
  // values without triggering re-renders or effect re-runs (avoids stale closures)
  const instanceNameRef = useRef(instanceName);
  instanceNameRef.current = instanceName;
  const connectingRef = useRef(connecting);
  connectingRef.current = connecting;
  const checkStatusRef2 = useRef(checkStatusConnect);
  checkStatusRef2.current = checkStatusConnect;

  /**
   * SSE integration for real-time status updates.
   * Callbacks are stored in refs by useSessionStatusSSE, so no reconnection cycling.
   */
  useSessionStatusSSE({
    autoConnect: true,
    onStatusChange: (data) => {
      // Use ref to get the latest instanceName value (avoids stale closure)
      const currentInstanceName = instanceNameRef.current;

      // Only process if we have an instance and it matches the event session
      // If instanceName is empty, ignore all SSE events
      if (!currentInstanceName || data.session !== currentInstanceName) {
        return;
      }

      console.log('[WhatsApp] SSE status change:', data.status, 'for session:', data.session);

      switch (data.status) {
        case 'WORKING':
        case 'CONNECTED':
          // Session is now connected
          setConnected(true);
          setConnecting(false);
          setQrCode(null);
          toast({
            variant: 'success',
            title: t('pages.whatsapp.success.connected'),
            description: t('pages.whatsapp.success.connectedMessage'),
          });
          refetchInstances();
          break;

        case 'SCAN_QR_CODE':
          // Waiting for QR scan — refresh QR code from backend
          setConnected(false);
          setConnecting(true);
          // Trigger QR code fetch via CheckStatus RPC
          refreshQrCodeRef.current();
          break;

        case 'STARTING':
          // Session is starting
          setConnecting(true);
          break;

        case 'FAILED':
        case 'STOPPED':
          // Ignore transient STOPPED/FAILED during a manual QR refresh — reconnect
          // triggers logout which briefly puts the session in STOPPED state before restart.
          if (refreshingQrRef.current) break;

          // Session disconnected or failed
          setConnected(false);
          setConnecting(false);
          setQrCode(null);
          toast({
            variant: 'destructive',
            title: t('pages.whatsapp.errors.disconnected'),
            description: t('pages.whatsapp.errors.disconnectedMessage'),
          });
          refetchInstances();
          break;
      }
    },
    onQRCode: (qrCodeData) => {
      // New QR code received from SSE
      setQrCode(qrCodeData);
      setQrCountdown(30);
    },
    onError: (error) => {
      console.warn('[WhatsApp] SSE error:', error.message);
      // SSE errors are not critical - polling continues to work
    },
  });

  /**
   * Update instance status via ConnectRPC.
   */
  const updateInstanceStatus = useCallback(
    async (status: InstanceStatus) => {
      try {
        if (!hasRealInstanceName(instanceName)) return;

        const statusMap: Record<InstanceStatus, 'pending' | 'connected' | 'disconnected'> = {
          pending: 'pending',
          connected: 'connected',
          disconnected: 'disconnected',
        };

        const result = await upsertInstanceStatus(instanceName, statusMap[status]);
        if (result) {
          setInstanceId(result.id);
        }
      } catch (err) {
        console.warn('[WhatsApp] Failed to update instance status:', err);
      }
    },
    [instanceName, upsertInstanceStatus]
  );

  useEffect(() => {
    localStorage.setItem('whatsapp-auto-sync', String(autoSync));
  }, [autoSync]);

  /**
   * Cleanup invalid instances once on mount.
   */
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally run only on mount to avoid infinite loop
  useEffect(() => {
    cleanupInvalidInstances();
  }, []);

  // Track if initial load has been done to avoid loops
  const initialLoadDoneRef = useRef(false);
  // Track if disconnecting to prevent state changes
  const disconnectingRef = useRef(false);

  /**
   * Load connection state from the backend via ConnectRPC.
   * Only marks as connected after verifying real status from WAHA.
   */
  useEffect(() => {
    const loadConnectionState = async () => {
      // Skip if disconnecting — user initiated disconnect
      if (disconnectingRef.current) {
        return;
      }

      // Skip if a manual connect/reconnect is in progress — handleConnect manages state
      if (connectingRef.current) {
        setVerifying(false);
        return;
      }

      // No instance in database — show disconnected state
      if (!latestInstance) {
        setInstanceName('');
        setInstanceId(null);
        setConnected(false);
        setVerifying(false);
        initialLoadDoneRef.current = true;
        return;
      }

      // Invalid instance name — show disconnected state
      if (!hasRealInstanceName(latestInstance.instance_id)) {
        setInstanceName('');
        setInstanceId(null);
        setConnected(false);
        setVerifying(false);
        initialLoadDoneRef.current = true;
        return;
      }

      // Skip verification if already done and instance hasn't changed
      if (initialLoadDoneRef.current && instanceNameRef.current === latestInstance.instance_id) {
        return;
      }

      // Have a valid instance — verify with WAHA
      setVerifying(true);
      setInstanceName(latestInstance.instance_id);
      setInstanceId(latestInstance.id || null);
      // Don't trust database status - start as disconnected
      setConnected(false);

      try {
        const statusResp = await checkStatusRef2.current(latestInstance.instance_id);

        // Only mark as connected if WAHA confirms the session is open
        if (statusResp.state === 'open') {
          setConnected(true);
        } else {
          setConnected(false);
        }
      } catch (err) {
        console.warn('[WhatsApp] Failed to check initial status:', err);
        // On error, keep as disconnected - safer than assuming connected
        setConnected(false);
        setInstanceName('');
        setInstanceId(null);
      } finally {
        setVerifying(false);
        initialLoadDoneRef.current = true;
      }
    };

    loadConnectionState();
  }, [latestInstance]);

  /**
   * Polling: check status every 3 seconds while connecting.
   * Uses ref to always call the latest checkStatus without restarting the interval.
   */
  const checkStatusRef = useRef<() => Promise<void>>(() => Promise.resolve());

  useEffect(() => {
    let interval: NodeJS.Timeout;

    if (connecting) {
      interval = setInterval(() => {
        checkStatusRef.current();
      }, 3000);
    }

    return () => {
      if (interval) clearInterval(interval);
    };
  }, [connecting]);

  /**
   * Verification: check status every 30 seconds while connected.
   */
  const verifyRef = useRef<() => Promise<void>>(() => Promise.resolve());

  useEffect(() => {
    let interval: NodeJS.Timeout;

    if (connected && hasRealInstanceName(instanceName) && !connecting) {
      interval = setInterval(() => {
        verifyRef.current();
      }, 30000);
    }

    return () => {
      if (interval) clearInterval(interval);
    };
  }, [connected, instanceName, connecting]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: handleImportGroups and importing are intentionally not included to prevent infinite sync restart
  useEffect(() => {
    let interval: NodeJS.Timeout;
    let importCalled = false;

    if (autoSync && connected && hasRealInstanceName(instanceName) && instanceId) {
      if (!importing && !importCalled) {
        importCalled = true;
        handleImportGroups(true);
      }

      interval = setInterval(
        async () => {
          if (!importing) {
            await handleImportGroups(true);
          }
        },
        60 * 60 * 1000
      );
    }

    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [autoSync, connected, instanceName, instanceId]);

  /**
   * SSE listener for job.status events during groups sync.
   * Updates importedGroups list and progress in real-time.
   */
  useEffect(() => {
    const handleJobEvent = (event: Event) => {
      const customEvent = event as CustomEvent<{
        type: string;
        data: {
          jobType?: string;
          jobId?: string;
          group?: { id: string; name: string };
          progress?: number;
          current?: number;
          total?: number;
          status?: string;
          newGroups?: number;
          updatedGroups?: number;
        };
      }>;
      const { data } = customEvent.detail;

      // Only process groups_sync job events
      if (data.jobType !== 'groups_sync') return;

      console.log('[WhatsApp] SSE job event:', data);

      // Update progress
      if (data.progress !== undefined) {
        setImportProgress(data.progress);
      }

      // Add group to imported list
      if (data.group) {
        const groupName = data.group.name || data.group.id;
        setImportedGroups((prev) => {
          // Avoid duplicates
          if (prev.includes(groupName)) return prev;
          return [...prev, groupName];
        });
      }

      // Handle completion
      if (data.status === 'completed') {
        // Clear fallback timeout
        const fallbackTimeout = (window as unknown as { __importFallbackTimeout?: NodeJS.Timeout })
          .__importFallbackTimeout;
        if (fallbackTimeout) {
          clearTimeout(fallbackTimeout);
          (
            window as unknown as { __importFallbackTimeout?: NodeJS.Timeout }
          ).__importFallbackTimeout = undefined;
        }

        setImporting(false);
        setLastSync(new Date());
        refetchGroups();

        toast({
          variant: 'success',
          title: t('pages.whatsapp.success.imported'),
          description: t('pages.whatsapp.success.importedMessage', {
            newGroups: data.newGroups || 0,
            updatedGroups: data.updatedGroups || 0,
          }),
        });

        // Reset progress after a delay
        setTimeout(() => setImportProgress(0), 2000);
      }
    };

    window.addEventListener('sse:job-event', handleJobEvent);
    return () => {
      window.removeEventListener('sse:job-event', handleJobEvent);
    };
  }, [toast, t, refetchGroups]);

  /**
   * Fetch current QR code from backend (read-only, no session restart).
   * Used by SSE handler and countdown auto-refresh.
   */
  const fetchQrCode = useCallback(async () => {
    const currentInstanceName = instanceNameRef.current;

    if (!hasRealInstanceName(currentInstanceName)) return;
    if (refreshingQrRef.current) return;

    refreshingQrRef.current = true;
    try {
      const data = await checkStatusConnect(currentInstanceName);
      if (data.qrcodeBase64) {
        setQrCode(data.qrcodeBase64);
        setQrCountdown(30);
      }
      if (data.state === 'open') {
        setConnected(true);
        setConnecting(false);
        setQrCode(null);
        await updateInstanceStatus('connected');
      }
    } catch (err) {
      console.warn('[WhatsApp] QR fetch failed:', err);
    } finally {
      refreshingQrRef.current = false;
    }
  }, [checkStatusConnect, updateInstanceStatus]);

  // SSE and countdown use fetchQrCode (read-only)
  useEffect(() => {
    refreshQrCodeRef.current = fetchQrCode;
  }, [fetchQrCode]);

  /**
   * Refresh QR code (manual button click).
   * Deletes the current session and creates a new one — the only flow that
   * reliably returns a QR code directly in the response (same as handleConnect).
   * reconnectInstance was unreliable: SSE STOPPED events, polling race conditions,
   * and session state transitions made it impossible to guarantee QR delivery.
   */
  const refreshQrCode = useCallback(async () => {
    const currentInstanceName = instanceNameRef.current;

    if (!hasRealInstanceName(currentInstanceName)) return;
    if (refreshingQrRef.current) return;

    refreshingQrRef.current = true;
    setQrCode(null);

    try {
      // 1. Delete current session (ignore errors — may already be gone)
      try {
        await deleteInstance(currentInstanceName);
        await new Promise((resolve) => setTimeout(resolve, 1000));
      } catch {
        // ignore
      }

      // 2. Create fresh session — response includes QR code
      const data = await createInstance();

      if (data.success && data.qrcodeBase64) {
        if (data.instanceName) {
          setInstanceName(data.instanceName);
          instanceNameRef.current = data.instanceName;
        }
        setQrCode(data.qrcodeBase64);
        setQrCountdown(30);
      } else if (data.success) {
        // Created but no QR yet — update name and let polling handle QR
        if (data.instanceName) {
          setInstanceName(data.instanceName);
          instanceNameRef.current = data.instanceName;
        }
      }
    } catch (err) {
      console.warn('[WhatsApp] QR refresh failed:', err);
    } finally {
      setConnecting(true);
      refreshingQrRef.current = false;
    }
  }, [deleteInstance, createInstance]);

  // Keep countdownRefreshRef pointing to the full delete+create refresh
  useEffect(() => {
    countdownRefreshRef.current = refreshQrCode;
  }, [refreshQrCode]);

  /**
   * QR code countdown timer.
   * Starts when connecting becomes true and stops when connected or not connecting.
   * Only resets to 30s when countdown reaches 0 (which triggers QR refresh).
   */
  useEffect(() => {
    // Only run countdown when connecting and not connected
    if (connecting && !connected) {
      // Start countdown if not already running
      if (!countdownRef.current) {
        setQrCountdown(30);

        countdownRef.current = setInterval(() => {
          setQrCountdown((prev) => {
            if (prev <= 1) {
              // Countdown reached 0, generate a new QR via delete+create
              countdownRefreshRef.current();
              return 30;
            }
            return prev - 1;
          });
        }, 1000);
      }
    } else {
      // Clear interval when not connecting or when connected
      if (countdownRef.current) {
        clearInterval(countdownRef.current);
        countdownRef.current = null;
      }
    }

    return () => {
      if (countdownRef.current) {
        clearInterval(countdownRef.current);
        countdownRef.current = null;
      }
    };
  }, [connecting, connected]);

  /**
   * Verify connection status (called every 30s when connected).
   */
  const verifyConnectionStatus = useCallback(async () => {
    const currentInstanceName = instanceNameRef.current;
    if (!hasRealInstanceName(currentInstanceName)) return;

    try {
      const data = await checkStatusConnect(currentInstanceName);

      if (data.qrcodeBase64 && !qrCode) {
        setQrCode(data.qrcodeBase64);
      }

      const currentStatus = data.state;

      if (!data.success) {
        if (connected) {
          setConnected(false);
          await updateInstanceStatus('disconnected');
          toast({
            variant: 'destructive',
            title: t('pages.whatsapp.errors.connectionLost'),
            description: t('pages.whatsapp.errors.connectionLostMessage'),
            duration: 10000,
          });
        }
        return;
      }

      if (currentStatus === 'open') {
        if (!connected) {
          setConnected(true);
          await updateInstanceStatus('connected');
        }
      } else if (currentStatus === 'close') {
        if (connected) {
          setConnected(false);
          await updateInstanceStatus('disconnected');
          toast({
            variant: 'destructive',
            title: t('pages.whatsapp.errors.connectionLost'),
            description: t('pages.whatsapp.errors.disconnectedMessage'),
            duration: 10000,
          });
        }
      } else if (currentStatus === 'connecting') {
        // Session is in SCAN_QR_CODE or STARTING state — may have been disconnected
        if (connected) {
          setConnected(false);
          setConnecting(true);
          await updateInstanceStatus('pending');
        }
      }
    } catch (err) {
      console.warn('[WhatsApp] Verify connection failed:', err);
    }
  }, [qrCode, connected, updateInstanceStatus, toast, t, checkStatusConnect]);

  // Keep verify ref updated
  useEffect(() => {
    verifyRef.current = verifyConnectionStatus;
  }, [verifyConnectionStatus]);

  /**
   * Check status (called every 3s while connecting).
   * Handles ALL states: open, close, connecting, unspecified.
   */
  const checkStatus = useCallback(async () => {
    // Skip polling while a manual reconnect is in progress to avoid
    // hitting WAHA during session restart (triggers circuit breaker)
    if (refreshingQrRef.current) return;

    const currentInstanceName = instanceNameRef.current;
    if (!hasRealInstanceName(currentInstanceName)) return;

    setCheckingStatus(true);
    try {
      const data = await checkStatusConnect(currentInstanceName);

      // Update QR code if available (for 'connecting'/SCAN_QR_CODE state)
      // Skip if a manual refresh is in progress to avoid overwriting the loading state
      if (data.qrcodeBase64 && !refreshingQrRef.current) {
        setQrCode(data.qrcodeBase64);
      }

      const currentStatus = data.state;

      if (!data.success) {
        console.warn('[WhatsApp] CheckStatus returned success=false');
        setConnected(false);
        setConnecting(false);
        await updateInstanceStatus('disconnected');
        return;
      }

      if (currentStatus === 'open') {
        // Connected! Transition to connected state.
        console.log('[WhatsApp] Polling detected OPEN state — connected');
        setConnected(true);
        setConnecting(false);
        setQrCode(null);
        await updateInstanceStatus('connected');
        toast({
          variant: 'success',
          title: t('pages.whatsapp.success.connected'),
          description: t('pages.whatsapp.success.connectedMessage'),
        });
      } else if (currentStatus === 'close') {
        // Session closed — disconnected
        console.warn('[WhatsApp] Polling detected CLOSE state');
        setConnected(false);
        setConnecting(false);
        await updateInstanceStatus('disconnected');
        toast({
          variant: 'destructive',
          title: t('pages.whatsapp.errors.disconnected'),
          description: t('pages.whatsapp.errors.disconnectedMessage'),
        });
      } else if (currentStatus === 'connecting') {
        // SCAN_QR_CODE or STARTING — still waiting, keep polling.
        // QR code was already updated above if available.
        // No state change needed; just continue.
      }
      // 'unspecified' — do nothing, keep polling
    } catch (err) {
      // Log but don't show toast (polling runs every 3s, toast spam is bad UX)
      console.warn('[WhatsApp] CheckStatus poll error:', err);
    } finally {
      setCheckingStatus(false);
    }
  }, [updateInstanceStatus, toast, t, checkStatusConnect]);

  // Keep checkStatus ref updated
  useEffect(() => {
    checkStatusRef.current = checkStatus;
  }, [checkStatus]);

  const handleConnect = useCallback(async () => {
    setConnecting(true);
    setQrCode(null);

    try {
      if (hasRealInstanceName(instanceName)) {
        try {
          await deleteInstance(instanceName);
          await new Promise((resolve) => setTimeout(resolve, 2000));
        } catch {
          // ignore delete errors
        }
      }

      const data = await createInstance();

      if (data.success) {
        setInstanceName(data.instanceName || '');

        if (data.qrcodeBase64) {
          setQrCode(data.qrcodeBase64);
          toast({
            variant: 'success',
            title: t('pages.whatsapp.success.qrGenerated'),
            description: t('pages.whatsapp.success.qrGeneratedMessage'),
          });
        }
      } else {
        throw new Error(data.error || t('pages.whatsapp.errors.connect'));
      }
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : t('pages.whatsapp.errors.connectMessage');
      toast({
        variant: 'destructive',
        title: t('pages.whatsapp.errors.connect'),
        description: errorMessage,
        duration: 7000,
      });
      setConnecting(false);
      setConnected(false);
    }
  }, [instanceName, toast, t, createInstance, deleteInstance]);

  /**
   * Import groups using ConnectRPC syncGroups.
   * Uses polling with getSyncStatus for reliable progress updates.
   */
  const handleImportGroups = useCallback(
    async (isSilent = false) => {
      if (!instanceName || !instanceId) return;

      if (importing) {
        return;
      }

      setImporting(true);
      setImportProgress(0);
      if (!isSilent) {
        setImportedGroups([]);
      }

      const completeImport = async (message?: string, groupsSynced?: number) => {
        await refetchGroups();
        setLastSync(new Date());
        setImporting(false);
        setImportProgress(100);

        if (!isSilent) {
          toast({
            variant: 'success',
            title: t('pages.whatsapp.success.imported'),
            description:
              groupsSynced !== undefined
                ? t('pages.whatsapp.success.importedMessage', {
                    newGroups: groupsSynced,
                    updatedGroups: 0,
                  })
                : message || t('pages.whatsapp.success.syncComplete'),
          });
        }

        setTimeout(() => setImportProgress(0), 2000);
      };

      const failImport = (errorMessage?: string) => {
        setImporting(false);
        setImportProgress(0);

        if (!isSilent) {
          toast({
            variant: 'destructive',
            title: t('pages.whatsapp.errors.import'),
            description: errorMessage || t('pages.whatsapp.errors.importMessage'),
          });
        }
      };

      try {
        console.log('[WhatsApp] Starting sync for instance:', instanceName);
        const result = await syncGroupsConnect(instanceName);
        console.log('[WhatsApp] Sync result:', result);

        if (!result.success) {
          throw new Error(result.message || t('pages.whatsapp.errors.import'));
        }

        // If we have a job_id, poll for status updates
        if (result.job_id) {
          console.log('[WhatsApp] Job ID received, starting polling:', result.job_id);
          let pollCount = 0;
          let errorCount = 0;
          const maxPolls = 30; // Max 30 polls (30 seconds)
          const maxErrors = 5;

          const pollInterval = setInterval(async () => {
            pollCount++;

            try {
              const status = await getSyncStatus(result.job_id as string);
              console.log('[WhatsApp] Poll status:', status);
              errorCount = 0; // Reset error count on success

              // Update progress
              if (status.progress !== undefined && status.progress > 0) {
                setImportProgress(status.progress);
              }

              // Check for completion
              if (status.status === 'completed' || status.status === 'success') {
                console.log('[WhatsApp] Sync completed via polling');
                clearInterval(pollInterval);
                await completeImport(undefined, status.groups_synced);
                return;
              }

              if (status.status === 'failed' || status.status === 'error') {
                console.log('[WhatsApp] Sync failed via polling:', status.error);
                clearInterval(pollInterval);
                failImport(status.error);
                return;
              }

              // Timeout after maxPolls
              if (pollCount >= maxPolls) {
                console.log('[WhatsApp] Max polls reached, completing');
                clearInterval(pollInterval);
                await completeImport(result.message);
              }
            } catch (pollError) {
              errorCount++;
              console.warn('[WhatsApp] Poll error:', pollError, `(${errorCount}/${maxErrors})`);

              if (errorCount >= maxErrors) {
                console.log('[WhatsApp] Max poll errors, completing');
                clearInterval(pollInterval);
                await completeImport(result.message);
              }
            }
          }, 1000);
        } else {
          // No job_id - sync was immediate or synchronous, complete now
          console.log('[WhatsApp] No job_id, sync was synchronous');
          await completeImport(result.message);
        }
      } catch (error: unknown) {
        console.error('[WhatsApp] Sync error:', error);
        const errorMessage =
          error instanceof Error ? error.message : t('pages.whatsapp.errors.importMessage');
        failImport(errorMessage);
      }
    },
    [instanceName, instanceId, importing, toast, t, syncGroupsConnect, getSyncStatus, refetchGroups]
  );

  /**
   * Clean old disconnected/pending instances via ConnectRPC.
   */
  const handleCleanOldInstances = useCallback(async () => {
    setCleaning(true);
    try {
      await deleteDisconnectedInstances();
      await refetchInstances();
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : t('common.error');
      toast({
        variant: 'destructive',
        title: t('pages.whatsapp.errors.clean'),
        description: errorMessage,
      });
    } finally {
      setCleaning(false);
    }
  }, [deleteDisconnectedInstances, refetchInstances, toast, t]);

  /**
   * Disconnect the current instance and reset state.
   * Deletes both the WAHA session and the database record.
   */
  const disconnectInstance = useCallback(async () => {
    // Prevent multiple disconnects and block state changes
    if (disconnectingRef.current) return;

    disconnectingRef.current = true;
    setDisconnecting(true);

    const currentInstanceName = instanceNameRef.current;
    const currentInstanceId = instanceId;

    try {
      // 1. Delete WAHA session so it's no longer "open"
      if (hasRealInstanceName(currentInstanceName)) {
        try {
          await deleteInstance(currentInstanceName);
        } catch {
          // Session may already be gone — continue with cleanup
        }
      }

      // 2. Delete instance from database
      if (currentInstanceId) {
        try {
          await deleteInstanceFromDb(currentInstanceId);
        } catch {
          // Instance may already be deleted — continue with cleanup
        }
      }

      // 3. Refetch instances to update UI
      await refetchInstances();

      // 4. Clear local state
      setConnected(false);
      setConnecting(false);
      setVerifying(false);
      setInstanceName('');
      setInstanceId(null);
      setQrCode(null);
      initialLoadDoneRef.current = false;
    } finally {
      setDisconnecting(false);
      disconnectingRef.current = false;
    }
  }, [instanceId, deleteInstance, deleteInstanceFromDb, refetchInstances]);

  return {
    instanceName,
    instanceId,
    connecting,
    connected,
    verifying,
    disconnecting,
    qrCode,
    qrCountdown,
    checkingStatus,
    importing,
    importProgress,
    importedGroups,
    autoSync,
    lastSync,
    cleaning,
    setAutoSync,
    handleConnect,
    handleImportGroups,
    handleCleanOldInstances,
    checkStatus,
    disconnectInstance,
    refreshQrCode,
  };
}
