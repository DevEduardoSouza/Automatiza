package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAutomationRun_IsLocked(t *testing.T) {
	t.Parallel()

	t.Run("nil LockUntil returns false", func(t *testing.T) {
		t.Parallel()
		r := &AutomationRun{LockUntil: nil}
		assert.False(t, r.IsLocked())
	})

	t.Run("future LockUntil returns true", func(t *testing.T) {
		t.Parallel()
		future := time.Now().Add(1 * time.Hour)
		r := &AutomationRun{LockUntil: &future}
		assert.True(t, r.IsLocked())
	})

	t.Run("past LockUntil returns false", func(t *testing.T) {
		t.Parallel()
		past := time.Now().Add(-1 * time.Hour)
		r := &AutomationRun{LockUntil: &past}
		assert.False(t, r.IsLocked())
	})

	t.Run("epoch (zero time) returns false", func(t *testing.T) {
		t.Parallel()
		epoch := time.Unix(0, 0)
		r := &AutomationRun{LockUntil: &epoch}
		assert.False(t, r.IsLocked())
	})
}
