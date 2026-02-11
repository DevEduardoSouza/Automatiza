import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { Edit, Mail, Plus, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useEmailTemplatesTab } from './hooks';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

/**
 * Aba de gerenciamento de templates de email.
 * Permite criar, editar e ativar/desativar templates.
 */
export default function EmailTemplatesTab() {
  const { t } = useTranslation();
  const {
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
  } = useEmailTemplatesTab();

  if (loading) {
    return (
      <div className="flex items-center justify-center py-6 xs:py-8" role="status" aria-live="polite">
        <div className="text-center space-y-4">
          <div
            className="h-8 w-8 xs:h-10 xs:w-10 animate-spin rounded-full border-[3px] xs:border-4 border-t-transparent mx-auto"
            style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
          />
          <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
            {t('common.loading')}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 sm:mb-5">
          <div
            className="p-1.5 xs:p-2 sm:p-2.5 rounded-lg"
            style={{ backgroundColor: colors.iconBg }}
          >
            <Mail
              className="w-4 h-4 xs:w-5 xs:h-5"
              style={{ color: colors.wine }}
              aria-hidden="true"
            />
          </div>
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.emailTemplates.title')}
          </span>
        </div>
        <p className="text-xs xs:text-sm mb-3 xs:mb-4" style={{ color: colors.wineTextMuted }}>
          {t('admin.emailTemplates.description')}
        </p>
        <div className="space-y-3 xs:space-y-4">
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search
                className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4"
                style={{ color: colors.wineTextMuted }}
              />
              <Input
                placeholder={t('admin.emailTemplates.search')}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-10"
                aria-label={t('admin.emailTemplates.search')}
              />
            </div>
            <button
              type="button"
              onClick={handleCreateNew}
              className="auth-btn-glow relative inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all"
            >
              <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                <div className="auth-btn-glow__glow absolute inset-0" />
              </div>
              <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                <Plus className="h-4 w-4" />
                {t('admin.emailTemplates.createTemplate')}
              </span>
            </button>
          </div>

          {filteredTemplates.length === 0 ? (
            <p className="text-center py-8" style={{ color: colors.wineTextMuted }}>
              {t('admin.emailTemplates.noTemplates')}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <Table aria-label={t('admin.emailTemplates.tableAriaLabel')}>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('admin.emailTemplates.columns.name')}</TableHead>
                    <TableHead>{t('admin.emailTemplates.columns.subject')}</TableHead>
                    <TableHead>{t('admin.emailTemplates.columns.status')}</TableHead>
                    <TableHead className="text-right">
                      {t('admin.emailTemplates.columns.actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredTemplates.map((template) => (
                    <TableRow key={template.id}>
                      <TableCell className="font-medium">{template.name}</TableCell>
                      <TableCell className="text-blue-600">{template.subject}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <Switch
                            checked={template.is_active}
                            onCheckedChange={() => handleToggleActive(template)}
                            aria-label={t('admin.emailTemplates.toggleAriaLabel', {
                              name: template.name,
                            })}
                          />
                          <Badge variant={template.is_active ? 'default' : 'secondary'}>
                            {template.is_active ? t('common.active') : t('common.inactive')}
                          </Badge>
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setEditingTemplate(template);
                            setIsDialogOpen(true);
                          }}
                          aria-label={t('admin.emailTemplates.editAriaLabel', {
                            name: template.name,
                          })}
                        >
                          <Edit className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>

      <Dialog
        open={isDialogOpen}
        onOpenChange={(open) => {
          setIsDialogOpen(open);
          if (!open) setEditingTemplate(null);
        }}
      >
        <DialogContent
          className="max-w-2xl max-h-[90vh] overflow-y-auto"
          aria-describedby="email-template-dialog-description"
        >
          <DialogHeader>
            <DialogTitle>
              {editingTemplate?.id
                ? t('admin.emailTemplates.dialog.editTitle')
                : t('admin.emailTemplates.dialog.createTitle')}
            </DialogTitle>
            <DialogDescription id="email-template-dialog-description">
              {editingTemplate?.id
                ? t('admin.emailTemplates.dialog.editDescription')
                : t('admin.emailTemplates.dialog.createDescription')}
            </DialogDescription>
          </DialogHeader>
          {editingTemplate && (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">{t('admin.emailTemplates.dialog.nameLabel')}</Label>
                <Input
                  id="name"
                  value={editingTemplate.name}
                  onChange={(e) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      name: e.target.value,
                    })
                  }
                />
                <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.emailTemplates.dialog.nameHint')}
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="subject">{t('admin.emailTemplates.dialog.subjectLabel')}</Label>
                <Input
                  id="subject"
                  value={editingTemplate.subject}
                  onChange={(e) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      subject: e.target.value,
                    })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="description">
                  {t('admin.emailTemplates.dialog.descriptionLabel')}
                </Label>
                <Textarea
                  id="description"
                  value={editingTemplate.description || ''}
                  onChange={(e) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      description: e.target.value,
                    })
                  }
                  rows={2}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="body">{t('admin.emailTemplates.dialog.contentLabel')}</Label>
                <Textarea
                  id="body"
                  value={editingTemplate.body}
                  onChange={(e) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      body: e.target.value,
                    })
                  }
                  rows={15}
                  className="font-mono text-sm"
                />
                <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('admin.emailTemplates.dialog.contentHint')}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  id="template-active-switch"
                  checked={editingTemplate.is_active}
                  onCheckedChange={(checked) =>
                    setEditingTemplate({
                      ...editingTemplate,
                      is_active: checked,
                    })
                  }
                  aria-describedby="template-active-label"
                />
                <Label id="template-active-label" htmlFor="template-active-switch">
                  {t('admin.emailTemplates.dialog.activeLabel')}
                </Label>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsDialogOpen(false);
                setEditingTemplate(null);
              }}
            >
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSaveTemplate}>
              {editingTemplate?.id
                ? t('admin.emailTemplates.dialog.saveChanges')
                : t('admin.emailTemplates.createTemplate')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
