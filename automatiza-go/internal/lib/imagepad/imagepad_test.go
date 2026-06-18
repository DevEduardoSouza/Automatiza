package imagepad

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

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
