package text

import (
	"regexp"
	"strconv"
	"testing"
)

// gluedURLEmojiRe detecta o bug: uma corrida de URL (ASCII imprimível) seguida
// IMEDIATAMENTE por um rune não-ASCII (emoji, ‼️, etc.). URLs RFC 3986 são
// ASCII puro, então qualquer não-ASCII colado no fim do link é o emoji que o
// WhatsApp engole no trecho clicável — o link não abre.
var gluedURLEmojiRe = regexp.MustCompile(`https?://[!-~]+[^\x00-\x7F]`)

// TestVary_NeverGluesEmojiToTrailingURL cobre a queixa "links indo com emoji"
// pela raiz: a variação anti-ban (emojiJitter) anexa um emoji do EmojiSet à
// ÚLTIMA linha não-vazia. Quando essa linha termina numa URL (caso comuníssimo:
// a oferta termina no link de afiliado), o emoji era colado direto no link —
// e isso acontece DEPOIS do NormalizeOutgoingWhatsAppText/SeparateURLEmojiGlue,
// que rodam antes do Vary no worker/monitoring/offer_generate.
//
// Varremos muitas seeds (cada grupo de destino vira uma seed distinta) para
// pegar o ramo aleatório do emojiJitter. Nenhuma saída pode ter emoji colado.
func TestVary_NeverGluesEmojiToTrailingURL(t *testing.T) {
	t.Parallel()
	// Preset "high" real: EmojiJitter ligado, com o EmojiSet default de produção.
	opts := VaryOptions{
		Spintax:      true,
		ZeroWidth:    true,
		ZeroWidthMax: 4,
		EmojiJitter:  true,
		EmojiSet:     []string{"", "", "🔥", "✅", "👍", "🛒", "⚡"},
		PunctJitter:  true,
		LineBreak:    true,
	}

	// Mensagens que TERMINAM numa URL — o cenário que quebrava.
	inputs := []string{
		"🔥 OFERTA IMPERDÍVEL 🔥\nAproveite agora:\nhttps://s.shopee.com.br/qem5OVT0i",
		"Fone bluetooth com 70% OFF\nhttps://mercadolivre.com/sec/1AbC2dE",
		"https://s.shopee.com.br/AbCdEf123",
		"Corre que acaba!\n\nLink: https://shp.ee/xyz9876",
	}

	for _, in := range inputs {
		for i := 0; i < 3000; i++ {
			seed := "5511" + strconv.Itoa(900000000+i) + "@g.us"
			got := Vary(in, opts, seed)
			if gluedURLEmojiRe.MatchString(got) {
				t.Fatalf("Vary colou emoji no link (seed=%s)\n in: %q\nout: %q", seed, in, got)
			}
		}
	}
}

// TestSendPipeline_NeverEmitsGluedURLEmoji simula FIELMENTE a sequência que o
// worker aplica antes do WAHA (message_worker.go): NormalizeOutgoingWhatsAppText
// -> Vary -> [mapper] SeparateURLEmojiGlue. Prova ponta-a-ponta que, seja qual
// for a origem da mensagem (fonte com emoji colado, template do usuário
// terminando no link, texto de IA que o prompt manda encher de emoji, links no
// meio da linha), NENHUM link sai com emoji colado. Cobre "independente da
// origem ou do caminho".
func TestSendPipeline_NeverEmitsGluedURLEmoji(t *testing.T) {
	t.Parallel()
	opts := VaryOptions{
		Spintax:      true,
		ZeroWidth:    true,
		ZeroWidthMax: 4,
		EmojiJitter:  true,
		EmojiSet:     []string{"", "", "🔥", "✅", "👍", "🛒", "⚡"},
		PunctJitter:  true,
		LineBreak:    true,
	}

	// mapperNet replica o chokepoint do mapper (SeparateURLEmojiGlue no campo
	// Text/Caption). sendPipeline é a composição exata do worker.
	sendPipeline := func(in, seed string) string {
		return SeparateURLEmojiGlue(Vary(NormalizeOutgoingWhatsAppText(in), opts, seed))
	}

	inputs := []string{
		// oferta terminando no link (o caso clássico)
		"🔥 SUPER OFERTA\nhttps://s.shopee.com.br/qem5OVT0i",
		// texto de IA: prompt manda usar emoji, LLM cola no link
		"🔥 Fone TWS\n\n💰 Por R$ 89,90\n\n🛒 Compre agora: https://s.shopee.com.br/AbC123🔥",
		// fonte com emoji JÁ colado (espelho/lista)
		"OFERTA https://s.shopee.com.br/qem5OVT0i‼️",
		// emoji com variation selector + ZWJ
		"corre: https://shp.ee/xyz🏃‍♂️",
		// link no MEIO da linha com emoji colado + mais texto depois
		"Veja https://a.com/x🔥 e também https://mercadolivre.com/sec/1AbC agora",
		// link cru sozinho
		"https://s.shopee.com.br/AbCdEf123",
		// template do usuário: emoji colado direto no placeholder renderizado
		"Aproveite 👉https://s.shopee.com.br/zzz e compartilhe https://s.shopee.com.br/www😍",
	}

	for _, in := range inputs {
		for i := 0; i < 2000; i++ {
			seed := "5511" + strconv.Itoa(980000000+i) + "@g.us"
			got := sendPipeline(in, seed)
			if gluedURLEmojiRe.MatchString(got) {
				t.Fatalf("pipeline emitiu link com emoji colado (seed=%s)\n in: %q\nout: %q", seed, in, got)
			}
		}
	}
}
