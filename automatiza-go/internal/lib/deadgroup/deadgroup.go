// Package deadgroup centraliza a deteccao do erro do WAHA/GOWS quando a sessao
// deixou de ser membro de um grupo de destino. Varios caminhos de envio (worker
// de lista, monitoramento, etc.) precisam reconhecer o mesmo erro para parar de
// martelar grupos dos quais o bot foi removido.
package deadgroup

import "strings"

// IsNotParticipatingError reconhece a resposta do WAHA/GOWS para envios a um
// grupo do qual a sessao deixou de ser membro ("you're not participating in
// that group" / "failed to get group members"). Casa no texto porque o erro
// chega embrulhado como erro comum (WAHA error 500), nao como um tipo dedicado.
func IsNotParticipatingError(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "not participating in that group") ||
		strings.Contains(e, "failed to get group members")
}
