package repository

import (
	"automatiza-go/internal/models"
	"context"
	"time"
)

// ProfileRepository handles profile data operations
type ProfileRepository interface {
	EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error)
	GetProfile(ctx context.Context, userID string) (*models.Profile, error)
	GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error)
	GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error)
	CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error)
	CreateUserCredentials(ctx context.Context, userID, passwordHash string) error
	UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error
	PauseUserAutomations(ctx context.Context, userID string) error
	ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error)
	SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error)
	GetMaxAutomations(ctx context.Context, userID string) (int, error)
	CountUserAutomations(ctx context.Context, userID string) (int, error)
	GetProfileStats(ctx context.Context) (*ProfileStats, error)
}

// PlanStat represents the count of users per plan
type PlanStat struct {
	Plan  string
	Count int
}

// GrowthPoint represents a daily user count for growth charts
type GrowthPoint struct {
	Date  string
	Users int
}

// ProfileStats holds aggregated profile statistics for the admin dashboard
type ProfileStats struct {
	TotalUsers   int
	ActiveUsers  int
	InactiveUsers int
	PlanStats    []PlanStat
	GrowthData   []GrowthPoint
}

// InstanceRepository handles instance data operations
type InstanceRepository interface {
	GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error)
	GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error)
	GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error)
	GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error)
	GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error)
	UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error)
	UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error
	DeleteInstance(ctx context.Context, instanceID int) error
	// SetDefaultInstance marks the given instance as the user's default chip.
	// Atomically clears any prior default for the user, so the partial unique
	// index uniq_instances_user_default is preserved.
	SetDefaultInstance(ctx context.Context, userID string, instanceID int) error
	// UpdateInstanceLabel sets the friendly_name on an instance owned by userID.
	// Pass empty string to clear (NULL) the label so the UI falls back to instance_id.
	UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error
	// UpdateInstancePhoneNumber records the phone number harvested from WAHA's
	// `me.id` once a session reaches WORKING. Idempotent: skips the write when
	// the value already matches so the row does not churn updated_at.
	UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error
	// CountUserInstances returns the number of instances owned by a user.
	// Used for quota enforcement before creating a new instance.
	CountUserInstances(ctx context.Context, userID string) (int, error)
}

// GroupPaginatedResult holds paginated groups result
type GroupPaginatedResult struct {
	Groups     []models.Group
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
	HasMore    bool
}

// GroupRepository handles group data operations
type GroupRepository interface {
	GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error)
	// instanceID = 0 means "no chip filter" (all chips). When > 0, filters
	// to groups where the chip is a participant (via participating_instance_ids).
	GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string, instanceID int) (*GroupPaginatedResult, error)
	GetGroupByID(ctx context.Context, groupID string) (*models.Group, error)
	GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error)
	GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error)
	GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error)
	CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error)
	SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options SyncOptions) error
	SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options SyncOptions) (*SyncResult, error)
	IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error)
	UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error
	ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error
	DeleteGroup(ctx context.Context, groupID, userID string) error
	DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error
	BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error)
	CountAll(ctx context.Context) (int64, error)
	// CountGroupsByInstance returns the number of groups bound to an instance for a user.
	// Used for quota enforcement (MaxGroupsPerInstance).
	CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error)
	// GetInstancesForGroups returns a map keyed by wa_group_id of the instance that
	// owns each group, joining groups.instance_id -> instances. Used by the dispatch
	// router to send messages through the chip that owns the group instead of the
	// user's first connected instance. Missing groups are simply absent from the map.
	GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error)
}

// SyncOptions represents options for group syncing
type SyncOptions struct {
	BatchSize int
}

// SyncResult contains statistics from a sync operation
type SyncResult struct {
	Created int
	Updated int
	Deleted int
}

// AutomationRepository handles automation data operations
type AutomationRepository interface {
	GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error)
	GetActiveAutomations(ctx context.Context) ([]models.Automation, error)
	GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error)
	GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error)
	CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error)
	UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error
	DeleteAutomation(ctx context.Context, automationID, userID string) error
	GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error)
	GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error)
	UpdateLastRunAt(ctx context.Context, automationID string) error
	UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error
	// Admin-specific methods for bulk operations
	BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error)
	BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error
}

// ActionHistoryRepository handles action history data operations
type ActionHistoryRepository interface {
	Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error)
	Update(ctx context.Context, historyID string, updates map[string]interface{}) error
	UpdateStatus(ctx context.Context, id, status, errorMessage string) error
	GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error)
	GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error)
	GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error)
	GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error)
	CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error)
	MarkCompleted(ctx context.Context, historyID string) error
	MarkFailed(ctx context.Context, historyID, errorMessage string) error
	MarkRunning(ctx context.Context, historyID string) error
	IncrementSuccessCount(ctx context.Context, historyID string) error
	IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error
	UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error
	DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error)
}

// SettingsRepository handles settings data operations
type SettingsRepository interface {
	Get(ctx context.Context, keys ...string) (models.SettingsMap, error)
	GetAll(ctx context.Context) (models.SettingsMap, error)
	Update(ctx context.Context, key string, value interface{}) error

	// DispatchTexts CRUD
	GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error)
	GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error)
	GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error)
	GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error)
	CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error)
	UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error
	DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error
}

// CouponRepository handles coupon data operations
type CouponRepository interface {
	GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error)
	GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error)
	GetCouponByCode(ctx context.Context, userID, code string) (*models.Coupon, error)
	CreateCoupon(ctx context.Context, userID string, data *models.Coupon) (*models.Coupon, error)
	UpdateCoupon(ctx context.Context, couponID, userID string, updates map[string]interface{}) error
	DeleteCoupon(ctx context.Context, couponID, userID string) error
	GetActiveCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error)
	GetBestCouponForProduct(ctx context.Context, userID, store string, productPrice float64, category *string) (*models.Coupon, error)
}

// DispatchLogWithGroup represents a dispatch log with group name from a join
type DispatchLogWithGroup struct {
	models.DispatchLog
	GroupName *string `db:"group_name"`
}

// DispatchLogFilterResult represents dispatch logs with total count for pagination
type DispatchLogFilterResult struct {
	Logs       []DispatchLogWithGroup
	TotalCount int64
}

// DispatchLogRepository handles dispatch log data operations
type DispatchLogRepository interface {
	Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error)
	Update(ctx context.Context, logID int, updates map[string]interface{}) error
	GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error)
	GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error)
	DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error)
	// GetByAutomationIDWithCount retrieves dispatch logs for a specific automation with total count for pagination.
	// Verifies user ownership via automation_id -> automations.user_id check.
	GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]DispatchLogWithGroup, int, error)
	// GetUserLogsWithFilters retrieves dispatch logs for a user with optional filters.
	// If automationID is provided, filters by that automation (verifying user ownership).
	// If automationID is empty, returns logs from all user's automations.
	// Supports optional filters: group_id, status, store, search_url (ILIKE on product_url).
	// Returns logs with group names, total count for pagination.
	GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]DispatchLogWithGroup, int64, error)
	// ExistsByCanonicalKey checks if a dispatch log with the given canonical_key exists
	// for the user and group within the specified time window.
	// Used for reconciliation to avoid creating duplicate logs.
	ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error)
	// CreateIfNotExists creates a dispatch log only if one doesn't already exist
	// for the same user, group, and canonical_key within the time window.
	// Returns the created log and true if created, or nil and false if already exists.
	// This is idempotent and safe for concurrent reconciliation.
	CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error)
	// DeleteOldLogsByStatus deletes dispatch logs with a specific status older than the given duration.
	DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error)
}

// ScheduledJobRepository handles scheduled job data operations
type ScheduledJobRepository interface {
	GetPendingJobs(ctx context.Context) ([]models.ScheduledJob, error)
	UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error
	CreateJob(ctx context.Context, job *models.ScheduledJob) (*models.ScheduledJob, error)
	GetJobByID(ctx context.Context, jobID string) (*models.ScheduledJob, error)
}

// SystemEventRepository handles system event data operations
type SystemEventRepository interface {
	Create(ctx context.Context, eventType string, payload map[string]interface{}) error
	GetRecent(ctx context.Context, limit int) ([]models.SystemEvent, error)
}

// UserRoleRepository handles user role data operations
type UserRoleRepository interface {
	GetUserRole(ctx context.Context, userID string) (*models.UserRole, error)
	SetUserRole(ctx context.Context, userID, role string) error
	RemoveUserRole(ctx context.Context, userID, role string) error
	IsAdmin(ctx context.Context, userID string) (bool, error)
	GetAllUserRoles(ctx context.Context, userID string) ([]string, error)
	GetAdminUsers(ctx context.Context) ([]models.UserRole, error)
}

// PasswordResetToken represents a password reset token record
type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
	IPAddress string
	UserAgent string
	Email     string // Populated from join with profiles
}

// PasswordResetTokenRepository handles password reset token operations
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *PasswordResetToken) error
	GetByHash(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	MarkUsed(ctx context.Context, tokenHash string) error
	InvalidateUserTokens(ctx context.Context, userID string) error
	CleanupExpired(ctx context.Context) (int64, error)
}

// RateLimitRepository handles rate limiting operations
type RateLimitRepository interface {
	GetAttempts(ctx context.Context, identifier, identifierType string, window time.Duration) (int, error)
	Increment(ctx context.Context, identifier, identifierType string) error
	Reset(ctx context.Context, identifier, identifierType string) error
	Cleanup(ctx context.Context) (int64, error)
}

// AffiliateCredentialsRepository handles affiliate credentials
type AffiliateCredentialsRepository interface {
	// GetByUserAndStore retrieves credentials for a specific user and store (only active)
	GetByUserAndStore(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error)
	// GetAllByUser retrieves all credentials for a user (only active)
	GetAllByUser(ctx context.Context, userID string) ([]models.AffiliateCredentials, error)
	// Upsert creates or updates credentials for a user and store
	Upsert(ctx context.Context, userID, store string, data *models.AffiliateCredentials) error
	// Delete hard deletes credentials for a user and store
	Delete(ctx context.Context, userID, store string) error
	// Deactivate soft deletes credentials (sets is_active = false)
	Deactivate(ctx context.Context, userID, store string) error
	// GetActiveStores returns a list of stores with active credentials for a user
	GetActiveStores(ctx context.Context, userID string) ([]string, error)
}

// MLCredentialsRepository handles ML credentials
type MLCredentialsRepository interface {
	Get(ctx context.Context, userID string) (*models.MLCredentialsDB, error)
	Save(ctx context.Context, userID, encryptedData string) error
}

// MLLinkRepository handles ML link entries
type MLLinkRepository interface {
	GetByOriginalURL(ctx context.Context, userID, originalURL string) (*models.MLLinkEntry, error)
	Create(ctx context.Context, link *models.MLLinkEntry) error
	GetRecentByUser(ctx context.Context, userID string, limit int) ([]models.MLLinkEntry, error)
}

// UserServerRepository handles user-server mappings for load balancing
type UserServerRepository interface {
	GetByUserID(ctx context.Context, userID string) (*models.UserServerMapping, error)
	Create(ctx context.Context, mapping *models.UserServerMapping) error
	Update(ctx context.Context, userID, serverID string) error
	Delete(ctx context.Context, userID string) error
	CountByServer(ctx context.Context, serverID string) (int, error)
}

// EmailTemplateRepository handles email templates
type EmailTemplateRepository interface {
	GetByName(ctx context.Context, name string) (*models.EmailTemplate, error)
	GetByID(ctx context.Context, id string) (*models.EmailTemplate, error)
	GetAll(ctx context.Context) ([]models.EmailTemplate, error)
	Create(ctx context.Context, template *models.EmailTemplate) (*models.EmailTemplate, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	SetActive(ctx context.Context, id string, isActive bool) error
}

// WebhookEventsRepository handles webhook event idempotency checking
type WebhookEventsRepository interface {
	InsertIfNotExists(ctx context.Context, eventID, sessionName, eventType string) (bool, error)
	Exists(ctx context.Context, eventID string) (bool, error)
	MarkProcessing(ctx context.Context, eventID string) error
	MarkProcessed(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID, errorMessage string) error
	MarkDLQ(ctx context.Context, eventID, errorMessage string) error
	IncrementRetryCount(ctx context.Context, eventID string) error
	GetPendingEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error)
	GetFailedEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
}

// DashboardStats represents aggregated dashboard statistics
type DashboardStats struct {
	TotalGroups  int
	FullGroups   int
	TotalMembers int
	ActiveLinks  int
}

// AnnouncementSettings represents dashboard announcement configuration
type AnnouncementSettings struct {
	Enabled bool
	Title   string
	Body    string
}

// HistoryPeriod represents the time period for historical data
type HistoryPeriod string

const (
	HistoryPeriodWeekly  HistoryPeriod = "weekly"
	HistoryPeriodMonthly HistoryPeriod = "monthly"
	HistoryPeriodYearly  HistoryPeriod = "yearly"
)

// HistoryDataPoint represents a single data point in the dashboard history
type HistoryDataPoint struct {
	Label      string
	Members    int
	Groups     int
	NewMembers int
	NewGroups  int
}

// DashboardRepository handles dashboard-related data operations
type DashboardRepository interface {
	GetGroupsStats(ctx context.Context, userID string) (*DashboardStats, error)
	GetActiveLinksCount(ctx context.Context, userID string) (int, error)
	GetAnnouncementSettings(ctx context.Context) (*AnnouncementSettings, error)
	GetHistoryStats(ctx context.Context, userID string, period HistoryPeriod) ([]HistoryDataPoint, error)
}

// ReconciliationCheckpoint represents a checkpoint for message reconciliation
type ReconciliationCheckpoint struct {
	ID                   int
	SessionName          string
	ChatID               string
	LastMessageTimestamp int64
	LastMessageID        *string
	LastRunAt            time.Time
	Status               string
}

// ReconciliationCheckpointRepository handles checkpoint persistence for message reconciliation
type ReconciliationCheckpointRepository interface {
	// GetOrCreate gets an existing checkpoint or creates a new one with timestamp 0
	GetOrCreate(ctx context.Context, sessionName, chatID string) (*ReconciliationCheckpoint, error)
	// Get gets a checkpoint, returns nil if not found
	Get(ctx context.Context, sessionName, chatID string) (*ReconciliationCheckpoint, error)
	// MarkRunning marks a checkpoint as running (for exclusive lock)
	// Returns error if already running
	MarkRunning(ctx context.Context, sessionName, chatID string) error
	// MarkIdle marks a checkpoint as idle
	MarkIdle(ctx context.Context, sessionName, chatID string) error
	// MarkFailed marks a checkpoint as failed
	MarkFailed(ctx context.Context, sessionName, chatID string) error
	// UpdateCheckpoint updates the checkpoint with new timestamp and message ID
	UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error
	// GetBySession gets all checkpoints for a session
	GetBySession(ctx context.Context, sessionName string) ([]ReconciliationCheckpoint, error)
	// GetActive gets all checkpoints that are running (for recovery after restart)
	GetActive(ctx context.Context) ([]ReconciliationCheckpoint, error)
	// GetStale gets checkpoints that haven't been updated in a while
	GetStale(ctx context.Context, olderThan time.Duration) ([]ReconciliationCheckpoint, error)
	// Delete deletes a checkpoint
	Delete(ctx context.Context, sessionName, chatID string) error
	// DeleteBySession deletes all checkpoints for a session
	DeleteBySession(ctx context.Context, sessionName string) (int64, error)
	// ResetStaleCheckpoints resets checkpoints stuck in running status back to idle
	ResetStaleCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error)
}

// RedirectLinkRepository handles redirect link data operations
type RedirectLinkRepository interface {
	// GetByID retrieves a redirect link by its ID
	GetByID(ctx context.Context, linkID string) (*models.RedirectLink, error)
	// GetBySlug retrieves a redirect link by its slug
	GetBySlug(ctx context.Context, slug string) (*models.RedirectLink, error)
	// GetByUserID retrieves all redirect links for a user
	GetByUserID(ctx context.Context, userID string) ([]models.RedirectLink, error)
	// CheckSlugExists checks if a slug is already in use
	CheckSlugExists(ctx context.Context, slug string, excludeID string) (bool, error)
	// Create creates a new redirect link
	Create(ctx context.Context, link *models.RedirectLink) (*models.RedirectLink, error)
	// Update updates an existing redirect link
	Update(ctx context.Context, linkID string, updates map[string]interface{}) error
	// Delete deletes a redirect link
	Delete(ctx context.Context, linkID, userID string) error
	// IncrementClicks increments the click count and group-specific click count
	IncrementClicks(ctx context.Context, linkID, groupID string) error
}

// PlanRepository handles plan data operations for billing integration
type PlanRepository interface {
	// GetByKiwifyProductID finds a plan by its Kiwify product ID (monthly, semester, or annual)
	GetByKiwifyProductID(ctx context.Context, productID string) (*models.Plan, error)
	// GetBySlug finds a plan by its slug
	GetBySlug(ctx context.Context, slug string) (*models.Plan, error)
	// GetByID finds a plan by its ID
	GetByID(ctx context.Context, id string) (*models.Plan, error)
	// GetActivePlans returns all active plans
	GetActivePlans(ctx context.Context) ([]models.Plan, error)
	// GetAllPlans returns all plans (including inactive)
	GetAllPlans(ctx context.Context) ([]models.Plan, error)
	// Create creates a new plan
	Create(ctx context.Context, plan *models.Plan) (*models.Plan, error)
	// Update updates an existing plan
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	// Delete deletes a plan
	Delete(ctx context.Context, id string) error
}

// JobRepository handles standalone job data access (distinct from ScheduledJobRepository)
// JobListParams holds parameters for cursor-based job listing.
type JobListParams struct {
	UserID   string
	PageSize int
	Cursor   string // opaque cursor token (base64 of "scheduled_for|id")
}

// JobListResult holds the result of a cursor-based job listing.
type JobListResult struct {
	Jobs       []models.StandaloneJob
	NextCursor string
	TotalCount int
}

type JobRepository interface {
	GetByID(ctx context.Context, id string) (*models.StandaloneJob, error)
	GetByUserID(ctx context.Context, userID string, status *models.StandaloneJobStatus, limit int) ([]models.StandaloneJob, error)
	ListByUserID(ctx context.Context, params JobListParams) (*JobListResult, error)
	GetPendingDueJobs(ctx context.Context, limit int) ([]models.StandaloneJob, error)
	Create(ctx context.Context, job *models.StandaloneJob) (*models.StandaloneJob, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	UpdateStatus(ctx context.Context, id string, status models.StandaloneJobStatus, errorMsg *string) error
	ScheduleRetry(ctx context.Context, id string, nextRunAt time.Time) error
	Delete(ctx context.Context, id, userID string) error
	CountByStatus(ctx context.Context, userID string) (map[models.StandaloneJobStatus]int, error)
}

// MLLinkListRepository handles ML link list data operations
type MLLinkListRepository interface {
	// List operations
	GetListsByUserID(ctx context.Context, userID string) ([]models.MlLinkList, error)
	GetListByID(ctx context.Context, listID string) (*models.MlLinkList, error)
	GetListByIDAndUser(ctx context.Context, listID, userID string) (*models.MlLinkList, error)
	CreateList(ctx context.Context, userID string, list *models.MlLinkList) (*models.MlLinkList, error)
	UpdateList(ctx context.Context, listID string, updates map[string]interface{}) error
	DeleteList(ctx context.Context, listID, userID string) error

	// Item operations
	GetItemsByListID(ctx context.Context, listID string) ([]models.MlLinkListItem, error)
	GetItemsByListIDPaginated(ctx context.Context, listID string, pageSize int, cursor string) (items []models.MlLinkListItem, nextCursor string, totalCount int, err error)
	GetItemByID(ctx context.Context, itemID string) (*models.MlLinkListItem, error)
	CreateItem(ctx context.Context, item *models.MlLinkListItem) (*models.MlLinkListItem, error)
	BulkCreateItems(ctx context.Context, items []models.MlLinkListItem) ([]models.MlLinkListItem, error)
	UpdateItem(ctx context.Context, itemID string, updates map[string]interface{}) error
	DeleteItem(ctx context.Context, itemID string) error
	ResetConsumedItems(ctx context.Context, listID string) (int64, error)
	ReorderItems(ctx context.Context, listID string, orderMap map[string]int32) error
	GetMaxOrderIndex(ctx context.Context, listID string) (int, error)

	// Automation consumption operations
	GetUnconsumedItems(ctx context.Context, listID string, limit int) ([]models.MlLinkListItem, error)
	MarkConsumed(ctx context.Context, itemID, automationID string, consumedAt time.Time) error
}

// MlLinkListItemRepository manages individual items in ML link lists for LIST mode automation consumption
type MlLinkListItemRepository interface {
	// GetNextUnconsumed returns the next N unconsumed items from a list, ordered by position/order_index
	GetNextUnconsumed(ctx context.Context, listID string, limit int) ([]*models.MlLinkListItem, error)

	// MarkAsConsumed marks items as consumed with the automation ID and optional group ID
	MarkAsConsumed(ctx context.Context, itemIDs []string, automationID string, groupID *string) error

	// CountRemaining returns the count of unconsumed items in a list
	CountRemaining(ctx context.Context, listID string) (int, error)

	// GetByListID returns all items for a list ordered by position
	GetByListID(ctx context.Context, listID string) ([]*models.MlLinkListItem, error)

	// Create creates a new list item
	Create(ctx context.Context, item *models.MlLinkListItem) error

	// BulkCreate creates multiple list items in a single transaction
	BulkCreate(ctx context.Context, items []*models.MlLinkListItem) error

	// ResetConsumed resets consumed status for all items in a list
	ResetConsumed(ctx context.Context, listID string) (int64, error)

	// GetConsumedByAutomation returns all items consumed by a specific automation
	GetConsumedByAutomation(ctx context.Context, automationID string) ([]*models.MlLinkListItem, error)
}

// AutomationRunRepository manages automation execution locks and run records
type AutomationRunRepository interface {
	// GetLock retrieves the current lock status for an automation
	// Returns nil if no active lock exists
	GetLock(ctx context.Context, automationID string) (*time.Time, error)

	// AcquireLock attempts to acquire an execution lock for the specified duration
	// Returns the lock expiry time or nil if lock cannot be acquired (already locked)
	AcquireLock(ctx context.Context, automationID string, duration time.Duration) (*time.Time, error)

	// ReleaseLock releases the execution lock for an automation
	ReleaseLock(ctx context.Context, automationID string) error

	// CreateRun creates a new automation run record
	// Returns the run ID
	CreateRun(ctx context.Context, automationID string, status string) (string, error)

	// UpdateRunStatus updates the status of an automation run
	UpdateRunStatus(ctx context.Context, runID string, status string, errorMsg *string) error

	// UpdateRunStats updates the statistics of an automation run
	UpdateRunStats(ctx context.Context, runID string, offersFound, messagesSent, messagesFailed int) error

	// GetRunByID retrieves an automation run by ID
	GetRunByID(ctx context.Context, runID string) (*models.AutomationRun, error)

	// GetRunsByAutomationID retrieves recent runs for an automation
	GetRunsByAutomationID(ctx context.Context, automationID string, limit int) ([]models.AutomationRun, error)

	// CleanupOldRuns deletes old completed runs
	CleanupOldRuns(ctx context.Context, olderThan time.Duration) (int64, error)
}

// SyncJob represents a group synchronization job
type SyncJob struct {
	ID           string
	JobID        string
	UserID       string
	InstanceID   string
	InstanceName string
	State        string
	Progress     int
	TotalGroups  int
	SyncedGroups int
	ErrorMessage string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SyncJobState constants
const (
	SyncJobStatePending    = "pending"
	SyncJobStateInProgress = "in_progress"
	SyncJobStateCompleted  = "completed"
	SyncJobStateFailed     = "failed"
)

// SyncJobRepository handles sync job tracking for group synchronization
type SyncJobRepository interface {
	// Create creates a new sync job record
	Create(ctx context.Context, job *SyncJob) (*SyncJob, error)
	// GetByJobID retrieves a sync job by its Asynq job ID
	GetByJobID(ctx context.Context, jobID string) (*SyncJob, error)
	// UpdateState updates the job state
	UpdateState(ctx context.Context, jobID, state string) error
	// UpdateProgress updates the job progress
	UpdateProgress(ctx context.Context, jobID string, progress, syncedGroups, totalGroups int) error
	// MarkStarted marks the job as started
	MarkStarted(ctx context.Context, jobID string) error
	// MarkCompleted marks the job as completed
	MarkCompleted(ctx context.Context, jobID string, syncedGroups, totalGroups int) error
	// MarkFailed marks the job as failed with error message
	MarkFailed(ctx context.Context, jobID, errorMessage string) error
	// GetActiveJobsByUser returns active (pending/in_progress) jobs for a user
	GetActiveJobsByUser(ctx context.Context, userID string) ([]SyncJob, error)
	// Cleanup removes old completed/failed jobs
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
}

// CategoryRepository handles category and subcategory data operations
type CategoryRepository interface {
	// ListCategories returns all categories, optionally including inactive ones
	ListCategories(ctx context.Context, includeInactive bool) ([]models.Category, error)
	// ListCategoriesWithSubcategories returns categories with their subcategories loaded
	ListCategoriesWithSubcategories(ctx context.Context, includeInactive bool) ([]models.Category, error)
	// GetCategoryByID retrieves a category by its ID
	GetCategoryByID(ctx context.Context, id string) (*models.Category, error)
	// GetCategoryBySlug retrieves a category by its slug
	GetCategoryBySlug(ctx context.Context, slug string) (*models.Category, error)
	// CreateCategory creates a new category
	CreateCategory(ctx context.Context, req *models.CreateCategoryRequest) (*models.Category, error)
	// UpdateCategory updates an existing category
	UpdateCategory(ctx context.Context, id string, req *models.UpdateCategoryRequest) (*models.Category, error)
	// DeleteCategory deletes a category (only if not is_system)
	DeleteCategory(ctx context.Context, id string) error

	// GetSubcategoriesByCategoryID retrieves all subcategories for a category
	GetSubcategoriesByCategoryID(ctx context.Context, categoryID string) ([]models.Subcategory, error)
	// GetSubcategoryByID retrieves a subcategory by its ID
	GetSubcategoryByID(ctx context.Context, id string) (*models.Subcategory, error)
	// CreateSubcategory creates a new subcategory
	CreateSubcategory(ctx context.Context, req *models.CreateSubcategoryRequest) (*models.Subcategory, error)
	// UpdateSubcategory updates an existing subcategory
	UpdateSubcategory(ctx context.Context, id string, req *models.UpdateSubcategoryRequest) (*models.Subcategory, error)
	// DeleteSubcategory deletes a subcategory (only if not is_system)
	DeleteSubcategory(ctx context.Context, id string) error
}

// CTARepository handles CTA (Call-to-Action) data operations
type CTARepository interface {
	GetUserCTAs(ctx context.Context, userID string, filters map[string]interface{}) ([]models.CTA, error)
	GetCTAByID(ctx context.Context, ctaID, userID string) (*models.CTA, error)
	GetCTAsByIDs(ctx context.Context, ctaIDs []string, userID string) ([]models.CTA, error)
	CreateCTA(ctx context.Context, userID string, data *models.CTA) (*models.CTA, error)
	UpdateCTA(ctx context.Context, ctaID, userID string, updates map[string]interface{}) error
	DeleteCTA(ctx context.Context, ctaID, userID string) error
}
