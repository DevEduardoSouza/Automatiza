/**
 * E2E test fixtures and helpers.
 * Re-exports all fixtures for convenient importing.
 */

// Auth fixtures
export {
  TEST_USER,
  MOCK_AUTH_USER,
  MOCK_AUTH_SESSION,
  test,
  expect,
  loginUser,
  logoutUser,
  waitForToast,
  mockPasswordResetEmail,
  mockProfileApi,
  mockPasswordChangeApi,
  mockPhoneUpdateApi,
  setupAuthMocks,
  setupUnauthenticatedMocks,
} from './auth.fixture';

// Automations fixtures
export {
  TEST_AUTOMATION,
  MOCK_AUTOMATIONS,
  MOCK_GROUPS,
  MOCK_DIAGNOSTIC_SUCCESS,
  mockAutomationsListApi,
  mockCreateAutomationApi,
  mockRunAutomationApi,
  mockDiagnoseAutomationApi,
  mockResetHistoryApi,
  mockDeleteAutomationApi,
  mockUpdateAutomationApi,
  mockGroupsApi,
  mockLimitsApi,
  mockWhatsAppStatusApi,
  setupAutomationMocks,
} from './automations.fixture';

// Affiliate programs fixtures
export {
  TEST_SHOPEE_CREDENTIALS,
  TEST_ML_CREDENTIALS,
  MOCK_CREDENTIALS_LIST,
  MOCK_ML_PREVIEW,
  MOCK_ML_CREATE_LINK,
  MOCK_SHOPEE_SEARCH,
  mockListCredentials,
  mockUpsertCredential,
  mockDeleteCredential,
  mockMlConnect,
  mockMlPreview,
  mockMlCreateLink,
  mockAllAffiliateApis,
} from './affiliate-programs.fixture';

// Dashboard fixtures
export {
  MOCK_DASHBOARD_STATS,
  MOCK_RECENT_ACTIVITY,
  MOCK_OFFERS_CHART_DATA,
  mockDashboardStatsApi,
  mockRecentActivityApi,
  mockOffersChartApi,
  mockDashboardWhatsAppStatus,
  mockDashboardEmptyState,
  mockDashboardError,
  setupDashboardMocks,
} from './dashboard.fixture';

// WhatsApp fixtures
export {
  MOCK_WHATSAPP_INSTANCE,
  MOCK_DISCONNECTED_INSTANCE,
  MOCK_QR_CODE,
  MOCK_INSTANCE_STATS,
  mockWhatsAppStatusApi as mockWhatsAppInstanceStatusApi,
  mockWhatsAppInstanceApi,
  mockGenerateQRCodeApi,
  mockPairingCodeApi,
  mockDisconnectApi,
  mockRestartApi,
  mockInstanceStatsApi,
  setupWhatsAppConnectedMocks,
  setupWhatsAppDisconnectedMocks,
} from './whatsapp.fixture';

// Groups fixtures
export {
  MOCK_GROUPS as MOCK_GROUPS_LIST,
  MOCK_SYNC_RESULT,
  mockGroupsListApi,
  mockSyncGroupsApi,
  mockUpdateGroupApi,
  mockGetGroupApi,
  mockDeleteGroupApi,
  mockGroupsWhatsAppStatus,
  setupGroupsMocks,
  setupEmptyGroupsMocks,
} from './groups.fixture';

// History fixtures
export {
  ACTION_TYPES,
  MOCK_HISTORY_ENTRIES,
  MOCK_HISTORY_STATS,
  mockHistoryListApi,
  mockHistoryStatsApi,
  mockHistoryEntryApi,
  mockClearHistoryApi,
  mockHistoryFilteredApi,
  mockHistoryErrorApi,
  setupHistoryMocks,
  setupEmptyHistoryMocks,
  setupPaginatedHistoryMocks,
} from './history.fixture';

// Jobs fixtures
export {
  JOB_TYPES,
  JOB_STATUS,
  MOCK_JOBS,
  MOCK_TARGET_GROUPS,
  TEST_JOB,
  mockJobsListApi,
  mockCreateJobApi,
  mockUpdateJobApi,
  mockDeleteJobApi,
  mockCancelJobApi,
  mockRunJobNowApi,
  mockJobGroupsApi,
  setupJobsMocks,
  setupEmptyJobsMocks,
} from './jobs.fixture';

// Coupons fixtures
export {
  COUPON_STATUS,
  STORE_TYPES,
  MOCK_COUPONS,
  TEST_COUPON,
  mockCouponsListApi,
  mockCreateCouponApi,
  mockUpdateCouponApi,
  mockDeleteCouponApi,
  mockToggleCouponStatusApi,
  mockCopyCouponApi,
  setupCouponsMocks,
  setupEmptyCouponsMocks,
  setupFilteredCouponsMocks,
} from './coupons.fixture';

// Dispatch texts fixtures
export {
  TEXT_TYPES,
  MOCK_DISPATCH_TEXTS,
  TEST_DISPATCH_TEXT,
  mockDispatchTextsListApi,
  mockCreateDispatchTextApi,
  mockUpdateDispatchTextApi,
  mockDeleteDispatchTextApi,
  mockToggleDispatchTextStatusApi,
  mockDuplicateDispatchTextApi,
  setupDispatchTextsMocks,
  setupEmptyDispatchTextsMocks,
  setupFilteredDispatchTextsMocks,
} from './dispatch-texts.fixture';

// Admin fixtures
export {
  USER_ROLES,
  PLAN_TYPES,
  MOCK_ADMIN_USERS,
  MOCK_SYSTEM_STATS,
  MOCK_AUDIT_LOGS,
  mockAdminUsersListApi,
  mockSystemStatsApi,
  mockAuditLogsApi,
  mockUpdateUserRoleApi,
  mockUpdateUserPlanApi,
  mockToggleUserStatusApi,
  mockImpersonateUserApi,
  mockCurrentUserAsAdmin,
  mockAdminAccessDenied,
  setupAdminMocks,
  setupEmptyAdminMocks,
} from './admin.fixture';

// Automation logs fixtures
export {
  MOCK_DISPATCH_LOGS,
  MOCK_LOG_AUTOMATIONS,
  MOCK_LOG_GROUPS,
  MOCK_QUEUE_STATUS,
  mockAppShellApis,
  mockAuthApi,
  mockDispatchLogsApi,
  mockLogAutomationsApi,
  mockLogGroupsApi,
  mockQueueStatusApi,
  mockListJobsApi,
  setupAutomationLogsMocks,
  setupEmptyAutomationLogsMocks,
} from './automation-logs.fixture';

// ML Link Lists fixtures
export {
  MOCK_ML_LINK_LISTS,
  MOCK_ML_LINK_LIST_ITEMS,
  TEST_ML_LINK_LIST,
  TEST_VALID_ML_URL,
  TEST_INVALID_URL,
  TEST_DUPLICATE_URL,
  MOCK_PREVIEW,
  setupMlLinkListsMocks,
  setupEmptyMlLinkListsMocks,
  mockCreateListApi,
  mockUpdateListApi,
  mockDeleteListApi,
  mockCreateItemApi,
  mockBulkCreateItemsApi,
  mockUpdateItemApi,
  mockDeleteItemApi,
  mockResetConsumedApi,
  mockReorderItemsApi,
  mockPreviewApi,
} from './ml-link-lists.fixture';
