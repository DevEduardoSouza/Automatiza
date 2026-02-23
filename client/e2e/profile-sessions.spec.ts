import { expect, test } from '@playwright/test';
import { setupAuthMocks, waitForToast } from './fixtures/auth.fixture';
import {
  MOCK_SESSIONS,
  mockListSessionsApi,
  mockLogoutAllApi,
  mockRevokeSessionApi,
  setupProfileWithSessionsMocks,
} from './fixtures/profile.fixture';
import { ProfilePage } from './pages/profile.page';

test.describe('Profile - Session Management', () => {
  let profilePage: ProfilePage;

  test.beforeEach(async ({ page }) => {
    profilePage = new ProfilePage(page);
    await setupProfileWithSessionsMocks(page);
    await page.goto('/profile');
    await profilePage.waitForLoad();
  });

  test('displays active sessions list with device info', async ({ page }) => {
    // Should show 3 sessions with their IP addresses
    await expect(page.getByText('10.0.0.1')).toBeVisible();
    await expect(page.getByText('192.168.1.100')).toBeVisible();
    await expect(page.getByText('172.16.0.50')).toBeVisible();
  });

  test('shows current session badge', async ({ page }) => {
    // The current session should have a "current" badge
    const currentBadge = page.locator('span').filter({ hasText: /atual|current/i });
    await expect(currentBadge).toBeVisible();
  });

  test('hides revoke button on current session', async ({ page }) => {
    // The current session (Chrome - 10.0.0.1) should NOT have a revoke button
    const currentSessionCard = page.locator('[class*="rounded"]').filter({
      hasText: '10.0.0.1',
    });
    const revokeInCurrent = currentSessionCard.locator('.sr-only').filter({
      hasText: /revogar|revoke/i,
    });
    await expect(revokeInCurrent).toHaveCount(0);
  });

  test('shows revoke buttons on non-current sessions', async ({ page }) => {
    // Non-current sessions should have revoke buttons (sr-only text)
    const srOnlyRevoke = page.locator('.sr-only').filter({
      hasText: /revogar|revoke/i,
    });
    // 2 non-current sessions = 2 revoke buttons
    await expect(srOnlyRevoke).toHaveCount(2);
  });

  test.describe('Revoke Session', () => {
    test('shows confirmation dialog', async ({ page }) => {
      await mockRevokeSessionApi(page, 'success');

      // Click the first revoke button (for non-current session)
      const revokeButtons = page.locator('button').filter({
        has: page.locator('.sr-only'),
      });
      await revokeButtons.first().click();

      // Confirmation dialog should appear
      const dialog = page.locator('[role="alertdialog"]');
      await expect(dialog).toBeVisible();
    });

    test('dismisses on cancel', async ({ page }) => {
      // Click revoke button
      const revokeButtons = page.locator('button').filter({
        has: page.locator('.sr-only'),
      });
      await revokeButtons.first().click();

      // Click cancel in dialog
      const dialog = page.locator('[role="alertdialog"]');
      await expect(dialog).toBeVisible();

      const cancelBtn = dialog.locator('button').filter({
        hasText: /cancelar|cancel/i,
      });
      await cancelBtn.click();

      // Dialog should close
      await expect(dialog).not.toBeVisible();

      // Session should still be visible
      await expect(page.getByText('192.168.1.100')).toBeVisible();
    });

    test('revokes and removes card after confirm', async ({ page }) => {
      await mockRevokeSessionApi(page, 'success');

      // Click revoke on the Safari/iPhone session (192.168.1.100)
      const revokeButtons = page.locator('button').filter({
        has: page.locator('.sr-only'),
      });
      await revokeButtons.first().click();

      // Confirm in dialog
      const dialog = page.locator('[role="alertdialog"]');
      const confirmBtn = dialog.locator('button').filter({
        hasText: /confirmar|confirm|revogar|revoke/i,
      });
      await confirmBtn.click();

      // Session should be removed
      await expect(page.getByText('192.168.1.100')).not.toBeVisible({ timeout: 5000 });
    });

    test('shows success toast after revoke', async ({ page }) => {
      await mockRevokeSessionApi(page, 'success');

      const revokeButtons = page.locator('button').filter({
        has: page.locator('.sr-only'),
      });
      await revokeButtons.first().click();

      const dialog = page.locator('[role="alertdialog"]');
      const confirmBtn = dialog.locator('button').filter({
        hasText: /confirmar|confirm|revogar|revoke/i,
      });
      await confirmBtn.click();

      await waitForToast(page, /revog|revoke|desconect|success/i);
    });

    test('shows error toast on failure', async ({ page }) => {
      await mockRevokeSessionApi(page, 'error');

      const revokeButtons = page.locator('button').filter({
        has: page.locator('.sr-only'),
      });
      await revokeButtons.first().click();

      const dialog = page.locator('[role="alertdialog"]');
      const confirmBtn = dialog.locator('button').filter({
        hasText: /confirmar|confirm|revogar|revoke/i,
      });
      await confirmBtn.click();

      await waitForToast(page, /erro|error|falha|fail/i, 'error');
    });
  });

  test.describe('Logout All Devices', () => {
    test('shows logout-all button when multiple sessions', async ({ page }) => {
      const logoutAllBtn = page.locator('button').filter({
        hasText: /desconectar todos|logout all|sair de todos/i,
      });
      await expect(logoutAllBtn).toBeVisible();
    });

    test('shows confirmation dialog', async ({ page }) => {
      await mockLogoutAllApi(page);

      const logoutAllBtn = page.locator('button').filter({
        hasText: /desconectar todos|logout all|sair de todos/i,
      });
      await logoutAllBtn.click();

      const dialog = page.locator('[role="alertdialog"]');
      await expect(dialog).toBeVisible();
    });

    test('executes and shows success toast', async ({ page }) => {
      await mockLogoutAllApi(page);

      // Re-mock listSessions to return only current session after logout
      await page.route('**/auth.v1.AuthService/ListSessions', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            sessions: [MOCK_SESSIONS[0]],
            currentSessionId: 'session-current',
          }),
        });
      });

      const logoutAllBtn = page.locator('button').filter({
        hasText: /desconectar todos|logout all|sair de todos/i,
      });
      await logoutAllBtn.click();

      const dialog = page.locator('[role="alertdialog"]');
      const confirmBtn = dialog.locator('button').filter({
        hasText: /confirmar|confirm|desconectar|logout/i,
      });
      await confirmBtn.click();

      await waitForToast(page, /desconect|logout|success/i);
    });

    test('hides button with single session', async ({ page }) => {
      // Navigate fresh with only 1 session
      await mockListSessionsApi(page, [MOCK_SESSIONS[0]]); // only current session
      await page.goto('/profile');
      await profilePage.waitForLoad();

      const logoutAllBtn = page.locator('button').filter({
        hasText: /desconectar todos|logout all|sair de todos/i,
      });
      await expect(logoutAllBtn).not.toBeVisible();
    });
  });
});
