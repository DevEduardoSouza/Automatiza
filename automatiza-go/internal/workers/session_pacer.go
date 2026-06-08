package workers

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// sessionPaceScript reserva o próximo "slot" de envio para uma sessão e devolve,
// em ms, quanto esperar (slot - now). É atômico: cada chamada avança o slot por
// `spacing`, então envios concorrentes para a MESMA sessão saem espaçados em vez
// de em rajada — mesmo quando disparados por automações diferentes ao mesmo
// tempo. Não é rate-limit (não rejeita nada): só empurra o agendamento.
var sessionPaceScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local spacing = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local slot = now
local cur = redis.call('GET', KEYS[1])
if cur then
  local c = tonumber(cur)
  if c and c > now then slot = c end
end
redis.call('SET', KEYS[1], slot + spacing, 'PX', ttl)
return slot - now
`)

// sessionPacer suaviza o ritmo de envio por sessão do WhatsApp. O device linkado
// (engine GOWS) distribui sender-keys por grupo; quando muitas mensagens saem na
// mesma fração de segundo, os destinatários ficam sem a chave e veem "Aguardando
// mensagem" até o retry. Espaçar os envios reduz esse efeito.
type sessionPacer struct {
	rdb     redis.Scripter
	spacing time.Duration
	logger  *zap.Logger
}

// newSessionPacer devolve nil (desligado) quando spacing <= 0 ou sem Redis.
func newSessionPacer(rdb redis.Scripter, spacing time.Duration, logger *zap.Logger) *sessionPacer {
	if rdb == nil || spacing <= 0 {
		return nil
	}
	return &sessionPacer{rdb: rdb, spacing: spacing, logger: logger}
}

// reserve devolve o atraso a aplicar no envio para suavizar a sessão.
// Fail-open: pacer nil, sessão vazia ou QUALQUER erro de Redis devolve 0 — o
// pacer nunca bloqueia nem atrasa um envio por conta própria em caso de falha.
func (p *sessionPacer) reserve(ctx context.Context, session string) time.Duration {
	if p == nil || session == "" {
		return 0
	}
	now := time.Now().UnixMilli()
	spacingMs := p.spacing.Milliseconds()
	// TTL generoso: a chave some sozinha quando a sessão fica ociosa, mas vive o
	// suficiente para segurar o slot durante uma rajada.
	ttlMs := spacingMs*8 + 60_000
	key := "pace:session:" + session

	res, err := sessionPaceScript.Run(ctx, p.rdb, []string{key}, now, spacingMs, ttlMs).Int64()
	if err != nil {
		p.logger.Warn("session pacer failed, enqueueing without pacing",
			zap.String("session", session), zap.Error(err))
		return 0
	}
	if res <= 0 {
		return 0
	}
	return time.Duration(res) * time.Millisecond
}

// sessionPaceMsFromEnv lê SESSION_PACE_MS. Ausente/0/invalido => desligado.
func sessionPaceMsFromEnv() int {
	v := os.Getenv("SESSION_PACE_MS")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
