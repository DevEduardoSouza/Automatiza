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
    body: '',
    is_active: true,
  });

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
    }),
    [t]
  );

  const preview = useMemo(() => {
    return renderTemplate(form.body, sampleVars);
  }, [form.body, sampleVars]);

  // items is an alias for dispatchTexts to maintain backward compatibility
  const items = dispatchTexts as DispatchText[];

  const openCreate = useCallback(() => {
    setEditing(null);
    setForm({
      name: '',
      body: t('pages.dispatchTexts.defaultTemplate'),
      is_active: true,
    });
    setDialogOpen(true);
  }, [t]);

  const openEdit = useCallback((item: DispatchText) => {
    setEditing(item);
    setForm({
      name: item.name || '',
      body: item.body || '',
      is_active: item.is_active !== false,
    });
    setDialogOpen(true);
  }, []);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    setEditing(null);
  }, []);

  const save = useCallback(async () => {
    if (!user) return;
    if (!form.name.trim() || !form.body.trim()) {
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
          body: form.body,
          is_active: form.is_active,
        });
      } else {
        await createDispatchText({
          name: form.name.trim(),
          body: form.body,
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

  const insertAtCursor = useCallback((token: string) => {
    setForm((prev) => ({
      ...prev,
      body: `${prev.body}${prev.body ? '\n' : ''}${token}`,
    }));
  }, []);

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
  };
}
