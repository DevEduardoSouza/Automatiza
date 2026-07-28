package repository

import (
	"automatiza-go/internal/models"
	"context"
	"errors"
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
	// PauseUserAutomations pauses all active automations for a user (billing
	// suspension), marking them paused_by_billing. Returns the count paused.
	PauseUserAutomations(ctx context.Context, userID string) (int, error)
	// ResumeUserAutomations reactivates ONLY billing-paused automations. Symmetric
	// counterpart of PauseUserAutomations. Returns the count reactivated.
	ResumeUserAutomations(ctx context.Context, userID string) (int, error)
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
	TotalUsers    int
	ActiveUsers   int
	InactiveUsers int
	PlanStats     []PlanStat
	GrowthData    []GrowthPoint
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
	// SupersedePriorInstances marks past instance rows of the same user as
	// `superseded` once a fresh session connects. Targets two cohorts that
	// the UI no longer needs to display: zombies (status='disconnected'
	// with NULL/empty phone — abandoned QR scans that never authenticated)
	// and old sessions of the same phone that are no longer the active one.
	//
	// Excludes rows currently `connected` so a multi-phone account keeps
	// its other live chips. Returns the count of rows updated.
	SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error)
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
	// GetGroupsByWAGroupIDs is the batch counterpart of GetGroupByWAGroupID
	// — used by the offer_generate fan-out to look up participating chip
	// IDs for every destination group in one round-trip instead of N+1.
	GetGroupsByWAGroupIDs(ctx context.Context, userID string, waGroupIDs []string) ([]models.Group, error)
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
	// GetAdminInstanceIDsForGroups returns a map keyed by wa_group_id of the
	// chip ids (stringified) that administer each group (groups.admin_instance_ids).
	// Used by the bulk picture fan-out to route each group through a chip that
	// can actually change its photo.
	GetAdminInstanceIDsForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string][]string, error)
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
	// CleanupOldByActionType deletes rows of a given action_type older than olderThan,
	// across all users. Used by the retention cron to prune high-churn operational
	// history (e.g. "automation_run", ~97% of the table) without touching user-facing
	// actions of other types.
	CleanupOldByActionType(ctx context.Context, actionType string, olderThan time.Duration) (int64, error)
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

// CouponRuleRepository handles auto-apply rule data operations.
//
// FindMatchingCoupon is the hot path called from the offer dispatcher:
// pass in (userID, platform, productPrice) and get back the Coupon to
// inject into the message, or nil when no rule applies. Tie-break order
// is min_price DESC → priority DESC → created_at DESC, so an operator
// that designed tiered rules (R$30 OFF > R$200, R$50 OFF > R$400, etc.)
// gets the largest coupon the product clears.
type CouponRuleRepository interface {
	GetUserCouponRules(ctx context.Context, userID string) ([]models.CouponRule, error)
	GetCouponRuleByID(ctx context.Context, ruleID, userID string) (*models.CouponRule, error)
	CreateCouponRule(ctx context.Context, userID string, data *models.CouponRule) (*models.CouponRule, error)
	UpdateCouponRule(ctx context.Context, ruleID, userID string, updates map[string]interface{}) error
	DeleteCouponRule(ctx context.Context, ruleID, userID string) error
	// productIsOfficial flags Shopee Mall items so rules with
	// official_only=true can be filtered correctly. Pass false for
	// non-Shopee platforms or when the flag is unknown.
	// officialFirst, when true, sorts official_only rules ahead of the
	// min_price tie-break (rollout-gated); false keeps the historical
	// min_price-first ordering.
	// productCategoryIDs são os IDs oficiais de categoria da Shopee do
	// produto (ProductOfferV2.productCatIds). nil/vazio = produto sem
	// categoria: só regras genéricas ficam elegíveis. Regra específica
	// casa por interseção e tem precedência sobre qualquer genérica.
	FindMatchingCoupon(ctx context.Context, userID, platform string, productPrice float64, productIsOfficial, officialFirst bool, productCategoryIDs []int64) (*models.Coupon, error)
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
	// GetRecentByCanonicalKey looks for a recent sent/queued dispatch of the
	// same canonical_key within the given lookback window. Returns nil when
	// no match — caller treats nil as "free slot". Used by offer_generate
	// and monitoring dedup paths; the window is a time.Duration so each
	// caller can pick its own granularity (offer_generate uses 30min,
	// monitoring uses dedupe_hours * time.Hour from the automation config).
	GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) (*models.DispatchLog, error)
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
	// MarkStaleQueuedListLogs marca como 'error' dispatch de LIST preso em
	// 'queued' há mais de staleAfter (task perdida: redis/worker morreu antes
	// do desfecho), dentro da janela `window`. Alimenta o reconciliador.
	MarkStaleQueuedListLogs(ctx context.Context, staleAfter, window time.Duration) (int64, error)
	// GetListResurrectCandidates devolve os destinos FALHOS de itens de lista
	// consumidos mas nunca entregues (0 sent, 0 queued, ≥1 failed/error na
	// janela). O reconciliador usa para devolver o item e liberar os claims.
	GetListResurrectCandidates(ctx context.Context, window time.Duration) ([]ListResurrectCandidate, error)
	// MarkStaleQueuedOrphans marca como 'reconciled' os 'queued' órfãos há mais
	// de staleAfter (dentro de window) que NÃO são LIST-com-item — o gap deixado
	// pelo MarkStaleQueuedListLogs (search/monitoring/offer e list sem item).
	// Desfecho DESCONHECIDO: 'reconciled', nunca 'sent'. Higiene de backlog.
	MarkStaleQueuedOrphans(ctx context.Context, staleAfter, window time.Duration) (int64, error)
}

// ListResurrectCandidate é um destino falho de um item de lista consumido sem
// nenhuma entrega confirmada — insumo do ReconcileListDeliveries.
type ListResurrectCandidate struct {
	ListItemID   string
	UserID       string
	GroupID      *string
	CanonicalKey *string
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
	// CountByTypeAndActorSince counts events of eventType attributed to a given admin
	// (payload->>'admin_id') at or after `since`. Backs per-admin rate-limiting of bulk
	// operations (e.g. capping password resets to stop mass-reset abuse).
	CountByTypeAndActorSince(ctx context.Context, eventType, adminID string, since time.Time) (int, error)
}

// KiwifyEventRepository provides idempotency and audit for Kiwify webhooks.
type KiwifyEventRepository interface {
	// Claim atomically reserves an idempotency key. Returns claimed=true only for
	// the FIRST caller; concurrent/duplicate deliveries get claimed=false with the
	// prior status ("processing" | "done"). Backed by an INSERT ... ON CONFLICT.
	Claim(ctx context.Context, key, email, event string) (claimed bool, priorStatus string, err error)
	// Finish marks a claimed key as done (with the user and result) so future
	// duplicates are recognized as already processed.
	Finish(ctx context.Context, key, userID, result string) error
	// Release removes a claimed key after a processing failure, so the Kiwify
	// retry can reprocess the event instead of being deduped away.
	Release(ctx context.Context, key string) error
	// RecordBillingAudit appends a billing state-transition audit event.
	RecordBillingAudit(ctx context.Context, payload map[string]interface{}) error
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

// ErrInstagramAccountAlreadyConnected indica que a conta profissional (ou o id
// app-scoped) já pertence a OUTRO usuário local. Só as violações dos dois
// índices únicos de instagram_credentials viram este sentinel — qualquer outro
// 23505 continua subindo cru, para não mascarar defeito de schema.
var ErrInstagramAccountAlreadyConnected = errors.New("instagram account already connected to another user")

// InstagramCredentialsRepository handles per-user Instagram tokens (encrypted).
type InstagramCredentialsRepository interface {
	Get(ctx context.Context, userID string) (*models.InstagramCredentialsDB, error)
	// GetByAppScopedUserID resolve a credencial a partir do id que o
	// signed_request da Meta carrega (deauthorize / data deletion).
	GetByAppScopedUserID(ctx context.Context, appScopedUserID string) (*models.InstagramCredentialsDB, error)
	Save(ctx context.Context, userID, encryptedData, appScopedUserID, igUserID, username string, expiresAt time.Time) error
	Delete(ctx context.Context, userID string) error
	DeleteByAppScopedUserID(ctx context.Context, appScopedUserID string) error
	// ListExpiring returns credentials whose token expires before `before`
	// (used by the refresh cron to renew ~30d ahead).
	ListExpiring(ctx context.Context, before time.Time) ([]models.InstagramCredentialsDB, error)
}

// InstagramStoryPostRepository handles the story-publish ledger.
//
// O ledger — não o Redis — é a fonte de verdade da idempotência: GetOrCreate é
// atômico contra o índice único (user_id, source_group, message_id), então
// redelivery, corrida entre chips e Redis indisponível não conseguem publicar
// o mesmo Story duas vezes.
type InstagramStoryPostRepository interface {
	// GetOrCreate insere a linha do evento ou devolve a existente. created=true
	// somente quando ESTA chamada inseriu — é o sinal de "posso enfileirar".
	GetOrCreate(ctx context.Context, p *models.InstagramStoryPost) (row *models.InstagramStoryPost, created bool, err error)
	GetByID(ctx context.Context, id string) (*models.InstagramStoryPost, error)
	// TryStartProcessing reivindica o lease da linha num único UPDATE. Só pega
	// linha pending cujo lease esteja ausente ou mais velho que staleBefore.
	// Devolve o carimbo do lease adquirido — ele é o TOKEN de posse: só quem o
	// tem pode devolver o lease depois.
	TryStartProcessing(ctx context.Context, id string, staleBefore time.Time) (claimed bool, lease time.Time, err error)
	// ReleaseProcessing só libera se o lease ainda for o que `lease` descreve.
	// Sem isso, um worker zumbi (contexto cancelado no timeout, goroutine viva)
	// libera o lease de quem o tomou legitimamente depois dele.
	ReleaseProcessing(ctx context.Context, id string, lease time.Time) error
	SaveContainerID(ctx context.Context, id, containerID string) error
	MarkResult(ctx context.Context, id, status, igMediaID, errorCode, safeMessage string, reconciled bool) error
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramStoryPost, error)
	CountPublishedSince(ctx context.Context, userID string, since time.Time) (int, error)
	DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error)
}

// InstagramAppRepository guarda o app da Meta que cada cliente registrou.
//
// GetByWebhookToken é o método que destrava o webhook multi-tenant: a Meta
// assina o corpo com o app secret do cliente, mas quem identifica o cliente
// está DENTRO do corpo. Dando a cada um uma URL própria com este token, o
// servidor resolve o segredo antes de ler qualquer byte do corpo.
type InstagramAppRepository interface {
	Get(ctx context.Context, userID string) (*models.InstagramApp, error)
	GetByWebhookToken(ctx context.Context, webhookToken string) (*models.InstagramApp, error)
	// Upsert grava/atualiza o app do usuário. webhookToken e verifyToken só são
	// gerados na PRIMEIRA gravação: trocá-los depois quebraria a URL que o
	// cliente já configurou no painel da Meta.
	Upsert(ctx context.Context, userID, appID, encryptedAppSecret string) (*models.InstagramApp, error)
	MarkVerified(ctx context.Context, userID string) error
	Delete(ctx context.Context, userID string) error
}

// InstagramInboundRepository guarda os eventos que chegam pelo webhook da Meta.
type InstagramInboundRepository interface {
	// Create insere o evento. created=false significa que o provider_event_id
	// já existia — a Meta reenviou, e reprocessar responderia duas vezes o
	// mesmo comentário.
	Create(ctx context.Context, e *models.InstagramInboundEvent) (row *models.InstagramInboundEvent, created bool, err error)
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramInboundEvent, error)
	MarkStatus(ctx context.Context, id, status, errorCode string) error
	DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error)
}

// InstagramAIConfigRepository guarda o treinamento da IA por usuário.
type InstagramAIConfigRepository interface {
	Get(ctx context.Context, userID string) (*models.InstagramAIConfig, error)
	Upsert(ctx context.Context, cfg *models.InstagramAIConfig) (*models.InstagramAIConfig, error)
}

// InstagramReplyRepository guarda o que a IA respondeu e o custo.
type InstagramReplyRepository interface {
	// Claim reserva o direito de responder ESTE evento. created=false significa
	// que já existe resposta para ele — é a segunda trava contra responder duas
	// vezes o mesmo comentário (a primeira é o unique do inbound), e aqui o
	// efeito colateral é público: aparece no perfil do cliente.
	Claim(ctx context.Context, userID, providerEventID, eventType string) (row *models.InstagramReply, created bool, err error)
	MarkResult(ctx context.Context, id, status, replyText, errorCode string, promptTokens, completionTokens int) error
	// CountSince conta respostas ENVIADAS na janela — base da cota diária.
	CountSince(ctx context.Context, userID string, since time.Time) (int, error)
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramReply, error)
}

// InstagramPlanCapRepository expõe a cota diária de resposta do plano do
// usuário. Interface mínima de propósito: o pacote Instagram não precisa
// conhecer plano nem perfil, só o teto.
type InstagramPlanCapRepository interface {
	MaxAIRepliesPerDay(ctx context.Context, userID string) (int, error)
}

// InstagramDataDeletionRepository grava os comprovantes de exclusão exigidos
// pela Meta e executa a exclusão em si.
type InstagramDataDeletionRepository interface {
	// Process apaga ledger + credencial do sujeito e grava o comprovante NUMA
	// ÚNICA transação. Sujeito desconhecido também recebe comprovante concluído
	// — responder diferente revelaria se a conta existe aqui.
	Process(ctx context.Context, appScopedUserID, subjectHash, candidateConfirmationCode string, expiresAt time.Time) (*models.InstagramDataDeletionRequest, error)
	GetByConfirmationCode(ctx context.Context, confirmationCode string, now time.Time) (*models.InstagramDataDeletionRequest, error)
	DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}

// WebPushSubscriptionRepository handles browser push subscriptions (PWA).
type WebPushSubscriptionRepository interface {
	// Upsert stores or refreshes a subscription (unique by endpoint). An endpoint
	// that changes owner follows the latest login.
	Upsert(ctx context.Context, sub *models.WebPushSubscription) error
	ListByUser(ctx context.Context, userID string) ([]models.WebPushSubscription, error)
	ListAll(ctx context.Context) ([]models.WebPushSubscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	// DeleteFailedBefore removes subscriptions marked failed before `before`.
	DeleteFailedBefore(ctx context.Context, before time.Time) (int64, error)
	CountByUser(ctx context.Context, userID string) (int, error)
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
	// MarkProcessedEnsure finaliza em 1 upsert cobrindo linha ausente —
	// caminho feliz do modo lean-persist (WEBHOOK_LEAN_PERSIST).
	MarkProcessedEnsure(ctx context.Context, eventID, sessionName, eventType string) error
	MarkFailed(ctx context.Context, eventID, errorMessage string) error
	MarkDLQ(ctx context.Context, eventID, errorMessage string) error
	IncrementRetryCount(ctx context.Context, eventID string) error
	GetPendingEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error)
	GetFailedEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
	CleanupUnrecoverable(ctx context.Context, olderThan time.Duration) (int64, error)
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
	NewMembers int // = Joins, mantido pelo contrato proto já existente
	NewGroups  int
	Joins      int // entradas observadas no bucket (SUM joins_count)
	Leaves     int // saídas observadas no bucket (SUM leaves_count)
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

// ErrDuplicatePendingJob indica que a operação criaria um segundo agendamento
// logicamente idêntico (mesmo usuário, ação, horário e payload) com o primeiro
// ainda pending. É estado do usuário, não falha do sistema.
var ErrDuplicatePendingJob = errors.New("já existe um agendamento idêntico pendente para este horário")

type JobRepository interface {
	GetByID(ctx context.Context, id string) (*models.StandaloneJob, error)
	GetByUserID(ctx context.Context, userID string, status *models.StandaloneJobStatus, limit int) ([]models.StandaloneJob, error)
	ListByUserID(ctx context.Context, params JobListParams) (*JobListResult, error)
	// ClaimPendingDueJobs marca como 'running' e devolve na MESMA transação os
	// jobs vencidos. Reivindicar e mudar o status em statements separados abria
	// janela para dois ticks/réplicas pegarem o mesmo job e enviarem duas vezes.
	ClaimPendingDueJobs(ctx context.Context, limit int) ([]models.StandaloneJob, error)
	// ReleaseClaimedJobs devolve para 'pending' jobs reivindicados que não
	// chegaram a ser executados (ctx do cron morreu no meio do lote). Sem isso
	// eles ficariam 'running' até o reaper marcá-los como failed 30min depois —
	// perda silenciosa de um agendamento que só precisava do próximo tick.
	ReleaseClaimedJobs(ctx context.Context, ids []string) (int64, error)
	// Create é idempotente: agendamento logicamente idêntico (mesmo usuário,
	// ação, horário e payload) com job ainda pending devolve o job existente
	// com deduped=true em vez de criar uma segunda linha.
	Create(ctx context.Context, job *models.StandaloneJob) (created *models.StandaloneJob, deduped bool, err error)
	// CreateBatch cria o lote inteiro numa única transação: item inválido
	// desfaz tudo, itens duplicados devolvem o job já existente.
	CreateBatch(ctx context.Context, jobs []*models.StandaloneJob) (created []models.StandaloneJob, dedupedCount int, err error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	UpdateStatus(ctx context.Context, id string, status models.StandaloneJobStatus, errorMsg *string) error
	ScheduleRetry(ctx context.Context, id string, nextRunAt time.Time) error
	Delete(ctx context.Context, id, userID string) error
	CountByStatus(ctx context.Context, userID string) (map[models.StandaloneJobStatus]int, error)
	// FailStaleRunning marca 'running' órfãos (>olderThan) como failed —
	// crash/timeout no meio do job não pode deixá-lo rodando para sempre.
	FailStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error)
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
	GetItemsByListIDPaginated(ctx context.Context, listID string, pageSize int, cursor string) (items []models.MlLinkListItem, nextCursor string, totalCount, availableCount, sentCount int, err error)
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

	// ResetConsumedByIDs devolve itens específicos ao estado não-consumido
	// (consumed_at=NULL). Usado pelo reconciliador quando um item foi
	// consumido no enfileiramento mas NENHUM envio se confirmou.
	ResetConsumedByIDs(ctx context.Context, itemIDs []string) (int64, error)

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

	// ReapStaleRunningLocks fails 'running' rows whose execution lock expired
	// more than `grace` ago (runs killed before releaseExecutionLock could run).
	// Returns the number of rows reaped.
	ReapStaleRunningLocks(ctx context.Context, grace time.Duration) (int64, error)
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

// ErrAmbiguousTrackingTenant é devolvido quando o nome da sessão WAHA resolve
// para MAIS de uma instância. `instances` só garante UNIQUE(user_id,
// instance_id), então nomes repetidos entre usuários são possíveis: resolver
// "o primeiro" atribuiria o webhook ao tenant errado. Ambiguidade é falha de
// segurança, não caso normal — o chamador não escreve nada.
var ErrAmbiguousTrackingTenant = errors.New("ambiguous tracking tenant")

// GroupMemberRepository é a fronteira de persistência da fundação histórica de
// membros de grupo (Melhoria 2, Fase A).
//
// Invariantes:
//   - todo método de mutação recebe o tenant/user_id e o inclui no predicado;
//   - group_id NUNCA vem calculado de fora: é resolvido aqui por
//     (user_id, wa_group_id);
//   - ledger, projeção e agregado diário de um envelope mudam na MESMA
//     transação — não existe commit de ledger sem projeção;
//   - a reconciliação nunca escreve no ledger (lá só entram fatos de webhook).
type GroupMemberRepository interface {
	// ResolveWebhookTenant resolve o nome da sessão WAHA para no máximo uma
	// instância. Zero linhas devolve erro TRANSITÓRIO (a instância pode estar
	// sendo criada); duas ou mais devolvem ErrAmbiguousTrackingTenant.
	ResolveWebhookTenant(ctx context.Context, sessionName string) (*models.TrackingTenant, error)

	// RecordWebhookBatch aplica um envelope inteiro atomicamente: insere os
	// fatos (idempotente por (user_id, provider_event_id, participant_hash)),
	// projeta apenas as linhas efetivamente inseridas e incrementa o agregado
	// diário. Replay do mesmo envelope é no-op.
	RecordWebhookBatch(ctx context.Context, tenant models.TrackingTenant, event models.GroupMemberWebhookEvent) (models.RecordBatchResult, error)

	// ClaimGroupsForReconciliation reivindica até limit grupos elegíveis com
	// FOR UPDATE SKIP LOCKED, escolhendo de forma determinística um chip
	// conectado participante (preferindo o owner). Claims mais antigos que
	// lease voltam a ser elegíveis.
	ClaimGroupsForReconciliation(ctx context.Context, limit int, lease time.Duration) ([]models.GroupReconciliationCandidate, error)

	// ReconcileGroup aplica a fotografia de participantes de um grupo. Só
	// altera o estado de projeções cujo last_event_applied_at seja anterior a
	// observationStartedAt — um webhook aplicado durante a chamada WAHA nunca
	// é desfeito por uma foto mais velha.
	ReconcileGroup(ctx context.Context, candidate models.GroupReconciliationCandidate, members []models.GroupReconciliationMember, observationStartedAt time.Time) error

	// ReleaseReconciliationClaim libera o claim de um grupo que falhou.
	ReleaseReconciliationClaim(ctx context.Context, groupID string) error

	// CleanupOldGroupMemberEvents apaga fatos com occurred_at anterior a
	// olderThan, em lotes limitados. Devolve o total apagado.
	CleanupOldGroupMemberEvents(ctx context.Context, olderThan time.Time, batchSize, maxBatches int) (int64, error)

	// CleanupOldGroupMemberSnapshots apaga agregados com snapshot_date
	// anterior a olderThan, em lotes limitados. Devolve o total apagado.
	CleanupOldGroupMemberSnapshots(ctx context.Context, olderThan time.Time, batchSize, maxBatches int) (int64, error)

	// GetMemberMetrics monta o pacote de leitura da tela de métricas, escopado
	// pelo tenant. groupID vazio devolve o agregado da conta. Nunca troca
	// "não observado" por zero: sem baseline, os campos vêm nil.
	GetMemberMetrics(ctx context.Context, userID string, days int, groupID string) (*models.GroupMemberMetrics, error)
}

// =============================================================================
// Melhoria 5 — chip reserva e failover de instância WhatsApp
// =============================================================================

// ErrFailoverInProgress indica que uma operação de failover não terminal já
// cobre esta automação: o gate está fechado e nenhum produtor pode iniciar
// enqueue/envio novo por ela.
var ErrFailoverInProgress = errors.New("failover operation in progress for this automation")

// ErrFailoverVersionConflict indica que a versão esperada da política não bate
// (outra sessão/réplica mudou a configuração no meio).
var ErrFailoverVersionConflict = errors.New("failover policy version conflict")

// ErrFailoverIdempotencyMismatch indica replay da mesma idempotency key com
// payload diferente.
var ErrFailoverIdempotencyMismatch = errors.New("failover idempotency key reused with a different request")

// ErrFailoverConcurrentOperation indica que o usuário já tem outra operação
// não terminal (chave diferente).
var ErrFailoverConcurrentOperation = errors.New("another failover operation is already running for this user")

// ErrFailoverStaleConfiguration indica que o conjunto elegível ou algum
// fingerprint mudou entre o ensaio e a aplicação. Aborta tudo, nunca aplica
// parcialmente.
var ErrFailoverStaleConfiguration = errors.New("failover configuration changed since the dry run")

// ErrFailoverIndeterminate indica estado que não admite inferência segura: a
// política/operação vai para `failed`, o gate continua fechado e a correção é
// humana e forward-only.
var ErrFailoverIndeterminate = errors.New("failover state is indeterminate")

// InstanceFailoverRole é o papel de um chip numa política.
type InstanceFailoverRole struct {
	InstanceID int
	PolicyID   int64
	UserID     string
	Role       string
}

// FailoverAutomationRouting é a projeção MÍNIMA de `automations` que o
// failover precisa: identidade, escopo e roteamento. Nenhuma outra coluna é
// lida — e nenhuma outra é escrita.
type FailoverAutomationRouting struct {
	Automation models.Automation
	Name       string
	Status     string
}

// SetFailoverPolicyInput descreve uma criação ou atualização de política.
// PolicyID zero cria; caso contrário atualiza exigindo ExpectedVersion exata.
type SetFailoverPolicyInput struct {
	PolicyID                  int64
	UserID                    string
	ActorUserID               string
	PrimaryInstanceID         int
	BackupInstanceID          int
	Mode                      string
	ExpectedVersion           int64
	IdempotencyKey            string
	RequestHash               string
	UnhealthyAfterSeconds     *int
	RequiredConsecutiveProbes *int
	DrainDeadlineSeconds      *int
	ProposalTTLSeconds        *int
}

// DisableFailoverPolicyInput descreve a remoção de uma política.
type DisableFailoverPolicyInput struct {
	PolicyID        int64
	UserID          string
	ActorUserID     string
	ExpectedVersion int64
	IdempotencyKey  string
	RequestHash     string
}

// FailoverPolicyMutation é o resultado de SetPolicy/DisablePolicy. Replay
// devolve Replayed=true e o evento gravado da primeira execução.
type FailoverPolicyMutation struct {
	Policy   *models.InstanceFailoverPolicy
	Event    *models.InstanceFailoverEvent
	Replayed bool
}

// FailoverProbeInput é uma observação do detector assisted. NUNCA incrementa
// `version`: telemetria de saúde a cada minuto não pode invalidar dry-runs.
type FailoverProbeInput struct {
	PolicyID int64
	UserID   string
	// Healthy=true zera timestamp e contador. Healthy=false com Valid=true
	// acumula. Valid=false (WAHA indisponível/indeterminado) também zera —
	// incerteza nunca conta como falha.
	Healthy bool
	Valid   bool
	Status  string
	Now     time.Time
	// RequireDetectorLock faz a transação tentar
	// pg_try_advisory_xact_lock('instance-failover-detector:<policy>') antes de
	// qualquer outro lock. Sem o lock, a réplica pula sem erro (nil,nil,nil):
	// é o que garante UM evento por política com duas réplicas rodando o mesmo
	// cron.
	RequireDetectorLock bool
}

// FailoverCloseGateInput fecha o gate de uma operação: vincula os itens,
// muda a política para switching/reverting e marca o deadline de drain.
type FailoverCloseGateInput struct {
	OperationID     string
	PolicyID        int64
	UserID          string
	ActorUserID     string
	ExpectedVersion int64
	ExpectedState   string
	NewState        string
	Items           []models.InstanceFailoverOperationItem
	DrainDeadlineAt time.Time
	RequestedEvent  string
	GateClosedEvent string
}

// FailoverApplyInput aplica a projeção de TODAS as automações do snapshot numa
// única transação. Qualquer divergência de fingerprint aborta a transação
// inteira antes da primeira escrita.
type FailoverApplyInput struct {
	OperationID string
	PolicyID    int64
	UserID      string
	ActorUserID string
	// Estado exigido da política (switching ou reverting).
	ExpectedState string
	Items         []models.InstanceFailoverOperationItem
	// Direção do fingerprint conferido antes de escrever: para apply é
	// FingerprintBefore, para restore é FingerprintAfter.
	ExpectFingerprintAfter bool
	// Result gravado nos itens: applied ou restored.
	ItemResult string
	Event      string
}

// FailoverTerminateInput leva a operação a um estado terminal e devolve a
// política ao estado combinado.
type FailoverTerminateInput struct {
	OperationID string
	PolicyID    int64
	UserID      string
	ActorUserID string
	// completed | aborted | failed
	OperationStatus string
	// Estado final da política. Vazio mantém o estado atual (caso failed).
	PolicyState string
	Event       string
	ReasonCodes []string
	Report      []byte
}

// FailoverLeaseInput pede uma lease de dispatch.
type FailoverLeaseInput struct {
	UserID       string
	AutomationID string
	HolderID     string
	TTL          time.Duration
}

// ListFailoverEventsInput pagina o ledger por cursor (created_at,id).
type ListFailoverEventsInput struct {
	PolicyID int64
	UserID   string
	Cursor   string
	Limit    int
}

// InstanceFailoverRepository é o acesso a dados da Melhoria 5.
//
// Os métodos transacionais têm granularidade GROSSA de propósito: a ordem
// global de locks (advisory do usuário → policy → operation → automações por
// UUID ascendente → ledger) precisa valer dentro de UMA transação, e deixar
// isso para o serviço montar convidaria a deadlock entre réplicas.
type InstanceFailoverRepository interface {
	// --- leitura de política ---
	GetPolicy(ctx context.Context, policyID int64, userID string) (*models.InstanceFailoverPolicy, error)
	ListPolicies(ctx context.Context, userID string) ([]models.InstanceFailoverPolicy, error)
	// GetRoleByInstance devolve o papel do chip em QUALQUER política (sem
	// escopo de usuário): é o precheck dos handlers de delete de instância.
	GetRoleByInstance(ctx context.Context, instanceID int) (*InstanceFailoverRole, error)
	ListAssistedPolicies(ctx context.Context) ([]models.InstanceFailoverPolicy, error)

	// --- mutação de política ---
	SetPolicy(ctx context.Context, in SetFailoverPolicyInput) (*FailoverPolicyMutation, error)
	DisablePolicy(ctx context.Context, in DisableFailoverPolicyInput) (*FailoverPolicyMutation, error)
	// RecordProbe grava telemetria do detector e, quando os limiares batem,
	// promove a política para proposed. Devolve a política resultante.
	RecordProbe(ctx context.Context, in FailoverProbeInput) (*models.InstanceFailoverPolicy, *models.InstanceFailoverEvent, error)
	// ExpireProposals volta para active as propostas vencidas.
	ExpireProposals(ctx context.Context, now time.Time) (int, error)

	// --- dry-run ---
	FindDryRunByKey(ctx context.Context, userID, direction, key string) (*models.InstanceFailoverDryRun, error)
	GetDryRun(ctx context.Context, dryRunID, userID string) (*models.InstanceFailoverDryRun, error)
	ListDryRunItems(ctx context.Context, dryRunID string) ([]models.InstanceFailoverDryRunItem, error)
	SaveDryRun(ctx context.Context, dr *models.InstanceFailoverDryRun, items []models.InstanceFailoverDryRunItem, ev *models.InstanceFailoverEvent) error

	// --- operações ---
	InsertOperation(ctx context.Context, op *models.InstanceFailoverOperation) (*models.InstanceFailoverOperation, bool, error)
	GetOperation(ctx context.Context, operationID, userID string) (*models.InstanceFailoverOperation, error)
	FindOperationByKey(ctx context.Context, userID, kind, key string) (*models.InstanceFailoverOperation, error)
	GetNonTerminalOperation(ctx context.Context, userID string) (*models.InstanceFailoverOperation, error)
	ListNonTerminalOperations(ctx context.Context) ([]models.InstanceFailoverOperation, error)
	ListOperationItems(ctx context.Context, operationID string) ([]models.InstanceFailoverOperationItem, error)
	GetLastCompletedOperation(ctx context.Context, policyID int64, userID, kind string) (*models.InstanceFailoverOperation, error)
	CloseGate(ctx context.Context, in FailoverCloseGateInput) (*models.InstanceFailoverOperation, error)
	ApplyProjection(ctx context.Context, in FailoverApplyInput) error
	TerminateOperation(ctx context.Context, in FailoverTerminateInput) (*models.InstanceFailoverPolicy, error)

	// --- gate de dispatch ---
	// AcquireLease insere a lease SE nenhuma operação não terminal contiver a
	// automação. Devolve ErrFailoverInProgress quando contém.
	AcquireLease(ctx context.Context, in FailoverLeaseInput) (string, error)
	RenewLease(ctx context.Context, leaseID string, ttl time.Duration) (bool, error)
	ReleaseLease(ctx context.Context, leaseID string) error
	CountActiveLeases(ctx context.Context, automationIDs []string, now time.Time) (int, error)
	CleanupLeases(ctx context.Context, before time.Time) (int64, error)
	// CountNonTerminalDispatches conta dispatch_logs pending/queued criados
	// até `before` para as automações do snapshot.
	CountNonTerminalDispatches(ctx context.Context, automationIDs []string, before time.Time) (int, error)

	// --- automações (leitura de roteamento) ---
	ListUserAutomationsRouting(ctx context.Context, userID string) ([]FailoverAutomationRouting, error)
	// GetOperationForAutomations devolve a operação não terminal do usuário
	// que contenha qualquer uma das automações informadas — é o guard de
	// create/update/delete de automação.
	FindBlockingOperationForAutomations(ctx context.Context, userID string, automationIDs []string) (*models.InstanceFailoverOperation, error)

	// --- ledger ---
	InsertEvent(ctx context.Context, ev *models.InstanceFailoverEvent) (*models.InstanceFailoverEvent, error)
	GetLastEvent(ctx context.Context, policyID int64, userID string) (*models.InstanceFailoverEvent, error)
	ListEvents(ctx context.Context, in ListFailoverEventsInput) ([]models.InstanceFailoverEvent, string, error)
}

// MonitorOfferRetryRepository é o ledger de ofertas/campanhas do monitor
// (Melhoria 9, migração 079). Guarda o estado POR LINK COMERCIAL de uma
// mensagem-fonte, e por destino de cada saída.
//
// Não substitui dispatch_logs: aquela é histórico por destino, volumosa, e o
// status dela não responde "esta mensagem já produziu sua oferta". Aqui vivem
// as unique keys que fazem redelivery de webhook virar no-op e o crash
// "WAHA aceitou e o processo morreu" virar `ambiguous` visível em vez de
// duplicata.
//
// Duas regras valem para TODA implementação desta interface, iguais às da
// MonitorCadenceRepository: o relógio é o do PostgreSQL
// (transaction_timestamp()), e toda transição é CAS — um worker antigo
// acordando depois de um lease reemitido faz zero linhas e vira no-op.
type MonitorOfferRetryRepository interface {
	// --- criação ---

	// CreateJobWithItems grava job + itens sintáticos numa ÚNICA transação,
	// ANTES de qualquer I/O externo. created=false significa que a identidade
	// da mensagem já existia: o chamador NÃO chama API nem envia nada — o dono
	// vivo ou o sweep concluem o job. É este retorno que torna redelivery de
	// webhook idempotente.
	CreateJobWithItems(ctx context.Context, job *models.MonitorOfferJob, items []models.MonitorOfferItem) (bool, error)

	// GetJob lê um job escopado por tenant. Outro tenant recebe not-found —
	// nunca "forbidden" com dados dentro.
	GetJob(ctx context.Context, userID, jobID string) (*models.MonitorOfferJob, error)
	ListItems(ctx context.Context, jobID string) ([]models.MonitorOfferItem, error)

	// --- ciclo de tentativa ---

	// ClaimDueItems reivindica itens pending vencidos (FOR UPDATE SKIP LOCKED),
	// incrementa enqueue_generation e grava lease na MESMA transação. Só o item
	// de MENOR ordinal por job entra enquanto o job não tem produto escolhido:
	// é isso que torna determinística a corrida de dois unknowns que depois
	// viram produto.
	ClaimDueItems(ctx context.Context, limit int) ([]models.MonitorOfferItemLease, error)

	// ClaimItemForAttempt é o CAS do handler: (id, generation, status=pending)
	// → processing. Generation obsoleta ou status terminal devolve nil, nil.
	ClaimItemForAttempt(ctx context.Context, itemID string, generation int) (*models.MonitorOfferAttempt, error)

	// MarkAttemptStarted carimba attempt_started_at e incrementa attempts.
	// Separado do claim de propósito: crash ANTES deste marcador devolve o item
	// a pending sem consumir tentativa.
	MarkAttemptStarted(ctx context.Context, itemID string) error

	// FinishItem aplica o desfecho e recomputa o status do job na MESMA
	// transação.
	FinishItem(ctx context.Context, out models.MonitorOfferItemOutcome) error

	// RescheduleItem devolve o item a pending. consumeAttempt=false para os
	// casos que NÃO gastam tentativa de negócio (fora da janela, sem chip,
	// shutdown).
	RescheduleItem(ctx context.Context, itemID string, nextAttemptAt time.Time, reasonCode string, consumeAttempt bool) error

	// PublishCanonicalKey resolve a corrida de duas ocorrências do mesmo
	// produto: SELECT job FOR UPDATE, menor ordinal vence. Depois que a key
	// pertence a uma saída preparada/enviada ela nunca troca de dono.
	PublishCanonicalKey(ctx context.Context, itemID string, canonicalKey string) (models.CanonicalKeyVerdict, error)

	// --- entrega por destino ---

	// UpsertDelivery cria a linha em 'prepared'. Idempotente na PK.
	UpsertDelivery(ctx context.Context, d models.MonitorOfferDelivery) error

	// MarkDeliverySending é o CAS imediatamente ANTES do POST na WAHA. false
	// significa que outro processo já assumiu ou o desfecho já é terminal.
	MarkDeliverySending(ctx context.Context, jobID string, outputKey, destinationGroupID string, token string) (bool, error)

	// FinishDelivery grava o desfecho. sent e ambiguous são ABSORVENTES: nunca
	// voltam para sending, porque reenviar um 'sent' duplica no grupo do
	// cliente e reenviar um 'ambiguous' pode duplicar.
	FinishDelivery(ctx context.Context, jobID string, outputKey, destinationGroupID string, token string, status, reasonCode string, providerMessageID *string) error

	ListDeliveries(ctx context.Context, jobID string, outputKey string) ([]models.MonitorOfferDelivery, error)

	// --- manutenção ---

	// ReapStaleLeases: 'prepared' órfã vira failed/delivery_rejected (prova que
	// nenhum POST começou); 'sending' vencida vira ambiguous (NUNCA pending);
	// item em processing sem attempt_started_at volta a pending sem consumir
	// tentativa.
	ReapStaleLeases(ctx context.Context) (models.MonitorOfferReapStats, error)

	// ExpirePayloads limpa ciphertext vencido (6h) e marca item ainda ativo
	// como exhausted/payload_expired.
	ExpirePayloads(ctx context.Context) (int, error)

	// DeleteTerminalOlderThan apaga metadata terminal em lotes.
	DeleteTerminalOlderThan(ctx context.Context, olderThan time.Duration, batch int) (int, error)

	// --- replay manual ---

	// GetItemForReplay lê item + job escopados por tenant.
	GetItemForReplay(ctx context.Context, userID, itemID string) (*models.MonitorOfferItem, *models.MonitorOfferJob, error)

	// ResetItemForReplay zera attempts e lease, PRESERVA total_attempts,
	// incrementa replay_count e enqueue_generation, e agenda agora.
	ResetItemForReplay(ctx context.Context, userID, itemID string, allowAmbiguous bool) error
}

// MonitorCadenceRepository é o ledger durável da cadência do espelho
// (Melhoria 7, migração 078). PostgreSQL é a primitiva de verdade: Redis/Asynq
// podem ser zerados e todo pending não terminal continua recuperável daqui.
type MonitorCadenceRepository interface {
	// --- admissão ---

	// HasNonTerminalPending diz se alguma das automações ainda tem item na
	// fila. Usado para continuar enfileirando durante o dreno (intervalo
	// voltou a zero) em vez de deixar mensagem nova ultrapassar a fila.
	HasNonTerminalPending(ctx context.Context, automationIDs []string) (bool, error)

	// Admit persiste payload + reservas numa ÚNICA transação. Ou todas as
	// automações elegíveis ficam reservadas, ou nenhuma fica. Devolve
	// models.ErrMonitorBacklogFull quando alguma estourou o teto de fila.
	Admit(ctx context.Context, in models.MonitorAdmissionInput) (*models.MonitorAdmissionResult, error)

	// --- outbox (poller) ---

	// ClaimDueHeads reivindica, por CAS, o head devido de até `limit`
	// automações distintas. Réplicas concorrentes não colidem (SKIP LOCKED).
	ClaimDueHeads(ctx context.Context, limit int) ([]models.MonitorPendingLease, error)
	MarkEnqueued(ctx context.Context, pendingID int64, generation int, token string) error
	// ReleaseEnqueueLease devolve o item a pending quando o enqueue falhou.
	ReleaseEnqueueLease(ctx context.Context, pendingID int64, generation int, token string, notBefore time.Time) error

	// --- worker ---

	// ClaimPending faz o CAS enqueuing|enqueued -> processing. Zero linhas
	// (nil, nil) significa task velha/replay: o worker retorna sem efeito.
	ClaimPending(ctx context.Context, pendingID int64, generation int, token string) (*models.MonitorPendingWork, error)
	// MarkPendingStarted grava first_started_at e o last_dispatch_started_at
	// do estado na MESMA transação — é o que impede rajada de catch-up.
	MarkPendingStarted(ctx context.Context, pendingID int64, processingToken string) error
	// ReschedulePending devolve o item a pending (janela fechou, destino
	// retentável) mexendo APENAS em not_before_at.
	ReschedulePending(ctx context.Context, pendingID int64, processingToken string, notBefore time.Time, code string) error
	FinalizePending(ctx context.Context, pendingID int64, processingToken, status string, result models.MonitorPendingResult, code string) error

	// MaterializeDestinations cria o ledger por destino na primeira execução e
	// devolve as linhas (existentes, se já materializado) em ordem de ordinal.
	MaterializeDestinations(ctx context.Context, pendingID int64, dests []models.MonitorPendingDestination) ([]models.MonitorPendingDestination, error)
	ListDestinations(ctx context.Context, pendingID int64) ([]models.MonitorPendingDestination, error)
	ClaimDestination(ctx context.Context, destinationID int64, token string) (bool, error)
	// MarkDestinationSending é a fronteira do resultado incerto.
	MarkDestinationSending(ctx context.Context, destinationID int64, token string) (bool, error)
	FinishDestination(ctx context.Context, destinationID int64, status, outcomeCode string, notBefore *time.Time) error

	// --- manutenção ---

	RecoverLeases(ctx context.Context) (models.MonitorRecoveryStats, error)
	// CancelForAutomation cancela itens/destinos não iniciados de uma
	// automação (desativada, tipo trocado ou apagada).
	CancelForAutomation(ctx context.Context, automationID string) (int, error)
	Cleanup(ctx context.Context, before time.Time, batch int) (int, error)

	// --- API/UI ---

	GetMonitorQueueSummaries(ctx context.Context, userID string, automationIDs []string) (map[string]models.MonitorQueueSummary, error)
}
