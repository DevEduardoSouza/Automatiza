package whatsapp

import (
	"context"
	"encoding/json"
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
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Identificadores SINTÉTICOS, declaradamente não reais.
const (
	trackPhone = "5511000000000"
	trackLID   = "999000111222"
)

// =============================================================================
// Lista canônica de eventos
// =============================================================================

func TestDefaultWebhookEvents(t *testing.T) {
	events := DefaultWebhookEvents()

	// O evento novo aparece exatamente UMA vez.
	count := 0
	for _, e := range events {
		if e == "group.v2.participants" {
			count++
		}
	}
	assert.Equal(t, 1, count, "group.v2.participants deve aparecer uma única vez")

	// Legados permanecem durante a transição.
	assert.Contains(t, events, "group.join")
	assert.Contains(t, events, "group.leave")

	// message.any é o superset com fromMe; "message" junto duplicaria incoming.
	assert.Contains(t, events, "message.any")
	assert.NotContains(t, events, "message")

	assert.Contains(t, events, "message.ack")
	assert.Contains(t, events, "session.status")
}

// A função devolve uma CÓPIA: um caller que mutar o slice não pode envenenar
// os demais (o adapter e o admin compartilham a mesma fonte).
func TestDefaultWebhookEvents_ReturnsCopy(t *testing.T) {
	first := DefaultWebhookEvents()
	first[0] = "mutado"
	assert.Equal(t, "message.any", DefaultWebhookEvents()[0])
}

func TestWahaAdapter_CreateSessionUsesCanonicalEvents(t *testing.T) {
	var body string
	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/sessions") {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		jsonResponse(t, http.StatusCreated, map[string]interface{}{
			"name": "test-session", "status": "STARTING",
		})(w, r)
	})

	_, err := adapter.CreateSession(context.Background(), CreateSessionInput{
		SessionName: "test-session",
		WebhookURL:  "http://example.com/webhook",
	})
	require.NoError(t, err)

	for _, e := range DefaultWebhookEvents() {
		assert.Contains(t, body, e, "default de CreateSession deve conter %s", e)
	}
}

// =============================================================================
// SetWebhook: HMAC não pode sumir num update
// =============================================================================

func TestWahaAdapter_SetWebhookFallsBackToAdapterHMAC(t *testing.T) {
	var body string
	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Caller NÃO informa HMACKey — o segredo do adapter deve ser reaplicado.
	err := adapter.SetWebhook(context.Background(), SetWebhookInput{
		SessionName: "test-session",
		URL:         "http://example.com/webhook",
		Events:      DefaultWebhookEvents(),
	})
	require.NoError(t, err)

	// newTestAdapter configura WebhookSecret: "test-secret".
	assert.Contains(t, body, `"test-secret"`,
		"sem isso o update deixaria a sessão enviando webhooks sem assinatura")
	assert.Contains(t, body, "group.v2.participants")
}

func TestWahaAdapter_SetWebhookKeepsExplicitHMAC(t *testing.T) {
	var body string
	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		w.WriteHeader(http.StatusOK)
	})

	require.NoError(t, adapter.SetWebhook(context.Background(), SetWebhookInput{
		SessionName: "test-session",
		URL:         "http://example.com/webhook",
		Events:      []string{"message.any"},
		HMACKey:     "chave-do-caller",
	}))

	assert.Contains(t, body, "chave-do-caller")
	assert.NotContains(t, body, "test-secret")
}

// =============================================================================
// GetGroupParticipantsForTracking: uma chamada, zero /lids
// =============================================================================

func TestGetGroupParticipantsForTracking_NoLidLookup(t *testing.T) {
	var participantsCalls, lidsCalls int32

	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/lids") {
			atomic.AddInt32(&lidsCalls, 1)
			jsonResponse(t, http.StatusOK, []map[string]string{})(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/participants/v2") {
			atomic.AddInt32(&participantsCalls, 1)
			jsonResponse(t, http.StatusOK, []map[string]interface{}{
				// @lid SEM pn: é justamente o caso que dispararia /lids no
				// método público.
				{"id": trackLID + "@lid", "role": "participant"},
				{"id": trackPhone + "@c.us", "pn": trackPhone + "@c.us", "role": "admin"},
			})(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	result, err := adapter.GetGroupParticipantsForTracking(context.Background(), "test-session", "group@g.us")
	require.NoError(t, err)

	require.Len(t, result, 2)
	assert.Equal(t, trackLID+"@lid", result[0].ID)
	assert.Empty(t, result[0].PN)
	assert.Equal(t, "participant", result[0].Role)
	assert.Equal(t, "admin", result[1].Role, "o enum role exato é preservado")

	assert.Equal(t, int32(1), atomic.LoadInt32(&participantsCalls), "exatamente UMA chamada WAHA por grupo")
	assert.Equal(t, int32(0), atomic.LoadInt32(&lidsCalls), "a reconciliação NÃO resolve LID")
}

func TestGetGroupParticipantsForTracking_PreservesLeftRole(t *testing.T) {
	adapter, _ := newTestAdapter(t, jsonResponse(t, http.StatusOK, []map[string]interface{}{
		{"id": trackPhone + "@c.us", "role": "left"},
	}))

	result, err := adapter.GetGroupParticipantsForTracking(context.Background(), "test-session", "group@g.us")
	require.NoError(t, err)
	require.Len(t, result, 1)
	// ParticipantInfo só expõe IsAdmin/IsSuperAdmin: "left" seria indistinguível
	// de "participant". Por isso o tracking usa o DTO cru.
	assert.Equal(t, "left", result[0].Role)
}

// =============================================================================
// Privacidade: nenhum id/pn em log
// =============================================================================

func TestGetGroupParticipants_DoesNotLogRawBody(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	server := httptest.NewServer(jsonResponse(t, http.StatusOK, []map[string]interface{}{
		{"id": trackPhone + "@c.us", "pn": trackPhone + "@c.us", "role": "participant"},
	}))
	t.Cleanup(server.Close)

	adapter, err := NewWahaAdapter(WahaAdapterConfig{
		APIURL:      server.URL,
		APIKey:      "test-api-key",
		HTTPTimeout: 5 * time.Second,
	}, logger)
	require.NoError(t, err)

	_, err = adapter.GetGroupParticipants(context.Background(), "test-session", "group@g.us")
	require.NoError(t, err)

	for _, entry := range logs.All() {
		serialized, mErr := json.Marshal(entry.ContextMap())
		require.NoError(t, mErr)
		line := entry.Message + string(serialized)

		assert.NotContains(t, line, trackPhone, "telefone não pode aparecer em log")
		assert.NotContains(t, line, `"pn"`, "corpo cru não pode ser logado")
		assert.NotContains(t, line, "raw response")
	}
}

// =============================================================================
// Mapper: @lid não é telefone
// =============================================================================

func TestParticipantInfoFromWaha_LIDWithoutPNHasNoPhone(t *testing.T) {
	m := NewMapper()

	// @lid sem pn: aplicar ExtractPhoneFromID colocaria os dígitos do LID em
	// PhoneNumber e poderia fabricar um DDD brasileiro inexistente.
	info := m.ParticipantInfoFromWaha(&wahaParticipant{ID: trackLID + "@lid"})
	assert.Empty(t, info.PhoneNumber)
	assert.Equal(t, trackLID+"@lid", info.ID)

	// @lid COM pn resolvido: o telefone vem do pn.
	withPN := m.ParticipantInfoFromWaha(&wahaParticipant{
		ID:          trackLID + "@lid",
		PhoneNumber: trackPhone + "@c.us",
	})
	assert.Equal(t, trackPhone, withPN.PhoneNumber)

	// @c.us sem pn: o próprio id É o telefone.
	cus := m.ParticipantInfoFromWaha(&wahaParticipant{ID: trackPhone + "@c.us"})
	assert.Equal(t, trackPhone, cus.PhoneNumber)
}
