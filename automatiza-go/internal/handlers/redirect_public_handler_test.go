package handlers

import (
	"automatiza-go/internal/models"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Success with slug
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_SuccessWithSlug(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()

	mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		if slug != "my-link" {
			t.Errorf("expected slug %q, got %q", "my-link", slug)
		}
		return &models.ProcessRedirectResult{
			Success:     true,
			RedirectURL: "https://chat.whatsapp.com/ABC",
			GroupName:   "Test Group",
			GroupID:     "g1",
		}, nil
	}

	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"my-link"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp models.PublicRedirectResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.RedirectURL != "https://chat.whatsapp.com/ABC" {
		t.Errorf("expected redirect_url %q, got %q", "https://chat.whatsapp.com/ABC", resp.RedirectURL)
	}
	if resp.GroupName != "Test Group" {
		t.Errorf("expected group_name %q, got %q", "Test Group", resp.GroupName)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Success with linkId
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_SuccessWithLinkID(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()

	mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		if linkID != "link-123" {
			t.Errorf("expected linkID %q, got %q", "link-123", linkID)
		}
		return &models.ProcessRedirectResult{
			Success:     true,
			RedirectURL: "https://chat.whatsapp.com/XYZ",
			GroupName:   "Group 2",
			GroupID:     "g2",
		}, nil
	}

	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"linkId":"link-123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp models.PublicRedirectResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.RedirectURL != "https://chat.whatsapp.com/XYZ" {
		t.Errorf("expected redirect_url %q, got %q", "https://chat.whatsapp.com/XYZ", resp.RedirectURL)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Invalid JSON
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()
	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader("{invalid"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Missing slug and linkId
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_MissingSlugAndLinkID(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()
	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	body := mustParseBody(w)
	if body == nil {
		t.Fatal("failed to parse response body")
	}
	errMsg, ok := body["error"].(string)
	if !ok || errMsg != "slug or linkId is required" {
		t.Errorf("expected error %q, got %q", "slug or linkId is required", errMsg)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Nil service
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_NilService(t *testing.T) {
	t.Parallel()
	handler := NewRedirectPublicHandler(nil, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"test"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Service error
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_ServiceError(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()

	mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		return nil, errors.New("database connection lost")
	}

	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"my-link"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Process — Result with success=false
// ---------------------------------------------------------------------------

func TestRedirectPublicHandler_Process_ResultSuccessFalse(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()

	mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "link expired",
		}, nil
	}

	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"expired-link"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Process(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp models.PublicRedirectResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "link expired" {
		t.Errorf("expected error %q, got %q", "link expired", resp.Error)
	}
}
