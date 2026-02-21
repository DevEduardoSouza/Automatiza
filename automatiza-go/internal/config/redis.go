package config

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the redis client with additional functionality
type RedisClient struct {
	*redis.Client
	prefix string
}

// NewRedisClient creates a new Redis client from configuration
func NewRedisClient(cfg *Config) (*RedisClient, error) {
	opts, err := parseRedisURI(cfg.CacheRedisURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URI: %w", err)
	}

	client := redis.NewClient(opts)

	// OTel: instrument all Redis operations for distributed tracing
	if err := redisotel.InstrumentTracing(client); err != nil {
		// Non-fatal: continue without tracing if instrumentation fails
		fmt.Printf("warning: failed to instrument Redis tracing: %v\n", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{
		Client: client,
		prefix: cfg.AppCachePrefix,
	}, nil
}

// GetPrefix returns the cache prefix
func (r *RedisClient) GetPrefix() string {
	return r.prefix
}

// PrefixedKey returns a key with the configured prefix
func (r *RedisClient) PrefixedKey(key string) string {
	return fmt.Sprintf("%s:%s", r.prefix, key)
}

// parseRedisURI parses a Redis URI into client options
func parseRedisURI(uri string) (*redis.Options, error) {
	if uri == "" {
		uri = "redis://localhost:6379/0"
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	opts := &redis.Options{
		Addr: u.Host,
	}

	// Parse password
	if u.User != nil {
		if password, ok := u.User.Password(); ok {
			opts.Password = password
		}
	}

	// Parse database number from path
	if u.Path != "" && u.Path != "/" {
		dbStr := u.Path[1:] // Remove leading /
		db, err := strconv.Atoi(dbStr)
		if err == nil {
			opts.DB = db
		}
	}

	return opts, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.Client.Close()
}
