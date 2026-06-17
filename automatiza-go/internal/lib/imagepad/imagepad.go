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
// inside the [minAspect, maxAspect] window. If the image is already in range,
// it returns w, h unchanged (no padding). Otherwise it grows the short axis:
// too tall → wider canvas; too wide → taller canvas. The image itself is never
// scaled, only centered on the (possibly larger) canvas.
func safeCanvasSize(w, h int) (int, int) {
	aspect := float64(w) / float64(h)
	switch {
	case aspect < minAspect:
		return int(math.Round(float64(h) * minAspect)), h
	case aspect > maxAspect:
		return w, int(math.Round(float64(w) / maxAspect))
	default:
		return w, h
	}
}

// PadToSafeAspectJPEG decodes src (JPEG or PNG) and, if its aspect ratio is
// outside WhatsApp's safe window, composites it centered on a white canvas just
// large enough to bring it into range; otherwise it re-encodes the image as-is
// (rectangular, no gutters). Returns an error if src can't be decoded — callers
// should fall back to sending the original image untouched.
func PadToSafeAspectJPEG(src []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image bounds %dx%d", w, h)
	}

	cw, ch := safeCanvasSize(w, h)

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	// ML/Shopee CDNs are friendlier with a browser-ish UA.
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch image: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	padded, err := PadToSafeAspectJPEG(raw)
	if err != nil {
		return "", err
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
	// Strip an optional data-URL prefix ("data:image/png;base64,<payload>").
	if strings.HasPrefix(b64, "data:") {
		if i := strings.IndexByte(b64, ','); i >= 0 {
			b64 = b64[i+1:]
		}
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	padded, err := PadToSafeAspectJPEG(raw)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(padded), nil
}
