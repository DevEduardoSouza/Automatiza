// Command replay-monitor-offer rearma um item terminal do ledger da Melhoria 9.
//
// Uso:
//
//	go run ./cmd/replay-monitor-offer --item <uuid> --user <uuid>
//	go run ./cmd/replay-monitor-offer --item <uuid> --user <uuid> --confirm-ambiguous
//
// O que é replayável (§8.4 do plano):
//
//   - exhausted com error_class=transient — a Shopee/ML estava fora e as 5
//     tentativas acabaram; agora pode ter voltado;
//   - blocked com error_class=permanent_config — o cliente corrigiu a
//     credencial;
//   - ambiguous, APENAS com --confirm-ambiguous, porque pode duplicar.
//
// O que NÃO é, e por quê:
//
//   - permanent_content: a página não existe / é carrinho. Reprocessar dá o
//     mesmo desfecho.
//   - security_block: host forjado, IP privado, redirect escapando. Reabrir
//     seria desfazer de propósito a barreira que fechou o buraco.
//   - unsupported_commerce: não temos conversor para a loja.
//   - sent/succeeded/superseded: reenviar duplica no grupo do cliente.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository/postgres"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	itemID := flag.String("item", "", "UUID do item (monitor_offer_items.id)")
	userID := flag.String("user", "", "UUID do dono (tenant) — obrigatório")
	confirmAmbiguous := flag.Bool("confirm-ambiguous", false,
		"autoriza replay de entrega AMBÍGUA (pode duplicar a mensagem no grupo)")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	if err := run(*itemID, *userID, *confirmAmbiguous, logger); err != nil {
		// A mensagem de erro nunca carrega payload nem URL — só IDs e motivo.
		logger.Error("replay recusado", zap.Error(err))
		os.Exit(1)
	}
}

func run(itemID, userID string, confirmAmbiguous bool, logger *zap.Logger) error {
	if _, err := uuid.Parse(itemID); err != nil {
		return errors.New("--item precisa ser um UUID válido")
	}
	if _, err := uuid.Parse(userID); err != nil {
		// Exigir o tenant é o que impede um replay de tocar item de outro
		// cliente: ele entra no WHERE da query, não numa checagem posterior.
		return errors.New("--user precisa ser um UUID válido (o tenant dono do item)")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	pgCfg := cfg.GetPostgresConfig()
	port, err := strconv.Atoi(pgCfg.Port)
	if err != nil {
		return fmt.Errorf("DB_PORT inválido: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.NewDB(ctx, &postgres.Config{
		Host: pgCfg.Host, Port: port, User: pgCfg.User, Password: pgCfg.Password,
		Database: pgCfg.Database, SSLMode: pgCfg.SSLMode, MaxConns: 2,
	}, logger)
	if err != nil {
		return fmt.Errorf("conectar ao banco: %w", err)
	}
	defer db.Close()

	repo := postgres.NewMonitorOfferRetryRepository(db, logger)

	item, job, err := repo.GetItemForReplay(ctx, userID, itemID)
	if err != nil {
		return fmt.Errorf("ler item: %w", err)
	}
	if item == nil || job == nil {
		// Not-found, não "forbidden com os dados dentro": um tenant não
		// descobre nem que o item existe.
		return errors.New("item não encontrado para este tenant")
	}

	if err := checkReplayable(*item, *job, confirmAmbiguous); err != nil {
		return err
	}

	if err := repo.ResetItemForReplay(ctx, userID, itemID, confirmAmbiguous); err != nil {
		return fmt.Errorf("rearmar item: %w", err)
	}

	logger.Info("item rearmado para nova tentativa",
		zap.String("item_id", itemID),
		zap.String("user_id", userID),
		zap.String("job_id", item.JobID),
		zap.String("status_anterior", item.Status),
		zap.String("reason_anterior", item.ReasonCode),
		zap.Int("total_attempts", item.TotalAttempts),
		zap.Int("replay_count_anterior", item.ReplayCount))

	fmt.Printf("OK: item %s rearmado (era %s/%s). O sweep de 1 min o enfileira.\n",
		itemID, item.Status, item.ReasonCode)
	return nil
}

// checkReplayable devolve o motivo exato da recusa, em vez de um erro genérico
// do banco: quem roda isto está num incidente e precisa saber o que fazer.
func checkReplayable(item models.MonitorOfferItem, job models.MonitorOfferJob, confirmAmbiguous bool) error {
	switch item.Status {
	case models.MonitorItemSucceeded, models.MonitorItemSuperseded:
		return fmt.Errorf("item está %q: reenviar duplicaria a mensagem no grupo", item.Status)
	case models.MonitorItemPending, models.MonitorItemProcessing:
		return fmt.Errorf("item está %q: já há uma tentativa em andamento", item.Status)
	case models.MonitorItemAmbiguous:
		if !confirmAmbiguous {
			return errors.New("item AMBÍGUO: a entrega pode ter chegado ao grupo. " +
				"Confirme com --confirm-ambiguous se aceita o risco de duplicata")
		}
	case models.MonitorItemExhausted:
		if item.ErrorClass != models.ErrorClassTransient {
			return fmt.Errorf("exhausted com error_class=%q não é replayável", item.ErrorClass)
		}
	case models.MonitorItemBlocked:
		if !models.IsReplayableBlock(item.ErrorClass) {
			return fmt.Errorf("blocked com error_class=%q não é replayável "+
				"(reprocessar dá o mesmo desfecho, ou reabriria uma barreira de segurança)",
				item.ErrorClass)
		}
	default:
		return fmt.Errorf("status %q desconhecido", item.Status)
	}

	if job.PayloadCiphertext == nil || *job.PayloadCiphertext == "" {
		return errors.New("payload já foi limpo: sem ele não há como reclassificar o link")
	}
	if !job.PayloadExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("payload expirou em %s (retenção de %s)",
			job.PayloadExpiresAt.Format(time.RFC3339), models.PayloadTTL)
	}
	return nil
}
