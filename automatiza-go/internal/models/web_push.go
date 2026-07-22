package models

import "time"

// WebPushSubscription is a browser push subscription for the PWA.
// Mirrors the web_push_subscriptions table (migration 074).
type WebPushSubscription struct {
	ID         string
	UserID     string
	Endpoint   string
	P256dh     string
	Auth       string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	FailedAt   *time.Time
}
