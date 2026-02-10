import { useAuth } from '@/contexts';
import { listRedirectLinks } from '@/gen/redirect/v1/redirect-RedirectLinkService_connectquery';
import {
  DistributionStrategy,
  type SavedRedirectLink as ProtoSavedRedirectLink,
  type RedirectGroup,
} from '@/gen/redirect/v1/redirect_pb';
import {
  CheckSlugAvailabilityRequestSchema,
  CreateRedirectLinkRequestSchema,
  DeleteRedirectLinkRequestSchema,
  RedirectGroupSchema,
  UpdateRedirectLinkRequestSchema,
} from '@/gen/redirect/v1/redirect_pb';
import { RedirectLinkService } from '@/gen/redirect/v1/redirect_pb';
import { useDebouncedValue } from '@/hooks/use-debounced-value';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useToast } from '@/hooks/use-toast';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import type { DragEndEvent } from '@dnd-kit/core';
import { arrayMove } from '@dnd-kit/sortable';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

type LocalDistributionStrategy = 'member_limit' | 'click_limit' | 'round_robin';

/**
 * Local group interface matching the UI component expectations (snake_case).
 * Used internally to avoid breaking existing UI components.
 */
interface LocalGroup {
  id: string;
  name: string;
  wa_group_id: string;
  invite_code?: string;
  members_count: number;
  member_limit: number;
  priority?: number;
  click_limit?: number;
  is_favorite?: boolean;
}

/**
 * Local saved redirect link interface matching UI expectations (snake_case).
 */
interface LocalSavedRedirectLink {
  id: string;
  user_id: string;
  name: string;
  slug: string;
  distribution_strategy: LocalDistributionStrategy;
  group_priorities: LocalGroup[];
  is_active: boolean;
  total_clicks: number;
  created_at: string;
  updated_at: string;
}

interface UseRedirectLinkReturn {
  slug: string;
  linkName: string;
  allGroups: LocalGroup[];
  groupPriorities: LocalGroup[];
  selectedGroupIds: string[];
  savedLinks: LocalSavedRedirectLink[];
  loading: boolean;
  searchTerm: string;
  debouncedSearchTerm: string;
  dialogOpen: boolean;
  distributionStrategy: LocalDistributionStrategy;
  editingLink: LocalSavedRedirectLink | null;
  setSlug: (value: string) => void;
  setLinkName: (value: string) => void;
  setSearchTerm: (value: string) => void;
  setDialogOpen: (value: boolean) => void;
  setDistributionStrategy: (value: LocalDistributionStrategy) => void;
  setGroupPriorities: React.Dispatch<React.SetStateAction<LocalGroup[]>>;
  handleDragEnd: (event: DragEndEvent) => void;
  toggleGroupSelection: (groupId: string) => void;
  applyGroupSelection: () => void;
  moveGroup: (index: number, direction: 'up' | 'down') => void;
  handleCreateOrUpdate: () => Promise<void>;
  deleteLink: (linkId: string) => Promise<void>;
  startEditLink: (link: LocalSavedRedirectLink) => void;
  cancelEdit: () => void;
  copyToClipboard: (linkSlug: string) => void;
}

/**
 * Converts proto RedirectGroup (camelCase) to local group (snake_case).
 */
function fromProtoGroup(group: RedirectGroup): LocalGroup {
  return {
    id: group.id,
    name: group.name,
    wa_group_id: group.waGroupId,
    invite_code: group.inviteCode,
    members_count: group.membersCount,
    member_limit: group.memberLimit,
    priority: group.priority,
    click_limit: group.clickLimit,
    is_favorite: group.isFavorite,
  };
}

/**
 * Converts local group (snake_case) to proto RedirectGroup (camelCase).
 */
function toProtoGroup(group: LocalGroup): RedirectGroup {
  return create(RedirectGroupSchema, {
    id: group.id,
    name: group.name,
    waGroupId: group.wa_group_id,
    inviteCode: group.invite_code,
    membersCount: group.members_count,
    memberLimit: group.member_limit,
    priority: group.priority,
    clickLimit: group.click_limit ?? 0,
    isFavorite: group.is_favorite,
  });
}

/**
 * Converts proto SavedRedirectLink to local format.
 */
function fromProtoSavedLink(link: ProtoSavedRedirectLink): LocalSavedRedirectLink {
  return {
    id: link.id,
    user_id: link.userId,
    name: link.name,
    slug: link.slug,
    distribution_strategy: fromProtoStrategy(link.distributionStrategy),
    group_priorities: link.groupPriorities.map(fromProtoGroup),
    is_active: link.isActive,
    total_clicks: link.totalClicks,
    created_at: link.createdAt,
    updated_at: link.updatedAt,
  };
}

/**
 * Converts local strategy to proto enum.
 */
function toProtoStrategy(strategy: LocalDistributionStrategy): DistributionStrategy {
  switch (strategy) {
    case 'click_limit':
      return DistributionStrategy.CLICK_LIMIT;
    case 'round_robin':
      return DistributionStrategy.ROUND_ROBIN;
    default:
      return DistributionStrategy.MEMBER_LIMIT;
  }
}

/**
 * Converts proto enum to local strategy.
 */
function fromProtoStrategy(strategy: DistributionStrategy): LocalDistributionStrategy {
  switch (strategy) {
    case DistributionStrategy.CLICK_LIMIT:
      return 'click_limit';
    case DistributionStrategy.ROUND_ROBIN:
      return 'round_robin';
    default:
      return 'member_limit';
  }
}

/**
 * Hook for redirect link page logic.
 * Manages redirect link creation, editing, and group priority using ConnectRPC.
 */
export function useRedirectLink(): UseRedirectLinkReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const transport = useTransport();

  const [slug, setSlug] = useState('');
  const [linkName, setLinkName] = useState('');
  const [groupPriorities, setGroupPriorities] = useState<LocalGroup[]>([]);
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [distributionStrategy, setDistributionStrategy] =
    useState<LocalDistributionStrategy>('click_limit');
  const [editingLink, setEditingLink] = useState<LocalSavedRedirectLink | null>(null);

  const debouncedSearchTerm = useDebouncedValue(searchTerm, 300);

  // Use the same groups hook as other pages for consistency
  const { groups: connectGroups, loading: loadingGroups } = useGroupsConnect();

  // Map groups from useGroupsConnect to LocalGroup format
  const allGroups: LocalGroup[] = connectGroups.map((g) => ({
    id: g.id,
    name: g.name,
    wa_group_id: g.wa_group_id,
    invite_code: g.invite_code,
    members_count: g.members_count,
    member_limit: g.member_limit,
    is_favorite: g.is_favorite,
  }));

  // Query: List saved redirect links using ConnectRPC Query
  const { data: linksResponse, isLoading: loadingLinks } = useQuery(
    listRedirectLinks,
    {},
    { enabled: !!user }
  );

  const savedLinks: LocalSavedRedirectLink[] = (linksResponse?.links ?? []).map(fromProtoSavedLink);

  // Mutation: Create redirect link
  const createMutation = useMutation({
    mutationFn: async (params: {
      name: string;
      slug: string;
      distributionStrategy: DistributionStrategy;
      groupPriorities: RedirectGroup[];
    }) => {
      const client = createClient(RedirectLinkService, transport);
      return client.createRedirectLink(create(CreateRedirectLinkRequestSchema, params));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listRedirectLinks, cardinality: 'finite' }),
      });
      toast({
        title: t('pages.redirectLink.success.created'),
        description: t('pages.redirectLink.success.createdMessage'),
      });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.create'),
        description: t('pages.redirectLink.errors.createMessage'),
      });
    },
  });

  // Mutation: Update redirect link
  const updateMutation = useMutation({
    mutationFn: async (params: {
      id: string;
      name: string;
      distributionStrategy: DistributionStrategy;
      groupPriorities: RedirectGroup[];
    }) => {
      const client = createClient(RedirectLinkService, transport);
      return client.updateRedirectLink(create(UpdateRedirectLinkRequestSchema, params));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listRedirectLinks, cardinality: 'finite' }),
      });
      toast({
        title: t('pages.redirectLink.success.updated'),
        description: t('pages.redirectLink.success.updatedMessage'),
      });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.update'),
        description: t('pages.redirectLink.errors.updateMessage'),
      });
    },
  });

  // Mutation: Delete redirect link
  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const client = createClient(RedirectLinkService, transport);
      return client.deleteRedirectLink(create(DeleteRedirectLinkRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listRedirectLinks, cardinality: 'finite' }),
      });
      toast({
        title: t('pages.redirectLink.success.deleted'),
        description: t('pages.redirectLink.success.deletedMessage'),
      });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.delete'),
        description: t('pages.redirectLink.errors.deleteMessage'),
      });
    },
  });

  // Combined loading state
  const loading =
    loadingGroups ||
    loadingLinks ||
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending;

  const generateRandomSlug = useCallback(() => {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    let randomSlug = '';
    for (let i = 0; i < 8; i++) {
      randomSlug += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    setSlug(randomSlug);
  }, []);

  // Generate slug on mount
  useEffect(() => {
    generateRandomSlug();
  }, [generateRandomSlug]);

  const resetForm = useCallback(() => {
    setLinkName('');
    setSlug('');
    setGroupPriorities([]);
    setSelectedGroupIds([]);
    setEditingLink(null);
    generateRandomSlug();
  }, [generateRandomSlug]);

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;

      if (over && active.id !== over.id) {
        setGroupPriorities((items) => {
          const oldIndex = items.findIndex((item) => item.id === active.id);
          const newIndex = items.findIndex((item) => item.id === over.id);

          const newItems = arrayMove(items, oldIndex, newIndex);
          const updatedItems = newItems.map((item, index) => ({
            ...item,
            priority: index + 1,
          }));

          toast({
            title: t('pages.redirectLink.success.orderUpdated'),
            description: t('pages.redirectLink.success.orderUpdatedMessage'),
          });

          return updatedItems;
        });
      }
    },
    [toast, t]
  );

  const toggleGroupSelection = useCallback((groupId: string) => {
    setSelectedGroupIds((prev) => {
      if (prev.includes(groupId)) {
        return prev.filter((id) => id !== groupId);
      }
      return [...prev, groupId];
    });
  }, []);

  const applyGroupSelection = useCallback(() => {
    const selectedGroups = allGroups
      .filter((g) => selectedGroupIds.includes(g.id))
      .map((g, index) => ({
        ...g,
        priority: index + 1,
      }));
    setGroupPriorities(selectedGroups);
    setSelectedGroupIds(selectedGroups.map((g) => g.id));
    setDialogOpen(false);
    toast({
      title: t('pages.redirectLink.success.groupsUpdated'),
      description: t('pages.redirectLink.success.groupsUpdatedMessage', {
        count: selectedGroups.length,
      }),
    });
  }, [allGroups, selectedGroupIds, toast, t]);

  const moveGroup = useCallback(
    (index: number, direction: 'up' | 'down') => {
      const targetIndex = direction === 'up' ? index - 1 : index + 1;
      if (targetIndex < 0 || targetIndex >= groupPriorities.length) return;

      const newPriorities = [...groupPriorities];
      [newPriorities[index], newPriorities[targetIndex]] = [
        newPriorities[targetIndex],
        newPriorities[index],
      ];
      newPriorities.forEach((group, idx) => {
        group.priority = idx + 1;
      });
      setGroupPriorities(newPriorities);
      toast({
        title: t('pages.redirectLink.success.orderUpdated'),
        description: t('pages.redirectLink.success.orderUpdatedMessage'),
      });
    },
    [groupPriorities, toast, t]
  );

  const handleCreateLink = useCallback(async () => {
    if (!linkName.trim()) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.nameRequired'),
        description: t('pages.redirectLink.errors.nameRequiredMessage'),
      });
      return;
    }

    if (!slug.trim()) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.slugRequired'),
        description: t('pages.redirectLink.errors.slugRequiredMessage'),
      });
      return;
    }

    if (groupPriorities.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.groupsRequired'),
        description: t('pages.redirectLink.errors.groupsRequiredMessage'),
      });
      return;
    }

    const slugRegex = /^[a-z0-9-]+$/;
    if (!slugRegex.test(slug)) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.invalidSlug'),
        description: t('pages.redirectLink.errors.invalidSlugMessage'),
      });
      return;
    }

    if (distributionStrategy === 'click_limit') {
      const hasInvalidLimits = groupPriorities.some((g) => !g.click_limit || g.click_limit <= 0);

      if (hasInvalidLimits) {
        toast({
          variant: 'destructive',
          title: t('pages.redirectLink.errors.clickLimitsRequired'),
          description: t('pages.redirectLink.errors.clickLimitsRequiredMessage'),
        });
        return;
      }
    }

    // Check slug availability
    try {
      const client = createClient(RedirectLinkService, transport);
      const checkResponse = await client.checkSlugAvailability(
        create(CheckSlugAvailabilityRequestSchema, { slug: slug.toLowerCase() })
      );

      if (!checkResponse.available) {
        toast({
          variant: 'destructive',
          title: t('pages.redirectLink.errors.slugExists'),
          description: t('pages.redirectLink.errors.slugExistsMessage'),
        });
        return;
      }
    } catch {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.checkSlug'),
        description: t('pages.redirectLink.errors.checkSlugMessage'),
      });
      return;
    }

    createMutation.mutate(
      {
        name: linkName.trim(),
        slug: slug.toLowerCase(),
        distributionStrategy: toProtoStrategy(distributionStrategy),
        groupPriorities: groupPriorities.map(toProtoGroup),
      },
      {
        onSuccess: resetForm,
      }
    );
  }, [
    linkName,
    slug,
    groupPriorities,
    distributionStrategy,
    toast,
    t,
    transport,
    createMutation,
    resetForm,
  ]);

  const handleUpdateLink = useCallback(async () => {
    if (!editingLink) return;

    if (!linkName.trim()) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.nameRequired'),
        description: t('pages.redirectLink.errors.nameRequiredMessage'),
      });
      return;
    }

    if (groupPriorities.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.redirectLink.errors.groupsRequired'),
        description: t('pages.redirectLink.errors.groupsRequiredMessage'),
      });
      return;
    }

    if (distributionStrategy === 'click_limit') {
      const hasInvalidLimits = groupPriorities.some((g) => !g.click_limit || g.click_limit <= 0);

      if (hasInvalidLimits) {
        toast({
          variant: 'destructive',
          title: t('pages.redirectLink.errors.clickLimitsRequired'),
          description: t('pages.redirectLink.errors.clickLimitsRequiredMessage'),
        });
        return;
      }
    }

    updateMutation.mutate(
      {
        id: editingLink.id,
        name: linkName.trim(),
        distributionStrategy: toProtoStrategy(distributionStrategy),
        groupPriorities: groupPriorities.map(toProtoGroup),
      },
      {
        onSuccess: resetForm,
      }
    );
  }, [
    editingLink,
    linkName,
    groupPriorities,
    distributionStrategy,
    toast,
    t,
    updateMutation,
    resetForm,
  ]);

  const handleCreateOrUpdate = useCallback(async () => {
    if (editingLink) {
      await handleUpdateLink();
    } else {
      await handleCreateLink();
    }
  }, [editingLink, handleUpdateLink, handleCreateLink]);

  const deleteLink = useCallback(
    async (linkId: string) => {
      deleteMutation.mutate(linkId);
    },
    [deleteMutation]
  );

  const startEditLink = useCallback(
    (link: LocalSavedRedirectLink) => {
      setEditingLink(link);
      setLinkName(link.name || '');
      setSlug(link.slug);
      setDistributionStrategy(link.distribution_strategy || 'click_limit');

      const linkGroups = link.group_priorities;
      setGroupPriorities(
        linkGroups.map((g) => ({
          ...g,
          is_favorite: allGroups.find((ag) => ag.id === g.id)?.is_favorite || false,
        }))
      );
      setSelectedGroupIds(linkGroups.map((g) => g.id));

      window.scrollTo({ top: 0, behavior: 'smooth' });

      toast({
        title: t('pages.redirectLink.success.editMode'),
        description: t('pages.redirectLink.success.editModeMessage'),
      });
    },
    [allGroups, toast, t]
  );

  const cancelEdit = useCallback(() => {
    resetForm();

    toast({
      title: t('pages.redirectLink.success.editCancelled'),
      description: t('pages.redirectLink.success.editCancelledMessage'),
    });
  }, [resetForm, toast, t]);

  const copyToClipboard = useCallback(
    (linkSlug: string) => {
      const link = `${window.location.origin}/r/${linkSlug}`;
      navigator.clipboard.writeText(link);
      toast({
        title: t('pages.redirectLink.success.copied'),
        description: t('pages.redirectLink.success.copiedMessage'),
      });
    },
    [toast, t]
  );

  return {
    slug,
    linkName,
    allGroups,
    groupPriorities,
    selectedGroupIds,
    savedLinks,
    loading,
    searchTerm,
    debouncedSearchTerm,
    dialogOpen,
    distributionStrategy,
    editingLink,
    setSlug,
    setLinkName,
    setSearchTerm,
    setDialogOpen,
    setDistributionStrategy,
    setGroupPriorities,
    handleDragEnd,
    toggleGroupSelection,
    applyGroupSelection,
    moveGroup,
    handleCreateOrUpdate,
    deleteLink,
    startEditLink,
    cancelEdit,
    copyToClipboard,
  };
}
