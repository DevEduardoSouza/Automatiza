package handlers

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// CreateInstance
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_CreateInstance_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.WhatsAppProvider.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
		if input.SessionName != "my-instance" {
			t.Errorf("expected session name 'my-instance', got %q", input.SessionName)
		}
		return &whatsapp.SessionInfo{Name: input.SessionName, Status: whatsapp.SessionStatusScanQR}, nil
	}
	mocks.InstanceRepo.UpsertInstanceFunc = func(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
		data.ID = 1
		return data, nil
	}
	mocks.WhatsAppProvider.GetQRCodeFunc = func(ctx context.Context, name string) (*whatsapp.QRCodeData, error) {
		return &whatsapp.QRCodeData{Base64Image: "qr-base64"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"my-instance"}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["instanceName"] != "my-instance" {
		t.Errorf("expected instanceName 'my-instance', got %v", data["instanceName"])
	}
	if data["qrcode"] == nil {
		t.Error("expected qrcode in response")
	}
}

func TestWhatsAppHandler_CreateInstance_EmptyName_AutoGenerates(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	name, ok := data["instanceName"].(string)
	if !ok || name == "" {
		t.Error("expected auto-generated instanceName, got empty or missing")
	}
}

func TestWhatsAppHandler_CreateInstance_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{invalid}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CreateInstance_NilProvider(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"inst-1"}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CreateInstance_CreateSessionError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.WhatsAppProvider.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
		return nil, errors.New("waha error")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"inst-1"}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CreateInstance_QuotaExceeded(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	called := false
	mocks.InstanceRepo.CountUserInstancesFunc = func(ctx context.Context, userID string) (int, error) {
		return 10, nil // at the cap
	}
	mocks.WhatsAppProvider.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
		called = true
		return &whatsapp.SessionInfo{Name: input.SessionName}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"inst-11"}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if called {
		t.Error("CreateSession should not be called when quota is exceeded")
	}
}

func TestWhatsAppHandler_CreateInstance_BelowQuota_Allowed(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.CountUserInstancesFunc = func(ctx context.Context, userID string) (int, error) {
		return 9, nil // one slot left
	}
	mocks.WhatsAppProvider.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{Name: input.SessionName, Status: whatsapp.SessionStatusScanQR}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"inst-10"}`)
	handler.CreateInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DeleteInstance
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_DeleteInstance_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.DeleteSessionFunc = func(ctx context.Context, name string) error {
		return nil
	}
	mocks.InstanceRepo.DeleteInstanceFunc = func(ctx context.Context, instanceID int) error {
		return nil
	}
	mocks.GroupRepo.DeleteByInstanceFn = func(ctx context.Context, userID string, instanceID int) error {
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["deleted"] != true {
		t.Errorf("expected deleted:true, got %v", data["deleted"])
	}
}

func TestWhatsAppHandler_DeleteInstance_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{invalid}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_DeleteInstance_MissingInstanceName(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_DeleteInstance_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// ReconnectInstance
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_ReconnectInstance_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "disconnected"}, nil
	}
	mocks.WhatsAppProvider.RestartSessionFunc = func(ctx context.Context, name string) error {
		return nil
	}
	mocks.WhatsAppProvider.GetQRCodeFunc = func(ctx context.Context, name string) (*whatsapp.QRCodeData, error) {
		return &whatsapp.QRCodeData{Base64Image: "qr-reconnect"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/reconnect-instance", `{"instanceName":"inst-1"}`)
	handler.ReconnectInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "reconnecting" {
		t.Errorf("expected status 'reconnecting', got %v", data["status"])
	}
	if data["qrcode"] == nil {
		t.Error("expected qrcode in response")
	}
}

func TestWhatsAppHandler_ReconnectInstance_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/reconnect-instance", `{invalid}`)
	handler.ReconnectInstance(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_ReconnectInstance_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/reconnect-instance", `{"instanceName":"inst-1"}`)
	handler.ReconnectInstance(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWhatsAppHandler_ReconnectInstance_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	// handler checks ownership BEFORE provider nil check
	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "disconnected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/reconnect-instance", `{"instanceName":"inst-1"}`)
	handler.ReconnectInstance(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_ReconnectInstance_RestartError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "disconnected"}, nil
	}
	mocks.WhatsAppProvider.RestartSessionFunc = func(ctx context.Context, name string) error {
		return errors.New("restart failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/reconnect-instance", `{"instanceName":"inst-1"}`)
	handler.ReconnectInstance(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CheckStatus
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_CheckStatus_Connected(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: models.InstanceStatusConnected}, nil
	}
	mocks.WhatsAppProvider.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{Name: name, Status: whatsapp.SessionStatusConnected}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status?instanceName=inst-1", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["connected"] != true {
		t.Errorf("expected connected:true, got %v", data["connected"])
	}
	// When connected, no QR code should be returned
	if data["qrcode"] != nil {
		t.Error("expected no qrcode for connected instance")
	}
}

func TestWhatsAppHandler_CheckStatus_NotConnected(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: models.InstanceStatusPending}, nil
	}
	mocks.WhatsAppProvider.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{Name: name, Status: whatsapp.SessionStatusScanQR}, nil
	}
	mocks.WhatsAppProvider.GetQRCodeFunc = func(ctx context.Context, name string) (*whatsapp.QRCodeData, error) {
		return &whatsapp.QRCodeData{Base64Image: "qr-data"}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status?instanceName=inst-1", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["connected"] != false {
		t.Errorf("expected connected:false, got %v", data["connected"])
	}
	if data["qrcode"] == nil {
		t.Error("expected qrcode in response for not-connected instance")
	}
}

func TestWhatsAppHandler_CheckStatus_MissingInstanceName(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CheckStatus_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status?instanceName=inst-1", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CheckStatus_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: models.InstanceStatusPending}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status?instanceName=inst-1", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_CheckStatus_StatusCheckError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: models.InstanceStatusPending}, nil
	}
	mocks.WhatsAppProvider.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return nil, errors.New("status check failed")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/check-status?instanceName=inst-1", "")
	handler.CheckStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "unknown" {
		t.Errorf("expected status 'unknown', got %v", data["status"])
	}
}

// ---------------------------------------------------------------------------
// FetchGroups
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_FetchGroups_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.GetGroupsFunc = func(ctx context.Context, name string) ([]whatsapp.GroupInfo, error) {
		return []whatsapp.GroupInfo{
			{ID: "g1@g.us", Subject: "Group 1", Size: 10},
			{ID: "g2@g.us", Subject: "Group 2", Size: 20},
		}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-groups?instanceName=inst-1", "")
	handler.FetchGroups(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	groups := data["groups"].([]interface{})
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
	if data["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", data["total"])
	}
}

func TestWhatsAppHandler_FetchGroups_MissingInstanceName(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-groups", "")
	handler.FetchGroups(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_FetchGroups_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-groups?instanceName=inst-1", "")
	handler.FetchGroups(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWhatsAppHandler_FetchGroups_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-groups?instanceName=inst-1", "")
	handler.FetchGroups(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_FetchGroups_GetGroupsError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.GetGroupsFunc = func(ctx context.Context, name string) ([]whatsapp.GroupInfo, error) {
		return nil, errors.New("fetch error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-groups?instanceName=inst-1", "")
	handler.FetchGroups(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SyncGroups
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SyncGroups_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/sync-groups", `{"instanceName":"inst-1"}`)
	handler.SyncGroups(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SyncGroups_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/sync-groups", `{invalid}`)
	handler.SyncGroups(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SyncGroupsStatus
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SyncGroupsStatus_Success(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/sync-groups-status", "")
	handler.SyncGroupsStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "completed" {
		t.Errorf("expected status 'completed', got %v", data["status"])
	}
}

// ---------------------------------------------------------------------------
// FetchGroupParticipants
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_FetchGroupParticipants_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.GetGroupParticipantsFunc = func(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error) {
		return []whatsapp.ParticipantInfo{
			{ID: "5511999999999@c.us", IsAdmin: true},
			{ID: "5511888888888@c.us", IsAdmin: false},
		}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?instanceName=inst-1&groupId=g1@g.us", "")
	handler.FetchGroupParticipants(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	participants := data["participants"].([]interface{})
	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}
	if data["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", data["total"])
	}
}

func TestWhatsAppHandler_FetchGroupParticipants_MissingParams(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	// Missing both params
	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants", "")
	handler.FetchGroupParticipants(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Missing groupId
	w2, c2 := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?instanceName=inst-1", "")
	handler.FetchGroupParticipants(c2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing groupId, got %d", w2.Code)
	}

	// Missing instanceName
	w3, c3 := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?groupId=g1@g.us", "")
	handler.FetchGroupParticipants(c3)

	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing instanceName, got %d", w3.Code)
	}
}

func TestWhatsAppHandler_FetchGroupParticipants_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?instanceName=inst-1&groupId=g1@g.us", "")
	handler.FetchGroupParticipants(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWhatsAppHandler_FetchGroupParticipants_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?instanceName=inst-1&groupId=g1@g.us", "")
	handler.FetchGroupParticipants(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_FetchGroupParticipants_Error(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.GetGroupParticipantsFunc = func(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error) {
		return nil, errors.New("participants error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/whatsapp/fetch-group-participants?instanceName=inst-1&groupId=g1@g.us", "")
	handler.FetchGroupParticipants(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SendMessage (QueueClient is nil — test only validation/error paths)
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SendMessage_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message", `{invalid}`)
	handler.SendMessage(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SendMessage_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message", `{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	body := mustParseBody(w)
	if body["error"] != "Instância não pertence ao usuário" {
		t.Errorf("expected ownership error message, got %v", body["error"])
	}
}

// ---------------------------------------------------------------------------
// SendAudio (QueueClient is nil — test only validation/error paths)
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SendAudio_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-audio", `{invalid}`)
	handler.SendAudio(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SendAudio_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-audio", `{"instanceName":"inst-1","groupId":"g1@g.us","audioBase64":"base64data","mimetype":"audio/ogg","ptt":true}`)
	handler.SendAudio(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// SendPoll
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SendPoll_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SendPollFunc = func(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
		return &whatsapp.MessageResult{ID: "poll-1"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-poll", `{"instanceName":"inst-1","groupId":"g1@g.us","question":"Test?","options":["A","B"],"selectableCount":1}`)
	handler.SendPoll(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "sent" {
		t.Errorf("expected status 'sent', got %v", data["status"])
	}
}

func TestWhatsAppHandler_SendPoll_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-poll", `{invalid}`)
	handler.SendPoll(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SendPoll_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-poll", `{"instanceName":"inst-1","groupId":"g1@g.us","question":"Test?","options":["A","B"],"selectableCount":1}`)
	handler.SendPoll(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SendPoll_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-poll", `{"instanceName":"inst-1","groupId":"g1@g.us","question":"Test?","options":["A","B"],"selectableCount":1}`)
	handler.SendPoll(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_SendPoll_SendError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SendPollFunc = func(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
		return nil, errors.New("poll failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-poll", `{"instanceName":"inst-1","groupId":"g1@g.us","question":"Test?","options":["A","B"],"selectableCount":1}`)
	handler.SendPoll(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateGroupSubject
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_UpdateGroupSubject_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupSubjectFunc = func(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
		if input.Subject != "New Subject" {
			t.Errorf("expected subject 'New Subject', got %q", input.Subject)
		}
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-subject", `{"instanceName":"inst-1","groupId":"g1@g.us","subject":"New Subject"}`)
	handler.UpdateGroupSubject(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["updated"] != true {
		t.Errorf("expected updated:true, got %v", data["updated"])
	}
}

func TestWhatsAppHandler_UpdateGroupSubject_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-subject", `{invalid}`)
	handler.UpdateGroupSubject(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSubject_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-subject", `{"instanceName":"inst-1","groupId":"g1@g.us","subject":"New Subject"}`)
	handler.UpdateGroupSubject(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSubject_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-subject", `{"instanceName":"inst-1","groupId":"g1@g.us","subject":"New Subject"}`)
	handler.UpdateGroupSubject(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSubject_UpdateError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupSubjectFunc = func(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
		return errors.New("update failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-subject", `{"instanceName":"inst-1","groupId":"g1@g.us","subject":"New Subject"}`)
	handler.UpdateGroupSubject(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateGroupDescription
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_UpdateGroupDescription_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupDescFunc = func(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
		if input.Description != "New Description" {
			t.Errorf("expected description 'New Description', got %q", input.Description)
		}
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-description", `{"instanceName":"inst-1","groupId":"g1@g.us","description":"New Description"}`)
	handler.UpdateGroupDescription(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["updated"] != true {
		t.Errorf("expected updated:true, got %v", data["updated"])
	}
}

func TestWhatsAppHandler_UpdateGroupDescription_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-description", `{invalid}`)
	handler.UpdateGroupDescription(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupDescription_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-description", `{"instanceName":"inst-1","groupId":"g1@g.us","description":"New Description"}`)
	handler.UpdateGroupDescription(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupDescription_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-description", `{"instanceName":"inst-1","groupId":"g1@g.us","description":"New Description"}`)
	handler.UpdateGroupDescription(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupDescription_UpdateError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupDescFunc = func(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
		return errors.New("update failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-description", `{"instanceName":"inst-1","groupId":"g1@g.us","description":"New Description"}`)
	handler.UpdateGroupDescription(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateGroupPicture
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_UpdateGroupPicture_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupPictureFunc = func(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error {
		if input.ImageBase64 != "base64img" {
			t.Errorf("expected imageBase64 'base64img', got %q", input.ImageBase64)
		}
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-picture", `{"instanceName":"inst-1","groupId":"g1@g.us","imageBase64":"base64img"}`)
	handler.UpdateGroupPicture(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["updated"] != true {
		t.Errorf("expected updated:true, got %v", data["updated"])
	}
}

func TestWhatsAppHandler_UpdateGroupPicture_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-picture", `{invalid}`)
	handler.UpdateGroupPicture(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupPicture_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-picture", `{"instanceName":"inst-1","groupId":"g1@g.us","imageBase64":"base64img"}`)
	handler.UpdateGroupPicture(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupPicture_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-picture", `{"instanceName":"inst-1","groupId":"g1@g.us","imageBase64":"base64img"}`)
	handler.UpdateGroupPicture(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupPicture_UpdateError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.UpdateGroupPictureFunc = func(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error {
		return errors.New("update failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-picture", `{"instanceName":"inst-1","groupId":"g1@g.us","imageBase64":"base64img"}`)
	handler.UpdateGroupPicture(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateGroupSettings
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_UpdateGroupSettings_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SetGroupAnnouncementFunc = func(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
		if !input.Announce {
			t.Error("expected announce=true for action 'announcement'")
		}
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-settings", `{"instanceName":"inst-1","groupId":"g1@g.us","action":"announcement"}`)
	handler.UpdateGroupSettings(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["updated"] != true {
		t.Errorf("expected updated:true, got %v", data["updated"])
	}
}

func TestWhatsAppHandler_UpdateGroupSettings_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-settings", `{invalid}`)
	handler.UpdateGroupSettings(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSettings_InstanceNotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-settings", `{"instanceName":"inst-1","groupId":"g1@g.us","action":"announcement"}`)
	handler.UpdateGroupSettings(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSettings_NilProvider(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-settings", `{"instanceName":"inst-1","groupId":"g1@g.us","action":"announcement"}`)
	handler.UpdateGroupSettings(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWhatsAppHandler_UpdateGroupSettings_UpdateError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.SetGroupAnnouncementFunc = func(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
		return errors.New("settings update failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/update-group-settings", `{"instanceName":"inst-1","groupId":"g1@g.us","action":"announcement"}`)
	handler.UpdateGroupSettings(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// mapWahaStatusToInternal (unit tests)
// ---------------------------------------------------------------------------

func TestMapWahaStatusToInternal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"WORKING", models.InstanceStatusConnected},
		{"open", models.InstanceStatusConnected},
		{"connected", models.InstanceStatusConnected},
		{"STOPPED", models.InstanceStatusDisconnected},
		{"close", models.InstanceStatusDisconnected},
		{"closed", models.InstanceStatusDisconnected},
		{"anything_else", models.InstanceStatusPending},
		{"", models.InstanceStatusPending},
	}

	for _, tt := range tests {
		t.Run("status_"+tt.input, func(t *testing.T) {
			t.Parallel()
			result := mapWahaStatusToInternal(tt.input)
			if result != tt.expected {
				t.Errorf("mapWahaStatusToInternal(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SendMessage — success paths (require real QueueClient via miniredis)
// ---------------------------------------------------------------------------

// newTestDepsWithQueue creates test deps with a real QueueClient backed by miniredis.
// The caller must defer mr.Close() and qc.Close() after the test.
func newTestDepsWithQueue(t *testing.T) (*Dependencies, *testMocks, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	qc := workers.NewQueueClient(mr.Addr(), testLogger())

	deps, mocks := newTestDeps()
	deps.QueueClient = qc

	t.Cleanup(func() {
		_ = qc.Close()
		mr.Close()
	})

	return deps, mocks, mr
}

func TestWhatsAppHandler_SendMessage_Success_Queued(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.DispatchLogRepo.CreateFunc = func(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
		log.ID = 42
		return log, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello World"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
	if data["dispatchLogId"] == nil {
		t.Error("expected dispatchLogId in response")
	}
	if int(data["dispatchLogId"].(float64)) != 42 {
		t.Errorf("expected dispatchLogId=42, got %v", data["dispatchLogId"])
	}
}

func TestWhatsAppHandler_SendMessage_Success_WithImageURL(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Check this","imageUrl":"https://example.com/img.png"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
}

func TestWhatsAppHandler_SendMessage_Success_WithImageBase64(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Image msg","imageBase64":"data:image/png;base64,iVBOR"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
}

func TestWhatsAppHandler_SendMessage_BackpressureRejects(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)

	// Override backpressure with one that rejects after 1 message (needs Redis for counting)
	bpMr := miniredis.RunT(t)
	bpRc := redis.NewClient(&redis.Options{Addr: bpMr.Addr()})
	t.Cleanup(func() { bpRc.Close() })
	bpCache := cache.NewCache(bpRc, "test", false)
	bp := backpressure.NewProducerBackpressure(backpressure.Config{
		Enabled:         true,
		UserMaxEnqueued: 1,
		WindowMinutes:   10,
	}, bpCache, testLogger())
	deps.Backpressure = bp

	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	// Pre-record one enqueue to hit the per-user limit (max=1)
	_ = bp.RecordEnqueue(context.Background(), "user-123")

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppHandler_SendMessage_BackpressureCheckError(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)

	// Override with mock that returns error from CanEnqueue
	deps.Backpressure = &mockBackpressureChecker{
		CanEnqueueFunc: func(ctx context.Context, userID string) (bool, string, error) {
			return false, "", errors.New("backpressure check failed")
		},
	}

	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello"}`)
	handler.SendMessage(c)

	// Should succeed — CanEnqueue error is non-fatal, handler logs and continues
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
}

func TestWhatsAppHandler_SendMessage_EnqueueFails(t *testing.T) {
	t.Parallel()
	deps, mocks, mr := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	// Close miniredis before calling to force enqueue failure
	mr.Close()

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello"}`)
	handler.SendMessage(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppHandler_SendMessage_DispatchLogCreateFails_StillEnqueues(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.DispatchLogRepo.CreateFunc = func(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
		return nil, errors.New("db error")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-message",
		`{"instanceName":"inst-1","groupId":"g1@g.us","message":"Hello"}`)
	handler.SendMessage(c)

	// Should still succeed because dispatch log creation failure is non-fatal
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
	// dispatchLogId should be nil when creation fails
	if data["dispatchLogId"] != nil {
		t.Errorf("expected dispatchLogId to be nil when creation fails, got %v", data["dispatchLogId"])
	}
}

// ---------------------------------------------------------------------------
// SendAudio — success paths (require real QueueClient via miniredis)
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SendAudio_Success_Queued(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-audio",
		`{"instanceName":"inst-1","groupId":"g1@g.us","audioBase64":"base64audiodata","mimetype":"audio/ogg","ptt":true}`)
	handler.SendAudio(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", data["status"])
	}
}

func TestWhatsAppHandler_SendAudio_EnqueueFails(t *testing.T) {
	t.Parallel()
	deps, mocks, mr := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	// Close miniredis to force enqueue failure
	mr.Close()

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/send-audio",
		`{"instanceName":"inst-1","groupId":"g1@g.us","audioBase64":"base64audiodata","mimetype":"audio/ogg","ptt":true}`)
	handler.SendAudio(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SyncGroups — success paths (require real QueueClient via miniredis)
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_SyncGroups_Success_Processing(t *testing.T) {
	t.Parallel()
	deps, mocks, _ := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/sync-groups",
		`{"instanceName":"inst-1"}`)
	handler.SyncGroups(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["status"] != "processing" {
		t.Errorf("expected status 'processing', got %v", data["status"])
	}
	if data["message"] != "Sincronização iniciada" {
		t.Errorf("expected message 'Sincronização iniciada', got %v", data["message"])
	}
}

func TestWhatsAppHandler_SyncGroups_EnqueueFails(t *testing.T) {
	t.Parallel()
	deps, mocks, mr := newTestDepsWithQueue(t)
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}

	// Close miniredis to force enqueue failure
	mr.Close()

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/sync-groups",
		`{"instanceName":"inst-1"}`)
	handler.SyncGroups(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DeleteInstance — edge cases for full coverage
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_DeleteInstance_DeleteSessionFails_ContinuesToDB(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.DeleteSessionFunc = func(ctx context.Context, name string) error {
		return errors.New("session delete failed")
	}
	deleteInstanceCalled := false
	mocks.InstanceRepo.DeleteInstanceFunc = func(ctx context.Context, instanceID int) error {
		deleteInstanceCalled = true
		return nil
	}
	deleteGroupsCalled := false
	mocks.GroupRepo.DeleteByInstanceFn = func(ctx context.Context, userID string, instanceID int) error {
		deleteGroupsCalled = true
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when DeleteSession fails, got %d: %s", w.Code, w.Body.String())
	}
	if !deleteInstanceCalled {
		t.Error("expected DeleteInstance to be called even when DeleteSession fails")
	}
	if !deleteGroupsCalled {
		t.Error("expected DeleteGroupsByInstance to be called even when DeleteSession fails")
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["deleted"] != true {
		t.Errorf("expected deleted:true, got %v", data["deleted"])
	}
}

func TestWhatsAppHandler_DeleteInstance_DeleteFromDBFails_ContinuesToGroups(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.DeleteSessionFunc = func(ctx context.Context, name string) error {
		return nil
	}
	mocks.InstanceRepo.DeleteInstanceFunc = func(ctx context.Context, instanceID int) error {
		return errors.New("db delete failed")
	}
	deleteGroupsCalled := false
	mocks.GroupRepo.DeleteByInstanceFn = func(ctx context.Context, userID string, instanceID int) error {
		deleteGroupsCalled = true
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when DB delete fails, got %d: %s", w.Code, w.Body.String())
	}
	if !deleteGroupsCalled {
		t.Error("expected DeleteGroupsByInstance to be called even when DB delete fails")
	}
}

func TestWhatsAppHandler_DeleteInstance_DeleteGroupsFails_StillReturnsSuccess(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	mocks.WhatsAppProvider.DeleteSessionFunc = func(ctx context.Context, name string) error {
		return nil
	}
	mocks.InstanceRepo.DeleteInstanceFunc = func(ctx context.Context, instanceID int) error {
		return nil
	}
	mocks.GroupRepo.DeleteByInstanceFn = func(ctx context.Context, userID string, instanceID int) error {
		return errors.New("groups delete failed")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when groups delete fails, got %d: %s", w.Code, w.Body.String())
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	if data["deleted"] != true {
		t.Errorf("expected deleted:true, got %v", data["deleted"])
	}
}

func TestWhatsAppHandler_DeleteInstance_NilProvider_ContinuesToDB(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	deps.WhatsAppProvider = nil
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
	}
	deleteInstanceCalled := false
	mocks.InstanceRepo.DeleteInstanceFunc = func(ctx context.Context, instanceID int) error {
		deleteInstanceCalled = true
		return nil
	}
	deleteGroupsCalled := false
	mocks.GroupRepo.DeleteByInstanceFn = func(ctx context.Context, userID string, instanceID int) error {
		deleteGroupsCalled = true
		return nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !deleteInstanceCalled {
		t.Error("expected DeleteInstance to be called even when provider is nil")
	}
	if !deleteGroupsCalled {
		t.Error("expected DeleteGroupsByInstance to be called even when provider is nil")
	}
}

func TestWhatsAppHandler_DeleteInstance_InstanceLookupError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.InstanceRepo.GetInstanceByUserAndNameFunc = func(ctx context.Context, userID, name string) (*models.Instance, error) {
		return nil, errors.New("db connection error")
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/delete-instance", `{"instanceName":"inst-1"}`)
	handler.DeleteInstance(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when instance lookup fails, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CreateInstance — UpsertInstance error (logs but continues, lines 70-74)
// ---------------------------------------------------------------------------

func TestWhatsAppHandler_CreateInstance_UpsertInstanceError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewWhatsAppHandler(deps)

	mocks.WhatsAppProvider.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{Name: input.SessionName, Status: whatsapp.SessionStatusScanQR}, nil
	}
	mocks.InstanceRepo.UpsertInstanceFunc = func(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
		return nil, errors.New("database write error")
	}
	mocks.WhatsAppProvider.GetQRCodeFunc = func(ctx context.Context, name string) (*whatsapp.QRCodeData, error) {
		return &whatsapp.QRCodeData{Base64Image: "qr-base64"}, nil
	}

	w, c := newAuthContext(http.MethodPost, "/api/v1/whatsapp/create-instance", `{"instanceName":"inst-1"}`)
	handler.CreateInstance(c)

	// Handler logs the error but does NOT fail — still returns 200
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite UpsertInstance error, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Helpers to suppress unused import warnings
// ---------------------------------------------------------------------------

var _ = httptest.NewRequest
