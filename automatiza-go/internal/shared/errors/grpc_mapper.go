package errors

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
)

// MapToConnectError maps domain errors to ConnectRPC errors with appropriate codes.
// This ensures consistent error responses across all RPC handlers.
func MapToConnectError(err error) *connect.Error {
	if err == nil {
		return nil
	}

	// Check if it's already a connect.Error
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr
	}

	// Map domain errors to gRPC codes
	switch {
	// Authentication errors -> Unauthenticated
	case errors.Is(err, ErrInvalidCredentials):
		return connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("Email ou senha inválidos"))

	case errors.Is(err, ErrSessionExpired),
		errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrInvalidSession):
		return connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("Sessão expirada. Faça login novamente"))

	case errors.Is(err, ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("Autenticação necessária"))

	// Permission errors -> PermissionDenied
	case errors.Is(err, ErrAccountLocked):
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("Conta bloqueada por tentativas excessivas. Tente novamente em 30 minutos"))

	case errors.Is(err, ErrAccountSuspended):
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("Sua conta está suspensa por pendência de pagamento. Entre em contato com o suporte"))

	case errors.Is(err, ErrNotAdmin),
		errors.Is(err, ErrInsufficientRole),
		errors.Is(err, ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("Você não tem permissão para esta ação"))

	// Rate limiting -> ResourceExhausted
	case errors.Is(err, ErrRateLimited):
		return connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("Muitas tentativas. Aguarde alguns minutos"))

	case errors.Is(err, ErrIPBlocked):
		return connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("Acesso temporariamente bloqueado"))

	// Validation errors -> InvalidArgument
	case errors.Is(err, ErrWeakPassword):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Senha muito fraca. Use pelo menos 8 caracteres com letras e números"))

	case errors.Is(err, ErrInvalidEmail):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Email inválido"))

	case errors.Is(err, ErrInvalidToken),
		errors.Is(err, ErrTokenExpired),
		errors.Is(err, ErrTokenUsed):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Link inválido ou expirado. Solicite um novo"))

	case errors.Is(err, ErrInvalidCSRF):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Token de segurança inválido. Recarregue a página"))

	case errors.Is(err, ErrInvalidUserID):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("ID de usuário inválido"))

	case errors.Is(err, ErrSelfPasswordReset):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Use a opção 'Alterar senha' para alterar sua própria senha"))

	// Conflict errors -> AlreadyExists
	case errors.Is(err, ErrEmailExists):
		return connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("Este email já está cadastrado"))

	case errors.Is(err, ErrDuplicateEntry):
		return connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("Registro já existe"))

	// Not found errors -> NotFound
	case errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrNotFound):
		return connect.NewError(connect.CodeNotFound,
			fmt.Errorf("Registro não encontrado"))

	// Precondition errors -> FailedPrecondition
	case errors.Is(err, ErrEmailNotVerified):
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("Verifique seu email antes de continuar"))

	// Retryable errors -> Unavailable
	case errors.Is(err, ErrRetryable),
		errors.Is(err, ErrRedisUnavailable),
		errors.Is(err, ErrExternalServiceDown):
		return connect.NewError(connect.CodeUnavailable,
			fmt.Errorf("Serviço temporariamente indisponível. Tente novamente"))

	// Timeout -> DeadlineExceeded
	case errors.Is(err, ErrTimeout):
		return connect.NewError(connect.CodeDeadlineExceeded,
			fmt.Errorf("Operação demorou demais. Tente novamente"))

	// Reference errors -> InvalidArgument
	case errors.Is(err, ErrInvalidReference):
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("Referência inválida"))

	// Email errors -> Internal (but with safe message)
	case errors.Is(err, ErrEmailSendFailed):
		return connect.NewError(connect.CodeInternal,
			fmt.Errorf("Falha ao enviar email. Tente novamente mais tarde"))

	// Default -> Internal
	default:
		// Log the original error for debugging, but return safe message
		return connect.NewError(connect.CodeInternal,
			fmt.Errorf("Erro interno. Tente novamente ou entre em contato com o suporte"))
	}
}

// MapToConnectErrorWithDetails maps errors and includes the original error message for debugging.
// Use this only in development mode as it exposes error details.
func MapToConnectErrorWithDetails(err error, isDev bool) *connect.Error {
	connectErr := MapToConnectError(err)
	if connectErr == nil {
		return nil
	}

	if isDev && err != nil {
		// In development, wrap the connect error with original error context
		// Note: For full proto-based error details, define a proper proto message
		return connect.NewError(
			connectErr.Code(),
			fmt.Errorf("%s (debug: %v)", connectErr.Message(), err),
		)
	}

	return connectErr
}

// Helper functions for common error patterns

// WrapAuthError wraps an error with authentication context.
func WrapAuthError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// IsAuthError checks if an error is an authentication-related error.
func IsAuthError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrSessionExpired) ||
		errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, ErrInvalidSession) ||
		errors.Is(err, ErrUnauthorized)
}

// IsPermissionError checks if an error is a permission-related error.
func IsPermissionError(err error) bool {
	return errors.Is(err, ErrAccountLocked) ||
		errors.Is(err, ErrAccountSuspended) ||
		errors.Is(err, ErrNotAdmin) ||
		errors.Is(err, ErrInsufficientRole) ||
		errors.Is(err, ErrPermissionDenied)
}

// IsRetryableError checks if an error indicates the operation should be retried.
func IsRetryableError(err error) bool {
	return errors.Is(err, ErrRetryable) ||
		errors.Is(err, ErrRedisUnavailable) ||
		errors.Is(err, ErrExternalServiceDown) ||
		errors.Is(err, ErrTimeout)
}
