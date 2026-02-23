import { expect, test } from '@playwright/test';
import {
  MOCK_JOBS,
  setupEmptyJobsMocks,
  setupJobsMocks,
  setupJobsMocksWithDelay,
  setupJobsMocksWithPagination,
  mockCreateJobApi,
  mockDeleteJobApi,
  mockUpdateJobApi,
} from './fixtures/jobs.fixture';
import { JobsPage } from './pages/jobs.page';

test.describe('Jobs Management', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
  });

  // ============================================================
  // 1. Page Load
  // ============================================================

  test.describe('Page Load', () => {
    test('should display page title and subtitle', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.pageTitle).toBeVisible();
      await expect(jobsPage.pageSubtitle).toBeVisible();
    });

    test('should display warning alert', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.warningAlert).toBeVisible();
    });

    test('should display how it works card', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.howItWorksCard).toBeVisible();
    });

    test('should display table with correct job count', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.jobsTable).toBeVisible();
      const count = await jobsPage.getJobCount();
      expect(count).toBe(MOCK_JOBS.length);
    });

    test('should display create button', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.newJobButton).toBeVisible();
    });
  });

  // ============================================================
  // 2. Empty State
  // ============================================================

  test.describe('Empty State', () => {
    test('should display empty state message when no jobs', async ({ page }) => {
      await setupEmptyJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.emptyState).toBeVisible();
    });

    test('should still display create button in empty state', async ({ page }) => {
      await setupEmptyJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.newJobButton).toBeVisible();
    });
  });

  // ============================================================
  // 3. Loading State
  // ============================================================

  test.describe('Loading State', () => {
    test('should display loading spinner while fetching jobs', async ({ page }) => {
      await setupJobsMocksWithDelay(page, 3000);
      await jobsPage.goto();

      // Spinner should be visible while the delayed response is in flight
      await expect(jobsPage.loadingSpinner).toBeVisible({ timeout: 2000 });
    });
  });

  // ============================================================
  // 4. Create Job
  // ============================================================

  test.describe('Create Job', () => {
    test('should create a send_message job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await expect(jobsPage.dialogContent).toBeVisible();

      await jobsPage.selectActionType('mensagem');
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.selectGroups(['Promocoes Tech']);
      await jobsPage.fillMessageForm('Bom dia! Confira as ofertas de hoje.');
      await jobsPage.submitForm();

      // Dialog should close or toast should appear on success
      await expect(
        jobsPage.successToast.or(jobsPage.dialogContent)
      ).toBeVisible({ timeout: 5000 });
      const dialogHidden = await jobsPage.dialogContent.isHidden();
      const toastVisible = await jobsPage.successToast.isVisible();
      expect(dialogHidden || toastVisible).toBe(true);
    });

    test('should create a send_poll job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('enquete');
      await jobsPage.fillDateAndTime('2026-12-31', '15:00');
      await jobsPage.selectGroups(['Ofertas Casa']);
      await jobsPage.fillPollForm('Qual categoria?', ['Eletronicos', 'Casa', 'Moda']);
      await jobsPage.submitForm();

      const dialogHidden = await jobsPage.dialogContent.isHidden();
      const toastVisible = await jobsPage.successToast.isVisible();
      expect(dialogHidden || toastVisible).toBe(true);
    });

    test('should create a send_ml_offer job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('oferta');
      await jobsPage.fillDateAndTime('2026-12-31', '16:00');
      await jobsPage.selectGroups(['VIP Club']);
      await jobsPage.fillMlOfferForm(
        'https://www.mercadolivre.com.br/item/123',
        'Oferta imperdivel!',
        true
      );
      await jobsPage.submitForm();

      const dialogHidden = await jobsPage.dialogContent.isHidden();
      const toastVisible = await jobsPage.successToast.isVisible();
      expect(dialogHidden || toastVisible).toBe(true);
    });

    test('should create a close_groups job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('fechar');
      await jobsPage.fillDateAndTime('2026-12-31', '17:00');
      await jobsPage.selectGroups(['Promocoes Tech', 'Ofertas Casa']);
      await jobsPage.submitForm();

      const dialogHidden = await jobsPage.dialogContent.isHidden();
      const toastVisible = await jobsPage.successToast.isVisible();
      expect(dialogHidden || toastVisible).toBe(true);
    });
  });

  // ============================================================
  // 5. Validation Errors
  // ============================================================

  test.describe('Validation Errors', () => {
    test('should keep dialog open when submitting without action type', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.submitForm();

      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should keep dialog open when submitting without groups', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('mensagem');
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.fillMessageForm('Test message without groups');
      await jobsPage.submitForm();

      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should keep dialog open when submitting without date/time', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('mensagem');
      await jobsPage.selectGroups(['Promocoes Tech']);
      await jobsPage.fillMessageForm('Test message without date');
      await jobsPage.submitForm();

      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should keep dialog open when send_message has no message', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('mensagem');
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.selectGroups(['Promocoes Tech']);
      // Do not fill message
      await jobsPage.submitForm();

      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should keep dialog open when send_poll has fewer than 2 options', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('enquete');
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.selectGroups(['Promocoes Tech']);
      await jobsPage.fillPollForm('Qual categoria?', ['Apenas uma opcao']);
      await jobsPage.submitForm();

      await expect(jobsPage.dialogContent).toBeVisible();
    });
  });

  // ============================================================
  // 6. Edit Job
  // ============================================================

  test.describe('Edit Job', () => {
    test('should open edit dialog for a pending job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // job-1 is PENDING (index 0)
      await jobsPage.editJobByIndex(0);

      await expect(jobsPage.dialogContent).toBeVisible();
      await expect(jobsPage.dialogTitle).toBeVisible();
    });

    test('should save an edited job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Edit the first pending job (index 0)
      await jobsPage.editJobByIndex(0);
      await expect(jobsPage.dialogContent).toBeVisible();

      // Modify the message field
      await jobsPage.fillMessageForm('Mensagem atualizada via teste E2E');
      await jobsPage.submitForm();

      const dialogHidden = await jobsPage.dialogContent.isHidden();
      const toastVisible = await jobsPage.successToast.isVisible();
      expect(dialogHidden || toastVisible).toBe(true);
    });
  });

  // ============================================================
  // 7. Delete Job
  // ============================================================

  test.describe('Delete Job', () => {
    test('should delete a pending job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // job-1 is PENDING (index 0) — delete button should be present
      const row = jobsPage.getJobRow(0);
      const deleteBtn = jobsPage.getDeleteButton(row);
      await expect(deleteBtn).toBeVisible();

      await jobsPage.deleteJobByIndex(0);

      // Should show success toast after deletion
      await expect(jobsPage.successToast).toBeVisible({ timeout: 5000 });
    });

    test('should NOT show delete button for done or running jobs', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // job-2 (index 1) is DONE, job-3 (index 2) is RUNNING
      const doneRow = jobsPage.getJobRow(1);
      const doneDeleteBtn = jobsPage.getDeleteButton(doneRow);
      expect(await doneDeleteBtn.count()).toBe(0);

      const runningRow = jobsPage.getJobRow(2);
      const runningDeleteBtn = jobsPage.getDeleteButton(runningRow);
      expect(await runningDeleteBtn.count()).toBe(0);
    });
  });

  // ============================================================
  // 8. API Errors
  // ============================================================

  test.describe('API Errors', () => {
    test('should show error toast on CreateJob INTERNAL error', async ({ page }) => {
      await setupJobsMocks(page);
      // Override CreateJob with error scenario
      await mockCreateJobApi(page, 'error');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.selectActionType('fechar');
      await jobsPage.fillDateAndTime('2026-12-31', '14:00');
      await jobsPage.selectGroups(['Promocoes Tech']);
      await jobsPage.submitForm();

      await expect(jobsPage.successToast).toBeVisible({ timeout: 5000 });
      const toastText = await jobsPage.successToast.textContent();
      expect(toastText?.toLowerCase()).toMatch(/erro|error|falha|failed/i);
    });

    test('should show error toast on DeleteJob error', async ({ page }) => {
      await setupJobsMocks(page);
      // Override DeleteJob with error scenario
      await mockDeleteJobApi(page, 'error');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Delete the first pending job (index 0)
      await jobsPage.deleteJobByIndex(0);

      await expect(jobsPage.successToast).toBeVisible({ timeout: 5000 });
      const toastText = await jobsPage.successToast.textContent();
      expect(toastText?.toLowerCase()).toMatch(/erro|error|falha|failed/i);
    });

    test('should show error toast on UpdateJob error', async ({ page }) => {
      await setupJobsMocks(page);
      // Override UpdateJob with error scenario
      await mockUpdateJobApi(page, 'error');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.editJobByIndex(0);
      await expect(jobsPage.dialogContent).toBeVisible();

      await jobsPage.fillMessageForm('Trigger update error');
      await jobsPage.submitForm();

      await expect(jobsPage.successToast).toBeVisible({ timeout: 5000 });
      const toastText = await jobsPage.successToast.textContent();
      expect(toastText?.toLowerCase()).toMatch(/erro|error|falha|failed/i);
    });
  });

  // ============================================================
  // 9. Pagination
  // ============================================================

  test.describe('Pagination', () => {
    test('should display load more button with pagination', async ({ page }) => {
      await setupJobsMocksWithPagination(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.loadMoreButton).toBeVisible();
    });

    test('should display showing count text', async ({ page }) => {
      await setupJobsMocksWithPagination(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.showingCountText).toBeVisible();
    });

    test('should hide load more button when no next page', async ({ page }) => {
      await setupJobsMocksWithPagination(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Click load more to fetch second (final) page
      await jobsPage.loadMore();

      // After loading all pages, load more button should disappear
      await expect(jobsPage.loadMoreButton).toBeHidden({ timeout: 5000 });
    });
  });

  // ============================================================
  // 10. Job Status Display
  // ============================================================

  test.describe('Job Status Display', () => {
    test('should display status badges in table rows', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Check the first 3 rows have status badges
      for (let i = 0; i < 3; i++) {
        const row = jobsPage.getJobRow(i);
        const badge = jobsPage.getStatusBadge(row);
        await expect(badge).toBeVisible();
      }
    });

    test('should display error message for failed job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // job-4 (index 3) is FAILED with errorMessage: 'WhatsApp disconnected'
      const failedRow = jobsPage.getJobRow(3);
      await expect(failedRow).toBeVisible();
      const failedBadge = jobsPage.getStatusBadge(failedRow);
      await expect(failedBadge).toBeVisible();

      // The row or its details should contain the error message text
      await expect(failedRow).toContainText(/WhatsApp disconnected|falha|failed|erro/i);
    });
  });

  // ============================================================
  // 11. Mobile Responsive
  // ============================================================

  test.describe('Mobile Responsive', () => {
    test('should display mobile cards on small viewport', async ({ page }) => {
      await setupJobsMocks(page);
      await page.setViewportSize({ width: 375, height: 812 });
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const mobileCount = await jobsPage.getMobileJobCount();
      expect(mobileCount).toBeGreaterThan(0);
    });

    test('should display table on desktop viewport', async ({ page }) => {
      await setupJobsMocks(page);
      await page.setViewportSize({ width: 1280, height: 800 });
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.jobsTable).toBeVisible();
    });
  });

  // ============================================================
  // 12. ConnectRPC Integration
  // ============================================================

  test.describe('ConnectRPC Integration', () => {
    test('should make ListJobs and GetQueueStatus gRPC calls', async ({ page }) => {
      const grpcCalls: string[] = [];

      page.on('request', (request) => {
        const url = request.url();
        if (url.includes('jobs.v1.JobsService')) {
          grpcCalls.push(url);
        }
      });

      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const hasListJobs = grpcCalls.some((url) => url.includes('ListJobs'));
      const hasQueueStatus = grpcCalls.some((url) => url.includes('GetQueueStatus'));

      expect(hasListJobs).toBe(true);
      expect(hasQueueStatus).toBe(true);
    });
  });
});
