package whatsapp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Chaves lidas pelo painel de suporte (SupportService.GetSystemHealth):
// hash sessão→contagem de erros 420 e hash sessão→unix do último erro.
// TTL deslizante de 24h nas duas — chip que parou de falhar some sozinho.
const (
	RestrictedChipsCountKey = "support:restricted_chips"
	RestrictedChipsSeenKey  = "support:restricted_chips_seen"
	restrictedChipsTTL      = 24 * time.Hour
)

// restrictedChipMarker é a assinatura do WhatsApp recusando envios de um chip
// específico (rate/restrição): o WAHA devolve 500 com "server returned error
// 420" no corpo (gRPC do GOWS). O fallback imagem→texto do worker mascara o
// caso no dispatch_logs (fecha 'sent'), então a ÚNICA detecção confiável é
// aqui, no momento do erro, com a sessão em mãos.
const restrictedChipMarker = "server returned error 420"

// RestrictedChipRecorder anota, por sessão, envios recusados com erro 420 —
// alimenta o card "Chips restritos" do painel de suporte. Nil-safe e
// fail-open como o UploadPacer: QUALQUER erro de Redis é engolido (log debug);
// o caminho de envio nunca é afetado por este side-effect.
type RestrictedChipRecorder struct {
	rdb    redis.Cmdable
	logger *zap.Logger
}

// NewRestrictedChipRecorder returns nil (disabled) when rdb is nil, mirroring
// NewRedisUploadPacer.
func NewRestrictedChipRecorder(rdb redis.Cmdable, logger *zap.Logger) *RestrictedChipRecorder {
	if rdb == nil {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RestrictedChipRecorder{rdb: rdb, logger: logger}
}

// Note registra o erro se (e só se) ele carrega a assinatura 420. Usa contexto
// próprio desacoplado: o ctx do envio pode já estar morto no caminho de erro,
// e o registro não pode nem falhar junto nem segurar o caller.
func (r *RestrictedChipRecorder) Note(ctx context.Context, sessionName string, err error) {
	if r == nil || sessionName == "" || err == nil {
		return
	}
	if !strings.Contains(err.Error(), restrictedChipMarker) {
		return
	}
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()

	pipe := r.rdb.Pipeline()
	pipe.HIncrBy(opCtx, RestrictedChipsCountKey, sessionName, 1)
	pipe.HSet(opCtx, RestrictedChipsSeenKey, sessionName, strconv.FormatInt(time.Now().Unix(), 10))
	pipe.Expire(opCtx, RestrictedChipsCountKey, restrictedChipsTTL)
	pipe.Expire(opCtx, RestrictedChipsSeenKey, restrictedChipsTTL)
	if _, pErr := pipe.Exec(opCtx); pErr != nil {
		r.logger.Debug("restricted chip note failed (fail-open)",
			zap.String("session", sessionName), zap.Error(pErr))
	}
}
