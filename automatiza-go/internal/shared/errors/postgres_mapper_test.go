package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// --- MapPostgresError tests ---

func TestMapPostgresError(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		if got := MapPostgresError(nil); got != nil {
			t.Errorf("MapPostgresError(nil) = %v, want nil", got)
		}
	})

	t.Run("context.DeadlineExceeded returns ErrTimeout", func(t *testing.T) {
		t.Parallel()

		got := MapPostgresError(context.DeadlineExceeded)
		if !errors.Is(got, ErrTimeout) {
			t.Errorf("MapPostgresError(DeadlineExceeded) = %v, want ErrTimeout", got)
		}
	})

	t.Run("wrapped context.DeadlineExceeded returns ErrTimeout", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("query: %w", context.DeadlineExceeded)
		got := MapPostgresError(wrapped)
		if !errors.Is(got, ErrTimeout) {
			t.Errorf("MapPostgresError(wrapped DeadlineExceeded) = %v, want ErrTimeout", got)
		}
	})

	t.Run("context.Canceled wraps original error", func(t *testing.T) {
		t.Parallel()

		got := MapPostgresError(context.Canceled)
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(got, context.Canceled) {
			t.Errorf("expected error to wrap context.Canceled, got %v", got)
		}
		if got.Error() != "operation canceled: context canceled" {
			t.Errorf("unexpected error message: %s", got.Error())
		}
	})

	t.Run("non-PG error returned as-is", func(t *testing.T) {
		t.Parallel()

		original := errors.New("some random error")
		got := MapPostgresError(original)
		if got != original {
			t.Errorf("expected original error returned, got %v", got)
		}
	})

	// Table-driven PG error code tests
	pgCodeTests := []struct {
		name           string
		code           string
		constraintName string
		detail         string
		message        string
		columnName     string
		checkFn        func(t *testing.T, err error)
	}{
		{
			name:           "unique violation with email constraint",
			code:           PGCodeUniqueViolation,
			constraintName: "users_email_unique",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrEmailExists) {
					t.Errorf("expected ErrEmailExists, got %v", err)
				}
			},
		},
		{
			name:           "unique violation with email in detail",
			code:           PGCodeUniqueViolation,
			constraintName: "some_other_constraint",
			detail:         "Key (email)=(test@test.com) already exists",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrEmailExists) {
					t.Errorf("expected ErrEmailExists, got %v", err)
				}
			},
		},
		{
			name:           "unique violation with user constraint",
			code:           PGCodeUniqueViolation,
			constraintName: "user_roles_unique",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrDuplicateEntry) {
					t.Errorf("expected ErrDuplicateEntry, got %v", err)
				}
			},
		},
		{
			name:           "unique violation with session constraint",
			code:           PGCodeUniqueViolation,
			constraintName: "session_token_unique",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrDuplicateEntry) {
					t.Errorf("expected ErrDuplicateEntry, got %v", err)
				}
			},
		},
		{
			name:           "unique violation generic constraint",
			code:           PGCodeUniqueViolation,
			constraintName: "automations_slug_unique",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrDuplicateEntry) {
					t.Errorf("expected ErrDuplicateEntry, got %v", err)
				}
				if err.Error() != "duplicate_entry: automations_slug_unique" {
					t.Errorf("unexpected message: %s", err.Error())
				}
			},
		},
		{
			name:           "foreign key violation with user constraint",
			code:           PGCodeForeignKeyViolation,
			constraintName: "fk_user_id",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrUserNotFound) {
					t.Errorf("expected ErrUserNotFound, got %v", err)
				}
			},
		},
		{
			name:           "foreign key violation with profile constraint",
			code:           PGCodeForeignKeyViolation,
			constraintName: "fk_profile_id",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrUserNotFound) {
					t.Errorf("expected ErrUserNotFound, got %v", err)
				}
			},
		},
		{
			name:           "foreign key violation generic constraint",
			code:           PGCodeForeignKeyViolation,
			constraintName: "fk_automation_id",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrInvalidReference) {
					t.Errorf("expected ErrInvalidReference, got %v", err)
				}
				if err.Error() != "invalid_reference: fk_automation_id" {
					t.Errorf("unexpected message: %s", err.Error())
				}
			},
		},
		{
			name:       "not null violation",
			code:       PGCodeNotNullViolation,
			columnName: "email",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if err.Error() != "required field missing: email" {
					t.Errorf("unexpected message: %s", err.Error())
				}
			},
		},
		{
			name:           "check violation",
			code:           PGCodeCheckViolation,
			constraintName: "positive_amount",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if err.Error() != "validation failed: positive_amount" {
					t.Errorf("unexpected message: %s", err.Error())
				}
			},
		},
		{
			name: "serialization failure",
			code: PGCodeSerializationFailure,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "deadlock detected",
			code: PGCodeDeadlockDetected,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "query canceled",
			code: PGCodeQueryCanceled,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrTimeout) {
					t.Errorf("expected ErrTimeout, got %v", err)
				}
			},
		},
		{
			name:    "raise exception with not found",
			code:    PGCodeRaiseException,
			message: "Record not found",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("expected ErrNotFound, got %v", err)
				}
			},
		},
		{
			name:    "raise exception with unauthorized",
			code:    PGCodeRaiseException,
			message: "Unauthorized access",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("expected ErrUnauthorized, got %v", err)
				}
			},
		},
		{
			name:    "raise exception with invalid",
			code:    PGCodeRaiseException,
			message: "Invalid input parameter",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				expected := "invalid operation: Invalid input parameter"
				if err.Error() != expected {
					t.Errorf("got %q, want %q", err.Error(), expected)
				}
			},
		},
		{
			name:    "raise exception generic message",
			code:    PGCodeRaiseException,
			message: "Something unexpected happened",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				expected := "database error: Something unexpected happened"
				if err.Error() != expected {
					t.Errorf("got %q, want %q", err.Error(), expected)
				}
			},
		},
		{
			name: "no data found",
			code: PGCodeNoDataFound,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("expected ErrNotFound, got %v", err)
				}
			},
		},
		{
			name: "connection exception",
			code: PGCodeConnectionException,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "connection does not exist",
			code: PGCodeConnectionDoesNotExist,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "connection failure",
			code: PGCodeConnectionFailure,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "disk full",
			code: PGCodeDiskFull,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
				expected := "database resource exhausted: retryable"
				if err.Error() != expected {
					t.Errorf("got %q, want %q", err.Error(), expected)
				}
			},
		},
		{
			name: "out of memory",
			code: PGCodeOutOfMemory,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "too many connections",
			code: PGCodeTooManyConnections,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "admin shutdown",
			code: PGCodeAdminShutdown,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
				expected := "database unavailable: retryable"
				if err.Error() != expected {
					t.Errorf("got %q, want %q", err.Error(), expected)
				}
			},
		},
		{
			name: "crash shutdown",
			code: PGCodeCrashShutdown,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "database dropped",
			code: PGCodeDatabaseDropped,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected wrapped ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "idle session timeout",
			code: PGCodeIdleSessionTimeout,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name: "idle in transaction timeout",
			code: PGCodeIdleInTxnTimeout,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, ErrRetryable) {
					t.Errorf("expected ErrRetryable, got %v", err)
				}
			},
		},
		{
			name:    "unknown code falls through to default",
			code:    "99999",
			message: "unknown error",
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				expected := "database error [99999]: unknown error"
				if err.Error() != expected {
					t.Errorf("got %q, want %q", err.Error(), expected)
				}
			},
		},
		{
			name: "too many rows",
			code: PGCodeTooManyRows,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				// PGCodeTooManyRows (P0003) is not explicitly handled,
				// should fall through to default
				if err == nil {
					t.Fatal("expected non-nil error")
				}
			},
		},
		{
			name: "cannot connect now",
			code: PGCodeCannotConnectNow,
			checkFn: func(t *testing.T, err error) {
				t.Helper()
				// PGCodeCannotConnectNow (57P03) is not explicitly handled in the switch,
				// should fall through to default
				if err == nil {
					t.Fatal("expected non-nil error")
				}
			},
		},
	}

	for _, tt := range pgCodeTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{
				Code:           tt.code,
				ConstraintName: tt.constraintName,
				Detail:         tt.detail,
				Message:        tt.message,
				ColumnName:     tt.columnName,
			}
			got := MapPostgresError(pgErr)
			if got == nil {
				t.Fatal("expected non-nil error")
			}
			tt.checkFn(t, got)
		})
	}
}

// --- mapUniqueViolation tests (internal) ---

func TestMapUniqueViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		constraintName string
		detail         string
		wantErr        error
		wantExact      string
	}{
		{
			name:           "email in constraint name",
			constraintName: "users_email_key",
			wantErr:        ErrEmailExists,
		},
		{
			name:           "EMAIL uppercase in constraint name",
			constraintName: "USERS_EMAIL_KEY",
			wantErr:        ErrEmailExists,
		},
		{
			name:           "email in detail",
			constraintName: "some_constraint",
			detail:         "Key (email)=(user@example.com) already exists.",
			wantErr:        ErrEmailExists,
		},
		{
			name:           "user in constraint name",
			constraintName: "user_roles_pkey",
			wantErr:        ErrDuplicateEntry,
		},
		{
			name:           "session in constraint name",
			constraintName: "session_token_unique",
			wantErr:        ErrDuplicateEntry,
		},
		{
			name:           "generic constraint",
			constraintName: "automations_name_key",
			wantErr:        ErrDuplicateEntry,
			wantExact:      "duplicate_entry: automations_name_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{
				ConstraintName: tt.constraintName,
				Detail:         tt.detail,
			}
			got := mapUniqueViolation(pgErr)
			if !errors.Is(got, tt.wantErr) {
				t.Errorf("mapUniqueViolation() = %v, want %v", got, tt.wantErr)
			}
			if tt.wantExact != "" && got.Error() != tt.wantExact {
				t.Errorf("mapUniqueViolation().Error() = %q, want %q", got.Error(), tt.wantExact)
			}
		})
	}
}

// --- mapForeignKeyViolation tests (internal) ---

func TestMapForeignKeyViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		constraintName string
		wantErr        error
		wantExact      string
	}{
		{
			name:           "user constraint",
			constraintName: "fk_user_id",
			wantErr:        ErrUserNotFound,
		},
		{
			name:           "profile constraint",
			constraintName: "fk_profile_id",
			wantErr:        ErrUserNotFound,
		},
		{
			name:           "user constraint uppercase",
			constraintName: "FK_USER_AUTOMATION",
			wantErr:        ErrUserNotFound,
		},
		{
			name:           "generic constraint",
			constraintName: "fk_automation_id",
			wantErr:        ErrInvalidReference,
			wantExact:      "invalid_reference: fk_automation_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{
				ConstraintName: tt.constraintName,
			}
			got := mapForeignKeyViolation(pgErr)
			if !errors.Is(got, tt.wantErr) {
				t.Errorf("mapForeignKeyViolation() = %v, want %v", got, tt.wantErr)
			}
			if tt.wantExact != "" && got.Error() != tt.wantExact {
				t.Errorf("mapForeignKeyViolation().Error() = %q, want %q", got.Error(), tt.wantExact)
			}
		})
	}
}

// --- mapRaisedException tests (internal) ---

func TestMapRaisedException(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		message   string
		wantErr   error
		wantExact string
	}{
		{
			name:    "not found message",
			message: "User not found in database",
			wantErr: ErrNotFound,
		},
		{
			name:    "NOT FOUND uppercase",
			message: "Record NOT FOUND",
			wantErr: ErrNotFound,
		},
		{
			name:    "unauthorized message",
			message: "Unauthorized access to resource",
			wantErr: ErrUnauthorized,
		},
		{
			name:      "invalid message",
			message:   "Invalid input parameter",
			wantExact: "invalid operation: Invalid input parameter",
		},
		{
			name:      "generic message",
			message:   "Something unexpected happened",
			wantExact: "database error: Something unexpected happened",
		},
		{
			name:      "empty message",
			message:   "",
			wantExact: "database error: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{
				Message: tt.message,
			}
			got := mapRaisedException(pgErr)
			if tt.wantErr != nil && !errors.Is(got, tt.wantErr) {
				t.Errorf("mapRaisedException() = %v, want %v", got, tt.wantErr)
			}
			if tt.wantExact != "" && got.Error() != tt.wantExact {
				t.Errorf("mapRaisedException().Error() = %q, want %q", got.Error(), tt.wantExact)
			}
		})
	}
}

// --- IsPostgresError tests ---

func TestIsPostgresError(t *testing.T) {
	t.Parallel()

	t.Run("with PgError", func(t *testing.T) {
		t.Parallel()

		pgErr := &pgconn.PgError{Code: "23505"}
		if !IsPostgresError(pgErr) {
			t.Error("expected IsPostgresError to return true for PgError")
		}
	})

	t.Run("with wrapped PgError", func(t *testing.T) {
		t.Parallel()

		pgErr := &pgconn.PgError{Code: "23505"}
		wrapped := fmt.Errorf("query failed: %w", pgErr)
		if !IsPostgresError(wrapped) {
			t.Error("expected IsPostgresError to return true for wrapped PgError")
		}
	})

	t.Run("with non-PG error", func(t *testing.T) {
		t.Parallel()

		if IsPostgresError(errors.New("not a pg error")) {
			t.Error("expected IsPostgresError to return false for non-PG error")
		}
	})

	t.Run("with nil", func(t *testing.T) {
		t.Parallel()

		if IsPostgresError(nil) {
			t.Error("expected IsPostgresError to return false for nil")
		}
	})
}

// --- GetPostgresCode tests ---

func TestGetPostgresCode(t *testing.T) {
	t.Parallel()

	t.Run("with PgError", func(t *testing.T) {
		t.Parallel()

		pgErr := &pgconn.PgError{Code: "23505"}
		if got := GetPostgresCode(pgErr); got != "23505" {
			t.Errorf("GetPostgresCode() = %q, want %q", got, "23505")
		}
	})

	t.Run("with wrapped PgError", func(t *testing.T) {
		t.Parallel()

		pgErr := &pgconn.PgError{Code: "40001"}
		wrapped := fmt.Errorf("wrapped: %w", pgErr)
		if got := GetPostgresCode(wrapped); got != "40001" {
			t.Errorf("GetPostgresCode() = %q, want %q", got, "40001")
		}
	})

	t.Run("with non-PG error", func(t *testing.T) {
		t.Parallel()

		if got := GetPostgresCode(errors.New("not pg")); got != "" {
			t.Errorf("GetPostgresCode() = %q, want empty string", got)
		}
	})

	t.Run("with nil", func(t *testing.T) {
		t.Parallel()

		if got := GetPostgresCode(nil); got != "" {
			t.Errorf("GetPostgresCode(nil) = %q, want empty string", got)
		}
	})
}

// --- IsRetryablePostgresError tests ---

func TestIsRetryablePostgresError(t *testing.T) {
	t.Parallel()

	retryableCodes := []struct {
		name string
		code string
	}{
		{"serialization failure", PGCodeSerializationFailure},
		{"deadlock detected", PGCodeDeadlockDetected},
		{"connection exception", PGCodeConnectionException},
		{"connection does not exist", PGCodeConnectionDoesNotExist},
		{"connection failure", PGCodeConnectionFailure},
		{"idle session timeout", PGCodeIdleSessionTimeout},
		{"idle in transaction timeout", PGCodeIdleInTxnTimeout},
	}

	for _, tt := range retryableCodes {
		t.Run("retryable: "+tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{Code: tt.code}
			if !IsRetryablePostgresError(pgErr) {
				t.Errorf("IsRetryablePostgresError(%s) = false, want true", tt.code)
			}
		})
	}

	nonRetryableCodes := []struct {
		name string
		code string
	}{
		{"unique violation", PGCodeUniqueViolation},
		{"foreign key violation", PGCodeForeignKeyViolation},
		{"not null violation", PGCodeNotNullViolation},
		{"check violation", PGCodeCheckViolation},
		{"query canceled", PGCodeQueryCanceled},
		{"raise exception", PGCodeRaiseException},
		{"no data found", PGCodeNoDataFound},
		{"too many rows", PGCodeTooManyRows},
		{"disk full", PGCodeDiskFull},
		{"out of memory", PGCodeOutOfMemory},
		{"too many connections", PGCodeTooManyConnections},
		{"admin shutdown", PGCodeAdminShutdown},
		{"crash shutdown", PGCodeCrashShutdown},
		{"database dropped", PGCodeDatabaseDropped},
		{"cannot connect now", PGCodeCannotConnectNow},
		{"unknown code", "99999"},
	}

	for _, tt := range nonRetryableCodes {
		t.Run("non-retryable: "+tt.name, func(t *testing.T) {
			t.Parallel()

			pgErr := &pgconn.PgError{Code: tt.code}
			if IsRetryablePostgresError(pgErr) {
				t.Errorf("IsRetryablePostgresError(%s) = true, want false", tt.code)
			}
		})
	}

	t.Run("non-PG error returns false", func(t *testing.T) {
		t.Parallel()

		if IsRetryablePostgresError(errors.New("not pg")) {
			t.Error("expected false for non-PG error")
		}
	})

	t.Run("nil returns false", func(t *testing.T) {
		t.Parallel()

		if IsRetryablePostgresError(nil) {
			t.Error("expected false for nil")
		}
	})
}

// --- WrapPostgresError tests ---

func TestWrapPostgresError(t *testing.T) {
	t.Parallel()

	t.Run("nil error returns nil", func(t *testing.T) {
		t.Parallel()

		if got := WrapPostgresError(nil, "create user"); got != nil {
			t.Errorf("WrapPostgresError(nil) = %v, want nil", got)
		}
	})

	t.Run("mapped PG error wraps with operation context", func(t *testing.T) {
		t.Parallel()

		pgErr := &pgconn.PgError{
			Code:           PGCodeUniqueViolation,
			ConstraintName: "users_email_key",
		}
		got := WrapPostgresError(pgErr, "create user")
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		// The mapped error is ErrEmailExists, wrapped with operation
		if !errors.Is(got, ErrEmailExists) {
			t.Errorf("expected error to wrap ErrEmailExists, got %v", got)
		}
		if got.Error() != "create user: email_exists" {
			t.Errorf("unexpected message: %q", got.Error())
		}
	})

	t.Run("non-PG error wraps with operation failed", func(t *testing.T) {
		t.Parallel()

		original := errors.New("some random error")
		got := WrapPostgresError(original, "query data")
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		// Non-PG error is returned as-is from MapPostgresError, so mapped == err
		// and WrapPostgresError wraps with "operation failed" format
		if got.Error() != "query data failed: some random error" {
			t.Errorf("unexpected message: %q", got.Error())
		}
	})

	t.Run("context deadline exceeded wraps with operation context", func(t *testing.T) {
		t.Parallel()

		got := WrapPostgresError(context.DeadlineExceeded, "fetch records")
		if got == nil {
			t.Fatal("expected non-nil error")
		}
		if !errors.Is(got, ErrTimeout) {
			t.Errorf("expected error to wrap ErrTimeout, got %v", got)
		}
		if got.Error() != "fetch records: timeout" {
			t.Errorf("unexpected message: %q", got.Error())
		}
	})
}
