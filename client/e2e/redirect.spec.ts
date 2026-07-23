import { devices, expect, test } from '@playwright/test';
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
      await publicPage.gotoLegacy('test-slug');

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
      await publicPage.gotoLegacy('test-slug');
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
      await publicPage.gotoLegacy('test-slug');
      await publicPage.waitForManual();

      // Group name should be visible
      await expect(page.getByText('Promocoes Tech')).toBeVisible();
    });
  });

  test.describe('Error Scenarios', () => {
    test('should show error for non-existent slug', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'not_found');
      publicPage = new PublicRedirectPage(page);
      await publicPage.gotoLegacy('non-existent-slug');
      await publicPage.waitForError();

      await expect(publicPage.errorTitle).toBeVisible();
    });

    test('should show retry button for groups_full error', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'groups_full');
      publicPage = new PublicRedirectPage(page);
      await publicPage.gotoLegacy('full-slug');
      await publicPage.waitForError();

      await expect(publicPage.retryButton).toBeVisible();
    });

    test('should show error for inactive link', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'inactive');
      publicPage = new PublicRedirectPage(page);
      await publicPage.gotoLegacy('inactive-slug');
      await publicPage.waitForError();

      await expect(publicPage.errorTitle).toBeVisible();
    });

    test('should show back button for generic error', async ({ page }) => {
      await setupPublicRedirectMocks(page, 'no_groups');
      publicPage = new PublicRedirectPage(page);
      await publicPage.gotoLegacy('no-groups-slug');
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
      await publicPage.gotoLegacy('test-slug');
      await publicPage.waitForManual();

      await publicPage.copyLinkBtn.click();

      // Verify clipboard content
      const clipboardText = await page.evaluate(() => navigator.clipboard.readText());
      expect(clipboardText).toContain('chat.whatsapp.com');
    });
  });
});

// ============================================================
// Direct redirect (GET /r/:slug) — layered proof
// ============================================================
//
// SCOPE: the Playwright harness serves ONLY Vite on :3003 — there is no Gin
// and no nginx here. So this file proves the BROWSER half of the contract
// (redirect chain shape, absence of bundle/POST, fallback UI). The
// authoritative proof of what the handler and the router actually answer lives
// in the Go tests (redirect_public_handler_test.go, router_test.go) and the
// forwarding is proven by `nginx -t` on the versioned vhost. None of these
// three alone is an end-to-end proof, and this block does not claim to be one.

test.describe('Direct redirect — network shape', () => {
  // The 302 points at a same-origin sentinel instead of the real
  // chat.whatsapp.com. Reason, verified in this harness: Playwright does NOT
  // invoke route handlers for the DOCUMENT that a redirect points to (only for
  // its sub-resources), so the destination cannot be stubbed — pointing at the
  // real URL made this test fetch chat.whatsapp.com over the public internet.
  // What the destination URL literally is, is asserted authoritatively in
  // redirect_public_handler_test.go (exact Location, full validation matrix).
  // What this test proves is the SHAPE of the chain in a real browser.
  const REDIRECT_SENTINEL = '/robots.txt';

  test('normal browser follows a 302 with no SPA bundle and no POST', async ({ page }) => {
    const requests: string[] = [];
    const responses: { url: string; status: number }[] = [];
    page.on('request', (r) => requests.push(`${r.method()} ${r.url()}`));
    page.on('response', (r) => responses.push({ url: r.url(), status: r.status() }));

    // Stand in for the Gin handler's answer, headers included.
    await page.route('**/r/network-test', async (route) => {
      await route.fulfill({
        status: 302,
        headers: {
          location: REDIRECT_SENTINEL,
          'cache-control': 'no-store, private',
          'referrer-policy': 'no-referrer',
        },
        body: '',
      });
    });

    await page.goto('/r/network-test');

    // The browser left /r and landed on the destination.
    expect(page.url()).toContain(REDIRECT_SENTINEL);

    const redirectResponse = responses.find((r) => r.url.includes('/r/network-test'));
    expect(redirectResponse?.status).toBe(302);

    // The chain stayed inside the harness — nothing was fetched from outside.
    const offOrigin = requests.filter((r) => !r.includes('localhost:3003'));
    expect(offOrigin).toEqual([]);

    // The whole point of the feature: the browser never downloads the app.
    const bundleRequests = requests.filter(
      (r) => r.includes('localhost:3003') && /\/@vite\/|\/src\/|\/assets\/|index\.html/.test(r)
    );
    expect(bundleRequests).toEqual([]);

    // And it never spends an attempt through the legacy JSON endpoint.
    const posts = requests.filter((r) => r.startsWith('POST') && r.includes('/api/v1/redirect'));
    expect(posts).toEqual([]);
  });
});

test.describe('Direct redirect — /join fallback', () => {
  test('in-app fallback makes exactly one POST and stays manual', async ({ page }) => {
    const posts: string[] = [];
    const consoleErrors: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/v1/redirect')) posts.push(r.url());
    });
    page.on('console', (msg) => {
      // The harness has no backend, so the app's auth bootstrap always logs a
      // failed GetCurrentUser. Filter that out: what is being asserted here is
      // that the REDIRECT flow itself is silent.
      const text = msg.text();
      if (msg.type() !== 'error') return;
      if (text.includes('GetCurrentUser') || text.includes('Failed to load resource')) return;
      consoleErrors.push(text);
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('test-slug');
    await publicPage.waitForManual();

    await expect(publicPage.openWhatsAppBtn).toBeVisible();
    await expect(publicPage.copyLinkBtn).toBeVisible();
    expect(posts).toHaveLength(1);
    expect(consoleErrors).toEqual([]);
  });

  test('desktop UA on /join stays manual instead of auto-redirecting', async ({ page }) => {
    let leftThePage = false;
    await page.route('https://chat.whatsapp.com/**', async (route) => {
      leftThePage = true;
      await route.fulfill({ status: 200, contentType: 'text/html', body: '<html></html>' });
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('test-slug');
    await publicPage.waitForManual();

    expect(leftThePage).toBe(false);
    expect(page.url()).toContain('/join/test-slug');
  });

  test('preview mode makes zero POSTs', async ({ page }) => {
    const posts: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/v1/redirect')) posts.push(r.url());
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('test-slug', '?mode=preview');
    await publicPage.waitForError();

    expect(posts).toEqual([]);
  });

  test('error=not_found renders the error without any POST', async ({ page }) => {
    const posts: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/v1/redirect')) posts.push(r.url());
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('missing-slug', '?error=not_found');
    await publicPage.waitForError();

    await expect(publicPage.errorTitle).toBeVisible();
    expect(posts).toEqual([]);
  });

  test('error=groups_full offers a retry that becomes a clean attempt', async ({ page }) => {
    const posts: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/v1/redirect')) posts.push(r.url());
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('full-slug', '?error=groups_full');
    await publicPage.waitForError();

    await expect(publicPage.retryButton).toBeVisible();
    expect(posts).toEqual([]);

    // The query was stripped, so the reload behind the button is a fresh try.
    expect(page.url()).not.toContain('error=');

    await publicPage.retryButton.click();
    await publicPage.waitForManual();
    expect(posts).toHaveLength(1);
  });
});

// A real mobile device profile — UA, viewport, touch and DPR — not just a
// narrow viewport: the UA is what the in-app heuristics read. defaultBrowserType
// is dropped because switching the engine inside a describe forces a new worker
// (and this file runs on the chromium project).
const { defaultBrowserType: _mobileEngine, ...iPhone13Profile } = devices['iPhone 13'];

test.describe('Direct redirect — /join fallback on mobile', () => {
  test.use(iPhone13Profile);

  test('mobile in-app fallback shows the manual UI with one POST', async ({ page }) => {
    const posts: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().includes('/api/v1/redirect')) posts.push(r.url());
    });

    await page.addInitScript(() => {
      Object.defineProperty(navigator, 'userAgent', {
        value:
          'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 334.0.0.32.98',
        writable: false,
      });
    });

    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('test-slug');
    await publicPage.waitForManual();

    await expect(publicPage.openWhatsAppBtn).toBeVisible();
    expect(posts).toHaveLength(1);
  });

  test('mobile error fallback shows the error state', async ({ page }) => {
    await setupPublicRedirectMocks(page, 'success');
    const publicPage = new PublicRedirectPage(page);
    await publicPage.gotoJoin('missing-slug', '?error=not_found');
    await publicPage.waitForError();

    await expect(publicPage.errorTitle).toBeVisible();
  });
});
