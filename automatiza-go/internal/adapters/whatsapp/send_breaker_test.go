package whatsapp

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newSendBreakerAdapter() *WahaAdapter {
	return &WahaAdapter{
		logger:        zap.NewNop(),
		sendBreakers:  make(map[string]*circuitBreaker),
		sendBreakerOn: true,
	}
}

var err420 = errors.New("failed: 2 UNKNOWN: server returned error 420")

func TestSendBreaker_OpensAfterConsecutive420s(t *testing.T) {
	a := newSendBreakerAdapter()
	for i := 0; i < sendBreakerThreshold; i++ {
		if err := a.blockIfChipRestricted("s1"); err != nil {
			t.Fatalf("blocked before threshold (attempt %d): %v", i, err)
		}
		a.noteSendOutcome("s1", err420)
	}
	err := a.blockIfChipRestricted("s1")
	if !IsChipRestricted(err) {
		t.Fatalf("expected ErrCodeChipRestricted after %d consecutive 420s, got %v", sendBreakerThreshold, err)
	}
	// Other sessions are unaffected.
	if err := a.blockIfChipRestricted("s2"); err != nil {
		t.Fatalf("unrelated session blocked: %v", err)
	}
}

func TestSendBreaker_SuccessResetsStreak(t *testing.T) {
	a := newSendBreakerAdapter()
	a.noteSendOutcome("s1", err420)
	a.noteSendOutcome("s1", err420)
	a.noteSendOutcome("s1", nil) // reset
	a.noteSendOutcome("s1", err420)
	a.noteSendOutcome("s1", err420)
	if err := a.blockIfChipRestricted("s1"); err != nil {
		t.Fatalf("blocked without %d CONSECUTIVE 420s: %v", sendBreakerThreshold, err)
	}
}

func TestSendBreaker_Non420ErrorsAreNeutral(t *testing.T) {
	a := newSendBreakerAdapter()
	a.noteSendOutcome("s1", err420)
	a.noteSendOutcome("s1", err420)
	a.noteSendOutcome("s1", errors.New("connection refused")) // neither counts nor resets
	a.noteSendOutcome("s1", err420)
	if err := a.blockIfChipRestricted("s1"); !IsChipRestricted(err) {
		t.Fatalf("expected open circuit (non-420 must not reset streak), got %v", err)
	}
}

func TestSendBreaker_HalfOpensAfterBackoff(t *testing.T) {
	a := newSendBreakerAdapter()
	for i := 0; i < sendBreakerThreshold; i++ {
		a.noteSendOutcome("s1", err420)
	}
	cb := a.getOrCreateSendBreaker("s1")
	cb.mu.Lock()
	cb.lastFailure = time.Now().Add(-sendBreakerBackoff - time.Second)
	cb.mu.Unlock()
	if err := a.blockIfChipRestricted("s1"); err != nil {
		t.Fatalf("expected half-open probe after backoff, got %v", err)
	}
}

func TestSendBreaker_KillSwitchDisables(t *testing.T) {
	a := newSendBreakerAdapter()
	a.sendBreakerOn = false
	for i := 0; i < sendBreakerThreshold*2; i++ {
		a.noteSendOutcome("s1", err420)
	}
	if err := a.blockIfChipRestricted("s1"); err != nil {
		t.Fatalf("kill-switch off must never block: %v", err)
	}
}
