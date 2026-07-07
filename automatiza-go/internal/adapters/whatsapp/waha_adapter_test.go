package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestAdapter creates a WahaAdapter for testing with a mock server.
func newTestAdapter(t *testing.T, handler http.HandlerFunc) (*WahaAdapter, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(func() { server.Close() })

	adapter, err := NewWahaAdapter(WahaAdapterConfig{
		APIURL:         server.URL,
		APIKey:         "test-api-key",
		WebhookBaseURL: "http://localhost:8080",
		WebhookSecret:  "test-secret",
		HTTPTimeout:    5 * time.Second,
		StatusCacheTTL: 100 * time.Millisecond,
	}, zap.NewNop())
	require.NoError(t, err)

	return adapter, server
}

// jsonResponse is a helper to create JSON response handlers.
func jsonResponse(t *testing.T, statusCode int, body interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			err := json.NewEncoder(w).Encode(body)
			require.NoError(t, err)
		}
	}
}

// =============================================================================
// Configuration Tests
// =============================================================================

func TestWahaAdapterConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := WahaAdapterConfig{
			APIURL: "http://localhost:3000",
			APIKey: "test-key",
		}
		assert.NoError(t, cfg.validate())
	})

	t.Run("missing APIURL", func(t *testing.T) {
		cfg := WahaAdapterConfig{
			APIKey: "test-key",
		}
		err := cfg.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "APIURL")
	})

	t.Run("missing APIKey", func(t *testing.T) {
		cfg := WahaAdapterConfig{
			APIURL: "http://localhost:3000",
		}
		err := cfg.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "APIKey")
	})
}

func TestNewWahaAdapter(t *testing.T) {
	t.Run("creates adapter with valid config", func(t *testing.T) {
		adapter, err := NewWahaAdapter(WahaAdapterConfig{
			APIURL: "http://localhost:3000",
			APIKey: "test-key",
		}, nil)
		require.NoError(t, err)
		assert.NotNil(t, adapter)
	})

	t.Run("returns error for invalid config", func(t *testing.T) {
		_, err := NewWahaAdapter(WahaAdapterConfig{}, nil)
		assert.Error(t, err)
	})

	t.Run("uses default values", func(t *testing.T) {
		adapter, err := NewWahaAdapter(WahaAdapterConfig{
			APIURL: "http://localhost:3000",
			APIKey: "test-key",
		}, nil)
		require.NoError(t, err)

		// Should use default HTTP timeout (30s) and status cache TTL (30s)
		assert.NotNil(t, adapter.httpClient)
		assert.Equal(t, 30*time.Second, adapter.statusCacheTTL)
	})
}

// =============================================================================
// Session Manager Tests
// =============================================================================

func TestWahaAdapter_CreateSession(t *testing.T) {
	ctx := context.Background()

	t.Run("creates session successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/sessions", r.URL.Path)
			assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))

			// Verify request body
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, "test-session", payload["name"])
			assert.Equal(t, true, payload["start"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"name":   "test-session",
				"status": "SCAN_QR_CODE",
				"engine": "GOWS",
			})(w, r)
		})

		result, err := adapter.CreateSession(ctx, CreateSessionInput{
			SessionName: "test-session",
		})

		require.NoError(t, err)
		assert.Equal(t, "test-session", result.Name)
		assert.Equal(t, SessionStatusScanQR, result.Status)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusInternalServerError, map[string]string{
			"error":   "Internal Server Error",
			"message": "Something went wrong",
		}))

		_, err := adapter.CreateSession(ctx, CreateSessionInput{
			SessionName: "test-session",
		})

		assert.Error(t, err)
	})
}

func TestWahaAdapter_GetSessionStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("returns session status", func(t *testing.T) {
		callCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/api/sessions/test-session")

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"name":   "test-session",
				"status": "CONNECTED",
				"me": map[string]interface{}{
					"id":       "5511999999999@c.us",
					"pushName": "Test User",
				},
			})(w, r)
		})

		result, err := adapter.GetSessionStatus(ctx, "test-session")

		require.NoError(t, err)
		assert.Equal(t, "test-session", result.Name)
		assert.Equal(t, SessionStatusConnected, result.Status)
		assert.NotNil(t, result.AuthenticatedUser)
		assert.Equal(t, "5511999999999@c.us", result.AuthenticatedUser.ID)
		assert.Equal(t, 1, callCount)
	})

	t.Run("caches status", func(t *testing.T) {
		callCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"name":   "test-session",
				"status": "CONNECTED",
			})(w, r)
		})

		// First call - should hit API
		_, err := adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Second call - should use cache
		_, err = adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, 1, callCount, "should use cached status")

		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		// Third call - cache expired, should hit API
		_, err = adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusNotFound, map[string]string{
			"message": "Session not found",
		}))

		_, err := adapter.GetSessionStatus(ctx, "non-existent")

		assert.Error(t, err)
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			assert.Equal(t, ErrCodeSessionNotFound, providerErr.Code)
		}
	})
}

func TestWahaAdapter_DeleteSession(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes session successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/api/sessions/test-session")
			w.WriteHeader(http.StatusOK)
		})

		err := adapter.DeleteSession(ctx, "test-session")
		assert.NoError(t, err)
	})

	t.Run("clears cache on delete", func(t *testing.T) {
		getCallCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/sessions/") {
				getCallCount++
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "CONNECTED",
				})(w, r)
			} else if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
			}
		})

		// Get status to populate cache
		_, _ = adapter.GetSessionStatus(ctx, "test-session")
		assert.Equal(t, 1, getCallCount)

		// Get again - should use cache
		_, _ = adapter.GetSessionStatus(ctx, "test-session")
		assert.Equal(t, 1, getCallCount, "should use cached status")

		// Delete session (this clears the cache)
		_ = adapter.DeleteSession(ctx, "test-session")

		// Get status again - should hit API (cache was cleared)
		_, _ = adapter.GetSessionStatus(ctx, "test-session")
		assert.Equal(t, 2, getCallCount, "should hit API after cache cleared")
	})
}

// =============================================================================
// Message Sender Tests
// =============================================================================

func TestWahaAdapter_SendText(t *testing.T) {
	ctx := context.Background()

	t.Run("sends text message successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendText")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, "group@g.us", payload["chatId"])
			assert.Equal(t, "Hello, World!", payload["text"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-123",
				"ack":       0,
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello, World!",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-123", result.ID)
		assert.False(t, result.Timestamp.IsZero())
	})

	t.Run("returns error on failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusBadRequest, map[string]string{
				"message": "Invalid chat ID",
			})(w, r)
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "invalid",
			Text:        "Hello",
		})

		assert.Error(t, err)
	})
}

func TestWahaAdapter_SendImage(t *testing.T) {
	ctx := context.Background()

	t.Run("sends image message successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendImage")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, "group@g.us", payload["chatId"])
			assert.Equal(t, "Check this out!", payload["caption"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-456",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Caption:     "Check this out!",
			ImageBase64: "base64data",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-456", result.ID)
	})
}

// TestWahaAdapter_SendImage_RelayBase64Unchanged verifies that relayed/forwarded
// media (the offer-generate/monitor relay path, supplied as base64) is sent
// byte-for-byte AS-IS — no aspect padding. Squaring it onto a white canvas added
// huge borders to wide coupon banners and portrait coupon grids; verified on a
// real device that the group bubble does not crop these, and a forwarded post is
// required to look identical to the source. Product photos sent by URL still get
// aspect-normalized (separate path, asserted elsewhere).
func TestWahaAdapter_SendImage_RelayBase64Unchanged(t *testing.T) {
	ctx := context.Background()

	// 1910x500 (3.82:1) wide banner → must reach the wire unchanged (1910x500).
	var src bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1910, 500))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 10, G: 120, B: 200, A: 255}}, image.Point{}, draw.Src)
	require.NoError(t, jpeg.Encode(&src, img, &jpeg.Options{Quality: 90}))
	wideB64 := base64.StdEncoding.EncodeToString(src.Bytes())

	var gotData string
	var gotW, gotH int
	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
			jsonResponse(t, http.StatusOK, map[string]interface{}{"name": "s", "status": "WORKING"})(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))
		file, _ := payload["file"].(map[string]interface{})
		gotData, _ = file["data"].(string)
		raw, derr := base64.StdEncoding.DecodeString(gotData)
		require.NoError(t, derr)
		cfg, _, cerr := image.DecodeConfig(bytes.NewReader(raw))
		require.NoError(t, cerr)
		gotW, gotH = cfg.Width, cfg.Height
		jsonResponse(t, http.StatusOK, map[string]interface{}{"id": "ok"})(w, r)
	})

	_, err := adapter.SendImage(ctx, SendImageInput{
		SessionName: "s",
		ChatID:      "g@g.us",
		Caption:     "promo",
		ImageBase64: wideB64,
	})
	require.NoError(t, err)
	assert.Equal(t, wideB64, gotData, "relayed base64 must be forwarded byte-for-byte, unmodified")
	assert.Equal(t, 1910, gotW, "relayed banner width must be preserved")
	assert.Equal(t, 500, gotH, "relayed banner height must be preserved (not squared)")
}

func TestIsTransientWahaUploadError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"500 wrapping upstream 429", http.StatusInternalServerError, `{"statusCode":500,"message":"2 UNKNOWN: upload failed with status code 429"}`, true},
		{"500 wrapping upstream 500", http.StatusInternalServerError, `{"message":"upload failed with status code 500"}`, true},
		{"500 wrapping upstream 503", http.StatusInternalServerError, `{"message":"upload failed with status code 503"}`, true},
		{"500 wrapping upstream 400 handled by isTransientUpload400, not here", http.StatusInternalServerError, `{"message":"upload failed with status code 400"}`, false},
		{"500 not-participating not retried", http.StatusInternalServerError, `{"message":"you're not participating in that group"}`, false},
		{"non-500 status ignored", http.StatusServiceUnavailable, `{"message":"upload failed with status code 429"}`, false},
		{"empty body", http.StatusInternalServerError, ``, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isTransientWahaUploadError(tc.statusCode, []byte(tc.body)))
		})
	}
}

func TestIsTransientUpload400(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"500 wrapping upstream 400", http.StatusInternalServerError, `{"statusCode":500,"message":"2 UNKNOWN: upload failed with status code 400"}`, true},
		{"500 wrapping upstream 429 is not a 400", http.StatusInternalServerError, `{"message":"upload failed with status code 429"}`, false},
		{"500 not-participating is not an upload 400", http.StatusInternalServerError, `{"message":"you're not participating in that group"}`, false},
		{"plain 400 status (not the wrapped form) ignored", http.StatusBadRequest, `{"message":"upload failed with status code 400"}`, false},
		{"empty body", http.StatusInternalServerError, ``, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isTransientUpload400(tc.statusCode, []byte(tc.body)))
		})
	}
}

// TestWahaAdapter_SendImage_RetriesTransientUpload verifies that a WAHA HTTP 500
// wrapping a transient upstream media-upload failure (429) is retried by
// doRequest and recovers, that a wrapped 400 is also retried but capped at
// softUploadMaxRetries (so a persistent 400 fails fast to the text fallback
// instead of burning the full retry budget), and that a wrapped 400 recovers
// when the retry succeeds.
func TestWahaAdapter_SendImage_RetriesTransientUpload(t *testing.T) {
	ctx := context.Background()

	statusHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
			jsonResponse(t, http.StatusOK, map[string]interface{}{"name": "s", "status": "WORKING"})(w, r)
			return true
		}
		return false
	}

	t.Run("retries wrapped 429 then succeeds", func(t *testing.T) {
		var sendCalls int32
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if statusHandler(w, r) {
				return
			}
			n := atomic.AddInt32(&sendCalls, 1)
			if n == 1 {
				jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
					"statusCode": 500,
					"message":    "2 UNKNOWN: upload failed with status code 429",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusOK, map[string]interface{}{"id": "msg-ok"})(w, r)
		})

		result, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "s",
			ChatID:      "group@g.us",
			Caption:     "promo",
			ImageBase64: "base64data",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-ok", result.ID)
		assert.Equal(t, int32(2), atomic.LoadInt32(&sendCalls), "should retry the wrapped 429 exactly once")
	})

	t.Run("retries wrapped 400 once then succeeds", func(t *testing.T) {
		var sendCalls int32
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if statusHandler(w, r) {
				return
			}
			n := atomic.AddInt32(&sendCalls, 1)
			if n == 1 {
				jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
					"statusCode": 500,
					"message":    "2 UNKNOWN: upload failed with status code 400",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusOK, map[string]interface{}{"id": "msg-ok"})(w, r)
		})

		result, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "s",
			ChatID:      "group@g.us",
			Caption:     "promo",
			ImageBase64: "base64data",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-ok", result.ID)
		assert.Equal(t, int32(2), atomic.LoadInt32(&sendCalls), "transient wrapped 400 should recover on the capped retry")
	})

	t.Run("persistent wrapped 400 is capped at softUploadMaxRetries then fails", func(t *testing.T) {
		var sendCalls int32
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if statusHandler(w, r) {
				return
			}
			atomic.AddInt32(&sendCalls, 1)
			jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
				"statusCode": 500,
				"message":    "2 UNKNOWN: upload failed with status code 400",
			})(w, r)
		})

		_, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "s",
			ChatID:      "group@g.us",
			Caption:     "promo",
			ImageBase64: "base64data",
		})

		require.Error(t, err)
		// 1 initial attempt + softUploadMaxRetries(1) retry = 2, NOT the full
		// retry budget — a genuinely bad image must not hammer WAHA.
		assert.Equal(t, int32(softUploadMaxRetries+1), atomic.LoadInt32(&sendCalls),
			"persistent wrapped 400 must be capped, then fall through to the text fallback")
	})
}

// TestWahaAdapter_SendImage_RetriesRequestTimeout verifies that a per-request
// httpClient timeout (WAHA/WhatsApp media upload momentarily slow) is retried
// once — while the caller's ctx still has budget — instead of dropping the
// image to the text-only fallback. Once 429/400 codes are retried, this timeout
// is the dominant remaining "sem foto" cause.
func TestWahaAdapter_SendImage_RetriesRequestTimeout(t *testing.T) {
	ctx := context.Background()

	var sendCalls int32
	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
			jsonResponse(t, http.StatusOK, map[string]interface{}{"name": "s", "status": "WORKING"})(w, r)
			return
		}
		n := atomic.AddInt32(&sendCalls, 1)
		if n == 1 {
			time.Sleep(400 * time.Millisecond) // hang past the client timeout
			return
		}
		jsonResponse(t, http.StatusOK, map[string]interface{}{"id": "msg-ok"})(w, r)
	})
	// Shorten the per-request timeout so the test is fast; the caller ctx
	// (Background, no deadline) stays alive, so the timeout must be retried.
	adapter.httpClient.Timeout = 120 * time.Millisecond

	result, err := adapter.SendImage(ctx, SendImageInput{
		SessionName: "s",
		ChatID:      "group@g.us",
		Caption:     "promo",
		ImageBase64: "base64data",
	})

	require.NoError(t, err)
	assert.Equal(t, "msg-ok", result.ID)
	assert.Equal(t, int32(2), atomic.LoadInt32(&sendCalls), "a request timeout should be retried exactly once")
}

// =============================================================================
// Group Manager Tests
// =============================================================================

func TestWahaAdapter_GetGroups(t *testing.T) {
	ctx := context.Background()

	t.Run("returns list of groups", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/groups")

			jsonResponse(t, http.StatusOK, []map[string]interface{}{
				{
					"id":      "group1@g.us",
					"subject": "Group One",
					"size":    10,
				},
				{
					"id":      "group2@g.us",
					"subject": "Group Two",
					"size":    20,
				},
			})(w, r)
		})

		result, err := adapter.GetGroups(ctx, "test-session")

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "group1@g.us", result[0].ID)
		assert.Equal(t, "Group One", result[0].Subject)
	})
}

func TestWahaAdapter_GetGroupParticipants(t *testing.T) {
	ctx := context.Background()

	t.Run("returns list of participants", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/participants")

			// WAHA returns participants as an array directly
			jsonResponse(t, http.StatusOK, []map[string]interface{}{
				{
					"id":           "user1@c.us",
					"isAdmin":      true,
					"isSuperAdmin": true,
				},
				{
					"id":           "user2@c.us",
					"isAdmin":      false,
					"isSuperAdmin": false,
				},
			})(w, r)
		})

		result, err := adapter.GetGroupParticipants(ctx, "test-session", "group@g.us")

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.True(t, result[0].IsSuperAdmin)
		assert.False(t, result[1].IsAdmin)
	})
}

func TestWahaAdapter_GetGroupInviteCode(t *testing.T) {
	ctx := context.Background()

	t.Run("returns invite code successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/groups/")
			assert.Contains(t, r.URL.Path, "/invite-code")

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"inviteCode": "AbCdEfGhIjKlMnOpQrStUv",
			})(w, r)
		})

		result, err := adapter.GetGroupInviteCode(ctx, "test-session", "group@g.us")

		require.NoError(t, err)
		assert.Equal(t, "AbCdEfGhIjKlMnOpQrStUv", result)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusNotFound, map[string]string{
			"message": "Group not found",
		}))

		_, err := adapter.GetGroupInviteCode(ctx, "test-session", "non-existent@g.us")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get group invite code")
	})

	t.Run("returns error for permission denied", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusForbidden, map[string]string{
			"message": "Not admin of group",
		}))

		_, err := adapter.GetGroupInviteCode(ctx, "test-session", "group@g.us")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get group invite code")
	})
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestWahaAdapter_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name           string
		statusCode     int
		expectedErrMsg string
		responseBody   map[string]string
	}{
		{
			name:           "401 unauthorized",
			statusCode:     http.StatusUnauthorized,
			expectedErrMsg: "failed to get session status",
			responseBody:   map[string]string{"message": "Unauthorized"},
		},
		{
			name:           "403 forbidden",
			statusCode:     http.StatusForbidden,
			expectedErrMsg: "failed to get session status",
			responseBody:   map[string]string{"message": "Forbidden"},
		},
		{
			name:           "404 not found",
			statusCode:     http.StatusNotFound,
			expectedErrMsg: "failed to get session status",
			responseBody:   map[string]string{"message": "Not found"},
		},
		{
			name:           "429 rate limited",
			statusCode:     http.StatusTooManyRequests,
			expectedErrMsg: "failed to get session status",
			responseBody:   map[string]string{"message": "Rate limited"},
		},
		{
			name:           "500 internal error",
			statusCode:     http.StatusInternalServerError,
			expectedErrMsg: "failed to get session status",
			responseBody:   map[string]string{"message": "Internal error"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := newTestAdapter(t, jsonResponse(t, tc.statusCode, tc.responseBody))

			_, err := adapter.GetSessionStatus(ctx, "test-session")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErrMsg)
		})
	}
}

func TestWahaAdapter_ContextCancellation(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(time.Second)
			w.WriteHeader(http.StatusOK)
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := adapter.GetSessionStatus(ctx, "test-session")

		assert.Error(t, err)
	})

	t.Run("respects context timeout", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(time.Second)
			w.WriteHeader(http.StatusOK)
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := adapter.GetSessionStatus(ctx, "test-session")

		assert.Error(t, err)
	})
}

// =============================================================================
// URL Escaping Tests
// =============================================================================

func TestWahaAdapter_URLEscaping(t *testing.T) {
	ctx := context.Background()

	t.Run("handles session names with special characters", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// The raw URL in the request will have escaped characters
			// The Path is already decoded by the http server
			// This test verifies the request is made successfully
			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"name":   "session/with/slashes",
				"status": "CONNECTED",
			})(w, r)
		})

		result, err := adapter.GetSessionStatus(ctx, "session/with/slashes")
		assert.NoError(t, err)
		assert.Equal(t, "session/with/slashes", result.Name)
	})

	t.Run("handles session names with spaces", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"name":   "my session",
				"status": "CONNECTED",
			})(w, r)
		})

		result, err := adapter.GetSessionStatus(ctx, "my session")
		assert.NoError(t, err)
		assert.Equal(t, "my session", result.Name)
	})
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestWahaAdapter_InterfaceCompliance(t *testing.T) {
	// Verify WahaAdapter implements WhatsAppProvider
	var _ WhatsAppProvider = (*WahaAdapter)(nil)
	var _ SessionManager = (*WahaAdapter)(nil)
	var _ GroupManager = (*WahaAdapter)(nil)
	var _ MessageSender = (*WahaAdapter)(nil)
	var _ WebhookManager = (*WahaAdapter)(nil)
	var _ MessageReconciler = (*WahaAdapter)(nil)
}

// =============================================================================
// Request Header Tests
// =============================================================================

func TestWahaAdapter_RequestHeaders(t *testing.T) {
	ctx := context.Background()

	t.Run("includes API key in requests", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		})

		_, _ = adapter.GetSessionStatus(ctx, "test")
	})
}

// =============================================================================
// SendTextMessage Tests
// =============================================================================

func TestWahaAdapter_SendTextMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("sends text message with mentions", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendText")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			assert.Equal(t, "group@g.us", payload["chatId"])
			assert.Equal(t, "Hello @user1 @user2", payload["text"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-text-123",
				"ack":       0,
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello @user1 @user2",
			Mentions:    []string{"user1@c.us", "user2@c.us"},
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-text-123", result.ID)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusInternalServerError, map[string]string{
				"error":   "Internal Server Error",
				"message": "WAHA service unavailable",
			})(w, r)
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send text message")
	})

	t.Run("handles session not connected error", func(t *testing.T) {
		// Now this test verifies the pre-send status check catches disconnected sessions
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Return disconnected status
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "disconnected-session",
					"status": "DISCONNECTED",
				})(w, r)
				return
			}
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "disconnected-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
	})
}

// =============================================================================
// SendVoice Tests
// =============================================================================

func TestWahaAdapter_SendVoice(t *testing.T) {
	ctx := context.Background()

	t.Run("sends voice message successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendVoice")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			assert.Equal(t, "group@g.us", payload["chatId"])
			assert.Contains(t, string(body), "audio")

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-voice-123",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendVoice(ctx, SendVoiceInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			AudioBase64: "SGVsbG8gV29ybGQ=",
			Mimetype:    "audio/ogg; codecs=opus",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-voice-123", result.ID)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusBadRequest, map[string]string{
				"message": "Invalid audio format",
			})(w, r)
		})

		_, err := adapter.SendVoice(ctx, SendVoiceInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			AudioBase64: "invalid-base64",
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send voice message")
	})
}

func TestWahaAdapter_SendVideo(t *testing.T) {
	ctx := context.Background()

	// Regressão: o send de vídeo DEVE mandar convert:true pro WAHA. Sem isso,
	// vídeo repassado (não-streamable) fica preso em ack=1 (aceito pelo servidor
	// do WhatsApp mas nunca entregue aos aparelhos).
	t.Run("sends video with convert:true so WhatsApp delivers it", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendVideo")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			assert.Equal(t, "group@g.us", payload["chatId"])
			assert.Equal(t, true, payload["convert"], "video send must request WAHA transcode (convert:true)")

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-video-123",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendVideo(ctx, SendVideoInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			MediaBase64: "SGVsbG8gV29ybGQ=",
			MimeType:    "video/mp4",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-video-123", result.ID)
	})
}

// =============================================================================
// SendImage Additional Tests
// =============================================================================

func TestWahaAdapter_SendImage_Advanced(t *testing.T) {
	ctx := context.Background()

	t.Run("sends image with base64 data", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendImage")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			// Should have base64 image data
			assert.NotEmpty(t, payload)

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-img-base64",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Caption:     "Image from base64",
			ImageBase64: "iVBORw0KGgoAAAANSUhEUg==",
			Mimetype:    "image/png",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-img-base64", result.ID)
	})

	t.Run("sends image with URL", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			// Should have URL
			assert.NotEmpty(t, payload)

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-img-url",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Caption:     "Image from URL",
			ImageURL:    "https://example.com/image.jpg",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-img-url", result.ID)
	})
}

// =============================================================================
// SendPoll Tests
// =============================================================================

func TestWahaAdapter_SendPoll(t *testing.T) {
	ctx := context.Background()

	t.Run("sends poll successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/sendPoll")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			assert.Equal(t, "group@g.us", payload["chatId"])
			poll, ok := payload["poll"].(map[string]interface{})
			require.True(t, ok, "payload should have a nested poll object")
			assert.Equal(t, "What is your favorite color?", poll["name"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-poll-123",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendPoll(ctx, SendPollInput{
			SessionName:     "test-session",
			ChatID:          "group@g.us",
			Question:        "What is your favorite color?",
			Options:         []string{"Red", "Blue", "Green"},
			SelectableCount: 1,
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-poll-123", result.ID)
	})

	t.Run("sends multi-select poll", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)

			// Verify multi-select is passed correctly
			poll, ok := payload["poll"].(map[string]interface{})
			require.True(t, ok, "payload should have a nested poll object")
			assert.Equal(t, true, poll["multipleAnswers"])

			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"id":        "msg-poll-multi",
				"timestamp": 1234567890,
			})(w, r)
		})

		result, err := adapter.SendPoll(ctx, SendPollInput{
			SessionName:     "test-session",
			ChatID:          "group@g.us",
			Question:        "Select your favorites",
			Options:         []string{"A", "B", "C", "D"},
			SelectableCount: 3,
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-poll-multi", result.ID)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Handle status check first
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			jsonResponse(t, http.StatusBadRequest, map[string]string{
				"message": "Too many options",
			})(w, r)
		})

		_, err := adapter.SendPoll(ctx, SendPollInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Question:    "Poll",
			Options:     []string{"A", "B"},
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send poll message")
	})
}

// =============================================================================
// GetGroupInviteCode Additional Tests
// =============================================================================

func TestWahaAdapter_GetGroupInviteCode_Advanced(t *testing.T) {
	ctx := context.Background()

	t.Run("handles empty invite code", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(t, http.StatusOK, map[string]interface{}{
				"inviteCode": "",
			})(w, r)
		})

		result, err := adapter.GetGroupInviteCode(ctx, "test-session", "group@g.us")

		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("handles API timeout", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := adapter.GetGroupInviteCode(ctx, "test-session", "group@g.us")

		assert.Error(t, err)
	})
}

// =============================================================================
// UpdateGroupSubject Tests
// =============================================================================

func TestWahaAdapter_UpdateGroupSubject(t *testing.T) {
	ctx := context.Background()

	t.Run("updates group subject successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/groups/")
			assert.Contains(t, r.URL.Path, "/subject")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, "New Group Name", payload["subject"])

			w.WriteHeader(http.StatusOK)
		})

		err := adapter.UpdateGroupSubject(ctx, UpdateGroupSubjectInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			Subject:     "New Group Name",
		})

		assert.NoError(t, err)
	})

	t.Run("returns error on permission denied", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusForbidden, map[string]string{
			"message": "Not admin of this group",
		}))

		err := adapter.UpdateGroupSubject(ctx, UpdateGroupSubjectInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			Subject:     "New Name",
		})

		assert.Error(t, err)
	})
}

// =============================================================================
// UpdateGroupDescription Tests
// =============================================================================

func TestWahaAdapter_UpdateGroupDescription(t *testing.T) {
	ctx := context.Background()

	t.Run("updates group description successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/description")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, "New group description", payload["description"])

			w.WriteHeader(http.StatusOK)
		})

		err := adapter.UpdateGroupDescription(ctx, UpdateGroupDescriptionInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			Description: "New group description",
		})

		assert.NoError(t, err)
	})
}

// =============================================================================
// UpdateGroupPicture Tests
// =============================================================================

func TestWahaAdapter_UpdateGroupPicture(t *testing.T) {
	ctx := context.Background()

	t.Run("updates group picture with base64", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/picture")

			body, _ := io.ReadAll(r.Body)
			assert.NotEmpty(t, body)

			w.WriteHeader(http.StatusOK)
		})

		err := adapter.UpdateGroupPicture(ctx, UpdateGroupPictureInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			ImageBase64: "iVBORw0KGgoAAAANSUhEUg==",
		})

		assert.NoError(t, err)
	})
}

// =============================================================================
// SetGroupAnnouncement Tests
// =============================================================================

func TestWahaAdapter_SetGroupAnnouncement(t *testing.T) {
	ctx := context.Background()

	t.Run("enables announcement mode", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/settings/security/messages-admin-only")

			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, true, payload["adminsOnly"])

			w.WriteHeader(http.StatusOK)
		})

		err := adapter.SetGroupAnnouncement(ctx, SetGroupAnnouncementInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			Announce:    true,
		})

		assert.NoError(t, err)
	})

	t.Run("disables announcement mode", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			assert.Equal(t, false, payload["adminsOnly"])

			w.WriteHeader(http.StatusOK)
		})

		err := adapter.SetGroupAnnouncement(ctx, SetGroupAnnouncementInput{
			SessionName: "test-session",
			GroupID:     "group@g.us",
			Announce:    false,
		})

		assert.NoError(t, err)
	})
}

// =============================================================================
// GetMessages (Reconciliation) Tests
// =============================================================================

func TestWahaAdapter_GetMessages(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieves messages successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/messages")

			// Check query parameters
			assert.Contains(t, r.URL.RawQuery, "limit=")

			jsonResponse(t, http.StatusOK, []map[string]interface{}{
				{
					"id":        "msg-1",
					"timestamp": 1234567890,
					"body":      "Hello",
					"fromMe":    false,
				},
				{
					"id":        "msg-2",
					"timestamp": 1234567891,
					"body":      "World",
					"fromMe":    true,
				},
			})(w, r)
		})

		result, err := adapter.GetMessages(ctx, GetMessagesInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Limit:       50,
		})

		require.NoError(t, err)
		assert.Len(t, result.Messages, 2)
	})

	t.Run("handles empty message list", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(t, http.StatusOK, []map[string]interface{}{})(w, r)
		})

		result, err := adapter.GetMessages(ctx, GetMessagesInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Limit:       50,
		})

		require.NoError(t, err)
		assert.Len(t, result.Messages, 0)
		assert.False(t, result.HasMore)
	})
}

// =============================================================================
// Webhook Manager Tests
// =============================================================================

func TestWahaAdapter_SetWebhook(t *testing.T) {
	ctx := context.Background()

	t.Run("sets webhook via session update", func(t *testing.T) {
		sessionUpdateCalled := false

		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// SetWebhook uses PUT /api/sessions/{session} (session update)
			if r.URL.Path == "/api/sessions/test-session" && r.Method == http.MethodPut {
				sessionUpdateCalled = true

				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)
				assert.Contains(t, bodyStr, "http://example.com/webhook")
				assert.Contains(t, bodyStr, `"webhooks"`)
				assert.Contains(t, bodyStr, `"key"`)

				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		err := adapter.SetWebhook(ctx, SetWebhookInput{
			SessionName: "test-session",
			URL:         "http://example.com/webhook",
			Events:      []string{"message", "message.ack"},
			HMACKey:     "secret",
		})

		assert.NoError(t, err)
		assert.True(t, sessionUpdateCalled, "Expected PUT /api/sessions/{session} to be called")
	})
}

// =============================================================================
// Session Management Additional Tests
// =============================================================================

func TestWahaAdapter_RestartSession(t *testing.T) {
	ctx := context.Background()

	t.Run("restarts session successfully", func(t *testing.T) {
		restartCalled := false

		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			// WAHA uses dedicated /restart endpoint (atomic operation)
			// per WAHA OpenAPI spec: POST /api/sessions/{session}/restart
			if strings.Contains(r.URL.Path, "/restart") && r.Method == http.MethodPost {
				restartCalled = true
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		err := adapter.RestartSession(ctx, "test-session")

		assert.NoError(t, err)
		assert.True(t, restartCalled, "Expected restart endpoint to be called")
	})

	t.Run("returns error on restart failure", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/restart") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		})

		err := adapter.RestartSession(ctx, "test-session")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to restart session")
	})
}

func TestWahaAdapter_LogoutSession(t *testing.T) {
	ctx := context.Background()

	t.Run("logs out session successfully", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/logout")
			w.WriteHeader(http.StatusOK)
		})

		err := adapter.LogoutSession(ctx, "test-session")

		assert.NoError(t, err)
	})
}

// =============================================================================
// Session Status Check Before Send Tests
// =============================================================================

func TestWahaAdapter_RequireWorkingSession(t *testing.T) {
	ctx := context.Background()

	t.Run("allows sending when session is WORKING", func(t *testing.T) {
		requestCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				// Return WORKING status (maps to SessionStatusConnected)
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			if strings.Contains(r.URL.Path, "/sendText") {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"id":        "msg-123",
					"timestamp": 1234567890,
				})(w, r)
				return
			}
		})

		result, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-123", result.ID)
		// Should have called status check first, then send
		assert.GreaterOrEqual(t, requestCount, 2)
	})

	t.Run("blocks sending when session is SCAN_QR_CODE", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "SCAN_QR_CODE",
				})(w, r)
				return
			}
			// SendText should NOT be called
			t.Fatal("SendText should not be called when session is not working")
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
		assert.Contains(t, err.Error(), "SCAN_QR")

		// Verify it's a ProviderError with correct code
		var providerErr *ProviderError
		if assert.ErrorAs(t, err, &providerErr) {
			assert.Equal(t, ErrCodeSessionNotConnected, providerErr.Code)
		}
	})

	t.Run("blocks sending when session is STARTING", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "STARTING",
				})(w, r)
				return
			}
			t.Fatal("Send should not be called when session is starting")
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
	})

	t.Run("blocks sending when session is DISCONNECTED", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "DISCONNECTED",
				})(w, r)
				return
			}
			t.Fatal("Send should not be called when session is disconnected")
		})

		_, err := adapter.SendImage(ctx, SendImageInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Caption:     "Image",
			ImageURL:    "https://example.com/image.jpg",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
	})

	t.Run("blocks sending when session is STOPPED", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "STOPPED",
				})(w, r)
				return
			}
			t.Fatal("Send should not be called when session is stopped")
		})

		_, err := adapter.SendVoice(ctx, SendVoiceInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			AudioBase64: "base64data",
			Mimetype:    "audio/ogg",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
	})

	t.Run("blocks sending when session is FAILED", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "FAILED",
				})(w, r)
				return
			}
			t.Fatal("Send should not be called when session has failed")
		})

		_, err := adapter.SendPoll(ctx, SendPollInput{
			SessionName:     "test-session",
			ChatID:          "group@g.us",
			Question:        "Poll?",
			Options:         []string{"A", "B"},
			SelectableCount: 1,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not working")
	})

	t.Run("returns error when status check fails", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusNotFound, map[string]interface{}{
					"message": "Session not found",
				})(w, r)
				return
			}
		})

		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "non-existent",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check session status")
	})

	t.Run("uses cached status for subsequent sends", func(t *testing.T) {
		statusCheckCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				statusCheckCount++
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
			if strings.Contains(r.URL.Path, "/send") {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"id":        "msg-123",
					"timestamp": 1234567890,
				})(w, r)
				return
			}
		})

		// First send - should check status
		_, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello 1",
		})
		require.NoError(t, err)

		// Second send - should use cached status (within TTL)
		_, err = adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello 2",
		})
		require.NoError(t, err)

		// Status should only be checked once due to caching
		assert.Equal(t, 1, statusCheckCount, "status should be cached for subsequent sends")
	})

	t.Run("allows CONNECTED status (same as WORKING)", func(t *testing.T) {
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "CONNECTED",
				})(w, r)
				return
			}
			if strings.Contains(r.URL.Path, "/sendText") {
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"id":        "msg-123",
					"timestamp": 1234567890,
				})(w, r)
				return
			}
		})

		result, err := adapter.SendText(ctx, SendTextInput{
			SessionName: "test-session",
			ChatID:      "group@g.us",
			Text:        "Hello",
		})

		require.NoError(t, err)
		assert.Equal(t, "msg-123", result.ID)
	})
}

// =============================================================================
// Per-Session Circuit Breaker Tests
// =============================================================================

func TestWahaAdapter_CircuitBreaker_PerSession(t *testing.T) {
	ctx := context.Background()

	t.Run("isolation: failure in session A does not block session B", func(t *testing.T) {
		callCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/sessions/") {
				sessionName := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
				if sessionName == "session-a" {
					// session-a always fails
					jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
						"message": "internal error",
					})(w, r)
					return
				}
				// session-b always succeeds
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "session-b",
					"status": "WORKING",
				})(w, r)
				return
			}
		})

		// Trip session-a's circuit breaker (3 failures)
		for i := 0; i < 3; i++ {
			_, err := adapter.GetSessionStatus(ctx, "session-a")
			require.Error(t, err)
		}

		// session-a should now be blocked by circuit breaker
		_, err := adapter.GetSessionStatus(ctx, "session-a")
		require.Error(t, err)
		var providerErr *ProviderError
		require.ErrorAs(t, err, &providerErr)
		assert.Equal(t, ErrCodeCircuitBreakerOpen, providerErr.Code)

		// session-b should still work fine
		info, err := adapter.GetSessionStatus(ctx, "session-b")
		require.NoError(t, err)
		assert.Equal(t, "session-b", info.Name)
	})

	t.Run("resets after successful request", func(t *testing.T) {
		failCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/sessions/") {
				failCount++
				if failCount <= 2 {
					// First 2 calls fail
					jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
						"message": "internal error",
					})(w, r)
					return
				}
				// After that, succeed
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
		})

		// 2 failures (below threshold of 3)
		for i := 0; i < 2; i++ {
			_, err := adapter.GetSessionStatus(ctx, "test-session")
			require.Error(t, err)
		}

		// Clear cache so next call hits the server
		adapter.ClearStatusCache("test-session")

		// 3rd call succeeds → resets failure counter
		info, err := adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, "test-session", info.Name)

		// Now fail 2 more times — should NOT trip breaker since counter was reset
		failCount = 0 // reset mock counter to make it fail again
		adapter.ClearStatusCache("test-session")
		_, err = adapter.GetSessionStatus(ctx, "test-session")
		require.Error(t, err) // failure 1

		adapter.ClearStatusCache("test-session")
		_, err = adapter.GetSessionStatus(ctx, "test-session")
		require.Error(t, err) // failure 2

		// Should still be allowed (only 2 failures since last reset)
		failCount = 100 // make mock succeed
		adapter.ClearStatusCache("test-session")
		info, err = adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, "test-session", info.Name)
	})

	t.Run("ClearCircuitBreaker resets the breaker for a session", func(t *testing.T) {
		callCount := 0
		adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/sessions/") {
				if callCount <= 3 {
					// First 3 calls fail to trip the breaker
					jsonResponse(t, http.StatusInternalServerError, map[string]interface{}{
						"message": "internal error",
					})(w, r)
					return
				}
				// After clear, succeed
				jsonResponse(t, http.StatusOK, map[string]interface{}{
					"name":   "test-session",
					"status": "WORKING",
				})(w, r)
				return
			}
		})

		// Trip the circuit breaker (3 failures)
		for i := 0; i < 3; i++ {
			_, err := adapter.GetSessionStatus(ctx, "test-session")
			require.Error(t, err)
		}

		// Should be blocked
		_, err := adapter.GetSessionStatus(ctx, "test-session")
		require.Error(t, err)
		var providerErr *ProviderError
		require.ErrorAs(t, err, &providerErr)
		assert.Equal(t, ErrCodeCircuitBreakerOpen, providerErr.Code)

		// Clear the circuit breaker
		adapter.ClearCircuitBreaker("test-session")
		adapter.ClearStatusCache("test-session")

		// Should be allowed again
		info, err := adapter.GetSessionStatus(ctx, "test-session")
		require.NoError(t, err)
		assert.Equal(t, "test-session", info.Name)
	})
}
