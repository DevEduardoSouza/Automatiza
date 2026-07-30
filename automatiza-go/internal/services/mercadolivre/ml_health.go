package mercadolivre

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Parâmetros do health check. O ritmo aqui é deliberadamente lento: o risco a
// evitar não é perder uma verificação, é o próprio health check virar a causa
// do bloqueio que ele deveria detectar. Com ~600 usuários, verificar todos de
// hora em hora seriam 600 requests autenticadas/h saindo do MESMO IP contra o
// endpoint mais sensível do ML — exatamente o padrão que um antifraude agrupa.
const (
	// De quanto em quanto tempo uma credencial é reverificada.
	healthCheckStaleness = 12 * time.Hour
	// Teto de credenciais por execução (o cron roda de hora em hora).
	healthCheckBatchSize = 40
	// Espaçamento entre chamadas dentro do lote, com jitter.
	healthCheckSpacing = 1500 * time.Millisecond
)

// HealthChecker verifica periodicamente se as sessões do ML ainda estão vivas.
//
// É um tipo separado, e não um método de Service, de propósito: Service é
// implementada por mocks em vários pacotes, e ampliá-la obrigaria a mexer em
// todos eles para uma funcionalidade que nenhum deles usa.
type HealthChecker struct {
	repo          repository.MLCredentialsRepository
	encryptionKey string
	client        *http.Client
	logger        *zap.Logger
	// notify é chamado quando o estado da sessão MUDA (ex.: live → expired).
	// Opcional: nil desliga a notificação sem desligar a verificação.
	notify func(ctx context.Context, userID, oldStatus, newStatus string)
}

func NewHealthChecker(
	repo repository.MLCredentialsRepository,
	encryptionKey string,
	logger *zap.Logger,
) *HealthChecker {
	return &HealthChecker{
		repo:          repo,
		encryptionKey: encryptionKey,
		client:        &http.Client{Timeout: 10 * time.Second},
		logger:        logger,
	}
}

// OnStatusChange registra o callback de mudança de estado (painel/push).
func (h *HealthChecker) OnStatusChange(fn func(ctx context.Context, userID, oldStatus, newStatus string)) {
	h.notify = fn
}

// Run verifica um lote de sessões. Chamado pelo cron de hora em hora.
//
// Nunca devolve erro por causa de UMA credencial: uma conta com problema não
// pode interromper a verificação das outras (mesma regra de
// instagram.RefreshExpiring).
func (h *HealthChecker) Run(ctx context.Context) error {
	if h.repo == nil {
		return nil
	}

	creds, err := h.repo.ListForHealthCheck(ctx, time.Now().UTC().Add(-healthCheckStaleness), healthCheckBatchSize)
	if err != nil {
		return fmt.Errorf("list ml credentials for health check: %w", err)
	}
	if len(creds) == 0 {
		return nil
	}

	var live, expired, blocked, unknown int

	for i := range creds {
		if ctx.Err() != nil {
			break
		}
		cred := &creds[i]

		status, tag := h.checkOne(ctx, cred)

		if status != cred.SessionStatus {
			if h.notify != nil {
				h.notify(ctx, cred.UserID, cred.SessionStatus, status)
			}
			h.logger.Info("ML session status changed",
				zap.String("userID", cred.UserID),
				zap.String("from", cred.SessionStatus),
				zap.String("to", status))
		}

		if upErr := h.repo.UpdateSessionStatus(ctx, cred.UserID, status, tag); upErr != nil {
			h.logger.Warn("failed to persist ML session status",
				zap.String("userID", cred.UserID), zap.Error(upErr))
		}

		switch status {
		case "live":
			live++
		case "expired":
			expired++
		case "blocked":
			blocked++
		default:
			unknown++
		}

		// Bloqueio é sinal de PARE: se o ML está nos recusando, continuar o
		// lote só aprofunda o problema. O resto do lote fica para a próxima
		// hora (as credenciais não verificadas continuam no topo da fila).
		if status == "blocked" {
			h.logger.Warn("ML blocked during health check — aborting batch",
				zap.Int("checked", i+1), zap.Int("batch", len(creds)))
			break
		}

		// Espaçamento com jitter: um lote em rajada é o que chama atenção.
		if i < len(creds)-1 {
			sleep(ctx, healthCheckSpacing+time.Duration(rand.N(600))*time.Millisecond)
		}
	}

	h.logger.Info("ML session health check done",
		zap.Int("live", live), zap.Int("expired", expired),
		zap.Int("blocked", blocked), zap.Int("unknown", unknown))
	return nil
}

// checkOne devolve o novo status e a tag descoberta (vazia = não sobrescrever).
func (h *HealthChecker) checkOne(ctx context.Context, cred *models.MLCredentialsDB) (string, string) {
	creds, err := decryptMLCredentials(cred.EncryptedData, h.encryptionKey)
	if err != nil {
		// Não conseguimos ler a credencial: é problema NOSSO (chave, blob
		// corrompido), não do cliente. Não marcar 'expired' — isso mandaria
		// alguém com sessão boa reconectar à toa.
		h.logger.Warn("ML health check: cannot decrypt credentials",
			zap.String("userID", cred.UserID))
		return "unknown", ""
	}

	check, err := ValidateMLSession(ctx, h.client, creds.Cookie)
	switch {
	case err == nil:
		tag := ""
		// Só grava a tag descoberta quando é inequívoca. Com várias, escolher
		// creditaria comissão na sub-tag errada em silêncio.
		if len(check.Tags) == 1 {
			tag = check.Tags[0]
		}
		return "live", tag
	case errors.Is(err, ErrMLSessionExpired):
		return "expired", ""
	case errors.Is(err, ErrMLBlocked):
		return "blocked", ""
	default:
		// ErrMLUnavailable e afins: indeterminado. Carimbar 'unknown' tira a
		// credencial da fila por 12h, o que evita insistir contra um ML fora
		// do ar — sem acusar o cliente de nada.
		return "unknown", ""
	}
}

// sleep dorme respeitando o cancelamento do contexto.
func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// decryptMLCredentials decifra o blob salvo. Extraída de LoadCredentials para
// ser reusada pelo health checker sem passar pelo Service inteiro.
func decryptMLCredentials(encryptedData, key string) (*models.MLCredentials, error) {
	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(encryptedData), &blob); err != nil {
		return nil, fmt.Errorf("%w: parse encrypted blob: %v", ErrMLCredentialsUnreadable, err)
	}
	var creds models.MLCredentials
	if err := crypto.DecryptJSONV1(&blob, key, &creds); err != nil {
		return nil, fmt.Errorf("%w: decrypt: %v", ErrMLCredentialsUnreadable, err)
	}
	return &creds, nil
}
