package workers

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// --- fakes ---

// fakeStoryAutomationRepo embute a interface para satisfazê-la por completo sem
// escrever ~50 métodos mortos: só o que o trigger chama é implementado, e
// qualquer chamada inesperada explode em vez de passar despercebida.
type fakeStoryAutomationRepo struct {
	repository.AutomationRepository
	monitoring []models.Automation
	offerGen   []models.Automation
}

func (f *fakeStoryAutomationRepo) GetMonitoringAutomationsForGroup(context.Context, string) ([]models.Automation, error) {
	return f.monitoring, nil
}

func (f *fakeStoryAutomationRepo) GetOfferGenerateAutomationsForGroup(context.Context, string) ([]models.Automation, error) {
	return f.offerGen, nil
}

type fakeIGStoryService struct {
	disabled  bool
	connected map[string]bool
}

func (f *fakeIGStoryService) FeatureEnabled() bool { return !f.disabled }
func (f *fakeIGStoryService) HasCredentials(_ context.Context, userID string) bool {
	return f.connected[userID]
}

// fakeMediaProvider entrega bytes controlados pelo teste (o FakeAdapter padrão
// devolve bytes que não são imagem).
type fakeMediaProvider struct {
	whatsapp.WhatsAppProvider
	data []byte
	mime string
}

func (f *fakeMediaProvider) DownloadMediaByURL(context.Context, string) (*whatsapp.DownloadMediaResult, error) {
	if len(f.data) == 0 {
		return nil, nil
	}
	return &whatsapp.DownloadMediaResult{Data: f.data, Mimetype: f.mime}, nil
}

func (f *fakeMediaProvider) RedownloadMedia(context.Context, string, string, string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}

// --- harness ---

type triggerHarness struct {
	tr    *StoryTrigger
	repo  *fakeStoryRepo
	ig    *fakeIGStoryService
	store *instagram.MediaStore
	mr    *miniredis.Miniredis
}

func newTriggerTest(t *testing.T, owners []string, media *fakeMediaProvider) *triggerHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := instagram.NewMediaStore(rdb, zap.NewNop())

	autos := make([]models.Automation, 0, len(owners))
	connected := map[string]bool{}
	for _, o := range owners {
		autos = append(autos, models.Automation{UserID: o})
		connected[o] = true
	}
	ig := &fakeIGStoryService{connected: connected}
	repo := newFakeStoryRepo()
	if media == nil {
		media = &fakeMediaProvider{}
	}
	queue := NewQueueClient(mr.Addr(), zap.NewNop())
	t.Cleanup(func() { _ = queue.Close() })

	tr := NewStoryTrigger(
		&fakeStoryAutomationRepo{monitoring: autos},
		ig, repo, media, store, queue, zap.NewNop(),
	)
	return &triggerHarness{tr: tr, repo: repo, ig: ig, store: store, mr: mr}
}

func storyMsg(id string) *models.IncomingMessage {
	return &models.IncomingMessage{
		RemoteJID: "grupo@g.us",
		MessageID: id,
		Body:      "Promoção do dia {story}",
	}
}

func (h *triggerHarness) ledgerCount() int {
	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()
	return len(h.repo.rows)
}

func (h *triggerHarness) queuedStoryTasks() int {
	n := 0
	for _, k := range h.mr.Keys() {
		if strings.HasPrefix(k, "asynq:{default}:t:ig-story:") {
			n++
		}
	}
	return n
}

// --- testes ---

// Duas entregas simultâneas do MESMO evento (mesmo dono, grupo e mensagem) —
// o caso real de dois chips e duas automações vendo a mesma mensagem — precisam
// produzir UMA linha e UM job. Antes a dedupe era um SETNX de 1h que falhava
// aberto, então Redis indisponível publicava o Story duas vezes.
func TestStoryTrigger_ConcurrentDeliveriesProduceSingleStory(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A"}, nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.tr.HandleStoryMessage(ctx, storyMsg("msg-1"))
		}()
	}
	wg.Wait()

	if got := h.ledgerCount(); got != 1 {
		t.Fatalf("esperado 1 linha no ledger, got %d", got)
	}
	if got := h.queuedStoryTasks(); got != 1 {
		t.Fatalf("esperado 1 job enfileirado, got %d", got)
	}
}

// Dois donos distintos do mesmo grupo: uma linha e um job POR DONO.
func TestStoryTrigger_TwoOwnersGetOneStoryEach(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A", "user-B"}, nil)
	h.tr.HandleStoryMessage(context.Background(), storyMsg("msg-1"))

	if got := h.ledgerCount(); got != 2 {
		t.Fatalf("esperado 2 linhas (uma por dono), got %d", got)
	}
	if got := h.queuedStoryTasks(); got != 2 {
		t.Fatalf("esperado 2 jobs, got %d", got)
	}
}

// O TaskID é determinístico e NÃO carrega user/grupo/mensagem em claro — ele
// fica visível no Redis e no dashboard do asynq.
func TestInstagramStoryTaskID_IsDeterministicAndOpaque(t *testing.T) {
	a := InstagramStoryTaskID("user-A", "grupo@g.us", "msg-1")
	b := InstagramStoryTaskID("user-A", "grupo@g.us", "msg-1")
	if a != b {
		t.Fatalf("TaskID deveria ser determinístico: %q != %q", a, b)
	}
	if c := InstagramStoryTaskID("user-B", "grupo@g.us", "msg-1"); c == a {
		t.Fatal("donos diferentes deveriam produzir TaskIDs diferentes")
	}
	for _, leak := range []string{"user-A", "grupo@g.us", "msg-1"} {
		if strings.Contains(a, leak) {
			t.Errorf("TaskID vaza %q em claro: %s", leak, a)
		}
	}
	// Separador explícito: sem ele, ("ab","c") e ("a","bc") colidiriam.
	if InstagramStoryTaskID("ab", "c", "x") == InstagramStoryTaskID("a", "bc", "x") {
		t.Error("concatenação sem separador permite colisão entre eventos distintos")
	}
}

// Mensagem sem id, sem grupo, vídeo ou texto vazio: nada é gravado nem
// enfileirado.
func TestStoryTrigger_IneligibleMessagesProduceNothing(t *testing.T) {
	cases := map[string]*models.IncomingMessage{
		"sem messageID": {RemoteJID: "grupo@g.us", Body: "oi"},
		"sem grupo":     {MessageID: "m1", Body: "oi"},
		"texto vazio":   {RemoteJID: "grupo@g.us", MessageID: "m1", Body: "   "},
		"vídeo":         {RemoteJID: "grupo@g.us", MessageID: "m1", Body: "x", HasMedia: true, MediaType: "video/mp4"},
		"nil":           nil,
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTriggerTest(t, []string{"user-A"}, nil)
			h.tr.HandleStoryMessage(context.Background(), msg)
			if got := h.ledgerCount(); got != 0 {
				t.Errorf("não deveria gravar ledger, got %d", got)
			}
			if got := h.queuedStoryTasks(); got != 0 {
				t.Errorf("não deveria enfileirar job, got %d", got)
			}
		})
	}
}

// Kill switch desligado: o trigger é no-op completo.
func TestStoryTrigger_FeatureDisabledIsNoop(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A"}, nil)
	h.ig.disabled = true

	h.tr.HandleStoryMessage(context.Background(), storyMsg("msg-1"))
	if h.ledgerCount() != 0 || h.queuedStoryTasks() != 0 {
		t.Error("feature desligada não pode gravar ledger nem enfileirar")
	}
}

// Dono sem credencial utilizável (inclui token vencido, que HasCredentials
// passa a reprovar) não gera job condenado a falhar.
func TestStoryTrigger_DisconnectedOwnerProducesNothing(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A"}, nil)
	h.ig.connected["user-A"] = false

	h.tr.HandleStoryMessage(context.Background(), storyMsg("msg-1"))
	if h.ledgerCount() != 0 || h.queuedStoryTasks() != 0 {
		t.Error("dono desconectado não deveria gerar ledger/job")
	}
}

// Imagem é normalizada para JPEG UMA vez e compartilhada entre os donos.
func TestStoryTrigger_ImageIsNormalizedOnceAndShared(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A", "user-B"}, &fakeMediaProvider{
		data: testJPEG(t), mime: "image/jpeg",
	})
	msg := storyMsg("msg-1")
	msg.HasMedia = true
	msg.MediaType = "image/jpeg"
	msg.MediaURL = "http://waha:3000/api/files/x.jpeg"

	h.tr.HandleStoryMessage(context.Background(), msg)

	keys := 0
	for _, k := range h.mr.Keys() {
		if strings.HasPrefix(k, "ig_media:") {
			keys++
		}
	}
	if keys != 1 {
		t.Fatalf("a mídia deveria ser guardada UMA vez e compartilhada, got %d chaves", keys)
	}
	if h.ledgerCount() != 2 {
		t.Fatalf("esperado 2 linhas (uma por dono), got %d", h.ledgerCount())
	}
}

// Bytes que não decodificam como imagem são recusados antes de qualquer
// gravação — o store agora valida de verdade.
func TestStoryTrigger_UndecodableImageIsRejected(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A"}, &fakeMediaProvider{
		data: []byte("isto-nao-e-uma-imagem"), mime: "image/jpeg",
	})
	msg := storyMsg("msg-1")
	msg.HasMedia = true
	msg.MediaType = "image/jpeg"
	msg.MediaURL = "http://waha:3000/api/files/x.jpeg"

	h.tr.HandleStoryMessage(context.Background(), msg)
	if h.ledgerCount() != 0 || h.queuedStoryTasks() != 0 {
		t.Error("imagem indecodificável não deveria virar ledger/job")
	}
}

// Reentrega TARDIA (depois do TTL de 1h do claim antigo) continua sendo no-op:
// é exatamente o caso que o dedupe em Redis não cobria.
func TestStoryTrigger_LateRedeliveryIsStillDeduped(t *testing.T) {
	h := newTriggerTest(t, []string{"user-A"}, nil)
	ctx := context.Background()

	h.tr.HandleStoryMessage(ctx, storyMsg("msg-1"))
	// Simula o tempo passando além de qualquer TTL de cache.
	h.mr.FastForward(2 * time.Hour)
	h.tr.HandleStoryMessage(ctx, storyMsg("msg-1"))

	if got := h.ledgerCount(); got != 1 {
		t.Fatalf("redelivery tardia criou linha nova: %d", got)
	}
}
