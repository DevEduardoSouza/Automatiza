package text

import "testing"

// Cobre a queixa "ofertas indo com emoji no link" (dayanemira, 2026-07-15):
// emoji colado no fim da URL faz o WhatsApp incluir o emoji no trecho
// clicável e o redirecionamento quebra. O relay as-is repassa o corpo da
// fonte verbatim, então depende desta função (e não do normalizador inteiro,
// que reescreveria a formatação do canal de origem).
func TestSeparateURLEmojiGlue_RelayBody(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "emoji colado depois da URL ganha espaco",
			in:   "OFERTA https://s.shopee.com.br/qem5OVT0i‼️",
			want: "OFERTA https://s.shopee.com.br/qem5OVT0i ‼️",
		},
		{
			name: "emoji antes da URL nao e alterado (WhatsApp linkifica pelo https)",
			in:   "Compre 👉https://s.shopee.com.br/qem5OVT0i",
			want: "Compre 👉https://s.shopee.com.br/qem5OVT0i",
		},
		{
			name: "formatacao ASCII do WhatsApp fica intacta",
			in:   "*oferta* _https://s.shopee.com.br/abc_",
			want: "*oferta* _https://s.shopee.com.br/abc_",
		},
		{
			name: "quebras de linha da fonte sao preservadas (relay as-is)",
			in:   "linha1\n\n\n\nlinha2 https://mercadolivre.com/sec/1AbC🔥",
			want: "linha1\n\n\n\nlinha2 https://mercadolivre.com/sec/1AbC 🔥",
		},
		{
			name: "texto sem URL nao e tocado",
			in:   "promoção 🔥🔥 imperdível",
			want: "promoção 🔥🔥 imperdível",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SeparateURLEmojiGlue(tt.in); got != tt.want {
				t.Errorf("SeparateURLEmojiGlue()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
