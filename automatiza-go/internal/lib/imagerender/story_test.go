package imagerender

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func TestRenderStoryImage_ProducesDecodableJPEG(t *testing.T) {
	data, err := RenderStoryImage("Oferta imperdível hoje!", Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Fatalf("missing JPEG magic bytes FFD8, got % x", data[:min(2, len(data))])
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	b := img.Bounds()
	if b.Dx() != 1080 || b.Dy() != 1920 {
		t.Fatalf("bounds = %dx%d, want 1080x1920", b.Dx(), b.Dy())
	}
}

func TestRenderStoryImage_LongTextDoesNotError(t *testing.T) {
	var sb strings.Builder
	para := "Este é um parágrafo longo que descreve uma promoção incrível com muitos detalhes sobre o produto, o preço e o cupom aplicável. "
	for sb.Len() < 1500 {
		sb.WriteString(para)
		sb.WriteString("\n\n")
	}
	data, err := RenderStoryImage(sb.String(), Options{})
	if err != nil {
		t.Fatalf("render long text: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	if img.Bounds().Dx() != 1080 || img.Bounds().Dy() != 1920 {
		t.Fatalf("bounds = %v, want 1080x1920", img.Bounds())
	}
}

func TestRenderStoryImage_EmptyAfterSanitize(t *testing.T) {
	for _, in := range []string{"😀🔥✨", "   \n\t  ", "  😀 🔥  "} {
		if _, err := RenderStoryImage(in, Options{}); err == nil {
			t.Fatalf("RenderStoryImage(%q) = nil error, want error", in)
		}
	}
}

func TestRenderStoryImage_Deterministic(t *testing.T) {
	const text = "Mesmo texto, mesmos bytes — sem rand, sem time."
	a, err := RenderStoryImage(text, Options{})
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := RenderStoryImage(text, Options{})
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic output: len(a)=%d len(b)=%d", len(a), len(b))
	}
}
