// Package text: message-variation (anti-ban) helpers.
//
// Vary produces a per-recipient variant of an outgoing WhatsApp message so
// that identical broadcast copies do not go out byte-for-byte to every number
// (a strong ban signal). It is STRICTLY ADDITIVE: it never removes visible
// content and never turns a non-empty message into an empty one. Protected
// spans — URLs, WhatsApp markdown runs and `{token}`/`{{token}}` placeholders —
// are masked before any jitter and restored verbatim afterwards, so no
// transformation can ever mutate a character inside a URL.
package text

import (
	"hash/fnv"
	mathrand "math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// VaryOptions configures Vary. All jitters are opt-in and cosmetic.
type VaryOptions struct {
	Spintax      bool
	ZeroWidth    bool
	ZeroWidthMax int // max zero-width chars to insert; <=0 disables ZW even if ZeroWidth true
	EmojiJitter  bool
	EmojiSet     []string // may include "" meaning "append nothing"; nil => a sensible default incl ""
	PunctJitter  bool
	// LineBreak applies SUBTLE, word-preserving structural jitter: an invisible
	// trailing space on a random line and, occasionally, one extra blank line at
	// a paragraph boundary. It never removes, reorders or rewrites content.
	LineBreak bool
}

// protectRe matches, left-to-right and non-overlapping, every span that must
// survive Vary untouched. Ordered so URLs win first (their greedy \S+ would
// otherwise be clipped by a later alternative) and double braces win over
// single braces.
var protectRe = regexp.MustCompile(
	`https?://\S+` + // URLs
		"|" + "```[\\s\\S]*?```" + // triple-backtick fenced code
		`|\*[^*\n]+\*` + // *bold*
		`|_[^_\n]+_` + // _italic_
		`|~[^~\n]+~` + // ~strike~
		`|\{\{[^{}|]+\}\}` + // {{placeholder}}
		`|\{[^{}|]+\}`, // {placeholder}
)

// restoreRe matches a sentinel token \x00<idx>\x00 emitted while masking.
var restoreRe = regexp.MustCompile("\x00([0-9]+)\x00")

// zwChars are the zero-width runes inserted by the ZW jitter:
// U+200B ZERO WIDTH SPACE, U+200C ZERO WIDTH NON-JOINER, U+FEFF ZERO WIDTH NO-BREAK SPACE.
var zwChars = []rune{'\u200B', '\u200C', '\uFEFF'}

// defaultEmojiSet is biased toward the two no-op entries so most sends are
// left alone.
var defaultEmojiSet = []string{"", "", "🔥", "✅", "👍"}

// Vary returns a per-recipient variant of input. It is STRICTLY ADDITIVE: it must
// never remove visible content and must never turn a non-empty string into an empty
// one. When seed is provided, output is DETERMINISTIC (same input+opts+seed => same
// bytes) so an at-least-once retry re-sends identical bytes.
func Vary(input string, opts VaryOptions, seed ...string) string {
	if input == "" {
		return input
	}

	rng := newVaryRNG(input, seed)

	// Step 2: mask protected spans -> sentinels.
	var originals []string
	result := protectRe.ReplaceAllStringFunc(input, func(m string) string {
		idx := len(originals)
		originals = append(originals, m)
		return sentinelToken(idx)
	})

	// Step 3: spintax expansion (on masked text).
	if opts.Spintax {
		result = expandSpintax(result, rng)
	}

	// Step 4: zero-width jitter.
	if opts.ZeroWidth && opts.ZeroWidthMax > 0 {
		result = zeroWidthJitter(result, opts.ZeroWidthMax, rng)
	}

	// Step 5: emoji / punctuation jitter (cosmetic, append-only).
	if opts.EmojiJitter {
		result = emojiJitter(result, opts.EmojiSet, rng)
	}
	if opts.PunctJitter {
		result = punctJitter(result, rng)
	}
	if opts.LineBreak {
		result = lineBreakJitter(result, rng)
	}

	// Step 6: restore sentinels verbatim.
	result = restoreRe.ReplaceAllStringFunc(result, func(m string) string {
		i, err := strconv.Atoi(m[1 : len(m)-1]) // strip the two NUL delimiters
		if err != nil || i < 0 || i >= len(originals) {
			return m
		}
		return originals[i]
	})

	// Step 7: safety net — never emit empty from non-empty input.
	if result == "" {
		return input
	}
	return result
}

// sentinelToken builds a NUL-delimited placeholder that contains no spaces,
// no braces/pipe and no letters, so neither spintax nor the jitters touch it.
func sentinelToken(idx int) string {
	return "\x00" + strconv.Itoa(idx) + "\x00"
}

// newVaryRNG derives a PCG-backed *Rand. With a seed the state comes from
// FNV-64a of the joined seed and of the input (deterministic per recipient).
// Without a seed it still derives purely from the input so output is stable
// per content (no time / global rand reliance).
func newVaryRNG(input string, seed []string) *mathrand.Rand {
	key := input
	if len(seed) > 0 {
		key = strings.Join(seed, "\x00")
	}
	h1 := fnv64a(key)
	h2 := fnv64a(input)
	return mathrand.New(mathrand.NewPCG(h1, h2)) //nolint:gosec // G404: non-security random for message variation
}

func fnv64a(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// expandSpintax expands {a|b|c} groups, supporting nesting, using rng to pick
// one branch per group. Groups without a top-level '|' are emitted verbatim;
// unbalanced braces cause the remainder to be emitted literally (never panics).
func expandSpintax(input string, rng *mathrand.Rand) string {
	return expandSpintaxRunes([]rune(input), rng)
}

func expandSpintaxRunes(runes []rune, rng *mathrand.Rand) string {
	var b strings.Builder
	i := 0
	for i < len(runes) {
		if runes[i] != '{' {
			b.WriteRune(runes[i])
			i++
			continue
		}
		closeIdx, ok := matchBrace(runes, i)
		if !ok {
			// Unbalanced: emit the rest literally and stop.
			b.WriteString(string(runes[i:]))
			return b.String()
		}
		branches := splitTopLevelPipe(runes[i+1 : closeIdx])
		if len(branches) > 1 {
			pick := branches[rng.IntN(len(branches))] //nolint:gosec // G404: non-security random for message variation
			b.WriteString(expandSpintaxRunes(pick, rng))
		} else {
			// No top-level '|': emit the whole group verbatim.
			b.WriteString(string(runes[i : closeIdx+1]))
		}
		i = closeIdx + 1
	}
	return b.String()
}

// matchBrace returns the index of the '}' that closes the '{' at start, or
// (-1, false) when the braces are unbalanced.
func matchBrace(runes []rune, start int) (int, bool) {
	depth := 0
	for i := start; i < len(runes); i++ {
		switch runes[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}

// splitTopLevelPipe splits on '|' runes at brace-depth 0. A single returned
// branch means there was no top-level pipe.
func splitTopLevelPipe(runes []rune) [][]rune {
	var branches [][]rune
	depth := 0
	start := 0
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '|':
			if depth == 0 {
				branches = append(branches, runes[start:i])
				start = i + 1
			}
		}
	}
	branches = append(branches, runes[start:])
	return branches
}

// zeroWidthJitter inserts up to maxN zero-width runes at interior boundaries
// that sit immediately after a space rune and before a non-space, non-sentinel
// rune. It never inserts inside (or adjacent to) a \x00 sentinel token.
func zeroWidthJitter(s string, maxN int, rng *mathrand.Rand) string {
	var cand []int
	prev := rune(-1)
	for i, r := range s {
		if i > 0 && prev == ' ' && r != '\x00' && !unicode.IsSpace(r) {
			cand = append(cand, i)
		}
		prev = r
	}

	n := rng.IntN(maxN + 1) //nolint:gosec // G404: non-security random for message variation
	if n > len(cand) {
		n = len(cand)
	}
	if n == 0 {
		return s
	}

	// Partial Fisher-Yates to pick n distinct candidate offsets.
	for i := 0; i < n; i++ {
		j := i + rng.IntN(len(cand)-i) //nolint:gosec // G404: non-security random for message variation
		cand[i], cand[j] = cand[j], cand[i]
	}
	chosen := append([]int(nil), cand[:n]...)
	sort.Ints(chosen)

	picks := make([]rune, len(chosen))
	for i := range picks {
		picks[i] = zwChars[rng.IntN(len(zwChars))] //nolint:gosec // G404: non-security random for message variation
	}

	var b strings.Builder
	b.Grow(len(s) + n*3)
	ci := 0
	for i := 0; i < len(s); i++ {
		for ci < len(chosen) && chosen[ci] == i {
			b.WriteRune(picks[ci])
			ci++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// emojiJitter, with ~50% probability, appends one element from set to the last
// non-empty line — but only if that line does not already end with an emoji /
// symbol rune. Appending happens after any masked span, so URLs stay intact.
func emojiJitter(s string, set []string, rng *mathrand.Rand) string {
	if set == nil {
		set = defaultEmojiSet
	}
	if len(set) == 0 {
		return s
	}
	if rng.IntN(2) != 0 { //nolint:gosec // G404: non-security random for message variation
		return s
	}
	pick := set[rng.IntN(len(set))] //nolint:gosec // G404: non-security random for message variation
	if pick == "" {
		return s
	}

	lines := strings.Split(s, "\n")
	idx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			idx = i
			break
		}
	}
	if idx == -1 {
		return s
	}
	if r, size := utf8.DecodeLastRuneInString(lines[idx]); size > 0 && unicode.In(r, unicode.So, unicode.Sk) {
		return s // already ends with an emoji/symbol
	}
	lines[idx] += pick
	return strings.Join(lines, "\n")
}

// punctJitter, with ~30% probability, toggles a single trailing '!' to '.' or
// appends one trailing newline. Cosmetic only.
func punctJitter(s string, rng *mathrand.Rand) string {
	if rng.IntN(10) >= 3 { //nolint:gosec // G404: non-security random for message variation
		return s
	}
	if strings.HasSuffix(s, "!") {
		return s[:len(s)-1] + "."
	}
	return s + "\n"
}

// lineBreakJitter applies SUBTLE, word-preserving structural variation so the
// message's byte layout differs per recipient without rewriting the template:
//   - ~45%: append a single trailing space to a random non-empty line (invisible).
//   - ~20%: insert one extra blank line at an interior paragraph boundary, only
//     when the message already has 3+ non-empty lines (a multi-section message).
//
// It never removes, reorders, or edits words, and it operates on the masked text
// so it can never split or alter a URL/markdown/placeholder span.
func lineBreakJitter(s string, rng *mathrand.Rand) string {
	lines := strings.Split(s, "\n")
	var nonEmpty []int
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, i)
		}
	}
	if len(nonEmpty) == 0 {
		return s
	}

	// (a) Invisible trailing space on a random non-empty line.
	if rng.IntN(100) < 45 { //nolint:gosec // G404: non-security random for message variation
		idx := nonEmpty[rng.IntN(len(nonEmpty))] //nolint:gosec // G404: non-security random for message variation
		if !strings.HasSuffix(lines[idx], " ") {
			lines[idx] += " "
		}
	}

	// (b) One extra blank line before a random interior non-empty line.
	if len(nonEmpty) >= 3 && rng.IntN(100) < 20 { //nolint:gosec // G404: non-security random for message variation
		before := nonEmpty[1+rng.IntN(len(nonEmpty)-1)] //nolint:gosec // G404: non-security random for message variation
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:before]...)
		out = append(out, "")
		out = append(out, lines[before:]...)
		lines = out
	}

	return strings.Join(lines, "\n")
}
