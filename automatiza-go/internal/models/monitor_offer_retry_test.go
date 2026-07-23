package models

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestReasonCodesBatemComOCheckDaMigracao é o teste que impede o modo de falha
// mais chato deste ledger: alguém acrescenta um reason code no Go, o INSERT
// viola o CHECK em produção e o erro que chega é um 23514 genérico, no meio de
// um caminho que já está tratando outro erro.
//
// Compara os dois conjuntos nas DUAS direções: código sem CHECK quebra o
// INSERT; CHECK sem código é enum morto no banco.
func TestReasonCodesBatemComOCheckDaMigracao(t *testing.T) {
	const migration = "../../scripts/db/migrations/079_monitor_offer_retry.up.sql"
	raw, err := os.ReadFile(migration)
	if err != nil {
		t.Fatalf("não consegui ler a migração: %v", err)
	}

	// Extrai o primeiro bloco `reason_code IN ( ... )`.
	block := regexp.MustCompile(`(?s)reason_code IN \((.*?)\)`).FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatal("não achei o CHECK de reason_code na migração 079")
	}
	inSQL := make(map[string]struct{})
	for _, m := range regexp.MustCompile(`'([a-z0-9_]+)'`).FindAllStringSubmatch(block[1], -1) {
		inSQL[m[1]] = struct{}{}
	}

	inGo := make(map[string]struct{})
	for _, r := range AllReasonCodes() {
		inGo[r] = struct{}{}
	}

	var faltamNoSQL, faltamNoGo []string
	for r := range inGo {
		if _, ok := inSQL[r]; !ok {
			faltamNoSQL = append(faltamNoSQL, r)
		}
	}
	for r := range inSQL {
		if _, ok := inGo[r]; !ok {
			faltamNoGo = append(faltamNoGo, r)
		}
	}
	if len(faltamNoSQL) > 0 {
		t.Errorf("reason codes no Go que o CHECK da 079 recusaria: %s", strings.Join(faltamNoSQL, ", "))
	}
	if len(faltamNoGo) > 0 {
		t.Errorf("reason codes no CHECK sem constante no Go (enum morto): %s", strings.Join(faltamNoGo, ", "))
	}
}

// TestEnumsDoLedgerBatemComOsChecks faz o mesmo para url_class, error_class,
// store, outcome e os status — colunas cujo CHECK também é fechado.
func TestEnumsDoLedgerBatemComOsChecks(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/db/migrations/079_monitor_offer_retry.up.sql")
	if err != nil {
		t.Fatalf("migração: %v", err)
	}
	sql := string(raw)

	casos := []struct {
		coluna string
		goVals []string
	}{
		{"url_class", []string{URLClassProduct, URLClassCampaignOrCoupon, URLClassNonCommerce, URLClassUnsupportedCommerce, URLClassUnknown}},
		{"error_class", []string{ErrorClassNone, ErrorClassTransient, ErrorClassPermanentConfig, ErrorClassPermanentConte, ErrorClassSecurityBlock}},
		{"store", []string{MonitorStoreShopee, MonitorStoreMercadoLivr, MonitorStoreUnsupported}},
		{"outcome", []string{MonitorOutcomeNone, MonitorOutcomeConverted, MonitorOutcomeOwnerPassthru, MonitorOutcomeRemoved, MonitorOutcomeIncluded}},
	}
	for _, c := range casos {
		t.Run(c.coluna, func(t *testing.T) {
			block := regexp.MustCompile(c.coluna + `\s+IN \(([^)]*)\)`).FindStringSubmatch(sql)
			if block == nil {
				t.Fatalf("não achei o CHECK de %s", c.coluna)
			}
			for _, v := range c.goVals {
				if !strings.Contains(block[1], "'"+v+"'") {
					t.Errorf("valor %q do Go não está no CHECK de %s", v, c.coluna)
				}
			}
		})
	}
}

// TestNextAttemptDelayEhDeterministicoEComJitter prova as duas propriedades que
// importam: o mesmo item recalculado após um crash cai no MESMO instante (senão
// um reprocessamento reembaralharia a fila), e itens diferentes se espalham
// (senão 100 itens vencidos disparariam juntos).
func TestNextAttemptDelayEhDeterministicoEComJitter(t *testing.T) {
	const item = "11111111-1111-1111-1111-111111111111"

	for attempt := 0; attempt < MaxItemAttempts; attempt++ {
		a := NextAttemptDelay(item, attempt)
		b := NextAttemptDelay(item, attempt)
		if a != b {
			t.Fatalf("tentativa %d não é determinística: %v != %v", attempt, a, b)
		}
		base := BackoffDelays[attempt]
		if a < base {
			t.Errorf("tentativa %d: %v abaixo da base %v", attempt, a, base)
		}
		if max := base + base/5; a > max {
			t.Errorf("tentativa %d: %v acima de base+20%% (%v)", attempt, a, max)
		}
	}

	// Itens diferentes na mesma tentativa não colidem em massa.
	vistos := make(map[time.Duration]int)
	for i := 0; i < 50; i++ {
		vistos[NextAttemptDelay(string(rune('a'+i%26))+"-item", 0)]++
	}
	if len(vistos) < 5 {
		t.Errorf("jitter não espalha: só %d valores distintos em 50 itens", len(vistos))
	}
}

func TestNextAttemptDelayClampaTentativaForaDaFaixa(t *testing.T) {
	// Índice negativo ou acima do teto não pode entrar em pânico no meio de um
	// retry: cai no primeiro/último degrau.
	if d := NextAttemptDelay("x", -3); d < BackoffDelays[0] {
		t.Errorf("attempt negativo devolveu %v", d)
	}
	if d := NextAttemptDelay("x", 99); d < BackoffDelays[MaxItemAttempts-1] {
		t.Errorf("attempt acima do teto devolveu %v", d)
	}
}

func TestStatusTerminais(t *testing.T) {
	for _, s := range []string{MonitorItemSucceeded, MonitorItemExhausted, MonitorItemBlocked, MonitorItemSuperseded, MonitorItemAmbiguous} {
		if !IsTerminalItemStatus(s) {
			t.Errorf("%q deveria ser terminal", s)
		}
	}
	for _, s := range []string{MonitorItemPending, MonitorItemProcessing} {
		if IsTerminalItemStatus(s) {
			t.Errorf("%q não é terminal", s)
		}
	}
}

// TestReplayavelSoPermanentConfig fixa a regra do §8.4: reprocessar
// permanent_content ou security_block não muda o desfecho, e no caso de
// security_block reabriria a superfície que foi fechada de propósito.
func TestReplayavelSoPermanentConfig(t *testing.T) {
	if !IsReplayableBlock(ErrorClassPermanentConfig) {
		t.Error("permanent_config deve ser replayável (o cliente corrige a credencial)")
	}
	for _, c := range []string{ErrorClassPermanentConte, ErrorClassSecurityBlock, ErrorClassNone, ErrorClassTransient} {
		if IsReplayableBlock(c) {
			t.Errorf("%q não pode ser replayável", c)
		}
	}
}
