// Package connectrpc provides ConnectRPC handlers for the Automatiza platform.
package connectrpc

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/auth"
	dispatchtextsv1 "automatiza-go/internal/gen/dispatch_texts/v1"
	"automatiza-go/internal/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// contextWithUserForDispatchTexts creates a context with a user for testing.

// =============================================================================
// Mock Implementations
// =============================================================================

// mockSettingsRepoForDispatchTexts implements repository.SettingsRepository for testing.
type mockSettingsRepoForDispatchTexts struct {
	mock.Mock
}

func (m *mockSettingsRepoForDispatchTexts) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) GetAll(ctx context.Context) (models.SettingsMap, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) Update(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *mockSettingsRepoForDispatchTexts) GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	args := m.Called(ctx, userID, dt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepoForDispatchTexts) UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error {
	args := m.Called(ctx, dispatchTextID, updates)
	return args.Error(0)
}

func (m *mockSettingsRepoForDispatchTexts) DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error {
	args := m.Called(ctx, dispatchTextID, userID)
	return args.Error(0)
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestUserContextForDispatchTexts(userID string) context.Context {
	user := &auth.User{
		ID:    userID,
		Email: "test@example.com",
	}
	return context.WithValue(context.Background(), UserContextKey, user)
}

// =============================================================================
// ListDispatchTexts Tests
// =============================================================================

func TestListDispatchTexts_Success(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	expectedTexts := []models.DispatchText{
		{
			ID:        "dt-1",
			UserID:    userID,
			Name:      "Template 1",
			Body:      "Hello {{name}}",
			IsActive:  true,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
		{
			ID:        "dt-2",
			UserID:    userID,
			Name:      "Template 2",
			Body:      "Goodbye {{name}}",
			IsActive:  false,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockRepo.On("GetDispatchTextsByUser", mock.Anything, userID).Return(expectedTexts, nil)

	req := connect.NewRequest(&dispatchtextsv1.ListDispatchTextsRequest{})
	resp, err := handler.ListDispatchTexts(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.DispatchTexts, 2)
	assert.Equal(t, "dt-1", resp.Msg.DispatchTexts[0].Id)
	assert.Equal(t, "Template 1", resp.Msg.DispatchTexts[0].Name)
	assert.True(t, resp.Msg.DispatchTexts[0].IsActive)
	assert.Equal(t, "dt-2", resp.Msg.DispatchTexts[1].Id)
	assert.False(t, resp.Msg.DispatchTexts[1].IsActive)

	mockRepo.AssertExpectations(t)
}

func TestListDispatchTexts_EmptyList(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("GetDispatchTextsByUser", mock.Anything, userID).Return([]models.DispatchText{}, nil)

	req := connect.NewRequest(&dispatchtextsv1.ListDispatchTextsRequest{})
	resp, err := handler.ListDispatchTexts(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.DispatchTexts)

	mockRepo.AssertExpectations(t)
}

func TestListDispatchTexts_Unauthenticated(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	ctx := context.Background() // No user in context

	req := connect.NewRequest(&dispatchtextsv1.ListDispatchTextsRequest{})
	_, err := handler.ListDispatchTexts(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestListDispatchTexts_RepositoryError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("GetDispatchTextsByUser", mock.Anything, userID).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&dispatchtextsv1.ListDispatchTextsRequest{})
	_, err := handler.ListDispatchTexts(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// GetDispatchText Tests
// =============================================================================

func TestGetDispatchText_Success(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	expectedText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Template 1",
		Body:      "Hello {{name}}",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(expectedText, nil)

	req := connect.NewRequest(&dispatchtextsv1.GetDispatchTextRequest{Id: "dt-1"})
	resp, err := handler.GetDispatchText(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "dt-1", resp.Msg.DispatchText.Id)
	assert.Equal(t, "Template 1", resp.Msg.DispatchText.Name)
	assert.Equal(t, "Hello {{name}}", resp.Msg.DispatchText.Body)
	assert.True(t, resp.Msg.DispatchText.IsActive)

	mockRepo.AssertExpectations(t)
}

func TestGetDispatchText_NotFound(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("GetDispatchText", mock.Anything, "dt-nonexistent").Return(nil, nil)

	req := connect.NewRequest(&dispatchtextsv1.GetDispatchTextRequest{Id: "dt-nonexistent"})
	_, err := handler.GetDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

func TestGetDispatchText_WrongOwner(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	// Return a dispatch text owned by a different user
	otherUserText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    "other-user",
		Name:      "Template 1",
		Body:      "Hello",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(otherUserText, nil)

	req := connect.NewRequest(&dispatchtextsv1.GetDispatchTextRequest{Id: "dt-1"})
	_, err := handler.GetDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// CreateDispatchText Tests
// =============================================================================

func TestCreateDispatchText_Success(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("CreateDispatchText", mock.Anything, userID, mock.MatchedBy(func(dt *models.DispatchText) bool {
		return dt.Name == "New Template" && len(dt.Copies) == 1 && dt.Copies[0] == "Hello {link_afiliado}" && dt.IsActive == true
	})).Return(&models.DispatchText{
		ID:        "dt-new",
		UserID:    userID,
		Name:      "New Template",
		Body:      "Hello {link_afiliado}",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
		Copies:    []string{"Hello {link_afiliado}"},
	}, nil)

	req := connect.NewRequest(&dispatchtextsv1.CreateDispatchTextRequest{
		Name:   "New Template",
		Copies: []string{"Hello {link_afiliado}"},
	})
	resp, err := handler.CreateDispatchText(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "dt-new", resp.Msg.DispatchText.Id)
	assert.Equal(t, "New Template", resp.Msg.DispatchText.Name)
	assert.True(t, resp.Msg.DispatchText.IsActive)
	assert.Equal(t, []string{"Hello {link_afiliado}"}, resp.Msg.DispatchText.Copies)

	mockRepo.AssertExpectations(t)
}

func TestCreateDispatchText_WithIsActiveFalse(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("CreateDispatchText", mock.Anything, userID, mock.MatchedBy(func(dt *models.DispatchText) bool {
		return dt.Name == "Inactive Template" && dt.IsActive == false && len(dt.Copies) == 1
	})).Return(&models.DispatchText{
		ID:        "dt-new",
		UserID:    userID,
		Name:      "Inactive Template",
		Body:      "Content {link_afiliado}",
		IsActive:  false,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
		Copies:    []string{"Content {link_afiliado}"},
	}, nil)

	isActive := false
	req := connect.NewRequest(&dispatchtextsv1.CreateDispatchTextRequest{
		Name:     "Inactive Template",
		Copies:   []string{"Content {link_afiliado}"},
		IsActive: &isActive,
	})
	resp, err := handler.CreateDispatchText(ctx, req)

	require.NoError(t, err)
	assert.False(t, resp.Msg.DispatchText.IsActive)

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// UpdateDispatchText Tests
// =============================================================================

func TestUpdateDispatchText_Success(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	existingText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Old Name",
		Body:      "Old Body",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	updatedText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "New Name",
		Body:      "New Body",
		IsActive:  false,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-02T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(existingText, nil).Once()
	mockRepo.On("UpdateDispatchText", mock.Anything, "dt-1", mock.MatchedBy(func(updates map[string]interface{}) bool {
		return updates["name"] == "New Name" && updates["body"] == "New Body" && updates["is_active"] == false
	})).Return(nil)
	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(updatedText, nil).Once()

	newName := "New Name"
	newBody := "New Body"
	isActive := false
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:       "dt-1",
		Name:     &newName,
		Body:     &newBody,
		IsActive: &isActive,
	})
	resp, err := handler.UpdateDispatchText(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "New Name", resp.Msg.DispatchText.Name)
	assert.Equal(t, "New Body", resp.Msg.DispatchText.Body)
	assert.False(t, resp.Msg.DispatchText.IsActive)

	mockRepo.AssertExpectations(t)
}

func TestUpdateDispatchText_PartialUpdate(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	existingText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Original Name",
		Body:      "Original Body",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	updatedText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Updated Name",
		Body:      "Original Body",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-02T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(existingText, nil).Once()
	mockRepo.On("UpdateDispatchText", mock.Anything, "dt-1", mock.MatchedBy(func(updates map[string]interface{}) bool {
		_, hasBody := updates["body"]
		_, hasIsActive := updates["is_active"]
		return updates["name"] == "Updated Name" && !hasBody && !hasIsActive
	})).Return(nil)
	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(updatedText, nil).Once()

	newName := "Updated Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-1",
		Name: &newName,
	})
	resp, err := handler.UpdateDispatchText(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Msg.DispatchText.Name)
	assert.Equal(t, "Original Body", resp.Msg.DispatchText.Body) // Unchanged
	assert.True(t, resp.Msg.DispatchText.IsActive)               // Unchanged

	mockRepo.AssertExpectations(t)
}

func TestUpdateDispatchText_WrongOwner(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	// Return a dispatch text owned by a different user
	otherUserText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    "other-user",
		Name:      "Template 1",
		Body:      "Hello",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(otherUserText, nil)

	newName := "Hacked Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-1",
		Name: &newName,
	})
	_, err := handler.UpdateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// DeleteDispatchText Tests
// =============================================================================

func TestDeleteDispatchText_Success(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("DeleteDispatchText", mock.Anything, "dt-1", userID).Return(nil)

	req := connect.NewRequest(&dispatchtextsv1.DeleteDispatchTextRequest{Id: "dt-1"})
	_, err := handler.DeleteDispatchText(ctx, req)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteDispatchText_RepositoryError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("DeleteDispatchText", mock.Anything, "dt-1", userID).Return(errors.New("database error"))

	req := connect.NewRequest(&dispatchtextsv1.DeleteDispatchTextRequest{Id: "dt-1"})
	_, err := handler.DeleteDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// GetDispatchText Error Cases
// =============================================================================

func TestGetDispatchText_Unauthenticated(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	ctx := context.Background() // No user in context

	req := connect.NewRequest(&dispatchtextsv1.GetDispatchTextRequest{Id: "dt-1"})
	_, err := handler.GetDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetDispatchText_RepositoryError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(nil, errors.New("database error"))

	req := connect.NewRequest(&dispatchtextsv1.GetDispatchTextRequest{Id: "dt-1"})
	_, err := handler.GetDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// CreateDispatchText Error Cases
// =============================================================================

func TestCreateDispatchText_Unauthenticated(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	ctx := context.Background() // No user in context

	req := connect.NewRequest(&dispatchtextsv1.CreateDispatchTextRequest{
		Name:   "New Template",
		Copies: []string{"Hello {link_afiliado}"},
	})
	_, err := handler.CreateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestCreateDispatchText_RepositoryError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("CreateDispatchText", mock.Anything, userID, mock.Anything).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&dispatchtextsv1.CreateDispatchTextRequest{
		Name:   "New Template",
		Copies: []string{"Hello {link_afiliado}"},
	})
	_, err := handler.CreateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// UpdateDispatchText Error Cases
// =============================================================================

func TestUpdateDispatchText_Unauthenticated(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	ctx := context.Background() // No user in context

	newName := "Updated Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-1",
		Name: &newName,
	})
	_, err := handler.UpdateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestUpdateDispatchText_NotFound(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	mockRepo.On("GetDispatchText", mock.Anything, "dt-nonexistent").Return(nil, nil)

	newName := "Updated Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-nonexistent",
		Name: &newName,
	})
	_, err := handler.UpdateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

func TestUpdateDispatchText_UpdateRepoError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	existingText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Old Name",
		Body:      "Old Body",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(existingText, nil).Once()
	mockRepo.On("UpdateDispatchText", mock.Anything, "dt-1", mock.Anything).Return(errors.New("database error"))

	newName := "New Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-1",
		Name: &newName,
	})
	_, err := handler.UpdateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

func TestUpdateDispatchText_ReGetError(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	userID := "user-123"
	ctx := createTestUserContextForDispatchTexts(userID)

	existingText := &models.DispatchText{
		ID:        "dt-1",
		UserID:    userID,
		Name:      "Old Name",
		Body:      "Old Body",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(existingText, nil).Once()
	mockRepo.On("UpdateDispatchText", mock.Anything, "dt-1", mock.Anything).Return(nil)
	mockRepo.On("GetDispatchText", mock.Anything, "dt-1").Return(nil, errors.New("database error")).Once()

	newName := "New Name"
	req := connect.NewRequest(&dispatchtextsv1.UpdateDispatchTextRequest{
		Id:   "dt-1",
		Name: &newName,
	})
	_, err := handler.UpdateDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockRepo.AssertExpectations(t)
}

// =============================================================================
// DeleteDispatchText Error Cases
// =============================================================================

func TestDeleteDispatchText_Unauthenticated(t *testing.T) {
	mockRepo := new(mockSettingsRepoForDispatchTexts)
	handler := NewDispatchTextsHandler(mockRepo)

	ctx := context.Background() // No user in context

	req := connect.NewRequest(&dispatchtextsv1.DeleteDispatchTextRequest{Id: "dt-1"})
	_, err := handler.DeleteDispatchText(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestDispatchTextToProto_NilInput(t *testing.T) {
	result := dispatchTextToProto(nil)
	assert.Nil(t, result)
}

func TestDispatchTextToProto_ValidInput(t *testing.T) {
	dt := &models.DispatchText{
		ID:        "dt-1",
		UserID:    "user-123",
		Name:      "Test Template",
		Body:      "Hello {{name}}",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-02T00:00:00Z",
	}

	result := dispatchTextToProto(dt)

	assert.NotNil(t, result)
	assert.Equal(t, "dt-1", result.Id)
	assert.Equal(t, "user-123", result.UserId)
	assert.Equal(t, "Test Template", result.Name)
	assert.Equal(t, "Hello {{name}}", result.Body)
	assert.True(t, result.IsActive)
	assert.Equal(t, "2026-01-01T00:00:00Z", result.CreatedAt)
	assert.Equal(t, "2026-01-02T00:00:00Z", result.UpdatedAt)
}

func TestDispatchTextToProto_InactiveTemplate(t *testing.T) {
	dt := &models.DispatchText{
		ID:        "dt-1",
		UserID:    "user-123",
		Name:      "Inactive Template",
		Body:      "Content",
		IsActive:  false,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	result := dispatchTextToProto(dt)

	assert.NotNil(t, result)
	assert.False(t, result.IsActive)
}
