package text

import "strings"

// SpamLinter holds a set of high-risk phrases (stored lowercased and trimmed)
// used to warn — never block — about copy that is likely to trip WhatsApp spam
// heuristics. Checking is a pure read; the caller decides what to do.
type SpamLinter struct {
	phrases []string // stored lowercased, trimmed, deduped
}

// NewSpamLinter builds a linter from the given phrases, lowercasing and
// trimming each and dropping empties and duplicates (order preserved).
func NewSpamLinter(phrases []string) *SpamLinter {
	seen := make(map[string]struct{}, len(phrases))
	out := make([]string, 0, len(phrases))
	for _, p := range phrases {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return &SpamLinter{phrases: out}
}

// DefaultSpamLinter returns a linter seeded with a PT-BR high-risk phrase list.
func DefaultSpamLinter() *SpamLinter {
	return NewSpamLinter([]string{
		"ganhe dinheiro",
		"ganhar dinheiro",
		"trabalhe em casa",
		"renda extra garantida",
		"dinheiro fácil",
		"clique aqui e ganhe",
		"promoção imperdível",
		"última chance",
		"você foi selecionado",
		"ganhe agora",
		"aposte agora",
		"lucro garantido",
		"renda extra",
		"trabalhe de casa",
	})
}

// Check returns the high-risk phrases present in text (case-insensitive
// substring match), or an empty slice if none. It never mutates text. A nil
// receiver yields an empty result (warn-only semantics — the caller decides).
func (l *SpamLinter) Check(text string) []string {
	if l == nil || len(l.phrases) == 0 {
		return []string{}
	}
	lower := strings.ToLower(text)
	found := []string{}
	for _, p := range l.phrases {
		if strings.Contains(lower, p) {
			found = append(found, p)
		}
	}
	return found
}
