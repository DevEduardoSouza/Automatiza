package cron

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/jobs"

	"go.uber.org/zap/zaptest"
)

// --- Mock Jobs Service ---

type mockJobsService struct {
	ProcessScheduledJobsFn func(ctx context.Context) (*jobs.ProcessResult, error)
}

func (m *mockJobsService) ProcessScheduledJobs(ctx context.Context) (*jobs.ProcessResult, error) {
	if m.ProcessScheduledJobsFn != nil {
		return m.ProcessScheduledJobsFn(ctx)
	}
	return &jobs.ProcessResult{}, nil
}

func (m *mockJobsService) GetQueueStatus(_ context.Context, _ string) (*jobs.QueueStatus, error) {
	return nil, nil
}

func (m *mockJobsService) ScheduleJob(_ context.Context, _ *models.ScheduledJob) error {
	return nil
}

func (m *mockJobsService) CancelJob(_ context.Context, _ string) error {
	return nil
}

func (m *mockJobsService) CreateJob(_ context.Context, _ string, _ *models.StandaloneJob) (*models.StandaloneJob, error) {
	return nil, nil
}

func (m *mockJobsService) GetJob(_ context.Context, _, _ string) (*models.StandaloneJob, error) {
	return nil, nil
}

func (m *mockJobsService) ListJobs(_ context.Context, _ string) ([]models.StandaloneJob, error) {
	return nil, nil
}

func (m *mockJobsService) ListJobsPaginated(_ context.Context, _ repository.JobListParams) (*repository.JobListResult, error) {
	return nil, nil
}

func (m *mockJobsService) UpdateJob(_ context.Context, _, _ string, _ map[string]interface{}) (*models.StandaloneJob, error) {
	return nil, nil
}

func (m *mockJobsService) DeleteJob(_ context.Context, _, _ string) error {
	return nil
}

// Compile-time check
var _ jobs.Service = (*mockJobsService)(nil)

// --- Mock Automation Scheduler ---

type mockAutomationScheduler struct {
	ProcessActiveAutomationsFn func(ctx context.Context) error
}

func (m *mockAutomationScheduler) ProcessActiveAutomations(ctx context.Context) error {
	if m.ProcessActiveAutomationsFn != nil {
		return m.ProcessActiveAutomationsFn(ctx)
	}
	return nil
}

var _ AutomationScheduler = (*mockAutomationScheduler)(nil)

// --- Tests ---

func TestNewCronService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	status := svc.GetStatus()
	if status.Running {
		t.Error("expected service not running on creation")
	}
}

func TestNewCronServiceWithScheduler(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronServiceWithScheduler(&mockJobsService{}, &mockAutomationScheduler{}, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestStart_SetsRunning(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	defer svc.Stop()

	status := svc.GetStatus()
	if !status.Running {
		t.Error("expected service to be running after Start")
	}
	if status.StartedAt == "" {
		t.Error("expected non-empty StartedAt")
	}
}

func TestStart_RegistersDefaultTasks(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	defer svc.Stop()

	status := svc.GetStatus()
	if len(status.Tasks) < 2 {
		t.Fatalf("expected at least 2 default tasks, got %d", len(status.Tasks))
	}

	taskNames := make(map[string]bool)
	for _, ts := range status.Tasks {
		taskNames[ts.Name] = true
	}

	if !taskNames["process_scheduled_jobs"] {
		t.Error("expected process_scheduled_jobs task")
	}
	if !taskNames["process_active_automations"] {
		t.Error("expected process_active_automations task")
	}
}

func TestStart_Idempotent(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	defer svc.Stop()

	startedAt := svc.GetStatus().StartedAt

	// Second start should be a no-op
	svc.Start()

	if svc.GetStatus().StartedAt != startedAt {
		t.Error("second Start should not change startedAt")
	}
}

func TestStop_SetsNotRunning(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	svc.Stop()

	status := svc.GetStatus()
	if status.Running {
		t.Error("expected service not running after Stop")
	}
}

func TestStop_Idempotent(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	svc.Stop()

	// Second stop should be a no-op (should not panic)
	svc.Stop()

	if svc.GetStatus().Running {
		t.Error("expected service not running after double Stop")
	}
}

func TestStop_WithoutStart(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	// Stop without Start should be a no-op (should not panic)
	svc.Stop()
}

func TestRegisterTask(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	called := make(chan struct{}, 1)
	svc.(*cronService).tasks = make(map[string]*cronTask) // clear default tasks

	svc.RegisterTask("test-task", 1*time.Hour, func(_ context.Context) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	})

	status := svc.GetStatus()
	found := false
	for _, ts := range status.Tasks {
		if ts.Name == "test-task" {
			found = true
			if ts.Interval != (1 * time.Hour).String() {
				t.Errorf("expected interval %s, got %s", (1 * time.Hour).String(), ts.Interval)
			}
		}
	}
	if !found {
		t.Error("expected test-task to be registered")
	}
}

func TestRegisterTask_StartsIfRunning(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	defer svc.Stop()

	var callCount atomic.Int32
	svc.RegisterTask("late-task", 50*time.Millisecond, func(_ context.Context) error {
		callCount.Add(1)
		return nil
	})

	// Wait for the task to execute at least once
	time.Sleep(200 * time.Millisecond)

	if callCount.Load() == 0 {
		t.Error("expected late-registered task to have run at least once")
	}
}

func TestGetStatus_NotRunning(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	status := svc.GetStatus()
	if status.Running {
		t.Error("expected not running")
	}
	if status.StartedAt != "" {
		t.Error("expected empty StartedAt when not running")
	}
}

func TestGetStatus_Running(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	svc.Start()
	defer svc.Stop()

	status := svc.GetStatus()
	if !status.Running {
		t.Error("expected running")
	}
	if status.StartedAt == "" {
		t.Error("expected non-empty StartedAt")
	}
	// Should have at least default tasks
	if len(status.Tasks) == 0 {
		t.Error("expected at least one task")
	}

	for _, ts := range status.Tasks {
		if ts.Name == "" {
			t.Error("expected non-empty task name")
		}
		if ts.Interval == "" {
			t.Error("expected non-empty interval")
		}
	}
}

func TestDefaultTask_ProcessScheduledJobs_CallsJobsService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	var called atomic.Bool
	mockJobs := &mockJobsService{
		ProcessScheduledJobsFn: func(_ context.Context) (*jobs.ProcessResult, error) {
			called.Store(true)
			return &jobs.ProcessResult{Processed: 1}, nil
		},
	}

	svc := NewCronService(mockJobs, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	// Find the process_scheduled_jobs task and execute it directly
	svc.registerDefaultTasks()

	task, ok := svc.tasks["process_scheduled_jobs"]
	if !ok {
		t.Fatal("expected process_scheduled_jobs task")
	}

	// Execute the task function directly
	err := task.task(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called.Load() {
		t.Error("expected ProcessScheduledJobs to be called")
	}
}

func TestDefaultTask_ProcessScheduledJobs_NilService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := NewCronService(nil, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	svc.registerDefaultTasks()

	task := svc.tasks["process_scheduled_jobs"]
	err := task.task(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when jobsService is nil, got %v", err)
	}
}

func TestDefaultTask_ProcessActiveAutomations_CallsScheduler(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	var called atomic.Bool
	mockScheduler := &mockAutomationScheduler{
		ProcessActiveAutomationsFn: func(_ context.Context) error {
			called.Store(true)
			return nil
		},
	}

	svc := NewCronServiceWithScheduler(&mockJobsService{}, mockScheduler, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	svc.registerDefaultTasks()

	task := svc.tasks["process_active_automations"]
	err := task.task(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called.Load() {
		t.Error("expected ProcessActiveAutomations to be called")
	}
}

func TestDefaultTask_ProcessActiveAutomations_NilScheduler(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := NewCronService(&mockJobsService{}, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	svc.registerDefaultTasks()

	task := svc.tasks["process_active_automations"]
	err := task.task(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when scheduler is nil, got %v", err)
	}
}

func TestDefaultTask_ProcessScheduledJobs_ReturnsError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	mockJobs := &mockJobsService{
		ProcessScheduledJobsFn: func(_ context.Context) (*jobs.ProcessResult, error) {
			return nil, errors.New("processing failed")
		},
	}

	svc := NewCronService(mockJobs, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	svc.registerDefaultTasks()

	task := svc.tasks["process_scheduled_jobs"]
	err := task.task(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "processing failed" {
		t.Errorf("expected 'processing failed', got %q", err.Error())
	}
}

func TestDefaultTask_ProcessActiveAutomations_ReturnsError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	mockScheduler := &mockAutomationScheduler{
		ProcessActiveAutomationsFn: func(_ context.Context) error {
			return errors.New("scheduler failed")
		},
	}

	svc := NewCronServiceWithScheduler(&mockJobsService{}, mockScheduler, logger).(*cronService)
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	svc.registerDefaultTasks()

	task := svc.tasks["process_active_automations"]
	err := task.task(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "scheduler failed" {
		t.Errorf("expected 'scheduler failed', got %q", err.Error())
	}
}

func TestExecuteTask_UpdatesMetrics(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := &cronService{
		logger: logger,
		tasks:  make(map[string]*cronTask),
	}
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	task := &cronTask{
		name:     "test-metrics",
		interval: 1 * time.Minute,
		task: func(_ context.Context) error {
			return nil
		},
	}
	svc.tasks["test-metrics"] = task

	svc.executeTask(task)

	if task.runCount != 1 {
		t.Errorf("expected runCount=1, got %d", task.runCount)
	}
	if task.lastRun.IsZero() {
		t.Error("expected lastRun to be set")
	}
	if task.nextRun.IsZero() {
		t.Error("expected nextRun to be set")
	}
	if task.lastError != nil {
		t.Errorf("expected lastError=nil, got %v", task.lastError)
	}
}

func TestExecuteTask_RecordsError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := &cronService{
		logger: logger,
		tasks:  make(map[string]*cronTask),
	}
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	expectedErr := errors.New("task failed")
	task := &cronTask{
		name:     "fail-task",
		interval: 1 * time.Minute,
		task: func(_ context.Context) error {
			return expectedErr
		},
	}
	svc.tasks["fail-task"] = task

	svc.executeTask(task)

	if task.lastError == nil {
		t.Fatal("expected lastError to be set")
	}
	if task.lastError.Error() != "task failed" {
		t.Errorf("expected 'task failed', got %q", task.lastError.Error())
	}
	if task.runCount != 1 {
		t.Errorf("expected runCount=1, got %d", task.runCount)
	}
}

func TestExecuteTask_IncrementsRunCount(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := &cronService{
		logger: logger,
		tasks:  make(map[string]*cronTask),
	}
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	task := &cronTask{
		name:     "count-task",
		interval: 1 * time.Minute,
		task: func(_ context.Context) error {
			return nil
		},
	}
	svc.tasks["count-task"] = task

	for i := 0; i < 5; i++ {
		svc.executeTask(task)
	}

	if task.runCount != 5 {
		t.Errorf("expected runCount=5, got %d", task.runCount)
	}
}

func TestTaskExecution_WithShortInterval(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	var callCount atomic.Int32
	mockJobs := &mockJobsService{
		ProcessScheduledJobsFn: func(_ context.Context) (*jobs.ProcessResult, error) {
			callCount.Add(1)
			return &jobs.ProcessResult{}, nil
		},
	}

	svc := NewCronService(mockJobs, logger)
	svc.Start()

	// Wait for at least 1 execution (tasks run immediately on start)
	time.Sleep(200 * time.Millisecond)
	svc.Stop()

	if callCount.Load() == 0 {
		t.Error("expected ProcessScheduledJobs to be called at least once")
	}
}

func TestGetStatus_TaskDetails(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := &cronService{
		logger:    logger,
		tasks:     make(map[string]*cronTask),
		running:   true,
		startedAt: time.Now(),
	}
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	defer svc.cancel()

	now := time.Now()
	errVal := errors.New("some error")
	svc.tasks["task-a"] = &cronTask{
		name:      "task-a",
		interval:  5 * time.Minute,
		lastRun:   now,
		nextRun:   now.Add(5 * time.Minute),
		lastError: errVal,
		runCount:  3,
	}
	svc.tasks["task-b"] = &cronTask{
		name:     "task-b",
		interval: 10 * time.Second,
		runCount: 0,
		// lastRun and nextRun are zero, lastError is nil
	}

	status := svc.GetStatus()

	if !status.Running {
		t.Error("expected running=true")
	}
	if len(status.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(status.Tasks))
	}

	taskMap := make(map[string]TaskStatus)
	for _, ts := range status.Tasks {
		taskMap[ts.Name] = ts
	}

	// task-a checks
	taskA := taskMap["task-a"]
	if taskA.RunCount != 3 {
		t.Errorf("task-a: expected runCount=3, got %d", taskA.RunCount)
	}
	if taskA.LastRun == "" {
		t.Error("task-a: expected non-empty LastRun")
	}
	if taskA.NextRun == "" {
		t.Error("task-a: expected non-empty NextRun")
	}
	if taskA.LastError != "some error" {
		t.Errorf("task-a: expected LastError='some error', got %q", taskA.LastError)
	}

	// task-b checks (zero values)
	taskB := taskMap["task-b"]
	if taskB.RunCount != 0 {
		t.Errorf("task-b: expected runCount=0, got %d", taskB.RunCount)
	}
	if taskB.LastRun != "" {
		t.Error("task-b: expected empty LastRun")
	}
	if taskB.LastError != "" {
		t.Error("task-b: expected empty LastError")
	}
}

func TestStartStop_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Start()
		}()
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Stop()
		}()
	}
	wg.Wait()

	// Should not panic and should be stopped
	if svc.GetStatus().Running {
		t.Error("expected service to be stopped")
	}
}

func TestExecuteTask_ClearsErrorOnSuccess(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	svc := &cronService{
		logger: logger,
		tasks:  make(map[string]*cronTask),
	}
	svc.ctx, svc.cancel = context.WithCancel(context.Background())
	svc.running = true
	defer svc.cancel()

	callCount := 0
	task := &cronTask{
		name:     "toggle-task",
		interval: 1 * time.Minute,
		task: func(_ context.Context) error {
			callCount++
			if callCount == 1 {
				return errors.New("first failure")
			}
			return nil
		},
	}
	svc.tasks["toggle-task"] = task

	// First execution should fail
	svc.executeTask(task)
	if task.lastError == nil {
		t.Fatal("expected error after first execution")
	}

	// Second execution should succeed and clear error
	svc.executeTask(task)
	if task.lastError != nil {
		t.Errorf("expected lastError=nil after success, got %v", task.lastError)
	}
}

// Regressão: tasks registradas ANTES de Start() (ex.: reconcile_instance_statuses
// fiada no main.go) entravam no mapa mas nunca tinham goroutine criada, porque
// RegisterTask só dispara quando running==true e Start() não iterava as
// pré-existentes. Efeito em prod: o reconciliador de status de instância nunca
// rodava e instâncias ficavam travadas em 'pending', bloqueando dispatches.
// runTask executa imediatamente no start da goroutine, então um task pré-
// registrado deve incrementar o contador logo após Start().
func TestStart_RunsTasksRegisteredBeforeStart(t *testing.T) {
	logger := zaptest.NewLogger(t)
	svc := NewCronService(&mockJobsService{}, logger).(*cronService)

	var ran atomic.Int32
	// Intervalo longo: garante UMA execução imediata e nenhuma repetição
	// durante o teste — isola a asserção do "run on start".
	svc.RegisterTask("preregistered-task", 1*time.Hour, func(_ context.Context) error {
		ran.Add(1)
		return nil
	})

	// Antes de Start(): não deve ter rodado (sem goroutine).
	if got := ran.Load(); got != 0 {
		t.Fatalf("task não deveria rodar antes de Start(), rodou %d vez(es)", got)
	}

	svc.Start()
	defer svc.Stop()

	// Aguarda a execução imediata da goroutine (até ~2s).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ran.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := ran.Load(); got < 1 {
		t.Fatalf("task pré-registrada não executou após Start() (contador=%d) — goroutine não foi criada", got)
	}
}
