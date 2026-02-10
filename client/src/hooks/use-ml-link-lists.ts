import { useAuth } from '@/contexts';
import type { ListForm, MlLinkList, MlLinkListItem, PreviewState } from '@/entities/ml-link-list';
import { useMercadoLivreConnect } from '@/hooks/use-mercadolivre-connect';
import { useMlLinksConnect } from '@/hooks/use-ml-links-connect';
import { useToast } from '@/hooks/use-toast';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Normalizes a URL by removing the hash fragment.
 */
function normalizeUrl(raw: string): string {
  const trimmed = String(raw || '').trim();
  const u = new URL(trimmed);
  u.hash = '';
  return u.toString();
}

/**
 * Checks if a URL is a valid Mercado Livre product URL.
 */
function isMercadoLivreUrl(raw: string): boolean {
  try {
    const u = new URL(String(raw || '').trim());
    const host = u.hostname.toLowerCase();
    if (!host.includes('mercadolivre.')) return false;
    const path = u.pathname.toLowerCase();
    return path.includes('/p/') || path.includes('mlb');
  } catch {
    return false;
  }
}

interface UseMlLinkListsReturn {
  loadingLists: boolean;
  loadingItems: boolean;
  loadingMoreItems: boolean;
  lists: MlLinkList[];
  selectedListId: string;
  items: MlLinkListItem[];
  listDialogOpen: boolean;
  editingList: MlLinkList | null;
  listForm: ListForm;
  singleUrl: string;
  bulkText: string;
  importing: boolean;
  lastPreview: PreviewState | null;
  selectedList: MlLinkList | null;
  availableCount: number;
  totalItemCount: number;
  hasMoreItems: boolean;
  deleteListTarget: MlLinkList | null;
  deleteItemTarget: MlLinkListItem | null;
  resetConsumedDialogOpen: boolean;
  setSelectedListId: (id: string) => void;
  setListDialogOpen: (open: boolean) => void;
  setListForm: React.Dispatch<React.SetStateAction<ListForm>>;
  setSingleUrl: (url: string) => void;
  setBulkText: (text: string) => void;
  loadLists: () => Promise<void>;
  loadMoreItems: () => Promise<void>;
  openCreateList: () => void;
  openEditList: (list: MlLinkList) => void;
  saveList: () => Promise<void>;
  requestDeleteList: (list: MlLinkList) => void;
  cancelDeleteList: () => void;
  confirmDeleteList: () => Promise<void>;
  requestDeleteItem: (item: MlLinkListItem) => void;
  cancelDeleteItem: () => void;
  confirmDeleteItem: () => Promise<void>;
  requestResetConsumed: () => void;
  cancelResetConsumed: () => void;
  confirmResetConsumed: () => Promise<void>;
  copyToClipboard: (text: string) => Promise<void>;
  addOneLink: () => Promise<void>;
  importBulk: () => Promise<void>;
  toggleItemActive: (item: MlLinkListItem) => Promise<void>;
  moveItem: (item: MlLinkListItem, dir: 'up' | 'down') => Promise<void>;
}

/**
 * Converts ConnectRPC local type to entity type for MlLinkList.
 */
function toEntityMlLinkList(local: import('./use-ml-links-connect').LocalMlLinkList): MlLinkList {
  return {
    id: local.id,
    user_id: local.user_id,
    name: local.name,
    description: local.description ?? null,
    is_active: local.is_active,
    created_at: local.created_at,
    updated_at: local.updated_at,
  };
}

/**
 * Converts ConnectRPC local type to entity type for MlLinkListItem.
 */
function toEntityMlLinkListItem(
  local: import('./use-ml-links-connect').LocalMlLinkListItem
): MlLinkListItem {
  let metadata: MlLinkListItem['metadata'] = null;
  if (local.metadata) {
    try {
      metadata = JSON.parse(local.metadata) as MlLinkListItem['metadata'];
    } catch {
      metadata = null;
    }
  }

  return {
    id: local.id,
    list_id: local.list_id,
    origin_url: local.origin_url,
    order_index: local.order_index,
    is_active: local.is_active,
    consumed_at: local.consumed_at ?? null,
    product_title: local.product_title ?? null,
    product_price: local.product_price ?? null,
    product_image_url: local.product_image_url ?? null,
    metadata,
    created_at: local.created_at,
    updated_at: local.updated_at,
  };
}

/**
 * Hook for ML Link Lists page logic.
 * Manages Mercado Livre link lists with items and confirmation dialogs.
 * Uses ConnectRPC for all backend operations.
 */
export function useMlLinkLists(): UseMlLinkListsReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const { preview: mlPreview } = useMercadoLivreConnect();

  // ConnectRPC hook for all ML Links operations
  const mlLinksConnect = useMlLinksConnect();

  const [loadingItems, setLoadingItems] = useState(false);
  const [loadingMoreItems, setLoadingMoreItems] = useState(false);
  const [selectedListId, setSelectedListId] = useState<string>('');
  const [items, setItems] = useState<MlLinkListItem[]>([]);
  const [nextPageToken, setNextPageToken] = useState<string>('');
  const [totalItemCount, setTotalItemCount] = useState<number>(0);
  const [hasMoreItems, setHasMoreItems] = useState<boolean>(false);

  const [listDialogOpen, setListDialogOpen] = useState(false);
  const [editingList, setEditingList] = useState<MlLinkList | null>(null);
  const [listForm, setListForm] = useState<ListForm>({
    name: '',
    description: '',
    is_active: true,
  });

  const [singleUrl, setSingleUrl] = useState('');
  const [bulkText, setBulkText] = useState('');
  const [importing, setImporting] = useState(false);

  const [lastPreview, setLastPreview] = useState<PreviewState | null>(null);

  const [deleteListTarget, setDeleteListTarget] = useState<MlLinkList | null>(null);
  const [deleteItemTarget, setDeleteItemTarget] = useState<MlLinkListItem | null>(null);
  const [resetConsumedDialogOpen, setResetConsumedDialogOpen] = useState(false);

  // Map ConnectRPC lists to entity types
  const lists = useMemo(() => mlLinksConnect.lists.map(toEntityMlLinkList), [mlLinksConnect.lists]);

  const loadingLists = mlLinksConnect.loadingLists;

  const selectedList = useMemo(
    () => lists.find((l) => l.id === selectedListId) || null,
    [lists, selectedListId]
  );

  const availableCount = useMemo(() => {
    return (items || []).filter((i) => !i.consumed_at && i.is_active).length;
  }, [items]);

  const selectedListIdRef = useRef(selectedListId);
  selectedListIdRef.current = selectedListId;

  const refetchListsRef = useRef(mlLinksConnect.refetchLists);
  refetchListsRef.current = mlLinksConnect.refetchLists;

  const loadLists = useCallback(async () => {
    if (!user) return;
    await refetchListsRef.current();
  }, [user]);

  const listMlLinkListItemsPaginatedRef = useRef(mlLinksConnect.listMlLinkListItemsPaginated);
  listMlLinkListItemsPaginatedRef.current = mlLinksConnect.listMlLinkListItemsPaginated;

  const loadItems = useCallback(
    async (listId: string, _reset = true) => {
      if (!user || !listId) return;
      setLoadingItems(true);
      try {
        const result = await listMlLinkListItemsPaginatedRef.current({
          listId,
          pageSize: 50,
          pageToken: '',
        });
        setItems(result.items.map(toEntityMlLinkListItem));
        setNextPageToken(result.nextPageToken);
        setTotalItemCount(result.totalCount);
        setHasMoreItems(result.hasMore);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.loadLinksMessage');
        toast({
          title: t('pages.mlLinkLists.errors.loadLinks'),
          description: errorMessage,
          variant: 'destructive',
        });
      } finally {
        setLoadingItems(false);
      }
    },
    [user, toast, t]
  );

  const loadMoreItems = useCallback(async () => {
    if (!user || !selectedListId || !hasMoreItems || loadingMoreItems) return;
    setLoadingMoreItems(true);
    try {
      const result = await listMlLinkListItemsPaginatedRef.current({
        listId: selectedListId,
        pageSize: 50,
        pageToken: nextPageToken,
      });
      setItems((prev) => [...prev, ...result.items.map(toEntityMlLinkListItem)]);
      setNextPageToken(result.nextPageToken);
      setHasMoreItems(result.hasMore);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.loadLinksMessage');
      toast({
        title: t('pages.mlLinkLists.errors.loadLinks'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setLoadingMoreItems(false);
    }
  }, [user, selectedListId, hasMoreItems, loadingMoreItems, nextPageToken, toast, t]);

  const userIdRef = useRef(user?.id);
  userIdRef.current = user?.id;

  // Auto-select first list when lists are loaded
  useEffect(() => {
    if (!selectedListIdRef.current && lists.length > 0) {
      setSelectedListId(lists[0].id);
    }
  }, [lists]);

  useEffect(() => {
    if (selectedListId) {
      loadItems(selectedListId);
    } else {
      setItems([]);
    }
  }, [selectedListId, loadItems]);

  const openCreateList = useCallback(() => {
    setEditingList(null);
    setListForm({ name: '', description: '', is_active: true });
    setListDialogOpen(true);
  }, []);

  const openEditList = useCallback((l: MlLinkList) => {
    setEditingList(l);
    setListForm({
      name: l.name || '',
      description: l.description || '',
      is_active: l.is_active !== false,
    });
    setListDialogOpen(true);
  }, []);

  const saveList = useCallback(async () => {
    if (!user) return;
    const name = String(listForm.name || '').trim();
    if (!name) {
      toast({
        title: t('pages.mlLinkLists.errors.nameRequired'),
        description: t('pages.mlLinkLists.errors.nameRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }
    try {
      if (editingList) {
        await mlLinksConnect.updateMlLinkList({
          id: editingList.id,
          name,
          description: String(listForm.description || '').trim() || undefined,
          is_active: listForm.is_active,
        });
      } else {
        const newList = await mlLinksConnect.createMlLinkList({
          name,
          description: String(listForm.description || '').trim() || undefined,
          is_active: listForm.is_active,
        });
        if (newList?.id) setSelectedListId(newList.id);
      }
      setListDialogOpen(false);
      setEditingList(null);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.saveMessage');
      toast({
        title: t('pages.mlLinkLists.errors.save'),
        description: errorMessage,
        variant: 'destructive',
      });
    }
  }, [user, listForm, editingList, toast, t, mlLinksConnect]);

  const requestDeleteList = useCallback((list: MlLinkList) => {
    setDeleteListTarget(list);
  }, []);

  const cancelDeleteList = useCallback(() => {
    setDeleteListTarget(null);
  }, []);

  const confirmDeleteList = useCallback(async () => {
    if (!user || !deleteListTarget) return;
    try {
      await mlLinksConnect.deleteMlLinkList(deleteListTarget.id);
      if (selectedListId === deleteListTarget.id) setSelectedListId('');
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.deleteMessage');
      toast({
        title: t('pages.mlLinkLists.errors.delete'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setDeleteListTarget(null);
    }
  }, [user, deleteListTarget, selectedListId, toast, t, mlLinksConnect]);

  const requestDeleteItem = useCallback((item: MlLinkListItem) => {
    setDeleteItemTarget(item);
  }, []);

  const cancelDeleteItem = useCallback(() => {
    setDeleteItemTarget(null);
  }, []);

  const confirmDeleteItem = useCallback(async () => {
    if (!deleteItemTarget) return;
    try {
      await mlLinksConnect.deleteMlLinkListItem(deleteItemTarget.id);
      loadItems(deleteItemTarget.list_id);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.removeMessage');
      toast({
        title: t('pages.mlLinkLists.errors.remove'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setDeleteItemTarget(null);
    }
  }, [deleteItemTarget, toast, t, loadItems, mlLinksConnect]);

  const requestResetConsumed = useCallback(() => {
    setResetConsumedDialogOpen(true);
  }, []);

  const cancelResetConsumed = useCallback(() => {
    setResetConsumedDialogOpen(false);
  }, []);

  const confirmResetConsumed = useCallback(async () => {
    if (!selectedListId) return;
    try {
      await mlLinksConnect.resetConsumedItems(selectedListId);
      loadItems(selectedListId);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.resetMessage');
      toast({
        title: t('pages.mlLinkLists.errors.reset'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setResetConsumedDialogOpen(false);
    }
  }, [selectedListId, toast, t, loadItems, mlLinksConnect]);

  const copyToClipboard = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        toast({ title: t('pages.mlLinkLists.success.copied') });
      } catch {
        // ignore
      }
    },
    [toast, t]
  );

  const addOneLink = useCallback(async () => {
    if (!user || !selectedListId) return;
    const raw = String(singleUrl || '').trim();
    if (!raw) return;
    if (!isMercadoLivreUrl(raw)) {
      toast({
        title: t('pages.mlLinkLists.errors.invalidLink'),
        description: t('pages.mlLinkLists.errors.invalidLinkMessage'),
        variant: 'destructive',
      });
      return;
    }

    let cleanUrl = raw;
    try {
      cleanUrl = normalizeUrl(raw);
    } catch {
      // keep raw
    }

    let preview: PreviewState | null = null;
    try {
      const previewData = await mlPreview(cleanUrl);
      if (previewData) {
        preview = {
          url: cleanUrl,
          title: previewData.title ?? null,
          price: previewData.price ?? null,
          thumbnail: previewData.imageUrl ?? null,
        };
        setLastPreview(preview);
      }
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.previewUnavailableMessage');
      toast({
        title: t('pages.mlLinkLists.errors.previewUnavailable'),
        description: errorMessage,
        variant: 'destructive',
      });
    }

    try {
      const nextOrder =
        items.length > 0 ? Math.max(...items.map((i) => i.order_index || 0)) + 1 : 0;

      const metadata = preview
        ? JSON.stringify({
            preview: { title: preview.title, price: preview.price, thumbnail: preview.thumbnail },
          })
        : undefined;

      await mlLinksConnect.createMlLinkListItem({
        list_id: selectedListId,
        origin_url: cleanUrl,
        order_index: nextOrder,
        is_active: true,
        product_title: preview?.title ?? undefined,
        product_price: preview?.price ?? undefined,
        product_image_url: preview?.thumbnail ?? undefined,
        metadata,
      });
      setSingleUrl('');
      loadItems(selectedListId);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.addLinkMessage');
      toast({
        title: t('pages.mlLinkLists.errors.addLink'),
        description: errorMessage,
        variant: 'destructive',
      });
    }
  }, [user, selectedListId, singleUrl, items, toast, t, loadItems, mlPreview, mlLinksConnect]);

  const importBulk = useCallback(async () => {
    if (!user || !selectedListId) return;
    const lines = String(bulkText || '')
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter(Boolean);
    if (lines.length === 0) return;

    const normalized = lines.filter(isMercadoLivreUrl).map((l) => {
      try {
        return normalizeUrl(l);
      } catch {
        return l;
      }
    });

    if (normalized.length === 0) {
      toast({
        title: t('pages.mlLinkLists.errors.noValidLinks'),
        description: t('pages.mlLinkLists.errors.noValidLinksMessage'),
        variant: 'destructive',
      });
      return;
    }

    setImporting(true);
    try {
      const baseOrder =
        items.length > 0 ? Math.max(...items.map((i) => i.order_index || 0)) + 1 : 0;
      const uniq = Array.from(new Set(normalized));

      await mlLinksConnect.bulkCreateMlLinkListItems(selectedListId, uniq, baseOrder);
      setBulkText('');
      loadItems(selectedListId);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.importMessage');
      toast({
        title: t('pages.mlLinkLists.errors.import'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setImporting(false);
    }
  }, [user, selectedListId, bulkText, items, toast, t, loadItems, mlLinksConnect]);

  const toggleItemActive = useCallback(
    async (it: MlLinkListItem) => {
      try {
        await mlLinksConnect.updateMlLinkListItem({
          id: it.id,
          is_active: !it.is_active,
        });
        loadItems(it.list_id);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.toggleMessage');
        toast({
          title: t('pages.mlLinkLists.errors.toggle'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    },
    [toast, t, loadItems, mlLinksConnect]
  );

  const moveItem = useCallback(
    async (it: MlLinkListItem, dir: 'up' | 'down') => {
      const idx = items.findIndex((x) => x.id === it.id);
      const swapIdx = dir === 'up' ? idx - 1 : idx + 1;
      if (idx < 0 || swapIdx < 0 || swapIdx >= items.length) return;
      const other = items[swapIdx];
      try {
        const a = it.order_index;
        const b = other.order_index;

        // Use reorderItems RPC with a map of swapped order indices
        await mlLinksConnect.reorderItems(it.list_id, {
          [it.id]: b,
          [other.id]: a,
        });
        loadItems(it.list_id);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.reorderMessage');
        toast({
          title: t('pages.mlLinkLists.errors.reorder'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    },
    [items, toast, t, loadItems, mlLinksConnect]
  );

  return {
    loadingLists,
    loadingItems,
    loadingMoreItems,
    lists,
    selectedListId,
    items,
    listDialogOpen,
    editingList,
    listForm,
    singleUrl,
    bulkText,
    importing,
    lastPreview,
    selectedList,
    availableCount,
    totalItemCount,
    hasMoreItems,
    deleteListTarget,
    deleteItemTarget,
    resetConsumedDialogOpen,
    setSelectedListId,
    setListDialogOpen,
    setListForm,
    setSingleUrl,
    setBulkText,
    loadLists,
    loadMoreItems,
    openCreateList,
    openEditList,
    saveList,
    requestDeleteList,
    cancelDeleteList,
    confirmDeleteList,
    requestDeleteItem,
    cancelDeleteItem,
    confirmDeleteItem,
    requestResetConsumed,
    cancelResetConsumed,
    confirmResetConsumed,
    copyToClipboard,
    addOneLink,
    importBulk,
    toggleItemActive,
    moveItem,
  };
}
