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
  MOCK_PARTICIPANTS,
  MOCK_MEMBERS,
  MOCK_SEND_MESSAGE_RESULT,
  MOCK_SEND_AUDIO_RESULT,
  MOCK_SEND_POLL_RESULT,
  mockGroupsListApi,
  mockSyncGroupsApi,
  mockUpdateGroupApi,
  mockGetGroupApi,
  mockDeleteGroupApi,
  mockGroupsWhatsAppStatus,
  mockSendMessageApi,
  mockSendImageApi,
  mockSendAudioApi,
  mockSendPollApi,
  mockGroupParticipantsApi,
  mockInstancesStatus,
  mockAddGroupMemberApi,
  mockRemoveGroupMemberApi,
  mockLeaveGroupApi,
  mockPromoteMemberApi,
  mockDemoteMemberApi,
  mockUpdateGroupSubjectApi,
  mockUpdateGroupSettingsApi,
  mockUpdateGroupDescriptionApi,
  mockUpdateGroupPictureApi,
  mockBulkUpdateGroupsApi,
  mockReconnectInstanceApi,
  mockGetSyncStatusApi,
  mockToggleGroupFavoriteApi,
  mockGetGroupInviteCodeApi,
  mockGroupsSSEEndpoint,
  mockGroupsListApiNetworkError,
  mockGroupsListApiUnavailable,
  setupGroupsMocks,
  setupEmptyGroupsMocks,
  setupMessagingMocks,
  setupMembersMocks,
  setupBulkActionsMocks,
  setupConnectionMocks,
  setupFullGroupsMocks,
  setupDisconnectedGroupsMocks,
  setupErrorGroupsMocks,
} from './groups.fixture';

// History fixtures
export {
  MOCK_HISTORY_EVENTS,
  MOCK_HISTORY_STATS,
  mockHistoryListApi,
  mockClearHistoryApi,
  mockHistoryErrorApi,
  setupHistoryMocks,
  setupEmptyHistoryMocks,
  setupPaginatedHistoryMocks,
} from './history.fixture';

// Jobs fixtures
export {
  JOB_ACTION_TYPES,
  JOB_STATUS,
  MOCK_JOBS,
  MOCK_QUEUE_STATUS as MOCK_JOBS_QUEUE_STATUS,
  MOCK_TARGET_GROUPS as MOCK_JOB_TARGET_GROUPS,
  mockJobsListApi,
  mockCreateJobApi,
  mockUpdateJobApi,
  mockDeleteJobApi,
  mockGetQueueStatusApi,
  mockProcessScheduledJobsApi,
  mockJobGroupsApi,
  mockSSEEndpoint as mockJobsSSEEndpoint,
  setupJobsMocks,
  setupEmptyJobsMocks,
  setupJobsMocksWithPagination,
  setupJobsMocksWithDelay,
} from './jobs.fixture';

// Coupons fixtures
export {
  MOCK_COUPONS,
  TEST_COUPON,
  mockCouponsListApi,
  mockCreateCouponApi,
  mockUpdateCouponApi,
  mockDeleteCouponApi,
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
  // Mock data constants
  MOCK_ADMIN_PROFILE_STATS,
  MOCK_OBSERVABILITY,
  MOCK_SHOPEE_AUDIT,
  MOCK_WEBHOOK_LOGS,
  MOCK_PROFILES,
  MOCK_ADMIN_USERS_LIST,
  MOCK_USER_INVESTIGATION,
  MOCK_PLANS,
  MOCK_CATEGORIES,
  MOCK_EMAIL_TEMPLATES,
  MOCK_SETTINGS,
  // Dashboard mocks
  mockGetAdminProfileStats,
  mockGetObservability,
  mockGetShopeeAffiliateAudit,
  mockListWebhookLogs,
  mockSimulateKiwifyWebhook,
  // Users mocks
  mockListProfiles,
  mockSearchProfiles,
  mockListAdminUsers,
  mockCreateUser,
  mockGetUserInvestigation,
  mockGrantAdminRole,
  mockRevokeAdminRole,
  mockAdminBlockUser,
  mockUnblockUser,
  mockResetUserPassword,
  mockAdminBulkUpdateAutomationStatus,
  mockAdminSpreadAutomationSchedule,
  mockUpdateUserPlan,
  // Plans mocks
  mockListPlans,
  mockCreatePlan,
  mockUpdatePlan,
  mockDeletePlan,
  // Categories mocks
  mockListCategories,
  mockCreateCategory,
  mockUpdateCategory,
  mockDeleteCategory,
  mockCreateSubcategory,
  mockUpdateSubcategory,
  mockDeleteSubcategory,
  // Email Templates mocks
  mockListEmailTemplates,
  mockCreateEmailTemplate,
  mockUpdateEmailTemplate,
  mockToggleEmailTemplateActive,
  // Settings mocks
  mockListSettings,
  mockUpdateSetting,
  // Auth helpers
  mockCurrentUserAsAdmin,
  mockAdminAccessDenied,
  // Setup composites (per tab)
  setupAdminDashboardMocks,
  setupAdminUsersMocks,
  setupAdminPlansMocks,
  setupAdminCategoriesMocks,
  setupAdminTeamMocks,
  setupAdminEmailTemplatesMocks,
  setupAdminSettingsMocks,
  setupFullAdminMocks,
} from './admin.fixture';

// Profile fixtures
export {
  MOCK_PROFILE_DATA,
  MOCK_LIMITS_DATA,
  MOCK_SESSIONS,
  mockProfileGetApi,
  mockProfileLimitsApi,
  mockListSessionsApi,
  mockRevokeSessionApi,
  mockLogoutAllApi,
  setupProfilePageMocks,
  setupProfileWithSessionsMocks,
} from './profile.fixture';

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

// Redirect fixtures
export {
  MOCK_REDIRECT_GROUPS,
  MOCK_SAVED_LINKS,
  MOCK_GROUPS_FOR_REDIRECT,
  MOCK_PUBLIC_REDIRECT_SUCCESS,
  mockListRedirectLinksApi,
  mockListGroupsForRedirectApi,
  mockCreateRedirectLinkApi,
  mockUpdateRedirectLinkApi,
  mockDeleteRedirectLinkApi,
  mockCheckSlugAvailabilityApi,
  mockPublicRedirectApi,
  setupRedirectLinkMocks,
  setupEmptyRedirectMocks,
  setupPublicRedirectMocks,
} from './redirect.fixture';

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
