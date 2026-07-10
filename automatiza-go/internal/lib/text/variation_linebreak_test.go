package text

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// stripWhitespace removes spaces/newlines/tabs so we can assert that the visible
// word content is untouched by whitespace-only jitter.
func stripWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestVaryLineBreakPreservesWordsAndURLs(t *testing.T) {
	input := "Oferta imperdível!\nConfira o produto:\nhttps://s.shopee.com.br/AbC123?x=1\nAproveite hoje!"
	want := stripWhitespace(input)
	urlRe := regexp.MustCompile(`https?://\S+`)
	wantURL := urlRe.FindString(input)

	for i := 0; i < 400; i++ {
		got := Vary(input, VaryOptions{LineBreak: true}, "group-"+strconv.Itoa(i))
		if stripWhitespace(got) != want {
			t.Fatalf("LineBreak altered non-whitespace content:\n in=%q\nout=%q", input, got)
		}
		if u := urlRe.FindString(got); u != wantURL {
			t.Fatalf("LineBreak mutated the URL: want %q got %q (out=%q)", wantURL, u, got)
		}
	}
}

func TestVaryLineBreakSingleLineSafe(t *testing.T) {
	// A one-line message must never gain a blank line (only maybe a trailing space).
	in := "Só uma linha com https://s.shopee.com.br/Z"
	for i := 0; i < 100; i++ {
		got := Vary(in, VaryOptions{LineBreak: true}, strconv.Itoa(i))
		if strings.Contains(got, "\n\n") {
			t.Fatalf("single-line input gained a blank line: %q", got)
		}
	}
}
