import { type GroupParticipant, useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import * as Sentry from '@sentry/react';
import type { Group } from '@/types/group';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsMessagingReturn } from './types';

interface UseGroupsMessagingParams {
  selectedGroups: string[];
  groups: Group[];
  clearSelection: () => void;
}

/**
 * Hook for managing message sending functionality.
 * Uses ConnectRPC (GroupsService) for messaging and InstancesService for status checks.
 * Uses gRPC via ConnectRPC.
 */
export function useGroupsMessaging({
  selectedGroups,
  groups,
  clearSelection,
}: UseGroupsMessagingParams): UseGroupsMessagingReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { sendGroupMessage, sendGroupImage, getGroupParticipants } = useGroupsConnect();
  const { primaryInstance, checkInstanceStatus } = useInstancesConnect();

  const [message, setMessage] = useState('');
  const [mediaFile, setMediaFile] = useState<File | null>(null);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [sendMessageDialogOpen, setSendMessageDialogOpen] = useState(false);
  const [mentionParticipants, setMentionParticipants] = useState<GroupParticipant[]>([]);
  const [loadingMentionParticipants, setLoadingMentionParticipants] = useState(false);

  const insertMention = useCallback(
    (mention: string) => {
      setMessage((prev) => prev + mention);
      setMentionOpen(false);

      toast({
        title: t('pages.groups.success.mentionAdded'),
        description: t('pages.groups.success.mentionAddedMessage', {
          mention: mention.trim(),
        }),
      });
    },
    [toast, t]
  );

  const loadMentionParticipants = useCallback(async () => {
    if (selectedGroups.length === 0 || !primaryInstance || primaryInstance.status !== 'connected') {
      setMentionParticipants([]);
      return;
    }

    setLoadingMentionParticipants(true);
    try {
      const groupsToFetch = groups
        .filter((g) => selectedGroups.includes(g.id))
        .slice(0, 5);

      const allParticipants: GroupParticipant[] = [];
      const seen = new Set<string>();
      let errorCount = 0;

      for (const group of groupsToFetch) {
        try {
          const participants = await getGroupParticipants({
            instance_name: primaryInstance.instance_id,
            group_id: group.wa_group_id,
          });
          for (const p of participants) {
            if (!seen.has(p.phone_number)) {
              seen.add(p.phone_number);
              allParticipants.push(p);
            }
          }
        } catch (error) {
          errorCount++;
          Sentry.addBreadcrumb({
            category: 'groups',
            message: `Failed to fetch participants for group ${group.name}`,
            level: 'warning',
            data: { groupId: group.id },
          });
        }
      }

      // SF11: If ALL groups failed, capture to Sentry
      if (errorCount === groupsToFetch.length && groupsToFetch.length > 0) {
        Sentry.captureMessage('All participant fetches failed', {
          level: 'error',
          extra: { errorCount, totalGroups: groupsToFetch.length },
        });
      }

      setMentionParticipants(allParticipants);
    } finally {
      setLoadingMentionParticipants(false);
    }
  }, [selectedGroups, groups, primaryInstance, getGroupParticipants]);

  const handleMentionOpenChange = useCallback(
    (open: boolean) => {
      setMentionOpen(open);
      if (open) {
        loadMentionParticipants();
      }
    },
    [loadMentionParticipants]
  );

  const handleMediaUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];

      if (!file) {
        setMediaFile(null);
        return;
      }

      // Validate file size: max 7MB
      const maxBytes = 7 * 1024 * 1024;
      if (file.size > maxBytes) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.fileTooLarge'),
          description: t('pages.groups.errors.fileTooLargeMessage'),
        });
        e.target.value = '';
        setMediaFile(null);
        return;
      }

      const isImage = file.type.startsWith('image/');
      const isVideo = file.type.startsWith('video/');

      if (!isImage && !isVideo) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.invalidFile'),
          description: t('pages.groups.errors.onlyImagesAndVideos'),
        });
        setMediaFile(null);
        return;
      }

      setMediaFile(file);
    },
    [toast, t]
  );

  const removeMediaFile = useCallback(() => {
    setMediaFile(null);
  }, []);

  /**
   * Send message to selected groups.
   * Uses ConnectRPC GroupsService for messaging and InstancesService for status checks.
   */
  const handleSendMessage = useCallback(async () => {
    if (selectedGroups.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.noGroupSelected'),
        description: t('pages.groups.errors.noGroupSelectedMessage'),
      });
      return;
    }

    if (!message.trim()) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.emptyMessage'),
        description: t('pages.groups.errors.emptyMessageDesc'),
      });
      return;
    }

    // Get instance from ConnectRPC hook
    if (!primaryInstance || primaryInstance.status !== 'connected') {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.noInstance'),
        description: t('pages.groups.errors.connectWhatsAppFirst'),
      });
      return;
    }

    const msgInstanceName = primaryInstance.instance_id;

    try {
      // Check instance status via InstancesService gRPC
      const statusData = await checkInstanceStatus(msgInstanceName);

      const isDisconnected = statusData?.state === 'close';

      if (!statusData?.success || isDisconnected) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.whatsappDisconnected'),
          description: t('pages.groups.errors.reconnectWhatsApp'),
        });
        return;
      }
    } catch (error) {
      captureHookError(error, 'useGroupsMessaging', 'checkInstanceStatus');
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.verifyConnection'),
        description: t('pages.groups.errors.verifyConnectionMessage'),
      });
      return;
    }

    // Get selected groups data from the groups prop
    const selectedGroupsData = groups
      .filter((g) => selectedGroups.includes(g.id))
      .map((g) => ({
        wa_group_id: g.wa_group_id,
        name: g.name,
      }));

    if (selectedGroupsData.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.noGroupSelected'),
        description: t('pages.groups.errors.noGroupSelectedMessage'),
      });
      return;
    }

    const processedMessage = message;
    const hasMentionAll = message.includes('@todos');

    // Detect individual @number mentions (e.g. @5511999999999)
    const individualMentionMatches = message.match(/@(\d{8,15})/g);
    const individualMentions = individualMentionMatches
      ? individualMentionMatches.map((m) => m.slice(1))
      : [];
    const hasIndividualMentions = individualMentions.length > 0;

    let imageBase64: string | undefined = undefined;
    if (mediaFile) {
      try {
        imageBase64 = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => {
            const result = reader.result as string;
            resolve(result);
          };
          reader.onerror = reject;
          reader.readAsDataURL(mediaFile);
        });
      } catch (error) {
        captureHookError(error, 'useGroupsMessaging', 'fileReader');
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.processImage'),
          description: t('pages.groups.errors.processImageMessage', {
            filename: mediaFile.name,
          }),
        });
        // WF16: Return after file read error to prevent sending without image
        return;
      }
    }

    let successCount = 0;
    let errorCount = 0;

    for (const group of selectedGroupsData) {
      try {
        let mentions: string[] | undefined = undefined;

        if (hasMentionAll) {
          try {
            // Fetch participants via GroupsService gRPC
            const participants = await getGroupParticipants({
              instance_name: msgInstanceName,
              group_id: group.wa_group_id,
            });

            if (!participants || participants.length === 0) {
              toast({
                variant: 'destructive',
                title: t('pages.groups.warnings.groupWarning', {
                  name: group.name,
                }),
                description: t('pages.groups.errors.emptyGroupMention'),
              });
              errorCount++;
              continue;
            }

            mentions = participants.map((p) => p.id);
          } catch (error) {
            captureHookError(error, 'useGroupsMessaging', 'fetchParticipants', {
              groupId: group.wa_group_id,
              groupName: group.name,
            });
            toast({
              variant: 'destructive',
              title: t('pages.groups.errors.groupError', { name: group.name }),
              description: t('pages.groups.errors.fetchParticipantsFailed'),
            });
            errorCount++;
            continue;
          }
        } else if (hasIndividualMentions) {
          // Build JIDs from individual phone numbers
          mentions = individualMentions.map((num) => `${num}@s.whatsapp.net`);
        }

        // Use GroupsService for sending messages
        if (imageBase64) {
          // Send image with caption via GroupsService
          await sendGroupImage({
            instance_name: msgInstanceName,
            group_id: group.wa_group_id,
            image: imageBase64,
            caption: processedMessage,
            mimetype: mediaFile?.type,
          });
        } else {
          // Send text message via GroupsService
          await sendGroupMessage({
            instance_name: msgInstanceName,
            group_id: group.wa_group_id,
            message: processedMessage,
            mentions,
          });
        }

        successCount++;

        // Rate limiting delay between messages
        await new Promise((resolve) => setTimeout(resolve, 1000));
      } catch (error) {
        captureHookError(error, 'useGroupsMessaging', 'sendMessage', {
          groupId: group.wa_group_id,
          groupName: group.name,
        });
        errorCount++;
      }
    }

    toast({
      title: t('pages.groups.success.messagesSent'),
      description: t('pages.groups.success.messagesSentMessage', {
        success: successCount,
        errors: errorCount,
      }),
    });

    setMessage('');
    setMediaFile(null);
    clearSelection();
    setSendMessageDialogOpen(false);
  }, [
    selectedGroups,
    message,
    groups,
    mediaFile,
    toast,
    t,
    clearSelection,
    checkInstanceStatus,
    getGroupParticipants,
    sendGroupMessage,
    sendGroupImage,
    primaryInstance,
  ]);

  return {
    message,
    mediaFile,
    mentionOpen,
    sendMessageDialogOpen,
    mentionParticipants,
    loadingMentionParticipants,
    setMessage,
    setMentionOpen: handleMentionOpenChange,
    setSendMessageDialogOpen,
    insertMention,
    handleSendMessage,
    handleMediaUpload,
    removeMediaFile,
  };
}
