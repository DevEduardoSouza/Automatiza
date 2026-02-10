import { Button } from '@/components/ui/button';
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
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4">
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
              <Button onClick={() => handleOpenDialog()} className="w-full md:w-auto">
                <Plus className="h-4 w-4 mr-2" />
                {t('admin.plans.newPlan')}
              </Button>
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
            <div className="text-center py-8 text-sm" role="status" aria-live="polite">
              {t('common.loading')}
            </div>
          ) : plans.length === 0 ? (
            <div className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
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
