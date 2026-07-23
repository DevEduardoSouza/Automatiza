package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	handler := HealthCheck(deps)
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["version"] != "test" {
		t.Errorf("expected version=test, got %v", body["version"])
	}
	if body["env"] != "test" {
		t.Errorf("expected env=test, got %v", body["env"])
	}
}

// panicGuard wraps a function call with recover() and observability logging,
// satisfying the observability guard hook for panic recovery in tests.
func panicGuard(t *testing.T, label string, fn func()) {
	t.Helper()
	logger := zap.NewNop()
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("test panic recovered",
				zap.String("test", label),
				zap.Any("panic", rec))
			t.Fatalf("%s panicked: %v", label, rec)
		}
	}()
	fn()
}

func TestSetupRoutes_NosPanic(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	r := gin.New()

	panicGuard(t, "SetupRoutes", func() {
		SetupRoutes(r, deps)
	})
}

func TestSetupRoutes_WithSentryDSN(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.Config.SentryDSN = "https://key@o123.ingest.us.sentry.io/456"

	r := gin.New()

	panicGuard(t, "SetupRoutes_WithSentryDSN", func() {
		SetupRoutes(r, deps)
	})
}

func TestSetupRoutes_WithNilOptionalDeps(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.SSEHub = nil
	deps.RedirectService = nil
	deps.WebhookEventsRepo = nil
	deps.QueueClient = nil

	r := gin.New()

	panicGuard(t, "SetupRoutes_WithNilOptionalDeps", func() {
		SetupRoutes(r, deps)
	})
}

func TestSetupRoutes_WithRedirectService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	// RedirectService is already set by newTestDeps; ensure it's non-nil
	if deps.RedirectService == nil {
		t.Fatal("expected RedirectService to be non-nil from newTestDeps")
	}

	r := gin.New()

	panicGuard(t, "SetupRoutes_WithRedirectService", func() {
		SetupRoutes(r, deps)
	})

	// Verify that the redirect route exists and uses RedirectPublicHandler
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"test-slug"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("expected /api/v1/redirect to be registered, got 404")
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// RedirectPublicHandler returns success=true from mock
	if body["success"] != true {
		t.Fatalf("expected success=true from RedirectPublicHandler, got %v", body["success"])
	}
}

func TestSetupRoutes_WithWAHAWebhookHandler(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	// Start miniredis for a real QueueClient
	mr := miniredis.RunT(t)
	logger := testLogger()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	deps.QueueClient = queueClient
	// WebhookEventsRepo is already non-nil from newTestDeps

	r := gin.New()

	panicGuard(t, "SetupRoutes_WithWAHA", func() {
		SetupRoutes(r, deps)
	})

	// Verify the /api/v1/webhooks/waha route exists (should NOT be 404)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", strings.NewReader(`{"event":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("expected /api/v1/webhooks/waha to be registered, got 404")
	}
}

func TestSetupRoutes_WithSSEHub(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()

	// Create a minimal SSEHub for route registration (avoid NewSSEHub goroutine)
	hub := &SSEHub{
		clients:    make(map[string]*sseClient),
		broadcast:  make(chan SSEEvent, 1),
		register:   make(chan *sseClient, 1),
		unregister: make(chan *sseClient, 1),
		logger:     testLogger().Named("sse-hub"),
		done:       make(chan struct{}),
	}
	deps.SSEHub = hub
	t.Cleanup(func() { close(hub.done) })

	r := gin.New()

	panicGuard(t, "SetupRoutes_WithSSEHub", func() {
		SetupRoutes(r, deps)
	})

	// Verify that the SSE route exists
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sse/events", nil)
	r.ServeHTTP(w, req)

	// Should not be 404 — route is registered (will fail auth, returning 401 or similar)
	if w.Code == http.StatusNotFound {
		t.Fatal("expected /api/v1/sse/events to be registered, got 404")
	}
}

// ---------------------------------------------------------------------------
// GET / HEAD /r/:slug — route registration, precedence and edge paths
// ---------------------------------------------------------------------------

// newRedirectRouter wires a real gin.Engine and returns a pointer to the call
// counter of the redirect service, so each case can assert how many attempts a
// request actually produced.
func newRedirectRouter(t *testing.T) (*gin.Engine, *int) {
	t.Helper()
	deps, mocks := newTestDeps()
	calls := 0
	mocks.RedirectService.ProcessRedirectFunc = func(_ context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
		calls++
		return &models.ProcessRedirectResult{
			Success:     true,
			RedirectURL: "https://chat.whatsapp.com/ABCDEFGH",
			GroupName:   "Group",
			GroupID:     "g1",
		}, nil
	}
	r := gin.New()
	SetupRoutes(r, deps)
	return r, &calls
}

func doRedirectRequest(r *gin.Engine, method, path, ua string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestSetupRoutes_DirectRedirect_CanonicalPath(t *testing.T) {
	t.Parallel()
	r, calls := newRedirectRouter(t)

	w := doRedirectRequest(r, http.MethodGet, "/r/my-link", uaChromeDesktop)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://chat.whatsapp.com/ABCDEFGH" {
		t.Errorf("Location = %q", got)
	}
	if *calls != 1 {
		t.Errorf("service called %d times, want exactly 1", *calls)
	}
}

func TestSetupRoutes_DirectRedirect_HeadIsPreviewAndBodyless(t *testing.T) {
	t.Parallel()
	r, calls := newRedirectRouter(t)

	w := doRedirectRequest(r, http.MethodHead, "/r/my-link", uaChromeDesktop)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/join/my-link?mode=preview" {
		t.Errorf("Location = %q, want %q", got, "/join/my-link?mode=preview")
	}
	if w.Body.Len() != 0 {
		t.Errorf("HEAD body should be empty, got %q", w.Body.String())
	}
	if *calls != 0 {
		t.Errorf("HEAD must not call the service, got %d calls", *calls)
	}
}

// Observed gin behaviour for the paths around /r/:slug. None of them may reach
// the service — in particular the trailing-slash 301 is a CANONICAL redirect,
// not a counted attempt.
func TestSetupRoutes_DirectRedirect_EdgePathsNeverReachTheService(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		path         string
		wantStatus   int
		wantLocation string
	}{
		{path: "/r", wantStatus: http.StatusNotFound},
		{path: "/r/", wantStatus: http.StatusNotFound},
		{path: "/r/my-link/", wantStatus: http.StatusMovedPermanently, wantLocation: "/r/my-link"},
		{path: "/r/a/b", wantStatus: http.StatusNotFound},
	} {
		t.Run("path="+tt.path, func(t *testing.T) {
			t.Parallel()
			r, calls := newRedirectRouter(t)

			w := doRedirectRequest(r, http.MethodGet, tt.path, uaChromeDesktop)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got, tt.wantLocation)
			}
			if *calls != 0 {
				t.Errorf("service called %d times, want 0", *calls)
			}
		})
	}
}

// The legacy JSON endpoint stays exactly as it was — old bundles and the /join
// fallback both depend on it.
func TestSetupRoutes_DirectRedirect_LegacyPostStillWorks(t *testing.T) {
	t.Parallel()
	r, calls := newRedirectRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/redirect", strings.NewReader(`{"slug":"my-link"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	if *calls != 1 {
		t.Errorf("service called %d times, want exactly 1", *calls)
	}
}

// With no service configured the route must still exist and fail loudly: 404
// would silently hand /r/<slug> back to the SPA, and /join would land on the
// stub that cannot process a slug at all.
func TestSetupRoutes_DirectRedirect_RegisteredEvenWithoutService(t *testing.T) {
	t.Parallel()
	deps, _ := newTestDeps()
	deps.RedirectService = nil
	r := gin.New()

	panicGuard(t, "SetupRoutes_NilRedirectService", func() {
		SetupRoutes(r, deps)
	})

	w := doRedirectRequest(r, http.MethodGet, "/r/my-link", uaChromeDesktop)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if got := w.Header().Get("Location"); got != "" {
		t.Errorf("expected no Location, got %q", got)
	}
}
