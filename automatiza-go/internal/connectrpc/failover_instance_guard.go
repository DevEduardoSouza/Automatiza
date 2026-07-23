package connectrpc

import (
	"context"
	"fmt"

	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// =============================================================================
// Precheck de remoção de chip (Melhoria 5, seção 1 "Altas" item 7)
//
// As FKs da migração 077 são ON DELETE RESTRICT: o banco já se recusa a apagar
// uma instância que ocupa papel de primária ou reserva. Isso é a garantia — mas
// sozinha ela chega tarde e feia: o DELETE explode com um erro de FK depois de
// a sessão já ter sido desemparelhada na WAHA, e o usuário vê "erro interno"
// tendo perdido o número.
//
// Por isso o precheck roda ANTES de qualquer chamada mutável ao provider.
// =============================================================================

// failoverRoleBlocksDelete devolve erro amigável quando o chip está em uma
// política de failover. repo nil (ou consulta com falha) NÃO bloqueia: a FK
// RESTRICT continua sendo a rede de segurança real, e um soluço de banco não
// pode impedir o usuário de gerenciar os próprios chips.
func failoverRoleBlocksDelete(ctx context.Context, repo repository.InstanceFailoverRepository, instanceID int) error {
	if repo == nil {
		return nil
	}
	role, err := repo.GetRoleByInstance(ctx, instanceID)
	if err != nil {
		zap.L().Warn("failover precheck: could not read chip role, relying on FK RESTRICT",
			zap.Int("instanceID", instanceID), zap.Error(err))
		return nil
	}
	if role == nil {
		return nil
	}

	what := "reserva"
	if role.Role == "primary" {
		what = "primária"
	}
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(
		"este numero e o chip %s de uma politica de chip reserva - desative a politica antes de remove-lo", what))
}

// failoverRoleProtects é a variante silenciosa para rotinas de limpeza
// automática, que devem apenas registrar e PULAR a instância protegida — nunca
// abortar o lote inteiro por causa de um chip.
func failoverRoleProtects(ctx context.Context, repo repository.InstanceFailoverRepository, instanceID int) bool {
	if repo == nil {
		return false
	}
	role, err := repo.GetRoleByInstance(ctx, instanceID)
	if err != nil {
		zap.L().Warn("failover precheck: could not read chip role during cleanup",
			zap.Int("instanceID", instanceID), zap.Error(err))
		return false
	}
	if role == nil {
		return false
	}
	zap.L().Info("failover precheck: skipping cleanup of a chip that holds a failover role",
		zap.Int("instanceID", instanceID), zap.String("role", role.Role))
	return true
}
