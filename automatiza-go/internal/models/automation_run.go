package models

import "time"

// AutomationRun represents an automation execution record
type AutomationRun struct {
	ID             string     `json:"id" db:"id"`
	AutomationID   string     `json:"automation_id" db:"automation_id"`
	StartedAt      time.Time  `json:"started_at" db:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	Status         string     `json:"status" db:"status"`
	OffersFound    int        `json:"offers_found" db:"offers_found"`
	MessagesSent   int        `json:"messages_sent" db:"messages_sent"`
	MessagesFailed int        `json:"messages_failed" db:"messages_failed"`
	ErrorMessage   *string    `json:"error_message,omitempty" db:"error_message"`
	LockUntil      *time.Time `json:"lock_until,omitempty" db:"lock_until"`
}

// AutomationRunStatus constants
const (
	AutomationRunStatusRunning   = "running"
	AutomationRunStatusCompleted = "completed"
	AutomationRunStatusFailed    = "failed"
)

// IsLocked returns true if the run is currently locked
func (r *AutomationRun) IsLocked() bool {
	if r.LockUntil == nil {
		return false
	}
	return r.LockUntil.After(time.Now())
}
