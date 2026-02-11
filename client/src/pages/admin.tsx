import {
  CategoriesTab,
  DashboardTab,
  EmailTemplatesTab,
  PlansTab,
  SettingsTab,
  TeamTab,
  UsersTab,
} from '@/components/admin';
import { ErrorBoundary } from '@/components/error-boundary';
import { Tabs, TabsContent } from '@/components/ui/tabs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const AdminIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
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
    <path d="M5 20.5H19" />
    <path d="M16.8717 17.5H7.1283C6.10017 17.5 5.58611 17.5 5.19623 17.2234C4.80634 16.9468 4.63649 16.4616 4.29679 15.4912L2.05123 9.07668C1.93172 8.72325 2.02503 8.3336 2.29225 8.07016C2.62854 7.73864 3.15545 7.6872 3.55117 7.94727L4.78349 8.75718C6.02739 9.5747 6.64935 9.98345 7.27815 9.83488C7.90696 9.68631 8.28019 9.04241 9.02665 7.75461L11.2412 3.93412C11.3968 3.66567 11.6864 3.5 12 3.5C12.3136 3.5 12.6032 3.66567 12.7588 3.93412L14.9733 7.75461C15.7198 9.04241 16.093 9.68631 16.7218 9.83488C17.3507 9.98345 17.9726 9.5747 19.2165 8.75718L20.4488 7.94727C20.8445 7.6872 21.3715 7.73864 21.7078 8.07016C21.975 8.3336 22.0683 8.72325 21.9488 9.07668L19.7032 15.4912C19.3635 16.4616 19.1937 16.9468 18.8038 17.2234C18.4139 17.5 17.8998 17.5 16.8717 17.5Z" />
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
 * Admin dashboard page with multiple management tabs.
 */
export default function Admin() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('dashboard');

  const tabs = ['dashboard', 'users', 'plans', 'categories', 'team', 'emails', 'settings'] as const;

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 xl:space-y-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 sm:p-2.5 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <AdminIcon
                  className="w-5 h-5 xs:w-6 xs:h-6 sm:w-7 sm:h-7"
                  style={{ color: colors.wine }}
                  aria-hidden="true"
                />
              </div>
              <div>
                <h2
                  className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('admin.title')}
                </h2>
                <p
                  className="text-xs xs:text-sm tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('admin.subtitle')}
                </p>
              </div>
            </div>

            <div className="h-px w-full" style={{ backgroundColor: colors.wineBorder }} />

            <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4 xs:space-y-5 sm:space-y-6">
              <div
                className="flex flex-wrap gap-1.5 xs:gap-2"
                role="tablist"
                aria-label={t('admin.title')}
              >
                {tabs.map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    role="tab"
                    aria-selected={activeTab === tab}
                    onClick={() => setActiveTab(tab)}
                    className={`inline-flex cursor-pointer items-center justify-center rounded-full px-3 xs:px-4 py-1.5 xs:py-2 text-xs xs:text-sm font-semibold transition-all ${
                      activeTab === tab
                        ? 'bg-[#8D2D53] text-white shadow-sm hover:bg-[#8D2D53]/90'
                        : 'border border-gray-300 bg-white text-[#8D2D53] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md'
                    }`}
                  >
                    {t(`admin.tabs.${tab}`)}
                  </button>
                ))}
              </div>

              <TabsContent value="dashboard">
                <DashboardTab />
              </TabsContent>

              <TabsContent value="users">
                <UsersTab />
              </TabsContent>

              <TabsContent value="plans">
                <PlansTab />
              </TabsContent>

              <TabsContent value="categories">
                <CategoriesTab />
              </TabsContent>

              <TabsContent value="team">
                <TeamTab />
              </TabsContent>

              <TabsContent value="emails">
                <EmailTemplatesTab />
              </TabsContent>

              <TabsContent value="settings">
                <SettingsTab />
              </TabsContent>
            </Tabs>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
