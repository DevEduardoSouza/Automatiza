package config

import (
	"context"
	"sync"
	"time"

	"automatiza-go/internal/repository"
)

// WahaConfig holds dynamic Waha Plus configuration
type WahaConfig struct {
	APIURL string
	APIKey string
}

// WahaConfigProvider provides Waha configuration with caching
type WahaConfigProvider struct {
	staticConfig *Config
	settingsRepo repository.SettingsRepository
	cache        *WahaConfig
	cacheMu      sync.RWMutex
	cacheExpiry  time.Time
	cacheTTL     time.Duration
}

// NewWahaConfigProvider creates a new Waha config provider using settings repository
func NewWahaConfigProvider(cfg *Config, settingsRepo repository.SettingsRepository) *WahaConfigProvider {
	return &WahaConfigProvider{
		staticConfig: cfg,
		settingsRepo: settingsRepo,
		cacheTTL:     5 * time.Minute,
	}
}

// GetConfig returns the current Waha configuration
// It first checks static config, then falls back to database settings
func (p *WahaConfigProvider) GetConfig(ctx context.Context) (*WahaConfig, error) {
	// If static config is set, use it
	if p.staticConfig.WahaAPIURL != "" && p.staticConfig.WahaAPIKey != "" {
		return &WahaConfig{
			APIURL: p.staticConfig.WahaAPIURL,
			APIKey: p.staticConfig.WahaAPIKey,
		}, nil
	}

	// Check cache
	p.cacheMu.RLock()
	if p.cache != nil && time.Now().Before(p.cacheExpiry) {
		cfg := p.cache
		p.cacheMu.RUnlock()
		return cfg, nil
	}
	p.cacheMu.RUnlock()

	// Fetch from database
	return p.refreshConfig(ctx)
}

// refreshConfig fetches Waha config from the settings table
func (p *WahaConfigProvider) refreshConfig(ctx context.Context) (*WahaConfig, error) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	// Double-check after acquiring lock
	if p.cache != nil && time.Now().Before(p.cacheExpiry) {
		return p.cache, nil
	}

	cfg := &WahaConfig{}

	// Use settings repository
	if p.settingsRepo != nil {
		settings, err := p.settingsRepo.Get(ctx, "waha_api_url", "waha_api_key")
		if err != nil {
			// If we have cached config, return it even if expired
			if p.cache != nil {
				return p.cache, nil
			}
			return nil, err
		}

		cfg.APIURL = settings.GetString("waha_api_url")
		cfg.APIKey = settings.GetString("waha_api_key")
	}

	// Update cache
	p.cache = cfg
	p.cacheExpiry = time.Now().Add(p.cacheTTL)

	return cfg, nil
}

// ClearCache clears the cached configuration
func (p *WahaConfigProvider) ClearCache() {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	p.cache = nil
	p.cacheExpiry = time.Time{}
}
