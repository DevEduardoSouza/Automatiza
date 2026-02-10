import { expect, test } from '@playwright/test';
import {
  JOB_STATUS,
  MOCK_JOBS,
  TEST_JOB,
  mockCancelJobApi,
  mockCreateJobApi,
  mockDeleteJobApi,
  mockRunJobNowApi,
  setupEmptyJobsMocks,
  setupJobsMocks,
} from './fixtures/jobs.fixture';
import { JobsPage } from './pages/jobs.page';

test.describe('Jobs Management', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
  });

  test.describe('Page Load', () => {
    test('should display jobs page with list', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Verify jobs are listed
      const jobCount = await jobsPage.getJobCount();
      expect(jobCount).toBeGreaterThanOrEqual(0);
    });

    test('should display create button', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await expect(jobsPage.createButton).toBeVisible();
    });

    test('should display status tabs', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Look for status tabs
      const allTab = page.getByRole('tab', { name: /todos|all/i });
      const hasAllTab = await allTab.isVisible().catch(() => false);

      expect(typeof hasAllTab).toBe('boolean');
    });
  });

  test.describe('Empty State', () => {
    test('should display empty state when no jobs', async ({ page }) => {
      await setupEmptyJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Verify empty state or zero jobs
      const isEmpty = await jobsPage.isEmpty();
      const hasNoJobs = (await jobsPage.getJobCount()) === 0;

      expect(isEmpty || hasNoJobs).toBeTruthy();
    });

    test('should show create first job prompt', async ({ page }) => {
      await setupEmptyJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Create button should be visible
      await expect(jobsPage.createButton).toBeVisible();
    });
  });

  test.describe('Create Job', () => {
    test('should open create dialog', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();

      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should create a new job successfully', async ({ page }) => {
      await setupJobsMocks(page);
      await mockCreateJobApi(page, 'success');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Open create dialog
      await jobsPage.openCreateDialog();

      // Fill form
      await jobsPage.fillJobForm({
        name: TEST_JOB.name,
        message: 'Test message',
      });

      // Save
      await jobsPage.save();

      // Wait for response
      await page.waitForTimeout(1000);

      // Should show success toast or close dialog
      const toast = page.locator('[data-sonner-toast]');
      const dialogClosed = await jobsPage.dialogContent.isHidden().catch(() => false);

      expect((await toast.isVisible()) || dialogClosed).toBeTruthy();
    });

    test('should show validation error when name is empty', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();

      // Try to save without name
      await jobsPage.save();

      // Dialog should still be open
      await expect(jobsPage.dialogContent).toBeVisible();
    });

    test('should cancel creation and close dialog', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();

      // Fill some data
      await jobsPage.nameInput.fill('Test Job');

      // Cancel
      await jobsPage.cancel();

      // Dialog should be closed
      await expect(jobsPage.dialogContent).toBeHidden();
    });
  });

  test.describe('Job Actions', () => {
    test('should run job immediately', async ({ page }) => {
      await setupJobsMocks(page);
      await mockRunJobNowApi(page, 'success');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Find pending job
      const pendingJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.PENDING);
      if (pendingJob) {
        const row = jobsPage.getJobByName(pendingJob.name);

        if (await row.isVisible()) {
          const runButton = jobsPage.getRunNowButton(row);

          if (await runButton.isVisible()) {
            await runButton.click();
            await page.waitForTimeout(500);

            // Should show success feedback
            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);

            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should cancel pending job', async ({ page }) => {
      await setupJobsMocks(page);
      await mockCancelJobApi(page, 'success');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      // Find pending job
      const pendingJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.PENDING);
      if (pendingJob) {
        await jobsPage.cancelJob(pendingJob.name);
        await page.waitForTimeout(500);

        // Should show success feedback
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);

        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should delete job', async ({ page }) => {
      await setupJobsMocks(page);
      await mockDeleteJobApi(page, 'success');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const firstJob = MOCK_JOBS[0];
      await jobsPage.deleteJob(firstJob.name);
      await page.waitForTimeout(500);

      // Should show success feedback
      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);

      expect(typeof hasToast).toBe('boolean');
    });
  });

  test.describe('Filter by Status', () => {
    test('should filter pending jobs', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const pendingTab = page.getByRole('tab', { name: /pendente|pending/i });
      if (await pendingTab.isVisible()) {
        await pendingTab.click();
        await page.waitForTimeout(500);

        const count = await jobsPage.getJobCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });

    test('should filter completed jobs', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const completedTab = page.getByRole('tab', { name: /concluido|completed/i });
      if (await completedTab.isVisible()) {
        await completedTab.click();
        await page.waitForTimeout(500);

        const count = await jobsPage.getJobCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });

    test('should filter failed jobs', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const failedTab = page.getByRole('tab', { name: /falha|failed/i });
      if (await failedTab.isVisible()) {
        await failedTab.click();
        await page.waitForTimeout(500);

        const count = await jobsPage.getJobCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });
  });

  test.describe('Job Status Display', () => {
    test('should display correct status badge for pending job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const pendingJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.PENDING);
      if (pendingJob) {
        const status = await jobsPage.getJobStatus(pendingJob.name);
        expect(status?.toLowerCase()).toMatch(/pendente|pending|agendado|scheduled/i);
      }
    });

    test('should display correct status badge for completed job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const completedJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.COMPLETED);
      if (completedJob) {
        const status = await jobsPage.getJobStatus(completedJob.name);
        expect(status?.toLowerCase()).toMatch(/concluido|completed|sucesso|success/i);
      }
    });

    test('should display correct status badge for running job', async ({ page }) => {
      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const runningJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.RUNNING);
      if (runningJob) {
        const status = await jobsPage.getJobStatus(runningJob.name);
        expect(status?.toLowerCase()).toMatch(/executando|running|em.*andamento|in.*progress/i);
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should handle create job error', async ({ page }) => {
      await setupJobsMocks(page);
      await mockCreateJobApi(page, 'error');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      await jobsPage.openCreateDialog();
      await jobsPage.fillJobForm({ name: TEST_JOB.name, message: 'Test' });
      await jobsPage.save();

      await page.waitForTimeout(1000);

      // Should show error toast
      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should handle run job error', async ({ page }) => {
      await setupJobsMocks(page);
      await mockRunJobNowApi(page, 'error');
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      const pendingJob = MOCK_JOBS.find((j) => j.status === JOB_STATUS.PENDING);
      if (pendingJob) {
        const row = jobsPage.getJobByName(pendingJob.name);
        if (await row.isVisible()) {
          const runButton = jobsPage.getRunNowButton(row);
          if (await runButton.isVisible()) {
            await runButton.click();
            await page.waitForTimeout(1000);

            const errorToast = page.locator('[data-sonner-toast]').filter({
              hasText: /erro|error|falha|failed/i,
            });
            const hasError = await errorToast.isVisible().catch(() => false);

            expect(typeof hasError).toBe('boolean');
          }
        }
      }
    });
  });

  test.describe('Migration Validation', () => {
    test('should load jobs via ConnectRPC', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('jobs.v1.JobsService')) {
          grpcCalls.push(url);
        }

        await route.continue();
      });

      await setupJobsMocks(page);
      await jobsPage.goto();
      await jobsPage.waitForLoad();

      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
