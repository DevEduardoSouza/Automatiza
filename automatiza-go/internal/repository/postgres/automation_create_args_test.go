package postgres

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// Guard do INSERT de automações. Em 23/07/2026 a coluna monitor_interval_minutes
// entrou na query (placeholders até $51) sem o valor correspondente na lista de
// args: o pgx recusou com "expected 51 arguments, got 50" e TODA criação de
// automação quebrou em produção — o build passava, nada acusava até um usuário
// clicar em "criar".
//
// Sem banco e sem Docker de propósito: roda em milissegundos, no CI, antes de a
// imagem subir.
var placeholderRe = regexp.MustCompile(`\$(\d+)`)

func TestCreateAutomationArgsMatchQueryPlaceholders(t *testing.T) {
	r := &automationRepository{logger: zap.NewNop()}

	// Automação "vazia" é o pior caso: é assim que o handler entrega tudo que o
	// request não trouxe, e é onde ponteiro nil vira NULL em coluna NOT NULL.
	args := r.createAutomationArgs("11111111-1111-1111-1111-111111111111", &models.Automation{}, time.Now().UTC())

	want := 0
	for _, m := range placeholderRe.FindAllStringSubmatch(createAutomationQuery, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("placeholder ilegível em createAutomationQuery: %q", m[0])
		}
		if n > want {
			want = n
		}
	}

	if want == 0 {
		t.Fatal("createAutomationQuery sem placeholders — query errada no guard")
	}
	if len(args) != want {
		t.Fatalf("INSERT de automations pede %d valores ($1..$%d), createAutomationArgs monta %d.\n"+
			"Coluna nova na query = valor novo no fim de createAutomationArgs.", want, want, len(args))
	}
}

// monitor_interval_minutes é NOT NULL DEFAULT 0 (migração 078): request sem o
// campo tem de escrever 0, nunca NULL.
func TestCreateAutomationArgsMonitorIntervalNeverNil(t *testing.T) {
	r := &automationRepository{logger: zap.NewNop()}
	args := r.createAutomationArgs("11111111-1111-1111-1111-111111111111", &models.Automation{}, time.Now().UTC())

	last := args[len(args)-1]
	if got, ok := last.(int); !ok || got != 0 {
		t.Fatalf("último arg (monitor_interval_minutes) = %#v; queria int 0", last)
	}
}
