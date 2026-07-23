package workers

import (
	"testing"

	"automatiza-go/internal/services/monitoring"

	"go.uber.org/zap"
)

// TestMonitoringImplementaRetryExecutor fecha o buraco encontrado na revisão:
// o main.go faz `monitoringService.(workers.RetryExecutor)` e, se a asserção
// falhar, o executor vira nil SILENCIOSAMENTE — o worker reivindica o item,
// reagenda 5 vezes e o encerra como exhausted sem nunca ter reprocessado.
//
// O teste replica a asserção EXATA do main.go: sobre o valor devolvido pelo
// construtor (tipo dinâmico), não sobre a interface monitoring.Service.
// Testar a interface estática passaria mesmo com a asserção de runtime
// falhando, que é justamente o modo de falha silencioso.
func TestMonitoringImplementaRetryExecutor(t *testing.T) {
	svc := monitoring.NewMonitoringService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())

	if _, ok := svc.(RetryExecutor); !ok {
		t.Fatal("o valor devolvido por NewMonitoringService não implementa " +
			"workers.RetryExecutor — a asserção do main.go cairia para nil e " +
			"nenhum item seria retentado")
	}
}
