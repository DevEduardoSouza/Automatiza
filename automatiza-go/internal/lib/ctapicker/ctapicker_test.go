package ctapicker

import (
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickByPlatform(t *testing.T) {
	t.Parallel()

	shopeeCTA := models.CTA{Message: "OFERTA DA SHÔ", Platform: models.CTAPlatformShopee, IsActive: true}
	mlCTA := models.CTA{Message: "MERCADO LIVRE 🔥", Platform: models.CTAPlatformMercadoLivre, IsActive: true}
	allCTA := models.CTA{Message: "QUALQUER LOJA", Platform: models.CTAPlatformAll, IsActive: true}
	inactiveShopee := models.CTA{Message: "DESATIVADA", Platform: models.CTAPlatformShopee, IsActive: false}

	pool := []models.CTA{shopeeCTA, mlCTA, allCTA, inactiveShopee}

	t.Run("ML product picks ML or all, never Shopee/inactive", func(t *testing.T) {
		t.Parallel()
		seen := map[string]bool{}
		for i := 0; i < 200; i++ {
			msg := PickByPlatform(pool, true, false)
			require.NotEmpty(t, msg)
			seen[msg] = true
		}
		assert.False(t, seen[shopeeCTA.Message], "Shopee CTA picked for ML product")
		assert.False(t, seen[inactiveShopee.Message], "inactive CTA picked")
		assert.True(t, seen[mlCTA.Message] || seen[allCTA.Message])
	})

	t.Run("Shopee product picks Shopee or all, never ML/inactive", func(t *testing.T) {
		t.Parallel()
		seen := map[string]bool{}
		for i := 0; i < 200; i++ {
			msg := PickByPlatform(pool, false, true)
			require.NotEmpty(t, msg)
			seen[msg] = true
		}
		assert.False(t, seen[mlCTA.Message], "ML CTA picked for Shopee product")
		assert.False(t, seen[inactiveShopee.Message], "inactive CTA picked")
		assert.True(t, seen[shopeeCTA.Message] || seen[allCTA.Message])
	})

	t.Run("unknown platform falls back to 'all' only", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, allCTA.Message, PickByPlatform(pool, false, false))
	})

	t.Run("empty pool returns empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, PickByPlatform(nil, true, false))
		assert.Empty(t, PickByPlatform([]models.CTA{}, false, true))
	})

	t.Run("no matching platform returns empty (lets caller fallback)", func(t *testing.T) {
		t.Parallel()
		onlyShopee := []models.CTA{shopeeCTA}
		assert.Empty(t, PickByPlatform(onlyShopee, true, false))
	})
}
