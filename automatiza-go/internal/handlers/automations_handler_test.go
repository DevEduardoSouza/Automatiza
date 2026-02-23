package handlers

import (
	"automatiza-go/internal/models"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// AutomationsHandler.List
// ---------------------------------------------------------------------------

func TestAutomationsHandler_List_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetUserAutomationsFunc = func(ctx context.Context, userID string) ([]models.Automation, error) {
		return []models.Automation{
			{ID: "a1", Name: "Auto 1", UserID: userID},
			{ID: "a2", Name: "Auto 2", UserID: userID},
		}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/automations/", "")
	handler.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := mustParseBody(w)
	if body == nil {
		t.Fatal("failed to parse response body")
	}

	data := body["data"].(map[string]interface{})
	automations := data["automations"].([]interface{})
	if len(automations) != 2 {
		t.Errorf("expected 2 automations, got %d", len(automations))
	}

	total := data["total"].(float64)
	if int(total) != 2 {
		t.Errorf("expected total=2, got %v", total)
	}
}

func TestAutomationsHandler_List_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetUserAutomationsFunc = func(ctx context.Context, userID string) ([]models.Automation, error) {
		return nil, errors.New("db error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/automations/", "")
	handler.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Get
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Get_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, Name: "Test Auto", UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := mustParseBody(w)
	if body == nil {
		t.Fatal("failed to parse response body")
	}

	if body["success"] != true {
		t.Error("expected success=true")
	}
}

func TestAutomationsHandler_Get_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAutomationsHandler_Get_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, errors.New("db error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Get(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Create
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Create_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.CreateAutomationFunc = func(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
		data.ID = "new-auto-1"
		return data, nil
	}

	body := `{"name":"Test Auto","group_ids":["g1"],"send_groups":["g2"]}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/", body)
	handler.Create(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	if resp["success"] != true {
		t.Error("expected success=true")
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] != "new-auto-1" {
		t.Errorf("expected id=new-auto-1, got %v", data["id"])
	}
	if data["name"] != "Test Auto" {
		t.Errorf("expected name=Test Auto, got %v", data["name"])
	}
}

func TestAutomationsHandler_Create_Defaults(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	var capturedAutomation *models.Automation
	mocks.AutomationRepo.CreateAutomationFunc = func(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
		capturedAutomation = data
		data.ID = "new-auto-2"
		return data, nil
	}

	body := `{"name":"Test"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/", body)
	handler.Create(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	if capturedAutomation == nil {
		t.Fatal("expected automation to be captured")
	}

	if capturedAutomation.Status != models.AutomationStatusInactive {
		t.Errorf("expected default status=%q, got %q", models.AutomationStatusInactive, capturedAutomation.Status)
	}

	if capturedAutomation.Type == nil || *capturedAutomation.Type != models.AutomationTypeSearch {
		t.Errorf("expected default type=%q, got %v", models.AutomationTypeSearch, capturedAutomation.Type)
	}
}

func TestAutomationsHandler_Create_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewAutomationsHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/", "invalid-json")
	handler.Create(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Update
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Update_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	// First call: verify ownership; second call: fetch updated result
	callCount := 0
	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		callCount++
		return &models.Automation{ID: id, Name: "Updated Name", UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	mocks.AutomationRepo.UpdateAutomationFunc = func(ctx context.Context, id string, updates map[string]interface{}) error {
		if _, ok := updates["name"]; !ok {
			t.Error("expected 'name' in updates map")
		}
		return nil
	}

	body := `{"name":"Updated Name"}`
	w, c := newAuthContext(http.MethodPut, "/api/v1/automations/auto-1", body)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Update(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	if resp["success"] != true {
		t.Error("expected success=true")
	}
}

func TestAutomationsHandler_Update_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	body := `{"name":"Updated Name"}`
	w, c := newAuthContext(http.MethodPut, "/api/v1/automations/auto-1", body)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Update(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAutomationsHandler_Update_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	// Ownership check passes
	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	w, c := newAuthContext(http.MethodPut, "/api/v1/automations/auto-1", "invalid-json")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAutomationsHandler_Update_EmptyUpdates(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	// Ownership check passes
	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	// Empty JSON object: ToMap returns empty map
	w, c := newAuthContext(http.MethodPut, "/api/v1/automations/auto-1", "{}")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Update(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAutomationsHandler_Update_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	// Ownership check passes
	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	mocks.AutomationRepo.UpdateAutomationFunc = func(ctx context.Context, id string, updates map[string]interface{}) error {
		return errors.New("db error")
	}

	body := `{"name":"Updated Name"}`
	w, c := newAuthContext(http.MethodPut, "/api/v1/automations/auto-1", body)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Update(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Delete
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Delete_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	mocks.AutomationRepo.DeleteAutomationFunc = func(ctx context.Context, id, userID string) error {
		return nil
	}

	w, c := newAuthContext(http.MethodDelete, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Delete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := mustParseBody(w)
	if body == nil {
		t.Fatal("failed to parse response body")
	}

	data := body["data"].(map[string]interface{})
	if data["deleted"] != true {
		t.Error("expected deleted=true")
	}
}

func TestAutomationsHandler_Delete_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodDelete, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Delete(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAutomationsHandler_Delete_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	// Ownership check passes
	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	mocks.AutomationRepo.DeleteAutomationFunc = func(ctx context.Context, id, userID string) error {
		return errors.New("db error")
	}

	w, c := newAuthContext(http.MethodDelete, "/api/v1/automations/auto-1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "auto-1"})
	handler.Delete(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Diagnose
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Diagnose_ActiveAutomation(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{
			ID:         id,
			UserID:     userID,
			Status:     models.AutomationStatusActive,
			GroupIDs:   []string{"g1"},
			SendGroups: []string{"g2"},
		}, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/diagnose", body)
	handler.Diagnose(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	data := resp["data"].(map[string]interface{})
	if data["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", data["status"])
	}

	checks := data["checks"].([]interface{})
	// Active automation with groups: instance check + groups check (no status warning)
	if len(checks) != 2 {
		t.Errorf("expected 2 checks for active automation with groups, got %d", len(checks))
	}
}

func TestAutomationsHandler_Diagnose_InactiveAutomation(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{
			ID:         id,
			UserID:     userID,
			Status:     models.AutomationStatusInactive,
			GroupIDs:   []string{"g1"},
			SendGroups: []string{"g2"},
		}, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/diagnose", body)
	handler.Diagnose(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	data := resp["data"].(map[string]interface{})
	checks := data["checks"].([]interface{})

	// Inactive automation: status warning check + instance check + groups check
	foundStatusWarning := false
	for _, chk := range checks {
		check := chk.(map[string]interface{})
		if check["name"] == "status" && check["status"] == "warning" {
			foundStatusWarning = true
			break
		}
	}

	if !foundStatusWarning {
		t.Error("expected a 'status' check with 'warning' status for inactive automation")
	}
}

func TestAutomationsHandler_Diagnose_NoGroups(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{
			ID:         id,
			UserID:     userID,
			Status:     models.AutomationStatusActive,
			GroupIDs:   nil,
			SendGroups: nil,
		}, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/diagnose", body)
	handler.Diagnose(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	data := resp["data"].(map[string]interface{})
	if data["status"] != "warning" {
		t.Errorf("expected status=warning for no groups, got %v", data["status"])
	}

	warnings := data["warnings"].([]interface{})
	if len(warnings) == 0 {
		t.Error("expected at least one warning about no groups configured")
	}
}

func TestAutomationsHandler_Diagnose_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewAutomationsHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/diagnose", "invalid-json")
	handler.Diagnose(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAutomationsHandler_Diagnose_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/diagnose", body)
	handler.Diagnose(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Run
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Run_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/run", body)
	handler.Run(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	data := resp["data"].(map[string]interface{})
	if data["success"] != true {
		t.Error("expected success=true in response data")
	}
}

func TestAutomationsHandler_Run_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewAutomationsHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/run", "invalid-json")
	handler.Run(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAutomationsHandler_Run_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/run", body)
	handler.Run(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.ResetHistory
// ---------------------------------------------------------------------------

func TestAutomationsHandler_ResetHistory_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return &models.Automation{ID: id, UserID: userID, Status: models.AutomationStatusActive}, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/reset-history", body)
	handler.ResetHistory(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := mustParseBody(w)
	if resp == nil {
		t.Fatal("failed to parse response body")
	}

	data := resp["data"].(map[string]interface{})
	if data["reset"] != true {
		t.Error("expected reset=true in response data")
	}
}

func TestAutomationsHandler_ResetHistory_InvalidJSON(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	handler := NewAutomationsHandler(deps)

	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/reset-history", "invalid-json")
	handler.ResetHistory(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAutomationsHandler_ResetHistory_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.GetByIDAndUserFunc = func(ctx context.Context, id, userID string) (*models.Automation, error) {
		return nil, nil
	}

	body := `{"automationId":"auto-1"}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/reset-history", body)
	handler.ResetHistory(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// AutomationsHandler.Create — Repo error (lines 103-106)
// ---------------------------------------------------------------------------

func TestAutomationsHandler_Create_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewAutomationsHandler(deps)

	mocks.AutomationRepo.CreateAutomationFunc = func(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
		return nil, errors.New("database error")
	}

	body := `{"name":"Test Auto","group_ids":["g1"]}`
	w, c := newAuthContext(http.MethodPost, "/api/v1/automations/", body)
	handler.Create(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
