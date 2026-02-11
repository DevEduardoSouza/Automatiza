import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { AlertCircle, MousePointerClick, Settings, Users, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { DistributionStrategy, SavedRedirectLink } from '../types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface LinkFormCardProps {
  linkName: string;
  slug: string;
  distributionStrategy: DistributionStrategy;
  selectedGroupsCount: number;
  loading: boolean;
  editingLink: SavedRedirectLink | null;
  onLinkNameChange: (value: string) => void;
  onSlugChange: (value: string) => void;
  onStrategyChange: (value: DistributionStrategy) => void;
  onOpenGroupDialog: () => void;
  onSubmit: () => void;
  onCancelEdit: () => void;
  isSubmitDisabled: boolean;
}

export function LinkFormCard({
  linkName,
  slug,
  distributionStrategy,
  selectedGroupsCount,
  loading,
  editingLink,
  onLinkNameChange,
  onSlugChange,
  onStrategyChange,
  onOpenGroupDialog,
  onSubmit,
  onCancelEdit,
  isSubmitDisabled,
}: LinkFormCardProps) {
  const { t } = useTranslation();
  const [showAdminAlert, setShowAdminAlert] = useState(true);

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Settings
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
            {editingLink
              ? t('pages.redirectLink.configureLink.editTitle')
              : t('pages.redirectLink.configureLink.title')}
          </span>
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {editingLink
              ? t('pages.redirectLink.configureLink.editDescription')
              : t('pages.redirectLink.configureLink.description')}
          </p>
        </div>
      </div>

      <div className="space-y-4">
        {/* Editing Alert - Glassmorphism */}
        {editingLink && (
          <div
            className="rounded-xl xs:rounded-2xl border p-3 xs:p-4"
            role="alert"
            style={{
              backgroundColor: 'rgba(180, 120, 60, 0.08)',
              borderColor: 'rgba(180, 120, 60, 0.25)',
              backdropFilter: 'blur(12px) saturate(180%)',
              WebkitBackdropFilter: 'blur(12px) saturate(180%)',
            }}
          >
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <div
                  className="p-1 xs:p-1.5 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-3.5 h-3.5 xs:w-4 xs:h-4"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-xs md:text-sm leading-relaxed"
                  style={{ color: 'rgba(120, 75, 30, 0.8)' }}
                >
                  {t('pages.redirectLink.alerts.editing')}
                </span>
              </div>
              <Button variant="ghost" size="sm" onClick={onCancelEdit}>
                {t('pages.redirectLink.buttons.cancel')}
              </Button>
            </div>
          </div>
        )}

        {/* Admin Only Alert - Glassmorphism */}
        {showAdminAlert && (
          <div
            className="rounded-xl xs:rounded-2xl border p-3 xs:p-4"
            role="alert"
            style={{
              backgroundColor: 'rgba(180, 120, 60, 0.08)',
              borderColor: 'rgba(180, 120, 60, 0.25)',
              backdropFilter: 'blur(12px) saturate(180%)',
              WebkitBackdropFilter: 'blur(12px) saturate(180%)',
            }}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <div
                  className="p-1 xs:p-1.5 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-3.5 h-3.5 xs:w-4 xs:h-4"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-xs md:text-sm leading-relaxed"
                  style={{ color: 'rgba(120, 75, 30, 0.8)' }}
                >
                  {t('pages.redirectLink.alerts.adminOnly')}
                </span>
              </div>
              <button
                type="button"
                onClick={() => setShowAdminAlert(false)}
                className="shrink-0 p-1 rounded-md hover:bg-black/5 transition-colors cursor-pointer"
                style={{ color: 'rgba(120, 75, 30, 0.6)' }}
              >
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        )}

        <div className="space-y-2">
          <label
            htmlFor="linkName"
            className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
          >
            {t('pages.redirectLink.form.name')}
          </label>
          <input
            id="linkName"
            placeholder={t('pages.redirectLink.form.namePlaceholder')}
            value={linkName}
            onChange={(e) => onLinkNameChange(e.target.value)}
            className="w-full rounded-2xl !border-2 !border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base text-gray-900 placeholder-gray-400 outline-none transition-all hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
          />
          <p className="text-xs text-gray-400">{t('pages.redirectLink.form.nameHint')}</p>
        </div>

        <div className="space-y-2">
          <label
            htmlFor="slug"
            className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
          >
            {t('pages.redirectLink.form.slug')}
          </label>
          <div className="flex">
            <span className="flex items-center px-3 rounded-l-2xl border-2 border-r-0 border-[#8D2D53] bg-[#8D2D53] text-white text-xs xs:text-sm font-medium">
              {t('pages.redirectLink.form.slugPrefix')}
            </span>
            <input
              id="slug"
              placeholder={t('pages.redirectLink.form.slugPlaceholder')}
              value={slug}
              onChange={(e) => onSlugChange(e.target.value)}
              disabled={!!editingLink}
              className="flex-1 rounded-r-2xl !border-2 !border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base text-gray-900 placeholder-gray-400 outline-none transition-all hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30 disabled:opacity-50 disabled:cursor-not-allowed"
            />
          </div>
          <p className="text-xs text-gray-400">
            {editingLink
              ? t('pages.redirectLink.form.slugEditHint')
              : t('pages.redirectLink.form.slugHint')}
          </p>
        </div>

        <div className="space-y-2">
          <Label>{t('pages.redirectLink.form.strategy')}</Label>
          <div className="grid grid-cols-2 gap-3">
            <button
              type="button"
              onClick={() => onStrategyChange('click_limit')}
              className="group relative flex flex-col items-center gap-2 rounded-xl border-2 p-3 xs:p-4 text-center transition-all cursor-pointer"
              style={{
                borderColor:
                  distributionStrategy === 'click_limit' ? colors.wine : 'rgba(141, 45, 83, 0.15)',
                backgroundColor:
                  distributionStrategy === 'click_limit'
                    ? 'rgba(141, 45, 83, 0.1)'
                    : 'rgba(255, 255, 255, 0.5)',
              }}
            >
              <div
                className="flex h-9 w-9 xs:h-10 xs:w-10 items-center justify-center rounded-lg transition-colors"
                style={{
                  backgroundColor:
                    distributionStrategy === 'click_limit'
                      ? 'rgba(141, 45, 83, 0.15)'
                      : 'rgba(141, 45, 83, 0.06)',
                }}
              >
                <MousePointerClick
                  className="h-4 w-4 xs:h-5 xs:w-5 transition-colors"
                  style={{
                    color:
                      distributionStrategy === 'click_limit' ? colors.wine : colors.wineTextMuted,
                  }}
                />
              </div>
              <span
                className="text-xs xs:text-sm font-medium transition-colors"
                style={{
                  color:
                    distributionStrategy === 'click_limit' ? colors.wine : colors.wineTextMuted,
                }}
              >
                {t('pages.redirectLink.form.clickLimit')}
              </span>
              <div
                className="absolute top-2 right-2 flex h-4 w-4 xs:h-5 xs:w-5 items-center justify-center rounded-full border-2 transition-all"
                style={{
                  borderColor:
                    distributionStrategy === 'click_limit'
                      ? colors.wine
                      : 'rgba(141, 45, 83, 0.25)',
                  backgroundColor:
                    distributionStrategy === 'click_limit' ? colors.wine : 'transparent',
                }}
              >
                {distributionStrategy === 'click_limit' && (
                  <div className="h-1.5 w-1.5 xs:h-2 xs:w-2 rounded-full bg-white" />
                )}
              </div>
            </button>

            <button
              type="button"
              onClick={() => onStrategyChange('member_limit')}
              className="group relative flex flex-col items-center gap-2 rounded-xl border-2 p-3 xs:p-4 text-center transition-all cursor-pointer"
              style={{
                borderColor:
                  distributionStrategy === 'member_limit' ? colors.wine : 'rgba(141, 45, 83, 0.15)',
                backgroundColor:
                  distributionStrategy === 'member_limit'
                    ? 'rgba(141, 45, 83, 0.1)'
                    : 'rgba(255, 255, 255, 0.5)',
              }}
            >
              <div
                className="flex h-9 w-9 xs:h-10 xs:w-10 items-center justify-center rounded-lg transition-colors"
                style={{
                  backgroundColor:
                    distributionStrategy === 'member_limit'
                      ? 'rgba(141, 45, 83, 0.15)'
                      : 'rgba(141, 45, 83, 0.06)',
                }}
              >
                <Users
                  className="h-4 w-4 xs:h-5 xs:w-5 transition-colors"
                  style={{
                    color:
                      distributionStrategy === 'member_limit' ? colors.wine : colors.wineTextMuted,
                  }}
                />
              </div>
              <span
                className="text-xs xs:text-sm font-medium transition-colors"
                style={{
                  color:
                    distributionStrategy === 'member_limit' ? colors.wine : colors.wineTextMuted,
                }}
              >
                {t('pages.redirectLink.form.memberLimit')}
              </span>
              <div
                className="absolute top-2 right-2 flex h-4 w-4 xs:h-5 xs:w-5 items-center justify-center rounded-full border-2 transition-all"
                style={{
                  borderColor:
                    distributionStrategy === 'member_limit'
                      ? colors.wine
                      : 'rgba(141, 45, 83, 0.25)',
                  backgroundColor:
                    distributionStrategy === 'member_limit' ? colors.wine : 'transparent',
                }}
              >
                {distributionStrategy === 'member_limit' && (
                  <div className="h-1.5 w-1.5 xs:h-2 xs:w-2 rounded-full bg-white" />
                )}
              </div>
            </button>
          </div>
          <p className="text-xs text-gray-400">
            {distributionStrategy === 'click_limit'
              ? t('pages.redirectLink.form.clickLimitHint')
              : t('pages.redirectLink.form.memberLimitHint')}
          </p>
        </div>

        <div className="space-y-2">
          <Label>{t('pages.redirectLink.form.campaignGroups')}</Label>
          <button
            type="button"
            onClick={onOpenGroupDialog}
            className="inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-full !border !border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#520B27] shadow-sm transition-all hover:-translate-y-0.5 hover:!border-gray-400 hover:shadow-md"
          >
            {t('pages.redirectLink.form.selectGroups')} ({selectedGroupsCount})
          </button>
          <p className="text-xs text-gray-400">
            {selectedGroupsCount} {t('pages.redirectLink.form.groupsSelected')}
          </p>
        </div>

        <button
          type="button"
          onClick={onSubmit}
          disabled={isSubmitDisabled || loading}
          className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
        >
          <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
            <div className="auth-btn-glow__glow absolute inset-0" />
          </div>
          <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
            {loading
              ? t('pages.redirectLink.buttons.saving')
              : editingLink
                ? t('pages.redirectLink.buttons.update')
                : t('pages.redirectLink.buttons.create')}
          </span>
        </button>
      </div>
    </div>
  );
}
