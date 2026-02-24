import { TextsVirtualTable } from '@/components/dispatch-texts';
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
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useDispatchTexts } from '@/hooks/use-dispatch-texts';
import { WhatsAppFormattedText } from '@/lib/whatsapp-formatter';
import { AlertCircle, Copy, FileText, HelpCircle, Plus, Trash2, Type } from 'lucide-react';
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
 * Dispatch texts management page.
 * Create customizable text templates with variable placeholders.
 */
export default function DispatchTexts() {
  const { t } = useTranslation();
  const {
    loading,
    items,
    dialogOpen,
    editing,
    form,
    deleteTarget,
    variableChips,
    preview,
    saving,
    activeCopyIndex,
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
    setActiveCopyIndex,
    addCopyVariation,
    removeCopyVariation,
    updateCopyAtIndex,
  } = useDispatchTexts();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <Type
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
                    {t('pages.dispatchTexts.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dispatchTexts.subtitle')}
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
                    {t('pages.dispatchTexts.newText')}
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-[calc(100vw-2rem)] xs:max-w-[calc(100vw-2.5rem)] sm:max-w-[calc(100vw-3rem)] md:max-w-3xl lg:max-w-4xl xl:max-w-5xl 2xl:max-w-6xl max-h-[85vh] xs:max-h-[90vh] overflow-y-auto p-4 xs:p-5 sm:p-6 rounded-xl xs:rounded-2xl text-black">
                  <DialogHeader>
                    <DialogTitle className="text-base xs:text-lg md:text-xl xl:text-2xl text-black">
                      {editing
                        ? t('pages.dispatchTexts.editTitle')
                        : t('pages.dispatchTexts.createTitle')}
                    </DialogTitle>
                  </DialogHeader>

                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 xs:gap-4 sm:gap-5 lg:gap-6 xl:gap-8">
                    <div className="space-y-3 xs:space-y-4 lg:space-y-5">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 xs:gap-4">
                        <div className="space-y-1.5 xs:space-y-2">
                          <Label htmlFor="dispatch-text-name" className="text-[#8D2D53]">
                            {t('pages.dispatchTexts.form.name')}
                          </Label>
                          <Input
                            id="dispatch-text-name"
                            className="text-black"
                            value={form.name}
                            onChange={(e) => updateFormField('name', e.target.value)}
                            placeholder={t('pages.dispatchTexts.form.namePlaceholder')}
                          />
                        </div>
                        <div className="flex items-end justify-between gap-3">
                          <div className="space-y-1">
                            <Label htmlFor="dispatch-text-active" className="text-[#8D2D53]">
                              {t('pages.dispatchTexts.form.active')}
                            </Label>
                            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                              {t('pages.dispatchTexts.form.activeHint')}
                            </p>
                          </div>
                          <Switch
                            id="dispatch-text-active"
                            checked={form.is_active}
                            onCheckedChange={(v) => updateFormField('is_active', v)}
                          />
                        </div>
                      </div>

                      <div
                        className="rounded-xl border p-3 xs:p-4 md:p-5"
                        role="alert"
                        style={{
                          backgroundColor: 'rgba(180, 120, 60, 0.08)',
                          borderColor: 'rgba(180, 120, 60, 0.25)',
                          backdropFilter: 'blur(12px) saturate(180%)',
                          WebkitBackdropFilter: 'blur(12px) saturate(180%)',
                        }}
                      >
                        <div className="flex items-center gap-2 xs:gap-3">
                          <div
                            className="p-1.5 rounded-lg"
                            style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                          >
                            <AlertCircle
                              className="w-4 h-4"
                              style={{ color: 'rgb(160, 100, 40)' }}
                              aria-hidden="true"
                            />
                          </div>
                          <p
                            className="text-xs xs:text-sm leading-relaxed"
                            style={{ color: 'rgba(120, 75, 30, 0.8)' }}
                          >
                            {t('pages.dispatchTexts.alert.formatting')}
                          </p>
                        </div>
                      </div>

                      <div className="space-y-1.5 xs:space-y-2">
                        <Label className="text-[#8D2D53]">
                          {t('pages.dispatchTexts.form.variables')}
                        </Label>
                        <div className="flex flex-wrap gap-1.5 xs:gap-2" role="list">
                          {variableChips.map((v) => (
                            <span
                              key={v.key}
                              className="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium cursor-pointer transition-colors"
                              style={{ backgroundColor: colors.wine, color: '#ffffff' }}
                              title={v.label}
                              onClick={() => insertAtCursor(v.key)}
                              onKeyDown={(e) => {
                                if (e.key === 'Enter' || e.key === ' ') {
                                  e.preventDefault();
                                  insertAtCursor(v.key);
                                }
                              }}
                              role="button"
                              tabIndex={0}
                            >
                              {v.key}
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6 ml-1"
                                aria-label={t('pages.dispatchTexts.success.copied')}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  copyToken(v.key);
                                }}
                              >
                                <Copy className="h-3 w-3" aria-hidden="true" />
                              </Button>
                            </span>
                          ))}
                        </div>
                      </div>

                      <div className="space-y-1.5 xs:space-y-2">
                        <div className="flex items-center justify-between">
                          <Label className="text-[#8D2D53]">
                            {t('pages.dispatchTexts.form.text')}
                          </Label>
                          {form.copies.length < 10 && (
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={addCopyVariation}
                              className="h-7 text-xs"
                            >
                              <Plus className="h-3 w-3 mr-1" aria-hidden="true" />
                              {t('pages.dispatchTexts.copies.add')}
                            </Button>
                          )}
                        </div>

                        {form.copies.length > 1 && (
                          <div className="flex flex-wrap gap-1.5" role="tablist">
                            {form.copies.map((_, idx) => (
                              <button
                                key={`copy-tab-${idx}`}
                                type="button"
                                role="tab"
                                aria-selected={activeCopyIndex === idx}
                                className={`inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                                  activeCopyIndex === idx
                                    ? 'bg-[#8D2D53] text-white'
                                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                                }`}
                                onClick={() => setActiveCopyIndex(idx)}
                              >
                                {t('pages.dispatchTexts.copies.variation', { n: idx + 1 })}
                                {form.copies.length > 1 && (
                                  <span
                                    role="button"
                                    tabIndex={0}
                                    className="ml-0.5 rounded-full p-0.5 hover:bg-white/20"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      removeCopyVariation(idx);
                                    }}
                                    onKeyDown={(e) => {
                                      if (e.key === 'Enter' || e.key === ' ') {
                                        e.preventDefault();
                                        e.stopPropagation();
                                        removeCopyVariation(idx);
                                      }
                                    }}
                                    aria-label={t('pages.dispatchTexts.copies.remove')}
                                  >
                                    <Trash2 className="h-3 w-3" />
                                  </span>
                                )}
                              </button>
                            ))}
                          </div>
                        )}

                        <Textarea
                          id="dispatch-text-body"
                          className="text-black min-h-[120px] xs:min-h-[150px] lg:min-h-[200px] xl:min-h-[250px] 2xl:min-h-[300px]"
                          rows={12}
                          value={form.copies[activeCopyIndex] ?? ''}
                          onChange={(e) => updateCopyAtIndex(activeCopyIndex, e.target.value)}
                          placeholder={t('pages.dispatchTexts.form.textPlaceholder')}
                        />

                        {form.copies.length > 1 && (
                          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                            {t('pages.dispatchTexts.copies.hint', { count: form.copies.length })}
                          </p>
                        )}
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
                            {t('pages.dispatchTexts.preview.title')}
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
                            {t('pages.dispatchTexts.preview.empty')}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                </DialogContent>
              </Dialog>
            </div>

            {/* Alert Card - Glassmorphism */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              role="alert"
              style={{
                backgroundColor: 'rgba(180, 120, 60, 0.08)',
                borderColor: 'rgba(180, 120, 60, 0.25)',
                backdropFilter: 'blur(12px) saturate(180%)',
                WebkitBackdropFilter: 'blur(12px) saturate(180%)',
              }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-sm xs:text-base font-semibold tracking-tight"
                  style={{ color: 'rgb(120, 75, 30)' }}
                >
                  {t('pages.dispatchTexts.alert.title')}
                </span>
              </div>
              <p
                className="text-xs xs:text-sm leading-relaxed"
                style={{ color: 'rgba(120, 75, 30, 0.8)' }}
              >
                {t('pages.dispatchTexts.alert.description')}
              </p>
            </div>

            {/* Texts Table Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <FileText
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
                    {t('pages.dispatchTexts.table.title')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.dispatchTexts.table.description')}
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
                  {t('pages.dispatchTexts.empty')}
                </p>
              ) : (
                <TextsVirtualTable items={items} onEdit={openEdit} onRemove={requestDelete} />
              )}
            </div>

            {/* Como funciona Card */}
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
                  {t('pages.dispatchTexts.howItWorks.title')}
                </span>
              </div>

              <div className="space-y-3">
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.dispatchTexts.howItWorks.description')}
                </p>
                <ul
                  className="list-disc list-inside space-y-1.5 text-xs xs:text-sm"
                  style={{ color: '#71717a' }}
                >
                  <li>{t('pages.dispatchTexts.howItWorks.variables')}</li>
                  <li>{t('pages.dispatchTexts.howItWorks.formatting')}</li>
                  <li>{t('pages.dispatchTexts.howItWorks.automations')}</li>
                </ul>
                <div
                  className="p-2.5 xs:p-3 rounded-lg xs:rounded-xl border"
                  style={{
                    backgroundColor: colors.iconBg,
                    borderColor: colors.wineBorder,
                  }}
                >
                  <p className="text-xs xs:text-sm">
                    <strong>{t('common.tip')}:</strong> {t('pages.dispatchTexts.howItWorks.note')}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => !open && cancelDelete()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('pages.dispatchTexts.confirmDeleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('pages.dispatchTexts.confirmDeleteDescription', {
                name: deleteTarget?.name ?? '',
              })}
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
