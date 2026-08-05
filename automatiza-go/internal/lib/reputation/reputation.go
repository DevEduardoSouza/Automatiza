// Package reputation implements a Redis-backed per-(session,group) reputation
// and suppression store for anti-ban. Um grupo de WhatsApp que falha ou remove o
// bot repetidamente acumula pontuação; ao cruzar um limiar ele passa a ser
// "suprimido" e o dispatcher deixa de enviar para ele até que se recupere.
//
// Penalize/Clear são fail-open: qualquer erro de Redis é logado em Warn e
// ignorado. IsSuppressed é fail-open por default, mas com ANTIBAN_STRICT=on
// (R3) passa a ser fail-CLOSED: um erro de Redis suprime o envio para não
// disparar sem anti-ban quando o Redis está indisponível (ver campo `strict`).
package reputation

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// penalizeScript soma `weight` à pontuação do grupo e renova o TTL da chave num
// único passo atômico (INCRBY + PEXPIRE), evitando corrida entre duas
// penalizações concorrentes. O TTL deslizante faz a reputação "esfriar" sozinha:
// um grupo que para de falhar tem a chave expirada e volta à rotação. Devolve a
// pontuação acumulada.
var penalizeScript = redis.NewScript(`
local score = redis.call('INCRBY', KEYS[1], ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return score
`)

// Store guarda a reputação por (sessão, grupo) no Redis. O zero value não é
// utilizável; use NewStore. Um Store com rdb nil é um no-op seguro.
type Store struct {
	rdb       redis.Scripter
	threshold int
	ttl       time.Duration
	logger    *zap.Logger
	// strict (R3): quando true, um ERRO de Redis em IsSuppressed passa a
	// SUPRIMIR o envio (fail-closed) em vez de liberar (fail-open). Protege
	// contra ban quando o Redis está estressado e todo o anti-ban cairia junto,
	// ao custo de PARAR envios durante um blip de Redis. Controlado por
	// ANTIBAN_STRICT (default on); ANTIBAN_STRICT=false/0 volta ao fail-open.
	strict bool
}

// NewStore devolve um store. Um rdb nil produz um store no-op nil-safe
// (Penalize/Clear não fazem nada, IsSuppressed sempre devolve false) para que os
// callers nunca precisem checar nil e um Redis ausente nunca bloqueie um envio.
// threshold<=0 assume 5; ttl<=0 assume 24h.
func NewStore(rdb redis.Scripter, threshold int, ttl time.Duration, logger *zap.Logger) *Store {
	if threshold <= 0 {
		threshold = 5
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Store{rdb: rdb, threshold: threshold, ttl: ttl, logger: logger, strict: antibanStrictFromEnv()}
}

// antibanStrictFromEnv lê ANTIBAN_STRICT. Default TRUE (fail-closed conforme a
// decisão da Rodada 2); ANTIBAN_STRICT=false/0 desliga (volta ao fail-open) e
// é o kill-switch de reversão sem mexer no código.
func antibanStrictFromEnv() bool {
	switch os.Getenv("ANTIBAN_STRICT") {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

// key monta a chave Redis do par (sessão, grupo).
func key(session, group string) string {
	return "reputation:group:" + session + "|" + group
}

// Penalize soma `weight` à pontuação de (sessão, grupo) com TTL deslizante.
// Pesos típicos: falha de envio=1, remoção do grupo (bot expulso)=3. Fail-open:
// qualquer erro é logado em Warn e ignorado. No-op em store/rdb nil ou
// sessão/grupo vazios.
func (s *Store) Penalize(ctx context.Context, session, group string, weight int) {
	if s == nil || s.rdb == nil || session == "" || group == "" {
		return
	}
	if weight <= 0 {
		return
	}
	ttlMs := s.ttl.Milliseconds()
	if err := penalizeScript.Run(ctx, s.rdb, []string{key(session, group)}, weight, ttlMs).Err(); err != nil {
		s.logger.Warn("reputation penalize failed, ignoring",
			zap.String("session", session), zap.String("group", group),
			zap.Int("weight", weight), zap.Error(err))
	}
}

// IsSuppressed SEMPRE devolve false desde 05/08/2026: a supressão por reputação
// foi DESLIGADA por decisão de produto e nenhum envio é mais bloqueado por ela.
//
// Os 5 chamadores (offer_generate ×2, process_list, automation_service,
// monitoring) passam a enviar para o grupo mesmo depois de o chip ter sido
// removido dele. O desligamento mora aqui, e não nos chamadores, porque um único
// ponto cobre os cinco caminhos de envio e a reversão é um `git revert` deste
// commit — a implementação original (GET no Redis, limiar, modo fail-closed do
// ANTIBAN_STRICT) está no histórico.
//
// Penalize/Clear continuam gravando a pontuação (chaves com TTL de 24h), então o
// sinal segue sendo coletado e a proteção pode voltar sem perder histórico.
//
// Risco assumido: insistir em grupo que expulsou o chip é o sinal mais forte de
// bloqueio/denúncia para o WhatsApp — era exatamente o que esta trava evitava.
func (s *Store) IsSuppressed(ctx context.Context, session, group string) bool {
	return false
}

// getScript lê a pontuação via GET. Usar um script mantém tudo sobre a interface
// redis.Scripter (que não expõe GET diretamente), permitindo injetar tanto
// *redis.Client quanto um client apontado para o miniredis nos testes. Chave
// ausente vira '0' (GET devolve false em Lua) para que "missing => não suprimido"
// não vire um erro redis.Nil.
var getScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v == false then return '0' end
return v
`)

// Clear zera a pontuação de um grupo (chame após um envio bem-sucedido para
// deixar um grupo recuperado voltar à rotação). Fail-open: no-op em qualquer
// erro.
func (s *Store) Clear(ctx context.Context, session, group string) {
	if s == nil || s.rdb == nil || session == "" || group == "" {
		return
	}
	if err := delScript.Run(ctx, s.rdb, []string{key(session, group)}).Err(); err != nil {
		s.logger.Warn("reputation clear failed, ignoring",
			zap.String("session", session), zap.String("group", group), zap.Error(err))
	}
}

// delScript apaga a chave de reputação. Mantido como script pela mesma razão de
// getScript: opera sobre redis.Scripter.
var delScript = redis.NewScript(`return redis.call('DEL', KEYS[1])`)
