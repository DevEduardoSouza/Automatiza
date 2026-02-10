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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Shield } from 'lucide-react';
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
};

/**
 * Admin dashboard page with multiple management tabs.
 */
export default function Admin() {
  const { t } = useTranslation();

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <Shield
                  className="w-5 h-5 xs:w-6 xs:h-6"
                  style={{ color: colors.wine }}
                  aria-hidden="true"
                />
              </div>
              <div>
                <h1
                  className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('admin.title')}
                </h1>
                <p
                  className="text-xs xs:text-sm md:text-base tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('admin.subtitle')}
                </p>
              </div>
            </div>

            <Tabs defaultValue="dashboard" className="space-y-4">
              <TabsList
                className="grid grid-cols-4 md:grid-cols-7 h-auto gap-1"
                aria-label={t('admin.title')}
              >
                <TabsTrigger value="dashboard" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.dashboard')}
                </TabsTrigger>
                <TabsTrigger value="users" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.users')}
                </TabsTrigger>
                <TabsTrigger value="plans" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.plans')}
                </TabsTrigger>
                <TabsTrigger value="categories" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.categories')}
                </TabsTrigger>
                <TabsTrigger value="team" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.team')}
                </TabsTrigger>
                <TabsTrigger value="emails" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.emails')}
                </TabsTrigger>
                <TabsTrigger value="settings" className="text-xs md:text-sm px-2 md:px-3">
                  {t('admin.tabs.settings')}
                </TabsTrigger>
              </TabsList>

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
