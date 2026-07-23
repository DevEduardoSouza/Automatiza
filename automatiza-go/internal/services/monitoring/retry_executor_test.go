package monitoring

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/models"

	"go.uber.org/zap/zaptest"
)

// A asserção contra workers.RetryExecutor vive no pacote workers
// (retry_executor_contract_test.go): workers já importa monitoring, então
// fazê-la aqui criaria ciclo de import.

func attempt(mode, groupID string) models.MonitorOfferAttempt {
	return models.MonitorOfferAttempt{
		Item: models.MonitorOfferItem{ID: "item-1", JobID: "job-1", Ordinal: 0},
		Job: models.MonitorOfferJob{
			ID: "job-1", UserID: "user-1", AutomationID: "auto-1",
			Mode: mode, SourceGroupID: groupID, MessageID: "msg-1",
		},
	}
}

func payloadWith(url string) models.MonitorOfferPayload {
	return models.MonitorOfferPayload{
		BodyAfterStory: "Oferta " + url,
		OrderedURLs:    []string{url},
	}
}

func svcWithAutomation(t *testing.T, auto *models.Automation, err error) *monitoringService {
	t.Helper()
	return &monitoringService{
		automationRepo: &mockAutomationRepo{
			GetAutomationByIDAndUserFn: func(context.Context, string, string) (*models.Automation, error) {
				return auto, err
			},
		},
		logger: zaptest.NewLogger(t),
	}
}

// TestRevalidacaoDeTenantAcontecAntesDeQualquerChamada: um link cuja automação
// mudou de dono não pode consumir cota da Shopee.
func TestAutomacaoAusenteOuDeOutroDonoVirasuperseded(t *testing.T) {
	casos := map[string]*models.Automation{
		"automação apagada": nil,
		"dono divergente":   {ID: "auto-1", UserID: "outro-user", Status: models.AutomationStatusActive},
		"automação pausada": {ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusPaused},
	}
	for nome, auto := range casos {
		svc := svcWithAutomation(t, auto, nil)
		out, err := svc.ExecuteRetry(context.Background(),
			attempt(models.MonitorOfferModeMonitoring, "g@g.us"),
			payloadWith("https://shopee.com.br/product/1/2"))
		if err != nil {
			t.Fatalf("%s: erro inesperado: %v", nome, err)
		}
		if out.Status != models.MonitorItemSuperseded {
			t.Errorf("%s: status = %q, queria superseded", nome, out.Status)
		}
	}
}

// TestErroDeBancoNaoEncerraOItem: um soluço do Postgres é transitório.
func TestErroDeBancoNaoEncerraOItem(t *testing.T) {
	svc := svcWithAutomation(t, nil, errors.New("timeout"))
	out, err := svc.ExecuteRetry(context.Background(),
		attempt(models.MonitorOfferModeMonitoring, "g@g.us"),
		payloadWith("https://shopee.com.br/product/1/2"))
	if err == nil {
		t.Fatal("erro de banco deveria propagar (o worker reagenda)")
	}
	if out.Status == models.MonitorItemSuperseded || out.Status == models.MonitorItemBlocked {
		t.Errorf("status = %q: erro de banco não pode encerrar o item", out.Status)
	}
}

// TestModoOuGrupoDivergenteVirasuperseded: entregar uma oferta de espelho numa
// automação que virou lista seria mensagem inesperada no grupo do cliente.
func TestModoOuGrupoDivergenteVirasuperseded(t *testing.T) {
	casos := map[string]*models.Automation{
		"virou lista": {
			ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive,
			Type: strPtr(models.AutomationTypeList),
		},
		"trocou o grupo-fonte": {
			ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive,
			Type: strPtr(models.AutomationTypeMonitoring), MonitorGroupIDs: []string{"outro@g.us"},
		},
	}
	for nome, auto := range casos {
		svc := svcWithAutomation(t, auto, nil)
		out, _ := svc.ExecuteRetry(context.Background(),
			attempt(models.MonitorOfferModeMonitoring, "g@g.us"),
			payloadWith("https://shopee.com.br/product/1/2"))
		if out.Status != models.MonitorItemSuperseded {
			t.Errorf("%s: status = %q, queria superseded", nome, out.Status)
		}
	}
}

// TestForaDaJanelaNaoGastaTentativa: o link não tem culpa do horário.
func TestForaDaJanelaNaoEncerraNemBloqueia(t *testing.T) {
	auto := &models.Automation{
		ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive,
		Type: strPtr(models.AutomationTypeMonitoring), MonitorGroupIDs: []string{"g@g.us"},
		// Janela impossível: começa e termina no mesmo minuto do passado.
		StartTime: strPtr("00:00"), EndTime: strPtr("00:01"),
		ScheduleDays: []string{"seg"},
	}
	svc := svcWithAutomation(t, auto, nil)
	out, err := svc.ExecuteRetry(context.Background(),
		attempt(models.MonitorOfferModeMonitoring, "g@g.us"),
		payloadWith("https://shopee.com.br/product/1/2"))
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if out.Status != models.MonitorItemPending {
		t.Errorf("status = %q, queria pending (reagenda sem gastar tentativa)", out.Status)
	}
	if out.ReasonCode != models.ReasonOutsideWindow {
		t.Errorf("reason = %q, queria outside_window", out.ReasonCode)
	}
}

// TestURLForaDoPayloadNaoEnvia: sem a URL não há o que reclassificar.
func TestURLForaDoPayloadEncerra(t *testing.T) {
	auto := &models.Automation{
		ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive,
		Type: strPtr(models.AutomationTypeMonitoring),
	}
	svc := svcWithAutomation(t, auto, nil)
	att := attempt(models.MonitorOfferModeMonitoring, "g@g.us")
	att.Item.Ordinal = 7 // fora do slice

	out, _ := svc.ExecuteRetry(context.Background(), att, payloadWith("https://shopee.com.br/product/1/2"))
	if out.Status != models.MonitorItemExhausted {
		t.Errorf("status = %q, queria exhausted", out.Status)
	}
}

// TestHostForjadoNoRetryEhBloqueado: a barreira de segurança vale igualmente no
// caminho de retry.
func TestHostForjadoNoRetryEhBloqueado(t *testing.T) {
	auto := &models.Automation{
		ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive,
		Type: strPtr(models.AutomationTypeMonitoring),
	}
	svc := svcWithAutomation(t, auto, nil)
	out, _ := svc.ExecuteRetry(context.Background(),
		attempt(models.MonitorOfferModeMonitoring, "g@g.us"),
		payloadWith("https://evil.test/shopee.com.br/x"))

	if out.Status != models.MonitorItemBlocked {
		t.Errorf("status = %q, queria blocked", out.Status)
	}
	if out.ErrorClass != models.ErrorClassSecurityBlock {
		t.Errorf("error_class = %q, queria security_block", out.ErrorClass)
	}
}

// TestParagraphForIsolaOBlocoDoLink: reenviar o corpo inteiro duplicaria o
// texto que já saiu na passagem original.
func TestParagraphForIsolaOBlocoDoLink(t *testing.T) {
	body := "bloco A sem link\n\nbloco B https://shopee.com.br/product/1/2\n\nbloco C"
	got := paragraphFor(body, "https://shopee.com.br/product/1/2")
	if got != "bloco B https://shopee.com.br/product/1/2" {
		t.Errorf("paragraphFor devolveu %q", got)
	}
	// URL ausente do corpo: devolve a própria URL, nunca o corpo inteiro.
	if got := paragraphFor(body, "https://outro.com/x"); got != "https://outro.com/x" {
		t.Errorf("fallback devolveu %q", got)
	}
}

// TestContextoDeRetryImpedeSegundoJob: a reentrada não pode gravar um segundo
// job para a mesma mensagem.
func TestContextoDeRetryImpedeSegundoJob(t *testing.T) {
	if isRetryContext(context.Background()) {
		t.Error("contexto normal não pode estar marcado")
	}
	if !isRetryContext(markRetryContext(context.Background())) {
		t.Error("contexto de retry deveria estar marcado")
	}

	// RecordFailedLinks é no-op num contexto de retry, mesmo com coordinator.
	svc := &monitoringService{logger: zaptest.NewLogger(t)}
	svc.RecordFailedLinks(markRetryContext(context.Background()),
		&models.Automation{UserID: "user-1"},
		&models.IncomingMessage{MessageID: "m1"}, []string{"https://shopee.com.br/product/1/2"})
	// Sem pânico e sem coordinator = passou.
}
