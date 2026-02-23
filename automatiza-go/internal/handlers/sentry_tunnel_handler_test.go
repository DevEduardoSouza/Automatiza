package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// roundTripFunc is an adapter to allow the use of ordinary functions as http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// ginStatus returns the status code from gin's writer (handles cases where
// c.Status() is called without a body, which doesn't flush to httptest.ResponseRecorder).
func ginStatus(c *gin.Context) int {
	return c.Writer.Status()
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Valid envelope forwarding
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_ValidEnvelope(t *testing.T) {
	t.Parallel()
	const allowedHost = "o123.ingest.us.sentry.io"
	body := `{"dsn":"https://key@` + allowedHost + `/456"}` + "\n" + `{"type":"event"}`

	handler := NewSentryTunnelHandler(allowedHost, testLogger())

	var capturedReq *http.Request
	var capturedBody string
	handler.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedReq = r
			b, _ := io.ReadAll(r.Body)
			capturedBody = string(b)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusOK {
		t.Fatalf("expected 200, got %d", ginStatus(c))
	}
	if capturedReq == nil {
		t.Fatal("upstream request was never sent")
	}
	expectedURL := "https://" + allowedHost + "/api/456/envelope/"
	if capturedReq.URL.String() != expectedURL {
		t.Errorf("expected upstream URL %q, got %q", expectedURL, capturedReq.URL.String())
	}
	if capturedReq.Header.Get("Content-Type") != "application/x-sentry-envelope" {
		t.Errorf("expected Content-Type header application/x-sentry-envelope, got %q", capturedReq.Header.Get("Content-Type"))
	}
	if capturedBody != body {
		t.Errorf("expected body %q, got %q", body, capturedBody)
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Missing newline
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_MissingNewline(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("sentry.io", testLogger())
	body := `{"dsn":"https://key@sentry.io/123","type":"event"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Invalid JSON header
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_InvalidJSONHeader(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("sentry.io", testLogger())
	body := "not-valid-json\n{\"type\":\"event\"}"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Invalid DSN
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_InvalidDSN(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("sentry.io", testLogger())
	body := `{"dsn":"://not-a-url"}` + "\n" + `{"type":"event"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Wrong host
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_WrongHost(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("allowed.sentry.io", testLogger())
	body := `{"dsn":"https://key@evil.sentry.io/123"}` + "\n" + `{"type":"event"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Upstream error
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_UpstreamError(t *testing.T) {
	t.Parallel()
	const allowedHost = "o123.ingest.us.sentry.io"
	body := `{"dsn":"https://key@` + allowedHost + `/789"}` + "\n" + `{"type":"event"}`

	handler := NewSentryTunnelHandler(allowedHost, testLogger())
	handler.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — Upstream network error
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_UpstreamNetworkError(t *testing.T) {
	t.Parallel()
	const allowedHost = "o123.ingest.us.sentry.io"
	body := `{"dsn":"https://key@` + allowedHost + `/789"}` + "\n" + `{"type":"event"}`

	handler := NewSentryTunnelHandler(allowedHost, testLogger())
	handler.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle -- Body read error (non-MaxBytes)
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_BodyReadError(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("sentry.io", testLogger())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Use an errorReader that returns a generic read error
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", &sentryErrorReader{})
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusBadRequest {
		t.Fatalf("expected 400 for body read error, got %d", ginStatus(c))
	}
}

// sentryErrorReader is an io.Reader that always returns an error.
type sentryErrorReader struct{}

func (r *sentryErrorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle — NewRequest error
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_NewRequestError(t *testing.T) {
	t.Parallel()
	const allowedHost = "o123.ingest.us.sentry.io"
	body := `{"dsn":"https://key@` + allowedHost + `/456"}` + "\n" + `{"type":"event"}`

	handler := NewSentryTunnelHandler(allowedHost, testLogger())
	handler.newRequest = func(_ context.Context, _, _ string, _ io.Reader) (*http.Request, error) {
		return nil, errors.New("request creation failed")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", ginStatus(c))
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle -- DSN missing project ID (path is just "/")
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_DSNMissingProjectID(t *testing.T) {
	t.Parallel()
	const allowedHost = "o123.ingest.us.sentry.io"
	// DSN with path "/" which means projectID will be empty after TrimPrefix
	body := `{"dsn":"https://key@` + allowedHost + `/"}` + "\n" + `{"type":"event"}`

	handler := NewSentryTunnelHandler(allowedHost, testLogger())

	var capturedURL string
	handler.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedURL = r.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusOK {
		t.Fatalf("expected 200, got %d", ginStatus(c))
	}

	// The upstream URL should have an empty project ID segment
	expectedURL := "https://" + allowedHost + "/api//envelope/"
	if capturedURL != expectedURL {
		t.Errorf("expected upstream URL %q, got %q", expectedURL, capturedURL)
	}
}

// ---------------------------------------------------------------------------
// SentryTunnelHandler.Handle -- Empty DSN host
// ---------------------------------------------------------------------------

func TestSentryTunnelHandler_Handle_EmptyDSNHost(t *testing.T) {
	t.Parallel()
	handler := NewSentryTunnelHandler("sentry.io", testLogger())
	// DSN with no host results in empty host after parsing
	body := `{"dsn":"https:///456"}` + "\n" + `{"type":"event"}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sentry-tunnel", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/x-sentry-envelope")

	handler.Handle(c)

	if ginStatus(c) != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty DSN host, got %d", ginStatus(c))
	}
}
