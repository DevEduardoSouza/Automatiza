package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"math"

	"connectrpc.com/connect"

	"automatiza-go/internal/config"
	"automatiza-go/internal/connectrpc/mappers"
	jobsv1 "automatiza-go/internal/gen/jobs/v1"
	"automatiza-go/internal/gen/jobs/v1/jobsv1connect"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/jobs"

	"go.uber.org/zap"
)

// JobsHandler implements the jobsv1connect.JobsServiceHandler interface.
type JobsHandler struct {
	jobsService  jobs.Service
	userRoleRepo repository.UserRoleRepository
	logger       *zap.Logger
	cfg          *config.Config
}

// Compile-time check to ensure JobsHandler implements the interface.
var _ jobsv1connect.JobsServiceHandler = (*JobsHandler)(nil)

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(jobsService jobs.Service, userRoleRepo repository.UserRoleRepository, logger *zap.Logger, cfg *config.Config) *JobsHandler {
	return &JobsHandler{
		jobsService:  jobsService,
		userRoleRepo: userRoleRepo,
		logger:       logger,
		cfg:          cfg,
	}
}

// requireAdmin checks if the user is an admin and returns an error if not.
func (h *JobsHandler) requireAdmin(ctx context.Context) error {
	user := GetUserFromContext(ctx)
	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, user.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}

	if !isAdmin {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("Acesso negado: requer permissao de administrador"))
	}

	return nil
}

// ListJobs returns jobs for the authenticated user with cursor-based pagination.
func (h *JobsHandler) ListJobs(
	ctx context.Context,
	req *connect.Request[jobsv1.ListJobsRequest],
) (*connect.Response[jobsv1.ListJobsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	pageSize := int(req.Msg.GetPageSize())
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	result, err := h.jobsService.ListJobsPaginated(ctx, repository.JobListParams{
		UserID:   user.ID,
		PageSize: pageSize,
		Cursor:   req.Msg.GetPageToken(),
	})
	if err != nil {
		h.logger.Error("Failed to list jobs",
			zap.String("userID", user.ID),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list jobs: %w", err))
	}

	return connect.NewResponse(&jobsv1.ListJobsResponse{
		Jobs:          mappers.JobsToProto(result.Jobs),
		NextPageToken: result.NextCursor,
		TotalCount:    int32(result.TotalCount),
	}), nil
}

// GetJob returns a specific job by ID.
func (h *JobsHandler) GetJob(
	ctx context.Context,
	req *connect.Request[jobsv1.GetJobRequest],
) (*connect.Response[jobsv1.GetJobResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	job, err := h.jobsService.GetJob(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("job not found"))
		}
		h.logger.Error("Failed to get job",
			zap.String("jobID", req.Msg.GetId()),
			zap.String("userID", user.ID),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get job: %w", err))
	}

	return connect.NewResponse(&jobsv1.GetJobResponse{
		Job: mappers.JobToProto(job),
	}), nil
}

// CreateJob creates a new job.
func (h *JobsHandler) CreateJob(
	ctx context.Context,
	req *connect.Request[jobsv1.CreateJobRequest],
) (*connect.Response[jobsv1.CreateJobResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Build domain model from proto
	job := &models.StandaloneJob{
		ActionType: mappers.ProtoToActionType(req.Msg.GetActionType()),
		Status:     models.StandaloneJobStatusPending,
	}

	if req.Msg.GetScheduledFor() != nil {
		job.ScheduledFor = req.Msg.GetScheduledFor().AsTime()
	}

	if req.Msg.GetPayload() != nil {
		job.Payload = *mappers.ProtoToPayload(req.Msg.GetPayload())
	}

	created, err := h.jobsService.CreateJob(ctx, user.ID, job)
	if err != nil {
		h.logger.Error("Failed to create job",
			zap.String("userID", user.ID),
			zap.String("actionType", string(job.ActionType)),
			zap.Error(err))
		// Validation errors from service should return InvalidArgument
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	h.logger.Info("Job created",
		zap.String("jobID", created.ID),
		zap.String("userID", user.ID),
		zap.String("actionType", string(created.ActionType)))

	return connect.NewResponse(&jobsv1.CreateJobResponse{
		Job: mappers.JobToProto(created),
	}), nil
}

// UpdateJob updates an existing job.
func (h *JobsHandler) UpdateJob(
	ctx context.Context,
	req *connect.Request[jobsv1.UpdateJobRequest],
) (*connect.Response[jobsv1.UpdateJobResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Build updates map from optional fields
	updates := make(map[string]interface{})

	if req.Msg.ActionType != nil {
		updates["action_type"] = string(mappers.ProtoToActionType(*req.Msg.ActionType))
	}
	if req.Msg.ScheduledFor != nil {
		updates["scheduled_for"] = req.Msg.GetScheduledFor().AsTime()
	}
	if req.Msg.Payload != nil {
		updates["payload"] = *mappers.ProtoToPayload(req.Msg.GetPayload())
	}

	if len(updates) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no updates provided"))
	}

	updated, err := h.jobsService.UpdateJob(ctx, req.Msg.GetId(), user.ID, updates)
	if err != nil {
		if isNotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("job not found"))
		}
		// Business rule, not a server fault: the job already ran (left the
		// pending state). Return a clean FailedPrecondition with a
		// user-facing message so the UI shows "já foi executado…" instead of
		// the raw "[internal] failed to update job: …" that leaked before.
		if errors.Is(err, jobs.ErrJobNotPending) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("Este agendamento já foi executado e não pode mais ser editado. Crie um novo agendamento."))
		}
		h.logger.Error("Failed to update job",
			zap.String("jobID", req.Msg.GetId()),
			zap.String("userID", user.ID),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update job: %w", err))
	}

	h.logger.Info("Job updated",
		zap.String("jobID", updated.ID),
		zap.String("userID", user.ID))

	return connect.NewResponse(&jobsv1.UpdateJobResponse{
		Job: mappers.JobToProto(updated),
	}), nil
}

// DeleteJob deletes a job.
func (h *JobsHandler) DeleteJob(
	ctx context.Context,
	req *connect.Request[jobsv1.DeleteJobRequest],
) (*connect.Response[jobsv1.DeleteJobResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	err := h.jobsService.DeleteJob(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("job not found"))
		}
		if errors.Is(err, jobs.ErrJobNotDeletable) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("Este agendamento já foi executado e não pode mais ser excluído."))
		}
		h.logger.Error("Failed to delete job",
			zap.String("jobID", req.Msg.GetId()),
			zap.String("userID", user.ID),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete job: %w", err))
	}

	h.logger.Info("Job deleted",
		zap.String("jobID", req.Msg.GetId()),
		zap.String("userID", user.ID))

	return connect.NewResponse(&jobsv1.DeleteJobResponse{}), nil
}

// ProcessScheduledJobs processes all pending scheduled jobs that are due.
// This is typically called by a cron job or scheduler, not end users.
// Requires admin role.
func (h *JobsHandler) ProcessScheduledJobs(
	ctx context.Context,
	req *connect.Request[jobsv1.ProcessScheduledJobsRequest],
) (*connect.Response[jobsv1.ProcessScheduledJobsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	result, err := h.jobsService.ProcessScheduledJobs(ctx)
	if err != nil {
		h.logger.Error("Failed to process scheduled jobs",
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to process scheduled jobs: %w", err))
	}

	h.logger.Info("Processed scheduled jobs",
		zap.Int("processed", result.Processed),
		zap.Int("failed", result.Failed),
		zap.Int("skipped", result.Skipped))

	return connect.NewResponse(&jobsv1.ProcessScheduledJobsResponse{
		Processed: int32(result.Processed),
		Failed:    int32(result.Failed),
		Errors:    result.Errors,
	}), nil
}

// GetQueueStatus returns queue status metrics for the authenticated user.
func (h *JobsHandler) GetQueueStatus(
	ctx context.Context,
	req *connect.Request[jobsv1.GetQueueStatusRequest],
) (*connect.Response[jobsv1.GetQueueStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	status, err := h.jobsService.GetQueueStatus(ctx, user.ID)
	if err != nil {
		h.logger.Error("Failed to get queue status",
			zap.String("userID", user.ID),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get queue status: %w", err))
	}

	queued := int32(status.Pending + status.Running)

	limitPerMinute := int32(h.cfg.WhatsAppSendLimitPerMinute)
	workerConcurrency := int32(h.cfg.WorkerConcurrency)

	resp := &jobsv1.GetQueueStatusResponse{
		Queue: &jobsv1.QueueMetrics{
			Waiting: int32(status.Pending),
			Active:  int32(status.Running),
			Queued:  queued,
		},
		Settings: &jobsv1.QueueSettings{
			LimitPerMinute:    &limitPerMinute,
			WorkerConcurrency: &workerConcurrency,
		},
	}

	if limitPerMinute > 0 && queued > 0 {
		est := math.Ceil(float64(queued) / float64(limitPerMinute))
		resp.EstimatedMinutes = &est
	}

	return connect.NewResponse(resp), nil
}

// isNotFoundError checks if an error indicates a not found condition.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg == "job not found" ||
		errors.Is(err, errors.New("job not found")) ||
		contains(errMsg, "not found")
}

// contains checks if a string contains a substring (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
