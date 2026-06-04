package offer_generate //nolint:revive // nome do pacote já existe com underscore; renomear está fora de escopo

import (
	"context"
	"fmt"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/shopee"

	"go.uber.org/zap"
)

// --- fakes mínimos (embedam a interface; só sobrescrevem o que é chamado) ---

type couponFakeShopee struct {
	shopee.Service
	expanded string
}

func (f couponFakeShopee) ExpandURL(_ context.Context, _ string) (string, error) {
	return f.expanded, nil
}

// ExtractProductIDs falha exatamente como o serviço real falha numa página de
// campanha/cupom (shopee.com.br/m/...): sem shopID/itemID na URL. Antes do fix
// esse erro descartava a mensagem; depois do fix nem chega aqui (relay antes).
func (f couponFakeShopee) ExtractProductIDs(_ string) (int64, int64, error) {
	return 0, 0, fmt.Errorf("could not extract product IDs from URL")
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

// Regressão (incidente isaa_deoliveira@hotmail.com, 04/06): uma mensagem de
// cupom postada no grupo monitor do offer_generate tinha um link Shopee curto
// que expandia pra uma página de campanha (shopee.com.br/m/super-ofertas).
// Como a mensagem TINHA link, não caía no relay; e como o link não é produto,
// ExtractProductIDs falhava e a mensagem era DESCARTADA (return nil) — nunca
// chegava aos grupos destino. O fix repassa (relay) links de cupom/campanha.
func TestProcessAutomation_ShopeeCouponLink_RelaysInsteadOfDropping(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	var sentText whatsapp.SendTextInput
	var sentCount int
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		sentText = in
		sentCount++
		return &whatsapp.MessageResult{ID: "msg-1"}, nil
	}

	svc := &offerGenerateService{
		logger:       zap.NewNop(),
		waProvider:   fake,
		instanceRepo: couponFakeInstanceRepo{},
		settingsRepo: couponFakeSettingsRepo{},
		shopeeService: couponFakeShopee{
			expanded: "https://shopee.com.br/m/super-ofertas?utm_content=cupom----",
		},
		// cache, dispatchLogRepo, affiliateRepo, ctaRepo, couponRepo,
		// couponHandler, groupRepo, mlService = nil → todos nil-safe nesse caminho.
	}

	dtID := "dt-1"
	zero := 0
	monitorGroup := "120363410863986891@g.us"
	destGroup := "120363408267381739@g.us"
	auto := &models.Automation{
		ID:             "auto-coupon",
		UserID:         "user-1",
		Name:           "Gerar Oferta",
		Status:         models.AutomationStatusActive,
		DispatchTextID: &dtID,
		SendGroups:     []string{destGroup},
		DelaySeconds:   &zero, // mantém o delay aleatório mínimo, sem custo extra
	}

	msg := &models.IncomingMessage{
		InstanceName: "session_test",
		MessageID:    "msg-coupon-1",
		RemoteJID:    monitorGroup,
		Body:         "🚨 SAIUUUU CUPONS SHOPEE 🚨\n🎟 R$30 OFF acima R$299\n🔗 https://s.shopee.com.br/3VhcRPRgiQ",
	}

	// O link curto do cupom é o único link da mensagem.
	links := []string{"https://s.shopee.com.br/3VhcRPRgiQ"}

	if err := svc.processAutomation(context.Background(), auto, msg, links); err != nil {
		t.Fatalf("processAutomation retornou erro: %v", err)
	}

	if sentCount != 1 {
		t.Fatalf("esperado 1 envio de relay, obtido %d (cupom foi descartado em vez de repassado)", sentCount)
	}
	if sentText.ChatID != destGroup {
		t.Errorf("relay enviado para %q, esperado grupo destino %q", sentText.ChatID, destGroup)
	}
	if sentText.Text != msg.Body {
		t.Errorf("relay deveria repassar o corpo original como veio.\n got: %q\nwant: %q", sentText.Text, msg.Body)
	}
}
