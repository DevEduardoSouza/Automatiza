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

// TestExchangeCode cobre o code->short token com user_id vindo como number E
// como string (os dois formatos que o Instagram devolve conforme o endpoint).
func TestExchangeCode(t *testing.T) {
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				fmt.Fprintf(w, `{"access_token":"SHORTTOKEN","user_id":%s,"permissions":["instagram_business_basic"]}`, tc.userIDJSON)
			}))
			defer srv.Close()

			c := NewAPIClient("appid", "appsecret", "https://cb.example/redirect", zap.NewNop())
			c.baseAPI = srv.URL

			short, uid, err := c.ExchangeCode(context.Background(), "AUTHCODE")
			if err != nil {
				t.Fatalf("ExchangeCode: %v", err)
			}
			if short != "SHORTTOKEN" {
				t.Errorf("shortToken = %q, want SHORTTOKEN", short)
			}
			if uid != tc.want {
				t.Errorf("igUserID = %q, want %q", uid, tc.want)
			}
		})
	}
}

// TestStoryPublishFlow cobre o caminho feliz create -> status(FINISHED) -> publish.
func TestStoryPublishFlow(t *testing.T) {
	var createHit, statusHit, publishHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer srv.Close()

	c := NewAPIClient("appid", "appsecret", "cb", zap.NewNop())
	c.baseGraph = srv.URL
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

// TestExchangeCodeAuthError garante que um 400 com envelope de erro (code 190)
// é classificado como ErrInstagramAuth (permanente -> SkipRetry no worker).
func TestExchangeCodeAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"Invalid platform app","type":"OAuthException","code":190,"error_subcode":458}}`)
	}))
	defer srv.Close()

	c := NewAPIClient("appid", "appsecret", "cb", zap.NewNop())
	c.baseAPI = srv.URL

	_, _, err := c.ExchangeCode(context.Background(), "BADCODE")
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}
	if !errors.Is(err, ErrInstagramAuth) {
		t.Fatalf("esperava ErrInstagramAuth, veio: %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid platform app") {
		t.Errorf("erro deveria conter a mensagem do envelope: %v", err)
	}
}

// TestTransientErrorNotAuth garante que 5xx NÃO carrega ErrInstagramAuth
// (transitório -> o worker pode reprocessar).
func TestTransientErrorNotAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"temporary","type":"Internal","code":1}}`)
	}))
	defer srv.Close()

	c := NewAPIClient("appid", "appsecret", "cb", zap.NewNop())
	c.baseGraph = srv.URL

	_, err := c.GetContainerStatus(context.Background(), "CONTAINER123", "TOKEN")
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}
	if errors.Is(err, ErrInstagramAuth) {
		t.Fatalf("5xx não deveria ser ErrInstagramAuth: %v", err)
	}
}
