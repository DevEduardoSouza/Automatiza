import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type { Group } from '@/types/group';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsUIReturn } from './types';

interface UseGroupsUIParams {
  loadGroups: () => Promise<void>;
  updateGroupOptimistic: (groupId: string, updates: Partial<Group>) => void;
}

/**
 * Hook for managing UI-related functionality (dialogs, favorites, status badges).
 * Uses ConnectRPC GroupsService for all operations.
 */
export function useGroupsUI({
  loadGroups,
  updateGroupOptimistic,
}: UseGroupsUIParams): UseGroupsUIReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { toggleGroupFavorite: toggleFavoriteRpc, updateGroup } = useGroupsConnect();
  const { latestInstance } = useInstancesConnect();

  const [editInviteCodeDialogOpen, setEditInviteCodeDialogOpen] = useState(false);
  const [selectedGroupForInvite, setSelectedGroupForInvite] = useState<Group | null>(null);
  const [inviteCodeInput, setInviteCodeInput] = useState('');

  const handleSaveInviteCode = useCallback(async () => {
    if (!selectedGroupForInvite) return;

    try {
      let code = inviteCodeInput.trim();

      // Extract code from full URL if provided
      if (code.includes('chat.whatsapp.com/')) {
        const match = code.match(/chat\.whatsapp\.com\/([A-Za-z0-9]+)/);
        if (match) {
          code = match[1];
        }
      }

      if (!code) {
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: t('pages.groups.errors.emptyInviteCode'),
        });
        return;
      }

      // Update group with new invite code via ConnectRPC
      await updateGroup({
        id: selectedGroupForInvite.id,
        invite_code: code,
      });

      toast({
        title: t('pages.groups.success.inviteCodeUpdated'),
        description: t('pages.groups.success.inviteCodeUpdatedMessage'),
      });

      setEditInviteCodeDialogOpen(false);
      setSelectedGroupForInvite(null);
      setInviteCodeInput('');

      await loadGroups();
    } catch (error: unknown) {
      captureHookError(error, 'useGroupsUI', 'handleSaveInviteCode');
      const errorMessage =
        error instanceof Error ? error.message : t('pages.groups.errors.saveInviteCode');
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.saveInviteCode'),
        description: errorMessage,
      });
    }
  }, [selectedGroupForInvite, inviteCodeInput, toast, t, loadGroups, updateGroup]);

  const toggleFavorite = useCallback(
    async (groupId: string, currentFavorite: boolean) => {
      // Optimistically update UI immediately
      updateGroupOptimistic(groupId, { is_favorite: !currentFavorite });

      try {
        // Use ConnectRPC to toggle favorite on server
        await toggleFavoriteRpc(groupId, !currentFavorite);

        toast({
          title: currentFavorite
            ? t('pages.groups.success.removedFavorite')
            : t('pages.groups.success.addedFavorite'),
          description: currentFavorite
            ? t('pages.groups.success.removedFavoriteMessage')
            : t('pages.groups.success.addedFavoriteMessage'),
        });
      } catch (error: unknown) {
        captureHookError(error, 'useGroupsUI', 'toggleFavorite');
        // Revert optimistic update on error
        updateGroupOptimistic(groupId, { is_favorite: currentFavorite });

        const errorMessage =
          error instanceof Error ? error.message : t('pages.groups.errors.favoriteUpdate');
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.favoriteUpdate'),
          description: errorMessage,
        });
      }
    },
    [toast, t, updateGroupOptimistic, toggleFavoriteRpc]
  );

  const getStatusBadge = useCallback(
    (status: string) => {
      const variants: Record<string, { label: string; className: string }> = {
        open: {
          label: t('pages.groups.status.open'),
          className: 'bg-gradient-success',
        },
        closed: {
          label: t('pages.groups.status.closed'),
          className: 'bg-destructive',
        },
        full: {
          label: t('pages.groups.status.full'),
          className: 'bg-primary',
        },
      };

      return variants[status] || variants.open;
    },
    [t]
  );

  return {
    editInviteCodeDialogOpen,
    selectedGroupForInvite,
    inviteCodeInput,
    setEditInviteCodeDialogOpen,
    setSelectedGroupForInvite,
    setInviteCodeInput,
    handleSaveInviteCode,
    toggleFavorite,
    getStatusBadge,
  };
}
