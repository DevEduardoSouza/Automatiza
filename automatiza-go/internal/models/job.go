package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StandaloneJobStatus represents the execution status of a standalone job
// (distinct from ScheduledJob which is tied to automations)
type StandaloneJobStatus string

const (
	StandaloneJobStatusPending   StandaloneJobStatus = "pending"
	StandaloneJobStatusRunning   StandaloneJobStatus = "running"
	StandaloneJobStatusDone      StandaloneJobStatus = "done"
	StandaloneJobStatusFailed    StandaloneJobStatus = "failed"
	StandaloneJobStatusCancelled StandaloneJobStatus = "canceled"
)

// JobActionType represents the type of action to perform
type JobActionType string

const (
	JobActionSendMessage       JobActionType = "send_message"
	JobActionSendPoll          JobActionType = "send_poll"
	JobActionUpdateDescription JobActionType = "update_description"
	JobActionCloseGroups       JobActionType = "close_groups"
	JobActionOpenGroups        JobActionType = "open_groups"
	JobActionChangeGroupName   JobActionType = "change_group_name"
	JobActionChangeGroupPhoto  JobActionType = "change_group_photo"
)

// StandaloneJob represents a scheduled job for bulk group actions
// (distinct from ScheduledJob which is tied to automations)
type StandaloneJob struct {
	ID           string              `json:"id" db:"id"`
	UserID       string              `json:"user_id" db:"user_id"`
	ActionType   JobActionType       `json:"action_type" db:"action_type"`
	ScheduledFor time.Time           `json:"scheduled_for" db:"scheduled_for"`
	Status       StandaloneJobStatus `json:"status" db:"status"`
	Payload      JobPayload          `json:"payload" db:"payload"`
	ErrorMessage *string             `json:"error_message,omitempty" db:"error_message"`
	RetryCount   int                 `json:"retry_count" db:"retry_count"`
	MaxRetries   int                 `json:"max_retries" db:"max_retries"`
	CreatedAt    time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at" db:"updated_at"`
}

// CanRetry checks if the job has remaining retries.
func (j *StandaloneJob) CanRetry() bool {
	return j.RetryCount < j.MaxRetries
}

// NextRetryDelay calculates the backoff delay for the next retry attempt.
// Uses exponential backoff with jitter: base * 2^retryCount (30s, 60s, 120s, ...).
func (j *StandaloneJob) NextRetryDelay() time.Duration {
	base := 30 * time.Second
	multiplier := time.Duration(1 << j.RetryCount) // 2^retryCount
	return base * multiplier
}

// JobPayload contains action-specific data
type JobPayload struct {
	// Common
	Groups []string `json:"groups,omitempty"`

	// send_message
	Message   string `json:"message,omitempty"`
	Media     string `json:"media,omitempty"`     // base64 or URL
	MediaType string `json:"mediaType,omitempty"` // "image" | "video" | "audio"
	FileName  string `json:"fileName,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`

	// send_poll (when Type = "poll")
	Type            string   `json:"type,omitempty"` // "poll"
	Question        string   `json:"question,omitempty"`
	Options         []string `json:"options,omitempty"`
	SelectableCount int      `json:"selectableCount,omitempty"`

	// update_description
	Description string `json:"description,omitempty"`

	// change_group_name
	Name       string `json:"name,omitempty"`
	AutoNumber bool   `json:"autoNumber,omitempty"`

	// change_group_photo
	Image string `json:"image,omitempty"`
}

// Scan implements sql.Scanner for JSONB
func (p *JobPayload) Scan(value interface{}) error {
	if value == nil {
		*p = JobPayload{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("failed to scan JobPayload: expected []byte or string, got %T", value)
	}

	return json.Unmarshal(data, p)
}

// Value implements driver.Valuer for JSONB
func (p JobPayload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// IsPoll checks if the job is a poll action
func (j *StandaloneJob) IsPoll() bool {
	return j.ActionType == JobActionSendMessage && j.Payload.Type == "poll"
}

// IsAdminOnlyAction checks if the action requires admin privileges
func (j *StandaloneJob) IsAdminOnlyAction() bool {
	switch j.ActionType {
	case JobActionUpdateDescription, JobActionCloseGroups, JobActionOpenGroups,
		JobActionChangeGroupName, JobActionChangeGroupPhoto:
		return true
	default:
		return false
	}
}

// HasMedia checks if the job has media attached
func (j *StandaloneJob) HasMedia() bool {
	return j.Payload.Media != "" || j.Payload.Image != ""
}
