import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useToast } from '@/hooks/use-toast';
import type { Group } from '@/types/group';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ACTION_STRING_MAP, BULK_ACTION_CONFIG, type UseGroupsBulkActionsReturn } from './types';

interface UseGroupsBulkActionsParams {
  selectedGroups: string[];
  groups: Group[];
  instanceName: string;
  loadGroups: () => Promise<void>;
  clearSelection: () => void;
  setLoading: (value: boolean) => void;
}

/**
 * Hook for managing bulk actions on groups (close, open, rename, photo, description).
 * Uses ConnectRPC (GroupsService).
 */
export function useGroupsBulkActions({
  selectedGroups,
  groups,
  instanceName,
  loadGroups,
  clearSelection,
  setLoading,
}: UseGroupsBulkActionsParams): UseGroupsBulkActionsReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    updateGroupSettings,
    updateGroupSubject,
    updateGroupPicture,
    updateGroupDescription,
    updateGroup,
  } = useGroupsConnect();

  const [groupName, setGroupName] = useState('');
  const [groupPhoto, setGroupPhoto] = useState<File | null>(null);
  const [description, setDescription] = useState('');
  const [autoNumberGroups, setAutoNumberGroups] = useState(false);
  const [changeNameDialogOpen, setChangeNameDialogOpen] = useState(false);
  const [changePhotoDialogOpen, setChangePhotoDialogOpen] = useState(false);
  const [changeDescriptionDialogOpen, setChangeDescriptionDialogOpen] = useState(false);

  const handlePhotoUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file?.type.startsWith('image/')) {
        setGroupPhoto(file);
      } else {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.invalidFile'),
          description: t('pages.groups.errors.onlyImages'),
        });
      }
    },
    [toast, t]
  );

  const handleBulkAction = useCallback(
    async (action: string) => {
      if (selectedGroups.length === 0) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.noGroupSelected'),
          description: t('pages.groups.errors.selectGroupFirst'),
        });
        return;
      }

      const selectedGroupsData = groups.filter((g) => selectedGroups.includes(g.id));

      setLoading(true);
      let successCount = 0;
      let errorCount = 0;
      const errorDetails: string[] = [];

      const actionType = ACTION_STRING_MAP[action];
      const actionConfig = actionType ? BULK_ACTION_CONFIG[actionType] : null;

      try {
        for (const group of selectedGroupsData) {
          try {
            switch (action) {
              case 'Fechar grupos': {
                // Close group (announcement mode = admin only)
                const closeResult = await updateGroupSettings({
                  instance_name: instanceName,
                  group_id: group.wa_group_id,
                  messages_admin_only: true,
                });
                if (!closeResult) {
                  throw new Error(t('pages.groups.errors.closeGroup'));
                }

                // Update local status
                await updateGroup({ id: group.id, status: 'closed' });
                successCount++;
                break;
              }

              case 'Abrir grupos': {
                // Open group (not announcement mode = everyone can send)
                const openResult = await updateGroupSettings({
                  instance_name: instanceName,
                  group_id: group.wa_group_id,
                  messages_admin_only: false,
                });
                if (!openResult) {
                  throw new Error(t('pages.groups.errors.openGroup'));
                }

                // Update local status
                await updateGroup({ id: group.id, status: 'open' });
                successCount++;
                break;
              }

              case 'Alterar nome': {
                if (!autoNumberGroups && !groupName.trim()) {
                  throw new Error(t('pages.groups.errors.nameRequired'));
                }

                const newGroupName = autoNumberGroups
                  ? `#${selectedGroupsData.indexOf(group) + 1} ${
                      groupName || t('pages.groups.defaults.newGroupName')
                    }`
                  : groupName;

                const subjectResult = await updateGroupSubject({
                  instance_name: instanceName,
                  group_id: group.wa_group_id,
                  subject: newGroupName,
                });
                if (!subjectResult) {
                  throw new Error(t('pages.groups.errors.changeName'));
                }

                // Update local name
                await updateGroup({ id: group.id, name: newGroupName });
                successCount++;
                break;
              }

              case 'Alterar foto': {
                if (!groupPhoto) {
                  throw new Error(t('pages.groups.errors.noPhotoSelected'));
                }

                const reader = new FileReader();
                const base64Image = await new Promise<string>((resolve, reject) => {
                  reader.onload = () => {
                    const base64 = reader.result as string;
                    const base64Data = base64.split(',')[1];
                    resolve(base64Data);
                  };
                  reader.onerror = reject;
                  reader.readAsDataURL(groupPhoto);
                });

                const pictureResult = await updateGroupPicture({
                  instance_name: instanceName,
                  group_id: group.wa_group_id,
                  base64_data: base64Image,
                  mimetype: groupPhoto.type,
                });
                if (!pictureResult) {
                  throw new Error(t('pages.groups.errors.changePhoto'));
                }
                successCount++;
                break;
              }

              case 'Alterar descrição': {
                if (!description.trim()) {
                  throw new Error(t('pages.groups.errors.descriptionRequired'));
                }
                const descResult = await updateGroupDescription({
                  instance_name: instanceName,
                  group_id: group.wa_group_id,
                  description,
                });
                if (!descResult) {
                  throw new Error(t('pages.groups.errors.changeDescription'));
                }

                // Update local description
                await updateGroup({ id: group.id, description });
                successCount++;
                break;
              }

              default:
                throw new Error(t('pages.groups.errors.unknownAction'));
            }
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : t('common.unknownError');
            errorDetails.push(`${group.name}: ${errorMessage}`);
            errorCount++;
          }
        }

        await loadGroups();

        if (errorDetails.length > 0) {
          toast({
            title: successCount > 0 ? t('pages.groups.success.partialComplete') : t('common.error'),
            description: t('pages.groups.success.groupsUpdatedWithErrors', {
              success: successCount,
              errors: errorCount,
            }),
            variant: successCount === 0 ? 'destructive' : 'default',
          });
        } else {
          toast({
            title: t('pages.groups.success.actionComplete'),
            description: t('pages.groups.success.groupsUpdated', {
              count: successCount,
            }),
          });
        }

        setGroupName('');
        setGroupPhoto(null);
        setDescription('');
        clearSelection();
        setAutoNumberGroups(false);
        setChangeNameDialogOpen(false);
        setChangePhotoDialogOpen(false);
        setChangeDescriptionDialogOpen(false);
      } catch (error) {
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: error instanceof Error ? error.message : t('pages.groups.errors.bulkAction'),
        });
      } finally {
        setLoading(false);
      }
    },
    [
      selectedGroups,
      groups,
      instanceName,
      autoNumberGroups,
      groupName,
      groupPhoto,
      description,
      toast,
      t,
      loadGroups,
      clearSelection,
      setLoading,
      updateGroupSettings,
      updateGroupSubject,
      updateGroupPicture,
      updateGroupDescription,
      updateGroup,
    ]
  );

  return {
    groupName,
    groupPhoto,
    description,
    autoNumberGroups,
    changeNameDialogOpen,
    changePhotoDialogOpen,
    changeDescriptionDialogOpen,
    setGroupName,
    setDescription,
    setAutoNumberGroups,
    setChangeNameDialogOpen,
    setChangePhotoDialogOpen,
    setChangeDescriptionDialogOpen,
    handlePhotoUpload,
    handleBulkAction,
  };
}
