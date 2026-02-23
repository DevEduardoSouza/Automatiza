package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectLink_MarshalTargetGroups(t *testing.T) {
	t.Parallel()

	t.Run("nil returns empty array JSON", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{TargetGroups: nil}
		data, err := r.MarshalTargetGroups()
		assert.NoError(t, err)
		assert.Equal(t, []byte("[]"), data)
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{TargetGroups: []GroupPriority{}}
		data, err := r.MarshalTargetGroups()
		assert.NoError(t, err)
		assert.Equal(t, "[]", string(data))
	})

	t.Run("populated slice", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{TargetGroups: []GroupPriority{
			{GroupID: "g1", Priority: 1, IsActive: true},
		}}
		data, err := r.MarshalTargetGroups()
		assert.NoError(t, err)

		var decoded []GroupPriority
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Len(t, decoded, 1)
		assert.Equal(t, "g1", decoded[0].GroupID)
	})
}

func TestRedirectLink_UnmarshalTargetGroups(t *testing.T) {
	t.Parallel()

	t.Run("nil data returns empty slice", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalTargetGroups(nil)
		assert.NoError(t, err)
		assert.Empty(t, r.TargetGroups)
	})

	t.Run("empty bytes returns empty slice", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalTargetGroups([]byte{})
		assert.NoError(t, err)
		assert.Empty(t, r.TargetGroups)
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalTargetGroups([]byte(`[{"group_id":"g1","priority":1,"is_active":true}]`))
		assert.NoError(t, err)
		require.Len(t, r.TargetGroups, 1)
		assert.Equal(t, "g1", r.TargetGroups[0].GroupID)
		assert.True(t, r.TargetGroups[0].IsActive)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalTargetGroups([]byte(`{invalid`))
		assert.Error(t, err)
	})
}

func TestRedirectLink_MarshalGroupClicks(t *testing.T) {
	t.Parallel()

	t.Run("nil returns empty object JSON", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{GroupClicks: nil}
		data, err := r.MarshalGroupClicks()
		assert.NoError(t, err)
		assert.Equal(t, []byte("{}"), data)
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{GroupClicks: map[string]int{}}
		data, err := r.MarshalGroupClicks()
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(data))
	})

	t.Run("populated map", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{GroupClicks: map[string]int{"g1": 5, "g2": 10}}
		data, err := r.MarshalGroupClicks()
		assert.NoError(t, err)

		var decoded map[string]int
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, 5, decoded["g1"])
		assert.Equal(t, 10, decoded["g2"])
	})
}

func TestRedirectLink_UnmarshalGroupClicks(t *testing.T) {
	t.Parallel()

	t.Run("nil data returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalGroupClicks(nil)
		assert.NoError(t, err)
		assert.NotNil(t, r.GroupClicks)
		assert.Empty(t, r.GroupClicks)
	})

	t.Run("empty bytes returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalGroupClicks([]byte{})
		assert.NoError(t, err)
		assert.NotNil(t, r.GroupClicks)
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalGroupClicks([]byte(`{"g1":3,"g2":7}`))
		assert.NoError(t, err)
		assert.Equal(t, 3, r.GroupClicks["g1"])
		assert.Equal(t, 7, r.GroupClicks["g2"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{}
		err := r.UnmarshalGroupClicks([]byte(`not json`))
		assert.Error(t, err)
	})
}

func TestRedirectLink_GetAvailableGroup(t *testing.T) {
	t.Parallel()

	activeGroup := func(id string, memberLimit, clickLimit int) GroupPriority {
		return GroupPriority{GroupID: id, IsActive: true, MemberLimit: memberLimit, ClickLimit: clickLimit}
	}
	inactiveGroup := func(id string) GroupPriority {
		return GroupPriority{GroupID: id, IsActive: false}
	}

	t.Run("empty groups returns nil", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{TargetGroups: []GroupPriority{}}
		assert.Nil(t, r.GetAvailableGroup(nil))
	})

	// member_limit strategy
	t.Run("member_limit/under limit returns group", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyMemberLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 100, 0)},
		}
		result := r.GetAvailableGroup(map[string]int{"g1": 50})
		require.NotNil(t, result)
		assert.Equal(t, "g1", result.GroupID)
	})

	t.Run("member_limit/at limit skips group", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyMemberLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 100, 0)},
		}
		result := r.GetAvailableGroup(map[string]int{"g1": 100})
		assert.Nil(t, result)
	})

	t.Run("member_limit/limit 0 means unlimited", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyMemberLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 0, 0)},
		}
		result := r.GetAvailableGroup(map[string]int{"g1": 9999})
		require.NotNil(t, result)
		assert.Equal(t, "g1", result.GroupID)
	})

	t.Run("member_limit/skips inactive groups", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyMemberLimit,
			TargetGroups: []GroupPriority{inactiveGroup("g1"), activeGroup("g2", 100, 0)},
		}
		result := r.GetAvailableGroup(map[string]int{"g2": 10})
		require.NotNil(t, result)
		assert.Equal(t, "g2", result.GroupID)
	})

	// click_limit strategy
	t.Run("click_limit/under limit returns group", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyClickLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 0, 50)},
			GroupClicks:  map[string]int{"g1": 10},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g1", result.GroupID)
	})

	t.Run("click_limit/at limit skips group", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyClickLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 0, 50)},
			GroupClicks:  map[string]int{"g1": 50},
		}
		result := r.GetAvailableGroup(nil)
		assert.Nil(t, result)
	})

	t.Run("click_limit/limit 0 means unlimited", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyClickLimit,
			TargetGroups: []GroupPriority{activeGroup("g1", 0, 0)},
			GroupClicks:  map[string]int{"g1": 9999},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g1", result.GroupID)
	})

	t.Run("click_limit/first full then second", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy: RedirectStrategyClickLimit,
			TargetGroups: []GroupPriority{
				activeGroup("g1", 0, 10),
				activeGroup("g2", 0, 10),
			},
			GroupClicks: map[string]int{"g1": 10, "g2": 5},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g2", result.GroupID)
	})

	// round_robin strategy
	t.Run("round_robin/selects lowest clicks", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy: RedirectStrategyRoundRobin,
			TargetGroups: []GroupPriority{
				activeGroup("g1", 0, 0),
				activeGroup("g2", 0, 0),
				activeGroup("g3", 0, 0),
			},
			GroupClicks: map[string]int{"g1": 10, "g2": 3, "g3": 7},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g2", result.GroupID)
	})

	t.Run("round_robin/all equal selects first", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy: RedirectStrategyRoundRobin,
			TargetGroups: []GroupPriority{
				activeGroup("g1", 0, 0),
				activeGroup("g2", 0, 0),
			},
			GroupClicks: map[string]int{"g1": 5, "g2": 5},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g1", result.GroupID)
	})

	t.Run("round_robin/skips inactive", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy: RedirectStrategyRoundRobin,
			TargetGroups: []GroupPriority{
				inactiveGroup("g1"),
				activeGroup("g2", 0, 0),
			},
			GroupClicks: map[string]int{"g1": 0, "g2": 5},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g2", result.GroupID)
	})

	// Edge cases
	t.Run("all inactive returns nil", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     RedirectStrategyMemberLimit,
			TargetGroups: []GroupPriority{inactiveGroup("g1"), inactiveGroup("g2")},
		}
		result := r.GetAvailableGroup(nil)
		assert.Nil(t, result)
	})

	t.Run("unknown strategy returns first active", func(t *testing.T) {
		t.Parallel()
		r := &RedirectLink{
			Strategy:     "unknown_strategy",
			TargetGroups: []GroupPriority{inactiveGroup("g1"), activeGroup("g2", 0, 0)},
		}
		result := r.GetAvailableGroup(nil)
		require.NotNil(t, result)
		assert.Equal(t, "g2", result.GroupID)
	})
}
