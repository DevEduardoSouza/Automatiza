import { useAuth } from '@/contexts';
import {
  createEmailTemplate,
  listEmailTemplates,
  toggleEmailTemplateActive,
  updateEmailTemplate,
} from '@/gen/admin/v1/admin-AdminService_connectquery';
import {
  CreateEmailTemplateRequestSchema,
  ListEmailTemplatesRequestSchema,
  ToggleEmailTemplateActiveRequestSchema,
  UpdateEmailTemplateRequestSchema,
} from '@/gen/admin/v1/admin_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { useMutation, useQuery } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import type {
  EmailTemplate,
  TranslationFn,
  UseEmailTemplatesTabReturn,
} from '../types/email-templates.types';

/**
 * Returns the default email template HTML content.
 * Uses only backend-supported variables: {{email}}, {{password}}, {{loginUrl}}, {{company_name}}, {{plan_name}}
 */
const getDefaultTemplateContent = (t: TranslationFn): string => `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{company_name}}</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
  <div style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
    <h1 style="color: white; margin: 0;">{{company_name}}</h1>
  </div>

  <div style="background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px;">
    <p>${t('admin.emailTemplates.defaults.greeting')}</p>

    <p>${t('admin.emailTemplates.defaults.accountCreated')}</p>

    <div style="background: white; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #667eea;">
      <p style="margin: 5px 0;"><strong>${t('admin.emailTemplates.defaults.emailLabel')}</strong> {{email}}</p>
      <p style="margin: 5px 0;"><strong>${t('admin.emailTemplates.defaults.passwordLabel')}</strong> <code style="background: #f4f4f4; padding: 5px 10px; border-radius: 3px; font-size: 14px;">{{password}}</code></p>
    </div>

    <p style="margin: 20px 0;">
      <a href="{{loginUrl}}" style="background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">${t('admin.emailTemplates.defaults.accessButton')}</a>
    </p>

    <p style="color: #666; font-size: 14px; margin-top: 30px;">
      ${t('admin.emailTemplates.defaults.securityTip')}
    </p>

    <p style="color: #666; font-size: 14px; margin-top: 20px;">
      ${t('admin.emailTemplates.defaults.ignoreNote')}
    </p>
  </div>

  <div style="text-align: center; margin-top: 20px; color: #999; font-size: 12px;">
    <p>© {{company_name}}. ${t('admin.emailTemplates.defaults.copyright')}</p>
  </div>
</body>
</html>`;

/**
 * Converts proto EmailTemplate to local type.
 */
function fromProtoEmailTemplate(proto: {
  id: string;
  name: string;
  subject: string;
  body: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
  description?: string;
}): EmailTemplate {
  return {
    id: proto.id,
    name: proto.name,
    subject: proto.subject,
    body: proto.body,
    is_active: proto.isActive,
    created_at: proto.createdAt,
    updated_at: proto.updatedAt,
    description: proto.description ?? null,
  };
}

/**
 * Hook que gerencia a logica da aba de templates de email.
 * Permite criar, editar e ativar/desativar templates.
 */
export function useEmailTemplatesTab(): UseEmailTemplatesTabReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const [searchTerm, setSearchTerm] = useState('');
  const [editingTemplate, setEditingTemplate] = useState<EmailTemplate | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  // Query templates via ConnectRPC
  const { data: templatesResponse, isLoading: loading } = useQuery(
    listEmailTemplates,
    create(ListEmailTemplatesRequestSchema, {}),
    { enabled: !!user }
  );

  // Transform proto templates to local format
  const templates: EmailTemplate[] = useMemo(
    () => (templatesResponse?.templates ?? []).map(fromProtoEmailTemplate),
    [templatesResponse]
  );

  // Filter templates by search term
  const filteredTemplates = useMemo(() => {
    return templates.filter(
      (template) =>
        template.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        template.subject.toLowerCase().includes(searchTerm.toLowerCase())
    );
  }, [templates, searchTerm]);

  // Create template mutation
  const createTemplateMutation = useMutation(createEmailTemplate, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      toast.success(t('admin.emailTemplates.success.created'));
      setIsDialogOpen(false);
      setEditingTemplate(null);
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.emailTemplates.errors.saveError'));
    },
  });

  // Update template mutation
  const updateTemplateMutation = useMutation(updateEmailTemplate, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      toast.success(t('admin.emailTemplates.success.updated'));
      setIsDialogOpen(false);
      setEditingTemplate(null);
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.emailTemplates.errors.saveError'));
    },
  });

  // Toggle active mutation
  const toggleActiveMutation = useMutation(toggleEmailTemplateActive, {
    onSuccess: (response) => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      const isActive = response.template?.isActive;
      toast.success(
        isActive
          ? t('admin.emailTemplates.success.activated')
          : t('admin.emailTemplates.success.deactivated')
      );
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.emailTemplates.errors.toggleError'));
    },
  });

  const handleSaveTemplate = useCallback(async () => {
    if (!editingTemplate) return;

    if (!editingTemplate.name.trim()) {
      toast.error(t('admin.emailTemplates.errors.nameRequired'));
      return;
    }

    if (!editingTemplate.subject.trim()) {
      toast.error(t('admin.emailTemplates.errors.subjectRequired'));
      return;
    }

    if (editingTemplate.id) {
      // Update existing template
      await updateTemplateMutation.mutateAsync(
        create(UpdateEmailTemplateRequestSchema, {
          id: editingTemplate.id,
          name: editingTemplate.name,
          subject: editingTemplate.subject,
          body: editingTemplate.body,
          isActive: editingTemplate.is_active,
          description: editingTemplate.description || undefined,
        })
      );
    } else {
      // Create new template
      await createTemplateMutation.mutateAsync(
        create(CreateEmailTemplateRequestSchema, {
          name: editingTemplate.name,
          subject: editingTemplate.subject,
          body: editingTemplate.body,
          isActive: editingTemplate.is_active,
          description: editingTemplate.description || undefined,
        })
      );
    }
  }, [editingTemplate, createTemplateMutation, updateTemplateMutation, t]);

  const handleToggleActive = useCallback(
    async (template: EmailTemplate) => {
      await toggleActiveMutation.mutateAsync(
        create(ToggleEmailTemplateActiveRequestSchema, {
          id: template.id,
          isActive: !template.is_active,
        })
      );
    },
    [toggleActiveMutation]
  );

  const handleCreateNew = useCallback(() => {
    setEditingTemplate({
      id: '',
      name: 'welcome_email',
      subject: t('admin.emailTemplates.defaults.subject'),
      body: getDefaultTemplateContent(t),
      description: t('admin.emailTemplates.defaults.description'),
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    });
    setIsDialogOpen(true);
  }, [t]);

  return {
    templates,
    filteredTemplates,
    loading,
    searchTerm,
    setSearchTerm,
    editingTemplate,
    setEditingTemplate,
    isDialogOpen,
    setIsDialogOpen,
    handleSaveTemplate,
    handleToggleActive,
    handleCreateNew,
    getDefaultTemplateContent: () => getDefaultTemplateContent(t),
  };
}
