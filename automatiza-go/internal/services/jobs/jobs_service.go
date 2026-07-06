package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the jobs service interface.
// This interface supports both standalone jobs (new) and automation-scheduled jobs (legacy).
type Service interface {
	// Processing - shared between both service types
	ProcessScheduledJobs(ctx context.Context) (*ProcessResult, error)
	GetQueueStatus(ctx context.Context, userID string) (*QueueStatus, error)

	// Legacy methods for automation-based scheduled jobs
	ScheduleJob(ctx context.Context, job *models.ScheduledJob) error
	CancelJob(ctx context.Context, jobID string) error

	// New CRUD operations for standalone jobs
	CreateJob(ctx context.Context, userID string, job *models.StandaloneJob) (*models.StandaloneJob, error)
	GetJob(ctx context.Context, id, userID string) (*models.StandaloneJob, error)
	ListJobs(ctx context.Context, userID string) ([]models.StandaloneJob, error)
	ListJobsPaginated(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error)
	UpdateJob(ctx context.Context, id, userID string, updates map[string]interface{}) (*models.StandaloneJob, error)
	DeleteJob(ctx context.Context, id, userID string) error
}

// ProcessResult represents the result of processing scheduled jobs.
type ProcessResult struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

// QueueStatus represents the current queue status for a user.
type QueueStatus struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// =============================================================================
// Standalone Jobs Service (New Implementation)
// =============================================================================

// JobStatusNotifier sends real-time job status updates to connected clients.
type JobStatusNotifier interface {
	PublishJobStatus(userID, jobID, status string, errorMessage *string)
}

// HistoryNotifier sends real-time history change events to connected clients.
type HistoryNotifier interface {
	PublishHistoryChange(userID, historyID, status string)
}

type standaloneJobsService struct {
	jobRepo         repository.JobRepository
	instanceRepo    repository.InstanceRepository
	groupRepo       repository.GroupRepository
	historyRepo     repository.ActionHistoryRepository
	waProvider      whatsapp.WhatsAppProvider
	notifier        JobStatusNotifier
	historyNotifier HistoryNotifier
	logger          *zap.Logger
}

// NewService creates a new standalone jobs service.
// This service handles the 8 action types for bulk group operations.
// notifier is optional - pass nil to disable real-time notifications.
// historyRepo is optional - pass nil to disable action history tracking.
// historyNotifier is optional - pass nil to disable real-time history notifications.
func NewService(
	jobRepo repository.JobRepository,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	waProvider whatsapp.WhatsAppProvider,
	notifier JobStatusNotifier,
	historyRepo repository.ActionHistoryRepository,
	historyNotifier HistoryNotifier,
	logger *zap.Logger,
) Service {
	return &standaloneJobsService{
		jobRepo:         jobRepo,
		instanceRepo:    instanceRepo,
		groupRepo:       groupRepo,
		historyRepo:     historyRepo,
		waProvider:      waProvider,
		notifier:        notifier,
		historyNotifier: historyNotifier,
		logger:          logger,
	}
}

// notifyJobStatus sends a real-time status update if a notifier is configured.
func (s *standaloneJobsService) notifyJobStatus(userID, jobID string, status models.StandaloneJobStatus, errorMessage *string) {
	if s.notifier != nil {
		s.notifier.PublishJobStatus(userID, jobID, string(status), errorMessage)
	}
}

// notifyHistoryChange sends a real-time history change event if a notifier is configured.
func (s *standaloneJobsService) notifyHistoryChange(userID, historyID, status string) {
	if s.historyNotifier != nil && historyID != "" {
		s.historyNotifier.PublishHistoryChange(userID, historyID, status)
	}
}

// mapJobActionToHistoryType converts a JobActionType to the action_type string used in action_history.
func mapJobActionToHistoryType(actionType models.JobActionType) string {
	switch actionType {
	case models.JobActionSendMessage:
		return models.ActionTypeSendMessage
	case models.JobActionSendPoll:
		return models.ActionTypeSendMessage
	case models.JobActionUpdateDescription:
		return models.ActionTypeChangeDescription
	case models.JobActionCloseGroups:
		return models.ActionTypeCloseGroups
	case models.JobActionOpenGroups:
		return models.ActionTypeOpenGroups
	case models.JobActionChangeGroupName:
		return models.ActionTypeChangeName
	case models.JobActionChangeGroupPhoto:
		return models.ActionTypeChangePhoto
	default:
		return string(actionType)
	}
}

// descriptionForAction returns a human-readable description for a job action type.
func descriptionForAction(actionType models.JobActionType, groupCount int) string {
	switch actionType {
	case models.JobActionSendMessage:
		return fmt.Sprintf("Enviar mensagem para %d grupo(s)", groupCount)
	case models.JobActionSendPoll:
		return fmt.Sprintf("Enviar enquete para %d grupo(s)", groupCount)
	case models.JobActionUpdateDescription:
		return fmt.Sprintf("Alterar descricao de %d grupo(s)", groupCount)
	case models.JobActionCloseGroups:
		return fmt.Sprintf("Fechar %d grupo(s)", groupCount)
	case models.JobActionOpenGroups:
		return fmt.Sprintf("Abrir %d grupo(s)", groupCount)
	case models.JobActionChangeGroupName:
		return fmt.Sprintf("Alterar nome de %d grupo(s)", groupCount)
	case models.JobActionChangeGroupPhoto:
		return fmt.Sprintf("Alterar foto de %d grupo(s)", groupCount)
	default:
		return fmt.Sprintf("Executar acao em %d grupo(s)", groupCount)
	}
}

// createActionHistory creates an action_history entry for a job (best effort).
func (s *standaloneJobsService) createActionHistory(ctx context.Context, job *models.StandaloneJob) string {
	if s.historyRepo == nil {
		return ""
	}

	groupCount := len(job.Payload.Groups)
	historyType := mapJobActionToHistoryType(job.ActionType)
	description := descriptionForAction(job.ActionType, groupCount)

	targetCount := int32(groupCount)
	history, err := s.historyRepo.Create(ctx, job.UserID, historyType, description, map[string]interface{}{
		"job_id":      job.ID,
		"action_type": string(job.ActionType),
		"group_count": groupCount,
	})
	if err != nil {
		s.logger.Warn("Failed to create action history for job",
			zap.String("jobID", job.ID),
			zap.Error(err))
		return ""
	}
	if history == nil {
		return ""
	}

	// Set target_count and mark as running
	if err := s.historyRepo.Update(ctx, history.ID, map[string]interface{}{
		"target_count": targetCount,
		"status":       models.ActionStatusRunning,
	}); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError,
			"Failed to update history target_count and status", err,
			map[string]string{"module": "jobs", "operation": "history-target-count-update"})
	}

	s.notifyHistoryChange(job.UserID, history.ID, models.ActionStatusRunning)

	return history.ID
}

// completeActionHistory updates the action_history entry after job execution.
func (s *standaloneJobsService) completeActionHistory(ctx context.Context, userID, historyID string, successCount, errorCount int, errorMsg *string) {
	if s.historyRepo == nil || historyID == "" {
		return
	}

	status := models.ActionStatusCompleted
	if successCount == 0 && errorCount > 0 {
		status = models.ActionStatusFailed
	}

	errStr := ""
	if errorMsg != nil {
		errStr = *errorMsg
	}

	if err := s.historyRepo.UpdateCounts(ctx, historyID, int32(successCount), int32(errorCount)); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError,
			"Failed to update history counts", err,
			map[string]string{"module": "jobs", "operation": "history-update-counts"})
	}
	if err := s.historyRepo.UpdateStatus(ctx, historyID, status, errStr); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError,
			"Failed to update history status", err,
			map[string]string{"module": "jobs", "operation": "history-update-status"})
	}

	s.notifyHistoryChange(userID, historyID, status)
}

// failActionHistory marks an action_history entry as failed before execution.
func (s *standaloneJobsService) failActionHistory(ctx context.Context, userID, historyID string, errMsg string) {
	if s.historyRepo == nil || historyID == "" {
		return
	}
	if err := s.historyRepo.MarkFailed(ctx, historyID, errMsg); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError,
			"Failed to mark history as failed", err,
			map[string]string{"module": "jobs", "operation": "history-mark-failed"})
	}

	s.notifyHistoryChange(userID, historyID, models.ActionStatusFailed)
}

// ResolvedGroup contains pre-fetched group data for executor validation.
type ResolvedGroup struct {
	DBID      string
	WAGroupID string
	IsAdmin   bool
	AdminOnly bool // announcement mode (closed group)
}

// resolveGroups fetches group data from DB and maps DB IDs to WA group IDs.
// Returns a map of DB ID -> ResolvedGroup, and errors for groups not found.
func (s *standaloneJobsService) resolveGroups(ctx context.Context, userID string, groupIDs []string) (map[string]*ResolvedGroup, []ActionResult) {
	groups, err := s.groupRepo.GetGroupsByIDs(ctx, userID, groupIDs)
	if err != nil {
		s.logger.Error("Failed to fetch groups for job", zap.Error(err))
		// Return all as errors
		results := make([]ActionResult, len(groupIDs))
		for i, gID := range groupIDs {
			results[i] = ActionResult{GroupID: gID, Success: false, Error: "falha ao buscar dados do grupo"}
		}
		return nil, results
	}

	// Build lookup map
	resolved := make(map[string]*ResolvedGroup, len(groups))
	for _, g := range groups {
		resolved[g.ID] = &ResolvedGroup{
			DBID:      g.ID,
			WAGroupID: g.WAGroupID,
			IsAdmin:   g.IsAdmin,
			AdminOnly: g.AdminOnly,
		}
	}

	// Check for groups not found
	var errors []ActionResult
	for _, gID := range groupIDs {
		if _, ok := resolved[gID]; !ok {
			errors = append(errors, ActionResult{GroupID: gID, Success: false, Error: "Grupo nao encontrado"})
		}
	}

	return resolved, errors
}

// validateAndResolveWAGroups resolves group IDs to WA group IDs.
// Admin and closed group checks are delegated to WAHA - if the action fails,
// WAHA will return the appropriate error.
func (s *standaloneJobsService) validateAndResolveWAGroups(
	ctx context.Context,
	job *models.StandaloneJob,
) (validWAGroupIDs []string, errorResults []ActionResult) {
	resolved, notFoundErrors := s.resolveGroups(ctx, job.UserID, job.Payload.Groups)
	errorResults = append(errorResults, notFoundErrors...)

	if resolved == nil {
		return nil, errorResults
	}

	for _, dbID := range job.Payload.Groups {
		rg, ok := resolved[dbID]
		if !ok {
			continue // already in notFoundErrors
		}

		validWAGroupIDs = append(validWAGroupIDs, rg.WAGroupID)
	}

	return validWAGroupIDs, errorResults
}

// CreateJob creates a new standalone job.
func (s *standaloneJobsService) CreateJob(ctx context.Context, userID string, job *models.StandaloneJob) (*models.StandaloneJob, error) {
	// Validate groups
	if err := ValidateGroups(job.Payload.Groups); err != nil {
		return nil, err
	}

	// Set user ID
	job.UserID = userID

	// Validate action-specific requirements
	switch job.ActionType {
	case models.JobActionSendMessage:
		if job.Payload.Type == "poll" {
			if err := ValidatePollOptions(job.Payload.Options); err != nil {
				return nil, err
			}
		} else if job.Payload.Message == "" && job.Payload.Media == "" {
			return nil, fmt.Errorf("mensagem ou midia e obrigatoria")
		}
	case models.JobActionSendPoll:
		if err := ValidatePollOptions(job.Payload.Options); err != nil {
			return nil, err
		}
		if job.Payload.Question == "" {
			return nil, fmt.Errorf("pergunta da enquete e obrigatoria")
		}
	case models.JobActionChangeGroupPhoto:
		if job.Payload.Image == "" {
			return nil, fmt.Errorf("imagem e obrigatoria")
		}
	case models.JobActionChangeGroupName:
		if job.Payload.Name == "" {
			return nil, fmt.Errorf("nome do grupo e obrigatorio")
		}
	}

	// Set default max retries if not specified
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}

	created, err := s.jobRepo.Create(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return created, nil
}

// GetJob retrieves a job by ID (with ownership check).
func (s *standaloneJobsService) GetJob(ctx context.Context, id, userID string) (*models.StandaloneJob, error) {
	job, err := s.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found")
	}
	if job.UserID != userID {
		return nil, fmt.Errorf("job not found")
	}
	return job, nil
}

// ListJobs returns all jobs for a user.
func (s *standaloneJobsService) ListJobs(ctx context.Context, userID string) ([]models.StandaloneJob, error) {
	return s.jobRepo.GetByUserID(ctx, userID, nil, 100)
}

// ListJobsPaginated returns a paginated list of jobs for a user using cursor-based pagination.
func (s *standaloneJobsService) ListJobsPaginated(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error) {
	return s.jobRepo.ListByUserID(ctx, params)
}

// ErrJobNotPending is returned by UpdateJob when the target job already left
// the pending state (e.g. already ran) and can no longer be edited. Exported
// so the connect handler maps it to a clean FailedPrecondition + user-facing
// message instead of a raw "[internal]" error leaking to the UI.
var ErrJobNotPending = errors.New("apenas jobs pendentes podem ser atualizados")

// ErrJobNotDeletable is returned by DeleteJob when the job is neither pending
// nor cancelled and thus cannot be removed.
var ErrJobNotDeletable = errors.New("apenas jobs pendentes ou cancelados podem ser deletados")

// UpdateJob updates a job (only pending jobs can be updated).
func (s *standaloneJobsService) UpdateJob(ctx context.Context, id, userID string, updates map[string]interface{}) (*models.StandaloneJob, error) {
	job, err := s.GetJob(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if job.Status != models.StandaloneJobStatusPending {
		return nil, ErrJobNotPending
	}

	if err := s.jobRepo.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	return s.jobRepo.GetByID(ctx, id)
}

// DeleteJob deletes a job (with ownership check).
func (s *standaloneJobsService) DeleteJob(ctx context.Context, id, userID string) error {
	job, err := s.GetJob(ctx, id, userID)
	if err != nil {
		return err
	}

	if job.Status != models.StandaloneJobStatusPending && job.Status != models.StandaloneJobStatusCancelled {
		return ErrJobNotDeletable
	}

	return s.jobRepo.Delete(ctx, id, userID)
}

// ProcessScheduledJobs processes all pending standalone jobs that are due.
func (s *standaloneJobsService) ProcessScheduledJobs(ctx context.Context) (*ProcessResult, error) {
	result := &ProcessResult{
		Errors: []string{},
	}

	jobs, err := s.jobRepo.GetPendingDueJobs(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending jobs: %w", err)
	}

	s.logger.Info("Processing standalone jobs", zap.Int("count", len(jobs)))

	for _, job := range jobs {
		if err := s.jobRepo.UpdateStatus(ctx, job.ID, models.StandaloneJobStatusRunning, nil); err != nil {
			s.logger.Warn("Failed to mark job as running",
				zap.String("jobID", job.ID),
				zap.Error(err))
			continue
		}
		s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusRunning, nil)

		// Create action history entry for this job
		historyID := s.createActionHistory(ctx, &job)

		sessionName, err := s.getConnectedSession(ctx, job.UserID)
		if err != nil {
			errMsg := err.Error()
			if updateErr := s.jobRepo.UpdateStatus(ctx, job.ID, models.StandaloneJobStatusFailed, &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update job status to failed (no session)", updateErr,
					map[string]string{"module": "jobs", "operation": "job-status-update"})
			}
			s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusFailed, &errMsg)
			s.failActionHistory(ctx, job.UserID, historyID, errMsg)
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Job %s: %s", job.ID, errMsg))
			continue
		}

		actionResults := s.executeJob(ctx, &job, sessionName)
		success, failed, errorMsg := AggregateResults(actionResults)

		if failed > 0 && success == 0 {
			// All groups failed - check if we can retry
			if job.CanRetry() {
				nextRun := time.Now().Add(job.NextRetryDelay())
				if err := s.jobRepo.ScheduleRetry(ctx, job.ID, nextRun); err != nil {
					s.logger.Error("Failed to schedule retry, marking as failed",
						zap.String("jobID", job.ID),
						zap.Int("retryCount", job.RetryCount),
						zap.Error(err))
					if updateErr := s.jobRepo.UpdateStatus(ctx, job.ID, models.StandaloneJobStatusFailed, errorMsg); updateErr != nil {
						observability.LogAndCapture(s.logger, sentry.LevelError,
							"Failed to update job status to failed (retry schedule failed)", updateErr,
							map[string]string{"module": "jobs", "operation": "job-status-update"})
					}
					s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusFailed, errorMsg)
					s.completeActionHistory(ctx, job.UserID, historyID, success, failed, errorMsg)
					result.Failed++
				} else {
					s.logger.Info("Scheduled retry for job",
						zap.String("jobID", job.ID),
						zap.Int("retryCount", job.RetryCount+1),
						zap.Int("maxRetries", job.MaxRetries),
						zap.Time("nextRun", nextRun))
					// Notify as pending again (scheduled for retry)
					s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusPending, nil)
					s.completeActionHistory(ctx, job.UserID, historyID, success, failed, errorMsg)
					result.Skipped++
				}
				if errorMsg != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("Job %s (retry %d/%d): %s", job.ID, job.RetryCount+1, job.MaxRetries, *errorMsg))
				}
			} else {
				// Retries exhausted - mark as failed
				if updateErr := s.jobRepo.UpdateStatus(ctx, job.ID, models.StandaloneJobStatusFailed, errorMsg); updateErr != nil {
					observability.LogAndCapture(s.logger, sentry.LevelError,
						"Failed to update job status to failed (retries exhausted)", updateErr,
						map[string]string{"module": "jobs", "operation": "job-status-update"})
				}
				s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusFailed, errorMsg)
				s.completeActionHistory(ctx, job.UserID, historyID, success, failed, errorMsg)
				result.Failed++
				if errorMsg != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("Job %s (retries exhausted): %s", job.ID, *errorMsg))
				}
			}
		} else {
			// At least partial success - mark as done
			if updateErr := s.jobRepo.UpdateStatus(ctx, job.ID, models.StandaloneJobStatusDone, errorMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update job status to done", updateErr,
					map[string]string{"module": "jobs", "operation": "job-status-update"})
			}
			s.notifyJobStatus(job.UserID, job.ID, models.StandaloneJobStatusDone, errorMsg)
			s.completeActionHistory(ctx, job.UserID, historyID, success, failed, errorMsg)
			result.Processed++
		}

		s.logger.Info("Processed job",
			zap.String("jobID", job.ID),
			zap.String("actionType", string(job.ActionType)),
			zap.Int("success", success),
			zap.Int("failed", failed),
			zap.Int("retryCount", job.RetryCount))
	}

	return result, nil
}

// GetQueueStatus returns queue statistics for a user.
func (s *standaloneJobsService) GetQueueStatus(ctx context.Context, userID string) (*QueueStatus, error) {
	counts, err := s.jobRepo.CountByStatus(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &QueueStatus{
		Pending:   counts[models.StandaloneJobStatusPending],
		Running:   counts[models.StandaloneJobStatusRunning],
		Completed: counts[models.StandaloneJobStatusDone],
		Failed:    counts[models.StandaloneJobStatusFailed],
	}, nil
}

// ScheduleJob is not supported in standalone jobs service.
// Use CreateJob instead.
func (s *standaloneJobsService) ScheduleJob(_ context.Context, _ *models.ScheduledJob) error {
	return fmt.Errorf("ScheduleJob not supported in standalone jobs service; use CreateJob instead")
}

// CancelJob is not supported in standalone jobs service.
// Use DeleteJob instead.
func (s *standaloneJobsService) CancelJob(_ context.Context, _ string) error {
	return fmt.Errorf("CancelJob not supported in standalone jobs service; use DeleteJob instead")
}

func (s *standaloneJobsService) getConnectedSession(ctx context.Context, userID string) (string, error) {
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get instances: %w", err)
	}

	for _, inst := range instances {
		if inst.Status == "connected" {
			return inst.InstanceID, nil
		}
	}

	return "", fmt.Errorf("nenhuma instancia conectada")
}

func (s *standaloneJobsService) executeJob(ctx context.Context, job *models.StandaloneJob, sessionName string) []ActionResult {
	// Resolve DB group IDs to WA group IDs with admin/closed validation
	validWAGroupIDs, errorResults := s.validateAndResolveWAGroups(ctx, job)

	if len(validWAGroupIDs) == 0 {
		// All groups failed validation
		return errorResults
	}

	// Replace payload groups with resolved WA group IDs for executor
	resolvedJob := *job
	resolvedJob.Payload.Groups = validWAGroupIDs

	var executorResults []ActionResult
	switch job.ActionType {
	case models.JobActionSendMessage:
		executorResults = ExecuteSendMessage(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionSendPoll:
		executorResults = ExecuteSendPoll(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionUpdateDescription:
		executorResults = ExecuteUpdateDescription(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionCloseGroups:
		executorResults = ExecuteCloseGroups(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionOpenGroups:
		executorResults = ExecuteOpenGroups(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionChangeGroupName:
		executorResults = ExecuteChangeGroupName(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	case models.JobActionChangeGroupPhoto:
		executorResults = ExecuteChangeGroupPhoto(ctx, s.waProvider, sessionName, &resolvedJob, s.logger)
	default:
		for _, g := range validWAGroupIDs {
			executorResults = append(executorResults, ActionResult{GroupID: g, Success: false, Error: "tipo de acao desconhecido"})
		}
	}

	// Combine validation errors with execution results
	return append(errorResults, executorResults...)
}

// =============================================================================
// Legacy Jobs Service (Automation-based Scheduled Jobs)
// =============================================================================

type legacyJobsService struct {
	scheduledJobRepo repository.ScheduledJobRepository
	automationRepo   repository.AutomationRepository
	instanceRepo     repository.InstanceRepository
	queueClient      *workers.QueueClient
	cache            *cache.Cache
	logger           *zap.Logger
}

// NewJobsService creates a legacy jobs service for automation-based scheduled jobs.
// Deprecated: Use NewService for new standalone jobs functionality.
func NewJobsService(
	scheduledJobRepo repository.ScheduledJobRepository,
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
	queueClient *workers.QueueClient,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &legacyJobsService{
		scheduledJobRepo: scheduledJobRepo,
		automationRepo:   automationRepo,
		instanceRepo:     instanceRepo,
		queueClient:      queueClient,
		cache:            cache,
		logger:           logger,
	}
}

func (s *legacyJobsService) ProcessScheduledJobs(ctx context.Context) (*ProcessResult, error) {
	result := &ProcessResult{
		Errors: []string{},
	}

	jobs, err := s.scheduledJobRepo.GetPendingJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending jobs: %w", err)
	}

	s.logger.Info("Processing automation scheduled jobs", zap.Int("count", len(jobs)))

	for _, job := range jobs {
		scheduledTime, err := time.Parse(time.RFC3339, job.ScheduledFor)
		if err != nil {
			s.logger.Warn("Invalid scheduled time",
				zap.String("jobID", job.ID),
				zap.String("scheduledFor", job.ScheduledFor))
			result.Skipped++
			continue
		}

		if time.Now().Before(scheduledTime) {
			result.Skipped++
			continue
		}

		err = s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "processing", nil)
		if err != nil {
			s.logger.Warn("Failed to update job status",
				zap.String("jobID", job.ID),
				zap.Error(err))
			result.Skipped++
			continue
		}

		automation, err := s.automationRepo.GetAutomationByID(ctx, job.AutomationID)
		if err != nil || automation == nil {
			errMsg := "automation not found"
			if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update scheduled job status (automation not found)", updateErr,
					map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
			}
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Job %s: %s", job.ID, errMsg))
			continue
		}

		if !automation.IsActive() {
			errMsg := "automation is not active"
			if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update scheduled job status (automation inactive)", updateErr,
					map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
			}
			result.Failed++
			continue
		}

		instances, err := s.instanceRepo.GetUserInstances(ctx, automation.UserID)
		if err != nil || len(instances) == 0 {
			errMsg := "no connected instance found"
			if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update scheduled job status (no instance)", updateErr,
					map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
			}
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Job %s: %s", job.ID, errMsg))
			continue
		}

		var connectedInstance *models.Instance
		for i := range instances {
			if instances[i].Status == "connected" {
				connectedInstance = &instances[i]
				break
			}
		}

		if connectedInstance == nil {
			errMsg := "no connected instance"
			if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update scheduled job status (no connected instance)", updateErr,
					map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
			}
			result.Failed++
			continue
		}

		err = s.executeAutomation(ctx, automation, connectedInstance)
		if err != nil {
			errMsg := err.Error()
			if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "failed", &errMsg); updateErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError,
					"Failed to update scheduled job status (execution failed)", updateErr,
					map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
			}
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("Job %s: %s", job.ID, errMsg))
			continue
		}

		if updateErr := s.scheduledJobRepo.UpdateJobStatus(ctx, job.ID, "completed", nil); updateErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError,
				"Failed to update scheduled job status to completed", updateErr,
				map[string]string{"module": "jobs", "operation": "scheduled-job-status-update"})
		}
		result.Processed++

		now := time.Now().UTC().Format(time.RFC3339)
		if updateErr := s.automationRepo.UpdateAutomation(ctx, automation.ID, map[string]interface{}{
			"last_run_at": now,
		}); updateErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError,
				"Failed to update automation last_run_at", updateErr,
				map[string]string{"module": "jobs", "operation": "automation-next-run-update"})
		}

		s.logger.Info("Processed scheduled job",
			zap.String("jobID", job.ID),
			zap.String("automationID", automation.ID))
	}

	return result, nil
}

func (s *legacyJobsService) GetQueueStatus(ctx context.Context, _ string) (*QueueStatus, error) {
	jobs, err := s.scheduledJobRepo.GetPendingJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue status: %w", err)
	}

	status := &QueueStatus{}
	for _, job := range jobs {
		switch job.Status {
		case "pending":
			status.Pending++
		case "processing":
			status.Running++
		case "completed":
			status.Completed++
		case "failed":
			status.Failed++
		}
	}

	return status, nil
}

func (s *legacyJobsService) ScheduleJob(ctx context.Context, job *models.ScheduledJob) error {
	if job.AutomationID == "" {
		return fmt.Errorf("automation_id is required")
	}

	if job.ScheduledFor == "" {
		return fmt.Errorf("scheduled_for is required")
	}

	job.Status = "pending"
	_, err := s.scheduledJobRepo.CreateJob(ctx, job)
	return err
}

func (s *legacyJobsService) CancelJob(ctx context.Context, jobID string) error {
	errMsg := "canceled by user"
	return s.scheduledJobRepo.UpdateJobStatus(ctx, jobID, "canceled", &errMsg)
}

// CreateJob is not supported in legacy jobs service.
func (s *legacyJobsService) CreateJob(_ context.Context, _ string, _ *models.StandaloneJob) (*models.StandaloneJob, error) {
	return nil, fmt.Errorf("CreateJob not supported in legacy jobs service; use ScheduleJob instead")
}

// GetJob is not supported in legacy jobs service.
func (s *legacyJobsService) GetJob(_ context.Context, _, _ string) (*models.StandaloneJob, error) {
	return nil, fmt.Errorf("GetJob not supported in legacy jobs service")
}

// ListJobs is not supported in legacy jobs service.
func (s *legacyJobsService) ListJobs(_ context.Context, _ string) ([]models.StandaloneJob, error) {
	return nil, fmt.Errorf("ListJobs not supported in legacy jobs service")
}

// ListJobsPaginated is not supported in legacy jobs service.
func (s *legacyJobsService) ListJobsPaginated(_ context.Context, _ repository.JobListParams) (*repository.JobListResult, error) {
	return nil, fmt.Errorf("ListJobsPaginated not supported in legacy jobs service")
}

// UpdateJob is not supported in legacy jobs service.
func (s *legacyJobsService) UpdateJob(_ context.Context, _, _ string, _ map[string]interface{}) (*models.StandaloneJob, error) {
	return nil, fmt.Errorf("UpdateJob not supported in legacy jobs service")
}

// DeleteJob is not supported in legacy jobs service.
func (s *legacyJobsService) DeleteJob(_ context.Context, _, _ string) error {
	return fmt.Errorf("DeleteJob not supported in legacy jobs service; use CancelJob instead")
}

func (s *legacyJobsService) executeAutomation(ctx context.Context, automation *models.Automation, instance *models.Instance) error {
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		return fmt.Errorf("no target groups configured")
	}

	var message string
	if automation.GetCustomMessage() != "" {
		message = automation.GetCustomMessage()
	} else {
		message = "Confira as ofertas do dia!"
	}

	for _, groupID := range targetGroups {
		jobData := &workers.MessageJobData{
			InstanceName: instance.InstanceID,
			GroupID:      groupID,
			Message:      message,
			UserID:       automation.UserID,
		}

		if s.queueClient != nil {
			_, err := s.queueClient.EnqueueMessage(ctx, jobData, 0)
			if err != nil {
				s.logger.Warn("Failed to enqueue message",
					zap.String("groupID", groupID),
					zap.Error(err))
			}
		}
	}

	return nil
}
