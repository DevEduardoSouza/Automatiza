package whatsapp

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newPacerTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRedisUploadPacer_Disabled(t *testing.T) {
	if NewRedisUploadPacer(nil, time.Second, zap.NewNop()) != nil {
		t.Fatal("without redis the pacer must be nil (disabled)")
	}
	rdb := newPacerTestRedis(t)
	if NewRedisUploadPacer(rdb, 0, zap.NewNop()) != nil {
		t.Fatal("spacing<=0 must disable the pacer")
	}

	var p *RedisUploadPacer // nil receiver
	if d := p.Reserve(context.Background(), "s1"); d != 0 {
		t.Fatalf("nil pacer must return 0, got %v", d)
	}
}

func TestRedisUploadPacer_SpacesConsecutiveUploads(t *testing.T) {
	rdb := newPacerTestRedis(t)
	p := NewRedisUploadPacer(rdb, time.Second, zap.NewNop())
	ctx := context.Background()

	d1 := p.Reserve(ctx, "session-A")
	d2 := p.Reserve(ctx, "session-A")
	d3 := p.Reserve(ctx, "session-A")

	if d1 != 0 {
		t.Fatalf("first upload must not be delayed, got %v", d1)
	}
	// 2nd and 3rd are pushed ~1 and ~2 spacings out (margin for the clock).
	if d2 < 800*time.Millisecond || d2 > time.Second {
		t.Fatalf("2nd upload should be ~1s, got %v", d2)
	}
	if d3 < 1800*time.Millisecond || d3 > 2*time.Second {
		t.Fatalf("3rd upload should be ~2s, got %v", d3)
	}
}

func TestRedisUploadPacer_IndependentPerSession(t *testing.T) {
	rdb := newPacerTestRedis(t)
	p := NewRedisUploadPacer(rdb, time.Second, zap.NewNop())
	ctx := context.Background()

	_ = p.Reserve(ctx, "session-A")
	_ = p.Reserve(ctx, "session-A")

	// A different session starts fresh — it does not inherit A's slot.
	if d := p.Reserve(ctx, "session-B"); d != 0 {
		t.Fatalf("a different session must start without delay, got %v", d)
	}

	// Empty session => 0.
	if d := p.Reserve(ctx, ""); d != 0 {
		t.Fatalf("empty session must return 0, got %v", d)
	}
}

func TestMediaPaceMsFromEnv(t *testing.T) {
	cases := map[string]int{"": 0, "0": 0, "-5": 0, "abc": 0, "500": 500}
	for in, want := range cases {
		t.Setenv("MEDIA_PACE_MS", in)
		if got := MediaPaceMsFromEnv(); got != want {
			t.Fatalf("MEDIA_PACE_MS=%q => %d, want %d", in, got, want)
		}
	}
}

// recordingPacer is a test UploadPacer that records the session it was asked to
// pace and returns a fixed (small) delay, so SendImage wiring can be asserted
// without depending on Redis or real wall-clock spacing.
type recordingPacer struct {
	gotSession string
	calls      int
	delay      time.Duration
}

func (r *recordingPacer) Reserve(_ context.Context, session string) time.Duration {
	r.calls++
	r.gotSession = session
	return r.delay
}

// TestWahaAdapter_SendImage_InvokesUploadPacer verifies SendImage consults the
// configured pacer with the send's session before uploading.
func TestWahaAdapter_SendImage_InvokesUploadPacer(t *testing.T) {
	ctx := context.Background()
	rec := &recordingPacer{delay: 5 * time.Millisecond}

	adapter, _ := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/sessions/") && r.Method == http.MethodGet {
			jsonResponse(t, http.StatusOK, map[string]interface{}{"name": "s", "status": "WORKING"})(w, r)
			return
		}
		jsonResponse(t, http.StatusOK, map[string]interface{}{"id": "msg-ok"})(w, r)
	})
	adapter.SetUploadPacer(rec)

	_, err := adapter.SendImage(ctx, SendImageInput{
		SessionName: "session-xyz",
		ChatID:      "group@g.us",
		Caption:     "promo",
		ImageBase64: "base64data",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, rec.calls, "SendImage should consult the pacer exactly once")
	assert.Equal(t, "session-xyz", rec.gotSession, "pacer must be keyed by the send's session")
}
