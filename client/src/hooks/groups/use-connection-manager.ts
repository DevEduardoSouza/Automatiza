import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { ConnectionStatus, UseConnectionManagerReturn } from './types';

/**
 * Hook for managing WhatsApp connection status and reconnection.
 * Fully migrated to ConnectRPC hooks.
 */
export function useConnectionManager(loadGroups: () => Promise<void>): UseConnectionManagerReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    latestInstance,
    upsertInstanceStatus,
    refetch: refetchInstances,
    checkInstanceStatus,
    reconnectInstance,
  } = useInstancesConnect();

  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('checking');
  const [instanceName, setInstanceName] = useState<string>('');
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectQrCode, setReconnectQrCode] = useState<string | null>(null);
  const [showReconnectDialog, setShowReconnectDialog] = useState(false);

  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const checkConnectionStatus = useCallback(async () => {
    try {
      // Use ConnectRPC hook data for instance status
      if (!latestInstance) {
        setConnectionStatus('disconnected');
        setInstanceName('');
        return;
      }

      if (latestInstance.status === 'connected') {
        setInstanceName(latestInstance.instance_id);
        setConnectionStatus('connected');
      } else {
        setConnectionStatus('disconnected');
        setInstanceName(latestInstance.instance_id);
      }
    } catch {
      setConnectionStatus('disconnected');
    }
  }, [latestInstance]);

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
            } catch {
              // Ignore polling errors
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
