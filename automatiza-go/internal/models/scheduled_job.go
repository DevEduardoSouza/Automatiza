package models

// ScheduledJob represents a scheduled job in the database
type ScheduledJob struct {
	ID           string  `json:"id" db:"id"`
	AutomationID string  `json:"automation_id" db:"automation_id"`
	ScheduledFor string  `json:"scheduled_for" db:"scheduled_for"`
	Status       string  `json:"status" db:"status"` // pending|processing|completed|failed
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
	CreatedAt    string  `json:"created_at" db:"created_at"`
	UpdatedAt    string  `json:"updated_at" db:"updated_at"`
}

// JobStatus constants
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)

// IsPending returns true if the job is pending
func (j *ScheduledJob) IsPending() bool {
	return j.Status == JobStatusPending
}

// ProcessScheduledJobsRequest represents a request to process scheduled jobs
type ProcessScheduledJobsRequest struct {
	// Optional: specific automation ID to process
	AutomationID *string `json:"automationId,omitempty"`
}

// ProcessScheduledJobsResult represents the result of processing jobs
type ProcessScheduledJobsResult struct {
	ProcessedCount int      `json:"processedCount"`
	SuccessCount   int      `json:"successCount"`
	FailedCount    int      `json:"failedCount"`
	Errors         []string `json:"errors,omitempty"`
}

// QueueStatus represents the status of the job queue
type QueueStatus struct {
	Pending     int `json:"pending"`
	Processing  int `json:"processing"`
	Completed   int `json:"completed"`
	Failed      int `json:"failed"`
	TotalQueued int `json:"totalQueued"`
}
