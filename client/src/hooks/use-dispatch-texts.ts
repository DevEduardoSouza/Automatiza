import { useAuth } from '@/contexts';
import {
  type DispatchText,
  type DispatchTextFormState,
  VARIABLE_KEYS,
  VARIABLE_MAP,
  renderTemplate,
} from '@/entities/dispatch-text';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatchTextsConnect } from './use-dispatch-texts-connect';

/**
 * Hook that manages all dispatch texts page logic.
 * Handles CRUD operations, form state, and preview rendering.
 * Uses ConnectRPC for all data operations.
 */
export function useDispatchTexts() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();

  const {
    dispatchTexts,
    loading,
    refetch,
    createDispatchText,
    updateDispatchText,
    deleteDispatchText,
    isCreatingDispatchText,
    isUpdatingDispatchText,
    isDeletingDispatchText,
  } = useDispatchTextsConnect();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<DispatchText | null>(null);
  const [form, setForm] = useState<DispatchTextFormState>({
    name: '',
    copies: [''],
    is_active: true,
  });

  const [activeCopyIndex, setActiveCopyIndex] = useState(0);

  const [deleteTarget, setDeleteTarget] = useState<DispatchText | null>(null);

  const variableChips = useMemo(
    () =>
      VARIABLE_KEYS.map((key) => ({
        key: VARIABLE_MAP[key],
        label: t(`pages.dispatchTexts.variables.${key}`),
      })),
    [t]
  );

  const sampleVars = useMemo(
    () => ({
      nome_produto: t('pages.dispatchTexts.sampleVars.productName'),
      preco: t('pages.dispatchTexts.sampleVars.price'),
      preco_formatado: t('pages.dispatchTexts.sampleVars.priceFormatted'),
      preco_original: t('pages.dispatchTexts.sampleVars.originalPrice'),
      preco_original_formatado: t('pages.dispatchTexts.sampleVars.originalPriceFormatted'),
      link_afiliado: t('pages.dispatchTexts.sampleVars.affiliateLink'),
      aviso_preco: t('pages.dispatchTexts.sampleVars.priceWarning'),
      cta: t('pages.dispatchTexts.sampleVars.cta'),
      cupom_bloco: t('pages.dispatchTexts.sampleVars.couponBlock'),
    }),
    [t]
  );

  const preview = useMemo(() => {
    const currentCopy = form.copies[activeCopyIndex] ?? '';
    return renderTemplate(currentCopy, sampleVars);
  }, [form.copies, activeCopyIndex, sampleVars]);

  // items is an alias for dispatchTexts to maintain backward compatibility
  const items = dispatchTexts as DispatchText[];

  const openCreate = useCallback(() => {
    setEditing(null);
    setForm({
      name: '',
      copies: [t('pages.dispatchTexts.defaultTemplate')],
      is_active: true,
    });
    setActiveCopyIndex(0);
    setDialogOpen(true);
  }, [t]);

  const openEdit = useCallback((item: DispatchText) => {
    setEditing(item);
    const copies = item.copies?.length > 0 ? [...item.copies] : item.body ? [item.body] : [''];
    setForm({
      name: item.name || '',
      copies,
      is_active: item.is_active !== false,
    });
    setActiveCopyIndex(0);
    setDialogOpen(true);
  }, []);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setEditing(null);
  }, []);

  const save = useCallback(async () => {
    if (!user) return;
    const nonEmptyCopies = form.copies.filter((c) => c.trim() !== '');
    if (!form.name.trim() || nonEmptyCopies.length === 0) {
      toast({
        title: t('pages.dispatchTexts.errors.requiredFields'),
        description: t('pages.dispatchTexts.errors.requiredFieldsMessage'),
        variant: 'destructive',
      });
      return;
    }
    try {
      if (editing) {
        await updateDispatchText({
          id: editing.id,
          name: form.name.trim(),
          copies: nonEmptyCopies,
          is_active: form.is_active,
        });
      } else {
        await createDispatchText({
          name: form.name.trim(),
          copies: nonEmptyCopies,
          is_active: form.is_active,
        });
      }
      closeDialog();
    } catch (error: unknown) {
      captureHookError(error, 'useDispatchTexts', 'save');
    }
  }, [user, form, editing, closeDialog, createDispatchText, updateDispatchText, t, toast]);

  const requestDelete = useCallback((item: DispatchText) => {
    setDeleteTarget(item);
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!user || !deleteTarget) return;
    try {
      await deleteDispatchText(deleteTarget.id);
    } catch (error: unknown) {
      captureHookError(error, 'useDispatchTexts', 'confirmDelete');
    } finally {
      setDeleteTarget(null);
    }
  }, [user, deleteTarget, deleteDispatchText]);

  const updateFormField = useCallback(
    <K extends keyof DispatchTextFormState>(field: K, value: DispatchTextFormState[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }));
    },
    []
  );

  const insertAtCursor = useCallback(
    (token: string) => {
      setForm((prev) => {
        const newCopies = [...prev.copies];
        const current = newCopies[activeCopyIndex] ?? '';
        newCopies[activeCopyIndex] = `${current}${current ? '\n' : ''}${token}`;
        return { ...prev, copies: newCopies };
      });
    },
    [activeCopyIndex]
  );

  const copyToken = useCallback(
    async (token: string) => {
      try {
        await navigator.clipboard.writeText(token);
        toast({
          title: t('pages.dispatchTexts.success.copied'),
          description: token,
        });
      } catch (error: unknown) {
        captureHookError(error, 'useDispatchTexts', 'copyToken');
      }
    },
    [toast, t]
  );

  const addCopyVariation = useCallback(() => {
    setForm((prev) => {
      if (prev.copies.length >= 10) return prev;
      const newCopies = [...prev.copies, ''];
      return { ...prev, copies: newCopies };
    });
    setActiveCopyIndex((prev) => prev + 1);
  }, []);

  const removeCopyVariation = useCallback(
    (index: number) => {
      setForm((prev) => {
        if (prev.copies.length <= 1) return prev;
        const newCopies = prev.copies.filter((_, i) => i !== index);
        return { ...prev, copies: newCopies };
      });
      setActiveCopyIndex((prev) => Math.min(prev, form.copies.length - 2));
    },
    [form.copies.length]
  );

  const updateCopyAtIndex = useCallback((index: number, value: string) => {
    setForm((prev) => {
      const newCopies = [...prev.copies];
      newCopies[index] = value;
      return { ...prev, copies: newCopies };
    });
  }, []);

  // Derive saving state from mutations
  const saving = isCreatingDispatchText || isUpdatingDispatchText;
  const deleting = isDeletingDispatchText;

  return {
    // State
    loading,
    items,
    dialogOpen,
    editing,
    form,
    deleteTarget,
    variableChips,
    preview,
    saving,
    deleting,
    activeCopyIndex,

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
    insertAtCursor,
    copyToken,
    refetch,
    setActiveCopyIndex,
    addCopyVariation,
    removeCopyVariation,
    updateCopyAtIndex,
  };
}
