package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalKeyFromMLProduct(t *testing.T) {
	t.Parallel()

	t.Run("standard product", func(t *testing.T) {
		t.Parallel()
		p := &MLProduct{ID: "MLB1234567890"}
		assert.Equal(t, "mercadolivre:MLB1234567890", CanonicalKeyFromMLProduct(p))
	})

	t.Run("empty ID", func(t *testing.T) {
		t.Parallel()
		p := &MLProduct{ID: ""}
		assert.Equal(t, "mercadolivre:", CanonicalKeyFromMLProduct(p))
	})
}
