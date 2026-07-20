package automation

import (
	"context"
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

// TestLogDispatch_DedupeSkipNotPersisted é a checagem executável do corte de
// dispatch_logs de dedupe (96% dos inserts, ~63/s): skip de dedupe NÃO chama
// o repositório; qualquer outro motivo de skip continua sendo gravado.
// (Kill-switch LOG_DEDUPE_SKIPS não é coberto aqui: a flag é sync.OnceValue
// de processo — o default false é o comportamento em produção.)
func TestLogDispatch_DedupeSkipNotPersisted(t *testing.T) {
	repo := new(mockDispatchLogRepo)
	svc := &automationService{
		dispatchLogRepo: repo,
		logger:          zaptest.NewLogger(t),
	}
	autoType := "search"
	auto := &models.Automation{
		ID:     "auto-1",
		UserID: "user-1",
		Name:   "teste",
		Type:   &autoType,
	}
	ctx := context.Background()

	// Dedupe puro e a variante da lista: nenhum Create.
	svc.logDispatch(ctx, auto, "g1", "http://p", "skipped", "dedupe")
	svc.logDispatch(ctx, auto, "g1", "http://p", "skipped", "dedupe (produto repetido na lista)")
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	// Skip por outro motivo: Create DEVE acontecer.
	repo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{ID: 1}, nil).Once()
	svc.logDispatch(ctx, auto, "g1", "http://p", "skipped", "WhatsApp nao esta conectado")
	repo.AssertExpectations(t)
}
