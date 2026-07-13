package workers_test

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/workers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Quando o WAHA recusa o envio com "not participating in that group", o worker
// deve: (1) não retentar (retorna nil), (2) marcar o dispatch como skipped e
// (3) cachear o grupo para que o próximo envio à mesma sessão/grupo saia cedo,
// sem tocar o WAHA de novo.
func TestProcessTask_NotParticipating_SkipsNoRetryAndCaches(t *testing.T) {
	fakeAdapter := whatsapp.NewFakeAdapter()
	dispatchRepo := new(mockDispatchLogRepo)
	logger := zap.NewNop()
	w := workers.NewMessageWorker(fakeAdapter, nil, nil, dispatchRepo, nil, logger)

	notPart := errors.New("failed to send image message: WAHA error 500: you're not participating in that group")
	fakeAdapter.SendImageFunc = func(_ context.Context, _ whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		return nil, notPart
	}
	// O fallback texto também falha pelo mesmo motivo (não dá pra enviar a grupo
	// do qual a sessão não participa).
	fakeAdapter.SendTextFunc = func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		return nil, notPart
	}

	dispatchRepo.On("Update", mock.Anything, mock.Anything, mock.MatchedBy(func(u map[string]interface{}) bool {
		return u["status"] == "skipped"
	})).Return(nil)

	imgURL := "https://cf.shopee.com.br/file/abc"
	id1 := 101
	data1 := workers.MessageJobData{
		InstanceName:  "session-x",
		GroupID:       "dead@g.us",
		Message:       "Oferta 1",
		ImageURL:      &imgURL,
		DispatchLogID: &id1,
		AutomationID:  "auto-1",
	}

	// Primeiro envio: tenta imagem (falha) + fallback texto (falha) e encerra
	// sem retry, marcando skipped e cacheando o grupo.
	err := w.ProcessTask(context.Background(), newTask(t, data1))
	require.NoError(t, err, "not-participating é terminal: retorna nil (sem retry)")
	assert.Equal(t, 1, fakeAdapter.CallCount("SendImage"), "primeira tentativa de imagem")
	assert.Equal(t, 1, fakeAdapter.CallCount("SendText"), "fallback texto na primeira tentativa")

	// Segundo envio à mesma sessão/grupo: sai cedo pelo cache, sem tocar o WAHA.
	id2 := 102
	data2 := data1
	data2.DispatchLogID = &id2
	data2.Message = "Oferta 2"
	err = w.ProcessTask(context.Background(), newTask(t, data2))
	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendImage"), "segundo envio nao deve chamar o WAHA (cache)")
	assert.Equal(t, 1, fakeAdapter.CallCount("SendText"), "segundo envio nao deve chamar o WAHA (cache)")

	dispatchRepo.AssertExpectations(t)
}
