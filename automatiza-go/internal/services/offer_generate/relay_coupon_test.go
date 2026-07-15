package offer_generate //nolint:revive // nome do pacote já existe com underscore; renomear está fora de escopo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/shopee"

	"go.uber.org/zap"
)

// --- fakes mínimos (embedam a interface; só sobrescrevem o que é chamado) ---

// couponFakeShopee é usado por PONTEIRO: registra os userIDs recebidos, porque
// é o userID — não o link — que decide de quem é a comissão (shopee_service.go:671
// resolve a credencial por ele, e o app_id que assina a chamada é a atribuição).
type couponFakeShopee struct {
	shopee.Service
	expanded string
	affLink  string
	affErr   error
	// affByOrig mapeia link original -> link de afiliado, para os casos com
	// mais de um link. Se ausente, cai em affLink.
	affByOrig map[string]string

	gotUserIDs []string
	gotOrigins []string
}

func (f *couponFakeShopee) ExpandURL(_ context.Context, _ string) (string, error) {
	return f.expanded, nil
}

// ExtractProductIDs falha exatamente como o serviço real falha numa página de
// campanha/cupom (shopee.com.br/m/...): sem shopID/itemID na URL.
func (f *couponFakeShopee) ExtractProductIDs(_ string) (int64, int64, error) {
	return 0, 0, fmt.Errorf("could not extract product IDs from URL")
}

func (f *couponFakeShopee) GenerateAffiliateLink(_ context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	f.gotUserIDs = append(f.gotUserIDs, userID)
	f.gotOrigins = append(f.gotOrigins, productURL)
	if f.affErr != nil {
		return nil, f.affErr
	}
	link := f.affLink
	if f.affByOrig != nil {
		if l, ok := f.affByOrig[productURL]; ok {
			link = l
		}
	}
	return &models.ShopeeAffiliateLink{AffiliateLink: link, ShortLink: link}, nil
}

type couponFakeInstanceRepo struct {
	repository.InstanceRepository
}

func (couponFakeInstanceRepo) GetUserInstances(_ context.Context, _ string) ([]models.Instance, error) {
	return []models.Instance{{ID: 1, InstanceID: "session_test", Status: models.InstanceStatusConnected}}, nil
}

type couponFakeSettingsRepo struct {
	repository.SettingsRepository
}

func (couponFakeSettingsRepo) GetActiveDispatchTextByID(_ context.Context, _, _ string) (*models.DispatchText, error) {
	return &models.DispatchText{ID: "dt-1", Body: "{{title}} {{link}}", IsActive: true}, nil
}

const (
	couponMonitorGroup = "120363410863986891@g.us"
	couponDestGroup    = "120363408267381739@g.us"
	couponSourceLink   = "https://s.shopee.com.br/3VhcRPRgiQ"
	couponBody         = "🚨 SAIUUUU CUPONS SHOPEE 🚨\n🎟 R$30 OFF acima R$299\n🔗 https://s.shopee.com.br/3VhcRPRgiQ"
	couponOwnerID      = "user-1"
	couponExpandedNP   = "https://shopee.com.br/m/super-ofertas?utm_content=cupom----&mmp_pid=an_18330220029"
)

// newCouponSvc monta o serviço com um fake Shopee e captura os envios.
// cache, dispatchLogRepo, affiliateRepo, ctaRepo, couponRepo, couponHandler,
// groupRepo e mlService ficam nil — todos nil-safe nesse caminho.
func newCouponSvc(sh *couponFakeShopee, sent *[]whatsapp.SendTextInput) *offerGenerateService {
	fake := whatsapp.NewFakeAdapter()
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		*sent = append(*sent, in)
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	return &offerGenerateService{
		logger:        zap.NewNop(),
		waProvider:    fake,
		instanceRepo:  couponFakeInstanceRepo{},
		settingsRepo:  couponFakeSettingsRepo{},
		shopeeService: sh,
	}
}

func newCouponAutomation() *models.Automation {
	dtID := "dt-1"
	zero := 0
	return &models.Automation{
		ID:             "auto-coupon",
		UserID:         couponOwnerID,
		Name:           "Gerar Oferta",
		Status:         models.AutomationStatusActive,
		DispatchTextID: &dtID,
		SendGroups:     []string{couponDestGroup},
		DelaySeconds:   &zero, // mantém o delay aleatório mínimo, sem custo extra
	}
}

func newCouponMsg() *models.IncomingMessage {
	return &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-coupon-1",
		RemoteJID:    couponMonitorGroup,
		Body:         couponBody,
	}
}

// Regressão 1 (incidente isaa_deoliveira@hotmail.com, 04/06): cupom postado no
// grupo monitor era DESCARTADO (ExtractProductIDs falha em página de campanha) e
// nunca chegava aos grupos destino. O cupom precisa chegar.
//
// Regressão 2 (auditoria 15/07, mesma conta): o relay repassava o cupom COMO
// VEIO, com o mmp_pid do afiliado da fonte, e a Shopee creditava a comissão pra
// ele (medido: ~3.6k envios/dia). O link precisa sair reatribuído ao dono.
func TestProcessAutomation_ShopeeCouponLink_RelaysWithConvertedLink(t *testing.T) {
	const affLink = "https://s.shopee.com.br/AFFdoDono"
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{expanded: couponExpandedNP, affLink: affLink}
	svc := newCouponSvc(sh, &sent)

	msg := newCouponMsg()
	if err := svc.processAutomation(context.Background(), newCouponAutomation(), msg, []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}

	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio de relay, obtido %d (cupom foi descartado em vez de repassado)", len(sent))
	}
	if sent[0].ChatID != couponDestGroup {
		t.Errorf("relay enviado para %q, esperado grupo destino %q", sent[0].ChatID, couponDestGroup)
	}
	if !strings.Contains(sent[0].Text, affLink) {
		t.Errorf("relay deveria conter o link de afiliado do dono %q.\n got: %q", affLink, sent[0].Text)
	}
	if strings.Contains(sent[0].Text, couponSourceLink) {
		t.Errorf("relay ainda carrega o link da FONTE %q — comissão vai pro afiliado errado.\n got: %q",
			couponSourceLink, sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, "SAIUUUU CUPONS SHOPEE") {
		t.Errorf("relay perdeu o texto original do cupom.\n got: %q", sent[0].Text)
	}
	// A msg original não pode ser mutada: o mesmo ponteiro é reaproveitado para
	// as outras automações que assistem o grupo-fonte.
	if msg.Body != couponBody {
		t.Errorf("msg.Body foi mutada (vaza o link do dono A pro dono B).\n got: %q\nwant: %q", msg.Body, couponBody)
	}
}

// A conversão TEM que ser pedida com o userID do DONO da automação: é ele que
// resolve a credencial (shopee_service.go:671) e, portanto, o app_id que assina
// a chamada — que é a própria atribuição da comissão. Sem esta asserção, um
// mutante que passasse msg.InstanceName ou o dono da FONTE passaria batido.
func TestProcessAutomation_ShopeeCouponLink_ConvertsWithOwnerUserID(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{expanded: couponExpandedNP, affLink: "https://s.shopee.com.br/AFF"}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}

	if len(sh.gotUserIDs) == 0 {
		t.Fatal("GenerateAffiliateLink nunca foi chamado — o cupom não passou por conversão")
	}
	for _, got := range sh.gotUserIDs {
		if got != couponOwnerID {
			t.Errorf("conversão pedida com userID %q, esperado o DONO %q — comissão iria para a conta errada",
				got, couponOwnerID)
		}
	}
}

// Mensagem de cupom com mais de um link: TODOS têm de ser convertidos. Um
// mutante que converta só o primeiro (ou dê break no loop) passaria sem isto.
func TestProcessAutomation_ShopeeCouponLink_ConvertsEveryLink(t *testing.T) {
	const (
		l1   = "https://s.shopee.com.br/CUPOM1"
		l2   = "https://s.shopee.com.br/CUPOM2"
		aff1 = "https://s.shopee.com.br/AFF1"
		aff2 = "https://s.shopee.com.br/AFF2"
	)
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded:  couponExpandedNP,
		affByOrig: map[string]string{l1: aff1, l2: aff2},
	}
	svc := newCouponSvc(sh, &sent)

	msg := newCouponMsg()
	msg.Body = "🎟 R$30 OFF: " + l1 + "\n🎟 R$50 OFF acima 500: " + l2

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), msg, []string{l1, l2}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio, obtido %d", len(sent))
	}
	for _, want := range []string{aff1, aff2} {
		if !strings.Contains(sent[0].Text, want) {
			t.Errorf("link %q não foi convertido no corpo enviado.\n got: %q", want, sent[0].Text)
		}
	}
	for _, leak := range []string{l1, l2} {
		if strings.Contains(sent[0].Text, leak) {
			t.Errorf("link da FONTE %q sobreviveu no corpo — comissão vaza.\n got: %q", leak, sent[0].Text)
		}
	}
}

// Colisão de prefixo (achado da auditoria adversarial 15/07): quando a mesma
// campanha aparece nua E com querystring, ExtractLinks devolve as duas como
// entradas distintas. Substituindo a curta primeiro, o ReplaceAll casaria DENTRO
// da longa e soldaria o mmp_pid da fonte no shortlink do dono. O loop ordena por
// tamanho decrescente justamente para isso.
func TestProcessAutomation_ShopeeCouponLink_PrefixCollisionDoesNotWeldSourceTracking(t *testing.T) {
	const (
		bare   = "https://shopee.com.br/m/cupom-julho"
		full   = "https://shopee.com.br/m/cupom-julho?smtt=0.0.9&mmp_pid=an_18330220029"
		affB   = "https://s.shopee.com.br/AFFbare"
		affF   = "https://s.shopee.com.br/AFFfull"
		mmpSrc = "mmp_pid=an_18330220029"
	)
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded:  couponExpandedNP,
		affByOrig: map[string]string{bare: affB, full: affF},
	}
	svc := newCouponSvc(sh, &sent)

	msg := newCouponMsg()
	msg.Body = "Cupom aqui " + bare + " e o oficial " + full

	// Ordem do corpo: a NUA primeiro — o pior caso.
	if err := svc.processAutomation(context.Background(), newCouponAutomation(), msg, []string{bare, full}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio, obtido %d", len(sent))
	}
	if strings.Contains(sent[0].Text, mmpSrc) {
		t.Errorf("o tracking da FONTE (%s) sobreviveu soldado no corpo — comissão vaza.\n got: %q",
			mmpSrc, sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, affF) {
		t.Errorf("o link longo não foi convertido (ReplaceAll virou no-op).\n got: %q", sent[0].Text)
	}
}

// Falha transitória de conversão (10030/timeout/breaker): repassar com o link da
// fonte entregaria a comissão pra ela, então o repasse é retido — igual ao
// espelho (monitoring_service.go:431-439).
func TestProcessAutomation_ShopeeCouponLink_WithheldOnConversionFailure(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded: couponExpandedNP,
		affErr:   fmt.Errorf("shopee graphql error: error code: 10030 (rate limit)"),
	}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 0 {
		t.Fatalf("esperado 0 envios (repasse retido), obtido %d — cupom saiu com o link da fonte e a comissão vazou", len(sent))
	}
}

// Falha ao LER a credencial (timeout de Postgres, ctx cancelado no shutdown) NÃO
// é "usuário sem credencial": é transitória e tem de reter. Antes de
// ErrShopeeNoCredentials as duas colapsavam na string "credentials not
// configured", e um soluço de banco fazia o cupom sair cru para todo mundo.
func TestProcessAutomation_ShopeeCouponLink_WithheldOnCredentialReadFailure(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded: couponExpandedNP,
		affErr:   fmt.Errorf("failed to load shopee credentials: %w", errors.New("context deadline exceeded")),
	}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 0 {
		t.Fatalf("esperado 0 envios (falha de leitura = transitória, retém), obtido %d — soluço de banco vazou a comissão", len(sent))
	}
}

// Um erro QUALQUER cujo texto contenha "credentials not configured" mas que não
// seja o sentinela NÃO pode abrir o portão do relay cru. Este teste é o que
// separa errors.Is de strings.Contains: com substring, qualquer erro que
// mencione a frase (um wrap de camada inferior, uma futura mensagem de DB)
// passaria a vazar comissão silenciosamente.
func TestProcessAutomation_ShopeeCouponLink_SubstringLookalikeDoesNotOpenRelay(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded: couponExpandedNP,
		// mesma frase, erro NÃO tipado — o que o shopee_service devolvia num
		// timeout de banco antes do sentinela existir.
		affErr: errors.New("shopee credentials not configured"),
	}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 0 {
		t.Fatalf("esperado 0 envios (só o sentinela libera relay cru), obtido %d — match por substring vazou a comissão", len(sent))
	}
}

// 10035 (conta sem acesso à Open API): converter é impossível enquanto o
// usuário não reativar a conta na Shopee. Reter para sempre mataria o repasse
// de cupom, então o original passa — sem comissão, mas o grupo recebe o aviso.
func TestProcessAutomation_ShopeeCouponLink_RelaysAsIsWhenNoAPIAccess(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{expanded: couponExpandedNP, affErr: shopee.ErrShopeeNoAPIAccess}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio (relay do original), obtido %d", len(sent))
	}
	if !strings.Contains(sent[0].Text, couponSourceLink) {
		t.Errorf("sem Open API o link original deveria passar como veio.\n got: %q", sent[0].Text)
	}
}

// Shopee nunca configurada: mesma regra do 10035 — impossível converter até o
// usuário agir, e reter tiraria o cupom de quem hoje recebe (havia 1 conta assim
// com offer_generate ativo em 15/07). Casado por errors.Is, não por substring.
func TestProcessAutomation_ShopeeCouponLink_RelaysAsIsWhenNoCredentials(t *testing.T) {
	var sent []whatsapp.SendTextInput
	sh := &couponFakeShopee{
		expanded: couponExpandedNP,
		affErr:   fmt.Errorf("converting coupon: %w", shopee.ErrShopeeNoCredentials),
	}
	svc := newCouponSvc(sh, &sent)

	if err := svc.processAutomation(context.Background(), newCouponAutomation(), newCouponMsg(), []string{couponSourceLink}); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("esperado 1 envio (relay do original), obtido %d — conta sem credencial parou de receber cupom", len(sent))
	}
	if !strings.Contains(sent[0].Text, couponSourceLink) {
		t.Errorf("sem credencial o link original deveria passar como veio.\n got: %q", sent[0].Text)
	}
}
