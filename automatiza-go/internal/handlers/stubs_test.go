package handlers

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/coupons"
	"automatiza-go/internal/services/jobs"
	"automatiza-go/internal/services/loadbalancer"
	"automatiza-go/internal/services/user"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// SettingsHandler
// ---------------------------------------------------------------------------

func TestSettingsHandler_Get_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SettingsRepo.GetAllFunc = func(ctx context.Context) (models.SettingsMap, error) {
		return models.SettingsMap{"theme": "dark", "lang": "pt"}, nil
	}

	h := NewSettingsHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/settings", "")
	h.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestSettingsHandler_Get_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SettingsRepo.GetAllFunc = func(ctx context.Context) (models.SettingsMap, error) {
		return nil, errors.New("db error")
	}

	h := NewSettingsHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/settings", "")
	h.Get(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSettingsHandler_Update_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SettingsRepo.UpdateFunc = func(ctx context.Context, key string, value interface{}) error {
		return nil
	}

	h := NewSettingsHandler(deps)
	w, c := newAdminContext("PUT", "/api/v1/settings", `{"key":"setting_key","value":"new_value"}`)
	h.Update(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["updated"] != true {
		t.Fatalf("expected updated=true, got %v", data["updated"])
	}
}

func TestSettingsHandler_Update_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	h := NewSettingsHandler(deps)
	w, c := newAdminContext("PUT", "/api/v1/settings", `{invalid}`)
	h.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSettingsHandler_Update_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SettingsRepo.UpdateFunc = func(ctx context.Context, key string, value interface{}) error {
		return errors.New("db error")
	}

	h := NewSettingsHandler(deps)
	w, c := newAdminContext("PUT", "/api/v1/settings", `{"key":"setting_key","value":"new_value"}`)
	h.Update(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HistoryHandler
// ---------------------------------------------------------------------------

func TestHistoryHandler_List_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ActionHistoryRepo.GetUserHistoryFunc = func(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
		return []models.ActionHistory{{ID: "h1", UserID: userID}, {ID: "h2", UserID: userID}}, nil
	}

	h := NewHistoryHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/history", "")
	h.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	history, ok := data["history"].([]interface{})
	if !ok {
		t.Fatalf("expected history array, got %T", data["history"])
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(history))
	}
	if data["total"] != float64(2) {
		t.Fatalf("expected total=2, got %v", data["total"])
	}
}

func TestHistoryHandler_List_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ActionHistoryRepo.GetUserHistoryFunc = func(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
		return nil, errors.New("db error")
	}

	h := NewHistoryHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/history", "")
	h.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// ContactsHandler
// ---------------------------------------------------------------------------

func TestContactsHandler_Extract_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.GetGroupParticipantsFunc = func(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error) {
		return []whatsapp.ParticipantInfo{
			{ID: "5511999999999@c.us", IsAdmin: true},
			{ID: "5511888888888@c.us", IsAdmin: false},
		}, nil
	}

	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{"instanceName":"inst-1","groupId":"g1@g.us"}`)
	h.Extract(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestContactsHandler_Extract_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{invalid}`)
	h.Extract(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestContactsHandler_Extract_NilProvider(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.WhatsAppProvider = nil

	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{"instanceName":"inst-1","groupId":"g1@g.us"}`)
	h.Extract(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestContactsHandler_Extract_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{"instanceName":"inst-1","groupId":"g1@g.us"}`)
	h.Extract(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestContactsHandler_Extract_InstanceLookupError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, errors.New("db error")
	}

	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{"instanceName":"inst-1","groupId":"g1@g.us"}`)
	h.Extract(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestContactsHandler_Extract_ExtractError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name}, nil
	}
	mocks.WhatsAppProvider.GetGroupParticipantsFunc = func(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error) {
		return nil, errors.New("whatsapp error")
	}

	h := NewContactsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/contacts/extract", `{"instanceName":"inst-1","groupId":"g1@g.us"}`)
	h.Extract(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// ShopeeHandler
// ---------------------------------------------------------------------------

func TestShopeeHandler_SearchOffers_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ShopeeService.SearchOffersFunc = func(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
		return &models.ShopeeSearchResult{}, nil
	}

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/search", `{"keyword":"test"}`)
	h.SearchOffers(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestShopeeHandler_SearchOffers_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.ShopeeService = nil

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/search", `{"keyword":"test"}`)
	h.SearchOffers(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestShopeeHandler_SearchOffers_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ShopeeService.SearchOffersFunc = func(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
		return nil, errors.New("search error")
	}

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/search", `{"keyword":"test"}`)
	h.SearchOffers(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestShopeeHandler_GenerateAffiliateLink_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ShopeeService.GenerateAffiliateLinkFunc = func(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
		return &models.ShopeeAffiliateLink{}, nil
	}

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/link", `{"productUrl":"https://shopee.com.br/product"}`)
	h.GenerateAffiliateLink(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestShopeeHandler_GenerateAffiliateLink_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.ShopeeService = nil

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/link", `{"productUrl":"https://shopee.com.br/product"}`)
	h.GenerateAffiliateLink(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestShopeeHandler_GenerateAffiliateLink_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.ShopeeService.GenerateAffiliateLinkFunc = func(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
		return nil, errors.New("link error")
	}

	h := NewShopeeHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/shopee/link", `{"productUrl":"https://shopee.com.br/product"}`)
	h.GenerateAffiliateLink(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// MLHandler
// ---------------------------------------------------------------------------

func TestMLHandler_CreateLink_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.CreateAffiliateLinkFunc = func(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
		return &models.MLAffiliateLink{}, nil
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/link", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.CreateLink(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMLHandler_CreateLink_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.MLService = nil

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/link", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.CreateLink(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestMLHandler_CreateLink_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.CreateAffiliateLinkFunc = func(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
		return nil, errors.New("ml error")
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/link", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.CreateLink(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestMLHandler_Preview_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.GetProductPreviewFunc = func(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
		return &models.MLProductPreview{}, nil
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/preview", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.Preview(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMLHandler_Preview_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.MLService = nil

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/preview", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.Preview(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestMLHandler_Preview_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.GetProductPreviewFunc = func(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
		return nil, errors.New("preview error")
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/preview", `{"productUrl":"https://mercadolivre.com/product"}`)
	h.Preview(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestMLHandler_Connect_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.SaveCredentialsFunc = func(ctx context.Context, userID, cookie, csrf, tag string) error {
		return nil
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/connect", `{"cookie":"xxx","csrf":"yyy"}`)
	h.Connect(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["connected"] != true {
		t.Fatalf("expected connected=true, got %v", data["connected"])
	}
}

func TestMLHandler_Connect_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.MLService = nil

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/connect", `{"cookie":"xxx","csrf":"yyy"}`)
	h.Connect(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestMLHandler_Connect_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.MLService.SaveCredentialsFunc = func(ctx context.Context, userID, cookie, csrf, tag string) error {
		return errors.New("connect error")
	}

	h := NewMLHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/ml/connect", `{"cookie":"xxx","csrf":"yyy"}`)
	h.Connect(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// JobsHandler
// ---------------------------------------------------------------------------

func TestJobsHandler_ProcessScheduled_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.JobsService.ProcessScheduledJobsFunc = func(ctx context.Context) (*jobs.ProcessResult, error) {
		return &jobs.ProcessResult{Processed: 3, Failed: 1}, nil
	}

	h := NewJobsHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/jobs/process", "")
	h.ProcessScheduled(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["processedCount"] != float64(3) {
		t.Fatalf("expected processedCount=3, got %v", data["processedCount"])
	}
	if data["failedCount"] != float64(1) {
		t.Fatalf("expected failedCount=1, got %v", data["failedCount"])
	}
}

func TestJobsHandler_ProcessScheduled_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.JobsService = nil

	h := NewJobsHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/jobs/process", "")
	h.ProcessScheduled(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["processedCount"] != float64(0) {
		t.Fatalf("expected processedCount=0, got %v", data["processedCount"])
	}
}

func TestJobsHandler_ProcessScheduled_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.JobsService.ProcessScheduledJobsFunc = func(ctx context.Context) (*jobs.ProcessResult, error) {
		return nil, errors.New("process error")
	}

	h := NewJobsHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/jobs/process", "")
	h.ProcessScheduled(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestJobsHandler_QueueStatus_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.JobsService.GetQueueStatusFunc = func(ctx context.Context, userID string) (*jobs.QueueStatus, error) {
		return &jobs.QueueStatus{Pending: 5, Running: 2}, nil
	}

	h := NewJobsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/jobs/status", "")
	h.QueueStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["pending"] != float64(5) {
		t.Fatalf("expected pending=5, got %v", data["pending"])
	}
	if data["processing"] != float64(2) {
		t.Fatalf("expected processing=2, got %v", data["processing"])
	}
}

func TestJobsHandler_QueueStatus_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.JobsService = nil

	h := NewJobsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/jobs/status", "")
	h.QueueStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["pending"] != float64(0) {
		t.Fatalf("expected pending=0, got %v", data["pending"])
	}
	if data["processing"] != float64(0) {
		t.Fatalf("expected processing=0, got %v", data["processing"])
	}
}

func TestJobsHandler_QueueStatus_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.JobsService.GetQueueStatusFunc = func(ctx context.Context, userID string) (*jobs.QueueStatus, error) {
		return nil, errors.New("queue error")
	}

	h := NewJobsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/jobs/status", "")
	h.QueueStatus(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// WebhooksHandler
// ---------------------------------------------------------------------------

func TestWebhooksHandler_WhatsApp_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/whatsapp", `{"instanceName":"inst-1","event":"message"}`)
	h.WhatsApp(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["received"] != true {
		t.Fatalf("expected received=true, got %v", body["received"])
	}
}

func TestWebhooksHandler_WhatsApp_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/whatsapp", `{invalid}`)
	h.WhatsApp(c)

	// Webhooks always return 200 even on parse failure
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["received"] != true {
		t.Fatalf("expected received=true, got %v", body["received"])
	}
}

func TestWebhooksHandler_Kiwify_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.KiwifyService.ProcessWebhookFunc = func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
		return &models.KiwifyWebhookResult{Success: true, Message: "processed"}, nil
	}

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/kiwify",
		`{"Customer":{"email":"test@test.com","full_name":"Test"},"Product":{"product_name":"Test Product","product_id":"P1"},"webhook_event_type":"order_approved"}`)
	h.Kiwify(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestWebhooksHandler_Kiwify_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/kiwify", `{invalid}`)
	h.Kiwify(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebhooksHandler_Kiwify_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.KiwifyService = nil

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/kiwify",
		`{"Customer":{"email":"test@test.com","full_name":"Test"},"Product":{"product_name":"Test Product","product_id":"P1"},"webhook_event_type":"order_approved"}`)
	h.Kiwify(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["message"] == "" {
		t.Fatal("expected non-empty message for nil service")
	}
}

func TestWebhooksHandler_Kiwify_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.KiwifyService.ProcessWebhookFunc = func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
		return nil, errors.New("kiwify error")
	}

	h := NewWebhooksHandler(deps)
	w, c := newTestContext("POST", "/api/v1/webhooks/kiwify",
		`{"Customer":{"email":"test@test.com","full_name":"Test"},"Product":{"product_name":"Test Product","product_id":"P1"},"webhook_event_type":"order_approved"}`)
	h.Kiwify(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestWebhooksHandler_KiwifySimulate_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.KiwifyService.ProcessWebhookFunc = func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
		return &models.KiwifyWebhookResult{Success: true, Message: "simulated"}, nil
	}

	h := NewWebhooksHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/webhooks/kiwify/simulate",
		`{"email":"test@test.com","evento":"order_approved","produto":"Test"}`)
	h.KiwifySimulate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestWebhooksHandler_KiwifySimulate_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewWebhooksHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/webhooks/kiwify/simulate", `{invalid}`)
	h.KiwifySimulate(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebhooksHandler_KiwifySimulate_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.KiwifyService = nil

	h := NewWebhooksHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/webhooks/kiwify/simulate",
		`{"email":"test@test.com","evento":"order_approved","produto":"Test"}`)
	h.KiwifySimulate(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// RedirectHandler
// ---------------------------------------------------------------------------

func TestRedirectHandler_Process_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewRedirectHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/redirect", `{"url":"https://example.com"}`)
	h.Process(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["originalUrl"] == nil && data["originalURL"] == nil {
		t.Fatal("expected originalUrl or originalURL in response data")
	}
}

func TestRedirectHandler_Process_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewRedirectHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/redirect", `{invalid}`)
	h.Process(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// InstancesHandler
// ---------------------------------------------------------------------------

func TestInstancesHandler_SyncWebhook_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByNameFunc = func(ctx context.Context, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SetWebhookFunc = func(ctx context.Context, input whatsapp.SetWebhookInput) error {
		return nil
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/inst-1/sync-webhook", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "inst-1"})
	h.SyncWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["synced"] != true {
		t.Fatalf("expected synced=true, got %v", data["synced"])
	}
}

func TestInstancesHandler_SyncWebhook_NilProvider(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.WhatsAppProvider = nil

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/inst-1/sync-webhook", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "inst-1"})
	h.SyncWebhook(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestInstancesHandler_SyncWebhook_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByNameFunc = func(ctx context.Context, name string) (*models.Instance, error) {
		return nil, nil
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/inst-1/sync-webhook", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "inst-1"})
	h.SyncWebhook(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestInstancesHandler_SyncWebhook_SetWebhookError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetInstanceByNameFunc = func(ctx context.Context, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SetWebhookFunc = func(ctx context.Context, input whatsapp.SetWebhookInput) error {
		return errors.New("webhook error")
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/inst-1/sync-webhook", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "inst-1"})
	h.SyncWebhook(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestInstancesHandler_SyncAllWebhooks_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetAllConnectedFunc = func(ctx context.Context) ([]models.Instance, error) {
		return []models.Instance{
			{ID: 1, InstanceID: "inst-1"},
			{ID: 2, InstanceID: "inst-2"},
		}, nil
	}
	mocks.WhatsAppProvider.SetWebhookFunc = func(ctx context.Context, input whatsapp.SetWebhookInput) error {
		return nil
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/sync-all-webhooks", "")
	h.SyncAllWebhooks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["synced"] != float64(2) {
		t.Fatalf("expected synced=2, got %v", data["synced"])
	}
	if data["failed"] != float64(0) {
		t.Fatalf("expected failed=0, got %v", data["failed"])
	}
	if data["total"] != float64(2) {
		t.Fatalf("expected total=2, got %v", data["total"])
	}
}

func TestInstancesHandler_SyncAllWebhooks_NilProvider(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.WhatsAppProvider = nil

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/sync-all-webhooks", "")
	h.SyncAllWebhooks(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestInstancesHandler_SyncAllWebhooks_GetInstancesError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetAllConnectedFunc = func(ctx context.Context) ([]models.Instance, error) {
		return nil, errors.New("db error")
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/sync-all-webhooks", "")
	h.SyncAllWebhooks(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestInstancesHandler_SyncAllWebhooks_PartialFailures(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.InstanceRepo.GetAllConnectedFunc = func(ctx context.Context) ([]models.Instance, error) {
		return []models.Instance{
			{ID: 1, InstanceID: "inst-1"},
			{ID: 2, InstanceID: "inst-2"},
			{ID: 3, InstanceID: "inst-3"},
		}, nil
	}
	mocks.WhatsAppProvider.SetWebhookFunc = func(ctx context.Context, input whatsapp.SetWebhookInput) error {
		if input.SessionName == "inst-2" {
			return errors.New("webhook error for inst-2")
		}
		return nil
	}

	h := NewInstancesHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/instances/sync-all-webhooks", "")
	h.SyncAllWebhooks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["synced"] != float64(2) {
		t.Fatalf("expected synced=2, got %v", data["synced"])
	}
	if data["failed"] != float64(1) {
		t.Fatalf("expected failed=1, got %v", data["failed"])
	}
	if data["total"] != float64(3) {
		t.Fatalf("expected total=3, got %v", data["total"])
	}
}

// ---------------------------------------------------------------------------
// AdminHandler
// ---------------------------------------------------------------------------

func TestAdminHandler_ResetPassword_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.UserService.ResetUserPasswordFunc = func(ctx context.Context, userID, adminID, adminEmail string) (*user.PasswordResetResult, error) {
		return &user.PasswordResetResult{Success: true, Message: "Password reset"}, nil
	}

	h := NewAdminHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/admin/users/target-user/reset-password", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "target-user"})
	h.ResetPassword(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	if body["success"] != true {
		t.Fatalf("expected success=true, got %v", body["success"])
	}
}

func TestAdminHandler_ResetPassword_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.UserService = nil

	h := NewAdminHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/admin/users/target-user/reset-password", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "target-user"})
	h.ResetPassword(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAdminHandler_ResetPassword_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.UserService.ResetUserPasswordFunc = func(ctx context.Context, userID, adminID, adminEmail string) (*user.PasswordResetResult, error) {
		return nil, errors.New("reset error")
	}

	h := NewAdminHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/admin/users/target-user/reset-password", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "target-user"})
	h.ResetPassword(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAdminHandler_Observability_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewAdminHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/admin/observability", "")
	h.Observability(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", data["status"])
	}
}

func TestAdminHandler_EnsureWebhooks_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewAdminHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/admin/ensure-webhooks", "")
	h.EnsureWebhooks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["ensured"] != true {
		t.Fatalf("expected ensured=true, got %v", data["ensured"])
	}
}

func TestAdminHandler_ShopeeAffiliateAudit_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewAdminHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/admin/shopee-audit", "")
	h.ShopeeAffiliateAudit(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	audit, ok := data["audit"].([]interface{})
	if !ok {
		t.Fatalf("expected audit array, got %T", data["audit"])
	}
	if len(audit) != 0 {
		t.Fatalf("expected empty audit array, got %d items", len(audit))
	}
}

func TestAdminHandler_SystemEvents_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SystemEventRepo.GetRecentFunc = func(ctx context.Context, limit int) ([]models.SystemEvent, error) {
		return []models.SystemEvent{{ID: 1, EventType: "test"}}, nil
	}

	h := NewAdminHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/admin/system-events", "")
	h.SystemEvents(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	events, ok := data["events"].([]interface{})
	if !ok {
		t.Fatalf("expected events array, got %T", data["events"])
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestAdminHandler_SystemEvents_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.SystemEventRepo.GetRecentFunc = func(ctx context.Context, limit int) ([]models.SystemEvent, error) {
		return nil, errors.New("db error")
	}

	h := NewAdminHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/admin/system-events", "")
	h.SystemEvents(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CouponsHandler
// ---------------------------------------------------------------------------

func TestCouponsHandler_List_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetUserCouponsFunc = func(ctx context.Context, userID string, filters *coupons.CouponFilters) ([]models.Coupon, error) {
		return []models.Coupon{{ID: "c1", Code: "TEST10"}, {ID: "c2", Code: "TEST20"}}, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons", "")
	h.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	couponsArr, ok := data["coupons"].([]interface{})
	if !ok {
		t.Fatalf("expected coupons array, got %T", data["coupons"])
	}
	if len(couponsArr) != 2 {
		t.Fatalf("expected 2 coupons, got %d", len(couponsArr))
	}
	if data["total"] != float64(2) {
		t.Fatalf("expected total=2, got %v", data["total"])
	}
}

func TestCouponsHandler_List_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons", "")
	h.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["total"] != float64(0) {
		t.Fatalf("expected total=0, got %v", data["total"])
	}
}

func TestCouponsHandler_List_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetUserCouponsFunc = func(ctx context.Context, userID string, filters *coupons.CouponFilters) ([]models.Coupon, error) {
		return nil, errors.New("db error")
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons", "")
	h.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCouponsHandler_Get_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetCouponByIDFunc = func(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
		return &models.Coupon{ID: couponID, Code: "TEST10"}, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCouponsHandler_Get_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCouponsHandler_Get_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetCouponByIDFunc = func(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
		return nil, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCouponsHandler_GetByStore_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetCouponsByStoreFunc = func(ctx context.Context, userID, store string) ([]models.Coupon, error) {
		return []models.Coupon{{ID: "c1", Code: "TEST10"}}, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/store/shopee", "")
	c.Params = append(c.Params, gin.Param{Key: "store", Value: "shopee"})
	h.GetByStore(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCouponsHandler_GetByStore_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/store/shopee", "")
	c.Params = append(c.Params, gin.Param{Key: "store", Value: "shopee"})
	h.GetByStore(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["total"] != float64(0) {
		t.Fatalf("expected total=0, got %v", data["total"])
	}
}

func TestCouponsHandler_GetByStore_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.GetCouponsByStoreFunc = func(ctx context.Context, userID, store string) ([]models.Coupon, error) {
		return nil, errors.New("db error")
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("GET", "/api/v1/coupons/store/shopee", "")
	c.Params = append(c.Params, gin.Param{Key: "store", Value: "shopee"})
	h.GetByStore(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCouponsHandler_Create_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.CreateCouponFunc = func(ctx context.Context, userID string, data *coupons.CreateCouponData) (*models.Coupon, error) {
		return &models.Coupon{ID: "new-c1", Code: "TEST10"}, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/coupons", `{"code":"TEST10","title":"Test","store":"shopee"}`)
	h.Create(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestCouponsHandler_Create_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/coupons", `{invalid}`)
	h.Create(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCouponsHandler_Create_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/coupons", `{"code":"TEST10","title":"Test","store":"shopee"}`)
	h.Create(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCouponsHandler_Create_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.CreateCouponFunc = func(ctx context.Context, userID string, data *coupons.CreateCouponData) (*models.Coupon, error) {
		return nil, errors.New("create error")
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("POST", "/api/v1/coupons", `{"code":"TEST10","title":"Test","store":"shopee"}`)
	h.Create(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCouponsHandler_Update_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.UpdateCouponFunc = func(ctx context.Context, couponID, userID string, data *coupons.UpdateCouponData) (*models.Coupon, error) {
		return &models.Coupon{ID: couponID, Code: "TEST20"}, nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("PUT", "/api/v1/coupons/c1", `{"code":"TEST20"}`)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Update(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCouponsHandler_Update_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("PUT", "/api/v1/coupons/c1", `{invalid}`)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCouponsHandler_Update_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("PUT", "/api/v1/coupons/c1", `{"code":"TEST20"}`)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Update(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCouponsHandler_Update_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.UpdateCouponFunc = func(ctx context.Context, couponID, userID string, data *coupons.UpdateCouponData) (*models.Coupon, error) {
		return nil, errors.New("update error")
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("PUT", "/api/v1/coupons/c1", `{"code":"TEST20"}`)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Update(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCouponsHandler_Delete_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.DeleteCouponFunc = func(ctx context.Context, couponID, userID string) error {
		return nil
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("DELETE", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Delete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["deleted"] != true {
		t.Fatalf("expected deleted=true, got %v", data["deleted"])
	}
}

func TestCouponsHandler_Delete_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.CouponsService = nil

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("DELETE", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Delete(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCouponsHandler_Delete_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.CouponsService.DeleteCouponFunc = func(ctx context.Context, couponID, userID string) error {
		return errors.New("delete error")
	}

	h := NewCouponsHandler(deps)
	w, c := newAuthContext("DELETE", "/api/v1/coupons/c1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "c1"})
	h.Delete(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// LoadBalancerHandler
// ---------------------------------------------------------------------------

func TestLoadBalancerHandler_Stats_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.GetServerStatsFunc = func(ctx context.Context) (*loadbalancer.ServerStats, error) {
		return &loadbalancer.ServerStats{
			TotalUsers: 10,
			Servers: []loadbalancer.ServerInfo{
				{ID: "srv-1", URL: "http://waha:3000", UserCount: 10, MaxUsers: 100, Utilization: 10.0, IsActive: true},
			},
		}, nil
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/stats", "")
	h.Stats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	servers, ok := data["servers"].([]interface{})
	if !ok {
		t.Fatalf("expected servers array, got %T", data["servers"])
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
}

func TestLoadBalancerHandler_Stats_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.LBService = nil

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/stats", "")
	h.Stats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	servers, ok := data["servers"].([]interface{})
	if !ok {
		t.Fatalf("expected servers array, got %T", data["servers"])
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(servers))
	}
}

func TestLoadBalancerHandler_Stats_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.GetServerStatsFunc = func(ctx context.Context) (*loadbalancer.ServerStats, error) {
		return nil, errors.New("stats error")
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/stats", "")
	h.Stats(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestLoadBalancerHandler_GetUserServer_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.GetServerForUserFunc = func(ctx context.Context, userID string) (*models.WahaServer, error) {
		return &models.WahaServer{ID: "srv-1", URL: "http://waha:3000"}, nil
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/users/u1/server", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "u1"})
	h.GetUserServer(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["server"] == nil {
		t.Fatal("expected non-nil server in response")
	}
}

func TestLoadBalancerHandler_GetUserServer_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.LBService = nil

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/users/u1/server", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "u1"})
	h.GetUserServer(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["server"] != nil {
		t.Fatalf("expected server=nil, got %v", data["server"])
	}
}

func TestLoadBalancerHandler_GetUserServer_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.GetServerForUserFunc = func(ctx context.Context, userID string) (*models.WahaServer, error) {
		return nil, errors.New("server error")
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("GET", "/api/v1/lb/users/u1/server", "")
	c.Params = append(c.Params, gin.Param{Key: "userId", Value: "u1"})
	h.GetUserServer(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestLoadBalancerHandler_Migrate_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.MigrateUserFunc = func(ctx context.Context, userID, targetServerID string) error {
		return nil
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/lb/migrate", `{"userId":"u1","targetServerId":"srv-2"}`)
	h.Migrate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["success"] != true {
		t.Fatalf("expected success=true, got %v", data["success"])
	}
}

func TestLoadBalancerHandler_Migrate_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/lb/migrate", `{invalid}`)
	h.Migrate(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoadBalancerHandler_Migrate_NilService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.LBService = nil

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/lb/migrate", `{"userId":"u1","targetServerId":"srv-2"}`)
	h.Migrate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := mustParseBody(w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", body["data"])
	}
	if data["success"] != false {
		t.Fatalf("expected success=false, got %v", data["success"])
	}
}

func TestLoadBalancerHandler_Migrate_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.LBService.MigrateUserFunc = func(ctx context.Context, userID, targetServerID string) error {
		return errors.New("migrate error")
	}

	h := NewLoadBalancerHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/lb/migrate", `{"userId":"u1","targetServerId":"srv-2"}`)
	h.Migrate(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// stringVal helper
// ---------------------------------------------------------------------------

func TestWebhooksHandler_KiwifySimulate_MissingProductName(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	var capturedPayload *models.KiwifyWebhookPayload
	mocks.KiwifyService.ProcessWebhookFunc = func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
		capturedPayload = payload
		return &models.KiwifyWebhookResult{Success: true, Message: "simulated"}, nil
	}

	h := NewWebhooksHandler(deps)
	// "produto" is empty — handler should default to "Test Product"
	w, c := newAdminContext("POST", "/api/v1/webhooks/kiwify/simulate",
		`{"email":"test@test.com","evento":"order_approved","produto":""}`)
	h.KiwifySimulate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedPayload == nil {
		t.Fatal("expected ProcessWebhook to be called")
	}
	if capturedPayload.Product.ProductName != "Test Product" {
		t.Fatalf("expected product name 'Test Product', got %q", capturedPayload.Product.ProductName)
	}
}

func TestWebhooksHandler_KiwifySimulate_ProcessWebhookError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	mocks.KiwifyService.ProcessWebhookFunc = func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
		return nil, errors.New("processing failed")
	}

	h := NewWebhooksHandler(deps)
	w, c := newAdminContext("POST", "/api/v1/webhooks/kiwify/simulate",
		`{"email":"test@test.com","evento":"order_approved","produto":"Test"}`)
	h.KiwifySimulate(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestStringVal_NilPointer(t *testing.T) {
	t.Parallel()
	result := stringVal(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil pointer, got %q", result)
	}
}

func TestStringVal_ValidPointer(t *testing.T) {
	t.Parallel()
	v := "hello"
	result := stringVal(&v)
	if result != "hello" {
		t.Fatalf("expected %q, got %q", "hello", result)
	}
}

// ---------------------------------------------------------------------------
// ShopeeHandler.SearchOffers — ShouldBindJSON error (invalid JSON body)
// ---------------------------------------------------------------------------

func TestShopeeHandler_SearchOffers_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewShopeeHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/shopee/search-offers", `{invalid}`)
	handler.SearchOffers(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// ShopeeHandler.GenerateAffiliateLink — ShouldBindJSON error (invalid JSON body)
// ---------------------------------------------------------------------------

func TestShopeeHandler_GenerateAffiliateLink_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewShopeeHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/shopee/generate-link", `{invalid}`)
	handler.GenerateAffiliateLink(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// MLHandler.CreateLink — ShouldBindJSON error (invalid JSON body)
// ---------------------------------------------------------------------------

func TestMLHandler_CreateLink_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewMLHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/ml/create-link", `{invalid}`)
	handler.CreateLink(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// MLHandler.Preview — ShouldBindJSON error (invalid JSON body)
// ---------------------------------------------------------------------------

func TestMLHandler_Preview_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewMLHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/ml/preview", `{invalid}`)
	handler.Preview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// MLHandler.Connect — ShouldBindJSON error (invalid JSON body)
// ---------------------------------------------------------------------------

func TestMLHandler_Connect_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewMLHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/ml/connect", `{invalid}`)
	handler.Connect(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// WebhooksHandler.WhatsApp — Monitoring ProcessIncomingMessage error (SafeGo)
// ---------------------------------------------------------------------------

func TestWebhooksHandler_WhatsApp_MonitoringError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWebhooksHandler(deps)

	errCh := make(chan error, 1)
	mocks.MonitoringService.ProcessIncomingMessageFunc = func(ctx context.Context, msg *models.IncomingMessage) error {
		err := fmt.Errorf("monitoring failure")
		errCh <- err
		return err
	}

	body := `{"instanceName":"inst-1","body":"hello","messageId":"msg-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/webhooks/whatsapp", body)
	handler.WhatsApp(c)

	// Webhook always returns 200
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Wait for the SafeGo goroutine to finish processing
	select {
	case <-errCh:
		// ProcessIncomingMessage was called and returned an error — the error
		// is handled by observability.LogAndCapture inside SafeGo, covering lines 286-294.
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for monitoring goroutine to fire")
	}
}
