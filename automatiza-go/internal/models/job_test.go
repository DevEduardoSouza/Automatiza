package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStandaloneJob_CanRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		retryCount int
		maxRetries int
		want       bool
	}{
		{"retries remaining", 1, 3, true},
		{"at max", 3, 3, false},
		{"above max", 5, 3, false},
		{"both zero", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &StandaloneJob{RetryCount: tt.retryCount, MaxRetries: tt.maxRetries}
			assert.Equal(t, tt.want, j.CanRetry())
		})
	}
}

func TestStandaloneJob_NextRetryDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		retryCount int
		want       time.Duration
	}{
		{"first retry (2^0=1)", 0, 30 * time.Second},
		{"second retry (2^1=2)", 1, 60 * time.Second},
		{"third retry (2^2=4)", 2, 120 * time.Second},
		{"fourth retry (2^3=8)", 3, 240 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &StandaloneJob{RetryCount: tt.retryCount}
			assert.Equal(t, tt.want, j.NextRetryDelay())
		})
	}
}

func TestStandaloneJob_IsPoll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		actionType JobActionType
		payloadTyp string
		want       bool
	}{
		{"send_message with poll type", JobActionSendMessage, "poll", true},
		{"send_message without poll type", JobActionSendMessage, "", false},
		{"send_poll with poll type", JobActionSendPoll, "poll", false},
		{"other action type", JobActionUpdateDescription, "poll", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &StandaloneJob{
				ActionType: tt.actionType,
				Payload:    JobPayload{Type: tt.payloadTyp},
			}
			assert.Equal(t, tt.want, j.IsPoll())
		})
	}
}

func TestStandaloneJob_IsAdminOnlyAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		actionType JobActionType
		want       bool
	}{
		{"update_description is admin", JobActionUpdateDescription, true},
		{"close_groups is admin", JobActionCloseGroups, true},
		{"open_groups is admin", JobActionOpenGroups, true},
		{"change_group_name is admin", JobActionChangeGroupName, true},
		{"change_group_photo is admin", JobActionChangeGroupPhoto, true},
		{"send_message is not admin", JobActionSendMessage, false},
		{"send_poll is not admin", JobActionSendPoll, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &StandaloneJob{ActionType: tt.actionType}
			assert.Equal(t, tt.want, j.IsAdminOnlyAction())
		})
	}
}

func TestStandaloneJob_HasMedia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		media string
		image string
		want  bool
	}{
		{"both empty", "", "", false},
		{"media set", "base64data", "", true},
		{"image set", "", "http://img.png", true},
		{"both set", "base64data", "http://img.png", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := &StandaloneJob{
				Payload: JobPayload{Media: tt.media, Image: tt.image},
			}
			assert.Equal(t, tt.want, j.HasMedia())
		})
	}
}

func TestJobPayload_Scan(t *testing.T) {
	t.Parallel()

	t.Run("nil value resets payload", func(t *testing.T) {
		t.Parallel()
		p := &JobPayload{Message: "old"}
		err := p.Scan(nil)
		assert.NoError(t, err)
		assert.Equal(t, JobPayload{}, *p)
	})

	t.Run("valid bytes", func(t *testing.T) {
		t.Parallel()
		p := &JobPayload{}
		err := p.Scan([]byte(`{"message":"hello","groups":["g1"]}`))
		assert.NoError(t, err)
		assert.Equal(t, "hello", p.Message)
		assert.Equal(t, []string{"g1"}, p.Groups)
	})

	t.Run("valid string", func(t *testing.T) {
		t.Parallel()
		p := &JobPayload{}
		err := p.Scan(`{"question":"Q?","options":["A","B"]}`)
		assert.NoError(t, err)
		assert.Equal(t, "Q?", p.Question)
		assert.Equal(t, []string{"A", "B"}, p.Options)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		p := &JobPayload{}
		err := p.Scan([]byte(`{invalid`))
		assert.Error(t, err)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		t.Parallel()
		p := &JobPayload{}
		err := p.Scan(42)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected []byte or string")
	})
}

func TestJobPayload_Value(t *testing.T) {
	t.Parallel()

	t.Run("empty payload", func(t *testing.T) {
		t.Parallel()
		p := JobPayload{}
		val, err := p.Value()
		assert.NoError(t, err)
		assert.NotNil(t, val)
	})

	t.Run("populated payload", func(t *testing.T) {
		t.Parallel()
		p := JobPayload{
			Message: "test",
			Groups:  []string{"g1", "g2"},
		}
		val, err := p.Value()
		assert.NoError(t, err)

		bytes, ok := val.([]byte)
		assert.True(t, ok)
		assert.Contains(t, string(bytes), `"message":"test"`)
		assert.Contains(t, string(bytes), `"groups":["g1","g2"]`)
	})
}
