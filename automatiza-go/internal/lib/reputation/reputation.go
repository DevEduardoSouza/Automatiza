// Package reputation implements a Redis-backed per-(session,group) reputation
// and suppression store for anti-ban. Um grupo de WhatsApp que falha ou remove o
// bot repetidamente acumula pontuação; ao cruzar um limiar ele passa a ser
// "suprimido" e o dispatcher deixa de enviar para ele até que se recupere.
//
// Todos os métodos são fail-open: qualquer erro de Redis (ou Redis ausente) é
// logado em Warn e ignorado, então um problema de infraestrutura nunca bloqueia
// um envio nem suprime um grupo por engano.
package reputation

import (
	"context"
	"strconv"
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
	return &Store{rdb: rdb, threshold: threshold, ttl: ttl, logger: logger}
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

// IsSuppressed informa se a pontuação de (sessão, grupo) atingiu o limiar
// (score >= threshold). Fail-open: devolve false em store/rdb nil, argumentos
// vazios ou qualquer erro de Redis — nunca suprime por causa de falha.
func (s *Store) IsSuppressed(ctx context.Context, session, group string) bool {
	if s == nil || s.rdb == nil || session == "" || group == "" {
		return false
	}
	// redis.Scripter não expõe GET; go-redis o resolve via EvalSha/Eval, mas aqui
	// basta um GET simples via script para manter a mesma interface injetável.
	res, err := getScript.Run(ctx, s.rdb, []string{key(session, group)}).Text()
	if err != nil {
		s.logger.Warn("reputation lookup failed, not suppressing",
			zap.String("session", session), zap.String("group", group), zap.Error(err))
		return false
	}
	if res == "" {
		return false
	}
	score, err := strconv.Atoi(res)
	if err != nil {
		s.logger.Warn("reputation score parse failed, not suppressing",
			zap.String("session", session), zap.String("group", group),
			zap.String("raw", res), zap.Error(err))
		return false
	}
	return score >= s.threshold
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
