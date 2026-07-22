package instagram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// maxMediaBytes caps a single story image at 4MB. The Instagram Story image is
// a 1080x1920 JPEG (well under 1MB rendered, and WhatsApp product photos are
// similar), so 4MB is generous headroom while protecting Redis from an
// oversized blob (the OOM lesson).
const maxMediaBytes = 4 << 20

// ErrMediaNotFound is returned when a media id has expired or never existed.
var ErrMediaNotFound = errors.New("instagram: media not found or expired")

// MediaStore holds story image bytes ephemerally in Redis so the Instagram
// Graph API can fetch them from a public URL. The worker Puts, the public HTTP
// route Gets — different processes, so it must go through Redis (not a local
// cache). Entries carry a short TTL and are deleted right after publish.
type MediaStore struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewMediaStore builds the store over a shared go-redis client.
func NewMediaStore(rdb *redis.Client, logger *zap.Logger) *MediaStore {
	return &MediaStore{rdb: rdb, logger: logger}
}

func mediaKey(id string) string { return "ig_media:" + id }
func mimeKey(id string) string  { return "ig_media:" + id + ":mime" }

// Put stores the bytes under a fresh 128-bit random id and returns it. The id
// is unguessable, so the public GET route needs no auth.
func (m *MediaStore) Put(ctx context.Context, data []byte, mimetype string, ttl time.Duration) (string, error) {
	if m == nil || m.rdb == nil {
		return "", errors.New("instagram: media store not configured")
	}
	if len(data) == 0 {
		return "", errors.New("instagram: empty media")
	}
	if len(data) > maxMediaBytes {
		return "", fmt.Errorf("instagram: media too large (%d bytes, cap %d)", len(data), maxMediaBytes)
	}
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes[:])
	if err := m.rdb.Set(ctx, mediaKey(id), data, ttl).Err(); err != nil {
		return "", err
	}
	if mimetype == "" {
		mimetype = "image/jpeg"
	}
	if err := m.rdb.Set(ctx, mimeKey(id), mimetype, ttl).Err(); err != nil {
		// Bytes já foram gravados; melhor limpar do que deixar órfão sem mime.
		m.rdb.Del(ctx, mediaKey(id))
		return "", err
	}
	return id, nil
}

// Get returns the bytes and mimetype for an id, or ErrMediaNotFound if expired.
func (m *MediaStore) Get(ctx context.Context, id string) ([]byte, string, error) {
	if m == nil || m.rdb == nil {
		return nil, "", errors.New("instagram: media store not configured")
	}
	data, err := m.rdb.Get(ctx, mediaKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, "", ErrMediaNotFound
		}
		return nil, "", err
	}
	mimetype, err := m.rdb.Get(ctx, mimeKey(id)).Result()
	if err != nil {
		mimetype = "image/jpeg" // mime expirou primeiro: JPEG é o único formato que gravamos
	}
	return data, mimetype, nil
}

// Delete removes both keys (called right after a successful publish).
func (m *MediaStore) Delete(ctx context.Context, id string) {
	if m == nil || m.rdb == nil {
		return
	}
	if err := m.rdb.Del(ctx, mediaKey(id), mimeKey(id)).Err(); err != nil {
		m.logger.Warn("failed to delete story media from redis", zap.String("id", id), zap.Error(err))
	}
}
