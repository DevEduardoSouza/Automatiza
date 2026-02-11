// Package whatsapp provides domain services for WhatsApp operations.
package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// maskPhone partially masks a phone number for safe logging.
// Preserves the first 5 characters (country + area code) and the last 4,
// masking the middle portion. Example: "+5511999887766" -> "+5511*****7766".
func maskPhone(phone string) string {
	if len(phone) <= 9 {
		return "****"
	}
	return phone[:5] + strings.Repeat("*", len(phone)-9) + phone[len(phone)-4:]
}

// GroupsService handles WhatsApp group operations.
// It orchestrates between the WhatsApp provider (adapter) and the database.
type GroupsService interface {
	// GetGroups fetches all groups for a session from the WhatsApp provider.
	GetGroups(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error)

	// SyncGroups fetches groups from WhatsApp and syncs them to the database.
	SyncGroups(ctx context.Context, userID, sessionName string, instanceID int) (int, error)

	// GetGroupParticipants retrieves participants for a specific group.
	GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]whatsapp.ParticipantInfo, error)

	// UpdateGroupSubject updates the group name/subject.
	UpdateGroupSubject(ctx context.Context, sessionName, groupID, subject string) error

	// UpdateGroupDescription updates the group description.
	UpdateGroupDescription(ctx context.Context, sessionName, groupID, description string) error

	// UpdateGroupPicture updates the group profile picture.
	UpdateGroupPicture(ctx context.Context, sessionName, groupID, imageBase64 string) error

	// SetGroupAnnouncement toggles announcement mode (admin-only messages).
	SetGroupAnnouncement(ctx context.Context, sessionName, groupID string, announce bool) error

	// IsUserAdmin checks if the session user is admin in a group.
	IsUserAdmin(ctx context.Context, userID, waGroupID string) (bool, error)

	// GetUserGroups retrieves groups from the database for a user.
	GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error)

	// AddGroupMembers adds one or more members to a group.
	AddGroupMembers(ctx context.Context, sessionName, groupID string, phoneNumbers []string) (*whatsapp.MemberOperationResult, error)

	// RemoveGroupMembers removes one or more members from a group.
	RemoveGroupMembers(ctx context.Context, sessionName, groupID string, phoneNumbers []string) (*whatsapp.MemberOperationResult, error)

	// LeaveGroup makes the authenticated user leave a group.
	LeaveGroup(ctx context.Context, userID, sessionName, groupID string) error

	// PromoteMember promotes a group member to admin.
	PromoteMember(ctx context.Context, sessionName, groupID, phoneNumber string) error

	// DemoteMember demotes a group admin to regular member.
	DemoteMember(ctx context.Context, sessionName, groupID, phoneNumber string) error
}

// groupsService implements GroupsService.
type groupsService struct {
	provider whatsapp.WhatsAppProvider
	repo     repository.GroupRepository
	logger   *zap.Logger
}

// NewGroupsService creates a new groups service.
func NewGroupsService(
	provider whatsapp.WhatsAppProvider,
	repo repository.GroupRepository,
	logger *zap.Logger,
) GroupsService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &groupsService{
		provider: provider,
		repo:     repo,
		logger:   logger.Named("groups-service"),
	}
}

// GetGroups fetches all groups for a session from the WhatsApp provider.
func (s *groupsService) GetGroups(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error) {
	groups, err := s.provider.GetGroups(ctx, sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups from provider: %w", err)
	}
	return groups, nil
}

// SyncGroups fetches groups from WhatsApp and syncs them to the database.
func (s *groupsService) SyncGroups(ctx context.Context, userID, sessionName string, instanceID int) (int, error) {
	startTime := time.Now()

	// Get session status to retrieve authenticated user ID
	sessionInfo, err := s.provider.GetSessionStatus(ctx, sessionName)
	if err != nil {
		return 0, fmt.Errorf("failed to get session status: %w", err)
	}

	// Extract authenticated user ID for admin detection
	var authenticatedUserID string
	if sessionInfo.AuthenticatedUser != nil {
		authenticatedUserID = sessionInfo.AuthenticatedUser.ID
	}

	// Fetch groups from WhatsApp provider
	providerGroups, err := s.provider.GetGroups(ctx, sessionName)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch groups from WhatsApp: %w", err)
	}

	if len(providerGroups) == 0 {
		s.logger.Info("No groups found for session",
			zap.String("sessionName", sessionName))
		return 0, nil
	}

	// Convert provider groups to model groups
	modelGroups := make([]models.Group, 0, len(providerGroups))
	for _, pg := range providerGroups {
		isAdmin := s.detectAdminStatus(pg, authenticatedUserID)

		var description *string
		if pg.Description != "" {
			description = &pg.Description
		}

		modelGroup := models.Group{
			WAGroupID:    pg.ID,
			Name:         pg.Subject,
			Description:  description,
			InstanceID:   sessionName,
			UserID:       userID,
			AdminOnly:    pg.Announce,
			IsAdmin:      isAdmin,
			MembersCount: pg.Size,
			Status:       determineGroupStatus(pg.Size),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		modelGroups = append(modelGroups, modelGroup)
	}

	// Sync to database
	syncOpts := repository.SyncOptions{
		BatchSize: 100,
	}
	if err := s.repo.SyncGroups(ctx, userID, instanceID, modelGroups, syncOpts); err != nil {
		return 0, fmt.Errorf("failed to sync groups to database: %w", err)
	}

	s.logger.Info("Groups synced successfully",
		zap.String("userID", userID),
		zap.String("sessionName", sessionName),
		zap.Int("count", len(modelGroups)),
		zap.Duration("duration", time.Since(startTime)))

	return len(modelGroups), nil
}

// detectAdminStatus determines if the session user is admin in the group.
// It checks if the authenticatedUserID matches any admin participant in the group.
func (s *groupsService) detectAdminStatus(group whatsapp.GroupInfo, authenticatedUserID string) bool {
	// If we don't have the authenticated user ID, we can't determine admin status
	if authenticatedUserID == "" {
		s.logger.Debug("No authenticated user ID available for admin detection",
			zap.String("groupID", group.ID))
		return false
	}

	// Check if the authenticated user is the group owner
	if group.Owner != "" && group.Owner == authenticatedUserID {
		return true
	}

	// Check participants for admin status - only for the authenticated user
	for _, p := range group.Participants {
		if p.ID == authenticatedUserID {
			return p.IsAdmin || p.IsSuperAdmin
		}
	}

	// User not found in participants list (edge case)
	s.logger.Debug("Authenticated user not found in group participants",
		zap.String("groupID", group.ID),
		zap.String("userID", authenticatedUserID))
	return false
}

// determineGroupStatus determines group status based on member count.
func determineGroupStatus(memberCount int) string {
	// WhatsApp group limit is typically 1024 members
	const groupLimit = 1024
	const nearFullThreshold = 950

	if memberCount >= groupLimit {
		return models.GroupStatusFull
	}
	if memberCount >= nearFullThreshold {
		return models.GroupStatusClosed // Near full, might want to close
	}
	return models.GroupStatusOpen
}

// GetGroupParticipants retrieves participants for a specific group.
func (s *groupsService) GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]whatsapp.ParticipantInfo, error) {
	participants, err := s.provider.GetGroupParticipants(ctx, sessionName, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group participants: %w", err)
	}
	return participants, nil
}

// UpdateGroupSubject updates the group name/subject.
func (s *groupsService) UpdateGroupSubject(ctx context.Context, sessionName, groupID, subject string) error {
	if subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}
	if len(subject) > 100 {
		return fmt.Errorf("subject cannot exceed 100 characters")
	}

	input := whatsapp.UpdateGroupSubjectInput{
		SessionName: sessionName,
		GroupID:     groupID,
		Subject:     subject,
	}

	if err := s.provider.UpdateGroupSubject(ctx, input); err != nil {
		return fmt.Errorf("failed to update group subject: %w", err)
	}

	// Update in database (fire-and-forget)
	if err := s.repo.UpdateGroup(ctx, groupID, map[string]interface{}{
		"name":       subject,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "group DB sync failed (fire-and-forget)", err, map[string]string{
			"module": "whatsapp", "operation": "update_group_subject_db", "groupID": groupID,
		})
	}

	return nil
}

// UpdateGroupDescription updates the group description.
func (s *groupsService) UpdateGroupDescription(ctx context.Context, sessionName, groupID, description string) error {
	if len(description) > 2048 {
		return fmt.Errorf("description cannot exceed 2048 characters")
	}

	input := whatsapp.UpdateGroupDescriptionInput{
		SessionName: sessionName,
		GroupID:     groupID,
		Description: description,
	}

	if err := s.provider.UpdateGroupDescription(ctx, input); err != nil {
		return fmt.Errorf("failed to update group description: %w", err)
	}

	// Update in database (fire-and-forget)
	if err := s.repo.UpdateGroup(ctx, groupID, map[string]interface{}{
		"description": description,
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "group DB sync failed (fire-and-forget)", err, map[string]string{
			"module": "whatsapp", "operation": "update_group_description_db", "groupID": groupID,
		})
	}

	return nil
}

// UpdateGroupPicture updates the group profile picture.
func (s *groupsService) UpdateGroupPicture(ctx context.Context, sessionName, groupID, imageBase64 string) error {
	if imageBase64 == "" {
		return fmt.Errorf("image data cannot be empty")
	}

	input := whatsapp.UpdateGroupPictureInput{
		SessionName: sessionName,
		GroupID:     groupID,
		ImageBase64: imageBase64,
	}

	if err := s.provider.UpdateGroupPicture(ctx, input); err != nil {
		return fmt.Errorf("failed to update group picture: %w", err)
	}

	return nil
}

// SetGroupAnnouncement toggles announcement mode (admin-only messages).
func (s *groupsService) SetGroupAnnouncement(ctx context.Context, sessionName, groupID string, announce bool) error {
	input := whatsapp.SetGroupAnnouncementInput{
		SessionName: sessionName,
		GroupID:     groupID,
		Announce:    announce,
	}

	if err := s.provider.SetGroupAnnouncement(ctx, input); err != nil {
		return fmt.Errorf("failed to set group announcement mode: %w", err)
	}

	// Update in database (fire-and-forget)
	if err := s.repo.UpdateGroup(ctx, groupID, map[string]interface{}{
		"admin_only": announce,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "group DB sync failed (fire-and-forget)", err, map[string]string{
			"module": "whatsapp", "operation": "set_group_announcement_db", "groupID": groupID,
		})
	}

	return nil
}

// IsUserAdmin checks if the session user is admin in a group.
func (s *groupsService) IsUserAdmin(ctx context.Context, userID, waGroupID string) (bool, error) {
	return s.repo.IsUserAdminInGroup(ctx, userID, waGroupID)
}

// GetUserGroups retrieves groups from the database for a user.
func (s *groupsService) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	groups, err := s.repo.GetUserGroups(ctx, userID, adminOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	return groups, nil
}

// AddGroupMembers adds one or more members to a group.
func (s *groupsService) AddGroupMembers(ctx context.Context, sessionName, groupID string, phoneNumbers []string) (*whatsapp.MemberOperationResult, error) {
	if len(phoneNumbers) == 0 {
		return nil, fmt.Errorf("at least one phone number is required")
	}

	input := whatsapp.AddGroupMembersInput{
		SessionName:  sessionName,
		GroupID:      groupID,
		PhoneNumbers: phoneNumbers,
	}

	result, err := s.provider.AddGroupMembers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to add group members: %w", err)
	}

	s.logger.Info("Members added to group",
		zap.String("groupID", groupID),
		zap.Int("successCount", result.SuccessCount),
		zap.Int("failedCount", len(result.FailedNumbers)))

	return result, nil
}

// RemoveGroupMembers removes one or more members from a group.
func (s *groupsService) RemoveGroupMembers(ctx context.Context, sessionName, groupID string, phoneNumbers []string) (*whatsapp.MemberOperationResult, error) {
	if len(phoneNumbers) == 0 {
		return nil, fmt.Errorf("at least one phone number is required")
	}

	input := whatsapp.RemoveGroupMembersInput{
		SessionName:  sessionName,
		GroupID:      groupID,
		PhoneNumbers: phoneNumbers,
	}

	result, err := s.provider.RemoveGroupMembers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to remove group members: %w", err)
	}

	s.logger.Info("Members removed from group",
		zap.String("groupID", groupID),
		zap.Int("successCount", result.SuccessCount),
		zap.Int("failedCount", len(result.FailedNumbers)))

	return result, nil
}

// LeaveGroup makes the authenticated user leave a group.
func (s *groupsService) LeaveGroup(ctx context.Context, userID, sessionName, groupID string) error {
	input := whatsapp.LeaveGroupInput{
		SessionName: sessionName,
		GroupID:     groupID,
	}

	if err := s.provider.LeaveGroup(ctx, input); err != nil {
		return fmt.Errorf("failed to leave group: %w", err)
	}

	// Remove group from database after leaving (look up by WA group ID first)
	group, err := s.repo.GetGroupByWAGroupID(ctx, userID, groupID)
	if err == nil && group != nil {
		if deleteErr := s.repo.DeleteGroup(ctx, group.ID, userID); deleteErr != nil {
			s.logger.Warn("Failed to delete group from database after leaving",
				zap.String("groupID", groupID),
				zap.Error(deleteErr))
		}
	}

	s.logger.Info("Left group",
		zap.String("groupID", groupID))

	return nil
}

// PromoteMember promotes a group member to admin.
func (s *groupsService) PromoteMember(ctx context.Context, sessionName, groupID, phoneNumber string) error {
	if phoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}

	input := whatsapp.PromoteMemberInput{
		SessionName: sessionName,
		GroupID:     groupID,
		PhoneNumber: phoneNumber,
	}

	if err := s.provider.PromoteMember(ctx, input); err != nil {
		return fmt.Errorf("failed to promote member: %w", err)
	}

	s.logger.Info("Member promoted to admin",
		zap.String("groupID", groupID),
		zap.String("phoneNumber", maskPhone(phoneNumber)))

	return nil
}

// DemoteMember demotes a group admin to regular member.
func (s *groupsService) DemoteMember(ctx context.Context, sessionName, groupID, phoneNumber string) error {
	if phoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}

	input := whatsapp.DemoteMemberInput{
		SessionName: sessionName,
		GroupID:     groupID,
		PhoneNumber: phoneNumber,
	}

	if err := s.provider.DemoteMember(ctx, input); err != nil {
		return fmt.Errorf("failed to demote member: %w", err)
	}

	s.logger.Info("Member demoted from admin",
		zap.String("groupID", groupID),
		zap.String("phoneNumber", maskPhone(phoneNumber)))

	return nil
}
