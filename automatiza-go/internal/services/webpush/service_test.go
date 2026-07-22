package webpush

import (
	"automatiza-go/internal/models"
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeRepo struct {
	subs    []models.WebPushSubscription
	deleted []string
}

func (f *fakeRepo) Upsert(context.Context, *models.WebPushSubscription) error { return nil }
func (f *fakeRepo) ListByUser(context.Context, string) ([]models.WebPushSubscription, error) {
	return f.subs, nil
}
func (f *fakeRepo) ListAll(context.Context) ([]models.WebPushSubscription, error) { return f.subs, nil }
func (f *fakeRepo) DeleteByEndpoint(_ context.Context, endpoint string) error {
	f.deleted = append(f.deleted, endpoint)
	return nil
}
func (f *fakeRepo) DeleteFailedBefore(context.Context, time.Time) (int64, error) { return 0, nil }
func (f *fakeRepo) CountByUser(context.Context, string) (int, error)             { return len(f.subs), nil }

// Sem VAPID => desligado e NotifyUser é no-op (nunca envia).
func TestWebPush_DisabledIsNoOp(t *testing.T) {
	svc := NewService(&fakeRepo{}, "", "", "", zap.NewNop())
	if svc.Enabled() {
		t.Fatal("expected disabled without VAPID keys")
	}
	if n := svc.NotifyUser(context.Background(), "u1", Notification{Title: "x"}); n != 0 {
		t.Fatalf("disabled NotifyUser should deliver 0, got %d", n)
	}
}

// Fail-open: subscription com chaves inválidas não deve entregar nada nem
// derrubar o chamador (o envio falha graciosamente, sem pânico propagado).
func TestWebPush_FailOpenOnBadSubscription(t *testing.T) {
	repo := &fakeRepo{subs: []models.WebPushSubscription{
		{Endpoint: "https://example.com/gone", P256dh: "not-valid", Auth: "not-valid"},
	}}
	svc := NewService(repo, "pub", "priv", "mailto:x@y.z", zap.NewNop())
	if !svc.Enabled() {
		t.Fatal("expected enabled with VAPID keys present")
	}
	if n := svc.NotifyUser(context.Background(), "u1", Notification{Title: "x"}); n != 0 {
		t.Fatalf("expected 0 delivered on invalid keys, got %d", n)
	}
}
