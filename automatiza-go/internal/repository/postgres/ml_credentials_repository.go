package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type mlCredentialsRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMLCredentialsRepository creates a new PostgreSQL ML credentials repository
func NewMLCredentialsRepository(db *DB, logger *zap.Logger) repository.MLCredentialsRepository {
	return &mlCredentialsRepository{
		db:     db,
		logger: logger,
	}
}

func (r *mlCredentialsRepository) Get(ctx context.Context, userID string) (*models.MLCredentialsDB, error) {
	query := `
		SELECT id, user_id, encrypted_data, created_at, updated_at
		FROM ml_credentials
		WHERE user_id = $1
	`

	var creds models.MLCredentialsDB
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&creds.ID, &creds.UserID, &creds.EncryptedData, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	creds.CreatedAt = createdAt.Format(time.RFC3339)
	creds.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &creds, nil
}

func (r *mlCredentialsRepository) Save(ctx context.Context, userID, encryptedData string) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO ml_credentials (user_id, encrypted_data, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			encrypted_data = EXCLUDED.encrypted_data,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.pool.Exec(ctx, query, userID, encryptedData, now, now)
	if err != nil {
		r.logger.Error("Failed to save ML credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

// ListForHealthCheck devolve as credenciais que estão há mais tempo sem
// verificação (nunca verificadas primeiro, via NULLS FIRST).
//
// Pula quem já está 'expired': uma sessão morta não ressuscita sozinha, então
// re-checá-la só gastaria request autenticada contra o endpoint mais sensível
// do ML. Ela volta à fila quando o cliente reconecta (o Save reseta o estado).
//
// NOTA: seleciona as colunas da migração 084. É chamado APENAS pelo cron de
// health check, que está atrás de ML_SESSION_HEALTH_ENABLED (default false) —
// então, se o binário subir antes da migração aplicar, nada aqui é executado.
// O Get() de leitura normal foi deixado intocado de propósito, justamente para
// não depender das colunas novas no caminho quente.
func (r *mlCredentialsRepository) ListForHealthCheck(ctx context.Context, olderThan time.Time, limit int) ([]models.MLCredentialsDB, error) {
	query := `
		SELECT id, user_id, encrypted_data, session_status,
		       COALESCE(last_verified_at, 'epoch'::timestamptz), discovered_tag
		FROM ml_credentials
		WHERE session_status <> 'expired'
		  AND (last_verified_at IS NULL OR last_verified_at < $1)
		ORDER BY last_verified_at NULLS FIRST
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.MLCredentialsDB
	for rows.Next() {
		var c models.MLCredentialsDB
		var lastVerified time.Time
		if err := rows.Scan(&c.ID, &c.UserID, &c.EncryptedData, &c.SessionStatus, &lastVerified, &c.DiscoveredTag); err != nil {
			return nil, err
		}
		c.LastVerifiedAt = lastVerified.Format(time.RFC3339)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateSessionStatus grava o resultado de uma verificação de sessão.
//
// NÃO mexe em updated_at: essa coluna significa "quando a credencial mudou", e
// carimbá-la a cada health check apagaria a informação de há quanto tempo o
// cliente não reconecta — que é exatamente o que se olha num incidente.
func (r *mlCredentialsRepository) UpdateSessionStatus(ctx context.Context, userID, status, discoveredTag string) error {
	// Tag vazia não sobrescreve a conhecida: uma verificação que não conseguiu
	// parsear o formato não pode apagar uma tag boa.
	query := `
		UPDATE ml_credentials
		SET session_status = $2,
		    last_verified_at = now(),
		    discovered_tag = CASE WHEN $3 = '' THEN discovered_tag ELSE $3 END
		WHERE user_id = $1
	`

	_, err := r.db.pool.Exec(ctx, query, userID, status, discoveredTag)
	if err != nil {
		r.logger.Error("Failed to update ML session status",
			zap.String("userID", userID), zap.Error(err))
		return err
	}
	return nil
}

// Delete removes ML credentials for a user
func (r *mlCredentialsRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM ml_credentials WHERE user_id = $1`

	_, err := r.db.pool.Exec(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete ML credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

// Exists checks if ML credentials exist for a user
func (r *mlCredentialsRepository) Exists(ctx context.Context, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM ml_credentials WHERE user_id = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
