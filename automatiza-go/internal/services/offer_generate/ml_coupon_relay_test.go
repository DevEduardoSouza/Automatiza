package offer_generate //nolint:revive // nome do pacote já existe com underscore; renomear está fora de escopo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/mercadolivre"

	"go.uber.org/zap"
)

// Bug (relato 20/07): aviso de cupom do Mercado Livre postado no grupo monitor
// saía BUGADO nos grupos destino porque o ramo ML do offer_generate forçava a
// mensagem no pipeline de OFERTA. Diferente do Shopee (que tem gate de link
// não-produto), o ML montava um card com o preview de uma página de cupom —
// preview vazio ou com título/preço errados — ou fazia `return nil` e a
// mensagem SUMIA. O fix desvia links ML sem dados de produto para um aviso de
// cupom que preserva o texto original e converte os links ao afiliado do dono.

const (
	mlCouponSecLink = "https://mercadolivre.com.br/sec/2Ab3Cd4"
	mlProductLink   = "https://www.mercadolivre.com.br/fone-bluetooth/p/MLB123456"
	mlCouponBody    = "SAIU CUPOM! R$50 OFF acima de R$300 no ML 🔗 https://mercadolivre.com.br/sec/2Ab3Cd4"
)

// couponFakeML embeda a interface e sobrescreve só os 2 métodos exercidos pelo
// desvio de aviso de cupom (GetProductPreview decide a rota; CreateAffiliateLink
// converte no ramo de cupom).
type couponFakeML struct {
	mercadolivre.Service
	preview    *models.MLProductPreview
	previewErr error
	// affByOrig mapeia link original -> afiliado; ausente => passthrough (o link
	// segue igual, como o CreateAffiliateLink real faz p/ meli.la/sec).
	affByOrig map[string]*models.MLAffiliateLink
	affErr    error

	gotConvertOrigins []string
	gotConvertUserIDs []string
}

func (f *couponFakeML) GetProductPreview(_ context.Context, _, _ string) (*models.MLProductPreview, error) {
	if f.previewErr != nil {
		return nil, f.previewErr
	}
	return f.preview, nil
}

func (f *couponFakeML) CreateAffiliateLink(_ context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	f.gotConvertOrigins = append(f.gotConvertOrigins, originURL)
	f.gotConvertUserIDs = append(f.gotConvertUserIDs, userID)
	if f.affErr != nil {
		return nil, f.affErr
	}
	if f.affByOrig != nil {
		if l, ok := f.affByOrig[originURL]; ok {
			return l, nil
		}
	}
	return &models.MLAffiliateLink{OriginalURL: originURL, AffiliateLink: originURL}, nil
}

func newMLCouponSvc(ml *couponFakeML, sent *[]whatsapp.SendTextInput) *offerGenerateService {
	fake := whatsapp.NewFakeAdapter()
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		*sent = append(*sent, in)
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	return &offerGenerateService{
		logger:       zap.NewNop(),
		waProvider:   fake,
		instanceRepo: couponFakeInstanceRepo{},
		settingsRepo: couponFakeSettingsRepo{},
		mlService:    ml,
	}
}

func newMLCouponAutomation() *models.Automation {
	dtID := "dt-1"
	zero := 0
	return &models.Automation{
		ID:             "auto-ml-coupon",
		UserID:         couponOwnerID,
		Name:           "Gerar Oferta ML",
		Status:         models.AutomationStatusActive,
		DispatchTextID: &dtID,
		SendGroups:     []string{couponDestGroup},
		DelaySeconds:   &zero,
		Stores:         []string{"mercadolivre"}, // sem isto o default ["shopee"] zera os mlLinks
	}
}

func newMLCouponMsg(body string) *models.IncomingMessage {
	return &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-ml-coupon-1",
		RemoteJID:    couponMonitorGroup,
		Body:         body,
	}
}

// Caso central: link ML de cupom/campanha (preview sem produto) → sai como aviso
// de cupom com o texto original preservado, NÃO como card de oferta.
func TestProcessAutomation_MLCouponLink_RelaysCouponNotice(t *testing.T) {
	var sent []whatsapp.SendTextInput
	// Preview vazio não-nil: exatamente o que o scrape de uma página de cupom
	// devolve (sem Title/Price) — HasProductData()==false.
	ml := &couponFakeML{preview: &models.MLProductPreview{Permalink: mlCouponSecLink, Currency: "BRL"}}
	svc := newMLCouponSvc(ml, &sent)

	msg := newMLCouponMsg(mlCouponBody)
	if err := svc.processAutomation(context.Background(), newMLCouponAutomation(), msg, []string{mlCouponSecLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}

	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio de aviso de cupom, obtido %d (aviso ML virou oferta ou foi descartado)", len(sent))
	}
	if sent[0].ChatID != couponDestGroup {
		t.Errorf("enviado para %q, esperado grupo destino %q", sent[0].ChatID, couponDestGroup)
	}
	if !strings.Contains(sent[0].Text, "🎟️ *CUPOM MERCADO LIVRE*") {
		t.Errorf("faltou o cabeçalho de cupom.\n got: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, "R$50 OFF acima de R$300") {
		t.Errorf("o texto original do aviso foi perdido.\n got: %q", sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, mlCouponSecLink) {
		t.Errorf("o link do cupom sumiu do corpo.\n got: %q", sent[0].Text)
	}
	// msg original não pode ser mutada: o mesmo ponteiro serve outras automações.
	if msg.Body != mlCouponBody {
		t.Errorf("msg.Body foi mutada (vaza entre donos).\n got: %q\nwant: %q", msg.Body, mlCouponBody)
	}
}

// Produto ML REAL continua no caminho de oferta — o fix não pode desviar oferta
// legítima. Sinal decisivo: CreateAffiliateLink só é chamado no ramo de cupom
// (o ramo de oferta ML faz passthrough do link, sem convertê-lo).
func TestProcessAutomation_MLProductLink_UsesOfferPathNotCouponNotice(t *testing.T) {
	var sent []whatsapp.SendTextInput
	ml := &couponFakeML{preview: &models.MLProductPreview{
		Title:     "Fone Bluetooth",
		Price:     149.90,
		Image:     "https://http2.mlstatic.com/x.jpg",
		Permalink: mlProductLink,
	}}
	svc := newMLCouponSvc(ml, &sent)
	fake := svc.waProvider.(*whatsapp.FakeAdapter)

	msg := newMLCouponMsg("Oferta imperdível " + mlProductLink)
	if err := svc.processAutomation(context.Background(), newMLCouponAutomation(), msg, []string{mlProductLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}

	if len(ml.gotConvertOrigins) != 0 {
		t.Errorf("produto real foi roteado para o aviso de cupom (CreateAffiliateLink chamado %d×)", len(ml.gotConvertOrigins))
	}
	totalSends := fake.CallCount("SendText") + fake.CallCount("SendImage") + fake.CallCount("SendLinkCustomPreview")
	if totalSends == 0 {
		t.Fatal("oferta de produto não enviou nada")
	}
	for _, s := range sent {
		if strings.Contains(s.Text, "CUPOM MERCADO LIVRE") {
			t.Errorf("oferta de produto saiu com cabeçalho de cupom.\n got: %q", s.Text)
		}
	}
}

// Preview falha (rede/HTTP 500): a mensagem NÃO pode sumir — antes o `return nil`
// engolia o aviso. Agora cai no aviso de cupom.
func TestProcessAutomation_MLPreviewError_FallsBackToCouponNotice(t *testing.T) {
	var sent []whatsapp.SendTextInput
	ml := &couponFakeML{previewErr: errors.New("ml preview http 500")}
	svc := newMLCouponSvc(ml, &sent)

	if err := svc.processAutomation(context.Background(), newMLCouponAutomation(), newMLCouponMsg(mlCouponBody), []string{mlCouponSecLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("aviso de cupom sumiu quando o preview falhou (esperado 1 envio, obtido %d)", len(sent))
	}
	if !strings.Contains(sent[0].Text, "🎟️ *CUPOM MERCADO LIVRE*") {
		t.Errorf("faltou cabeçalho de cupom no fallback.\n got: %q", sent[0].Text)
	}
}

// Conversão do link falha (URL sem item ID / falha transitória): repassa com o
// link ORIGINAL — nunca reter, senão o aviso sumiria.
func TestProcessAutomation_MLCouponConvertFailure_KeepsOriginalLink(t *testing.T) {
	var sent []whatsapp.SendTextInput
	ml := &couponFakeML{
		preview: &models.MLProductPreview{Permalink: mlCouponSecLink, Currency: "BRL"},
		affErr:  errors.New("invalid mercadolivre URL"),
	}
	svc := newMLCouponSvc(ml, &sent)

	if err := svc.processAutomation(context.Background(), newMLCouponAutomation(), newMLCouponMsg(mlCouponBody), []string{mlCouponSecLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio mesmo com conversão falhando, obtido %d", len(sent))
	}
	if !strings.Contains(sent[0].Text, mlCouponSecLink) {
		t.Errorf("link original deveria seguir quando a conversão falha.\n got: %q", sent[0].Text)
	}
}

// URL cheia de campanha (não-produto) com conversão possível → link reatribuído
// ao afiliado do DONO da automação (é o userID que credita a comissão).
func TestProcessAutomation_MLCouponFullURL_ConvertsToOwnerLink(t *testing.T) {
	const (
		fullURL    = "https://www.mercadolivre.com.br/cupom-campanha-julho/MLB999"
		ownerShort = "https://mercadolivre.com.br/sec/OWNERaff"
	)
	var sent []whatsapp.SendTextInput
	ml := &couponFakeML{
		preview:   &models.MLProductPreview{Permalink: fullURL, Currency: "BRL"},
		affByOrig: map[string]*models.MLAffiliateLink{fullURL: {OriginalURL: fullURL, AffiliateLink: ownerShort}},
	}
	svc := newMLCouponSvc(ml, &sent)

	msg := newMLCouponMsg("Campanha ML de julho " + fullURL)
	if err := svc.processAutomation(context.Background(), newMLCouponAutomation(), msg, []string{fullURL}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio, obtido %d", len(sent))
	}
	if !strings.Contains(sent[0].Text, ownerShort) {
		t.Errorf("link não foi reatribuído ao dono.\n got: %q", sent[0].Text)
	}
	if strings.Contains(sent[0].Text, fullURL) {
		t.Errorf("link original da campanha sobreviveu (comissão vaza).\n got: %q", sent[0].Text)
	}
	if len(ml.gotConvertUserIDs) == 0 || ml.gotConvertUserIDs[0] != couponOwnerID {
		t.Errorf("conversão deveria usar o userID do dono %q, got %v", couponOwnerID, ml.gotConvertUserIDs)
	}
}
