import type { Page } from '@playwright/test';

/**
 * Job types enum matching the proto.
 */
export const JOB_TYPES = {
  MESSAGE: 1,
  POLL: 2,
  ML_OFFER: 3,
} as const;

/**
 * Job status enum.
 */
export const JOB_STATUS = {
  PENDING: 1,
  RUNNING: 2,
  COMPLETED: 3,
  FAILED: 4,
  CANCELLED: 5,
} as const;

/**
 * Mock jobs data.
 */
export const MOCK_JOBS = [
  {
    id: 'job-1',
    name: 'Morning Message',
    type: JOB_TYPES.MESSAGE,
    status: JOB_STATUS.PENDING,
    scheduledAt: new Date(Date.now() + 3600000).toISOString(),
    targetGroups: ['group-1', 'group-2'],
    message: 'Bom dia! Confira as promocoes de hoje.',
    createdAt: new Date().toISOString(),
  },
  {
    id: 'job-2',
    name: 'Daily Poll',
    type: JOB_TYPES.POLL,
    status: JOB_STATUS.COMPLETED,
    scheduledAt: new Date(Date.now() - 3600000).toISOString(),
    completedAt: new Date(Date.now() - 3500000).toISOString(),
    targetGroups: ['group-1'],
    pollQuestion: 'Qual categoria voce prefere?',
    pollOptions: ['Eletronicos', 'Casa', 'Moda', 'Outros'],
    createdAt: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'job-3',
    name: 'ML Offer Share',
    type: JOB_TYPES.ML_OFFER,
    status: JOB_STATUS.RUNNING,
    scheduledAt: new Date(Date.now() - 600000).toISOString(),
    targetGroups: ['group-1', 'group-2', 'group-3'],
    mlOfferId: 'ml-offer-123',
    createdAt: new Date(Date.now() - 7200000).toISOString(),
  },
  {
    id: 'job-4',
    name: 'Failed Message',
    type: JOB_TYPES.MESSAGE,
    status: JOB_STATUS.FAILED,
    scheduledAt: new Date(Date.now() - 1800000).toISOString(),
    targetGroups: ['group-3'],
    message: 'Test message that failed',
    error: 'WhatsApp disconnected',
    createdAt: new Date(Date.now() - 3600000).toISOString(),
  },
];

/**
 * Mock groups for job targeting.
 */
export const MOCK_TARGET_GROUPS = [
  { id: 'group-1', name: 'Promocoes Tech', jid: '123@g.us' },
  { id: 'group-2', name: 'Ofertas Casa', jid: '456@g.us' },
  { id: 'group-3', name: 'VIP Club', jid: '789@g.us' },
];

/**
 * Test job data for creation.
 */
export const TEST_JOB = {
  name: 'E2E Test Job',
  type: JOB_TYPES.MESSAGE,
  message: 'Test message from E2E',
  scheduledAt: new Date(Date.now() + 7200000).toISOString(),
};

/**
 * Mocks the jobs list API.
 */
export async function mockJobsListApi(page: Page, jobs = MOCK_JOBS): Promise<void> {
  await page.route('**/jobs.v1.JobsService/ListJobs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ jobs })),
    });
  });
}

/**
 * Mocks the create job API.
 */
export async function mockCreateJobApi(
  page: Page,
  scenario: 'success' | 'error' | 'validation_error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/CreateJob', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              job: {
                id: 'new-job-id',
                name: TEST_JOB.name,
                type: TEST_JOB.type,
                status: JOB_STATUS.PENDING,
                message: TEST_JOB.message,
                scheduledAt: TEST_JOB.scheduledAt,
                createdAt: new Date().toISOString(),
              },
            })
          ),
        });
        break;
      case 'validation_error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '3', // INVALID_ARGUMENT
            'grpc-message': 'Job name is required',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to create job',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the update job API.
 */
export async function mockUpdateJobApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/UpdateJob', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ job: MOCK_JOBS[0] })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update job',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the delete job API.
 */
export async function mockDeleteJobApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/DeleteJob', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to delete job',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the cancel job API.
 */
export async function mockCancelJobApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/CancelJob', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            job: { ...MOCK_JOBS[0], status: JOB_STATUS.CANCELLED },
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to cancel job',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the run job immediately API.
 */
export async function mockRunJobNowApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/RunJobNow', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            job: { ...MOCK_JOBS[0], status: JOB_STATUS.RUNNING },
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to run job',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the groups list for job creation.
 */
export async function mockJobGroupsApi(page: Page): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ groups: MOCK_TARGET_GROUPS })),
    });
  });
}

/**
 * Sets up all common mocks for jobs tests.
 */
export async function setupJobsMocks(page: Page): Promise<void> {
  await mockJobsListApi(page);
  await mockJobGroupsApi(page);
  await mockCreateJobApi(page);
  await mockUpdateJobApi(page);
  await mockDeleteJobApi(page);
  await mockCancelJobApi(page);
  await mockRunJobNowApi(page);
}

/**
 * Sets up mocks for empty jobs state.
 */
export async function setupEmptyJobsMocks(page: Page): Promise<void> {
  await mockJobsListApi(page, []);
  await mockJobGroupsApi(page);
  await mockCreateJobApi(page);
}
