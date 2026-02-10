import { ErrorBoundary } from '@/components/error-boundary';
import { SessionList } from '@/components/sessions';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useProfilePage } from '@/hooks/use-profile-page';
import { Eye, EyeOff, Lock, Phone } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const ProfileIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <path d="M2 12C2 8.22876 2 6.34315 3.17157 5.17157C4.34315 4 6.22876 4 10 4H14C17.7712 4 19.6569 4 20.8284 5.17157C22 6.34315 22 8.22876 22 12C22 15.7712 22 17.6569 20.8284 18.8284C19.6569 20 17.7712 20 14 20H10C6.22876 20 4.34315 20 3.17157 18.8284C2 17.6569 2 15.7712 2 12Z" />
    <path d="M9 12.5C7.61929 12.5 6.5 11.3807 6.5 10C6.5 8.61929 7.61929 7.5 9 7.5C10.3807 7.5 11.5 8.61929 11.5 10C11.5 11.3807 10.3807 12.5 9 12.5ZM9 12.5C11.2091 12.5 13 14.2909 13 16.5M9 12.5C6.79086 12.5 5 14.2909 5 16.5" />
    <path d="M15 9H19" />
    <path d="M15 12H19" />
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
};

/**
 * User profile page component
 */
export default function Profile() {
  const { t } = useTranslation();
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const {
    loading,
    profile,
    connectedInstancesCount,
    phoneDraft,
    setPhoneDraft,
    savingPhone,
    savePhone,
    currentPassword,
    setCurrentPassword,
    newPassword,
    setNewPassword,
    confirmPassword,
    setConfirmPassword,
    savingPassword,
    savePassword,
    automationsLimitText,
    formatDateBR,
  } = useProfilePage();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-9 xl:space-y-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <ProfileIcon
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
                  {t('pages.profile.title')}
                </h2>
                <p
                  className="text-xs xs:text-sm tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.profile.subtitle')}
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-3 xs:gap-4 sm:gap-6 lg:gap-8 xl:gap-10">
              <div
                className="lg:col-span-2 rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 sm:mb-5">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <ProfileIcon
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                      aria-hidden="true"
                    />
                  </div>
                  <span
                    className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                    style={{ color: colors.wineText }}
                  >
                    {t('pages.profile.accountInfo.title')}
                  </span>
                </div>
                <div className="space-y-3 xs:space-y-4 sm:space-y-5">
                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 xs:gap-4 sm:gap-5 lg:gap-6">
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.email')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">{profile?.email || '-'}</div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.plan')}
                      </Label>
                      <div className="flex items-center gap-2">
                        <span
                          className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium text-white"
                          style={{ backgroundColor: colors.wine }}
                        >
                          {(profile?.plan || 'free').toUpperCase()}
                        </span>
                        <span className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
                          {automationsLimitText}
                        </span>
                      </div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.nextPayment')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">
                        {formatDateBR(profile?.billing_next_payment_at ?? null)}
                      </div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.accessUntil')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">
                        {formatDateBR(profile?.plan_access_until ?? null)}
                      </div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.userId')}
                      </Label>
                      <div className="text-sm xs:text-base text-black font-mono break-all">
                        {profile?.id || '-'}
                      </div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.connectedWhatsApp')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">
                        {connectedInstancesCount} {t('pages.profile.accountInfo.instances')}
                      </div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.createdAt')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">{formatDateBR(profile?.created_at)}</div>
                    </div>
                    <div className="space-y-1">
                      <Label
                        className="text-xs xs:text-sm font-medium"
                        style={{ color: colors.wineTextLight }}
                      >
                        {t('pages.profile.accountInfo.updatedAt')}
                      </Label>
                      <div className="text-sm xs:text-base text-black">{formatDateBR(profile?.updated_at)}</div>
                    </div>
                  </div>

                  {loading && (
                    <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                      {t('pages.profile.loading')}
                    </p>
                  )}
                </div>
              </div>

              <div
                className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 sm:mb-5">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <Phone
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                      aria-hidden="true"
                    />
                  </div>
                  <span
                    className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                    style={{ color: colors.wineText }}
                  >
                    {t('pages.profile.phone.title')}
                  </span>
                </div>
                <div className="space-y-3 xs:space-y-4">
                  <div className="space-y-2">
                    <Label
                      htmlFor="phone_number"
                      className="text-xs xs:text-sm font-medium"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.profile.phone.label')}
                    </Label>
                    <Input
                      id="phone_number"
                      placeholder={t('pages.profile.phone.placeholder')}
                      value={phoneDraft}
                      onChange={(e) => setPhoneDraft(e.target.value)}
                      className="text-black"
                    />
                    <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                      {t('pages.profile.phone.hint')}
                    </p>
                  </div>
                  <Button className="w-full" onClick={savePhone} disabled={savingPhone || loading}>
                    {savingPhone ? t('pages.profile.phone.saving') : t('pages.profile.phone.save')}
                  </Button>
                </div>
              </div>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 md:p-7 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 sm:mb-5">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <Lock
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                  style={{ color: colors.wineText }}
                >
                  {t('pages.profile.security.title')}
                </span>
              </div>
              <div className="space-y-3 xs:space-y-4 sm:space-y-5">
                <div className="space-y-2">
                  <Label
                    htmlFor="current_password"
                    className="text-xs xs:text-sm font-medium"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.profile.security.currentPassword')}
                  </Label>
                  <div className="relative">
                    <Input
                      id="current_password"
                      type={showCurrentPassword ? 'text' : 'password'}
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      placeholder={t('pages.profile.security.placeholder')}
                      className="pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowCurrentPassword(!showCurrentPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 transition-colors"
                      style={{ color: colors.wineTextMuted }}
                      aria-label={showCurrentPassword ? 'Hide password' : 'Show password'}
                    >
                      {showCurrentPassword ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 xs:gap-4 sm:gap-5 lg:gap-6">
                  <div className="space-y-2">
                    <Label
                      htmlFor="new_password"
                      className="text-xs xs:text-sm font-medium"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.profile.security.newPassword')}
                    </Label>
                    <div className="relative">
                      <Input
                        id="new_password"
                        type={showNewPassword ? 'text' : 'password'}
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        placeholder={t('pages.profile.security.placeholder')}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowNewPassword(!showNewPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 transition-colors"
                        style={{ color: colors.wineTextMuted }}
                        aria-label={showNewPassword ? 'Hide password' : 'Show password'}
                      >
                        {showNewPassword ? (
                          <EyeOff className="h-4 w-4" />
                        ) : (
                          <Eye className="h-4 w-4" />
                        )}
                      </button>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <Label
                      htmlFor="confirm_password"
                      className="text-xs xs:text-sm font-medium"
                      style={{ color: colors.wineTextLight }}
                    >
                      {t('pages.profile.security.confirmPassword')}
                    </Label>
                    <div className="relative">
                      <Input
                        id="confirm_password"
                        type={showConfirmPassword ? 'text' : 'password'}
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        placeholder={t('pages.profile.security.placeholder')}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 transition-colors"
                        style={{ color: colors.wineTextMuted }}
                        aria-label={showConfirmPassword ? 'Hide password' : 'Show password'}
                      >
                        {showConfirmPassword ? (
                          <EyeOff className="h-4 w-4" />
                        ) : (
                          <Eye className="h-4 w-4" />
                        )}
                      </button>
                    </div>
                  </div>
                </div>
                <div className="flex justify-end">
                  <Button onClick={savePassword} disabled={savingPassword || loading}>
                    {savingPassword
                      ? t('pages.profile.security.saving')
                      : t('pages.profile.security.save')}
                  </Button>
                </div>
              </div>
            </div>

            <SessionList />
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
