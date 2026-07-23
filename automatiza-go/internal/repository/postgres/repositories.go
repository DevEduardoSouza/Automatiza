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
	InstagramCredentials     repository.InstagramCredentialsRepository
	InstagramStoryPost       repository.InstagramStoryPostRepository
	InstagramDataDeletion    repository.InstagramDataDeletionRepository
	InstagramApp             repository.InstagramAppRepository
	InstagramInbound         repository.InstagramInboundRepository
	InstagramAIConfig        repository.InstagramAIConfigRepository
	InstagramReply           repository.InstagramReplyRepository
	InstagramPlanCap         repository.InstagramPlanCapRepository
	WebPushSubscription      repository.WebPushSubscriptionRepository
	GroupMember              repository.GroupMemberRepository
	InstanceFailover         repository.InstanceFailoverRepository
	MonitorCadence           repository.MonitorCadenceRepository
	MonitorOfferRetry        repository.MonitorOfferRetryRepository
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
		InstagramCredentials:     NewInstagramCredentialsRepository(db, logger),
		InstagramStoryPost:       NewInstagramStoryPostRepository(db, logger),
		InstagramDataDeletion:    NewInstagramDataDeletionRepository(db, logger),
		InstagramApp:             NewInstagramAppRepository(db, logger),
		InstagramInbound:         NewInstagramInboundRepository(db, logger),
		InstagramAIConfig:        NewInstagramAIConfigRepository(db, logger),
		InstagramReply:           NewInstagramReplyRepository(db, logger),
		InstagramPlanCap:         NewInstagramPlanCapRepository(db, logger),
		WebPushSubscription:      NewWebPushSubscriptionRepository(db, logger),
		GroupMember:              NewGroupMemberRepository(db, logger),
		InstanceFailover:         NewInstanceFailoverRepository(db, logger),
		MonitorCadence:           NewMonitorCadenceRepository(db, logger),
		MonitorOfferRetry:        NewMonitorOfferRetryRepository(db, logger),
	}
}
