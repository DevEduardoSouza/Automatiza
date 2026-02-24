package models

// Setting represents a key-value setting in the database
type Setting struct {
	Key       string `json:"key" db:"key"`
	Value     string `json:"value" db:"value"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

// SettingsMap is a map of settings by key
type SettingsMap map[string]interface{}

// GetString gets a string value from settings
func (s SettingsMap) GetString(key string) string {
	if val, ok := s[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetBool gets a boolean value from settings
func (s SettingsMap) GetBool(key string) bool {
	if val, ok := s[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
		if str, ok := val.(string); ok {
			return str == "true" || str == "1"
		}
	}
	return false
}

// GetInt gets an integer value from settings
func (s SettingsMap) GetInt(key string) int {
	if val, ok := s[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// Common settings keys
const (
	SettingKeyWahaAPIURL = "waha_api_url"
	SettingKeyWahaAPIKey = "waha_api_key"
)

// GetSettingsRequest represents a request to get settings
type GetSettingsRequest struct {
	Keys []string `json:"keys" form:"keys"`
}

// UpdateSettingRequest represents a request to update a setting
type UpdateSettingRequest struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
}

// DispatchText represents a dispatch text template in the database
type DispatchText struct {
	ID        string   `json:"id" db:"id"`
	UserID    string   `json:"user_id" db:"user_id"`
	Name      string   `json:"name" db:"name"`
	Body      string   `json:"body" db:"body"`
	IsActive  bool     `json:"is_active" db:"is_active"`
	CreatedAt string   `json:"created_at" db:"created_at"`
	UpdatedAt string   `json:"updated_at" db:"updated_at"`
	Copies    []string `json:"copies" db:"copies"`
}

// GetBody returns copies[0] if available, otherwise falls back to Body.
func (dt *DispatchText) GetBody() string {
	if len(dt.Copies) > 0 {
		return dt.Copies[0]
	}
	return dt.Body
}

// SystemEvent represents a system event log entry
type SystemEvent struct {
	ID        int                    `json:"id" db:"id"`
	EventType string                 `json:"event_type" db:"event_type"`
	Payload   map[string]interface{} `json:"payload" db:"payload"`
	CreatedAt string                 `json:"created_at" db:"created_at"`
}

// SystemEventType constants
const (
	SystemEventTypeError   = "error"
	SystemEventTypeWarning = "warning"
	SystemEventTypeInfo    = "info"
)
