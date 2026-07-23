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

// ---------------------------------------------------------------------------
// RedirectPublicHandler.Direct — GET/HEAD /r/:slug
// ---------------------------------------------------------------------------

// newDirectRequest builds a test context for GET/HEAD /r/<slug>.
func newDirectRequest(method, slug, ua string, header http.Header) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/r/"+slug, nil)
	if ua != "" {
		c.Request.Header.Set("User-Agent", ua)
	}
	for k, vs := range header {
		for _, v := range vs {
			c.Request.Header.Add(k, v)
		}
	}
	c.Params = gin.Params{{Key: "slug", Value: slug}}
	return w, c
}

// assertNoStoreHeaders checks the three headers that must hold on EVERY branch.
func assertNoStoreHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if got := w.Header().Get("Cache-Control"); got != "no-store, private" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store, private")
	}
	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want %q", got, "no-cache")
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "no-referrer")
	}
}

func TestDirect_NormalBrowser_RedirectsStraightToWhatsApp(t *testing.T) {
	t.Parallel()
	_, mocks := newTestDeps()

	calls := 0
	mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		calls++
		if slug != "my-link" {
			t.Errorf("slug = %q, want %q", slug, "my-link")
		}
		if linkID != "" {
			t.Errorf("linkID = %q, want empty", linkID)
		}
		return &models.ProcessRedirectResult{
			Success:     true,
			RedirectURL: "https://chat.whatsapp.com/ABCDEFGH",
			GroupName:   "Group",
			GroupID:     "g1",
		}, nil
	}

	handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
	w, c := newDirectRequest(http.MethodGet, "my-link", uaChromeDesktop, nil)
	handler.Direct(c)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://chat.whatsapp.com/ABCDEFGH" {
		t.Errorf("Location = %q", got)
	}
	if calls != 1 {
		t.Errorf("service called %d times, want exactly 1", calls)
	}
	assertNoStoreHeaders(t, w)
}

func TestDirect_InAppBrowser_FallsBackWithoutCallingService(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		ua   string
	}{
		{"instagram", uaInstagram},
		{"tiktok", uaTikTok},
		{"android webview", uaAndroidWV},
		{"ios wkwebview", uaIOSWKWebVw},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			calls := 0
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				calls++
				return &models.ProcessRedirectResult{Success: true, RedirectURL: "https://chat.whatsapp.com/ABCDEFGH"}, nil
			}

			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w, c := newDirectRequest(http.MethodGet, "my-link", tt.ua, nil)
			handler.Direct(c)

			if w.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", w.Code)
			}
			if got := w.Header().Get("Location"); got != "/join/my-link" {
				t.Errorf("Location = %q, want %q", got, "/join/my-link")
			}
			if calls != 0 {
				t.Errorf("service called %d times, want 0", calls)
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestDirect_Preview_FallsBackInPreviewModeWithoutCallingService(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		method string
		ua     string
		header http.Header
	}{
		{name: "whatsapp unfurler", ua: uaWhatsApp},
		{name: "twitterbot", ua: uaTwitterbot},
		{name: "googlebot", ua: uaGoogleBot},
		{name: "curl", ua: uaCurl},
		{name: "empty ua"},
		{name: "HEAD", method: http.MethodHead, ua: uaChromeDesktop},
		{name: "prefetch", ua: uaChromeDesktop, header: http.Header{"Sec-Purpose": []string{"prefetch"}}},
		{name: "prerender", ua: uaChromeDesktop, header: http.Header{"Purpose": []string{"prerender"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			calls := 0
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				calls++
				return &models.ProcessRedirectResult{Success: true, RedirectURL: "https://chat.whatsapp.com/ABCDEFGH"}, nil
			}

			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w, c := newDirectRequest(method, "my-link", tt.ua, tt.header)
			handler.Direct(c)

			if w.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", w.Code)
			}
			if got := w.Header().Get("Location"); got != "/join/my-link?mode=preview" {
				t.Errorf("Location = %q, want %q", got, "/join/my-link?mode=preview")
			}
			if calls != 0 {
				t.Errorf("service called %d times, want 0", calls)
			}
			if method == http.MethodHead && w.Body.Len() != 0 {
				t.Errorf("HEAD body should be empty, got %d bytes", w.Body.Len())
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestDirect_BusinessError_MapsToPublicCodeWithoutSecondCall(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		serviceError string
		wantCode     string
	}{
		{"link not found", "not_found"},
		{"link is inactive", "inactive"},
		{"no target groups configured", "no_groups"},
		{"no available groups (all groups are full or inactive)", "groups_full"},
		{"group not found", "group_not_found"},
		{"instance not found", "instance_not_found"},
		{"bot is not admin in group", "not_admin"},
		{"service temporarily unavailable", "temporary"},
		{"internal error", "temporary"},
		{"failed to get invite code", "invite_unavailable"},
		{"invite code not available", "invite_unavailable"},
		{"WhatsApp provider not configured", "provider_unavailable"},
		{"something nobody mapped", "generic"},
		{"", "generic"},
	} {
		t.Run(tt.wantCode+"/"+tt.serviceError, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			calls := 0
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				calls++
				return &models.ProcessRedirectResult{Success: false, Error: tt.serviceError}, nil
			}

			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w, c := newDirectRequest(http.MethodGet, "my-link", uaChromeDesktop, nil)
			handler.Direct(c)

			if w.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", w.Code)
			}
			want := "/join/my-link?error=" + tt.wantCode
			if got := w.Header().Get("Location"); got != want {
				t.Errorf("Location = %q, want %q", got, want)
			}
			// The raw message must never travel in the query string.
			if tt.serviceError != "" && strings.Contains(w.Header().Get("Location"), tt.serviceError) {
				t.Errorf("Location leaked the raw service error: %q", w.Header().Get("Location"))
			}
			if calls != 1 {
				t.Errorf("service called %d times, want exactly 1", calls)
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestDirect_InvalidSlug_404WithoutEchoOrServiceCall(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{
		"",
		"UPPERCASE",
		"has_underscore",
		"has.dot",
		"has space",
		"has/slash",
		"../etc/passwd",
		"a%0d%0aSet-Cookie:+x=y",
		"a\r\nSet-Cookie: x=y",
		strings.Repeat("a", 101),
	} {
		t.Run("slug="+slug, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			calls := 0
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				calls++
				return &models.ProcessRedirectResult{Success: true, RedirectURL: "https://chat.whatsapp.com/ABCDEFGH"}, nil
			}

			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/r/x", nil)
			c.Request.Header.Set("User-Agent", uaChromeDesktop)
			c.Params = gin.Params{{Key: "slug", Value: slug}}

			handler.Direct(c)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", w.Code)
			}
			if got := w.Header().Get("Location"); got != "" {
				t.Errorf("expected no Location on 404, got %q", got)
			}
			if slug != "" && strings.Contains(w.Body.String(), slug) {
				t.Errorf("response body echoed the slug: %q", w.Body.String())
			}
			if calls != 0 {
				t.Errorf("service called %d times, want 0", calls)
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestDirect_NilService_503WithoutLocation(t *testing.T) {
	t.Parallel()

	handler := NewRedirectPublicHandler(nil, testLogger())
	w, c := newDirectRequest(http.MethodGet, "my-link", uaChromeDesktop, nil)
	handler.Direct(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if got := w.Header().Get("Location"); got != "" {
		t.Errorf("expected no Location, got %q", got)
	}
	assertNoStoreHeaders(t, w)
}

func TestDirect_ServiceFailures_500WithoutLocation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		result *models.ProcessRedirectResult
		err    error
	}{
		{name: "service returns an error", err: errors.New("database connection lost")},
		{name: "service returns (nil, nil)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				return tt.result, tt.err
			}

			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w, c := newDirectRequest(http.MethodGet, "my-link", uaChromeDesktop, nil)
			handler.Direct(c)

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", w.Code)
			}
			if got := w.Header().Get("Location"); got != "" {
				t.Errorf("expected no Location, got %q", got)
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestDirect_InvalidDestination_500WithoutLocation(t *testing.T) {
	t.Parallel()

	for _, badURL := range []string{
		"",
		"http://chat.whatsapp.com/ABCDEFGH",
		"https://chat.whatsapp.com.evil.test/ABCDEFGH",
		"https://evil.test/ABCDEFGH",
		"https://user:pass@chat.whatsapp.com/ABCDEFGH",
		"https://chat.whatsapp.com:443/ABCDEFGH",
		"https://chat.whatsapp.com/ABCDEFGH?x=1",
		"https://chat.whatsapp.com/ABCDEFGH#frag",
		"https://chat.whatsapp.com/ABCDEFGH/extra",
		"https://chat.whatsapp.com/AB",
		"https://chat.whatsapp.com/",
		"https://chat.whatsapp.com/%41BCDEFGH",
		"https://chat.whatsapp.com./ABCDEFGH",
	} {
		t.Run("url="+badURL, func(t *testing.T) {
			t.Parallel()
			_, mocks := newTestDeps()
			mocks.RedirectService.ProcessRedirectFunc = func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
				return &models.ProcessRedirectResult{Success: true, RedirectURL: badURL, GroupID: "g1"}, nil
			}

			handler := NewRedirectPublicHandler(mocks.RedirectService, testLogger())
			w, c := newDirectRequest(http.MethodGet, "my-link", uaChromeDesktop, nil)
			handler.Direct(c)

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d for %q, want 500", w.Code, badURL)
			}
			if got := w.Header().Get("Location"); got != "" {
				t.Errorf("a rejected destination must never reach Location, got %q", got)
			}
			assertNoStoreHeaders(t, w)
		})
	}
}

func TestIsValidWhatsAppInviteURL(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://chat.whatsapp.com/ABCDE",
		"https://chat.whatsapp.com/ABCDEFGHIJKLMNOPQRST",
		"https://chat.whatsapp.com/Abc_1-Xyz9",
		"https://chat.whatsapp.com/" + strings.Repeat("a", 128),
	}
	for _, u := range valid {
		if !isValidWhatsAppInviteURL(u) {
			t.Errorf("expected %q to be valid", u)
		}
	}

	invalid := []string{
		"",
		"not a url",
		"http://chat.whatsapp.com/ABCDE",
		"//chat.whatsapp.com/ABCDE",
		"https://chat.whatsapp.com",
		"https://chat.whatsapp.com/",
		"https://chat.whatsapp.com/ABC",
		"https://chat.whatsapp.com/" + strings.Repeat("a", 129),
		"https://chat.whatsapp.com:8443/ABCDE",
		"https://evil.test/ABCDE",
		"https://chat.whatsapp.com.evil.test/ABCDE",
		"https://notchat.whatsapp.com/ABCDE",
		"https://user@chat.whatsapp.com/ABCDE",
		"https://chat.whatsapp.com/ABCDE?x=1",
		"https://chat.whatsapp.com/ABCDE?",
		"https://chat.whatsapp.com/ABCDE#f",
		"https://chat.whatsapp.com/ABC/DEF",
		"https://chat.whatsapp.com/%41BCDE",
		"https://chat.whatsapp.com/ABC DE",
		"https://chat.whatsapp.com/ABCDE\r\nX: y",
	}
	for _, u := range invalid {
		if isValidWhatsAppInviteURL(u) {
			t.Errorf("expected %q to be REJECTED", u)
		}
	}
}
