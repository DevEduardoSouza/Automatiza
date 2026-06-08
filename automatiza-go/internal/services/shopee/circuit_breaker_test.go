package shopee

import (
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := newCircuitBreaker(3, time.Minute)

	// Below threshold: still allowed.
	for i := 0; i < 2; i++ {
		cb.recordFailure()
		if !cb.canRequest() {
			t.Fatalf("breaker opened too early after %d failures", i+1)
		}
	}

	// Hitting the threshold opens the breaker.
	cb.recordFailure()
	if cb.canRequest() {
		t.Fatal("breaker should be open after reaching threshold")
	}
}

func TestCircuitBreaker_HalfOpensAfterBackoff(t *testing.T) {
	cb := newCircuitBreaker(2, 10*time.Millisecond)

	cb.recordFailure()
	cb.recordFailure()
	if cb.canRequest() {
		t.Fatal("breaker should be open")
	}

	time.Sleep(15 * time.Millisecond)
	if !cb.canRequest() {
		t.Fatal("breaker should half-open (allow a probe) after backoff elapses")
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := newCircuitBreaker(3, time.Minute)

	cb.recordFailure()
	cb.recordFailure()
	cb.recordSuccess() // reset before reaching threshold

	// Two more failures should not open it (counter was reset).
	cb.recordFailure()
	cb.recordFailure()
	if !cb.canRequest() {
		t.Fatal("breaker should remain closed; success should have reset the failure counter")
	}
}
