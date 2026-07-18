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

// processOfferGenerateMode processes OFFER_GENERATE mode automations.
//
// OFFER_GENERATE mode is event-driven: it listens for incoming messages containing
// product links in monitored groups via webhooks, scrapes the product, generates an
// affiliate offer, and dispatches it to destination groups.
//
// When a user manually "runs" an offer_generate automation, this function:
// 1. Validates the automation configuration
// 2. Verifies WhatsApp connectivity
// 3. Checks time window settings
// 4. Returns a status message indicating the automation is active and listening
//
// The actual message processing happens in offer_generate.Service.ProcessIncomingMessage
// which is triggered by webhook events.
func (s *automationService) processOfferGenerateMode(ctx context.Context, automation *models.Automation) (*models.RunAutomationResult, error) {
	s.logger.Info("Processing OFFER_GENERATE mode automation",
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

	// 2. Validate target/destination groups
	targetGroups := automation.GetTargetGroups()
	if len(targetGroups) == 0 {
		result.Message = "Nenhum grupo de destino configurado. Configure pelo menos um grupo para enviar as ofertas."
		s.logDispatch(ctx, automation, "system", "config:target_groups", "error", result.Message)
		return result, nil
	}

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

	// 3. Validate dispatch_text_id is configured (required for offer_generate)
	if automation.DispatchTextID == nil || *automation.DispatchTextID == "" {
		result.Message = "Template de texto obrigatorio para o modo Gerar Oferta. Configure um template de despacho."
		s.logDispatch(ctx, automation, "system", "config:dispatch_text", "error", result.Message)
		return result, nil
	}

	// 4. Verify WhatsApp instance is connected
	instance, err := s.getConnectedInstance(ctx, automation.UserID)
	if err != nil {
		// Chip desconectado/ausente = stand-by, nao erro de disparo (log 'skipped',
		// fora do card de erros). Erro real de DB continua 'error'.
		status := "error"
		if errors.Is(err, errWhatsAppNotConnected) {
			status = "skipped"
		}
		result.Message = fmt.Sprintf("WhatsApp nao conectado: %v", err)
		s.logDispatch(ctx, automation, "system", "config:whatsapp", status, result.Message)
		return result, nil
	}

	// 5. Check time window
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

	// 6. Get stores configuration
	stores := automation.Stores
	if len(stores) == 0 {
		stores = []string{"shopee"}
	}
	storesMsg := fmt.Sprintf(" Lojas: %s.", strings.Join(stores, ", "))

	// 7. Build success message
	result.Success = true
	result.Message = fmt.Sprintf(
		"Gerador de ofertas ativo. Instancia: %s. Monitorando %d grupo(s), enviando para %d grupo(s).%s%s",
		instance.InstanceID,
		len(validMonitorGroups),
		len(validTargetGroups),
		storesMsg,
		timeWindowMsg,
	)

	s.logDispatch(ctx, automation, "system", "offer_generate:active", "info",
		fmt.Sprintf("Offer generate activated - %d source groups, %d target groups",
			len(validMonitorGroups), len(validTargetGroups)))

	s.logger.Info("OFFER_GENERATE mode automation validated successfully",
		zap.String("automationID", automation.ID),
		zap.String("instanceName", instance.InstanceID),
		zap.Int("monitorGroups", len(validMonitorGroups)),
		zap.Int("targetGroups", len(validTargetGroups)),
		zap.Strings("stores", stores))

	return result, nil
}
