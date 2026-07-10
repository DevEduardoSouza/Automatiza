package text

import "testing"

func containsPhrase(list []string, want string) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}

func TestSpamLinterFindsPlantedPhraseCaseInsensitive(t *testing.T) {
	t.Parallel()
	l := DefaultSpamLinter()
	got := l.Check("Aproveite e GANHE DINHEIRO sem sair de casa!")
	if !containsPhrase(got, "ganhe dinheiro") {
		t.Fatalf("expected to detect 'ganhe dinheiro', got %v", got)
	}
}

func TestSpamLinterEmptyForCleanText(t *testing.T) {
	t.Parallel()
	l := DefaultSpamLinter()
	got := l.Check("Olá! Segue o link do produto que você pediu, um abraço.")
	if len(got) != 0 {
		t.Fatalf("expected no phrases for clean text, got %v", got)
	}
}

func TestSpamLinterNilReceiverSafe(t *testing.T) {
	t.Parallel()
	var l *SpamLinter
	got := l.Check("ganhe dinheiro agora")
	if len(got) != 0 {
		t.Fatalf("nil receiver should return empty, got %v", got)
	}
}

func TestNewSpamLinterTrimsLowersDedupes(t *testing.T) {
	t.Parallel()
	l := NewSpamLinter([]string{"  Renda Extra ", "renda extra", "", "   "})
	got := l.Check("procuro RENDA EXTRA hoje")
	if len(got) != 1 || got[0] != "renda extra" {
		t.Fatalf("expected single deduped 'renda extra', got %v", got)
	}
}
