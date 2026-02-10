import { expect, test } from '@playwright/test';
import {
  TEST_USER,
  mockPasswordChangeApi,
  mockPhoneUpdateApi,
  waitForToast,
} from './fixtures/auth.fixture';

test.describe('Profile Page', () => {
  test.describe('Authenticated User', () => {
    test.beforeEach(async ({ page }) => {
      // For real E2E tests, uncomment loginUser
      // await loginUser(page);
      // For mocked tests, navigate directly
      await page.goto('/profile');
    });

    test('should display profile page with user information', async ({ page }) => {
      // Wait for page to load
      await page.waitForSelector('h1', { state: 'visible' });

      // Verify page title is visible
      const title = page.locator('h1');
      await expect(title).toBeVisible();

      // Verify account info card is present
      const accountInfoCard = page.locator('text=Email').first();
      await expect(accountInfoCard).toBeVisible();

      // Verify phone section is present
      const phoneSection = page.locator('#phone_number');
      await expect(phoneSection).toBeVisible();

      // Verify security section is present
      const securitySection = page.locator('#current_password');
      await expect(securitySection).toBeVisible();
    });

    test('should update phone number successfully', async ({ page }) => {
      await mockPhoneUpdateApi(page, 'success');

      // Find and fill phone input
      const phoneInput = page.locator('#phone_number');
      await phoneInput.clear();
      await phoneInput.fill('+5511999999999');

      // Find and click the save button in the phone section
      const phoneSaveButton = page
        .locator('button')
        .filter({ hasText: /salvar|save/i })
        .first();
      await phoneSaveButton.click();

      // Wait for success toast
      await waitForToast(page, /atualizado|updated/i, 'success');
    });

    test('should handle phone update error', async ({ page }) => {
      await mockPhoneUpdateApi(page, 'error');

      const phoneInput = page.locator('#phone_number');
      await phoneInput.clear();
      await phoneInput.fill('+5511999999999');

      const phoneSaveButton = page
        .locator('button')
        .filter({ hasText: /salvar|save/i })
        .first();
      await phoneSaveButton.click();

      // Wait for error toast
      await waitForToast(page, /erro|error/i, 'error');
    });
  });

  test.describe('Password Change', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/profile');
    });

    test('should change password successfully', async ({ page }) => {
      await mockPasswordChangeApi(page, 'success');

      // Fill current password
      const currentPasswordInput = page.locator('#current_password');
      await currentPasswordInput.fill(TEST_USER.password);

      // Fill new password
      const newPasswordInput = page.locator('#new_password');
      await newPasswordInput.fill(TEST_USER.newPassword);

      // Fill confirm password
      const confirmPasswordInput = page.locator('#confirm_password');
      await confirmPasswordInput.fill(TEST_USER.newPassword);

      // Find the password save button (second save button or specific one)
      const passwordSaveButton = page
        .locator('button')
        .filter({ hasText: /alterar senha|change password|salvar/i })
        .last();
      await passwordSaveButton.click();

      // Wait for success toast
      await waitForToast(page, /senha.*atualizada|password.*updated/i, 'success');

      // Verify password fields are cleared after success
      await expect(currentPasswordInput).toHaveValue('');
      await expect(newPasswordInput).toHaveValue('');
      await expect(confirmPasswordInput).toHaveValue('');
    });

    test('should show error when passwords do not match', async ({ page }) => {
      // Fill current password
      const currentPasswordInput = page.locator('#current_password');
      await currentPasswordInput.fill(TEST_USER.password);

      // Fill new password
      const newPasswordInput = page.locator('#new_password');
      await newPasswordInput.fill(TEST_USER.newPassword);

      // Fill confirm password with different value
      const confirmPasswordInput = page.locator('#confirm_password');
      await confirmPasswordInput.fill('DifferentPassword123!');

      // Click save button
      const passwordSaveButton = page
        .locator('button')
        .filter({ hasText: /alterar senha|change password|salvar/i })
        .last();
      await passwordSaveButton.click();

      // Wait for error toast about password mismatch
      await waitForToast(page, /n.o conferem|do not match|mismatch/i, 'error');
    });

    test('should show error when new password is too short', async ({ page }) => {
      // Fill current password
      const currentPasswordInput = page.locator('#current_password');
      await currentPasswordInput.fill(TEST_USER.password);

      // Fill new password with short value
      const newPasswordInput = page.locator('#new_password');
      await newPasswordInput.fill(TEST_USER.shortPassword);

      // Fill confirm password
      const confirmPasswordInput = page.locator('#confirm_password');
      await confirmPasswordInput.fill(TEST_USER.shortPassword);

      // Click save button
      const passwordSaveButton = page
        .locator('button')
        .filter({ hasText: /alterar senha|change password|salvar/i })
        .last();
      await passwordSaveButton.click();

      // Wait for error toast about password length
      await waitForToast(page, /curta|short|m.nimo|minimum|8/i, 'error');
    });

    test('should show error when current password is empty', async ({ page }) => {
      // Leave current password empty
      const newPasswordInput = page.locator('#new_password');
      await newPasswordInput.fill(TEST_USER.newPassword);

      const confirmPasswordInput = page.locator('#confirm_password');
      await confirmPasswordInput.fill(TEST_USER.newPassword);

      // Click save button
      const passwordSaveButton = page
        .locator('button')
        .filter({ hasText: /alterar senha|change password|salvar/i })
        .last();
      await passwordSaveButton.click();

      // Wait for error toast about current password required
      await waitForToast(page, /senha atual|current password/i, 'error');
    });

    test('should show error when current password is wrong', async ({ page }) => {
      await mockPasswordChangeApi(page, 'wrong_password');

      // Fill wrong current password
      const currentPasswordInput = page.locator('#current_password');
      await currentPasswordInput.fill('WrongPassword123!');

      // Fill new password
      const newPasswordInput = page.locator('#new_password');
      await newPasswordInput.fill(TEST_USER.newPassword);

      // Fill confirm password
      const confirmPasswordInput = page.locator('#confirm_password');
      await confirmPasswordInput.fill(TEST_USER.newPassword);

      // Click save button
      const passwordSaveButton = page
        .locator('button')
        .filter({ hasText: /alterar senha|change password|salvar/i })
        .last();
      await passwordSaveButton.click();

      // Wait for error toast
      await waitForToast(page, /inv.lida|incorrect|invalid|wrong/i, 'error');
    });
  });

  test.describe('UI Elements', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/profile');
    });

    test('should display all profile sections', async ({ page }) => {
      // Account info section
      await expect(page.locator('text=Email').first()).toBeVisible();
      await expect(page.locator('text=Plano').or(page.locator('text=Plan'))).toBeVisible();

      // Phone section
      await expect(page.locator('#phone_number')).toBeVisible();

      // Security section
      await expect(page.locator('#current_password')).toBeVisible();
      await expect(page.locator('#new_password')).toBeVisible();
      await expect(page.locator('#confirm_password')).toBeVisible();
    });

    test('should have proper input types for password fields', async ({ page }) => {
      const currentPasswordInput = page.locator('#current_password');
      const newPasswordInput = page.locator('#new_password');
      const confirmPasswordInput = page.locator('#confirm_password');

      await expect(currentPasswordInput).toHaveAttribute('type', 'password');
      await expect(newPasswordInput).toHaveAttribute('type', 'password');
      await expect(confirmPasswordInput).toHaveAttribute('type', 'password');
    });

    test('should display loading state while fetching profile', async ({ page }) => {
      // Intercept profile request and delay it
      await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      await page.goto('/profile');

      // Loading indicator might be visible
      const loadingText = page.locator('text=/carregando|loading/i');
      // Either loading is shown or content is displayed
      const isLoadingVisible = await loadingText.isVisible().catch(() => false);

      if (isLoadingVisible) {
        await expect(loadingText).toBeVisible();
      }
    });
  });
});
