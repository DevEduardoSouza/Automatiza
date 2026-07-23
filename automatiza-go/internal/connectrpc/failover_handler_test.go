package connectrpc

import (
	"context"
	"testing"

	failoverv1 "automatiza-go/internal/gen/failover/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/failover"
	"automatiza-go/internal/auth"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// failoverTestRepo é um repositório vazio de propósito: com a feature
// desligada NENHUM método pode ser chamado, e o embed da interface transforma
// qualquer chamada num panic — que é exatamente o alarme que este arquivo
// quer disparar se alguém remover o ensureEnabled de uma RPC.
type failoverTestRepo struct {
	repository.InstanceFailoverRepository
}

type failoverTestInstanceRepo struct {
	repository.InstanceRepository
}

func disabledHandler() *FailoverHandler {
	return NewFailoverHandler(failover.NewService(
		&failoverTestRepo{}, &failoverTestInstanceRepo{}, nil,
		func() bool { return false }, zap.NewNop(),
	))
}

func userCtx() context.Context {
	return context.WithValue(context.Background(), UserContextKey, &auth.User{ID: "user-1"})
}

// 13.7.1 — sem autenticação, Unauthenticated. E a checagem vem ANTES de
// qualquer coisa: nenhuma RPC pode vazar existência para quem não se
// identificou.
func TestFailoverHandler_RequiresAuth(t *testing.T) {
	h := disabledHandler()
	ctx := context.Background()

	calls := map[string]func() error{
		"SetPolicy": func() error {
			_, err := h.SetPolicy(ctx, connect.NewRequest(&failoverv1.SetPolicyRequest{}))
			return err
		},
		"GetPolicy": func() error {
			_, err := h.GetPolicy(ctx, connect.NewRequest(&failoverv1.GetPolicyRequest{}))
			return err
		},
		"ListPolicies": func() error {
			_, err := h.ListPolicies(ctx, connect.NewRequest(&failoverv1.ListPoliciesRequest{}))
			return err
		},
		"RunDryRun": func() error {
			_, err := h.RunDryRun(ctx, connect.NewRequest(&failoverv1.RunDryRunRequest{}))
			return err
		},
		"ConfirmFailover": func() error {
			_, err := h.ConfirmFailover(ctx, connect.NewRequest(&failoverv1.ConfirmFailoverRequest{}))
			return err
		},
		"RevertFailover": func() error {
			_, err := h.RevertFailover(ctx, connect.NewRequest(&failoverv1.RevertFailoverRequest{}))
			return err
		},
		"DisablePolicy": func() error {
			_, err := h.DisablePolicy(ctx, connect.NewRequest(&failoverv1.DisablePolicyRequest{}))
			return err
		},
		"ListEvents": func() error {
			_, err := h.ListEvents(ctx, connect.NewRequest(&failoverv1.ListEventsRequest{}))
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("%s: expected Unauthenticated, got %v (%v)", name, connect.CodeOf(err), err)
			}
		})
	}
}

// 13.7.6/13.7.7 — flag off responde FailedPrecondition com o reason estável,
// em TODAS as RPCs (inclusive leitura), e NUNCA Unimplemented.
func TestFailoverHandler_DisabledFlagReason(t *testing.T) {
	h := disabledHandler()
	ctx := userCtx()

	calls := map[string]func() error{
		"GetPolicy": func() error {
			_, err := h.GetPolicy(ctx, connect.NewRequest(&failoverv1.GetPolicyRequest{PolicyId: 1}))
			return err
		},
		"ListPolicies": func() error {
			_, err := h.ListPolicies(ctx, connect.NewRequest(&failoverv1.ListPoliciesRequest{}))
			return err
		},
		"ListEvents": func() error {
			_, err := h.ListEvents(ctx, connect.NewRequest(&failoverv1.ListEventsRequest{PolicyId: 1}))
			return err
		},
		"RunDryRun": func() error {
			_, err := h.RunDryRun(ctx, connect.NewRequest(&failoverv1.RunDryRunRequest{
				PolicyId:       1,
				Direction:      failoverv1.FailoverDirection_FAILOVER_DIRECTION_FAILOVER,
				IdempotencyKey: "chave-idempotente-valida",
			}))
			return err
		},
		"ConfirmFailover": func() error {
			_, err := h.ConfirmFailover(ctx, connect.NewRequest(&failoverv1.ConfirmFailoverRequest{
				PolicyId: 1, IdempotencyKey: "chave-idempotente-valida",
			}))
			return err
		},
		"RevertFailover": func() error {
			_, err := h.RevertFailover(ctx, connect.NewRequest(&failoverv1.RevertFailoverRequest{
				PolicyId: 1, IdempotencyKey: "chave-idempotente-valida",
			}))
			return err
		},
		"DisablePolicy": func() error {
			_, err := h.DisablePolicy(ctx, connect.NewRequest(&failoverv1.DisablePolicyRequest{
				PolicyId: 1, IdempotencyKey: "chave-idempotente-valida",
			}))
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if connect.CodeOf(err) == connect.CodeUnimplemented {
				t.Fatalf("%s: disabled feature must never answer Unimplemented", name)
			}
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("%s: expected FailedPrecondition, got %v (%v)", name, connect.CodeOf(err), err)
			}
			var ce *connect.Error
			if ok := asConnectError(err, &ce); !ok {
				t.Fatalf("%s: expected a connect error", name)
			}
			if ce.Meta().Get("failover-reason") != failover.ReasonDisabled {
				t.Fatalf("%s: missing stable reason code, meta=%v msg=%q",
					name, ce.Meta(), ce.Message())
			}
			// A UI só esconde a seção com este código; qualquer outro erro é
			// exibido como falha real.
			if !containsStr(ce.Message(), failover.ReasonDisabled) {
				t.Fatalf("%s: message should carry the reason code: %q", name, ce.Message())
			}
		})
	}
}

// SetPolicy com a flag off também precisa parar antes de tocar o repositório.
// Ele lê a política ANTES do ensureEnabled para resolver o modo default, então
// esse caminho tem seu próprio teste.
func TestFailoverHandler_SetPolicyDisabled(t *testing.T) {
	_, err := disabledHandler().SetPolicy(userCtx(), connect.NewRequest(&failoverv1.SetPolicyRequest{
		PrimaryInstanceId: 1,
		BackupInstanceId:  2,
		Mode:              failoverv1.FailoverMode_FAILOVER_MODE_MANUAL,
		IdempotencyKey:    "chave-idempotente-valida",
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (%v)", connect.CodeOf(err), err)
	}
}

// 13.7.5 — enum numérico desconhecido e tentativa de "automatic" viram
// InvalidArgument. Não existe valor de modo automático nem por acidente.
func TestModeFromProto_RejectsUnknown(t *testing.T) {
	if _, err := modeFromProto(failoverv1.FailoverMode(99), models.FailoverModeManual); err == nil {
		t.Fatal("unknown numeric enum must be rejected")
	}
	// UNSPECIFIED resolve para o fallback (MANUAL no create, o modo atual no
	// update) — nunca para algo que o usuário não pediu.
	got, err := modeFromProto(failoverv1.FailoverMode_FAILOVER_MODE_UNSPECIFIED, models.FailoverModeAssisted)
	if err != nil || got != models.FailoverModeAssisted {
		t.Fatalf("UNSPECIFIED must use the fallback: got %q err %v", got, err)
	}
	for in, want := range map[failoverv1.FailoverMode]string{
		failoverv1.FailoverMode_FAILOVER_MODE_MANUAL:   models.FailoverModeManual,
		failoverv1.FailoverMode_FAILOVER_MODE_ASSISTED: models.FailoverModeAssisted,
	} {
		got, err := modeFromProto(in, models.FailoverModeManual)
		if err != nil || got != want {
			t.Fatalf("modeFromProto(%v) = %q, %v", in, got, err)
		}
	}
}

// O enum de modo só tem três valores, e nenhum deles é automático.
func TestFailoverModeEnum_HasNoAutomatic(t *testing.T) {
	for value, name := range failoverv1.FailoverMode_name {
		switch failoverv1.FailoverMode(value) {
		case failoverv1.FailoverMode_FAILOVER_MODE_UNSPECIFIED,
			failoverv1.FailoverMode_FAILOVER_MODE_MANUAL,
			failoverv1.FailoverMode_FAILOVER_MODE_ASSISTED:
		default:
			t.Fatalf("unexpected failover mode in the wire contract: %s", name)
		}
	}
}

func TestDirectionFromProto(t *testing.T) {
	if _, err := directionFromProto(failoverv1.FailoverDirection_FAILOVER_DIRECTION_UNSPECIFIED); err == nil {
		t.Fatal("UNSPECIFIED direction must be rejected")
	}
	if _, err := directionFromProto(failoverv1.FailoverDirection(42)); err == nil {
		t.Fatal("unknown direction must be rejected")
	}
}

func TestInt64ToInstanceID(t *testing.T) {
	for _, bad := range []int64{0, -1} {
		if _, err := int64ToInstanceID(bad); err == nil {
			t.Fatalf("instance id %d must be rejected", bad)
		}
	}
	if v, err := int64ToInstanceID(42); err != nil || v != 42 {
		t.Fatalf("int64ToInstanceID(42) = %d, %v", v, err)
	}
}

// Mapeamento de erros do domínio para códigos Connect (seção 8 do plano).
func TestFailoverErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want connect.Code
	}{
		{failover.ErrDisabled, connect.CodeFailedPrecondition},
		{failover.ErrNotFound, connect.CodeNotFound},
		{failover.ErrInvalidArgument, connect.CodeInvalidArgument},
		{failover.ErrStateConflict, connect.CodeFailedPrecondition},
		{failover.ErrCoverageBlocked, connect.CodeFailedPrecondition},
		{failover.ErrDryRunExpired, connect.CodeFailedPrecondition},
		{failover.ErrDryRunMismatch, connect.CodeFailedPrecondition},
		{repository.ErrFailoverIdempotencyMismatch, connect.CodeAlreadyExists},
		{repository.ErrFailoverVersionConflict, connect.CodeAborted},
		{repository.ErrFailoverConcurrentOperation, connect.CodeAborted},
		{repository.ErrFailoverStaleConfiguration, connect.CodeAborted},
		{repository.ErrFailoverInProgress, connect.CodeAborted},
		{failover.ErrDrainTimeout, connect.CodeDeadlineExceeded},
		{failover.ErrIndeterminate, connect.CodeInternal},
	}
	for _, tc := range cases {
		if got := connect.CodeOf(failoverError(tc.err)); got != tc.want {
			t.Fatalf("%v: got %v, want %v", tc.err, got, tc.want)
		}
	}
	if failoverError(nil) != nil {
		t.Fatal("nil must map to nil")
	}
}

// Erro interno não pode vazar detalhe sensível para o cliente.
func TestFailoverError_InternalIsOpaque(t *testing.T) {
	err := failoverError(errNotMapped{})
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unmapped error should be Internal, got %v", connect.CodeOf(err))
	}
	var ce *connect.Error
	if asConnectError(err, &ce) && containsStr(ce.Message(), "senha-secreta") {
		t.Fatal("internal errors must not leak the original message")
	}
}

type errNotMapped struct{}

func (errNotMapped) Error() string { return "conexão falhou com senha-secreta no dsn" }

func asConnectError(err error, target **connect.Error) bool {
	ce, ok := err.(*connect.Error)
	if ok {
		*target = ce
	}
	return ok
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
