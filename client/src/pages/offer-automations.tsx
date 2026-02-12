import {
  AutomationCards,
  AutomationFormDialog,
  AutomationTable,
  OffersPreviewDialog,
} from '@/components/automations';
import { ErrorBoundary } from '@/components/error-boundary';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useAutomations } from '@/hooks/use-automations';
import { useNavigationGuard } from '@/hooks/use-navigation-guard';
import { useToast } from '@/hooks/use-toast';
import { AlertCircle, FileText, List, Plus, WifiOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const AIIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinejoin="round"
    className={className}
    style={style}
    role="img"
    aria-label="AI"
  >
    <path d="M4 12C4 8.22876 4 6.34315 5.17157 5.17157C6.34315 4 8.22876 4 12 4C15.7712 4 17.6569 4 18.8284 5.17157C20 6.34315 20 8.22876 20 12C20 15.7712 20 17.6569 18.8284 18.8284C17.6569 20 15.7712 20 12 20C8.22876 20 6.34315 20 5.17157 18.8284C4 17.6569 4 15.7712 4 12Z" />
    <path d="M7.5 15L9.34189 9.47434C9.43631 9.19107 9.7014 9 10 9C10.2986 9 10.5637 9.19107 10.6581 9.47434L12.5 15M8.5 13H11.5" />
    <path d="M15.5 9V15" />
    <path d="M8 2V4M16 2V4M12 2V4M8 20V22M12 20V22M16 20V22M22 16H20M4 8H2M4 16H2M4 12H2M22 8H20M22 12H20" />
  </svg>
);

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

export default function OfferAutomations() {
  const { t } = useTranslation();
  const { toast } = useToast();

  const {
    automations,
    maxAutomations,
    groups,
    allGroups,
    loading,
    dialogOpen,
    editingId,
    whatsappConnected,
    activeStores,
    runningId,
    resettingId,
    previewOffers,
    loadingPreview,
    previewDialogOpen,
    formData,
    monitorDelayUnit,
    monitorDelayInput,
    availableCoupons,
    dispatchTexts,
    mlLinkLists,
    loadingMlLinkLists,
    limitReached,
    nearLimit,
    refinementOpen,
    savingAutomation,
    setDialogOpen,
    setPreviewDialogOpen,
    setFormData,
    setMonitorDelayUnit,
    setMonitorDelayInput,
    setRefinementOpen,
    toggleFavorite,
    handleSave,
    toggleStatus,
    handleDelete,
    openEdit,
    resetForm,
    handleRunNow,
    handleResetHistory,
    handlePreviewOffers,
  } = useAutomations();

  /** FR12: Block browser close/reload while mutations are pending */
  useNavigationGuard(savingAutomation);

  const handleLimitReachedToast = () => {
    toast({
      title: t('pages.offerAutomations.limits.automationsReached'),
      description: t('pages.offerAutomations.limits.automationsReachedMessage', {
        count: automations.length,
        max: maxAutomations,
      }),
      variant: 'destructive',
    });
  };

  if (loading) {
    return (
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center min-h-[400px]">
            <div className="text-center space-y-4">
              <div
                className="h-12 w-12 animate-spin rounded-full border-4 border-t-transparent mx-auto"
                style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
              />
              <p style={{ color: colors.wineTextMuted }}>{t('common.loading')}</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-9 xl:space-y-10">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <AIIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <div className="flex items-center gap-2 flex-wrap">
                    <h2
                      className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                      style={{ color: '#18181b' }}
                    >
                      {t('pages.offerAutomations.title')}
                    </h2>
                    <span
                      className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                      style={
                        limitReached
                          ? {
                              backgroundColor: 'rgba(239, 68, 68, 0.1)',
                              color: 'rgb(185, 40, 40)',
                            }
                          : {
                              backgroundColor: 'rgba(141, 45, 83, 0.15)',
                              color: colors.wine,
                            }
                      }
                    >
                      {automations.length}/{maxAutomations}{' '}
                      {t('pages.offerAutomations.automationsCount')}
                    </span>
                  </div>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.offerAutomations.subtitle')}
                  </p>
                </div>
              </div>

              <AutomationFormDialog
                open={dialogOpen}
                onOpenChange={setDialogOpen}
                editingId={editingId}
                limitReached={limitReached}
                formData={formData}
                setFormData={setFormData}
                groups={groups}
                allGroups={allGroups}
                activeStores={activeStores}
                availableCoupons={availableCoupons}
                dispatchTexts={dispatchTexts}
                mlLinkLists={mlLinkLists}
                loadingMlLinkLists={loadingMlLinkLists}
                monitorDelayUnit={monitorDelayUnit}
                monitorDelayInput={monitorDelayInput}
                setMonitorDelayUnit={setMonitorDelayUnit}
                setMonitorDelayInput={setMonitorDelayInput}
                refinementOpen={refinementOpen}
                setRefinementOpen={setRefinementOpen}
                saving={savingAutomation}
                onSave={handleSave}
                onReset={resetForm}
                onToggleFavorite={toggleFavorite}
                onLimitReachedToast={handleLimitReachedToast}
              />
            </div>

            {/* Limit Alert - Glassmorphism */}
            {(nearLimit || limitReached) && (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-7 xl:p-8"
                role="alert"
                style={
                  limitReached
                    ? {
                        backgroundColor: 'rgba(239, 68, 68, 0.06)',
                        borderColor: 'rgba(239, 68, 68, 0.25)',
                        backdropFilter: 'blur(12px) saturate(180%)',
                        WebkitBackdropFilter: 'blur(12px) saturate(180%)',
                      }
                    : {
                        backgroundColor: 'rgba(180, 120, 60, 0.08)',
                        borderColor: 'rgba(180, 120, 60, 0.25)',
                        backdropFilter: 'blur(12px) saturate(180%)',
                        WebkitBackdropFilter: 'blur(12px) saturate(180%)',
                      }
                }
              >
                <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{
                      backgroundColor: limitReached
                        ? 'rgba(239, 68, 68, 0.1)'
                        : 'rgba(180, 120, 60, 0.12)',
                    }}
                  >
                    <AlertCircle
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{
                        color: limitReached ? 'rgb(220, 50, 50)' : 'rgb(160, 100, 40)',
                      }}
                      aria-hidden="true"
                    />
                  </div>
                  {limitReached && (
                    <span
                      className="text-sm xs:text-base font-semibold tracking-tight"
                      style={{ color: 'rgb(185, 40, 40)' }}
                    >
                      {t('pages.offerAutomations.limits.automationsReached')}
                    </span>
                  )}
                </div>
                <p
                  className="text-xs xs:text-sm leading-relaxed"
                  style={{
                    color: limitReached ? 'rgba(185, 40, 40, 0.8)' : 'rgba(120, 75, 30, 0.8)',
                  }}
                >
                  {limitReached
                    ? t('pages.offerAutomations.limits.reachedMessage', { max: maxAutomations })
                    : t('pages.offerAutomations.limits.nearMessage', {
                        count: automations.length,
                        max: maxAutomations,
                      })}
                </p>
              </div>
            )}

            {/* WhatsApp Disconnected Alert - Red Glassmorphism */}
            {!whatsappConnected && (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-7 xl:p-8"
                role="alert"
                style={{
                  backgroundColor: 'rgba(239, 68, 68, 0.06)',
                  borderColor: 'rgba(239, 68, 68, 0.25)',
                  backdropFilter: 'blur(12px) saturate(180%)',
                  WebkitBackdropFilter: 'blur(12px) saturate(180%)',
                }}
              >
                <div className="flex items-center gap-2 xs:gap-3">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: 'rgba(239, 68, 68, 0.1)' }}
                  >
                    <WifiOff
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: 'rgb(220, 50, 50)' }}
                      aria-hidden="true"
                    />
                  </div>
                  <p
                    className="text-xs xs:text-sm leading-relaxed"
                    style={{ color: 'rgba(185, 40, 40, 0.8)' }}
                  >
                    {t('pages.offerAutomations.whatsappDisconnected')}
                  </p>
                </div>
              </div>
            )}

            {/* Content */}
            {automations.length === 0 ? (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-7 xl:p-8"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex flex-col items-center justify-center py-8 xs:py-10 sm:py-12 lg:py-16 xl:py-20">
                  <div
                    className="p-3 xs:p-4 rounded-xl mb-4"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <FileText
                      className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 lg:h-14 lg:w-14"
                      style={{ color: colors.wine }}
                    />
                  </div>
                  <p
                    className="text-sm xs:text-base lg:text-lg mb-4 text-center"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.offerAutomations.noAutomations')}
                  </p>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span>
                          <Button
                            onClick={() => !limitReached && setDialogOpen(true)}
                            disabled={limitReached}
                            className="bg-[#8D2D53] text-white hover:bg-[#7a2747]"
                          >
                            <Plus className="h-4 w-4 mr-2" />
                            {t('pages.offerAutomations.createFirst')}
                          </Button>
                        </span>
                      </TooltipTrigger>
                      {limitReached && (
                        <TooltipContent>
                          <p>{t('pages.offerAutomations.limits.automationsReached')}</p>
                        </TooltipContent>
                      )}
                    </Tooltip>
                  </TooltipProvider>
                </div>
              </div>
            ) : (
              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-7 xl:p-8"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <List
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
                      {t('pages.offerAutomations.listTitle')}
                    </span>
                    <p
                      className="text-xs xs:text-sm tracking-tight"
                      style={{ color: colors.wineTextMuted }}
                    >
                      {t('pages.offerAutomations.listDescription', {
                        count: automations.length,
                        max: maxAutomations,
                      })}
                    </p>
                  </div>
                </div>

                <AutomationTable
                  automations={automations}
                  runningId={runningId}
                  resettingId={resettingId}
                  loadingPreview={loadingPreview}
                  onEdit={openEdit}
                  onToggleStatus={toggleStatus}
                  onPreview={handlePreviewOffers}
                  onRunNow={handleRunNow}
                  onResetHistory={handleResetHistory}
                  onDelete={handleDelete}
                />

                <AutomationCards
                  automations={automations}
                  runningId={runningId}
                  resettingId={resettingId}
                  loadingPreview={loadingPreview}
                  onEdit={openEdit}
                  onToggleStatus={toggleStatus}
                  onPreview={handlePreviewOffers}
                  onRunNow={handleRunNow}
                  onResetHistory={handleResetHistory}
                  onDelete={handleDelete}
                />
              </div>
            )}
          </div>
        </div>
      </div>

      <OffersPreviewDialog
        open={previewDialogOpen}
        onOpenChange={setPreviewDialogOpen}
        loading={loadingPreview}
        offers={previewOffers}
      />
    </ErrorBoundary>
  );
}
