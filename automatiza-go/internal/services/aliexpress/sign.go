package aliexpress

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// sign implementa a assinatura do gateway TOP (api-sg.aliexpress.com/sync):
// ordena os parâmetros por nome, concatena "chave+valor" sem separador, e tira
// o HMAC-SHA256 com o app secret, em hexadecimal MAIÚSCULO.
//
// Isolado em arquivo próprio porque é o único ponto desta integração que falha
// em SILÊNCIO: uma assinatura errada não estoura exceção, volta como erro de
// negócio genérico do AliExpress. O teste ao lado congela o algoritmo; a prova
// de que ele casa com o servidor só existe contra a API real — e é isso que o
// validador de credencial faz no cadastro.
func sign(params map[string]string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == signParam {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params[k])
	}

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(sb.String()))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}
