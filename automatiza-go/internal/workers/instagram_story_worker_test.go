package workers

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/services/instagram"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type fakeStoryPublisher struct {
	calls []string // public URLs passed to PublishStoryImage
}

func (f *fakeStoryPublisher) PublishStoryImage(_ context.Context, _, publicURL string) (string, error) {
	f.calls = append(f.calls, publicURL)
	return "media-" + strconv.Itoa(len(f.calls)), nil
}

type fakeStoryRepo struct {
	published int
	results   []string // "id:status"
}

func (f *fakeStoryRepo) Create(context.Context, *models.InstagramStoryPost) (string, error) {
	return "ledger-x", nil
}
func (f *fakeStoryRepo) MarkResult(_ context.Context, id, status, _, _ string) error {
	f.results = append(f.results, id+":"+status)
	return nil
}
func (f *fakeStoryRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramStoryPost, error) {
	return nil, nil
}
func (f *fakeStoryRepo) CountPublishedSince(context.Context, string, time.Time) (int, error) {
	return f.published, nil
}

func newStoryWorkerTest(t *testing.T) (*InstagramStoryWorker, *instagram.MediaStore, *fakeStoryPublisher, *fakeStoryRepo, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := instagram.NewMediaStore(rdb, zap.NewNop())
	pub := &fakeStoryPublisher{}
	repo := &fakeStoryRepo{}
	w := NewInstagramStoryWorker(pub, store, repo, "https://test.example", zap.NewNop())
	return w, store, pub, repo, mr
}

func storyTask(t *testing.T, data InstagramStoryJobData) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask(TypeInstagramPublishStory, b)
}

// Regressão: quando dois donos DISTINTOS monitoram o mesmo grupo, o trigger
// guarda a imagem UMA vez (MediaKey compartilhada) e enfileira um job por dono
// com a mesma chave. A versão anterior deletava a chave compartilhada no fim do
// 1º job, e o 2º dono falhava com "mídia expirou". O fix serve a chave
// compartilhada direto (JPEG) e nunca a deleta (TTL cuida disso).
func TestInstagramStoryWorker_SharedImageKeySurvivesMultipleOwners(t *testing.T) {
	w, store, pub, _, _ := newStoryWorkerTest(t)
	ctx := context.Background()

	sharedKey, err := store.Put(ctx, []byte("fake-jpeg-bytes"), "image/jpeg", time.Hour)
	if err != nil {
		t.Fatalf("put shared media: %v", err)
	}

	for _, owner := range []string{"user-A", "user-B"} {
		task := storyTask(t, InstagramStoryJobData{UserID: owner, MediaKey: sharedKey, LedgerID: "l-" + owner})
		if err := w.ProcessTask(ctx, task); err != nil {
			t.Fatalf("owner %s: ProcessTask erro: %v", owner, err)
		}
	}

	if len(pub.calls) != 2 {
		t.Fatalf("esperado 2 publicações (1 por dono), obtido %d — a chave compartilhada foi deletada e o 2º dono falhou", len(pub.calls))
	}
	for _, url := range pub.calls {
		if !strings.Contains(url, sharedKey) {
			t.Errorf("URL de publicação deveria servir a chave compartilhada %q, got %q", sharedKey, url)
		}
	}
	if _, _, err := store.Get(ctx, sharedKey); err != nil {
		t.Errorf("chave compartilhada foi removida pelo worker (deveria sobreviver por TTL): %v", err)
	}
}

// A imagem gerada a partir do texto é criada pelo próprio worker e DEVE ser
// deletada após a publicação — não pode vazar no Redis.
func TestInstagramStoryWorker_RenderedTextKeyIsCleanedUp(t *testing.T) {
	w, _, pub, _, mr := newStoryWorkerTest(t)
	ctx := context.Background()

	task := storyTask(t, InstagramStoryJobData{UserID: "user-A", Text: "Promoção imperdível hoje", LedgerID: "l-1"})
	if err := w.ProcessTask(ctx, task); err != nil {
		t.Fatalf("ProcessTask erro: %v", err)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("esperado 1 publicação, obtido %d", len(pub.calls))
	}
	for _, k := range mr.Keys() {
		if strings.HasPrefix(k, "ig_media:") {
			t.Errorf("chave de mídia própria vazou (deveria ser deletada após publish): %s", k)
		}
	}
}

// Mídia expirada antes da publicação: falha PERMANENTE (SkipRetry), marca o
// ledger como failed e não publica — nunca crasha o worker.
func TestInstagramStoryWorker_ExpiredMediaMarksFailed(t *testing.T) {
	w, _, pub, repo, _ := newStoryWorkerTest(t)
	ctx := context.Background()

	task := storyTask(t, InstagramStoryJobData{UserID: "user-A", MediaKey: "nonexistent", LedgerID: "l-1"})
	err := w.ProcessTask(ctx, task)
	if err == nil {
		t.Fatal("esperado erro quando a mídia expirou")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("erro deveria ser permanente (SkipRetry), got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Error("não deveria publicar com mídia expirada")
	}
	if len(repo.results) != 1 || !strings.HasSuffix(repo.results[0], ":failed") {
		t.Errorf("ledger deveria ser marcado failed, got %v", repo.results)
	}
}
