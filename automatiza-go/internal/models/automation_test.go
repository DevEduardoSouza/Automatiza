package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutomation_IsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"active", AutomationStatusActive, true},
		{"inactive", AutomationStatusInactive, false},
		{"paused", AutomationStatusPaused, false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{Status: tt.status}
			assert.Equal(t, tt.want, a.IsActive())
		})
	}
}

func TestAutomation_IsMonitoring(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeVal  *string
		want     bool
	}{
		{"nil defaults to false", nil, false},
		{"monitoring", ptr(AutomationTypeMonitoring), true},
		{"search", ptr(AutomationTypeSearch), false},
		{"list", ptr(AutomationTypeList), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{Type: tt.typeVal}
			assert.Equal(t, tt.want, a.IsMonitoring())
		})
	}
}

func TestAutomation_IsSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeVal *string
		want    bool
	}{
		{"nil defaults to true", nil, true},
		{"search", ptr(AutomationTypeSearch), true},
		{"monitoring", ptr(AutomationTypeMonitoring), false},
		{"list", ptr(AutomationTypeList), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{Type: tt.typeVal}
			assert.Equal(t, tt.want, a.IsSearch())
		})
	}
}

func TestAutomation_IsList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeVal *string
		want    bool
	}{
		{"nil defaults to false", nil, false},
		{"list", ptr(AutomationTypeList), true},
		{"search", ptr(AutomationTypeSearch), false},
		{"monitoring", ptr(AutomationTypeMonitoring), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{Type: tt.typeVal}
			assert.Equal(t, tt.want, a.IsList())
		})
	}
}

func TestAutomation_GetDedupeHours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dedupeHours *int
		want        int
	}{
		{"nil returns default 168", nil, 168},
		{"explicit value", ptr(24), 24},
		{"zero value", ptr(0), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{DedupeHours: tt.dedupeHours}
			assert.Equal(t, tt.want, a.GetDedupeHours())
		})
	}
}

func TestAutomation_GetDelaySeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		delaySeconds *int
		want         int
	}{
		{"nil returns default 1", nil, 1},
		{"explicit value", ptr(5), 5},
		{"zero value", ptr(0), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{DelaySeconds: tt.delaySeconds}
			assert.Equal(t, tt.want, a.GetDelaySeconds())
		})
	}
}

func TestAutomation_GetTargetGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sendGroups []string
		groupIDs   []string
		want       []string
	}{
		{
			"SendGroups populated returns SendGroups",
			[]string{"sg1", "sg2"},
			[]string{"g1"},
			[]string{"sg1", "sg2"},
		},
		{
			"SendGroups empty returns GroupIDs",
			[]string{},
			[]string{"g1", "g2"},
			[]string{"g1", "g2"},
		},
		{
			"SendGroups nil returns GroupIDs",
			nil,
			[]string{"g1"},
			[]string{"g1"},
		},
		{
			"both nil returns nil",
			nil,
			nil,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{SendGroups: tt.sendGroups, GroupIDs: tt.groupIDs}
			assert.Equal(t, tt.want, a.GetTargetGroups())
		})
	}
}

func TestAutomation_GetType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeVal *string
		want    string
	}{
		{"nil returns search", nil, AutomationTypeSearch},
		{"monitoring", ptr(AutomationTypeMonitoring), AutomationTypeMonitoring},
		{"list", ptr(AutomationTypeList), AutomationTypeList},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{Type: tt.typeVal}
			assert.Equal(t, tt.want, a.GetType())
		})
	}
}

func TestAutomation_GetCustomMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		customMessage *string
		want          string
	}{
		{"nil returns empty", nil, ""},
		{"with message", ptr("hello"), "hello"},
		{"empty string", ptr(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Automation{CustomMessage: tt.customMessage}
			assert.Equal(t, tt.want, a.GetCustomMessage())
		})
	}
}

func TestAutomation_MarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("round trip preserves fields", func(t *testing.T) {
		t.Parallel()
		a := &Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
			Status: AutomationStatusActive,
		}

		data, err := json.Marshal(a)
		assert.NoError(t, err)

		var decoded Automation
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, a.ID, decoded.ID)
		assert.Equal(t, a.Name, decoded.Name)
		assert.Equal(t, a.Status, decoded.Status)
	})

	t.Run("omitempty omits nil pointer fields", func(t *testing.T) {
		t.Parallel()
		a := &Automation{
			ID:     "auto-2",
			Status: AutomationStatusInactive,
		}

		data, err := json.Marshal(a)
		assert.NoError(t, err)

		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		assert.NoError(t, err)
		_, hasShopeeLink := raw["shopee_link"]
		assert.False(t, hasShopeeLink, "nil shopee_link should be omitted")
	})
}

func TestUpdateAutomationRequest_ToMap(t *testing.T) {
	t.Parallel()

	t.Run("all nil returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &UpdateAutomationRequest{}
		m := r.ToMap()
		assert.Empty(t, m)
	})

	t.Run("single field", func(t *testing.T) {
		t.Parallel()
		r := &UpdateAutomationRequest{Name: ptr("new name")}
		m := r.ToMap()
		assert.Len(t, m, 1)
		assert.Equal(t, "new name", m["name"])
	})

	t.Run("all fields populated", func(t *testing.T) {
		t.Parallel()
		r := &UpdateAutomationRequest{
			Name:            ptr("n"),
			GroupIDs:        []string{"g1"},
			Type:            ptr("search"),
			Status:          ptr("active"),
			Stores:          []string{"shopee"},
			Categories:      []string{"cat1"},
			Keyword:         ptr("kw"),
			MinPrice:        ptr(10.0),
			MaxPrice:        ptr(100.0),
			MinDiscount:     ptr(5.0),
			CustomMessage:   ptr("msg"),
			IncludeImage:    ptr(true),
			ScheduleTime:    ptr("10:00"),
			ScheduleDays:    []string{"Mon"},
			SendGroups:      []string{"sg1"},
			MonitorGroupIDs: []string{"mg1"},
			DelaySeconds:    ptr(3),
			DedupeHours:     ptr(48),
			DispatchTextID:  ptr("dt-1"),
			StartTime:       ptr("08:00"),
			EndTime:         ptr("22:00"),
			IntervalMinutes: ptr(30),
			CouponID:        ptr("coupon-1"),
		}
		m := r.ToMap()
		assert.Len(t, m, 23)
		assert.Equal(t, "n", m["name"])
		assert.Equal(t, []string{"g1"}, m["group_ids"])
		assert.Equal(t, "search", m["type"])
		assert.Equal(t, "active", m["status"])
		assert.Equal(t, []string{"shopee"}, m["stores"])
		assert.Equal(t, []string{"cat1"}, m["categories"])
		assert.Equal(t, "kw", m["keyword"])
		assert.Equal(t, 10.0, m["min_price"])
		assert.Equal(t, 100.0, m["max_price"])
		assert.Equal(t, 5.0, m["min_discount"])
		assert.Equal(t, "msg", m["custom_message"])
		assert.Equal(t, true, m["include_image"])
		assert.Equal(t, "10:00", m["schedule_time"])
		assert.Equal(t, []string{"Mon"}, m["schedule_days"])
		assert.Equal(t, []string{"sg1"}, m["send_groups"])
		assert.Equal(t, []string{"mg1"}, m["monitor_group_ids"])
		assert.Equal(t, 3, m["delay_seconds"])
		assert.Equal(t, 48, m["dedupe_hours"])
		assert.Equal(t, "dt-1", m["dispatch_text_id"])
		assert.Equal(t, "08:00", m["start_time"])
		assert.Equal(t, "22:00", m["end_time"])
		assert.Equal(t, 30, m["interval_minutes"])
		assert.Equal(t, "coupon-1", m["coupon_id"])
	})

	t.Run("slices included when non-nil", func(t *testing.T) {
		t.Parallel()
		r := &UpdateAutomationRequest{
			GroupIDs:   []string{},
			SendGroups: []string{"a", "b"},
		}
		m := r.ToMap()
		assert.Len(t, m, 2)
		assert.Equal(t, []string{}, m["group_ids"])
		assert.Equal(t, []string{"a", "b"}, m["send_groups"])
	})
}
