import type { GroupParticipant } from '@/hooks/use-groups-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from '../use-toast';

/**
 * Group the members dialog operates on. `instance_name` is the session
 * (instances.instance_id) of a chip that actually participates in this group.
 *
 * WAHA's GOWS engine answers group calls only for sessions that are members of
 * the group — querying participants with any other session returns
 * "2 UNKNOWN: not found" (surfaced to the client as a 500). The old code always
 * used the *latest* chip, which broke every group owned by a different chip.
 * Callers now pass the group's owning/participating chip here; when omitted we
 * fall back to the latest connected instance (legacy callers, or groups with no
 * resolvable chip — no worse than the previous behaviour).
 */
export interface MembersDialogTarget {
  wa_group_id: string;
  name: string;
  instance_name?: string;
}

export interface UseGroupsMembersReturn {
  participants: GroupParticipant[];
  loadingParticipants: boolean;
  membersDialogOpen: boolean;
  selectedGroupForMembers: MembersDialogTarget | null;
  phoneNumberInput: string;
  searchQuery: string;
  filteredParticipants: GroupParticipant[];
  openMembersDialog: (group: MembersDialogTarget) => void;
  closeMembersDialog: () => void;
  setPhoneNumberInput: (value: string) => void;
  setSearchQuery: (value: string) => void;
  handleAddMember: () => Promise<void>;
  handleRemoveMember: (participantId: string) => Promise<void>;
  handleLeaveGroup: () => Promise<void>;
  handlePromoteMember: (participantId: string) => Promise<void>;
  handleDemoteMember: (participantId: string) => Promise<void>;
  handleExportMembers: () => void;
  isAddingMember: boolean;
  isRemovingMember: boolean;
  isLeavingGroup: boolean;
  isPromotingMember: boolean;
  isDemotingMember: boolean;
}

function extractPhoneFromWaId(waId: string): string {
  return waId.replace(/@s\.whatsapp\.net$/, '');
}

/**
 * Turns a participant into a digits-only phone number for CSV export. Prefers
 * the resolved `phone_number`; falls back to the numeric part of the WhatsApp
 * id (mirrors what the dialog shows on screen) so the export matches the list.
 */
function participantToDigits(p: GroupParticipant): string {
  const raw = p.phone_number || p.id.replace(/@.*$/, '');
  return raw.replace(/\D/g, '');
}

/**
 * Builds the phone-list CSV in the exact shape of `modelo-lista-numeros.csv`:
 * a UTF-8 BOM, a `telefone` header, one digits-only number per line (LF),
 * duplicates removed, with a trailing newline. The BOM keeps Excel from
 * mangling the numbers, and the format round-trips into the list importer.
 */
export function buildMembersCsv(participants: GroupParticipant[]): string {
  const seen = new Set<string>();
  const numbers: string[] = [];
  for (const p of participants) {
    const digits = participantToDigits(p);
    if (digits && !seen.has(digits)) {
      seen.add(digits);
      numbers.push(digits);
    }
  }
  // Prepend the UTF-8 BOM (U+FEFF) so Excel reads the numbers as text. Each
  // number is terminated by its own LF, so an empty list yields just the
  // header line (no stray blank line).
  const body = numbers.map((n) => `${n}\n`).join('');
  return `\uFEFFtelefone\n${body}`;
}

/** Safe, readable filename slug from a (possibly emoji-laden) group name. */
export function slugifyGroupName(name: string): string {
  const slug = name
    .normalize('NFKD')
    .replace(/[^a-zA-Z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .toLowerCase();
  return slug || 'grupo';
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
  const [selectedGroupForMembers, setSelectedGroupForMembers] =
    useState<MembersDialogTarget | null>(null);
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
    async (groupId: string, forInstance?: string) => {
      // Query the chip that owns/participates in this group; fall back to the
      // latest instance only when the caller has no better chip. Using the
      // wrong chip makes WAHA return "not found" (500).
      const useInstance = forInstance || instanceName;
      if (!useInstance) return;
      setLoadingParticipants(true);
      try {
        const result = await getGroupParticipants({
          instance_name: useInstance,
          group_id: groupId,
        });
        setParticipants(result);
      } catch (error: unknown) {
        captureHookError(error, 'useGroupsMembers', 'loadParticipants');
        setParticipants([]);
      } finally {
        setLoadingParticipants(false);
      }
    },
    [instanceName, getGroupParticipants]
  );

  const openMembersDialog = useCallback(
    (group: MembersDialogTarget) => {
      setSelectedGroupForMembers(group);
      setMembersDialogOpen(true);
      setPhoneNumberInput('');
      setSearchQuery('');
      loadParticipants(group.wa_group_id, group.instance_name);
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
    const useInstance = selectedGroupForMembers?.instance_name || instanceName;
    if (!selectedGroupForMembers || !phoneNumberInput.trim() || !useInstance) return;
    setIsAddingMember(true);
    try {
      const result = await addGroupMember({
        instance_name: useInstance,
        group_id: selectedGroupForMembers.wa_group_id,
        phone_numbers: [phoneNumberInput.trim()],
      });
      if (result.added_count > 0) {
        toast({
          title: t('pages.groups.members.memberAdded'),
        });
        setPhoneNumberInput('');
        await loadParticipants(selectedGroupForMembers.wa_group_id, useInstance);
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
      const useInstance = selectedGroupForMembers?.instance_name || instanceName;
      if (!selectedGroupForMembers || !useInstance) return;
      setIsRemovingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        const result = await removeGroupMember({
          instance_name: useInstance,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_numbers: [phone],
        });
        if (result.removed_count > 0) {
          toast({
            title: t('pages.groups.members.memberRemoved'),
          });
          await loadParticipants(selectedGroupForMembers.wa_group_id, useInstance);
        }
      } finally {
        setIsRemovingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, removeGroupMember, toast, t, loadParticipants]
  );

  const handleLeaveGroup = useCallback(async () => {
    const useInstance = selectedGroupForMembers?.instance_name || instanceName;
    if (!selectedGroupForMembers || !useInstance) return;
    setIsLeavingGroup(true);
    try {
      await leaveGroupMutation({
        instance_name: useInstance,
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
      const useInstance = selectedGroupForMembers?.instance_name || instanceName;
      if (!selectedGroupForMembers || !useInstance) return;
      setIsPromotingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        await promoteMemberMutation({
          instance_name: useInstance,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_number: phone,
        });
        toast({
          title: t('pages.groups.members.memberPromoted'),
        });
        await loadParticipants(selectedGroupForMembers.wa_group_id, useInstance);
      } finally {
        setIsPromotingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, promoteMemberMutation, toast, t, loadParticipants]
  );

  const handleDemoteMember = useCallback(
    async (participantId: string) => {
      const useInstance = selectedGroupForMembers?.instance_name || instanceName;
      if (!selectedGroupForMembers || !useInstance) return;
      setIsDemotingMember(true);
      try {
        const phone = extractPhoneFromWaId(participantId);
        await demoteMemberMutation({
          instance_name: useInstance,
          group_id: selectedGroupForMembers.wa_group_id,
          phone_number: phone,
        });
        toast({
          title: t('pages.groups.members.memberDemoted'),
        });
        await loadParticipants(selectedGroupForMembers.wa_group_id, useInstance);
      } finally {
        setIsDemotingMember(false);
      }
    },
    [selectedGroupForMembers, instanceName, demoteMemberMutation, toast, t, loadParticipants]
  );

  const handleExportMembers = useCallback(() => {
    if (participants.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.members.exportEmpty'),
      });
      return;
    }
    try {
      const csv = buildMembersCsv(participants);
      const count = csv.trimEnd().split('\n').length - 1; // minus header
      if (count === 0) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.members.exportEmpty'),
        });
        return;
      }
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `membros-${slugifyGroupName(selectedGroupForMembers?.name ?? 'grupo')}.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      toast({
        title: t('pages.groups.members.exportSuccess', { count }),
      });
    } catch (error: unknown) {
      captureHookError(error, 'useGroupsMembers', 'handleExportMembers');
      toast({
        variant: 'destructive',
        title: t('pages.groups.members.exportError'),
      });
    }
  }, [participants, selectedGroupForMembers, toast, t]);

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
    handleExportMembers,
    isAddingMember,
    isRemovingMember,
    isLeavingGroup,
    isPromotingMember,
    isDemotingMember,
  };
}
