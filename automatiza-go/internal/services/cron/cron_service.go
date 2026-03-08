package cron

import (
	"context"
	"sync"
	"time"

	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/jobs"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Service defines the cron service interface
type Service interface {
	Start()
	Stop()
	RegisterTask(name string, interval time.Duration, task func(ctx context.Context) error)
	GetStatus() *CronStatus
}

// AutomationScheduler defines the interface for processing scheduled automations.
// This is implemented by the automation service and injected to avoid circular dependencies.
type AutomationScheduler interface {
	// ProcessActiveAutomations processes all active automations that are due for execution.
	// It handles time window filtering, interval checking, and execution.
	ProcessActiveAutomations(ctx context.Context) error
}

// CronStatus represents the current status of the cron service
type CronStatus struct {
	Running   bool         `json:"running"`
	Tasks     []TaskStatus `json:"tasks"`
	StartedAt string       `json:"startedAt,omitempty"`
}

// TaskStatus represents the status of a single cron task
type TaskStatus struct {
	Name      string `json:"name"`
	Interval  string `json:"interval"`
	LastRun   string `json:"lastRun,omitempty"`
	NextRun   string `json:"nextRun,omitempty"`
	LastError string `json:"lastError,omitempty"`
	RunCount  int64  `json:"runCount"`
}

type cronTask struct {
	name      string
	interval  time.Duration
	task      func(ctx context.Context) error
	lastRun   time.Time
	nextRun   time.Time
	lastError error
	runCount  int64
	ticker    *time.Ticker
	stopCh    chan struct{}
}

type cronService struct {
	jobsService         jobs.Service
	automationScheduler AutomationScheduler
	dispatchLogRepo     repository.DispatchLogRepository
	webhookEventsRepo   repository.WebhookEventsRepository
	logger              *zap.Logger
	tasks               map[string]*cronTask
	tasksMu             sync.RWMutex
	running             bool
	startedAt           time.Time
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup // tracks running goroutines for graceful shutdown
}

// NewCronService creates a new cron service
func NewCronService(
	jobsService jobs.Service,
	logger *zap.Logger,
) Service {
	return &cronService{
		jobsService: jobsService,
		logger:      logger,
		tasks:       make(map[string]*cronTask),
	}
}

// NewCronServiceWithScheduler creates a new cron service with automation scheduler support.
// Use this constructor when you want automations to be processed automatically.
func NewCronServiceWithScheduler(
	jobsService jobs.Service,
	automationScheduler AutomationScheduler,
	logger *zap.Logger,
) Service {
	return &cronService{
		jobsService:         jobsService,
		automationScheduler: automationScheduler,
		logger:              logger,
		tasks:               make(map[string]*cronTask),
	}
}

// NewCronServiceFull creates a cron service with all dependencies including cleanup repos.
func NewCronServiceFull(
	jobsService jobs.Service,
	automationScheduler AutomationScheduler,
	dispatchLogRepo repository.DispatchLogRepository,
	webhookEventsRepo repository.WebhookEventsRepository,
	logger *zap.Logger,
) Service {
	return &cronService{
		jobsService:         jobsService,
		automationScheduler: automationScheduler,
		dispatchLogRepo:     dispatchLogRepo,
		webhookEventsRepo:   webhookEventsRepo,
		logger:              logger,
		tasks:               make(map[string]*cronTask),
	}
}

func (s *cronService) Start() {
	s.tasksMu.Lock()
	if s.running {
		s.tasksMu.Unlock()
		return
	}
	s.running = true
	s.startedAt = time.Now()
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.tasksMu.Unlock()

	// Register default tasks.
	// Because s.running is already true, RegisterTask will start a goroutine
	// for each task automatically — no separate loop needed.
	s.registerDefaultTasks()

	s.logger.Info("Cron service started",
		zap.Int("tasks", len(s.tasks)))
}

func (s *cronService) Stop() {
	s.tasksMu.Lock()
	if !s.running {
		s.tasksMu.Unlock()
		return
	}

	// Cancel context to signal all goroutines
	if s.cancel != nil {
		s.cancel()
	}

	// Stop all task tickers and signal stop channels
	for _, task := range s.tasks {
		if task.ticker != nil {
			task.ticker.Stop()
		}
		if task.stopCh != nil {
			close(task.stopCh)
		}
	}

	s.running = false
	s.tasksMu.Unlock()

	// Wait for all goroutines to finish outside the lock to avoid deadlock
	// (goroutines acquire tasksMu in executeTask)
	s.wg.Wait()
	s.logger.Info("Cron service stopped")
}

func (s *cronService) RegisterTask(name string, interval time.Duration, task func(ctx context.Context) error) {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	ct := &cronTask{
		name:     name,
		interval: interval,
		task:     task,
		nextRun:  time.Now().Add(interval),
		stopCh:   make(chan struct{}),
	}

	s.tasks[name] = ct

	s.logger.Debug("Registered cron task",
		zap.String("name", name),
		zap.Duration("interval", interval))

	// If already running, start the task with SafeGo panic recovery (BG13)
	if s.running {
		s.wg.Add(1)
		observability.SafeGo("cron-task-"+ct.name, func() {
			defer s.wg.Done()
			s.runTask(ct)
		})
	}
}

func (s *cronService) GetStatus() *CronStatus {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	status := &CronStatus{
		Running: s.running,
		Tasks:   make([]TaskStatus, 0, len(s.tasks)),
	}

	if s.running {
		status.StartedAt = s.startedAt.Format(time.RFC3339)
	}

	for _, task := range s.tasks {
		ts := TaskStatus{
			Name:     task.name,
			Interval: task.interval.String(),
			RunCount: task.runCount,
		}

		if !task.lastRun.IsZero() {
			ts.LastRun = task.lastRun.Format(time.RFC3339)
		}

		if !task.nextRun.IsZero() {
			ts.NextRun = task.nextRun.Format(time.RFC3339)
		}

		if task.lastError != nil {
			ts.LastError = task.lastError.Error()
		}

		status.Tasks = append(status.Tasks, ts)
	}

	return status
}

func (s *cronService) runTask(task *cronTask) {
	task.ticker = time.NewTicker(task.interval)
	defer task.ticker.Stop()

	// Run immediately on start
	s.executeTask(task)

	for {
		select {
		case <-task.ticker.C:
			s.executeTask(task)
		case <-task.stopCh:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *cronService) executeTask(task *cronTask) {
	s.tasksMu.Lock()
	task.lastRun = time.Now()
	task.nextRun = time.Now().Add(task.interval)
	task.runCount++
	s.tasksMu.Unlock()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	// OTel: create root span per cron execution for end-to-end tracing
	tracer := otel.Tracer("cron")
	ctx, span := tracer.Start(ctx, "cron/"+task.name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("cron.task", task.name)),
	)
	defer span.End()

	err := task.task(ctx)

	s.tasksMu.Lock()
	task.lastError = err
	s.tasksMu.Unlock()

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		observability.LogAndCapture(s.logger, sentry.LevelError, "Cron task failed", err,
			map[string]string{
				"module": "cron",
				"task":   task.name,
			})
	} else {
		s.logger.Debug("Cron task completed",
			zap.String("task", task.name))
	}
}

func (s *cronService) registerDefaultTasks() {
	// Process scheduled jobs every minute (legacy scheduled_jobs table)
	s.RegisterTask("process_scheduled_jobs", 1*time.Minute, func(ctx context.Context) error {
		if s.jobsService == nil {
			return nil
		}
		_, err := s.jobsService.ProcessScheduledJobs(ctx)
		return err
	})

	// Process active automations every minute
	// This is the main automation scheduler that:
	// - Fetches all active automations
	// - Filters by time window (start_time/end_time)
	// - Checks if it's time to run (based on interval_minutes and last_run_at)
	// - Executes eligible automations
	s.RegisterTask("process_active_automations", 1*time.Minute, func(ctx context.Context) error {
		if s.automationScheduler == nil {
			return nil
		}
		return s.automationScheduler.ProcessActiveAutomations(ctx)
	})

	// Cleanup dispatch logs daily with differentiated retention by status
	s.RegisterTask("cleanup_dispatch_logs", 24*time.Hour, func(ctx context.Context) error {
		if s.dispatchLogRepo == nil {
			return nil
		}
		retentionPolicy := []struct {
			status string
			maxAge time.Duration
		}{
			{"skipped", 3 * 24 * time.Hour},
			{"sent", 14 * 24 * time.Hour},
			{"failed", 30 * 24 * time.Hour},
			{"info", 90 * 24 * time.Hour},
			{"error", 90 * 24 * time.Hour},
		}
		var totalDeleted int64
		for _, p := range retentionPolicy {
			deleted, err := s.dispatchLogRepo.DeleteOldLogsByStatus(ctx, p.status, p.maxAge)
			if err != nil {
				s.logger.Error("Dispatch log cleanup failed",
					zap.String("status", p.status),
					zap.Error(err))
				continue
			}
			totalDeleted += deleted
		}
		if totalDeleted > 0 {
			s.logger.Info("Dispatch log cleanup completed",
				zap.Int64("deleted", totalDeleted))
		}
		return nil
	})

	// Cleanup processed webhook events older than 7 days
	s.RegisterTask("cleanup_webhook_events", 24*time.Hour, func(ctx context.Context) error {
		if s.webhookEventsRepo == nil {
			return nil
		}
		deleted, err := s.webhookEventsRepo.Cleanup(ctx, 7*24*time.Hour)
		if err != nil {
			return err
		}
		if deleted > 0 {
			s.logger.Info("Webhook events cleanup completed",
				zap.Int64("deleted", deleted))
		}
		return nil
	})
}
