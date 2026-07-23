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

// --- configuração da IA ---

type instagramAIConfigRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramAIConfigRepository creates the PostgreSQL AI-config repository.
func NewInstagramAIConfigRepository(db *DB, logger *zap.Logger) repository.InstagramAIConfigRepository {
	return &instagramAIConfigRepository{db: db, logger: logger}
}

const igAIConfigColumns = `id, user_id, persona, business_context, tone, blocked_topics,
	fallback_message, reply_comments, reply_dms, reply_mentions, is_active, created_at, updated_at`

func scanIGAIConfig(row pgx.Row) (*models.InstagramAIConfig, error) {
	var c models.InstagramAIConfig
	var createdAt, updatedAt time.Time
	if err := row.Scan(&c.ID, &c.UserID, &c.Persona, &c.BusinessContext, &c.Tone, &c.BlockedTopics,
		&c.FallbackMessage, &c.ReplyComments, &c.ReplyDMs, &c.ReplyMentions, &c.IsActive,
		&createdAt, &updatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &c, nil
}

func (r *instagramAIConfigRepository) Get(ctx context.Context, userID string) (*models.InstagramAIConfig, error) {
	const query = `SELECT ` + igAIConfigColumns + ` FROM instagram_ai_configs WHERE user_id = $1`
	c, err := scanIGAIConfig(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *instagramAIConfigRepository) Upsert(ctx context.Context, cfg *models.InstagramAIConfig) (*models.InstagramAIConfig, error) {
	const query = `
		INSERT INTO instagram_ai_configs
			(user_id, persona, business_context, tone, blocked_topics, fallback_message,
			 reply_comments, reply_dms, reply_mentions, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (user_id) DO UPDATE SET
			persona          = EXCLUDED.persona,
			business_context = EXCLUDED.business_context,
			tone             = EXCLUDED.tone,
			blocked_topics   = EXCLUDED.blocked_topics,
			fallback_message = EXCLUDED.fallback_message,
			reply_comments   = EXCLUDED.reply_comments,
			reply_dms        = EXCLUDED.reply_dms,
			reply_mentions   = EXCLUDED.reply_mentions,
			is_active        = EXCLUDED.is_active,
			updated_at       = now()
		RETURNING ` + igAIConfigColumns
	return scanIGAIConfig(r.db.QueryRow(ctx, query,
		cfg.UserID, cfg.Persona, cfg.BusinessContext, cfg.Tone, cfg.BlockedTopics, cfg.FallbackMessage,
		cfg.ReplyComments, cfg.ReplyDMs, cfg.ReplyMentions, cfg.IsActive))
}

// --- respostas ---

type instagramReplyRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramReplyRepository creates the PostgreSQL reply repository.
func NewInstagramReplyRepository(db *DB, logger *zap.Logger) repository.InstagramReplyRepository {
	return &instagramReplyRepository{db: db, logger: logger}
}

const igReplyColumns = `id, user_id, provider_event_id, event_type, reply_text, status,
	error_code, prompt_tokens, completion_tokens, created_at, updated_at`

func scanIGReply(row pgx.Row) (*models.InstagramReply, error) {
	var r models.InstagramReply
	var createdAt, updatedAt time.Time
	if err := row.Scan(&r.ID, &r.UserID, &r.ProviderEventID, &r.EventType, &r.ReplyText,
		&r.Status, &r.ErrorCode, &r.PromptTokens, &r.CompletionTokens, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	r.CreatedAt = createdAt.Format(time.RFC3339)
	r.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &r, nil
}

// Claim é a trava pública contra resposta duplicada. O INSERT ... ON CONFLICT
// DO NOTHING contra uq_instagram_replies_event garante que só UM executor ganhe
// o direito de responder aquele evento — os demais recebem created=false e
// param. Sem isso, um redelivery da Meta ou um retry do worker deixaria duas
// respostas visíveis no perfil do cliente.
func (r *instagramReplyRepository) Claim(ctx context.Context, userID, providerEventID, eventType string) (*models.InstagramReply, bool, error) {
	const insertQuery = `
		INSERT INTO instagram_replies (user_id, provider_event_id, event_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, provider_event_id) DO NOTHING
		RETURNING ` + igReplyColumns
	row, err := scanIGReply(r.db.QueryRow(ctx, insertQuery, userID, providerEventID, eventType))
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	const selectQuery = `SELECT ` + igReplyColumns + ` FROM instagram_replies WHERE user_id = $1 AND provider_event_id = $2`
	existing, err := scanIGReply(r.db.QueryRow(ctx, selectQuery, userID, providerEventID))
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (r *instagramReplyRepository) MarkResult(ctx context.Context, id, status, replyText, errorCode string, promptTokens, completionTokens int) error {
	const query = `
		UPDATE instagram_replies
		SET status = $2, reply_text = $3, error_code = $4,
		    prompt_tokens = $5, completion_tokens = $6, updated_at = now()
		WHERE id = $1 AND status = 'pending'`
	_, err := r.db.pool.Exec(ctx, query, id, status, replyText, errorCode, promptTokens, completionTokens)
	return err
}

// CountSince conta apenas as ENVIADAS: uma tentativa que falhou não consumiu
// cota do cliente nem custo relevante de IA.
func (r *instagramReplyRepository) CountSince(ctx context.Context, userID string, since time.Time) (int, error) {
	const query = `SELECT count(*) FROM instagram_replies WHERE user_id = $1 AND status = 'sent' AND created_at >= $2`
	var n int
	if err := r.db.QueryRow(ctx, query, userID, since).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *instagramReplyRepository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramReply, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const query = `SELECT ` + igReplyColumns + ` FROM instagram_replies WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.InstagramReply
	for rows.Next() {
		rp, err := scanIGReply(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rp)
	}
	return out, rows.Err()
}

// --- cota do plano ---

type instagramPlanCapRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramPlanCapRepository creates the plan-quota lookup.
func NewInstagramPlanCapRepository(db *DB, logger *zap.Logger) repository.InstagramPlanCapRepository {
	return &instagramPlanCapRepository{db: db, logger: logger}
}

// MaxAIRepliesPerDay lê o teto do plano do usuário.
//
// Usuário sem plano associado devolve 0, que o worker traduz no padrão
// conservador — nunca em "ilimitado". A chave de IA é nossa.
func (r *instagramPlanCapRepository) MaxAIRepliesPerDay(ctx context.Context, userID string) (int, error) {
	const query = `
		SELECT COALESCE(pl.max_ai_replies_per_day, 0)
		FROM profiles p
		LEFT JOIN plans pl ON pl.slug = p.plan
		WHERE p.id = $1`
	var n int
	if err := r.db.QueryRow(ctx, query, userID).Scan(&n); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return n, nil
}
