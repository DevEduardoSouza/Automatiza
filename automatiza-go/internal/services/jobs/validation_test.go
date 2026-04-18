package jobs

import (
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ValidateGroups Tests
// =============================================================================

func TestValidateGroups(t *testing.T) {
	tests := []struct {
		name    string
		groups  []string
		wantErr bool
	}{
		{
			name:    "valid single group",
			groups:  []string{"group1@g.us"},
			wantErr: false,
		},
		{
			name:    "valid multiple groups",
			groups:  []string{"group1@g.us", "group2@g.us", "group3@g.us"},
			wantErr: false,
		},
		{
			name:    "empty groups slice",
			groups:  []string{},
			wantErr: true,
		},
		{
			name:    "nil groups slice",
			groups:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGroups(tt.groups)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "pelo menos um grupo")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ValidatePollOptions Tests
// =============================================================================

func TestValidatePollOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     []string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid minimum options (2)",
			options: []string{"Opcao 1", "Opcao 2"},
			wantErr: false,
		},
		{
			name:    "valid middle range options (6)",
			options: []string{"A", "B", "C", "D", "E", "F"},
			wantErr: false,
		},
		{
			name:    "valid maximum options (12)",
			options: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
			wantErr: false,
		},
		{
			name:        "too few options (1)",
			options:     []string{"Opcao unica"},
			wantErr:     true,
			errContains: "mínimo 2",
		},
		{
			name:        "empty options",
			options:     []string{},
			wantErr:     true,
			errContains: "mínimo 2",
		},
		{
			name:        "nil options",
			options:     nil,
			wantErr:     true,
			errContains: "mínimo 2",
		},
		{
			name:        "too many options (13)",
			options:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"},
			wantErr:     true,
			errContains: "máximo 12",
		},
		{
			name:        "too many options (15)",
			options:     []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o"},
			wantErr:     true,
			errContains: "máximo 12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePollOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ValidateSelectableCount Tests
// =============================================================================

func TestValidateSelectableCount(t *testing.T) {
	tests := []struct {
		name            string
		selectableCount int
		optionsCount    int
		wantErr         bool
		errContains     string
	}{
		{
			name:            "valid single selectable",
			selectableCount: 1,
			optionsCount:    3,
			wantErr:         false,
		},
		{
			name:            "valid multiple selectable",
			selectableCount: 2,
			optionsCount:    4,
			wantErr:         false,
		},
		{
			name:            "valid all selectable",
			selectableCount: 5,
			optionsCount:    5,
			wantErr:         false,
		},
		{
			name:            "zero selectable",
			selectableCount: 0,
			optionsCount:    3,
			wantErr:         true,
			errContains:     "pelo menos 1",
		},
		{
			name:            "negative selectable",
			selectableCount: -1,
			optionsCount:    3,
			wantErr:         true,
			errContains:     "pelo menos 1",
		},
		{
			name:            "selectable greater than options",
			selectableCount: 5,
			optionsCount:    3,
			wantErr:         true,
			errContains:     "maior que",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSelectableCount(tt.selectableCount, tt.optionsCount)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ValidateMessage Tests
// =============================================================================

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{
			name:    "valid message",
			message: "Hello, World!",
			wantErr: false,
		},
		{
			name:    "valid message with newlines",
			message: "Line 1\nLine 2\nLine 3",
			wantErr: false,
		},
		{
			name:    "valid message with emojis",
			message: "Hello! :)",
			wantErr: false,
		},
		{
			name:    "valid message with unicode",
			message: "Ola! Voce esta bem?",
			wantErr: false,
		},
		{
			name:    "empty message",
			message: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.message)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "vazia")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ValidateAdminRequired Tests
// =============================================================================

func TestValidateAdminRequired(t *testing.T) {
	tests := []struct {
		name       string
		actionType models.JobActionType
		isAdmin    bool
		wantErr    bool
	}{
		// Non-admin actions - should pass regardless of admin status
		{
			name:       "send_message as non-admin",
			actionType: models.JobActionSendMessage,
			isAdmin:    false,
			wantErr:    false,
		},
		{
			name:       "send_message as admin",
			actionType: models.JobActionSendMessage,
			isAdmin:    true,
			wantErr:    false,
		},
		{
			name:       "send_poll as non-admin",
			actionType: models.JobActionSendPoll,
			isAdmin:    false,
			wantErr:    false,
		},

		// Admin-only actions as admin - should pass
		{
			name:       "update_description as admin",
			actionType: models.JobActionUpdateDescription,
			isAdmin:    true,
			wantErr:    false,
		},
		{
			name:       "close_groups as admin",
			actionType: models.JobActionCloseGroups,
			isAdmin:    true,
			wantErr:    false,
		},
		{
			name:       "open_groups as admin",
			actionType: models.JobActionOpenGroups,
			isAdmin:    true,
			wantErr:    false,
		},
		{
			name:       "change_group_name as admin",
			actionType: models.JobActionChangeGroupName,
			isAdmin:    true,
			wantErr:    false,
		},
		{
			name:       "change_group_photo as admin",
			actionType: models.JobActionChangeGroupPhoto,
			isAdmin:    true,
			wantErr:    false,
		},

		// Admin-only actions as non-admin - should fail
		{
			name:       "update_description as non-admin",
			actionType: models.JobActionUpdateDescription,
			isAdmin:    false,
			wantErr:    true,
		},
		{
			name:       "close_groups as non-admin",
			actionType: models.JobActionCloseGroups,
			isAdmin:    false,
			wantErr:    true,
		},
		{
			name:       "open_groups as non-admin",
			actionType: models.JobActionOpenGroups,
			isAdmin:    false,
			wantErr:    true,
		},
		{
			name:       "change_group_name as non-admin",
			actionType: models.JobActionChangeGroupName,
			isAdmin:    false,
			wantErr:    true,
		},
		{
			name:       "change_group_photo as non-admin",
			actionType: models.JobActionChangeGroupPhoto,
			isAdmin:    false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.StandaloneJob{
				ActionType: tt.actionType,
				Payload: models.JobPayload{
					Groups: []string{"group1@g.us"},
				},
			}

			err := ValidateAdminRequired(job, tt.isAdmin)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "admin")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// ValidateGroupOpen Tests
// =============================================================================

func TestValidateGroupOpen(t *testing.T) {
	tests := []struct {
		name     string
		isClosed bool
		isAdmin  bool
		wantErr  bool
	}{
		{
			name:     "open group as non-admin",
			isClosed: false,
			isAdmin:  false,
			wantErr:  false,
		},
		{
			name:     "open group as admin",
			isClosed: false,
			isAdmin:  true,
			wantErr:  false,
		},
		{
			name:     "closed group as admin",
			isClosed: true,
			isAdmin:  true,
			wantErr:  false,
		},
		{
			name:     "closed group as non-admin",
			isClosed: true,
			isAdmin:  false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGroupOpen(tt.isClosed, tt.isAdmin)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "fechados")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestValidatePollOptions_BoundaryValues(t *testing.T) {
	// Test exactly at boundaries
	t.Run("exactly 2 options", func(t *testing.T) {
		err := ValidatePollOptions([]string{"A", "B"})
		assert.NoError(t, err)
	})

	t.Run("exactly 12 options", func(t *testing.T) {
		options := make([]string, 12)
		for i := range options {
			options[i] = "Option"
		}
		err := ValidatePollOptions(options)
		assert.NoError(t, err)
	})
}

func TestValidateSelectableCount_BoundaryValues(t *testing.T) {
	// Test exactly at boundaries
	t.Run("selectable equals options", func(t *testing.T) {
		err := ValidateSelectableCount(3, 3)
		assert.NoError(t, err)
	})

	t.Run("selectable is 1", func(t *testing.T) {
		err := ValidateSelectableCount(1, 10)
		assert.NoError(t, err)
	})
}

// =============================================================================
// StandaloneJob Method Tests
// =============================================================================

func TestStandaloneJob_IsPoll(t *testing.T) {
	tests := []struct {
		name        string
		actionType  models.JobActionType
		payloadType string
		wantPoll    bool
	}{
		{
			name:        "send_message with poll type",
			actionType:  models.JobActionSendMessage,
			payloadType: "poll",
			wantPoll:    true,
		},
		{
			name:        "send_message without type",
			actionType:  models.JobActionSendMessage,
			payloadType: "",
			wantPoll:    false,
		},
		{
			name:        "send_message with text type",
			actionType:  models.JobActionSendMessage,
			payloadType: "text",
			wantPoll:    false,
		},
		{
			name:        "send_poll action",
			actionType:  models.JobActionSendPoll,
			payloadType: "",
			wantPoll:    false, // Different action type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.StandaloneJob{
				ActionType: tt.actionType,
				Payload: models.JobPayload{
					Type: tt.payloadType,
				},
			}
			assert.Equal(t, tt.wantPoll, job.IsPoll())
		})
	}
}

func TestStandaloneJob_IsAdminOnlyAction(t *testing.T) {
	adminOnlyActions := []models.JobActionType{
		models.JobActionUpdateDescription,
		models.JobActionCloseGroups,
		models.JobActionOpenGroups,
		models.JobActionChangeGroupName,
		models.JobActionChangeGroupPhoto,
	}

	nonAdminActions := []models.JobActionType{
		models.JobActionSendMessage,
		models.JobActionSendPoll,
	}

	for _, action := range adminOnlyActions {
		t.Run(string(action)+"_is_admin_only", func(t *testing.T) {
			job := &models.StandaloneJob{ActionType: action}
			assert.True(t, job.IsAdminOnlyAction())
		})
	}

	for _, action := range nonAdminActions {
		t.Run(string(action)+"_is_not_admin_only", func(t *testing.T) {
			job := &models.StandaloneJob{ActionType: action}
			assert.False(t, job.IsAdminOnlyAction())
		})
	}
}

func TestStandaloneJob_HasMedia(t *testing.T) {
	tests := []struct {
		name     string
		media    string
		image    string
		hasMedia bool
	}{
		{
			name:     "has media",
			media:    "base64-data",
			image:    "",
			hasMedia: true,
		},
		{
			name:     "has image",
			media:    "",
			image:    "base64-image",
			hasMedia: true,
		},
		{
			name:     "has both",
			media:    "base64-data",
			image:    "base64-image",
			hasMedia: true,
		},
		{
			name:     "no media",
			media:    "",
			image:    "",
			hasMedia: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.StandaloneJob{
				Payload: models.JobPayload{
					Media: tt.media,
					Image: tt.image,
				},
			}
			assert.Equal(t, tt.hasMedia, job.HasMedia())
		})
	}
}
