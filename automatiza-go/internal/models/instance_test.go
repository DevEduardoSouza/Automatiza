package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstance_IsConnected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"connected", InstanceStatusConnected, true},
		{"open", InstanceStatusOpen, true},
		{"disconnected", InstanceStatusDisconnected, false},
		{"pending", InstanceStatusPending, false},
		{"close", InstanceStatusClose, false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			i := &Instance{Status: tt.status}
			assert.Equal(t, tt.want, i.IsConnected())
		})
	}
}
