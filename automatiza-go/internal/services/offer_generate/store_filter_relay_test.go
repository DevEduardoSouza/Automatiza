package offer_generate //nolint:revive // nome do pacote já existe com underscore; renomear está fora de escopo

import (
	"context"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// Regressão (silviadomkt@gmail.com, 14/07): mensagem do grupo-fonte com link
// de loja NÃO habilitada (meli.la com stores=["shopee"]) ou sem conversão de
// afiliado (Amazon/Magalu/Shein/AliExpress) caía no relay as-is — o cliente
// "Shopee-only" repassava produto de ML e Amazon com a comissão do canal de
// ORIGEM. Links de loja filtrados agora DROPAM a mensagem; conteúdo sem link
// de loja (Instagram, site, convite) continua repassando (lição do #216).
func newStoreFilterTestService(fake *whatsapp.FakeAdapter) *offerGenerateService {
	return &offerGenerateService{
		logger:       zap.NewNop(),
		waProvider:   fake,
		instanceRepo: couponFakeInstanceRepo{},
		settingsRepo: couponFakeSettingsRepo{},
		// affiliateRepo nil → sem fallback GetActiveStores; stores da automação valem.
	}
}

func storeFilterAutomation() *models.Automation {
	zero := 0
	return &models.Automation{
		ID:           "auto-store-filter",
		UserID:       "user-1",
		Name:         "Gerar Oferta",
		Status:       models.AutomationStatusActive,
		Stores:       []string{"shopee"},
		SendGroups:   []string{"120363408267381739@g.us"},
		DelaySeconds: &zero,
	}
}

func TestProcessAutomation_MLLinkWithShopeeOnlyStores_Drops(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var sentCount int
	fake.SendTextFunc = func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		sentCount++
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	svc := newStoreFilterTestService(fake)

	msg := &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-ml-1",
		RemoteJID:    "120363410863986891@g.us",
		Body:         "🔥 OFERTA MERCADO LIVRE 🔥\nhttps://meli.la/32kJrn7",
	}
	links := []string{"https://meli.la/32kJrn7"}

	if err := svc.processAutomation(context.Background(), storeFilterAutomation(), msg, links); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if sentCount != 0 {
		t.Fatalf("mensagem só-ML com stores=[shopee] deveria ser dropada, mas houve %d envio(s)", sentCount)
	}
}

func TestProcessAutomation_AmazonLink_Drops(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var sentCount int
	fake.SendTextFunc = func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		sentCount++
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	svc := newStoreFilterTestService(fake)

	msg := &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-amz-1",
		RemoteJID:    "120363410863986891@g.us",
		Body:         "Echo Dot em promoção! https://amzn.to/3xYzAbC corre!",
	}
	links := []string{"https://amzn.to/3xYzAbC"}

	if err := svc.processAutomation(context.Background(), storeFilterAutomation(), msg, links); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if sentCount != 0 {
		t.Fatalf("mensagem só-Amazon deveria ser dropada, mas houve %d envio(s)", sentCount)
	}
}

func TestProcessAutomation_NonStoreLink_StillRelays(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var sentText whatsapp.SendTextInput
	var sentCount int
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		sentText = in
		sentCount++
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}
	svc := newStoreFilterTestService(fake)

	msg := &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-insta-1",
		RemoteJID:    "120363410863986891@g.us",
		Body:         "Segue a gente no insta! https://instagram.com/achadinhos",
	}
	links := []string{"https://instagram.com/achadinhos"}

	auto := storeFilterAutomation()
	if err := svc.processAutomation(context.Background(), auto, msg, links); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}
	if sentCount != 1 {
		t.Fatalf("mensagem sem link de loja deveria repassar (1 envio), obtido %d", sentCount)
	}
	if sentText.Text != msg.Body {
		t.Errorf("relay deveria repassar o corpo original.\n got: %q\nwant: %q", sentText.Text, msg.Body)
	}
}
