package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// JobProgressPublisher publishes real-time job progress events.
// Implemented by SSEHub (or RedisBridge for cross-process communication).
type JobProgressPublisher interface {
	PublishJobProgress(userID string, data map[string]any)
}

// GroupsSyncWorker processes groups synchronization tasks.
// It uses the WhatsAppProvider interface (adapter pattern) for fetching groups,
// decoupling the worker from specific WhatsApp API implementations.
type GroupsSyncWorker struct {
	whatsappProvider whatsapp.WhatsAppProvider
	instanceRepo     repository.InstanceRepository
	groupRepo        repository.GroupRepository
	logger           *zap.Logger
	progressPub      JobProgressPublisher
}

// NewGroupsSyncWorker creates a new groups sync worker.
// IMPORTANT: This now requires a WhatsAppProvider instead of waha.Service.
// The progressPub parameter is optional - if nil, progress events won't be emitted.
func NewGroupsSyncWorker(
	whatsappProvider whatsapp.WhatsAppProvider,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	logger *zap.Logger,
	progressPub JobProgressPublisher,
) *GroupsSyncWorker {
	return &GroupsSyncWorker{
		whatsappProvider: whatsappProvider,
		instanceRepo:     instanceRepo,
		groupRepo:        groupRepo,
		logger:           logger,
		progressPub:      progressPub,
	}
}

// ProcessTask processes a groups sync task
func (w *GroupsSyncWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var data GroupsSyncJobData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	taskID := task.ResultWriter().TaskID()

	w.logger.Info("Processing groups sync job",
		zap.String("userID", data.UserID),
		zap.String("instanceName", data.InstanceName),
		zap.String("taskID", taskID))

	// 1. Validate instance ownership
	instance, err := w.instanceRepo.GetInstanceByUserAndName(ctx, data.UserID, data.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		w.logger.Warn("Instance not found or not owned by user",
			zap.String("userID", data.UserID),
			zap.String("instanceName", data.InstanceName))
		return nil // Don't retry
	}

	// 2. Check instance status via WhatsAppProvider
	sessionInfo, err := w.whatsappProvider.GetSessionStatus(ctx, data.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	if sessionInfo.Status != whatsapp.SessionStatusConnected {
		w.logger.Warn("Instance not connected, cannot sync groups",
			zap.String("instanceName", data.InstanceName),
			zap.String("status", string(sessionInfo.Status)))
		return fmt.Errorf("instance not connected: %s", sessionInfo.Status)
	}

	// 3. Fetch groups from WhatsApp via WhatsAppProvider
	groupInfos, err := w.whatsappProvider.GetGroups(ctx, data.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to fetch groups: %w", err)
	}

	w.logger.Info("Fetched groups from WhatsApp",
		zap.String("instanceName", data.InstanceName),
		zap.Int("count", len(groupInfos)))

	totalGroups := len(groupInfos)

	// 4. Get participants for each group to determine admin status
	groups := make([]models.Group, 0, len(groupInfos))
	for i, gi := range groupInfos {
		var description *string
		if gi.Description != "" {
			description = &gi.Description
		}

		group := models.Group{
			WAGroupID:    gi.ID,
			Name:         gi.Subject,
			Description:  description,
			InstanceID:   fmt.Sprintf("%d", instance.ID),
			UserID:       data.UserID,
			AdminOnly:    gi.Announce,
			MembersCount: gi.Size,
		}

		// Check if user is admin in this group
		participants, err := w.whatsappProvider.GetGroupParticipants(ctx, data.InstanceName, gi.ID)
		if err == nil {
			if sessionInfo.AuthenticatedUser != nil {
				// Compare by phone number only since WAHA uses different suffixes:
				// Me.ID = "5511999999999@c.us", participant.ID = "5511999999999@s.whatsapp.net"
				sessionPhone := whatsapp.ExtractPhoneFromID(sessionInfo.AuthenticatedUser.ID)
				for _, p := range participants {
					participantPhone := whatsapp.ExtractPhoneFromID(p.ID)
					if participantPhone == sessionPhone && (p.IsAdmin || p.IsSuperAdmin) {
						group.IsAdmin = true
						break
					}
				}
			} else {
				// Fallback: check embedded participants from GetGroups response
				for _, p := range gi.Participants {
					if p.IsAdmin || p.IsSuperAdmin {
						group.IsAdmin = true
						break
					}
				}
			}
		}

		groups = append(groups, group)

		// Emit SSE progress event for each group processed
		if w.progressPub != nil && totalGroups > 0 {
			progress := int((float64(i+1) / float64(totalGroups)) * 100)
			w.progressPub.PublishJobProgress(data.UserID, map[string]any{
				"jobType": "groups_sync",
				"jobId":   taskID,
				"group": map[string]string{
					"id":   gi.ID,
					"name": gi.Subject,
				},
				"progress": progress,
				"current":  i + 1,
				"total":    totalGroups,
			})
		}
	}

	// 5. Sync to database (batch)
	syncResult, err := w.groupRepo.SyncGroupsWithStats(ctx, data.UserID, instance.ID, groups, repository.SyncOptions{
		BatchSize: 50,
	})

	if err != nil {
		return fmt.Errorf("failed to sync groups to database: %w", err)
	}

	// Emit final SSE event with completion stats
	if w.progressPub != nil {
		w.progressPub.PublishJobProgress(data.UserID, map[string]any{
			"jobType":       "groups_sync",
			"jobId":         taskID,
			"status":        "completed",
			"progress":      100,
			"newGroups":     syncResult.Created,
			"updatedGroups": syncResult.Updated,
		})
	}

	w.logger.Info("Groups synced successfully",
		zap.String("userID", data.UserID),
		zap.String("instanceName", data.InstanceName),
		zap.Int("count", len(groups)),
		zap.Int("created", syncResult.Created),
		zap.Int("updated", syncResult.Updated))

	return nil
}

// Ensure GroupsSyncWorker implements asynq.Handler interface
func (w *GroupsSyncWorker) ProcessTaskWrapper() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessTask(ctx, task)
	}
}
