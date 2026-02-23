package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateDispatchLogRequest_ToMap(t *testing.T) {
	t.Parallel()

	t.Run("all nil returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{}
		m := r.ToMap()
		assert.Empty(t, m)
	})

	t.Run("status only", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{Status: ptr("sent")}
		m := r.ToMap()
		assert.Len(t, m, 1)
		assert.Equal(t, "sent", m["status"])
	})

	t.Run("both fields", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{
			Status: ptr("failed"),
			Error:  ptr("timeout connecting to WhatsApp"),
		}
		m := r.ToMap()
		assert.Len(t, m, 2)
		assert.Equal(t, "failed", m["status"])
		assert.Equal(t, "timeout connecting to WhatsApp", m["error"])
	})
}
