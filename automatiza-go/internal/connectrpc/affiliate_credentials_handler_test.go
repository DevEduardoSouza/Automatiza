package connectrpc

import (
	"context"
	"errors"
	"testing"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockAffiliateRepo implements repository.AffiliateCredentialsRepository for testing.
type mockAffiliateRepo struct {
	mock.Mock
}

func (m *mockAffiliateRepo) GetByUserAndStore(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error) {
	args := m.Called(ctx, userID, store)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AffiliateCredentials), args.Error(1)
}

func (m *mockAffiliateRepo) GetAllByUser(ctx context.Context, userID string) ([]models.AffiliateCredentials, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AffiliateCredentials), args.Error(1)
}

func (m *mockAffiliateRepo) Upsert(ctx context.Context, userID, store string, data *models.AffiliateCredentials) error {
	args := m.Called(ctx, userID, store, data)
	return args.Error(0)
}

func (m *mockAffiliateRepo) Delete(ctx context.Context, userID, store string) error {
	args := m.Called(ctx, userID, store)
	return args.Error(0)
}

func (m *mockAffiliateRepo) Deactivate(ctx context.Context, userID, store string) error {
	args := m.Called(ctx, userID, store)
	return args.Error(0)
}

func (m *mockAffiliateRepo) GetActiveStores(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createAffiliateHandler creates an AffiliateCredentialsHandler with mocked dependencies.
func createAffiliateHandler(t *testing.T) (*AffiliateCredentialsHandler, *mockAffiliateRepo) {
	t.Helper()
	repo := new(mockAffiliateRepo)
	handler := NewAffiliateCredentialsHandler(repo)
	return handler, repo
}

// =============================================================================
// ListCredentials Tests
// =============================================================================

func TestListCredentials_Success(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	credentials := []models.AffiliateCredentials{
		{
			ID:           "cred-1",
			UserID:       "user-123",
			Store:        "shopee",
			Credentials:  map[string]string{"appId": "123", "password": "secret"},
			AutoGenerate: true,
			IsActive:     true,
			CreatedAt:    "2024-01-01T00:00:00Z",
			UpdatedAt:    "2024-01-02T00:00:00Z",
		},
		{
			ID:           "cred-2",
			UserID:       "user-123",
			Store:        "amazon",
			Credentials:  map[string]string{"tag": "mytag", "accessKey": "ak", "secretKey": "sk"},
			AutoGenerate: false,
			IsActive:     true,
			CreatedAt:    "2024-01-03T00:00:00Z",
			UpdatedAt:    "2024-01-04T00:00:00Z",
		},
	}

	repo.On("GetAllByUser", ctx, "user-123").Return(credentials, nil)

	req := connect.NewRequest(&affiliatesv1.ListCredentialsRequest{})

	resp, err := handler.ListCredentials(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetCredentials(), 2)
	assert.Equal(t, "cred-1", resp.Msg.GetCredentials()[0].GetId())
	assert.Equal(t, affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE, resp.Msg.GetCredentials()[0].GetStore())
	assert.Equal(t, "cred-2", resp.Msg.GetCredentials()[1].GetId())
	assert.Equal(t, affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON, resp.Msg.GetCredentials()[1].GetStore())
	repo.AssertExpectations(t)
}

func TestListCredentials_Unauthenticated(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.ListCredentialsRequest{})

	_, err := handler.ListCredentials(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestListCredentials_RepoError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetAllByUser", ctx, "user-123").Return(nil, errors.New("db connection error"))

	req := connect.NewRequest(&affiliatesv1.ListCredentialsRequest{})

	_, err := handler.ListCredentials(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

func TestListCredentials_EmptyResult(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetAllByUser", ctx, "user-123").Return([]models.AffiliateCredentials{}, nil)

	req := connect.NewRequest(&affiliatesv1.ListCredentialsRequest{})

	resp, err := handler.ListCredentials(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetCredentials(), 0)
	repo.AssertExpectations(t)
}

// =============================================================================
// GetCredential Tests
// =============================================================================

func TestGetCredential_Success(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	credential := &models.AffiliateCredentials{
		ID:           "cred-1",
		UserID:       "user-123",
		Store:        "shopee",
		Credentials:  map[string]string{"appId": "123", "password": "secret"},
		AutoGenerate: true,
		IsActive:     true,
		CreatedAt:    "2024-01-01T00:00:00Z",
		UpdatedAt:    "2024-01-02T00:00:00Z",
	}

	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(credential, nil)

	req := connect.NewRequest(&affiliatesv1.GetCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	resp, err := handler.GetCredential(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.GetCredential())
	assert.Equal(t, "cred-1", resp.Msg.GetCredential().GetId())
	assert.Equal(t, affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE, resp.Msg.GetCredential().GetStore())
	assert.True(t, resp.Msg.GetCredential().GetAutoGenerate())
	assert.True(t, resp.Msg.GetCredential().GetIsActive())
	repo.AssertExpectations(t)
}

func TestGetCredential_Unauthenticated(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.GetCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	_, err := handler.GetCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetCredential_InvalidStore(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.GetCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED,
	})

	_, err := handler.GetCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetCredential_NotFound(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(nil, nil)

	req := connect.NewRequest(&affiliatesv1.GetCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	resp, err := handler.GetCredential(ctx, req)

	require.NoError(t, err)
	assert.Nil(t, resp.Msg.GetCredential())
	repo.AssertExpectations(t)
}

func TestGetCredential_RepoError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(nil, errors.New("db error"))

	req := connect.NewRequest(&affiliatesv1.GetCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	_, err := handler.GetCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

// =============================================================================
// UpsertCredential Tests
// =============================================================================

func TestUpsertCredential_Success(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Upsert", ctx, "user-123", "shopee", mock.AnythingOfType("*models.AffiliateCredentials")).Return(nil)
	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(&models.AffiliateCredentials{
		ID:           "cred-1",
		UserID:       "user-123",
		Store:        "shopee",
		Credentials:  map[string]string{"appId": "123", "password": "secret"},
		AutoGenerate: true,
		IsActive:     true,
		CreatedAt:    "2024-01-01T00:00:00Z",
		UpdatedAt:    "2024-01-02T00:00:00Z",
	}, nil)

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"123","password":"secret"}`,
	})

	resp, err := handler.UpsertCredential(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.GetCredential())
	assert.Equal(t, "cred-1", resp.Msg.GetCredential().GetId())
	repo.AssertExpectations(t)
}

func TestUpsertCredential_Unauthenticated(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"123","password":"secret"}`,
	})

	_, err := handler.UpsertCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpsertCredential_InvalidStore(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED,
		Credentials: `{"appId":"123","password":"secret"}`,
	})

	_, err := handler.UpsertCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpsertCredential_InvalidJSON(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `not-valid-json`,
	})

	_, err := handler.UpsertCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpsertCredential_AutoGenerateNilDefaultsToTrue(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Upsert", ctx, "user-123", "shopee", mock.MatchedBy(func(data *models.AffiliateCredentials) bool {
		return data.AutoGenerate == true
	})).Return(nil)
	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(&models.AffiliateCredentials{
		ID:           "cred-1",
		UserID:       "user-123",
		Store:        "shopee",
		AutoGenerate: true,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"123","password":"secret"}`,
		// AutoGenerate is nil (not set)
	})

	resp, err := handler.UpsertCredential(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.GetCredential())
	repo.AssertExpectations(t)
}

func TestUpsertCredential_AutoGenerateExplicitFalse(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	autoGen := false

	repo.On("Upsert", ctx, "user-123", "shopee", mock.MatchedBy(func(data *models.AffiliateCredentials) bool {
		return data.AutoGenerate == false
	})).Return(nil)
	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(&models.AffiliateCredentials{
		ID:           "cred-1",
		UserID:       "user-123",
		Store:        "shopee",
		AutoGenerate: false,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:        affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials:  `{"appId":"123","password":"secret"}`,
		AutoGenerate: &autoGen,
	})

	resp, err := handler.UpsertCredential(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.GetCredential())
	repo.AssertExpectations(t)
}

func TestUpsertCredential_IsActiveFromHasValidCredentials(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	// Shopee requires appId + password for HasValidCredentials to return true
	repo.On("Upsert", ctx, "user-123", "shopee", mock.MatchedBy(func(data *models.AffiliateCredentials) bool {
		return data.IsActive == true
	})).Return(nil)
	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(&models.AffiliateCredentials{
		ID:       "cred-1",
		UserID:   "user-123",
		Store:    "shopee",
		IsActive: true,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"my-app-id","password":"my-password"}`,
	})

	resp, err := handler.UpsertCredential(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.GetCredential())
	repo.AssertExpectations(t)
}

func TestUpsertCredential_RepoUpsertError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Upsert", ctx, "user-123", "shopee", mock.AnythingOfType("*models.AffiliateCredentials")).
		Return(errors.New("db write error"))

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"123","password":"secret"}`,
	})

	_, err := handler.UpsertCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

func TestUpsertCredential_RepoGetAfterUpsertError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Upsert", ctx, "user-123", "shopee", mock.AnythingOfType("*models.AffiliateCredentials")).Return(nil)
	repo.On("GetByUserAndStore", ctx, "user-123", "shopee").Return(nil, errors.New("db read error"))

	req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
		Credentials: `{"appId":"123","password":"secret"}`,
	})

	_, err := handler.UpsertCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

// =============================================================================
// DeleteCredential Tests
// =============================================================================

func TestDeleteCredential_Success(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Delete", ctx, "user-123", "shopee").Return(nil)

	req := connect.NewRequest(&affiliatesv1.DeleteCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	_, err := handler.DeleteCredential(ctx, req)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteCredential_Unauthenticated(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.DeleteCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	_, err := handler.DeleteCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDeleteCredential_InvalidStore(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.DeleteCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED,
	})

	_, err := handler.DeleteCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDeleteCredential_RepoError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("Delete", ctx, "user-123", "shopee").Return(errors.New("db error"))

	req := connect.NewRequest(&affiliatesv1.DeleteCredentialRequest{
		Store: affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
	})

	_, err := handler.DeleteCredential(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

// =============================================================================
// ListActiveStores Tests
// =============================================================================

func TestListActiveStores_Success(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	credentials := []models.AffiliateCredentials{
		{
			Store:        "shopee",
			Credentials:  map[string]string{"appId": "123", "password": "secret"},
			AutoGenerate: true,
			IsActive:     true,
		},
		{
			Store:        "amazon",
			Credentials:  map[string]string{"tag": "mytag", "accessKey": "ak", "secretKey": "sk"},
			AutoGenerate: false,
			IsActive:     true,
		},
	}

	repo.On("GetAllByUser", ctx, "user-123").Return(credentials, nil)

	req := connect.NewRequest(&affiliatesv1.ListActiveStoresRequest{})

	resp, err := handler.ListActiveStores(ctx, req)

	require.NoError(t, err)
	// Should always return all 7 known stores
	assert.Len(t, resp.Msg.GetStores(), 7)

	// Find shopee and amazon in the response and verify isConfigured
	for _, store := range resp.Msg.GetStores() {
		switch store.GetStore() {
		case affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE:
			assert.True(t, store.GetIsConfigured())
			assert.True(t, store.GetAutoGenerate())
		case affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON:
			assert.True(t, store.GetIsConfigured())
			assert.False(t, store.GetAutoGenerate())
		default:
			assert.False(t, store.GetIsConfigured())
		}
	}

	repo.AssertExpectations(t)
}

func TestListActiveStores_Unauthenticated(t *testing.T) {
	handler, _ := createAffiliateHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.ListActiveStoresRequest{})

	_, err := handler.ListActiveStores(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestListActiveStores_RepoError(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetAllByUser", ctx, "user-123").Return(nil, errors.New("db error"))

	req := connect.NewRequest(&affiliatesv1.ListActiveStoresRequest{})

	_, err := handler.ListActiveStores(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	repo.AssertExpectations(t)
}

func TestListActiveStores_NoConfiguredStores(t *testing.T) {
	handler, repo := createAffiliateHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	repo.On("GetAllByUser", ctx, "user-123").Return([]models.AffiliateCredentials{}, nil)

	req := connect.NewRequest(&affiliatesv1.ListActiveStoresRequest{})

	resp, err := handler.ListActiveStores(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetStores(), 7)

	for _, store := range resp.Msg.GetStores() {
		assert.False(t, store.GetIsConfigured())
		assert.False(t, store.GetAutoGenerate())
	}

	repo.AssertExpectations(t)
}

// =============================================================================
// modelStoreToProtoAffiliate Tests
// =============================================================================

func TestModelStoreToProtoAffiliate(t *testing.T) {
	tests := []struct {
		input    string
		expected affiliatesv1.AffiliateStore
	}{
		{"shopee", affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE},
		{"amazon", affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON},
		{"magalu", affiliatesv1.AffiliateStore_AFFILIATE_STORE_MAGALU},
		{"mercado_livre", affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE},
		{"mercadolivre", affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE},
		{"ml", affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE},
		{"shein", affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHEIN},
		{"aliexpress", affiliatesv1.AffiliateStore_AFFILIATE_STORE_ALIEXPRESS},
		{"awin", affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN},
		{"unknown", affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := modelStoreToProtoAffiliate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// protoStoreToModelAffiliate Tests
// =============================================================================

func TestProtoStoreToModelAffiliate(t *testing.T) {
	tests := []struct {
		input    affiliatesv1.AffiliateStore
		expected string
	}{
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE, "shopee"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_AMAZON, "amazon"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_MAGALU, "magalu"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_MERCADO_LIVRE, "mercado_livre"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHEIN, "shein"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_ALIEXPRESS, "aliexpress"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN, "awin"},
		{affiliatesv1.AffiliateStore_AFFILIATE_STORE_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := protoStoreToModelAffiliate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
