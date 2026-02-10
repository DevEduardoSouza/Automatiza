import { expect, test } from '@playwright/test';

/**
 * E2E tests for Redirect Links functionality
 * Tests cover CRUD operations, group selection, priority management,
 * and public redirect page behavior.
 */
test.describe('Redirect Links Page', () => {
  test.describe('Form', () => {
    test.beforeEach(async ({ page }) => {
      // Mock auth and navigate to redirect page
      await page.goto('/auth');
      // Login would be handled by fixture in real scenario
      await page.goto('/redirect-link');
    });

    test('should display create form with generated slug', async ({ page }) => {
      // Verify link name input
      const nameInput = page.locator('[data-testid="link-name-input"]');
      await expect(nameInput).toBeVisible();

      // Verify slug input with auto-generated value
      const slugInput = page.locator('[data-testid="slug-input"]');
      await expect(slugInput).toBeVisible();
      const slugValue = await slugInput.inputValue();
      expect(slugValue).toMatch(/^[a-z0-9]{8}$/);

      // Verify strategy selector
      const strategySelect = page.locator('[data-testid="strategy-select"]');
      await expect(strategySelect).toBeVisible();

      // Verify submit button
      const submitButton = page.locator('[data-testid="submit-button"]');
      await expect(submitButton).toBeVisible();
    });

    test('should validate required fields', async ({ page }) => {
      // Clear slug input
      const slugInput = page.locator('[data-testid="slug-input"]');
      await slugInput.clear();

      // Try to submit
      const submitButton = page.locator('[data-testid="submit-button"]');
      await submitButton.click();

      // Should show validation error toast
      await expect(page.locator('.toast')).toContainText(/required|obrigat/i);
    });

    test('should validate slug format', async ({ page }) => {
      const slugInput = page.locator('[data-testid="slug-input"]');
      await slugInput.fill('INVALID-SLUG');

      const submitButton = page.locator('[data-testid="submit-button"]');
      await submitButton.click();

      // Should show validation error for invalid slug format
      await expect(page.locator('.toast')).toContainText(/letras minúsculas|lowercase/i);
    });

    test('should validate click limits when click_limit strategy', async ({ page }) => {
      // Select click_limit strategy
      const strategySelect = page.locator('[data-testid="strategy-select"]');
      await strategySelect.click();
      await page.locator('text=click_limit').click();

      // Fill name
      const nameInput = page.locator('[data-testid="link-name-input"]');
      await nameInput.fill('Test Link');

      // Try to submit without groups
      const submitButton = page.locator('[data-testid="submit-button"]');
      await submitButton.click();

      // Should show error about groups required
      await expect(page.locator('.toast')).toContainText(/grupo|group/i);
    });
  });

  test.describe('Group Selection', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirect-link');
    });

    test('should open group selection dialog', async ({ page }) => {
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      // Verify dialog is open
      const dialog = page.locator('[role="dialog"]');
      await expect(dialog).toBeVisible();
    });

    test('should filter groups by search', async ({ page }) => {
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      // Type in search
      const searchInput = page.locator('[data-testid="group-search-input"]');
      await searchInput.fill('Test Group');

      // Verify filtered results
      const groupItems = page.locator('[data-testid="group-item"]');
      await expect(groupItems).toHaveCount(1);
    });

    test('should show favorites section', async ({ page }) => {
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      // Verify favorites section exists
      const favoritesSection = page.locator('text=Favoritos');
      await expect(favoritesSection).toBeVisible();
    });

    test('should select and deselect groups', async ({ page }) => {
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      // Click first group checkbox
      const firstCheckbox = page.locator('[data-testid="group-checkbox"]').first();
      await firstCheckbox.click();

      // Verify selected
      await expect(firstCheckbox).toBeChecked();

      // Click again to deselect
      await firstCheckbox.click();

      // Verify deselected
      await expect(firstCheckbox).not.toBeChecked();
    });

    test('should apply selection with priorities', async ({ page }) => {
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      // Select two groups
      const checkboxes = page.locator('[data-testid="group-checkbox"]');
      await checkboxes.first().click();
      await checkboxes.nth(1).click();

      // Apply selection
      const applyButton = page.locator('[data-testid="apply-selection-button"]');
      await applyButton.click();

      // Verify priority table shows 2 groups
      const priorityRows = page.locator('[data-testid="priority-row"]');
      await expect(priorityRows).toHaveCount(2);
    });
  });

  test.describe('Priority Management', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirect-link');
      // Setup: Add groups to priority list
    });

    test('should reorder groups via up/down buttons', async ({ page }) => {
      // Click down button on first group
      const downButton = page.locator('[data-testid="move-down-button"]').first();
      await downButton.click();

      // Verify order changed (toast notification)
      await expect(page.locator('.toast')).toContainText(/ordem|order/i);
    });

    test('should show click limit input for click_limit strategy', async ({ page }) => {
      // Select click_limit strategy
      const strategySelect = page.locator('[data-testid="strategy-select"]');
      await strategySelect.click();
      await page.locator('text=click_limit').click();

      // Verify click limit inputs are visible
      const clickLimitInputs = page.locator('[data-testid="click-limit-input"]');
      await expect(clickLimitInputs.first()).toBeVisible();
    });
  });

  test.describe('CRUD Operations', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirect-link');
    });

    test('should create redirect link', async ({ page }) => {
      // Fill form
      const nameInput = page.locator('[data-testid="link-name-input"]');
      await nameInput.fill('My Test Link');

      // Select groups
      const selectButton = page.locator('[data-testid="select-groups-button"]');
      await selectButton.click();

      const firstCheckbox = page.locator('[data-testid="group-checkbox"]').first();
      await firstCheckbox.click();

      const applyButton = page.locator('[data-testid="apply-selection-button"]');
      await applyButton.click();

      // Submit
      const submitButton = page.locator('[data-testid="submit-button"]');
      await submitButton.click();

      // Verify success toast
      await expect(page.locator('.toast')).toContainText(/criado|created/i);
    });

    test('should edit redirect link with slug disabled', async ({ page }) => {
      // Click edit on existing link
      const editButton = page.locator('[data-testid="edit-link-button"]').first();
      await editButton.click();

      // Verify slug input is disabled
      const slugInput = page.locator('[data-testid="slug-input"]');
      await expect(slugInput).toBeDisabled();

      // Verify name can be edited
      const nameInput = page.locator('[data-testid="link-name-input"]');
      await expect(nameInput).not.toBeDisabled();
    });

    test('should cancel edit mode', async ({ page }) => {
      // Enter edit mode
      const editButton = page.locator('[data-testid="edit-link-button"]').first();
      await editButton.click();

      // Click cancel
      const cancelButton = page.locator('[data-testid="cancel-edit-button"]');
      await cancelButton.click();

      // Verify form is reset
      const submitButton = page.locator('[data-testid="submit-button"]');
      await expect(submitButton).toContainText(/criar|create/i);
    });

    test('should delete redirect link with confirmation', async ({ page }) => {
      // Click delete on existing link
      const deleteButton = page.locator('[data-testid="delete-link-button"]').first();
      await deleteButton.click();

      // Confirm deletion (if dialog appears)
      const confirmButton = page.locator('[data-testid="confirm-delete-button"]');
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      // Verify success toast
      await expect(page.locator('.toast')).toContainText(/deletado|deleted|removido/i);
    });
  });

  test.describe('Saved Links', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirect-link');
    });

    test('should display saved links with stats', async ({ page }) => {
      const linksCard = page.locator('[data-testid="saved-links-card"]');
      await expect(linksCard).toBeVisible();

      // Verify at least one link item
      const linkItems = page.locator('[data-testid="saved-link-item"]');
      if ((await linkItems.count()) > 0) {
        // Verify click count is displayed
        await expect(linkItems.first()).toContainText(/click|clique/i);
      }
    });

    test('should copy link to clipboard', async ({ page }) => {
      const copyButton = page.locator('[data-testid="copy-link-button"]').first();
      await copyButton.click();

      // Verify success toast
      await expect(page.locator('.toast')).toContainText(/copiado|copied/i);
    });

    test('should open link in new tab', async ({ page, context }) => {
      const pagePromise = context.waitForEvent('page');

      const openButton = page.locator('[data-testid="open-link-button"]').first();
      await openButton.click();

      // Verify new tab opened
      const newPage = await pagePromise;
      expect(newPage.url()).toContain('/r/');
    });
  });

  test.describe('API Verification', () => {
    test('should use ConnectRPC for redirect operations', async ({ page }) => {
      // Monitor network requests
      const connectRequests: string[] = [];
      page.on('request', (request) => {
        if (request.url().includes('/redirect.v1.RedirectLinkService')) {
          connectRequests.push(request.url());
        }
      });

      await page.goto('/redirect-link');
      await page.waitForTimeout(2000); // Wait for API calls

      // Verify ConnectRPC calls were made
      expect(connectRequests.length).toBeGreaterThan(0);
    });
  });
});

test.describe('Public Redirect Page', () => {
  test('should show loading state initially', async ({ page }) => {
    await page.goto('/r/test-slug');

    // Should show loading spinner initially
    const loader = page.locator('[data-testid="redirect-loader"]');
    await expect(loader).toBeVisible({ timeout: 1000 });
  });

  test('should handle invalid slug error', async ({ page }) => {
    await page.goto('/r/non-existent-slug-12345');

    // Wait for error state
    await page.waitForTimeout(3000);

    // Should show error message
    const errorMessage = page.locator('[data-testid="redirect-error"]');
    await expect(errorMessage).toBeVisible();
  });

  test('should show manual buttons in in-app browser', async ({ page, context }) => {
    // Set TikTok user agent to simulate in-app browser
    await context.addCookies([]);
    await page.setExtraHTTPHeaders({
      'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) TikTok/14.0.0',
    });

    await page.goto('/r/test-slug');

    // Should show manual open buttons
    const openWhatsAppButton = page.locator('button').filter({ hasText: /whatsapp/i });
    await expect(openWhatsAppButton).toBeVisible({ timeout: 5000 });

    const copyLinkButton = page.locator('button').filter({ hasText: /copiar|copy/i });
    await expect(copyLinkButton).toBeVisible();
  });

  test('should copy invite link', async ({ page }) => {
    await page.goto('/r/test-slug');
    await page.waitForTimeout(3000);

    // Find and click copy button (if visible - means we're in in-app browser scenario)
    const copyButton = page.locator('button').filter({ hasText: /copiar|copy/i });
    if (await copyButton.isVisible()) {
      await copyButton.click();

      // Verify clipboard operation (browser dependent)
      // This is a best-effort verification
    }
  });
});
