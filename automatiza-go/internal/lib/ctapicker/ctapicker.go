// Package ctapicker escolhe o texto do CTA aleatoriamente entre os CTAs
// elegíveis para a plataforma do produto detectado. Compartilhado por
// automation (list/search modes) e offer_generate.
//
// "Vínculo automático" do affilipro: cliente cadastra CTAs com platform
// (ml, shopee, all) e o sistema sempre filtra pela plataforma do produto
// antes de sortear. Sem isso, uma automation com CTAs misturados
// (Shopee + ML + all) podia mandar o texto da Shopee num link ML.
package ctapicker

import (
	"math/rand/v2"

	"automatiza-go/internal/models"
)

// PickByPlatform retorna o texto de um CTA elegível, sorteado uniforme.
//
// Filtros aplicados:
//   - is_active = true
//   - platform == platform do produto OU platform == "all"
//
// Retorna string vazia quando nenhum CTA elegível existe — caller decide
// fallback (ex.: usar inline CTAs da automation, ou deixar a mensagem
// sair sem CTA).
func PickByPlatform(ctas []models.CTA, isML, isShopee bool) string {
	if len(ctas) == 0 {
		return ""
	}
	var targetPlatform string
	switch {
	case isML:
		targetPlatform = models.CTAPlatformMercadoLivre
	case isShopee:
		targetPlatform = models.CTAPlatformShopee
	}
	candidates := make([]models.CTA, 0, len(ctas))
	for _, c := range ctas {
		if !c.IsActive {
			continue
		}
		if c.Platform == models.CTAPlatformAll || (targetPlatform != "" && c.Platform == targetPlatform) {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rand.IntN(len(candidates))].Message //nolint:gosec // G404: non-security random for CTA selection
}
