package models

import "time"

// IncomingMessage represents a WhatsApp message received via webhook
type IncomingMessage struct {
	InstanceName string         `json:"instanceName"`
	MessageID    string         `json:"messageId"`
	RemoteJID    string         `json:"remoteJid"`
	FromMe       bool           `json:"fromMe"`
	SenderJID    string         `json:"senderJid,omitempty"`
	PushName     string         `json:"pushName,omitempty"`
	Body         string         `json:"body"`
	Timestamp    int64          `json:"timestamp"` // Unix timestamp
	HasMedia     bool           `json:"hasMedia"`
	MediaType    string         `json:"mediaType,omitempty"`
	MediaURL     string         `json:"mediaUrl,omitempty"`
	MediaBase64  string         `json:"mediaBase64,omitempty"`
	MimeType     string         `json:"mimeType,omitempty"`
	QuotedMsg    *QuotedMessage `json:"quotedMsg,omitempty"`
}

// GetTimestamp returns the timestamp as time.Time
func (m *IncomingMessage) GetTimestamp() time.Time {
	return time.Unix(m.Timestamp, 0)
}

// QuotedMessage represents a quoted/replied message
type QuotedMessage struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
	SenderJID string `json:"senderJid"`
}

// MonitoringMatch represents a matched monitoring automation
type MonitoringMatch struct {
	Automation     *Automation
	ExtractedLinks []string
	ProcessedText  string
	ShouldForward  bool
	SkipReason     string
}

// WebhookEvent represents a generic webhook event from Waha
type WebhookEvent struct {
	Event    string                 `json:"event"`
	Instance string                 `json:"instance"`
	Data     map[string]interface{} `json:"data"`
}

// SessionStatusEvent represents a session status change event
type SessionStatusEvent struct {
	Instance string `json:"instance"`
	Status   string `json:"status"` // WORKING, SCAN_QR_CODE, STOPPED, FAILED
	Me       *struct {
		ID       string `json:"id"`
		PushName string `json:"pushName"`
	} `json:"me,omitempty"`
}

// MessageEvent represents an incoming message event from Waha
type MessageEvent struct {
	Instance string `json:"instance"`
	Message  struct {
		ID        string `json:"id"`
		From      string `json:"from"`
		To        string `json:"to"`
		Body      string `json:"body"`
		Timestamp int64  `json:"timestamp"`
		FromMe    bool   `json:"fromMe"`
		HasMedia  bool   `json:"hasMedia"`
		MediaType string `json:"mediaType,omitempty"`
		MediaURL  string `json:"mediaUrl,omitempty"`
		Ack       int    `json:"ack"`
		AckName   string `json:"ackName"`
	} `json:"message"`
}

// ProcessedMessageResult represents the result of processing an incoming message
type ProcessedMessageResult struct {
	Processed      bool     `json:"processed"`
	AutomationID   string   `json:"automation_id,omitempty"`
	ExtractedLinks []string `json:"extracted_links,omitempty"`
	AffiliateLinks []string `json:"affiliate_links,omitempty"`
	SentToGroups   []string `json:"sent_to_groups,omitempty"`
	SkippedGroups  []string `json:"skipped_groups,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// ContentFingerprint represents a content fingerprint for deduplication
type ContentFingerprint struct {
	InstanceName string
	GroupID      string
	ContentHash  string
	Links        []string
	CreatedAt    string
}

// DeduplicationResult represents the result of deduplication check
type DeduplicationResult struct {
	IsDuplicate bool   `json:"is_duplicate"`
	Reason      string `json:"reason,omitempty"`
	CacheKey    string `json:"cache_key,omitempty"`
}

// LinkExtractionResult represents extracted links from a message
type LinkExtractionResult struct {
	OriginalLinks  []string          `json:"original_links"`
	ShopeeLinks    []string          `json:"shopee_links"`
	MLLinks        []string          `json:"ml_links"`
	OtherLinks     []string          `json:"other_links"`
	AffiliateLinks map[string]string `json:"affiliate_links"` // original -> affiliate
	CanonicalKeys  map[string]string `json:"canonical_keys"`  // link -> canonical key for dedup
}

// TimeWindow represents a time window for automation execution
type TimeWindow struct {
	StartTime string   `json:"start_time"` // HH:MM format
	EndTime   string   `json:"end_time"`   // HH:MM format
	Days      []string `json:"days"`       // sun, mon, tue, wed, thu, fri, sat
	Timezone  string   `json:"timezone"`   // e.g., America/Sao_Paulo
}

// IsWithinWindow checks if the current time is within the window
func (tw *TimeWindow) IsWithinWindow(now time.Time) bool {
	if tw.StartTime == "" || tw.EndTime == "" {
		return true // No restriction
	}

	// Load timezone
	loc, err := time.LoadLocation(tw.Timezone)
	if err != nil {
		loc = time.Local
	}
	now = now.In(loc)

	// Check day
	if len(tw.Days) > 0 {
		dayName := now.Weekday().String()[:3]
		dayFound := false
		for _, d := range tw.Days {
			if d == dayName || d == now.Weekday().String() {
				dayFound = true
				break
			}
		}
		if !dayFound {
			return false
		}
	}

	// Parse times
	start, err1 := time.Parse("15:04", tw.StartTime)
	end, err2 := time.Parse("15:04", tw.EndTime)
	if err1 != nil || err2 != nil {
		return true
	}

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := start.Hour()*60 + start.Minute()
	endMinutes := end.Hour()*60 + end.Minute()

	if startMinutes <= endMinutes {
		return currentMinutes >= startMinutes && currentMinutes <= endMinutes
	}
	// Crosses midnight
	return currentMinutes >= startMinutes || currentMinutes <= endMinutes
}
