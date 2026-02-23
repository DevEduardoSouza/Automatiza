package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
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
		var providerErr *whatsapp.ProviderError
		if errors.As(err, &providerErr) {
			if providerErr.Code == whatsapp.ErrCodeSessionNotFound {
				w.logger.Info("Session no longer exists, skipping sync",
					zap.String("instance", data.InstanceName))
				return nil
			}
			if providerErr.Code == whatsapp.ErrCodeProviderError {
				w.logger.Debug("Provider temporarily unavailable, skipping sync",
					zap.String("instance", data.InstanceName),
					zap.Error(err))
				return nil
			}
		}
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

	// W38: Validate instance ID format before processing
	instanceIDStr := fmt.Sprintf("%d", instance.ID)
	if instance.ID <= 0 {
		validationErr := fmt.Errorf("invalid instance ID format: %d for instance %s", instance.ID, data.InstanceName)
		observability.LogAndCapture(w.logger, sentry.LevelError, "Invalid instance ID format", validationErr, map[string]string{
			"module":       "groups_sync",
			"instanceName": data.InstanceName,
			"userID":       data.UserID,
		})
		return validationErr
	}

	// 4. Determine admin status for each group using tiered detection.
	// Tier 1: GOWS group-level flag (IsAdmin/IsSuperAdmin on GroupInfo) — zero API calls.
	// Tier 2: Embedded participants already in memory — zero API calls.
	// Tier 3: Separate GetGroupParticipants API call (N+1 fallback for non-GOWS engines).
	// Tier 4: Owner field fallback — zero API calls.
	groups := make([]models.Group, 0, len(groupInfos))
	adminCount := 0
	tier1Count := 0
	tier2Count := 0
	tier3Count := 0
	tier4Count := 0
	lidOnlyCount := 0

	for i, gi := range groupInfos {
		// W22: Check for context cancellation at the beginning of each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var description *string
		if gi.Description != "" {
			description = &gi.Description
		}

		group := models.Group{
			WAGroupID:    gi.ID,
			Name:         gi.Subject,
			Description:  description,
			InstanceID:   instanceIDStr,
			UserID:       data.UserID,
			AdminOnly:    gi.Announce,
			MembersCount: gi.Size,
		}

		// Tier 1: GOWS group-level flag (no API call needed).
		// The GOWS engine populates IsAdmin/IsSuperAdmin at the group level,
		// indicating if the authenticated session user is an admin.
		if gi.IsAdmin || gi.IsSuperAdmin {
			group.IsAdmin = true
			adminCount++
			tier1Count++
			w.logger.Debug("Admin detected via tier 1 (group-level flag)",
				zap.String("groupName", gi.Subject),
				zap.Bool("isAdmin", gi.IsAdmin),
				zap.Bool("isSuperAdmin", gi.IsSuperAdmin))
			groups = append(groups, group)
			w.emitProgress(data.UserID, taskID, gi, group, i, totalGroups)
			continue
		}

		// Tier 2: Check embedded participants (already in memory, no API call).
		if len(gi.Participants) > 0 && sessionInfo.AuthenticatedUser != nil {
			sessionPhone := whatsapp.ExtractPhoneFromID(sessionInfo.AuthenticatedUser.ID)
			foundInEmbedded := false
			for _, p := range gi.Participants {
				phone := p.PhoneNumber
				if phone == "" {
					phone = whatsapp.ExtractPhoneFromID(p.ID)
				}
				if phone == sessionPhone {
					foundInEmbedded = true
					if p.IsAdmin || p.IsSuperAdmin {
						group.IsAdmin = true
					}
					break
				}
			}
			if foundInEmbedded {
				if group.IsAdmin {
					adminCount++
					tier2Count++
					w.logger.Debug("Admin detected via tier 2 (embedded participants)",
						zap.String("groupName", gi.Subject))
				}
				groups = append(groups, group)
				w.emitProgress(data.UserID, taskID, gi, group, i, totalGroups)
				continue
			}
		}

		// Tier 3: Separate API call (N+1 fallback for non-GOWS engines).
		if sessionInfo.AuthenticatedUser != nil {
			// W21: Add Sentry breadcrumb before participant fetch
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "groups_sync",
				Message:  fmt.Sprintf("Fetching participants for group %s (%s)", gi.Subject, gi.ID),
				Level:    sentry.LevelInfo,
				Data: map[string]interface{}{
					"groupID":      gi.ID,
					"groupName":    gi.Subject,
					"instanceName": data.InstanceName,
					"index":        i,
					"total":        totalGroups,
				},
			})

			participants, participantsErr := w.whatsappProvider.GetGroupParticipants(ctx, data.InstanceName, gi.ID)
			if participantsErr != nil {
				w.logger.Warn("Failed to get participants",
					zap.String("groupID", gi.ID), zap.String("groupName", gi.Subject), zap.Error(participantsErr))
			} else {
				sessionPhone := whatsapp.ExtractPhoneFromID(sessionInfo.AuthenticatedUser.ID)
				matched := false
				groupHasLIDOnly := true
				for _, p := range participants {
					participantPhone := p.PhoneNumber
					if participantPhone == "" {
						participantPhone = whatsapp.ExtractPhoneFromID(p.ID)
					} else {
						groupHasLIDOnly = false
					}
					if participantPhone == sessionPhone {
						matched = true
						if p.IsAdmin || p.IsSuperAdmin {
							group.IsAdmin = true
							adminCount++
							tier3Count++
							w.logger.Debug("Admin detected via tier 3 (API call)",
								zap.String("groupName", gi.Subject))
						}
						break
					}
				}
				if groupHasLIDOnly && len(participants) > 0 {
					lidOnlyCount++
				}

				// Tier 4: Owner fallback — if no participant matched, check group owner.
				if !matched && gi.Owner != "" {
					ownerPhone := whatsapp.ExtractPhoneFromID(gi.Owner)
					if ownerPhone == sessionPhone {
						w.logger.Debug("Admin detected via tier 4 (owner fallback)",
							zap.String("groupName", gi.Subject),
							zap.String("owner", gi.Owner))
						group.IsAdmin = true
						adminCount++
						tier4Count++
					}
				}
			}
		} else {
			w.logger.Warn("AuthenticatedUser is nil, skipping admin detection",
				zap.String("groupName", gi.Subject))
		}

		groups = append(groups, group)
		w.emitProgress(data.UserID, taskID, gi, group, i, totalGroups)
	}

	// Log admin detection summary with tier breakdown
	w.logger.Info("Admin detection summary",
		zap.Int("totalGroups", totalGroups),
		zap.Int("adminGroups", adminCount),
		zap.Int("tier1GroupLevel", tier1Count),
		zap.Int("tier2Embedded", tier2Count),
		zap.Int("tier3ApiCall", tier3Count),
		zap.Int("tier4Owner", tier4Count),
		zap.Int("lidOnlyGroups", lidOnlyCount))

	if totalGroups > 0 && adminCount == 0 {
		w.logger.Warn("ZERO groups detected as admin — possible LID resolution failure",
			zap.Int("totalGroups", totalGroups),
			zap.Int("lidOnlyGroups", lidOnlyCount))
		sentry.CaptureMessage(fmt.Sprintf(
			"Admin detection found 0/%d admin groups (LID-only: %d) for user %s",
			totalGroups, lidOnlyCount, data.UserID,
		))
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
		w.logger.Info("[groups_sync_worker] Publishing final completion event",
			zap.String("jobId", taskID),
			zap.String("status", "completed"),
			zap.Int("newGroups", syncResult.Created),
			zap.Int("updatedGroups", syncResult.Updated))
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

// emitProgress publishes an SSE progress event for a single group processed.
func (w *GroupsSyncWorker) emitProgress(userID, taskID string, gi whatsapp.GroupInfo, group models.Group, index, total int) {
	if w.progressPub == nil || total <= 0 {
		return
	}
	progress := int((float64(index+1) / float64(total)) * 100)
	w.progressPub.PublishJobProgress(userID, map[string]any{
		"jobType": "groups_sync",
		"jobId":   taskID,
		"group": map[string]any{
			"id":      gi.ID,
			"name":    gi.Subject,
			"isAdmin": group.IsAdmin,
		},
		"progress": progress,
		"current":  index + 1,
		"total":    total,
	})
}

// Ensure GroupsSyncWorker implements asynq.Handler interface
func (w *GroupsSyncWorker) ProcessTaskWrapper() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessTask(ctx, task)
	}
}
