package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"time"

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

func (r *instagramStoryPostRepository) Create(ctx context.Context, p *models.InstagramStoryPost) (string, error) {
	const query = `
		INSERT INTO instagram_story_posts (user_id, source_group, message_id, media_source, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	status := p.Status
	if status == "" {
		status = models.InstagramStoryStatusPending
	}
	var id string
	if err := r.db.QueryRow(ctx, query, p.UserID, p.SourceGroup, p.MessageID, p.MediaSource, status).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *instagramStoryPostRepository) MarkResult(ctx context.Context, id, status, igMediaID, errMsg string) error {
	const query = `
		UPDATE instagram_story_posts
		SET status = $2, ig_media_id = $3, error = $4, updated_at = now()
		WHERE id = $1`
	if _, err := r.db.pool.Exec(ctx, query, id, status, igMediaID, errMsg); err != nil {
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
		SELECT id, user_id, source_group, message_id, media_source, status, ig_media_id, error, created_at, updated_at
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
		var p models.InstagramStoryPost
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.ID, &p.UserID, &p.SourceGroup, &p.MessageID, &p.MediaSource,
			&p.Status, &p.IGMediaID, &p.Error, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		p.UpdatedAt = updatedAt.Format(time.RFC3339)
		out = append(out, p)
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
