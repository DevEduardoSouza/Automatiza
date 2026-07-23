package cache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Helper: create a disabled Cache with local cache ready for manipulation.
// ---------------------------------------------------------------------------

func newDisabledCache(prefix string) *Cache {
	return &Cache{
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   true,
	}
}

func newDisabledCacheWithRedis(prefix string) *Cache {
	return &Cache{
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   false,
		redis:      nil, // no real connection, but disabled=false
	}
}

// newNilRedisCache creates a cache with disabled=false, redis=nil, and optionally a logger.
// This exercises the `c.redis == nil` branch (not the `c.disabled` branch).
func newNilRedisCache(prefix string, logger *zap.Logger) *Cache {
	return &Cache{
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   false,
		redis:      nil,
		logger:     logger,
	}
}

// ---------------------------------------------------------------------------
// TestNewCache
// ---------------------------------------------------------------------------

func TestNewCache(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with nil redis client", func(t *testing.T) {
		t.Parallel()
		c := NewCache(nil, "pfx", true)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.prefix != "pfx" {
			t.Errorf("expected prefix 'pfx', got %q", c.prefix)
		}
		if !c.disabled {
			t.Error("expected cache to be disabled")
		}
		if c.redis != nil {
			t.Error("expected nil redis client")
		}
		if c.localCache == nil {
			t.Error("expected non-nil localCache")
		}
	})

	t.Run("disabled cache returns nil for redis ops", func(t *testing.T) {
		t.Parallel()
		c := NewCache(nil, "test", true)
		ctx := context.Background()

		if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
			t.Errorf("Set on disabled cache should return nil, got %v", err)
		}

		val, err := c.Get(ctx, "k")
		if val != "" {
			t.Errorf("Get on disabled cache should return empty string, got %q", val)
		}
		if err != redis.Nil {
			t.Errorf("Get on disabled cache should return redis.Nil, got %v", err)
		}

		if err := c.Ping(ctx); err != nil {
			t.Errorf("Ping on disabled cache should return nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewCacheWithLogger
// ---------------------------------------------------------------------------

func TestNewCacheWithLogger(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with logger and starts monitor goroutine", func(t *testing.T) {
		t.Parallel()
		logger := zap.NewNop()
		ctx, cancel := context.WithCancel(context.Background())

		c := NewCacheWithLogger(nil, "logged", true, logger, ctx)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.logger != logger {
			t.Error("expected logger to be set")
		}
		if c.prefix != "logged" {
			t.Errorf("expected prefix 'logged', got %q", c.prefix)
		}

		// Cancel context to stop the monitor goroutine
		cancel()
		// Allow goroutine to exit
		time.Sleep(50 * time.Millisecond)
	})
}

// ---------------------------------------------------------------------------
// TestFullKey
// ---------------------------------------------------------------------------

func TestFullKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{
			name:     "standard prefix and key",
			prefix:   "prefix",
			key:      "key",
			expected: "prefix:key",
		},
		{
			name:     "empty prefix",
			prefix:   "",
			key:      "key",
			expected: ":key",
		},
		{
			name:     "empty key",
			prefix:   "prefix",
			key:      "",
			expected: "prefix:",
		},
		{
			name:     "both empty",
			prefix:   "",
			key:      "",
			expected: ":",
		},
		{
			name:     "key with colons",
			prefix:   "cache",
			key:      "user:123:profile",
			expected: "cache:user:123:profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Cache{prefix: tt.prefix}
			got := c.fullKey(tt.key)
			if got != tt.expected {
				t.Errorf("fullKey(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetJSON_Disabled
// ---------------------------------------------------------------------------

func TestGetJSON_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns redis.Nil when disabled and not in local cache", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("test")

		var dest string
		err := c.GetJSON(ctx, "missing", &dest)
		if err != redis.Nil {
			t.Errorf("expected redis.Nil, got %v", err)
		}
	})

	t.Run("returns from local cache when entry exists and not expired", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("test")

		data, _ := json.Marshal("hello-world")
		c.localCache.Store("test:mykey", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest string
		err := c.GetJSON(ctx, "mykey", &dest)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if dest != "hello-world" {
			t.Errorf("expected 'hello-world', got %q", dest)
		}
	})

	t.Run("removes expired local entry and returns redis.Nil", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("test")

		data, _ := json.Marshal("stale")
		c.localCache.Store("test:expired", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(-time.Second),
		})

		var dest string
		err := c.GetJSON(ctx, "expired", &dest)
		if err != redis.Nil {
			t.Errorf("expected redis.Nil for expired entry, got %v", err)
		}

		// Verify the entry was removed
		if _, ok := c.localCache.Load("test:expired"); ok {
			t.Error("expected expired entry to be removed from local cache")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetJSON_LocalCacheRoundTrip
// ---------------------------------------------------------------------------

func TestGetJSON_LocalCacheRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("stores and retrieves struct via local cache", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("rt")

		type testPayload struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		original := testPayload{Name: "test", Count: 42}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		c.localCache.Store("rt:payload", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest testPayload
		if err := c.GetJSON(ctx, "payload", &dest); err != nil {
			t.Fatalf("GetJSON failed: %v", err)
		}
		if dest.Name != original.Name || dest.Count != original.Count {
			t.Errorf("round-trip mismatch: got %+v, want %+v", dest, original)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetJSON_Disabled
// ---------------------------------------------------------------------------

func TestSetJSON_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns nil when disabled", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("test")

		err := c.SetJSON(ctx, "key", "value", time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("returns nil when redis is nil (not disabled flag)", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCacheWithRedis("test")

		err := c.SetJSON(ctx, "key", "value", time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDelete
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("removes entry from local cache", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("del")

		data, _ := json.Marshal("to-delete")
		c.localCache.Store("del:item", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		err := c.Delete(ctx, "item")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if _, ok := c.localCache.Load("del:item"); ok {
			t.Error("expected entry to be deleted from local cache")
		}
	})

	t.Run("returns nil when disabled", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("del")

		err := c.Delete(ctx, "nonexistent")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestGet_Disabled
// ---------------------------------------------------------------------------

func TestGet_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	val, err := c.Get(ctx, "any")
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
	if err != redis.Nil {
		t.Errorf("expected redis.Nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestSet_Disabled
// ---------------------------------------------------------------------------

func TestSet_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	err := c.Set(ctx, "key", "val", time.Minute)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestExists_Disabled
// ---------------------------------------------------------------------------

func TestExists_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	exists, err := c.Exists(ctx, "key")
	if exists {
		t.Error("expected false for disabled cache")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestIncr_Disabled
// ---------------------------------------------------------------------------

func TestIncr_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	val, err := c.Incr(ctx, "counter")
	if val != 0 {
		t.Errorf("expected 0, got %d", val)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestExpire_Disabled
// ---------------------------------------------------------------------------

func TestExpire_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	err := c.Expire(ctx, "key", time.Minute)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestSetNX_Disabled
// ---------------------------------------------------------------------------

func TestSetNX_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	ok, err := c.SetNX(ctx, "lock-key", "holder", 30*time.Second)
	if !ok {
		t.Error("expected true (lock acquired) for disabled cache")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClearLocal
// ---------------------------------------------------------------------------

func TestClearLocal(t *testing.T) {
	t.Parallel()

	c := newDisabledCache("clr")

	// Populate local cache
	data, _ := json.Marshal("val1")
	c.localCache.Store("clr:a", &localEntry{data: data, expiresAt: time.Now().Add(time.Hour)})
	c.localCache.Store("clr:b", &localEntry{data: data, expiresAt: time.Now().Add(time.Hour)})

	c.ClearLocal()

	count := 0
	c.localCache.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("expected 0 entries after ClearLocal, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// TestGetRedisClient
// ---------------------------------------------------------------------------

func TestGetRedisClient(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when redis is not set", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("test")
		if c.GetRedisClient() != nil {
			t.Error("expected nil redis client")
		}
	})
}

// ---------------------------------------------------------------------------
// TestPing_Disabled
// ---------------------------------------------------------------------------

func TestPing_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("test")

	err := c.Ping(ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLocalCacheExpiry
// ---------------------------------------------------------------------------

func TestLocalCacheExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("exp")

	// Store an entry that expires in 50ms
	data, _ := json.Marshal("short-lived")
	c.localCache.Store("exp:ttl", &localEntry{
		data:      data,
		expiresAt: time.Now().Add(50 * time.Millisecond),
	})

	// Should still be accessible immediately
	var dest string
	if err := c.GetJSON(ctx, "ttl", &dest); err != nil {
		t.Fatalf("expected value before expiry, got error: %v", err)
	}
	if dest != "short-lived" {
		t.Errorf("expected 'short-lived', got %q", dest)
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	// Now GetJSON should detect expiry and remove it
	var dest2 string
	err := c.GetJSON(ctx, "ttl", &dest2)
	if err != redis.Nil {
		t.Errorf("expected redis.Nil after expiry, got %v", err)
	}

	// Entry should have been removed
	if _, ok := c.localCache.Load("exp:ttl"); ok {
		t.Error("expected expired entry to be removed from local cache")
	}
}

// ---------------------------------------------------------------------------
// TestLocalCacheMonitorLoop
// ---------------------------------------------------------------------------

func TestLocalCacheMonitorLoop(t *testing.T) {
	t.Parallel()

	t.Run("cleans expired entries on manual invocation", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("mon")

		data, _ := json.Marshal("alive")
		expiredData, _ := json.Marshal("dead")

		// Add a valid entry
		c.localCache.Store("mon:alive", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		// Add an expired entry
		c.localCache.Store("mon:dead", &localEntry{
			data:      expiredData,
			expiresAt: time.Now().Add(-time.Second),
		})

		// Simulate what the monitor loop does (Range + delete expired)
		now := time.Now()
		var count int64
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

		// Verify expired entry was removed
		if _, ok := c.localCache.Load("mon:dead"); ok {
			t.Error("expected expired entry 'dead' to be cleaned")
		}

		// Verify valid entry remains
		if _, ok := c.localCache.Load("mon:alive"); !ok {
			t.Error("expected valid entry 'alive' to remain")
		}

		// Verify count
		if c.localCount.Load() != 1 {
			t.Errorf("expected localCount=1, got %d", c.localCount.Load())
		}
	})

	t.Run("context cancellation stops the loop", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("ctx")

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			c.localCacheMonitorLoop(ctx)
			close(done)
		}()

		cancel()

		select {
		case <-done:
			// success: goroutine exited
		case <-time.After(2 * time.Second):
			t.Error("localCacheMonitorLoop did not exit after context cancellation")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetJSON_ComplexTypes
// ---------------------------------------------------------------------------

func TestGetJSON_ComplexTypes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("handles map type", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("complex")

		original := map[string]int{"a": 1, "b": 2}
		data, _ := json.Marshal(original)
		c.localCache.Store("complex:map", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest map[string]int
		if err := c.GetJSON(ctx, "map", &dest); err != nil {
			t.Fatalf("GetJSON failed: %v", err)
		}
		if dest["a"] != 1 || dest["b"] != 2 {
			t.Errorf("map mismatch: got %v", dest)
		}
	})

	t.Run("handles slice type", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("complex")

		original := []string{"x", "y", "z"}
		data, _ := json.Marshal(original)
		c.localCache.Store("complex:slice", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest []string
		if err := c.GetJSON(ctx, "slice", &dest); err != nil {
			t.Fatalf("GetJSON failed: %v", err)
		}
		if len(dest) != 3 || dest[0] != "x" || dest[1] != "y" || dest[2] != "z" {
			t.Errorf("slice mismatch: got %v", dest)
		}
	})

	t.Run("returns unmarshal error for invalid JSON dest type", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("complex")

		data, _ := json.Marshal("not-a-number")
		c.localCache.Store("complex:bad", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest int
		err := c.GetJSON(ctx, "bad", &dest)
		if err == nil {
			t.Error("expected unmarshal error for string->int")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDisabledAndNilRedis_Parity
// ---------------------------------------------------------------------------

func TestDisabledAndNilRedis_Parity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Both disabled=true and redis=nil should behave identically for redis-dependent ops
	caches := map[string]*Cache{
		"disabled_true": newDisabledCache("p"),
		"redis_nil":     newDisabledCacheWithRedis("p"),
	}

	for name, c := range caches {
		t.Run(name+"/Get", func(t *testing.T) {
			t.Parallel()
			val, err := c.Get(ctx, "k")
			if val != "" || err != redis.Nil {
				t.Errorf("Get: val=%q, err=%v", val, err)
			}
		})

		t.Run(name+"/Set", func(t *testing.T) {
			t.Parallel()
			if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
				t.Errorf("Set: %v", err)
			}
		})

		t.Run(name+"/Exists", func(t *testing.T) {
			t.Parallel()
			exists, err := c.Exists(ctx, "k")
			if exists || err != nil {
				t.Errorf("Exists: %v, %v", exists, err)
			}
		})

		t.Run(name+"/Incr", func(t *testing.T) {
			t.Parallel()
			val, err := c.Incr(ctx, "k")
			if val != 0 || err != nil {
				t.Errorf("Incr: %d, %v", val, err)
			}
		})

		t.Run(name+"/Expire", func(t *testing.T) {
			t.Parallel()
			if err := c.Expire(ctx, "k", time.Minute); err != nil {
				t.Errorf("Expire: %v", err)
			}
		})

		t.Run(name+"/SetNX", func(t *testing.T) {
			t.Parallel()
			ok, err := c.SetNX(ctx, "k", "v", time.Minute)
			if !ok || err != nil {
				t.Errorf("SetNX: %v, %v", ok, err)
			}
		})

		t.Run(name+"/Ping", func(t *testing.T) {
			t.Parallel()
			if err := c.Ping(ctx); err != nil {
				t.Errorf("Ping: %v", err)
			}
		})

		t.Run(name+"/SetJSON", func(t *testing.T) {
			t.Parallel()
			if err := c.SetJSON(ctx, "k", "v", time.Minute); err != nil {
				t.Errorf("SetJSON: %v", err)
			}
		})

		t.Run(name+"/Delete", func(t *testing.T) {
			t.Parallel()
			if err := c.Delete(ctx, "k"); err != nil {
				t.Errorf("Delete: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMultipleLocalCacheEntries
// ---------------------------------------------------------------------------

func TestMultipleLocalCacheEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c := newDisabledCache("multi")

	// Add multiple entries with different expiration times
	entries := map[string]struct {
		value   string
		expires time.Duration
	}{
		"key1": {value: "val1", expires: time.Hour},
		"key2": {value: "val2", expires: time.Hour},
		"key3": {value: "val3", expires: -time.Second}, // already expired
	}

	for k, v := range entries {
		data, _ := json.Marshal(v.value)
		c.localCache.Store("multi:"+k, &localEntry{
			data:      data,
			expiresAt: time.Now().Add(v.expires),
		})
	}

	// key1 and key2 should be retrievable
	for _, key := range []string{"key1", "key2"} {
		var dest string
		if err := c.GetJSON(ctx, key, &dest); err != nil {
			t.Errorf("GetJSON(%s) failed: %v", key, err)
		}
	}

	// key3 should be expired
	var dest string
	err := c.GetJSON(ctx, "key3", &dest)
	if err != redis.Nil {
		t.Errorf("expected redis.Nil for expired key3, got %v", err)
	}

	// After Delete, key1 should be gone
	_ = c.Delete(ctx, "key1")
	err = c.GetJSON(ctx, "key1", &dest)
	if err != redis.Nil {
		t.Errorf("expected redis.Nil after deleting key1, got %v", err)
	}

	// ClearLocal removes everything
	c.ClearLocal()
	err = c.GetJSON(ctx, "key2", &dest)
	if err != redis.Nil {
		t.Errorf("expected redis.Nil after ClearLocal, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestLocalCacheMonitorLoop_TickerPath
// ---------------------------------------------------------------------------

func TestLocalCacheMonitorLoop_TickerPath(t *testing.T) {
	t.Parallel()

	t.Run("cleans expired entries when ticker fires", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping long-running monitor loop test in short mode")
		}

		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("ticker-test", logger)

		data, _ := json.Marshal("alive")
		expiredData, _ := json.Marshal("expired")

		// Add a valid entry (expires in 5 minutes)
		c.localCache.Store("ticker-test:alive", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(5 * time.Minute),
		})

		// Add an expired entry
		c.localCache.Store("ticker-test:expired1", &localEntry{
			data:      expiredData,
			expiresAt: time.Now().Add(-10 * time.Second),
		})
		c.localCache.Store("ticker-test:expired2", &localEntry{
			data:      expiredData,
			expiresAt: time.Now().Add(-1 * time.Minute),
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Info("recovered from panic in monitor loop", zap.Any("r", r))
				}
			}()
			c.localCacheMonitorLoop(ctx)
			close(done)
		}()

		// Wait for the ticker to fire (1 minute + buffer)
		time.Sleep(65 * time.Second)

		// After the ticker fires, expired entries should be cleaned
		if _, ok := c.localCache.Load("ticker-test:expired1"); ok {
			t.Error("expected expired1 to be cleaned by monitor loop")
		}
		if _, ok := c.localCache.Load("ticker-test:expired2"); ok {
			t.Error("expected expired2 to be cleaned by monitor loop")
		}

		// Valid entry should still be present
		if _, ok := c.localCache.Load("ticker-test:alive"); !ok {
			t.Error("expected alive entry to remain after monitor loop tick")
		}

		// localCount should be 1 (only the alive entry)
		if count := c.localCount.Load(); count != 1 {
			t.Errorf("expected localCount=1, got %d", count)
		}

		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("localCacheMonitorLoop did not exit after cancel")
		}
	})

	t.Run("emits warning when threshold exceeded with logger", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping long-running monitor loop threshold test in short mode")
		}

		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("threshold-test", logger)

		// Populate more than localCacheSizeWarningThreshold entries (10001)
		data, _ := json.Marshal("v")
		for i := 0; i < localCacheSizeWarningThreshold+1; i++ {
			key := "threshold-test:" + time.Now().Format("150405.000000000") + string(rune(i))
			c.localCache.Store(key, &localEntry{
				data:      data,
				expiresAt: time.Now().Add(10 * time.Minute),
			})
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Info("recovered from panic in monitor threshold test", zap.Any("r", r))
				}
			}()
			c.localCacheMonitorLoop(ctx)
			close(done)
		}()

		// Wait for ticker to fire
		time.Sleep(65 * time.Second)

		// Verify the count was stored
		if count := c.localCount.Load(); count <= localCacheSizeWarningThreshold {
			t.Errorf("expected localCount > %d, got %d", localCacheSizeWarningThreshold, count)
		}

		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("localCacheMonitorLoop did not exit after cancel")
		}
	})

	t.Run("emits warning without logger when threshold exceeded", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping long-running monitor loop no-logger threshold test in short mode")
		}

		// Cache WITHOUT logger to test the else branch (CaptureMessage path)
		c := newNilRedisCache("no-logger-threshold", nil)

		data, _ := json.Marshal("v")
		for i := 0; i < localCacheSizeWarningThreshold+1; i++ {
			key := "no-logger-threshold:" + time.Now().Format("150405.000000000") + string(rune(i))
			c.localCache.Store(key, &localEntry{
				data:      data,
				expiresAt: time.Now().Add(10 * time.Minute),
			})
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					zap.NewNop().Info("recovered from panic in no-logger threshold test", zap.Any("r", r))
				}
			}()
			c.localCacheMonitorLoop(ctx)
			close(done)
		}()

		// Wait for ticker
		time.Sleep(65 * time.Second)

		// Verify the count was stored (all entries are valid)
		if count := c.localCount.Load(); count <= localCacheSizeWarningThreshold {
			t.Errorf("expected localCount > %d, got %d", localCacheSizeWarningThreshold, count)
		}

		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("localCacheMonitorLoop did not exit after cancel")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetJSON_NilRedis
// ---------------------------------------------------------------------------

func TestGetJSON_NilRedis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns redis.Nil when no local entry and redis is nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("getjson-nil", logger)

		var dest string
		err := c.GetJSON(ctx, "nonexistent", &dest)
		if err != redis.Nil {
			t.Errorf("expected redis.Nil, got %v", err)
		}
	})

	t.Run("returns local cache hit even when redis is nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("getjson-nil", logger)

		data, _ := json.Marshal(map[string]string{"key": "value"})
		c.localCache.Store("getjson-nil:mapkey", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest map[string]string
		if err := c.GetJSON(ctx, "mapkey", &dest); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if dest["key"] != "value" {
			t.Errorf("expected map with key=value, got %v", dest)
		}
	})

	t.Run("returns unmarshal error for corrupt data in local cache", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("getjson-nil", logger)

		// Store raw bytes that are not valid JSON
		c.localCache.Store("getjson-nil:corrupt", &localEntry{
			data:      []byte("{invalid json!!!"),
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest map[string]string
		err := c.GetJSON(ctx, "corrupt", &dest)
		if err == nil {
			t.Error("expected unmarshal error for corrupt JSON, got nil")
		}
	})

	t.Run("evicts expired local entry then returns redis.Nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("getjson-nil", logger)

		data, _ := json.Marshal("stale-value")
		c.localCache.Store("getjson-nil:stale", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(-5 * time.Second),
		})

		var dest string
		err := c.GetJSON(ctx, "stale", &dest)
		if err != redis.Nil {
			t.Errorf("expected redis.Nil for expired entry with nil redis, got %v", err)
		}

		// Confirm entry was removed
		if _, ok := c.localCache.Load("getjson-nil:stale"); ok {
			t.Error("expected expired entry to be removed from local cache")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetJSON_NilRedis
// ---------------------------------------------------------------------------

func TestSetJSON_NilRedis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns nil without storing locally when redis is nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("setjson-nil", logger)

		err := c.SetJSON(ctx, "key1", map[string]int{"count": 42}, time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}

		// SetJSON returns early when redis==nil, so local cache is NOT populated
		if _, ok := c.localCache.Load("setjson-nil:key1"); ok {
			t.Error("expected local cache to NOT be populated when redis is nil (early return)")
		}
	})

	t.Run("returns nil for disabled cache", func(t *testing.T) {
		t.Parallel()
		c := newDisabledCache("setjson-disabled")

		err := c.SetJSON(ctx, "key1", "value", time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDelete_NilRedis
// ---------------------------------------------------------------------------

func TestDelete_NilRedis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("removes local entry and returns nil when redis is nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("del-nil", logger)

		data, _ := json.Marshal("to-be-deleted")
		c.localCache.Store("del-nil:target", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		err := c.Delete(ctx, "target")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if _, ok := c.localCache.Load("del-nil:target"); ok {
			t.Error("expected local cache entry to be removed")
		}
	})

	t.Run("returns nil for nonexistent key when redis is nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("del-nil", nil)

		err := c.Delete(ctx, "does-not-exist")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNilRedisOperations (table-driven)
// ---------------------------------------------------------------------------

func TestNilRedisOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Get returns empty and redis.Nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		val, err := c.Get(ctx, "key")
		if val != "" {
			t.Errorf("expected empty string, got %q", val)
		}
		if err != redis.Nil {
			t.Errorf("expected redis.Nil, got %v", err)
		}
	})

	t.Run("Set returns nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		if err := c.Set(ctx, "key", "val", time.Minute); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("Exists returns false, nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		exists, err := c.Exists(ctx, "key")
		if exists {
			t.Error("expected false")
		}
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("Incr returns 0, nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		val, err := c.Incr(ctx, "counter")
		if val != 0 {
			t.Errorf("expected 0, got %d", val)
		}
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("Expire returns nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		if err := c.Expire(ctx, "key", time.Minute); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("SetNX returns true, nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		ok, err := c.SetNX(ctx, "lock", "holder", time.Minute)
		if !ok {
			t.Error("expected true (lock acquired)")
		}
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("Ping returns nil", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("ops", nil)
		if err := c.Ping(ctx); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewCacheWithLogger_NilLogger
// ---------------------------------------------------------------------------

func TestNewCacheWithLogger_NilLogger(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with nil logger", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())

		c := NewCacheWithLogger(nil, "no-log", false, nil, ctx)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.logger != nil {
			t.Error("expected nil logger")
		}
		if c.prefix != "no-log" {
			t.Errorf("expected prefix 'no-log', got %q", c.prefix)
		}

		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("creates cache with zaptest logger", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		ctx, cancel := context.WithCancel(context.Background())

		c := NewCacheWithLogger(nil, "with-log", false, logger, ctx)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.logger != logger {
			t.Error("expected logger to be set")
		}

		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

// ---------------------------------------------------------------------------
// TestConcurrentLocalCacheAccess
// ---------------------------------------------------------------------------

func TestConcurrentLocalCacheAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("concurrent GetJSON reads are safe", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("concurrent", logger)

		// Pre-populate entries
		for i := 0; i < 100; i++ {
			key := "concurrent:" + string(rune('a'+i%26)) + string(rune('0'+i%10))
			data, _ := json.Marshal(i)
			c.localCache.Store(key, &localEntry{
				data:      data,
				expiresAt: time.Now().Add(time.Hour),
			})
		}

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer func() {
					if r := recover(); r != nil {
						logger.Info("recovered from panic in concurrent read", zap.Any("r", r))
					}
				}()
				defer wg.Done()
				key := string(rune('a'+idx%26)) + string(rune('0'+idx%10))
				var dest int
				_ = c.GetJSON(ctx, key, &dest)
			}(i)
		}
		wg.Wait()
	})

	t.Run("concurrent Delete and GetJSON are safe", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c := newNilRedisCache("concurrent-del", logger)

		// Pre-populate
		for i := 0; i < 50; i++ {
			key := "concurrent-del:k" + string(rune('a'+i%26))
			data, _ := json.Marshal(i)
			c.localCache.Store(key, &localEntry{
				data:      data,
				expiresAt: time.Now().Add(time.Hour),
			})
		}

		var wg sync.WaitGroup

		// Readers
		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(idx int) {
				defer func() {
					if r := recover(); r != nil {
						logger.Info("recovered from panic in concurrent read-del", zap.Any("r", r))
					}
				}()
				defer wg.Done()
				key := "k" + string(rune('a'+idx%26))
				var dest int
				_ = c.GetJSON(ctx, key, &dest)
			}(i)
		}

		// Deleters
		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(idx int) {
				defer func() {
					if r := recover(); r != nil {
						logger.Info("recovered from panic in concurrent delete", zap.Any("r", r))
					}
				}()
				defer wg.Done()
				key := "k" + string(rune('a'+idx%26))
				_ = c.Delete(ctx, key)
			}(i)
		}

		wg.Wait()
	})
}

// ---------------------------------------------------------------------------
// TestGetJSON_EdgeCases
// ---------------------------------------------------------------------------

func TestGetJSON_EdgeCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("handles null JSON value in local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("edge", nil)

		c.localCache.Store("edge:null", &localEntry{
			data:      []byte("null"),
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest *string
		err := c.GetJSON(ctx, "null", &dest)
		if err != nil {
			t.Errorf("expected nil error for JSON null, got %v", err)
		}
		if dest != nil {
			t.Errorf("expected nil pointer for JSON null, got %v", dest)
		}
	})

	t.Run("handles empty object JSON in local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("edge", nil)

		c.localCache.Store("edge:empty-obj", &localEntry{
			data:      []byte("{}"),
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest map[string]string
		err := c.GetJSON(ctx, "empty-obj", &dest)
		if err != nil {
			t.Errorf("expected nil error for empty object, got %v", err)
		}
		if len(dest) != 0 {
			t.Errorf("expected empty map, got %v", dest)
		}
	})

	t.Run("handles nested struct in local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("edge", nil)

		type Inner struct {
			Value int `json:"value"`
		}
		type Outer struct {
			Name  string `json:"name"`
			Inner Inner  `json:"inner"`
		}

		original := Outer{Name: "test", Inner: Inner{Value: 99}}
		data, _ := json.Marshal(original)
		c.localCache.Store("edge:nested", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest Outer
		err := c.GetJSON(ctx, "nested", &dest)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if dest.Name != "test" || dest.Inner.Value != 99 {
			t.Errorf("nested struct mismatch: got %+v", dest)
		}
	})

	t.Run("handles boolean JSON in local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("edge", nil)

		data, _ := json.Marshal(true)
		c.localCache.Store("edge:bool", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest bool
		err := c.GetJSON(ctx, "bool", &dest)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if !dest {
			t.Error("expected true, got false")
		}
	})

	t.Run("handles large number in local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("edge", nil)

		data, _ := json.Marshal(int64(9223372036854775807))
		c.localCache.Store("edge:bignum", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest int64
		err := c.GetJSON(ctx, "bignum", &dest)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if dest != 9223372036854775807 {
			t.Errorf("expected max int64, got %d", dest)
		}
	})
}

// ---------------------------------------------------------------------------
// TestLocalCount
// ---------------------------------------------------------------------------

func TestLocalCount(t *testing.T) {
	t.Parallel()

	t.Run("localCount starts at zero", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("count", nil)
		if count := c.localCount.Load(); count != 0 {
			t.Errorf("expected localCount=0, got %d", count)
		}
	})

	t.Run("localCount can be manually incremented and read", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("count", nil)
		c.localCount.Store(42)
		if count := c.localCount.Load(); count != 42 {
			t.Errorf("expected localCount=42, got %d", count)
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetRedisClient_NilRedis
// ---------------------------------------------------------------------------

func TestGetRedisClient_NilRedis(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for nil-redis cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("redis-client", nil)
		if c.GetRedisClient() != nil {
			t.Error("expected nil redis client")
		}
	})
}

// ---------------------------------------------------------------------------
// TestClearLocal_NilRedis
// ---------------------------------------------------------------------------

func TestClearLocal_NilRedis(t *testing.T) {
	t.Parallel()

	t.Run("clears all entries from local cache", func(t *testing.T) {
		t.Parallel()
		c := newNilRedisCache("clear-nil", nil)

		// Populate
		data, _ := json.Marshal("val")
		for i := 0; i < 10; i++ {
			key := "clear-nil:" + string(rune('a'+i))
			c.localCache.Store(key, &localEntry{
				data:      data,
				expiresAt: time.Now().Add(time.Hour),
			})
		}

		c.ClearLocal()

		count := 0
		c.localCache.Range(func(_, _ any) bool {
			count++
			return true
		})
		if count != 0 {
			t.Errorf("expected 0 entries after ClearLocal, got %d", count)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewCache_NilRedisNotDisabled
// ---------------------------------------------------------------------------

func TestNewCache_NilRedisNotDisabled(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with redis=nil and disabled=false", func(t *testing.T) {
		t.Parallel()
		c := NewCache(nil, "nrd", false)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.disabled {
			t.Error("expected disabled=false")
		}
		if c.redis != nil {
			t.Error("expected nil redis client")
		}
		if c.localCache == nil {
			t.Error("expected non-nil localCache")
		}
	})

	t.Run("all operations work with redis=nil disabled=false", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		c := NewCache(nil, "nrd-ops", false)

		// Get
		val, err := c.Get(ctx, "k")
		if val != "" || err != redis.Nil {
			t.Errorf("Get: val=%q err=%v", val, err)
		}

		// Set
		if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
			t.Errorf("Set: %v", err)
		}

		// SetJSON
		if err := c.SetJSON(ctx, "k", "v", time.Minute); err != nil {
			t.Errorf("SetJSON: %v", err)
		}

		// GetJSON
		var dest string
		if err := c.GetJSON(ctx, "missing", &dest); err != redis.Nil {
			t.Errorf("GetJSON: %v", err)
		}

		// Delete
		if err := c.Delete(ctx, "k"); err != nil {
			t.Errorf("Delete: %v", err)
		}

		// Exists
		exists, err := c.Exists(ctx, "k")
		if exists || err != nil {
			t.Errorf("Exists: %v %v", exists, err)
		}

		// Incr
		ival, err := c.Incr(ctx, "k")
		if ival != 0 || err != nil {
			t.Errorf("Incr: %d %v", ival, err)
		}

		// Expire
		if err := c.Expire(ctx, "k", time.Minute); err != nil {
			t.Errorf("Expire: %v", err)
		}

		// SetNX
		ok, err := c.SetNX(ctx, "k", "v", time.Minute)
		if !ok || err != nil {
			t.Errorf("SetNX: %v %v", ok, err)
		}

		// Ping
		if err := c.Ping(ctx); err != nil {
			t.Errorf("Ping: %v", err)
		}
	})
}

// ===========================================================================
// Miniredis-backed tests: cover ALL Redis code paths with a real connection
// ===========================================================================

// newMiniredisCache creates a Cache backed by miniredis for integration testing.
// The miniredis server is automatically closed when the test finishes.
func newMiniredisCache(t *testing.T, prefix string) *Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })
	return &Cache{
		localCache: &sync.Map{},
		prefix:     prefix,
		disabled:   false,
		redis:      rc,
	}
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_HappyPath
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("fetches value from Redis and caches locally", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-getjson")

		type payload struct {
			Name  string `json:"name"`
			Score int    `json:"score"`
		}

		// Pre-populate Redis directly
		original := payload{Name: "alice", Score: 100}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := c.redis.Set(ctx, "mr-getjson:user1", data, time.Minute).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		// GetJSON should fetch from Redis
		var dest payload
		if err := c.GetJSON(ctx, "user1", &dest); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if dest.Name != "alice" || dest.Score != 100 {
			t.Errorf("unexpected result: %+v", dest)
		}

		// Verify it was cached locally
		val, ok := c.localCache.Load("mr-getjson:user1")
		if !ok {
			t.Fatal("expected value to be stored in local cache after Redis fetch")
		}
		entry := val.(*localEntry)
		if time.Now().After(entry.expiresAt) {
			t.Error("local cache entry should not be expired yet")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_Miss
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_Miss(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns redis.Nil when key does not exist in Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-miss")

		var dest string
		err := c.GetJSON(ctx, "nonexistent", &dest)
		if err != redis.Nil {
			t.Errorf("expected redis.Nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_LocalCacheHit
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_LocalCacheHit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns from local cache without hitting Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-local")

		// Put value in local cache only (NOT in Redis)
		data, _ := json.Marshal("from-local")
		c.localCache.Store("mr-local:key1", &localEntry{
			data:      data,
			expiresAt: time.Now().Add(time.Hour),
		})

		var dest string
		if err := c.GetJSON(ctx, "key1", &dest); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if dest != "from-local" {
			t.Errorf("expected 'from-local', got %q", dest)
		}

		// Verify Redis does NOT have this key
		_, err := c.redis.Get(ctx, "mr-local:key1").Result()
		if err != redis.Nil {
			t.Errorf("expected key to not exist in Redis, got err=%v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_LocalCacheExpired
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_LocalCacheExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("expired local entry falls through to Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-expired")

		// Put stale value in local cache
		staleData, _ := json.Marshal("stale")
		c.localCache.Store("mr-expired:key1", &localEntry{
			data:      staleData,
			expiresAt: time.Now().Add(-time.Second),
		})

		// Put fresh value in Redis
		freshData, _ := json.Marshal("fresh")
		if err := c.redis.Set(ctx, "mr-expired:key1", freshData, time.Minute).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		var dest string
		if err := c.GetJSON(ctx, "key1", &dest); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if dest != "fresh" {
			t.Errorf("expected 'fresh' from Redis, got %q", dest)
		}

		// Verify old local entry was evicted and replaced with fresh data
		val, ok := c.localCache.Load("mr-expired:key1")
		if !ok {
			t.Fatal("expected fresh value in local cache after Redis fetch")
		}
		entry := val.(*localEntry)
		var localDest string
		if err := json.Unmarshal(entry.data, &localDest); err != nil {
			t.Fatalf("unmarshal local: %v", err)
		}
		if localDest != "fresh" {
			t.Errorf("expected local cache to contain 'fresh', got %q", localDest)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_BadJSON
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_BadJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns unmarshal error when Redis value is invalid JSON", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-badjson")

		// Store invalid JSON directly in Redis
		if err := c.redis.Set(ctx, "mr-badjson:bad", "{not valid json!!!", time.Minute).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		var dest map[string]string
		err := c.GetJSON(ctx, "bad", &dest)
		if err == nil {
			t.Fatal("expected unmarshal error, got nil")
		}

		// The value should still have been cached locally (bad data included)
		if _, ok := c.localCache.Load("mr-badjson:bad"); !ok {
			t.Error("expected raw bytes to be stored in local cache even if unmarshal fails")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_SetJSON_HappyPath
// ---------------------------------------------------------------------------

func TestMiniredis_SetJSON_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("stores value in both local cache and Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-setjson")

		type item struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		val := item{ID: 42, Name: "widget"}
		if err := c.SetJSON(ctx, "item1", val, 5*time.Minute); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}

		// Verify Redis has the value
		raw, err := c.redis.Get(ctx, "mr-setjson:item1").Bytes()
		if err != nil {
			t.Fatalf("redis get: %v", err)
		}
		var fromRedis item
		if err := json.Unmarshal(raw, &fromRedis); err != nil {
			t.Fatalf("unmarshal redis: %v", err)
		}
		if fromRedis.ID != 42 || fromRedis.Name != "widget" {
			t.Errorf("redis value mismatch: %+v", fromRedis)
		}

		// Verify local cache has the value
		localVal, ok := c.localCache.Load("mr-setjson:item1")
		if !ok {
			t.Fatal("expected local cache to contain the value")
		}
		entry := localVal.(*localEntry)
		var fromLocal item
		if err := json.Unmarshal(entry.data, &fromLocal); err != nil {
			t.Fatalf("unmarshal local: %v", err)
		}
		if fromLocal.ID != 42 || fromLocal.Name != "widget" {
			t.Errorf("local cache value mismatch: %+v", fromLocal)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_SetJSON_MarshalError
// ---------------------------------------------------------------------------

func TestMiniredis_SetJSON_MarshalError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns error when value cannot be marshaled", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-marshal-err")

		// Channels cannot be marshaled to JSON
		ch := make(chan int)
		err := c.SetJSON(ctx, "bad", ch, time.Minute)
		if err == nil {
			t.Fatal("expected marshal error for channel type, got nil")
		}

		// Verify nothing was stored in Redis
		_, redisErr := c.redis.Get(ctx, "mr-marshal-err:bad").Result()
		if redisErr != redis.Nil {
			t.Errorf("expected key to not exist in Redis, got err=%v", redisErr)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Delete
// ---------------------------------------------------------------------------

func TestMiniredis_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("removes from both local cache and Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-del")

		// Set a value via SetJSON (populates both caches)
		if err := c.SetJSON(ctx, "to-delete", "goodbye", time.Minute); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}

		// Verify it exists in both
		if _, ok := c.localCache.Load("mr-del:to-delete"); !ok {
			t.Fatal("expected value in local cache before delete")
		}
		if _, err := c.redis.Get(ctx, "mr-del:to-delete").Result(); err != nil {
			t.Fatalf("expected value in Redis before delete: %v", err)
		}

		// Delete
		if err := c.Delete(ctx, "to-delete"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Verify removed from local cache
		if _, ok := c.localCache.Load("mr-del:to-delete"); ok {
			t.Error("expected local cache entry to be removed after Delete")
		}

		// Verify removed from Redis
		_, err := c.redis.Get(ctx, "mr-del:to-delete").Result()
		if err != redis.Nil {
			t.Errorf("expected redis.Nil after Delete, got err=%v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Get_HappyPath
// ---------------------------------------------------------------------------

func TestMiniredis_Get_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("retrieves string value from Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-get")

		// Store directly in Redis
		if err := c.redis.Set(ctx, "mr-get:greeting", "hello", time.Minute).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		val, err := c.Get(ctx, "greeting")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if val != "hello" {
			t.Errorf("expected 'hello', got %q", val)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Get_Miss
// ---------------------------------------------------------------------------

func TestMiniredis_Get_Miss(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns redis.Nil for missing key", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-get-miss")

		val, err := c.Get(ctx, "absent")
		if val != "" {
			t.Errorf("expected empty string, got %q", val)
		}
		if err != redis.Nil {
			t.Errorf("expected redis.Nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Set_HappyPath
// ---------------------------------------------------------------------------

func TestMiniredis_Set_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("stores string value in Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-set")

		if err := c.Set(ctx, "color", "blue", 5*time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}

		// Verify directly from Redis
		val, err := c.redis.Get(ctx, "mr-set:color").Result()
		if err != nil {
			t.Fatalf("redis get: %v", err)
		}
		if val != "blue" {
			t.Errorf("expected 'blue', got %q", val)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Exists
// ---------------------------------------------------------------------------

func TestMiniredis_Exists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns true for existing key", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-exists")

		if err := c.redis.Set(ctx, "mr-exists:present", "yes", time.Minute).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		exists, err := c.Exists(ctx, "present")
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if !exists {
			t.Error("expected true for existing key")
		}
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-exists-miss")

		exists, err := c.Exists(ctx, "absent")
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists {
			t.Error("expected false for missing key")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Incr
// ---------------------------------------------------------------------------

func TestMiniredis_Incr(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("increments counter correctly", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-incr")

		// First increment creates the key with value 1
		val1, err := c.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr 1: %v", err)
		}
		if val1 != 1 {
			t.Errorf("expected 1 after first Incr, got %d", val1)
		}

		// Second increment
		val2, err := c.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr 2: %v", err)
		}
		if val2 != 2 {
			t.Errorf("expected 2 after second Incr, got %d", val2)
		}

		// Third increment
		val3, err := c.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr 3: %v", err)
		}
		if val3 != 3 {
			t.Errorf("expected 3 after third Incr, got %d", val3)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Expire
// ---------------------------------------------------------------------------

func TestMiniredis_Expire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("sets TTL on existing key", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-expire")

		// Create key without TTL
		if err := c.redis.Set(ctx, "mr-expire:temp", "data", 0).Err(); err != nil {
			t.Fatalf("redis set: %v", err)
		}

		// Set TTL via Expire
		if err := c.Expire(ctx, "temp", 30*time.Second); err != nil {
			t.Fatalf("Expire: %v", err)
		}

		// Verify TTL was set (miniredis supports TTL check)
		ttl, err := c.redis.TTL(ctx, "mr-expire:temp").Result()
		if err != nil {
			t.Fatalf("TTL: %v", err)
		}
		if ttl <= 0 {
			t.Errorf("expected positive TTL, got %v", ttl)
		}
		if ttl > 30*time.Second {
			t.Errorf("expected TTL <= 30s, got %v", ttl)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_SetNX
// ---------------------------------------------------------------------------

func TestMiniredis_SetNX(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("first call acquires lock, second is rejected", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-setnx")

		// First SetNX should succeed
		ok1, err := c.SetNX(ctx, "lock", "holder-1", time.Minute)
		if err != nil {
			t.Fatalf("SetNX 1: %v", err)
		}
		if !ok1 {
			t.Error("expected first SetNX to return true (lock acquired)")
		}

		// Second SetNX should fail (key already exists)
		ok2, err := c.SetNX(ctx, "lock", "holder-2", time.Minute)
		if err != nil {
			t.Fatalf("SetNX 2: %v", err)
		}
		if ok2 {
			t.Error("expected second SetNX to return false (lock already held)")
		}

		// Verify the original holder is still set
		val, err := c.redis.Get(ctx, "mr-setnx:lock").Result()
		if err != nil {
			t.Fatalf("redis get: %v", err)
		}
		if val != "holder-1" {
			t.Errorf("expected 'holder-1', got %q", val)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Ping
// ---------------------------------------------------------------------------

func TestMiniredis_Ping(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("succeeds with active miniredis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-ping")

		if err := c.Ping(ctx); err != nil {
			t.Errorf("Ping: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_NewCacheWithLogger
// ---------------------------------------------------------------------------

func TestMiniredis_NewCacheWithLogger(t *testing.T) {
	t.Parallel()

	t.Run("constructor works with real redis client", func(t *testing.T) {
		t.Parallel()
		mr := miniredis.RunT(t)
		rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { rc.Close() })

		logger := zaptest.NewLogger(t)
		ctx, cancel := context.WithCancel(context.Background())

		c := NewCacheWithLogger(rc, "mr-logger", false, logger, ctx)
		if c == nil {
			t.Fatal("expected non-nil cache")
		}
		if c.redis != rc {
			t.Error("expected redis client to be set")
		}
		if c.logger != logger {
			t.Error("expected logger to be set")
		}
		if c.disabled {
			t.Error("expected disabled=false")
		}
		if c.prefix != "mr-logger" {
			t.Errorf("expected prefix 'mr-logger', got %q", c.prefix)
		}

		// Verify Ping works through the real client
		if err := c.Ping(context.Background()); err != nil {
			t.Errorf("Ping through NewCacheWithLogger: %v", err)
		}

		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetRedisClient
// ---------------------------------------------------------------------------

func TestMiniredis_GetRedisClient(t *testing.T) {
	t.Parallel()

	t.Run("returns the underlying redis client", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-client")

		client := c.GetRedisClient()
		if client == nil {
			t.Fatal("expected non-nil redis client")
		}
		if client != c.redis {
			t.Error("expected GetRedisClient to return the same client")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_GetJSON_RoundTrip
// ---------------------------------------------------------------------------

func TestMiniredis_GetJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("SetJSON then GetJSON returns same value", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-rt")

		type user struct {
			ID    int      `json:"id"`
			Email string   `json:"email"`
			Tags  []string `json:"tags"`
		}

		original := user{
			ID:    1,
			Email: "test@example.com",
			Tags:  []string{"admin", "active"},
		}

		if err := c.SetJSON(ctx, "user:1", original, 5*time.Minute); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}

		// Clear local cache to force Redis fetch
		c.ClearLocal()

		var dest user
		if err := c.GetJSON(ctx, "user:1", &dest); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if dest.ID != original.ID || dest.Email != original.Email {
			t.Errorf("round-trip mismatch: got %+v, want %+v", dest, original)
		}
		if len(dest.Tags) != 2 || dest.Tags[0] != "admin" || dest.Tags[1] != "active" {
			t.Errorf("tags mismatch: got %v", dest.Tags)
		}
	})

	t.Run("second GetJSON serves from local cache", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-rt2")

		if err := c.SetJSON(ctx, "item", "value-from-redis", 5*time.Minute); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}

		// Clear local cache to force first GetJSON from Redis
		c.ClearLocal()

		var dest1 string
		if err := c.GetJSON(ctx, "item", &dest1); err != nil {
			t.Fatalf("GetJSON 1: %v", err)
		}
		if dest1 != "value-from-redis" {
			t.Errorf("expected 'value-from-redis', got %q", dest1)
		}

		// Delete from Redis; second GetJSON should still work via local cache
		c.redis.Del(ctx, "mr-rt2:item")

		var dest2 string
		if err := c.GetJSON(ctx, "item", &dest2); err != nil {
			t.Fatalf("GetJSON 2 (from local): %v", err)
		}
		if dest2 != "value-from-redis" {
			t.Errorf("expected 'value-from-redis' from local cache, got %q", dest2)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_ClearLocal_ThenFetchFromRedis
// ---------------------------------------------------------------------------

func TestMiniredis_ClearLocal_ThenFetchFromRedis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ClearLocal forces next GetJSON to hit Redis", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-clear")

		if err := c.SetJSON(ctx, "key", "original", 5*time.Minute); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}

		// Verify local cache has the value
		if _, ok := c.localCache.Load("mr-clear:key"); !ok {
			t.Fatal("expected local cache to have value after SetJSON")
		}

		c.ClearLocal()

		// Verify local cache is empty
		if _, ok := c.localCache.Load("mr-clear:key"); ok {
			t.Fatal("expected local cache to be empty after ClearLocal")
		}

		// GetJSON should fetch from Redis
		var dest string
		if err := c.GetJSON(ctx, "key", &dest); err != nil {
			t.Fatalf("GetJSON after ClearLocal: %v", err)
		}
		if dest != "original" {
			t.Errorf("expected 'original', got %q", dest)
		}

		// Verify local cache is repopulated
		if _, ok := c.localCache.Load("mr-clear:key"); !ok {
			t.Error("expected local cache to be repopulated after GetJSON from Redis")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Delete_NonexistentKey
// ---------------------------------------------------------------------------

func TestMiniredis_Delete_NonexistentKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("deleting a nonexistent key returns nil error", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-del-absent")

		err := c.Delete(ctx, "does-not-exist")
		if err != nil {
			t.Errorf("expected nil error for deleting nonexistent key, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Incr_CreatesThenIncrements
// ---------------------------------------------------------------------------

func TestMiniredis_Incr_CreatesThenIncrements(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("creates key if not present and increments", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-incr-new")

		// Key does not exist; Incr should create it with value 1
		val, err := c.Incr(ctx, "views")
		if err != nil {
			t.Fatalf("Incr: %v", err)
		}
		if val != 1 {
			t.Errorf("expected 1, got %d", val)
		}

		// Verify in Redis directly
		raw, err := c.redis.Get(ctx, "mr-incr-new:views").Result()
		if err != nil {
			t.Fatalf("redis get: %v", err)
		}
		if raw != "1" {
			t.Errorf("expected '1' in Redis, got %q", raw)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMiniredis_Set_OverwriteExisting
// ---------------------------------------------------------------------------

func TestMiniredis_Set_OverwriteExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("overwrites existing key", func(t *testing.T) {
		t.Parallel()
		c := newMiniredisCache(t, "mr-overwrite")

		if err := c.Set(ctx, "key", "first", time.Minute); err != nil {
			t.Fatalf("Set 1: %v", err)
		}
		if err := c.Set(ctx, "key", "second", time.Minute); err != nil {
			t.Fatalf("Set 2: %v", err)
		}

		val, err := c.Get(ctx, "key")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if val != "second" {
			t.Errorf("expected 'second', got %q", val)
		}
	})
}

// ---------------------------------------------------------------------------
// GetDel / CompareAndDelete — primitivas de uso único e de lock.
// ---------------------------------------------------------------------------

func newRedisBackedCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewCache(rdb, "test", false), mr
}

func TestGetDel_ReadsAndRemovesAtomically(t *testing.T) {
	c, _ := newRedisBackedCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "nonce", "user-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetDel(ctx, "nonce")
	if err != nil {
		t.Fatalf("GetDel: %v", err)
	}
	if got != "user-1" {
		t.Errorf("valor = %q, want user-1", got)
	}
	if _, err := c.GetDel(ctx, "nonce"); err == nil {
		t.Error("segunda leitura deveria falhar: a chave foi consumida")
	}
}

// A propriedade que importa: sob concorrência, exatamente UM chamador recebe o
// valor. É isto que impede o replay do callback OAuth.
func TestGetDel_ExactlyOneWinnerUnderConcurrency(t *testing.T) {
	c, _ := newRedisBackedCache(t)
	ctx := context.Background()
	if err := c.Set(ctx, "nonce", "user-1", time.Minute); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	winners := 0
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := c.GetDel(ctx, "nonce"); err == nil && v == "user-1" {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if winners != 1 {
		t.Fatalf("exatamente 1 chamador deveria consumir o nonce, got %d", winners)
	}
}

// Cache desligado NÃO pode devolver sucesso: "não consegui verificar" jamais
// vale como "nonce válido".
func TestGetDel_DisabledCacheIsNotSuccess(t *testing.T) {
	c := newDisabledCache("test")
	if _, err := c.GetDel(context.Background(), "nonce"); err == nil {
		t.Fatal("cache desligado deveria falhar, não devolver sucesso vazio")
	}
}

func TestCompareAndDelete_OnlyDeletesMatchingValue(t *testing.T) {
	c, mr := newRedisBackedCache(t)
	ctx := context.Background()
	if err := c.Set(ctx, "lock", "token-A", time.Minute); err != nil {
		t.Fatal(err)
	}

	// Token errado não apaga — este é o caso do lock que OUTRO worker já
	// readquiriu depois de o nosso expirar.
	deleted, err := c.CompareAndDelete(ctx, "lock", "token-B")
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("não deveria apagar lock de outro dono")
	}
	if v, _ := mr.Get("test:lock"); v != "token-A" {
		t.Fatalf("valor foi alterado: %q", v)
	}

	// Token certo apaga.
	deleted, err = c.CompareAndDelete(ctx, "lock", "token-A")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("deveria apagar o próprio lock")
	}
	if mr.Exists("test:lock") {
		t.Error("chave deveria ter sido removida")
	}
}

func TestCompareAndDelete_MissingKeyIsNoop(t *testing.T) {
	c, _ := newRedisBackedCache(t)
	deleted, err := c.CompareAndDelete(context.Background(), "ausente", "qualquer")
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Error("chave inexistente não deveria reportar deleção")
	}
}

func TestCompareAndDelete_DisabledCacheReportsNoDeletion(t *testing.T) {
	c := newDisabledCache("test")
	deleted, err := c.CompareAndDelete(context.Background(), "lock", "token")
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Error("cache desligado não apagou nada e não pode dizer que apagou")
	}
}
