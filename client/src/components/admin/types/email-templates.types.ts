/**
 * Types for the EmailTemplatesTab component and useEmailTemplatesTab hook.
 * Manages email templates for the system.
 */

export interface EmailTemplate {
  id: string;
  name: string;
  subject: string;
  body: string;
  description: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ApiError {
  message: string;
  code?: string;
}

export type TranslationFn = (key: string) => string;

export interface UseEmailTemplatesTabReturn {
  templates: EmailTemplate[];
  filteredTemplates: EmailTemplate[];
  loading: boolean;
  searchTerm: string;
  setSearchTerm: (term: string) => void;
  editingTemplate: EmailTemplate | null;
  setEditingTemplate: (template: EmailTemplate | null) => void;
  isDialogOpen: boolean;
  setIsDialogOpen: (open: boolean) => void;
  handleSaveTemplate: () => Promise<void>;
  handleToggleActive: (template: EmailTemplate) => Promise<void>;
  handleCreateNew: () => void;
  getDefaultTemplateContent: () => string;
}
