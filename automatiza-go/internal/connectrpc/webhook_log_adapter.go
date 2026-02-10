package connectrpc

import (
	"context"
	"fmt"

	"automatiza-go/internal/repository/postgres"
)

// webhookLogAdapter adapts postgres.WebhookLogRepository to the connectrpc.WebhookLogRepository interface.
type webhookLogAdapter struct {
	repo postgres.WebhookLogRepository
}

// NewWebhookLogAdapter creates a new adapter that converts postgres.WebhookLog to connectrpc.WebhookLogEntry.
func NewWebhookLogAdapter(repo postgres.WebhookLogRepository) WebhookLogRepository {
	return &webhookLogAdapter{repo: repo}
}

func (a *webhookLogAdapter) GetRecent(ctx context.Context, source string, limit int) ([]WebhookLogEntry, error) {
	logs, err := a.repo.GetRecent(ctx, source, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]WebhookLogEntry, len(logs))
	for i, log := range logs {
		entries[i] = WebhookLogEntry{
			ID:        fmt.Sprintf("%d", log.ID),
			Source:    log.Source,
			EventType: log.EventType,
			Email:     log.Email,
			Plan:      log.Plan,
			Status:    log.Status,
			CreatedAt: log.CreatedAt,
		}
	}

	return entries, nil
}
