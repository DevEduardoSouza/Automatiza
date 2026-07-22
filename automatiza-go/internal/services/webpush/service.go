// Package webpush envia notificações Web Push (PWA) via VAPID/aes128gcm.
//
// TUDO aqui é fail-open: qualquer erro é logado e engolido, nunca propaga para
// o chamador. Os ganchos de evento chamam NotifyUser dentro de observability.SafeGo,
// então uma falha de push jamais derruba o fluxo de envio/webhook.
package webpush

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"io"

	webpushgo "github.com/SherClockHolmes/webpush-go"
	"go.uber.org/zap"
)

// Notification é o payload entregue ao service worker (showNotification).
type Notification struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Service struct {
	repo       repository.WebPushSubscriptionRepository
	publicKey  string
	privateKey string
	subscriber string
	logger     *zap.Logger
}

func NewService(repo repository.WebPushSubscriptionRepository, publicKey, privateKey, subscriber string, logger *zap.Logger) *Service {
	if subscriber == "" {
		subscriber = "mailto:suporte@automatisagrupos.com"
	}
	return &Service{
		repo:       repo,
		publicKey:  publicKey,
		privateKey: privateKey,
		subscriber: subscriber,
		logger:     logger.Named("webpush"),
	}
}

// Enabled reports whether Web Push is configured (VAPID keys present).
func (s *Service) Enabled() bool {
	return s != nil && s.privateKey != "" && s.publicKey != ""
}

// PublicKey returns the VAPID public key (safe to expose to the frontend).
func (s *Service) PublicKey() string {
	if s == nil {
		return ""
	}
	return s.publicKey
}

// NotifyUser sends a notification to every device of a user. Fail-open:
// returns how many were delivered; logs and continues on errors; deletes the
// subscription on 404/410 (gone). A nil/disabled service is a no-op.
func (s *Service) NotifyUser(ctx context.Context, userID string, n Notification) int {
	if !s.Enabled() || s.repo == nil {
		return 0
	}
	subs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Warn("list subscriptions failed", zap.String("userID", userID), zap.Error(err))
		return 0
	}
	payload, err := json.Marshal(n)
	if err != nil {
		return 0
	}
	delivered := 0
	for i := range subs {
		if s.sendOne(ctx, &subs[i], payload) {
			delivered++
		}
	}
	return delivered
}

// sendOne delivers to a single subscription. Returns true on success; deletes
// the subscription on 404/410 (endpoint gone). Never panics.
func (s *Service) sendOne(ctx context.Context, sub *models.WebPushSubscription, payload []byte) bool {
	resp, err := webpushgo.SendNotification(payload, &webpushgo.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpushgo.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpushgo.Options{
		Subscriber:      s.subscriber,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		TTL:             3600,
	})
	if err != nil {
		s.logger.Warn("send failed", zap.Error(err))
		return false
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		// Endpoint gone — remove it so we stop trying.
		if delErr := s.repo.DeleteByEndpoint(ctx, sub.Endpoint); delErr != nil {
			s.logger.Warn("delete gone subscription failed", zap.Error(delErr))
		}
		return false
	default:
		s.logger.Warn("push rejected", zap.Int("status", resp.StatusCode))
		return false
	}
}
