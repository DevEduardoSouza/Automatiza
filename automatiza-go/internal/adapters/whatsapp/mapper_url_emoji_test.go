package whatsapp

import "testing"

// O mapper é o chokepoint final antes do WAHA. Garante que NENHUM envio
// (texto, imagem, vídeo, documento, card de preview) solte um emoji colado no
// fim de uma URL, independente do caminho que montou a mensagem. Cobre a
// queixa "links indo com emoji" na fronteira que TODO send atravessa.
func TestMapper_SeparatesGluedURLEmojiOnEveryTextField(t *testing.T) {
	m := NewMapper()
	const glued = "OFERTA https://s.shopee.com.br/qem5OVT0i‼️"
	const want = "OFERTA https://s.shopee.com.br/qem5OVT0i ‼️"

	if got := m.SendTextToWaha(&SendTextInput{Text: glued}).Text; got != want {
		t.Errorf("SendTextToWaha.Text\n got: %q\nwant: %q", got, want)
	}
	if got := m.SendImageToWaha(&SendImageInput{Caption: glued, ImageURL: "http://x/y.jpg"}).Caption; got != want {
		t.Errorf("SendImageToWaha.Caption\n got: %q\nwant: %q", got, want)
	}
	if got := m.SendVideoToWaha(&SendVideoInput{Caption: glued, MediaURL: "http://x/y.mp4"}).Caption; got != want {
		t.Errorf("SendVideoToWaha.Caption\n got: %q\nwant: %q", got, want)
	}
	if got := m.SendDocumentToWaha(&SendDocumentInput{Caption: glued, MediaURL: "http://x/y.pdf"}).Caption; got != want {
		t.Errorf("SendDocumentToWaha.Caption\n got: %q\nwant: %q", got, want)
	}
	if got := m.SendLinkPreviewToWaha(&SendLinkPreviewInput{Text: glued, URL: "https://s.shopee.com.br/qem5OVT0i"}).Text; got != want {
		t.Errorf("SendLinkPreviewToWaha.Text\n got: %q\nwant: %q", got, want)
	}
}

// Idempotência + no-op: texto já separado, ou sem fronteira URL→emoji, não é
// reescrito. Protege o relay verbatim (que já roda SeparateURLEmojiGlue antes)
// de ganhar espaços duplicados ou perder formatação.
func TestMapper_URLEmojiGuardIsIdempotentAndNoop(t *testing.T) {
	m := NewMapper()
	cases := []string{
		"OFERTA https://s.shopee.com.br/qem5OVT0i ‼️",            // já tem espaço
		"promoção 🔥🔥 sem link",                                   // sem URL
		"*bold* _https://s.shopee.com.br/abc_",                   // formatação ASCII intacta
		"linha1\n\n\n\nlinha2 https://mercadolivre.com/sec/1AbC", // sem emoji colado
	}
	for _, in := range cases {
		if got := m.SendTextToWaha(&SendTextInput{Text: in}).Text; got != in {
			t.Errorf("guard mexeu em texto que devia ficar intacto\n got: %q\nwant: %q", got, in)
		}
	}
}
