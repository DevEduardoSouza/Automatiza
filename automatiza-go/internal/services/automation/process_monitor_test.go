package automation_test

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/services/automation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// ProcessMonitorMode Tests
// =============================================================================

func TestProcessMonitorMode_Success(t *testing.T) {
	// Config validation passes
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us", "source2@g.us"},
		GroupIDs:        []string{"target1@g.us", "target2@g.us"},
		Stores:          []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "ativo")
	assert.Contains(t, result.Message, "2 grupo(s)")
}

func TestProcessMonitorMode_NoMonitorGroups(t *testing.T) {
	// Error: no monitor groups configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{}, // No monitor groups
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "monitoramento")
}

func TestProcessMonitorMode_NoTargetGroups(t *testing.T) {
	// Error: no target groups configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{}, // No target groups
		SendGroups:      []string{}, // Also no send groups
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "destino")
}

func TestProcessMonitorMode_InvalidGroupID(t *testing.T) {
	// Error: group ID is not WhatsApp format
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"invalid-group-id", "another-invalid"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "@g.us")
}

func TestProcessMonitorMode_InvalidTargetGroupID(t *testing.T) {
	// Error: target group ID is not WhatsApp format
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"invalid-target"}, // Invalid format
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "@g.us")
}

func TestProcessMonitorMode_InstanceNotConnected(t *testing.T) {
	// Error: WhatsApp not connected
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
	}

	disconnectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "disconnected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(disconnectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "conectad")
}

func TestProcessMonitorMode_WithTimeWindow(t *testing.T) {
	// Monitor mode with time window configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	startTime := "08:00"
	endTime := "22:00"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
		StartTime:       &startTime,
		EndTime:         &endTime,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// Check that time window info is included in message
	assert.Contains(t, result.Message, "08:00")
	assert.Contains(t, result.Message, "22:00")
}

func TestProcessMonitorMode_WithDedupeHours(t *testing.T) {
	// Monitor mode with dedupe hours configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	dedupeHours := 48
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
		DedupeHours:     &dedupeHours,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "48h")
}

func TestProcessMonitorMode_DefaultStore(t *testing.T) {
	// Monitor mode with no stores configured should default to shopee
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{}, // No stores configured
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "shopee")
}

func TestProcessMonitorMode_MultipleStores(t *testing.T) {
	// Monitor mode with multiple stores configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee", "mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "shopee")
	assert.Contains(t, result.Message, "mercadolivre")
}

func TestProcessMonitorMode_MixedValidInvalidGroups(t *testing.T) {
	// Monitor mode with some valid and some invalid monitor groups
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"invalid-group", "source1@g.us", "another-invalid"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should succeed because at least one valid group exists
	assert.True(t, result.Success)
	// Should report only 1 valid monitor group
	assert.Contains(t, result.Message, "1 grupo(s)")
}

func TestProcessMonitorMode_NoInstances(t *testing.T) {
	// Monitor mode with no WhatsApp instances
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{"target1@g.us"},
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "conectad")
}

func TestProcessMonitorMode_UseSendGroupsAsTarget(t *testing.T) {
	// Monitor mode should use SendGroups if GroupIDs is empty
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		MonitorGroupIDs: []string{"source1@g.us"},
		GroupIDs:        []string{},                           // Empty
		SendGroups:      []string{"send1@g.us", "send2@g.us"}, // Use these instead
		Stores:          []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// Should report 2 target groups from SendGroups
	assert.Contains(t, result.Message, "2 grupo(s)")
}

// =============================================================================
// Table-Driven Tests for MONITOR Mode Edge Cases
// =============================================================================

func TestProcessMonitorMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		monitorGroupIDs []string
		groupIDs        []string
		sendGroups      []string
		instanceStatus  string
		wantSuccess     bool
		wantMsgContain  string
	}{
		{
			name:            "no monitor groups",
			monitorGroupIDs: []string{},
			groupIDs:        []string{"target@g.us"},
			instanceStatus:  "connected",
			wantSuccess:     false,
			wantMsgContain:  "monitoramento",
		},
		{
			name:            "all invalid monitor groups",
			monitorGroupIDs: []string{"invalid1", "invalid2"},
			groupIDs:        []string{"target@g.us"},
			instanceStatus:  "connected",
			wantSuccess:     false,
			wantMsgContain:  "@g.us",
		},
		{
			name:            "no target groups",
			monitorGroupIDs: []string{"source@g.us"},
			groupIDs:        []string{},
			sendGroups:      []string{},
			instanceStatus:  "connected",
			wantSuccess:     false,
			wantMsgContain:  "destino",
		},
		{
			name:            "all invalid target groups",
			monitorGroupIDs: []string{"source@g.us"},
			groupIDs:        []string{"invalid1", "invalid2"},
			instanceStatus:  "connected",
			wantSuccess:     false,
			wantMsgContain:  "@g.us",
		},
		{
			name:            "disconnected instance",
			monitorGroupIDs: []string{"source@g.us"},
			groupIDs:        []string{"target@g.us"},
			instanceStatus:  "disconnected",
			wantSuccess:     false,
			wantMsgContain:  "conectad",
		},
		{
			name:            "valid configuration",
			monitorGroupIDs: []string{"source@g.us"},
			groupIDs:        []string{"target@g.us"},
			instanceStatus:  "connected",
			wantSuccess:     true,
			wantMsgContain:  "ativo",
		},
		{
			name:            "mixed valid and invalid monitor groups",
			monitorGroupIDs: []string{"invalid", "source@g.us", "also-invalid"},
			groupIDs:        []string{"target@g.us"},
			instanceStatus:  "connected",
			wantSuccess:     true,
			wantMsgContain:  "1 grupo(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			deps := &testServiceDeps{
				automationRepo:     new(MockAutomationRepository),
				automationRunRepo:  new(MockAutomationRunRepository),
				instanceRepo:       new(MockInstanceRepository),
				groupRepo:          new(MockGroupRepository),
				dispatchLogRepo:    new(MockDispatchLogRepository),
				historyRepo:        new(MockActionHistoryRepository),
				settingsRepo:       new(MockSettingsRepository),
				profileRepo:        new(MockProfileRepository),
				mlLinkListItemRepo: new(MockMlLinkListItemRepository),
				shopeeService:      new(MockShopeeService),
				mlService:          new(MockMLService),
			}

			setupDefaultHistoryExpectations(deps.historyRepo)

			svc := automation.NewAutomationService(
				deps.automationRepo,
				deps.automationRunRepo,
				deps.instanceRepo,
				deps.groupRepo,
				deps.dispatchLogRepo,
				deps.historyRepo,
				deps.settingsRepo,
				deps.profileRepo,
				deps.mlLinkListItemRepo,
				nil, // mlLinkListRepo
				nil, // ctaRepo
				deps.shopeeService,
				deps.mlService,
				nil, nil, nil, nil,
				nil, // couponRepo
				nil, // couponRuleRepo
				logger,
			)

			ctx := context.Background()
			monitorType := "monitoring"
			auto := &models.Automation{
				ID:              "auto-123",
				UserID:          "user-1",
				Name:            "Test Automation",
				Status:          models.AutomationStatusActive,
				Type:            &monitorType,
				MonitorGroupIDs: tt.monitorGroupIDs,
				GroupIDs:        tt.groupIDs,
				SendGroups:      tt.sendGroups,
				Stores:          []string{"shopee"},
			}

			lockTime := time.Now().Add(10 * time.Minute)

			deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
			deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
			deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
			deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
			deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "h-1"}, nil)
			deps.historyRepo.On("Update", ctx, "h-1", mock.Anything).Return(nil)
			deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()

			// Check if we should expect instance lookup
			hasValidMonitorGroups := false
			for _, g := range tt.monitorGroupIDs {
				if len(g) > 5 && g[len(g)-5:] == "@g.us" {
					hasValidMonitorGroups = true
					break
				}
			}

			allTargetGroups := append(tt.groupIDs, tt.sendGroups...)
			hasValidTargetGroups := false
			for _, g := range allTargetGroups {
				if len(g) > 5 && g[len(g)-5:] == "@g.us" {
					hasValidTargetGroups = true
					break
				}
			}

			if len(tt.monitorGroupIDs) > 0 && hasValidMonitorGroups && len(allTargetGroups) > 0 && hasValidTargetGroups {
				deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
					{ID: 1, InstanceID: "instance-1", Status: tt.instanceStatus},
				}, nil)
			}
			deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

			result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSuccess, result.Success)
			if tt.wantMsgContain != "" {
				assert.Contains(t, result.Message, tt.wantMsgContain)
			}
		})
	}
}
