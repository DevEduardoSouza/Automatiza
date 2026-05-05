import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { ConnectionStatus, UseConnectionManagerReturn } from './types';

/**
 * Hook for managing WhatsApp connection status and reconnection.
 * Fully migrated to ConnectRPC hooks.
 *
 * Multi-instance aware: connection status is "connected" when ANY of the
 * user's instances reports connected. Reconnect targets the user's default
 * instance (or any connected one as fallback) instead of the singleton
 * "latestInstance", so a 2nd-chip-in-pending state does not flip /groups
 * into "Desconectado".
 */
export function useConnectionManager(loadGroups: () => Promise<void>): UseConnectionManagerReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    instances,
    upsertInstanceStatus,
    refetch: refetchInstances,
    checkInstanceStatus,
    reconnectInstance,
  } = useInstancesConnect();

  // Pick the chip that best represents "the user's WhatsApp": prefer the
  // default chip when connected, otherwise any connected chip, falling back
  // to the default chip even if disconnected (so reconnect has a target).
  const primaryInstance = useMemo(() => {
    if (instances.length === 0) return null;
    const defaultConnected = instances.find((i) => i.is_default && i.status === 'connected');
    if (defaultConnected) return defaultConnected;
    const anyConnected = instances.find((i) => i.status === 'connected');
    if (anyConnected) return anyConnected;
    const defaultInstance = instances.find((i) => i.is_default);
    return defaultInstance ?? instances[0];
  }, [instances]);

  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('checking');
  const [instanceName, setInstanceName] = useState<string>('');
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectQrCode, setReconnectQrCode] = useState<string | null>(null);
  const [showReconnectDialog, setShowReconnectDialog] = useState(false);

  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const checkConnectionStatus = useCallback(async () => {
    try {
      if (!primaryInstance) {
        setConnectionStatus('disconnected');
        setInstanceName('');
        return;
      }

      setInstanceName(primaryInstance.instance_id);
      setConnectionStatus(primaryInstance.status === 'connected' ? 'connected' : 'disconnected');
    } catch (error: unknown) {
      captureHookError(error, 'useConnectionManager', 'checkConnectionStatus');
      setConnectionStatus('disconnected');
    }
  }, [primaryInstance]);

  const handleReconnect = useCallback(async () => {
    if (!instanceName) {
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('pages.groups.errors.instanceNotFound'),
      });
      return;
    }

    setReconnecting(true);
    setReconnectQrCode(null);

    try {
      // Use InstancesService gRPC for reconnection
      const data = await reconnectInstance(instanceName);

      if (data.success && data.qrcodeBase64) {
        const qrCodeBase64 = data.qrcodeBase64;

        if (qrCodeBase64) {
          setReconnectQrCode(qrCodeBase64);
          setShowReconnectDialog(true);
          toast({
            title: t('pages.groups.whatsapp.qrGenerated'),
            description: t('pages.groups.whatsapp.scanQRCode'),
          });

          // Clear any existing intervals before starting new ones
          if (pollIntervalRef.current) {
            clearInterval(pollIntervalRef.current);
            pollIntervalRef.current = null;
          }
          if (pollTimeoutRef.current) {
            clearTimeout(pollTimeoutRef.current);
            pollTimeoutRef.current = null;
          }

          pollIntervalRef.current = setInterval(async () => {
            try {
              // Use InstancesService gRPC for status check
              const statusData = await checkInstanceStatus(instanceName);

              if (statusData?.success && statusData?.state === 'open') {
                if (pollIntervalRef.current) {
                  clearInterval(pollIntervalRef.current);
                  pollIntervalRef.current = null;
                }
                if (pollTimeoutRef.current) {
                  clearTimeout(pollTimeoutRef.current);
                  pollTimeoutRef.current = null;
                }
                setConnectionStatus('connected');
                setShowReconnectDialog(false);
                setReconnectQrCode(null);
                setReconnecting(false);

                // Update instance status via ConnectRPC
                await upsertInstanceStatus(instanceName, 'connected');

                // Refetch instances to update cache
                await refetchInstances();

                toast({
                  title: t('pages.groups.success.reconnected'),
                  description: t('pages.groups.success.reconnectedMessage'),
                });

                setTimeout(() => {
                  loadGroups();
                }, 1000);
              }
            } catch (error: unknown) {
              captureHookError(error, 'useConnectionManager', 'pollReconnectStatus');
            }
          }, 3000);

          pollTimeoutRef.current = setTimeout(() => {
            if (pollIntervalRef.current) {
              clearInterval(pollIntervalRef.current);
              pollIntervalRef.current = null;
            }
          }, 120000);
        } else {
          throw new Error(t('pages.groups.errors.qrNotFound'));
        }
      } else {
        throw new Error(data.error || t('pages.groups.errors.qrGeneration'));
      }
    } catch (error: unknown) {
      captureHookError(error, 'useConnectionManager', 'handleReconnect');
      const errorMessage =
        error instanceof Error ? error.message : t('pages.groups.errors.reconnect');
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.reconnect'),
        description: errorMessage,
      });
    } finally {
      setReconnecting(false);
    }
  }, [
    instanceName,
    toast,
    t,
    loadGroups,
    reconnectInstance,
    checkInstanceStatus,
    upsertInstanceStatus,
    refetchInstances,
  ]);

  // Initial status check - update when latestInstance changes
  useEffect(() => {
    checkConnectionStatus();
  }, [checkConnectionStatus]);

  // Cleanup intervals on unmount
  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
      if (pollTimeoutRef.current) {
        clearTimeout(pollTimeoutRef.current);
      }
    };
  }, []);

  return {
    connectionStatus,
    instanceName,
    reconnecting,
    reconnectQrCode,
    showReconnectDialog,
    setShowReconnectDialog,
    handleReconnect,
    checkConnectionStatus,
  };
}
