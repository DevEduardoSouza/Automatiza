package automation

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"automatiza-go/internal/config"
	bpLib "automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/mercadolivre"
	"automatiza-go/internal/services/shopee"
	sharedErrors "automatiza-go/internal/shared/errors"
	"automatiza-go/internal/workers"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// executionLockDuration defines how long an automation execution lock is held
const executionLockDuration = 10 * time.Minute

// NOTE: This service no longer depends on internal/services/waha directly.
// WhatsApp operations are handled via:
// 1. The queue client (workers.QueueClient) for message sending
// 2. The whatsapp services package (internal/services/whatsapp) for direct operations
// 3. The whatsapp adapter (internal/adapters/whatsapp) which implements WhatsAppProvider

// Service defines the automation service interface
type Service interface {
	DiagnoseAutomation(ctx context.Context, automationID, userID string) (*models.DiagnoseResult, error)
	RunAutomation(ctx context.Context, automationID, userID string) (*models.RunAutomationResult, error)
	ResetHistory(ctx context.Context, automationID, userID string) error
	CanCreateAutomation(ctx context.Context, userID string) (canCreate bool, currentCount int, maxCount int, err error)
}

// urlSafetyEvaluator abstracts URL safety evaluation for testability.
type urlSafetyEvaluator interface {
	Evaluate(ctx context.Context, message string) (*urlsafety.UrlSafetyDecision, error)
}

// backpressureProvider abstracts backpressure checking for testability.
type backpressureProvider interface {
	CanEnqueue(ctx context.Context, userID string) (bool, string, error)
	RecordEnqueue(ctx context.Context, userID string) error
}

type automationService struct {
	automationRepo     repository.AutomationRepository
	automationRunRepo  repository.AutomationRunRepository
	instanceRepo       repository.InstanceRepository
	groupRepo          repository.GroupRepository
	dispatchLogRepo    repository.DispatchLogRepository
	historyRepo        repository.ActionHistoryRepository
	settingsRepo       repository.SettingsRepository
	profileRepo        repository.ProfileRepository
	mlLinkListItemRepo repository.MlLinkListItemRepository

	shopeeService    shopee.Service
	mlService        mercadolivre.Service
	urlSafetyService urlSafetyEvaluator
	backpressure     backpressureProvider

	queueClient *workers.QueueClient
	cache       *cache.Cache
	logger      *zap.Logger
}

// NewAutomationService creates a new automation service.
//
// NOTE: This service uses the queue client for WhatsApp message sending.
// Direct WhatsApp operations should use the whatsapp services package
// (internal/services/whatsapp) which depends on the WhatsAppProvider interface.
func NewAutomationService(
	automationRepo repository.AutomationRepository,
	automationRunRepo repository.AutomationRunRepository,
	instanceRepo repository.InstanceRepository,
	groupRepo repository.GroupRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	historyRepo repository.ActionHistoryRepository,
	settingsRepo repository.SettingsRepository,
	profileRepo repository.ProfileRepository,
	mlLinkListItemRepo repository.MlLinkListItemRepository,
	shopeeService shopee.Service,
	mlService mercadolivre.Service,
	urlSafetyService *urlsafety.Service,
	bp *bpLib.ProducerBackpressure,
	queueClient *workers.QueueClient,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	svc := &automationService{
		automationRepo:     automationRepo,
		automationRunRepo:  automationRunRepo,
		instanceRepo:       instanceRepo,
		groupRepo:          groupRepo,
		dispatchLogRepo:    dispatchLogRepo,
		historyRepo:        historyRepo,
		settingsRepo:       settingsRepo,
		profileRepo:        profileRepo,
		mlLinkListItemRepo: mlLinkListItemRepo,
		shopeeService:      shopeeService,
		mlService:          mlService,
		queueClient:        queueClient,
		cache:              cache,
		logger:             logger,
	}
	// Assign concrete pointers to interface fields only when non-nil
	// to preserve correct nil-check semantics (avoid non-nil interface wrapping nil pointer).
	if urlSafetyService != nil {
		svc.urlSafetyService = urlSafetyService
	}
	if bp != nil {
		svc.backpressure = bp
	}
	return svc
}

func (s *automationService) DiagnoseAutomation(ctx context.Context, automationID, userID string) (*models.DiagnoseResult, error) {
	// Get automation
	automation, err := s.automationRepo.GetAutomationByIDAndUser(ctx, automationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get automation: %w", err)
	}
	if automation == nil {
		return nil, fmt.Errorf("automation not found")
	}

	result := &models.DiagnoseResult{
		Status: "ok",
		Checks: []models.DiagnoseCheck{},
	}

	// Step 1: CONFIG - Automation configuration valid
	configCheck := models.DiagnoseCheck{
		Name:   "CONFIG",
		Status: "ok",
	}
	if !automation.IsActive() {
		configCheck.Status = "warning"
		configCheck.Message = "Automacao esta inativa"
	} else {
		configCheck.Message = "Configuracao valida"
	}
	result.Checks = append(result.Checks, configCheck)

	// Step 2: WHATSAPP - Instance connected and working
	whatsappCheck := models.DiagnoseCheck{
		Name:   "WHATSAPP",
		Status: "ok",
	}
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil || len(instances) == 0 {
		result.Status = "error"
		whatsappCheck.Status = "error"
		whatsappCheck.Message = "Nenhuma instancia WhatsApp encontrada"
		result.Errors = append(result.Errors, whatsappCheck.Message)
	} else {
		connectedCount := 0
		for _, inst := range instances {
			if inst.Status == "connected" {
				connectedCount++
			}
		}
		if connectedCount == 0 {
			result.Status = "error"
			whatsappCheck.Status = "error"
			whatsappCheck.Message = "Nenhuma instancia conectada"
			result.Errors = append(result.Errors, whatsappCheck.Message)
		} else {
			whatsappCheck.Message = fmt.Sprintf("%d instancia(s) conectada(s)", connectedCount)
		}
	}
	result.Checks = append(result.Checks, whatsappCheck)

	// Step 3: GRUPOS - Target groups exist and accessible
	gruposCheck := models.DiagnoseCheck{
		Name:   "GRUPOS",
		Status: "ok",
	}
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		if result.Status != "error" {
			result.Status = "warning"
		}
		result.Warnings = append(result.Warnings, "Nenhum grupo configurado para envio")
		gruposCheck.Status = "error"
		gruposCheck.Message = "Nenhum grupo configurado"
	} else {
		gruposCheck.Message = fmt.Sprintf("%d grupos configurados", len(targetGroups))
	}
	result.Checks = append(result.Checks, gruposCheck)

	// Step 4: BUSCA_STORE - Store API accessible (Shopee, ML, etc)
	buscaStoreCheck := models.DiagnoseCheck{
		Name:   "BUSCA_STORE",
		Status: "ok",
	}
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}

	automationType := automation.GetType()
	if automationType == "search" {
		storeErrors := []string{}
		for _, store := range stores {
			switch store {
			case "shopee":
				if s.shopeeService != nil {
					_, err := s.shopeeService.GetCategories(ctx, nil)
					if err != nil {
						storeErrors = append(storeErrors, fmt.Sprintf("Shopee: %v", err))
					}
				} else {
					storeErrors = append(storeErrors, "Shopee: servico nao configurado")
				}
			case "mercadolivre":
				if s.mlService != nil {
					connected, _ := s.mlService.IsConnected(ctx, userID)
					if !connected {
						storeErrors = append(storeErrors, "Mercado Livre: nao conectado")
					}
				} else {
					storeErrors = append(storeErrors, "Mercado Livre: servico nao configurado")
				}
			}
		}

		if len(storeErrors) > 0 {
			buscaStoreCheck.Status = "error"
			buscaStoreCheck.Message = strings.Join(storeErrors, "; ")
			result.Warnings = append(result.Warnings, storeErrors...)
		} else {
			buscaStoreCheck.Message = fmt.Sprintf("APIs acessiveis: %s", strings.Join(stores, ", "))
		}
	} else {
		buscaStoreCheck.Status = "ok"
		buscaStoreCheck.Message = "Busca nao necessaria para este tipo de automacao"
	}
	result.Checks = append(result.Checks, buscaStoreCheck)

	// Step 5: ANTIDUP - Deduplication cache working
	antidupCheck := models.DiagnoseCheck{
		Name:   "ANTIDUP",
		Status: "ok",
	}
	if s.cache != nil {
		if err := s.cache.Ping(ctx); err != nil {
			antidupCheck.Status = "warning"
			antidupCheck.Message = fmt.Sprintf("Cache indisponivel: %v. Deduplicacao pode nao funcionar", err)
		} else {
			antidupCheck.Message = "Cache de deduplicacao operational"
		}
	} else {
		antidupCheck.Status = "warning"
		antidupCheck.Message = "Cache nao configurado. Deduplicacao pode nao funcionar"
	}
	result.Checks = append(result.Checks, antidupCheck)

	// Step 6: ML_LINK_LIST - Check ML link list configuration (for type="list" only)
	if automationType == "list" {
		mlLinkListCheck := models.DiagnoseCheck{
			Name:   "ML_LINK_LIST",
			Status: "ok",
		}

		if automation.MlLinkListID == nil || *automation.MlLinkListID == "" {
			result.Status = "error"
			mlLinkListCheck.Status = "error"
			mlLinkListCheck.Message = "Lista de links nao configurada. Selecione uma Lista de Links (Mercado Livre)."
			result.Errors = append(result.Errors, mlLinkListCheck.Message)
		} else {
			// Check if list has unconsumed items
			listID := *automation.MlLinkListID
			items, err := s.mlLinkListItemRepo.GetNextUnconsumed(ctx, listID, 1)
			if err != nil {
				mlLinkListCheck.Status = "warning"
				mlLinkListCheck.Message = fmt.Sprintf("Erro ao verificar lista: %v", err)
				result.Warnings = append(result.Warnings, mlLinkListCheck.Message)
			} else if len(items) == 0 {
				mlLinkListCheck.Status = "warning"
				mlLinkListCheck.Message = "Lista sem itens disponiveis (todos consumidos ou inativos)"
				result.Warnings = append(result.Warnings, mlLinkListCheck.Message)
			} else {
				mlLinkListCheck.Message = "Lista configurada com itens disponiveis"
			}
		}
		result.Checks = append(result.Checks, mlLinkListCheck)
	}

	// Additional check: Time window (if configured) - added as warning only
	if automation.StartTime != nil && automation.EndTime != nil {
		tw := &models.TimeWindow{
			StartTime: *automation.StartTime,
			EndTime:   *automation.EndTime,
			Timezone:  "America/Sao_Paulo",
		}
		if automation.ScheduleDays != nil {
			tw.Days = automation.ScheduleDays
		}

		if !tw.IsWithinWindow(time.Now()) {
			result.Warnings = append(result.Warnings, "Fora da janela de horario configurada")
		}
	}

	return result, nil
}

func (s *automationService) RunAutomation(ctx context.Context, automationID, userID string) (*models.RunAutomationResult, error) {
	// Get automation
	automation, err := s.automationRepo.GetAutomationByIDAndUser(ctx, automationID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get automation: %w", err)
	}
	if automation == nil {
		return nil, fmt.Errorf("automation not found")
	}

	s.logger.Info("RunAutomation started",
		zap.String("automationID", automation.ID),
		zap.String("userID", userID),
		zap.String("type", automation.GetType()),
		zap.String("name", automation.Name))

	// Acquire execution lock to prevent concurrent runs
	_, err = s.acquireExecutionLock(ctx, automation.ID)
	if err != nil {
		// Check if it's a conflict error (lock held by another execution)
		if sharedErrors.IsConflictError(err) {
			return nil, err // Handler will map to appropriate gRPC code
		}
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	s.logger.Info("Execution lock acquired",
		zap.String("automationID", automation.ID))

	// Release lock when done
	defer func() { _ = s.releaseExecutionLock(ctx, automation.ID) }()

	// Always update next_run_at when execution completes (success or error)
	// This ensures the scheduler knows when to run again even if this run fails
	defer func() {
		if automation.IntervalMinutes != nil && *automation.IntervalMinutes > 0 {
			nextRun := time.Now().UTC().Add(time.Duration(*automation.IntervalMinutes) * time.Minute)
			if updateErr := s.automationRepo.UpdateNextRunAt(ctx, automation.ID, nextRun); updateErr != nil {
				s.logger.Warn("Failed to update next_run_at",
					zap.String("automationID", automation.ID),
					zap.Error(updateErr))
			}
		}
	}()

	// Create action history entry (fire-and-forget)
	history, histCreateErr := s.historyRepo.Create(ctx, userID, "automation_run", "Executando automacao: "+automation.Name, map[string]interface{}{
		"automation_id": automationID,
		"type":          automation.GetType(),
	})
	if histCreateErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "history create failed (fire-and-forget)", histCreateErr, map[string]string{
			"module": "automation", "operation": "run_automation_history_create",
		})
	}

	var historyID string
	if history != nil {
		historyID = history.ID
		// Set target_count based on configured groups and mark as running
		targetGroups := automation.GetTargetGroups()
		if histErr := s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"target_count": int32(len(targetGroups)),
		}); histErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "history update failed (fire-and-forget)", histErr, map[string]string{
				"module": "automation", "operation": "run_automation_history_target_count",
			})
		}
		if histErr := s.historyRepo.MarkRunning(ctx, historyID); histErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "history mark running failed (fire-and-forget)", histErr, map[string]string{
				"module": "automation", "operation": "run_automation_history_mark_running",
			})
		}
		s.logger.Info("History entry created",
			zap.String("automationID", automation.ID),
			zap.String("historyID", historyID),
			zap.Int("targetGroupCount", len(targetGroups)))
	}

	// Determine automation type and execute
	var result *models.RunAutomationResult

	switch automation.GetType() {
	case "search":
		s.logger.Info("Dispatching search mode", zap.String("automationID", automation.ID))
		result, err = s.executeSearchAutomation(ctx, automation, historyID)
	case "monitoring":
		s.logger.Info("Dispatching monitor mode", zap.String("automationID", automation.ID))
		// MONITOR mode: validate configuration and return status
		// Actual message processing happens via webhooks in monitoring_service
		result, err = s.processMonitorMode(ctx, automation)
	case "list":
		s.logger.Info("Dispatching list mode", zap.String("automationID", automation.ID))
		// LIST mode: process items from ML link list
		result, err = s.processListMode(ctx, automation)
	default:
		s.logger.Info("Dispatching default (search) mode", zap.String("automationID", automation.ID))
		result, err = s.executeSearchAutomation(ctx, automation, historyID)
	}

	// Update history with status and counters (fire-and-forget)
	if historyID != "" {
		status := "completed"
		if err != nil {
			status = "failed"
		}
		if histErr := s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"status":       status,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		}); histErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "history update failed (fire-and-forget)", histErr, map[string]string{
				"module": "automation", "operation": "run_automation_history_status_update",
			})
		}
		// Update counters from result if available
		if result != nil && result.MessagesSent > 0 {
			if histErr := s.historyRepo.UpdateCounts(ctx, historyID, int32(result.MessagesSent), 0); histErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "history update counts failed (fire-and-forget)", histErr, map[string]string{
					"module": "automation", "operation": "run_automation_history_update_counts",
				})
			}
		}
	}

	if err != nil {
		return &models.RunAutomationResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Update last_run_at after successful execution
	if updateErr := s.automationRepo.UpdateLastRunAt(ctx, automation.ID); updateErr != nil {
		s.logger.Warn("Failed to update last_run_at",
			zap.String("automationID", automation.ID),
			zap.Error(updateErr))
	}

	if observability.GlobalMetrics != nil {
		mode := "unknown"
		if automation.Mode != nil {
			mode = *automation.Mode
		}
		observability.GlobalMetrics.AutomationRuns.Add(ctx, 1,
			metric.WithAttributes(attribute.String("mode", mode)))
	}

	return result, nil
}

func (s *automationService) executeSearchAutomation(ctx context.Context, automation *models.Automation, historyID string) (*models.RunAutomationResult, error) {
	// Get target groups
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		return nil, fmt.Errorf("no target groups configured")
	}

	// Get connected instance
	instances, err := s.instanceRepo.GetUserInstances(ctx, automation.UserID)
	if err != nil || len(instances) == 0 {
		return nil, fmt.Errorf("no WhatsApp instance found")
	}

	var connectedInstance *models.Instance
	for _, inst := range instances {
		if inst.Status == "connected" {
			connectedInstance = &inst
			break
		}
	}
	if connectedInstance == nil {
		return nil, fmt.Errorf("no connected WhatsApp instance")
	}

	// Determine stores to search
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}

	s.logger.Info("Search automation setup complete",
		zap.String("automationID", automation.ID),
		zap.Int("targetGroups", len(targetGroups)),
		zap.String("instanceID", connectedInstance.InstanceID),
		zap.Strings("stores", stores))

	// Build keywords list: prefer Texts array, fallback to single Keyword
	keywords := automation.Texts
	if len(keywords) == 0 && automation.Keyword != nil && *automation.Keyword != "" {
		keywords = []string{*automation.Keyword}
	}
	if len(keywords) == 0 {
		keywords = []string{""} // empty keyword = broad search
	}

	// Get dispatch text template (only if active) — resolve once before search loop
	var dispatchTemplate string
	if automation.DispatchTextID != nil {
		dt, _ := s.settingsRepo.GetActiveDispatchTextByID(ctx, *automation.DispatchTextID, automation.UserID)
		if dt != nil {
			dispatchTemplate = dt.Body
		}
	}
	if dispatchTemplate == "" {
		dispatchTemplate = automation.GetCustomMessage()
	}
	if dispatchTemplate == "" {
		dispatchTemplate = "🛍️ {{PRODUTO_NOME}}\n\nDe {{PRODUTO_PRECO_ORIGINAL}}\n💸 Por {{PRODUTO_PRECO}}\n\n🛒Compre aqui: {{LINK_AFILIADO}}\n\n⚠ Promoção sujeita a alteração a qualquer momento."
	}

	// Detect templates without variables — all messages will be identical, risking WhatsApp anti-spam suppression
	templateVars := text.ExtractTemplateVariables(dispatchTemplate)
	if len(templateVars) == 0 {
		s.logger.Warn("Dispatch template has no variables — all messages will be identical",
			zap.String("automationID", automation.ID),
			zap.Int("templateLen", len(dispatchTemplate)))
	}

	s.logger.Info("Dispatch template resolved",
		zap.String("automationID", automation.ID),
		zap.Int("templateLen", len(dispatchTemplate)),
		zap.Int("variableCount", len(templateVars)))

	// Send offers to groups
	sentCount := 0
	skippedCount := 0
	dedupeHours := automation.GetDedupeHours()

	// Pagination: when all offers from page N are deduped, try page N+1.
	// This prevents niche categories with limited product variety from getting stuck.
	const maxSearchPages = 3
	const limitPerKeyword = 20
	var totalOffersFound int

	for pageAttempt := 0; pageAttempt < maxSearchPages; pageAttempt++ {
		searchOffset := pageAttempt * limitPerKeyword

		// Search for products
		var allOffers []models.ShopeeOffer
		for _, store := range stores {
			switch store {
			case "shopee":
				if s.shopeeService != nil {
					for _, kw := range keywords {
						kwCopy := kw
						var kwPtr *string
						if kwCopy != "" {
							kwPtr = &kwCopy
						}
						params := models.ShopeeSearchParams{
							Keyword:     kwPtr,
							Categories:  automation.Categories,
							MinPrice:    automation.MinPrice,
							MaxPrice:    automation.MaxPrice,
							MinDiscount: automation.MinDiscount,
							Limit:       limitPerKeyword,
							Offset:      searchOffset,
						}
						result, err := s.shopeeService.SearchOffers(ctx, automation.UserID, params)
						if err != nil {
							s.logger.Warn("Shopee search failed",
								zap.String("automationID", automation.ID),
								zap.String("keyword", kwCopy),
								zap.Int("page", pageAttempt+1),
								zap.Error(err))
						} else if result != nil {
							s.logger.Info("Shopee search completed",
								zap.String("automationID", automation.ID),
								zap.String("keyword", kwCopy),
								zap.Int("page", pageAttempt+1),
								zap.Int("offersReturned", len(result.Offers)))
							allOffers = append(allOffers, result.Offers...)
						}
					}
				}
			case "mercadolivre":
				// ML search would go here
				// For now, ML is mainly used for affiliate link generation
			}
		}

		if len(allOffers) == 0 {
			if totalOffersFound == 0 {
				return &models.RunAutomationResult{
					Success:     true,
					Message:     "Nenhuma oferta encontrada com os filtros configurados",
					OffersFound: 0,
				}, nil
			}
			// No more pages available
			break
		}

		totalOffersFound += len(allOffers)

		// Shuffle offers to ensure variety even if dedup has gaps
		rand.Shuffle(len(allOffers), func(i, j int) {
			allOffers[i], allOffers[j] = allOffers[j], allOffers[i]
		})

		// Iterate through offers and send the FIRST non-duplicate to all groups.
		// We fetch ~20 offers but only send 1 per run; dedup ensures variety across runs.
		for i, offer := range allOffers {
			canonicalKey := models.CanonicalKeyFromShopeeOffer(&offer)
			store := "shopee"

			// Pre-check: if this offer is already a duplicate for ALL target groups, skip entirely
			allGroupsDuped := true
			for _, groupID := range targetGroups {
				if !s.isDuplicate(ctx, automation.UserID, groupID, canonicalKey, dedupeHours) {
					allGroupsDuped = false
					break
				}
			}
			if allGroupsDuped {
				s.logger.Info("Offer skipped: dedup (all groups)",
					zap.String("automationID", automation.ID),
					zap.String("canonicalKey", canonicalKey),
					zap.Int("offerIndex", i),
					zap.Int("page", pageAttempt+1))
				for _, groupID := range targetGroups {
					s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "dedupe", withCanonicalKey(canonicalKey, store))
				}
				skippedCount += len(targetGroups)
				continue
			}

			// This offer is new for at least one group — refetch price and send to all groups
			if s.shopeeService != nil && offer.ItemID > 0 && offer.ShopID > 0 {
				refreshed, err := s.shopeeService.GetProduct(ctx, offer.ItemID, offer.ShopID, automation.UserID)
				if err != nil {
					s.logger.Warn("Failed to refetch product price, using search data",
						zap.Int64("itemID", offer.ItemID),
						zap.Int64("shopID", offer.ShopID),
						zap.Error(err))
				} else if refreshed != nil {
					if refreshed.Price > 0 {
						allOffers[i].Price = refreshed.Price
					}
					if refreshed.OriginalPrice > 0 {
						allOffers[i].OriginalPrice = refreshed.OriginalPrice
					}
					if allOffers[i].Price > 0 && allOffers[i].OriginalPrice > allOffers[i].Price {
						allOffers[i].Discount = ((allOffers[i].OriginalPrice - allOffers[i].Price) / allOffers[i].OriginalPrice) * 100
					}
					s.logger.Debug("Refetched product price",
						zap.Int64("itemID", offer.ItemID),
						zap.Float64("searchPrice", offer.Price),
						zap.Float64("refreshedPrice", refreshed.Price),
						zap.Float64("refreshedOriginal", refreshed.OriginalPrice),
						zap.Float64("finalPrice", allOffers[i].Price),
						zap.Float64("finalOriginal", allOffers[i].OriginalPrice))
					offer = allOffers[i]
				}
			}

			// Skip offers with no real discount (price >= original price)
			if !offer.HasRealDiscount() {
				s.logger.Info("Offer skipped: no discount (price >= original)",
					zap.String("automationID", automation.ID),
					zap.Float64("price", offer.Price),
					zap.Float64("originalPrice", offer.OriginalPrice),
					zap.Int64("itemID", offer.ItemID),
					zap.Int("offerIndex", i))
				for _, groupID := range targetGroups {
					s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "no_discount", withCanonicalKey(canonicalKey, store))
				}
				skippedCount += len(targetGroups)
				continue
			}

			// Generate proper affiliate link via generateShortLink mutation
			if s.shopeeService != nil && offer.ProductURL != "" {
				affLink, affErr := s.shopeeService.GenerateAffiliateLink(ctx, offer.ProductURL, automation.UserID)
				if affErr != nil {
					s.logger.Warn("Failed to generate affiliate link, using API offerLink",
						zap.String("automationID", automation.ID),
						zap.String("productURL", offer.ProductURL),
						zap.Error(affErr))
				} else if affLink != nil && affLink.AffiliateLink != "" {
					allOffers[i].AffiliateLink = affLink.AffiliateLink
					offer = allOffers[i]
					s.logger.Info("Affiliate link generated",
						zap.String("automationID", automation.ID),
						zap.String("affiliateLink", affLink.AffiliateLink))
				}
			}

			for _, groupID := range targetGroups {
				// Per-group dedup check (an offer may be new for some groups but not others)
				if s.isDuplicate(ctx, automation.UserID, groupID, canonicalKey, dedupeHours) {
					s.logger.Info("Offer skipped: dedup",
						zap.String("automationID", automation.ID),
						zap.String("canonicalKey", canonicalKey),
						zap.String("groupID", groupID))
					s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "dedupe", withCanonicalKey(canonicalKey, store))
					skippedCount++
					continue
				}

				// Build message from template
				message := s.buildOfferMessage(dispatchTemplate, &offer, automation)

				s.logger.Debug("Message built for offer",
					zap.String("automationID", automation.ID),
					zap.Int("offerIndex", i),
					zap.String("productName", offer.Name),
					zap.Int("messageLen", len(message)))

				// Apply URL safety check
				if s.urlSafetyService != nil {
					decision, _ := s.urlSafetyService.Evaluate(ctx, message)
					if decision != nil && decision.Action == "block" {
						s.logger.Info("Offer skipped: URL safety",
							zap.String("automationID", automation.ID),
							zap.String("groupID", groupID),
							zap.String("reason", decision.Reason))
						s.logDispatch(ctx, automation, groupID, offer.ProductURL, "blocked", decision.Reason, withCanonicalKey(canonicalKey, store))
						skippedCount++
						continue
					}
				}

				// Check backpressure
				if s.backpressure != nil {
					can, _, bpErr := s.backpressure.CanEnqueue(ctx, automation.UserID)
					if bpErr != nil {
						observability.LogAndCapture(s.logger, sentry.LevelWarning, "backpressure check failed, allowing enqueue", bpErr, map[string]string{
							"module": "automation", "operation": "backpressure_check", "user_id": automation.UserID,
						})
					}
					if !can {
						s.logger.Info("Offer skipped: backpressure",
							zap.String("automationID", automation.ID),
							zap.String("groupID", groupID))
						s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "backpressure", withCanonicalKey(canonicalKey, store))
						skippedCount++
						continue
					}
				}

				var imageURL *string
				if offer.Image != "" {
					imageURL = &offer.Image
				}

				jobData := &workers.MessageJobData{
					InstanceName: connectedInstance.InstanceID,
					GroupID:      groupID,
					Message:      message,
					ImageURL:     imageURL,
					HistoryID:    &historyID,
					AutomationID: automation.ID,
				}

				if s.queueClient != nil {
					delay := time.Duration(sentCount) * time.Duration(automation.GetDelaySeconds()) * time.Second
					_, err := s.queueClient.EnqueueMessage(ctx, jobData, delay)
					if err != nil {
						s.logger.Error("Failed to enqueue message",
							zap.String("automationID", automation.ID),
							zap.String("groupID", groupID),
							zap.Error(err))
						s.logDispatch(ctx, automation, groupID, offer.ProductURL, "failed", err.Error(), withCanonicalKey(canonicalKey, store))
						continue
					}
				}

				// Mark as sent for deduplication
				s.markAsSent(ctx, automation.UserID, groupID, canonicalKey, dedupeHours)
				s.logDispatch(ctx, automation, groupID, offer.ProductURL, "sent", "", withCanonicalKey(canonicalKey, store))
				sentCount++

				// Record backpressure
				if s.backpressure != nil {
					if bpErr := s.backpressure.RecordEnqueue(ctx, automation.UserID); bpErr != nil {
						observability.LogAndCapture(s.logger, sentry.LevelWarning, "backpressure record failed", bpErr, map[string]string{
							"module": "automation", "operation": "backpressure_record", "user_id": automation.UserID,
						})
					}
				}
			}

			// 1 offer per run: stop after processing the first non-duplicate offer
			break
		}

		// If we sent at least one message, no need to try more pages
		if sentCount > 0 {
			break
		}

		// All offers on this page were deduped — try next page for fresh products
		s.logger.Info("All offers deduped on current page, trying next page",
			zap.String("automationID", automation.ID),
			zap.Int("page", pageAttempt+1),
			zap.Int("offersChecked", len(allOffers)),
			zap.Int("skippedSoFar", skippedCount))
	} // end pageAttempt loop

	s.logger.Info("Search automation completed",
		zap.String("automationID", automation.ID),
		zap.Int("offersFound", totalOffersFound),
		zap.Int("messagesSent", sentCount),
		zap.Int("skipped", skippedCount),
		zap.Int("targetGroups", len(targetGroups)))

	return &models.RunAutomationResult{
		Success:      true,
		Message:      fmt.Sprintf("Automacao executada: %d mensagens enviadas, %d ignoradas", sentCount, skippedCount),
		MessagesSent: sentCount,
		OffersFound:  totalOffersFound,
	}, nil
}

func (s *automationService) buildOfferMessage(template string, offer *models.ShopeeOffer, automation *models.Automation) string {
	// Select random CTA if available
	cta := ""
	if len(automation.CTAs) > 0 {
		cta = automation.CTAs[rand.IntN(len(automation.CTAs))]
	}

	// Build coupon info
	couponCode := ""
	couponBlock := ""
	if automation.CouponID != nil && s.settingsRepo != nil {
		// Coupon loading is handled elsewhere; placeholder for when coupon is available
	}

	store := "Shopee"
	if len(automation.Stores) > 0 {
		store = automation.Stores[0]
	}

	// Replace legacy {{PRODUTO_*}} placeholders BEFORE RenderShopeeOfferTemplate,
	// because RenderTemplate's cleanup regex strips all unreplaced {{...}} patterns.
	legacyVars := map[string]string{
		"{{PRODUTO_NOME}}":           offer.Name,
		"{{PRODUTO_PRECO}}":          text.FormatBRLSpaced(offer.Price),
		"{{PRODUTO_PRECO_ORIGINAL}}": text.FormatBRLSpaced(offer.OriginalPrice),
		"{{PRODUTO_DESCONTO}}":       fmt.Sprintf("%.0f%%", offer.Discount),
		"{{LINK_AFILIADO}}":          offer.AffiliateLink,
		"{{LINK_PRODUTO}}":           offer.ProductURL,
		"{{LOJA_NOME}}":              offer.ShopName,
		"{{PRODUTO_IMAGEM}}":         offer.Image,
	}
	for key, value := range legacyVars {
		template = strings.ReplaceAll(template, key, value)
	}

	vars := text.ShopeeOfferVars{
		Title:         offer.Name,
		Price:         offer.Price,
		OriginalPrice: offer.OriginalPrice,
		Discount:      offer.Discount,
		Link:          offer.ProductURL,
		AffiliateLink: offer.AffiliateLink,
		Image:         offer.Image,
		Category:      "",
		Store:         store,
		CTA:           cta,
		CouponCode:    couponCode,
		CouponBlock:   couponBlock,
	}

	message := text.RenderShopeeOfferTemplate(template, vars)

	return text.NormalizeOutgoingWhatsAppText(message)
}

func (s *automationService) isDuplicate(ctx context.Context, userID, groupID, canonicalKey string, hours int) bool {
	// Fast path: check Redis cache first (written by markAsSent)
	if s.cache != nil {
		cacheKey := fmt.Sprintf("dedup:%s:%s:%s", userID, groupID, canonicalKey)
		exists, err := s.cache.Exists(ctx, cacheKey)
		if err == nil && exists {
			return true
		}
	}

	// Slow path: check database
	if s.dispatchLogRepo == nil {
		return false
	}

	log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, hours)
	if err != nil {
		s.logger.Warn("dedup DB check failed",
			zap.String("userID", userID),
			zap.String("groupID", groupID),
			zap.String("canonicalKey", canonicalKey),
			zap.Error(err))
		return false
	}

	// Backfill cache from DB hit to accelerate future checks
	if log != nil && s.cache != nil {
		cacheKey := fmt.Sprintf("dedup:%s:%s:%s", userID, groupID, canonicalKey)
		if cacheErr := s.cache.Set(ctx, cacheKey, "1", time.Duration(hours)*time.Hour); cacheErr != nil {
			s.logger.Debug("dedup cache backfill failed", zap.Error(cacheErr))
		}
	}

	return log != nil
}

func (s *automationService) markAsSent(ctx context.Context, userID, groupID, canonicalKey string, hours int) {
	if s.cache == nil {
		return
	}

	cacheKey := fmt.Sprintf("dedup:%s:%s:%s", userID, groupID, canonicalKey)
	if cacheErr := s.cache.Set(ctx, cacheKey, "1", time.Duration(hours)*time.Hour); cacheErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "dedup cache set failed", cacheErr, map[string]string{
			"module": "automation", "operation": "mark_as_sent_cache", "cache_policy": "fail-open",
		})
	}
}

// logDispatchOpt applies optional fields to a DispatchLog before creation.
type logDispatchOpt func(*models.DispatchLog)

// withCanonicalKey sets the canonical_key and store on the dispatch log for deduplication.
func withCanonicalKey(canonicalKey, store string) logDispatchOpt {
	return func(log *models.DispatchLog) {
		log.CanonicalKey = &canonicalKey
		log.Store = &store
	}
}

func (s *automationService) logDispatch(ctx context.Context, automation *models.Automation, groupID, productURL, status, errorMsg string, opts ...logDispatchOpt) {
	if s.dispatchLogRepo == nil {
		return
	}

	// Check feature flag: skip if Go should not create dispatch logs
	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldCreateDispatchLog() {
		s.logger.Debug("Skipping dispatch log creation (feature flag disabled)",
			zap.String("automationID", automation.ID),
			zap.String("groupID", groupID),
			zap.String("status", status))
		return
	}

	log := &models.DispatchLog{
		UserID:         automation.UserID,
		AutomationID:   &automation.ID,
		AutomationName: &automation.Name,
		GroupID:        &groupID,
		ProductURL:     &productURL,
		Status:         status,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	for _, opt := range opts {
		opt(log)
	}

	if errorMsg != "" {
		log.Error = &errorMsg
	}

	// W35: Capture dispatch log creation errors with Sentry tags
	if _, err := s.dispatchLogRepo.Create(ctx, log); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to create dispatch log", err, map[string]string{
			"module":       "automation",
			"operation":    "dispatch_log_create",
			"automationID": automation.ID,
			"groupID":      groupID,
			"status":       status,
		})
	}
}

func (s *automationService) ResetHistory(ctx context.Context, automationID, userID string) error {
	// Verify ownership
	automation, err := s.automationRepo.GetAutomationByIDAndUser(ctx, automationID, userID)
	if err != nil || automation == nil {
		return fmt.Errorf("automation not found")
	}

	// Delete dispatch logs for this automation within the last 24 hours
	// (matches Node.js parity: resetAutomationHistory deletes last 24h only)
	const resetDays = 1
	deleted, err := s.dispatchLogRepo.DeleteRecentByAutomation(ctx, automationID, resetDays)
	if err != nil {
		s.logger.Error("Failed to reset history",
			zap.String("automationID", automationID),
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("failed to reset history: %w", err)
	}

	s.logger.Info("Reset history completed",
		zap.String("automationID", automationID),
		zap.String("userID", userID),
		zap.Int64("logsDeleted", deleted))

	return nil
}

// CanCreateAutomation checks if the user can create a new automation
// Returns true if count < max, false otherwise
func (s *automationService) CanCreateAutomation(ctx context.Context, userID string) (bool, int, int, error) {
	// Default max automations
	const defaultMaxAutomations = 5

	// Get max automations for user
	maxAutomations, err := s.profileRepo.GetMaxAutomations(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get max automations",
			zap.String("userID", userID),
			zap.Error(err))
		// Use default on error
		maxAutomations = defaultMaxAutomations
	}

	// Count current active automations
	currentCount, err := s.profileRepo.CountUserAutomations(ctx, userID)
	if err != nil {
		return false, 0, maxAutomations, fmt.Errorf("failed to count automations: %w", err)
	}

	canCreate := currentCount < maxAutomations

	return canCreate, currentCount, maxAutomations, nil
}

// acquireExecutionLock attempts to acquire a lock for automation execution.
// Returns the lock expiry time, or ConflictError if lock is held by another execution.
func (s *automationService) acquireExecutionLock(ctx context.Context, automationID string) (*time.Time, error) {
	if s.automationRunRepo == nil {
		// If no repository configured, skip locking (for backward compatibility)
		return nil, nil
	}

	// Check if lock already exists
	existingLock, err := s.automationRunRepo.GetLock(ctx, automationID)
	if err != nil {
		return nil, fmt.Errorf("failed to check lock: %w", err)
	}

	if existingLock != nil {
		// Lock already held - return conflict error
		return nil, sharedErrors.NewLockConflictError("automation", *existingLock)
	}

	// Try to acquire lock
	lockUntil, err := s.automationRunRepo.AcquireLock(ctx, automationID, executionLockDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if lockUntil == nil {
		// Race condition - someone else got the lock
		existingLock, _ := s.automationRunRepo.GetLock(ctx, automationID)
		if existingLock != nil {
			return nil, sharedErrors.NewLockConflictError("automation", *existingLock)
		}
		return nil, sharedErrors.NewConflictError("automation", "lock acquisition failed")
	}

	return lockUntil, nil
}

// releaseExecutionLock releases the execution lock for an automation.
// Errors are logged but not returned - lock will expire naturally if release fails.
func (s *automationService) releaseExecutionLock(ctx context.Context, automationID string) error {
	if s.automationRunRepo == nil {
		return nil
	}

	if err := s.automationRunRepo.ReleaseLock(ctx, automationID); err != nil {
		s.logger.Error("Failed to release execution lock",
			zap.String("automationID", automationID),
			zap.Error(err))
		// Don't return error - lock will expire naturally
	}
	return nil
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
