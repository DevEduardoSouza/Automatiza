package jobs

import (
	"fmt"

	"automatiza-go/internal/models"
)

// ValidateAdminRequired validates that the user is admin when required by the action.
func ValidateAdminRequired(job *models.StandaloneJob, isAdmin bool) error {
	if job.IsAdminOnlyAction() && !isAdmin {
		return fmt.Errorf("você não é admin deste grupo")
	}
	return nil
}

// ValidateGroupOpen validates that messages can be sent to a closed group.
// Only admins can send to closed groups.
func ValidateGroupOpen(isClosed, isAdmin bool) error {
	if isClosed && !isAdmin {
		return fmt.Errorf("mensagens não podem ser enviadas em grupos fechados")
	}
	return nil
}

// ValidatePollOptions validates poll options count (2-12).
func ValidatePollOptions(options []string) error {
	if len(options) < 2 {
		return fmt.Errorf("enquete deve ter no mínimo 2 opções")
	}
	if len(options) > 12 {
		return fmt.Errorf("enquete deve ter no máximo 12 opções")
	}
	return nil
}

// ValidateSelectableCount validates the selectable count for a poll.
func ValidateSelectableCount(selectableCount int, optionsCount int) error {
	if selectableCount < 1 {
		return fmt.Errorf("selectableCount deve ser pelo menos 1")
	}
	if selectableCount > optionsCount {
		return fmt.Errorf("selectableCount não pode ser maior que o número de opções")
	}
	return nil
}

// ValidateGroups validates that at least one group is selected.
func ValidateGroups(groups []string) error {
	if len(groups) == 0 {
		return fmt.Errorf("pelo menos um grupo deve ser selecionado")
	}
	return nil
}

// ValidateMessage validates that a message is not empty.
func ValidateMessage(message string) error {
	if message == "" {
		return fmt.Errorf("mensagem não pode estar vazia")
	}
	return nil
}

