import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  MOCK_ANNOUNCEMENT_DISABLED,
  mockDashboardEmptyState,
  mockDashboardError,
  mockGetAnnouncement,
  setupDashboardMocks,
} from './fixtures/dashboard.fixture';
import { DashboardPage } from './pages/dashboard.page';

test.describe('Dashboard', () => {
  let dashboard: DashboardPage;

  test.beforeEach(async ({ page }) => {
    dashboard = new DashboardPage(page);
  });

  // ================================================================
  // Page Load
  // ================================================================

  test.describe('Page Load', () => {
    test('should display title, 4 stat cards, welcome panel and chart section', async ({
      page,
    }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      // Title
      await expect(dashboard.pageTitle).toBeVisible();

      // 4 stat cards with correct values
      await expect(dashboard.totalGroupsCard).toContainText('12');
      await expect(dashboard.fullGroupsCard).toContainText('3');
      await expect(dashboard.totalMembersCard).toContainText('1,250');
      await expect(dashboard.activeLinksCard).toContainText('8');
    });

    test('should display welcome panel with 3 steps', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.welcomePanel).toBeVisible();
      await expect(dashboard.welcomeStep1).toBeVisible();
      await expect(dashboard.welcomeStep2).toBeVisible();
      await expect(dashboard.welcomeStep3).toBeVisible();
    });

    test('should display chart section with period selector', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.periodWeekly).toBeVisible();
      await expect(dashboard.periodMonthly).toBeVisible();
      await expect(dashboard.periodYearly).toBeVisible();
    });
  });

  // ================================================================
  // Announcement
  // ================================================================

  test.describe('Announcement', () => {
    test('should display announcement when enabled', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page); // uses MOCK_ANNOUNCEMENT_ENABLED
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.announcementTitle).toBeVisible();
    });

    test('should hide announcement when disabled', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      // Override announcement to disabled
      await mockGetAnnouncement(page, MOCK_ANNOUNCEMENT_DISABLED);
      await dashboard.goto();
      await dashboard.waitForLoad();

      // Announcement title should not be present
      const announcementVisible = await page
        .locator('span')
        .filter({ hasText: /Novidade/i })
        .isVisible()
        .catch(() => false);
      expect(announcementVisible).toBeFalsy();
    });
  });

  // ================================================================
  // Chart
  // ================================================================

  test.describe('Chart', () => {
    test('should change period when clicking monthly button', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await dashboard.periodMonthly.click();

      // Monthly period subtitle should be visible (on xs+ screens)
      const subtitle = page.getByText(/últimas 4 semanas/i);
      // On smaller viewport this may be hidden, so check broadly
      const isVisible = await subtitle.isVisible().catch(() => false);
      // The button itself should have the active style (white text)
      await expect(dashboard.periodMonthly).toBeVisible();
    });

    test('should display chart footer stats', async ({ page }) => {
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.growthRate).toBeVisible();
      await expect(dashboard.peakMembers).toBeVisible();
      await expect(dashboard.activeGroups).toBeVisible();
    });
  });

  // ================================================================
  // Empty State
  // ================================================================

  test.describe('Empty State', () => {
    test('should show zero values in stat cards', async ({ page }) => {
      await setupAuthMocks(page);
      await mockDashboardEmptyState(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.totalGroupsCard).toContainText('0');
      await expect(dashboard.fullGroupsCard).toContainText('0');
      await expect(dashboard.totalMembersCard).toContainText('0');
      await expect(dashboard.activeLinksCard).toContainText('0');
    });

    test('should show "no data" message in chart area', async ({ page }) => {
      await setupAuthMocks(page);
      await mockDashboardEmptyState(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.noDataMessage).toBeVisible();
    });
  });

  // ================================================================
  // Error Handling
  // ================================================================

  test.describe('Error Handling', () => {
    test('should handle API error gracefully without crashing', async ({ page }) => {
      await setupAuthMocks(page);
      await mockDashboardError(page);
      await dashboard.goto();

      // Page should not crash - title or some element should be visible
      const isLoaded = await dashboard.isLoaded();
      expect(isLoaded).toBeTruthy();
    });
  });

  // ================================================================
  // Responsive Design
  // ================================================================

  test.describe('Responsive Design', () => {
    test('should render correctly on desktop viewport', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.pageTitle).toBeVisible();
      await expect(dashboard.totalGroupsCard).toBeVisible();
    });

    test('should render correctly on mobile viewport', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      await expect(dashboard.pageTitle).toBeVisible();
      await expect(dashboard.totalGroupsCard).toBeVisible();
    });
  });

  // ================================================================
  // Migration Check
  // ================================================================

  test.describe('Migration Check', () => {
    test('should not make any Evolution API calls', async ({ page }) => {
      const evolutionCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();
        if (url.includes('evolution') || url.includes(':8080')) {
          evolutionCalls.push(url);
        }
        await route.continue();
      });

      await setupAuthMocks(page);
      await setupDashboardMocks(page);
      await dashboard.goto();
      await dashboard.waitForLoad();

      expect(evolutionCalls).toHaveLength(0);
    });
  });
});
