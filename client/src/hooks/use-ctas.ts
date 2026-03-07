import { useAuth } from '@/contexts';
import type { CTA, CTAFormState, CTAPlatform, CTATypeValue } from '@/entities/cta';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useCTAsConnect } from './use-ctas-connect';

/**
 * Hook that manages all CTAs page logic.
 * Handles CRUD operations, form state, filtering, and preview.
 * Uses ConnectRPC for all data operations.
 */
export function useCTAs() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();

  const {
    ctas,
    loading,
    refetch,
    createCTA,
    updateCTA,
    deleteCTA,
    isCreatingCTA,
    isUpdatingCTA,
    isDeletingCTA,
  } = useCTAsConnect();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<CTA | null>(null);
  const [form, setForm] = useState<CTAFormState>({
    message: '',
    platform: 'all',
    cta_type: 'all',
    is_active: true,
  });

  const [deleteTarget, setDeleteTarget] = useState<CTA | null>(null);

  // Filters
  const [filterPlatform, setFilterPlatform] = useState<CTAPlatform | 'all'>('all');
  const [filterType, setFilterType] = useState<CTATypeValue | 'all'>('all');
  const [filterActive, setFilterActive] = useState<'all' | 'active' | 'inactive'>('all');

  const filteredItems = useMemo(() => {
    return ctas.filter((cta) => {
      if (filterPlatform !== 'all' && cta.platform !== filterPlatform) return false;
      if (filterType !== 'all' && cta.cta_type !== filterType) return false;
      if (filterActive === 'active' && !cta.is_active) return false;
      if (filterActive === 'inactive' && cta.is_active) return false;
      return true;
    });
  }, [ctas, filterPlatform, filterType, filterActive]);

  const preview = useMemo(() => {
    return form.message || '';
  }, [form.message]);

  const openCreate = useCallback(() => {
    setEditing(null);
    setForm({
      message: '',
      platform: 'all',
      cta_type: 'all',
      is_active: true,
    });
    setDialogOpen(true);
  }, []);

  const openEdit = useCallback((item: CTA) => {
    setEditing(item);
    setForm({
      message: item.message,
      platform: item.platform,
      cta_type: item.cta_type,
      is_active: item.is_active,
    });
    setDialogOpen(true);
  }, []);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setEditing(null);
  }, []);

  const save = useCallback(async () => {
    if (!user) return;
    if (!form.message.trim()) {
      toast({
        title: t('pages.ctas.errors.messageRequired'),
        variant: 'destructive',
      });
      return;
    }
    try {
      if (editing) {
        await updateCTA({
          id: editing.id,
          message: form.message.trim(),
          platform: form.platform,
          cta_type: form.cta_type,
          is_active: form.is_active,
        });
      } else {
        await createCTA({
          message: form.message.trim(),
          platform: form.platform,
          cta_type: form.cta_type,
        });
      }
      closeDialog();
    } catch (error: unknown) {
      captureHookError(error, 'useCTAs', 'save');
    }
  }, [user, form, editing, closeDialog, createCTA, updateCTA, t, toast]);

  const requestDelete = useCallback((item: CTA) => {
    setDeleteTarget(item);
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!user || !deleteTarget) return;
    try {
      await deleteCTA(deleteTarget.id);
    } catch (error: unknown) {
      captureHookError(error, 'useCTAs', 'confirmDelete');
    } finally {
      setDeleteTarget(null);
    }
  }, [user, deleteTarget, deleteCTA]);

  const updateFormField = useCallback(
    <K extends keyof CTAFormState>(field: K, value: CTAFormState[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }));
    },
    []
  );

  const toggleActive = useCallback(
    async (cta: CTA) => {
      if (!user) return;
      try {
        await updateCTA({ id: cta.id, is_active: !cta.is_active });
      } catch (error: unknown) {
        captureHookError(error, 'useCTAs', 'toggleActive');
      }
    },
    [user, updateCTA]
  );

  const saving = isCreatingCTA || isUpdatingCTA;
  const deleting = isDeletingCTA;

  return {
    // State
    loading,
    items: filteredItems,
    allItems: ctas,
    dialogOpen,
    editing,
    form,
    deleteTarget,
    preview,
    saving,
    deleting,

    // Filters
    filterPlatform,
    filterType,
    filterActive,
    setFilterPlatform,
    setFilterType,
    setFilterActive,

    // Actions
    setDialogOpen,
    openCreate,
    openEdit,
    closeDialog,
    save,
    requestDelete,
    cancelDelete,
    confirmDelete,
    updateFormField,
    toggleActive,
    refetch,
  };
}
