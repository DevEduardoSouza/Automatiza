package errors_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	apperrors "automatiza-go/internal/shared/errors"
)

// --- MapToConnectError tests ---

func TestMapToConnectError(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		got := apperrors.MapToConnectError(nil)
		if got != nil {
			t.Errorf("MapToConnectError(nil) = %v, want nil", got)
		}
	})

	t.Run("already connect.Error returned as-is", func(t *testing.T) {
		t.Parallel()

		original := connect.NewError(connect.CodeNotFound, fmt.Errorf("already mapped"))
		got := apperrors.MapToConnectError(original)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeNotFound {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeNotFound)
		}
		if got.Message() != "already mapped" {
			t.Errorf("Message = %q, want %q", got.Message(), "already mapped")
		}
	})

	t.Run("wrapped connect.Error extracted", func(t *testing.T) {
		t.Parallel()

		original := connect.NewError(connect.CodeAborted, fmt.Errorf("inner"))
		wrapped := fmt.Errorf("outer: %w", original)
		got := apperrors.MapToConnectError(wrapped)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeAborted {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeAborted)
		}
	})

	// Table-driven tests for sentinel error -> connect code mapping
	sentinelTests := []struct {
		name       string
		err        error
		expectCode connect.Code
		expectMsg  string
	}{
		// Authentication errors -> Unauthenticated
		{
			name:       "ErrInvalidCredentials",
			err:        apperrors.ErrInvalidCredentials,
			expectCode: connect.CodeUnauthenticated,
			expectMsg:  "Email ou senha inválidos",
		},
		{
			name:       "ErrSessionExpired",
			err:        apperrors.ErrSessionExpired,
			expectCode: connect.CodeUnauthenticated,
			expectMsg:  "Sessão expirada. Faça login novamente",
		},
		{
			name:       "ErrSessionNotFound",
			err:        apperrors.ErrSessionNotFound,
			expectCode: connect.CodeUnauthenticated,
			expectMsg:  "Sessão expirada. Faça login novamente",
		},
		{
			name:       "ErrInvalidSession",
			err:        apperrors.ErrInvalidSession,
			expectCode: connect.CodeUnauthenticated,
			expectMsg:  "Sessão expirada. Faça login novamente",
		},
		{
			name:       "ErrUnauthorized",
			err:        apperrors.ErrUnauthorized,
			expectCode: connect.CodeUnauthenticated,
			expectMsg:  "Autenticação necessária",
		},

		// Permission errors -> PermissionDenied
		{
			name:       "ErrAccountLocked",
			err:        apperrors.ErrAccountLocked,
			expectCode: connect.CodePermissionDenied,
			expectMsg:  "Conta bloqueada por tentativas excessivas. Tente novamente em 30 minutos",
		},
		{
			name:       "ErrAccountSuspended",
			err:        apperrors.ErrAccountSuspended,
			expectCode: connect.CodePermissionDenied,
			expectMsg:  "Sua conta está suspensa por pendência de pagamento. Entre em contato com o suporte",
		},
		{
			name:       "ErrNotAdmin",
			err:        apperrors.ErrNotAdmin,
			expectCode: connect.CodePermissionDenied,
			expectMsg:  "Você não tem permissão para esta ação",
		},
		{
			name:       "ErrInsufficientRole",
			err:        apperrors.ErrInsufficientRole,
			expectCode: connect.CodePermissionDenied,
			expectMsg:  "Você não tem permissão para esta ação",
		},
		{
			name:       "ErrPermissionDenied",
			err:        apperrors.ErrPermissionDenied,
			expectCode: connect.CodePermissionDenied,
			expectMsg:  "Você não tem permissão para esta ação",
		},

		// Rate limiting -> ResourceExhausted
		{
			name:       "ErrRateLimited",
			err:        apperrors.ErrRateLimited,
			expectCode: connect.CodeResourceExhausted,
			expectMsg:  "Muitas tentativas. Aguarde alguns minutos",
		},
		{
			name:       "ErrIPBlocked",
			err:        apperrors.ErrIPBlocked,
			expectCode: connect.CodeResourceExhausted,
			expectMsg:  "Acesso temporariamente bloqueado",
		},

		// Validation errors -> InvalidArgument
		{
			name:       "ErrWeakPassword",
			err:        apperrors.ErrWeakPassword,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Senha muito fraca. Use pelo menos 8 caracteres com letras e números",
		},
		{
			name:       "ErrInvalidEmail",
			err:        apperrors.ErrInvalidEmail,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Email inválido",
		},
		{
			name:       "ErrInvalidToken",
			err:        apperrors.ErrInvalidToken,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Link inválido ou expirado. Solicite um novo",
		},
		{
			name:       "ErrTokenExpired",
			err:        apperrors.ErrTokenExpired,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Link inválido ou expirado. Solicite um novo",
		},
		{
			name:       "ErrTokenUsed",
			err:        apperrors.ErrTokenUsed,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Link inválido ou expirado. Solicite um novo",
		},
		{
			name:       "ErrInvalidCSRF",
			err:        apperrors.ErrInvalidCSRF,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Token de segurança inválido. Recarregue a página",
		},
		{
			name:       "ErrInvalidUserID",
			err:        apperrors.ErrInvalidUserID,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "ID de usuário inválido",
		},
		{
			name:       "ErrSelfPasswordReset",
			err:        apperrors.ErrSelfPasswordReset,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Use a opção 'Alterar senha' para alterar sua própria senha",
		},
		{
			name:       "ErrInvalidReference",
			err:        apperrors.ErrInvalidReference,
			expectCode: connect.CodeInvalidArgument,
			expectMsg:  "Referência inválida",
		},

		// Conflict -> AlreadyExists
		{
			name:       "ErrEmailExists",
			err:        apperrors.ErrEmailExists,
			expectCode: connect.CodeAlreadyExists,
			expectMsg:  "Este email já está cadastrado",
		},
		{
			name:       "ErrDuplicateEntry",
			err:        apperrors.ErrDuplicateEntry,
			expectCode: connect.CodeAlreadyExists,
			expectMsg:  "Registro já existe",
		},

		// Not found -> NotFound
		{
			name:       "ErrUserNotFound",
			err:        apperrors.ErrUserNotFound,
			expectCode: connect.CodeNotFound,
			expectMsg:  "Registro não encontrado",
		},
		{
			name:       "ErrNotFound",
			err:        apperrors.ErrNotFound,
			expectCode: connect.CodeNotFound,
			expectMsg:  "Registro não encontrado",
		},

		// Precondition -> FailedPrecondition
		{
			name:       "ErrEmailNotVerified",
			err:        apperrors.ErrEmailNotVerified,
			expectCode: connect.CodeFailedPrecondition,
			expectMsg:  "Verifique seu email antes de continuar",
		},

		// Retryable -> Unavailable
		{
			name:       "ErrRetryable",
			err:        apperrors.ErrRetryable,
			expectCode: connect.CodeUnavailable,
			expectMsg:  "Serviço temporariamente indisponível. Tente novamente",
		},
		{
			name:       "ErrRedisUnavailable",
			err:        apperrors.ErrRedisUnavailable,
			expectCode: connect.CodeUnavailable,
			expectMsg:  "Serviço temporariamente indisponível. Tente novamente",
		},
		{
			name:       "ErrExternalServiceDown",
			err:        apperrors.ErrExternalServiceDown,
			expectCode: connect.CodeUnavailable,
			expectMsg:  "Serviço temporariamente indisponível. Tente novamente",
		},

		// Timeout -> DeadlineExceeded
		{
			name:       "ErrTimeout",
			err:        apperrors.ErrTimeout,
			expectCode: connect.CodeDeadlineExceeded,
			expectMsg:  "Operação demorou demais. Tente novamente",
		},

		// Email send failed -> Internal
		{
			name:       "ErrEmailSendFailed",
			err:        apperrors.ErrEmailSendFailed,
			expectCode: connect.CodeInternal,
			expectMsg:  "Falha ao enviar email. Tente novamente mais tarde",
		},
	}

	for _, tt := range sentinelTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := apperrors.MapToConnectError(tt.err)
			if got == nil {
				t.Fatal("expected non-nil connect.Error")
			}
			if got.Code() != tt.expectCode {
				t.Errorf("Code = %v, want %v", got.Code(), tt.expectCode)
			}
			if got.Message() != tt.expectMsg {
				t.Errorf("Message = %q, want %q", got.Message(), tt.expectMsg)
			}
		})
	}

	t.Run("ConflictError maps to CodeAborted", func(t *testing.T) {
		t.Parallel()

		conflictErr := apperrors.NewConflictError("automation", "already running")
		got := apperrors.MapToConnectError(conflictErr)
		if got == nil {
			t.Fatal("expected non-nil connect.Error")
		}
		if got.Code() != connect.CodeAborted {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeAborted)
		}
		if got.Message() != "Recurso temporariamente bloqueado. Tente novamente em alguns minutos" {
			t.Errorf("Message = %q", got.Message())
		}
	})

	t.Run("unknown error maps to CodeInternal", func(t *testing.T) {
		t.Parallel()

		unknownErr := errors.New("something completely unknown")
		got := apperrors.MapToConnectError(unknownErr)
		if got == nil {
			t.Fatal("expected non-nil connect.Error")
		}
		if got.Code() != connect.CodeInternal {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeInternal)
		}
		if got.Message() != "Erro interno. Tente novamente ou entre em contato com o suporte" {
			t.Errorf("Message = %q", got.Message())
		}
	})

	t.Run("wrapped sentinel error is still recognized", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("service layer: %w", apperrors.ErrNotFound)
		got := apperrors.MapToConnectError(wrapped)
		if got == nil {
			t.Fatal("expected non-nil connect.Error")
		}
		if got.Code() != connect.CodeNotFound {
			t.Errorf("Code = %v, want %v for wrapped ErrNotFound", got.Code(), connect.CodeNotFound)
		}
	})
}

// --- MapToConnectErrorWithDetails tests ---

func TestMapToConnectErrorWithDetails(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		got := apperrors.MapToConnectErrorWithDetails(nil, true)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("isDev false returns standard error without debug info", func(t *testing.T) {
		t.Parallel()

		err := apperrors.ErrNotFound
		got := apperrors.MapToConnectErrorWithDetails(err, false)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeNotFound {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeNotFound)
		}
		// Should not contain debug info
		if strings.Contains(got.Message(), "debug:") {
			t.Errorf("Message should not contain debug info in non-dev mode: %q", got.Message())
		}
	})

	t.Run("isDev true includes debug info", func(t *testing.T) {
		t.Parallel()

		err := apperrors.ErrNotFound
		got := apperrors.MapToConnectErrorWithDetails(err, true)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeNotFound {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeNotFound)
		}
		if !strings.Contains(got.Message(), "debug:") {
			t.Errorf("Message should contain debug info in dev mode: %q", got.Message())
		}
		if !strings.Contains(got.Message(), "not_found") {
			t.Errorf("Message should contain original error text: %q", got.Message())
		}
	})

	t.Run("isDev true with unknown error includes debug", func(t *testing.T) {
		t.Parallel()

		err := errors.New("database connection pool exhausted")
		got := apperrors.MapToConnectErrorWithDetails(err, true)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeInternal {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeInternal)
		}
		if !strings.Contains(got.Message(), "database connection pool exhausted") {
			t.Errorf("Message should contain original error: %q", got.Message())
		}
	})

	t.Run("isDev true with conflict error includes debug", func(t *testing.T) {
		t.Parallel()

		err := apperrors.NewConflictError("automation", "locked")
		got := apperrors.MapToConnectErrorWithDetails(err, true)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Code() != connect.CodeAborted {
			t.Errorf("Code = %v, want %v", got.Code(), connect.CodeAborted)
		}
		if !strings.Contains(got.Message(), "debug:") {
			t.Errorf("Message should contain debug info: %q", got.Message())
		}
		if !strings.Contains(got.Message(), "conflict: automation - locked") {
			t.Errorf("Message should contain original conflict error: %q", got.Message())
		}
	})
}

// --- WrapAuthError tests ---

func TestWrapAuthError(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		if got := apperrors.WrapAuthError(nil, "login"); got != nil {
			t.Errorf("WrapAuthError(nil) = %v, want nil", got)
		}
	})

	t.Run("wraps error with context", func(t *testing.T) {
		t.Parallel()

		err := apperrors.ErrInvalidCredentials
		got := apperrors.WrapAuthError(err, "login handler")
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(got, apperrors.ErrInvalidCredentials) {
			t.Error("expected wrapped error to contain ErrInvalidCredentials")
		}
		if got.Error() != "login handler: invalid_credentials" {
			t.Errorf("got %q, want %q", got.Error(), "login handler: invalid_credentials")
		}
	})

	t.Run("wraps non-sentinel error", func(t *testing.T) {
		t.Parallel()

		original := errors.New("jwt parse failed")
		got := apperrors.WrapAuthError(original, "token validation")
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if got.Error() != "token validation: jwt parse failed" {
			t.Errorf("got %q", got.Error())
		}
	})
}

// --- IsAuthError tests ---

func TestIsAuthError(t *testing.T) {
	t.Parallel()

	authErrors := []struct {
		name string
		err  error
	}{
		{"ErrInvalidCredentials", apperrors.ErrInvalidCredentials},
		{"ErrSessionExpired", apperrors.ErrSessionExpired},
		{"ErrSessionNotFound", apperrors.ErrSessionNotFound},
		{"ErrInvalidSession", apperrors.ErrInvalidSession},
		{"ErrUnauthorized", apperrors.ErrUnauthorized},
	}

	for _, tt := range authErrors {
		t.Run("is auth: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if !apperrors.IsAuthError(tt.err) {
				t.Errorf("IsAuthError(%s) = false, want true", tt.name)
			}
		})
	}

	// Wrapped auth errors should also be recognized
	t.Run("wrapped auth error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("handler: %w", apperrors.ErrUnauthorized)
		if !apperrors.IsAuthError(wrapped) {
			t.Error("expected IsAuthError to return true for wrapped ErrUnauthorized")
		}
	})

	nonAuthErrors := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", apperrors.ErrNotFound},
		{"ErrPermissionDenied", apperrors.ErrPermissionDenied},
		{"ErrNotAdmin", apperrors.ErrNotAdmin},
		{"ErrEmailExists", apperrors.ErrEmailExists},
		{"ErrTimeout", apperrors.ErrTimeout},
		{"ErrRetryable", apperrors.ErrRetryable},
		{"ErrRateLimited", apperrors.ErrRateLimited},
		{"generic error", errors.New("generic")},
	}

	for _, tt := range nonAuthErrors {
		t.Run("not auth: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if apperrors.IsAuthError(tt.err) {
				t.Errorf("IsAuthError(%s) = true, want false", tt.name)
			}
		})
	}
}

// --- IsPermissionError tests ---

func TestIsPermissionError(t *testing.T) {
	t.Parallel()

	permErrors := []struct {
		name string
		err  error
	}{
		{"ErrAccountLocked", apperrors.ErrAccountLocked},
		{"ErrAccountSuspended", apperrors.ErrAccountSuspended},
		{"ErrNotAdmin", apperrors.ErrNotAdmin},
		{"ErrInsufficientRole", apperrors.ErrInsufficientRole},
		{"ErrPermissionDenied", apperrors.ErrPermissionDenied},
	}

	for _, tt := range permErrors {
		t.Run("is permission: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if !apperrors.IsPermissionError(tt.err) {
				t.Errorf("IsPermissionError(%s) = false, want true", tt.name)
			}
		})
	}

	// Wrapped permission errors should also be recognized
	t.Run("wrapped permission error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("middleware: %w", apperrors.ErrNotAdmin)
		if !apperrors.IsPermissionError(wrapped) {
			t.Error("expected IsPermissionError to return true for wrapped ErrNotAdmin")
		}
	})

	nonPermErrors := []struct {
		name string
		err  error
	}{
		{"ErrUnauthorized", apperrors.ErrUnauthorized},
		{"ErrInvalidCredentials", apperrors.ErrInvalidCredentials},
		{"ErrNotFound", apperrors.ErrNotFound},
		{"ErrTimeout", apperrors.ErrTimeout},
		{"ErrEmailExists", apperrors.ErrEmailExists},
		{"generic error", errors.New("generic")},
	}

	for _, tt := range nonPermErrors {
		t.Run("not permission: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if apperrors.IsPermissionError(tt.err) {
				t.Errorf("IsPermissionError(%s) = true, want false", tt.name)
			}
		})
	}
}

// --- IsRetryableError tests ---

func TestIsRetryableError(t *testing.T) {
	t.Parallel()

	retryableErrors := []struct {
		name string
		err  error
	}{
		{"ErrRetryable", apperrors.ErrRetryable},
		{"ErrRedisUnavailable", apperrors.ErrRedisUnavailable},
		{"ErrExternalServiceDown", apperrors.ErrExternalServiceDown},
		{"ErrTimeout", apperrors.ErrTimeout},
	}

	for _, tt := range retryableErrors {
		t.Run("is retryable: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if !apperrors.IsRetryableError(tt.err) {
				t.Errorf("IsRetryableError(%s) = false, want true", tt.name)
			}
		})
	}

	// Wrapped retryable errors should also be recognized
	t.Run("wrapped retryable error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("service: %w", apperrors.ErrRetryable)
		if !apperrors.IsRetryableError(wrapped) {
			t.Error("expected IsRetryableError to return true for wrapped ErrRetryable")
		}
	})

	nonRetryableErrors := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", apperrors.ErrNotFound},
		{"ErrUnauthorized", apperrors.ErrUnauthorized},
		{"ErrPermissionDenied", apperrors.ErrPermissionDenied},
		{"ErrEmailExists", apperrors.ErrEmailExists},
		{"ErrDuplicateEntry", apperrors.ErrDuplicateEntry},
		{"ErrInvalidCredentials", apperrors.ErrInvalidCredentials},
		{"ErrWeakPassword", apperrors.ErrWeakPassword},
		{"generic error", errors.New("generic")},
	}

	for _, tt := range nonRetryableErrors {
		t.Run("not retryable: "+tt.name, func(t *testing.T) {
			t.Parallel()

			if apperrors.IsRetryableError(tt.err) {
				t.Errorf("IsRetryableError(%s) = true, want false", tt.name)
			}
		})
	}
}
