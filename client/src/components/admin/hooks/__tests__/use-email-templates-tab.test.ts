import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('@/contexts', () => ({
  useAuth: () => ({ user: { id: 'user-1' } }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (e: unknown) => ({
    userMessage: e instanceof Error ? e.message : 'error',
    code: 'INTERNAL',
  }),
}));

vi.mock('@/gen/admin/v1/admin_pb', () => ({
  CreateEmailTemplateRequestSchema: 'CreateEmailTemplateRequestSchema',
  UpdateEmailTemplateRequestSchema: 'UpdateEmailTemplateRequestSchema',
  ListEmailTemplatesRequestSchema: 'ListEmailTemplatesRequestSchema',
  ToggleEmailTemplateActiveRequestSchema: 'ToggleEmailTemplateActiveRequestSchema',
}));

vi.mock('@/gen/admin/v1/admin-AdminService_connectquery', () => ({
  listEmailTemplates: 'listEmailTemplates',
  createEmailTemplate: 'createEmailTemplate',
  updateEmailTemplate: 'updateEmailTemplate',
  toggleEmailTemplateActive: 'toggleEmailTemplateActive',
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

const mockProtoTemplate = {
  id: 'tpl-1',
  name: 'welcome_email',
  subject: 'Welcome!',
  body: '<h1>Hello</h1>',
  isActive: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  description: 'Welcome template',
};

const mockProtoTemplate2 = {
  id: 'tpl-2',
  name: 'password_reset',
  subject: 'Reset your password',
  body: '<h1>Reset</h1>',
  isActive: false,
  createdAt: '2026-01-02T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
  description: undefined,
};

let mockTemplatesResponse: { templates: (typeof mockProtoTemplate)[] } | undefined = {
  templates: [mockProtoTemplate, mockProtoTemplate2],
};

const mockCreateMutateAsync = vi.fn();
const mockUpdateMutateAsync = vi.fn();
const mockToggleMutateAsync = vi.fn();

let createOnSuccess: (() => void) | undefined;
let updateOnSuccess: (() => void) | undefined;
let toggleOnSuccess: ((response: { template?: { isActive: boolean } }) => void) | undefined;

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: () => ({ data: mockTemplatesResponse, isLoading: false }),
  useMutation: (_mutationFn: unknown, opts?: Record<string, unknown>) => {
    if (_mutationFn === 'createEmailTemplate') {
      createOnSuccess = opts?.onSuccess as typeof createOnSuccess;
      return { mutateAsync: mockCreateMutateAsync, isPending: false };
    }
    if (_mutationFn === 'updateEmailTemplate') {
      updateOnSuccess = opts?.onSuccess as typeof updateOnSuccess;
      return { mutateAsync: mockUpdateMutateAsync, isPending: false };
    }
    if (_mutationFn === 'toggleEmailTemplateActive') {
      toggleOnSuccess = opts?.onSuccess as typeof toggleOnSuccess;
      return { mutateAsync: mockToggleMutateAsync, isPending: false };
    }
    return { mutateAsync: vi.fn(), isPending: false };
  },
}));

const mockInvalidateQueries = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: mockInvalidateQueries }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { toast } from 'sonner';
import type { EmailTemplate } from '../../types/email-templates.types';
import { useEmailTemplatesTab } from '../use-email-templates-tab';

// ============================================================
// Test Helpers
// ============================================================

function renderAndGet() {
  const { result } = renderHook(() => useEmailTemplatesTab());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useEmailTemplatesTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTemplatesResponse = { templates: [mockProtoTemplate, mockProtoTemplate2] };
    mockCreateMutateAsync.mockResolvedValue({});
    mockUpdateMutateAsync.mockResolvedValue({});
    mockToggleMutateAsync.mockResolvedValue({});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Initial state ───────────────────────────────────────────

  describe('initial state', () => {
    it('should have searchTerm as empty string', () => {
      const result = renderAndGet();
      expect(result.current.searchTerm).toBe('');
    });

    it('should have editingTemplate = null', () => {
      const result = renderAndGet();
      expect(result.current.editingTemplate).toBeNull();
    });

    it('should have isDialogOpen = false', () => {
      const result = renderAndGet();
      expect(result.current.isDialogOpen).toBe(false);
    });
  });

  // ── Templates list ──────────────────────────────────────────

  describe('templates list', () => {
    it('should map proto templates to local format', () => {
      const result = renderAndGet();
      expect(result.current.templates).toHaveLength(2);
      expect(result.current.templates[0]).toEqual({
        id: 'tpl-1',
        name: 'welcome_email',
        subject: 'Welcome!',
        body: '<h1>Hello</h1>',
        is_active: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
        description: 'Welcome template',
      });
    });

    it('should return null for missing description', () => {
      const result = renderAndGet();
      expect(result.current.templates[1].description).toBeNull();
    });

    it('should return empty array when no response', () => {
      mockTemplatesResponse = undefined;
      const result = renderAndGet();
      expect(result.current.templates).toEqual([]);
    });
  });

  // ── filteredTemplates ───────────────────────────────────────

  describe('filteredTemplates', () => {
    it('should filter by name case insensitively', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchTerm('WELCOME'));

      expect(result.current.filteredTemplates).toHaveLength(1);
      expect(result.current.filteredTemplates[0].name).toBe('welcome_email');
    });

    it('should filter by subject case insensitively', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchTerm('reset'));

      expect(result.current.filteredTemplates).toHaveLength(1);
      expect(result.current.filteredTemplates[0].name).toBe('password_reset');
    });

    it('should return all when search is empty', () => {
      const result = renderAndGet();
      expect(result.current.filteredTemplates).toHaveLength(2);
    });
  });

  // ── handleCreateNew ─────────────────────────────────────────

  describe('handleCreateNew', () => {
    it('should set defaults for new template', () => {
      const result = renderAndGet();

      act(() => result.current.handleCreateNew());

      expect(result.current.editingTemplate).not.toBeNull();
      const tpl = result.current.editingTemplate as EmailTemplate;
      expect(tpl.id).toBe('');
      expect(tpl.name).toBe('welcome_email');
      expect(tpl.subject).toBe('admin.emailTemplates.defaults.subject');
      expect(tpl.body).toContain('<!DOCTYPE html>');
      expect(tpl.is_active).toBe(true);
      expect(result.current.isDialogOpen).toBe(true);
    });
  });

  // ── handleSaveTemplate ──────────────────────────────────────

  describe('handleSaveTemplate', () => {
    it('should return early when editingTemplate is null', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleSaveTemplate();
      });

      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
      expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
    });

    it('should toast error when name is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleCreateNew());
      act(() =>
        result.current.setEditingTemplate({
          ...result.current.editingTemplate!,
          name: '  ',
        })
      );

      await act(async () => {
        await result.current.handleSaveTemplate();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.emailTemplates.errors.nameRequired');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should toast error when subject is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleCreateNew());
      act(() =>
        result.current.setEditingTemplate({
          ...result.current.editingTemplate!,
          name: 'valid_name',
          subject: '',
        })
      );

      await act(async () => {
        await result.current.handleSaveTemplate();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.emailTemplates.errors.subjectRequired');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should create template when no id (new)', async () => {
      const result = renderAndGet();

      act(() => result.current.handleCreateNew());

      await act(async () => {
        await result.current.handleSaveTemplate();
      });

      expect(mockCreateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'welcome_email',
          subject: 'admin.emailTemplates.defaults.subject',
        })
      );
      expect(mockUpdateMutateAsync).not.toHaveBeenCalled();
    });

    it('should update template when id exists', async () => {
      const result = renderAndGet();

      act(() =>
        result.current.setEditingTemplate({
          id: 'tpl-1',
          name: 'welcome_email',
          subject: 'Updated Subject',
          body: '<h1>Updated</h1>',
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          description: null,
        })
      );

      await act(async () => {
        await result.current.handleSaveTemplate();
      });

      expect(mockUpdateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'tpl-1',
          name: 'welcome_email',
          subject: 'Updated Subject',
        })
      );
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });
  });

  // ── handleToggleActive ──────────────────────────────────────

  describe('handleToggleActive', () => {
    it('should call toggleEmailTemplateActive with inverted isActive', async () => {
      const result = renderAndGet();
      const template = result.current.templates[0];

      await act(async () => {
        await result.current.handleToggleActive(template);
      });

      expect(mockToggleMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'tpl-1',
          isActive: false,
        })
      );
    });
  });

  // ── Mutation callbacks ──────────────────────────────────────

  describe('mutation callbacks', () => {
    it('should invalidate queries and toast on create success', () => {
      renderAndGet();
      createOnSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalled();
      expect(toast.success).toHaveBeenCalledWith('admin.emailTemplates.success.created');
    });

    it('should invalidate queries and toast on update success', () => {
      renderAndGet();
      updateOnSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalled();
      expect(toast.success).toHaveBeenCalledWith('admin.emailTemplates.success.updated');
    });

    it('should toast activated on toggle success when active', () => {
      renderAndGet();
      toggleOnSuccess?.({ template: { isActive: true } });

      expect(toast.success).toHaveBeenCalledWith('admin.emailTemplates.success.activated');
    });

    it('should toast deactivated on toggle success when inactive', () => {
      renderAndGet();
      toggleOnSuccess?.({ template: { isActive: false } });

      expect(toast.success).toHaveBeenCalledWith('admin.emailTemplates.success.deactivated');
    });
  });

  // ── getDefaultTemplateContent ───────────────────────────────

  describe('getDefaultTemplateContent', () => {
    it('should return HTML string', () => {
      const result = renderAndGet();
      const content = result.current.getDefaultTemplateContent();
      expect(content).toContain('<!DOCTYPE html>');
      expect(content).toContain('{{company_name}}');
      expect(content).toContain('{{email}}');
      expect(content).toContain('{{password}}');
      expect(content).toContain('{{loginUrl}}');
    });
  });
});
