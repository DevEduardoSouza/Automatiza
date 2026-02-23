package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSettingsMap_GetString(t *testing.T) {
	t.Parallel()

	t.Run("key exists with string value", func(t *testing.T) {
		t.Parallel()
		s := SettingsMap{"url": "http://example.com"}
		assert.Equal(t, "http://example.com", s.GetString("url"))
	})

	t.Run("key absent returns empty", func(t *testing.T) {
		t.Parallel()
		s := SettingsMap{}
		assert.Equal(t, "", s.GetString("missing"))
	})

	t.Run("key exists with non-string type returns empty", func(t *testing.T) {
		t.Parallel()
		s := SettingsMap{"count": 42}
		assert.Equal(t, "", s.GetString("count"))
	})

	t.Run("nil map returns empty", func(t *testing.T) {
		t.Parallel()
		var s SettingsMap
		assert.Equal(t, "", s.GetString("any"))
	})
}

func TestSettingsMap_GetBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    SettingsMap
		key  string
		want bool
	}{
		{"bool true", SettingsMap{"k": true}, "k", true},
		{"bool false", SettingsMap{"k": false}, "k", false},
		{"string true", SettingsMap{"k": "true"}, "k", true},
		{"string 1", SettingsMap{"k": "1"}, "k", true},
		{"string false", SettingsMap{"k": "false"}, "k", false},
		{"string 0", SettingsMap{"k": "0"}, "k", false},
		{"absent key", SettingsMap{}, "k", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.m.GetBool(tt.key))
		})
	}
}

func TestSettingsMap_GetInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    SettingsMap
		key  string
		want int
	}{
		{"int value", SettingsMap{"k": 42}, "k", 42},
		{"int64 value", SettingsMap{"k": int64(100)}, "k", 100},
		{"float64 value truncates", SettingsMap{"k": 3.7}, "k", 3},
		{"absent key returns 0", SettingsMap{}, "k", 0},
		{"wrong type returns 0", SettingsMap{"k": "not-int"}, "k", 0},
		{"nil map returns 0", nil, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.m.GetInt(tt.key))
		})
	}
}
