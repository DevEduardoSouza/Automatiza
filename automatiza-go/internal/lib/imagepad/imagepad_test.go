package imagepad

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// makeNoisyImage builds a JPEG whose pixels vary per position, so a change in
// JPEG quality actually changes the encoded bytes (a solid-color image can
// compress to identical bytes across nearby qualities).
func makeNoisyImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*7 + y*13) % 256),
				G: uint8((x*29 + y*3) % 256),
				B: uint8((x*x + y*y) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode noisy jpeg: %v", err)
	}
	return buf.Bytes()
}

// JitterJPEGBase64 must produce a valid JPEG, be seed-deterministic (same seed →
// identical bytes), vary bytes across seeds that map to different qualities, and
// fail-safe by returning the input unchanged on undecodable base64.
func TestJitterJPEGBase64(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(makeNoisyImage(t, 64, 64))

	// Output decodes as a valid JPEG.
	out, err := JitterJPEGBase64(b64, "seed-a")
	if err != nil {
		t.Fatalf("jitter: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("result not valid base64: %v", err)
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("output must decode as an image: %v", err)
	}

	// Deterministic: same seed → identical output bytes.
	out2, err := JitterJPEGBase64(b64, "seed-a")
	if err != nil {
		t.Fatalf("jitter (2nd): %v", err)
	}
	if out != out2 {
		t.Fatal("same seed must yield identical output bytes")
	}

	// Two seeds that map to different qualities must produce different bytes.
	// Pick them deterministically so the assertion never flakes on a collision.
	var seedLo, seedHi string
	qLo := -1
	for i := 0; i < 10000; i++ {
		s := fmt.Sprintf("seed-%d", i)
		q := 82 + int(fnv32(s)%11)
		if qLo == -1 {
			qLo, seedLo = q, s
			continue
		}
		if q != qLo {
			seedHi = s
			break
		}
	}
	if seedHi == "" {
		t.Fatal("could not find two seeds with distinct quality (unexpected)")
	}
	outLo, err := JitterJPEGBase64(b64, seedLo)
	if err != nil {
		t.Fatalf("jitter lo: %v", err)
	}
	outHi, err := JitterJPEGBase64(b64, seedHi)
	if err != nil {
		t.Fatalf("jitter hi: %v", err)
	}
	if outLo == outHi {
		t.Fatal("different-quality seeds must yield different output bytes")
	}

	// Fail-safe: undecodable base64 is returned unchanged with a nil error.
	const bad = "!!!not base64!!!"
	got, err := JitterJPEGBase64(bad, "seed-a")
	if err != nil {
		t.Fatalf("bad input must not error, got: %v", err)
	}
	if got != bad {
		t.Fatalf("bad input must be returned unchanged, got %q", got)
	}
}

// makeImage builds a solid-color image of the given size, encoded as jpeg or png.
func makeImage(t *testing.T, w, h int, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	default:
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	}
	if err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return buf.Bytes()
}

func decodedSize(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return cfg.Width, cfg.Height
}

// Portrait preview images must be edge-extended to a square (full product, no
// crop). The base64 helper returns a fresh base64 only for portrait.
func TestEdgeExtendPortrait_Squares(t *testing.T) {
	src := base64.StdEncoding.EncodeToString(makeImage(t, 240, 500, "jpeg")) // 0.48 portrait
	out, err := EdgeExtendPortraitJPEGBase64(src)
	if err != nil {
		t.Fatalf("edge-extend: %v", err)
	}
	if out == "" {
		t.Fatal("portrait must be edge-extended, got empty (unchanged)")
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("decode out: %v", err)
	}
	if w, h := decodedSize(t, raw); w != h || w != 500 {
		t.Fatalf("portrait must become 500x500 square, got %dx%d", w, h)
	}
}

// Square/landscape preview images are sent UNCHANGED — the helper signals this
// by returning an empty string (no error), so the caller keeps the original URL.
func TestEdgeExtendPortrait_SquareAndLandscapeUnchanged(t *testing.T) {
	for _, c := range [][2]int{{500, 500}, {500, 329}, {1000, 600}} {
		src := base64.StdEncoding.EncodeToString(makeImage(t, c[0], c[1], "jpeg"))
		out, err := EdgeExtendPortraitJPEGBase64(src)
		if err != nil {
			t.Fatalf("%dx%d: %v", c[0], c[1], err)
		}
		if out != "" {
			t.Fatalf("%dx%d (square/landscape) must be unchanged, got non-empty", c[0], c[1])
		}
	}
}

// In-range images (square→landscape) must pass through unchanged (no padding).
func TestPadToSafeAspect_InRangeUnchanged(t *testing.T) {
	cases := [][2]int{
		{1000, 885}, // landscape ~1.13 — the real ML sample
		{500, 500},  // square, exactly minAspect
		{1000, 600}, // wide ~1.67
	}
	for _, c := range cases {
		out, err := PadToSafeAspectJPEG(makeImage(t, c[0], c[1], "jpeg"))
		if err != nil {
			t.Fatalf("%dx%d: %v", c[0], c[1], err)
		}
		w, h := decodedSize(t, out)
		if w != c[0] || h != c[1] {
			t.Fatalf("in-range %dx%d should be unchanged, got %dx%d", c[0], c[1], w, h)
		}
	}
}

// Any portrait (below 1:1) must be padded up to a square — WhatsApp crops
// portrait bubbles even at 4:5, so the floor is 1.0.
func TestPadToSafeAspect_PortraitBecomesSquare(t *testing.T) {
	for _, c := range [][2]int{{500, 1000}, {854, 1200}, {281, 1000}} {
		out, err := PadToSafeAspectJPEG(makeImage(t, c[0], c[1], "jpeg"))
		if err != nil {
			t.Fatalf("%dx%d: %v", c[0], c[1], err)
		}
		w, h := decodedSize(t, out)
		if w != h || h != c[1] {
			t.Fatalf("portrait %dx%d should pad to %dx%d square, got %dx%d", c[0], c[1], c[1], c[1], w, h)
		}
	}
}

// Too-wide landscape must be heightened to maxAspect.
func TestPadToSafeAspect_WideLandscapeHeightened(t *testing.T) {
	out, err := PadToSafeAspectJPEG(makeImage(t, 1910, 500, "jpeg")) // 3.82 aspect
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, h := decodedSize(t, out)
	if w != 1910 {
		t.Fatalf("width should be unchanged at 1910, got %d", w)
	}
	if h != 1000 { // 1910 / 1.91
		t.Fatalf("height should be padded to 1000 (maxAspect), got %d", h)
	}
}

// Padding gutters must be white.
func TestPadToSafeAspect_PadIsWhite(t *testing.T) {
	out, err := PadToSafeAspectJPEG(makeImage(t, 100, 300, "jpeg")) // very tall → wide gutters
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode out: %v", err)
	}
	r, g, b, _ := img.At(2, img.Bounds().Dy()/2).RGBA() // left gutter, mid-height
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("expected near-white padding, got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

// PNG input must decode and normalize too (image.Decode needs the png decoder).
func TestPadToSafeAspect_PNGInput(t *testing.T) {
	out, err := PadToSafeAspectJPEG(makeImage(t, 300, 600, "png")) // tall PNG → square
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, h := decodedSize(t, out)
	if w != h || h != 600 {
		t.Fatalf("expected 600x600 square from PNG, got %dx%d", w, h)
	}
}

// WebP input must decode and normalize too. This is the regression guard for
// the ML/Shopee crop: their CDNs serve .webp, and without the webp decoder
// image.Decode failed → SendImage fail-opened → the raw portrait shipped and
// WhatsApp center-cropped it. A 200x400 portrait webp (aspect 0.5) must pad to
// a 400x400 square just like the jpeg/png paths.
func TestPadToSafeAspect_WebPInput(t *testing.T) {
	// 200x400 solid-color portrait, encoded as WebP (no stdlib webp encoder, so
	// embedded as a fixture). Decodes only if the webp decoder is registered.
	const portraitWebPB64 = "UklGRuIAAABXRUJQVlA4INYAAABQFQCdASrIAJABPjEYi0SiIaEQFAAgAwS0t3C7sI9uA/AD8AAAC0tk5D32ych77ZOQ99snIe+2TkPfbJyHvtk5D32ych77ZOQ99snIe+2TkPfbJyHvtk5D32ych77ZOQ99snIe+2TkPfbJyHvtk5D32ych77ZOQ99snIe+2TkPfbJyHvtk5D32ych77ZOQ99snIe+2TkPfbJyHvtk5D32ych77ZOQ99snIe+2TkPfbJyHrAAD+/7kxP//94zzoLm/oR///v9V4v4H/rgAAAAAAAAAAAAAA"
	raw, err := base64.StdEncoding.DecodeString(portraitWebPB64)
	if err != nil {
		t.Fatalf("decode fixture base64: %v", err)
	}
	out, err := PadToSafeAspectJPEG(raw)
	if err != nil {
		t.Fatalf("webp must decode (decoder registered): %v", err)
	}
	w, h := decodedSize(t, out)
	if w != h || h != 400 {
		t.Fatalf("expected 400x400 square from 200x400 webp, got %dx%d", w, h)
	}
}

func TestPadToSafeAspect_BadInput(t *testing.T) {
	if _, err := PadToSafeAspectJPEG([]byte("not an image")); err == nil {
		t.Fatal("expected error for undecodable input")
	}
}

func TestAspectFromEnv(t *testing.T) {
	if got := aspectFromEnv("IMAGE_ASPECT_UNSET_XYZ", 1.91); got != 1.91 {
		t.Fatalf("absent env should fall back to default, got %v", got)
	}
	t.Setenv("IMAGE_ASPECT_TEST", "1.3")
	if got := aspectFromEnv("IMAGE_ASPECT_TEST", 1.91); got != 1.3 {
		t.Fatalf("env override should win, got %v", got)
	}
	t.Setenv("IMAGE_ASPECT_TEST", "garbage")
	if got := aspectFromEnv("IMAGE_ASPECT_TEST", 1.91); got != 1.91 {
		t.Fatalf("unparseable env should fall back to default, got %v", got)
	}
	t.Setenv("IMAGE_ASPECT_TEST", "-1")
	if got := aspectFromEnv("IMAGE_ASPECT_TEST", 1.91); got != 1.91 {
		t.Fatalf("non-positive env should fall back to default, got %v", got)
	}
}

// The base64 entry point (used by the relay path) must normalize a wide banner
// the same way the byte-slice path does — this is the coupon-banner crop fix.
func TestPadToSafeAspectJPEGBase64_WideBanner(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(makeImage(t, 1910, 500, "jpeg")) // 3.82
	out, err := PadToSafeAspectJPEGBase64(b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("result not valid base64: %v", err)
	}
	w, h := decodedSize(t, raw)
	if w != 1910 || h != 1000 { // heightened to maxAspect 1.91
		t.Fatalf("wide banner should be padded to 1910x1000, got %dx%d", w, h)
	}
}

// A "data:<mime>;base64,<payload>" prefix must be tolerated and stripped.
func TestPadToSafeAspectJPEGBase64_DataURLPrefix(t *testing.T) {
	b64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makeImage(t, 300, 600, "png"))
	out, err := PadToSafeAspectJPEGBase64(b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(out)
	w, h := decodedSize(t, raw)
	if w != h || h != 600 {
		t.Fatalf("tall data-URL image should pad to 600x600 square, got %dx%d", w, h)
	}
}

func TestPadToSafeAspectJPEGBase64_BadInput(t *testing.T) {
	if _, err := PadToSafeAspectJPEGBase64("!!!not base64!!!"); err == nil {
		t.Fatal("expected error for undecodable base64")
	}
}

// The relay path squares forwarded banners (default relayMaxAspect=1.0), because
// the group bubble only shows square images in full.
func TestPadToSquareJPEGBase64(t *testing.T) {
	cases := [][2]int{
		{1910, 500}, // wide banner → square
		{500, 1910}, // tall → square
		{800, 800},  // already square → unchanged
	}
	for _, c := range cases {
		b64 := base64.StdEncoding.EncodeToString(makeImage(t, c[0], c[1], "jpeg"))
		out, err := PadToSquareJPEGBase64(b64)
		if err != nil {
			t.Fatalf("%dx%d: %v", c[0], c[1], err)
		}
		raw, derr := base64.StdEncoding.DecodeString(out)
		if derr != nil {
			t.Fatalf("%dx%d: result not base64: %v", c[0], c[1], derr)
		}
		w, h := decodedSize(t, raw)
		if w != h {
			t.Fatalf("%dx%d relay output must be square, got %dx%d", c[0], c[1], w, h)
		}
		// Square side = the larger original dimension (image never scaled).
		want := c[0]
		if c[1] > want {
			want = c[1]
		}
		if w != want {
			t.Fatalf("%dx%d square side should be %d, got %d", c[0], c[1], want, w)
		}
	}
}
