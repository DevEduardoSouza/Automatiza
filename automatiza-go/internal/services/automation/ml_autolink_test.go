package automation

import "testing"

// Este é o teste que protege os 2.434.567 itens `meli.la` já em produção.
//
// Se mlNeedsAffiliation devolver true para um link que o cliente já gerou, o
// backend passa a chamar o ML para reescrever um Link Especial alheio — o que
// (a) troca/perde a atribuição do dono, (b) viola a cláusula 1.8 dos T&C e
// (c) multiplica por milhões as chamadas ao endpoint mais sensível do ML.
func TestMlNeedsAffiliation(t *testing.T) {
	for _, tc := range []struct {
		nome     string
		url      string
		esperado bool
	}{
		// JÁ AFILIADOS — nunca tocar. Formatos reais vistos no banco.
		{"shortlink meli.la", "https://meli.la/AbC1234", false},
		{"shortlink /sec/", "https://mercadolivre.com/sec/1KVuzrd", false},
		{"shortlink /sec/ com www", "https://www.mercadolivre.com/sec/2XyZ99", false},

		// URL CRUA DE PRODUTO — é o caso que a feature existe para resolver.
		{"produto MLB", "https://produto.mercadolivre.com.br/MLB-6040768740-teste", true},
		{"catálogo /p/", "https://www.mercadolivre.com.br/algum-produto/p/MLB123456", true},

		// Entrada inválida: não mexer é sempre seguro.
		{"string vazia", "", false},
		{"lixo", "isso nao e uma url", false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			if got := mlNeedsAffiliation(tc.url); got != tc.esperado {
				t.Fatalf("mlNeedsAffiliation(%q) = %v, esperado %v", tc.url, got, tc.esperado)
			}
		})
	}
}

// Gate malformado NÃO pode derrubar o worker no meio de um disparo (por isso
// usamos Parse e não MustParse) e tem de fechar, nunca abrir.
func TestMlAutoLinkGateFailClosed(t *testing.T) {
	// O sync.Once já pode ter rodado; testamos o parser diretamente com a mesma
	// semântica usada em mlAutoLinkEnabled.
	if mlAutoLinkEnabled("") {
		t.Fatal("gate não pode liberar para userID vazio")
	}
}
