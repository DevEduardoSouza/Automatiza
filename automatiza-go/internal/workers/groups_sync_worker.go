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

	// Log session info for admin detection debugging
	if sessionInfo.AuthenticatedUser != nil {
		w.logger.Info("Session authenticated user",
			zap.String("meID", sessionInfo.AuthenticatedUser.ID),
			zap.String("mePhone", whatsapp.ExtractPhoneFromID(sessionInfo.AuthenticatedUser.ID)))
	} else {
		w.logger.Warn("Session has no AuthenticatedUser (Me is nil)")
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
		if err != nil {
			w.logger.Warn("Failed to get participants",
				zap.String("groupID", gi.ID), zap.String("groupName", gi.Subject), zap.Error(err))
		} else {
			w.logger.Info("Got participants for group",
				zap.String("groupID", gi.ID), zap.String("groupName", gi.Subject),
				zap.Int("count", len(participants)))
			for j, p := range participants {
				if j >= 3 {
					break
				}
				w.logger.Debug("Participant sample",
					zap.String("groupName", gi.Subject),
					zap.String("pID", p.ID),
					zap.String("pPhone", p.PhoneNumber),
					zap.Bool("isAdmin", p.IsAdmin),
					zap.Bool("isSuperAdmin", p.IsSuperAdmin))
			}
		}
		if err == nil && sessionInfo.AuthenticatedUser != nil {
			// WAHA GOWS returns participant IDs in @lid format (e.g. "56243167223861@lid")
			// while Me.ID uses @c.us format (e.g. "5511999999999@c.us").
			// The /participants/v2 endpoint resolves LIDs to phone numbers in the PhoneNumber field,
			// so we compare using PhoneNumber (which has the actual phone) instead of ID.
			sessionPhone := whatsapp.ExtractPhoneFromID(sessionInfo.AuthenticatedUser.ID)
			for _, p := range participants {
				participantPhone := p.PhoneNumber
				if participantPhone == "" {
					// PhoneNumber not resolved (LID mapping failed) — try extracting from ID
					participantPhone = whatsapp.ExtractPhoneFromID(p.ID)
				}
				if participantPhone == sessionPhone {
					w.logger.Info("Found session user in group",
						zap.String("groupName", gi.Subject),
						zap.String("participantID", p.ID),
						zap.String("participantPhone", participantPhone),
						zap.Bool("isAdmin", p.IsAdmin),
						zap.Bool("isSuperAdmin", p.IsSuperAdmin))
				}
				if participantPhone == sessionPhone && (p.IsAdmin || p.IsSuperAdmin) {
					group.IsAdmin = true
					break
				}
			}
		} else if err == nil {
			// AuthenticatedUser is nil — use embedded participants from GetGroups
			// and match by phone number extracted from participant IDs
			for _, p := range gi.Participants {
				if p.IsAdmin || p.IsSuperAdmin {
					// Without session user context, we can't determine if WE are admin.
					// Skip — is_admin stays false (safe default).
					break
				}
			}
		} else {
			w.logger.Warn("Failed to get group participants, skipping admin check",
				zap.String("groupID", gi.ID),
				zap.Error(err))
		}

		w.logger.Info("Admin detection result",
			zap.String("groupName", gi.Subject),
			zap.Bool("isAdmin", group.IsAdmin))

		groups = append(groups, group)

		// Emit SSE progress event for each group processed
		if w.progressPub != nil && totalGroups > 0 {
			progress := int((float64(i+1) / float64(totalGroups)) * 100)
			w.progressPub.PublishJobProgress(data.UserID, map[string]any{
				"jobType": "groups_sync",
				"jobId":   taskID,
				"group": map[string]any{
					"id":      gi.ID,
					"name":    gi.Subject,
					"isAdmin": group.IsAdmin,
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
