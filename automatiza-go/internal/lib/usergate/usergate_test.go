package usergate

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseFechado(t *testing.T) {
	for _, raw := range []string{"", "   ", "off", "OFF", "Off"} {
		g, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) erro: %v", raw, err)
		}
		if !g.GloballyOff() {
			t.Errorf("Parse(%q) deveria estar fechado", raw)
		}
		if g.Allows(uuid.New()) {
			t.Errorf("Parse(%q) liberou usuário", raw)
		}
	}
}

func TestParseAll(t *testing.T) {
	for _, raw := range []string{"all", "ALL", " All "} {
		g, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) erro: %v", raw, err)
		}
		if g.GloballyOff() {
			t.Errorf("Parse(%q) deveria estar aberto", raw)
		}
		if !g.Allows(uuid.New()) {
			t.Errorf("Parse(%q) negou usuário", raw)
		}
	}
}

func TestParseAllowlist(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	fora := uuid.New()
	// Espaços, duplicata e vírgula sobrando são tolerados.
	raw := " " + a.String() + " , " + b.String() + "," + a.String() + ", "
	g, err := Parse(raw)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if g.GloballyOff() {
		t.Fatal("allowlist não é globalmente off")
	}
	if !g.Allows(a) || !g.Allows(b) {
		t.Error("usuário da lista foi negado")
	}
	if g.Allows(fora) {
		t.Error("usuário fora da lista foi liberado")
	}
	if got := g.String(); got != "allowlist(2)" {
		t.Errorf("String() = %q, queria allowlist(2) — e sem listar UUIDs", got)
	}
}

// TestParseTokenInvalidoNuncaViraAll é o teste que dá razão ao pacote: um erro
// de digitação na env não pode abrir a feature para todo mundo.
func TestParseTokenInvalidoNuncaViraAll(t *testing.T) {
	ruins := []string{
		"todos",
		"true",
		"1",
		"yes",
		uuid.New().String() + ",nao-e-uuid",
		uuid.New().String() + ",all",
		"off," + uuid.New().String(),
	}
	for _, raw := range ruins {
		g, err := Parse(raw)
		if err == nil {
			t.Errorf("Parse(%q) deveria falhar, veio %v", raw, g)
		}
		if g.Allows(uuid.New()) {
			t.Errorf("Parse(%q) devolveu gate que libera", raw)
		}
	}
}

func TestZeroValueNega(t *testing.T) {
	var g Gate
	if g.Allows(uuid.New()) {
		t.Error("Gate zero deve negar")
	}
	if !g.GloballyOff() {
		t.Error("Gate zero deve ser globalmente off")
	}
}

func TestAllowsStringIDInvalidoNega(t *testing.T) {
	g, _ := Parse(uuid.New().String())
	if g.AllowsString("nao-e-uuid") {
		t.Error("ID não parseável deve negar")
	}
	if g.AllowsString("") {
		t.Error("ID vazio deve negar")
	}
}

func TestMustParsePanicaEmGateRuim(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse deveria entrar em pânico com gate malformado")
		}
	}()
	MustParse("TESTE", "todos")
}
