package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes (SQLSTATE)
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	// Class 23 - Integrity Constraint Violation
	PGCodeUniqueViolation     = "23505"
	PGCodeForeignKeyViolation = "23503"
	PGCodeNotNullViolation    = "23502"
	PGCodeCheckViolation      = "23514"

	// Class 40 - Transaction Rollback
	PGCodeSerializationFailure = "40001"
	PGCodeDeadlockDetected     = "40P01"

	// Class 57 - Operator Intervention
	PGCodeQueryCanceled       = "57014"
	PGCodeAdminShutdown       = "57P01"
	PGCodeCrashShutdown       = "57P02"
	PGCodeCannotConnectNow    = "57P03"
	PGCodeDatabaseDropped     = "57P04"
	PGCodeIdleSessionTimeout  = "57P05"
	PGCodeIdleInTxnTimeout    = "57P06"

	// Class P0 - PL/pgSQL Error
	PGCodeRaiseException = "P0001"
	PGCodeNoDataFound    = "P0002"
	PGCodeTooManyRows    = "P0003"

	// Class 08 - Connection Exception
	PGCodeConnectionException = "08000"
	PGCodeConnectionDoesNotExist = "08003"
	PGCodeConnectionFailure   = "08006"

	// Class 53 - Insufficient Resources
	PGCodeDiskFull    = "53100"
	PGCodeOutOfMemory = "53200"
	PGCodeTooManyConnections = "53300"
)

// MapPostgresError converts PostgreSQL errors to domain errors.
// This function examines the SQLSTATE code and constraint names
// to determine the appropriate domain error.
func MapPostgresError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context errors first
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("operation canceled: %w", err)
	}

	// Check for PostgreSQL specific errors
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		// Not a PostgreSQL error, return as-is
		return err
	}

	return mapPgError(pgErr)
}

// mapPgError maps a pgconn.PgError to a domain error.
func mapPgError(pgErr *pgconn.PgError) error {
	switch pgErr.Code {
	// Unique violation (duplicate entry)
	case PGCodeUniqueViolation:
		return mapUniqueViolation(pgErr)

	// Foreign key violation
	case PGCodeForeignKeyViolation:
		return mapForeignKeyViolation(pgErr)

	// Not null violation
	case PGCodeNotNullViolation:
		return fmt.Errorf("required field missing: %s", pgErr.ColumnName)

	// Check constraint violation
	case PGCodeCheckViolation:
		return fmt.Errorf("validation failed: %s", pgErr.ConstraintName)

	// Serialization failure (concurrent update conflict)
	case PGCodeSerializationFailure, PGCodeDeadlockDetected:
		return ErrRetryable

	// Query canceled (timeout)
	case PGCodeQueryCanceled:
		return ErrTimeout

	// Raised exception from PL/pgSQL (custom errors)
	case PGCodeRaiseException:
		return mapRaisedException(pgErr)

	// No data found
	case PGCodeNoDataFound:
		return ErrNotFound

	// Connection errors
	case PGCodeConnectionException, PGCodeConnectionDoesNotExist, PGCodeConnectionFailure:
		return ErrRetryable

	// Resource errors
	case PGCodeDiskFull, PGCodeOutOfMemory, PGCodeTooManyConnections:
		return fmt.Errorf("database resource exhausted: %w", ErrRetryable)

	// Admin/shutdown errors
	case PGCodeAdminShutdown, PGCodeCrashShutdown, PGCodeDatabaseDropped:
		return fmt.Errorf("database unavailable: %w", ErrRetryable)

	// Idle timeout
	case PGCodeIdleSessionTimeout, PGCodeIdleInTxnTimeout:
		return ErrRetryable

	default:
		// Return original error wrapped with code info
		return fmt.Errorf("database error [%s]: %s", pgErr.Code, pgErr.Message)
	}
}

// mapUniqueViolation maps unique constraint violations to specific domain errors.
func mapUniqueViolation(pgErr *pgconn.PgError) error {
	constraint := strings.ToLower(pgErr.ConstraintName)
	detail := strings.ToLower(pgErr.Detail)

	// Check for email uniqueness
	if strings.Contains(constraint, "email") ||
		strings.Contains(detail, "email") {
		return ErrEmailExists
	}

	// Check for user uniqueness
	if strings.Contains(constraint, "user") {
		return ErrDuplicateEntry
	}

	// Check for session uniqueness
	if strings.Contains(constraint, "session") {
		return ErrDuplicateEntry
	}

	// Generic duplicate
	return fmt.Errorf("%w: %s", ErrDuplicateEntry, pgErr.ConstraintName)
}

// mapForeignKeyViolation maps foreign key violations to domain errors.
func mapForeignKeyViolation(pgErr *pgconn.PgError) error {
	constraint := strings.ToLower(pgErr.ConstraintName)

	// Check for user reference
	if strings.Contains(constraint, "user") ||
		strings.Contains(constraint, "profile") {
		return ErrUserNotFound
	}

	// Generic invalid reference
	return fmt.Errorf("%w: %s", ErrInvalidReference, pgErr.ConstraintName)
}

// mapRaisedException maps custom PL/pgSQL exceptions to domain errors.
// These are exceptions raised explicitly in database functions.
func mapRaisedException(pgErr *pgconn.PgError) error {
	message := strings.ToLower(pgErr.Message)

	// Check for specific error messages from database functions
	if strings.Contains(message, "not found") {
		return ErrNotFound
	}
	if strings.Contains(message, "unauthorized") {
		return ErrUnauthorized
	}
	if strings.Contains(message, "invalid") {
		return fmt.Errorf("invalid operation: %s", pgErr.Message)
	}

	// Return the message as error
	return fmt.Errorf("database error: %s", pgErr.Message)
}

// IsPostgresError checks if an error is a PostgreSQL error.
func IsPostgresError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr)
}

// GetPostgresCode extracts the SQLSTATE code from a PostgreSQL error.
// Returns empty string if not a PostgreSQL error.
func GetPostgresCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsRetryablePostgresError checks if a PostgreSQL error is retryable.
func IsRetryablePostgresError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case PGCodeSerializationFailure,
		PGCodeDeadlockDetected,
		PGCodeConnectionException,
		PGCodeConnectionDoesNotExist,
		PGCodeConnectionFailure,
		PGCodeIdleSessionTimeout,
		PGCodeIdleInTxnTimeout:
		return true
	}
	return false
}

// WrapPostgresError wraps a PostgreSQL error with context.
func WrapPostgresError(err error, operation string) error {
	if err == nil {
		return nil
	}

	mapped := MapPostgresError(err)

	// If mapping changed the error, return it directly
	if mapped != err {
		return fmt.Errorf("%s: %w", operation, mapped)
	}

	// Otherwise wrap with operation context
	return fmt.Errorf("%s failed: %w", operation, err)
}
