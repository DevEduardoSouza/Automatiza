// Package imagepad normalizes product images so WhatsApp doesn't crop them.
//
// WhatsApp center-crops ("zooms") images in the chat bubble only when their
// aspect ratio is outside a displayable window — very tall portrait or very
// wide landscape photos lose their top/bottom (or sides). Images already
// inside that window are shown in full, rectangular, with no cropping.
//
// So we don't force a square (which wastes space with white gutters on images
// WhatsApp would have shown fine): we only pad enough to bring an out-of-range
// image to the nearest edge of the safe window, centered on white. Images
// already in range are returned untouched in shape — rectangular, bubble-
// filling, like any normal photo.
package imagepad

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // register PNG decoder for image.Decode
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp" // register WebP decoder for image.Decode (ML/Shopee serve .webp)
)

const (
	fetchTimeout  = 8 * time.Second
	maxImageBytes = 12 << 20 // 12 MiB cap on downloaded images
	jpegQuality   = 88
)

// Safe aspect window (width/height) WhatsApp renders without cropping. The
// FULLSCREEN view tolerates up to ~1.91 landscape, but the GROUP BUBBLE
// thumbnail crops wide images to a near-portrait shape (it cut the sides off a
// wide coupon banner — the badge/icon vanished in-bubble while the full image
// still opened on tap). The exact bubble ceiling sits between 1.0 (square,
// shown in full) and 1.91 (cropped), so both bounds are overridable via
// IMAGE_MIN_ASPECT / IMAGE_MAX_ASPECT to calibrate against the real bubble
// without a rebuild. Defaults preserve the prior behavior.
var (
	minAspect = aspectFromEnv("IMAGE_MIN_ASPECT", 1.0)
	maxAspect = aspectFromEnv("IMAGE_MAX_ASPECT", 1.91)
)

// aspectFromEnv reads a positive float from env, falling back to def when the
// var is absent, unparseable, or non-positive.
func aspectFromEnv(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return def
	}
	return f
}

// defaultClient is reused across calls so we don't allocate a client (and its
// transport/connection pool) per image.
var defaultClient = &http.Client{Timeout: fetchTimeout}

// safeCanvasSize returns the canvas dimensions needed to bring a w×h image
// inside the [minA, maxA] window. If the image is already in range, it returns
// w, h unchanged (no padding). Otherwise it grows the short axis: too tall →
// wider canvas; too wide → taller canvas. The image itself is never scaled,
// only centered on the (possibly larger) canvas.
func safeCanvasSize(w, h int, minA, maxA float64) (int, int) {
	aspect := float64(w) / float64(h)
	switch {
	case aspect < minA:
		return int(math.Round(float64(h) * minA)), h
	case aspect > maxA:
		return w, int(math.Round(float64(w) / maxA))
	default:
		return w, h
	}
}

// PadToSafeAspectJPEG decodes src (JPEG or PNG) and, if its aspect ratio is
// outside WhatsApp's safe window [minAspect, maxAspect], composites it centered
// on a white canvas just large enough to bring it into range; otherwise it
// re-encodes the image as-is (rectangular, no gutters). Returns an error if src
// can't be decoded — callers should fall back to the original image untouched.
func PadToSafeAspectJPEG(src []byte) ([]byte, error) {
	return padToAspectJPEG(src, minAspect, maxAspect)
}

// padToAspectJPEG is the shared core: it pads src into the [minA, maxA] window.
// Callers pick the window — the default safe window for URL/product images, or
// the tighter relay window (square) for forwarded media that lands in the group
// bubble.
func padToAspectJPEG(src []byte, minA, maxA float64) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image bounds %dx%d", w, h)
	}

	cw, ch := safeCanvasSize(w, h, minA, maxA)

	canvas := image.NewRGBA(image.Rect(0, 0, cw, ch))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	offX := (cw - w) / 2
	offY := (ch - h) / 2
	draw.Draw(canvas, image.Rect(offX, offY, offX+w, offY+h), img, b.Min, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return out.Bytes(), nil
}

// FetchAndPadToSafeAspectJPEGBase64 downloads url, normalizes it to WhatsApp's
// safe aspect window, and returns the result as a base64-encoded JPEG string
// ready for SendImageInput.ImageBase64. Any failure (download, non-200, decode)
// returns an error so the caller can fall back to sending the raw URL.
func FetchAndPadToSafeAspectJPEGBase64(ctx context.Context, url string) (string, error) {
	raw, err := fetchImageBytes(ctx, url)
	if err != nil {
		return "", err
	}
	padded, err := PadToSafeAspectJPEG(raw)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}

// fetchImageBytes downloads an image URL with a browser-ish UA and a size cap.
func fetchImageBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// ML/Shopee CDNs are friendlier with a browser-ish UA.
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return raw, nil
}

// edgeExtendPortraitToSquareJPEG squares a PORTRAIT image (aspect < 1) by
// filling the side margins with the image's own edge columns, so the photo's
// background flows continuously into the margins — no white bar, no blur, no
// crop. It reads as a natural product-on-background shot.
//
// WhatsApp center-crops anything taller than square in BOTH the link-preview
// card and the photo bubble (measured via probe), so a tall bottle loses its
// cap/base. Squaring shows the full product; edge-extend (vs white/blur) keeps
// it looking native. Returns padded=false for square/landscape images, which
// must be sent unchanged (they already render in full).
func edgeExtendPortraitToSquareJPEG(src []byte) (out []byte, padded bool, err error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, false, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, false, fmt.Errorf("invalid image bounds %dx%d", w, h)
	}
	if w >= h {
		return nil, false, nil // square/landscape: leave unchanged
	}

	canvas := image.NewRGBA(image.Rect(0, 0, h, h))
	offX := (h - w) / 2
	// The product photo, centered horizontally.
	draw.Draw(canvas, image.Rect(offX, 0, offX+w, h), img, b.Min, draw.Src)
	// Fill each side margin by replicating that row's edge pixel.
	for y := 0; y < h; y++ {
		left := img.At(b.Min.X, b.Min.Y+y)
		for x := 0; x < offX; x++ {
			canvas.Set(x, y, left)
		}
		right := img.At(b.Min.X+w-1, b.Min.Y+y)
		for x := offX + w; x < h; x++ {
			canvas.Set(x, y, right)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, false, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), true, nil
}

// FetchEdgeExtendPortraitJPEGBase64 downloads url and, if it's a portrait image,
// returns an edge-extended square JPEG as base64. Returns "" (no error) for
// square/landscape images — the caller should then send the original URL
// unchanged. Any download/decode failure returns an error (caller keeps URL).
func FetchEdgeExtendPortraitJPEGBase64(ctx context.Context, url string) (string, error) {
	raw, err := fetchImageBytes(ctx, url)
	if err != nil {
		return "", err
	}
	padded, ok, err := edgeExtendPortraitToSquareJPEG(raw)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}

// EdgeExtendPortraitJPEGBase64 edge-extends an already-base64 image to a square
// when it's portrait; returns "" (no error) for square/landscape (send original).
func EdgeExtendPortraitJPEGBase64(b64 string) (string, error) {
	raw, err := decodeBase64Image(b64)
	if err != nil {
		return "", err
	}
	padded, ok, err := edgeExtendPortraitToSquareJPEG(raw)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}

// PadToSafeAspectJPEGBase64 normalizes an already-base64-encoded image (JPEG or
// PNG, with or without a "data:<mime>;base64," prefix) to WhatsApp's safe aspect
// window and returns it as a plain base64 JPEG string.
//
// It exists for senders that already hold base64 rather than a URL — chiefly
// the offer-generate/monitor RELAY, which downloads source media from WAHA and
// forwards it as ImageBase64. Those bypassed the URL-only normalization in
// SendImage, so a wide relayed banner (e.g. a Shopee coupon card) went out raw
// and WhatsApp center-cropped its sides. Routing base64 through here closes that
// gap. Fail-open: callers should send the original base64 on any error.
func PadToSafeAspectJPEGBase64(b64 string) (string, error) {
	raw, err := decodeBase64Image(b64)
	if err != nil {
		return "", err
	}
	padded, err := padToAspectJPEG(raw, minAspect, maxAspect)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}

// relayMaxAspect bounds RELAYED/forwarded images, which go out as base64 and
// land in the group bubble. Unlike the fullscreen view (which tolerates up to
// maxAspect), the group bubble on mobile only shows a SQUARE image in full —
// anything wider is side-cropped, so a relayed coupon banner lost its badge and
// icon in-bubble. Relayed media is therefore squared by default (1.0).
// Overridable via IMAGE_RELAY_MAX_ASPECT if the bubble tolerance ever changes.
// This is intentionally separate from maxAspect so product photos sent by URL
// (list/search/generated offers) keep their natural rectangular shape.
var relayMaxAspect = aspectFromEnv("IMAGE_RELAY_MAX_ASPECT", 1.0)

// PadToSquareJPEGBase64 normalizes a base64 image (JPEG/PNG, with or without a
// "data:" prefix) into the relay window [minAspect, relayMaxAspect] — square by
// default — so forwarded banners show in full in the group bubble instead of
// being side-cropped. Fail-open: callers should send the original on any error.
func PadToSquareJPEGBase64(b64 string) (string, error) {
	raw, err := decodeBase64Image(b64)
	if err != nil {
		return "", err
	}
	padded, err := padToAspectJPEG(raw, minAspect, relayMaxAspect)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}

// decodeBase64Image decodes a base64 image payload, tolerating an optional
// "data:<mime>;base64," prefix and surrounding whitespace.
func decodeBase64Image(b64 string) ([]byte, error) {
	if strings.HasPrefix(b64, "data:") {
		if i := strings.IndexByte(b64, ','); i >= 0 {
			b64 = b64[i+1:]
		}
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	return raw, nil
}

// fnv32 hashes s with FNV-1a (32-bit). It's a cheap, stable, non-cryptographic
// hash used only to derive a seed-deterministic value — same input, same output
// across processes and builds.
func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// JitterJPEGBase64 re-encodes a base64 JPEG with a tiny, seed-DETERMINISTIC perturbation
// (a small JPEG quality delta) so the output BYTES differ per seed while the image looks
// identical to a human. Purpose: anti-ban — avoid broadcasting a byte-identical media hash
// to many WhatsApp groups. Deterministic: same (b64, seed) => same output bytes, so an
// at-least-once retry re-sends identical bytes. FAIL-SAFE: on any decode/encode error it
// returns the ORIGINAL b64 unchanged with a nil error (never break a send over cosmetics).
func JitterJPEGBase64(b64 string, seed string) (string, error) {
	raw, err := decodeBase64Image(b64)
	if err != nil {
		return b64, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return b64, nil
	}
	// Deterministic quality in [82, 92] derived from the seed.
	q := 82 + int(fnv32(seed)%11)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return b64, nil
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
