package automation

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// perAutomationTimeout caps a single automation's execution inside a
// ProcessActiveAutomations tick. Normal runs finish in well under this; the
// cap exists so a run stuck on a degraded dependency can't consume the shared
// 5-minute cron context and starve the rest of the batch.
const perAutomationTimeout = 90 * time.Second

// defaultSchedulerConcurrency = 1 preserva o comportamento histórico (loop
// serial). SCHEDULER_CONCURRENCY > 1 roda as automações due em paralelo dentro
// do tick, com um pool limitado — uma lista deixa de esperar a outra terminar,
// e uma automação travada não congela as demais. O anti-ban se mantém: o
// session pacer é atômico por chip mesmo sob concorrência (ver session_pacer.go).
const defaultSchedulerConcurrency = 1

// schedulerConcurrencyFromEnv lê SCHEDULER_CONCURRENCY (default 1 = serial).
// Valores inválidos ou < 1 caem no default.
func schedulerConcurrencyFromEnv() int {
	v := strings.TrimSpace(os.Getenv("SCHEDULER_CONCURRENCY"))
	if v == "" {
		return defaultSchedulerConcurrency
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultSchedulerConcurrency
	}
	return n
}

// Scheduler implements the cron.AutomationScheduler interface.
// It processes active automations based on their schedule configuration.
type Scheduler struct {
	automationRepo repository.AutomationRepository
	service        Service
	logger         *zap.Logger
	mu             sync.Mutex // prevents concurrent ProcessActiveAutomations calls
}

// NewScheduler creates a new automation scheduler.
func NewScheduler(
	automationRepo repository.AutomationRepository,
	service Service,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		automationRepo: automationRepo,
		service:        service,
		logger:         logger,
	}
}

// ProcessActiveAutomations processes all active automations that are due for execution.
// It implements the cron.AutomationScheduler interface.
func (s *Scheduler) ProcessActiveAutomations(ctx context.Context) error {
	// Prevent concurrent executions — if another goroutine is already processing,
	// skip this tick to avoid double-triggering automations with stale in-memory data.
	if !s.mu.TryLock() {
		s.logger.Warn("ProcessActiveAutomations already running, skipping this tick")
		return nil
	}
	defer s.mu.Unlock()

	automations, err := s.automationRepo.GetActiveAutomations(ctx)
	if err != nil {
		s.logger.Error("Failed to get active automations", zap.Error(err))
		return fmt.Errorf("failed to get active automations: %w", err)
	}

	if len(automations) == 0 {
		s.logger.Debug("No active automations to process")
		return nil
	}

	s.logger.Info("Processing active automations", zap.Int("count", len(automations)))

	now := time.Now()
	skippedCount := 0
	var processedCount, errorCount int64

	// 1ª passada (serial, barata): filtra para as automações realmente due
	// neste tick. Os gates (janela, datas, intervalo) leem só campos locais.
	due := make([]*models.Automation, 0, len(automations))
	for i := range automations {
		automation := &automations[i]

		// Skip event-driven automations - they are triggered by incoming messages, not by the scheduler
		if automation.IsMonitoring() || automation.IsOfferGenerate() {
			skippedCount++
			continue
		}

		// Check if automation's start_date is reached (migration 053).
		// NULL/empty start_date = sem agendamento, roda imediatamente.
		if !s.isStartDateReached(automation, now) {
			skippedCount++
			continue
		}

		// Check if automation's end_date is not exceeded (migration 054).
		// NULL/empty end_date = sem expiração.
		if s.isEndDateExceeded(automation, now) {
			skippedCount++
			continue
		}

		// Check if automation is within time window
		if !s.isInTimeWindow(automation, now) {
			skippedCount++
			continue
		}

		// Check if it's time to run based on interval
		if !s.isDueForExecution(automation, now) {
			skippedCount++
			continue
		}

		due = append(due, automation)
	}

	// runOne executa uma automação due com seu próprio timeout. É seguro
	// chamar concorrentemente: cada automação é independente (IDs distintos,
	// repos/Redis/asynq são thread-safe) e o session pacer é atômico por chip,
	// então o anti-ban se mantém sob concorrência.
	runOne := func(automation *models.Automation) {
		s.logger.Info("Running scheduled automation",
			zap.String("id", automation.ID),
			zap.String("name", automation.Name),
			zap.String("type", automation.GetType()),
			zap.String("userID", automation.UserID))

		// Per-automation timeout: cap each run with its own deadline so one
		// slow automation (e.g. hanging on a degraded external API) can't eat
		// the whole shared tick budget. Derived from the tick ctx, so shutdown
		// still cancels promptly.
		autoCtx, cancel := context.WithTimeout(ctx, perAutomationTimeout)
		result, err := s.service.RunAutomation(autoCtx, automation.ID, automation.UserID)
		cancel()
		if err != nil {
			s.logger.Error("Failed to run automation",
				zap.String("id", automation.ID),
				zap.String("name", automation.Name),
				zap.Error(err))
			atomic.AddInt64(&errorCount, 1)
			return
		}

		s.logger.Info("Automation completed",
			zap.String("id", automation.ID),
			zap.String("name", automation.Name),
			zap.Int("messagesSent", result.MessagesSent),
			zap.Int("offersFound", result.OffersFound))

		atomic.AddInt64(&processedCount, 1)
	}

	// 2ª passada: executa as due. Serial quando concurrency<=1 (default,
	// comportamento histórico); senão, pool limitado em paralelo.
	concurrency := schedulerConcurrencyFromEnv()
	if concurrency <= 1 {
		for _, automation := range due {
			runOne(automation)
		}
	} else {
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		for _, automation := range due {
			wg.Add(1)
			sem <- struct{}{} // bloqueia quando o pool está cheio
			go func(a *models.Automation) {
				defer wg.Done()
				defer func() { <-sem }()
				runOne(a)
			}(automation)
		}
		wg.Wait()
	}

	s.logger.Info("Finished processing automations",
		zap.Int("processed", int(processedCount)),
		zap.Int("skipped", skippedCount),
		zap.Int("errors", int(errorCount)),
		zap.Int("due", len(due)),
		zap.Int("concurrency", concurrency))

	return nil
}

// isStartDateReached returns true se o scheduler pode considerar essa
// automation pra execução hoje. StartDate é uma data calendário no
// formato YYYY-MM-DD (vinda do scanner). NULL/empty = sem restrição.
//
// Comparação é por dia (server-local time). Antes da data, automation
// é pulada silenciosamente — não loga erro nem dispatcha skipped, só
// fica fora do batch desta tick (paridade com a janela horária).
func (s *Scheduler) isStartDateReached(automation *models.Automation, now time.Time) bool {
	if automation.StartDate == nil || *automation.StartDate == "" {
		return true
	}
	// Parse no MESMO fuso de `now` (now.Location()). Usar time.Parse aqui
	// retornaria meia-noite UTC, enquanto `today` é meia-noite local — em
	// fusos negativos (ex.: America/Sao_Paulo, UTC-3) a meia-noite local
	// cai depois da meia-noite UTC do mesmo dia, deslocando a comparação
	// em 1 dia. ParseInLocation alinha os dois lados.
	start, err := time.ParseInLocation("2006-01-02", *automation.StartDate, now.Location())
	if err != nil {
		s.logger.Warn("Failed to parse start_date, ignoring schedule gate",
			zap.String("automationID", automation.ID),
			zap.String("startDate", *automation.StartDate),
			zap.Error(err))
		return true
	}
	// Compara só a data (descarta hora). today >= start_date → libera.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return !today.Before(start)
}

// isEndDateExceeded retorna true se HOJE > end_date (automation já
// expirou). NULL/empty end_date = sem expiração, retorna false.
// Comparação por dia (server-local), mesmo padrão do start_date.
//
// Quando true, scheduler pula a automation silenciosamente — não muda
// status (cliente vê a automation como ativa, mas ela não dispara).
// Decisão consciente: alterar status='paused' aqui criaria loop com
// quem reativar manualmente sem atualizar end_date.
func (s *Scheduler) isEndDateExceeded(automation *models.Automation, now time.Time) bool {
	if automation.EndDate == nil || *automation.EndDate == "" {
		return false
	}
	// Parse no MESMO fuso de `now` — ver nota em isStartDateReached. Sem
	// isso, end_date = hoje era tratado como expirado o dia inteiro em
	// fuso negativo (today local 00:00 = 03:00 UTC > end 00:00 UTC).
	end, err := time.ParseInLocation("2006-01-02", *automation.EndDate, now.Location())
	if err != nil {
		s.logger.Warn("Failed to parse end_date, ignoring expiration gate",
			zap.String("automationID", automation.ID),
			zap.String("endDate", *automation.EndDate),
			zap.Error(err))
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.After(end)
}

// isInTimeWindow checks if the current time is within the automation's configured time window.
// If no time window is configured, the automation is always within the window.
func (s *Scheduler) isInTimeWindow(automation *models.Automation, now time.Time) bool {
	// If no time window is configured, always allow
	if automation.StartTime == nil && automation.EndTime == nil {
		return true
	}

	currentTime := now.Format("15:04")

	// Parse start time
	var startTime string
	if automation.StartTime != nil {
		startTime = *automation.StartTime
	} else {
		startTime = "00:00"
	}

	// Parse end time
	var endTime string
	if automation.EndTime != nil {
		endTime = *automation.EndTime
	} else {
		endTime = "23:59"
	}

	// Handle overnight windows (e.g., 22:00 to 06:00)
	if startTime > endTime {
		// Overnight window: current time should be >= start OR <= end
		return currentTime >= startTime || currentTime <= endTime
	}

	// Normal window: current time should be >= start AND <= end
	return currentTime >= startTime && currentTime <= endTime
}

// isDueForExecution checks if the automation is due for execution based on its interval.
// Returns true if enough time has passed since the last run.
func (s *Scheduler) isDueForExecution(automation *models.Automation, now time.Time) bool {
	// Get interval in minutes (default 60)
	intervalMinutes := 60
	if automation.IntervalMinutes != nil && *automation.IntervalMinutes > 0 {
		intervalMinutes = *automation.IntervalMinutes
	}

	// If never run before, it's due
	if automation.LastRunAt == nil || *automation.LastRunAt == "" {
		return true
	}

	// Parse last run time
	lastRunAt, err := time.Parse(time.RFC3339, *automation.LastRunAt)
	if err != nil {
		// If we can't parse, assume it's due
		s.logger.Warn("Failed to parse last_run_at",
			zap.String("id", automation.ID),
			zap.String("lastRunAt", *automation.LastRunAt),
			zap.Error(err))
		return true
	}

	// Check if next run time has passed
	nextRunAt := lastRunAt.Add(time.Duration(intervalMinutes) * time.Minute)
	if !now.After(nextRunAt) && !now.Equal(nextRunAt) {
		return false
	}

	// Defense in depth: also respect next_run_at if set.
	// If next_run_at is in the future, the automation is not due yet
	// (even if last_run_at suggests it is).
	if automation.NextRunAt != nil && *automation.NextRunAt != "" {
		nextScheduled, err := time.Parse(time.RFC3339, *automation.NextRunAt)
		if err == nil && now.Before(nextScheduled) {
			return false
		}
	}

	return true
}
