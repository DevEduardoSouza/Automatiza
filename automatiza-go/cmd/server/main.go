package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/auth"
	"automatiza-go/internal/config"
	"automatiza-go/internal/connectrpc"
	"automatiza-go/internal/gen/admin/v1/adminv1connect"
	"automatiza-go/internal/gen/affiliates/v1/affiliatesv1connect"
	aiv1connect "automatiza-go/internal/gen/ai/v1/aiv1connect"
	"automatiza-go/internal/gen/auth/v1/authv1connect"
	"automatiza-go/internal/gen/automations/v1/automationsv1connect"
	"automatiza-go/internal/gen/coupons/v1/couponsv1connect"
	"automatiza-go/internal/gen/ctas/v1/ctasv1connect"
	"automatiza-go/internal/gen/dashboard/v1/dashboardv1connect"
	dispatchtextsv1connect "automatiza-go/internal/gen/dispatch_texts/v1/dispatch_textsv1connect"
	"automatiza-go/internal/gen/groups/v1/groupsv1connect"
	"automatiza-go/internal/gen/history/v1/historyv1connect"
	"automatiza-go/internal/gen/instances/v1/instancesv1connect"
	"automatiza-go/internal/gen/jobs/v1/jobsv1connect"
	mllinksv1connect "automatiza-go/internal/gen/ml_links/v1/ml_linksv1connect"
	"automatiza-go/internal/gen/plans/v1/plansv1connect"
	"automatiza-go/internal/gen/profiles/v1/profilesv1connect"
	redirectv1connect "automatiza-go/internal/gen/redirect/v1/redirectv1connect"
	"automatiza-go/internal/gen/settings/v1/settingsv1connect"
	"automatiza-go/internal/gen/webhooks/v1/webhooksv1connect"
	"automatiza-go/internal/gen/whatsapp/v1/whatsappv1connect"
	"automatiza-go/internal/handlers"
	"automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/repository/postgres"
	"automatiza-go/internal/services/ai"
	"automatiza-go/internal/services/automation"
	"automatiza-go/internal/services/coupons"
	"automatiza-go/internal/services/cron"
	"automatiza-go/internal/services/ctas"
	"automatiza-go/internal/services/dashboard"
	"automatiza-go/internal/services/email"
	"automatiza-go/internal/services/jobs"
	"automatiza-go/internal/services/kiwify"
	"automatiza-go/internal/services/loadbalancer"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/mllinklists"
	"automatiza-go/internal/services/monitoring"
	"automatiza-go/internal/services/offer_generate"
	"automatiza-go/internal/services/reconciliation"
	"automatiza-go/internal/services/redirect"
	"automatiza-go/internal/services/shopee"
	"automatiza-go/internal/services/user"
	"automatiza-go/internal/startup"
	"automatiza-go/internal/workers"

	emailadapter "automatiza-go/internal/adapters/email"
	profileservice "automatiza-go/internal/services/profile"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	// Parse flags
	workerMode := flag.Bool("worker", false, "Run in worker mode")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize Sentry BEFORE logger so early panics are captured
	if cfg.SentryDSN != "" {
		if err := observability.InitSentry(observability.SentryConfig{
			DSN:         cfg.SentryDSN,
			Environment: cfg.NodeEnv,
			Release:     "automatiza-go@" + buildVersion(),
			Debug:       cfg.IsDevelopment(),
			SampleRate:  1.0,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize Sentry: %v\n", err)
			// Non-fatal: continue without Sentry
		}
	}
	defer sentry.Flush(2 * time.Second)

	// Initialize OpenTelemetry (after Sentry, before logger)
	serviceName := "automatiza-server"
	if *workerMode {
		serviceName = "automatiza-worker"
	}
	otelShutdown, otelLogProvider, otelErr := observability.InitOTel(context.Background(), observability.OTelConfig{
		ServiceName:    serviceName,
		ServiceVersion: buildVersion(),
		Environment:    cfg.NodeEnv,
		OTLPEndpoint:   cfg.OTLPEndpoint,
		Enabled:        cfg.OTelEnabled,
	})
	if otelErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize OpenTelemetry: %v\n", otelErr)
		// Non-fatal: continue without OTel
	} else if cfg.OTelEnabled {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "OTel shutdown error: %v\n", err)
			}
		}()
	}

	// Initialize custom business metrics (no-op when OTel disabled)
	if err := observability.InitMetrics(cfg.OTelEnabled); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize OTel metrics: %v\n", err)
		// Non-fatal: continue without custom metrics
	}

	// WaitGroup available for tracking background goroutines
	wg := &sync.WaitGroup{}
	_ = wg // available for SafeGo callers that need coordination

	// Create logger (with OTel log bridge when enabled)
	logger, err := config.NewLogger(cfg, otelLogProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			// In containers, syncing /dev/stdout or /dev/stderr fails with "invalid argument"
			// because they are pipes, not regular files. This is expected and safe to ignore.
			if isStdioSyncErr(syncErr) {
				return
			}
			observability.LogAndCapture(logger, sentry.LevelWarning, "logger sync failed on shutdown", syncErr, map[string]string{"module": "main", "flow": "shutdown"})
		}
	}()

	logger.Info("Starting application",
		zap.String("env", cfg.NodeEnv),
		zap.String("port", cfg.Port),
		zap.Bool("workerMode", *workerMode),
		zap.Bool("otelEnabled", cfg.OTelEnabled))

	if cfg.OTelEnabled {
		logger.Info("OpenTelemetry initialized",
			zap.String("service", serviceName),
			zap.String("endpoint", cfg.OTLPEndpoint))
	}

	// Initialize token HMAC key for secure token hashing
	hmacKey := cfg.GetTokenHMACKey()
	if hmacKey != nil {
		if err := auth.InitTokenHMAC(hmacKey); err != nil {
			logger.Error("Failed to initialize token HMAC - using fallback",
				zap.Error(err))
		} else {
			logger.Info("Token HMAC initialized successfully")
		}
	} else {
		logger.Warn("TOKEN_HMAC_KEY not configured - token hashing will use fallback SHA-256 (less secure)")
	}

	// Create Redis client
	redisClient, err := config.NewRedisClient(cfg)
	if err != nil {
		logger.Warn("Failed to connect to Redis, continuing without cache",
			zap.Error(err))
	} else {
		defer func() {
			if err := redisClient.Close(); err != nil {
				observability.LogAndCapture(logger, sentry.LevelWarning, "redis close failed on shutdown", err,
					map[string]string{"module": "main", "flow": "shutdown"})
			}
		}()
	}

	// Create PostgreSQL connection
	pgCfg := cfg.GetPostgresConfig()
	port, portErr := strconv.Atoi(pgCfg.Port)
	if portErr != nil {
		logger.Error("Invalid DB_PORT value, must be a number", zap.String("port", pgCfg.Port), zap.Error(portErr))
		sentry.CaptureException(fmt.Errorf("startup: invalid DB_PORT %q: %w", pgCfg.Port, portErr))
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}
	db, err := postgres.NewDB(context.Background(), &postgres.Config{
		Host:     pgCfg.Host,
		Port:     port,
		User:     pgCfg.User,
		Password: pgCfg.Password,
		Database: pgCfg.Database,
		SSLMode:  pgCfg.SSLMode,
		MaxConns: int32(pgCfg.MaxConns),
	}, config.NewModuleLogger(logger, "postgres"))
	if err != nil {
		logger.Error("Failed to connect to PostgreSQL", zap.Error(err))
		sentry.CaptureException(fmt.Errorf("startup: failed to connect to PostgreSQL: %w", err))
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}
	defer db.Close()

	// Create cache
	var cacheClient *cache.Cache
	if redisClient != nil {
		cacheClient = cache.NewCache(redisClient.Client, cfg.AppCachePrefix, cfg.DisableAppRedisCache)
	}

	// Create repositories (PostgreSQL)
	repos := postgres.NewRepositories(db, logger)

	// Create Waha config provider (using PostgreSQL settings repo)
	wahaConfigProvider := config.NewWahaConfigProvider(cfg, repos.Settings)

	// Create WhatsApp adapter (implements whatsapp.WhatsAppProvider)
	// Use static config first, fall back to config provider if needed
	wahaAPIURL := cfg.WahaAPIURL
	wahaAPIKey := cfg.WahaAPIKey
	if wahaAPIURL == "" || wahaAPIKey == "" {
		// Try to get from config provider (requires context)
		wahaConfig, err := wahaConfigProvider.GetConfig(context.Background())
		if err != nil {
			logger.Warn("Failed to get WAHA config from provider", zap.Error(err))
		} else if wahaConfig != nil {
			wahaAPIURL = wahaConfig.APIURL
			wahaAPIKey = wahaConfig.APIKey
		}
	}

	var wahaAdapter whatsapp.WhatsAppProvider
	if wahaAPIURL != "" && wahaAPIKey != "" {
		// W27: Validate WebhookSecret is not empty in production
		if cfg.IsProduction() && cfg.WebhookSecret == "" {
			sentry.CaptureException(fmt.Errorf("CRITICAL: WebhookSecret is empty in production — webhooks are unprotected"))
			logger.Error("WebhookSecret is empty in production — HMAC validation will be disabled")
		}

		adapter, err := whatsapp.NewWahaAdapter(whatsapp.WahaAdapterConfig{
			APIURL:        wahaAPIURL,
			APIKey:        wahaAPIKey,
			WebhookSecret: cfg.WebhookSecret,
			// WebhookBaseURL intentionally omitted: WAHA's global WHATSAPP_HOOK_URL
			// env var (docker-compose) handles webhook delivery via Docker internal
			// network. Setting per-session webhooks caused duplicate delivery.
		}, config.NewModuleLogger(logger, "waha-adapter"))
		if err != nil {
			logger.Error("Failed to create WAHA adapter", zap.Error(err))
			sentry.CaptureException(fmt.Errorf("startup: failed to create WAHA adapter: %w", err))
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
		wahaAdapter = adapter
	} else {
		logger.Warn("WAHA not configured, WhatsApp adapter disabled")
	}

	// Shorthand references for services
	profileRepo := repos.Profile
	instanceRepo := repos.Instance
	groupRepo := repos.Group
	automationRepo := repos.Automation
	actionHistoryRepo := repos.ActionHistory
	settingsRepo := repos.Settings
	ctaRepo := repos.CTA
	couponRepo := repos.Coupon
	couponRuleRepo := repos.CouponRule
	dispatchLogRepo := repos.DispatchLog
	scheduledJobRepo := repos.ScheduledJob
	systemEventRepo := repos.SystemEvent
	userRoleRepo := repos.UserRole
	affiliateRepo := repos.Affiliate
	mlCredentialsRepo := repos.MLCredentials
	mlLinkRepo := repos.MLLink
	userServerRepo := repos.UserServer

	// Create URL safety service
	urlSafetyService := urlsafety.NewService(urlsafety.Config{
		Mode:             cfg.LinkSafetyMode,
		BlockedHosts:     cfg.GetLinkSafetyBlockedHosts(),
		AllowedHosts:     cfg.GetLinkSafetyAllowedHosts(),
		EnforceAllowlist: cfg.LinkSafetyEnforceAllowlist,
	}, cacheClient, config.NewModuleLogger(logger, "urlsafety"))

	// Create backpressure manager
	backpressureManager := backpressure.NewProducerBackpressure(backpressure.Config{
		DelayMS:         cfg.ProducerBackpressureDelayMS,
		UserMaxEnqueued: cfg.ProducerUserMaxEnqueued10Min,
		WindowMinutes:   10,
		Enabled:         true,
	}, cacheClient, config.NewModuleLogger(logger, "backpressure"))

	// Get Redis address for queue
	redisAddr := "localhost:6379"
	if redisClient != nil {
		redisAddr = redisClient.Options().Addr
	}

	// Create queue client (needed for services)
	queueClient := workers.NewQueueClient(redisAddr, config.NewModuleLogger(logger, "queue"))
	defer func() {
		if err := queueClient.Close(); err != nil {
			observability.LogAndCapture(logger, sentry.LevelWarning, "queue client close failed on shutdown", err,
				map[string]string{"module": "main", "flow": "shutdown"})
		}
	}()

	// Create services
	shopeeService := shopee.NewShopeeService(
		cacheClient,
		affiliateRepo,
		settingsRepo,
		config.NewModuleLogger(logger, "shopee"),
	)

	mlService := mercadolivre.NewMLService(
		cacheClient,
		mlCredentialsRepo,
		mlLinkRepo,
		cfg.MLCredentialsEncryptionKey,
		config.NewModuleLogger(logger, "ml"),
	)

	aiService := ai.NewAIService(
		cfg.AIApiURL,
		cfg.AIApiKey,
		cfg.AIModel,
		cacheClient,
		config.NewModuleLogger(logger, "ai"),
	)

	// Create email service (SMTP Gmail) - reads config from database with env fallback
	var fallbackEmailConfig *email.Config
	if cfg.HasSMTP() {
		fallbackEmailConfig = &email.Config{
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			User:      cfg.SMTPUser,
			Password:  cfg.SMTPPassword,
			FromEmail: cfg.SMTPFromEmail,
			FromName:  cfg.SMTPFromName,
		}
	}

	// Use dynamic email service that reads SMTP config from database
	// Falls back to environment variables if database settings are not available
	emailService := email.NewDynamicEmailService(
		settingsRepo,
		fallbackEmailConfig,
		config.NewModuleLogger(logger, "email"),
	)

	userService := user.NewUserService(
		profileRepo,
		emailService,
		config.NewModuleLogger(logger, "user"),
	)

	jobsService := jobs.NewJobsService(
		scheduledJobRepo,
		automationRepo,
		instanceRepo,
		queueClient,
		cacheClient,
		config.NewModuleLogger(logger, "jobs"),
	)

	// Create SSE Hub for real-time notifications (created early so it can be passed to services)
	sseHub := handlers.NewSSEHub(config.NewModuleLogger(logger, "sse-hub"))
	defer sseHub.Close()

	// Create standalone jobs service for new job management (8 action types)
	standaloneJobsService := jobs.NewService(
		repos.Job,
		instanceRepo,
		groupRepo,
		wahaAdapter,
		sseHub,
		actionHistoryRepo,
		sseHub, // historyNotifier: publishes history.change SSE events
		config.NewModuleLogger(logger, "standalone-jobs"),
	)

	lbService := loadbalancer.NewLoadBalancerService(
		userServerRepo,
		settingsRepo,
		cacheClient,
		config.NewModuleLogger(logger, "loadbalancer"),
	)

	couponsService := coupons.NewCouponsService(
		couponRepo,
		cacheClient,
		config.NewModuleLogger(logger, "coupons"),
	)

	ctasService := ctas.NewCTAsService(
		ctaRepo,
		cacheClient,
		config.NewModuleLogger(logger, "ctas"),
	)

	automationService := automation.NewAutomationService(
		automationRepo,
		repos.AutomationRun,
		instanceRepo,
		groupRepo,
		dispatchLogRepo,
		actionHistoryRepo,
		settingsRepo,
		profileRepo,
		repos.MlLinkListItem,
		repos.MLLinkList,
		ctaRepo,
		shopeeService,
		mlService,
		urlSafetyService,
		backpressureManager,
		queueClient,
		cacheClient,
		couponRepo,
		couponRuleRepo,
		config.NewModuleLogger(logger, "automation"),
	)

	// Create automation scheduler for cron-based execution
	automationScheduler := automation.NewScheduler(
		automationRepo,
		automationService,
		config.NewModuleLogger(logger, "automation-scheduler"),
	)

	// Create cron service with automation scheduler + cleanup repos
	// Uses standaloneJobsService to process jobs from the 'jobs' table (standalone jobs)
	cronService := cron.NewCronServiceFull(
		standaloneJobsService,
		automationScheduler,
		dispatchLogRepo,
		repos.WebhookEvents,
		config.NewModuleLogger(logger, "cron"),
	)

	// Register WAHA failed session cleanup job (P0: reclaims RAM from zombie sessions)
	if concreteAdapter, ok := wahaAdapter.(*whatsapp.WahaAdapter); ok {
		cronService.RegisterTask("cleanup_waha_failed_sessions", 1*time.Hour, func(ctx context.Context) error {
			deleted, err := concreteAdapter.CleanupFailedSessions(ctx)
			if err != nil {
				return err
			}
			if deleted > 0 {
				logger.Info("WAHA session cleanup completed",
					zap.Int("deleted", deleted))
			}
			return nil
		})

		// Reconciliação de status de instância: corrige instâncias travadas em
		// 'pending'/'disconnected' no banco quando a sessão está WORKING no WAHA
		// (acontece a cada restart/deploy, quando eventos session.status se
		// perdem). Sem isso, os envios afetados ficam bloqueados.
		statusReconciler := reconciliation.NewInstanceStatusReconciler(
			instanceRepo,
			concreteAdapter,
			config.NewModuleLogger(logger, "status-reconcile"),
		)
		// Periódico (2 min) — pega qualquer drift contínuo.
		cronService.RegisterTask("reconcile_instance_statuses", 2*time.Minute, func(ctx context.Context) error {
			_, err := statusReconciler.Reconcile(ctx)
			return err
		})
		// No boot — fecha a janela do deploy (eventos perdidos durante o restart).
		observability.SafeGo("startup.reconcile-instance-statuses", func() {
			sctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if _, err := statusReconciler.Reconcile(sctx); err != nil {
				logger.Warn("startup instance-status reconcile failed", zap.Error(err))
			}
		})
	}

	monitoringService := monitoring.NewMonitoringService(
		automationRepo,
		instanceRepo,
		groupRepo,
		dispatchLogRepo,
		shopeeService,
		mlService,
		wahaAdapter,
		cacheClient,
		config.NewModuleLogger(logger, "monitoring"),
	)

	var ogCouponHandler offer_generate.CouponApplier
	if couponRepo != nil {
		ogCouponHandler = &couponHandlerAdapter{
			handler: automation.NewCouponHandler(couponRepo, couponRuleRepo, logger),
		}
	}
	offerGenerateService := offer_generate.NewOfferGenerateService(
		automationRepo,
		instanceRepo,
		groupRepo,
		dispatchLogRepo,
		settingsRepo,
		couponRepo,
		ctaRepo,
		affiliateRepo,
		shopeeService,
		mlService,
		wahaAdapter,
		ogCouponHandler,
		cacheClient,
		config.NewModuleLogger(logger, "offer-generate"),
	)

	dashboardService := dashboard.NewDashboardService(
		repos.Dashboard,
		config.NewModuleLogger(logger, "dashboard"),
	)

	// Create redirect service for public redirect links
	redirectService := redirect.NewRedirectService(
		repos.RedirectLink,
		groupRepo,
		instanceRepo,
		wahaAdapter,
		config.NewModuleLogger(logger, "redirect"),
	)

	// Create ML link lists service (with audit trail support)
	mlLinkListsService := mllinklists.NewServiceWithAudit(
		repos.MLLinkList,
		actionHistoryRepo,
		config.NewModuleLogger(logger, "ml-link-lists"),
	)

	// Create Kiwify webhook service
	kiwifyService := kiwify.NewKiwifyService(
		profileRepo,
		repos.Plan,
		settingsRepo,
		userRoleRepo,
		emailService,
		&kiwify.Config{
			WebhookToken: cfg.KiwifyWebhookToken,
		},
		config.NewModuleLogger(logger, "kiwify"),
	)

	// Run in worker mode
	if *workerMode {
		var redisClientForWorker *redis.Client
		if redisClient != nil {
			redisClientForWorker = redisClient.Client
		}
		runWorker(cfg, redisAddr, db, wahaAdapter, urlSafetyService, instanceRepo, groupRepo, actionHistoryRepo, dispatchLogRepo, repos.WebhookEvents, queueClient, redisClientForWorker, monitoringService, offerGenerateService, logger)
		return
	}

	// Start Redis SSE bridge (server-side subscriber)
	// This receives SSE events published by the worker process via Redis Pub/Sub
	// and forwards them to the local SSEHub for delivery to connected frontend clients.
	if redisClient != nil {
		bridge := handlers.NewSSERedisBridge(redisClient.Client, config.NewModuleLogger(logger, "sse-bridge"))
		bridge.SetHub(sseHub)

		// If wahaAdapter is a concrete *WahaAdapter, wire up cache clearing
		// so that when a status event arrives via Redis, the stale 5s adapter cache is invalidated
		if concreteAdapter, ok := wahaAdapter.(*whatsapp.WahaAdapter); ok {
			bridge.SetCacheClearer(concreteAdapter)
		}

		ctx, cancelBridge := context.WithCancel(context.Background())
		defer cancelBridge()
		if err := bridge.Subscribe(ctx); err != nil {
			observability.LogAndCapture(logger, sentry.LevelError, "failed to start Redis SSE bridge", err,
				map[string]string{"module": "sse-bridge", "flow": "startup"})
		} else {
			logger.Info("Redis SSE bridge started (server-side subscriber)")
		}
	}

	// Create DB clients (for auth service - uses PostgreSQL directly)
	dbClients, err := config.NewDBClients(cfg)
	if err != nil {
		logger.Error("Failed to create DB clients", zap.Error(err))
		sentry.CaptureException(fmt.Errorf("startup: failed to create DB clients: %w", err))
		sentry.Flush(2 * time.Second)
		os.Exit(1)
	}

	// Create dependencies
	deps := &handlers.Dependencies{
		Config:           cfg,
		Cache:            cacheClient,
		Logger:           logger,
		SSEHub:           sseHub, // Real-time notifications
		WhatsAppProvider: wahaAdapter,
		QueueClient:      queueClient,
		URLSafetyService: urlSafetyService,
		Backpressure:     backpressureManager,

		// Services
		ShopeeService:     shopeeService,
		MLService:         mlService,
		AutomationService: automationService,
		MonitoringService: monitoringService,
		JobsService:       jobsService,
		UserService:       userService,
		CouponsService:    couponsService,
		LBService:         lbService,
		RedirectService:   redirectService,
		KiwifyService:     kiwifyService,

		// Repositories
		ProfileRepo:       profileRepo,
		InstanceRepo:      instanceRepo,
		GroupRepo:         groupRepo,
		AutomationRepo:    automationRepo,
		ActionHistoryRepo: actionHistoryRepo,
		SettingsRepo:      settingsRepo,
		CouponRepo:        couponRepo,
		DispatchLogRepo:   dispatchLogRepo,
		ScheduledJobRepo:  scheduledJobRepo,
		SystemEventRepo:   systemEventRepo,
		UserRoleRepo:      userRoleRepo,
		WebhookEventsRepo: repos.WebhookEvents,
	}

	// Create Gin router
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware (otelgin FIRST for distributed tracing, sentrygin SECOND for error capture)
	router.Use(otelgin.Middleware("automatiza-server"))
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	router.Use(gin.Recovery())
	router.Use(middleware.LoggingMiddleware(logger))
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.CORSMiddleware(middleware.DefaultCORSConfig(cfg.CorsOrigin, cfg.IsDevelopment())))
	router.Use(middleware.ErrorHandler(logger, cfg.IsDevelopment()))

	// Register custom validators
	middleware.RegisterCustomValidators()

	// Setup routes
	handlers.SetupRoutes(router, deps)

	// Create cookie config for auth
	cookieConfig := auth.DefaultCookieConfig(cfg.IsProduction())

	// Create auth service
	var redisClientForAuth *redis.Client
	if redisClient != nil {
		redisClientForAuth = redisClient.Client
	}
	// Create rate limit config from environment (production-safe defaults)
	rateLimitConfig := auth.NewRateLimitConfig(
		cfg.RateLimitMaxAttempts,
		cfg.RateLimitWindowSec,
		cfg.RateLimitBlockSec,
	)

	authService := auth.NewAuthService(&auth.AuthServiceConfig{
		DB:              dbClients.Admin,
		Redis:           redisClientForAuth,
		CookieConfig:    cookieConfig,
		EmailService:    emailService,
		PublicBaseURL:   cfg.PublicBaseURL,
		Logger:          config.NewModuleLogger(logger, "auth"),
		RateLimitConfig: rateLimitConfig,
	})

	// Create ConnectRPC server with suspension check
	connectServer := connectrpc.NewServer(&connectrpc.ServerConfig{
		AuthService:  authService,
		CookieConfig: cookieConfig,
		ProfileRepo:  profileRepo,
		Logger:       config.NewModuleLogger(logger, "connect"),
	})

	// Register auth service handler
	authHandler := connectrpc.NewAuthHandler(authService, cookieConfig)
	authPath, authHTTPHandler := authv1connect.NewAuthServiceHandler(authHandler)
	connectServer.RegisterService(authPath, authHTTPHandler)

	// Register dashboard service handler (requires auth)
	dashboardHandler := connectrpc.NewDashboardHandler(dashboardService, logger)
	dashboardPath, dashboardHTTPHandler := dashboardv1connect.NewDashboardServiceHandler(
		dashboardHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(dashboardPath, dashboardHTTPHandler)

	// Create asynq inspector for GetSyncStatus polling endpoint
	asynqInspectorServer := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	defer func() {
		if err := asynqInspectorServer.Close(); err != nil {
			observability.LogAndCapture(logger, sentry.LevelWarning, "asynq inspector close failed on shutdown", err,
				map[string]string{"module": "main", "flow": "shutdown"})
		}
	}()

	// Create sync progress reader for real-time progress from Redis
	var syncProgressReader *handlers.RedisSyncProgressReader
	if redisClient != nil {
		syncProgressReader = handlers.NewRedisSyncProgressReader(redisClient.Client)
	}

	// Register groups service handler (requires auth)
	groupsHandler := connectrpc.NewGroupsHandler(groupRepo, instanceRepo, wahaAdapter, queueClient,
		connectrpc.WithGroupEventPublisher(sseHub),
		connectrpc.WithGroupActionHistoryRepo(actionHistoryRepo),
		connectrpc.WithGroupHistoryPublisher(sseHub),
		connectrpc.WithAsynqInspector(asynqInspectorServer),
		connectrpc.WithSyncProgressReader(syncProgressReader),
	)
	groupsPath, groupsHTTPHandler := groupsv1connect.NewGroupsServiceHandler(
		groupsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(groupsPath, groupsHTTPHandler)

	// Register automations service handler (requires auth)
	automationsHandler := connectrpc.NewAutomationsHandler(automationRepo, dispatchLogRepo, groupRepo, automationService, jobsService, logger)
	automationsPath, automationsHTTPHandler := automationsv1connect.NewAutomationsServiceHandler(
		automationsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(automationsPath, automationsHTTPHandler)

	// Register WhatsApp service handler (canonical service, requires auth)
	whatsappHandler := connectrpc.NewWhatsAppHandler(wahaAdapter, instanceRepo, repos.SyncJob, queueClient, cfg.WebhookSecret)
	whatsappPath, whatsappHTTPHandler := whatsappv1connect.NewWhatsAppServiceHandler(
		whatsappHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(whatsappPath, whatsappHTTPHandler)

	// Register instances service handler (requires auth)
	instancesHandler := connectrpc.NewInstancesHandlerWithAdapter(instanceRepo, wahaAdapter)
	instancesPath, instancesHTTPHandler := instancesv1connect.NewInstancesServiceHandler(
		instancesHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(instancesPath, instancesHTTPHandler)

	// Create profile services
	profileSvc := profileservice.NewProfileService(
		profileRepo,
		automationRepo,
		instanceRepo,
		userRoleRepo,
		config.NewModuleLogger(logger, "profile-service"),
	)

	// Create email adapter for password reset
	var emailProvider emailadapter.EmailProvider
	smtpHost := os.Getenv("SMTP_HOST")
	smtpUsername := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	if smtpHost != "" && smtpUsername != "" && smtpPassword != "" {
		smtpPort := 587
		if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				smtpPort = p
			}
		}
		fromEmail := os.Getenv("SMTP_FROM_EMAIL")
		if fromEmail == "" {
			fromEmail = smtpUsername
		}
		fromName := os.Getenv("SMTP_FROM_NAME")
		if fromName == "" {
			fromName = "Automatiza"
		}

		adapter, err := emailadapter.NewSMTPAdapter(emailadapter.SMTPConfig{
			Host:        smtpHost,
			Port:        smtpPort,
			Username:    smtpUsername,
			Password:    smtpPassword,
			FromName:    fromName,
			FromEmail:   fromEmail,
			TLSRequired: true,
			Timeout:     30 * time.Second,
		}, config.NewModuleLogger(logger, "email-adapter"))
		if err != nil {
			logger.Warn("Failed to create email adapter, password reset will be disabled",
				zap.Error(err))
		} else {
			emailProvider = adapter
		}
	} else {
		logger.Warn("SMTP not configured, password reset emails disabled")
		// Use fake adapter for development
		emailProvider = emailadapter.NewFakeEmailAdapter()
	}

	// Get HMAC key for password reset tokens
	// Use the global token HMAC key if available, otherwise fallback to separate env var
	pwResetHMACKey := cfg.GetTokenHMACKey()
	if pwResetHMACKey == nil {
		pwResetHMACKeyStr := os.Getenv("PASSWORD_RESET_HMAC_KEY")
		if pwResetHMACKeyStr != "" {
			pwResetHMACKey = []byte(pwResetHMACKeyStr)
		} else {
			// Generate random key (not ideal for production, should be from env)
			pwResetHMACKey = make([]byte, 32)
			// In production, this should come from environment
			logger.Warn("PASSWORD_RESET_HMAC_KEY not set, using zero key (not for production)")
		}
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = os.Getenv("PUBLIC_BASE_URL")
	}
	if frontendURL == "" {
		frontendURL = "http://localhost:3003"
	}

	passwordSvc := profileservice.NewPasswordService(
		profileRepo,
		repos.PasswordResetToken,
		repos.RateLimit,
		emailProvider,
		profileservice.PasswordServiceConfig{
			HMACKey:     pwResetHMACKey,
			FrontendURL: frontendURL,
		},
		config.NewModuleLogger(logger, "password-service"),
	)

	// Register profiles service handler (requires auth for most RPCs)
	profilesHandler := connectrpc.NewProfilesHandler(profileSvc, passwordSvc, config.NewModuleLogger(logger, "profiles-handler"))
	profilesPath, profilesHTTPHandler := profilesv1connect.NewProfilesServiceHandler(
		profilesHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(profilesPath, profilesHTTPHandler)

	// Register settings service handler (requires auth)
	settingsHandler := connectrpc.NewSettingsHandler(settingsRepo, userRoleRepo)
	settingsPath, settingsHTTPHandler := settingsv1connect.NewSettingsServiceHandler(
		settingsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(settingsPath, settingsHTTPHandler)

	// Register history service handler (requires auth)
	historyHandler := connectrpc.NewHistoryHandler(actionHistoryRepo, dispatchLogRepo)
	historyPath, historyHTTPHandler := historyv1connect.NewHistoryServiceHandler(
		historyHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(historyPath, historyHTTPHandler)

	// Register coupons service handler (requires auth)
	couponsHandler := connectrpc.NewCouponsHandler(couponsService, couponRuleRepo)
	couponsPath, couponsHTTPHandler := couponsv1connect.NewCouponsServiceHandler(
		couponsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(couponsPath, couponsHTTPHandler)

	// Register CTAs service handler (requires auth)
	ctasHandler := connectrpc.NewCTAsHandler(ctasService)
	ctasPath, ctasHTTPHandler := ctasv1connect.NewCTAsServiceHandler(
		ctasHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(ctasPath, ctasHTTPHandler)

	// Register admin service handler (requires auth + admin role)
	adminHandler := connectrpc.NewAdminHandler(
		userRoleRepo,
		systemEventRepo,
		instanceRepo,
		profileRepo,
		repos.Plan,
		repos.EmailTemplate,
		actionHistoryRepo,
		wahaAdapter,
		emailService,
		connectrpc.WithAffiliateRepo(affiliateRepo),
		connectrpc.WithDispatchLogRepo(dispatchLogRepo),
		connectrpc.WithWebhookBaseURL(cfg.BackendPublicURL),
		connectrpc.WithAutomationRepo(automationRepo),
		connectrpc.WithGroupRepo(groupRepo),
		connectrpc.WithKiwifyService(kiwifyService),
		connectrpc.WithLBService(lbService),
		connectrpc.WithWebhookLogRepo(connectrpc.NewWebhookLogAdapter(repos.WebhookLog)),
		connectrpc.WithWebhookLogWriteRepo(repos.WebhookLog),
		connectrpc.WithCategoryRepo(repos.Category),
	)
	adminPath, adminHTTPHandler := adminv1connect.NewAdminServiceHandler(
		adminHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(adminPath, adminHTTPHandler)

	// Register load balancer service handler (requires auth + admin role)
	loadBalancerHandler := connectrpc.NewLoadBalancerHandler(
		config.NewModuleLogger(logger, "loadbalancer-handler"),
		userRoleRepo,
		lbService,
	)
	lbPath, lbHTTPHandler := adminv1connect.NewLoadBalancerServiceHandler(
		loadBalancerHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(lbPath, lbHTTPHandler)

	// Register plans service handler (requires auth + admin role)
	plansHandler := connectrpc.NewPlansHandler(repos.Plan, userRoleRepo)
	plansPath, plansHTTPHandler := plansv1connect.NewPlansServiceHandler(
		plansHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(plansPath, plansHTTPHandler)

	// Register dispatch texts service handler (requires auth)
	dispatchTextsHandler := connectrpc.NewDispatchTextsHandler(settingsRepo)
	dispatchTextsPath, dispatchTextsHTTPHandler := dispatchtextsv1connect.NewDispatchTextsServiceHandler(
		dispatchTextsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(dispatchTextsPath, dispatchTextsHTTPHandler)

	// Register ML links service handler (requires auth)
	mlLinksHandler := connectrpc.NewMlLinksHandler(mlLinkListsService)
	mlLinksPath, mlLinksHTTPHandler := mllinksv1connect.NewMlLinksServiceHandler(
		mlLinksHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(mlLinksPath, mlLinksHTTPHandler)

	// Register jobs service handler (requires auth)
	jobsHandler := connectrpc.NewJobsHandler(standaloneJobsService, userRoleRepo, config.NewModuleLogger(logger, "jobs-handler"), cfg)
	jobsPath, jobsHTTPHandler := jobsv1connect.NewJobsServiceHandler(
		jobsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(jobsPath, jobsHTTPHandler)

	// Register webhooks service handler (no auth for external webhooks, admin check internal)
	webhooksHandler := connectrpc.NewWebhooksHandler(userRoleRepo, kiwifyService, repos.WebhookLog, logger)
	webhooksPath, webhooksHTTPHandler := webhooksv1connect.NewWebhooksServiceHandler(webhooksHandler)
	connectServer.RegisterService(webhooksPath, webhooksHTTPHandler)

	// Register affiliate credentials service handler (requires auth)
	affiliateCredentialsHandler := connectrpc.NewAffiliateCredentialsHandler(affiliateRepo)
	affiliateCredentialsPath, affiliateCredentialsHTTPHandler := affiliatesv1connect.NewAffiliateCredentialsServiceHandler(
		affiliateCredentialsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(affiliateCredentialsPath, affiliateCredentialsHTTPHandler)

	// Register Mercado Livre service handler (requires auth)
	mercadoLivreHandler := connectrpc.NewMercadoLivreHandler(mlService)
	mercadoLivrePath, mercadoLivreHTTPHandler := affiliatesv1connect.NewMercadoLivreServiceHandler(
		mercadoLivreHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(mercadoLivrePath, mercadoLivreHTTPHandler)

	// Register Shopee service handler (requires auth)
	shopeeHandler := connectrpc.NewShopeeHandler(shopeeService)
	shopeePath, shopeeHTTPHandler := affiliatesv1connect.NewShopeeServiceHandler(
		shopeeHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(shopeePath, shopeeHTTPHandler)

	// Register redirect link service handler (requires auth)
	redirectHandler := connectrpc.NewRedirectHandler(redirectService)
	redirectPath, redirectHTTPHandler := redirectv1connect.NewRedirectLinkServiceHandler(
		redirectHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(redirectPath, redirectHTTPHandler)

	// Register AI tools service handler (requires auth)
	aiToolsHandler := connectrpc.NewAiToolsHandler(aiService, config.NewModuleLogger(logger, "ai-tools"))
	aiToolsPath, aiToolsHTTPHandler := aiv1connect.NewAiToolsServiceHandler(
		aiToolsHandler,
		connectrpc.WithInterceptors(connectServer.AuthInterceptors()...),
	)
	connectServer.RegisterService(aiToolsPath, aiToolsHTTPHandler)

	// Create CORS config for HTTP middleware (ConnectRPC paths)
	corsConfig := middleware.DefaultCORSConfig(cfg.CorsOrigin, cfg.IsDevelopment())

	// Create combined HTTP handler with path-based routing
	// ConnectRPC handles /connect/* paths, Gin handles everything else
	combinedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/connect/") {
			// Strip /connect prefix and forward to ConnectRPC
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/connect")
			connectServer.Handler().ServeHTTP(w, r)
			return
		}
		// Forward to Gin router
		router.ServeHTTP(w, r)
	})

	// Apply CORS middleware to the combined handler (handles ConnectRPC CORS)
	corsHandler := middleware.HTTPCORSMiddleware(corsConfig)(combinedHandler)

	// Wrap with h2c for HTTP/2 cleartext support (required for ConnectRPC gRPC protocol)
	h2cHandler := h2c.NewHandler(corsHandler, &http2.Server{})

	// Start cron service for scheduled automation processing
	cronService.Start()
	logger.Info("Cron service started for automation scheduling")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h2cHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	observability.SafeGo("http-server", func() {
		logger.Info("Starting HTTP server",
			zap.String("addr", srv.Addr))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", zap.Error(err))
			sentry.CaptureException(fmt.Errorf("http server failed: %w", err))
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
	})

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop cron service
	cronService.Stop()
	logger.Info("Cron service stopped")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// couponHandlerAdapter adapts automation.CouponHandler to offer_generate.CouponApplier
// to break the import cycle between the automation and offer_generate packages.
type couponHandlerAdapter struct {
	handler *automation.CouponHandler
}

func (a *couponHandlerAdapter) LoadCouponOnce(ctx context.Context, auto *models.Automation) (*models.Coupon, string) {
	return a.handler.LoadCouponOnce(ctx, auto)
}

func (a *couponHandlerAdapter) ApplyCouponWithPreloaded(coupon *models.Coupon, offerPrice float64, offerStore string) *offer_generate.CouponResult {
	result := a.handler.ApplyCouponWithPreloaded(coupon, offerPrice, offerStore)
	return &offer_generate.CouponResult{
		Applied:         result.Applied,
		CouponText:      result.CouponText,
		DiscountedPrice: result.DiscountedPrice,
	}
}

func (a *couponHandlerAdapter) ApplyCoupon(ctx context.Context, auto *models.Automation, offerPrice float64, offerStore string, productIsOfficial bool) (*offer_generate.CouponResult, error) {
	result, err := a.handler.ApplyCoupon(ctx, auto, offerPrice, offerStore, productIsOfficial)
	if err != nil || result == nil {
		return nil, err
	}
	return &offer_generate.CouponResult{
		Applied:         result.Applied,
		CouponText:      result.CouponText,
		DiscountedPrice: result.DiscountedPrice,
	}, nil
}

// queueEnqueuerAdapter adapts *workers.QueueClient to startup.ReconcileEnqueuer.
type queueEnqueuerAdapter struct {
	client *workers.QueueClient
}

func (a *queueEnqueuerAdapter) EnqueueReconcileSession(ctx context.Context, sessionName string) error {
	_, err := a.client.EnqueueReconcileSession(ctx, sessionName)
	return err
}

func (a *queueEnqueuerAdapter) EnqueueReconcileChat(ctx context.Context, sessionName, chatID string) error {
	_, err := a.client.EnqueueReconcileChat(ctx, sessionName, chatID)
	return err
}

func runWorker(
	cfg *config.Config,
	redisAddr string,
	db *postgres.DB,
	wahaAdapter whatsapp.WhatsAppProvider,
	urlSafetyService *urlsafety.Service,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	historyRepo repository.ActionHistoryRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	webhookEventsRepo repository.WebhookEventsRepository,
	queueClient *workers.QueueClient,
	redisClientForSSE *redis.Client,
	monitoringService monitoring.Service,
	offerGenerateService offer_generate.Service,
	logger *zap.Logger,
) {
	logger.Info("Starting worker mode",
		zap.Int("concurrency", cfg.WorkerConcurrency))

	// Create queue server
	queueServer := workers.NewQueueServer(redisAddr, cfg.WorkerConcurrency, config.NewModuleLogger(logger, "worker"))

	// Create workers
	messageWorker := workers.NewMessageWorker(
		wahaAdapter,
		urlSafetyService,
		historyRepo,
		dispatchLogRepo,
		config.NewModuleLogger(logger, "message_worker"),
	)

	// Create job progress publisher for SSE notifications during long-running tasks
	var jobProgressPub workers.JobProgressPublisher
	if redisClientForSSE != nil {
		bridge := handlers.NewSSERedisBridge(redisClientForSSE, config.NewModuleLogger(logger, "sse-bridge-progress"))
		jobProgressPub = handlers.NewRedisJobProgressPublisher(
			bridge,
			config.NewModuleLogger(logger, "job-progress-publisher"),
		)
	}

	groupsSyncWorker := workers.NewGroupsSyncWorker(
		wahaAdapter,
		instanceRepo,
		groupRepo,
		config.NewModuleLogger(logger, "groups_sync_worker"),
		jobProgressPub,
	)

	// Create user resolver function for SSE notifications
	// This resolves session name (instance_id) to user_id using the instance repository
	userResolver := func(sessionName string) (string, error) {
		instance, err := instanceRepo.GetInstanceByName(context.Background(), sessionName)
		if err != nil {
			return "", err
		}
		if instance == nil {
			return "", fmt.Errorf("instance not found: %s", sessionName)
		}
		return instance.UserID, nil
	}

	// Create SSE status notifier for real-time UI updates
	// Uses Redis Pub/Sub bridge so status events from the worker process
	// reach the server process's SSEHub (they run as separate OS processes).
	var statusProcessor workers.StatusWebhookProcessor
	if redisClientForSSE != nil {
		bridge := handlers.NewSSERedisBridge(redisClientForSSE, config.NewModuleLogger(logger, "sse-bridge"))
		statusProcessor = handlers.NewRedisSSEStatusNotifier(
			bridge,
			config.NewModuleLogger(logger, "sse-notifier"),
			userResolver,
		)
	}

	// Create monitoring message processor for MONITOR mode automations
	// This processes incoming WhatsApp messages and triggers monitoring automations
	var messageProcessor workers.MessageWebhookProcessor
	if monitoringService != nil {
		messageProcessor = workers.NewMonitoringMessageProcessor(
			monitoringService,
			offerGenerateService,
			config.NewModuleLogger(logger, "monitoring-processor"),
		)
	}

	// Create dispatch log processor for reconciliation
	// This creates dispatch_logs for reconciled messages to fill gaps in history
	dispatchLogProcessor := workers.NewDispatchLogProcessor(workers.DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           config.NewModuleLogger(logger, "dispatch-log-processor"),
		DedupWindowHours: 168, // 7 days dedup window
	})

	// Create reconciliation service checkpoint repository adapter
	// The reconciliation service expects its own CheckpointRepository interface
	checkpointRepoAdapter := postgres.NewCheckpointRepository(db, config.NewModuleLogger(logger, "checkpoint-repo"))

	// Create reconciliation service
	// This handles message backfill when gaps are detected via silence watchdog
	var reconcileSvc *reconciliation.Service
	if wahaAdapter != nil {
		reconcileSvc = reconciliation.New(
			wahaAdapter,
			checkpointRepoAdapter,
			dispatchLogProcessor, // Integrates with dispatch_logs
			config.NewModuleLogger(logger, "reconciliation"),
			reconciliation.DefaultConfig(),
		)

		// Start the silence watchdog in background
		// It will detect when a session goes silent and trigger reconciliation
		cancelWatchdog := reconcileSvc.StartWatchdog(context.Background())
		defer cancelWatchdog()

		// Create and run the backfill service for comprehensive startup recovery
		// This checks all active sessions for gaps and enqueues reconciliation tasks
		backfillSvc := startup.NewBackfillService(
			instanceRepo,
			checkpointRepoAdapter,
			wahaAdapter,
			&queueEnqueuerAdapter{client: queueClient},
			reconcileSvc,
			config.NewModuleLogger(logger, "backfill"),
			startup.DefaultBackfillConfig(),
		)

		// Run startup backfill in background to avoid blocking worker startup
		observability.SafeGo("backfill-startup", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			result, err := backfillSvc.RunOnStartup(ctx)
			if err != nil {
				observability.LogAndCapture(logger, sentry.LevelError, "startup backfill failed", err,
					map[string]string{"module": "backfill", "flow": "startup"})
				return
			}

			if len(result.Errors) > 0 {
				logger.Warn("Startup backfill completed with errors",
					zap.Int("totalSessions", result.TotalSessions),
					zap.Int("sessionsWithGaps", result.SessionsWithGaps),
					zap.Int("reconciliationsEnqueued", result.ReconciliationsEnqueued),
					zap.Int("errors", len(result.Errors)),
					zap.Duration("duration", result.Duration))
			} else {
				logger.Info("Startup backfill completed successfully",
					zap.Int("totalSessions", result.TotalSessions),
					zap.Int("sessionsWithGaps", result.SessionsWithGaps),
					zap.Int("reconciliationsEnqueued", result.ReconciliationsEnqueued),
					zap.Duration("duration", result.Duration))
			}
		})
	}

	// Create webhook consumer worker with SSE integration and monitoring processor
	webhookConsumerWorker := workers.NewWebhookConsumerWorker(workers.WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		ReconcileSvc:      reconcileSvc, // Records events for silence detection
		InstanceRepo:      instanceRepo,
		GroupRepo:         groupRepo,
		QueueClient:       queueClient,
		Logger:            config.NewModuleLogger(logger, "webhook-consumer"),
		StatusProcessor:   statusProcessor,  // SSE notifications for status changes
		MessageProcessor:  messageProcessor, // MONITOR mode automation processing
	})

	// Create reconciliation worker for processing queued reconciliation tasks
	var reconciliationWorker *workers.ReconciliationWorker
	if reconcileSvc != nil {
		reconciliationWorker = workers.NewReconciliationWorker(
			reconcileSvc,
			config.NewModuleLogger(logger, "reconciliation-worker"),
		)
	}

	// Create asynq inspector for queue monitoring (used by MessageQueueWorker for backpressure)
	asynqInspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	defer func() {
		if err := asynqInspector.Close(); err != nil {
			observability.LogAndCapture(logger, sentry.LevelWarning, "asynq inspector close failed on shutdown", err,
				map[string]string{"module": "worker", "flow": "shutdown"})
		}
	}()

	// Create asynq client for enqueueing tasks (used by MessageQueueWorker)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer func() {
		if err := asynqClient.Close(); err != nil {
			observability.LogAndCapture(logger, sentry.LevelWarning, "asynq client close failed on shutdown", err,
				map[string]string{"module": "worker", "flow": "shutdown"})
		}
	}()

	// Create message queue worker for new message types (text, audio, image, poll)
	// This worker handles messages enqueued with type-specific task types for better routing
	messageQueueWorker := workers.NewMessageQueueWorker(
		wahaAdapter,
		historyRepo,
		asynqInspector,
		asynqClient,
		config.NewModuleLogger(logger, "message_queue_worker"),
	)

	// Register handlers (wrapped with SentryAsynqMiddleware for error capture + panic recovery)
	queueServer.RegisterHandler(workers.TypeSendMessage, observability.SentryAsynqMiddleware("message", messageWorker.ProcessTaskWrapper()))
	queueServer.RegisterHandler(workers.TypeSyncGroups, observability.SentryAsynqMiddleware("groups_sync", groupsSyncWorker.ProcessTaskWrapper()))
	queueServer.RegisterHandler(workers.TypeWAHAWebhook, observability.SentryAsynqMiddleware("webhook_consumer", webhookConsumerWorker))

	// Register message queue worker handlers for typed messages
	// These handle messages enqueued via MessageQueueWorker.Enqueue* methods
	queueServer.RegisterHandler(workers.TypeMessageText, observability.SentryAsynqMiddleware("message_queue", messageQueueWorker))
	queueServer.RegisterHandler(workers.TypeMessageAudio, observability.SentryAsynqMiddleware("message_queue", messageQueueWorker))
	queueServer.RegisterHandler(workers.TypeMessageImage, observability.SentryAsynqMiddleware("message_queue", messageQueueWorker))
	queueServer.RegisterHandler(workers.TypeMessagePoll, observability.SentryAsynqMiddleware("message_queue", messageQueueWorker))
	logger.Info("Message queue worker registered for typed message handling")

	// Register reconciliation handlers if service is available
	if reconciliationWorker != nil {
		queueServer.RegisterHandler(workers.TypeReconcileChat, observability.SentryAsynqMiddleware("reconciliation", asynq.HandlerFunc(reconciliationWorker.ProcessChatWrapper())))
		queueServer.RegisterHandler(workers.TypeReconcileSession, observability.SentryAsynqMiddleware("reconciliation", asynq.HandlerFunc(reconciliationWorker.ProcessSessionWrapper())))
		logger.Info("Reconciliation worker registered")
	}

	// Start server
	observability.SafeGo("queue-server", func() {
		if err := queueServer.Start(); err != nil {
			logger.Error("Failed to start queue server", zap.Error(err))
			sentry.CaptureException(fmt.Errorf("queue server failed: %w", err))
			sentry.Flush(2 * time.Second)
			os.Exit(1)
		}
	})

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker...")
	queueServer.Shutdown()
	logger.Info("Worker exited")
}

// buildVersion returns the build version from Go's debug info.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			if len(s.Value) > 8 {
				return s.Value[:8]
			}
			return s.Value
		}
	}
	return info.Main.Version
}

// isStdioSyncErr returns true if the error is the well-known "sync /dev/stdout:
// invalid argument" (or /dev/stderr) that occurs when zap.Logger.Sync() is called
// in a container where stdout/stderr are pipes, not regular files.
func isStdioSyncErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "/dev/stdout") ||
		strings.Contains(msg, "/dev/stderr") ||
		strings.Contains(msg, "invalid argument")
}
