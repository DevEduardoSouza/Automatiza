package main

import (
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/models"
)

func jobValido() models.MonitorOfferJob {
	ct := "cifrado"
	return models.MonitorOfferJob{
		PayloadCiphertext: &ct,
		PayloadExpiresAt:  time.Now().UTC().Add(time.Hour),
	}
}

func TestReplayRecusaOQuePodeDuplicar(t *testing.T) {
	for _, st := range []string{models.MonitorItemSucceeded, models.MonitorItemSuperseded} {
		err := checkReplayable(models.MonitorOfferItem{Status: st}, jobValido(), true)
		if err == nil {
			t.Errorf("status %q deveria ser recusado", st)
		}
	}
}

func TestReplayRecusaTentativaEmAndamento(t *testing.T) {
	for _, st := range []string{models.MonitorItemPending, models.MonitorItemProcessing} {
		if err := checkReplayable(models.MonitorOfferItem{Status: st}, jobValido(), true); err == nil {
			t.Errorf("status %q deveria ser recusado", st)
		}
	}
}

// TestAmbiguoExigeConfirmacaoExplicita: a entrega pode ter chegado ao grupo.
// Um replay silencioso aqui duplicaria a mensagem para o cliente final.
func TestAmbiguoExigeConfirmacaoExplicita(t *testing.T) {
	item := models.MonitorOfferItem{Status: models.MonitorItemAmbiguous}

	err := checkReplayable(item, jobValido(), false)
	if err == nil {
		t.Fatal("ambíguo sem --confirm-ambiguous deveria ser recusado")
	}
	if !strings.Contains(err.Error(), "confirm-ambiguous") {
		t.Errorf("a mensagem tem de dizer como confirmar: %v", err)
	}
	if err := checkReplayable(item, jobValido(), true); err != nil {
		t.Errorf("com confirmação deveria passar: %v", err)
	}
}

// TestBloqueiosNaoReplayaveis: reprocessar não muda o desfecho, e no caso de
// security_block reabriria a barreira fechada de propósito.
func TestBloqueiosNaoReplayaveis(t *testing.T) {
	casos := map[string]bool{
		models.ErrorClassPermanentConfig: true,  // cliente corrigiu credencial
		models.ErrorClassPermanentConte:  false, // página não existe
		models.ErrorClassSecurityBlock:   false, // host forjado
	}
	for class, deveriaPassar := range casos {
		err := checkReplayable(
			models.MonitorOfferItem{Status: models.MonitorItemBlocked, ErrorClass: class},
			jobValido(), true)
		if deveriaPassar && err != nil {
			t.Errorf("blocked/%s deveria ser replayável: %v", class, err)
		}
		if !deveriaPassar && err == nil {
			t.Errorf("blocked/%s NÃO pode ser replayável", class)
		}
	}
}

func TestExhaustedSoTransientEhReplayavel(t *testing.T) {
	ok := models.MonitorOfferItem{Status: models.MonitorItemExhausted, ErrorClass: models.ErrorClassTransient}
	if err := checkReplayable(ok, jobValido(), false); err != nil {
		t.Errorf("exhausted/transient deveria passar: %v", err)
	}
	nao := models.MonitorOfferItem{Status: models.MonitorItemExhausted, ErrorClass: models.ErrorClassPermanentConte}
	if err := checkReplayable(nao, jobValido(), false); err == nil {
		t.Error("exhausted/permanent_content não pode ser replayável")
	}
}

// TestPayloadExpiradoOuLimpoRecusa: sem payload não há como reclassificar, e
// enviar às cegas mandaria conteúdo errado ao grupo.
func TestPayloadExpiradoOuLimpoRecusa(t *testing.T) {
	item := models.MonitorOfferItem{Status: models.MonitorItemExhausted, ErrorClass: models.ErrorClassTransient}

	semPayload := models.MonitorOfferJob{PayloadExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := checkReplayable(item, semPayload, false); err == nil {
		t.Error("payload limpo deveria recusar")
	}

	ct := "cifrado"
	expirado := models.MonitorOfferJob{PayloadCiphertext: &ct, PayloadExpiresAt: time.Now().UTC().Add(-time.Minute)}
	if err := checkReplayable(item, expirado, false); err == nil {
		t.Error("payload expirado deveria recusar")
	}
}
