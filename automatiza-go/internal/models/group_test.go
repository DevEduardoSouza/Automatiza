package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWahaGroup_ToGroup(t *testing.T) {
	t.Parallel()

	t.Run("standard conversion", func(t *testing.T) {
		t.Parallel()
		wg := &WahaGroup{
			ID:          "120363022145917299@g.us",
			Subject:     "Test Group",
			Description: "A test group",
			Announce:    true,
			Size:        42,
		}

		g := wg.ToGroup("user-1", "instance-1")
		require.NotNil(t, g)
		assert.Equal(t, "120363022145917299@g.us", g.WAGroupID)
		assert.Equal(t, "Test Group", g.Name)
		require.NotNil(t, g.Description)
		assert.Equal(t, "A test group", *g.Description)
		assert.Equal(t, "instance-1", g.InstanceID)
		assert.Equal(t, "user-1", g.UserID)
		assert.True(t, g.AdminOnly)
		assert.Equal(t, 42, g.MembersCount)
	})

	t.Run("empty description", func(t *testing.T) {
		t.Parallel()
		wg := &WahaGroup{
			ID:          "120363@g.us",
			Subject:     "Minimal Group",
			Description: "",
			Size:        0,
		}

		g := wg.ToGroup("user-2", "instance-2")
		require.NotNil(t, g)
		assert.Equal(t, "Minimal Group", g.Name)
		require.NotNil(t, g.Description)
		assert.Equal(t, "", *g.Description)
		assert.False(t, g.AdminOnly)
		assert.Equal(t, 0, g.MembersCount)
	})
}
