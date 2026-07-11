package automation

import (
	"errors"
	"testing"
)

// TestIsTransientShopeeEnrichError locks the classification that decides whether
// a degraded Shopee list item is retried (transient) or consumed (permanent).
// The strings below are the real error texts produced by the Shopee service and
// its HTTP/GraphQL layers, so a wording change there that would flip the
// consume/retry decision fails this test.
func TestIsTransientShopeeEnrichError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// Transient — must be retried, item kept pending.
		{"rate limit 10030 (the 10/07 incident)", errors.New("shopee API error: error [10030]: Rate limit exceeded"), true},
		{"context deadline", errors.New("failed to expand short URL: context deadline exceeded"), true},
		{"request failed timeout", errors.New("shopee API request failed: context deadline exceeded"), true},
		{"http 500", errors.New("shopee API error 500: internal error"), true},
		{"http 503", errors.New("shopee API error 503: service unavailable"), true},
		{"http 429 too many requests", errors.New("shopee API error 429: too many requests"), true},
		{"connection reset", errors.New("read tcp: connection reset by peer"), true},

		// Permanent — must consume so the list does not stall on a dead item.
		{"product not found", errors.New("product not found"), false},
		{"credentials not configured", errors.New("shopee credentials not configured"), false},
		{"credentials incomplete", errors.New("shopee credentials incomplete - please configure App ID and Password"), false},
		{"invalid signature", errors.New("shopee API error: Invalid Signature"), false},
		{"no open api access 10035", errors.New("shopee: account has no Affiliate Open API Platform access (error 10035)"), false},
		{"could not extract ids", errors.New("could not extract product IDs from URL"), false},

		// Guards.
		{"nil error", nil, false},
		{"unknown error is treated as permanent", errors.New("some brand new failure mode"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientShopeeEnrichError(tc.err); got != tc.want {
				t.Fatalf("isTransientShopeeEnrichError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
