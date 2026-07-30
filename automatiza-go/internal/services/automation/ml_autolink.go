package automation

import (
	"os"
	"sync"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/usergate"
)

// Gate da conversão automática de link do Mercado Livre (ML_AUTO_LINK):
// "off"/vazio (default) | "all" | lista de UUIDs.
//
// Deliberadamente usa Parse e NÃO usergate.MustParse: MustParse panica, e este
// gate é lido no primeiro disparo (não no boot), então um valor malformado
// derrubaria o worker NO MEIO de um envio. Fail-closed silencioso é a escolha
// certa aqui — o pior caso é o ML continuar em passthrough, que é exatamente o
// comportamento de hoje. mlAutoLinkErr guarda o motivo para o caller logar uma
// vez, sem transformar env errada em incidente de entrega.
var (
	mlAutoLinkOnce sync.Once
	mlAutoLinkGate usergate.Gate
	mlAutoLinkErr  error
)

func mlAutoLinkEnabled(userID string) bool {
	mlAutoLinkOnce.Do(func() {
		mlAutoLinkGate, mlAutoLinkErr = usergate.Parse(os.Getenv("ML_AUTO_LINK"))
	})
	return mlAutoLinkGate.AllowsString(userID)
}

// mlNeedsAffiliation reporta se esta URL do ML deve ser convertida por nós.
//
// É o guard que protege os 2.434.567 itens `meli.la`/`/sec/` já em produção:
// esses links foram gerados pelo PRÓPRIO dono no linkbuilder do ML e carregam a
// atribuição dele. Tocá-los seria, na melhor hipótese, trabalho perdido e, na
// pior, troca de atribuição — além de violar a cláusula 1.8 dos T&C do
// programa, que proíbe editar um Link Especial.
//
// URL que não conseguimos sequer parsear devolve false: não mexer é sempre
// seguro, converter às cegas não.
func mlNeedsAffiliation(rawURL string) bool {
	u, err := commerceurl.Parse(rawURL)
	if err != nil {
		return false
	}
	return !commerceurl.IsMLShortener(u)
}
