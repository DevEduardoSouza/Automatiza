package mercadolivre

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractCSRFFromHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "empty HTML returns empty",
			html: "",
			want: "",
		},
		{
			name: "meta tag with name csrf-token",
			html: `<html><head><meta name="csrf-token" content="abc123"></head></html>`,
			want: "abc123",
		},
		{
			name: "meta tag with property csrf-token",
			html: `<html><head><meta property="csrf-token" content="abc123"></head></html>`,
			want: "abc123",
		},
		{
			name: "meta tag with name x-csrf-token",
			html: `<html><head><meta name="x-csrf-token" content="def456"></head></html>`,
			want: "def456",
		},
		{
			name: "content before name attribute order",
			html: `<html><head><meta content="abc" name="csrf-token"></head></html>`,
			want: "",
			// The regex expects name before content, so this won't match meta patterns.
			// It will fall through to JSON patterns which also won't match.
		},
		{
			name: "JSON pattern x-csrf-token",
			html: `<script>var config = {"x-csrf-token": "token123"};</script>`,
			want: "token123",
		},
		{
			name: "JSON pattern csrfToken",
			html: `<script>var config = {"csrfToken": "token456"};</script>`,
			want: "token456",
		},
		{
			name: "JSON pattern csrf_token",
			html: `<script>var config = {"csrf_token": "token789"};</script>`,
			want: "token789",
		},
		{
			name: "no CSRF found returns empty",
			html: `<html><head><title>No CSRF</title></head><body>Hello</body></html>`,
			want: "",
		},
		{
			name: "whitespace trimming in token value",
			html: `<html><head><meta name="csrf-token" content="  trimmed  "></head></html>`,
			want: "trimmed",
		},
		{
			name: "empty token value in meta skipped falls to JSON",
			html: `<html><head><meta name="csrf-token" content=""></head>
			<script>{"csrfToken": "fallback"}</script></html>`,
			want: "fallback",
		},
		{
			name: "empty token value everywhere returns empty",
			html: `<html><head><meta name="csrf-token" content=""></head></html>`,
			want: "",
		},
		{
			name: "meta tag with single quotes",
			html: `<html><head><meta name='csrf-token' content='single123'></head></html>`,
			want: "single123",
		},
		{
			name: "meta tag prefers first match",
			html: `<html><head>
				<meta name="csrf-token" content="first">
				<meta name="x-csrf-token" content="second">
			</head></html>`,
			want: "first",
		},
		{
			name: "JSON pattern with spaces",
			html: `{"x-csrf-token" : "spaced123"}`,
			want: "spaced123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCSRFFromHTML(tt.html)
			if got != tt.want {
				t.Errorf("ExtractCSRFFromHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCSRFError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{name: "401 is CSRF error", statusCode: http.StatusUnauthorized, want: true},
		{name: "403 is CSRF error", statusCode: http.StatusForbidden, want: true},
		{name: "200 is not CSRF error", statusCode: http.StatusOK, want: false},
		{name: "500 is not CSRF error", statusCode: http.StatusInternalServerError, want: false},
		{name: "404 is not CSRF error", statusCode: http.StatusNotFound, want: false},
		{name: "400 is CSRF error", statusCode: http.StatusBadRequest, want: true},
		{name: "302 is not CSRF error", statusCode: http.StatusFound, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCSRFError(tt.statusCode)
			if got != tt.want {
				t.Errorf("IsCSRFError(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestNewCSRFRefresher(t *testing.T) {
	client := &http.Client{}
	refresher := NewCSRFRefresher(client)
	if refresher == nil {
		t.Fatal("NewCSRFRefresher returned nil")
	}
	if refresher.httpClient != client {
		t.Error("httpClient not set correctly")
	}
}

func TestNewCSRFRefresher_NilClient(t *testing.T) {
	refresher := NewCSRFRefresher(nil)
	if refresher == nil {
		t.Fatal("NewCSRFRefresher returned nil with nil client")
	}
}

func TestCSRFRefresher_RefreshCSRFToken_EmptyCookie(t *testing.T) {
	refresher := NewCSRFRefresher(&http.Client{})
	_, err := refresher.RefreshCSRFToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty cookie")
	}
	if err.Error() != "cookie is required for CSRF refresh" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestCSRFRefresher_RefreshCSRFToken_HTTPError(t *testing.T) {
	// Server that immediately closes connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return non-200
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// We can't override MLLinkbuilderURL since it's a const.
	// Test with an unreachable server to verify the HTTP error path.
	client := &http.Client{
		Transport: &http.Transport{},
	}
	refresher := NewCSRFRefresher(client)

	// This will fail because MLLinkbuilderURL points to the real ML site.
	// We test only the empty cookie case directly; the HTTP path
	// is implicitly tested through other integration-like scenarios.
	_, err := refresher.RefreshCSRFToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty cookie")
	}
}

func TestCSRFRefresher_RefreshCSRFToken_MockServer_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	// Create a transport that redirects all requests to our test server
	transport := &redirectTransport{
		targetURL: server.URL,
		inner:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}
	refresher := NewCSRFRefresher(client)

	_, err := refresher.RefreshCSRFToken(context.Background(), "test-cookie=abc")
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if got := err.Error(); got != "linkbuilder returned status 403" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestCSRFRefresher_RefreshCSRFToken_MockServer_NoCSRF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>No CSRF here</body></html>`))
	}))
	defer server.Close()

	transport := &redirectTransport{
		targetURL: server.URL,
		inner:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}
	refresher := NewCSRFRefresher(client)

	_, err := refresher.RefreshCSRFToken(context.Background(), "test-cookie=abc")
	if err == nil {
		t.Fatal("expected error when no CSRF found")
	}
	if got := err.Error(); got != "could not extract CSRF token from page" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestCSRFRefresher_RefreshCSRFToken_MockServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify cookie was sent
		cookie := r.Header.Get("Cookie")
		if cookie != "test-cookie=abc" {
			t.Errorf("expected cookie header 'test-cookie=abc', got %q", cookie)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><head><meta name="csrf-token" content="fresh-csrf-token"></head></html>`))
	}))
	defer server.Close()

	transport := &redirectTransport{
		targetURL: server.URL,
		inner:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}
	refresher := NewCSRFRefresher(client)

	token, err := refresher.RefreshCSRFToken(context.Background(), "test-cookie=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "fresh-csrf-token" {
		t.Errorf("expected token 'fresh-csrf-token', got %q", token)
	}
}

// redirectTransport rewrites all request URLs to point at a test server.
type redirectTransport struct {
	targetURL string
	inner     http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point at our test server
	newReq := req.Clone(req.Context())
	parsed, _ := req.URL.Parse(t.targetURL)
	newReq.URL = parsed
	return t.inner.RoundTrip(newReq)
}
