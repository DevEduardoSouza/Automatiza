package models

// Instance represents a WhatsApp instance in the database
type Instance struct {
	ID           int     `json:"id" db:"id"`
	UserID       string  `json:"user_id" db:"user_id"`
	InstanceID   string  `json:"instance_id" db:"instance_id"`
	FriendlyName *string `json:"friendly_name,omitempty" db:"friendly_name"`
	Status       string  `json:"status" db:"status"` // connected|disconnected|pending
	QRCode       *string `json:"qr_code,omitempty" db:"qr_code"`
	IsDefault    bool    `json:"is_default" db:"is_default"`
	CreatedAt    string  `json:"created_at" db:"created_at"`
	UpdatedAt    string  `json:"updated_at" db:"updated_at"`
}

// WahaInstance represents instance info from the WAHA API
type WahaInstance struct {
	InstanceName string  `json:"instanceName"`
	Status       string  `json:"status"`
	QRCode       *QRCode `json:"qrcode,omitempty"`
}

// QRCode represents QR code data
type QRCode struct {
	Base64 string `json:"base64"`
	Code   string `json:"code,omitempty"`
}

// InstanceStatus constants
const (
	InstanceStatusConnected    = "connected"
	InstanceStatusDisconnected = "disconnected"
	InstanceStatusPending      = "pending"
	InstanceStatusOpen         = "open"
	InstanceStatusClose        = "close"
)

// IsConnected returns true if the instance is connected
func (i *Instance) IsConnected() bool {
	return i.Status == InstanceStatusConnected || i.Status == InstanceStatusOpen
}

// WahaSession represents a session from Waha API
type WahaSession struct {
	Name   string                 `json:"name"`
	Status string                 `json:"status"`
	Me     *WahaSessionMe         `json:"me,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// WahaSessionMe represents the authenticated user info
type WahaSessionMe struct {
	ID       string `json:"id"`
	PushName string `json:"pushName"`
}

// WahaQRCode represents QR code response from Waha
type WahaQRCode struct {
	Value    string `json:"value"`
	Mimetype string `json:"mimetype"`
}
