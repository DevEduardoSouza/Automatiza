package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

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
	processedCount := 0
	skippedCount := 0
	errorCount := 0

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

		// Execute the automation
		s.logger.Info("Running scheduled automation",
			zap.String("id", automation.ID),
			zap.String("name", automation.Name),
			zap.String("type", automation.GetType()),
			zap.String("userID", automation.UserID))

		result, err := s.service.RunAutomation(ctx, automation.ID, automation.UserID)
		if err != nil {
			s.logger.Error("Failed to run automation",
				zap.String("id", automation.ID),
				zap.String("name", automation.Name),
				zap.Error(err))
			errorCount++
			continue
		}

		s.logger.Info("Automation completed",
			zap.String("id", automation.ID),
			zap.String("name", automation.Name),
			zap.Int("messagesSent", result.MessagesSent),
			zap.Int("offersFound", result.OffersFound))

		processedCount++
	}

	s.logger.Info("Finished processing automations",
		zap.Int("processed", processedCount),
		zap.Int("skipped", skippedCount),
		zap.Int("errors", errorCount))

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
	start, err := time.Parse("2006-01-02", *automation.StartDate)
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
	end, err := time.Parse("2006-01-02", *automation.EndDate)
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
