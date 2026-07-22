// Package imagerender renders free text into a Story-format JPEG (1080x1920),
// the only image shape the Meta Stories API accepts. It has no external
// dependency beyond golang.org/x/image (already vendored) and the stdlib, and
// every render is deterministic: the same text and Options always produce the
// same bytes (background color is a hash of the text, no rand/time is used).
package imagerender

import (
	"bytes"
	"errors"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// margin is the safe inset (px) kept clear of text on every side.
const margin = 96

// Options controls the output geometry and the font-size search range. A zero
// (or negative) field falls back to its default via withDefaults.
type Options struct {
	Width       int     // default 1080
	Height      int     // default 1920
	MaxFontSize float64 // default 76
	MinFontSize float64 // default 34
}

func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 1080
	}
	if o.Height <= 0 {
		o.Height = 1920
	}
	if o.MaxFontSize <= 0 {
		o.MaxFontSize = 76
	}
	if o.MinFontSize <= 0 {
		o.MinFontSize = 34
	}
	return o
}

// The parsed goregular font is immutable and shared across calls; each render
// uses its own sfnt.Buffer, so concurrent use is safe.
var (
	fontOnce sync.Once
	fontVal  *opentype.Font
	fontErr  error
)

func loadFont() (*opentype.Font, error) {
	fontOnce.Do(func() {
		fontVal, fontErr = opentype.Parse(goregular.TTF)
	})
	return fontVal, fontErr
}

// gradient is a vertical two-stop background. Tops are dark, bottoms a touch
// lighter, all chosen for legible white text.
type gradient struct{ top, bottom color.RGBA }

var palettes = []gradient{
	{color.RGBA{0x0f, 0x20, 0x3a, 0xff}, color.RGBA{0x27, 0x54, 0x8a, 0xff}}, // deep blue
	{color.RGBA{0x2b, 0x10, 0x3a, 0xff}, color.RGBA{0x5a, 0x2a, 0x66, 0xff}}, // purple
	{color.RGBA{0x06, 0x2a, 0x28, 0xff}, color.RGBA{0x1d, 0x5c, 0x50, 0xff}}, // teal
	{color.RGBA{0x3a, 0x14, 0x0f, 0xff}, color.RGBA{0x8a, 0x3a, 0x27, 0xff}}, // sunset
	{color.RGBA{0x1a, 0x1f, 0x2b, 0xff}, color.RGBA{0x3c, 0x46, 0x5c, 0xff}}, // slate
	{color.RGBA{0x24, 0x14, 0x3a, 0xff}, color.RGBA{0x6c, 0x2a, 0x54, 0xff}}, // indigo-rose
}

// paletteIndex derives a stable gradient choice from the original (unsanitized)
// text, so the background stays consistent for a given caption.
func paletteIndex(text string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	return int(h.Sum32() % uint32(len(palettes)))
}

// RenderStoryImage draws text centered over a vertical gradient and returns the
// JPEG bytes (quality 85). It errors if the font fails to load or the text is
// empty after sanitizing.
func RenderStoryImage(text string, opts Options) ([]byte, error) {
	fnt, err := loadFont()
	if err != nil {
		return nil, err
	}
	opts = opts.withDefaults()

	clean := sanitize(fnt, text)
	if clean == "" {
		return nil, errors.New("imagerender: empty text after sanitize")
	}

	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	drawGradient(img, palettes[paletteIndex(text)])

	lay, err := computeLayout(fnt, clean, opts)
	if err != nil {
		return nil, err
	}
	drawText(img, lay, opts)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sanitize keeps '\n', turns '\t' into a space, drops C0/C1 control chars and
// any rune goregular has no glyph for (emoji, etc.), then trims the result.
// Control chars are dropped by codepoint range, not by glyph lookup, because
// the font maps some of them (e.g. NUL) to a visible .notdef box.
func sanitize(fnt *opentype.Font, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b sfnt.Buffer
	var out strings.Builder
	out.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\n':
			out.WriteByte('\n')
		case r == '\t' || r == ' ':
			out.WriteByte(' ')
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			// control character, drop
		default:
			idx, err := fnt.GlyphIndex(&b, r)
			if err != nil || idx == 0 {
				continue
			}
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}

// drawGradient fills dst row by row, lerping top→bottom over the full height.
func drawGradient(dst *image.RGBA, g gradient) {
	b := dst.Bounds()
	h := b.Dy()
	if h <= 0 {
		return
	}
	for y := 0; y < h; y++ {
		t := 0.0
		if h > 1 {
			t = float64(y) / float64(h-1)
		}
		c := lerpColor(g.top, g.bottom, t)
		draw.Draw(dst, image.Rect(b.Min.X, b.Min.Y+y, b.Max.X, b.Min.Y+y+1),
			&image.Uniform{C: c}, image.Point{}, draw.Src)
	}
}

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: lerpU8(a.R, b.R, t),
		G: lerpU8(a.G, b.G, t),
		B: lerpU8(a.B, b.B, t),
		A: 0xff,
	}
}

func lerpU8(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
}

// layout is the wrapped text at the largest font size that fits (or MinFontSize
// with excess lines truncated).
type layout struct {
	lines      []string
	face       font.Face
	lineHeight int
}

func newFace(fnt *opentype.Font, size float64) (font.Face, error) {
	return opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     72, // 72 DPI ⇒ point size == pixel size
		Hinting: font.HintingFull,
	})
}

// computeLayout wraps clean text and shrinks the font from Max to Min in 6px
// steps until the block fits the usable box. At MinFontSize it stops shrinking
// and truncates the overflow, ellipsizing the last visible line.
func computeLayout(fnt *opentype.Font, clean string, opts Options) (layout, error) {
	logicals := strings.Split(clean, "\n")
	usableW := opts.Width - 2*margin
	usableH := opts.Height - 2*margin
	if usableW < 1 {
		usableW = 1
	}
	const step = 6.0

	var last layout
	for size := opts.MaxFontSize; ; size -= step {
		if size < opts.MinFontSize {
			size = opts.MinFontSize
		}
		face, err := newFace(fnt, size)
		if err != nil {
			return layout{}, err
		}
		var lines []string
		for _, lg := range logicals {
			wrapped := wrapLine(face, lg, usableW)
			if len(wrapped) == 0 {
				lines = append(lines, "") // preserve blank paragraph line
			} else {
				lines = append(lines, wrapped...)
			}
		}
		lineHeight := int(size*1.3 + 0.5)
		last = layout{lines: lines, face: face, lineHeight: lineHeight}

		if len(lines)*lineHeight <= usableH {
			return last, nil
		}
		if size <= opts.MinFontSize {
			break
		}
	}

	// At MinFontSize and still too tall: keep only the lines that fit and mark
	// the last one with an ellipsis.
	maxLines := usableH / last.lineHeight
	if maxLines < 1 {
		maxLines = 1
	}
	if len(last.lines) > maxLines {
		last.lines = last.lines[:maxLines]
		li := maxLines - 1
		last.lines[li] = ellipsize(last.face, last.lines[li], usableW)
	}
	return last, nil
}

// wrapLine greedily packs words of one logical line into visual lines no wider
// than maxWidthPx. A single word wider than the box is hard-broken by rune.
func wrapLine(face font.Face, logical string, maxWidthPx int) []string {
	words := strings.Fields(logical)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	cur := ""
	for _, w := range words {
		cand := w
		if cur != "" {
			cand = cur + " " + w
		}
		if font.MeasureString(face, cand).Ceil() <= maxWidthPx {
			cur = cand
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
		if font.MeasureString(face, w).Ceil() > maxWidthPx {
			pieces := breakLongWord(face, w, maxWidthPx)
			lines = append(lines, pieces[:len(pieces)-1]...)
			cur = pieces[len(pieces)-1]
		} else {
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// breakLongWord splits a single unbreakable word into rune chunks that each fit
// maxWidthPx. It always returns at least one piece.
func breakLongWord(face font.Face, w string, maxWidthPx int) []string {
	var lines []string
	cur := ""
	for _, r := range w {
		cand := cur + string(r)
		if cur != "" && font.MeasureString(face, cand).Ceil() > maxWidthPx {
			lines = append(lines, cur)
			cur = string(r)
		} else {
			cur = cand
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = append(lines, w)
	}
	return lines
}

// ellipsize trims the line rune by rune until it plus "…" fits maxWidthPx.
func ellipsize(face font.Face, line string, maxWidthPx int) string {
	const ell = "…"
	if font.MeasureString(face, line+ell).Ceil() <= maxWidthPx {
		return line + ell
	}
	runes := []rune(line)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		cand := strings.TrimRight(string(runes), " ") + ell
		if font.MeasureString(face, cand).Ceil() <= maxWidthPx {
			return cand
		}
	}
	return ell
}

// drawText renders the layout white and centered, both axes.
func drawText(dst *image.RGBA, lay layout, opts Options) {
	n := len(lay.lines)
	blockH := n * lay.lineHeight
	startY := (opts.Height - blockH) / 2
	if startY < margin {
		startY = margin
	}

	m := lay.face.Metrics()
	ascent := m.Ascent.Ceil()
	descent := m.Descent.Ceil()
	gap := lay.lineHeight - (ascent + descent)
	if gap < 0 {
		gap = 0
	}
	baseFromTop := gap/2 + ascent

	for i, line := range lay.lines {
		if line == "" {
			continue
		}
		adv := font.MeasureString(lay.face, line)
		x := (fixed.I(opts.Width) - adv) / 2
		baseline := startY + i*lay.lineHeight + baseFromTop
		d := &font.Drawer{
			Dst:  dst,
			Src:  image.White,
			Face: lay.face,
			Dot:  fixed.Point26_6{X: x, Y: fixed.I(baseline)},
		}
		d.DrawString(line)
	}
}
