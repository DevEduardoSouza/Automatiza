package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIncomingMessage_GetTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		timestamp int64
		want      time.Time
	}{
		{"known timestamp", 1700000000, time.Unix(1700000000, 0)},
		{"zero", 0, time.Unix(0, 0)},
		{"negative", -1, time.Unix(-1, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := &IncomingMessage{Timestamp: tt.timestamp}
			assert.Equal(t, tt.want, m.GetTimestamp())
		})
	}
}

func TestTimeWindow_IsWithinWindow(t *testing.T) {
	t.Parallel()

	// Use a fixed time for deterministic tests: Wednesday, 2024-01-10 at 14:30 UTC
	fixedTime := time.Date(2024, 1, 10, 14, 30, 0, 0, time.UTC)

	t.Run("no restriction when start and end empty", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "", EndTime: ""}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("only start empty means no restriction", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "", EndTime: "18:00"}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("only end empty means no restriction", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "08:00", EndTime: ""}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("within window", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "08:00", EndTime: "18:00", Timezone: "UTC"}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("before window", func(t *testing.T) {
		t.Parallel()
		earlyTime := time.Date(2024, 1, 10, 6, 0, 0, 0, time.UTC)
		tw := &TimeWindow{StartTime: "08:00", EndTime: "18:00", Timezone: "UTC"}
		assert.False(t, tw.IsWithinWindow(earlyTime))
	})

	t.Run("after window", func(t *testing.T) {
		t.Parallel()
		lateTime := time.Date(2024, 1, 10, 20, 0, 0, 0, time.UTC)
		tw := &TimeWindow{StartTime: "08:00", EndTime: "18:00", Timezone: "UTC"}
		assert.False(t, tw.IsWithinWindow(lateTime))
	})

	t.Run("wrong day", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{
			StartTime: "08:00", EndTime: "18:00",
			Days:     []string{"Mon", "Tue"},
			Timezone: "UTC",
		}
		// fixedTime is Wednesday
		assert.False(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("correct day by abbreviation", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{
			StartTime: "08:00", EndTime: "18:00",
			Days:     []string{"Wed"},
			Timezone: "UTC",
		}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("correct day by full name", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{
			StartTime: "08:00", EndTime: "18:00",
			Days:     []string{"Wednesday"},
			Timezone: "UTC",
		}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("empty days list means no day restriction", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{
			StartTime: "08:00", EndTime: "18:00",
			Days:     []string{},
			Timezone: "UTC",
		}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	// Midnight crossing
	t.Run("midnight crossing/within after-midnight portion", func(t *testing.T) {
		t.Parallel()
		earlyMorning := time.Date(2024, 1, 10, 1, 0, 0, 0, time.UTC)
		tw := &TimeWindow{StartTime: "22:00", EndTime: "06:00", Timezone: "UTC"}
		assert.True(t, tw.IsWithinWindow(earlyMorning))
	})

	t.Run("midnight crossing/within before-midnight portion", func(t *testing.T) {
		t.Parallel()
		lateNight := time.Date(2024, 1, 10, 23, 0, 0, 0, time.UTC)
		tw := &TimeWindow{StartTime: "22:00", EndTime: "06:00", Timezone: "UTC"}
		assert.True(t, tw.IsWithinWindow(lateNight))
	})

	t.Run("midnight crossing/outside gap", func(t *testing.T) {
		t.Parallel()
		midday := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
		tw := &TimeWindow{StartTime: "22:00", EndTime: "06:00", Timezone: "UTC"}
		assert.False(t, tw.IsWithinWindow(midday))
	})

	// Timezone handling
	t.Run("valid timezone America/Sao_Paulo", func(t *testing.T) {
		t.Parallel()
		// 14:30 UTC = 11:30 BRT (UTC-3)
		tw := &TimeWindow{StartTime: "10:00", EndTime: "12:00", Timezone: "America/Sao_Paulo"}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	t.Run("invalid timezone falls back to local", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "00:00", EndTime: "23:59", Timezone: "Invalid/Zone"}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})

	// Invalid time format
	t.Run("invalid time format returns true", func(t *testing.T) {
		t.Parallel()
		tw := &TimeWindow{StartTime: "not-a-time", EndTime: "also-bad", Timezone: "UTC"}
		assert.True(t, tw.IsWithinWindow(fixedTime))
	})
}
