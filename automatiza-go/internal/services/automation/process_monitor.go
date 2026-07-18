package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// errWhatsAppNotConnected marca o caso "não há chip conectado para disparar"
// (chip desconectado ou nenhum chip). NÃO é erro de disparo: nada foi tentado e
// a automação fica em stand-by (o cron re-roda e retoma quando reconectar). O
// caller loga como 'skipped' (fora do card de erros); erro real de DB continua
// 'error'. errors.Is casa mesmo quando envolvido.
var errWhatsAppNotConnected = errors.New("WhatsApp nao esta conectado")

// processMonitorMode processes MONITOR mode automations.
//
// MONITOR mode is event-driven: it listens for incoming messages in monitored groups
// via webhooks and forwards them (with affiliate link conversion) to destination groups.
//
// When a user manually "runs" a monitoring automation, this function:
// 1. Validates the automation configuration
// 2. Verifies WhatsApp connectivity
// 3. Checks time window and dedupe settings
// 4. Returns a status message indicating the automation is active and listening
//
// The actual message processing happens in monitoring_service.ProcessIncomingMessage
// which is triggered by webhook events.
func (s *automationService) processMonitorMode(ctx context.Context, automation *models.Automation) (*models.RunAutomationResult, error) {
	s.logger.Info("Processing MONITOR mode automation",
		zap.String("automationID", automation.ID),
		zap.String("automationName", automation.Name))

	result := &models.RunAutomationResult{
		Success:      false,
		OffersFound:  0,
		MessagesSent: 0,
	}

	// 1. Validate monitor groups are configured
	monitorGroupIDs := automation.MonitorGroupIDs
	if len(monitorGroupIDs) == 0 {
		result.Message = "Nenhum grupo de monitoramento configurado. Configure pelo menos um grupo para monitorar."
		s.logDispatch(ctx, automation, "system", "config:monitor_groups", "error", result.Message)
		return result, nil
	}

	// Validate monitor group JIDs
	validMonitorGroups := make([]string, 0, len(monitorGroupIDs))
	for _, gid := range monitorGroupIDs {
		if strings.Contains(gid, "@g.us") {
			validMonitorGroups = append(validMonitorGroups, gid)
		}
	}

	if len(validMonitorGroups) == 0 {
		result.Message = "Nenhum grupo de monitoramento valido. Os grupos devem ser grupos do WhatsApp (@g.us)."
		s.logDispatch(ctx, automation, "system", "config:monitor_groups", "error", result.Message)
		return result, nil
	}

	// 2. Validate target/destination groups are configured
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		result.Message = "Nenhum grupo de destino configurado. Configure pelo menos um grupo para enviar as mensagens."
		s.logDispatch(ctx, automation, "system", "config:target_groups", "error", result.Message)
		return result, nil
	}

	// Validate target group JIDs
	validTargetGroups := make([]string, 0, len(targetGroups))
	for _, gid := range targetGroups {
		if strings.Contains(gid, "@g.us") {
			validTargetGroups = append(validTargetGroups, gid)
		}
	}

	if len(validTargetGroups) == 0 {
		result.Message = "Nenhum grupo de destino valido. Os grupos devem ser grupos do WhatsApp (@g.us)."
		s.logDispatch(ctx, automation, "system", "config:target_groups", "error", result.Message)
		return result, nil
	}

	// 3. Verify WhatsApp instance is connected
	instance, err := s.getConnectedInstance(ctx, automation.UserID)
	if err != nil {
		// Chip desconectado/ausente = stand-by, não erro de disparo (log 'skipped',
		// fora do card de erros). Erro real de DB continua 'error'.
		status := "error"
		if errors.Is(err, errWhatsAppNotConnected) {
			status = "skipped"
		}
		result.Message = fmt.Sprintf("WhatsApp nao conectado: %v", err)
		s.logDispatch(ctx, automation, "system", "config:whatsapp", status, result.Message)
		return result, nil
	}

	// 4. Check time window (if configured)
	timeWindowMsg := ""
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
			timeWindowMsg = fmt.Sprintf(" (Janela de horario: %s - %s, atualmente FORA do horario)",
				*automation.StartTime, *automation.EndTime)
		} else {
			timeWindowMsg = fmt.Sprintf(" (Janela de horario: %s - %s, DENTRO do horario)",
				*automation.StartTime, *automation.EndTime)
		}
	}

	// 5. Get dedupe settings
	dedupeHours := automation.GetDedupeHours()
	dedupeMsg := ""
	if dedupeHours > 0 {
		dedupeMsg = fmt.Sprintf(" Anti-duplicacao: %dh.", dedupeHours)
	}

	// 6. Get stores configuration
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"} // Default to Shopee
	}
	storesMsg := fmt.Sprintf(" Lojas: %s.", strings.Join(stores, ", "))

	// 7. Build success message
	result.Success = true
	result.Message = fmt.Sprintf(
		"Monitoramento ativo. Instancia: %s. Monitorando %d grupo(s), enviando para %d grupo(s).%s%s%s",
		instance.InstanceID,
		len(validMonitorGroups),
		len(validTargetGroups),
		storesMsg,
		dedupeMsg,
		timeWindowMsg,
	)

	// Log successful activation
	s.logDispatch(ctx, automation, "system", "monitor:active", "info",
		fmt.Sprintf("Monitoring activated - %d source groups, %d target groups",
			len(validMonitorGroups), len(validTargetGroups)))

	s.logger.Info("MONITOR mode automation validated successfully",
		zap.String("automationID", automation.ID),
		zap.String("instanceName", instance.InstanceID),
		zap.Int("monitorGroups", len(validMonitorGroups)),
		zap.Int("targetGroups", len(validTargetGroups)),
		zap.Strings("stores", stores),
		zap.Int("dedupeHours", dedupeHours))

	return result, nil
}

// getConnectedInstance returns a connected WhatsApp instance for the user.
// Returns an error if no instance is found or no instance is connected.
func (s *automationService) getConnectedInstance(ctx context.Context, userID string) (*models.Instance, error) {
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instances: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("nenhuma instancia WhatsApp encontrada: %w", errWhatsAppNotConnected)
	}

	for i := range instances {
		if instances[i].Status == "connected" {
			return &instances[i], nil
		}
	}

	return nil, fmt.Errorf("nenhuma instancia WhatsApp conectada: %w", errWhatsAppNotConnected)
}

// validateMonitorAutomation performs validation for a MONITOR mode automation.
// This can be used by the diagnose endpoint to check configuration before enabling.
func (s *automationService) validateMonitorAutomation(ctx context.Context, automation *models.Automation) (*models.DiagnoseResult, error) {
	result := &models.DiagnoseResult{
		Status: "ok",
		Checks: []models.DiagnoseCheck{},
	}

	// Check 1: Monitor groups configured
	monitorCheck := models.DiagnoseCheck{
		Name:   "GRUPOS_MONITORADOS",
		Status: "ok",
	}
	if len(automation.MonitorGroupIDs) == 0 {
		result.Status = "error"
		monitorCheck.Status = "error"
		monitorCheck.Message = "Nenhum grupo de monitoramento configurado"
	} else {
		validCount := 0
		for _, gid := range automation.MonitorGroupIDs {
			if strings.Contains(gid, "@g.us") {
				validCount++
			}
		}
		if validCount == 0 {
			result.Status = "error"
			monitorCheck.Status = "error"
			monitorCheck.Message = "Nenhum grupo de monitoramento valido (devem ser grupos @g.us)"
		} else {
			monitorCheck.Message = fmt.Sprintf("%d grupo(s) de monitoramento configurado(s)", validCount)
		}
	}
	result.Checks = append(result.Checks, monitorCheck)

	// Check 2: Target groups configured
	targetCheck := models.DiagnoseCheck{
		Name:   "GRUPOS_DESTINO",
		Status: "ok",
	}
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		result.Status = "error"
		targetCheck.Status = "error"
		targetCheck.Message = "Nenhum grupo de destino configurado"
	} else {
		validCount := 0
		for _, gid := range targetGroups {
			if strings.Contains(gid, "@g.us") {
				validCount++
			}
		}
		if validCount == 0 {
			result.Status = "error"
			targetCheck.Status = "error"
			targetCheck.Message = "Nenhum grupo de destino valido (devem ser grupos @g.us)"
		} else {
			targetCheck.Message = fmt.Sprintf("%d grupo(s) de destino configurado(s)", validCount)
		}
	}
	result.Checks = append(result.Checks, targetCheck)

	// Check 3: WhatsApp connected
	whatsappCheck := models.DiagnoseCheck{
		Name:   "WHATSAPP",
		Status: "ok",
	}
	_, err := s.getConnectedInstance(ctx, automation.UserID)
	if err != nil {
		result.Status = "error"
		whatsappCheck.Status = "error"
		whatsappCheck.Message = err.Error()
	} else {
		whatsappCheck.Message = "Instancia WhatsApp conectada"
	}
	result.Checks = append(result.Checks, whatsappCheck)

	// Check 4: Stores configured
	storesCheck := models.DiagnoseCheck{
		Name:   "LOJAS",
		Status: "ok",
	}
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}
	storesCheck.Message = fmt.Sprintf("Lojas configuradas: %s", strings.Join(stores, ", "))
	result.Checks = append(result.Checks, storesCheck)

	// Check 5: Time window
	if automation.StartTime != nil && automation.EndTime != nil {
		timeCheck := models.DiagnoseCheck{
			Name:   "JANELA_HORARIO",
			Status: "ok",
		}
		tw := &models.TimeWindow{
			StartTime: *automation.StartTime,
			EndTime:   *automation.EndTime,
			Timezone:  "America/Sao_Paulo",
		}
		if automation.ScheduleDays != nil {
			tw.Days = automation.ScheduleDays
		}

		if !tw.IsWithinWindow(time.Now()) {
			timeCheck.Status = "warning"
			timeCheck.Message = fmt.Sprintf("Fora da janela de horario (%s - %s)", *automation.StartTime, *automation.EndTime)
			if result.Status != "error" {
				result.Status = "warning"
			}
			result.Warnings = append(result.Warnings, timeCheck.Message)
		} else {
			timeCheck.Message = fmt.Sprintf("Dentro da janela de horario (%s - %s)", *automation.StartTime, *automation.EndTime)
		}
		result.Checks = append(result.Checks, timeCheck)
	}

	// Check 6: Dedupe settings
	dedupeCheck := models.DiagnoseCheck{
		Name:   "ANTI_DUPLICACAO",
		Status: "ok",
	}
	dedupeHours := automation.GetDedupeHours()
	if dedupeHours > 0 {
		dedupeCheck.Message = fmt.Sprintf("Anti-duplicacao ativa: %dh", dedupeHours)
	} else {
		dedupeCheck.Status = "warning"
		dedupeCheck.Message = "Anti-duplicacao desativada (pode enviar duplicates)"
		if result.Status != "error" {
			result.Status = "warning"
		}
		result.Warnings = append(result.Warnings, dedupeCheck.Message)
	}
	result.Checks = append(result.Checks, dedupeCheck)

	return result, nil
}
