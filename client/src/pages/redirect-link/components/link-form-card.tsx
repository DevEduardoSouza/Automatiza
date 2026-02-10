import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AlertCircle, Settings } from 'lucide-react';
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
        </div>

        <div className="space-y-2">
          <Label htmlFor="linkName">{t('pages.redirectLink.form.name')}</Label>
          <Input
            id="linkName"
            placeholder={t('pages.redirectLink.form.namePlaceholder')}
            value={linkName}
            onChange={(e) => onLinkNameChange(e.target.value)}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('pages.redirectLink.form.nameHint')}
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="slug">{t('pages.redirectLink.form.slug')}</Label>
          <div className="flex gap-2">
            <span className="flex items-center px-3 bg-secondary rounded-l-lg text-muted-foreground">
              {t('pages.redirectLink.form.slugPrefix')}
            </span>
            <Input
              id="slug"
              placeholder={t('pages.redirectLink.form.slugPlaceholder')}
              value={slug}
              onChange={(e) => onSlugChange(e.target.value)}
              className="flex-1 rounded-l-none"
              disabled={!!editingLink}
            />
          </div>
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {editingLink
              ? t('pages.redirectLink.form.slugEditHint')
              : t('pages.redirectLink.form.slugHint')}
          </p>
        </div>

        <div className="space-y-2">
          <Label>{t('pages.redirectLink.form.strategy')}</Label>
          <div className="flex gap-4">
            <div className="flex items-center space-x-2">
              <input
                type="radio"
                id="click_limit"
                name="strategy"
                value="click_limit"
                checked={distributionStrategy === 'click_limit'}
                onChange={() => onStrategyChange('click_limit')}
                className="h-4 w-4"
              />
              <Label htmlFor="click_limit" className="font-normal cursor-pointer">
                {t('pages.redirectLink.form.clickLimit')}
              </Label>
            </div>
            <div className="flex items-center space-x-2">
              <input
                type="radio"
                id="member_limit"
                name="strategy"
                value="member_limit"
                checked={distributionStrategy === 'member_limit'}
                onChange={() => onStrategyChange('member_limit')}
                className="h-4 w-4"
              />
              <Label htmlFor="member_limit" className="font-normal cursor-pointer">
                {t('pages.redirectLink.form.memberLimit')}
              </Label>
            </div>
          </div>
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {distributionStrategy === 'click_limit'
              ? t('pages.redirectLink.form.clickLimitHint')
              : t('pages.redirectLink.form.memberLimitHint')}
          </p>
        </div>

        <div className="space-y-2">
          <Label>{t('pages.redirectLink.form.campaignGroups')}</Label>
          <Button variant="outline" className="w-full" onClick={onOpenGroupDialog}>
            {t('pages.redirectLink.form.selectGroups')} ({selectedGroupsCount})
          </Button>
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {selectedGroupsCount} {t('pages.redirectLink.form.groupsSelected')}
          </p>
        </div>

        <Button onClick={onSubmit} disabled={isSubmitDisabled || loading} className="w-full">
          {loading
            ? t('pages.redirectLink.buttons.saving')
            : editingLink
              ? t('pages.redirectLink.buttons.update')
              : t('pages.redirectLink.buttons.create')}
        </Button>
      </div>
    </div>
  );
}
