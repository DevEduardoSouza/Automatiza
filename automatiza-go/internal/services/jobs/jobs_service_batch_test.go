package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A criação em lote é a substituta do loop de N RPCs do frontend. As garantias
// que ela precisa dar e que o loop não dava: validar TUDO antes de gravar
// qualquer coisa, respeitar um teto próprio (o do cliente é sugestão) e
// repassar a contagem de duplicatas para a UI avisar sem alarmar.

func batchJob(scheduledFor time.Time, message string, groups ...string) *models.StandaloneJob {
	return &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		Status:       models.StandaloneJobStatusPending,
		ScheduledFor: scheduledFor,
		Payload: models.JobPayload{
			Groups:  groups,
			Message: message,
		},
	}
}

func TestCreateJobs_PassesBatchThroughAndReportsDedupe(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()
	at := time.Now().Add(time.Hour)

	input := []*models.StandaloneJob{
		batchJob(at, "oferta", "g1"),
		batchJob(at, "oferta", "g2"),
	}
	stored := []models.StandaloneJob{{ID: "job-1"}, {ID: "job-2"}}

	jobRepo.On("CreateBatch", ctx, mock.Anything).Return(stored, 1, nil)

	created, deduped, err := svc.CreateJobs(ctx, "user-1", input)

	require.NoError(t, err)
	assert.Len(t, created, 2)
	assert.Equal(t, 1, deduped)
	// O dono vem do contexto autenticado, nunca do request.
	for _, job := range input {
		assert.Equal(t, "user-1", job.UserID)
	}
	jobRepo.AssertExpectations(t)
}

// Item inválido no meio do lote não pode gravar nada: o repositório sequer é
// chamado, então não existe estado parcial para o usuário desfazer.
func TestCreateJobs_InvalidItemAbortsBeforeTouchingRepo(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()
	at := time.Now().Add(time.Hour)

	invalid := batchJob(at, "", "g2") // sem mensagem nem mídia

	_, _, err := svc.CreateJobs(ctx, "user-1", []*models.StandaloneJob{
		batchJob(at, "valido", "g1"),
		invalid,
	})

	require.Error(t, err)
	// O erro aponta QUAL item falhou — com 500 agendamentos, "inválido" sozinho
	// não ajuda ninguém.
	assert.Contains(t, err.Error(), "agendamento 2")
	jobRepo.AssertNotCalled(t, "CreateBatch", mock.Anything, mock.Anything)
}

func TestCreateJobs_RejectsBatchOverCap(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()
	at := time.Now().Add(time.Hour)

	oversized := make([]*models.StandaloneJob, 501)
	for i := range oversized {
		oversized[i] = batchJob(at.Add(time.Duration(i)*time.Second), "spam", "g1")
	}

	_, _, err := svc.CreateJobs(ctx, "user-1", oversized)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	jobRepo.AssertNotCalled(t, "CreateBatch", mock.Anything, mock.Anything)
}

func TestCreateJobs_RejectsEmptyBatch(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)

	_, _, err := svc.CreateJobs(context.Background(), "user-1", nil)

	require.Error(t, err)
	jobRepo.AssertNotCalled(t, "CreateBatch", mock.Anything, mock.Anything)
}

func TestCreateJobs_PropagatesRepositoryFailure(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("CreateBatch", ctx, mock.Anything).Return(nil, 0, errors.New("deadlock detected"))

	_, _, err := svc.CreateJobs(ctx, "user-1", []*models.StandaloneJob{
		batchJob(time.Now().Add(time.Hour), "oferta", "g1"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock detected")
}

// CreateJob unário continua vivo (frontend antigo em cache do Cloudflare ainda
// o usa) e agora é idempotente: a resposta de uma duplicata é o job original.
func TestCreateJob_DuplicateReturnsExistingJob(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existing := &models.StandaloneJob{ID: "job-original"}
	jobRepo.On("Create", ctx, mock.Anything).Return(existing, true, nil)

	created, err := svc.CreateJob(ctx, "user-1", batchJob(time.Now().Add(time.Hour), "oferta", "g1"))

	require.NoError(t, err)
	assert.Equal(t, "job-original", created.ID, "double-click devolve o job já existente, não um erro")
}
