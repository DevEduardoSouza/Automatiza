package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// HasValidCredentials Tests
// =============================================================================

func TestHasValidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		cred     AffiliateCredentials
		expected bool
	}{
		// Shopee
		{
			name: "shopee: valid with appId + password (camelCase)",
			cred: AffiliateCredentials{
				Store:       "shopee",
				Credentials: map[string]string{"appId": "my-app", "password": "my-pass"},
			},
			expected: true,
		},
		{
			name: "shopee: valid with app_id + app_secret (snake_case legacy)",
			cred: AffiliateCredentials{
				Store:       "shopee",
				Credentials: map[string]string{"app_id": "my-app", "app_secret": "my-secret"},
			},
			expected: true,
		},
		{
			name: "shopee: valid with legacy AppID + AppSecret fields (no map)",
			cred: AffiliateCredentials{
				Store:     "shopee",
				AppID:     "legacy-id",
				AppSecret: "legacy-secret",
			},
			expected: true,
		},
		{
			name: "shopee: missing password",
			cred: AffiliateCredentials{
				Store:       "shopee",
				Credentials: map[string]string{"appId": "my-app"},
			},
			expected: false,
		},
		{
			name: "shopee: empty appId",
			cred: AffiliateCredentials{
				Store:       "shopee",
				Credentials: map[string]string{"appId": "", "password": "my-pass"},
			},
			expected: false,
		},
		{
			name: "shopee: empty credentials map",
			cred: AffiliateCredentials{
				Store:       "shopee",
				Credentials: map[string]string{},
			},
			expected: false,
		},

		// Mercado Livre / ML
		{
			name: "mercado_livre: valid with encrypted_token",
			cred: AffiliateCredentials{
				Store:       "mercado_livre",
				Credentials: map[string]string{"encrypted_token": "enc-tok-123"},
			},
			expected: true,
		},
		{
			name: "mercado_livre: valid with access_token",
			cred: AffiliateCredentials{
				Store:       "mercado_livre",
				Credentials: map[string]string{"access_token": "acc-tok-456"},
			},
			expected: true,
		},
		{
			name: "mercado_livre: valid with tag",
			cred: AffiliateCredentials{
				Store:       "mercado_livre",
				Credentials: map[string]string{"tag": "my-tag"},
			},
			expected: true,
		},
		{
			name: "ml: valid with tag (alias)",
			cred: AffiliateCredentials{
				Store:       "ml",
				Credentials: map[string]string{"tag": "my-tag"},
			},
			expected: true,
		},
		{
			name: "mercado_livre: all empty",
			cred: AffiliateCredentials{
				Store:       "mercado_livre",
				Credentials: map[string]string{"encrypted_token": "", "access_token": "", "tag": ""},
			},
			expected: false,
		},

		// Amazon
		{
			name: "amazon: valid with tag + accessKey + secretKey",
			cred: AffiliateCredentials{
				Store:       "amazon",
				Credentials: map[string]string{"tag": "amz-tag", "accessKey": "AK123", "secretKey": "SK456"},
			},
			expected: true,
		},
		{
			name: "amazon: valid with partner_tag + access_key + secret_key (snake_case)",
			cred: AffiliateCredentials{
				Store:       "amazon",
				Credentials: map[string]string{"partner_tag": "amz-tag", "access_key": "AK123", "secret_key": "SK456"},
			},
			expected: true,
		},
		{
			// A tag basta: a conversão é reescrita de URL, e accessKey/secretKey
			// eram credenciais da PA-API 5.0, desligada em 15/05/2026.
			name: "amazon: so a tag ja e valida",
			cred: AffiliateCredentials{
				Store:       "amazon",
				Credentials: map[string]string{"tag": "amz-tag"},
			},
			expected: true,
		},
		{
			name: "amazon: all empty",
			cred: AffiliateCredentials{
				Store:       "amazon",
				Credentials: map[string]string{},
			},
			expected: false,
		},

		// Awin
		{
			name: "awin: valid with awinId + oauth2Token",
			cred: AffiliateCredentials{
				Store:       "awin",
				Credentials: map[string]string{"awinId": "awin-123", "oauth2Token": "tok-abc"},
			},
			expected: true,
		},
		{
			name: "awin: valid with publisher_id + api_key (aliases)",
			cred: AffiliateCredentials{
				Store:       "awin",
				Credentials: map[string]string{"publisher_id": "pub-123", "api_key": "key-abc"},
			},
			expected: true,
		},
		{
			name: "awin: missing token",
			cred: AffiliateCredentials{
				Store:       "awin",
				Credentials: map[string]string{"awinId": "awin-123"},
			},
			expected: false,
		},

		// Magalu
		{
			name: "magalu: valid with partnerStore",
			cred: AffiliateCredentials{
				Store:       "magalu",
				Credentials: map[string]string{"partnerStore": "my-store"},
			},
			expected: true,
		},
		{
			name: "magalu: empty partnerStore",
			cred: AffiliateCredentials{
				Store:       "magalu",
				Credentials: map[string]string{"partnerStore": ""},
			},
			expected: false,
		},

		// Shein
		{
			name: "shein: valid with affiliateLink",
			cred: AffiliateCredentials{
				Store:       "shein",
				Credentials: map[string]string{"affiliateLink": "https://shein.com/aff/123"},
			},
			expected: true,
		},
		{
			name: "shein: empty affiliateLink",
			cred: AffiliateCredentials{
				Store:       "shein",
				Credentials: map[string]string{"affiliateLink": ""},
			},
			expected: false,
		},

		// AliExpress
		{
			// appSecret entrou com o conversor: sem ele não há como assinar a
			// chamada da API, então a credencial não pode contar como ativa.
			name: "aliexpress: valid with trackingId + appKey + appSecret",
			cred: AffiliateCredentials{
				Store:       "aliexpress",
				Credentials: map[string]string{"trackingId": "track-123", "appKey": "app-key-456", "appSecret": "sec-789"},
			},
			expected: true,
		},
		{
			name: "aliexpress: missing appKey",
			cred: AffiliateCredentials{
				Store:       "aliexpress",
				Credentials: map[string]string{"trackingId": "track-123"},
			},
			expected: false,
		},

		// Edge cases
		{
			name:     "unknown store",
			cred:     AffiliateCredentials{Store: "unknown_store"},
			expected: false,
		},
		{
			name:     "empty Store field",
			cred:     AffiliateCredentials{},
			expected: false,
		},
		{
			name: "nil credentials map for store that needs map",
			cred: AffiliateCredentials{
				Store: "shopee",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cred.HasValidCredentials()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// GetCredential Tests
// =============================================================================

func TestGetCredential(t *testing.T) {
	t.Run("returns value from credentials map", func(t *testing.T) {
		cred := AffiliateCredentials{
			Credentials: map[string]string{"appId": "123"},
		}
		assert.Equal(t, "123", cred.GetCredential("appId"))
	})

	t.Run("fallback to AppID for app_id key", func(t *testing.T) {
		cred := AffiliateCredentials{AppID: "legacy-id"}
		assert.Equal(t, "legacy-id", cred.GetCredential("app_id"))
	})

	t.Run("fallback to AppSecret for app_secret key", func(t *testing.T) {
		cred := AffiliateCredentials{AppSecret: "legacy-secret"}
		assert.Equal(t, "legacy-secret", cred.GetCredential("app_secret"))
	})

	t.Run("returns empty for unknown key", func(t *testing.T) {
		cred := AffiliateCredentials{}
		assert.Equal(t, "", cred.GetCredential("unknown"))
	})

	t.Run("map takes priority over legacy fields", func(t *testing.T) {
		cred := AffiliateCredentials{
			AppID:       "legacy",
			Credentials: map[string]string{"app_id": "from-map"},
		}
		assert.Equal(t, "from-map", cred.GetCredential("app_id"))
	})
}

// =============================================================================
// SetCredential Tests
// =============================================================================

func TestSetCredential(t *testing.T) {
	t.Run("sets value in existing map", func(t *testing.T) {
		cred := AffiliateCredentials{
			Credentials: map[string]string{"existing": "value"},
		}
		cred.SetCredential("new", "val")
		assert.Equal(t, "val", cred.Credentials["new"])
		assert.Equal(t, "value", cred.Credentials["existing"])
	})

	t.Run("initializes nil map", func(t *testing.T) {
		cred := AffiliateCredentials{}
		cred.SetCredential("key", "value")
		assert.NotNil(t, cred.Credentials)
		assert.Equal(t, "value", cred.Credentials["key"])
	})

	t.Run("overwrites existing value", func(t *testing.T) {
		cred := AffiliateCredentials{
			Credentials: map[string]string{"key": "old"},
		}
		cred.SetCredential("key", "new")
		assert.Equal(t, "new", cred.Credentials["key"])
	})
}

// =============================================================================
// CanonicalKeyFromShopeeOffer Tests
// =============================================================================

func TestCanonicalKeyFromShopeeOffer(t *testing.T) {
	offer := &ShopeeOffer{ShopID: 123, ItemID: 456}
	assert.Equal(t, "shopee:123:456", CanonicalKeyFromShopeeOffer(offer))
}

// =============================================================================
// FormatCanonicalKey Tests
// =============================================================================

func TestFormatCanonicalKey(t *testing.T) {
	assert.Equal(t, "shopee:100:200", FormatCanonicalKey("shopee", 100, 200))
	assert.Equal(t, "ml:0:0", FormatCanonicalKey("ml", 0, 0))
}
