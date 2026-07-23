package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type instagramCredentialsRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramCredentialsRepository creates a PostgreSQL Instagram credentials repository.
func NewInstagramCredentialsRepository(db *DB, logger *zap.Logger) repository.InstagramCredentialsRepository {
	return &instagramCredentialsRepository{db: db, logger: logger}
}

const igCredentialColumns = `id, user_id, encrypted_data, app_scoped_user_id, ig_user_id, username, token_expires_at, created_at, updated_at`

func scanIGCredentials(row pgx.Row) (*models.InstagramCredentialsDB, error) {
	var c models.InstagramCredentialsDB
	var createdAt, updatedAt time.Time
	var expires *time.Time
	if err := row.Scan(&c.ID, &c.UserID, &c.EncryptedData, &c.AppScopedUserID, &c.IGUserID, &c.Username, &expires, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.TokenExpiresAt = expires
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &c, nil
}

func (r *instagramCredentialsRepository) Get(ctx context.Context, userID string) (*models.InstagramCredentialsDB, error) {
	const query = `SELECT ` + igCredentialColumns + ` FROM instagram_credentials WHERE user_id = $1`
	c, err := scanIGCredentials(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *instagramCredentialsRepository) GetByAppScopedUserID(ctx context.Context, appScopedUserID string) (*models.InstagramCredentialsDB, error) {
	// String vazia é o DEFAULT das linhas herdadas da 073 e casaria com várias
	// delas: nunca deve ser tratada como um sujeito válido.
	if appScopedUserID == "" {
		return nil, nil
	}
	const query = `SELECT ` + igCredentialColumns + ` FROM instagram_credentials WHERE app_scoped_user_id = $1`
	c, err := scanIGCredentials(r.db.QueryRow(ctx, query, appScopedUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *instagramCredentialsRepository) Save(ctx context.Context, userID, encryptedData, appScopedUserID, igUserID, username string, expiresAt time.Time) error {
	now := time.Now().UTC()
	const query = `
		INSERT INTO instagram_credentials (user_id, encrypted_data, app_scoped_user_id, ig_user_id, username, token_expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			encrypted_data     = EXCLUDED.encrypted_data,
			app_scoped_user_id = EXCLUDED.app_scoped_user_id,
			ig_user_id         = EXCLUDED.ig_user_id,
			username           = EXCLUDED.username,
			token_expires_at   = EXCLUDED.token_expires_at,
			updated_at         = EXCLUDED.updated_at`
	if _, err := r.db.pool.Exec(ctx, query, userID, encryptedData, appScopedUserID, igUserID, username, expiresAt, now); err != nil {
		if isIGAccountTakenErr(err) {
			// Conta da Meta já pertence a outro usuário local. Sentinel tipado
			// para o callback responder already_connected em vez de save_failed.
			return repository.ErrInstagramAccountAlreadyConnected
		}
		r.logger.Error("Failed to save Instagram credentials", zap.String("userID", userID), zap.Error(err))
		return err
	}
	return nil
}

// isIGAccountTakenErr reconhece SOMENTE as duas violações de unicidade que
// significam "esta conta do Instagram já está ligada a outro usuário". Qualquer
// outro 23505 (inclusive um índice novo criado no futuro) continua subindo cru:
// mascarar violação de schema como conflito de conta esconderia bug real.
func isIGAccountTakenErr(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return pgErr.ConstraintName == "uq_instagram_credentials_app_scoped_user_id" ||
		pgErr.ConstraintName == "uq_instagram_credentials_ig_user_id"
}

func (r *instagramCredentialsRepository) Delete(ctx context.Context, userID string) error {
	if _, err := r.db.pool.Exec(ctx, `DELETE FROM instagram_credentials WHERE user_id = $1`, userID); err != nil {
		r.logger.Error("Failed to delete Instagram credentials", zap.String("userID", userID), zap.Error(err))
		return err
	}
	return nil
}

func (r *instagramCredentialsRepository) DeleteByAppScopedUserID(ctx context.Context, appScopedUserID string) error {
	if appScopedUserID == "" {
		return nil
	}
	if _, err := r.db.pool.Exec(ctx, `DELETE FROM instagram_credentials WHERE app_scoped_user_id = $1`, appScopedUserID); err != nil {
		// O id app-scoped não entra no log: é identificador da conta na Meta.
		r.logger.Error("Failed to delete Instagram credentials by app-scoped id", zap.Error(err))
		return err
	}
	return nil
}

func (r *instagramCredentialsRepository) ListExpiring(ctx context.Context, before time.Time) ([]models.InstagramCredentialsDB, error) {
	const query = `
		SELECT ` + igCredentialColumns + `
		FROM instagram_credentials
		WHERE token_expires_at IS NOT NULL AND token_expires_at < $1`
	rows, err := r.db.Query(ctx, query, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.InstagramCredentialsDB
	for rows.Next() {
		c, err := scanIGCredentials(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
