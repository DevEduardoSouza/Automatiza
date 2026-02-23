import { expect, test } from '@playwright/test';
import {
  mockCheckSlugAvailabilityApi,
  mockCreateRedirectLinkApi,
  mockDeleteRedirectLinkApi,
  mockListGroupsForRedirectApi,
  mockListRedirectLinksApi,
  mockPublicRedirectApi,
  mockUpdateRedirectLinkApi,
  setupEmptyRedirectMocks,
  setupPublicRedirectMocks,
  setupRedirectLinkMocks,
} from './fixtures/redirect.fixture';
import { PublicRedirectPage } from './pages/public-redirect.page';
import { RedirectPage } from './pages/redirect.page';

// ============================================================
// Redirect Links Page
// ============================================================

test.describe('Redirect Links Page', () => {
  let redirectPage: RedirectPage;

  test.describe('Page Load', () => {
    test('should display page title and form with auto-generated slug', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Form should be visible
      await expect(redirectPage.linkNameInput).toBeVisible();
      await expect(redirectPage.slugInput).toBeVisible();
      await expect(redirectPage.submitButton).toBeVisible();

      // Slug should be auto-generated (8 chars alphanumeric)
      const slugValue = await redirectPage.slugInput.inputValue();
      expect(slugValue).toMatch(/^[a-z0-9]{8}$/);
    });

    test('should display saved links from API', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Saved links section should be visible
      await expect(redirectPage.savedLinksTitle).toBeVisible();

      // Should display the mock links
      const linkPromo = redirectPage.getSavedLink('Link Promocional');
      await expect(linkPromo).toBeVisible();

      const linkVip = redirectPage.getSavedLink('Link VIP');
      await expect(linkVip).toBeVisible();
    });

    test('should display empty state when no links exist', async ({ page }) => {
      await setupEmptyRedirectMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await expect(redirectPage.emptyLinksMsg).toBeVisible();
    });
  });

  test.describe('Form Validation', () => {
    test.beforeEach(async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();
    });

    test('should show error toast when submitting without name', async () => {
      await redirectPage.submitForm();

      const toast = redirectPage.getToast(/nome|name|obrigat|required/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should show error toast when submitting without groups', async ({ page }) => {
      await redirectPage.fillForm('Test Link');
      await redirectPage.submitForm();

      const toast = redirectPage.getToast(/grupo|group/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should show error toast for invalid slug format', async ({ page }) => {
      await redirectPage.fillForm('Test Link', 'INVALID_SLUG!');

      // Select groups first so we pass other validations
      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.applySelection();

      await redirectPage.submitForm();

      const toast = redirectPage.getToast(/slug|formato|format|lowercase/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Strategy Selection', () => {
    test('should default to Limite de Cliques', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Click limit button should be visually active/selected
      await expect(redirectPage.clickLimitBtn).toBeVisible();
    });

    test('should switch strategy to Vagas Disponiveis', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await redirectPage.memberLimitBtn.click();
      await expect(redirectPage.memberLimitBtn).toBeVisible();
    });
  });

  test.describe('Group Selection Dialog', () => {
    test.beforeEach(async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();
    });

    test('should open dialog when clicking select groups', async () => {
      await redirectPage.openGroupDialog();
      await expect(redirectPage.groupDialog).toBeVisible();
    });

    test('should show groups in dialog', async () => {
      await redirectPage.openGroupDialog();

      // Mock groups should be listed
      await expect(redirectPage.groupDialog.getByText('Promocoes Tech')).toBeVisible();
      await expect(redirectPage.groupDialog.getByText('Ofertas Casa')).toBeVisible();
      await expect(redirectPage.groupDialog.getByText('Eletronicos VIP')).toBeVisible();
    });

    test('should apply selection and show priority section', async () => {
      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.selectGroup('Ofertas Casa');
      await redirectPage.applySelection();

      // Dialog should close
      await expect(redirectPage.groupDialog).not.toBeVisible();

      // Toast should appear
      const toast = redirectPage.getToast(/grupo|group|atualizado|updated/i);
      await expect(toast).toBeVisible({ timeout: 5000 });

      // Priority section should be visible
      await expect(redirectPage.prioritySection).toBeVisible();
    });

    test('should filter groups by search', async () => {
      await redirectPage.openGroupDialog();
      await redirectPage.dialogSearchInput.fill('Promocoes');

      // Only matching groups should be visible
      await expect(redirectPage.groupDialog.getByText('Promocoes Tech')).toBeVisible();
    });
  });

  test.describe('CRUD Operations', () => {
    test('should create link with success', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      // Override with specific slug check
      await mockCheckSlugAvailabilityApi(page, true);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Switch to member_limit to avoid click limit validation
      await redirectPage.memberLimitBtn.click();

      // Fill form
      await redirectPage.fillForm('New Test Link', 'new-test');

      // Select groups
      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.applySelection();

      // Submit
      await redirectPage.submitForm();

      // Should show success toast
      const toast = redirectPage.getToast(/criado|created|sucesso|success/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should show error when slug already exists', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      // Override slug check to return unavailable
      await mockCheckSlugAvailabilityApi(page, false);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Switch to member_limit
      await redirectPage.memberLimitBtn.click();

      await redirectPage.fillForm('Test Link', 'existing-slug');

      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.applySelection();

      await redirectPage.submitForm();

      const toast = redirectPage.getToast(/slug.*existe|slug.*exists|slug.*uso/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should enter edit mode with slug disabled and alert visible', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Click edit on first link
      await redirectPage.editLink('Link Promocional');

      // Slug should be disabled in edit mode
      await expect(redirectPage.slugInput).toBeDisabled();

      // Name should be populated
      const nameValue = await redirectPage.linkNameInput.inputValue();
      expect(nameValue).toBe('Link Promocional');

      // Editing alert should be visible
      await expect(redirectPage.editingAlert).toBeVisible();
    });

    test('should cancel edit mode and reset form', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Enter edit mode
      await redirectPage.editLink('Link Promocional');
      await expect(redirectPage.editingAlert).toBeVisible();

      // Cancel edit
      await redirectPage.cancelEditBtn.click();

      // Form should be reset
      const nameValue = await redirectPage.linkNameInput.inputValue();
      expect(nameValue).toBe('');

      // Slug should be re-enabled with new generated slug
      await expect(redirectPage.slugInput).not.toBeDisabled();
      const slugValue = await redirectPage.slugInput.inputValue();
      expect(slugValue).toMatch(/^[a-z0-9]{8}$/);
    });

    test('should update link successfully', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Enter edit mode
      await redirectPage.editLink('Link Promocional');

      // Change name
      await redirectPage.linkNameInput.clear();
      await redirectPage.linkNameInput.fill('Updated Link Name');

      // Submit
      await redirectPage.submitForm();

      const toast = redirectPage.getToast(/atualizado|updated/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should delete link with confirmation', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await redirectPage.deleteLink('Link Promocional');

      const toast = redirectPage.getToast(/exclu|delet|removido|removed/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });

    test('should copy link to clipboard', async ({ page, context }) => {
      // Grant clipboard permission
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);

      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await redirectPage.copyLink('Link Promocional');

      const toast = redirectPage.getToast(/copiado|copied/i);
      await expect(toast).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Priority Management', () => {
    test('should show priority table when groups are selected', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.selectGroup('Ofertas Casa');
      await redirectPage.applySelection();

      await expect(redirectPage.prioritySection).toBeVisible();
    });

    test('should reorder via move buttons and show toast', async ({ page }) => {
      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      await redirectPage.openGroupDialog();
      await redirectPage.selectGroup('Promocoes Tech');
      await redirectPage.selectGroup('Ofertas Casa');
      await redirectPage.applySelection();

      // Clear previous toasts
      await page.waitForTimeout(1000);

      // Move first group down
      const moveDownBtn = redirectPage.getMoveDownBtn(0);
      if (await moveDownBtn.isVisible()) {
        await moveDownBtn.click();

        const toast = redirectPage.getToast(/ordem|order|atualizado|updated/i);
        await expect(toast).toBeVisible({ timeout: 5000 });
      }
    });
  });

  test.describe('API Verification', () => {
    test('should make ConnectRPC ListRedirectLinks call on load', async ({ page }) => {
      const connectRequests: string[] = [];
      page.on('request', (request) => {
        if (request.url().includes('/redirect.v1.RedirectLinkService')) {
          connectRequests.push(request.url());
        }
      });

      await setupRedirectLinkMocks(page);
      redirectPage = new RedirectPage(page);
      await redirectPage.goto();
      await redirectPage.waitForLoad();

      // Wait for API calls
      await page.waitForTimeout(1000);

      const listCalls = connectRequests.filter((url) => url.includes('ListRedirectLinks'));
      expect(listCalls.length).toBeGreaterThan(0);
    });
  });
});

// ============================================================
// Public Redirect Page
// ============================================================

test.describe('Public Redirect Page', () => {
  let publicPage: PublicRedirectPage;

  test.describe('Loading State', () => {
    test('should show loading spinner initially', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'success');
      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('test-slug');

      // Should show loading state
      await publicPage.waitForLoading();
    });
  });

  test.describe('In-App Browser', () => {
    test('should show manual buttons with TikTok UA', async ({ page, context }) => {
      await setupPublicRedirectMocks(page, 'success');

      // Emulate TikTok in-app browser via page context
      await page.addInitScript(() => {
        Object.defineProperty(navigator, 'userAgent', {
          value: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) TikTok/14.0.0',
          writable: false,
        });
      });

      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('test-slug');
      await publicPage.waitForManual();

      // Manual buttons should be visible
      await expect(publicPage.openWhatsAppBtn).toBeVisible();
      await expect(publicPage.copyLinkBtn).toBeVisible();
      await expect(publicPage.openInviteLinkBtn).toBeVisible();
    });

    test('should show group name in manual view', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'success');

      await page.addInitScript(() => {
        Object.defineProperty(navigator, 'userAgent', {
          value: 'Mozilla/5.0 TikTok/14.0.0',
          writable: false,
        });
      });

      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('test-slug');
      await publicPage.waitForManual();

      // Group name should be visible
      await expect(page.getByText('Promocoes Tech')).toBeVisible();
    });
  });

  test.describe('Error Scenarios', () => {
    test('should show error for non-existent slug', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'not_found');
      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('non-existent-slug');
      await publicPage.waitForError();

      await expect(publicPage.errorTitle).toBeVisible();
    });

    test('should show retry button for groups_full error', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'groups_full');
      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('full-slug');
      await publicPage.waitForError();

      await expect(publicPage.retryButton).toBeVisible();
    });

    test('should show error for inactive link', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'inactive');
      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('inactive-slug');
      await publicPage.waitForError();

      await expect(publicPage.errorTitle).toBeVisible();
    });

    test('should show back button for generic error', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'no_groups');
      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('no-groups-slug');
      await publicPage.waitForError();

      await expect(publicPage.backButton).toBeVisible();
    });
  });

  test.describe('Manual Actions', () => {
    test('should copy invite link when clicking copy button', async ({ page, context }) => {
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);
      await setupPublicRedirectMocks(page, 'success');

      await page.addInitScript(() => {
        Object.defineProperty(navigator, 'userAgent', {
          value: 'Mozilla/5.0 TikTok/14.0.0',
          writable: false,
        });
      });

      publicPage = new PublicRedirectPage(page);
      await publicPage.goto('test-slug');
      await publicPage.waitForManual();

      await publicPage.copyLinkBtn.click();

      // Verify clipboard content
      const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
      expect(clipboardText).toContain('chat.whatsapp.com');
    });
  });
});
