package offer_generate

import (
	"context"
	"encoding/base64"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// Regressão: o relay enviava a URL interna do WAHA (/api/files) no sendImage,
// e o WAHA não conseguia buscar a própria URL protegida (401 → 500). O fix
// baixa a mídia autenticado (DownloadMediaByURL) e manda como base64.
func TestSendRelayMessage_ImageDownloadsAndSendsBase64(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var got whatsapp.SendImageInput
	fake.SendImageFunc = func(_ context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		got = in
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}

	svc := &offerGenerateService{
		logger:     zap.NewNop(),
		waProvider: fake,
	}

	msg := &models.IncomingMessage{
		HasMedia:  true,
		MediaType: "image",
		MediaURL:  "http://localhost:3000/api/files/sess/abc.jpeg",
		Body:      "AGORA É A HORA DE COMPRAR OS COREANOS AMIGAS",
	}

	if err := svc.sendRelayMessage(context.Background(), "sess", "120363@g.us", msg); err != nil {
		t.Fatalf("sendRelayMessage retornou erro: %v", err)
	}

	wantB64 := base64.StdEncoding.EncodeToString([]byte("fake-media-data"))
	if got.ImageBase64 != wantB64 {
		t.Errorf("ImageBase64 = %q, want %q", got.ImageBase64, wantB64)
	}
	if got.ImageURL != "" {
		t.Errorf("ImageURL = %q, esperado vazio (deve mandar base64, não a URL protegida)", got.ImageURL)
	}
	if got.Caption != msg.Body {
		t.Errorf("Caption = %q, want %q", got.Caption, msg.Body)
	}

	var downloaded bool
	for _, c := range fake.Calls {
		if c.Method == "DownloadMediaByURL" {
			downloaded = true
		}
	}
	if !downloaded {
		t.Error("DownloadMediaByURL não foi chamado — relay não baixou a mídia autenticada")
	}
}

// Quando o webhook já trouxe MediaBase64, não precisa baixar nada.
func TestSendRelayMessage_ImageUsesExistingBase64(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var got whatsapp.SendImageInput
	fake.SendImageFunc = func(_ context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		got = in
		return &whatsapp.MessageResult{ID: "msg-2"}, nil
	}

	svc := &offerGenerateService{logger: zap.NewNop(), waProvider: fake}

	msg := &models.IncomingMessage{
		HasMedia:    true,
		MediaType:   "image",
		MediaBase64: "YWxyZWFkeS1iYXNlNjQ=",
		Body:        "legenda",
	}

	if err := svc.sendRelayMessage(context.Background(), "sess", "120363@g.us", msg); err != nil {
		t.Fatalf("sendRelayMessage retornou erro: %v", err)
	}

	if got.ImageBase64 != "YWxyZWFkeS1iYXNlNjQ=" {
		t.Errorf("ImageBase64 = %q, want o base64 já existente", got.ImageBase64)
	}
	for _, c := range fake.Calls {
		if c.Method == "DownloadMediaByURL" {
			t.Error("DownloadMediaByURL foi chamado mesmo com MediaBase64 presente")
		}
	}
}
