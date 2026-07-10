package text

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func countZeroWidth(s string) int {
	n := 0
	for _, r := range s {
		if r == '\u200B' || r == '\u200C' || r == '\uFEFF' {
			n++
		}
	}
	return n
}

func TestVarySpintaxExpandsToOneBranch(t *testing.T) {
	t.Parallel()
	got := Vary("{a|b|c}", VaryOptions{Spintax: true}, "seed")
	switch got {
	case "a", "b", "c":
		// ok: exactly one branch survived
	default:
		t.Fatalf("spintax did not expand to a single branch: got %q", got)
	}
}

func TestVaryDeterministicWithSameSeed(t *testing.T) {
	t.Parallel()
	in := "confira {a|b|c} https://go.shopee.com/AbC?x=1 agora {oi|olá|opa}"
	opts := VaryOptions{
		Spintax:      true,
		ZeroWidth:    true,
		ZeroWidthMax: 5,
		EmojiJitter:  true,
		PunctJitter:  true,
	}
	first := Vary(in, opts, "recipient", "42")
	second := Vary(in, opts, "recipient", "42")
	if first != second {
		t.Fatalf("Vary not deterministic for same seed:\n first=%q\nsecond=%q", first, second)
	}
}

func TestVaryDeterministicNoSeed(t *testing.T) {
	t.Parallel()
	in := "olá mundo {a|b} tudo bem"
	opts := VaryOptions{Spintax: true, ZeroWidth: true, ZeroWidthMax: 4}
	if a, b := Vary(in, opts), Vary(in, opts); a != b {
		t.Fatalf("Vary without seed not stable per content:\n a=%q\n b=%q", a, b)
	}
}

func TestVaryNestedSpintax(t *testing.T) {
	t.Parallel()
	got := Vary("{a|b {c|d}}", VaryOptions{Spintax: true}, "seed")
	switch got {
	case "a", "b c", "b d":
		// ok
	default:
		t.Fatalf("nested spintax produced unexpected result: %q", got)
	}
}

func TestVaryUnbalancedBracesDoesNotPanic(t *testing.T) {
	t.Parallel()
	got := Vary("start {a|b end", VaryOptions{Spintax: true}, "seed")
	if !strings.Contains(got, "{a|b") {
		t.Fatalf("unbalanced spintax should keep the literal braces, got %q", got)
	}
}

func TestVaryZeroWidthCountWithinMax(t *testing.T) {
	t.Parallel()
	in := "esta é uma mensagem com varios espacos para inserir invisiveis aqui"
	const max = 5
	opts := VaryOptions{ZeroWidth: true, ZeroWidthMax: max}
	for s := 0; s < 100; s++ {
		out := Vary(in, opts, "seed", strconv.Itoa(s))
		if c := countZeroWidth(out); c > max {
			t.Fatalf("zero-width count %d exceeds max %d (seed %d): %q", c, max, s, out)
		}
	}
}

func TestVaryAdditiveNonEmpty(t *testing.T) {
	t.Parallel()
	opts := VaryOptions{
		Spintax:      true,
		ZeroWidth:    true,
		ZeroWidthMax: 5,
		EmojiJitter:  true,
		PunctJitter:  true,
	}
	inputs := []string{
		"a",
		"olá",
		"{a|b|c}",
		"confira https://x.io/y",
		"*bold* _italic_ ~strike~ {oi|olá}",
		"linha 1\nlinha 2\n",
	}
	for _, in := range inputs {
		for s := 0; s < 50; s++ {
			out := Vary(in, opts, "seed", strconv.Itoa(s))
			if out == "" {
				t.Fatalf("Vary turned non-empty input %q into empty output (seed %d)", in, s)
			}
		}
	}
}

func TestVarySafetyNetEmptyInput(t *testing.T) {
	t.Parallel()
	if got := Vary("", VaryOptions{Spintax: true, EmojiJitter: true}); got != "" {
		t.Fatalf("empty input should stay empty, got %q", got)
	}
}

// TestVaryURLInvariant is the critical invariant: no jitter may ever alter a
// character inside a URL. URLs are masked before jitter and restored verbatim,
// so every URL extracted from the input must appear byte-identical in every
// output across many seeds and with all options enabled.
func TestVaryURLInvariant(t *testing.T) {
	t.Parallel()
	urlRe := regexp.MustCompile(`https?://\S+`)
	opts := VaryOptions{
		Spintax:      true,
		ZeroWidth:    true,
		ZeroWidthMax: 5,
		EmojiJitter:  true,
		PunctJitter:  true,
	}
	inputs := []string{
		"confira https://go.shopee.com/AbC?x=1 agora {oi|olá}",
		"link https://example.com/path?a=1&b=2 e https://x.io/y aqui {promo|oferta}",
		"https://só.um.link/no-fim/da-linha {a|b|c}",
	}
	for _, in := range inputs {
		urls := urlRe.FindAllString(in, -1)
		if len(urls) == 0 {
			t.Fatalf("test input has no URL to check: %q", in)
		}
		for s := 0; s < 200; s++ {
			out := Vary(in, opts, "seed", strconv.Itoa(s))
			for _, u := range urls {
				if !strings.Contains(out, u) {
					t.Fatalf("URL %q not byte-identical in output (seed %d): %q", u, s, out)
				}
			}
		}
	}
}
