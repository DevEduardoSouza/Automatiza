package instagram

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newTestStore(t *testing.T) (*MediaStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewMediaStore(rdb, zap.NewNop()), mr
}

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 200, G: 10, B: 10, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMediaStore_PutGetRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	want := jpegOf(t, 32, 32)

	id, err := store.Put(ctx, want, time.Hour)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(id) != storyMediaIDLen {
		t.Fatalf("id deveria ter %d hex, got %q", storyMediaIDLen, id)
	}
	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("bytes recuperados diferem dos gravados")
	}
}

// Id fora do formato não pode nem chegar ao Redis: a rota é pública e o id vem
// direto da URL.
func TestMediaStore_RejectsMalformedIDBeforeRedis(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	// Redis derrubado de propósito: se o id malformado chegasse a ser
	// consultado, o erro seria de CONEXÃO, não ErrMediaNotFound. É assim que
	// este teste prova o curto-circuito, e não só o retorno.
	mr.Close()

	for _, bad := range []string{
		"",
		"curto",
		"ABCDEF01234567890123456789ABCDEF", // maiúsculas
		"../../etc/passwd",
		"0123456789012345678901234567890",   // 31 caracteres
		"012345678901234567890123456789012", // 33 caracteres
		"0123456789012345678901234567890g",  // não-hex
	} {
		if _, err := store.Get(ctx, bad); !errors.Is(err, ErrMediaNotFound) {
			t.Errorf("id %q deveria dar ErrMediaNotFound sem tocar o Redis, got %v", bad, err)
		}
	}
}

// Só JPEG entra: a rota pública responde SEMPRE image/jpeg, então guardar
// outro formato serviria bytes com Content-Type mentiroso.
func TestMediaStore_PutAcceptsOnlyJPEG(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if _, err := store.Put(ctx, pngOf(t, 16, 16), time.Hour); !errors.Is(err, ErrMediaInvalid) {
		t.Errorf("PNG deveria ser recusado, got %v", err)
	}
	if _, err := store.Put(ctx, []byte("nao-e-imagem"), time.Hour); !errors.Is(err, ErrMediaInvalid) {
		t.Errorf("lixo deveria ser recusado, got %v", err)
	}
	if _, err := store.Put(ctx, nil, time.Hour); !errors.Is(err, ErrMediaInvalid) {
		t.Errorf("vazio deveria ser recusado, got %v", err)
	}
}

func TestMediaStore_PutRejectsOversizedBytes(t *testing.T) {
	store, _ := newTestStore(t)
	big := make([]byte, maxMediaBytes+1)
	if _, err := store.Put(context.Background(), big, time.Hour); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("acima de 4MiB deveria ser ErrMediaTooLarge, got %v", err)
	}
}

// NormalizeJPEG converte formatos que o WhatsApp entrega (png, webp) no JPEG
// que o Story exige.
func TestNormalizeJPEG_ConvertsSupportedFormats(t *testing.T) {
	out, err := NormalizeJPEG(pngOf(t, 20, 20))
	if err != nil {
		t.Fatalf("PNG deveria converter: %v", err)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(out)); err != nil || format != "jpeg" {
		t.Fatalf("saída deveria ser JPEG, got format=%q err=%v", format, err)
	}
}

// Bomba de descompressão: poucos bytes declarando dimensões enormes. O teto de
// BYTES sozinho não protege — só a checagem de área antes do Decode protege.
func TestNormalizeJPEG_RejectsPixelBomb(t *testing.T) {
	// PNG de 30000x30000 comprime a alguns KB por ser uniforme.
	huge := pngOf(t, 6000, 4000) // 24M pixels > 20M
	if _, err := NormalizeJPEG(huge); !errors.Is(err, ErrMediaInvalid) {
		t.Fatalf("imagem acima de 20M pixels deveria ser recusada, got %v", err)
	}
	if len(huge) > maxMediaBytes {
		t.Skip("fixture ficou acima do teto de bytes; o teste de pixels não seria conclusivo")
	}
}

func TestNormalizeJPEG_RejectsGarbageAndOversized(t *testing.T) {
	if _, err := NormalizeJPEG([]byte("nao-e-imagem")); !errors.Is(err, ErrMediaInvalid) {
		t.Errorf("lixo deveria ser ErrMediaInvalid, got %v", err)
	}
	if _, err := NormalizeJPEG(nil); !errors.Is(err, ErrMediaInvalid) {
		t.Errorf("vazio deveria ser ErrMediaInvalid, got %v", err)
	}
	if _, err := NormalizeJPEG(make([]byte, maxMediaBytes+1)); !errors.Is(err, ErrMediaTooLarge) {
		t.Errorf("acima do teto deveria ser ErrMediaTooLarge, got %v", err)
	}
}

func TestMediaStore_GetMissingIsNotFound(t *testing.T) {
	store, _ := newTestStore(t)
	// Id bem-formado, porém inexistente.
	if _, err := store.Get(context.Background(), "0123456789abcdef0123456789abcdef"); !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("esperava ErrMediaNotFound, got %v", err)
	}
}

func TestMediaStore_DeleteRemovesKey(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	id, err := store.Put(ctx, jpegOf(t, 8, 8), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store.Delete(ctx, id)
	if _, err := store.Get(ctx, id); !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("chave deveria ter sido removida, got %v", err)
	}
}

// Ids são imprevisíveis: a ausência de autenticação na rota pública depende
// disso.
func TestMediaStore_IDsAreUnique(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		id, err := store.Put(ctx, jpegOf(t, 8, 8), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("id repetido: %s", id)
		}
		seen[id] = true
	}
}
