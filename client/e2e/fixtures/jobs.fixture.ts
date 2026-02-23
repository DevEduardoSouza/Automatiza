import type { Page } from '@playwright/test';

// ============================================================
// Enums (matching jobs.v1 proto)
// ============================================================

/**
 * Job action types matching JobActionType proto enum.
 */
export const JOB_ACTION_TYPES = {
  UNSPECIFIED: 0,
  SEND_MESSAGE: 1,
  SEND_POLL: 2,
  SEND_ML_OFFER: 3,
  UPDATE_DESCRIPTION: 4,
  CLOSE_GROUPS: 5,
  OPEN_GROUPS: 6,
  CHANGE_GROUP_NAME: 7,
  CHANGE_GROUP_PHOTO: 8,
} as const;

/**
 * Job status matching JobStatus proto enum.
 */
export const JOB_STATUS = {
  UNSPECIFIED: 0,
  PENDING: 1,
  RUNNING: 2,
  DONE: 3,
  FAILED: 4,
  CANCELLED: 5,
} as const;

// ============================================================
// Helpers
// ============================================================

/**
 * Creates a proto Timestamp object from a Date.
 */
function makeTimestamp(date: Date): { seconds: string; nanos: number } {
  return { seconds: String(Math.floor(date.getTime() / 1000)), nanos: 0 };
}

// ============================================================
// Mock Data
// ============================================================

const now = new Date();
const oneHourFromNow = new Date(now.getTime() + 3_600_000);
const oneHourAgo = new Date(now.getTime() - 3_600_000);
const twoHoursAgo = new Date(now.getTime() - 7_200_000);
const oneDayAgo = new Date(now.getTime() - 86_400_000);

/**
 * Mock jobs covering all 5 statuses and all 8 action types.
 */
export const MOCK_JOBS = [
  {
    id: 'job-1',
    actionType: JOB_ACTION_TYPES.SEND_MESSAGE,
    scheduledFor: makeTimestamp(oneHourFromNow),
    status: JOB_STATUS.PENDING,
    payload: { groups: ['group-1', 'group-2'], message: 'Bom dia! Confira as promocoes.' },
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'job-2',
    actionType: JOB_ACTION_TYPES.SEND_POLL,
    scheduledFor: makeTimestamp(oneHourAgo),
    status: JOB_STATUS.DONE,
    payload: {
      groups: ['group-1'],
      question: 'Qual categoria voce prefere?',
      options: ['Eletronicos', 'Casa', 'Moda', 'Outros'],
    },
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(oneHourAgo),
  },
  {
    id: 'job-3',
    actionType: JOB_ACTION_TYPES.SEND_ML_OFFER,
    scheduledFor: makeTimestamp(new Date(now.getTime() - 600_000)),
    status: JOB_STATUS.RUNNING,
    payload: {
      groups: ['group-1', 'group-2', 'group-3'],
      originUrl: 'https://www.mercadolivre.com.br/item/123',
      text: 'Oferta imperdivel!',
      includeImage: true,
    },
    createdAt: makeTimestamp(twoHoursAgo),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'job-4',
    actionType: JOB_ACTION_TYPES.SEND_MESSAGE,
    scheduledFor: makeTimestamp(new Date(now.getTime() - 1_800_000)),
    status: JOB_STATUS.FAILED,
    payload: { groups: ['group-3'], message: 'Test message that failed' },
    errorMessage: 'WhatsApp disconnected',
    createdAt: makeTimestamp(twoHoursAgo),
    updatedAt: makeTimestamp(oneHourAgo),
  },
  {
    id: 'job-5',
    actionType: JOB_ACTION_TYPES.CLOSE_GROUPS,
    scheduledFor: makeTimestamp(twoHoursAgo),
    status: JOB_STATUS.CANCELLED,
    payload: { groups: ['group-1', 'group-2'] },
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(twoHoursAgo),
  },
  {
    id: 'job-6',
    actionType: JOB_ACTION_TYPES.UPDATE_DESCRIPTION,
    scheduledFor: makeTimestamp(oneHourFromNow),
    status: JOB_STATUS.PENDING,
    payload: { groups: ['group-2'], description: 'Nova descricao do grupo' },
    createdAt: makeTimestamp(now),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'job-7',
    actionType: JOB_ACTION_TYPES.CHANGE_GROUP_NAME,
    scheduledFor: makeTimestamp(new Date(oneHourFromNow.getTime() + 3_600_000)),
    status: JOB_STATUS.PENDING,
    payload: { groups: ['group-1', 'group-2', 'group-3'], name: 'Grupo Promocoes', autoNumber: true },
    createdAt: makeTimestamp(now),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'job-8',
    actionType: JOB_ACTION_TYPES.OPEN_GROUPS,
    scheduledFor: makeTimestamp(oneHourAgo),
    status: JOB_STATUS.DONE,
    payload: { groups: ['group-1', 'group-3'] },
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(oneHourAgo),
  },
  {
    id: 'job-9',
    actionType: JOB_ACTION_TYPES.CHANGE_GROUP_PHOTO,
    scheduledFor: makeTimestamp(now),
    status: JOB_STATUS.RUNNING,
    payload: { groups: ['group-2'], image: 'data:image/png;base64,iVBORw0KGgo...' },
    createdAt: makeTimestamp(twoHoursAgo),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'job-10',
    actionType: JOB_ACTION_TYPES.SEND_MESSAGE,
    scheduledFor: makeTimestamp(new Date(oneHourFromNow.getTime() + 7_200_000)),
    status: JOB_STATUS.PENDING,
    payload: { groups: ['group-1'], message: 'Mensagem para teste de paginacao' },
    createdAt: makeTimestamp(now),
    updatedAt: makeTimestamp(now),
  },
];

/**
 * Mock queue status matching GetQueueStatusResponse proto.
 */
export const MOCK_QUEUE_STATUS = {
  queue: { waiting: 3, delayed: 1, active: 2, queued: 6 },
  settings: { limitPerMinute: 10, workerConcurrency: 2 },
  estimatedMinutes: 5,
};

/**
 * Mock target groups for job group selection (matching groups.v1 proto).
 */
export const MOCK_TARGET_GROUPS = [
  {
    id: 'group-1',
    name: 'Promocoes Tech',
    waGroupId: '123456789@g.us',
    status: 'SYNCED',
    isFavorite: true,
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'group-2',
    name: 'Ofertas Casa',
    waGroupId: '987654321@g.us',
    status: 'SYNCED',
    isFavorite: false,
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(now),
  },
  {
    id: 'group-3',
    name: 'VIP Club',
    waGroupId: '111222333@g.us',
    status: 'SYNCED',
    isFavorite: true,
    createdAt: makeTimestamp(oneDayAgo),
    updatedAt: makeTimestamp(now),
  },
];

// ============================================================
// Mock API Functions
// ============================================================

/**
 * Mocks the ListJobs ConnectRPC endpoint.
 */
export async function mockJobsListApi(
  page: Page,
  jobs = MOCK_JOBS,
  nextPageToken = '',
  totalCount?: number
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/ListJobs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        jobs,
        nextPageToken,
        totalCount: totalCount ?? jobs.length,
      }),
    });
  });
}

/**
 * Mocks the CreateJob ConnectRPC endpoint.
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
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            job: {
              id: 'new-job-id',
              actionType: JOB_ACTION_TYPES.SEND_MESSAGE,
              scheduledFor: makeTimestamp(oneHourFromNow),
              status: JOB_STATUS.PENDING,
              payload: { groups: ['group-1'], message: 'New job message' },
              createdAt: makeTimestamp(now),
              updatedAt: makeTimestamp(now),
            },
          }),
        });
        break;
      case 'validation_error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '3',
            'grpc-message': 'Payload is required',
          },
          body: JSON.stringify({}),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'Failed to create job',
          },
          body: JSON.stringify({}),
        });
        break;
    }
  });
}

/**
 * Mocks the UpdateJob ConnectRPC endpoint.
 */
export async function mockUpdateJobApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/UpdateJob', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ job: MOCK_JOBS[0] }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to update job',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the DeleteJob ConnectRPC endpoint.
 */
export async function mockDeleteJobApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/DeleteJob', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to delete job',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the GetQueueStatus ConnectRPC endpoint.
 */
export async function mockGetQueueStatusApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/GetQueueStatus', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify(MOCK_QUEUE_STATUS),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to get queue status',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the ProcessScheduledJobs ConnectRPC endpoint.
 */
export async function mockProcessScheduledJobsApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/jobs.v1.JobsService/ProcessScheduledJobs', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ processed: 3, failed: 0, errors: [] }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to process scheduled jobs',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the ListGroups endpoint for group selection in job creation.
 */
export async function mockJobGroupsApi(page: Page): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ groups: MOCK_TARGET_GROUPS }),
    });
  });
}

/**
 * Mocks the SSE endpoint for real-time job updates.
 */
export async function mockSSEEndpoint(page: Page): Promise<void> {
  await page.route('**/api/v1/sse/events**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      headers: {
        'cache-control': 'no-cache',
        connection: 'keep-alive',
      },
      body: 'event: connected\ndata: {"type":"connected","data":{},"timestamp":0}\n\n',
    });
  });
}

// ============================================================
// Auth Mock (inline for self-containment)
// ============================================================

async function mockAuthRefreshToken(page: Page): Promise<void> {
  await page.route('**/auth.v1.AuthService/RefreshToken', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ accessToken: 'mock-token', expiresIn: 3600 }),
    });
  });

  await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: {
          id: 'user-123',
          email: 'test@example.com',
          displayName: 'Test User',
          role: 'user',
          emailVerified: true,
          createdAt: '2026-01-01T00:00:00Z',
        },
        session: {
          sessionId: 'session-123',
          expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
          isCurrent: true,
        },
      }),
    });
  });
}

// ============================================================
// Setup Helpers
// ============================================================

/**
 * Sets up all standard mocks for jobs tests.
 * Includes auth, SSE, queue status, ListJobs, all mutations, and groups.
 */
export async function setupJobsMocks(page: Page): Promise<void> {
  await mockAuthRefreshToken(page);
  await mockSSEEndpoint(page);
  await mockGetQueueStatusApi(page);
  await mockJobsListApi(page);
  await mockCreateJobApi(page);
  await mockUpdateJobApi(page);
  await mockDeleteJobApi(page);
  await mockProcessScheduledJobsApi(page);
  await mockJobGroupsApi(page);
}

/**
 * Sets up mocks for empty jobs state.
 */
export async function setupEmptyJobsMocks(page: Page): Promise<void> {
  await mockAuthRefreshToken(page);
  await mockSSEEndpoint(page);
  await mockGetQueueStatusApi(page);
  await mockJobsListApi(page, [], '', 0);
  await mockCreateJobApi(page);
  await mockJobGroupsApi(page);
}

/**
 * Sets up mocks with pagination (first page has nextPageToken, second page does not).
 */
export async function setupJobsMocksWithPagination(page: Page): Promise<void> {
  await mockAuthRefreshToken(page);
  await mockSSEEndpoint(page);
  await mockGetQueueStatusApi(page);
  await mockCreateJobApi(page);
  await mockUpdateJobApi(page);
  await mockDeleteJobApi(page);
  await mockProcessScheduledJobsApi(page);
  await mockJobGroupsApi(page);

  const firstPage = MOCK_JOBS.slice(0, 5);
  const secondPage = MOCK_JOBS.slice(5);
  let requestCount = 0;

  await page.route('**/jobs.v1.JobsService/ListJobs', async (route) => {
    requestCount++;
    if (requestCount === 1) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          jobs: firstPage,
          nextPageToken: 'page-2-token',
          totalCount: MOCK_JOBS.length,
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          jobs: secondPage,
          nextPageToken: '',
          totalCount: MOCK_JOBS.length,
        }),
      });
    }
  });
}

/**
 * Sets up mocks with a delayed ListJobs response for loading state tests.
 */
export async function setupJobsMocksWithDelay(page: Page, delayMs: number): Promise<void> {
  await mockAuthRefreshToken(page);
  await mockSSEEndpoint(page);
  await mockGetQueueStatusApi(page);
  await mockCreateJobApi(page);
  await mockUpdateJobApi(page);
  await mockDeleteJobApi(page);
  await mockProcessScheduledJobsApi(page);
  await mockJobGroupsApi(page);

  await page.route('**/jobs.v1.JobsService/ListJobs', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        jobs: MOCK_JOBS,
        nextPageToken: '',
        totalCount: MOCK_JOBS.length,
      }),
    });
  });
}
