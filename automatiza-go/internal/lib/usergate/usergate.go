// Package usergate lê um gate de rollout por usuário no formato
// `off | all | <csv de UUIDs>`.
//
// Regra que define o pacote: qualquer token que não seja `off`, `all` ou um
// UUID válido é ERRO DE BOOT, nunca interpretado como `all`. Um gate que "abre
// no default" quando alguém erra a digitação da env é como uma feature nova
// chega a 870 usuários sem ninguém decidir isso.
package usergate

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Gate é a decisão já parseada. O valor zero é um gate fechado, então um campo
// não inicializado nega em vez de liberar.
type Gate struct {
	all   bool
	users map[uuid.UUID]struct{}
}

// Parse lê o valor cru da env.
//
// Vazio e `off` (case-insensitive) fecham. `all` abre para todos. Caso
// contrário, cada item separado por vírgula precisa ser um UUID válido depois
// do trim; duplicatas são absorvidas.
func Parse(raw string) (Gate, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "off") {
		return Gate{}, nil
	}
	if strings.EqualFold(trimmed, "all") {
		return Gate{all: true}, nil
	}

	users := make(map[uuid.UUID]struct{})
	for _, part := range strings.Split(trimmed, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		// `off`/`all` misturados num CSV são ambíguos por construção: o
		// operador quis uma coisa e escreveu outra. Recusar é mais barato que
		// adivinhar.
		if strings.EqualFold(token, "off") || strings.EqualFold(token, "all") {
			return Gate{}, fmt.Errorf("usergate: %q não pode aparecer numa lista de usuários", token)
		}
		id, err := uuid.Parse(token)
		if err != nil {
			return Gate{}, fmt.Errorf("usergate: token inválido na lista (esperado UUID): %w", err)
		}
		users[id] = struct{}{}
	}
	if len(users) == 0 {
		return Gate{}, nil
	}
	return Gate{users: users}, nil
}

// MustParse é para wiring de boot: um gate malformado derruba o processo na
// subida, quando ainda dá para consertar a env, em vez de silenciosamente
// liberar (ou bloquear) em produção.
func MustParse(name, raw string) Gate {
	g, err := Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("gate %s malformado: %v", name, err))
	}
	return g
}

// Allows reporta se o gate libera para este usuário.
func (g Gate) Allows(userID uuid.UUID) bool {
	if g.all {
		return true
	}
	if g.users == nil {
		return false
	}
	_, ok := g.users[userID]
	return ok
}

// AllowsString aceita o ID em texto. ID não parseável nega — nunca libera por
// não conseguir ler o argumento.
func (g Gate) AllowsString(userID string) bool {
	id, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return false
	}
	return g.Allows(id)
}

// GloballyOff reporta se o gate está fechado para todo mundo. É o que decide se
// handlers e crons do caminho novo chegam a ser registrados: gate globalmente
// off = componente inerte, sem handler, sem fila, sem exigir chave de
// criptografia.
func (g Gate) GloballyOff() bool {
	return !g.all && len(g.users) == 0
}

// String descreve o gate sem listar UUIDs (log de boot não é lugar de despejar
// a lista de clientes do canário).
func (g Gate) String() string {
	switch {
	case g.all:
		return "all"
	case len(g.users) == 0:
		return "off"
	default:
		return fmt.Sprintf("allowlist(%d)", len(g.users))
	}
}
