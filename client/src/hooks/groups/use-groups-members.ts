import type { GroupParticipant } from '@/hooks/use-groups-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from '../use-toast';

export interface UseGroupsMembersReturn {
  participants: GroupParticipant[];
  loadingParticipants: boolean;
  membersDialogOpen: boolean;
  selectedGroupForMembers: { wa_group_id: string; name: string } | null;
  phoneNumberInput: string;
  searchQuery: string;
  filteredParticipants: GroupParticipant[];
  openMembersDialog: (group: { wa_group_id: string; name: string }) => void;
  closeMembersDialog: () => void;
  setPhoneNumberInput: (value: string) => void;
  setSearchQuery: (value: string) => void;
  handleAddMember: () => Promise<void>;
  handleRemoveMember: (participantId: string) => Promise<void>;
  handleLeaveGroup: () => Promise<void>;
  handlePromoteMember: (participantId: string) => Promise<void>;
  handleDemoteMember: (participantId: string) => Promise<void>;
  isAddingMember: boolean;
  isRemovingMember: boolean;
  isLeavingGroup: boolean;
  isPromotingMember: boolean;
  isDemotingMember: boolean;
}

function extractPhoneFromWaId(waId: string): string {
  return waId.replace(/@s\.whatsapp\.net$/, '');
}

export function useGroupsMembers(): UseGroupsMembersReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    getGroupParticipants,
    addGroupMember,
    removeGroupMember,
    leaveGroup: leaveGroupMutation,
    promoteMember: promoteMemberMutation,
    demoteMember: demoteMemberMutation,
  } = useGroupsConnect();
  const { latestInstance } = useInstancesConnect();
  const instanceName = latestInstance?.instance_id ?? '';

  const [participants, setParticipants] = useState<GroupParticipant[]>([]);
  const [loadingParticipants, setLoadingParticipants] = useState(false);
  const [membersDialogOpen, setMembersDialogOpen] = useState(false);
  const [selectedGroupForMembers, setSelectedGroupForMembers] = useState<{
    wa_group_id: string;
    name: string;
  } | null>(null);
  const [phoneNumberInput, setPhoneNumberInput] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [isAddingMember, setIsAddingMember] = useState(false);
  const [isRemovingMember, setIsRemovingMember] = useState(false);
  const [isLeavingGroup, setIsLeavingGroup] = useState(false);
  const [isPromotingMember, setIsPromotingMember] = useState(false);
  const [isDemotingMember, setIsDemotingMember] = useState(false);

  const filteredParticipants = participants.filter((p) => {
    if (!searchQuery) return true;
    const query = searchQuery.toLowerCase();
    return p.id.toLowerCase().includes(query) || p.phone_number?.toLowerCase().includes(query);
  });

  const loadParticipants = useCallback(
    async (groupId: string) => {
      if (!instanceName) return;
      setLoadingParticipants(true);
      try {
        const result = await getGroupParticipants({
          instance_name: instanceName,
          group_id: groupId,
        });
        setParticipants(result);
      } catch {
        setParticipants([]);
      } finally {
        setLoadingParticipants(false);
      }
    },
    [instanceName, getGroupParticipants]
  );

  const openMembersDialog = useCallback(
    (group: { wa_group_id: string; name: string }) => {
      setSelectedGroupForMembers(group);
      setMembersDialogOpen(true);
      setPhoneNumberInput('');
      setSearchQuery('');
      loadParticipants(group.wa_group_id);
    },
    [loadParticipants]
  );

  const closeMembersDialog = useCallback(() => {
    setMembersDialogOpen(false);
    setSelectedGroupForMembers(null);
    setParticipants([]);
    setPhoneNumberInput('');
    setSearchQuery('');
  }, []);

  const handleAddMember = useCallback(async () => {
    if (!selectedGroupForMembers || !phoneNumberInput.trim() || !instanceName) return;
    setIsAddingMember(true);
    try {
      const result = await addGroupMember({
        instance_name: instanceName,
        group_id: selectedGroupForMembers.wa_group_id,
        phone_numbers: [phoneNumberInput.trim()],
      });
      if (result.added_count > 0) {
        toast({
          title: t('pages.groups.members.memberAdded'),
        });
        setPhoneNumberInput('');
        await loadParticipants(selectedGroupForMembers.wa_group_id);
      }
      if (result.failed_numbers.length > 0) {
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: t('pages.groups.members.memberAddedWithErrors', {
            success: result.added_count,
            errors: result.failed_numbers.length,
          }),
        });
      }
    } finally {
      setIsAddingMember(false);
    }
  }, [
    selectedGroupForMembers,
    phoneNumberInput,
    instanceName,
    addGroupMember,
    toast,
    t,
    loadParticipants,
  ]);

  const handleRemoveMember = useCallback(
    async (participantId: string) => {
      if (!selectedGroupForMembers || !instanceName) return;
      setIsRemovingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        const result = await removeGroupMember({
          instance_name: instanceName,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_numbers: [phone],
        });
        if (result.removed_count > 0) {
          toast({
            title: t('pages.groups.members.memberRemoved'),
          });
          await loadParticipants(selectedGroupForMembers.wa_group_id);
        }
      } finally {
        setIsRemovingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, removeGroupMember, toast, t, loadParticipants]
  );

  const handleLeaveGroup = useCallback(async () => {
    if (!selectedGroupForMembers || !instanceName) return;
    setIsLeavingGroup(true);
    try {
      await leaveGroupMutation({
        instance_name: instanceName,
        group_id: selectedGroupForMembers.wa_group_id,
      });
      toast({
        title: t('pages.groups.members.leftGroup'),
      });
      closeMembersDialog();
    } finally {
      setIsLeavingGroup(false);
    }
  }, [selectedGroupForMembers, instanceName, leaveGroupMutation, toast, t, closeMembersDialog]);

  const handlePromoteMember = useCallback(
    async (participantId: string) => {
      if (!selectedGroupForMembers || !instanceName) return;
      setIsPromotingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        await promoteMemberMutation({
          instance_name: instanceName,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_number: phone,
        });
        toast({
          title: t('pages.groups.members.memberPromoted'),
        });
        await loadParticipants(selectedGroupForMembers.wa_group_id);
      } finally {
        setIsPromotingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, promoteMemberMutation, toast, t, loadParticipants]
  );

  const handleDemoteMember = useCallback(
    async (participantId: string) => {
      if (!selectedGroupForMembers || !instanceName) return;
      setIsDemotingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        await demoteMemberMutation({
          instance_name: instanceName,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_number: phone,
        });
        toast({
          title: t('pages.groups.members.memberDemoted'),
        });
        await loadParticipants(selectedGroupForMembers.wa_group_id);
      } finally {
        setIsDemotingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, demoteMemberMutation, toast, t, loadParticipants]
  );

  return {
    participants,
    loadingParticipants,
    membersDialogOpen,
    selectedGroupForMembers,
    phoneNumberInput,
    searchQuery,
    filteredParticipants,
    openMembersDialog,
    closeMembersDialog,
    setPhoneNumberInput,
    setSearchQuery,
    handleAddMember,
    handleRemoveMember,
    handleLeaveGroup,
    handlePromoteMember,
    handleDemoteMember,
    isAddingMember,
    isRemovingMember,
    isLeavingGroup,
    isPromotingMember,
    isDemotingMember,
  };
}
