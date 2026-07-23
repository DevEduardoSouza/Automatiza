package instagram

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Fixtures obviamente sintéticas — 32 bytes em hex, como o cofre exige.
const (
	testEncryptionKey = "0000000000000000000000000000000000000000000000000000000000000000"
	testStateKey      = "chave-de-state-so-para-teste-0000000000"
	userA             = "11111111-1111-4111-8111-111111111111"
	userB             = "22222222-2222-4222-8222-222222222222"
)

// --- fakes ---

type fakeCredsRepo struct {
	mu     sync.Mutex
	byUser map[string]*models.InstagramCredentialsDB
	// takenBy simula os índices únicos: conta da Meta -> usuário local dono.
	takenBy   map[string]string
	saveCalls int
	saveErr   error
	listErr   error
}

func newFakeCredsRepo() *fakeCredsRepo {
	return &fakeCredsRepo{
		byUser:  map[string]*models.InstagramCredentialsDB{},
		takenBy: map[string]string{},
	}
}

func (f *fakeCredsRepo) Get(_ context.Context, userID string) (*models.InstagramCredentialsDB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byUser[userID]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

func (f *fakeCredsRepo) GetByAppScopedUserID(_ context.Context, id string) (*models.InstagramCredentialsDB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id == "" {
		return nil, nil
	}
	for _, c := range f.byUser {
		if c.AppScopedUserID == id {
			cp := *c
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeCredsRepo) Save(_ context.Context, userID, encryptedData, appScopedUserID, igUserID, username string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	// Reproduz os índices únicos parciais da migração 077.
	for _, key := range []string{appScopedUserID, igUserID} {
		if key == "" {
			continue
		}
		if owner, ok := f.takenBy[key]; ok && owner != userID {
			return repository.ErrInstagramAccountAlreadyConnected
		}
	}
	f.saveCalls++
	exp := expiresAt
	f.byUser[userID] = &models.InstagramCredentialsDB{
		UserID: userID, EncryptedData: encryptedData, AppScopedUserID: appScopedUserID,
		IGUserID: igUserID, Username: username, TokenExpiresAt: &exp,
	}
	for _, key := range []string{appScopedUserID, igUserID} {
		if key != "" {
			f.takenBy[key] = userID
		}
	}
	return nil
}

func (f *fakeCredsRepo) Delete(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byUser, userID)
	return nil
}

func (f *fakeCredsRepo) DeleteByAppScopedUserID(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for uid, c := range f.byUser {
		if c.AppScopedUserID == id && id != "" {
			delete(f.byUser, uid)
		}
	}
	return nil
}

func (f *fakeCredsRepo) ListExpiring(_ context.Context, before time.Time) ([]models.InstagramCredentialsDB, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []models.InstagramCredentialsDB
	for _, c := range f.byUser {
		if c.TokenExpiresAt != nil && c.TokenExpiresAt.Before(before) {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *fakeCredsRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byUser)
}

type fakeSvcStoryRepo struct {
	mu      sync.Mutex
	deleted []int
}

func (f *fakeSvcStoryRepo) GetOrCreate(context.Context, *models.InstagramStoryPost) (*models.InstagramStoryPost, bool, error) {
	return &models.InstagramStoryPost{ID: "l-1"}, true, nil
}
func (f *fakeSvcStoryRepo) GetByID(context.Context, string) (*models.InstagramStoryPost, error) {
	return nil, nil
}
func (f *fakeSvcStoryRepo) TryStartProcessing(context.Context, string, time.Time) (bool, time.Time, error) {
	return true, time.Time{}, nil
}
func (f *fakeSvcStoryRepo) ReleaseProcessing(context.Context, string, time.Time) error { return nil }
func (f *fakeSvcStoryRepo) SaveContainerID(context.Context, string, string) error      { return nil }
func (f *fakeSvcStoryRepo) MarkResult(context.Context, string, string, string, string, string, bool) error {
	return nil
}
func (f *fakeSvcStoryRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramStoryPost, error) {
	return nil, nil
}
func (f *fakeSvcStoryRepo) CountPublishedSince(context.Context, string, time.Time) (int, error) {
	return 0, nil
}
func (f *fakeSvcStoryRepo) DeleteOlderThan(_ context.Context, _ time.Time, limit int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Um lote cheio e depois nada — prova que o cleanup pagina e para.
	if len(f.deleted) == 0 {
		f.deleted = append(f.deleted, limit)
		return int64(limit), nil
	}
	f.deleted = append(f.deleted, 0)
	return 0, nil
}

type fakeDeletionRepo struct {
	mu        sync.Mutex
	receipts  map[string]*models.InstagramDataDeletionRequest // por subject_hash
	byCode    map[string]*models.InstagramDataDeletionRequest
	processed []string // app-scoped ids recebidos
	err       error
	creds     *fakeCredsRepo
}

func newFakeDeletionRepo(creds *fakeCredsRepo) *fakeDeletionRepo {
	return &fakeDeletionRepo{
		receipts: map[string]*models.InstagramDataDeletionRequest{},
		byCode:   map[string]*models.InstagramDataDeletionRequest{},
		creds:    creds,
	}
}

func (f *fakeDeletionRepo) Process(_ context.Context, appScopedUserID, subjectHash, candidate string, expiresAt time.Time) (*models.InstagramDataDeletionRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.processed = append(f.processed, appScopedUserID)

	// Repetição do MESMO sujeito preserva o código original.
	if existing, ok := f.receipts[subjectHash]; ok {
		existing.CompletedAt = time.Now()
		return existing, nil
	}
	r := &models.InstagramDataDeletionRequest{
		SubjectHash: subjectHash, ConfirmationCode: candidate,
		Status: "completed", ExpiresAt: expiresAt,
	}
	f.receipts[subjectHash] = r
	f.byCode[candidate] = r

	// A exclusão faz parte da MESMA operação.
	if f.creds != nil && appScopedUserID != "" {
		_ = f.creds.DeleteByAppScopedUserID(context.Background(), appScopedUserID)
	}
	return r, nil
}

func (f *fakeDeletionRepo) GetByConfirmationCode(_ context.Context, code string, now time.Time) (*models.InstagramDataDeletionRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byCode[code]
	if !ok || !r.ExpiresAt.After(now) {
		return nil, nil
	}
	return r, nil
}

func (f *fakeDeletionRepo) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

// --- harness ---

type svcHarness struct {
	svc    *Service
	creds  *fakeCredsRepo
	del    *fakeDeletionRepo
	story  *fakeSvcStoryRepo
	cache  *cache.Cache
	mr     *miniredis.Miniredis
	apiHit *int
}

type svcOpts struct {
	storiesEnabled bool
	noRedis        bool
	shortStateKey  bool
	handler        http.HandlerFunc
}

func newServiceTest(t *testing.T, o svcOpts) *svcHarness {
	t.Helper()
	hits := 0
	handler := o.handler
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) }
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	api := NewAPIClient("appid", testAppSecret, "https://cb.example/api/v1/instagram/oauth/callback", zap.NewNop())
	api.baseAPI = srv.URL
	api.baseGraph = srv.URL

	var c *cache.Cache
	var mr *miniredis.Miniredis
	if !o.noRedis {
		mr = miniredis.RunT(t)
		c = cache.NewCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", false)
	}

	stateKey := testStateKey
	if o.shortStateKey {
		stateKey = "curta"
	}

	creds := newFakeCredsRepo()
	del := newFakeDeletionRepo(creds)
	story := &fakeSvcStoryRepo{}

	svc := NewService(ServiceConfig{
		StoriesEnabled:  o.storiesEnabled,
		FrontendBaseURL: "https://app.example",
		BackendBaseURL:  "https://api.example",
		EncryptionKey:   testEncryptionKey,
		StateSigningKey: stateKey,
	}, api, creds, story, del, nil /* appsRepo: sem app por cliente, cai no app global */, c, zap.NewNop())

	return &svcHarness{svc: svc, creds: creds, del: del, story: story, cache: c, mr: mr, apiHit: &hits}
}

// happyOAuthHandler responde os três passos do fluxo com os envelopes oficiais.
func happyOAuthHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
			fmt.Fprint(w, `{"data":[{"access_token":"SHORT","user_id":"APPSCOPED-1","permissions":"x"}]}`)
		case strings.HasSuffix(r.URL.Path, "/access_token"):
			fmt.Fprint(w, `{"access_token":"LONGTOKEN","token_type":"bearer","expires_in":5183944}`)
		case strings.HasSuffix(r.URL.Path, "/me"):
			fmt.Fprint(w, `{"data":[{"user_id":"PROFISSIONAL-1","username":"loja","account_type":"BUSINESS"}]}`)
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	}
}

// authorizeState extrai o state da URL de consentimento.
func authorizeState(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	st := u.Query().Get("state")
	if st == "" {
		t.Fatal("state ausente na URL de autorização")
	}
	return st
}

// --- kill switch e prontidão ---

// Provisionar app id/secret NÃO pode ligar a feature: ativar é decisão
// operacional explícita.
func TestService_KillSwitchOffKeepsFeatureDisabled(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: false})
	if h.svc.FeatureEnabled() {
		t.Fatal("feature deveria estar desligada com o kill switch off")
	}
	if !h.svc.Configured() {
		t.Fatal("configuração está completa; só o kill switch deveria estar off")
	}
	if _, err := h.svc.BuildAuthorizeURL(context.Background(), userA); !errors.Is(err, ErrInstagramFeatureDisabled) {
		t.Errorf("authorize deveria recusar com feature off, got %v", err)
	}
	if *h.apiHit != 0 {
		t.Error("nenhuma chamada à Meta pode acontecer com a feature off")
	}
}

// Flag ligada + configuração incompleta = feature CONTINUA desligada.
func TestService_EnabledButUnconfiguredStaysOff(t *testing.T) {
	for name, o := range map[string]svcOpts{
		"sem redis":       {storiesEnabled: true, noRedis: true},
		"state key curta": {storiesEnabled: true, shortStateKey: true},
	} {
		t.Run(name, func(t *testing.T) {
			h := newServiceTest(t, o)
			if h.svc.Configured() {
				t.Error("configuração incompleta não deveria contar como Configured")
			}
			if h.svc.FeatureEnabled() {
				t.Error("feature deveria permanecer desligada")
			}
		})
	}
}

// Sem Redis não há como garantir uso único do nonce — melhor não emitir URL.
func TestService_BuildAuthorizeURLRequiresRedis(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, noRedis: true})
	if _, err := h.svc.BuildAuthorizeURL(context.Background(), userA); err == nil {
		t.Fatal("sem Redis o authorize deveria falhar")
	}
}

// --- state ---

// O state trafega pela Meta e fica no histórico do navegador: não pode conter
// o UUID do usuário local.
func TestService_StateDoesNotCarryUserID(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	authURL, err := h.svc.BuildAuthorizeURL(context.Background(), userA)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizeState(t, authURL)
	if strings.Contains(state, userA) {
		t.Fatalf("state contém o UUID do usuário: %s", state)
	}
	// Nem em claro nem depois de decodificar o payload.
	enc, _, _ := strings.Cut(state, ".")
	decoded, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("payload do state não decodifica: %v", err)
	}
	if strings.Contains(string(decoded), userA) {
		t.Fatalf("payload decodificado contém o UUID: %s", decoded)
	}
}

func TestService_TamperedStateIsRejectedWithoutCallingMeta(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	authURL, err := h.svc.BuildAuthorizeURL(context.Background(), userA)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizeState(t, authURL)
	enc, sig, _ := strings.Cut(state, ".")
	tampered := enc + "." + strings.Repeat("0", len(sig))

	path, err := h.svc.HandleCallback(context.Background(), "CODE", tampered, "")
	if err == nil {
		t.Fatal("state adulterado deveria falhar")
	}
	if path != redirectInvalidState {
		t.Errorf("redirect = %q, want %q", path, redirectInvalidState)
	}
	if *h.apiHit != 0 {
		t.Error("state inválido não pode gerar chamada à Meta")
	}
	if h.creds.saveCalls != 0 {
		t.Error("nada deveria ter sido salvo")
	}
}

func TestService_ExpiredAndFutureStatesAreRejected(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()

	// Expirado: assinado com prazo no passado.
	expired := h.svc.signState("nonce-x", time.Now().Add(-time.Minute).Unix())
	if _, err := h.svc.consumeState(ctx, expired); err == nil {
		t.Error("state expirado deveria falhar")
	}
	// Futuro além do TTL: um state "eterno" seria replay perpétuo se a chave vazasse.
	tooFar := h.svc.signState("nonce-y", time.Now().Add(24*time.Hour).Unix())
	if _, err := h.svc.consumeState(ctx, tooFar); err == nil {
		t.Error("state com validade longa demais deveria falhar")
	}
}

// O ponto central do GETDEL: dois callbacks concorrentes com o MESMO state —
// exatamente um pode vencer. Com Get-depois-Delete, ambos passavam.
func TestService_ConcurrentCallbacksConsumeStateExactlyOnce(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	ctx := context.Background()
	authURL, err := h.svc.BuildAuthorizeURL(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizeState(t, authURL)

	const racers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := h.svc.HandleCallback(ctx, "CODE", state, "")
			if err == nil && path == redirectConnectedOK {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("exatamente 1 callback deveria vencer, got %d", successes)
	}
	if h.creds.saveCalls != 1 {
		t.Fatalf("exatamente 1 credencial deveria ser salva, got %d", h.creds.saveCalls)
	}
}

// Reuso do mesmo state depois de consumido é replay.
func TestService_StateIsSingleUse(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	ctx := context.Background()
	authURL, _ := h.svc.BuildAuthorizeURL(ctx, userA)
	state := authorizeState(t, authURL)

	if path, err := h.svc.HandleCallback(ctx, "CODE", state, ""); err != nil || path != redirectConnectedOK {
		t.Fatalf("1ª chamada deveria conectar: path=%s err=%v", path, err)
	}
	path, err := h.svc.HandleCallback(ctx, "CODE", state, "")
	if err == nil || path != redirectInvalidState {
		t.Fatalf("2ª chamada com o mesmo state deveria ser invalid_state: path=%s err=%v", path, err)
	}
}

// --- callback ---

func TestService_HappyCallbackStoresBothIdentifiers(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	ctx := context.Background()
	authURL, _ := h.svc.BuildAuthorizeURL(ctx, userA)

	path, err := h.svc.HandleCallback(ctx, "CODE", authorizeState(t, authURL), "")
	if err != nil || path != redirectConnectedOK {
		t.Fatalf("callback feliz falhou: path=%s err=%v", path, err)
	}

	cred, _ := h.creds.Get(ctx, userA)
	if cred == nil {
		t.Fatal("credencial não foi salva")
	}
	if cred.AppScopedUserID != "APPSCOPED-1" {
		t.Errorf("app_scoped_user_id = %q, want APPSCOPED-1", cred.AppScopedUserID)
	}
	// O id de publicação tem de vir do /me, não da troca do code.
	if cred.IGUserID != "PROFISSIONAL-1" {
		t.Errorf("ig_user_id = %q, want PROFISSIONAL-1 (do /me)", cred.IGUserID)
	}

	// O token guardado precisa decriptar de volta.
	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(cred.EncryptedData), &blob); err != nil {
		t.Fatalf("blob inválido: %v", err)
	}
	var tokens models.InstagramTokens
	if err := crypto.DecryptJSONV1(&blob, testEncryptionKey, &tokens); err != nil {
		t.Fatalf("token não decripta: %v", err)
	}
	if tokens.AccessToken != "LONGTOKEN" {
		t.Errorf("token = %q, want LONGTOKEN", tokens.AccessToken)
	}
	// E o token em claro não pode estar na coluna.
	if strings.Contains(cred.EncryptedData, "LONGTOKEN") {
		t.Error("token gravado em claro")
	}
}

// Negação de consentimento com state VÁLIDO: consome o state e responde denied.
// Sem state válido, é invalid_state — antes esse ramo nem olhava o state.
func TestService_ProviderErrorStillConsumesState(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	ctx := context.Background()
	authURL, _ := h.svc.BuildAuthorizeURL(ctx, userA)
	state := authorizeState(t, authURL)

	path, err := h.svc.HandleCallback(ctx, "", state, "access_denied")
	if err == nil {
		t.Fatal("negação deveria devolver erro")
	}
	if path != redirectDenied {
		t.Errorf("redirect = %q, want %q", path, redirectDenied)
	}
	if *h.apiHit != 0 {
		t.Error("negação não pode gerar chamada à Meta")
	}
	// O state foi CONSUMIDO: repetir a negação já não encontra o nonce.
	if path, _ := h.svc.HandleCallback(ctx, "", state, "access_denied"); path != redirectInvalidState {
		t.Errorf("state deveria ter sido consumido na negação, got %q", path)
	}

	// Negação sem state válido = invalid_state.
	if path, _ := h.svc.HandleCallback(ctx, "", "lixo.deadbeef", "access_denied"); path != redirectInvalidState {
		t.Errorf("negação sem state válido deveria ser invalid_state, got %q", path)
	}
}

func TestService_CallbackFailurePaths(t *testing.T) {
	cases := []struct {
		name     string
		handler  http.HandlerFunc
		code     string
		wantPath string
	}{
		{
			name:     "code vazio",
			handler:  happyOAuthHandler(t),
			code:     "",
			wantPath: redirectNoCode,
		},
		{
			name: "/me falha",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
					fmt.Fprint(w, `{"data":[{"access_token":"SHORT","user_id":"APPSCOPED-1"}]}`)
				case strings.HasSuffix(r.URL.Path, "/access_token"):
					fmt.Fprint(w, `{"access_token":"LONG","token_type":"bearer","expires_in":5183944}`)
				default:
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"error":{"message":"boom"}}`)
				}
			},
			code:     "CODE",
			wantPath: redirectProfileFailed,
		},
		{
			name: "/me vazio",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
					fmt.Fprint(w, `{"data":[{"access_token":"SHORT","user_id":"APPSCOPED-1"}]}`)
				case strings.HasSuffix(r.URL.Path, "/access_token"):
					fmt.Fprint(w, `{"access_token":"LONG","token_type":"bearer","expires_in":5183944}`)
				default:
					fmt.Fprint(w, `{"data":[]}`)
				}
			},
			code:     "CODE",
			wantPath: redirectProfileFailed,
		},
		{
			name: "expiry zero",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
					fmt.Fprint(w, `{"data":[{"access_token":"SHORT","user_id":"APPSCOPED-1"}]}`)
				default:
					fmt.Fprint(w, `{"access_token":"LONG","token_type":"bearer","expires_in":0}`)
				}
			},
			code:     "CODE",
			wantPath: redirectExchangeFailed,
		},
		{
			name: "troca do code falha",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"bad code","code":100}}`)
			},
			code:     "CODE",
			wantPath: redirectExchangeFailed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: tc.handler})
			ctx := context.Background()
			authURL, _ := h.svc.BuildAuthorizeURL(ctx, userA)

			path, err := h.svc.HandleCallback(ctx, tc.code, authorizeState(t, authURL), "")
			if err == nil {
				t.Fatal("esperava erro")
			}
			if path != tc.wantPath {
				t.Errorf("redirect = %q, want %q", path, tc.wantPath)
			}
			if h.creds.saveCalls != 0 {
				t.Errorf("nada deveria ser salvo, got %d saves", h.creds.saveCalls)
			}
		})
	}
}

// Conta profissional já vinculada a OUTRO usuário local: sem os índices únicos,
// dois clientes publicariam no mesmo perfil.
func TestService_AccountAlreadyConnectedToAnotherUser(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true, handler: happyOAuthHandler(t)})
	ctx := context.Background()

	// userB conecta primeiro.
	authURLB, _ := h.svc.BuildAuthorizeURL(ctx, userB)
	if path, err := h.svc.HandleCallback(ctx, "CODE", authorizeState(t, authURLB), ""); err != nil {
		t.Fatalf("primeira conexão falhou: %s %v", path, err)
	}

	// userA tenta a MESMA conta do Instagram.
	authURLA, _ := h.svc.BuildAuthorizeURL(ctx, userA)
	path, err := h.svc.HandleCallback(ctx, "CODE", authorizeState(t, authURLA), "")
	if !errors.Is(err, repository.ErrInstagramAccountAlreadyConnected) {
		t.Fatalf("esperava ErrInstagramAccountAlreadyConnected, got %v", err)
	}
	if path != redirectAlreadyLinked {
		t.Errorf("redirect = %q, want %q", path, redirectAlreadyLinked)
	}
}

// --- expiração e isolamento ---

func (h *svcHarness) seedCredential(t *testing.T, userID, appScoped, igUserID string, expiresAt time.Time) {
	t.Helper()
	blob, err := crypto.EncryptJSONV1(models.InstagramTokens{AccessToken: "TOKEN-" + userID}, testEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.creds.Save(context.Background(), userID, string(raw), appScoped, igUserID, "u", expiresAt); err != nil {
		t.Fatal(err)
	}
}

// Linha presente com token VENCIDO é conexão morta. Antes a UI dizia
// "conectado" enquanto todo envio falhava.
func TestService_ExpiredTokenCountsAsDisconnected(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	h.seedCredential(t, userA, "APP-A", "IG-A", time.Now().Add(-time.Hour))

	status, err := h.svc.Status(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if status.Connected {
		t.Error("Status.Connected deveria ser falso com token vencido")
	}
	if h.svc.HasCredentials(ctx, userA) {
		t.Error("HasCredentials deveria ser falso com token vencido")
	}
	token, igID, err := h.svc.AccessToken(ctx, userA)
	if !errors.Is(err, ErrInstagramTokenExpired) {
		t.Fatalf("esperava ErrInstagramTokenExpired, got %v", err)
	}
	if token != "" || igID != "" {
		t.Error("token/id não podem vazar quando a credencial está vencida")
	}
}

// Expiry NULO é credencial de origem desconhecida — tratá-la como eterna é
// exatamente o erro que se quer evitar.
func TestService_NilExpiryIsNotUsable(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	h.creds.byUser[userA] = &models.InstagramCredentialsDB{
		UserID: userA, IGUserID: "IG-A", EncryptedData: "{}", TokenExpiresAt: nil,
	}
	if h.svc.HasCredentials(ctx, userA) {
		t.Error("expiry nulo não pode contar como conectado")
	}
}

func TestService_CredentialsAreIsolatedPerUser(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	future := time.Now().Add(30 * 24 * time.Hour)
	h.seedCredential(t, userA, "APP-A", "IG-A", future)
	h.seedCredential(t, userB, "APP-B", "IG-B", future)

	tokenA, igA, err := h.svc.AccessToken(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	tokenB, igB, err := h.svc.AccessToken(ctx, userB)
	if err != nil {
		t.Fatal(err)
	}
	if tokenA == tokenB || igA == igB {
		t.Fatal("credenciais de usuários distintos vazaram entre si")
	}
	if tokenA != "TOKEN-"+userA || tokenB != "TOKEN-"+userB {
		t.Errorf("tokens trocados: A=%q B=%q", tokenA, tokenB)
	}
}

func TestService_DisconnectRemovesOnlyRequestedUser(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	future := time.Now().Add(30 * 24 * time.Hour)
	h.seedCredential(t, userA, "APP-A", "IG-A", future)
	h.seedCredential(t, userB, "APP-B", "IG-B", future)

	if err := h.svc.Disconnect(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if c, _ := h.creds.Get(ctx, userA); c != nil {
		t.Error("credencial de A deveria ter sido apagada")
	}
	if c, _ := h.creds.Get(ctx, userB); c == nil {
		t.Error("credencial de B NÃO deveria ter sido tocada")
	}
}

// --- refresh ---

// Uma conta com problema não pode interromper a renovação das outras — e o
// refresh precisa rodar mesmo com o kill switch de publicação desligado.
func TestService_RefreshContinuesPastBrokenAccounts(t *testing.T) {
	var refreshCalls int
	h := newServiceTest(t, svcOpts{
		storiesEnabled: false, // kill switch OFF de propósito
		handler: func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "refresh_access_token") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			refreshCalls++
			// A primeira conta falha na rede, a segunda renova.
			if refreshCalls == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"error":{"message":"boom"}}`)
				return
			}
			fmt.Fprint(w, `{"access_token":"NOVO","token_type":"bearer","expires_in":5183944}`)
		},
	})
	ctx := context.Background()
	soon := time.Now().Add(24 * time.Hour)
	h.seedCredential(t, userA, "APP-A", "IG-A", soon)
	h.seedCredential(t, userB, "APP-B", "IG-B", soon)
	// Credencial corrompida não pode derrubar o laço.
	h.creds.byUser["corrompida"] = &models.InstagramCredentialsDB{
		UserID: "corrompida", IGUserID: "IG-C", EncryptedData: "{{{nao-e-json",
		TokenExpiresAt: &soon,
	}

	if !h.svc.TokenMaintenanceConfigured() {
		t.Fatal("manutenção de token deveria estar configurada mesmo com o kill switch off")
	}
	if err := h.svc.RefreshExpiring(ctx); err != nil {
		t.Fatalf("RefreshExpiring não deveria falhar por conta individual: %v", err)
	}
	if refreshCalls < 2 {
		t.Errorf("o laço parou cedo: %d chamadas de refresh", refreshCalls)
	}
}

// Resposta de refresh sem token ou sem expiry não pode ser persistida.
func TestService_RefreshRejectsUnusableResponse(t *testing.T) {
	h := newServiceTest(t, svcOpts{
		storiesEnabled: true,
		handler: func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"access_token":"NOVO","token_type":"bearer","expires_in":0}`)
		},
	})
	ctx := context.Background()
	soon := time.Now().Add(24 * time.Hour)
	h.seedCredential(t, userA, "APP-A", "IG-A", soon)
	before := h.creds.saveCalls

	if err := h.svc.RefreshExpiring(ctx); err != nil {
		t.Fatal(err)
	}
	if h.creds.saveCalls != before {
		t.Error("expiry inválido não pode ser persistido (nem virar 60 dias inventados)")
	}
}

// --- desautorização e exclusão ---

func TestService_DeauthorizeRemovesCredentialByAppScopedID(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()
	h.seedCredential(t, userA, "APP-A", "IG-A", now.Add(30*24*time.Hour))

	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
	})
	if err := h.svc.Deauthorize(ctx, raw, now); err != nil {
		t.Fatalf("Deauthorize: %v", err)
	}
	if c, _ := h.creds.Get(ctx, userA); c != nil {
		t.Error("credencial deveria ter sido apagada")
	}
}

func TestService_DeauthorizeRejectsForgedSignature(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()
	h.seedCredential(t, userA, "APP-A", "IG-A", now.Add(30*24*time.Hour))

	forged := makeSignedRequest(t, "segredo-errado-de-teste-000000000", map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
	})
	if err := h.svc.Deauthorize(ctx, forged, now); err == nil {
		t.Fatal("assinatura forjada deveria ser recusada")
	}
	if h.creds.count() != 1 {
		t.Fatal("assinatura forjada APAGOU a credencial de um cliente")
	}
}

func TestService_DataDeletionReturnsReceiptAndDeletes(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()
	h.seedCredential(t, userA, "APP-A", "IG-A", now.Add(30*24*time.Hour))

	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
	})
	resp, err := h.svc.RequestDataDeletion(ctx, raw, now)
	if err != nil {
		t.Fatalf("RequestDataDeletion: %v", err)
	}
	if len(resp.ConfirmationCode) != 64 {
		t.Errorf("código deveria ter 64 hex, got %q", resp.ConfirmationCode)
	}
	if !strings.HasPrefix(resp.URL, "https://api.example/api/v1/instagram/data-deletion/status/") {
		t.Errorf("URL de status inesperada: %s", resp.URL)
	}
	// Nada de identificador da Meta nem do usuário local na resposta.
	for _, leak := range []string{"APP-A", "IG-A", userA} {
		if strings.Contains(resp.URL, leak) || strings.Contains(resp.ConfirmationCode, leak) {
			t.Errorf("resposta vaza %q", leak)
		}
	}
	if c, _ := h.creds.Get(ctx, userA); c != nil {
		t.Error("credencial deveria ter sido apagada na mesma operação")
	}

	// O status precisa reconhecer o código.
	ok, err := h.svc.DataDeletionStatus(ctx, resp.ConfirmationCode, now)
	if err != nil || !ok {
		t.Errorf("status do código emitido deveria ser encontrado: ok=%v err=%v", ok, err)
	}
}

// Repetir a solicitação do MESMO sujeito devolve o MESMO código: a Meta pode
// reconsultar o comprovante que já recebeu.
func TestService_RepeatedDataDeletionKeepsSameCode(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()
	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
	})

	first, err := h.svc.RequestDataDeletion(ctx, raw, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.svc.RequestDataDeletion(ctx, raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfirmationCode != second.ConfirmationCode {
		t.Errorf("código deveria ser estável: %q vs %q", first.ConfirmationCode, second.ConfirmationCode)
	}
}

// Sujeito desconhecido também recebe comprovante: responder diferente revelaria
// quem tem conta aqui.
func TestService_DataDeletionForUnknownSubjectStillSucceeds(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()
	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "NINGUEM", "issued_at": now.Unix(),
	})
	resp, err := h.svc.RequestDataDeletion(ctx, raw, now)
	if err != nil || resp.ConfirmationCode == "" {
		t.Fatalf("sujeito desconhecido deveria receber comprovante: %v", err)
	}
}

// Código malformado nem chega ao banco; expirado é tratado como inexistente.
func TestService_DataDeletionStatusValidatesCode(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	ctx := context.Background()
	now := time.Now()

	for _, bad := range []string{"", "curto", strings.Repeat("Z", 64), strings.Repeat("a", 63)} {
		if ok, _ := h.svc.DataDeletionStatus(ctx, bad, now); ok {
			t.Errorf("código %q não deveria ser aceito", bad)
		}
	}

	raw := makeSignedRequest(t, testAppSecret, map[string]any{
		"algorithm": "HMAC-SHA256", "user_id": "APP-A", "issued_at": now.Unix(),
	})
	resp, _ := h.svc.RequestDataDeletion(ctx, raw, now)
	// Depois da validade, o comprovante some.
	if ok, _ := h.svc.DataDeletionStatus(ctx, resp.ConfirmationCode, now.Add(retentionWindow+time.Hour)); ok {
		t.Error("comprovante expirado deveria responder como desconhecido")
	}
}

// --- retenção ---

func TestService_CleanupIsBatchedAndBounded(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	if err := h.svc.CleanupExpired(context.Background(), time.Now()); err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	h.story.mu.Lock()
	defer h.story.mu.Unlock()
	if len(h.story.deleted) == 0 {
		t.Fatal("cleanup não chamou o repositório")
	}
	if h.story.deleted[0] != retentionBatch {
		t.Errorf("primeiro lote deveria ser %d, got %d", retentionBatch, h.story.deleted[0])
	}
	if len(h.story.deleted) > retentionMaxBatches {
		t.Errorf("cleanup excedeu o teto de %d lotes: %d", retentionMaxBatches, len(h.story.deleted))
	}
}

// --- app da Meta por cliente ---

// fakeAppsRepo simula instagram_apps, incluindo o vínculo webhook_token -> app.
type fakeAppsRepo struct {
	mu     sync.Mutex
	byUser map[string]*models.InstagramApp
}

func newFakeAppsRepo() *fakeAppsRepo {
	return &fakeAppsRepo{byUser: map[string]*models.InstagramApp{}}
}

func (f *fakeAppsRepo) Get(_ context.Context, userID string) (*models.InstagramApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byUser[userID], nil
}

func (f *fakeAppsRepo) GetByWebhookToken(_ context.Context, token string) (*models.InstagramApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if token == "" {
		return nil, nil
	}
	for _, a := range f.byUser {
		if a.WebhookToken == token {
			return a, nil
		}
	}
	return nil, nil
}

func (f *fakeAppsRepo) Upsert(_ context.Context, userID, appID, enc string) (*models.InstagramApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := &models.InstagramApp{
		UserID: userID, AppID: appID, EncryptedAppSecret: enc,
		WebhookToken: "wt-" + userID, VerifyToken: "vt-" + userID,
	}
	f.byUser[userID] = a
	return a, nil
}

func (f *fakeAppsRepo) MarkVerified(_ context.Context, userID string) error { return nil }
func (f *fakeAppsRepo) Delete(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byUser, userID)
	return nil
}

var _ repository.InstagramAppRepository = (*fakeAppsRepo)(nil)

// encryptedAppSecret produz o blob que instagram_apps guarda.
func encryptedAppSecret(t *testing.T, secret string) string {
	t.Helper()
	blob, err := crypto.EncryptJSONV1(models.InstagramAppSecret{AppSecret: secret}, testEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// O ponto central da Fase 1: dois clientes com apps DIFERENTES da Meta. Cada
// URL de consentimento tem de sair com o client_id do app do respectivo dono —
// se vazar o app de um para o outro, o consentimento é pedido em nome da
// empresa errada.
func TestService_AuthorizeURLUsesEachUsersOwnMetaApp(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	apps := newFakeAppsRepo()
	h.svc.appsRepo = apps
	ctx := context.Background()

	if _, err := apps.Upsert(ctx, userA, "APP-DO-USUARIO-A", encryptedAppSecret(t, "segredo-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := apps.Upsert(ctx, userB, "APP-DO-USUARIO-B", encryptedAppSecret(t, "segredo-b")); err != nil {
		t.Fatal(err)
	}

	urlA, err := h.svc.BuildAuthorizeURL(ctx, userA)
	if err != nil {
		t.Fatalf("authorize A: %v", err)
	}
	urlB, err := h.svc.BuildAuthorizeURL(ctx, userB)
	if err != nil {
		t.Fatalf("authorize B: %v", err)
	}

	if !strings.Contains(urlA, "client_id=APP-DO-USUARIO-A") {
		t.Errorf("A deveria usar o app dele: %s", urlA)
	}
	if !strings.Contains(urlB, "client_id=APP-DO-USUARIO-B") {
		t.Errorf("B deveria usar o app dele: %s", urlB)
	}
	if strings.Contains(urlA, "APP-DO-USUARIO-B") || strings.Contains(urlB, "APP-DO-USUARIO-A") {
		t.Fatal("app de um cliente vazou para a URL do outro")
	}
}

// Sem app cadastrado e sem app global, não há como pedir consentimento — e o
// erro precisa ser tipado, para a UI dizer "cadastre seu app" em vez de um erro
// genérico.
func TestService_AuthorizeURLRequiresAnApp(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	h.svc.appsRepo = newFakeAppsRepo()
	// Zera o app global para simular produção sem INSTAGRAM_APP_ID.
	h.svc.api = NewAPIClient("", "", "https://cb.example/redirect", zap.NewNop())

	_, err := h.svc.BuildAuthorizeURL(context.Background(), userA)
	if !errors.Is(err, ErrInstagramAppNotConfigured) {
		t.Fatalf("esperava ErrInstagramAppNotConfigured, got %v", err)
	}
}

// O token da URL do webhook resolve o app ANTES de qualquer leitura de corpo —
// é o que torna possível validar assinatura num webhook multi-tenant.
func TestService_ClientByWebhookTokenResolvesTheRightApp(t *testing.T) {
	h := newServiceTest(t, svcOpts{storiesEnabled: true})
	apps := newFakeAppsRepo()
	h.svc.appsRepo = apps
	ctx := context.Background()

	appA, _ := apps.Upsert(ctx, userA, "APP-A", encryptedAppSecret(t, "segredo-a"))
	apps.Upsert(ctx, userB, "APP-B", encryptedAppSecret(t, "segredo-b"))

	client, app, err := h.svc.ClientByWebhookToken(ctx, appA.WebhookToken)
	if err != nil {
		t.Fatalf("resolver por token: %v", err)
	}
	if app.UserID != userA {
		t.Errorf("token de A resolveu para %s", app.UserID)
	}
	if client.AppSecret() != "segredo-a" {
		t.Error("segredo resolvido nao e o do dono do token")
	}

	// Token desconhecido não pode resolver para app nenhum.
	if _, _, err := h.svc.ClientByWebhookToken(ctx, "token-inventado"); !errors.Is(err, ErrInstagramAppNotConfigured) {
		t.Errorf("token desconhecido deveria falhar, got %v", err)
	}
	// Token vazio idem — é o default de linha malformada.
	if _, _, err := h.svc.ClientByWebhookToken(ctx, ""); !errors.Is(err, ErrInstagramAppNotConfigured) {
		t.Errorf("token vazio deveria falhar, got %v", err)
	}
}

// O segredo do app é guardado cifrado; o valor em claro só existe em memória.
func TestService_AppSecretIsStoredEncrypted(t *testing.T) {
	enc := encryptedAppSecret(t, "segredo-muito-secreto")
	if strings.Contains(enc, "segredo-muito-secreto") {
		t.Fatal("segredo do app gravado em claro")
	}
}
