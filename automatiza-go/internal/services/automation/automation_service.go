package automation

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/config"
	bpLib "automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/ctapicker"
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

// abortRetryBackoff is how soon a run is rescheduled when it was aborted by the
// per-automation/tick timeout (likely sending nothing). Short enough to recover
// delivery within minutes instead of skipping a whole interval, but long enough
// that a persistently-timing-out automation cannot become a tight retry loop.
const abortRetryBackoff = 5 * time.Minute

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
	// ReconcileListDeliveries devolve itens de lista consumidos no
	// enfileiramento cuja entrega nunca se confirmou (cron ~10min).
	ReconcileListDeliveries(ctx context.Context) error
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
	mlLinkListRepo     repository.MLLinkListRepository
	ctaRepo            repository.CTARepository

	shopeeService    shopee.Service
	mlService        mercadolivre.Service
	urlSafetyService urlSafetyEvaluator
	backpressure     backpressureProvider

	couponHandler *CouponHandler
	queueClient   *workers.QueueClient
	cache         *cache.Cache
	logger        *zap.Logger
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
	mlLinkListRepo repository.MLLinkListRepository,
	ctaRepo repository.CTARepository,
	shopeeService shopee.Service,
	mlService mercadolivre.Service,
	urlSafetyService *urlsafety.Service,
	bp *bpLib.ProducerBackpressure,
	queueClient *workers.QueueClient,
	cache *cache.Cache,
	couponRepo repository.CouponRepository,
	couponRuleRepo repository.CouponRuleRepository,
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
		mlLinkListRepo:     mlLinkListRepo,
		ctaRepo:            ctaRepo,
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
	if couponRepo != nil {
		svc.couponHandler = NewCouponHandler(couponRepo, couponRuleRepo, logger)
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
			mlLinkListCheck.Message = "Lista de links nao configurada. Selecione uma Lista de Links."
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

	// Release lock when done. Detach from the run context: if the run is killed
	// by the per-automation (90s) or tick (5min) timeout, the raw ctx is already
	// canceled and the release UPDATE never executes — leaving a stale 'running'
	// row that blocks this automation until lock_until expires (~10min) and leaks
	// a zombie row (never reaped by CleanupOldRuns). Same independent-context
	// treatment already used for last_run_at/next_run_at below.
	defer func() {
		relCtx, relCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer relCancel()
		_ = s.releaseExecutionLock(relCtx, automation.ID)
	}()

	// Update last_run_at immediately after lock to prevent double-trigger.
	// Uses independent context so it doesn't fail on cron batch timeout.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()
	if updateErr := s.automationRepo.UpdateLastRunAt(dbCtx, automation.ID); updateErr != nil {
		s.logger.Warn("Failed to update last_run_at",
			zap.String("automationID", automation.ID),
			zap.Error(updateErr))
	}

	// Update next_run_at when execution completes. If the run was aborted by the
	// per-automation/tick timeout (ctx.Err() != nil), it most likely enqueued
	// nothing, so scheduling a full interval ahead would silently drop this
	// cycle's delivery. Reschedule sooner (abortRetryBackoff, capped at the
	// interval) to recover, without turning a persistent timeout into a 1-min
	// hammer loop against Shopee/WAHA.
	//
	// SEMPRE grava (intervalo efetivo = interval_minutes ou 60min, o mesmo
	// default do scheduler): next_run_at agora é o gate autoritativo do
	// isDueForExecution, então todo run precisa reagendar o próximo — um
	// next_run_at obsoleto no passado deixaria a automação due a cada tick.
	defer func() {
		interval := 60 * time.Minute
		if automation.IntervalMinutes != nil && *automation.IntervalMinutes > 0 {
			interval = time.Duration(*automation.IntervalMinutes) * time.Minute
		}
		delay := interval
		if ctx.Err() != nil && abortRetryBackoff < interval {
			delay = abortRetryBackoff
		}
		nextRun := time.Now().UTC().Add(delay)
		nrCtx, nrCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer nrCancel()
		if updateErr := s.automationRepo.UpdateNextRunAt(nrCtx, automation.ID, nextRun); updateErr != nil {
			s.logger.Warn("Failed to update next_run_at",
				zap.String("automationID", automation.ID),
				zap.Error(updateErr))
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
	case "offer_generate":
		s.logger.Info("Dispatching offer_generate mode", zap.String("automationID", automation.ID))
		// OFFER_GENERATE mode: validate configuration and return status
		// Actual message processing happens via webhooks in offer_generate service
		result, err = s.processOfferGenerateMode(ctx, automation)
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

// resolveSenderForGroup returns the WAHA session name to use when dispatching
// to waGroupID. Priority:
//
//  1. automation.ChipOverrides[waGroupID] — pin explícito do usuário (mesma
//     semântica do PickChipForGroup do offer_generate: só vale se o chip
//     pinado está habilitado na automação, quando ela restringe chips, e
//     conectado; pin obsoleto cai adiante em silêncio). Sem este passo o
//     override só valia no offer_generate: grupo announce cujo chip dono
//     (groups.instance_id) foi removido do grupo ficava em loop de 401,
//     mesmo com o pin apontando para o chip admin correto.
//  2. Multi-instance dispatch: chip dono do grupo, quando conectado.
//  3. fallback (tipicamente o primeiro chip conectado do usuário).
//
// When groupInstances is nil or the flag is off, steps 2 collapses to
// fallback — i.e. legacy behaviour where every message goes through the
// same chip.
func (s *automationService) resolveSenderForGroup(automation *models.Automation, instances []models.Instance, fallback *models.Instance, groupInstances map[string]*models.Instance, waGroupID string) string {
	if automation != nil {
		if pinned := automation.ChipOverrides[waGroupID]; pinned != "" && chipEnabledOnAutomation(automation, pinned) {
			for i := range instances {
				if strconv.Itoa(instances[i].ID) == pinned && instances[i].IsConnected() {
					return instances[i].InstanceID
				}
			}
		}
	}
	if config.GetFeatureFlags().UseMultiInstanceDispatch && groupInstances != nil {
		if inst, ok := groupInstances[waGroupID]; ok && inst != nil && inst.IsConnected() {
			return inst.InstanceID
		}
	}
	if fallback != nil {
		return fallback.InstanceID
	}
	return ""
}

// chipEnabledOnAutomation reports whether chipID (instances.id as string) is
// allowed to dispatch for this automation. An automation without an explicit
// InstanceIDs restriction accepts any chip.
func chipEnabledOnAutomation(automation *models.Automation, chipID string) bool {
	if len(automation.InstanceIDs) == 0 {
		return true
	}
	for _, id := range automation.InstanceIDs {
		if id == chipID {
			return true
		}
	}
	return false
}

// fetchGroupInstancesIfEnabled returns the per-group instance map when the
// multi-instance dispatch flag is on; otherwise returns nil so callers
// short-circuit to the fallback path with zero overhead. Errors are logged
// and swallowed: the caller falls back to the singleton instance, which is
// always at least as available as the per-group lookup.
func (s *automationService) fetchGroupInstancesIfEnabled(ctx context.Context, userID string, waGroupIDs []string) map[string]*models.Instance {
	if !config.GetFeatureFlags().UseMultiInstanceDispatch {
		return nil
	}
	groupInstances, err := s.groupRepo.GetInstancesForGroups(ctx, userID, waGroupIDs)
	if err != nil {
		s.logger.Warn("Failed to load per-group instances; falling back to singleton routing",
			zap.String("userID", userID),
			zap.Error(err))
		return nil
	}
	return groupInstances
}

// defaultSearchDroughtPages é o teto de páginas que a busca percorre quando as
// páginas normais (maxSearchPages) deduplicam tudo. O probe em prod (2026-07-02)
// mostrou que categorias têm estoque linear-fundo (>=500 distintos em 20 páginas),
// então continuar paginando em drought quase sempre acha produto fresco. Só é
// exercitado quando a rodada ia enviar 0 — dia normal para em maxSearchPages.
const defaultSearchDroughtPages = 15

// searchDroughtPagesFromEnv lê SEARCH_MAX_DROUGHT_PAGES (default 15). Valores
// inválidos ou menores que maxSearchPages caem no default — a escalação nunca
// pode encurtar a janela normal de páginas.
func searchDroughtPagesFromEnv(maxSearchPages int) int {
	v := strings.TrimSpace(os.Getenv("SEARCH_MAX_DROUGHT_PAGES"))
	if v == "" {
		return defaultSearchDroughtPages
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < maxSearchPages {
		return defaultSearchDroughtPages
	}
	return n
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

	// Honor an explicit per-automation chip preference: if the user chose
	// a specific chip in the form, every dispatch goes through it (when
	// connected). Per-group routing is bypassed in that case — the user's
	// choice is the whole point.
	connectedInstance := pickPreferredOrFirstConnected(automation.InstanceID, instances)
	if connectedInstance == nil {
		return nil, fmt.Errorf("no connected WhatsApp instance")
	}

	// Resolve per-group instances when multi-instance dispatch is enabled
	// AND the automation has no chip preference. With a preference set,
	// we want every group to go through that chip, not the per-group
	// owner — so we skip the map (nil short-circuits inside the helper).
	var groupInstances map[string]*models.Instance
	if automation.InstanceID == nil || *automation.InstanceID == "" {
		groupInstances = s.fetchGroupInstancesIfEnabled(ctx, automation.UserID, targetGroups)
	}

	// Determine stores to search
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}

	// Pre-fetch CTA entities. Política:
	//   - automation.CTAIDs preenchido → override explícito, busca só esses
	//   - automation.CTAIDs vazio → pool inteiro de CTAs ativos do user
	//
	// Pool inteiro é o caminho default (paridade com affilipro): o user
	// cadastra CTAs em /ctas e o sistema escolhe automaticamente o que casa
	// com a plataforma do produto (search mode é Shopee-only).
	var ctaEntities []models.CTA
	if s.ctaRepo != nil {
		if len(automation.CTAIDs) > 0 {
			fetched, ctaErr := s.ctaRepo.GetCTAsByIDs(ctx, automation.CTAIDs, automation.UserID)
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch CTA entities by IDs, falling back to inline CTAs",
					zap.String("automationID", automation.ID),
					zap.Error(ctaErr))
			} else {
				ctaEntities = fetched
			}
		} else {
			fetched, ctaErr := s.ctaRepo.GetUserCTAs(ctx, automation.UserID, map[string]interface{}{"is_active": true})
			if ctaErr != nil {
				s.logger.Warn("Failed to fetch user CTA pool, falling back to inline CTAs",
					zap.String("automationID", automation.ID),
					zap.Error(ctaErr))
			} else {
				ctaEntities = fetched
			}
		}
	}

	s.logger.Info("Search automation setup complete",
		zap.String("automationID", automation.ID),
		zap.Int("targetGroups", len(targetGroups)),
		zap.String("instanceID", connectedInstance.InstanceID),
		zap.Strings("stores", stores),
		zap.Int("ctaEntities", len(ctaEntities)))

	// Build keywords list: prefer Texts array, fallback to single Keyword
	keywords := automation.Texts
	if len(keywords) == 0 && automation.Keyword != nil && *automation.Keyword != "" {
		keywords = []string{*automation.Keyword}
	}
	if len(keywords) == 0 {
		keywords = []string{""} // empty keyword = broad search
	}

	// Load dispatch text copies (resolve once, select randomly per message)
	var dispatchCopies []string
	if automation.DispatchTextID != nil {
		dt, _ := s.settingsRepo.GetActiveDispatchTextByID(ctx, *automation.DispatchTextID, automation.UserID)
		if dt != nil && len(dt.Copies) > 0 {
			dispatchCopies = dt.Copies
		} else if dt != nil && dt.Body != "" {
			dispatchCopies = []string{dt.Body}
		}
	}
	if len(dispatchCopies) == 0 {
		if msg := automation.GetCustomMessage(); msg != "" {
			dispatchCopies = []string{msg}
		}
	}
	if len(dispatchCopies) == 0 {
		dispatchCopies = []string{"🛍️ {{PRODUTO_NOME}}\n\nDe {{PRODUTO_PRECO_ORIGINAL}}\n💸 Por {{PRODUTO_PRECO}}\n\n🛒Compre aqui: {{LINK_AFILIADO}}\n\n⚠ Promoção sujeita a alteração a qualquer momento."} //nolint:misspell // "momento" is Portuguese, not a misspelling of "memento"
	}

	// Detect templates without variables on the first copy — log once
	templateVars := text.ExtractTemplateVariables(dispatchCopies[0])
	if len(templateVars) == 0 {
		s.logger.Warn("Dispatch template has no variables — all messages will be identical",
			zap.String("automationID", automation.ID),
			zap.Int("templateLen", len(dispatchCopies[0])))
	}

	s.logger.Info("Dispatch template resolved",
		zap.String("automationID", automation.ID),
		zap.Int("copyVariations", len(dispatchCopies)),
		zap.Int("variableCount", len(templateVars)))

	// Send offers to groups
	sentCount := 0
	skippedCount := 0
	dedupeHours := automation.GetDedupeHours()

	// Pagination: when all offers from page N are deduped, try page N+1.
	// This prevents niche categories with limited product variety from getting stuck.
	//
	// Normal runs cover the rotated window of maxSearchPages (~150 offers). Quando
	// TODAS essas páginas deduplicam (drought), a busca continua paginando fundo
	// até maxDroughtPages antes de desistir — o loop já para no primeiro envio, então
	// as páginas fundas só são buscadas quando a rodada ia enviar 0. Ver o probe
	// documentado: profundidade de página é o lever real contra "não envia nada".
	const maxSearchPages = 3
	const limitPerKeyword = 50
	maxDroughtPages := searchDroughtPagesFromEnv(maxSearchPages)
	var totalOffersFound int

	// Page rotation: start from a different page each run for product variety.
	// Each execution cycle (1→2→3→1) brings completely different products from the ranking.
	// With limit=50 and 3 pages, the effective pool triples (~135 unique products vs ~45).
	startPage := 0
	pageRotationKey := fmt.Sprintf("page_rotation:%s", automation.ID)
	if s.cache != nil {
		if val, err := s.cache.Get(ctx, pageRotationKey); err == nil && val != "" {
			if parsed, parseErr := strconv.Atoi(val); parseErr == nil {
				startPage = parsed % maxSearchPages
			}
		}
		nextPage := (startPage + 1) % maxSearchPages
		_ = s.cache.Set(ctx, pageRotationKey, strconv.Itoa(nextPage), 24*time.Hour)
	}

	s.logger.Info("Page rotation",
		zap.String("automationID", automation.ID),
		zap.Int("startPage", startPage))

	// Category rotation: for automations with many categories, search a subset
	// per run and rotate. This gives more products per category (higher variety)
	// while still covering all categories across runs.
	const maxCategoriesPerRun = 8
	searchCategories := automation.Categories

	if len(automation.Categories) > maxCategoriesPerRun {
		catRotationKey := fmt.Sprintf("cat_rotation:%s", automation.ID)
		catOffset := 0
		if s.cache != nil {
			if val, err := s.cache.Get(ctx, catRotationKey); err == nil && val != "" {
				if parsed, parseErr := strconv.Atoi(val); parseErr == nil {
					catOffset = parsed
				}
			}
		}

		// Wrap offset if categories changed since last run
		totalCats := len(automation.Categories)
		catOffset = catOffset % totalCats

		// Select subset starting from offset
		end := catOffset + maxCategoriesPerRun
		if end > totalCats {
			// Wrap around: take end of list + beginning
			searchCategories = append(
				automation.Categories[catOffset:],
				automation.Categories[:end-totalCats]...,
			)
		} else {
			searchCategories = automation.Categories[catOffset:end]
		}

		// Advance offset for next run
		nextOffset := (catOffset + maxCategoriesPerRun) % totalCats
		if s.cache != nil {
			_ = s.cache.Set(ctx, catRotationKey, strconv.Itoa(nextOffset), 24*time.Hour)
		}

		s.logger.Info("Category rotation",
			zap.String("automationID", automation.ID),
			zap.Int("totalCategories", totalCats),
			zap.Int("offset", catOffset),
			zap.Int("subsetSize", len(searchCategories)))
	}

	for pageAttempt := 0; pageAttempt < maxDroughtPages; pageAttempt++ {
		// Normal window: rotate within the first maxSearchPages (pages 1..3 in
		// rotated order). Drought escalation: once past the window, go strictly
		// deeper (pages 4, 5, …) — never re-covering the rotated pages.
		var currentPage int
		if pageAttempt < maxSearchPages {
			currentPage = (startPage + pageAttempt) % maxSearchPages
		} else {
			currentPage = pageAttempt
			s.logger.Info("Dedup drought: escalating to deeper search page",
				zap.String("automationID", automation.ID),
				zap.Int("page", currentPage+1),
				zap.Int("maxDroughtPages", maxDroughtPages),
				zap.Int("skippedSoFar", skippedCount))
		}
		searchOffset := currentPage * limitPerKeyword

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
							Categories:  searchCategories,
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
								zap.Int("page", currentPage+1),
								zap.Error(err))
						} else if result != nil {
							s.logger.Info("Shopee search completed",
								zap.String("automationID", automation.ID),
								zap.String("keyword", kwCopy),
								zap.Int("page", currentPage+1),
								zap.Int("offersReturned", len(result.Offers)))
							allOffers = append(allOffers, result.Offers...)
						}
					}
				}
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
					zap.Int("page", currentPage+1))
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
					// Loja oficial (shopType→IsMall): OR das duas fontes. Search e
					// GetProduct derivam IsMall do MESMO campo shopType, fail-closed
					// (só true com shopType=1). Um GetProduct sem shopType parseável
					// (fail-closed→false) não pode REBAIXAR um Mall já confirmado
					// pela busca — senão a regra official_only deixaria de casar.
					allOffers[i].IsMall = allOffers[i].IsMall || refreshed.IsMall
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

			// Generate proper affiliate link via generateShortLink mutation.
			// O AffiliateLink que veio da BUSCA é o offerLink do productOfferV2,
			// que NÃO carrega o sub_id (AUTOMATISA) — enviá-lo cru registra a
			// venda como "sem SubID" no painel do afiliado. Conversão falhou:
			// erro transitório (10030/timeout) encerra a rodada inteira (tentar
			// a próxima oferta só aprofunda o rate limit; a automação roda de
			// novo no próximo tick); erro permanente pula só esta oferta.
			if s.shopeeService != nil && offer.ProductURL != "" {
				affLink, affErr := s.shopeeService.GenerateAffiliateLink(ctx, offer.ProductURL, automation.UserID)
				if affErr != nil {
					reason := fmt.Sprintf("link de afiliado indisponivel (%v) - oferta pulada para nao perder o subid", affErr)
					transient := isTransientShopeeEnrichError(affErr)
					if transient {
						reason = "link de afiliado indisponivel (transitorio) - envio adiado para nao perder o subid"
					}
					s.logger.Warn("Failed to generate affiliate link, withholding offer (sem sub_id nao envia)",
						zap.String("automationID", automation.ID),
						zap.String("productURL", offer.ProductURL),
						zap.Bool("transient", transient),
						zap.Error(affErr))
					for _, groupID := range targetGroups {
						s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", reason, withCanonicalKey(canonicalKey, store))
					}
					skippedCount += len(targetGroups)
					if transient {
						break
					}
					continue
				}
				if affLink != nil && affLink.AffiliateLink != "" {
					allOffers[i].AffiliateLink = affLink.AffiliateLink
					offer = allOffers[i]
					s.logger.Info("Affiliate link generated",
						zap.String("automationID", automation.ID),
						zap.String("affiliateLink", affLink.AffiliateLink))
				}
			}

			// Resolve o cupom desta oferta via rules engine (por oferta: o cupom
			// depende do preço + flag de loja oficial, igual ao LIST). offer.IsMall
			// dirige as regras official_only. Cupom na BUSCA é novo e fica atrás do
			// gate de rollout por usuário (CouponRolloutEnabledFor) — usuários fora
			// do rollout mantêm a busca SEM cupom (comportamento histórico).
			// Fail-open: erro no cupom só significa "sem cupom", nunca bloqueia o envio.
			var couponResult *CouponResult
			if s.couponHandler != nil && offer.Price > 0 && s.couponHandler.CouponRolloutEnabledFor(automation.UserID) {
				if cr, err := s.couponHandler.ApplyCoupon(ctx, automation, offer.Price, store, offer.IsMall); err == nil {
					couponResult = cr
				}
			}

			// Anti-ban pacing for SEARCH: per-chip jittered delay accumulator +
			// micro-breaks (parity with LIST, honoring delay_seconds), destination
			// shuffle, and reputation-based suppression.
			perChipDelay := make(map[string]time.Duration)
			pacingCfg := config.GetHumanization().PacingConfigForMode(automation.HumanizeMode)
			rt := config.GetAntiBanRuntime()
			pacingCfg.Shuffle(targetGroups)
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

				// Select random copy variation per message
				copyIndex := 0
				if len(dispatchCopies) > 1 {
					copyIndex = rand.IntN(len(dispatchCopies)) //nolint:gosec // G404: non-security random for copy variation selection
				}
				selectedCopy := dispatchCopies[copyIndex]

				// Build message from selected copy
				message := s.buildOfferMessage(selectedCopy, &offer, automation, ctaEntities, couponResult)

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

				senderSession := s.resolveSenderForGroup(automation, instances, connectedInstance, groupInstances, groupID)
				if senderSession == "" {
					s.logger.Warn("No connected chip for group, skipping dispatch",
						zap.String("automationID", automation.ID),
						zap.String("groupID", groupID))
					s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "instance disconnected", withCanonicalKey(canonicalKey, store))
					continue
				}

				// Anti-ban: skip groups with a bad reputation (fail-open).
				if rt.Reputation.IsSuppressed(ctx, senderSession, groupID) {
					s.logDispatch(ctx, automation, groupID, offer.ProductURL, "skipped", "reputacao (grupo problematico)", withCanonicalKey(canonicalKey, store))
					continue
				}

				// "Enviar como imagem" (SEARCH, default TRUE): ligado envia a
				// imagem baixada + legenda (comportamento histórico da Busca);
				// desligado usa o preview nativo do WhatsApp (mais leve).
				// nil = true (default do schema).
				searchSendAsImage := automation.SearchSendAsImage == nil || *automation.SearchSendAsImage

				jobData := &workers.MessageJobData{
					InstanceName: senderSession,
					GroupID:      groupID,
					Message:      message,
					HistoryID:    &historyID,
					AutomationID: automation.ID,
					HumanizeMode: automation.HumanizeMode,
				}
				if searchSendAsImage {
					jobData.ImageURL = imageURL
				} else if offer.AffiliateLink != "" {
					jobData.LinkPreview = &workers.LinkPreviewData{
						URL:      offer.AffiliateLink,
						Title:    offer.Name,
						ImageURL: offer.Image,
					}
				}

				// Dispatch log criado ANTES do enqueue (status queued) com o ID
				// passado ao worker — mesmo contrato do LIST. Antes, o SEARCH
				// gravava 'sent' no ENQUEUE e nunca passava DispatchLogID: um
				// envio que falhasse definitivo no worker (chip caiu, bot fora
				// do grupo, descarte stale pós-backlog) ficava 'sent' para
				// sempre — perda silenciosa com log dizendo "enviado". Agora o
				// desfecho real (sent/failed/skipped) é escrito pelo worker, e
				// o TaskID dispatch:<id> ainda dedupa enfileiramento duplicado.
				dispatchLogID := s.createDispatchLog(ctx, automation, groupID, offer.ProductURL,
					models.DispatchLogStatusQueued, "",
					withCanonicalKey(canonicalKey, store), withCopyIndex(copyIndex), withAffiliateLink(offer.AffiliateLink),
					withCoupon(couponResult))
				jobData.DispatchLogID = dispatchLogID

				if s.queueClient != nil {
					// Per-chip jittered accumulator (anti-ban) instead of a global
					// sentCount×delay ramp — advanced after each successful enqueue.
					delay := perChipDelay[senderSession]
					_, err := s.queueClient.EnqueueMessage(ctx, jobData, delay)
					if err != nil {
						s.logger.Error("Failed to enqueue message",
							zap.String("automationID", automation.ID),
							zap.String("groupID", groupID),
							zap.Error(err))
						if dispatchLogID != nil && s.dispatchLogRepo != nil {
							if upErr := s.dispatchLogRepo.Update(ctx, *dispatchLogID, map[string]interface{}{
								"status": models.DispatchLogStatusFailed,
								"error":  err.Error(),
							}); upErr != nil {
								s.logger.Warn("Failed to mark dispatch log failed after enqueue error", zap.Error(upErr))
							}
						}
						continue
					}
				}

				// Mark as sent for deduplication (claim no enfileiramento, por
				// design: o próximo ciclo precisa ver a oferta como disparada).
				s.markAsSent(ctx, automation.UserID, groupID, canonicalKey, dedupeHours)
				sentCount++
				// Advance this chip's jittered delay accumulator (+ occasional micro-break).
				perChipDelay[senderSession] = pacingCfg.ClampSpread(
					perChipDelay[senderSession] +
						pacingCfg.Step(automation.GetDelaySeconds()) +
						rt.Pacing.MicroBreak(ctx, senderSession, pacingCfg))

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
			zap.Int("page", currentPage+1),
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

func (s *automationService) buildOfferMessage(template string, offer *models.ShopeeOffer, automation *models.Automation, ctaEntities []models.CTA, couponResult *CouponResult) string {
	// Search mode é exclusivamente Shopee (offer vem do ShopeeOffer). Filtra
	// ctaEntities por platform Shopee/all antes do sorteio — sem isso, uma
	// automation com CTA "MERCADO LIVRE 🔥" no pool mandava o texto de ML
	// num link Shopee. Fallback para CTAs inline da automation se nada bater.
	cta := ctapicker.PickByPlatform(ctaEntities, false, true)
	if cta == "" && len(automation.CTAs) > 0 {
		cta = automation.CTAs[rand.IntN(len(automation.CTAs))] //nolint:gosec // G404: non-security random for CTA selection
	}

	// Cupom resolvido pelo rules engine (mesmo padrão do LIST em
	// buildListModeMessage): injeta código/bloco e usa o preço com desconto no
	// template. couponResult nil (sem handler/oferta) → sem cupom.
	couponCode := ""
	couponBlock := ""
	effectivePrice := offer.Price
	if couponResult != nil && couponResult.Applied {
		couponCode = couponResult.CouponCode
		couponBlock = couponResult.CouponText
		if couponResult.DiscountedPrice > 0 {
			effectivePrice = couponResult.DiscountedPrice
		}
	}

	store := "Shopee"
	if len(automation.Stores) > 0 {
		store = automation.Stores[0]
	}

	// Replace legacy {{PRODUTO_*}} placeholders BEFORE RenderShopeeOfferTemplate,
	// because RenderTemplate's cleanup regex strips all unreplaced {{...}} patterns.
	legacyVars := map[string]string{
		"{{PRODUTO_NOME}}":           offer.Name,
		"{{PRODUTO_PRECO}}":          text.FormatBRLSpaced(effectivePrice),
		"{{PRODUTO_PRECO_ORIGINAL}}": text.FormatBRLSpaced(offer.OriginalPrice),
		"{{PRODUTO_DESCONTO}}":       fmt.Sprintf("%.0f%%", offer.Discount),
		"{{LINK_AFILIADO}}":          offer.AffiliateLink,
		"{{LINK_PRODUTO}}":           offer.ProductURL,
		"{{LOJA_NOME}}":              offer.ShopName,
		"{{PRODUTO_IMAGEM}}":         offer.Image,
	}
	// Remove a linha "De {{PRODUTO_PRECO_ORIGINAL}}" quando não há preço cheio —
	// senão a substituição abaixo a deixaria como "De " pendurado no disparo.
	// Tem que rodar ANTES do ReplaceAll (depois o placeholder já virou "").
	template = text.StripEmptyOriginalPriceLine(template, offer.OriginalPrice <= 0)
	for key, value := range legacyVars {
		template = strings.ReplaceAll(template, key, value)
	}

	vars := text.ShopeeOfferVars{
		Title:         offer.Name,
		Price:         effectivePrice,
		OriginalPrice: offer.OriginalPrice,
		Discount:      offer.Discount,
		Link:          offer.ProductURL,
		AffiliateLink: offer.AffiliateLink,
		Image:         offer.Image,
		Category:      "",
		Store:         store,
		CTA:           cta,
		CouponCode:    couponCode,
		CouponText:    couponBlock,
		CouponBlock:   couponBlock,
	}

	message := text.RenderShopeeOfferTemplate(template, vars)

	// Auto-append CTA if template doesn't contain {cta} variable but a CTA was selected
	if cta != "" && !strings.Contains(template, "{cta}") {
		message = message + "\n\n" + cta
	}

	// Auto-append do bloco de cupom SÓ quando o template não posiciona o cupom
	// por nenhum placeholder ({cupom_bloco} ou {couponText}) — senão o cupom
	// renderizaria duas vezes. Mesmo comportamento do LIST em buildListModeMessage.
	if couponResult != nil && couponResult.Applied &&
		!strings.Contains(template, "{cupom_bloco}") && !strings.Contains(template, "{couponText}") {
		message = message + couponResult.CouponText
	}

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

	log, err := s.dispatchLogRepo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, time.Duration(hours)*time.Hour)
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

func withCopyIndex(idx int) logDispatchOpt {
	return func(log *models.DispatchLog) {
		log.CopyIndex = &idx
	}
}

func withAffiliateLink(link string) logDispatchOpt {
	return func(log *models.DispatchLog) {
		if link != "" {
			log.AffiliateLink = &link
		}
	}
}

// withCoupon records which coupon rule fired (or why none did) on the dispatch
// log so support can audit coupon application. nil-safe: a nil result leaves
// the columns NULL. The column semantics live in DispatchLog.SetCouponAudit.
func withCoupon(cr *CouponResult) logDispatchOpt {
	return func(log *models.DispatchLog) {
		if cr == nil {
			return
		}
		log.SetCouponAudit(cr.Applied, cr.CouponCode, cr.SkipReason)
	}
}

func (s *automationService) logDispatch(ctx context.Context, automation *models.Automation, groupID, productURL, status, errorMsg string, opts ...logDispatchOpt) {
	// Skips de dedupe eram 96% dos inserts em dispatch_logs (~63/s) e ninguém
	// os consome (dedup lê só sent/queued/pending; suporte agrega sem skips) —
	// a informação é derivável da linha 'sent' anterior do mesmo produto.
	// HasPrefix cobre também "dedupe (produto repetido na lista)".
	// Kill-switch: LOG_DEDUPE_SKIPS=true restaura o comportamento antigo.
	if status == "skipped" && strings.HasPrefix(errorMsg, "dedupe") && !config.LogDedupeSkips() {
		return
	}
	s.createDispatchLog(ctx, automation, groupID, productURL, status, errorMsg, opts...)
}

// createDispatchLog cria o dispatch log e devolve o ID (nil quando desativado
// por flag ou em erro). O caminho de ENQUEUE usa o ID para passar ao worker
// (jobData.DispatchLogID), que grava o desfecho real — sem o ID, o log ficava
// congelado no status do enqueue e falhas terminais viravam perda silenciosa.
func (s *automationService) createDispatchLog(ctx context.Context, automation *models.Automation, groupID, productURL, status, errorMsg string, opts ...logDispatchOpt) *int {
	if s.dispatchLogRepo == nil {
		return nil
	}

	// Check feature flag: skip if Go should not create dispatch logs
	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldCreateDispatchLog() {
		s.logger.Debug("Skipping dispatch log creation (feature flag disabled)",
			zap.String("automationID", automation.ID),
			zap.String("groupID", groupID),
			zap.String("status", status))
		return nil
	}

	log := &models.DispatchLog{
		UserID:         automation.UserID,
		AutomationID:   &automation.ID,
		AutomationName: &automation.Name,
		GroupID:        &groupID,
		ProductURL:     &productURL,
		Status:         status,
		// Type vem do automation pra distinguir entries por modo
		// (list/search/monitoring) em queries de analytics. Sem isso o
		// campo fica NULL e o agrupamento na UI quebra pra search/list.
		// monitoring_service e offer_generate_service têm o próprio
		// log que já preenche Type — esse helper cobre os outros modos.
		Type:      automation.Type,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, opt := range opts {
		opt(log)
	}

	if errorMsg != "" {
		log.Error = &errorMsg
	}

	// W35: Capture dispatch log creation errors with Sentry tags
	created, err := s.dispatchLogRepo.Create(ctx, log)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to create dispatch log", err, map[string]string{
			"module":       "automation",
			"operation":    "dispatch_log_create",
			"automationID": automation.ID,
			"groupID":      groupID,
			"status":       status,
		})
		return nil
	}
	if created == nil {
		return nil
	}
	return &created.ID
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

// pickPreferredOrFirstConnected returns the instance the automation
// explicitly prefers (when set, parseable, present, and connected),
// otherwise the first connected instance in the slice.
//
// preferredID is the wire-format string of an instance database id —
// matches Automation.InstanceID. Empty / nil / unparseable / unconnected
// all degrade to the first-connected fallback so a stale preference
// doesn't strand the user.
func pickPreferredOrFirstConnected(preferredID *string, instances []models.Instance) *models.Instance {
	if preferredID != nil && *preferredID != "" {
		var n int
		if _, err := fmt.Sscanf(*preferredID, "%d", &n); err == nil && n > 0 {
			for i := range instances {
				if instances[i].ID == n && instances[i].Status == "connected" {
					return &instances[i]
				}
			}
		}
	}
	for i := range instances {
		if instances[i].Status == "connected" {
			return &instances[i]
		}
	}
	return nil
}
