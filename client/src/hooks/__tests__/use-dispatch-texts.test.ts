import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (inline values to avoid TDZ issues with vi.mock hoisting)
// ============================================================

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

const mockToast = vi.fn();
vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

const mockT = (key: string) => key;
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
    i18n: { language: 'pt-BR' },
  }),
}));

const mockUser = { id: 'user-123', email: 'test@example.com' };
let mockUserValue: typeof mockUser | null = mockUser;

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: mockUserValue,
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock ConnectRPC hooks
// ============================================================

const mockCreateDispatchText = vi.fn();
const mockUpdateDispatchText = vi.fn();
const mockDeleteDispatchText = vi.fn();
const mockRefetch = vi.fn();

vi.mock('@/hooks/use-dispatch-texts-connect', () => ({
  useDispatchTextsConnect: () => ({
    dispatchTexts: [
      {
        id: 'dt-1',
        user_id: 'user-123',
        name: 'Template 1',
        body: 'Hello {nome_produto}',
        is_active: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ],
    loading: false,
    refetch: mockRefetch,
    createDispatchText: mockCreateDispatchText,
    updateDispatchText: mockUpdateDispatchText,
    deleteDispatchText: mockDeleteDispatchText,
    isCreatingDispatchText: false,
    isUpdatingDispatchText: false,
    isDeletingDispatchText: false,
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { captureHookError } from '@/lib/sentry';
import { useDispatchTexts } from '../use-dispatch-texts';
import type { DispatchText } from '@/entities/dispatch-text';

// ============================================================
// Test Helpers
// ============================================================

const sampleDispatchText: DispatchText = {
  id: 'dt-1',
  user_id: 'user-123',
  name: 'Template 1',
  body: 'Hello {nome_produto}',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function renderAndGet() {
  const { result } = renderHook(() => useDispatchTexts());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useDispatchTexts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    mockCreateDispatchText.mockResolvedValue({
      id: 'dt-new',
      user_id: 'user-123',
      name: 'New',
      body: 'Body',
      is_active: true,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    });
    mockUpdateDispatchText.mockResolvedValue({
      ...sampleDispatchText,
      name: 'Updated',
    });
    mockDeleteDispatchText.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Initial state ───────────────────────────────────────────

  describe('initial state', () => {
    it('should return loading from connect hook', () => {
      const result = renderAndGet();
      expect(result.current.loading).toBe(false);
    });

    it('should return items from connect hook', () => {
      const result = renderAndGet();
      expect(result.current.items).toHaveLength(1);
      expect(result.current.items[0].name).toBe('Template 1');
    });

    it('should have dialogOpen = false initially', () => {
      const result = renderAndGet();
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should have editing = null initially', () => {
      const result = renderAndGet();
      expect(result.current.editing).toBeNull();
    });

    it('should have deleteTarget = null initially', () => {
      const result = renderAndGet();
      expect(result.current.deleteTarget).toBeNull();
    });

    it('should have 8 variable chips', () => {
      const result = renderAndGet();
      expect(result.current.variableChips).toHaveLength(8);
    });

    it('should have saving = false initially', () => {
      const result = renderAndGet();
      expect(result.current.saving).toBe(false);
    });

    it('should have deleting = false initially', () => {
      const result = renderAndGet();
      expect(result.current.deleting).toBe(false);
    });

    it('should compute preview from form body and sampleVars', () => {
      const result = renderAndGet();
      // Default form body is empty, so preview should be empty
      expect(typeof result.current.preview).toBe('string');
    });
  });

  // ── openCreate ──────────────────────────────────────────────

  describe('openCreate', () => {
    it('should open dialog', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());

      expect(result.current.dialogOpen).toBe(true);
    });

    it('should reset editing to null', () => {
      const result = renderAndGet();

      // First set editing
      act(() => result.current.openEdit(sampleDispatchText));
      expect(result.current.editing).not.toBeNull();

      // Then openCreate should clear it
      act(() => result.current.openCreate());
      expect(result.current.editing).toBeNull();
    });

    it('should set form with default template and is_active=true', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());

      expect(result.current.form.is_active).toBe(true);
      expect(result.current.form.name).toBe('');
      // body should be set to default template (the t() key)
      expect(result.current.form.body).toBe('pages.dispatchTexts.defaultTemplate');
    });
  });

  // ── openEdit ────────────────────────────────────────────────

  describe('openEdit', () => {
    it('should populate form with item data', () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));

      expect(result.current.form.name).toBe('Template 1');
      expect(result.current.form.body).toBe('Hello {nome_produto}');
      expect(result.current.form.is_active).toBe(true);
    });

    it('should set editing to the item', () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));

      expect(result.current.editing).toEqual(sampleDispatchText);
    });

    it('should open dialog', () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));

      expect(result.current.dialogOpen).toBe(true);
    });
  });

  // ── closeDialog ─────────────────────────────────────────────

  describe('closeDialog', () => {
    it('should close dialog and clear editing', () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));
      expect(result.current.dialogOpen).toBe(true);

      act(() => result.current.closeDialog());
      expect(result.current.dialogOpen).toBe(false);
      expect(result.current.editing).toBeNull();
    });
  });

  // ── save (create) ──────────────────────────────────────────

  describe('save - create', () => {
    it('should call createDispatchText with trimmed name', async () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', '  New Template  ');
        result.current.updateFormField('body', 'Hello World');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(mockCreateDispatchText).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'New Template',
          body: 'Hello World',
          is_active: true,
        })
      );
    });

    it('should close dialog on successful create', async () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', 'New Template');
        result.current.updateFormField('body', 'Body');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.dialogOpen).toBe(false);
    });

    it('should not call createDispatchText without user', async () => {
      mockUserValue = null;
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', 'New');
        result.current.updateFormField('body', 'Body');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(mockCreateDispatchText).not.toHaveBeenCalled();
    });

    it('should show toast when name is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', '');
        result.current.updateFormField('body', 'Body');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
      expect(mockCreateDispatchText).not.toHaveBeenCalled();
    });

    it('should show toast when body is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', 'Name');
        result.current.updateFormField('body', '   ');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
      expect(mockCreateDispatchText).not.toHaveBeenCalled();
    });

    it('should capture error in Sentry on create failure', async () => {
      const error = new Error('Create failed');
      mockCreateDispatchText.mockRejectedValue(error);

      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => {
        result.current.updateFormField('name', 'New');
        result.current.updateFormField('body', 'Body');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(captureHookError).toHaveBeenCalledWith(error, 'useDispatchTexts', 'save');
    });
  });

  // ── save (update) ──────────────────────────────────────────

  describe('save - update', () => {
    it('should call updateDispatchText with editing id', async () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));
      act(() => {
        result.current.updateFormField('name', 'Updated Name');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(mockUpdateDispatchText).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'dt-1',
          name: 'Updated Name',
        })
      );
    });

    it('should close dialog on successful update', async () => {
      const result = renderAndGet();

      act(() => result.current.openEdit(sampleDispatchText));

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.dialogOpen).toBe(false);
    });
  });

  // ── Delete flow ─────────────────────────────────────────────

  describe('delete flow', () => {
    it('should set deleteTarget on requestDelete', () => {
      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));

      expect(result.current.deleteTarget).toEqual(sampleDispatchText);
    });

    it('should clear deleteTarget on cancelDelete', () => {
      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));
      act(() => result.current.cancelDelete());

      expect(result.current.deleteTarget).toBeNull();
    });

    it('should call deleteDispatchText on confirmDelete', async () => {
      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(mockDeleteDispatchText).toHaveBeenCalledWith('dt-1');
    });

    it('should clear deleteTarget in finally block', async () => {
      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(result.current.deleteTarget).toBeNull();
    });

    it('should not call deleteDispatchText without user', async () => {
      mockUserValue = null;
      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(mockDeleteDispatchText).not.toHaveBeenCalled();
    });

    it('should capture error in Sentry on delete failure', async () => {
      const error = new Error('Delete failed');
      mockDeleteDispatchText.mockRejectedValue(error);

      const result = renderAndGet();

      act(() => result.current.requestDelete(sampleDispatchText));

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(captureHookError).toHaveBeenCalledWith(
        error,
        'useDispatchTexts',
        'confirmDelete'
      );
    });
  });

  // ── updateFormField ─────────────────────────────────────────

  describe('updateFormField', () => {
    it('should update a specific form field', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => result.current.updateFormField('name', 'Updated'));

      expect(result.current.form.name).toBe('Updated');
    });
  });

  // ── insertAtCursor ──────────────────────────────────────────

  describe('insertAtCursor', () => {
    it('should append with newline when body is not empty', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => result.current.updateFormField('body', 'existing'));
      act(() => result.current.insertAtCursor('{nome_produto}'));

      expect(result.current.form.body).toBe('existing\n{nome_produto}');
    });

    it('should append without newline when body is empty', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => result.current.updateFormField('body', ''));
      act(() => result.current.insertAtCursor('{nome_produto}'));

      expect(result.current.form.body).toBe('{nome_produto}');
    });
  });

  // ── copyToken ───────────────────────────────────────────────

  describe('copyToken', () => {
    it('should call clipboard.writeText and show toast', async () => {
      const writeTextMock = vi.fn().mockResolvedValue(undefined);
      Object.assign(navigator, {
        clipboard: { writeText: writeTextMock },
      });

      const result = renderAndGet();

      await act(async () => {
        await result.current.copyToken('{nome_produto}');
      });

      expect(writeTextMock).toHaveBeenCalledWith('{nome_produto}');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.dispatchTexts.success.copied',
          description: '{nome_produto}',
        })
      );
    });

    it('should capture error on clipboard failure', async () => {
      const clipboardError = new Error('Clipboard failed');
      Object.assign(navigator, {
        clipboard: { writeText: vi.fn().mockRejectedValue(clipboardError) },
      });

      const result = renderAndGet();

      await act(async () => {
        await result.current.copyToken('{token}');
      });

      expect(captureHookError).toHaveBeenCalledWith(
        clipboardError,
        'useDispatchTexts',
        'copyToken'
      );
    });
  });

  // ── preview ─────────────────────────────────────────────────

  describe('preview', () => {
    it('should render template with sample variables', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => result.current.updateFormField('body', 'Produto: {nome_produto}'));

      // preview uses sampleVars which are i18n keys (mockT returns the key)
      expect(result.current.preview).toContain('pages.dispatchTexts.sampleVars.productName');
    });

    it('should update when body changes', () => {
      const result = renderAndGet();

      act(() => result.current.openCreate());
      act(() => result.current.updateFormField('body', '{preco_formatado}'));

      const preview1 = result.current.preview;

      act(() => result.current.updateFormField('body', '{link_afiliado}'));

      const preview2 = result.current.preview;
      expect(preview1).not.toBe(preview2);
    });
  });
});
