import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { useIsMobile } from '@/hooks/use-mobile';
import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePlansTab } from './hooks';
import { PlanForm, PlansMobileList, PlansTable } from './plans';

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
 * Aba de gerenciamento de planos de assinatura.
 * Permite criar, editar e excluir planos.
 */
export default function PlansTab() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const {
    plans,
    loading,
    editingPlan,
    isDialogOpen,
    setIsDialogOpen,
    formData,
    setFormData,
    handleOpenDialog,
    handleSavePlan,
    handleDeletePlan,
    formatStorage,
  } = usePlansTab();

  return (
    <div className="space-y-4">
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4 sm:mb-5">
          <div>
            <span
              className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
              style={{ color: colors.wineText }}
            >
              {t('admin.plans.title')}
            </span>
            <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
              {t('admin.plans.description')}
            </p>
          </div>
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <button
                type="button"
                onClick={() => handleOpenDialog()}
                className="auth-btn-glow relative inline-flex w-full md:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all"
              >
                <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                  <div className="auth-btn-glow__glow absolute inset-0" />
                </div>
                <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                  <Plus className="h-4 w-4" />
                  {t('admin.plans.newPlan')}
                </span>
              </button>
            </DialogTrigger>
            <DialogContent
              className="max-w-2xl max-h-[90vh] overflow-y-auto"
              aria-describedby="plans-dialog-description"
            >
              <DialogHeader>
                <DialogTitle>
                  {editingPlan
                    ? t('admin.plans.dialog.editTitle')
                    : t('admin.plans.dialog.createTitle')}
                </DialogTitle>
                <DialogDescription id="plans-dialog-description">
                  {t('admin.plans.dialog.description')}
                </DialogDescription>
              </DialogHeader>
              <PlanForm
                formData={formData}
                setFormData={setFormData}
                onSave={handleSavePlan}
                t={t}
              />
            </DialogContent>
          </Dialog>
        </div>
        <div className="space-y-3 xs:space-y-4">
          {loading ? (
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
          ) : plans.length === 0 ? (
            <div
              className="text-center py-6 xs:py-8 text-sm"
              style={{ color: colors.wineTextMuted }}
            >
              {t('admin.plans.noPlans')}
            </div>
          ) : isMobile ? (
            <PlansMobileList
              plans={plans}
              formatStorage={formatStorage}
              onEdit={handleOpenDialog}
              onDelete={handleDeletePlan}
              t={t}
            />
          ) : (
            <PlansTable
              plans={plans}
              formatStorage={formatStorage}
              onEdit={handleOpenDialog}
              onDelete={handleDeletePlan}
              t={t}
            />
          )}
        </div>
      </div>
    </div>
  );
}
