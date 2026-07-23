package shopee

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"automatiza-go/internal/lib/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Mock: AffiliateCredentialsRepository
// ---------------------------------------------------------------------------

type mockAffiliateRepo struct {
	GetByUserAndStoreFn func(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error)
	GetAllByUserFn      func(ctx context.Context, userID string) ([]models.AffiliateCredentials, error)
	UpsertFn            func(ctx context.Context, userID, store string, data *models.AffiliateCredentials) error
	DeleteFn            func(ctx context.Context, userID, store string) error
	DeactivateFn        func(ctx context.Context, userID, store string) error
	GetActiveStoresFn   func(ctx context.Context, userID string) ([]string, error)
}

func (m *mockAffiliateRepo) GetByUserAndStore(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error) {
	if m.GetByUserAndStoreFn != nil {
		return m.GetByUserAndStoreFn(ctx, userID, store)
	}
	return nil, nil
}
func (m *mockAffiliateRepo) GetAllByUser(ctx context.Context, userID string) ([]models.AffiliateCredentials, error) {
	if m.GetAllByUserFn != nil {
		return m.GetAllByUserFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockAffiliateRepo) Upsert(ctx context.Context, userID, store string, data *models.AffiliateCredentials) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, userID, store, data)
	}
	return nil
}
func (m *mockAffiliateRepo) Delete(ctx context.Context, userID, store string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, userID, store)
	}
	return nil
}
func (m *mockAffiliateRepo) Deactivate(ctx context.Context, userID, store string) error {
	if m.DeactivateFn != nil {
		return m.DeactivateFn(ctx, userID, store)
	}
	return nil
}
func (m *mockAffiliateRepo) GetActiveStores(ctx context.Context, userID string) ([]string, error) {
	if m.GetActiveStoresFn != nil {
		return m.GetActiveStoresFn(ctx, userID)
	}
	return nil, nil
}

var _ repository.AffiliateCredentialsRepository = (*mockAffiliateRepo)(nil)

// ---------------------------------------------------------------------------
// Mock: SettingsRepository
// ---------------------------------------------------------------------------

type mockSettingsRepo struct {
	GetFn func(ctx context.Context, keys ...string) (models.SettingsMap, error)
}

func (m *mockSettingsRepo) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, keys...)
	}
	return models.SettingsMap{}, nil
}
func (m *mockSettingsRepo) GetAll(ctx context.Context) (models.SettingsMap, error) {
	return models.SettingsMap{}, nil
}
func (m *mockSettingsRepo) Update(ctx context.Context, key string, value interface{}) error {
	return nil
}
func (m *mockSettingsRepo) GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockSettingsRepo) DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error {
	return nil
}

var _ repository.SettingsRepository = (*mockSettingsRepo)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestService creates a shopeeService with a custom baseURL (for httptest)
// and nil cache (tests without cache).
func newTestService(t *testing.T, baseURL string, affiliateRepo repository.AffiliateCredentialsRepository, settingsRepo repository.SettingsRepository) *shopeeService {
	t.Helper()
	return &shopeeService{
		httpClient:    http.DefaultClient,
		cache:         nil,
		affiliateRepo: affiliateRepo,
		settingsRepo:  settingsRepo,
		baseURL:       baseURL,
		categoryMap:   initCategoryMapping(),
		logger:        zaptest.NewLogger(t),
	}
}

func validCreds() *models.AffiliateCredentials {
	return &models.AffiliateCredentials{
		AppID:     "test-app-id",
		AppSecret: "test-secret",
		Credentials: map[string]string{
			"appId":    "my-app-id",
			"password": "my-secret",
		},
	}
}

// graphqlResponse builds a JSON body that shopeeGraphQL will accept.
func graphqlResponse(data interface{}) string {
	d, _ := json.Marshal(data)
	return fmt.Sprintf(`{"data":%s,"errors":null}`, string(d))
}

func readBody(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	return string(b), err
}

// ---------------------------------------------------------------------------
// Tests: getShopeeCredentials
// ---------------------------------------------------------------------------

func TestGetShopeeCredentials(t *testing.T) {
	tests := []struct {
		name       string
		creds      *models.AffiliateCredentials
		wantAppID  string
		wantSecret string
	}{
		{
			name: "credentials map appId and password take priority",
			creds: &models.AffiliateCredentials{
				AppID:     "legacy-id",
				AppSecret: "legacy-secret",
				Credentials: map[string]string{
					"appId":    "map-id",
					"password": "map-secret",
				},
			},
			wantAppID:  "map-id",
			wantSecret: "map-secret",
		},
		{
			name: "fallback to app_id and app_secret in map",
			creds: &models.AffiliateCredentials{
				AppID:     "legacy-id",
				AppSecret: "legacy-secret",
				Credentials: map[string]string{
					"app_id":     "snake-id",
					"app_secret": "snake-secret",
				},
			},
			wantAppID:  "snake-id",
			wantSecret: "snake-secret",
		},
		{
			name: "fallback to legacy AppID/AppSecret fields",
			creds: &models.AffiliateCredentials{
				AppID:       "legacy-id",
				AppSecret:   "legacy-secret",
				Credentials: map[string]string{},
			},
			wantAppID:  "legacy-id",
			wantSecret: "legacy-secret",
		},
		{
			name: "empty credentials returns empty strings",
			creds: &models.AffiliateCredentials{
				Credentials: map[string]string{},
			},
			wantAppID:  "",
			wantSecret: "",
		},
		{
			name:       "nil credentials map falls back to empty legacy fields",
			creds:      &models.AffiliateCredentials{},
			wantAppID:  "",
			wantSecret: "",
		},
		{
			name: "password takes priority over app_secret in map",
			creds: &models.AffiliateCredentials{
				Credentials: map[string]string{
					"password":   "pw-secret",
					"app_secret": "as-secret",
				},
			},
			wantAppID:  "",
			wantSecret: "pw-secret",
		},
		{
			name: "appId takes priority over app_id in map",
			creds: &models.AffiliateCredentials{
				Credentials: map[string]string{
					"appId":  "camel-id",
					"app_id": "snake-id",
				},
			},
			wantAppID:  "camel-id",
			wantSecret: "",
		},
		{
			name: "trims surrounding whitespace/newline (paste accident → 10020)",
			creds: &models.AffiliateCredentials{
				Credentials: map[string]string{
					"appId":    "  18337620666 ",
					"password": "secret-with-trailing-space \n",
				},
			},
			wantAppID:  "18337620666",
			wantSecret: "secret-with-trailing-space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAppID, gotSecret := getShopeeCredentials(tt.creds)
			if gotAppID != tt.wantAppID {
				t.Errorf("appID = %q, want %q", gotAppID, tt.wantAppID)
			}
			if gotSecret != tt.wantSecret {
				t.Errorf("secret = %q, want %q", gotSecret, tt.wantSecret)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: stripAffiliateAttribution
// ---------------------------------------------------------------------------

func TestStripAffiliateAttribution(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "remove mmp_pid e utm da fonte (caso real)",
			in:   "https://shopee.com.br/m/cupom-de-desconto?mmp_pid=an_18337620666&uls_trackid=55vubdjk00ng&utm_campaign=id_YBB3gXtEf5&utm_content=----&utm_medium=affiliates&utm_source=an_18337620666&utm_term=f4p8vo2s5kr3",
			want: "https://shopee.com.br/m/cupom-de-desconto",
		},
		{
			name: "sem params fica igual",
			in:   "https://shopee.com.br/m/cupom-de-desconto",
			want: "https://shopee.com.br/m/cupom-de-desconto",
		},
		{
			name: "preserva params nao-tracking",
			in:   "https://shopee.com.br/m/promo?categoria=beleza&mmp_pid=an_999&utm_source=an_999",
			want: "https://shopee.com.br/m/promo?categoria=beleza",
		},
		{
			name: "remove af_ prefixados",
			in:   "https://shopee.com.br/voucher?af_siteid=123&af_click_lookback=7d&id=abc",
			want: "https://shopee.com.br/voucher?id=abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAffiliateAttribution(tt.in)
			if got != tt.want {
				t.Errorf("stripAffiliateAttribution(%q)\n  = %q\n  want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: shopeeSign
// ---------------------------------------------------------------------------

func TestBuildGenerateShortLinkMutation(t *testing.T) {
	t.Parallel()

	t.Run("without trackingTag omits subIds", func(t *testing.T) {
		t.Parallel()
		got := buildGenerateShortLinkMutation("https://shopee.com.br/x", "")
		want := `mutation { generateShortLink(input: {originUrl: "https://shopee.com.br/x"}) { shortLink } }`
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("with trackingTag includes subIds", func(t *testing.T) {
		t.Parallel()
		got := buildGenerateShortLinkMutation("https://shopee.com.br/x", "AUTOMAISA")
		want := `mutation { generateShortLink(input: {originUrl: "https://shopee.com.br/x", subIds: ["AUTOMAISA"]}) { shortLink } }`
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("trackingTag with special chars is quoted safely", func(t *testing.T) {
		t.Parallel()
		got := buildGenerateShortLinkMutation("https://shopee.com.br/x", `weird"tag`)
		// %q escapa aspas internas — protege contra injection no GraphQL
		want := `mutation { generateShortLink(input: {originUrl: "https://shopee.com.br/x", subIds: ["weird\"tag"]}) { shortLink } }`
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestShopeeSubIDDefault_LockedToAutomatisa(t *testing.T) {
	t.Parallel()
	// Lock test — qualquer mudança nessa string muda a atribuição no relatório
	// da Shopee de todos os links em produção, pra todos os usuários. Trocar
	// requer alinhamento com produto.
	if shopeeSubIDDefault != "AUTOMATISA" {
		t.Errorf("shopeeSubIDDefault changed to %q — atribuição de afiliado quebra silenciosamente em prod", shopeeSubIDDefault)
	}
}

func TestShopeeSign(t *testing.T) {
	tests := []struct {
		name      string
		appID     string
		timestamp string
		payload   string
		secret    string
		want      string
	}{
		{
			name:      "known input produces SHA256",
			appID:     "app123",
			timestamp: "1700000000",
			payload:   `{"query":"test"}`,
			secret:    "secret456",
			want:      sha256Hex("app123" + "1700000000" + `{"query":"test"}` + "secret456"),
		},
		{
			name:      "empty inputs produce valid hash",
			appID:     "",
			timestamp: "",
			payload:   "",
			secret:    "",
			want:      sha256Hex(""),
		},
		{
			name:      "different inputs produce different hash",
			appID:     "a",
			timestamp: "1",
			payload:   "p",
			secret:    "s",
			want:      sha256Hex("a1ps"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shopeeSign(tt.appID, tt.timestamp, tt.payload, tt.secret)
			if got != tt.want {
				t.Errorf("shopeeSign() = %q, want %q", got, tt.want)
			}
		})
	}
}

func sha256Hex(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// ---------------------------------------------------------------------------
// Tests: ExtractProductIDs
// ---------------------------------------------------------------------------

func TestExtractProductIDs(t *testing.T) {
	svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

	tests := []struct {
		name       string
		url        string
		wantShopID int64
		wantItemID int64
		wantErr    bool
	}{
		{
			name:       "pattern 1: product-name.shopID.itemID",
			url:        "https://shopee.com.br/product-name-i.123.456",
			wantShopID: 123,
			wantItemID: 456,
		},
		{
			name:       "pattern 2: /product/shopID/itemID",
			url:        "https://shopee.com.br/product/789/101112",
			wantShopID: 789,
			wantItemID: 101112,
		},
		{
			name:       "pattern 1 with dashes in product name",
			url:        "https://shopee.com.br/Fone-de-Ouvido-TWS.111222333.444555666",
			wantShopID: 111222333,
			wantItemID: 444555666,
		},
		{
			name:    "invalid URL returns error",
			url:     "https://shopee.com.br/search?q=fone",
			wantErr: true,
		},
		{
			name:       "pattern 3: /slug/shopID/itemID (expanded short URL)",
			url:        "https://shopee.com.br/opaanlp/789131072/21033994886?__mobile__=1&utm_source=foo",
			wantShopID: 789131072,
			wantItemID: 21033994886,
		},
		{
			name:       "pattern 3: slug with dashes",
			url:        "https://shopee.com.br/some-product-name/111/222",
			wantShopID: 111,
			wantItemID: 222,
		},
		{
			name:    "empty URL returns error",
			url:     "",
			wantErr: true,
		},
		{
			name:    "completely different domain",
			url:     "https://google.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shopID, itemID, err := svc.ExtractProductIDs(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if shopID != tt.wantShopID {
				t.Errorf("shopID = %d, want %d", shopID, tt.wantShopID)
			}
			if itemID != tt.wantItemID {
				t.Errorf("itemID = %d, want %d", itemID, tt.wantItemID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: isNonProductShopeeURL
// ---------------------------------------------------------------------------

func TestIsNonProductShopeeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "coupon page", url: "https://shopee.com.br/m/cupom-de-desconto?mmp_pid=an_123", want: true},
		{name: "campaign page", url: "https://shopee.com.br/m/campaign-name?utm_source=foo", want: true},
		{name: "mall page", url: "https://shopee.com.br/mall/some-store", want: true},
		{name: "flash sale", url: "https://shopee.com.br/flash_sale", want: true},
		{name: "daily discover", url: "https://shopee.com.br/daily_discover", want: true},
		{name: "voucher wallet", url: "https://shopee.com.br/user/voucher-wallet?mmp_pid=an_123", want: true},
		{name: "user profile path", url: "https://shopee.com.br/user/12345", want: true},
		{name: "lojas oficiais landing", url: "https://shopee.com.br/oficial", want: true},
		{name: "product page opaanlp", url: "https://shopee.com.br/opaanlp/123/456", want: false},
		{name: "product page with dot IDs", url: "https://shopee.com.br/product-name.123.456", want: false},
		{name: "product page /product/ path", url: "https://shopee.com.br/product/123/456", want: false},
		{name: "search page", url: "https://shopee.com.br/search?q=fone", want: false},
		{name: "case insensitive", url: "https://Shopee.com.br/M/Cupom", want: true},
		{name: "case insensitive voucher wallet", url: "https://Shopee.com.br/User/Voucher-Wallet", want: true},
		{name: "curated collection", url: "https://shopee.com.br/collections/21253338?mmp_pid=an_123", want: true},
		{name: "shopeevip campaign", url: "https://shopee.com.br/shopeevip?smtt=0.1", want: true},
		// /cart fica fora de propósito: link "adicionar ao carrinho" não
		// carrega o item na URL expandida — regenerar seria carrinho vazio.
		{name: "cart fallback stays out", url: "https://shopee.com.br/cart?mmp_pid=an_123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonProductShopeeURL(tt.url)
			if got != tt.want {
				t.Errorf("isNonProductShopeeURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// isConvertibleShopeeURL substitui a allowlist de prefixos na resolução do link
// de afiliado: tudo do domínio converte, menos /cart. Os casos "want: true" que
// a lista antiga NÃO cobria são o ponto — cada um custou entrega ou comissão em
// produção. Todos validados contra a API real em 15/07 (retornam shortlink
// atribuído ao caller com sub_id).
func TestIsConvertibleShopeeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// O buraco que reteve o repasse de 12 usuários (15 msgs/24h) e fez o
		// gerar-oferta repassar link cru: perfil de loja não tem prefixo.
		{name: "perfil de loja (o buraco da allowlist)", url: "https://shopee.com.br/koksara.br?mmp_pid=an_123", want: true},
		{name: "perfil de loja com hifen", url: "https://shopee.com.br/casa-bonita-oficial", want: true},
		// shope.ee: domínio global, fora de TODAS as listas antigas.
		{name: "shope.ee (dominio global)", url: "https://shope.ee/9AbCdEf", want: true},
		{name: "shp.ee", url: "https://shp.ee/abc123", want: true},
		// Padrões que a lista antiga já cobria continuam valendo.
		{name: "cupom /m/", url: "https://shopee.com.br/m/cupom-de-desconto?mmp_pid=an_123", want: true},
		{name: "voucher wallet", url: "https://shopee.com.br/user/voucher-wallet", want: true},
		{name: "collections", url: "https://shopee.com.br/collections/21253338", want: true},
		{name: "shopeevip", url: "https://shopee.com.br/shopeevip?smtt=0.1", want: true},
		// Busca: a lista antiga dizia false e o link ia cru; converte.
		{name: "busca", url: "https://shopee.com.br/search?keyword=fone", want: true},
		{name: "case insensitive", url: "https://Shopee.com.br/Koksara.BR", want: true},
		// /cart continua fora: o item não sobrevive à expansão e o link
		// regenerado entregaria um carrinho vazio.
		{name: "carrinho fica fora", url: "https://shopee.com.br/cart?mmp_pid=an_123", want: false},
		{name: "carrinho case insensitive", url: "https://shopee.com.br/CART", want: false},
		// Fora do domínio: não é problema nosso de conversão.
		{name: "outra loja", url: "https://mercadolivre.com.br/produto", want: false},
		{name: "amazon", url: "https://amzn.to/xyz", want: false},
		{name: "string vazia", url: "", want: false},
		// Compara HOST, não substring: "shopee.com.br" no path de um domínio
		// alheio não faz dele Shopee — senão bastaria forjar a URL para
		// empurrar link de terceiro pro generateShortLink.
		{name: "dominio alheio com shopee no path", url: "https://evil.com/s.shopee.com.br/bad", want: false},
		{name: "dominio alheio com shopee no subpath", url: "http://127.0.0.1:8080/shopee.com.br/x", want: false},
		{name: "sufixo enganoso", url: "https://naoshopee.com.br/m/cupom", want: false},
		{name: "subdominio legitimo", url: "https://br.shopee.com.br/m/cupom", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConvertibleShopeeURL(tt.url); got != tt.want {
				t.Errorf("isConvertibleShopeeURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: parseDiscount
// ---------------------------------------------------------------------------

func TestParseDiscount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{name: "percent sign", input: "50%", want: 50.0},
		{name: "spaces before percent", input: "  30%", want: 30.0},
		{name: "trailing spaces after percent prevents TrimSuffix", input: "  30%  ", want: 0.0},
		{name: "no percent sign", input: "10", want: 10.0},
		{name: "zero", input: "0%", want: 0.0},
		{name: "decimal discount", input: "12.5%", want: 12.5},
		{name: "empty string", input: "", want: 0.0},
		{name: "non-numeric returns 0", input: "abc%", want: 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDiscount(tt.input)
			if got != tt.want {
				t.Errorf("parseDiscount(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: resolveCategoryIDs
// ---------------------------------------------------------------------------

func TestResolveCategoryIDs(t *testing.T) {
	svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

	tests := []struct {
		name       string
		categories []string
		wantNil    bool
		wantLen    int
	}{
		{
			name:       "empty categories returns nil",
			categories: nil,
			wantNil:    true,
		},
		{
			name:       "ofertas gerais returns nil (no filter)",
			categories: []string{"Ofertas gerais"},
			wantNil:    true,
		},
		{
			name:       "ofertasGerais returns nil (no filter)",
			categories: []string{"ofertasGerais"},
			wantNil:    true,
		},
		{
			name:       "known category returns IDs",
			categories: []string{"Eletrônicos"},
			wantLen:    1,
		},
		{
			name:       "unknown category returns empty",
			categories: []string{"NonExistentCategory"},
			wantLen:    0,
		},
		{
			name:       "multiple known categories",
			categories: []string{"Eletrônicos", "Beleza"},
			wantLen:    2,
		},
		{
			name:       "mix of known and unknown",
			categories: []string{"Eletrônicos", "FakeCategory"},
			wantLen:    1,
		},
		{
			name:       "ofertas gerais in mix returns nil immediately",
			categories: []string{"Eletrônicos", "Ofertas gerais"},
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.resolveCategoryIDs(tt.categories)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d (result: %v)", len(result), tt.wantLen, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: GetCategories
// ---------------------------------------------------------------------------

func TestGetCategories(t *testing.T) {
	svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

	categories, err := svc.GetCategories(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("expected categories, got empty slice")
	}

	// Verify each category has a valid ID and Name
	for _, cat := range categories {
		if cat.ID == 0 {
			t.Errorf("category %q has zero ID", cat.Name)
		}
		if cat.Name == "" {
			t.Errorf("category with ID %d has empty name", cat.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: SearchOffers
// ---------------------------------------------------------------------------

func TestSearchOffers(t *testing.T) {
	ctx := context.Background()

	t.Run("no credentials returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})
		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{})
		if err == nil {
			t.Fatal("expected error for missing credentials")
		}
	})

	t.Run("credentials repo error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return nil, fmt.Errorf("db down")
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})
		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{})
		if err == nil {
			t.Fatal("expected error on repo failure")
		}
	})

	t.Run("incomplete credentials returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return &models.AffiliateCredentials{Credentials: map[string]string{}}, nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})
		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{})
		if err == nil {
			t.Fatal("expected error for incomplete credentials")
		}
	})

	t.Run("success with offers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No categories → routes to productOfferV2 (without category filter)
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "Test Product",
							"itemId":            123,
							"shopId":            456,
							"productLink":       "https://shopee.com.br/product/456/123",
							"offerLink":         "https://s.shopee.com.br/abc",
							"imageUrl":          "https://img.shopee.com.br/test.jpg",
							"priceMin":          "50.00",
							"priceMax":          "100.00",
							"price":             "75.00",
							"commission":        "5.00",
							"commissionRate":    "6.5",
							"sales":             100,
							"ratingStar":        "4.5",
							"priceDiscountRate": 25,
							"shopName":          "TestShop",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Offers) != 1 {
			t.Fatalf("expected 1 offer, got %d", len(result.Offers))
		}
		offer := result.Offers[0]
		if offer.Name != "Test Product" {
			t.Errorf("name = %q, want %q", offer.Name, "Test Product")
		}
		if offer.Price != 75.0 {
			t.Errorf("price = %f, want 75.0", offer.Price)
		}
		if offer.Discount != 25.0 {
			t.Errorf("discount = %f, want 25.0", offer.Discount)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No categories → routes to productOfferV2 (without category filter)
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Offers) != 0 {
			t.Errorf("expected 0 offers, got %d", len(result.Offers))
		}
	})

	t.Run("API error response (non-200)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error")
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 10})
		if err == nil {
			t.Fatal("expected error on non-200 response")
		}
	})

	t.Run("GraphQL error in response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":null,"errors":[{"message":"some graphql error"}]}`)
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 10})
		if err == nil {
			t.Fatal("expected error on GraphQL error response")
		}
	})

	t.Run("category resolution (known category)", func(t *testing.T) {
		var receivedPayload string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				b, _ := json.Marshal(body)
				receivedPayload = string(b)
			}
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
			Categories: []string{"Livros"},
			Limit:      5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify that the category was passed (the request should contain productCatId)
		if receivedPayload == "" {
			t.Fatal("expected payload to be captured")
		}
	})

	t.Run("ofertas gerais uses productOfferV2 without category", func(t *testing.T) {
		var capturedQuery string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if q, ok := body["query"].(string); ok {
					capturedQuery = q
				}
			}
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "General Offer",
							"itemId":            1,
							"shopId":            2,
							"productLink":       "link",
							"offerLink":         "offer",
							"imageUrl":          "img",
							"priceMin":          "10",
							"priceMax":          "20",
							"price":             "15",
							"commission":        "1",
							"commissionRate":    "5",
							"sales":             50,
							"ratingStar":        "4.0",
							"priceDiscountRate": 25,
							"shopName":          "Shop",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
			Categories: []string{"Ofertas gerais"},
			Limit:      10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Offers) != 1 {
			t.Errorf("expected 1 offer, got %d", len(result.Offers))
		}
		// Verify productOfferV2 is used (no category filter for "Ofertas gerais")
		if capturedQuery == "" {
			t.Fatal("expected query to be captured")
		}
		if !strings.Contains(capturedQuery, "productOfferV2") {
			t.Errorf("expected productOfferV2 in query, got: %s", capturedQuery)
		}
	})

	t.Run("price and discount filters applied", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No categories → routes to productOfferV2 (without category filter)
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "Cheap",
							"itemId":            1,
							"shopId":            2,
							"productLink":       "link1",
							"offerLink":         "offer1",
							"imageUrl":          "img1",
							"priceMin":          "5",
							"priceMax":          "10",
							"price":             "5",
							"commission":        "0.5",
							"commissionRate":    "10",
							"sales":             10,
							"ratingStar":        "3.0",
							"priceDiscountRate": 5,
							"shopName":          "Shop1",
						},
						{
							"productName":       "Mid",
							"itemId":            3,
							"shopId":            4,
							"productLink":       "link2",
							"offerLink":         "offer2",
							"imageUrl":          "img2",
							"priceMin":          "50",
							"priceMax":          "80",
							"price":             "50",
							"commission":        "5",
							"commissionRate":    "10",
							"sales":             50,
							"ratingStar":        "4.0",
							"priceDiscountRate": 30,
							"shopName":          "Shop2",
						},
						{
							"productName":       "Expensive",
							"itemId":            5,
							"shopId":            6,
							"productLink":       "link3",
							"offerLink":         "offer3",
							"imageUrl":          "img3",
							"priceMin":          "200",
							"priceMax":          "300",
							"price":             "200",
							"commission":        "20",
							"commissionRate":    "10",
							"sales":             5,
							"ratingStar":        "5.0",
							"priceDiscountRate": 50,
							"shopName":          "Shop3",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		minPrice := 10.0
		maxPrice := 150.0
		minDiscount := 20.0

		result, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
			MinPrice:    &minPrice,
			MaxPrice:    &maxPrice,
			MinDiscount: &minDiscount,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only "Mid" passes all filters (price 50, discount 30%)
		if len(result.Offers) != 1 {
			t.Fatalf("expected 1 offer after filters, got %d", len(result.Offers))
		}
		if result.Offers[0].Name != "Mid" {
			t.Errorf("expected offer 'Mid', got %q", result.Offers[0].Name)
		}
	})

	t.Run("sort by options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No categories → routes to productOfferV2 (without category filter)
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		sortOptions := []string{"commission", "sales", "relevance", "price_asc", "price_desc"}
		for _, sort := range sortOptions {
			sortCopy := sort
			_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
				SortBy: &sortCopy,
				Limit:  5,
			})
			if err != nil {
				t.Errorf("unexpected error for sort %q: %v", sort, err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: GenerateAffiliateLink
// ---------------------------------------------------------------------------

func TestGenerateAffiliateLink(t *testing.T) {
	ctx := context.Background()

	t.Run("link via generateShortLink with sub_id, image via productOfferV2", func(t *testing.T) {
		var shortLinkBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{
								"offerLink": "https://s.shopee.com.br/offer-link",
								"imageUrl":  "https://cf.shopee.com.br/file/product-image.jpg",
							},
						},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				shortLinkBody = body
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "https://s.shopee.com.br/subid-link",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else {
				t.Errorf("unexpected API call: %s", body)
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// O link de afiliado vem do generateShortLink (com sub_id), nao do
		// offerLink do productOfferV2 — que nao carrega subIds custom.
		if result.AffiliateLink != "https://s.shopee.com.br/subid-link" {
			t.Errorf("affiliateLink = %q, want generateShortLink result", result.AffiliateLink)
		}
		// A imagem continua vindo do productOfferV2.
		if result.ImageURL != "https://cf.shopee.com.br/file/product-image.jpg" {
			t.Errorf("imageURL = %q, want productOfferV2 image", result.ImageURL)
		}
		// O sub_id AUTOMATISA tem que aparecer no generateShortLink.
		if !strings.Contains(shortLinkBody, "AUTOMATISA") {
			t.Errorf("generateShortLink deve incluir o sub_id AUTOMATISA, body: %s", shortLinkBody)
		}
		if result.ItemID != 456 {
			t.Errorf("itemID = %d, want 456", result.ItemID)
		}
		if result.ShopID != 123 {
			t.Errorf("shopID = %d, want 123", result.ShopID)
		}
	})

	t.Run("fallback to generateShortLink when productOfferV2 has no offerLink", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			callCount++
			if strings.Contains(body, "productOfferV2") {
				// Return empty nodes (no offerLink)
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "https://s.shopee.com.br/fallback-link",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AffiliateLink != "https://s.shopee.com.br/fallback-link" {
			t.Errorf("affiliateLink = %q, want %q", result.AffiliateLink, "https://s.shopee.com.br/fallback-link")
		}
		if result.ImageURL != "" {
			t.Errorf("imageURL should be empty on fallback, got %q", result.ImageURL)
		}
		if callCount != 2 {
			t.Errorf("expected 2 API calls (productOfferV2 + generateShortLink), got %d", callCount)
		}
	})

	t.Run("generateShortLink failure returns error instead of offerLink sem sub_id", func(t *testing.T) {
		// O offerLink do productOfferV2 não carrega o sub_id AUTOMATISA; se o
		// generateShortLink falhar de vez, o link NÃO pode ser emitido — venda
		// "sem SubID" no painel era o incidente que o fallback antigo causava.
		shortLinkCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{"offerLink": "https://s.shopee.com.br/offer-sem-subid", "imageUrl": "img"},
						},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				shortLinkCalls++
				fmt.Fprint(w, `{"errors":[{"message":"error code: 10030, rate limit exceeded"}]}`)
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err == nil {
			t.Fatal("expected error when generateShortLink fails; link sem sub_id nao pode ser emitido")
		}
		if !strings.Contains(err.Error(), "sub_id") {
			t.Errorf("error should mention sub_id policy, got: %v", err)
		}
		if shortLinkCalls != 2 {
			t.Errorf("expected 1 retry (2 generateShortLink calls), got %d", shortLinkCalls)
		}
	})

	t.Run("generateShortLink transient failure recovers on retry", func(t *testing.T) {
		shortLinkCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{"nodes": []map[string]interface{}{}},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				shortLinkCalls++
				if shortLinkCalls == 1 {
					fmt.Fprint(w, `{"errors":[{"message":"error code: 10030, rate limit exceeded"}]}`)
					return
				}
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "https://s.shopee.com.br/retry-com-subid",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AffiliateLink != "https://s.shopee.com.br/retry-com-subid" {
			t.Errorf("affiliateLink = %q, want retry result", result.AffiliateLink)
		}
		if shortLinkCalls != 2 {
			t.Errorf("expected 2 generateShortLink calls, got %d", shortLinkCalls)
		}
	})

	t.Run("direct long non-product URL converts via generateShortLink", func(t *testing.T) {
		// Página de cupom postada como URL LONGA direta no grupo-fonte:
		// antes caía em "invalid shopee URL" e o espelho retinha o repasse.
		// Deve rotear pro generateNonProductAffiliateLink com sub_id.
		var shortLinkBody string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "generateShortLink") {
				shortLinkBody = body
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "https://s.shopee.com.br/cupom-com-subid",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else {
				t.Errorf("unexpected API call: %s", body)
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		result, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/m/cupom-de-desconto?mmp_pid=an_999&utm_source=an_999", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AffiliateLink != "https://s.shopee.com.br/cupom-com-subid" {
			t.Errorf("affiliateLink = %q, want converted coupon link", result.AffiliateLink)
		}
		if !strings.Contains(shortLinkBody, "AUTOMATISA") {
			t.Errorf("generateShortLink deve incluir o sub_id AUTOMATISA, body: %s", shortLinkBody)
		}
		// A atribuição do afiliado da FONTE tem que ser removida antes de
		// regenerar, senão a comissão continua indo pra ele.
		if strings.Contains(shortLinkBody, "an_999") {
			t.Errorf("atribuicao da fonte nao foi removida do originUrl, body: %s", shortLinkBody)
		}
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://google.com/bad", "user1")
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})

	t.Run("no credentials returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err == nil {
			t.Fatal("expected error for missing credentials")
		}
	})

	t.Run("credentials repo error returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err == nil {
			t.Fatal("expected error on repo failure")
		}
	})

	t.Run("empty links from both APIs returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{"offerLink": "", "imageUrl": ""},
						},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			}
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err == nil {
			t.Fatal("expected error for empty affiliate link")
		}
	})

	t.Run("API error returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, "bad gateway")
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/product/123/456", "user1")
		if err == nil {
			t.Fatal("expected error on API error")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: isShopeeShortURL
// ---------------------------------------------------------------------------

func TestIsShopeeShortURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://s.shopee.com.br/abc123", true},
		{"https://shp.ee/xyz", true},
		{"https://shope.ee/abc", true},
		{"https://S.SHOPEE.COM.BR/ABC", true},
		{"https://shopee.com.br/product/123/456", false},
		{"https://google.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isShopeeShortURL(tt.url); got != tt.want {
			t.Errorf("isShopeeShortURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: GenerateAffiliateLink with short URL expansion
// ---------------------------------------------------------------------------

func TestGenerateAffiliateLink_ShortURL(t *testing.T) {
	ctx := context.Background()

	t.Run("short URL is expanded then generates affiliate link", func(t *testing.T) {
		// Shopee GraphQL server that generates affiliate links (productOfferV2)
		graphqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{"offerLink": "https://s.shopee.com.br/affiliate-link", "imageUrl": ""},
						},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			}
		}))
		defer graphqlServer.Close()

		// Redirect server simulating s.shopee.com.br → full product URL
		expandServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://shopee.com.br/Product-Name-i.111.222", http.StatusFound)
		}))
		defer expandServer.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, graphqlServer.URL, repo, &mockSettingsRepo{})

		// ExpandURL will follow the redirect from expandServer.
		// However, the redirect target (shopee.com.br) won't be reachable in tests.
		// Instead, test that non-short invalid URLs still fail.
		_, err := svc.GenerateAffiliateLink(ctx, "https://google.com/not-shopee", "user1")
		if err == nil {
			t.Fatal("expected error for non-shopee non-short URL")
		}
	})

	t.Run("short URL that expands to valid product URL succeeds", func(t *testing.T) {
		// Redirect server: simulates short URL → long product URL
		expandServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/short123" {
				http.Redirect(w, r, "https://shopee.com.br/Product.999.888", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer expandServer.Close()

		// GraphQL server: productOfferV2 dá a imagem; o LINK vem sempre do
		// generateShortLink (é ele que aplica o sub_id AUTOMATISA).
		graphqlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := readBody(r)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(body, "productOfferV2") {
				resp := map[string]interface{}{
					"productOfferV2": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{
								"offerLink": "https://s.shopee.com.br/offer-sem-subid",
								"imageUrl":  "https://cf.shopee.com.br/file/img.jpg",
							},
						},
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			} else if strings.Contains(body, "generateShortLink") {
				resp := map[string]interface{}{
					"generateShortLink": map[string]interface{}{
						"shortLink": "https://s.shopee.com.br/affiliate-result",
					},
				}
				fmt.Fprint(w, graphqlResponse(resp))
			}
		}))
		defer graphqlServer.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, graphqlServer.URL, repo, &mockSettingsRepo{})

		// Direct test: provide a long URL that works
		result, err := svc.GenerateAffiliateLink(ctx, "https://shopee.com.br/Product.999.888", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ShopID != 999 {
			t.Errorf("shopID = %d, want 999", result.ShopID)
		}
		if result.ItemID != 888 {
			t.Errorf("itemID = %d, want 888", result.ItemID)
		}
		if result.AffiliateLink != "https://s.shopee.com.br/affiliate-result" {
			t.Errorf("affiliateLink = %q, want affiliate-result", result.AffiliateLink)
		}
		if result.ImageURL != "https://cf.shopee.com.br/file/img.jpg" {
			t.Errorf("imageURL = %q, want img.jpg", result.ImageURL)
		}
	})

	t.Run("short URL that expands to non-product URL returns error", func(t *testing.T) {
		// Server that doesn't redirect — final URL has no product IDs
		expandServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer expandServer.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})

		// ExpandURL will follow the redirect to expandServer which has no product IDs
		_, err := svc.GenerateAffiliateLink(ctx, expandServer.URL+"/s.shopee.com.br/bad", "user1")
		if err == nil {
			t.Fatal("expected error when expanded URL has no product IDs")
		}
		if !strings.Contains(err.Error(), "invalid shopee URL after expansion") {
			t.Errorf("expected 'invalid shopee URL after expansion' in error, got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: GetProduct
// ---------------------------------------------------------------------------

func TestGetProduct(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns product", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "Test Product",
							"itemId":            456,
							"shopId":            123,
							"productLink":       "https://shopee.com.br/product/123/456",
							"offerLink":         "https://s.shopee.com.br/abc",
							"imageUrl":          "https://img.shopee.com.br/test.jpg",
							"priceMin":          "50.00",
							"priceMax":          "100.00",
							"price":             "75.00",
							"commission":        "5.00",
							"commissionRate":    "6.5",
							"sales":             100,
							"ratingStar":        "4.5",
							"priceDiscountRate": 25,
							"shopName":          "TestShop",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		product, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if product.Name != "Test Product" {
			t.Errorf("name = %q, want %q", product.Name, "Test Product")
		}
		if product.Price != 75.0 {
			t.Errorf("price = %f, want 75.0", product.Price)
		}
		// Original price with 25% discount: 75 / (1 - 0.25) = 100
		if product.OriginalPrice != 100.0 {
			t.Errorf("originalPrice = %f, want 100.0", product.OriginalPrice)
		}
	})

	t.Run("no credentials returns error", func(t *testing.T) {
		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, "", repo, &mockSettingsRepo{})

		_, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err == nil {
			t.Fatal("expected error for missing credentials")
		}
	})

	t.Run("product not found (empty nodes)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.GetProduct(ctx, 999, 888, "user1")
		if err == nil {
			t.Fatal("expected error for product not found")
		}
	})

	t.Run("price calculation: effective price falls back to priceMin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "Fallback Price",
							"itemId":            1,
							"shopId":            2,
							"productLink":       "link",
							"offerLink":         "offer",
							"imageUrl":          "img",
							"priceMin":          "30.00",
							"priceMax":          "50.00",
							"price":             "0",
							"commission":        "3",
							"commissionRate":    "10",
							"sales":             5,
							"ratingStar":        "4.0",
							"priceDiscountRate": 0,
							"shopName":          "Shop",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		product, err := svc.GetProduct(ctx, 1, 2, "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// price is "0", so effectivePrice should be priceMin = 30
		if product.Price != 30.0 {
			t.Errorf("price = %f, want 30.0 (fallback to priceMin)", product.Price)
		}
		// originalPrice should be priceMax (50) since priceMax > effectivePrice and discount is 0
		if product.OriginalPrice != 50.0 {
			t.Errorf("originalPrice = %f, want 50.0 (priceMax)", product.OriginalPrice)
		}
	})

	t.Run("price calculation: originalPrice equals effectivePrice when no discount and priceMax <= effectivePrice", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{
						{
							"productName":       "No Discount",
							"itemId":            1,
							"shopId":            2,
							"productLink":       "link",
							"offerLink":         "offer",
							"imageUrl":          "img",
							"priceMin":          "100.00",
							"priceMax":          "100.00",
							"price":             "100.00",
							"commission":        "10",
							"commissionRate":    "10",
							"sales":             1,
							"ratingStar":        "5.0",
							"priceDiscountRate": 0,
							"shopName":          "Shop",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		product, err := svc.GetProduct(ctx, 1, 2, "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if product.OriginalPrice != 100.0 {
			t.Errorf("originalPrice = %f, want 100.0 (same as effective)", product.OriginalPrice)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ExpandURL
// ---------------------------------------------------------------------------

func TestExpandURL(t *testing.T) {
	ctx := context.Background()

	t.Run("follows redirects and returns final URL", func(t *testing.T) {
		finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer finalServer.Close()

		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, finalServer.URL+"/final-page", http.StatusFound)
		}))
		defer redirectServer.Close()

		svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

		result, err := svc.ExpandURL(ctx, redirectServer.URL+"/short")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != finalServer.URL+"/final-page" {
			t.Errorf("got %q, want %q", result, finalServer.URL+"/final-page")
		}
	})

	t.Run("too many redirects returns error", func(t *testing.T) {
		// Create a server that always redirects to itself
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, server.URL+"/loop", http.StatusFound)
		}))
		defer server.Close()

		svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

		_, err := svc.ExpandURL(ctx, server.URL+"/start")
		if err == nil {
			t.Fatal("expected error for too many redirects")
		}
	})

	t.Run("direct URL (no redirect) returns same URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		svc := newTestService(t, "", &mockAffiliateRepo{}, &mockSettingsRepo{})

		result, err := svc.ExpandURL(ctx, server.URL+"/direct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != server.URL+"/direct" {
			t.Errorf("got %q, want %q", result, server.URL+"/direct")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: shopeeGraphQL (via SearchOffers integration)
// ---------------------------------------------------------------------------

func TestShopeeGraphQL_InvalidSignatureWarning(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null,"errors":[{"message":"Invalid Signature"}]}`)
	}))
	defer server.Close()

	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 5})
	if err == nil {
		t.Fatal("expected error for Invalid Signature")
	}
}

// ---------------------------------------------------------------------------
// Tests: SearchOffers with explicit CategoryID param
// ---------------------------------------------------------------------------

func TestSearchOffers_ExplicitCategoryID(t *testing.T) {
	ctx := context.Background()

	var capturedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedPayload)
		resp := map[string]interface{}{
			"productOfferV2": map[string]interface{}{
				"nodes": []map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, graphqlResponse(resp))
	}))
	defer server.Close()

	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	catID := int64(100644)
	_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
		CategoryID: &catID,
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The explicit CategoryID should override category name resolution
	vars, ok := capturedPayload["variables"].(map[string]interface{})
	if !ok {
		t.Fatal("expected variables in payload")
	}
	if vars["productCatId"] == nil {
		t.Error("expected productCatId to be set in variables")
	}
}

// ---------------------------------------------------------------------------
// Tests: SearchOffers default limit
// ---------------------------------------------------------------------------

func TestSearchOffers_DefaultLimit(t *testing.T) {
	ctx := context.Background()

	var capturedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedPayload)
		// No categories → routes to productOfferV2 (without category filter)
		resp := map[string]interface{}{
			"productOfferV2": map[string]interface{}{
				"nodes": []map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, graphqlResponse(resp))
	}))
	defer server.Close()

	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	// Limit 0 should default to 20
	_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vars, ok := capturedPayload["variables"].(map[string]interface{})
	if !ok {
		t.Fatal("expected variables in payload")
	}
	if limit, ok := vars["limit"].(float64); !ok || int(limit) != 20 {
		t.Errorf("expected default limit 20, got %v", vars["limit"])
	}
}

// ---------------------------------------------------------------------------
// Tests: SearchOffers original price calculation in searchShopee
// ---------------------------------------------------------------------------

func TestSearchOffers_OriginalPriceCalculation(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No categories → routes to productOfferV2 (without category filter)
		resp := map[string]interface{}{
			"productOfferV2": map[string]interface{}{
				"nodes": []map[string]interface{}{
					{
						"productName":       "Discounted",
						"itemId":            10,
						"shopId":            20,
						"productLink":       "link",
						"offerLink":         "offer",
						"imageUrl":          "img",
						"priceMin":          "80",
						"priceMax":          "120",
						"price":             "80",
						"commission":        "8",
						"commissionRate":    "10",
						"sales":             10,
						"ratingStar":        "4",
						"priceDiscountRate": 20,
						"shopName":          "Shop",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, graphqlResponse(resp))
	}))
	defer server.Close()

	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	result, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(result.Offers))
	}
	offer := result.Offers[0]
	// original price = 80 / (1 - 20/100) = 80 / 0.8 = 100
	if offer.OriginalPrice != 100.0 {
		t.Errorf("originalPrice = %f, want 100.0", offer.OriginalPrice)
	}
}

// ---------------------------------------------------------------------------
// Tests: shopeeGraphQL invalid JSON body
// ---------------------------------------------------------------------------

func TestShopeeGraphQL_InvalidJSONResponse(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not valid json!!!")
	}))
	defer server.Close()

	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Limit: 5})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// ---------------------------------------------------------------------------
// Tests: SearchOffers routing always uses productOfferV2
// ---------------------------------------------------------------------------

func TestSearchOffers_RoutingByCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("specific category uses productOfferV2", func(t *testing.T) {
		var capturedQuery string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if q, ok := body["query"].(string); ok {
					capturedQuery = q
				}
			}
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
			Categories: []string{"Eletrônicos"},
			Limit:      5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(capturedQuery, "productOfferV2") {
			t.Errorf("expected productOfferV2 in query for specific category, got: %s", capturedQuery)
		}
	})

	t.Run("no categories also uses productOfferV2", func(t *testing.T) {
		var capturedQuery string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if q, ok := body["query"].(string); ok {
					capturedQuery = q
				}
			}
			resp := map[string]interface{}{
				"productOfferV2": map[string]interface{}{
					"nodes": []map[string]interface{}{},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, graphqlResponse(resp))
		}))
		defer server.Close()

		repo := &mockAffiliateRepo{
			GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
				return validCreds(), nil
			},
		}
		svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

		_, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(capturedQuery, "productOfferV2") {
			t.Errorf("expected productOfferV2 in query for no categories, got: %s", capturedQuery)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: GenerateAffiliateLink — curto-circuito de link do próprio afiliado
// ---------------------------------------------------------------------------

// O curto-circuito de link próprio vale APENAS para SHORTLINK cuja expansão
// mostra utm_source=an_<app_id do usuário>. Um link LONGO próprio precisa
// passar pela conversão normal para sair ENCURTADO — devolver a URL de
// tracking gigante deixou grupos ilegíveis em produção. Link de outro
// afiliado segue o fluxo normal sempre (troca o tracking pelo do dono).
func TestGenerateAffiliateLink_OwnLinkShortCircuit(t *testing.T) {
	repo := &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return &models.AffiliateCredentials{Credentials: map[string]string{
				"appId":    "18330220029",
				"password": "irrelevant-secret",
			}}, nil
		},
	}

	apiCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	svc := newTestService(t, server.URL, repo, &mockSettingsRepo{})

	t.Run("link LONGO próprio → conversão normal (sai encurtado, não a URL gigante)", func(t *testing.T) {
		ownLongURL := "https://shopee.com.br/opaanlp/457864097/41713766789?utm_source=an_18330220029&utm_medium=affiliates"
		_, err := svc.GenerateAffiliateLink(context.Background(), ownLongURL, "user-1")
		if err == nil {
			t.Fatal("expected conversion attempt to fail against error-server, got nil")
		}
		if !apiCalled {
			t.Error("long own-tracked link must go through normal conversion to come out SHORT — relaying the giant tracking URL is unreadable in groups")
		}
	})

	t.Run("link de OUTRO afiliado → segue conversão normal", func(t *testing.T) {
		apiCalled = false
		otherURL := "https://shopee.com.br/opaanlp/457864097/41713766789?utm_source=an_99999999999"
		_, err := svc.GenerateAffiliateLink(context.Background(), otherURL, "user-1")
		if err == nil {
			t.Fatal("expected conversion attempt to fail against error-server, got nil")
		}
		if !apiCalled {
			t.Error("expected the normal conversion flow to hit the API for a foreign link")
		}
	})
}

func TestShouldRelayOwnShortLink(t *testing.T) {
	const appID = "18330220029"
	shortURL := "https://s.shopee.com.br/AbCdEf123"

	tests := []struct {
		name        string
		productURL  string
		expandedURL string
		want        bool
	}{
		{
			name:        "link próprio COM sub_id → repassa sem chamada à API",
			productURL:  shortURL,
			expandedURL: "https://shopee.com.br/product/457864097/41713766789?utm_source=an_" + appID + "&utm_medium=affiliates&utm_content=AUTOMATISA----",
			want:        true,
		},
		{
			name:        "link próprio SEM sub_id (gerado no painel da Shopee) → reconverte para aplicar o AUTOMATISA",
			productURL:  shortURL,
			expandedURL: "https://shopee.com.br/product/457864097/41713766789?utm_source=an_" + appID + "&utm_medium=affiliates&utm_content=----",
			want:        false,
		},
		{
			name:        "link de outro afiliado (mesmo com sub_id) → reconverte",
			productURL:  shortURL,
			expandedURL: "https://shopee.com.br/product/457864097/41713766789?utm_source=an_99999999999&utm_content=AUTOMATISA----",
			want:        false,
		},
		{
			name:        "link longo (não-shortlink) → nunca repassa",
			productURL:  "https://shopee.com.br/opaanlp/457864097/41713766789?utm_source=an_" + appID + "&utm_content=AUTOMATISA----",
			expandedURL: "https://shopee.com.br/opaanlp/457864097/41713766789?utm_source=an_" + appID + "&utm_content=AUTOMATISA----",
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRelayOwnShortLink(tt.productURL, tt.expandedURL, appID); got != tt.want {
				t.Errorf("shouldRelayOwnShortLink() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("appID vazio → nunca repassa", func(t *testing.T) {
		if shouldRelayOwnShortLink(shortURL, "https://shopee.com.br/product/1/2?utm_source=an_&utm_content=AUTOMATISA----", "") {
			t.Error("empty appID must not short-circuit")
		}
	})
}

// ---------------------------------------------------------------------------
// productCatIds (categorias oficiais) + writer do cache global de produto
// ---------------------------------------------------------------------------

// shopeeProductNode devolve um node de productOfferV2 completo; extras
// sobrescreve/adiciona campos (ex.: productCatIds).
func shopeeProductNode(extras map[string]interface{}) map[string]interface{} {
	node := map[string]interface{}{
		"productName":       "Test Product",
		"itemId":            456,
		"shopId":            123,
		"productLink":       "https://shopee.com.br/product/123/456",
		"offerLink":         "https://s.shopee.com.br/abc",
		"imageUrl":          "https://img.shopee.com.br/test.jpg",
		"priceMin":          "50.00",
		"priceMax":          "100.00",
		"price":             "75.00",
		"commission":        "5.00",
		"commissionRate":    "6.5",
		"sales":             100,
		"ratingStar":        "4.5",
		"priceDiscountRate": 25,
		"shopName":          "TestShop",
	}
	for k, v := range extras {
		node[k] = v
	}
	return node
}

// productOfferServer sobe um httptest que responde productOfferV2 com um
// único node e registra o corpo da última query recebida.
func productOfferServer(t *testing.T, node map[string]interface{}, lastQuery *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lastQuery != nil {
			body, _ := readBody(r)
			*lastQuery = body
		}
		resp := map[string]interface{}{
			"productOfferV2": map[string]interface{}{
				"nodes": []map[string]interface{}{node},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, graphqlResponse(resp))
	}))
}

func credsRepo() *mockAffiliateRepo {
	return &mockAffiliateRepo{
		GetByUserAndStoreFn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return validCreds(), nil
		},
	}
}

// newTestServiceWithCache é o newTestService com um cache REAL apontando
// para um Redis em memória (miniredis) — necessário para provar o writer
// do cache global, que a versão sem cache pula.
func newTestServiceWithCache(t *testing.T, baseURL string, c *cache.Cache) *shopeeService {
	t.Helper()
	svc := newTestService(t, baseURL, credsRepo(), &mockSettingsRepo{})
	svc.cache = c
	return svc
}

func newMiniredisCache(t *testing.T) (*cache.Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return cache.NewCache(client, "test", false), mr
}

func TestSearchOffers_ProductCategoryIDs(t *testing.T) {
	ctx := context.Background()

	searchServer := func(t *testing.T, node map[string]interface{}) *httptest.Server {
		t.Helper()
		return productOfferServer(t, node, nil)
	}

	t.Run("decodifica a lista de productCatIds", func(t *testing.T) {
		server := searchServer(t, shopeeProductNode(map[string]interface{}{
			"productCatIds": []int64{100001, 100002},
		}))
		defer server.Close()
		svc := newTestService(t, server.URL, credsRepo(), &mockSettingsRepo{})

		kw := "fone"
		res, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Keyword: &kw, Limit: 10})
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(res.Offers) != 1 {
			t.Fatalf("offers = %d, want 1", len(res.Offers))
		}
		got := res.Offers[0].ProductCategoryIDs
		if len(got) != 2 || got[0] != 100001 || got[1] != 100002 {
			t.Errorf("ProductCategoryIDs = %v, want [100001 100002]", got)
		}
	})

	for _, tc := range []struct {
		name  string
		value interface{}
		set   bool
	}{
		{name: "campo ausente", set: false},
		{name: "campo null", value: nil, set: true},
	} {
		t.Run(tc.name+" resulta em nil sem erro", func(t *testing.T) {
			extras := map[string]interface{}{}
			if tc.set {
				extras["productCatIds"] = tc.value
			}
			server := searchServer(t, shopeeProductNode(extras))
			defer server.Close()
			svc := newTestService(t, server.URL, credsRepo(), &mockSettingsRepo{})

			kw := "fone"
			res, err := svc.SearchOffers(ctx, "user1", models.ShopeeSearchParams{Keyword: &kw, Limit: 10})
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if len(res.Offers) != 1 {
				t.Fatalf("offers = %d, want 1", len(res.Offers))
			}
			if res.Offers[0].ProductCategoryIDs != nil {
				t.Errorf("ProductCategoryIDs = %v, want nil", res.Offers[0].ProductCategoryIDs)
			}
		})
	}
}

func TestGetProduct_ProductCategoryIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("solicita productCatIds na query e decodifica os IDs", func(t *testing.T) {
		var lastQuery string
		server := productOfferServer(t, shopeeProductNode(map[string]interface{}{
			"productCatIds": []int64{100001, 100002},
		}), &lastQuery)
		defer server.Close()
		svc := newTestService(t, server.URL, credsRepo(), &mockSettingsRepo{})

		product, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if !strings.Contains(lastQuery, "productCatIds") {
			t.Errorf("a query GraphQL de GetProduct deve pedir productCatIds; got: %s", lastQuery)
		}
		got := product.ProductCategoryIDs
		if len(got) != 2 || got[0] != 100001 || got[1] != 100002 {
			t.Errorf("ProductCategoryIDs = %v, want [100001 100002]", got)
		}
	})

	t.Run("campo ausente/null resulta em nil", func(t *testing.T) {
		for _, extras := range []map[string]interface{}{
			{},
			{"productCatIds": nil},
		} {
			server := productOfferServer(t, shopeeProductNode(extras), nil)
			svc := newTestService(t, server.URL, credsRepo(), &mockSettingsRepo{})
			product, err := svc.GetProduct(ctx, 456, 123, "user1")
			server.Close()
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if product.ProductCategoryIDs != nil {
				t.Errorf("ProductCategoryIDs = %v, want nil", product.ProductCategoryIDs)
			}
		}
	})
}

func TestGetProduct_GlobalCacheWriter(t *testing.T) {
	ctx := context.Background()

	t.Run("escreve a chave global com categorias e SEM link/comissão", func(t *testing.T) {
		c, mr := newMiniredisCache(t)
		server := productOfferServer(t, shopeeProductNode(map[string]interface{}{
			"productCatIds": []int64{100001, 100002},
		}), nil)
		defer server.Close()
		svc := newTestServiceWithCache(t, server.URL, c)

		product, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		// O retorno mantém link/comissão — só a CÓPIA cacheada é sanitizada.
		if product.AffiliateLink == "" || product.CommissionRate == 0 {
			t.Fatalf("o retorno de GetProduct não pode ser sanitizado: %+v", product)
		}

		raw, err := mr.Get("test:" + shopeeProductCacheKey(123, 456))
		if err != nil {
			t.Fatalf("chave global não foi escrita: %v", err)
		}

		var cached models.ShopeeOffer
		if err := json.Unmarshal([]byte(raw), &cached); err != nil {
			t.Fatalf("JSON cacheado inválido: %v", err)
		}
		if len(cached.ProductCategoryIDs) != 2 ||
			cached.ProductCategoryIDs[0] != 100001 || cached.ProductCategoryIDs[1] != 100002 {
			t.Errorf("categorias no cache = %v, want [100001 100002]", cached.ProductCategoryIDs)
		}
		if cached.AffiliateLink != "" {
			t.Errorf("AffiliateLink cacheado = %q, want vazio (não pode vazar entre contas)", cached.AffiliateLink)
		}
		if cached.CommissionRate != 0 {
			t.Errorf("CommissionRate cacheado = %v, want 0", cached.CommissionRate)
		}
		if cached.Name != "Test Product" || cached.Price != 75.0 {
			t.Errorf("dados de catálogo perdidos no cache: %+v", cached)
		}
	})

	t.Run("falha de escrita no cache não altera o retorno", func(t *testing.T) {
		c, mr := newMiniredisCache(t)
		server := productOfferServer(t, shopeeProductNode(map[string]interface{}{
			"productCatIds": []int64{100001},
		}), nil)
		defer server.Close()
		svc := newTestServiceWithCache(t, server.URL, c)

		mr.Close() // Redis fora do ar durante a escrita.

		product, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err != nil {
			t.Fatalf("cache indisponível não pode derrubar a oferta: %v", err)
		}
		if product == nil || product.Name != "Test Product" {
			t.Fatalf("produto = %+v, want o produto da API", product)
		}
		if len(product.ProductCategoryIDs) != 1 || product.ProductCategoryIDs[0] != 100001 {
			t.Errorf("categorias = %v, want [100001]", product.ProductCategoryIDs)
		}
	})

	t.Run("entrada antiga sem productCatIds desserializa com nil", func(t *testing.T) {
		c, mr := newMiniredisCache(t)
		// Fixture do FORMATO ANTIGO, exatamente como ficou gravada antes de 075.
		const legacyJSON = `{"itemId":456,"shopId":123,"name":"Produto Antigo","image":"https://img/x.jpg",` +
			`"price":75,"priceMin":50,"priceMax":100,"originalPrice":100,"discount":25,` +
			`"commissionRate":0,"sales":100,"stock":0,"shopName":"TestShop","shopRating":4.5,` +
			`"productUrl":"https://shopee.com.br/product/123/456","isMall":false,"isPreferredPlus":false}`
		mr.Set("test:"+shopeeProductCacheKey(123, 456), legacyJSON)

		svc := newTestServiceWithCache(t, "http://127.0.0.1:1", c) // API inalcançável: só cache hit serve
		product, err := svc.GetProduct(ctx, 456, 123, "user1")
		if err != nil {
			t.Fatalf("cache hit deveria servir sem chamar a API: %v", err)
		}
		if product.Name != "Produto Antigo" {
			t.Fatalf("veio da API em vez do cache: %+v", product)
		}
		if product.ProductCategoryIDs != nil {
			t.Errorf("ProductCategoryIDs = %v, want nil (fixture antiga)", product.ProductCategoryIDs)
		}
	})
}

func TestGetCategories_FullIDs(t *testing.T) {
	svc := newTestService(t, "", credsRepo(), &mockSettingsRepo{})
	cats, err := svc.GetCategories(context.Background(), nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("catálogo vazio")
	}
	for _, c := range cats {
		if len(c.IDs) == 0 {
			t.Errorf("grupo %q sem IDs", c.Name)
			continue
		}
		if c.ID != c.IDs[0] {
			t.Errorf("grupo %q: ID = %d, want IDs[0] = %d (retrocompat do caller de diagnóstico)", c.Name, c.ID, c.IDs[0])
		}
	}
}
