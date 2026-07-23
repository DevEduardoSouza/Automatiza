package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type instagramAppRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramAppRepository creates the PostgreSQL per-user Meta app repository.
func NewInstagramAppRepository(db *DB, logger *zap.Logger) repository.InstagramAppRepository {
	return &instagramAppRepository{db: db, logger: logger}
}

const igAppColumns = `id, user_id, app_id, encrypted_app_secret, webhook_token, verify_token,
	is_verified, verified_at, created_at, updated_at`

func scanIGApp(row pgx.Row) (*models.InstagramApp, error) {
	var a models.InstagramApp
	var createdAt, updatedAt time.Time
	var verifiedAt *time.Time
	if err := row.Scan(&a.ID, &a.UserID, &a.AppID, &a.EncryptedAppSecret, &a.WebhookToken,
		&a.VerifyToken, &a.IsVerified, &verifiedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	a.VerifiedAt = verifiedAt
	a.CreatedAt = createdAt.Format(time.RFC3339)
	a.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &a, nil
}

func (r *instagramAppRepository) Get(ctx context.Context, userID string) (*models.InstagramApp, error) {
	const query = `SELECT ` + igAppColumns + ` FROM instagram_apps WHERE user_id = $1`
	a, err := scanIGApp(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// GetByWebhookToken resolve o cliente a partir da URL do webhook.
//
// Token vazio nunca casa: é o valor default de linha malformada, e deixá-lo
// passar entregaria um app arbitrário para validar assinatura alheia.
func (r *instagramAppRepository) GetByWebhookToken(ctx context.Context, webhookToken string) (*models.InstagramApp, error) {
	if webhookToken == "" {
		return nil, nil
	}
	const query = `SELECT ` + igAppColumns + ` FROM instagram_apps WHERE webhook_token = $1`
	a, err := scanIGApp(r.db.QueryRow(ctx, query, webhookToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// Upsert grava o app do usuário.
//
// Os tokens são gerados APENAS na primeira inserção (o DO UPDATE não os toca).
// Regerá-los a cada salvamento invalidaria a URL de webhook que o cliente já
// colou no painel da Meta — o canal pararia de receber sem nenhum erro visível.
func (r *instagramAppRepository) Upsert(ctx context.Context, userID, appID, encryptedAppSecret string) (*models.InstagramApp, error) {
	webhookToken, err := randomHex32()
	if err != nil {
		return nil, err
	}
	verifyToken, err := randomHex32()
	if err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO instagram_apps (user_id, app_id, encrypted_app_secret, webhook_token, verify_token)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			app_id               = EXCLUDED.app_id,
			encrypted_app_secret = EXCLUDED.encrypted_app_secret,
			-- Trocar credencial exige reverificar o webhook do lado da Meta.
			is_verified          = false,
			verified_at          = NULL,
			updated_at           = now()
		RETURNING ` + igAppColumns
	a, err := scanIGApp(r.db.QueryRow(ctx, query, userID, appID, encryptedAppSecret, webhookToken, verifyToken))
	if err != nil {
		r.logger.Error("Failed to upsert Instagram app", zap.String("userID", userID), zap.Error(err))
		return nil, err
	}
	return a, nil
}

func (r *instagramAppRepository) MarkVerified(ctx context.Context, userID string) error {
	const query = `UPDATE instagram_apps SET is_verified = true, verified_at = now(), updated_at = now() WHERE user_id = $1`
	_, err := r.db.pool.Exec(ctx, query, userID)
	return err
}

func (r *instagramAppRepository) Delete(ctx context.Context, userID string) error {
	_, err := r.db.pool.Exec(ctx, `DELETE FROM instagram_apps WHERE user_id = $1`, userID)
	return err
}

// randomHex32 devolve 32 bytes aleatórios em hex (64 caracteres), o formato das
// colunas CHAR(64).
func randomHex32() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
