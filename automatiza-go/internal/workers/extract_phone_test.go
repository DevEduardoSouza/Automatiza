package workers

import "testing"

func TestExtractPhoneFromMe(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
		want    string
	}{
		{name: "nil payload", payload: nil, want: ""},
		{name: "no me field", payload: map[string]interface{}{"status": "WORKING"}, want: ""},
		{name: "me without id", payload: map[string]interface{}{"me": map[string]interface{}{"pushName": "x"}}, want: ""},
		{
			name: "WAHA c.us suffix",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": "557498137258@c.us"},
			},
			want: "557498137258",
		},
		{
			name: "WAHA s.whatsapp.net suffix",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": "557498137258@s.whatsapp.net"},
			},
			want: "557498137258",
		},
		{
			name: "no suffix, raw digits",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": "557498137258"},
			},
			want: "557498137258",
		},
		{
			name: "with mixed characters",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": "+55 (74) 9813-7258@c.us"},
			},
			want: "557498137258",
		},
		{
			name: "too short rejected",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": "12345@c.us"},
			},
			want: "",
		},
		{
			name: "wrong types ignored",
			payload: map[string]interface{}{
				"me": map[string]interface{}{"id": 12345},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPhoneFromMe(tt.payload)
			if got != tt.want {
				t.Errorf("extractPhoneFromMe(%v) = %q, want %q", tt.payload, got, tt.want)
			}
		})
	}
}
