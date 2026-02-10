// Package metrics provides Prometheus metrics for observability.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WebhookMetrics contains all Prometheus metrics for webhook processing.
type WebhookMetrics struct {
	// ReceivedTotal counts total webhooks received by event type
	ReceivedTotal *prometheus.CounterVec

	// ProcessedTotal counts successfully processed webhooks by event type
	ProcessedTotal *prometheus.CounterVec

	// FailedTotal counts failed webhook processing by event type and error type
	FailedTotal *prometheus.CounterVec

	// DLQTotal counts webhooks moved to dead letter queue by event type
	DLQTotal *prometheus.CounterVec

	// DuplicatesTotal counts duplicate webhooks rejected by event type
	DuplicatesTotal *prometheus.CounterVec

	// ProcessingDuration tracks webhook processing time in seconds
	ProcessingDuration *prometheus.HistogramVec

	// QueuedTotal counts webhooks enqueued for async processing
	QueuedTotal *prometheus.CounterVec

	// InFlight tracks current number of webhooks being processed
	InFlight *prometheus.GaugeVec

	// HMACValidationTotal counts HMAC validation results
	HMACValidationTotal *prometheus.CounterVec

	// RetryTotal counts retry attempts by event type
	RetryTotal *prometheus.CounterVec

	// BodySizeBytes tracks webhook payload sizes
	BodySizeBytes *prometheus.HistogramVec
}

// NewWebhookMetrics creates and registers webhook metrics with the default registry.
func NewWebhookMetrics() *WebhookMetrics {
	return &WebhookMetrics{
		ReceivedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "received_total",
				Help:      "Total number of webhooks received",
			},
			[]string{"session", "event_type"},
		),

		ProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "processed_total",
				Help:      "Total number of webhooks successfully processed",
			},
			[]string{"session", "event_type"},
		),

		FailedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "failed_total",
				Help:      "Total number of webhook processing failures",
			},
			[]string{"session", "event_type", "error_type"},
		),

		DLQTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "dlq_total",
				Help:      "Total number of webhooks moved to dead letter queue",
			},
			[]string{"session", "event_type"},
		),

		DuplicatesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "duplicates_total",
				Help:      "Total number of duplicate webhooks rejected",
			},
			[]string{"session", "event_type"},
		),

		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "processing_duration_seconds",
				Help:      "Time taken to process webhooks in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"session", "event_type"},
		),

		QueuedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "queued_total",
				Help:      "Total number of webhooks enqueued for async processing",
			},
			[]string{"session", "event_type"},
		),

		InFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "in_flight",
				Help:      "Number of webhooks currently being processed",
			},
			[]string{"session"},
		),

		HMACValidationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "hmac_validation_total",
				Help:      "Total HMAC validation attempts by result",
			},
			[]string{"result"}, // "valid", "invalid", "disabled"
		),

		RetryTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "retry_total",
				Help:      "Total number of webhook processing retries",
			},
			[]string{"session", "event_type", "attempt"},
		),

		BodySizeBytes: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "automatiza",
				Subsystem: "webhook",
				Name:      "body_size_bytes",
				Help:      "Webhook payload size in bytes",
				Buckets:   prometheus.ExponentialBuckets(256, 2, 12), // 256B to 1MB
			},
			[]string{"event_type"},
		),
	}
}

// RecordReceived records a webhook being received.
func (m *WebhookMetrics) RecordReceived(session, eventType string, bodySize int) {
	m.ReceivedTotal.WithLabelValues(session, eventType).Inc()
	m.BodySizeBytes.WithLabelValues(eventType).Observe(float64(bodySize))
}

// RecordQueued records a webhook being enqueued for async processing.
func (m *WebhookMetrics) RecordQueued(session, eventType string) {
	m.QueuedTotal.WithLabelValues(session, eventType).Inc()
}

// RecordDuplicate records a duplicate webhook being rejected.
func (m *WebhookMetrics) RecordDuplicate(session, eventType string) {
	m.DuplicatesTotal.WithLabelValues(session, eventType).Inc()
}

// RecordHMACValidation records HMAC validation result.
func (m *WebhookMetrics) RecordHMACValidation(result string) {
	m.HMACValidationTotal.WithLabelValues(result).Inc()
}

// RecordProcessed records a webhook being successfully processed.
func (m *WebhookMetrics) RecordProcessed(session, eventType string, durationSeconds float64) {
	m.ProcessedTotal.WithLabelValues(session, eventType).Inc()
	m.ProcessingDuration.WithLabelValues(session, eventType).Observe(durationSeconds)
}

// RecordFailed records a webhook processing failure.
func (m *WebhookMetrics) RecordFailed(session, eventType, errorType string) {
	m.FailedTotal.WithLabelValues(session, eventType, errorType).Inc()
}

// RecordDLQ records a webhook being moved to dead letter queue.
func (m *WebhookMetrics) RecordDLQ(session, eventType string) {
	m.DLQTotal.WithLabelValues(session, eventType).Inc()
}

// RecordRetry records a retry attempt.
func (m *WebhookMetrics) RecordRetry(session, eventType string, attempt int) {
	m.RetryTotal.WithLabelValues(session, eventType, string(rune('0'+attempt))).Inc()
}

// IncrementInFlight increments the in-flight gauge for a session.
func (m *WebhookMetrics) IncrementInFlight(session string) {
	m.InFlight.WithLabelValues(session).Inc()
}

// DecrementInFlight decrements the in-flight gauge for a session.
func (m *WebhookMetrics) DecrementInFlight(session string) {
	m.InFlight.WithLabelValues(session).Dec()
}

// Global instance for convenience.
var defaultWebhookMetrics *WebhookMetrics

// InitWebhookMetrics initializes the global webhook metrics.
// Call this once during application startup.
func InitWebhookMetrics() *WebhookMetrics {
	if defaultWebhookMetrics == nil {
		defaultWebhookMetrics = NewWebhookMetrics()
	}
	return defaultWebhookMetrics
}

// GetWebhookMetrics returns the global webhook metrics instance.
// Returns nil if not initialized.
func GetWebhookMetrics() *WebhookMetrics {
	return defaultWebhookMetrics
}
