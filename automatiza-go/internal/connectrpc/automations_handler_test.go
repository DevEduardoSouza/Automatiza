// Package connectrpc provides ConnectRPC handlers for the Automatiza platform.
package connectrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/auth"
	automationsv1 "automatiza-go/internal/gen/automations/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockDispatchLogRepoForHandler implements repository.DispatchLogRepository for testing.
type mockDispatchLogRepoForHandler struct {
	mock.Mock
}

func (m *mockDispatchLogRepoForHandler) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	args := m.Called(ctx, log)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepoForHandler) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	args := m.Called(ctx, logID, updates)
	return args.Error(0)
}

func (m *mockDispatchLogRepoForHandler) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) (*models.DispatchLog, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, window)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepoForHandler) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepoForHandler) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	args := m.Called(ctx, automationID, sinceDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDispatchLogRepoForHandler) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	args := m.Called(ctx, automationID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Int(1), args.Error(2)
}

func (m *mockDispatchLogRepoForHandler) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	args := m.Called(ctx, userID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Get(1).(int64), args.Error(2)
}

func (m *mockDispatchLogRepoForHandler) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, hours)
	return args.Bool(0), args.Error(1)
}

func (m *mockDispatchLogRepoForHandler) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	args := m.Called(ctx, log, dedupWindowHours)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.DispatchLog), args.Bool(1), args.Error(2)
}

func (m *mockDispatchLogRepoForHandler) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, status, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createTestUserContextForAutomations creates a context with a user for testing.
func createTestUserContextForAutomations(userID string) context.Context {
	user := &auth.User{
		ID:    userID,
		Email: "test@example.com",
	}
	return context.WithValue(context.Background(), UserContextKey, user)
}

// createTestAutomationsHandler creates an AutomationsHandler with the mock repo and a no-op logger.
// Only dispatchLogRepo is needed for ListDispatchLogs tests; other dependencies are nil
// (groupRepo is nil so the per-chip quota check short-circuits).
func createTestAutomationsHandler(dispatchLogRepo repository.DispatchLogRepository) *AutomationsHandler {
	return NewAutomationsHandler(nil, dispatchLogRepo, nil, nil, nil, zap.NewNop())
}

// autoTestStr returns a pointer to the given string.
func autoTestStr(s string) *string { return &s }

// autoTestFloat64 returns a pointer to the given float64.
func autoTestFloat64(f float64) *float64 { return &f }

// autoTestBool returns a pointer to the given bool.
func autoTestBool(b bool) *bool { return &b }

// =============================================================================
// AutomationsHandler.ListDispatchLogs Tests
// =============================================================================

func TestAutomationsHandler_ListDispatchLogs_Success(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	returnedLogs := []repository.DispatchLogWithGroup{
		{
			DispatchLog: models.DispatchLog{
				ID:             1,
				UserID:         userID,
				AutomationID:   autoTestStr("auto-1"),
				AutomationName: autoTestStr("My Automation"),
				Store:          autoTestStr("shopee"),
				GroupID:        autoTestStr("group-1"),
				Status:         "sent",
				CreatedAt:      "2026-01-15T10:00:00Z",
			},
			GroupName: autoTestStr("Test Group"),
		},
		{
			DispatchLog: models.DispatchLog{
				ID:        2,
				UserID:    userID,
				Status:    "failed",
				CreatedAt: "2026-01-15T11:00:00Z",
			},
			GroupName: nil,
		},
	}

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 50, 0).
		Return(returnedLogs, int64(2), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	require.Len(t, resp.Msg.Logs, 2)
	assert.Equal(t, "1", resp.Msg.Logs[0].Id)
	assert.Equal(t, "sent", resp.Msg.Logs[0].Status)
	assert.Equal(t, "auto-1", resp.Msg.Logs[0].AutomationId)
	assert.Equal(t, "Test Group", resp.Msg.Logs[0].GroupName)
	assert.Equal(t, "2", resp.Msg.Logs[1].Id)
	assert.Equal(t, "failed", resp.Msg.Logs[1].Status)
	assert.Equal(t, int32(2), resp.Msg.TotalCount)
	assert.False(t, resp.Msg.HasMore)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_Unauthenticated(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	ctx := context.Background() // No user in context

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestAutomationsHandler_ListDispatchLogs_DefaultPagination(t *testing.T) {
	tests := []struct {
		name           string
		inputLimit     int32
		inputOffset    int32
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "zero limit defaults to 50",
			inputLimit:     0,
			inputOffset:    0,
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "negative limit defaults to 50",
			inputLimit:     -5,
			inputOffset:    0,
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "limit exceeding 500 defaults to 50",
			inputLimit:     600,
			inputOffset:    0,
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "negative offset defaults to 0",
			inputLimit:     50,
			inputOffset:    -1,
			expectedLimit:  50,
			expectedOffset: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(mockDispatchLogRepoForHandler)
			handler := createTestAutomationsHandler(mockRepo)

			userID := "user-123"
			ctx := createTestUserContextForAutomations(userID)

			mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, tc.expectedLimit, tc.expectedOffset).
				Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

			req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
				Limit:  tc.inputLimit,
				Offset: tc.inputOffset,
			})
			resp, err := handler.ListDispatchLogs(ctx, req)

			require.NoError(t, err)
			assert.Empty(t, resp.Msg.Logs)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAutomationsHandler_ListDispatchLogs_ValidPagination(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 25, 100).
		Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		Limit:  25,
		Offset: 100,
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_FilterAutomation(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.AutomationID != nil && *f.AutomationID == "auto-abc"
	}), 50, 0).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		AutomationId: autoTestStr("auto-abc"),
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_FilterGroup(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.GroupID != nil && *f.GroupID == "120363001234567890@g.us"
	}), 50, 0).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		GroupId: autoTestStr("120363001234567890@g.us"),
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_FilterStatus(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.Status != nil && *f.Status == "sent"
	}), 50, 0).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		Status: autoTestStr("sent"),
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_FilterStore(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.Store != nil && *f.Store == "shopee"
	}), 50, 0).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		Store: autoTestStr("shopee"),
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_FilterSearchUrl(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.SearchURL != nil && *f.SearchURL == "https://shopee.com.br/product/123"
	}), 50, 0).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		SearchUrl: autoTestStr("https://shopee.com.br/product/123"),
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_AllFiltersCombined(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.MatchedBy(func(f *models.DispatchLogFilters) bool {
		return f.AutomationID != nil && *f.AutomationID == "auto-1" &&
			f.GroupID != nil && *f.GroupID == "group-1" &&
			f.Status != nil && *f.Status == "sent" &&
			f.Store != nil && *f.Store == "amazon" &&
			f.SearchURL != nil && *f.SearchURL == "https://amazon.com.br/dp/B09"
	}), 20, 10).Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		AutomationId: autoTestStr("auto-1"),
		GroupId:      autoTestStr("group-1"),
		Status:       autoTestStr("sent"),
		Store:        autoTestStr("amazon"),
		SearchUrl:    autoTestStr("https://amazon.com.br/dp/B09"),
		Limit:        20,
		Offset:       10,
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_AccessDeniedError(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 50, 0).
		Return(nil, int64(0), errors.New("automation not found or access denied"))

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		AutomationId: autoTestStr("auto-other-user"),
	})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_InternalError(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 50, 0).
		Return(nil, int64(0), errors.New("database connection timeout"))

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_EmptyResult(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 50, 0).
		Return([]repository.DispatchLogWithGroup{}, int64(0), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Logs)
	assert.Equal(t, int32(0), resp.Msg.TotalCount)
	assert.False(t, resp.Msg.HasMore)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_HasMore_True(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	// Return 10 logs but totalCount is 25 (offset=0, len=10, 0+10 < 25 -> hasMore=true)
	logs := make([]repository.DispatchLogWithGroup, 10)
	for i := range logs {
		logs[i] = repository.DispatchLogWithGroup{
			DispatchLog: models.DispatchLog{
				ID:        i + 1,
				UserID:    userID,
				Status:    "sent",
				CreatedAt: "2026-01-15T10:00:00Z",
			},
		}
	}

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 10, 0).
		Return(logs, int64(25), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		Limit: 10,
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 10)
	assert.Equal(t, int32(25), resp.Msg.TotalCount)
	assert.True(t, resp.Msg.HasMore)

	mockRepo.AssertExpectations(t)
}

func TestAutomationsHandler_ListDispatchLogs_HasMore_False(t *testing.T) {
	mockRepo := new(mockDispatchLogRepoForHandler)
	handler := createTestAutomationsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForAutomations(userID)

	// Return 5 logs, totalCount=15, offset=10 -> 10+5 >= 15 -> hasMore=false
	logs := make([]repository.DispatchLogWithGroup, 5)
	for i := range logs {
		logs[i] = repository.DispatchLogWithGroup{
			DispatchLog: models.DispatchLog{
				ID:        i + 11,
				UserID:    userID,
				Status:    "sent",
				CreatedAt: "2026-01-15T10:00:00Z",
			},
		}
	}

	mockRepo.On("GetUserLogsWithFilters", mock.Anything, userID, mock.Anything, 10, 10).
		Return(logs, int64(15), nil)

	req := connect.NewRequest(&automationsv1.ListDispatchLogsRequest{
		Limit:  10,
		Offset: 10,
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 5)
	assert.Equal(t, int32(15), resp.Msg.TotalCount)
	assert.False(t, resp.Msg.HasMore)

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// dispatchLogWithGroupToProto Helper Tests
// =============================================================================

func TestDispatchLogWithGroupToProto_NilInput(t *testing.T) {
	result := dispatchLogWithGroupToProto(nil)
	assert.Nil(t, result)
}

func TestDispatchLogWithGroupToProto_FullFields(t *testing.T) {
	log := &repository.DispatchLogWithGroup{
		DispatchLog: models.DispatchLog{
			ID:               42,
			UserID:           "user-123",
			AutomationID:     autoTestStr("auto-1"),
			AutomationName:   autoTestStr("Shopee Monitor"),
			Store:            autoTestStr("shopee"),
			GroupID:          autoTestStr("120363001234567890@g.us"),
			ProductURL:       autoTestStr("https://shopee.com.br/product/123"),
			ProductPageURL:   autoTestStr("https://shopee.com.br/page/123"),
			CanonicalKey:     autoTestStr("shopee:product:123"),
			Status:           "sent",
			Error:            autoTestStr("timeout on delivery"),
			CreatedAt:        "2026-01-15T10:30:00Z",
			ProductCategory:  autoTestStr("electronics"),
			ProductPrice:     autoTestFloat64(99.90),
			ProductDiscount:  autoTestFloat64(20.0),
			AffiliateLink:    autoTestStr("https://s.shopee.com.br/aff/123"),
			CouponID:         autoTestStr("coupon-1"),
			CouponCode:       autoTestStr("SAVE10"),
			CouponApplied:    autoTestBool(true),
			CouponSkipReason: autoTestStr(""),
		},
		GroupName: autoTestStr("Ofertas Shopee"),
	}

	result := dispatchLogWithGroupToProto(log)

	require.NotNil(t, result)

	// Required fields
	assert.Equal(t, "42", result.Id)
	assert.Equal(t, "sent", result.Status)

	// Automation
	assert.Equal(t, "auto-1", result.AutomationId)

	// Group
	assert.Equal(t, "120363001234567890@g.us", result.GroupId)
	assert.Equal(t, "Ofertas Shopee", result.GroupName)

	// Canonical key
	assert.Equal(t, "shopee:product:123", result.CanonicalKey)

	// Offer title (uses AutomationName when present)
	assert.Equal(t, "Shopee Monitor", result.OfferTitle)

	// Store
	require.NotNil(t, result.Store)
	assert.Equal(t, "shopee", *result.Store)

	// Product URLs
	require.NotNil(t, result.ProductUrl)
	assert.Equal(t, "https://shopee.com.br/product/123", *result.ProductUrl)
	require.NotNil(t, result.ProductPageUrl)
	assert.Equal(t, "https://shopee.com.br/page/123", *result.ProductPageUrl)
	// OfferUrl should match ProductUrl when both are set
	require.NotNil(t, result.OfferUrl)
	assert.Equal(t, "https://shopee.com.br/product/123", *result.OfferUrl)

	// Affiliate link
	require.NotNil(t, result.AffiliateLink)
	assert.Equal(t, "https://s.shopee.com.br/aff/123", *result.AffiliateLink)

	// Product category
	require.NotNil(t, result.ProductCategory)
	assert.Equal(t, "electronics", *result.ProductCategory)

	// Product price (mapped to both OfferPrice and ProductPrice)
	require.NotNil(t, result.OfferPrice)
	assert.Equal(t, 99.90, *result.OfferPrice)
	require.NotNil(t, result.ProductPrice)
	assert.Equal(t, 99.90, *result.ProductPrice)

	// Coupon fields
	require.NotNil(t, result.CouponId)
	assert.Equal(t, "coupon-1", *result.CouponId)
	require.NotNil(t, result.CouponCode)
	assert.Equal(t, "SAVE10", *result.CouponCode)
	require.NotNil(t, result.CouponApplied)
	assert.True(t, *result.CouponApplied)
	require.NotNil(t, result.CouponSkipReason)
	assert.Equal(t, "", *result.CouponSkipReason)

	// Error message
	require.NotNil(t, result.ErrorMessage)
	assert.Equal(t, "timeout on delivery", *result.ErrorMessage)

	// CreatedAt timestamp
	require.NotNil(t, result.CreatedAt)
	assert.Equal(t, int64(2026), int64(result.CreatedAt.AsTime().Year()))
}

func TestDispatchLogWithGroupToProto_MinimalFields(t *testing.T) {
	log := &repository.DispatchLogWithGroup{
		DispatchLog: models.DispatchLog{
			ID:        1,
			UserID:    "user-123",
			Status:    "pending",
			CreatedAt: "",
		},
		GroupName: nil,
	}

	result := dispatchLogWithGroupToProto(log)

	require.NotNil(t, result)

	// Required fields are set
	assert.Equal(t, "1", result.Id)
	assert.Equal(t, "pending", result.Status)

	// Optional fields are at zero values
	assert.Equal(t, "", result.AutomationId)
	assert.Equal(t, "", result.GroupId)
	assert.Equal(t, "", result.GroupName)
	assert.Equal(t, "", result.CanonicalKey)
	assert.Equal(t, "", result.OfferTitle)
	assert.Nil(t, result.Store)
	assert.Nil(t, result.ProductUrl)
	assert.Nil(t, result.ProductPageUrl)
	assert.Nil(t, result.OfferUrl)
	assert.Nil(t, result.AffiliateLink)
	assert.Nil(t, result.ProductCategory)
	assert.Nil(t, result.OfferPrice)
	assert.Nil(t, result.ProductPrice)
	assert.Nil(t, result.CouponId)
	assert.Nil(t, result.CouponCode)
	assert.Nil(t, result.CouponApplied)
	assert.Nil(t, result.CouponSkipReason)
	assert.Nil(t, result.ErrorMessage)
	// CreatedAt is nil when the string is empty (time.Parse fails)
	assert.Nil(t, result.CreatedAt)
}
