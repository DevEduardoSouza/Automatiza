package instagram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*APIClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewAPIClient("appid", "appsecret", "https://cb.example/redirect", zap.NewNop())
	c.baseAPI = srv.URL
	c.baseGraph = srv.URL
	return c, srv
}

// TestExchangeCode cobre o ENVELOPE OFICIAL data[] do Business Login, com
// user_id vindo como number E como string (os dois formatos que a API devolve).
//
// Confirmado em 23/07/2026 na doc oficial de Business Login: a troca do code
// responde {"data":[{"access_token":...,"user_id":...,"permissions":...}]}.
// O código antigo decodificava o TOPO da resposta e por isso lia campos vazios.
func TestExchangeCode_OfficialDataEnvelope(t *testing.T) {
	cases := []struct {
		name       string
		userIDJSON string
		want       string
	}{
		{"numeric_user_id", `12345678`, "12345678"},
		{"string_user_id", `"12345678"`, "12345678"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/oauth/access_token" {
					t.Errorf("path = %q, want /oauth/access_token", r.URL.Path)
				}
				_ = r.ParseForm()
				if got := r.FormValue("grant_type"); got != "authorization_code" {
					t.Errorf("grant_type = %q", got)
				}
				if got := r.FormValue("code"); got != "AUTHCODE" {
					t.Errorf("code = %q", got)
				}
				fmt.Fprintf(w, `{"data":[{"access_token":"SHORTTOKEN","user_id":%s,"permissions":"instagram_business_basic"}]}`, tc.userIDJSON)
			})

			short, appScoped, err := c.ExchangeCode(context.Background(), "AUTHCODE")
			if err != nil {
				t.Fatalf("ExchangeCode: %v", err)
			}
			if short != "SHORTTOKEN" {
				t.Errorf("shortToken = %q, want SHORTTOKEN", short)
			}
			if appScoped != tc.want {
				t.Errorf("appScopedUserID = %q, want %q", appScoped, tc.want)
			}
		})
	}
}

// Envelope vazio, múltiplo, sem token ou sem id: nenhum deles pode "passar
// meio certo" — adivinhar aqui persiste o identificador errado. (O "topo plano"
// NÃO entra aqui: é o formato REAL de produção e é aceito — ver
// TestExchangeCode_AcceptsFlatTopLevel.)
func TestExchangeCode_RejectsMalformedEnvelopes(t *testing.T) {
	cases := map[string]string{
		"data vazio":     `{"data":[]}`,
		"data múltiplo":  `{"data":[{"access_token":"a","user_id":"1"},{"access_token":"b","user_id":"2"}]}`,
		"token vazio":    `{"data":[{"access_token":"","user_id":"1"}]}`,
		"sem app-scoped": `{"data":[{"access_token":"a","user_id":""}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			})
			if _, _, err := c.ExchangeCode(context.Background(), "X"); err == nil {
				t.Fatal("esperava erro, veio nil")
			}
		})
	}
}

// Forma REAL observada em produção (herança do Basic Display): objeto PLANO no
// topo, user_id NÚMERO e permissions ARRAY. Era isto que quebrava a conexão de
// 100% dos usuários com o parser antigo só de envelope.
func TestExchangeCode_AcceptsFlatTopLevel(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"SHORTTOKEN","user_id":12345678,"permissions":["instagram_business_basic","instagram_business_content_publish"]}`)
	})

	short, appScoped, err := c.ExchangeCode(context.Background(), "AUTHCODE")
	if err != nil {
		t.Fatalf("ExchangeCode (topo plano): %v", err)
	}
	if short != "SHORTTOKEN" {
		t.Errorf("shortToken = %q, want SHORTTOKEN", short)
	}
	if appScoped != "12345678" {
		t.Errorf("appScopedUserID = %q, want 12345678", appScoped)
	}
}

// GetMe também responde em data[] e devolve o id PROFISSIONAL — diferente do
// app-scoped da troca. Publicar com o id errado posta no objeto errado.
func TestGetMe_ReturnsProfessionalID(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fields"); !strings.Contains(got, "user_id") {
			t.Errorf("fields = %q", got)
		}
		fmt.Fprint(w, `{"data":[{"user_id":"PROFISSIONAL999","username":"loja","account_type":"BUSINESS"}]}`)
	})

	igID, username, accountType, err := c.GetMe(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if igID != "PROFISSIONAL999" {
		t.Errorf("igUserID = %q, want PROFISSIONAL999", igID)
	}
	if username != "loja" {
		t.Errorf("username = %q", username)
	}
	if accountType != "BUSINESS" {
		t.Errorf("accountType = %q", accountType)
	}
}

// /me também pode vir PLANO no topo (não só em data[]) — o parser tolerante
// aceita as duas formas.
func TestGetMe_AcceptsFlatTopLevel(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"user_id":"IG1","username":"loja","account_type":"BUSINESS"}`)
	})

	igID, username, accountType, err := c.GetMe(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("GetMe (topo plano): %v", err)
	}
	if igID != "IG1" || username != "loja" || accountType != "BUSINESS" {
		t.Errorf("GetMe = (%q,%q,%q), want (IG1,loja,BUSINESS)", igID, username, accountType)
	}
}

// A doc mostra "Business"/"Media_Creator"; outros endpoints já devolveram caixa
// alta. Ambos precisam passar, e conta pessoal precisa ser recusada.
func TestGetMe_AccountTypeHandling(t *testing.T) {
	cases := map[string]bool{
		"Business":      true,
		"BUSINESS":      true,
		"Media_Creator": true,
		"MEDIA_CREATOR": true,
		"":              true, // ausente no envelope oficial: não reprova
		"PERSONAL":      false,
		"Creator":       false, // fora do contrato: só Business e Media_Creator
	}
	for accountType, wantOK := range cases {
		t.Run("tipo_"+accountType, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"data":[{"user_id":"IG1","username":"u","account_type":%q}]}`, accountType)
			})
			_, _, _, err := c.GetMe(context.Background(), "TOKEN")
			if wantOK && err != nil {
				t.Fatalf("account_type %q deveria ser aceito: %v", accountType, err)
			}
			if !wantOK && err == nil {
				t.Fatalf("account_type %q deveria ser recusado", accountType)
			}
		})
	}
}

func TestGetMe_RejectsEmptyEnvelope(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	})
	if _, _, _, err := c.GetMe(context.Background(), "TOKEN"); err == nil {
		t.Fatal("envelope vazio deveria falhar")
	}
}

// expires_in <= 0 é resposta inútil. Antes o service "corrigia" isso inventando
// 60 dias — a credencial parecia válida por dois meses e todo envio falhava.
func TestTokenResponses_RejectNonPositiveExpiry(t *testing.T) {
	for _, body := range []string{
		`{"access_token":"T","token_type":"bearer","expires_in":0}`,
		`{"access_token":"T","token_type":"bearer","expires_in":-5}`,
		`{"access_token":"","token_type":"bearer","expires_in":5183944}`,
	} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, body)
		})
		if _, _, err := c.ExchangeLongLived(context.Background(), "SHORT"); err == nil {
			t.Errorf("ExchangeLongLived deveria recusar %s", body)
		}
		if _, _, err := c.RefreshLongLived(context.Background(), "LONG"); err == nil {
			t.Errorf("RefreshLongLived deveria recusar %s", body)
		}
	}
}

// TestStoryPublishFlow cobre o caminho feliz create -> status(FINISHED) -> publish.
func TestStoryPublishFlow(t *testing.T) {
	var createHit, statusHit, publishHit bool
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/media_publish"):
			publishHit = true
			_ = r.ParseForm()
			if got := r.FormValue("creation_id"); got != "CONTAINER123" {
				t.Errorf("creation_id = %q", got)
			}
			fmt.Fprint(w, `{"id":"MEDIA999"}`)
		case strings.HasSuffix(r.URL.Path, "/media"):
			createHit = true
			_ = r.ParseForm()
			if got := r.FormValue("media_type"); got != "STORIES" {
				t.Errorf("media_type = %q, want STORIES", got)
			}
			if got := r.FormValue("image_url"); got != "https://img.example/x.jpg" {
				t.Errorf("image_url = %q", got)
			}
			fmt.Fprint(w, `{"id":"CONTAINER123"}`)
		case strings.Contains(r.URL.Path, "CONTAINER123"):
			statusHit = true
			if got := r.URL.Query().Get("fields"); got != "status_code" {
				t.Errorf("fields = %q, want status_code", got)
			}
			fmt.Fprint(w, `{"status_code":"FINISHED"}`)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	ctx := context.Background()

	cid, err := c.CreateStoryContainer(ctx, "IG123", "TOKEN", "https://img.example/x.jpg")
	if err != nil {
		t.Fatalf("CreateStoryContainer: %v", err)
	}
	if cid != "CONTAINER123" {
		t.Fatalf("creationID = %q, want CONTAINER123", cid)
	}

	st, err := c.GetContainerStatus(ctx, cid, "TOKEN")
	if err != nil {
		t.Fatalf("GetContainerStatus: %v", err)
	}
	if st != "FINISHED" {
		t.Fatalf("statusCode = %q, want FINISHED", st)
	}

	mid, err := c.PublishContainer(ctx, "IG123", "TOKEN", cid)
	if err != nil {
		t.Fatalf("PublishContainer: %v", err)
	}
	if mid != "MEDIA999" {
		t.Fatalf("mediaID = %q, want MEDIA999", mid)
	}

	if !createHit || !statusHit || !publishHit {
		t.Fatalf("nem todos os endpoints foram atingidos: create=%v status=%v publish=%v", createHit, statusHit, publishHit)
	}
}

// Classificação por code/subcode/status. Antes, TODO 400 virava erro de
// autenticação: erro de processamento pedia "reconecte sua conta" e ainda
// perdia o retry.
func TestClassifyMetaError(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantIs   error
		wantNotv error
	}{
		{"code 190 é auth", http.StatusBadRequest,
			`{"error":{"message":"Invalid platform app","code":190,"error_subcode":458}}`,
			ErrInstagramAuth, nil},
		{"400 genérico NÃO é auth", http.StatusBadRequest,
			`{"error":{"message":"bad param","code":100}}`,
			ErrInstagramPermanent, ErrInstagramAuth},
		{"401 é auth", http.StatusUnauthorized, `{"error":{"message":"x"}}`, ErrInstagramAuth, nil},
		{"403 é auth", http.StatusForbidden, `{"error":{"message":"x"}}`, ErrInstagramAuth, nil},
		{"is_transient vence 403", http.StatusForbidden,
			`{"error":{"message":"x","is_transient":true}}`, ErrInstagramTransient, ErrInstagramAuth},
		{"429 é transitório", http.StatusTooManyRequests, `{"error":{"message":"x"}}`, ErrInstagramTransient, nil},
		{"500 é transitório", http.StatusInternalServerError, `{"error":{"message":"x","code":1}}`, ErrInstagramTransient, ErrInstagramAuth},
		{"code -1 é transitório", http.StatusBadRequest, `{"error":{"message":"x","code":-1}}`, ErrInstagramTransient, nil},
		{"code -2 é transitório", http.StatusBadRequest, `{"error":{"message":"x","code":-2}}`, ErrInstagramTransient, nil},
		{"subcode 2207003 é transitório", http.StatusBadRequest,
			`{"error":{"message":"x","code":9004,"error_subcode":2207003}}`, ErrInstagramTransient, nil},
		{"subcode 2207032 é transitório", http.StatusBadRequest,
			`{"error":{"message":"x","error_subcode":2207032}}`, ErrInstagramTransient, nil},
		{"subcode 2207053 é transitório", http.StatusBadRequest,
			`{"error":{"message":"x","error_subcode":2207053}}`, ErrInstagramTransient, nil},
		{"subcode 2207008 é transitório", http.StatusBadRequest,
			`{"error":{"message":"x","error_subcode":2207008}}`, ErrInstagramTransient, nil},
		{"subcode 2207027 é transitório", http.StatusBadRequest,
			`{"error":{"message":"x","error_subcode":2207027}}`, ErrInstagramTransient, nil},
		{"code 9 é cota", http.StatusBadRequest,
			`{"error":{"message":"x","code":9}}`, ErrInstagramRateLimit, ErrInstagramAuth},
		{"subcode 2207042 é cota", http.StatusBadRequest,
			`{"error":{"message":"x","error_subcode":2207042}}`, ErrInstagramRateLimit, ErrInstagramAuth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			_, err := c.GetContainerStatus(context.Background(), "CONTAINER123", "TOKEN")
			if err == nil {
				t.Fatal("esperava erro, veio nil")
			}
			if !errors.Is(err, tc.wantIs) {
				t.Errorf("esperava %v, veio %v", tc.wantIs, err)
			}
			if tc.wantNotv != nil && errors.Is(err, tc.wantNotv) {
				t.Errorf("NÃO deveria ser %v: %v", tc.wantNotv, err)
			}
		})
	}
}

// O erro tipado não pode carregar mensagem, corpo ou token: ele acaba em log
// e a mensagem da Meta pode conter URL da mídia e ids.
func TestMetaAPIError_DoesNotLeakProviderDetail(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"token SEGREDO123 invalido em https://cdn/x.jpg?sig=abc","code":100}}`)
	})
	_, err := c.GetContainerStatus(context.Background(), "CONTAINER123", "TOKEN-SECRETO")
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, leak := range []string{"SEGREDO123", "https://cdn", "sig=abc", "TOKEN-SECRETO", "invalido"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("erro vaza %q: %s", leak, err.Error())
		}
	}
	var metaErr *MetaAPIError
	if !errors.As(err, &metaErr) {
		t.Fatal("erro deveria ser *MetaAPIError")
	}
	if metaErr.Code != 100 || metaErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("metadados perdidos: %+v", metaErr)
	}
}

// Falha de rede (servidor fechado) é transitória — nunca auth.
func TestNetworkFailureIsTransient(t *testing.T) {
	c := NewAPIClient("appid", "appsecret", "cb", zap.NewNop())
	c.baseGraph = "http://127.0.0.1:1" // porta fechada

	_, err := c.GetContainerStatus(context.Background(), "C1", "TOKEN")
	if err == nil {
		t.Fatal("esperava erro de rede")
	}
	if !errors.Is(err, ErrInstagramTransient) {
		t.Errorf("falha de rede deveria ser transitória, got %v", err)
	}
	if errors.Is(err, ErrInstagramAuth) {
		t.Error("falha de rede não pode ser auth")
	}
}

// O escopo pedido é o mínimo da entrega: nada de messages/comments, que
// implicariam capacidade de inbound que não existe.
func TestAuthorizeURL_RequestsOnlyPublishScopes(t *testing.T) {
	c := NewAPIClient("appid", "appsecret", "https://cb.example/redirect", zap.NewNop())
	u := c.AuthorizeURL("STATE123")

	if !strings.Contains(u, "instagram_business_basic") || !strings.Contains(u, "instagram_business_content_publish") {
		t.Errorf("escopos de publicação ausentes: %s", u)
	}
	for _, forbidden := range []string{"manage_messages", "manage_comments"} {
		if strings.Contains(u, forbidden) {
			t.Errorf("escopo %q não pertence a esta entrega: %s", forbidden, u)
		}
	}
	if !strings.Contains(u, "state=STATE123") {
		t.Errorf("state ausente: %s", u)
	}
}
