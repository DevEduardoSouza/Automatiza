package automation

import (
	"context"
	"fmt"
	"time"

	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

const (
	// listQueuedStaleAfter: um dispatch de LIST em 'queued' vive minutos
	// (fila → worker → sent/failed). Mais de 2h sem desfecho = task perdida
	// (redis reiniciou, worker morto além do lease) — vira 'error' para o
	// reconciliador enxergar. Bem acima do staleMessageTTL (90min) do worker,
	// então nunca compete com um descarte legítimo em andamento.
	listQueuedStaleAfter = 2 * time.Hour

	// listReconcileWindow limita o olhar do reconciliador (e o custo da
	// query, via índice parcial por list_item_id+created_at).
	listReconcileWindow = 24 * time.Hour
)

// ReconcileListDeliveries fecha o ciclo de entrega do LIST mode (padrão
// "nunca confirmar antes da confirmação real"): o item é consumido no
// ENFILEIRAMENTO (necessário para a lista avançar), então este reconciliador
// — cron de ~10min no server — detecta pelo dispatch_logs os itens cuja
// entrega NUNCA se confirmou (0 sent, 0 queued, ≥1 failed/error na janela) e:
//
//  1. Marca como 'error' os 'queued' órfãos (>2h sem desfecho = task perdida)
//     — APENAS higiene/visibilidade: 'error' é desfecho DESCONHECIDO (o envio
//     pode ter acontecido e só o update de 'sent' se perdeu) e por isso NUNCA
//     dispara devolução; ao contrário, veta o item (ver
//     GetListResurrectCandidates). Duplicar é pior que perder o retry.
//  2. Libera no Redis os claims de dedup (dedup:user:group:canonical) apenas
//     dos destinos com 'failed' TERMINAL do worker — sem isso o retry cairia
//     no caminho "fully deduped → consume" e o item seria queimado de novo.
//  3. Devolve o item (consumed_at=NULL) para o próximo ciclo da automação.
//
// Idempotente e convergente: itens devolvidos saem da consulta (JOIN por
// consumed_at IS NOT NULL); um retry que falhar de novo re-entra até a janela
// de 24h expirar. Destinos com pelo menos 1 'sent' NÃO são devolvidos — a
// semântica do LIST sempre foi "1 entrega confirma o item".
func (s *automationService) ReconcileListDeliveries(ctx context.Context) error {
	if s.dispatchLogRepo == nil || s.mlLinkListItemRepo == nil {
		return nil
	}

	// 1. Desfecho terminal para envios perdidos.
	marked, err := s.dispatchLogRepo.MarkStaleQueuedListLogs(ctx, listQueuedStaleAfter, listReconcileWindow)
	if err != nil {
		return fmt.Errorf("mark stale queued list logs: %w", err)
	}
	if marked > 0 {
		s.logger.Warn("List reconciler: queued órfãos marcados como error",
			zap.Int64("marked", marked))
	}

	// 2. Itens consumidos sem nenhuma entrega confirmada.
	candidates, err := s.dispatchLogRepo.GetListResurrectCandidates(ctx, listReconcileWindow)
	if err != nil {
		return fmt.Errorf("get list resurrect candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil
	}

	// 2a. Libera os claims de dedup dos destinos falhos ANTES de devolver o
	// item: se a automação ticar entre o un-consume e o release, o item cai
	// no "fully deduped → consume" e queima de novo.
	released := 0
	if s.cache != nil {
		for _, c := range candidates {
			if c.GroupID == nil || c.CanonicalKey == nil || *c.CanonicalKey == "" {
				continue
			}
			key := fmt.Sprintf("dedup:%s:%s:%s", c.UserID, *c.GroupID, *c.CanonicalKey)
			if delErr := s.cache.Delete(ctx, key); delErr != nil {
				s.logger.Debug("List reconciler: failed to release dedup claim",
					zap.String("key", key), zap.Error(delErr))
				continue
			}
			released++
		}
	}

	// 2b. Devolve os itens (dedup por item — um item pode ter N destinos falhos).
	seen := make(map[string]struct{}, len(candidates))
	itemIDs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c.ListItemID]; ok {
			continue
		}
		seen[c.ListItemID] = struct{}{}
		itemIDs = append(itemIDs, c.ListItemID)
	}

	reset, err := s.mlLinkListItemRepo.ResetConsumedByIDs(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("reset consumed items: %w", err)
	}

	if reset > 0 {
		s.logger.Warn("List reconciler: itens devolvidos para retry (consumidos sem entrega confirmada)",
			zap.Int64("itemsReset", reset),
			zap.Int("failedDestinations", len(candidates)),
			zap.Int("dedupClaimsReleased", released))
		observability.CaptureMessage(sentry.LevelInfo,
			fmt.Sprintf("List reconciler: %d itens devolvidos para retry (%d destinos falhos)", reset, len(candidates)),
			map[string]string{"module": "automation", "flow": "list-reconcile"})
	}

	return nil
}
