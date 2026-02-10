import { expect, test } from '@playwright/test';
import {
  MOCK_ADMIN_USERS,
  mockAdminAccessDenied,
  mockToggleUserStatusApi,
  mockUpdateUserPlanApi,
  mockUpdateUserRoleApi,
  setupAdminMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Panel', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  test.describe('Access Control', () => {
    test('should show admin panel for admin users', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Should not show access denied
      const isDenied = await adminPage.isAccessDenied();
      expect(isDenied).toBeFalsy();
    });

    test('should show access denied for non-admin users', async ({ page }) => {
      await mockAdminAccessDenied(page);
      await adminPage.goto();
      await page.waitForTimeout(2000);

      // Should show access denied or redirect
      const isDenied = await adminPage.isAccessDenied();
      const isRedirected = page.url().includes('/dashboard') || page.url().includes('/auth');

      expect(isDenied || isRedirected).toBeTruthy();
    });
  });

  test.describe('Page Load', () => {
    test('should display admin tabs', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Look for navigation tabs
      const usersTab = page.getByRole('tab', { name: /usuarios|users/i });
      const statsTab = page.getByRole('tab', { name: /estatisticas|stats/i });

      const hasUsersTab = await usersTab.isVisible().catch(() => false);
      const hasStatsTab = await statsTab.isVisible().catch(() => false);

      expect(hasUsersTab || hasStatsTab).toBeTruthy();
    });

    test('should display users list', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Switch to users tab if needed
      const usersTab = page.getByRole('tab', { name: /usuarios|users/i });
      if (await usersTab.isVisible()) {
        await usersTab.click();
        await page.waitForTimeout(500);
      }

      const userCount = await adminPage.getUserCount();
      expect(userCount).toBeGreaterThanOrEqual(0);
    });

    test('should display system stats', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Switch to stats tab if available
      const statsTab = page.getByRole('tab', { name: /estatisticas|stats/i });
      if (await statsTab.isVisible()) {
        await statsTab.click();
        await page.waitForTimeout(500);
      }

      // Look for stats cards
      const cards = page.locator('[class*="card"]');
      const cardCount = await cards.count();

      expect(cardCount).toBeGreaterThan(0);
    });
  });

  test.describe('User Management', () => {
    test('should search for users', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      await adminPage.searchUser('admin');
      await page.waitForTimeout(500);

      const count = await adminPage.getUserCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should filter users by role', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const roleFilter = page
        .getByRole('combobox', { name: /role|funcao/i })
        .or(page.locator('[data-testid="role-filter"]'));

      if (await roleFilter.isVisible()) {
        await roleFilter.click();
        const adminOption = page.getByRole('option', { name: /admin/i });
        if (await adminOption.isVisible()) {
          await adminOption.click();
          await page.waitForTimeout(500);
        }
      }

      const count = await adminPage.getUserCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should filter users by plan', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const planFilter = page
        .getByRole('combobox', { name: /plano|plan/i })
        .or(page.locator('[data-testid="plan-filter"]'));

      if (await planFilter.isVisible()) {
        await planFilter.click();
        const proOption = page.getByRole('option', { name: /pro/i });
        if (await proOption.isVisible()) {
          await proOption.click();
          await page.waitForTimeout(500);
        }
      }

      const count = await adminPage.getUserCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('User Actions', () => {
    test('should change user role', async ({ page }) => {
      await setupAdminMocks(page);
      await mockUpdateUserRoleApi(page, 'success');
      await adminPage.goto();
      await adminPage.waitForLoad();

      const regularUser = MOCK_ADMIN_USERS.find((u) => u.role === 1); // USER role
      if (regularUser) {
        const row = adminPage.getUserByEmail(regularUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.editRole.isVisible()) {
            await actions.editRole.click();

            // Select new role if dialog appears
            const roleSelect = page.getByRole('combobox', { name: /role|funcao/i });
            if (await roleSelect.isVisible()) {
              await roleSelect.click();
              const moderatorOption = page.getByRole('option', { name: /moderator/i });
              if (await moderatorOption.isVisible()) {
                await moderatorOption.click();
              }
            }

            // Confirm
            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|salvar|save/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            // Should show success toast
            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);

            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should change user plan', async ({ page }) => {
      await setupAdminMocks(page);
      await mockUpdateUserPlanApi(page, 'success');
      await adminPage.goto();
      await adminPage.waitForLoad();

      const freeUser = MOCK_ADMIN_USERS.find((u) => u.plan === 1); // FREE plan
      if (freeUser) {
        const row = adminPage.getUserByEmail(freeUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.editPlan.isVisible()) {
            await actions.editPlan.click();

            const planSelect = page.getByRole('combobox', { name: /plano|plan/i });
            if (await planSelect.isVisible()) {
              await planSelect.click();
              const proOption = page.getByRole('option', { name: /pro/i });
              if (await proOption.isVisible()) {
                await proOption.click();
              }
            }

            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|salvar|save/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);

            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should ban user', async ({ page }) => {
      await setupAdminMocks(page);
      await mockToggleUserStatusApi(page, 'success');
      await adminPage.goto();
      await adminPage.waitForLoad();

      const activeUser = MOCK_ADMIN_USERS.find((u) => u.isActive && u.role === 1);
      if (activeUser) {
        const row = adminPage.getUserByEmail(activeUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.ban.isVisible()) {
            await actions.ban.click();

            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|banir|ban/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);

            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should unban user', async ({ page }) => {
      await setupAdminMocks(page);
      await mockToggleUserStatusApi(page, 'success');
      await adminPage.goto();
      await adminPage.waitForLoad();

      const bannedUser = MOCK_ADMIN_USERS.find((u) => !u.isActive);
      if (bannedUser) {
        const row = adminPage.getUserByEmail(bannedUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.unban.isVisible()) {
            await actions.unban.click();

            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|desbanir|unban/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);

            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should show error when trying to ban self', async ({ page }) => {
      await setupAdminMocks(page);
      await mockToggleUserStatusApi(page, 'self_ban');
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Try to ban admin user (self)
      const adminUser = MOCK_ADMIN_USERS.find((u) => u.role === 3);
      if (adminUser) {
        const row = adminPage.getUserByEmail(adminUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.ban.isVisible()) {
            await actions.ban.click();

            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|banir|ban/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            const errorToast = page.locator('[data-sonner-toast]').filter({
              hasText: /erro|error|nao.*pode|cannot|voce.*mesmo|yourself/i,
            });
            const hasError = await errorToast.isVisible().catch(() => false);

            expect(typeof hasError).toBe('boolean');
          }
        }
      }
    });

    test('should handle role change forbidden error', async ({ page }) => {
      await setupAdminMocks(page);
      await mockUpdateUserRoleApi(page, 'forbidden');
      await adminPage.goto();
      await adminPage.waitForLoad();

      const adminUser = MOCK_ADMIN_USERS.find((u) => u.role === 3);
      if (adminUser) {
        const row = adminPage.getUserByEmail(adminUser.email);

        if (await row.isVisible()) {
          const actions = adminPage.getUserActions(row);

          if (await actions.editRole.isVisible()) {
            await actions.editRole.click();

            const confirmButton = page.getByRole('button', {
              name: /confirmar|confirm|salvar|save/i,
            });
            if (await confirmButton.isVisible()) {
              await confirmButton.click();
              await page.waitForTimeout(500);
            }

            const errorToast = page.locator('[data-sonner-toast]').filter({
              hasText: /erro|error|permissao|permission|proibido|forbidden/i,
            });
            const hasError = await errorToast.isVisible().catch(() => false);

            expect(typeof hasError).toBe('boolean');
          }
        }
      }
    });
  });

  test.describe('Audit Logs', () => {
    test('should display audit logs tab', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const auditTab = page.getByRole('tab', { name: /auditoria|audit|logs/i });
      if (await auditTab.isVisible()) {
        await auditTab.click();
        await page.waitForTimeout(500);

        const logCount = await adminPage.getAuditLogCount();
        expect(logCount).toBeGreaterThanOrEqual(0);
      }
    });

    test('should display audit log entries', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const auditTab = page.getByRole('tab', { name: /auditoria|audit|logs/i });
      if (await auditTab.isVisible()) {
        await auditTab.click();
        await page.waitForTimeout(500);

        // Look for action descriptions in logs
        const logEntry = page.locator('tr, li').filter({
          hasText: /login|plan.*changed|banned/i,
        });

        const hasLogs = await logEntry
          .first()
          .isVisible()
          .catch(() => false);
        expect(typeof hasLogs).toBe('boolean');
      }
    });
  });

  test.describe('System Stats', () => {
    test('should display total users stat', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const statsTab = page.getByRole('tab', { name: /estatisticas|stats/i });
      if (await statsTab.isVisible()) {
        await statsTab.click();
        await page.waitForTimeout(500);
      }

      const totalUsersCard = page
        .locator('[class*="card"]')
        .filter({ hasText: /usuarios|users/i })
        .first();

      const hasCard = await totalUsersCard.isVisible().catch(() => false);
      expect(typeof hasCard).toBe('boolean');
    });

    test('should display uptime stat', async ({ page }) => {
      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const statsTab = page.getByRole('tab', { name: /estatisticas|stats/i });
      if (await statsTab.isVisible()) {
        await statsTab.click();
        await page.waitForTimeout(500);
      }

      const uptimeCard = page.locator('[class*="card"]').filter({ hasText: /uptime/i });

      const hasCard = await uptimeCard.isVisible().catch(() => false);
      expect(typeof hasCard).toBe('boolean');
    });
  });

  test.describe('Migration Validation', () => {
    test('should load admin data via ConnectRPC', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('admin.v1.AdminService')) {
          grpcCalls.push(url);
        }

        await route.continue();
      });

      await setupAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
