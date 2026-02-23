package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScheduledJob_IsPending(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"pending", JobStatusPending, true},
		{"processing", JobStatusProcessing, false},
		{"completed", JobStatusCompleted, false},
		{"failed", JobStatusFailed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &ScheduledJob{Status: tt.status}
			assert.Equal(t, tt.want, j.IsPending())
		})
	}
}
