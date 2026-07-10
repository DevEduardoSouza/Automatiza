package humanize

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
)

// fakeCtl implements the (unexported) presenceCtl interface and records calls.
type fakeCtl struct {
	mu       sync.Mutex
	calls    []string
	startErr error
}

func (f *fakeCtl) rec(s string) { f.mu.Lock(); f.calls = append(f.calls, s); f.mu.Unlock() }
func (f *fakeCtl) StartTyping(_ context.Context, _ whatsapp.TypingInput) error {
	f.rec("start")
	return f.startErr
}
func (f *fakeCtl) StopTyping(_ context.Context, _ whatsapp.TypingInput) error {
	f.rec("stop")
	return nil
}
func (f *fakeCtl) SendSeen(_ context.Context, _ whatsapp.SendSeenInput) error {
	f.rec("seen")
	return nil
}
func (f *fakeCtl) SetPresence(_ context.Context, _ whatsapp.SetPresenceInput) error {
	f.rec("presence")
	return nil
}
func (f *fakeCtl) has(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == s {
			return true
		}
	}
	return false
}

func fastCfg() Config {
	c := DefaultConfig()
	c.MinTyping = time.Millisecond
	c.MaxTyping = 2 * time.Millisecond
	c.SeenProbability = 1
	c.TypingProbability = 1
	return c
}

func TestBeforeSendDisabledNoop(t *testing.T) {
	ctl := &fakeCtl{}
	h := New(ctl, nil, nil)
	cfg := fastCfg()
	cfg.Enabled = false
	h.BeforeSend(context.Background(), Plan{Session: "s", ChatID: "g", Text: "hi", Cfg: cfg})()
	time.Sleep(30 * time.Millisecond)
	ctl.mu.Lock()
	n := len(ctl.calls)
	ctl.mu.Unlock()
	if n != 0 {
		t.Fatalf("disabled config should make no provider calls, got %v", ctl.calls)
	}
}

func TestBeforeSendChoreography(t *testing.T) {
	ctl := &fakeCtl{}
	h := New(ctl, nil, nil)
	stop := h.BeforeSend(context.Background(), Plan{Session: "s", ChatID: "g", Text: "hello world", Cfg: fastCfg()})
	stop()
	time.Sleep(60 * time.Millisecond) // let fire-and-forget seen/presence run
	for _, want := range []string{"seen", "presence", "start", "stop"} {
		if !ctl.has(want) {
			t.Errorf("expected %q to have been called; calls=%v", want, ctl.calls)
		}
	}
}

func TestBeforeSendStartTypingErrorDoesNotBlock(t *testing.T) {
	ctl := &fakeCtl{startErr: errors.New("boom")}
	h := New(ctl, nil, nil)
	done := make(chan struct{})
	go func() {
		h.BeforeSend(context.Background(), Plan{Session: "s", ChatID: "g", Text: "x", Cfg: fastCfg()})()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BeforeSend blocked despite a StartTyping error")
	}
}

func TestBeforeSendNilHumanizerNoop(t *testing.T) {
	var h *Humanizer // nil
	// Must not panic and must return a usable no-op cleanup.
	h.BeforeSend(context.Background(), Plan{Session: "s", ChatID: "g", Text: "x", Cfg: fastCfg()})()
}

func TestBeforeSendTypingProbabilityZeroSkipsTyping(t *testing.T) {
	ctl := &fakeCtl{}
	h := New(ctl, nil, nil)
	cfg := fastCfg()
	cfg.TypingProbability = 0 // never type
	cfg.SeenProbability = 0   // and never seen, to isolate typing
	cfg.PresenceEnabled = false
	h.BeforeSend(context.Background(), Plan{Session: "s", ChatID: "g", Text: "x", Cfg: cfg})()
	time.Sleep(20 * time.Millisecond)
	if ctl.has("start") {
		t.Error("StartTyping should not fire when TypingProbability=0")
	}
}
