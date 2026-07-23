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

type instagramDataDeletionRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramDataDeletionRepository creates the PostgreSQL data-deletion repository.
func NewInstagramDataDeletionRepository(db *DB, logger *zap.Logger) repository.InstagramDataDeletionRepository {
	return &instagramDataDeletionRepository{db: db, logger: logger}
}

const igDeletionColumns = `id, subject_hash, confirmation_code, status, requested_at, completed_at, expires_at`

func scanIGDeletion(row pgx.Row) (*models.InstagramDataDeletionRequest, error) {
	var d models.InstagramDataDeletionRequest
	if err := row.Scan(&d.ID, &d.SubjectHash, &d.ConfirmationCode, &d.Status,
		&d.RequestedAt, &d.CompletedAt, &d.ExpiresAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// Process executa comprovante + exclusão numa ÚNICA transação: ou a Meta recebe
// um código de confirmação com os dados realmente apagados, ou nada muda. Um
// comprovante emitido sobre uma exclusão que falhou seria uma declaração falsa.
//
// Sujeito desconhecido também recebe comprovante concluído — responder diferente
// revelaria à Meta (e a quem forjasse a chamada) se aquela conta existe aqui.
func (r *instagramDataDeletionRepository) Process(
	ctx context.Context,
	appScopedUserID, subjectHash, candidateConfirmationCode string,
	expiresAt time.Time,
) (*models.InstagramDataDeletionRequest, error) {
	if subjectHash == "" || candidateConfirmationCode == "" {
		return nil, errors.New("instagram data deletion: empty subject or confirmation code")
	}

	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op após um Commit bem-sucedido

	// 1. Comprovante. Repetição do MESMO sujeito preserva o código original (a
	//    Meta pode reconsultar o código que já recebeu) e só estende a validade.
	const upsertQuery = `
		INSERT INTO instagram_data_deletion_requests (subject_hash, confirmation_code, status, expires_at)
		VALUES ($1, $2, 'completed', $3)
		ON CONFLICT (subject_hash) DO UPDATE SET
			completed_at = now(),
			expires_at   = GREATEST(instagram_data_deletion_requests.expires_at, EXCLUDED.expires_at)
		RETURNING ` + igDeletionColumns
	receipt, err := scanIGDeletion(tx.QueryRow(ctx, upsertQuery, subjectHash, candidateConfirmationCode, expiresAt))
	if err != nil {
		return nil, err
	}

	// 2. Localizar a credencial do sujeito com FOR UPDATE, para que um callback
	//    OAuth concorrente não regrave a linha entre a leitura e o DELETE.
	if appScopedUserID != "" {
		var localUserID string
		err := tx.QueryRow(ctx,
			`SELECT user_id FROM instagram_credentials WHERE app_scoped_user_id = $1 FOR UPDATE`,
			appScopedUserID).Scan(&localUserID)
		switch {
		case err == nil:
			// 3. Ledger do dono primeiro, depois a credencial.
			if _, err := tx.Exec(ctx, `DELETE FROM instagram_story_posts WHERE user_id = $1`, localUserID); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM instagram_credentials WHERE user_id = $1`, localUserID); err != nil {
				return nil, err
			}
		case errors.Is(err, pgx.ErrNoRows):
			// Conta desconhecida: comprovante já criado, nada a apagar.
		default:
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

// GetByConfirmationCode devolve o comprovante ainda válido. Expirado é tratado
// como inexistente para a página de status responder 404 uniforme.
func (r *instagramDataDeletionRepository) GetByConfirmationCode(ctx context.Context, confirmationCode string, now time.Time) (*models.InstagramDataDeletionRequest, error) {
	if confirmationCode == "" {
		return nil, nil
	}
	const query = `
		SELECT ` + igDeletionColumns + `
		FROM instagram_data_deletion_requests
		WHERE confirmation_code = $1 AND expires_at > $2`
	d, err := scanIGDeletion(r.db.QueryRow(ctx, query, confirmationCode, now))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

func (r *instagramDataDeletionRepository) DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	const query = `
		WITH doomed AS (
			SELECT id FROM instagram_data_deletion_requests
			WHERE expires_at < $1
			ORDER BY expires_at, id
			LIMIT $2
		)
		DELETE FROM instagram_data_deletion_requests r USING doomed d WHERE r.id = d.id`
	tag, err := r.db.pool.Exec(ctx, query, before, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
