package handlers

import (
	"context"
	"net/url"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/metrics"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/automation"
	"automatiza-go/internal/services/coupons"
	"automatiza-go/internal/services/jobs"
	"automatiza-go/internal/services/kiwify"
	"automatiza-go/internal/services/loadbalancer"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/monitoring"
	"automatiza-go/internal/services/redirect"
	"automatiza-go/internal/services/shopee"
	"automatiza-go/internal/services/user"
	"automatiza-go/internal/workers"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Note: config import is kept because Dependencies.Config uses it

// BackpressureChecker checks whether the user is allowed to enqueue more messages
// and records successful enqueues. Satisfied by *backpressure.ProducerBackpressure.
type BackpressureChecker interface {
	CanEnqueue(ctx context.Context, userID string) (bool, string, error)
	RecordEnqueue(ctx context.Context, userID string) error
}

// Dependencies holds all handler dependencies
type Dependencies struct {
	Config           *config.Config
	Cache            *cache.Cache
	Logger           *zap.Logger
	QueueClient      *workers.QueueClient
	URLSafetyService *urlsafety.Service
	Backpressure     BackpressureChecker

	// SSE Hub for real-time notifications
	SSEHub *SSEHub

	// WhatsApp Provider (adapter pattern - all handlers use this)
	WhatsAppProvider whatsapp.WhatsAppProvider

	// Services
	ShopeeService     shopee.Service
	MLService         mercadolivre.Service
	AutomationService automation.Service
	MonitoringService monitoring.Service
	JobsService       jobs.Service
	UserService       user.Service
	CouponsService    coupons.Service
	LBService         loadbalancer.Service
	RedirectService   redirect.Service
	KiwifyService     kiwify.Service

	// Repositories
	ProfileRepo       repository.ProfileRepository
	InstanceRepo      repository.InstanceRepository
	GroupRepo         repository.GroupRepository
	AutomationRepo    repository.AutomationRepository
	ActionHistoryRepo repository.ActionHistoryRepository
	SettingsRepo      repository.SettingsRepository
	CouponRepo        repository.CouponRepository
	DispatchLogRepo   repository.DispatchLogRepository
	ScheduledJobRepo  repository.ScheduledJobRepository
	SystemEventRepo   repository.SystemEventRepository
	UserRoleRepo      repository.UserRoleRepository
	WebhookEventsRepo repository.WebhookEventsRepository

	// Metrics (optional - nil for no metrics collection)
	WebhookMetrics *metrics.WebhookMetrics
}

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, deps *Dependencies) {
	// API version group
	api := r.Group("/api/v1")

	// Health check (public)
	api.GET("/health", HealthCheck(deps))

	// Sentry tunnel (public, no auth) — proxies envelopes through same origin
	// to bypass ad blockers that block direct requests to *.sentry.io
	if deps.Config.SentryDSN != "" {
		if parsed, err := url.Parse(deps.Config.SentryDSN); err == nil && parsed.Host != "" {
			tunnelHandler := NewSentryTunnelHandler(parsed.Host, deps.Logger)
			api.POST("/sentry-tunnel", tunnelHandler.Handle)
		}
	}

	// Create auth middleware using repositories (PostgreSQL)
	authMW := middleware.AuthMiddlewareWithRepos(deps.ProfileRepo, deps.UserRoleRepo, deps.Logger)
	optionalAuthMW := middleware.OptionalAuthMiddlewareWithRepos(deps.ProfileRepo, deps.UserRoleRepo, deps.Logger)

	// WhatsApp routes (auth required) - new canonical routes
	whatsappHandler := NewWhatsAppHandler(deps)
	waRoutes := api.Group("/whatsapp")
	waRoutes.Use(authMW)
	{
		waRoutes.POST("/create-instance", whatsappHandler.CreateInstance)
		waRoutes.POST("/delete-instance", whatsappHandler.DeleteInstance)
		waRoutes.POST("/reconnect-instance", whatsappHandler.ReconnectInstance)
		waRoutes.GET("/check-status", whatsappHandler.CheckStatus)
		waRoutes.GET("/fetch-groups", whatsappHandler.FetchGroups)
		waRoutes.POST("/sync-groups", whatsappHandler.SyncGroups)
		waRoutes.GET("/sync-groups/status", whatsappHandler.SyncGroupsStatus)
		waRoutes.GET("/fetch-group-participants", whatsappHandler.FetchGroupParticipants)
		waRoutes.POST("/send-message", whatsappHandler.SendMessage)
		waRoutes.POST("/send-audio", whatsappHandler.SendAudio)
		waRoutes.POST("/send-poll", whatsappHandler.SendPoll)
		waRoutes.PUT("/update-group-subject", whatsappHandler.UpdateGroupSubject)
		waRoutes.PUT("/update-group-description", whatsappHandler.UpdateGroupDescription)
		waRoutes.PUT("/update-group-picture", whatsappHandler.UpdateGroupPicture)
		waRoutes.PUT("/update-group-settings", whatsappHandler.UpdateGroupSettings)
	}

	// Groups routes
	groups := api.Group("/groups")
	groups.Use(authMW)
	{
		groupsHandler := NewGroupsHandler(deps)
		groups.GET("/", groupsHandler.List)
		groups.GET("/:id", groupsHandler.Get)
	}

	// Automations routes
	automations := api.Group("/automations")
	automations.Use(authMW)
	{
		automationsHandler := NewAutomationsHandler(deps)
		automations.GET("/", automationsHandler.List)
		automations.GET("/:id", automationsHandler.Get)
		automations.POST("/", automationsHandler.Create)
		automations.PUT("/:id", automationsHandler.Update)
		automations.DELETE("/:id", automationsHandler.Delete)
		automations.POST("/diagnose", automationsHandler.Diagnose)
		automations.POST("/run", automationsHandler.Run)
		automations.POST("/reset-history", automationsHandler.ResetHistory)
	}

	// Settings routes
	settings := api.Group("/settings")
	settings.Use(authMW)
	{
		settingsHandler := NewSettingsHandler(deps)
		settings.GET("/", settingsHandler.Get)
		settings.PUT("/", middleware.AdminMiddleware(), settingsHandler.Update)
	}

	// History routes
	historyHandler := NewHistoryHandler(deps)
	api.GET("/history", authMW, historyHandler.List)

	// Contacts routes
	contacts := api.Group("/contacts")
	contacts.Use(authMW)
	{
		contactsHandler := NewContactsHandler(deps)
		contacts.POST("/extract", contactsHandler.Extract)
	}

	// Shopee routes
	shopee := api.Group("/shopee")
	shopee.Use(authMW)
	{
		shopeeHandler := NewShopeeHandler(deps)
		shopee.POST("/search-offers", shopeeHandler.SearchOffers)
		shopee.POST("/generate-affiliate-link", shopeeHandler.GenerateAffiliateLink)
	}

	// Mercado Livre routes
	ml := api.Group("/mercadolivre")
	ml.Use(authMW)
	{
		mlHandler := NewMLHandler(deps)
		ml.POST("/create-link", mlHandler.CreateLink)
		ml.POST("/preview", mlHandler.Preview)
		ml.POST("/connect", mlHandler.Connect)
	}

	// Jobs routes
	jobs := api.Group("/jobs")
	{
		jobsHandler := NewJobsHandler(deps)
		jobs.POST("/process-scheduled", optionalAuthMW, jobsHandler.ProcessScheduled)
		jobs.GET("/queue-status", authMW, jobsHandler.QueueStatus)
	}

	// Webhooks (public)
	webhooks := api.Group("/webhooks")
	{
		webhooksHandler := NewWebhooksHandler(deps)
		webhooks.POST("/whatsapp", webhooksHandler.WhatsApp)
		webhooks.POST("/kiwify", webhooksHandler.Kiwify)
		webhooks.POST("/kiwify/simulate", authMW, webhooksHandler.KiwifySimulate)

		// WAHA webhook handler with HMAC validation, idempotency, and rate limiting
		// WAHA Plus webhook handler with HMAC validation, idempotency, and rate limiting
		if deps.WebhookEventsRepo != nil && deps.QueueClient != nil {
			// Rate limiting for webhook endpoint (H2 security fix)
			webhookRateLimitConfig := middleware.DefaultWebhookRateLimitConfig()
			webhookRateLimitConfig.Logger = deps.Logger

			wahaHandler := NewWAHAWebhookHandler(
				deps.Config.WebhookSecret,
				deps.WebhookEventsRepo,
				deps.QueueClient,
				deps.Logger,
				deps.WebhookMetrics,
			)
			webhooks.POST("/waha",
				middleware.WebhookRateLimitMiddleware(webhookRateLimitConfig),
				wahaHandler.HandleWebhook,
			)
		}
	}

	// SSE (Server-Sent Events) for real-time notifications
	if deps.SSEHub != nil {
		sse := api.Group("/sse")
		{
			sseHandler := NewSSEHandler(deps.SSEHub, deps.Logger)
			// SSE endpoint uses query param auth since EventSource doesn't support headers
			sse.GET("/events", middleware.QueryTokenAuthMiddleware(deps.ProfileRepo, deps.UserRoleRepo, deps.Logger), sseHandler.HandleSSE)
		}
	}

	// Redirect (public) - no auth required
	// Uses the new RedirectPublicHandler for processing redirect links
	if deps.RedirectService != nil {
		redirectPublicHandler := NewRedirectPublicHandler(deps.RedirectService, deps.Logger)
		api.POST("/redirect", redirectPublicHandler.Process)
	} else {
		// Fallback to old stub handler if service not configured
		redirectHandler := NewRedirectHandler(deps)
		api.POST("/redirect", redirectHandler.Process)
	}

	// Instances (internal)
	instances := api.Group("/instances")
	{
		instancesHandler := NewInstancesHandler(deps)
		instances.POST("/:id/webhook/sync", instancesHandler.SyncWebhook)
		instances.POST("/webhook/sync-all", instancesHandler.SyncAllWebhooks)
	}

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(authMW, middleware.AdminMiddleware())
	{
		adminHandler := NewAdminHandler(deps)
		admin.POST("/users/:userId/reset-password", adminHandler.ResetPassword)
		admin.GET("/observability", adminHandler.Observability)
		admin.POST("/ensure-webhooks", adminHandler.EnsureWebhooks)
		admin.GET("/shopee-affiliate-audit", adminHandler.ShopeeAffiliateAudit)
		admin.GET("/system-events", adminHandler.SystemEvents)
	}

	// Coupons routes
	coupons := api.Group("/coupons")
	coupons.Use(authMW)
	{
		couponsHandler := NewCouponsHandler(deps)
		coupons.GET("/", couponsHandler.List)
		coupons.GET("/:id", couponsHandler.Get)
		coupons.GET("/store/:store", couponsHandler.GetByStore)
		coupons.POST("/", couponsHandler.Create)
		coupons.PUT("/:id", couponsHandler.Update)
		coupons.DELETE("/:id", couponsHandler.Delete)
	}

	// Load Balancer routes
	lb := api.Group("/loadbalancer")
	lb.Use(authMW)
	{
		lbHandler := NewLoadBalancerHandler(deps)
		lb.GET("/stats", lbHandler.Stats)
		lb.GET("/user/:userId", lbHandler.GetUserServer)
		lb.POST("/migrate", lbHandler.Migrate)
	}

	// 404 handler
	r.NoRoute(middleware.NotFoundHandler())
}

// HealthCheck handler
func HealthCheck(deps *Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"version": deps.Config.APIVersion,
			"env":     deps.Config.NodeEnv,
		})
	}
}
