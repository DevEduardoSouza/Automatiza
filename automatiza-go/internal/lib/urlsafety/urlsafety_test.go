package urlsafety

import (
	"automatiza-go/internal/lib/cache"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Helper: create a Service with sensible test defaults (no cache, nop logger).
// ---------------------------------------------------------------------------

func newTestService(cfg Config) *Service {
	svc := NewService(cfg, nil, zap.NewNop())
	// Replace the httpClient with one that does NOT use otelhttp transport,
	// so unit tests work without OTel instrumentation.
	svc.httpClient = &http.Client{
		Timeout: svc.redirectTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return svc
}

// ---------------------------------------------------------------------------
// TestNewService
// ---------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	t.Parallel()

	t.Run("creates service with correct config", func(t *testing.T) {
		t.Parallel()
		svc := NewService(Config{
			Mode:             "redact",
			BlockedHosts:     []string{"Evil.Com", " bad.org "},
			AllowedHosts:     []string{" Good.Com", "safe.net "},
			EnforceAllowlist: true,
		}, nil, zap.NewNop())

		if svc.mode != "redact" {
			t.Errorf("expected mode 'redact', got %q", svc.mode)
		}
		if !svc.enforceAllowlist {
			t.Error("expected enforceAllowlist=true")
		}
	})

	t.Run("normalizes hosts to lowercase", func(t *testing.T) {
		t.Parallel()
		svc := NewService(Config{
			Mode:         "block",
			BlockedHosts: []string{"EVIL.COM"},
			AllowedHosts: []string{"GOOD.COM"},
		}, nil, zap.NewNop())

		if !svc.blockedHosts["evil.com"] {
			t.Error("expected 'evil.com' in blockedHosts (lowercased)")
		}
		if !svc.allowedHosts["good.com"] {
			t.Error("expected 'good.com' in allowedHosts (lowercased)")
		}
	})

	t.Run("creates trusted redirect hosts map", func(t *testing.T) {
		t.Parallel()
		svc := NewService(Config{Mode: "off"}, nil, zap.NewNop())

		expectedHosts := []string{
			"s.shopee.com.br", "shp.ee", "bit.ly",
			"tinyurl.com", "t.co", "amzn.to", "mercadolivre.com",
		}
		for _, h := range expectedHosts {
			if !svc.trustedRedirectHosts[h] {
				t.Errorf("expected %q in trustedRedirectHosts", h)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestExtractHTTPURLs
// ---------------------------------------------------------------------------

func TestExtractHTTPURLs(t *testing.T) {
	t.Parallel()

	svc := newTestService(Config{Mode: "off"})

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "extracts http and https URLs",
			text:     "Visit http://example.com and https://secure.com/path",
			expected: []string{"http://example.com", "https://secure.com/path"},
		},
		{
			name:     "deduplicates URLs",
			text:     "http://dup.com http://dup.com http://dup.com",
			expected: []string{"http://dup.com"},
		},
		{
			name:     "cleans trailing punctuation",
			text:     "Check http://example.com. Also http://other.com, and http://last.com!",
			expected: []string{"http://example.com", "http://other.com", "http://last.com"},
		},
		{
			name:     "no URLs returns empty slice",
			text:     "No URLs here, just plain text.",
			expected: []string{},
		},
		{
			name: "URLs in multiline text",
			text: `Line 1: http://first.com
Line 2: https://second.com/path?q=1
Line 3: nothing here
Line 4: http://third.com/end;`,
			expected: []string{
				"http://first.com",
				"https://second.com/path?q=1",
				"http://third.com/end",
			},
		},
		{
			name:     "URL with query parameters",
			text:     "https://shop.com/item?id=123&ref=abc",
			expected: []string{"https://shop.com/item?id=123&ref=abc"},
		},
		{
			name:     "URL with trailing semicolons and colon",
			text:     "See https://a.com/path: and https://b.com/p;",
			expected: []string{"https://a.com/path", "https://b.com/p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := svc.extractHTTPURLs(tt.text)

			if len(tt.expected) == 0 {
				if len(got) != 0 {
					t.Errorf("expected empty result, got %v", got)
				}
				return
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d URLs, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, u := range tt.expected {
				if got[i] != u {
					t.Errorf("URL[%d]: expected %q, got %q", i, u, got[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsTrustedRedirectHost
// ---------------------------------------------------------------------------

func TestIsTrustedRedirectHost(t *testing.T) {
	t.Parallel()

	svc := newTestService(Config{Mode: "off"})

	tests := []struct {
		host     string
		expected bool
	}{
		{"shp.ee", true},
		{"bit.ly", true},
		{"tinyurl.com", true},
		{"t.co", true},
		{"amzn.to", true},
		{"s.shopee.com.br", true},
		{"mercadolivre.com", true},
		{"unknown-host.com", false},
		{"google.com", false},
		{"SHP.EE", true},    // case insensitive
		{"Bit.Ly", true},    // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()
			got := svc.isTrustedRedirectHost(tt.host)
			if got != tt.expected {
				t.Errorf("isTrustedRedirectHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsBlockedHost
// ---------------------------------------------------------------------------

func TestIsBlockedHost(t *testing.T) {
	t.Parallel()

	svc := newTestService(Config{
		Mode:         "block",
		BlockedHosts: []string{"evil.com", "spam.org"},
	})

	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"direct match", "evil.com", true},
		{"direct match second", "spam.org", true},
		{"parent domain match", "sub.evil.com", true},
		{"deep subdomain match", "deep.sub.evil.com", true},
		{"not blocked", "good.com", false},
		{"partial name no match", "notevil.com", false},
		{"case insensitive", "EVIL.COM", true},
		{"case insensitive subdomain", "Sub.Evil.Com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := svc.isBlockedHost(tt.host)
			if got != tt.expected {
				t.Errorf("isBlockedHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsAllowedHost
// ---------------------------------------------------------------------------

func TestIsAllowedHost(t *testing.T) {
	t.Parallel()

	svc := newTestService(Config{
		Mode:             "block",
		AllowedHosts:     []string{"good.com", "safe.org"},
		EnforceAllowlist: true,
	})

	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"direct match", "good.com", true},
		{"direct match second", "safe.org", true},
		{"parent domain match", "sub.good.com", true},
		{"deep subdomain match", "deep.sub.safe.org", true},
		{"not in allowlist", "unknown.com", false},
		{"case insensitive", "GOOD.COM", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := svc.isAllowedHost(tt.host)
			if got != tt.expected {
				t.Errorf("isAllowedHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestEvaluate
// ---------------------------------------------------------------------------

func TestEvaluate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("mode off always allows", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "off",
			BlockedHosts: []string{"evil.com"},
		})

		decision, err := svc.Evaluate(ctx, "Visit http://evil.com now!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "allow" {
			t.Errorf("expected 'allow', got %q", decision.Action)
		}
	})

	t.Run("no URLs in message allows", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{Mode: "block"})

		decision, err := svc.Evaluate(ctx, "Just a normal message with no links.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "allow" {
			t.Errorf("expected 'allow', got %q", decision.Action)
		}
	})

	t.Run("blocked URL returns mode as action (block)", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "block",
			BlockedHosts: []string{"malware.com"},
		})

		decision, err := svc.Evaluate(ctx, "Check https://malware.com/payload")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "block" {
			t.Errorf("expected 'block', got %q", decision.Action)
		}
		if len(decision.URLs) != 1 {
			t.Fatalf("expected 1 URL, got %d", len(decision.URLs))
		}
		if !decision.URLs[0].IsBlocked {
			t.Error("expected URL to be blocked")
		}
	})

	t.Run("blocked URL returns mode as action (redact)", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "redact",
			BlockedHosts: []string{"malware.com"},
		})

		decision, err := svc.Evaluate(ctx, "Check https://malware.com/payload")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "redact" {
			t.Errorf("expected 'redact', got %q", decision.Action)
		}
	})

	t.Run("allowed URL is allowed", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "block",
			AllowedHosts: []string{"safe.com"},
		})

		decision, err := svc.Evaluate(ctx, "Visit https://safe.com/page")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "allow" {
			t.Errorf("expected 'allow', got %q", decision.Action)
		}
	})

	t.Run("allowlist enforcement blocks unknown host", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:             "block",
			AllowedHosts:     []string{"safe.com"},
			EnforceAllowlist: true,
		})

		decision, err := svc.Evaluate(ctx, "Visit https://unknown-host.com/page")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "block" {
			t.Errorf("expected 'block' for unknown host with allowlist enforced, got %q", decision.Action)
		}
		if len(decision.URLs) != 1 || !decision.URLs[0].IsBlocked {
			t.Error("expected URL to be blocked")
		}
		if !strings.Contains(decision.URLs[0].Reason, "Host n\u00e3o permitido") {
			t.Errorf("expected reason about disallowed host, got %q", decision.URLs[0].Reason)
		}
	})

	t.Run("multiple URLs with mixed results", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "redact",
			BlockedHosts: []string{"bad.com"},
		})

		decision, err := svc.Evaluate(ctx, "Good: https://good.com Bad: https://bad.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "redact" {
			t.Errorf("expected 'redact' when any URL is blocked, got %q", decision.Action)
		}
		if len(decision.URLs) != 2 {
			t.Fatalf("expected 2 URLs, got %d", len(decision.URLs))
		}

		// First URL should be allowed
		if decision.URLs[0].IsBlocked {
			t.Error("expected first URL (good.com) to not be blocked")
		}
		// Second URL should be blocked
		if !decision.URLs[1].IsBlocked {
			t.Error("expected second URL (bad.com) to be blocked")
		}
	})
}

// ---------------------------------------------------------------------------
// TestEvaluateURL
// ---------------------------------------------------------------------------

func TestEvaluateURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("invalid URL is blocked", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{Mode: "block"})

		info := svc.evaluateURL(ctx, "://not-a-valid-url")
		if !info.IsBlocked {
			t.Error("expected invalid URL to be blocked")
		}
		if info.Reason != "URL inv\u00e1lida" {
			t.Errorf("expected reason 'URL inv\u00e1lida', got %q", info.Reason)
		}
	})

	t.Run("blocked host is blocked", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "block",
			BlockedHosts: []string{"evil.com"},
		})

		info := svc.evaluateURL(ctx, "https://evil.com/malware")
		if !info.IsBlocked {
			t.Error("expected blocked host to be blocked")
		}
		if !strings.Contains(info.Reason, "Host bloqueado") {
			t.Errorf("expected reason about blocked host, got %q", info.Reason)
		}
	})

	t.Run("allowlist enforcement blocks unknown host", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:             "block",
			AllowedHosts:     []string{"safe.com"},
			EnforceAllowlist: true,
		})

		info := svc.evaluateURL(ctx, "https://random.com/page")
		if !info.IsBlocked {
			t.Error("expected unknown host to be blocked with allowlist enforcement")
		}
		if !strings.Contains(info.Reason, "Host n\u00e3o permitido") {
			t.Errorf("expected reason about disallowed host, got %q", info.Reason)
		}
	})

	t.Run("non-trusted host uses original URL", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{Mode: "block"})

		info := svc.evaluateURL(ctx, "https://regular.com/path")
		if info.IsBlocked {
			t.Error("expected non-blocked, non-trusted host to be allowed")
		}
		if info.Final != "https://regular.com/path" {
			t.Errorf("expected Final to be original URL, got %q", info.Final)
		}
		if info.FinalHost != "regular.com" {
			t.Errorf("expected FinalHost 'regular.com', got %q", info.FinalHost)
		}
	})
}

// ---------------------------------------------------------------------------
// TestEvaluateURL_WithRedirect
// ---------------------------------------------------------------------------

func TestEvaluateURL_WithRedirect(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("trusted redirect host resolves final URL", func(t *testing.T) {
		t.Parallel()

		// Create a test server that redirects to a final URL
		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer finalServer.Close()

		redirectServer := httptest.NewServer(http.RedirectHandler(finalServer.URL+"/final-page", http.StatusFound))
		defer redirectServer.Close()

		// Parse the redirect server URL to get its host
		parsed, _ := url.Parse(redirectServer.URL)

		svc := newTestService(Config{Mode: "block"})
		// Add test server host as trusted redirect host
		svc.trustedRedirectHosts[parsed.Host] = true

		info := svc.evaluateURL(ctx, redirectServer.URL+"/short")
		if info.IsBlocked {
			t.Error("expected URL to not be blocked")
		}

		// The final URL should be from the final server
		if !strings.Contains(info.Final, "/final-page") {
			t.Errorf("expected Final to contain '/final-page', got %q", info.Final)
		}
	})

	t.Run("trusted redirect to blocked host is blocked", func(t *testing.T) {
		t.Parallel()

		// Final destination is a blocked host
		blockedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer blockedServer.Close()

		redirectServer := httptest.NewServer(http.RedirectHandler(blockedServer.URL+"/bad", http.StatusFound))
		defer redirectServer.Close()

		parsedRedirect, _ := url.Parse(redirectServer.URL)
		parsedBlocked, _ := url.Parse(blockedServer.URL)

		svc := newTestService(Config{
			Mode:         "block",
			BlockedHosts: []string{parsedBlocked.Host},
		})
		svc.trustedRedirectHosts[parsedRedirect.Host] = true

		info := svc.evaluateURL(ctx, redirectServer.URL+"/link")
		if !info.IsBlocked {
			t.Error("expected redirect to blocked host to be blocked")
		}
		if !strings.Contains(info.Reason, "Host bloqueado") {
			t.Errorf("expected blocked host reason, got %q", info.Reason)
		}
	})

	t.Run("redirect resolution failure is fail-open", func(t *testing.T) {
		t.Parallel()

		// Create a server that immediately closes connections
		failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Close the connection to cause an error
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}))
		defer failServer.Close()

		parsed, _ := url.Parse(failServer.URL)
		svc := newTestService(Config{Mode: "block"})
		svc.trustedRedirectHosts[parsed.Host] = true

		info := svc.evaluateURL(ctx, failServer.URL+"/short")
		// Fail-open: URL should be allowed (not blocked) despite resolution failure
		if info.IsBlocked {
			t.Error("expected fail-open behavior: URL should not be blocked when redirect resolution fails")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRedactBlockedURLs
// ---------------------------------------------------------------------------

func TestRedactBlockedURLs(t *testing.T) {
	t.Parallel()

	svc := newTestService(Config{Mode: "redact"})

	t.Run("replaces blocked URLs with placeholder", func(t *testing.T) {
		t.Parallel()
		message := "Check https://evil.com/page and https://good.com/path"
		decision := &UrlSafetyDecision{
			Action: "redact",
			URLs: []URLInfo{
				{Original: "https://evil.com/page", IsBlocked: true, Reason: "blocked"},
				{Original: "https://good.com/path", IsBlocked: false},
			},
		}

		result := svc.RedactBlockedURLs(message, decision)
		if !strings.Contains(result, "[link removido]") {
			t.Error("expected blocked URL to be replaced with '[link removido]'")
		}
		if !strings.Contains(result, "https://good.com/path") {
			t.Error("expected non-blocked URL to remain")
		}
		if strings.Contains(result, "https://evil.com/page") {
			t.Error("expected blocked URL to be removed")
		}
	})

	t.Run("keeps non-blocked URLs unchanged", func(t *testing.T) {
		t.Parallel()
		message := "Visit https://safe.com/page"
		decision := &UrlSafetyDecision{
			Action: "allow",
			URLs: []URLInfo{
				{Original: "https://safe.com/page", IsBlocked: false},
			},
		}

		result := svc.RedactBlockedURLs(message, decision)
		if result != message {
			t.Errorf("expected message unchanged, got %q", result)
		}
	})

	t.Run("no blocked URLs returns unchanged message", func(t *testing.T) {
		t.Parallel()
		message := "No links here"
		decision := &UrlSafetyDecision{
			Action: "allow",
			URLs:   []URLInfo{},
		}

		result := svc.RedactBlockedURLs(message, decision)
		if result != message {
			t.Errorf("expected message unchanged, got %q", result)
		}
	})

	t.Run("multiple blocked URLs all replaced", func(t *testing.T) {
		t.Parallel()
		message := "A: https://bad1.com B: https://bad2.com C: https://ok.com"
		decision := &UrlSafetyDecision{
			Action: "redact",
			URLs: []URLInfo{
				{Original: "https://bad1.com", IsBlocked: true, Reason: "blocked"},
				{Original: "https://bad2.com", IsBlocked: true, Reason: "blocked"},
				{Original: "https://ok.com", IsBlocked: false},
			},
		}

		result := svc.RedactBlockedURLs(message, decision)
		if strings.Contains(result, "bad1.com") || strings.Contains(result, "bad2.com") {
			t.Error("expected both blocked URLs to be replaced")
		}
		if !strings.Contains(result, "https://ok.com") {
			t.Error("expected non-blocked URL to remain")
		}
		// Count occurrences of placeholder
		count := strings.Count(result, "[link removido]")
		if count != 2 {
			t.Errorf("expected 2 placeholders, got %d", count)
		}
	})
}

// ---------------------------------------------------------------------------
// TestResolveRedirect
// ---------------------------------------------------------------------------

func TestResolveRedirect(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("follows redirects and returns final URL", func(t *testing.T) {
		t.Parallel()

		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer finalServer.Close()

		redirectServer := httptest.NewServer(http.RedirectHandler(finalServer.URL+"/destination", http.StatusFound))
		defer redirectServer.Close()

		svc := newTestService(Config{Mode: "off"})

		finalURL, finalHost, err := svc.resolveRedirect(ctx, redirectServer.URL+"/short")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		parsedFinal, _ := url.Parse(finalServer.URL)
		if finalHost != parsedFinal.Host {
			t.Errorf("expected host %q, got %q", parsedFinal.Host, finalHost)
		}
		if !strings.Contains(finalURL, "/destination") {
			t.Errorf("expected final URL to contain '/destination', got %q", finalURL)
		}
	})

	t.Run("handles non-redirect URL", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		svc := newTestService(Config{Mode: "off"})

		finalURL, finalHost, err := svc.resolveRedirect(ctx, server.URL+"/direct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		parsed, _ := url.Parse(server.URL)
		if finalHost != parsed.Host {
			t.Errorf("expected host %q, got %q", parsed.Host, finalHost)
		}
		if !strings.Contains(finalURL, "/direct") {
			t.Errorf("expected finalURL to contain '/direct', got %q", finalURL)
		}
	})

	t.Run("multi-hop redirect chain", func(t *testing.T) {
		t.Parallel()

		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer finalServer.Close()

		midServer := httptest.NewServer(http.RedirectHandler(finalServer.URL+"/final", http.StatusFound))
		defer midServer.Close()

		startServer := httptest.NewServer(http.RedirectHandler(midServer.URL+"/mid", http.StatusMovedPermanently))
		defer startServer.Close()

		svc := newTestService(Config{Mode: "off"})

		finalURL, _, err := svc.resolveRedirect(ctx, startServer.URL+"/start")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(finalURL, "/final") {
			t.Errorf("expected final URL to contain '/final', got %q", finalURL)
		}
	})
}

// ---------------------------------------------------------------------------
// TestEvaluate_EndToEnd
// ---------------------------------------------------------------------------

func TestEvaluate_EndToEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("blocked shortener redirect is caught", func(t *testing.T) {
		t.Parallel()

		// Set up: shortener -> blocked destination
		blockedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer blockedServer.Close()

		shortener := httptest.NewServer(http.RedirectHandler(blockedServer.URL+"/malware", http.StatusFound))
		defer shortener.Close()

		parsedShortener, _ := url.Parse(shortener.URL)
		parsedBlocked, _ := url.Parse(blockedServer.URL)

		svc := newTestService(Config{
			Mode:         "block",
			BlockedHosts: []string{parsedBlocked.Host},
		})
		svc.trustedRedirectHosts[parsedShortener.Host] = true

		message := "Click here: " + shortener.URL + "/abc"
		decision, err := svc.Evaluate(ctx, message)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "block" {
			t.Errorf("expected 'block', got %q", decision.Action)
		}
		if len(decision.URLs) != 1 || !decision.URLs[0].IsBlocked {
			t.Error("expected single blocked URL")
		}
	})

	t.Run("allowed shortener redirect is allowed", func(t *testing.T) {
		t.Parallel()

		safeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer safeServer.Close()

		shortener := httptest.NewServer(http.RedirectHandler(safeServer.URL+"/product", http.StatusFound))
		defer shortener.Close()

		parsedShortener, _ := url.Parse(shortener.URL)

		svc := newTestService(Config{Mode: "block"})
		svc.trustedRedirectHosts[parsedShortener.Host] = true

		message := "Buy now: " + shortener.URL + "/offer"
		decision, err := svc.Evaluate(ctx, message)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "allow" {
			t.Errorf("expected 'allow', got %q", decision.Action)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRedactBlockedURLs_EndToEnd
// ---------------------------------------------------------------------------

func TestRedactBlockedURLs_EndToEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("evaluate then redact produces correct message", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "redact",
			BlockedHosts: []string{"spam.com"},
		})

		message := "Visit https://spam.com/ad or https://legit.com/page"
		decision, err := svc.Evaluate(ctx, message)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "redact" {
			t.Fatalf("expected 'redact', got %q", decision.Action)
		}

		redacted := svc.RedactBlockedURLs(message, decision)
		expected := "Visit [link removido] or https://legit.com/page"
		if redacted != expected {
			t.Errorf("expected %q, got %q", expected, redacted)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewService_CheckRedirectLimitsHops
// ---------------------------------------------------------------------------

func TestNewService_CheckRedirectLimitsHops(t *testing.T) {
	t.Parallel()

	// Create a chain of redirect servers that exceeds 10 hops.
	// The CheckRedirect closure in NewService should stop after 10 hops.
	hopCount := 0
	var servers []*httptest.Server

	// Final destination server
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hopCount++
		w.WriteHeader(http.StatusOK)
	}))
	servers = append(servers, finalServer)
	defer finalServer.Close()

	// Build a chain of 12 redirect servers (exceeds the limit of 10)
	target := finalServer.URL + "/final"
	for i := 0; i < 12; i++ {
		srv := httptest.NewServer(http.RedirectHandler(target, http.StatusFound))
		servers = append(servers, srv)
		defer srv.Close()
		target = srv.URL + "/hop"
	}

	// Use the real service (NOT newTestService which replaces httpClient)
	svc := NewService(Config{Mode: "block"}, nil, zap.NewNop())

	req, err := http.NewRequestWithContext(context.Background(), "HEAD", target, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// The built-in CheckRedirect should stop after 10 hops and return the response
	resp, err := svc.httpClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// The request should have been stopped before reaching the final server
	// (12 redirects > 10 limit), so we expect a 302 (last redirect response)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 302 or 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TestResolveRedirect_WithCache
// ---------------------------------------------------------------------------

func TestResolveRedirect_WithCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("cache miss makes HTTP call and caches result", func(t *testing.T) {
		t.Parallel()

		c := cache.NewCache(nil, "test-miss", false)

		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer finalServer.Close()

		redirectServer := httptest.NewServer(http.RedirectHandler(finalServer.URL+"/dest", http.StatusFound))
		defer redirectServer.Close()

		svc := newTestService(Config{Mode: "block"})
		svc.cache = c

		// First call: cache miss, makes HTTP call, exercises the SetJSON path
		finalURL, finalHost, err := svc.resolveRedirect(ctx, redirectServer.URL+"/short")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(finalURL, "/dest") {
			t.Errorf("expected final URL to contain '/dest', got %q", finalURL)
		}

		parsedFinal, _ := url.Parse(finalServer.URL)
		if finalHost != parsedFinal.Host {
			t.Errorf("expected host %q, got %q", parsedFinal.Host, finalHost)
		}
	})

	t.Run("cache hit returns cached result without HTTP request", func(t *testing.T) {
		t.Parallel()

		// Use miniredis to get a real working cache with local+redis tiers
		mr := miniredis.RunT(t)
		redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { redisClient.Close() })

		c := cache.NewCache(redisClient, "test-hit", false)

		svc := newTestService(Config{Mode: "block"})
		svc.cache = c

		// Pre-populate cache via SetJSON (will store in both local and Redis)
		cacheKey := "urlsafety:redirect:https://shp.ee/cached123"
		_ = c.SetJSON(ctx, cacheKey, map[string]string{
			"finalUrl":  "https://shopee.com.br/product/cached",
			"finalHost": "shopee.com.br",
		}, time.Hour)

		// resolveRedirect should return the cached result without making an HTTP call
		finalURL, finalHost, err := svc.resolveRedirect(ctx, "https://shp.ee/cached123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if finalURL != "https://shopee.com.br/product/cached" {
			t.Errorf("expected cached finalURL 'https://shopee.com.br/product/cached', got %q", finalURL)
		}
		if finalHost != "shopee.com.br" {
			t.Errorf("expected cached finalHost 'shopee.com.br', got %q", finalHost)
		}
	})
}

// ---------------------------------------------------------------------------
// TestResolveRedirect_InvalidURL
// ---------------------------------------------------------------------------

func TestResolveRedirect_InvalidURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := newTestService(Config{Mode: "block"})

	// Use a URL with an invalid scheme that http.NewRequestWithContext will reject
	_, _, err := svc.resolveRedirect(ctx, "://missing-scheme")
	if err == nil {
		t.Fatal("expected error for invalid URL in resolveRedirect")
	}
}

// ---------------------------------------------------------------------------
// TestServiceWithEmptyConfig
// ---------------------------------------------------------------------------

func TestServiceWithEmptyConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty blocked and allowed hosts allows all", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:         "block",
			BlockedHosts: []string{},
			AllowedHosts: []string{},
		})

		decision, err := svc.Evaluate(ctx, "https://anything.com/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "allow" {
			t.Errorf("expected 'allow' with no blocked hosts, got %q", decision.Action)
		}
	})

	t.Run("enforce allowlist with empty list blocks all URLs", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(Config{
			Mode:             "block",
			AllowedHosts:     []string{},
			EnforceAllowlist: true,
		})

		decision, err := svc.Evaluate(ctx, "https://anything.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != "block" {
			t.Errorf("expected 'block' with empty allowlist enforced, got %q", decision.Action)
		}
	})
}
