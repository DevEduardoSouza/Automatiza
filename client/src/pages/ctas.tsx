import { ErrorBoundary } from '@/components/error-boundary';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { CTA_PLATFORMS, CTA_TYPES } from '@/entities/cta';
import { useCTAs } from '@/hooks/use-ctas';
import { WhatsAppFormattedText } from '@/lib/whatsapp-formatter';
import { Edit2, FileText, HelpCircle, Info, MessageSquare, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

/**
 * CTAs management page.
 * Create and manage call-to-action texts for automations.
 */
export default function CTAs() {
  const { t } = useTranslation();
  const {
    loading,
    items,
    dialogOpen,
    editing,
    form,
    deleteTarget,
    preview,
    saving,
    filterPlatform,
    filterType,
    filterActive,
    setFilterPlatform,
    setFilterType,
    setFilterActive,
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
  } = useCTAs();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <MessageSquare
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h2
                    className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.ctas.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.ctas.subtitle')}
                  </p>
                </div>
              </div>

              <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
                <DialogTrigger asChild>
                  <Button
                    className="w-full xs:w-auto bg-[#8D2D53] text-white hover:bg-[#7a2747]"
                    onClick={openCreate}
                  >
                    <Plus className="h-4 w-4 mr-2" aria-hidden="true" />
                    {t('pages.ctas.newCTA')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-[calc(100vw-2rem)] xs:max-w-[calc(100vw-2.5rem)] sm:max-w-[calc(100vw-3rem)] md:max-w-3xl lg:max-w-4xl xl:max-w-5xl 2xl:max-w-6xl max-h-[85vh] xs:max-h-[90vh] overflow-y-auto p-4 xs:p-5 sm:p-6 rounded-xl xs:rounded-2xl text-black">
                  <DialogHeader>
                    <DialogTitle className="text-base xs:text-lg md:text-xl xl:text-2xl text-black">
                      {editing ? t('pages.ctas.editTitle') : t('pages.ctas.createTitle')}
                    </DialogTitle>
                  </DialogHeader>

                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 xs:gap-4 sm:gap-5 lg:gap-6 xl:gap-8">
                    {/* Form */}
                    <div className="space-y-3 xs:space-y-4 lg:space-y-5">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 xs:gap-4">
                        <div className="space-y-1.5 xs:space-y-2">
                          <Label htmlFor="cta-platform" className="text-[#8D2D53]">
                            {t('pages.ctas.form.platform')}
                          </Label>
                          <Select
                            value={form.platform}
                            onValueChange={(v) =>
                              updateFormField('platform', v as typeof form.platform)
                            }
                          >
                            <SelectTrigger id="cta-platform" className="text-black">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {CTA_PLATFORMS.map((p) => (
                                <SelectItem key={p.value} value={p.value}>
                                  {t(p.labelKey)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        <div className="space-y-1.5 xs:space-y-2">
                          <Label htmlFor="cta-type" className="text-[#8D2D53]">
                            {t('pages.ctas.form.ctaType')}
                          </Label>
                          <Select
                            value={form.cta_type}
                            onValueChange={(v) =>
                              updateFormField('cta_type', v as typeof form.cta_type)
                            }
                          >
                            <SelectTrigger id="cta-type" className="text-black">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {CTA_TYPES.map((ct) => (
                                <SelectItem key={ct.value} value={ct.value}>
                                  {t(ct.labelKey)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      </div>

                      {editing && (
                        <div className="flex items-center justify-between gap-3">
                          <div className="space-y-1">
                            <Label htmlFor="cta-active" className="text-[#8D2D53]">
                              {t('pages.ctas.form.active')}
                            </Label>
                            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                              {t('pages.ctas.form.activeHint')}
                            </p>
                          </div>
                          <Switch
                            id="cta-active"
                            checked={form.is_active}
                            onCheckedChange={(v) => updateFormField('is_active', v)}
                          />
                        </div>
                      )}

                      <div className="space-y-1.5 xs:space-y-2">
                        <Label htmlFor="cta-message" className="text-[#8D2D53]">
                          {t('pages.ctas.form.message')}
                        </Label>
                        <Textarea
                          id="cta-message"
                          className="text-black min-h-[120px] xs:min-h-[150px] lg:min-h-[200px] xl:min-h-[250px] 2xl:min-h-[300px]"
                          rows={8}
                          value={form.message}
                          onChange={(e) => updateFormField('message', e.target.value)}
                          placeholder={t('pages.ctas.form.messagePlaceholder')}
                        />
                      </div>

                      <div className="flex flex-col-reverse xs:flex-row justify-end gap-2 xs:gap-3">
                        <Button variant="outline" onClick={closeDialog}>
                          {t('common.cancel')}
                        </Button>
                        <Button onClick={save} disabled={saving}>
                          {editing ? t('common.save') : t('common.create')}
                        </Button>
                      </div>
                    </div>

                    {/* Preview */}
                    <div className="space-y-2 xs:space-y-3">
                      <div
                        className="rounded-xl border p-3 xs:p-4 sm:p-5 lg:p-6 xl:p-8"
                        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
                      >
                        <div className="flex items-center gap-1.5 xs:gap-2 mb-2 xs:mb-3 lg:mb-4">
                          <div
                            className="p-1.5 rounded-lg"
                            style={{ backgroundColor: colors.iconBg }}
                          >
                            <FileText
                              className="w-4 h-4"
                              style={{ color: colors.wine }}
                              aria-hidden="true"
                            />
                          </div>
                          <span
                            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                            style={{ color: colors.wineText }}
                          >
                            {t('pages.ctas.preview.title')}
                          </span>
                        </div>
                        {preview ? (
                          <WhatsAppFormattedText
                            text={preview}
                            className="text-xs xs:text-sm leading-relaxed text-black min-h-[80px] xs:min-h-[100px] lg:min-h-[150px] xl:min-h-[200px] 2xl:min-h-[250px]"
                          />
                        ) : (
                          <p
                            className="text-xs xs:text-sm leading-relaxed min-h-[80px] xs:min-h-[100px] lg:min-h-[150px] xl:min-h-[200px] 2xl:min-h-[250px]"
                            style={{ color: colors.wineTextMuted }}
                          >
                            {t('pages.ctas.preview.empty')}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                </DialogContent>
              </Dialog>
            </div>

            {/* Vínculo automático — explica que basta cadastrar aqui */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-start gap-3 xs:gap-4">
                <div
                  className="p-1.5 xs:p-2 rounded-lg flex-shrink-0"
                  style={{ backgroundColor: colors.iconBgHover }}
                >
                  <Info
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div className="space-y-1 min-w-0">
                  <p
                    className="text-sm xs:text-base font-semibold"
                    style={{ color: colors.wineText }}
                  >
                    {t('pages.ctas.autoPickBannerTitle')}
                  </p>
                  <p
                    className="text-xs xs:text-sm leading-relaxed"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.ctas.autoPickBannerBody')}
                  </p>
                </div>
              </div>
            </div>

            {/* Filters */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex flex-col sm:flex-row gap-3 xs:gap-4">
                <div className="space-y-1.5 flex-1">
                  <Label className="text-xs" style={{ color: colors.wineTextLight }}>
                    {t('pages.ctas.filters.platform')}
                  </Label>
                  <Select
                    value={filterPlatform}
                    onValueChange={(v) => setFilterPlatform(v as typeof filterPlatform)}
                  >
                    <SelectTrigger className="text-black">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t('common.all')}</SelectItem>
                      <SelectItem value="shopee">{t('pages.ctas.platforms.shopee')}</SelectItem>
                      <SelectItem value="ml">{t('pages.ctas.platforms.ml')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1.5 flex-1">
                  <Label className="text-xs" style={{ color: colors.wineTextLight }}>
                    {t('pages.ctas.filters.type')}
                  </Label>
                  <Select
                    value={filterType}
                    onValueChange={(v) => setFilterType(v as typeof filterType)}
                  >
                    <SelectTrigger className="text-black">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t('common.all')}</SelectItem>
                      <SelectItem value="normal">{t('pages.ctas.types.normal')}</SelectItem>
                      <SelectItem value="discount">{t('pages.ctas.types.discount')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1.5 flex-1">
                  <Label className="text-xs" style={{ color: colors.wineTextLight }}>
                    {t('pages.ctas.filters.status')}
                  </Label>
                  <Select
                    value={filterActive}
                    onValueChange={(v) => setFilterActive(v as typeof filterActive)}
                  >
                    <SelectTrigger className="text-black">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t('common.all')}</SelectItem>
                      <SelectItem value="active">{t('common.active')}</SelectItem>
                      <SelectItem value="inactive">{t('common.inactive')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>

            {/* CTAs Table */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <MessageSquare
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <span
                    className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.ctas.table.title')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.ctas.table.description')}
                  </p>
                </div>
              </div>
              {loading ? (
                <div className="flex items-center justify-center py-8">
                  <div
                    className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                    style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                  />
                </div>
              ) : items.length === 0 ? (
                <p className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
                  {t('pages.ctas.empty')}
                </p>
              ) : (
                <div className="space-y-3">
                  {items.map((cta) => (
                    <div
                      key={cta.id}
                      className="rounded-xl border p-3 xs:p-4 flex flex-col sm:flex-row sm:items-center gap-3 transition-colors"
                      style={{
                        backgroundColor: '#ffffff',
                        borderColor: colors.wineBorder,
                      }}
                    >
                      <div className="flex-1 min-w-0">
                        <p className="text-sm text-black line-clamp-2 mb-2">{cta.message}</p>
                        <div className="flex flex-wrap gap-1.5">
                          <Badge
                            variant="outline"
                            className="text-[10px]"
                            style={{ borderColor: colors.wineBorder, color: colors.wineText }}
                          >
                            {t(
                              `pages.ctas.platforms.${cta.platform === 'ml' ? 'ml' : cta.platform}`
                            )}
                          </Badge>
                          <Badge
                            variant="outline"
                            className="text-[10px]"
                            style={{ borderColor: colors.wineBorder, color: colors.wineText }}
                          >
                            {t(`pages.ctas.types.${cta.cta_type}`)}
                          </Badge>
                          <Badge
                            variant={cta.is_active ? 'default' : 'secondary'}
                            className="text-[10px]"
                          >
                            {cta.is_active ? t('common.active') : t('common.inactive')}
                          </Badge>
                        </div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <div>
                                <Switch
                                  checked={cta.is_active}
                                  onCheckedChange={() => toggleActive(cta)}
                                  aria-label={
                                    cta.is_active ? t('common.deactivate') : t('common.activate')
                                  }
                                />
                              </div>
                            </TooltipTrigger>
                            <TooltipContent>
                              <p>{cta.is_active ? t('common.deactivate') : t('common.activate')}</p>
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => openEdit(cta)}
                          aria-label={t('common.edit')}
                        >
                          <Edit2 className="h-4 w-4" style={{ color: colors.wine }} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => requestDelete(cta)}
                          aria-label={t('common.delete')}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* How it works */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <HelpCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                  style={{ color: colors.wineTextLight }}
                >
                  {t('pages.ctas.howItWorks.title')}
                </span>
              </div>

              <div className="space-y-3">
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.ctas.howItWorks.description')}
                </p>
                <ul
                  className="list-disc list-inside space-y-1.5 text-xs xs:text-sm"
                  style={{ color: '#71717a' }}
                >
                  <li>{t('pages.ctas.howItWorks.platforms')}</li>
                  <li>{t('pages.ctas.howItWorks.types')}</li>
                  <li>{t('pages.ctas.howItWorks.automations')}</li>
                </ul>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Delete Confirmation */}
      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => !open && cancelDelete()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.ctas.confirmDeleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('pages.ctas.confirmDeleteDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>{t('common.delete')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ErrorBoundary>
  );
}
