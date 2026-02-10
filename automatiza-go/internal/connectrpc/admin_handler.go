package connectrpc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/auth"
	adminv1 "automatiza-go/internal/gen/admin/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/repository/postgres"
	"automatiza-go/internal/services/email"
	"automatiza-go/internal/services/loadbalancer"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

// AdminHandler implements the adminv1connect.AdminServiceHandler interface.
type AdminHandler struct {
	userRoleRepo      repository.UserRoleRepository
	systemEventRepo   repository.SystemEventRepository
	instanceRepo      repository.InstanceRepository
	profileRepo       repository.ProfileRepository
	planRepo          repository.PlanRepository
	emailTemplateRepo repository.EmailTemplateRepository
	actionHistoryRepo repository.ActionHistoryRepository
	affiliateRepo     repository.AffiliateCredentialsRepository
	dispatchLogRepo   repository.DispatchLogRepository
	automationRepo    repository.AutomationRepository
	groupRepo         repository.GroupRepository
	categoryRepo      repository.CategoryRepository
	webhookLogRepo      WebhookLogRepository
	webhookLogWriteRepo postgres.WebhookLogRepository
	kiwifyService       KiwifyService
	lbService         loadbalancer.Service
	waProvider        whatsapp.WhatsAppProvider
	emailService      email.Service
	webhookBaseURL    string
}

// WebhookLogRepository interface for webhook logs
type WebhookLogRepository interface {
	GetRecent(ctx context.Context, source string, limit int) ([]WebhookLogEntry, error)
}

// WebhookLogEntry represents a webhook log entry
type WebhookLogEntry struct {
	ID        string
	Source    string
	EventType *string
	Email     *string
	Status    string
	CreatedAt string
}

// KiwifyService interface for Kiwify operations
type KiwifyService interface {
	ProcessWebhook(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error)
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(
	userRoleRepo repository.UserRoleRepository,
	systemEventRepo repository.SystemEventRepository,
	instanceRepo repository.InstanceRepository,
	profileRepo repository.ProfileRepository,
	planRepo repository.PlanRepository,
	emailTemplateRepo repository.EmailTemplateRepository,
	actionHistoryRepo repository.ActionHistoryRepository,
	waProvider whatsapp.WhatsAppProvider,
	emailService email.Service,
	opts ...AdminHandlerOption,
) *AdminHandler {
	h := &AdminHandler{
		userRoleRepo:      userRoleRepo,
		systemEventRepo:   systemEventRepo,
		instanceRepo:      instanceRepo,
		profileRepo:       profileRepo,
		planRepo:          planRepo,
		emailTemplateRepo: emailTemplateRepo,
		actionHistoryRepo: actionHistoryRepo,
		waProvider:        waProvider,
		emailService:      emailService,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// AdminHandlerOption is a functional option for AdminHandler.
type AdminHandlerOption func(*AdminHandler)

// WithAffiliateRepo sets the affiliate credentials repository.
func WithAffiliateRepo(repo repository.AffiliateCredentialsRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.affiliateRepo = repo
	}
}

// WithDispatchLogRepo sets the dispatch log repository.
func WithDispatchLogRepo(repo repository.DispatchLogRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.dispatchLogRepo = repo
	}
}

// WithWebhookBaseURL sets the webhook base URL for EnsureWebhooks.
func WithWebhookBaseURL(url string) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.webhookBaseURL = url
	}
}

// WithAutomationRepo sets the automation repository.
func WithAutomationRepo(repo repository.AutomationRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.automationRepo = repo
	}
}

// WithGroupRepo sets the group repository.
func WithGroupRepo(repo repository.GroupRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.groupRepo = repo
	}
}

// WithWebhookLogRepo sets the webhook log repository.
func WithWebhookLogRepo(repo WebhookLogRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.webhookLogRepo = repo
	}
}

// WithWebhookLogWriteRepo sets the webhook log write repository for creating log entries.
func WithWebhookLogWriteRepo(repo postgres.WebhookLogRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.webhookLogWriteRepo = repo
	}
}

// WithKiwifyService sets the Kiwify service.
func WithKiwifyService(svc KiwifyService) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.kiwifyService = svc
	}
}

// WithLBService sets the load balancer service.
func WithLBService(svc loadbalancer.Service) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.lbService = svc
	}
}

// WithCategoryRepo sets the category repository.
func WithCategoryRepo(repo repository.CategoryRepository) AdminHandlerOption {
	return func(h *AdminHandler) {
		h.categoryRepo = repo
	}
}

// requireAdmin checks if the user is an admin and returns an error if not.
func (h *AdminHandler) requireAdmin(ctx context.Context) error {
	user := GetUserFromContext(ctx)
	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, user.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}

	if !isAdmin {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
	}

	return nil
}

// GetObservability returns comprehensive observability data.
func (h *AdminHandler) GetObservability(
	ctx context.Context,
	req *connect.Request[adminv1.GetObservabilityRequest],
) (*connect.Response[adminv1.GetObservabilityResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)

	// Build queue stats (placeholder - would integrate with Asynq inspector)
	queueStats := &adminv1.ObservabilityQueue{
		Waiting: 0,
		Delayed: 0,
		Active:  0,
		Queued:  0,
	}

	// Build load balancer stats from lbService
	lbSummary := &adminv1.ObservabilityLoadBalancerSummary{
		TotalServers:   1,
		TotalCapacity:  100,
		TotalMapped:    0,
		TotalAvailable: 100,
		UsagePercent:   0,
	}

	if h.lbService != nil {
		stats, err := h.lbService.GetServerStats(ctx)
		if err == nil && stats != nil {
			var totalCapacity, totalMapped int32
			for _, srv := range stats.Servers {
				totalCapacity += int32(srv.MaxUsers)
				totalMapped += int32(srv.UserCount)
			}
			totalAvailable := totalCapacity - totalMapped
			if totalAvailable < 0 {
				totalAvailable = 0
			}
			var usagePercent float64
			if totalCapacity > 0 {
				usagePercent = float64(totalMapped) / float64(totalCapacity) * 100
			}
			lbSummary = &adminv1.ObservabilityLoadBalancerSummary{
				TotalServers:   int32(stats.TotalServers),
				TotalCapacity:  totalCapacity,
				TotalMapped:    totalMapped,
				TotalAvailable: totalAvailable,
				UsagePercent:   usagePercent,
			}
		}
	}

	// Build dispatch logs stats
	dispatchLogs := &adminv1.ObservabilityDispatchLogs{
		WindowMinutes: 10,
		Total:         0,
		ByStatus:      make(map[string]int32),
		TopErrors:     []*adminv1.TopError{},
	}

	if h.dispatchLogRepo != nil {
		// Get dispatch logs from last 10 minutes using available filters
		filters := &models.DispatchLogFilters{}
		logs, count, err := h.dispatchLogRepo.GetUserLogsWithFilters(ctx, "", filters, 5000, 0)
		if err == nil {
			dispatchLogs.Total = int32(count)

			// Count by status
			statusCounts := make(map[string]int32)
			errorCounts := make(map[string]int32)

			for _, log := range logs {
				statusCounts[log.Status]++
				if log.Error != nil && *log.Error != "" {
					errMsg := *log.Error
					if len(errMsg) > 140 {
						errMsg = errMsg[:140] + "..."
					}
					errorCounts[errMsg]++
				}
			}

			dispatchLogs.ByStatus = statusCounts

			// Build top errors (max 10)
			type errCount struct {
				msg   string
				count int32
			}
			var topErrs []errCount
			for msg, cnt := range errorCounts {
				topErrs = append(topErrs, errCount{msg, cnt})
			}
			// Sort by count desc (simple bubble sort for small list)
			for i := 0; i < len(topErrs)-1; i++ {
				for j := i + 1; j < len(topErrs); j++ {
					if topErrs[j].count > topErrs[i].count {
						topErrs[i], topErrs[j] = topErrs[j], topErrs[i]
					}
				}
			}
			for i := 0; i < len(topErrs) && i < 10; i++ {
				dispatchLogs.TopErrors = append(dispatchLogs.TopErrors, &adminv1.TopError{
					Error: topErrs[i].msg,
					Count: topErrs[i].count,
				})
			}
		}
	}

	// Build totals
	totals := &adminv1.ObservabilityTotals{}
	if h.automationRepo != nil {
		allAutomations, err := h.automationRepo.GetActiveAutomations(ctx)
		if err == nil {
			totals.AutomationsActive = int32(len(allAutomations))

			// Count unique users with active automations
			userIDs := make(map[string]struct{})
			for _, a := range allAutomations {
				userIDs[a.UserID] = struct{}{}
			}
			totals.UsersWithActiveAutomations = int32(len(userIDs))
		}
	}
	if h.groupRepo != nil {
		count, err := h.groupRepo.CountAll(ctx)
		if err == nil {
			totals.GroupsTotal = int32(count)
		}
	}

	// Build top users (placeholder)
	topUsers := &adminv1.ObservabilityTopUsers{
		ByGroupsSlots: []*adminv1.TopUserByGroupSlots{},
	}

	return connect.NewResponse(&adminv1.GetObservabilityResponse{
		Data: &adminv1.ObservabilityData{
			GeneratedAt:  generatedAt,
			Queue:        queueStats,
			LoadBalancer: &adminv1.ObservabilityLoadBalancer{Summary: lbSummary},
			DispatchLogs: dispatchLogs,
			Totals:       totals,
			TopUsers:     topUsers,
		},
		Cached: false,
	}), nil
}

// GetShopeeAffiliateAudit returns Shopee affiliate link audit data.
func (h *AdminHandler) GetShopeeAffiliateAudit(
	ctx context.Context,
	req *connect.Request[adminv1.GetShopeeAffiliateAuditRequest],
) (*connect.Response[adminv1.GetShopeeAffiliateAuditResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Check if repos are available
	if h.affiliateRepo == nil || h.dispatchLogRepo == nil {
		return connect.NewResponse(&adminv1.GetShopeeAffiliateAuditResponse{
			Data: &adminv1.ShopeeAuditData{
				Window:  &adminv1.AuditWindow{},
				Samples: []*adminv1.ShopeeAuditSample{},
			},
		}), nil
	}

	// Parse request parameters with defaults
	days := int(req.Msg.GetDays())
	if days <= 0 || days > 90 {
		days = 7
	}
	limit := int(req.Msg.GetLimit())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	maxScan := int(req.Msg.GetMaxScan())
	if maxScan <= 0 || maxScan > 20000 {
		maxScan = 5000
	}

	sinceTime := time.Now().UTC().AddDate(0, 0, -days)
	sinceISO := sinceTime.Format(time.RFC3339)

	// Get all dispatch logs with store='shopee' in the time window
	filters := &models.DispatchLogFilters{
		Store: stringPtr("shopee"),
		Since: &sinceTime,
	}
	logs, _, err := h.dispatchLogRepo.GetUserLogsWithFilters(ctx, "", filters, maxScan, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch logs: %w", err))
	}

	// Build a map of user credentials for quick lookup
	credsByUser := make(map[string]*models.AffiliateCredentials)
	userIDs := make(map[string]struct{})
	for _, log := range logs {
		userIDs[log.UserID] = struct{}{}
	}

	// Fetch credentials for all users
	for userID := range userIDs {
		creds, err := h.affiliateRepo.GetByUserAndStore(ctx, userID, "shopee")
		if err == nil && creds != nil && creds.IsActive {
			credsByUser[userID] = creds
		}
	}

	// Analyze logs and build samples
	var scanned, withIdentifier, mismatches, unknown, missingCreds int32
	samples := make([]*adminv1.ShopeeAuditSample, 0, limit)
	profileCache := make(map[string]*models.Profile)

	for _, log := range logs {
		scanned++

		// Get user credentials
		creds, hasCreds := credsByUser[log.UserID]
		if !hasCreds {
			missingCreds++
			continue
		}

		// Get affiliate URL from log
		affiliateURL := ""
		if log.AffiliateLink != nil {
			affiliateURL = *log.AffiliateLink
		} else if log.ProductURL != nil {
			affiliateURL = *log.ProductURL
		}

		if affiliateURL == "" {
			unknown++
			continue
		}

		// Extract mmp_pid and af_siteid from URL
		actualMmpPid, actualAfSiteId := extractShopeeParams(affiliateURL)
		if actualMmpPid == "" && actualAfSiteId == "" {
			unknown++
			continue
		}

		withIdentifier++

		// Calculate expected values
		appID := creds.AppID
		if appID == "" {
			appID = creds.GetCredential("app_id")
		}
		expectedMmpPid := "an_" + appID

		// Check for mismatch
		if actualMmpPid != "" && actualMmpPid != expectedMmpPid {
			mismatches++

			// Only add to samples if within limit
			if len(samples) < limit {
				// Get profile for enrichment (with cache)
				profile, ok := profileCache[log.UserID]
				if !ok {
					profile, _ = h.profileRepo.GetProfile(ctx, log.UserID)
					profileCache[log.UserID] = profile
				}

				sample := &adminv1.ShopeeAuditSample{
					UserId:         log.UserID,
					ExpectedAppId:  appID,
					ExpectedMmpPid: expectedMmpPid,
					ActualMmpPid:   stringPtrProto(actualMmpPid),
					ActualAfSiteid: stringPtrProto(actualAfSiteId),
					CreatedAt:      log.CreatedAt,
					Status:         log.Status,
					Url:            affiliateURL,
				}

				if log.AutomationID != nil {
					sample.AutomationId = *log.AutomationID
				}
				if log.AutomationName != nil {
					sample.AutomationName = *log.AutomationName
				}
				if log.GroupID != nil {
					sample.GroupId = *log.GroupID
				}

				if profile != nil {
					sample.Email = stringPtrProto(profile.Email)
					if profile.PhoneNumber != nil {
						sample.PhoneNumber = stringPtrProto(*profile.PhoneNumber)
					}
				}

				samples = append(samples, sample)
			}
		}
	}

	return connect.NewResponse(&adminv1.GetShopeeAffiliateAuditResponse{
		Data: &adminv1.ShopeeAuditData{
			Window: &adminv1.AuditWindow{
				Days:     int32(days),
				SinceIso: sinceISO,
			},
			Scanned:        scanned,
			WithIdentifier: withIdentifier,
			Mismatches:     mismatches,
			Unknown:        unknown,
			MissingCreds:   missingCreds,
			Samples:        samples,
		},
	}), nil
}

// extractShopeeParams extracts mmp_pid and af_siteid from a Shopee URL.
func extractShopeeParams(rawURL string) (mmpPid, afSiteId string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}

	q := u.Query()
	mmpPid = q.Get("mmp_pid")
	afSiteId = q.Get("af_siteid")

	// Also check for URL-encoded parameters in path (for short links)
	if mmpPid == "" && strings.Contains(rawURL, "s.shopee.com.br") {
		// Short links may have params in different format
		if idx := strings.Index(rawURL, "mmp_pid="); idx != -1 {
			rest := rawURL[idx+8:]
			if endIdx := strings.IndexAny(rest, "&?"); endIdx != -1 {
				mmpPid = rest[:endIdx]
			} else {
				mmpPid = rest
			}
		}
	}

	return mmpPid, afSiteId
}

// stringPtr returns a pointer to a string.
func stringPtr(s string) *string {
	return &s
}

// stringPtrProto returns a pointer to a string for protobuf optional fields.
func stringPtrProto(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// EnsureWebhooks ensures webhooks are configured for all instances on a given server.
func (h *AdminHandler) EnsureWebhooks(
	ctx context.Context,
	req *connect.Request[adminv1.EnsureWebhooksRequest],
) (*connect.Response[adminv1.EnsureWebhooksResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Build absolute webhook URL
	webhookURL := h.webhookBaseURL + "/api/v1/webhooks/waha"
	if h.webhookBaseURL == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("webhook base URL not configured"))
	}

	// Get all connected instances
	instances, err := h.instanceRepo.GetAllConnectedInstances(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 500
	}

	concurrency := int(req.Msg.GetConcurrency())
	if concurrency <= 0 || concurrency > 20 {
		concurrency = 5
	}

	var okCount int32
	var failCount int32
	var totalCount int32

	// Default webhook events for WAHA
	webhookEvents := []string{
		"message",
		"message.ack",
		"session.status",
		"message.waiting",
		"message.revoked",
	}

	for _, inst := range instances {
		if totalCount >= int32(limit) {
			break
		}

		totalCount++

		// Skip dry run mode - just count candidates
		if req.Msg.GetDryRun() {
			okCount++
			continue
		}

		// Set webhook for each instance using absolute URL
		err := h.waProvider.SetWebhook(ctx, whatsapp.SetWebhookInput{
			SessionName: inst.InstanceID,
			URL:         webhookURL,
			Events:      webhookEvents,
		})
		if err != nil {
			failCount++
		} else {
			okCount++
		}
	}

	return connect.NewResponse(&adminv1.EnsureWebhooksResponse{
		Ok:    okCount,
		Total: totalCount,
		Fail:  failCount,
	}), nil
}

// ListWebhookLogs returns recent webhook logs.
func (h *AdminHandler) ListWebhookLogs(
	ctx context.Context,
	req *connect.Request[adminv1.ListWebhookLogsRequest],
) (*connect.Response[adminv1.ListWebhookLogsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	source := req.Msg.GetSource()
	if source == "" {
		source = "kiwify"
	}

	// Check if repo is available
	if h.webhookLogRepo == nil {
		return connect.NewResponse(&adminv1.ListWebhookLogsResponse{
			Logs: []*adminv1.WebhookLogEntry{},
		}), nil
	}

	logs, err := h.webhookLogRepo.GetRecent(ctx, source, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get webhook logs: %w", err))
	}

	protoLogs := make([]*adminv1.WebhookLogEntry, 0, len(logs))
	for _, log := range logs {
		entry := &adminv1.WebhookLogEntry{
			Id:        log.ID,
			Source:    log.Source,
			Status:    log.Status,
			CreatedAt: log.CreatedAt,
		}
		if log.EventType != nil {
			entry.EventType = log.EventType
		}
		if log.Email != nil {
			entry.Email = log.Email
		}
		protoLogs = append(protoLogs, entry)
	}

	return connect.NewResponse(&adminv1.ListWebhookLogsResponse{
		Logs: protoLogs,
	}), nil
}

// SimulateKiwifyWebhook simulates a Kiwify webhook for testing.
func (h *AdminHandler) SimulateKiwifyWebhook(
	ctx context.Context,
	req *connect.Request[adminv1.SimulateKiwifyWebhookRequest],
) (*connect.Response[adminv1.SimulateKiwifyWebhookResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	email := req.Msg.GetEmail()
	event := req.Msg.GetEvent()
	product := req.Msg.GetProduct()

	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}
	if event == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("event is required"))
	}
	if product == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("product is required"))
	}

	// Check if service is available
	if h.kiwifyService == nil {
		return connect.NewResponse(&adminv1.SimulateKiwifyWebhookResponse{
			Success: false,
			Message: "Kiwify service not configured",
		}), nil
	}

	// Build simulated Kiwify payload
	payload := &models.KiwifyWebhookPayload{
		WebhookEventType: event,
		OrderStatus:      event,
		Customer: &models.KiwifyCustomer{
			Email: email,
		},
		Product: &models.KiwifyProduct{
			ProductName: product,
		},
		Subscription: &models.KiwifySubscription{},
	}

	result, err := h.kiwifyService.ProcessWebhook(ctx, payload, "")
	if err != nil {
		// Log failed simulation
		if h.webhookLogWriteRepo != nil {
			errMsg := fmt.Sprintf("Webhook processing failed: %v", err)
			logEntry, _ := h.webhookLogWriteRepo.Create(ctx, "kiwify", &event, &email, nil, nil)
			if logEntry != nil {
				_ = h.webhookLogWriteRepo.UpdateStatus(ctx, logEntry.ID, "failed", &errMsg)
			}
		}
		return connect.NewResponse(&adminv1.SimulateKiwifyWebhookResponse{
			Success: false,
			Message: fmt.Sprintf("Webhook processing failed: %v", err),
		}), nil
	}

	if !result.Success {
		return connect.NewResponse(&adminv1.SimulateKiwifyWebhookResponse{
			Success: false,
			Message: result.Message,
		}), nil
	}

	// Log successful simulation
	if h.webhookLogWriteRepo != nil {
		var plan *string
		if result.Plan != "" {
			plan = &result.Plan
		}
		logEntry, _ := h.webhookLogWriteRepo.Create(ctx, "kiwify", &result.Event, &email, plan, nil)
		if logEntry != nil {
			_ = h.webhookLogWriteRepo.UpdateStatus(ctx, logEntry.ID, "processed", nil)
		}
	}

	return connect.NewResponse(&adminv1.SimulateKiwifyWebhookResponse{
		Success: true,
		Message: fmt.Sprintf("Webhook simulated successfully for %s with event %s", email, event),
	}), nil
}

// GetSystemEvents returns system events filtered by user/instance and time window.
func (h *AdminHandler) GetSystemEvents(
	ctx context.Context,
	req *connect.Request[adminv1.GetSystemEventsRequest],
) (*connect.Response[adminv1.GetSystemEventsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	events, err := h.systemEventRepo.GetRecent(ctx, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get system events: %w", err))
	}

	protoEvents := make([]*adminv1.SystemEvent, len(events))
	for i, e := range events {
		meta, _ := structpb.NewStruct(e.Payload)
		protoEvents[i] = &adminv1.SystemEvent{
			Id:        fmt.Sprintf("%d", e.ID),
			Scope:     e.EventType,
			Meta:      meta,
			CreatedAt: e.CreatedAt,
		}
	}

	return connect.NewResponse(&adminv1.GetSystemEventsResponse{
		Events: protoEvents,
	}), nil
}

// ResetUserPassword resets a user's password (admin only).
// Generates a new random password, updates it in the database, and optionally sends an email.
func (h *AdminHandler) ResetUserPassword(
	ctx context.Context,
	req *connect.Request[adminv1.ResetUserPasswordRequest],
) (*connect.Response[adminv1.ResetUserPasswordResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Prevent admin from resetting their own password via this endpoint
	currentUser := GetUserFromContext(ctx)
	if currentUser != nil && currentUser.ID == userID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot reset your own password via admin endpoint"))
	}

	// Get user profile
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Generate a secure random password (12 chars with letters, numbers, symbols)
	newPassword, err := generateRandomPassword(12)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate password: %w", err))
	}

	// Hash the password using Argon2id (per CLAUDE.md: Argon2id is the canonical algorithm)
	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
	}

	// Update password in database
	if err := h.profileRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update password: %w", err))
	}

	// Log the action in system events for audit
	if h.systemEventRepo != nil {
		adminEmail := ""
		adminID := ""
		if currentUser != nil {
			adminEmail = currentUser.Email
			adminID = currentUser.ID
		}
		_ = h.systemEventRepo.Create(ctx, "admin_reset_password", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": profile.Email,
			"admin_id":          adminID,
			"admin_email":       adminEmail,
		})
	}

	// Send email notification with new password (best effort - don't fail if email fails)
	if h.emailService != nil {
		// Build password reset notification email
		emailBody := buildPasswordResetNotificationHTML(profile.Email, newPassword)
		_ = h.emailService.SendCustomEmail(ctx, profile.Email, "🔐 Sua senha foi resetada - Automatiza", emailBody)
	}

	return connect.NewResponse(&adminv1.ResetUserPasswordResponse{
		Success:     true,
		Email:       profile.Email,
		NewPassword: &newPassword,
		Message:     fmt.Sprintf("Password reset successfully for user %s", profile.Email),
	}), nil
}

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	charsetLen := big.NewInt(int64(len(charset)))

	password := make([]byte, length)
	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random index: %w", err)
		}
		password[i] = charset[randomIndex.Int64()]
	}

	return string(password), nil
}

// buildPasswordResetNotificationHTML builds the HTML email body for password reset notification
func buildPasswordResetNotificationHTML(email, newPassword string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <style>
    body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
    .container { max-width: 600px; margin: 0 auto; padding: 20px; }
    .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
    .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
    .password-box { background: white; padding: 20px; border-radius: 5px; margin: 20px 0; border-left: 4px solid #667eea; }
    .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 5px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1 style="color: white; margin: 0;">🔐 Senha Resetada</h1>
    </div>

    <div class="content">
      <p>Olá!</p>

      <p>Sua senha foi resetada por um administrador do sistema.</p>

      <div class="password-box">
        <p style="margin: 5px 0;"><strong>Email:</strong> %s</p>
        <p style="margin: 5px 0;"><strong>Nova Senha Temporária:</strong></p>
        <p style="background: #f4f4f4; padding: 10px; border-radius: 3px; font-size: 16px; font-family: monospace; text-align: center; margin: 10px 0;">
          <strong>%s</strong>
        </p>
      </div>

      <div class="warning">
        <p style="margin: 0; color: #856404;">
          <strong>⚠️ Importante:</strong> Por segurança, recomendamos que você altere esta senha temporária após o primeiro acesso.
        </p>
      </div>

      <p style="color: #666; font-size: 14px; margin-top: 30px;">
        Se você não solicitou esta alteração, entre em contato com o suporte imediatamente.
      </p>
    </div>
  </div>
</body>
</html>`, email, newPassword)
}

// ============================================================
// Plans Management
// ============================================================

// ListPlans returns all plans.
func (h *AdminHandler) ListPlans(
	ctx context.Context,
	req *connect.Request[adminv1.ListPlansRequest],
) (*connect.Response[adminv1.ListPlansResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	var plans []models.Plan
	var err error

	if req.Msg.GetIncludeInactive() {
		plans, err = h.planRepo.GetAllPlans(ctx)
	} else {
		plans, err = h.planRepo.GetActivePlans(ctx)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list plans: %w", err))
	}

	protoPlans := make([]*adminv1.Plan, len(plans))
	for i, p := range plans {
		protoPlans[i] = planToProto(&p)
	}

	return connect.NewResponse(&adminv1.ListPlansResponse{
		Plans: protoPlans,
	}), nil
}

// CreatePlan creates a new plan.
func (h *AdminHandler) CreatePlan(
	ctx context.Context,
	req *connect.Request[adminv1.CreatePlanRequest],
) (*connect.Response[adminv1.CreatePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	plan := &models.Plan{
		Slug:         req.Msg.GetSlug(),
		Name:         req.Msg.GetName(),
		IsActive:     req.Msg.GetIsActive(),
		GroupLimit:   req.Msg.GetGroupLimit(),
		StorageLimit: req.Msg.GetStorageLimit(),
		Features:     req.Msg.GetFeatures(),
	}

	if monthly := req.Msg.GetKiwifyProductIdMonthly(); monthly != "" {
		plan.KiwifyProductIDMonthly = &monthly
	}
	if semester := req.Msg.GetKiwifyProductIdSemester(); semester != "" {
		plan.KiwifyProductIDSemester = &semester
	}
	if annual := req.Msg.GetKiwifyProductIdAnnual(); annual != "" {
		plan.KiwifyProductIDAnnual = &annual
	}
	if desc := req.Msg.GetDescription(); desc != "" {
		plan.Description = &desc
	}
	if priceMonthly := req.Msg.GetPriceMonthly(); priceMonthly > 0 {
		plan.PriceMonthly = &priceMonthly
	}
	if priceSemester := req.Msg.GetPriceSemester(); priceSemester > 0 {
		plan.PriceSemester = &priceSemester
	}
	if priceAnnual := req.Msg.GetPriceAnnual(); priceAnnual > 0 {
		plan.PriceAnnual = &priceAnnual
	}

	created, err := h.planRepo.Create(ctx, plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create plan: %w", err))
	}

	return connect.NewResponse(&adminv1.CreatePlanResponse{
		Plan: planToProto(created),
	}), nil
}

// UpdatePlan updates an existing plan.
func (h *AdminHandler) UpdatePlan(
	ctx context.Context,
	req *connect.Request[adminv1.UpdatePlanRequest],
) (*connect.Response[adminv1.UpdatePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Check if plan exists
	existing, err := h.planRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get plan: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan not found"))
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Msg.Slug != nil {
		updates["slug"] = req.Msg.GetSlug()
	}
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	if req.Msg.IsActive != nil {
		updates["is_active"] = req.Msg.GetIsActive()
	}
	if req.Msg.KiwifyProductIdMonthly != nil {
		val := req.Msg.GetKiwifyProductIdMonthly()
		if val == "" {
			updates["kiwify_product_id_monthly"] = nil
		} else {
			updates["kiwify_product_id_monthly"] = val
		}
	}
	if req.Msg.KiwifyProductIdSemester != nil {
		val := req.Msg.GetKiwifyProductIdSemester()
		if val == "" {
			updates["kiwify_product_id_semester"] = nil
		} else {
			updates["kiwify_product_id_semester"] = val
		}
	}
	if req.Msg.KiwifyProductIdAnnual != nil {
		val := req.Msg.GetKiwifyProductIdAnnual()
		if val == "" {
			updates["kiwify_product_id_annual"] = nil
		} else {
			updates["kiwify_product_id_annual"] = val
		}
	}
	if req.Msg.Description != nil {
		val := req.Msg.GetDescription()
		if val == "" {
			updates["description"] = nil
		} else {
			updates["description"] = val
		}
	}
	if req.Msg.PriceMonthly != nil {
		updates["price_monthly"] = req.Msg.GetPriceMonthly()
	}
	if req.Msg.PriceSemester != nil {
		updates["price_semester"] = req.Msg.GetPriceSemester()
	}
	if req.Msg.PriceAnnual != nil {
		updates["price_annual"] = req.Msg.GetPriceAnnual()
	}
	if req.Msg.GroupLimit != nil {
		updates["group_limit"] = req.Msg.GetGroupLimit()
	}
	if req.Msg.StorageLimit != nil {
		updates["storage_limit"] = req.Msg.GetStorageLimit()
	}
	if req.Msg.Features != nil {
		updates["features"] = req.Msg.GetFeatures()
	}

	if len(updates) > 0 {
		if err := h.planRepo.Update(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update plan: %w", err))
		}
	}

	// Fetch updated plan
	updated, err := h.planRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated plan: %w", err))
	}

	return connect.NewResponse(&adminv1.UpdatePlanResponse{
		Plan: planToProto(updated),
	}), nil
}

// DeletePlan deletes a plan.
func (h *AdminHandler) DeletePlan(
	ctx context.Context,
	req *connect.Request[adminv1.DeletePlanRequest],
) (*connect.Response[adminv1.DeletePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.planRepo.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete plan: %w", err))
	}

	return connect.NewResponse(&adminv1.DeletePlanResponse{
		Success: true,
	}), nil
}

// ============================================================
// Email Templates Management
// ============================================================

// ListEmailTemplates returns all email templates.
func (h *AdminHandler) ListEmailTemplates(
	ctx context.Context,
	req *connect.Request[adminv1.ListEmailTemplatesRequest],
) (*connect.Response[adminv1.ListEmailTemplatesResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	templates, err := h.emailTemplateRepo.GetAll(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list email templates: %w", err))
	}

	protoTemplates := make([]*adminv1.EmailTemplate, len(templates))
	for i, t := range templates {
		protoTemplates[i] = emailTemplateToProto(&t)
	}

	return connect.NewResponse(&adminv1.ListEmailTemplatesResponse{
		Templates: protoTemplates,
	}), nil
}

// CreateEmailTemplate creates a new email template.
func (h *AdminHandler) CreateEmailTemplate(
	ctx context.Context,
	req *connect.Request[adminv1.CreateEmailTemplateRequest],
) (*connect.Response[adminv1.CreateEmailTemplateResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	template := &models.EmailTemplate{
		Name:     req.Msg.GetName(),
		Subject:  req.Msg.GetSubject(),
		Body:     req.Msg.GetBody(),
		IsActive: req.Msg.GetIsActive(),
	}

	if desc := req.Msg.GetDescription(); desc != "" {
		template.Description = &desc
	}

	created, err := h.emailTemplateRepo.Create(ctx, template)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create email template: %w", err))
	}

	return connect.NewResponse(&adminv1.CreateEmailTemplateResponse{
		Template: emailTemplateToProto(created),
	}), nil
}

// UpdateEmailTemplate updates an existing email template.
func (h *AdminHandler) UpdateEmailTemplate(
	ctx context.Context,
	req *connect.Request[adminv1.UpdateEmailTemplateRequest],
) (*connect.Response[adminv1.UpdateEmailTemplateResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Check if template exists
	existing, err := h.emailTemplateRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get email template: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("email template not found"))
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	if req.Msg.Subject != nil {
		updates["subject"] = req.Msg.GetSubject()
	}
	if req.Msg.Body != nil {
		updates["body"] = req.Msg.GetBody()
	}
	if req.Msg.IsActive != nil {
		updates["is_active"] = req.Msg.GetIsActive()
	}
	if req.Msg.Description != nil {
		val := req.Msg.GetDescription()
		if val == "" {
			updates["description"] = nil
		} else {
			updates["description"] = val
		}
	}

	if len(updates) > 0 {
		if err := h.emailTemplateRepo.Update(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update email template: %w", err))
		}
	}

	// Fetch updated template
	updated, err := h.emailTemplateRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated email template: %w", err))
	}

	return connect.NewResponse(&adminv1.UpdateEmailTemplateResponse{
		Template: emailTemplateToProto(updated),
	}), nil
}

// ToggleEmailTemplateActive toggles the active status of an email template.
func (h *AdminHandler) ToggleEmailTemplateActive(
	ctx context.Context,
	req *connect.Request[adminv1.ToggleEmailTemplateActiveRequest],
) (*connect.Response[adminv1.ToggleEmailTemplateActiveResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Check if template exists
	existing, err := h.emailTemplateRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get email template: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("email template not found"))
	}

	if err := h.emailTemplateRepo.SetActive(ctx, req.Msg.GetId(), req.Msg.GetIsActive()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to toggle email template: %w", err))
	}

	// Fetch updated template
	updated, err := h.emailTemplateRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated email template: %w", err))
	}

	return connect.NewResponse(&adminv1.ToggleEmailTemplateActiveResponse{
		Template: emailTemplateToProto(updated),
	}), nil
}

// ============================================================
// Admin Users Management
// ============================================================

// ListAdminUsers returns all users with admin role.
func (h *AdminHandler) ListAdminUsers(
	ctx context.Context,
	req *connect.Request[adminv1.ListAdminUsersRequest],
) (*connect.Response[adminv1.ListAdminUsersResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	adminRoles, err := h.userRoleRepo.GetAdminUsers(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list admin users: %w", err))
	}

	// Fetch profile info for each admin
	protoAdmins := make([]*adminv1.AdminUser, 0, len(adminRoles))
	for _, role := range adminRoles {
		profile, err := h.profileRepo.GetProfile(ctx, role.UserID)
		if err != nil {
			continue // Skip if profile not found
		}

		email := ""
		if profile != nil {
			email = profile.Email
		}

		protoAdmins = append(protoAdmins, &adminv1.AdminUser{
			UserId:    role.UserID,
			Email:     email,
			Role:      role.Role,
			GrantedAt: role.CreatedAt,
		})
	}

	return connect.NewResponse(&adminv1.ListAdminUsersResponse{
		Admins: protoAdmins,
	}), nil
}

// GrantAdminRole grants admin role to a user.
func (h *AdminHandler) GrantAdminRole(
	ctx context.Context,
	req *connect.Request[adminv1.GrantAdminRoleRequest],
) (*connect.Response[adminv1.GrantAdminRoleResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Check if user exists
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Check if already admin
	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}
	if isAdmin {
		return connect.NewResponse(&adminv1.GrantAdminRoleResponse{
			Success: true,
			Message: "User is already an admin",
		}), nil
	}

	// Grant admin role
	if err := h.userRoleRepo.SetUserRole(ctx, userID, models.RoleAdmin); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to grant admin role: %w", err))
	}

	return connect.NewResponse(&adminv1.GrantAdminRoleResponse{
		Success: true,
		Message: fmt.Sprintf("Admin role granted to user %s", profile.Email),
	}), nil
}

// RevokeAdminRole revokes admin role from a user.
func (h *AdminHandler) RevokeAdminRole(
	ctx context.Context,
	req *connect.Request[adminv1.RevokeAdminRoleRequest],
) (*connect.Response[adminv1.RevokeAdminRoleResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Prevent self-revocation
	currentUser := GetUserFromContext(ctx)
	if currentUser != nil && currentUser.ID == userID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot revoke your own admin role"))
	}

	// Check if user is admin
	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}
	if !isAdmin {
		return connect.NewResponse(&adminv1.RevokeAdminRoleResponse{
			Success: true,
			Message: "User is not an admin",
		}), nil
	}

	// Revoke admin role
	if err := h.userRoleRepo.RemoveUserRole(ctx, userID, models.RoleAdmin); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to revoke admin role: %w", err))
	}

	return connect.NewResponse(&adminv1.RevokeAdminRoleResponse{
		Success: true,
		Message: "Admin role revoked successfully",
	}), nil
}

// ============================================================
// User Management
// ============================================================

// ListUsers lists all system users with pagination and filtering.
func (h *AdminHandler) ListUsers(
	ctx context.Context,
	req *connect.Request[adminv1.ListUsersRequest],
) (*connect.Response[adminv1.ListUsersResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Get pagination params
	limit := int(req.Msg.GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursor := req.Msg.GetCursor()
	search := req.Msg.GetSearch()

	// List profiles with filters
	var profiles []*models.Profile
	var nextCursor string
	var err error

	if search != "" {
		profiles, nextCursor, err = h.profileRepo.SearchProfiles(ctx, search, cursor, limit)
	} else {
		profiles, nextCursor, err = h.profileRepo.ListProfiles(ctx, cursor, limit)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list users: %w", err))
	}

	// Convert to proto and get roles for each user
	users := make([]*adminv1.User, len(profiles))
	for i, p := range profiles {
		roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, p.ID)
		users[i] = profileToUserProto(p, roles)
	}

	return connect.NewResponse(&adminv1.ListUsersResponse{
		Users:      users,
		NextCursor: nextCursor,
		TotalCount: int32(len(users)),
	}), nil
}

// CreateUser creates a new user.
func (h *AdminHandler) CreateUser(
	ctx context.Context,
	req *connect.Request[adminv1.CreateUserRequest],
) (*connect.Response[adminv1.CreateUserResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	email := req.Msg.GetEmail()
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}

	plan := req.Msg.GetPlan()
	if plan == "" {
		plan = "starter"
	}

	// Check if user already exists
	existing, _ := h.profileRepo.GetProfileByEmail(ctx, email)
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("user with email %s already exists", email))
	}

	// Generate temporary password
	tempPassword, err := generateTempPassword()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate password: %w", err))
	}

	// Hash password using Argon2id (per CLAUDE.md: Argon2id is the canonical algorithm)
	hashedStr, err := auth.HashPassword(tempPassword)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
	}

	// Create profile
	profile := &models.Profile{
		Email:        email,
		Plan:         plan,
		PasswordHash: &hashedStr,
	}

	// Note: DisplayName is not supported in the current Profile model
	// The proto field exists for future compatibility but is not stored

	if req.Msg.GetPhoneNumber() != "" {
		phone := req.Msg.GetPhoneNumber()
		profile.PhoneNumber = &phone
	}

	createdProfile, err := h.profileRepo.CreateProfile(ctx, profile)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
	}

	// Send welcome email if requested
	var responsePassword *string
	if req.Msg.GetSendWelcomeEmail() && h.emailService != nil {
		// Profile model doesn't have DisplayName, use email as fallback
		name := email
		if err := h.emailService.SendWelcomeEmail(ctx, email, name, tempPassword); err != nil {
			// Log but don't fail - user was created
			responsePassword = &tempPassword
		}
	} else {
		responsePassword = &tempPassword
	}

	roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, createdProfile.ID)

	return connect.NewResponse(&adminv1.CreateUserResponse{
		User:              profileToUserProto(createdProfile, roles),
		TemporaryPassword: responsePassword,
		Message:           "User created successfully",
	}), nil
}

// UpdateUserRole updates a user's role.
func (h *AdminHandler) UpdateUserRole(
	ctx context.Context,
	req *connect.Request[adminv1.UpdateUserRoleRequest],
) (*connect.Response[adminv1.UpdateUserRoleResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	role := req.Msg.GetRole()
	if role == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("role is required"))
	}

	action := req.Msg.GetAction()
	if action != "add" && action != "remove" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("action must be 'add' or 'remove'"))
	}

	// Verify user exists
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil || profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Prevent self-modification for admin role
	currentUser := GetUserFromContext(ctx)
	if role == models.RoleAdmin && currentUser != nil && currentUser.ID == userID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot modify your own admin role"))
	}

	// Apply role change
	if action == "add" {
		if err := h.userRoleRepo.SetUserRole(ctx, userID, role); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add role: %w", err))
		}
	} else {
		if err := h.userRoleRepo.RemoveUserRole(ctx, userID, role); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove role: %w", err))
		}
	}

	// Get updated roles
	roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, userID)

	return connect.NewResponse(&adminv1.UpdateUserRoleResponse{
		Success: true,
		Message: fmt.Sprintf("Role '%s' %sed successfully", role, action),
		Roles:   roles,
	}), nil
}

// ============================================================
// Account Management
// ============================================================

// UnlockUserAccount unlocks a user account by clearing the billing_status.
// This is used to restore access after payment issues are resolved.
func (h *AdminHandler) UnlockUserAccount(
	ctx context.Context,
	req *connect.Request[adminv1.UnlockUserAccountRequest],
) (*connect.Response[adminv1.UnlockUserAccountResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Get current user profile to verify it exists and capture previous billing status
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Capture previous billing status for audit trail
	var previousBillingStatus *string
	if profile.BillingStatus != nil && *profile.BillingStatus != "" {
		previousBillingStatus = profile.BillingStatus
	}

	// Check if account is already unlocked
	if previousBillingStatus == nil {
		return connect.NewResponse(&adminv1.UnlockUserAccountResponse{
			Success: true,
			Message: "Account is already unlocked (no billing suspension)",
		}), nil
	}

	// Clear billing status to unlock the account
	if err := h.profileRepo.UpdateBillingStatus(ctx, userID, nil); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to unlock account: %w", err))
	}

	// Get admin info for audit trail
	currentUser := GetUserFromContext(ctx)
	adminID := ""
	adminEmail := ""
	if currentUser != nil {
		adminID = currentUser.ID
		adminEmail = currentUser.Email
	}

	// Record the action in action_history for audit
	if h.actionHistoryRepo != nil {
		reason := req.Msg.GetReason()
		details := map[string]interface{}{
			"target_user_id":          userID,
			"target_user_email":       profile.Email,
			"previous_billing_status": *previousBillingStatus,
			"admin_id":                adminID,
			"admin_email":             adminEmail,
		}
		if reason != "" {
			details["reason"] = reason
		}

		_, _ = h.actionHistoryRepo.Create(
			ctx,
			adminID, // Use admin's ID as the actor
			models.ActionTypeAdminUnlockAccount,
			fmt.Sprintf("Admin %s unlocked account for user %s (previous status: %s)", adminEmail, profile.Email, *previousBillingStatus),
			details,
		)
	}

	// Also log to system events for observability
	if h.systemEventRepo != nil {
		_ = h.systemEventRepo.Create(ctx, "admin_unlock_account", map[string]interface{}{
			"target_user_id":          userID,
			"target_user_email":       profile.Email,
			"previous_billing_status": *previousBillingStatus,
			"admin_id":                adminID,
			"admin_email":             adminEmail,
			"reason":                  req.Msg.GetReason(),
		})
	}

	return connect.NewResponse(&adminv1.UnlockUserAccountResponse{
		Success:               true,
		Message:               fmt.Sprintf("Account unlocked successfully for user %s", profile.Email),
		PreviousBillingStatus: previousBillingStatus,
	}), nil
}

// ============================================================
// Helper functions
// ============================================================

// profileToUserProto converts a models.Profile to adminv1.User
func profileToUserProto(p *models.Profile, roles []string) *adminv1.User {
	user := &adminv1.User{
		Id:        p.ID,
		Email:     p.Email,
		Plan:      p.Plan,
		CreatedAt: p.CreatedAt, // Already a string in ISO format
		UpdatedAt: p.UpdatedAt, // Already a string in ISO format
		Roles:     roles,
	}

	// Note: DisplayName is not in the Profile model, field left nil
	if p.BillingStatus != nil {
		user.BillingStatus = p.BillingStatus
	}
	if p.PhoneNumber != nil {
		user.PhoneNumber = p.PhoneNumber
	}

	return user
}

// generateTempPassword generates a temporary password
func generateTempPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	password := make([]byte, 12)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}

// planToProto converts a models.Plan to adminv1.Plan
func planToProto(p *models.Plan) *adminv1.Plan {
	proto := &adminv1.Plan{
		Id:           p.ID,
		Slug:         p.Slug,
		Name:         p.Name,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		GroupLimit:   p.GroupLimit,
		StorageLimit: p.StorageLimit,
		Features:     p.Features,
	}

	if p.KiwifyProductIDMonthly != nil {
		proto.KiwifyProductIdMonthly = p.KiwifyProductIDMonthly
	}
	if p.KiwifyProductIDSemester != nil {
		proto.KiwifyProductIdSemester = p.KiwifyProductIDSemester
	}
	if p.KiwifyProductIDAnnual != nil {
		proto.KiwifyProductIdAnnual = p.KiwifyProductIDAnnual
	}
	if p.Description != nil {
		proto.Description = p.Description
	}
	if p.PriceMonthly != nil {
		proto.PriceMonthly = p.PriceMonthly
	}
	if p.PriceSemester != nil {
		proto.PriceSemester = p.PriceSemester
	}
	if p.PriceAnnual != nil {
		proto.PriceAnnual = p.PriceAnnual
	}

	return proto
}

// emailTemplateToProto converts a models.EmailTemplate to adminv1.EmailTemplate
func emailTemplateToProto(t *models.EmailTemplate) *adminv1.EmailTemplate {
	proto := &adminv1.EmailTemplate{
		Id:        t.ID,
		Name:      t.Name,
		Subject:   t.Subject,
		Body:      t.Body,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	if t.Description != nil {
		proto.Description = t.Description
	}

	return proto
}

// ============================================================
// User Investigation & Bulk Operations
// ============================================================

// GetUserInvestigation returns detailed user investigation data including instances and automations.
func (h *AdminHandler) GetUserInvestigation(
	ctx context.Context,
	req *connect.Request[adminv1.GetUserInvestigationRequest],
) (*connect.Response[adminv1.GetUserInvestigationResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Get user profile
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Get admin status
	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}

	// Get user instances
	instances, err := h.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user instances: %w", err))
	}

	// Get user automations
	automations, err := h.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user automations: %w", err))
	}

	// Calculate stats
	var activeCount, pausedCount int32
	for _, a := range automations {
		switch a.Status {
		case "active":
			activeCount++
		case "paused", "inactive":
			pausedCount++
		}
	}

	// Convert instances to proto
	protoInstances := make([]*adminv1.AdminUserInstance, len(instances))
	for i, inst := range instances {
		protoInstances[i] = &adminv1.AdminUserInstance{
			InstanceId: inst.InstanceID,
			Status:     inst.Status,
			UpdatedAt:  stringPtrProto(inst.UpdatedAt),
		}
	}

	// Convert automations to proto
	protoAutomations := make([]*adminv1.AdminUserAutomation, len(automations))
	for i, a := range automations {
		protoAuto := &adminv1.AdminUserAutomation{
			Id:         a.ID,
			Name:       a.Name,
			Status:     a.Status,
			Stores:     a.Stores,
			SendGroups: a.SendGroups,
			Categories: a.Categories,
		}
		if a.Mode != nil {
			protoAuto.Mode = a.Mode
		}
		if a.IntervalMinutes != nil {
			protoAuto.IntervalMinutes = int32Ptr(int32(*a.IntervalMinutes))
		}
		if a.StartTime != nil {
			protoAuto.StartTime = a.StartTime
		}
		if a.EndTime != nil {
			protoAuto.EndTime = a.EndTime
		}
		if a.LastRunAt != nil {
			protoAuto.LastRunAt = a.LastRunAt
		}
		if a.NextRunAt != nil {
			protoAuto.NextRunAt = a.NextRunAt
		}
		protoAuto.CreatedAt = stringPtrProto(a.CreatedAt)
		protoAutomations[i] = protoAuto
	}

	// Get user roles
	roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, userID)

	return connect.NewResponse(&adminv1.GetUserInvestigationResponse{
		User:              profileToUserProto(profile, roles),
		IsAdmin:           isAdmin,
		Instances:         protoInstances,
		Automations:       protoAutomations,
		AutomationsTotal:  int32(len(automations)),
		AutomationsActive: activeCount,
		AutomationsPaused: pausedCount,
	}), nil
}

// AdminBulkUpdateAutomationStatus bulk updates automation status for a user.
func (h *AdminHandler) AdminBulkUpdateAutomationStatus(
	ctx context.Context,
	req *connect.Request[adminv1.AdminBulkUpdateAutomationStatusRequest],
) (*connect.Response[adminv1.AdminBulkUpdateAutomationStatusResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	targetStatus := req.Msg.GetTargetStatus()
	if targetStatus != "active" && targetStatus != "paused" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target_status must be 'active' or 'paused'"))
	}

	// Check if user exists
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Get source status filter
	var sourceStatus *string
	if req.Msg.SourceStatus != nil && *req.Msg.SourceStatus != "" {
		sourceStatus = req.Msg.SourceStatus
	}

	// Perform bulk update
	count, err := h.automationRepo.BulkUpdateStatusByUserID(ctx, userID, targetStatus, sourceStatus)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update automations: %w", err))
	}

	// Log the action
	currentUser := GetUserFromContext(ctx)
	if h.systemEventRepo != nil && currentUser != nil {
		_ = h.systemEventRepo.Create(ctx, "admin_bulk_update_automations", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": profile.Email,
			"target_status":     targetStatus,
			"source_status":     sourceStatus,
			"updated_count":     count,
			"admin_id":          currentUser.ID,
			"admin_email":       currentUser.Email,
		})
	}

	return connect.NewResponse(&adminv1.AdminBulkUpdateAutomationStatusResponse{
		UpdatedCount: int32(count),
		Message:      fmt.Sprintf("Updated %d automations to status '%s'", count, targetStatus),
	}), nil
}

// AdminSpreadAutomationSchedule spreads next_run_at across user's automations.
func (h *AdminHandler) AdminSpreadAutomationSchedule(
	ctx context.Context,
	req *connect.Request[adminv1.AdminSpreadAutomationScheduleRequest],
) (*connect.Response[adminv1.AdminSpreadAutomationScheduleResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	spreadMinutes := int(req.Msg.GetSpreadMinutes())
	if spreadMinutes <= 0 {
		spreadMinutes = 2
	}

	// Check if user exists
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Get user automations
	automations, err := h.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user automations: %w", err))
	}

	// Filter active automations if requested
	var targetAutomations []models.Automation
	for _, a := range automations {
		if req.Msg.GetActiveOnly() && a.Status != "active" {
			continue
		}
		targetAutomations = append(targetAutomations, a)
	}

	if len(targetAutomations) == 0 {
		return connect.NewResponse(&adminv1.AdminSpreadAutomationScheduleResponse{
			UpdatedCount: 0,
			Message:      "No automations to spread",
		}), nil
	}

	// Calculate spread times starting 1 minute from now
	now := time.Now().UTC()
	baseTime := now.Add(time.Minute)

	automationIDs := make([]string, len(targetAutomations))
	nextRunAts := make([]time.Time, len(targetAutomations))
	for i, a := range targetAutomations {
		automationIDs[i] = a.ID
		nextRunAts[i] = baseTime.Add(time.Duration(i*spreadMinutes) * time.Minute)
	}

	// Perform bulk update
	if err := h.automationRepo.BulkUpdateNextRunAts(ctx, automationIDs, nextRunAts); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update next_run_at: %w", err))
	}

	// Log the action
	currentUser := GetUserFromContext(ctx)
	if h.systemEventRepo != nil && currentUser != nil {
		_ = h.systemEventRepo.Create(ctx, "admin_spread_automation_schedule", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": profile.Email,
			"spread_minutes":    spreadMinutes,
			"active_only":       req.Msg.GetActiveOnly(),
			"updated_count":     len(targetAutomations),
			"admin_id":          currentUser.ID,
			"admin_email":       currentUser.Email,
		})
	}

	return connect.NewResponse(&adminv1.AdminSpreadAutomationScheduleResponse{
		UpdatedCount: int32(len(targetAutomations)),
		Message:      fmt.Sprintf("Spread next_run_at for %d automations with %d minute intervals", len(targetAutomations), spreadMinutes),
	}), nil
}

// AdminBlockUser blocks a user with extended options.
func (h *AdminHandler) AdminBlockUser(
	ctx context.Context,
	req *connect.Request[adminv1.AdminBlockUserRequest],
) (*connect.Response[adminv1.AdminBlockUserResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user_id is required"))
	}

	// Prevent admin from blocking themselves
	currentUser := GetUserFromContext(ctx)
	if currentUser != nil && currentUser.ID == userID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cannot block your own account"))
	}

	// Check if user exists
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user profile: %w", err))
	}
	if profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Determine suspension reason and kind
	reason := req.Msg.GetReason()
	if reason == "" {
		reason = "admin_manual_block"
	}

	kind := req.Msg.GetKind()
	if kind == "" {
		kind = models.BillingSuspensionKindManual
	}

	now := time.Now().UTC()
	billingStatus := "suspended"

	// Update billing status to block the user
	billing := &models.ProfileBillingFields{
		BillingStatus:           &billingStatus,
		BillingSuspendedAt:      &now,
		BillingSuspensionReason: &reason,
		BillingSuspensionKind:   &kind,
	}

	if err := h.profileRepo.UpdateBillingStatus(ctx, userID, billing); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to block user: %w", err))
	}

	// Pause automations if requested
	var pausedCount int32
	if req.Msg.GetPauseAutomations() {
		sourceStatus := "active"
		count, err := h.automationRepo.BulkUpdateStatusByUserID(ctx, userID, "paused", &sourceStatus)
		if err != nil {
			// Log but don't fail the request
			if h.systemEventRepo != nil {
				_ = h.systemEventRepo.Create(ctx, "admin_block_user_pause_failed", map[string]interface{}{
					"user_id": userID,
					"error":   err.Error(),
				})
			}
		} else {
			pausedCount = int32(count)
		}
	}

	// Log the action
	if h.systemEventRepo != nil && currentUser != nil {
		_ = h.systemEventRepo.Create(ctx, "admin_block_user", map[string]interface{}{
			"target_user_id":     userID,
			"target_user_email":  profile.Email,
			"reason":             reason,
			"kind":               kind,
			"pause_automations":  req.Msg.GetPauseAutomations(),
			"paused_automations": pausedCount,
			"admin_id":           currentUser.ID,
			"admin_email":        currentUser.Email,
		})
	}

	// Fetch updated profile for response
	updatedProfile, _ := h.profileRepo.GetProfile(ctx, userID)
	if updatedProfile == nil {
		updatedProfile = profile
	}
	roles, _ := h.userRoleRepo.GetAllUserRoles(ctx, userID)

	return connect.NewResponse(&adminv1.AdminBlockUserResponse{
		User:              profileToUserProto(updatedProfile, roles),
		PausedAutomations: pausedCount,
		Message:           fmt.Sprintf("User %s blocked successfully", profile.Email),
	}), nil
}

// int32Ptr returns a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
}

// ============================================================
// Categories Management
// ============================================================

// ListCategories returns all categories with optional subcategories.
// Any authenticated user can list active categories; includeInactive requires admin.
func (h *AdminHandler) ListCategories(
	ctx context.Context,
	req *connect.Request[adminv1.ListCategoriesRequest],
) (*connect.Response[adminv1.ListCategoriesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	// Only admins can list inactive categories
	includeInactive := req.Msg.GetIncludeInactive()
	if includeInactive {
		if err := h.requireAdmin(ctx); err != nil {
			return nil, err
		}
	}
	includeSubcategories := req.Msg.GetIncludeSubcategories()

	var categories []models.Category
	var err error

	if includeSubcategories {
		categories, err = h.categoryRepo.ListCategoriesWithSubcategories(ctx, includeInactive)
	} else {
		categories, err = h.categoryRepo.ListCategories(ctx, includeInactive)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list categories: %w", err))
	}

	protoCategories := make([]*adminv1.Category, len(categories))
	for i, c := range categories {
		protoCategories[i] = categoryToProto(&c)
	}

	return connect.NewResponse(&adminv1.ListCategoriesResponse{
		Categories: protoCategories,
	}), nil
}

// CreateCategory creates a new category.
func (h *AdminHandler) CreateCategory(
	ctx context.Context,
	req *connect.Request[adminv1.CreateCategoryRequest],
) (*connect.Response[adminv1.CreateCategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	slug := req.Msg.GetSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	createReq := &models.CreateCategoryRequest{
		Slug:           slug,
		Name:           name,
		HasRefinements: req.Msg.GetHasRefinements(),
	}

	if req.Msg.Icon != nil {
		icon := req.Msg.GetIcon()
		createReq.Icon = &icon
	}
	if req.Msg.DisplayOrder != nil {
		order := int(req.Msg.GetDisplayOrder())
		createReq.DisplayOrder = &order
	}

	created, err := h.categoryRepo.CreateCategory(ctx, createReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create category: %w", err))
	}

	return connect.NewResponse(&adminv1.CreateCategoryResponse{
		Category: categoryToProto(created),
	}), nil
}

// UpdateCategory updates an existing category.
func (h *AdminHandler) UpdateCategory(
	ctx context.Context,
	req *connect.Request[adminv1.UpdateCategoryRequest],
) (*connect.Response[adminv1.UpdateCategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	updateReq := &models.UpdateCategoryRequest{}
	if req.Msg.Name != nil {
		name := req.Msg.GetName()
		updateReq.Name = &name
	}
	if req.Msg.Icon != nil {
		icon := req.Msg.GetIcon()
		updateReq.Icon = &icon
	}
	if req.Msg.DisplayOrder != nil {
		order := int(req.Msg.GetDisplayOrder())
		updateReq.DisplayOrder = &order
	}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		updateReq.IsActive = &active
	}
	if req.Msg.HasRefinements != nil {
		hasRef := req.Msg.GetHasRefinements()
		updateReq.HasRefinements = &hasRef
	}

	updated, err := h.categoryRepo.UpdateCategory(ctx, id, updateReq)
	if err != nil {
		if errors.Is(err, postgres.ErrCategoryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("category not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update category: %w", err))
	}

	return connect.NewResponse(&adminv1.UpdateCategoryResponse{
		Category: categoryToProto(updated),
	}), nil
}

// DeleteCategory deletes a category (only non-system categories).
func (h *AdminHandler) DeleteCategory(
	ctx context.Context,
	req *connect.Request[adminv1.DeleteCategoryRequest],
) (*connect.Response[adminv1.DeleteCategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	err := h.categoryRepo.DeleteCategory(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrCategoryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("category not found"))
		}
		if errors.Is(err, postgres.ErrSystemCategoryDelete) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete system category"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete category: %w", err))
	}

	return connect.NewResponse(&adminv1.DeleteCategoryResponse{
		Success: true,
	}), nil
}

// CreateSubcategory creates a new subcategory.
func (h *AdminHandler) CreateSubcategory(
	ctx context.Context,
	req *connect.Request[adminv1.CreateSubcategoryRequest],
) (*connect.Response[adminv1.CreateSubcategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	categoryID := req.Msg.GetCategoryId()
	if categoryID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("category_id is required"))
	}

	slug := req.Msg.GetSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	displayKey := req.Msg.GetDisplayKey()
	if displayKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("display_key is required"))
	}

	createReq := &models.CreateSubcategoryRequest{
		CategoryID: categoryID,
		Slug:       slug,
		Name:       name,
		DisplayKey: displayKey,
	}

	if req.Msg.GroupName != nil {
		groupName := req.Msg.GetGroupName()
		createReq.GroupName = &groupName
	}
	if req.Msg.DisplayOrder != nil {
		order := int(req.Msg.GetDisplayOrder())
		createReq.DisplayOrder = &order
	}

	created, err := h.categoryRepo.CreateSubcategory(ctx, createReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create subcategory: %w", err))
	}

	return connect.NewResponse(&adminv1.CreateSubcategoryResponse{
		Subcategory: subcategoryToProto(created),
	}), nil
}

// UpdateSubcategory updates an existing subcategory.
func (h *AdminHandler) UpdateSubcategory(
	ctx context.Context,
	req *connect.Request[adminv1.UpdateSubcategoryRequest],
) (*connect.Response[adminv1.UpdateSubcategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	updateReq := &models.UpdateSubcategoryRequest{}
	if req.Msg.Name != nil {
		name := req.Msg.GetName()
		updateReq.Name = &name
	}
	if req.Msg.DisplayKey != nil {
		displayKey := req.Msg.GetDisplayKey()
		updateReq.DisplayKey = &displayKey
	}
	if req.Msg.GroupName != nil {
		groupName := req.Msg.GetGroupName()
		updateReq.GroupName = &groupName
	}
	if req.Msg.DisplayOrder != nil {
		order := int(req.Msg.GetDisplayOrder())
		updateReq.DisplayOrder = &order
	}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		updateReq.IsActive = &active
	}

	updated, err := h.categoryRepo.UpdateSubcategory(ctx, id, updateReq)
	if err != nil {
		if errors.Is(err, postgres.ErrSubcategoryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("subcategory not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update subcategory: %w", err))
	}

	return connect.NewResponse(&adminv1.UpdateSubcategoryResponse{
		Subcategory: subcategoryToProto(updated),
	}), nil
}

// DeleteSubcategory deletes a subcategory (only non-system subcategories).
func (h *AdminHandler) DeleteSubcategory(
	ctx context.Context,
	req *connect.Request[adminv1.DeleteSubcategoryRequest],
) (*connect.Response[adminv1.DeleteSubcategoryResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if h.categoryRepo == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("category repository not configured"))
	}

	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	err := h.categoryRepo.DeleteSubcategory(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrSubcategoryNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("subcategory not found"))
		}
		if errors.Is(err, postgres.ErrSystemSubcatDelete) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete system subcategory"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete subcategory: %w", err))
	}

	return connect.NewResponse(&adminv1.DeleteSubcategoryResponse{
		Success: true,
	}), nil
}

// categoryToProto converts a models.Category to adminv1.Category
func categoryToProto(c *models.Category) *adminv1.Category {
	proto := &adminv1.Category{
		Id:             c.ID,
		Slug:           c.Slug,
		Name:           c.Name,
		DisplayOrder:   int32(c.DisplayOrder),
		IsActive:       c.IsActive,
		HasRefinements: c.HasRefinements,
		IsSystem:       c.IsSystem,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.Format(time.RFC3339),
	}

	if c.Icon != nil {
		proto.Icon = c.Icon
	}

	// Convert subcategories if present
	if len(c.Subcategories) > 0 {
		proto.Subcategories = make([]*adminv1.Subcategory, len(c.Subcategories))
		for i, s := range c.Subcategories {
			proto.Subcategories[i] = subcategoryToProto(&s)
		}
	}

	return proto
}

// subcategoryToProto converts a models.Subcategory to adminv1.Subcategory
func subcategoryToProto(s *models.Subcategory) *adminv1.Subcategory {
	proto := &adminv1.Subcategory{
		Id:           s.ID,
		CategoryId:   s.CategoryID,
		Slug:         s.Slug,
		Name:         s.Name,
		DisplayKey:   s.DisplayKey,
		DisplayOrder: int32(s.DisplayOrder),
		IsActive:     s.IsActive,
		IsSystem:     s.IsSystem,
		CreatedAt:    s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
	}

	if s.GroupName != nil {
		proto.GroupName = s.GroupName
	}

	return proto
}
