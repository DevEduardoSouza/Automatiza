import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import type { Group } from '@/types/group';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsPollReturn } from './types';

interface UseGroupsPollParams {
  selectedGroups: string[];
  groups: Group[];
  clearSelection: () => void;
}

/**
 * Hook for managing poll creation and sending functionality.
 * Uses ConnectRPC (GroupsService).
 */
export function useGroupsPoll({
  selectedGroups,
  groups,
  clearSelection,
}: UseGroupsPollParams): UseGroupsPollReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { sendGroupPoll } = useGroupsConnect();
  const { latestInstance } = useInstancesConnect();

  const [pollDialogOpen, setPollDialogOpen] = useState(false);
  const [pollQuestion, setPollQuestion] = useState('');
  const [pollOptions, setPollOptions] = useState<string[]>(['', '']);
  const [sendingPoll, setSendingPoll] = useState(false);

  const addPollOption = useCallback(() => {
    // Validation: max 12 options
    if (pollOptions.length < 12) {
      setPollOptions((prev) => [...prev, '']);
    } else {
      toast({
        title: t('pages.groups.errors.limitReached'),
        description: t('pages.groups.errors.maxPollOptions'),
        variant: 'destructive',
      });
    }
  }, [pollOptions.length, toast, t]);

  const removePollOption = useCallback(
    (index: number) => {
      // Validation: min 2 options
      if (pollOptions.length > 2) {
        setPollOptions((prev) => prev.filter((_, i) => i !== index));
      }
    },
    [pollOptions.length]
  );

  const updatePollOption = useCallback((index: number, value: string) => {
    setPollOptions((prev) => {
      const newOptions = [...prev];
      newOptions[index] = value;
      return newOptions;
    });
  }, []);

  const handleSendPoll = useCallback(async () => {
    if (!pollQuestion.trim()) {
      toast({
        title: t('pages.groups.errors.questionRequired'),
        description: t('pages.groups.errors.questionRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }

    const validOptions = pollOptions.filter((opt) => opt.trim());
    // Validation: 2-12 options
    if (validOptions.length < 2) {
      toast({
        title: t('pages.groups.errors.insufficientOptions'),
        description: t('pages.groups.errors.insufficientOptionsMessage'),
        variant: 'destructive',
      });
      return;
    }

    if (selectedGroups.length === 0) {
      toast({
        title: t('pages.groups.errors.selectGroups'),
        description: t('pages.groups.errors.selectGroupsPollMessage'),
        variant: 'destructive',
      });
      return;
    }

    setSendingPoll(true);
    try {
      // Get instance from ConnectRPC hook
      if (!latestInstance || latestInstance.status !== 'connected') {
        throw new Error(t('pages.groups.errors.noInstance'));
      }

      const pollInstanceName = latestInstance.instance_id;
      const selectedGroupsData = groups.filter((g) => selectedGroups.includes(g.id));

      let successCount = 0;
      let errorCount = 0;
      const errorDetails: string[] = [];

      for (const group of selectedGroupsData) {
        try {
          // Use GroupsService for sending polls
          await sendGroupPoll({
            instance_name: pollInstanceName,
            group_id: group.wa_group_id,
            name: pollQuestion,
            options: validOptions,
          });
          successCount++;

          await new Promise((resolve) => setTimeout(resolve, 1000));
        } catch (error: unknown) {
          errorCount++;
          const errorMsg = error instanceof Error ? error.message : t('common.unknownError');
          errorDetails.push(`${group.name}: ${errorMsg}`);
        }
      }

      if (errorDetails.length > 0) {
        toast({
          title: successCount > 0 ? t('pages.groups.success.pollPartiallySent') : t('common.error'),
          description: t('pages.groups.success.pollSentWithErrors', {
            success: successCount,
            errors: errorCount,
          }),
          variant: successCount === 0 ? 'destructive' : 'default',
        });
      } else {
        toast({
          title: t('pages.groups.success.pollSent'),
          description: t('pages.groups.success.pollSentMessage', {
            count: successCount,
          }),
        });
      }

      setPollDialogOpen(false);
      setPollQuestion('');
      setPollOptions(['', '']);
      clearSelection();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : t('pages.groups.errors.sendPoll');
      toast({
        title: t('pages.groups.errors.sendPoll'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setSendingPoll(false);
    }
  }, [
    pollQuestion,
    pollOptions,
    selectedGroups,
    groups,
    toast,
    t,
    clearSelection,
    sendGroupPoll,
    latestInstance,
  ]);

  return {
    pollDialogOpen,
    pollQuestion,
    pollOptions,
    sendingPoll,
    setPollDialogOpen,
    setPollQuestion,
    setPollOptions,
    addPollOption,
    removePollOption,
    updatePollOption,
    handleSendPoll,
  };
}
