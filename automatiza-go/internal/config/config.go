package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	// PostgreSQL Direct Connection
	// Supports both POSTGRES_* and DB_* prefixes for flexibility
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	DBSSLMode  string `mapstructure:"DB_SSLMODE"`
	DBMaxConns int    `mapstructure:"DB_MAX_CONNS"`

	// SMTP Email Configuration
	SMTPHost      string `mapstructure:"SMTP_HOST"`
	SMTPPort      int    `mapstructure:"SMTP_PORT"`
	SMTPUser      string `mapstructure:"SMTP_USER"`
	SMTPPassword  string `mapstructure:"SMTP_PASSWORD"`
	SMTPFromEmail string `mapstructure:"SMTP_FROM_EMAIL"`
	SMTPFromName  string `mapstructure:"SMTP_FROM_NAME"`

	// JWT Auth
	JWTSecret     string `mapstructure:"JWT_SECRET"`
	JWTExpiration string `mapstructure:"JWT_EXPIRATION"`
	CookieDomain  string `mapstructure:"COOKIE_DOMAIN"`
	CookieSecure  bool   `mapstructure:"COOKIE_SECURE"`

	// Token Security
	// TokenHMACKey is used for HMAC-SHA256 token hashing (min 32 bytes).
	// If not set, falls back to JWT_SECRET for backward compatibility.
	TokenHMACKey string `mapstructure:"TOKEN_HMAC_KEY"`

	// Server
	Port       string `mapstructure:"PORT"`
	NodeEnv    string `mapstructure:"NODE_ENV"`
	APIVersion string `mapstructure:"API_VERSION"`

	// CORS & Networking
	CorsOrigin       string `mapstructure:"CORS_ORIGIN"`
	PublicBaseURL    string `mapstructure:"PUBLIC_BASE_URL"`
	BackendPublicURL string `mapstructure:"BACKEND_PUBLIC_URL"`
	WebhookSecret    string `mapstructure:"WEBHOOK_SECRET"`

	// Redis/Cache
	CacheRedisURI        string `mapstructure:"CACHE_REDIS_URI"`
	AppCachePrefix       string `mapstructure:"APP_CACHE_PREFIX"`
	DisableAppRedisCache bool   `mapstructure:"DISABLE_APP_REDIS_CACHE"`

	// Waha Plus (GoWS Engine)
	WahaAPIURL string `mapstructure:"WAHA_API_URL"`
	WahaAPIKey string `mapstructure:"WAHA_API_KEY"`

	// AI
	AIApiURL     string `mapstructure:"AI_API_URL"`
	AIApiKey     string `mapstructure:"AI_API_KEY"`
	AIModel      string `mapstructure:"AI_MODEL"`
	AIImageModel string `mapstructure:"AI_IMAGE_MODEL"`

	// Worker
	WorkerConcurrency          int `mapstructure:"WORKER_CONCURRENCY"`
	WhatsAppSendLimitPerMinute int `mapstructure:"WHATSAPP_SEND_LIMIT_PER_MINUTE"`

	// Producer Backpressure
	ProducerBackpressureDelayMS  int `mapstructure:"PRODUCER_BACKPRESSURE_DELAY_MS"`
	ProducerUserMaxEnqueued10Min int `mapstructure:"PRODUCER_USER_MAX_ENQUEUED_10MIN"`

	// ML Encryption
	MLCredentialsEncryptionKey string `mapstructure:"ML_CREDENTIALS_ENCRYPTION_KEY"`

	// Log
	LogLevel string `mapstructure:"LOG_LEVEL"`

	// Cron
	DisableCron bool `mapstructure:"DISABLE_CRON"`

	// Rate Limiting
	RateLimitMaxAttempts int `mapstructure:"RATE_LIMIT_MAX_ATTEMPTS"`
	RateLimitWindowSec   int `mapstructure:"RATE_LIMIT_WINDOW_SEC"`
	RateLimitBlockSec    int `mapstructure:"RATE_LIMIT_BLOCK_SEC"`

	// Link Safety
	LinkSafetyMode             string `mapstructure:"LINK_SAFETY_MODE"`
	LinkSafetyBlockedHosts     string `mapstructure:"LINK_SAFETY_BLOCKED_HOSTS"`
	LinkSafetyAllowedHosts     string `mapstructure:"LINK_SAFETY_ALLOWED_HOSTS"`
	LinkSafetyEnforceAllowlist bool   `mapstructure:"LINK_SAFETY_ENFORCE_ALLOWLIST"`

	// Shopee
	ShopeeAppID     string `mapstructure:"SHOPEE_APP_ID"`
	ShopeeAppSecret string `mapstructure:"SHOPEE_APP_SECRET"`

	// Kiwify
	KiwifyWebhookToken string `mapstructure:"KIWIFY_WEBHOOK_TOKEN"`

	// Timezone
	Timezone string `mapstructure:"TZ"`
}

// GetLinkSafetyBlockedHosts returns blocked hosts as a slice
func (c *Config) GetLinkSafetyBlockedHosts() []string {
	if c.LinkSafetyBlockedHosts == "" {
		return []string{}
	}
	return strings.Split(c.LinkSafetyBlockedHosts, ",")
}

// GetLinkSafetyAllowedHosts returns allowed hosts as a slice
func (c *Config) GetLinkSafetyAllowedHosts() []string {
	if c.LinkSafetyAllowedHosts == "" {
		return []string{}
	}
	return strings.Split(c.LinkSafetyAllowedHosts, ",")
}

// GetTimezone returns the configured timezone location
func (c *Config) GetTimezone() *time.Location {
	tz := c.Timezone
	if tz == "" {
		tz = "America/Sao_Paulo"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return loc
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("PORT", "3000")
	viper.SetDefault("NODE_ENV", "development")
	viper.SetDefault("API_VERSION", "v1")
	viper.SetDefault("CORS_ORIGIN", "http://localhost:5173")
	viper.SetDefault("CACHE_REDIS_URI", "redis://localhost:6379/0")
	viper.SetDefault("APP_CACHE_PREFIX", "zappgrupos")
	viper.SetDefault("WORKER_CONCURRENCY", 5)
	viper.SetDefault("WHATSAPP_SEND_LIMIT_PER_MINUTE", 15)
	viper.SetDefault("PRODUCER_BACKPRESSURE_DELAY_MS", 15000)
	viper.SetDefault("PRODUCER_USER_MAX_ENQUEUED_10MIN", 200)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LINK_SAFETY_MODE", "off")
	viper.SetDefault("TZ", "America/Sao_Paulo")
	viper.SetDefault("AI_MODEL", "gpt-4o-mini")
	viper.SetDefault("AI_IMAGE_MODEL", "imagen-3.0-generate-002")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_SSLMODE", "disable")
	viper.SetDefault("DB_MAX_CONNS", 25)
	viper.SetDefault("SMTP_HOST", "smtp.gmail.com")
	viper.SetDefault("SMTP_PORT", 587)
	viper.SetDefault("SMTP_FROM_NAME", "Automatiza")
	viper.SetDefault("JWT_EXPIRATION", "24h")
	viper.SetDefault("COOKIE_DOMAIN", "localhost")

	// Rate limiting defaults: stricter in production, relaxed for development
	// These defaults assume production; override in development via .env
	viper.SetDefault("RATE_LIMIT_MAX_ATTEMPTS", 10)
	viper.SetDefault("RATE_LIMIT_WINDOW_SEC", 60)
	viper.SetDefault("RATE_LIMIT_BLOCK_SEC", 300) // 5 minutes

	// Bind all struct fields to env vars so Unmarshal picks them up
	// even when no .env file exists (e.g. Docker env_file)
	bindStructEnvVars(Config{})

	// Try to read config file (optional)
	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Belt-and-suspenders: override critical fields directly from OS env vars.
	// Viper's Unmarshal has known edge cases where it silently ignores env vars
	// (e.g. Docker env_file without a .env on disk). This ensures DB connection
	// always works regardless of Viper internals.
	applyEnvOverrides(&cfg)

	// Validate required fields
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// bindStructEnvVars registers all mapstructure-tagged fields with viper.BindEnv
// so that Unmarshal reads OS environment variables even without a .env file.
func bindStructEnvVars(s interface{}) {
	t := reflect.TypeOf(s)
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("mapstructure")
		if tag != "" {
			_ = viper.BindEnv(tag)
		}
	}
}

// applyEnvOverrides reads critical env vars directly from os.Getenv
// as a fallback in case Viper's Unmarshal missed them.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DB_HOST"); v != "" && cfg.DBHost == "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" && cfg.DBPort == "" {
		cfg.DBPort = v
	}
	if v := os.Getenv("DB_USER"); v != "" && cfg.DBUser == "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" && cfg.DBPassword == "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" && cfg.DBName == "" {
		cfg.DBName = v
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" && cfg.DBSSLMode == "" {
		cfg.DBSSLMode = v
	}
	if v := os.Getenv("DB_MAX_CONNS"); v != "" && cfg.DBMaxConns == 0 {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DBMaxConns = n
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" && cfg.JWTSecret == "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("CACHE_REDIS_URI"); v != "" && cfg.CacheRedisURI == "" {
		cfg.CacheRedisURI = v
	}
	if v := os.Getenv("NODE_ENV"); v != "" && cfg.NodeEnv == "" {
		cfg.NodeEnv = v
	}
	if v := os.Getenv("PORT"); v != "" && cfg.Port == "" {
		cfg.Port = v
	}
	if v := os.Getenv("CORS_ORIGIN"); v != "" && cfg.CorsOrigin == "" {
		cfg.CorsOrigin = v
	}
	if v := os.Getenv("WAHA_API_URL"); v != "" && cfg.WahaAPIURL == "" {
		cfg.WahaAPIURL = v
	}
	if v := os.Getenv("WAHA_API_KEY"); v != "" && cfg.WahaAPIKey == "" {
		cfg.WahaAPIKey = v
	}
}

func validateConfig(cfg *Config) error {
	// PostgreSQL connection is validated when NewDBClients is called
	return nil
}

// HasPostgres returns true if PostgreSQL direct connection is configured
func (c *Config) HasPostgres() bool {
	return c.DBHost != "" && c.DBUser != "" && c.DBPassword != "" && c.DBName != ""
}

// PostgresConfig holds PostgreSQL connection configuration
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int
}

// GetPostgresConfig returns PostgreSQL configuration
func (c *Config) GetPostgresConfig() *PostgresConfig {
	port := c.DBPort
	if port == "" {
		port = "5432"
	}
	sslmode := c.DBSSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	maxConns := c.DBMaxConns
	if maxConns <= 0 {
		maxConns = 25
	}
	return &PostgresConfig{
		Host:     c.DBHost,
		Port:     port,
		User:     c.DBUser,
		Password: c.DBPassword,
		Database: c.DBName,
		SSLMode:  sslmode,
		MaxConns: maxConns,
	}
}

// HasSMTP returns true if SMTP is configured
func (c *Config) HasSMTP() bool {
	return c.SMTPHost != "" && c.SMTPUser != "" && c.SMTPPassword != ""
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.NodeEnv == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.NodeEnv == "production"
}

// GetTokenHMACKey returns the HMAC key for token hashing.
// Falls back to JWT_SECRET if TOKEN_HMAC_KEY is not set.
func (c *Config) GetTokenHMACKey() []byte {
	if c.TokenHMACKey != "" {
		return []byte(c.TokenHMACKey)
	}
	// Fallback to JWT_SECRET for backward compatibility
	if c.JWTSecret != "" {
		return []byte(c.JWTSecret)
	}
	return nil
}
