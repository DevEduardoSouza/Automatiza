package observability

import (
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestIsUserStateError(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		err  error
		want bool
	}{
		{"shopee invalid credential no err", "shopee API error: error [10020]: Invalid Credential", nil, true},
		{"shopee 10035 no wrap", "", errors.New("shopee API error: error [10035]: no Open API access"), true},
		{"invalid signature", "", errors.New("shopee API error: Invalid Signature"), true},
		{"session not working", "", errors.New("session 'session_01k' is not working: current status is 'FAILED'"), true},
		{"session status unexpected", "", errors.New("failed to send text message: Session status is not as expected. Try again"), true},
		{"instance not connected", "", errors.New("instance not connected: STARTING"), true},
		{"chip removido do grupo", "", errors.New("WAHA error 500: you're not participating in that group"), true},
		{"erro de infra passa", "Failed to update history status", errors.New("ERROR: new row violates check constraint"), false},
		{"connection refused passa", "", errors.New("dial tcp: connection refused"), false},
		{"vazio", "", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUserStateError(tc.msg, tc.err); got != tc.want {
				t.Fatalf("isUserStateError(%q, %v) = %v, quer %v", tc.msg, tc.err, got, tc.want)
			}
		})
	}
}

func TestIsUserStateEvent(t *testing.T) {
	drop := &sentry.Event{Exception: []sentry.Exception{{Value: "shopee API error: error [10020]: Invalid Credential"}}}
	if !isUserStateEvent(drop) {
		t.Fatal("evento com exception de credencial inválida deveria ser descartado")
	}
	dropMsg := &sentry.Event{Message: "session 'x' is not working: current status is 'STOPPED'"}
	if !isUserStateEvent(dropMsg) {
		t.Fatal("evento com message de sessão não-WORKING deveria ser descartado")
	}
	keep := &sentry.Event{Exception: []sentry.Exception{{Value: "context deadline exceeded while updating dispatch log"}}}
	if isUserStateEvent(keep) {
		t.Fatal("erro de infraestrutura não pode ser descartado")
	}
}
