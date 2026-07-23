package instagram

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"time"

	// Decoders para converter mídia não-JPEG do WhatsApp no JPEG que o Story
	// exige (figurinha chega em webp, algumas imagens em png).
	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// maxMediaBytes caps a single story image at 4MB. O Story é um JPEG 1080x1920
// (bem abaixo de 1MB renderizado), então 4MB é folga generosa protegendo o
// Redis de um blob gigante (a lição do OOM).
const maxMediaBytes = 4 << 20

// maxImagePixels limita a ÁREA decodificada. O teto de bytes sozinho não
// protege: um JPEG de poucos KB pode declarar 30000x30000 e estourar a memória
// do processo ao ser decodificado (decompression bomb).
const maxImagePixels = 20_000_000

// storyMediaID é o formato do id público: 128 bits em hex minúsculo.
const storyMediaIDLen = 32

var (
	// ErrMediaNotFound is returned when a media id has expired or never existed.
	ErrMediaNotFound = errors.New("instagram: media not found or expired")
	// ErrMediaTooLarge sinaliza estouro do teto de bytes.
	ErrMediaTooLarge = errors.New("instagram: media too large")
	// ErrMediaInvalid cobre bytes indecodificáveis, dimensões absurdas ou
	// formato que não é JPEG onde JPEG é exigido.
	ErrMediaInvalid = errors.New("instagram: media invalid")
)

// MediaStore holds story image bytes ephemerally in Redis so the Instagram
// Graph API can fetch them from a public URL. O worker faz Put, a rota HTTP
// pública faz Get — processos diferentes, por isso Redis e não cache local.
//
// Só JPEG entra. O endpoint público responde SEMPRE image/jpeg, então guardar
// outro formato serviria bytes com Content-Type mentiroso.
type MediaStore struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewMediaStore builds the store over a shared go-redis client.
func NewMediaStore(rdb *redis.Client, logger *zap.Logger) *MediaStore {
	return &MediaStore{rdb: rdb, logger: logger}
}

func mediaKey(id string) string { return "ig_media:" + id }

// isStoryMediaID valida o formato ANTES de qualquer ida ao Redis: id malformado
// vindo da rota pública não deve nem virar tráfego de rede.
func isStoryMediaID(id string) bool {
	if len(id) != storyMediaIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// NormalizeJPEG valida e converte bytes de imagem num JPEG servível.
//
// Ordem deliberada: teto de bytes -> DecodeConfig (só cabeçalho) -> teto de
// pixels -> Decode. Decodificar antes de conferir as dimensões é exatamente o
// que uma imagem-bomba explora.
func NormalizeJPEG(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, ErrMediaInvalid
	}
	if len(raw) > maxMediaBytes {
		return nil, ErrMediaTooLarge
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrMediaInvalid
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrMediaInvalid
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, ErrMediaInvalid
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrMediaInvalid
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, ErrMediaInvalid
	}
	if buf.Len() > maxMediaBytes {
		return nil, ErrMediaTooLarge
	}
	return buf.Bytes(), nil
}

// Put stores JPEG bytes under a fresh 128-bit random id and returns it. O id é
// inadivinhável, então a rota GET pública dispensa autenticação.
//
// Revalida o conteúdo: um "JPEG" que não decodifica seria servido à Meta e
// falharia lá, com um erro muito mais difícil de rastrear do que aqui.
func (m *MediaStore) Put(ctx context.Context, jpegBytes []byte, ttl time.Duration) (string, error) {
	if m == nil || m.rdb == nil {
		return "", errors.New("instagram: media store not configured")
	}
	if len(jpegBytes) == 0 {
		return "", ErrMediaInvalid
	}
	if len(jpegBytes) > maxMediaBytes {
		return "", ErrMediaTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(jpegBytes))
	if err != nil || format != "jpeg" {
		return "", ErrMediaInvalid
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return "", ErrMediaInvalid
	}

	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes[:])
	if err := m.rdb.Set(ctx, mediaKey(id), jpegBytes, ttl).Err(); err != nil {
		return "", err
	}
	return id, nil
}

// Get returns the JPEG bytes for an id, or ErrMediaNotFound if expired.
func (m *MediaStore) Get(ctx context.Context, id string) ([]byte, error) {
	if m == nil || m.rdb == nil {
		return nil, errors.New("instagram: media store not configured")
	}
	if !isStoryMediaID(id) {
		// Mesmo erro do ausente: distinguir "malformado" de "expirado" contaria
		// a quem sonda o endpoint como são os ids válidos.
		return nil, ErrMediaNotFound
	}
	data, err := m.rdb.Get(ctx, mediaKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrMediaNotFound
		}
		return nil, err
	}
	return data, nil
}

// Delete removes the key (called right after a successful publish).
func (m *MediaStore) Delete(ctx context.Context, id string) {
	if m == nil || m.rdb == nil || !isStoryMediaID(id) {
		return
	}
	if err := m.rdb.Del(ctx, mediaKey(id)).Err(); err != nil {
		// O id não entra no log: é a capability que dá acesso aos bytes.
		m.logger.Warn("failed to delete story media from redis", zap.Error(err))
	}
}
