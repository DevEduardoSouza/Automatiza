package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides a two-tier caching system (local + Redis)
type Cache struct {
	redis      *redis.Client
	localCache *sync.Map
	prefix     string
	disabled   bool
}

type localEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewCache creates a new cache instance
func NewCache(redisClient *redis.Client, prefix string, disabled bool) *Cache {
	return &Cache{
		redis:      redisClient,
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   disabled,
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

// SetNX sets a value only if it doesn't exist (for locks)
func (c *Cache) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	if c.disabled || c.redis == nil {
		return true, nil
	}

	return c.redis.SetNX(ctx, c.fullKey(key), value, ttl).Result()
}

// ClearLocal clears the local cache
func (c *Cache) ClearLocal() {
	c.localCache = &sync.Map{}
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
