package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// =============================================================================
// Fake do ledger — registra o que foi chamado, para provar o que NÃO acontece
// =============================================================================

type fakeCadenceRepo struct {
	admitCalls  int
	hasPending  bool
	admitResult *models.MonitorAdmissionResult
	admitErr    error

	destinations []models.MonitorPendingDestination
	finished     map[string]string // groupID -> status
	rescheduled  int
	finalized    string
	finalCounts  models.MonitorPendingResult
}

func newFakeCadenceRepo() *fakeCadenceRepo {
	return &fakeCadenceRepo{finished: map[string]string{}}
}

func (f *fakeCadenceRepo) HasNonTerminalPending(context.Context, []string) (bool, error) {
	return f.hasPending, nil
}

func (f *fakeCadenceRepo) Admit(_ context.Context, in models.MonitorAdmissionInput) (*models.MonitorAdmissionResult, error) {
	f.admitCalls++
	if f.admitErr != nil {
		return nil, f.admitErr
	}
	if f.admitResult != nil {
		return f.admitResult, nil
	}
	res := &models.MonitorAdmissionResult{
		Deferred:   map[string]int64{},
		Suppressed: map[string]struct{}{},
	}
	for i, a := range in.Automations {
		res.Deferred[a.ID] = int64(i + 1)
	}
	return res, nil
}

func (f *fakeCadenceRepo) ClaimDueHeads(context.Context, int) ([]models.MonitorPendingLease, error) {
	return nil, nil
}
func (f *fakeCadenceRepo) MarkEnqueued(context.Context, int64, int, string) error { return nil }
func (f *fakeCadenceRepo) ReleaseEnqueueLease(context.Context, int64, int, string, time.Time) error {
	return nil
}
func (f *fakeCadenceRepo) ClaimPending(context.Context, int64, int, string) (*models.MonitorPendingWork, error) {
	return nil, nil
}
func (f *fakeCadenceRepo) MarkPendingStarted(context.Context, int64, string) error { return nil }
func (f *fakeCadenceRepo) ReschedulePending(context.Context, int64, string, time.Time, string) error {
	f.rescheduled++
	return nil
}
func (f *fakeCadenceRepo) FinalizePending(_ context.Context, _ int64, _, status string, result models.MonitorPendingResult, _ string) error {
	f.finalized = status
	f.finalCounts = result
	return nil
}
func (f *fakeCadenceRepo) MaterializeDestinations(_ context.Context, _ int64, dests []models.MonitorPendingDestination) ([]models.MonitorPendingDestination, error) {
	if len(f.destinations) == 0 {
		for i := range dests {
			d := dests[i]
			d.ID = int64(i + 100)
			d.Status = models.MonitorDestinationPending
			f.destinations = append(f.destinations, d)
		}
	}
	return f.destinations, nil
}
func (f *fakeCadenceRepo) ListDestinations(context.Context, int64) ([]models.MonitorPendingDestination, error) {
	return f.destinations, nil
}
func (f *fakeCadenceRepo) ClaimDestination(_ context.Context, id int64, _ string) (bool, error) {
	for i := range f.destinations {
		if f.destinations[i].ID == id && f.destinations[i].Status == models.MonitorDestinationPending {
			f.destinations[i].Status = models.MonitorDestinationClaimed
			f.destinations[i].Attempts++
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCadenceRepo) MarkDestinationSending(_ context.Context, id int64, _ string) (bool, error) {
	for i := range f.destinations {
		if f.destinations[i].ID == id && f.destinations[i].Status == models.MonitorDestinationClaimed {
			f.destinations[i].Status = models.MonitorDestinationSending
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCadenceRepo) FinishDestination(_ context.Context, id int64, status, _ string, _ *time.Time) error {
	for i := range f.destinations {
		if f.destinations[i].ID == id {
			f.destinations[i].Status = status
			f.finished[f.destinations[i].GroupID] = status
		}
	}
	return nil
}
func (f *fakeCadenceRepo) RecoverLeases(context.Context) (models.MonitorRecoveryStats, error) {
	return models.MonitorRecoveryStats{}, nil
}
func (f *fakeCadenceRepo) CancelForAutomation(context.Context, string) (int, error) { return 0, nil }
func (f *fakeCadenceRepo) Cleanup(context.Context, time.Time, int) (int, error)     { return 0, nil }
func (f *fakeCadenceRepo) GetMonitorQueueSummaries(context.Context, string, []string) (map[string]models.MonitorQueueSummary, error) {
	return nil, nil
}

var _ repository.MonitorCadenceRepository = (*fakeCadenceRepo)(nil)

func newCadenceService(t *testing.T, repo repository.MonitorCadenceRepository, media whatsapp.DurableMediaFetcher) *monitoringService {
	t.Helper()
	return &monitoringService{
		logger:       zap.NewNop(),
		cadenceRepo:  repo,
		durableMedia: media,
	}
}

// =============================================================================
// Gate: com a feature desligada, a admissão não pode tocar em NADA
// =============================================================================

func TestPrepareCadencedMessageIsInertWhenFlagOff(t *testing.T) {
	t.Setenv("FEATURE_MONITOR_DISPATCH_CADENCE", "false")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)

	repo := newFakeCadenceRepo()
	svc := newCadenceService(t, repo, nil)

	admission, err := svc.PrepareCadencedMessage(context.Background(), &models.IncomingMessage{
		RemoteJID: "123@g.us",
		MessageID: "abc",
		Body:      "confere https://shopee.com.br/product/1/2",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(admission.Deferred) != 0 || len(admission.Suppressed) != 0 {
		t.Errorf("com a flag desligada nada pode ser admitido: %+v", admission)
	}
	if repo.admitCalls != 0 {
		t.Errorf("com a flag desligada o banco não pode ser tocado (Admit chamado %d×)", repo.admitCalls)
	}
	if admission.Excluded() != nil {
		t.Error("nada a excluir do caminho imediato")
	}
}

func TestPrepareCadencedMessageIgnoresMessagesTheMirrorWouldSkip(t *testing.T) {
	t.Setenv("FEATURE_MONITOR_DISPATCH_CADENCE", "true")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)

	repo := newFakeCadenceRepo()
	svc := newCadenceService(t, repo, nil)

	cases := []struct {
		name string
		msg  *models.IncomingMessage
	}{
		{"não é grupo", &models.IncomingMessage{RemoteJID: "5531@c.us", MessageID: "a", Body: "https://shopee.com.br/x"}},
		{"é do próprio chip", &models.IncomingMessage{RemoteJID: "1@g.us", MessageID: "a", FromMe: true, Body: "https://shopee.com.br/x"}},
		{"sem message id", &models.IncomingMessage{RemoteJID: "1@g.us", Body: "https://shopee.com.br/x"}},
		{"sem link", &models.IncomingMessage{RemoteJID: "1@g.us", MessageID: "a", Body: "bom dia"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admission, err := svc.PrepareCadencedMessage(context.Background(), tc.msg)
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if len(admission.Deferred) != 0 {
				t.Error("mensagem que o espelho ignoraria não pode ocupar fila")
			}
			if repo.admitCalls != 0 {
				t.Error("nem chegar ao banco")
			}
		})
	}
}

// =============================================================================
// Mídia durável
// =============================================================================

type stubMediaFetcher struct {
	urlResult  *whatsapp.DownloadMediaResult
	urlErr     error
	redlResult *whatsapp.DownloadMediaResult
	redlErr    error
	urlCalls   int
	redlCalls  int
}

func (s *stubMediaFetcher) DownloadMediaByURLLimited(context.Context, string, int64) (*whatsapp.DownloadMediaResult, error) {
	s.urlCalls++
	return s.urlResult, s.urlErr
}

func (s *stubMediaFetcher) RedownloadMediaLimited(context.Context, string, string, string, int64) (*whatsapp.DownloadMediaResult, error) {
	s.redlCalls++
	return s.redlResult, s.redlErr
}

func imageMessage() *models.IncomingMessage {
	return &models.IncomingMessage{
		InstanceName: "s1",
		RemoteJID:    "1@g.us",
		MessageID:    "m1",
		Body:         "https://shopee.com.br/product/1/2",
		HasMedia:     true,
		MediaType:    "image",
		MimeType:     "image/jpeg",
		MediaURL:     "http://waha:3000/api/files/x.jpeg",
	}
}

func TestAcquireDurableMediaSkipsNonImages(t *testing.T) {
	svc := newCadenceService(t, newFakeCadenceRepo(), &stubMediaFetcher{})
	media, err := svc.acquireDurableMedia(context.Background(), &models.IncomingMessage{
		RemoteJID: "1@g.us", MessageID: "m", Body: "https://shopee.com.br/x",
	})
	if err != nil || media != nil {
		t.Fatalf("mensagem sem imagem não persiste mídia: media=%v err=%v", media, err)
	}
}

func TestAcquireDurableMediaUsesURLThenRedownload(t *testing.T) {
	stub := &stubMediaFetcher{
		urlErr:     errors.New("expirou"),
		redlResult: &whatsapp.DownloadMediaResult{Data: []byte("jpeg-bytes"), Mimetype: "image/png"},
	}
	svc := newCadenceService(t, newFakeCadenceRepo(), stub)

	media, err := svc.acquireDurableMedia(context.Background(), imageMessage())
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if stub.urlCalls != 1 || stub.redlCalls != 1 {
		t.Errorf("ordem errada de aquisição: url=%d redownload=%d", stub.urlCalls, stub.redlCalls)
	}
	if string(media.Bytes) != "jpeg-bytes" || media.Mimetype != "image/png" {
		t.Errorf("bytes/mimetype errados: %+v", media)
	}
	if media.SHA256 == "" || media.SizeBytes != len("jpeg-bytes") {
		t.Errorf("hash/tamanho não conferem: %+v", media)
	}
}

func TestAcquireDurableMediaFallsBackToInlineBase64(t *testing.T) {
	stub := &stubMediaFetcher{urlErr: errors.New("x"), redlErr: errors.New("y")}
	svc := newCadenceService(t, newFakeCadenceRepo(), stub)

	msg := imageMessage()
	msg.MediaBase64 = "aGVsbG8=" // "hello"
	media, err := svc.acquireDurableMedia(context.Background(), msg)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if string(media.Bytes) != "hello" {
		t.Errorf("esperava o base64 inline como último recurso, veio %q", media.Bytes)
	}
}

func TestAcquireDurableMediaRefusesWhenNothingIsDurable(t *testing.T) {
	stub := &stubMediaFetcher{urlErr: errors.New("x"), redlErr: errors.New("y")}
	svc := newCadenceService(t, newFakeCadenceRepo(), stub)

	_, err := svc.acquireDurableMedia(context.Background(), imageMessage())
	if err == nil || err.Error() != models.MonitorCodeMediaUnavailable {
		t.Fatalf("sem bytes duráveis a admissão precisa FALHAR (código fechado), veio: %v", err)
	}
}

func TestAcquireDurableMediaRefusesOversizedMedia(t *testing.T) {
	stub := &stubMediaFetcher{urlErr: whatsapp.ErrMediaTooLarge}
	svc := newCadenceService(t, newFakeCadenceRepo(), stub)

	_, err := svc.acquireDurableMedia(context.Background(), imageMessage())
	if err == nil || err.Error() != models.MonitorCodeMediaTooLarge {
		t.Fatalf("mídia acima do teto tem código próprio, veio: %v", err)
	}
	if stub.redlCalls != 0 {
		t.Error("mídia grande demais não deve ser re-baixada de outro jeito")
	}
}

// =============================================================================
// Ledger por destino
// =============================================================================

func TestDeferredRunFreezesDestinationOrderAndSkipsTerminals(t *testing.T) {
	repo := newFakeCadenceRepo()
	run := &deferredRun{repo: repo, work: &models.MonitorPendingWork{PendingID: 1}, logger: zap.NewNop()}

	remaining, err := run.materialize(context.Background(), []string{"g1", "g2", "g3"}, "shopee:1:2", "shopee")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("esperava 3 destinos, veio %d", len(remaining))
	}

	// g2 conclui; uma retomada não pode oferecê-lo de novo.
	run.finish(context.Background(), "g2", models.MonitorDestinationSent, "", 0)
	again, err := run.materialize(context.Background(), []string{"g3", "g2", "g1"}, "shopee:1:2", "shopee")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(again) != 2 || again[0] != "g1" || again[1] != "g3" {
		t.Errorf("retomada deve manter a ordem congelada e pular terminais, veio %v", again)
	}
}

func TestDeferredRunClaimIsSingleUse(t *testing.T) {
	repo := newFakeCadenceRepo()
	run := &deferredRun{repo: repo, work: &models.MonitorPendingWork{PendingID: 1}, logger: zap.NewNop()}
	if _, err := run.materialize(context.Background(), []string{"g1"}, "k", "shopee"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if !run.claim(context.Background(), "g1") {
		t.Fatal("primeiro claim deve passar")
	}
	if run.claim(context.Background(), "g1") {
		t.Error("segundo claim do mesmo destino não pode passar")
	}
	if !run.authorizeSend(context.Background(), "g1") {
		t.Error("autorização de envio deve passar após o claim")
	}
	if run.attempts["g1"] != 1 {
		t.Errorf("tentativas espelhadas em memória: %d", run.attempts["g1"])
	}
}

func TestDeferredRunRetriesUntilAttemptCapThenFails(t *testing.T) {
	repo := newFakeCadenceRepo()
	run := &deferredRun{repo: repo, work: &models.MonitorPendingWork{PendingID: 1}, logger: zap.NewNop()}
	if _, err := run.materialize(context.Background(), []string{"g1"}, "k", "shopee"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	run.attempts["g1"] = 1
	run.retryOrFailDestination(context.Background(), "g1", models.MonitorCodeSendFailed)
	if got := repo.finished["g1"]; got != models.MonitorDestinationPending {
		t.Errorf("com tentativas sobrando o destino volta para a fila, veio %q", got)
	}

	run.attempts["g1"] = models.MonitorMaxDestinationAttempts
	run.retryOrFailDestination(context.Background(), "g1", models.MonitorCodeSendFailed)
	if got := repo.finished["g1"]; got != models.MonitorDestinationFailed {
		t.Errorf("na última tentativa o destino falha em definitivo, veio %q", got)
	}
}

// =============================================================================
// Fechamento do item pelo estado dos destinos
// =============================================================================

func TestFinalizeDeferredClosesOnlyWhenEveryDestinationIsTerminal(t *testing.T) {
	ctx := context.Background()
	work := &models.MonitorPendingWork{PendingID: 1, ProcessingToken: "tok"}

	t.Run("todos terminais viram completed", func(t *testing.T) {
		repo := newFakeCadenceRepo()
		repo.destinations = []models.MonitorPendingDestination{
			{ID: 1, GroupID: "g1", Status: models.MonitorDestinationSent},
			{ID: 2, GroupID: "g2", Status: models.MonitorDestinationAssumedSent},
			{ID: 3, GroupID: "g3", Status: models.MonitorDestinationSkipped},
		}
		svc := newCadenceService(t, repo, nil)
		if err := svc.finalizeDeferred(ctx, work, &deferredRun{}, nil); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if repo.finalized != models.MonitorPendingCompleted {
			t.Errorf("status errado: %q", repo.finalized)
		}
		if repo.finalCounts.SentCount != 1 || repo.finalCounts.AssumedSentCount != 1 || repo.finalCounts.SkippedCount != 1 {
			t.Errorf("contadores errados: %+v", repo.finalCounts)
		}
	})

	t.Run("um destino falho reprova o item", func(t *testing.T) {
		repo := newFakeCadenceRepo()
		repo.destinations = []models.MonitorPendingDestination{
			{ID: 1, GroupID: "g1", Status: models.MonitorDestinationSent},
			{ID: 2, GroupID: "g2", Status: models.MonitorDestinationFailed},
		}
		svc := newCadenceService(t, repo, nil)
		if err := svc.finalizeDeferred(ctx, work, &deferredRun{}, nil); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if repo.finalized != models.MonitorPendingFailed {
			t.Errorf("status errado: %q", repo.finalized)
		}
	})

	t.Run("destino aberto devolve o item para a fila", func(t *testing.T) {
		repo := newFakeCadenceRepo()
		repo.destinations = []models.MonitorPendingDestination{
			{ID: 1, GroupID: "g1", Status: models.MonitorDestinationSent},
			{ID: 2, GroupID: "g2", Status: models.MonitorDestinationPending, NotBeforeAt: time.Now().Add(time.Minute)},
		}
		svc := newCadenceService(t, repo, nil)
		if err := svc.finalizeDeferred(ctx, work, &deferredRun{}, nil); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if repo.finalized != "" {
			t.Errorf("item com destino aberto NÃO pode ser fechado (veio %q)", repo.finalized)
		}
		if repo.rescheduled != 1 {
			t.Errorf("esperava reagendamento, veio %d", repo.rescheduled)
		}
	})

	t.Run("sem destino materializado e sem erro o item se encerra", func(t *testing.T) {
		repo := newFakeCadenceRepo()
		svc := newCadenceService(t, repo, nil)
		if err := svc.finalizeDeferred(ctx, work, &deferredRun{}, nil); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if repo.finalized != models.MonitorPendingCompleted {
			t.Errorf("nada a enviar é conclusão, não falha: %q", repo.finalized)
		}
	})
}
