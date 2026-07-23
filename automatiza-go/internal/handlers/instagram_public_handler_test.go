package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Fixtures obviamente sintéticas.
const (
	igTestAppSecret     = "segredo-de-teste-nao-e-real-0000"
	igTestEncryptionKey = "0000000000000000000000000000000000000000000000000000000000000000"
	igTestStateKey      = "chave-de-state-so-para-teste-0000000000"
	igTestLocalUser     = "11111111-1111-4111-8111-111111111111"
)

// --- fakes mínimos (o handler recebe o *instagram.Service concreto) ---

type igHandlerCredsRepo struct {
	mu     sync.Mutex
	byUser map[string]*models.InstagramCredentialsDB
}

func (f *igHandlerCredsRepo) Get(_ context.Context, userID string) (*models.InstagramCredentialsDB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byUser[userID], nil
}
func (f *igHandlerCredsRepo) GetByAppScopedUserID(_ context.Context, id string) (*models.InstagramCredentialsDB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.byUser {
		if id != "" && c.AppScopedUserID == id {
			return c, nil
		}
	}
	return nil, nil
}
func (f *igHandlerCredsRepo) Save(context.Context, string, string, string, string, string, time.Time) error {
	return nil
}
func (f *igHandlerCredsRepo) Delete(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byUser, userID)
	return nil
}
func (f *igHandlerCredsRepo) DeleteByAppScopedUserID(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for uid, c := range f.byUser {
		if id != "" && c.AppScopedUserID == id {
			delete(f.byUser, uid)
		}
	}
	return nil
}
func (f *igHandlerCredsRepo) ListExpiring(context.Context, time.Time) ([]models.InstagramCredentialsDB, error) {
	return nil, nil
}
func (f *igHandlerCredsRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byUser)
}

type igHandlerStoryRepo struct{}

func (igHandlerStoryRepo) GetOrCreate(context.Context, *models.InstagramStoryPost) (*models.InstagramStoryPost, bool, error) {
	return &models.InstagramStoryPost{}, false, nil
}
func (igHandlerStoryRepo) GetByID(context.Context, string) (*models.InstagramStoryPost, error) {
	return nil, nil
}
func (igHandlerStoryRepo) TryStartProcessing(context.Context, string, time.Time) (bool, time.Time, error) {
	return false, time.Time{}, nil
}
func (igHandlerStoryRepo) ReleaseProcessing(context.Context, string, time.Time) error { return nil }
func (igHandlerStoryRepo) SaveContainerID(context.Context, string, string) error      { return nil }
func (igHandlerStoryRepo) MarkResult(context.Context, string, string, string, string, string, bool) error {
	return nil
}
func (igHandlerStoryRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramStoryPost, error) {
	return nil, nil
}
func (igHandlerStoryRepo) CountPublishedSince(context.Context, string, time.Time) (int, error) {
	return 0, nil
}
func (igHandlerStoryRepo) DeleteOlderThan(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

type igHandlerDeletionRepo struct {
	mu       sync.Mutex
	bySubj   map[string]*models.InstagramDataDeletionRequest
	byCode   map[string]*models.InstagramDataDeletionRequest
	creds    *igHandlerCredsRepo
	failNext bool
}

func (f *igHandlerDeletionRepo) Process(_ context.Context, appScoped, subjectHash, candidate string, expiresAt time.Time) (*models.InstagramDataDeletionRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		return nil, context.DeadlineExceeded
	}
	if r, ok := f.bySubj[subjectHash]; ok {
		return r, nil
	}
	r := &models.InstagramDataDeletionRequest{
		SubjectHash: subjectHash, ConfirmationCode: candidate,
		Status: "completed", ExpiresAt: expiresAt,
	}
	f.bySubj[subjectHash] = r
	f.byCode[candidate] = r
	if f.creds != nil {
		_ = f.creds.DeleteByAppScopedUserID(context.Background(), appScoped)
	}
	return r, nil
}

func (f *igHandlerDeletionRepo) GetByConfirmationCode(_ context.Context, code string, now time.Time) (*models.InstagramDataDeletionRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byCode[code]
	if !ok || !r.ExpiresAt.After(now) {
		return nil, nil
	}
	return r, nil
}

func (f *igHandlerDeletionRepo) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

var (
	_ repository.InstagramCredentialsRepository  = (*igHandlerCredsRepo)(nil)
	_ repository.InstagramStoryPostRepository    = igHandlerStoryRepo{}
	_ repository.InstagramDataDeletionRepository = (*igHandlerDeletionRepo)(nil)
)

// --- harness ---

type igHandlerHarness struct {
	router *gin.Engine
	creds  *igHandlerCredsRepo
	del    *igHandlerDeletionRepo
	store  *instagram.MediaStore
	mr     *miniredis.Miniredis
}

func newIGHandlerTest(t *testing.T, appSecret string) *igHandlerHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cache.NewCache(rdb, "test", false)
	store := instagram.NewMediaStore(rdb, zap.NewNop())

	creds := &igHandlerCredsRepo{byUser: map[string]*models.InstagramCredentialsDB{}}
	del := &igHandlerDeletionRepo{
		bySubj: map[string]*models.InstagramDataDeletionRequest{},
		byCode: map[string]*models.InstagramDataDeletionRequest{},
		creds:  creds,
	}

	api := instagram.NewAPIClient("appid", appSecret, "https://api.example/api/v1/instagram/oauth/callback", zap.NewNop())
	svc := instagram.NewService(instagram.ServiceConfig{
		StoriesEnabled:  true,
		FrontendBaseURL: "https://app.example",
		BackendBaseURL:  "https://api.example",
		EncryptionKey:   igTestEncryptionKey,
		StateSigningKey: igTestStateKey,
	}, api, creds, igHandlerStoryRepo{}, del, nil /* appsRepo */, c, zap.NewNop())

	h := NewInstagramPublicHandler(svc, store, "https://app.example", zap.NewNop())
	r := gin.New()
	r.GET("/api/v1/instagram/oauth/callback", h.OAuthCallback)
	r.GET("/api/v1/public/story-media/:id", h.ServeStoryMedia)
	r.POST("/api/v1/instagram/deauthorize", h.Deauthorize)
	r.POST("/api/v1/instagram/data-deletion", h.DataDeletion)
	r.GET("/api/v1/instagram/data-deletion/status/:code", h.DataDeletionStatus)

	return &igHandlerHarness{router: r, creds: creds, del: del, store: store, mr: mr}
}

func igSignedRequest(t *testing.T, secret string, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + enc
}

func (h *igHandlerHarness) postForm(t *testing.T, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func (h *igHandlerHarness) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func (h *igHandlerHarness) seedCredential(appScoped string) {
	h.creds.mu.Lock()
	defer h.creds.mu.Unlock()
	exp := time.Now().Add(30 * 24 * time.Hour)
	h.creds.byUser[igTestLocalUser] = &models.InstagramCredentialsDB{
		UserID: igTestLocalUser, AppScopedUserID: appScoped, IGUserID: "IG-1",
		EncryptedData: "{}", TokenExpiresAt: &exp,
	}
}

// --- deauthorize ---

func TestInstagramDeauthorize_ValidSignatureDeletesCredential(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	h.seedCredential("APP-A")

	form := url.Values{"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": time.Now().Unix(),
	})}}
	w := h.postForm(t, "/api/v1/instagram/deauthorize", form)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if h.creds.count() != 0 {
		t.Error("credencial deveria ter sido apagada")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("resposta deveria ser no-store")
	}
}

// Cada caso é uma chamada forjada. O endpoint é público e apaga dados de
// cliente: aceitar qualquer um é perda de conta alheia.
func TestInstagramDeauthorize_RejectsForgeriesWithoutDeleting(t *testing.T) {
	now := time.Now()
	cases := map[string]url.Values{
		"assinatura errada": {"signed_request": {igSignedRequest(t, "outro-segredo-de-teste-000000000", map[string]any{
			"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
		})}},
		"algoritmo errado": {"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
			"algorithm": "none", "user_id": "APP-A",
		})}},
		"payload expirado": {"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
			"algorithm": "HMAC-SHA256", "user_id": "APP-A",
			"issued_at": now.Add(-2 * time.Hour).Unix(), "expires": now.Add(-time.Hour).Unix(),
		})}},
		"futuro excessivo": {"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
			"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Add(time.Hour).Unix(),
		})}},
		"campo ausente":  {},
		"campo vazio":    {"signed_request": {""}},
		"campo repetido": {"signed_request": {"a.b", "c.d"}},
	}
	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			h := newIGHandlerTest(t, igTestAppSecret)
			h.seedCredential("APP-A")

			w := h.postForm(t, "/api/v1/instagram/deauthorize", form)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if h.creds.count() != 1 {
				t.Error("chamada forjada APAGOU a credencial de um cliente")
			}
		})
	}
}

// Corpo acima de 64 KiB é recusado — endpoint público e não autenticado.
func TestInstagramDeauthorize_RejectsOversizedBody(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	h.seedCredential("APP-A")

	huge := url.Values{"signed_request": {strings.Repeat("A", 100<<10)}}
	w := h.postForm(t, "/api/v1/instagram/deauthorize", huge)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if h.creds.count() != 1 {
		t.Error("corpo gigante não deveria apagar nada")
	}
}

// App sem segredo configurado: 503, não 200 silencioso.
func TestInstagramDeauthorize_UnconfiguredReturns503(t *testing.T) {
	h := newIGHandlerTest(t, "") // sem app secret
	form := url.Values{"signed_request": {"a.b"}}
	w := h.postForm(t, "/api/v1/instagram/deauthorize", form)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// Repetição (conta já removida) segue 200: a Meta só quer saber que foi aceito,
// e diferenciar revelaria quem existe aqui.
func TestInstagramDeauthorize_RepeatIsIdempotent(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	h.seedCredential("APP-A")
	form := url.Values{"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": time.Now().Unix(),
	})}}

	for i := 0; i < 2; i++ {
		if w := h.postForm(t, "/api/v1/instagram/deauthorize", form); w.Code != http.StatusOK {
			t.Fatalf("chamada %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

// --- data deletion ---

func TestInstagramDataDeletion_ReturnsReceiptAndDeletes(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	h.seedCredential("APP-A")

	form := url.Values{"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": time.Now().Unix(),
	})}}
	w := h.postForm(t, "/api/v1/instagram/data-deletion", form)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		URL              string `json:"url"`
		ConfirmationCode string `json:"confirmation_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	if len(resp.ConfirmationCode) != 64 || resp.URL == "" {
		t.Fatalf("comprovante incompleto: %+v", resp)
	}
	if h.creds.count() != 0 {
		t.Error("credencial deveria ter sido apagada")
	}
	// Sem identificadores no corpo.
	for _, leak := range []string{"APP-A", "IG-1", igTestLocalUser} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("resposta vaza %q: %s", leak, w.Body.String())
		}
	}

	// A página de status reconhece o código, sem PII.
	statusResp := h.get(t, "/api/v1/instagram/data-deletion/status/"+resp.ConfirmationCode)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", statusResp.Code)
	}
	for _, leak := range []string{"APP-A", "IG-1", igTestLocalUser, resp.ConfirmationCode} {
		if strings.Contains(statusResp.Body.String(), leak) {
			t.Errorf("página de status vaza %q", leak)
		}
	}
	if got := statusResp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
}

// Falha no meio da operação não pode emitir comprovante: seria declarar uma
// exclusão que não aconteceu.
func TestInstagramDataDeletion_FailureEmitsNoReceipt(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	h.seedCredential("APP-A")
	h.del.failNext = true

	form := url.Values{"signed_request": {igSignedRequest(t, igTestAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": time.Now().Unix(),
	})}}
	w := h.postForm(t, "/api/v1/instagram/data-deletion", form)
	if w.Code == http.StatusOK {
		t.Fatal("falha na transação não pode responder 200 com comprovante")
	}
	if strings.Contains(w.Body.String(), "confirmation_code") {
		t.Error("resposta de erro não pode conter comprovante")
	}
	if h.creds.count() != 1 {
		t.Error("dados deveriam permanecer intactos após falha")
	}
}

func TestInstagramDataDeletionStatus_UnknownAndMalformedAre404(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	for _, code := range []string{
		strings.Repeat("a", 64), // bem-formado, inexistente
		"curto",
		strings.Repeat("Z", 64), // não-hex
		strings.Repeat("a", 63),
	} {
		w := h.get(t, "/api/v1/instagram/data-deletion/status/"+code)
		if w.Code != http.StatusNotFound {
			t.Errorf("código %q: status = %d, want 404", code, w.Code)
		}
	}
}

// --- mídia ---

func igTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstagramServeStoryMedia_ServesJPEGWithHardenedHeaders(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	want := igTestJPEG(t)
	id, err := h.store.Put(context.Background(), want, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	w := h.get(t, "/api/v1/public/story-media/"+id)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Error("bytes servidos diferem dos gravados")
	}
}

// Id malformado, ausente e expirado respondem o MESMO 404 — distinguir contaria
// a quem sonda como são os ids válidos.
func TestInstagramServeStoryMedia_Uniform404(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)
	for _, id := range []string{
		strings.Repeat("a", 32), // bem-formado, inexistente
		"curto",
		strings.Repeat("A", 32), // maiúsculas
		strings.Repeat("z", 32), // não-hex
	} {
		w := h.get(t, "/api/v1/public/story-media/"+id)
		if w.Code != http.StatusNotFound {
			t.Errorf("id %q: status = %d, want 404", id, w.Code)
		}
		if strings.Contains(w.Body.String(), id) {
			t.Errorf("resposta reflete o id %q", id)
		}
	}
}

// --- OAuth callback ---

// O redirect sempre aponta para a base CONFIGURADA — nunca para algo vindo da
// query, o que seria redirect aberto num fluxo de credencial.
func TestInstagramOAuthCallback_AlwaysRedirectsToConfiguredFrontend(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)

	w := h.get(t, "/api/v1/instagram/oauth/callback?code=X&state=lixo&redirect_uri=https://evil.example")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://app.example/instagram?") {
		t.Fatalf("Location = %q, deveria apontar para o frontend configurado", loc)
	}
	if strings.Contains(loc, "evil.example") {
		t.Fatal("redirect aberto: a query influenciou o destino")
	}
}

// Negação de consentimento com state inválido termina em invalid_state, e o
// valor do erro do provedor NÃO é refletido de volta.
func TestInstagramOAuthCallback_ProviderErrorIsNotReflected(t *testing.T) {
	h := newIGHandlerTest(t, igTestAppSecret)

	w := h.get(t, "/api/v1/instagram/oauth/callback?error=%3Cscript%3Ealert(1)%3C/script%3E&state=lixo")
	loc := w.Header().Get("Location")
	if strings.Contains(loc, "script") || strings.Contains(loc, "alert") {
		t.Fatalf("valor do provedor foi refletido no redirect: %q", loc)
	}
	if !strings.Contains(loc, "error=invalid_state") {
		t.Errorf("sem state válido o desfecho deveria ser invalid_state, got %q", loc)
	}
}

// O rate limit dos endpoints públicos é backstop de abuso — não autenticação —
// mas precisa existir de verdade: são rotas sem sessão, e uma delas apaga dados
// de cliente. Este teste registra o MESMO middleware do router.go e prova que,
// estourado o teto, o handler nem chega a ser executado.
func TestInstagramPublicRoutes_RateLimitBlocksBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlerRuns := 0
	limiter := middleware.WebhookRateLimitMiddleware(&middleware.WebhookRateLimiterConfig{
		MaxRequests: 2,
		WindowSize:  time.Minute,
		KeyPrefix:   "test:instagram_callback_rl",
	})
	r := gin.New()
	r.POST("/api/v1/instagram/deauthorize", limiter, func(c *gin.Context) {
		handlerRuns++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/instagram/deauthorize", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	if got := do(); got != http.StatusOK {
		t.Fatalf("1ª requisição: status = %d, want 200", got)
	}
	if got := do(); got != http.StatusOK {
		t.Fatalf("2ª requisição: status = %d, want 200", got)
	}
	if got := do(); got != http.StatusTooManyRequests {
		t.Fatalf("3ª requisição (acima do teto): status = %d, want 429", got)
	}
	if handlerRuns != 2 {
		t.Errorf("o handler deveria ter rodado só 2x (a 3ª é barrada antes), got %d", handlerRuns)
	}
}
