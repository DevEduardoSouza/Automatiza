import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type { Group } from '@/types/group';
import { type ChangeEvent, useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsAudioReturn } from './types';

interface UseGroupsAudioParams {
  selectedGroups: string[];
  groups: Group[];
  clearSelection: () => void;
}

/**
 * Hook for managing audio upload and sending functionality.
 * Uses ConnectRPC (GroupsService).
 */
export function useGroupsAudio({
  selectedGroups,
  groups,
  clearSelection,
}: UseGroupsAudioParams): UseGroupsAudioReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { sendGroupAudio } = useGroupsConnect();
  const { latestInstance } = useInstancesConnect();

  const [audioFile, setAudioFile] = useState<File | null>(null);
  const [audioPtt, setAudioPtt] = useState(true);
  const [audioCaption, setAudioCaption] = useState('');
  const [sendingAudio, setSendingAudio] = useState(false);
  const [sendAudioDialogOpen, setSendAudioDialogOpen] = useState(false);

  const handleAudioUpload = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0] || null;
      if (!file) return;

      // Validation: max 4MB
      const maxBytes = 4 * 1024 * 1024;
      if (file.size > maxBytes) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.audioTooLarge'),
          description: t('pages.groups.errors.audioTooLargeMessage'),
        });
        e.target.value = '';
        return;
      }

      setAudioFile(file);
    },
    [toast, t]
  );

  const removeAudioFile = useCallback(() => {
    setAudioFile(null);
  }, []);

  const handleSendAudio = useCallback(async () => {
    if (selectedGroups.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.noGroupSelected'),
        description: t('pages.groups.errors.noGroupSelectedMessage'),
      });
      return;
    }
    if (selectedGroups.length > 10) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.groupLimit'),
        description: t('pages.groups.errors.audioGroupLimit'),
      });
      return;
    }
    if (!audioFile) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.noAudioSelected'),
        description: t('pages.groups.errors.selectAudioFirst'),
      });
      return;
    }

    // PTT requires OGG format
    if (audioPtt) {
      const nameLower = audioFile.name.toLowerCase();
      const isOgg = nameLower.endsWith('.ogg') || audioFile.type === 'audio/ogg';
      if (!isOgg) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.invalidPttFormat'),
          description: t('pages.groups.errors.pttFormatMessage'),
        });
        return;
      }
    }

    setSendingAudio(true);

    try {
      // Get instance from ConnectRPC hook
      if (!latestInstance || latestInstance.status !== 'connected') {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.noInstance'),
          description: t('pages.groups.errors.connectWhatsAppFirst'),
        });
        return;
      }

      const currentInstance = latestInstance.instance_id;

      // Get selected groups data
      const selectedGroupsData = groups
        .filter((g) => selectedGroups.includes(g.id))
        .map((g) => ({
          wa_group_id: g.wa_group_id,
          name: g.name,
        }));

      if (selectedGroupsData.length === 0) return;

      // Convert audio to base64
      const audioBase64 = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = reject;
        reader.readAsDataURL(audioFile);
      });

      let successCount = 0;
      let errorCount = 0;

      for (const group of selectedGroupsData) {
        try {
          // Use GroupsService for sending audio
          await sendGroupAudio({
            instance_name: currentInstance,
            group_id: group.wa_group_id,
            audio: audioBase64,
            mimetype: audioFile.type || undefined,
            ptt: audioPtt,
            caption: audioCaption?.trim() ? audioCaption.trim() : undefined,
          });

          successCount++;
          await new Promise((resolve) => setTimeout(resolve, 1000));
        } catch (error: unknown) {
          captureHookError(error, 'useGroupsAudio', 'sendAudioToGroup');
          errorCount++;
        }
      }

      toast({
        title: t('pages.groups.success.audioSent'),
        description: t('pages.groups.success.audioSentMessage', {
          success: successCount,
          errors: errorCount,
        }),
      });

      setAudioFile(null);
      setAudioCaption('');
      clearSelection();
      setSendAudioDialogOpen(false);
    } finally {
      setSendingAudio(false);
    }
  }, [
    selectedGroups,
    audioFile,
    audioPtt,
    audioCaption,
    groups,
    toast,
    t,
    clearSelection,
    sendGroupAudio,
    latestInstance,
  ]);

  return {
    audioFile,
    audioPtt,
    audioCaption,
    sendingAudio,
    sendAudioDialogOpen,
    setAudioPtt,
    setAudioCaption,
    setSendAudioDialogOpen,
    handleAudioUpload,
    removeAudioFile,
    handleSendAudio,
  };
}
