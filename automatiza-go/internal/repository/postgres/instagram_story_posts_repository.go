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

type instagramStoryPostRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramStoryPostRepository creates a PostgreSQL story-post ledger repository.
func NewInstagramStoryPostRepository(db *DB, logger *zap.Logger) repository.InstagramStoryPostRepository {
	return &instagramStoryPostRepository{db: db, logger: logger}
}

const igStoryColumns = `id, user_id, source_group, message_id, media_source, status, ig_media_id, error,
	container_id, processing_started_at, attempt_count, error_code, published_reconciled, created_at, updated_at`

func scanIGStoryPost(row pgx.Row) (*models.InstagramStoryPost, error) {
	var p models.InstagramStoryPost
	var createdAt, updatedAt time.Time
	var processingStartedAt *time.Time
	if err := row.Scan(&p.ID, &p.UserID, &p.SourceGroup, &p.MessageID, &p.MediaSource, &p.Status,
		&p.IGMediaID, &p.Error, &p.ContainerID, &processingStartedAt, &p.AttemptCount,
		&p.ErrorCode, &p.PublishedReconciled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	p.ProcessingStartedAt = processingStartedAt
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &p, nil
}

// GetOrCreate é o ponto de idempotência durável do canal. O INSERT ... ON
// CONFLICT DO NOTHING contra uq_instagram_story_posts_source_event garante que
// duas entregas concorrentes do MESMO evento (mesmo dono, grupo e mensagem)
// produzam uma única linha; só quem realmente inseriu recebe created=true e
// portanto só esse caminho enfileira o job.
func (r *instagramStoryPostRepository) GetOrCreate(ctx context.Context, p *models.InstagramStoryPost) (*models.InstagramStoryPost, bool, error) {
	status := p.Status
	if status == "" {
		status = models.InstagramStoryStatusPending
	}
	const insertQuery = `
		INSERT INTO instagram_story_posts (user_id, source_group, message_id, media_source, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, source_group, message_id) WHERE message_id <> '' DO NOTHING
		RETURNING ` + igStoryColumns

	row, err := scanIGStoryPost(r.db.QueryRow(ctx, insertQuery, p.UserID, p.SourceGroup, p.MessageID, p.MediaSource, status))
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	// Sem linha retornada = o conflito disparou: a linha já existia. Buscar a
	// existente para o chamador poder ver se já é terminal.
	const selectQuery = `
		SELECT ` + igStoryColumns + `
		FROM instagram_story_posts
		WHERE user_id = $1 AND source_group = $2 AND message_id = $3`
	existing, err := scanIGStoryPost(r.db.QueryRow(ctx, selectQuery, p.UserID, p.SourceGroup, p.MessageID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// message_id vazio não participa do índice parcial, então o DO
			// NOTHING não pode ter disparado por ele — chegar aqui significa
			// que a linha sumiu entre as duas queries (retenção concorrente).
			return nil, false, errors.New("instagram story ledger: row vanished between insert and select")
		}
		return nil, false, err
	}
	return existing, false, nil
}

func (r *instagramStoryPostRepository) GetByID(ctx context.Context, id string) (*models.InstagramStoryPost, error) {
	const query = `SELECT ` + igStoryColumns + ` FROM instagram_story_posts WHERE id = $1`
	p, err := scanIGStoryPost(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// TryStartProcessing reivindica o lease num único UPDATE condicional — a
// atomicidade vem do próprio WHERE, sem SELECT-depois-UPDATE. Um lease mais
// velho que staleBefore pode ser tomado, para que um worker morto (OOM, deploy)
// não deixe a linha travada para sempre.
func (r *instagramStoryPostRepository) TryStartProcessing(ctx context.Context, id string, staleBefore time.Time) (bool, time.Time, error) {
	const query = `
		UPDATE instagram_story_posts
		SET processing_started_at = now(),
		    attempt_count         = attempt_count + 1,
		    updated_at            = now()
		WHERE id = $1
		  AND status = 'pending'
		  AND (processing_started_at IS NULL OR processing_started_at < $2)
		RETURNING processing_started_at`
	var lease time.Time
	if err := r.db.QueryRow(ctx, query, id, staleBefore).Scan(&lease); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, time.Time{}, nil // lease fresco de outro, ou linha terminal
		}
		return false, time.Time{}, err
	}
	return true, lease, nil
}

// ReleaseProcessing devolve o lease sem tocar no status: a linha volta a ser
// pending-sem-lease e o próximo retry pode reivindicá-la de imediato.
//
// O `AND processing_started_at = $2` é a parte que importa. O asynq cancela o
// contexto no timeout de 90s mas NÃO mata a goroutine: uma execução zumbi pode
// acordar minutos depois, quando o lease já foi legitimamente retomado por
// outro worker, e — sem esta condição — liberaria o lease alheio no meio de
// uma publicação em andamento.
func (r *instagramStoryPostRepository) ReleaseProcessing(ctx context.Context, id string, lease time.Time) error {
	const query = `
		UPDATE instagram_story_posts
		SET processing_started_at = NULL, updated_at = now()
		WHERE id = $1 AND status = 'pending' AND processing_started_at = $2`
	_, err := r.db.pool.Exec(ctx, query, id, lease)
	return err
}

// SaveContainerID persiste o container ANTES de qualquer polling/publicação.
// É o que impede o retry de criar um segundo container e publicar duplicado.
func (r *instagramStoryPostRepository) SaveContainerID(ctx context.Context, id, containerID string) error {
	const query = `UPDATE instagram_story_posts SET container_id = $2, updated_at = now() WHERE id = $1`
	_, err := r.db.pool.Exec(ctx, query, id, containerID)
	return err
}

// MarkResult grava o desfecho terminal e SEMPRE limpa o lease — uma linha
// terminal com lease pendurado confundiria a reconciliação.
//
// `AND status = 'pending'` impede que um desfecho chegue POR CIMA de outro: uma
// execução zumbi que acorda depois não pode carimbar failed sobre uma linha que
// outro worker já marcou published. Zero linhas afetadas não é erro — significa
// "alguém já concluiu", e o chamador trata isso como sucesso idempotente.
func (r *instagramStoryPostRepository) MarkResult(ctx context.Context, id, status, igMediaID, errorCode, safeMessage string, reconciled bool) error {
	const query = `
		UPDATE instagram_story_posts
		SET status = $2, ig_media_id = $3, error_code = $4, error = $5,
		    published_reconciled = $6, processing_started_at = NULL, updated_at = now()
		WHERE id = $1 AND status = 'pending'`
	if _, err := r.db.pool.Exec(ctx, query, id, status, igMediaID, errorCode, safeMessage, reconciled); err != nil {
		r.logger.Error("Failed to mark Instagram story result", zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

func (r *instagramStoryPostRepository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramStoryPost, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const query = `
		SELECT ` + igStoryColumns + `
		FROM instagram_story_posts
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`
	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.InstagramStoryPost
	for rows.Next() {
		p, err := scanIGStoryPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *instagramStoryPostRepository) CountPublishedSince(ctx context.Context, userID string, since time.Time) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM instagram_story_posts
		WHERE user_id = $1 AND status = 'published' AND created_at >= $2`
	var n int
	if err := r.db.QueryRow(ctx, query, userID, since).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// DeleteOlderThan apaga um LOTE do ledger antigo. Lote limitado + ordem
// (created_at, id) casada com idx_instagram_story_posts_retention: a limpeza
// diária nunca vira um DELETE gigante segurando lock em cima do caminho quente.
func (r *instagramStoryPostRepository) DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	const query = `
		WITH doomed AS (
			SELECT id FROM instagram_story_posts
			WHERE created_at < $1
			ORDER BY created_at, id
			LIMIT $2
		)
		DELETE FROM instagram_story_posts s USING doomed d WHERE s.id = d.id`
	tag, err := r.db.pool.Exec(ctx, query, before, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
