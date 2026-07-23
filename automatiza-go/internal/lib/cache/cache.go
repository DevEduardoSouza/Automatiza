package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// localCacheSizeWarningThreshold defines the max number of entries in the local cache
// before emitting a Sentry warning (T3.9).
const localCacheSizeWarningThreshold = 10000

// Cache provides a two-tier caching system (local + Redis)
type Cache struct {
	redis      *redis.Client
	localCache *sync.Map
	localCount atomic.Int64 // T3.9: approximate count for monitoring
	prefix     string
	disabled   bool
	logger     *zap.Logger
}

type localEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewCache creates a new cache instance.
// For cache size monitoring, use NewCacheWithLogger instead.
func NewCache(redisClient *redis.Client, prefix string, disabled bool) *Cache {
	return NewCacheWithLogger(redisClient, prefix, disabled, nil, context.Background())
}

// NewCacheWithLogger creates a new cache instance with logger and context for monitoring.
// The context controls the lifetime of the background cleanup/monitoring goroutine (T3.9).
func NewCacheWithLogger(redisClient *redis.Client, prefix string, disabled bool, logger *zap.Logger, ctx context.Context) *Cache {
	c := &Cache{
		redis:      redisClient,
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   disabled,
		logger:     logger,
	}

	// T3.9: Start periodic local cache cleanup and size monitoring
	observability.SafeGo("cache-local-monitor-"+prefix, func() {
		c.localCacheMonitorLoop(ctx)
	})

	return c
}

// localCacheMonitorLoop periodically cleans expired local entries and monitors size (T3.9).
func (c *Cache) localCacheMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var count int64
			now := time.Now()
			c.localCache.Range(func(key, value any) bool {
				entry, ok := value.(*localEntry)
				if ok && now.After(entry.expiresAt) {
					c.localCache.Delete(key)
				} else {
					count++
				}
				return true
			})
			c.localCount.Store(count)

			if count > localCacheSizeWarningThreshold {
				msg := fmt.Sprintf("local cache '%s' exceeds threshold: %d entries (threshold: %d)", c.prefix, count, localCacheSizeWarningThreshold)
				if c.logger != nil {
					observability.LogAndCapture(c.logger, sentry.LevelWarning, msg, nil,
						map[string]string{
							"module": "cache",
							"prefix": c.prefix,
							"count":  fmt.Sprintf("%d", count),
						})
				} else {
					observability.CaptureMessage(sentry.LevelWarning, msg,
						map[string]string{
							"module": "cache",
							"prefix": c.prefix,
							"count":  fmt.Sprintf("%d", count),
						})
				}
			}
		}
	}
}

// fullKey returns the full key with prefix
func (c *Cache) fullKey(key string) string {
	return c.prefix + ":" + key
}

// GetJSON retrieves a JSON value from cache
func (c *Cache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.fullKey(key)

	// 1. Check local cache first
	if val, ok := c.localCache.Load(fullKey); ok {
		entry := val.(*localEntry)
		if time.Now().Before(entry.expiresAt) {
			return json.Unmarshal(entry.data, dest)
		}
		c.localCache.Delete(fullKey)
	}

	// 2. Check Redis
	if c.disabled || c.redis == nil {
		return redis.Nil
	}

	val, err := c.redis.Get(ctx, fullKey).Bytes()
	if err != nil {
		return err
	}

	// Store in local cache (30s TTL for hot keys)
	c.localCache.Store(fullKey, &localEntry{
		data:      val,
		expiresAt: time.Now().Add(30 * time.Second),
	})

	return json.Unmarshal(val, dest)
}

// SetJSON stores a JSON value in cache
func (c *Cache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.disabled || c.redis == nil {
		return nil
	}

	fullKey := c.fullKey(key)

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Store in local cache too
	c.localCache.Store(fullKey, &localEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})

	return c.redis.Set(ctx, fullKey, data, ttl).Err()
}

// Delete removes a key from cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	fullKey := c.fullKey(key)

	c.localCache.Delete(fullKey)

	if c.disabled || c.redis == nil {
		return nil
	}

	return c.redis.Del(ctx, fullKey).Err()
}

// Get retrieves a string value from cache
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	if c.disabled || c.redis == nil {
		return "", redis.Nil
	}

	return c.redis.Get(ctx, c.fullKey(key)).Result()
}

// Set stores a string value in cache
func (c *Cache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if c.disabled || c.redis == nil {
		return nil
	}

	return c.redis.Set(ctx, c.fullKey(key), value, ttl).Err()
}

// Exists checks if a key exists
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if c.disabled || c.redis == nil {
		return false, nil
	}

	result, err := c.redis.Exists(ctx, c.fullKey(key)).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}

// Incr increments a counter
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	if c.disabled || c.redis == nil {
		return 0, nil
	}

	return c.redis.Incr(ctx, c.fullKey(key)).Result()
}

// Expire sets expiration on a key
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c.disabled || c.redis == nil {
		return nil
	}

	return c.redis.Expire(ctx, c.fullKey(key), ttl).Err()
}

// ExpireNX sets expiration on a key only if it has none (Redis >= 7).
// Útil para curar chaves de contador que ficaram órfãs de TTL (INCR e EXPIRE
// não são atômicos entre si).
func (c *Cache) ExpireNX(ctx context.Context, key string, ttl time.Duration) error {
	if c.disabled || c.redis == nil {
		return nil
	}

	return c.redis.ExpireNX(ctx, c.fullKey(key), ttl).Err()
}

// SetNX sets a value only if it doesn't exist (for locks)
func (c *Cache) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	if c.disabled || c.redis == nil {
		return true, nil
	}

	return c.redis.SetNX(ctx, c.fullKey(key), value, ttl).Result()
}

// GetDel lê e apaga a chave ATOMICAMENTE (comando GETDEL, Redis >= 6.2).
//
// Existe porque Get-e-depois-Delete não é single-use: dois callbacks OAuth
// concorrentes podiam ler o mesmo nonce antes de qualquer um apagá-lo e ambos
// seguirem adiante (replay). Diferente de SetNX, NÃO falha aberto: cache
// desligado devolve redis.Nil, porque "não consegui verificar" jamais pode
// valer como "nonce válido".
func (c *Cache) GetDel(ctx context.Context, key string) (string, error) {
	fullKey := c.fullKey(key)
	c.localCache.Delete(fullKey)

	if c.disabled || c.redis == nil {
		return "", redis.Nil
	}
	return c.redis.GetDel(ctx, fullKey).Result()
}

// compareAndDeleteScript apaga a chave somente se o valor for exatamente o
// esperado. É a liberação correta de um lock: um DEL cru apagaria o lock que
// OUTRO worker já readquiriu depois de o nosso ter expirado por TTL.
var compareAndDeleteScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

// CompareAndDelete apaga a chave apenas se ela ainda contiver `expected`.
// Retorna se apagou. Cache desligado retorna false (nada foi liberado).
func (c *Cache) CompareAndDelete(ctx context.Context, key, expected string) (bool, error) {
	fullKey := c.fullKey(key)
	c.localCache.Delete(fullKey)

	if c.disabled || c.redis == nil {
		return false, nil
	}
	n, err := compareAndDeleteScript.Run(ctx, c.redis, []string{fullKey}, expected).Int64()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClearLocal clears the local cache
func (c *Cache) ClearLocal() {
	c.localCache = &sync.Map{}
}

// Operational diz se este cache REALMENTE fala com o Redis.
//
// Existe porque `GetRedisClient() != nil` não basta: com APP_CACHE desligado
// (DisableAppRedisCache) o cliente continua presente, mas SetNX passa a
// devolver `true` sem gravar nada — fail-open. Isso é aceitável para cache,
// e inaceitável para lock ou nonce de uso único, onde "não verifiquei" jamais
// pode valer como "verificado".
func (c *Cache) Operational() bool {
	return c != nil && !c.disabled && c.redis != nil
}

// GetRedisClient returns the underlying Redis client
func (c *Cache) GetRedisClient() *redis.Client {
	return c.redis
}

// Ping checks if the cache (Redis) is accessible
func (c *Cache) Ping(ctx context.Context) error {
	if c.disabled || c.redis == nil {
		return nil
	}
	return c.redis.Ping(ctx).Err()
}
