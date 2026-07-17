package postgres

import (
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Repositories holds all PostgreSQL repository instances
type Repositories struct {
	Profile                  repository.ProfileRepository
	Instance                 repository.InstanceRepository
	Group                    repository.GroupRepository
	Automation               repository.AutomationRepository
	DispatchLog              repository.DispatchLogRepository
	ActionHistory            repository.ActionHistoryRepository
	ScheduledJob             repository.ScheduledJobRepository
	Job                      repository.JobRepository
	CTA                      repository.CTARepository
	Coupon                   repository.CouponRepository
	CouponRule               repository.CouponRuleRepository
	Affiliate                repository.AffiliateCredentialsRepository
	MLCredentials            repository.MLCredentialsRepository
	MLLink                   repository.MLLinkRepository
	MLLinkList               repository.MLLinkListRepository
	Settings                 repository.SettingsRepository
	SystemEvent              repository.SystemEventRepository
	UserServer               repository.UserServerRepository
	UserRole                 repository.UserRoleRepository
	EmailTemplate            repository.EmailTemplateRepository
	WebhookLog               WebhookLogRepository
	WebhookEvents            repository.WebhookEventsRepository
	Dashboard                repository.DashboardRepository
	PasswordResetToken       repository.PasswordResetTokenRepository
	RateLimit                repository.RateLimitRepository
	RedirectLink             repository.RedirectLinkRepository
	Plan                     repository.PlanRepository
	Category                 repository.CategoryRepository
	AutomationRun            repository.AutomationRunRepository
	MlLinkListItem           repository.MlLinkListItemRepository
	ReconciliationCheckpoint repository.ReconciliationCheckpointRepository
	SyncJob                  repository.SyncJobRepository
	KiwifyEvent              repository.KiwifyEventRepository
}

// NewRepositories creates all PostgreSQL repositories
func NewRepositories(db *DB, logger *zap.Logger) *Repositories {
	return &Repositories{
		Profile:                  NewProfileRepository(db, logger),
		Instance:                 NewInstanceRepository(db, logger),
		Group:                    NewGroupRepository(db, logger),
		Automation:               NewAutomationRepository(db, logger),
		DispatchLog:              NewDispatchLogRepository(db, logger),
		ActionHistory:            NewActionHistoryRepository(db, logger),
		ScheduledJob:             NewScheduledJobRepository(db, logger),
		Job:                      NewJobRepository(db, logger),
		CTA:                      NewCTARepository(db, logger),
		Coupon:                   NewCouponRepository(db, logger),
		CouponRule:               NewCouponRuleRepository(db, logger),
		Affiliate:                NewAffiliateCredentialsRepository(db, logger),
		MLCredentials:            NewMLCredentialsRepository(db, logger),
		MLLink:                   NewMLLinkRepository(db, logger),
		MLLinkList:               NewMLLinkListRepository(db, logger),
		Settings:                 NewSettingsRepository(db, logger),
		SystemEvent:              NewSystemEventRepository(db, logger),
		UserServer:               NewUserServerRepository(db, logger),
		UserRole:                 NewUserRoleRepository(db, logger),
		EmailTemplate:            NewEmailTemplateRepository(db, logger),
		WebhookLog:               NewWebhookLogRepository(db, logger),
		WebhookEvents:            NewWebhookEventsRepository(db, logger),
		Dashboard:                NewDashboardRepository(db, logger),
		PasswordResetToken:       NewPasswordResetTokenRepository(db, logger),
		RateLimit:                NewRateLimitRepository(db, logger),
		RedirectLink:             NewRedirectLinkRepository(db, logger),
		Plan:                     NewPlanRepository(db, logger),
		Category:                 NewCategoryRepository(db, logger),
		AutomationRun:            NewAutomationRunRepository(db, logger),
		MlLinkListItem:           NewMlLinkListItemRepository(db, logger),
		ReconciliationCheckpoint: NewReconciliationCheckpointRepository(db, logger),
		SyncJob:                  NewSyncJobRepository(db, logger),
		KiwifyEvent:              NewKiwifyEventRepository(db, logger),
	}
}
