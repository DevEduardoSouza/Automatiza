package relaymedia

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap/zaptest"
)

// fakeSender registra qual método foi chamado — é o oráculo que prova "um
// envio pelo método correto, e nenhum outro método".
type fakeSender struct {
	text, image, video, voice, doc int
	lastImage                      whatsapp.SendImageInput
	lastText                       whatsapp.SendTextInput
	err                            error
}

func (f *fakeSender) SendText(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	f.text++
	f.lastText = in
	return &whatsapp.MessageResult{}, f.err
}
func (f *fakeSender) SendImage(_ context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	f.image++
	f.lastImage = in
	return &whatsapp.MessageResult{}, f.err
}
func (f *fakeSender) SendVideo(_ context.Context, _ whatsapp.SendVideoInput) (*whatsapp.MessageResult, error) {
	f.video++
	return &whatsapp.MessageResult{}, f.err
}
func (f *fakeSender) SendVoice(_ context.Context, _ whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	f.voice++
	return &whatsapp.MessageResult{}, f.err
}
func (f *fakeSender) SendDocument(_ context.Context, _ whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error) {
	f.doc++
	return &whatsapp.MessageResult{}, f.err
}

type fakeDownloader struct {
	calls int
	data  []byte
	mime  string
	err   error
}

func (f *fakeDownloader) DownloadMediaByURL(_ context.Context, _ string) (*whatsapp.DownloadMediaResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &whatsapp.DownloadMediaResult{Data: f.data, Mimetype: f.mime}, nil
}

func prepared(b64, mime, sha string) Prepared {
	return Prepared{Base64: b64, Mime: mime, SHA256: sha, Reason: models.ReasonNone}
}

// TestCadaTipoUsaSeuMetodoEExatamenteUmaVez é o oráculo da matriz §13.1.2.
func TestCadaTipoUsaSeuMetodoEExatamenteUmaVez(t *testing.T) {
	casos := []struct {
		mediaType string
		check     func(*fakeSender) (int, string)
	}{
		{"image", func(f *fakeSender) (int, string) { return f.image, "image" }},
		{"video", func(f *fakeSender) (int, string) { return f.video, "video" }},
		{"audio", func(f *fakeSender) (int, string) { return f.voice, "voice" }},
		{"ptt", func(f *fakeSender) (int, string) { return f.voice, "voice" }},
		{"document", func(f *fakeSender) (int, string) { return f.doc, "document" }},
	}
	for _, c := range casos {
		t.Run(c.mediaType, func(t *testing.T) {
			f := &fakeSender{}
			_, err := Send(context.Background(), f, SendInput{
				Session: "s", ChatID: "g@g.us", Body: "legenda",
				MediaType: c.mediaType, Media: prepared("Ynl0ZXM=", "image/jpeg", "abc"),
			}, zaptest.NewLogger(t))
			if err != nil {
				t.Fatalf("erro: %v", err)
			}
			n, nome := c.check(f)
			if n != 1 {
				t.Errorf("%s chamado %d vezes, queria 1", nome, n)
			}
			total := f.text + f.image + f.video + f.voice + f.doc
			if total != 1 {
				t.Errorf("%d chamadas no total, queria exatamente 1", total)
			}
		})
	}
}

// TestSemBytesNuncaDevolveMediaURLParaWAHA é a correção nº 1 do pacote: a URL
// do store interno passada de volta faz a WAHA buscá-la sem a API key e levar
// 401→500. O cliente tem de receber ao menos a legenda.
func TestSemBytesNuncaDevolveMediaURLParaWAHA(t *testing.T) {
	f := &fakeSender{}
	res, err := Send(context.Background(), f, SendInput{
		Session: "s", ChatID: "g@g.us", Body: "legenda importante",
		MediaType: "image",
		Media:     Prepared{Reason: models.ReasonMediaUnavailable},
	}, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if f.image != 0 {
		t.Error("SendImage não pode ser chamado sem bytes")
	}
	if f.text != 1 {
		t.Errorf("SendText chamado %d vezes, queria 1 (o texto não pode se perder)", f.text)
	}
	if f.lastText.Text != "legenda importante" {
		t.Errorf("texto = %q", f.lastText.Text)
	}
	if res.Reason != models.ReasonMediaUnavailable {
		t.Errorf("reason = %q, queria media_unavailable", res.Reason)
	}
	if !res.TextOnly {
		t.Error("TextOnly deveria ser true")
	}
}

// TestTipoDesconhecidoNaoViraImagem é a correção nº 2: mandar documento ou
// sticker por SendImage chega corrompido no grupo do cliente.
func TestTipoDesconhecidoNaoViraImagem(t *testing.T) {
	f := &fakeSender{}
	res, err := Send(context.Background(), f, SendInput{
		Session: "s", ChatID: "g@g.us", Body: "olha isso",
		MediaType: "sticker", Media: prepared("Ynl0ZXM=", "application/octet-stream", "abc"),
	}, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if f.image != 0 {
		t.Error("tipo desconhecido não pode ser enviado como imagem")
	}
	if f.text != 1 {
		t.Errorf("o texto deveria sair mesmo assim (%d chamadas)", f.text)
	}
	if res.Reason != models.ReasonUnsupportedMedia {
		t.Errorf("reason = %q, queria unsupported_media", res.Reason)
	}
}

func TestTipoDesconhecidoSemTextoNaoEnviaNada(t *testing.T) {
	f := &fakeSender{}
	res, _ := Send(context.Background(), f, SendInput{
		Session: "s", ChatID: "g@g.us", Body: "  ",
		MediaType: "sticker", Media: prepared("Ynl0ZXM=", "x", "abc"),
	}, zaptest.NewLogger(t))
	if f.text+f.image+f.video+f.voice+f.doc != 0 {
		t.Error("sem texto e sem método dedicado, nada pode sair")
	}
	if res.Reason != models.ReasonUnsupportedMedia {
		t.Errorf("reason = %q", res.Reason)
	}
}

func TestSemMidiaNemTextoNaoEnviaEnaoErra(t *testing.T) {
	f := &fakeSender{}
	_, err := Send(context.Background(), f, SendInput{
		Session: "s", ChatID: "g@g.us", Body: "   ", Media: Prepared{Reason: models.ReasonNone},
	}, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("evento de sistema não é erro: %v", err)
	}
	if f.text != 0 {
		t.Error("nada deveria ser enviado")
	}
}

// ---------------------------------------------------------------------------
// Prepare
// ---------------------------------------------------------------------------

func TestPrepareBaixaUmaVezSoEUsaBase64Inline(t *testing.T) {
	dl := &fakeDownloader{data: []byte("bytes-do-waha"), mime: "image/png"}
	msg := &models.IncomingMessage{
		HasMedia: true, MediaType: "image", MediaURL: "https://waha/api/files/x.jpg",
	}
	p := Prepare(context.Background(), dl, msg, zaptest.NewLogger(t))

	if dl.calls != 1 {
		t.Errorf("%d downloads, queria 1", dl.calls)
	}
	if !p.HasBytes() {
		t.Fatal("deveria ter bytes")
	}
	if p.Mime != "image/png" {
		t.Errorf("mime = %q", p.Mime)
	}
	if p.SHA256 == "" {
		t.Error("SHA256 é obrigatório para o fingerprint")
	}

	// Com base64 inline, não baixa nada.
	dl2 := &fakeDownloader{data: []byte("nunca")}
	msg2 := &models.IncomingMessage{
		HasMedia: true, MediaType: "image", MimeType: "image/jpeg",
		MediaBase64: base64.StdEncoding.EncodeToString([]byte("inline")),
		MediaURL:    "https://waha/api/files/y.jpg",
	}
	p2 := Prepare(context.Background(), dl2, msg2, zaptest.NewLogger(t))
	if dl2.calls != 0 {
		t.Errorf("%d downloads com base64 inline, queria 0", dl2.calls)
	}
	if !p2.HasBytes() {
		t.Error("base64 inline deveria bastar")
	}
}

func TestPrepareDownloadFalhoNaoPropagaMediaURL(t *testing.T) {
	dl := &fakeDownloader{err: errors.New("401")}
	msg := &models.IncomingMessage{
		HasMedia: true, MediaType: "image", MediaURL: "https://waha/api/files/secreto.jpg",
	}
	p := Prepare(context.Background(), dl, msg, zaptest.NewLogger(t))

	if p.HasBytes() {
		t.Error("sem download não pode haver bytes")
	}
	if p.Reason != models.ReasonMediaUnavailable {
		t.Errorf("reason = %q, queria media_unavailable", p.Reason)
	}
	// A struct Prepared não tem campo de URL: é estrutural, não é convenção.
	if strings.Contains(p.Base64+p.Mime+p.SHA256, "secreto") {
		t.Error("MediaURL vazou para o Prepared")
	}
}

// ---------------------------------------------------------------------------
// Fingerprint
// ---------------------------------------------------------------------------

// TestFingerprintSemMidiaPreservaAChaveDeHoje é a garantia de
// retrocompatibilidade: o dedup em voo no momento do deploy não pode quebrar.
func TestFingerprintSemMidiaPreservaAChaveDeHoje(t *testing.T) {
	var p Prepared
	const body = "mensagem qualquer"
	if got := p.Fingerprint(body); got != body {
		t.Errorf("Fingerprint sem mídia = %q, tinha de ser o corpo cru", got)
	}
}

// TestFingerprintDistingueDuasMidiasSemLegenda é o bug §3.9: hoje as duas
// colapsam no hash do corpo vazio e o dedup de 2min confunde uma com a outra.
func TestFingerprintDistingueDuasMidiasSemLegenda(t *testing.T) {
	a := prepared("AAAA", "image/jpeg", "aaaaaaaa")
	b := prepared("BBBB", "image/jpeg", "bbbbbbbb")

	fa, fb := a.Fingerprint(""), b.Fingerprint("")
	if fa == fb {
		t.Fatal("duas imagens distintas sem legenda geraram o mesmo fingerprint")
	}
	if fa == "" || fb == "" {
		t.Fatal("fingerprint de mídia não pode ser vazio")
	}
	// Mesma mídia → mesmo fingerprint (o dedup real precisa funcionar).
	if a.Fingerprint("") != prepared("AAAA", "image/jpeg", "aaaaaaaa").Fingerprint("") {
		t.Error("a mesma mídia deveria dar o mesmo fingerprint")
	}
}

func TestFingerprintMimeNormalizado(t *testing.T) {
	a := prepared("X", "IMAGE/JPEG", "abc")
	b := prepared("X", "image/jpeg", "abc")
	if a.Fingerprint("legenda") != b.Fingerprint("legenda") {
		t.Error("mime deveria ser comparado case-insensitive")
	}
}
