package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================
// Kanban (aba Kanban do painel de suporte)
// ============================================================
//
// Quadro interno da equipe de suporte para acompanhar a correcao de bugs. Vive
// junto ao restante do SupportRepository, mas so toca as tabelas kanban_cards /
// kanban_attachments — nunca dados de cliente. Ver 070_kanban_board.up.sql.

// KanbanCardRow e uma linha de kanban_cards.
type KanbanCardRow struct {
	ID          string
	ColumnKey   string
	Position    float64
	Title       string
	Description string
	Priority    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// KanbanAttachmentRow e o metadado de um anexo (sem os bytes).
type KanbanAttachmentRow struct {
	ID          string
	CardID      string
	Filename    string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

// KanbanAttachmentContent carrega os bytes de um anexo para download.
type KanbanAttachmentContent struct {
	Filename    string
	ContentType string
	Data        []byte
}

// CreateKanbanCardParams sao os campos para inserir um card.
type CreateKanbanCardParams struct {
	Title       string
	Description string
	ColumnKey   string
	Priority    string
	CreatedBy   *string
}

// ListKanbanCards devolve todos os cards em ordem de coluna e posicao.
func (r *SupportRepository) ListKanbanCards(ctx context.Context) ([]KanbanCardRow, error) {
	sql := `
		SELECT id, column_key, position, title, description, priority,
		       created_at, updated_at
		FROM kanban_cards
		ORDER BY column_key, position, created_at
	`
	rows, err := r.db.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list kanban cards: %w", err)
	}
	defer rows.Close()

	var out []KanbanCardRow
	for rows.Next() {
		var c KanbanCardRow
		if err := rows.Scan(&c.ID, &c.ColumnKey, &c.Position, &c.Title,
			&c.Description, &c.Priority, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("kanban card scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListKanbanAttachments devolve os metadados de anexos de todos os cards,
// agrupados por card_id. Uma unica query para evitar N+1 na listagem.
func (r *SupportRepository) ListKanbanAttachments(ctx context.Context) (map[string][]KanbanAttachmentRow, error) {
	sql := `
		SELECT id, card_id, filename, content_type, size_bytes, created_at
		FROM kanban_attachments
		ORDER BY created_at
	`
	rows, err := r.db.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list kanban attachments: %w", err)
	}
	defer rows.Close()

	byCard := make(map[string][]KanbanAttachmentRow)
	for rows.Next() {
		var a KanbanAttachmentRow
		if err := rows.Scan(&a.ID, &a.CardID, &a.Filename, &a.ContentType,
			&a.SizeBytes, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("kanban attachment scan: %w", err)
		}
		byCard[a.CardID] = append(byCard[a.CardID], a)
	}
	return byCard, rows.Err()
}

// nextKanbanPosition devolve a proxima posicao (fim da coluna): max(position)+1,
// ou 0 se a coluna estiver vazia. Card novo entra no fim da coluna.
func (r *SupportRepository) nextKanbanPosition(ctx context.Context, columnKey string) (float64, error) {
	var pos *float64
	err := r.db.pool.QueryRow(ctx,
		`SELECT MAX(position) FROM kanban_cards WHERE column_key = $1`, columnKey,
	).Scan(&pos)
	if err != nil {
		return 0, fmt.Errorf("next kanban position: %w", err)
	}
	if pos == nil {
		return 0, nil
	}
	return *pos + 1, nil
}

// CreateKanbanCard insere um card no fim da sua coluna e devolve a linha.
func (r *SupportRepository) CreateKanbanCard(ctx context.Context, p CreateKanbanCardParams) (*KanbanCardRow, error) {
	pos, err := r.nextKanbanPosition(ctx, p.ColumnKey)
	if err != nil {
		return nil, err
	}
	sql := `
		INSERT INTO kanban_cards (column_key, position, title, description, priority, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, column_key, position, title, description, priority, created_at, updated_at
	`
	var c KanbanCardRow
	err = r.db.pool.QueryRow(ctx, sql,
		p.ColumnKey, pos, p.Title, p.Description, p.Priority, p.CreatedBy,
	).Scan(&c.ID, &c.ColumnKey, &c.Position, &c.Title, &c.Description,
		&c.Priority, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create kanban card: %w", err)
	}
	return &c, nil
}

// UpdateKanbanCard altera titulo/descricao/prioridade e devolve a linha nova.
// Devolve nil (sem erro) se o card nao existir.
func (r *SupportRepository) UpdateKanbanCard(ctx context.Context, id, title, description, priority string) (*KanbanCardRow, error) {
	sql := `
		UPDATE kanban_cards
		SET title = $2, description = $3, priority = $4, updated_at = now()
		WHERE id = $1
		RETURNING id, column_key, position, title, description, priority, created_at, updated_at
	`
	var c KanbanCardRow
	err := r.db.pool.QueryRow(ctx, sql, id, title, description, priority).
		Scan(&c.ID, &c.ColumnKey, &c.Position, &c.Title, &c.Description,
			&c.Priority, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update kanban card: %w", err)
	}
	return &c, nil
}

// MoveKanbanCard altera coluna e posicao (arrasto). Devolve nil se nao existir.
func (r *SupportRepository) MoveKanbanCard(ctx context.Context, id, columnKey string, position float64) (*KanbanCardRow, error) {
	sql := `
		UPDATE kanban_cards
		SET column_key = $2, position = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, column_key, position, title, description, priority, created_at, updated_at
	`
	var c KanbanCardRow
	err := r.db.pool.QueryRow(ctx, sql, id, columnKey, position).
		Scan(&c.ID, &c.ColumnKey, &c.Position, &c.Title, &c.Description,
			&c.Priority, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("move kanban card: %w", err)
	}
	return &c, nil
}

// DeleteKanbanCard remove o card (anexos caem em cascata). false se nao existia.
func (r *SupportRepository) DeleteKanbanCard(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `DELETE FROM kanban_cards WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete kanban card: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// CardExists diz se um card existe (validacao antes de anexar).
func (r *SupportRepository) CardExists(ctx context.Context, cardID string) (bool, error) {
	var exists bool
	err := r.db.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM kanban_cards WHERE id = $1)`, cardID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("card exists: %w", err)
	}
	return exists, nil
}

// CreateKanbanAttachment grava os bytes e devolve o metadado; toca updated_at
// do card para que a listagem reflita a mudanca.
func (r *SupportRepository) CreateKanbanAttachment(ctx context.Context, cardID, filename, contentType string, data []byte, createdBy *string) (*KanbanAttachmentRow, error) {
	sql := `
		INSERT INTO kanban_attachments (card_id, filename, content_type, size_bytes, data, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, card_id, filename, content_type, size_bytes, created_at
	`
	var a KanbanAttachmentRow
	err := r.db.pool.QueryRow(ctx, sql,
		cardID, filename, contentType, int64(len(data)), data, createdBy,
	).Scan(&a.ID, &a.CardID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create kanban attachment: %w", err)
	}
	_, _ = r.db.pool.Exec(ctx, `UPDATE kanban_cards SET updated_at = now() WHERE id = $1`, cardID)
	return &a, nil
}

// GetKanbanAttachmentContent devolve os bytes de um anexo. nil se nao existir.
func (r *SupportRepository) GetKanbanAttachmentContent(ctx context.Context, id string) (*KanbanAttachmentContent, error) {
	var c KanbanAttachmentContent
	err := r.db.pool.QueryRow(ctx,
		`SELECT filename, content_type, data FROM kanban_attachments WHERE id = $1`, id,
	).Scan(&c.Filename, &c.ContentType, &c.Data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get kanban attachment: %w", err)
	}
	return &c, nil
}

// DeleteKanbanAttachment remove um anexo. false se nao existia.
func (r *SupportRepository) DeleteKanbanAttachment(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `DELETE FROM kanban_attachments WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete kanban attachment: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
