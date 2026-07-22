package connectrpc

import (
	"context"
	"fmt"
	"strings"

	webpushv1 "automatiza-go/internal/gen/webpush/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/webpush"

	"connectrpc.com/connect"
)

// WebPushHandler implements webpushv1connect.WebPushServiceHandler.
type WebPushHandler struct {
	svc  *webpush.Service
	repo repository.WebPushSubscriptionRepository
}

// NewWebPushHandler creates the Web Push ConnectRPC handler.
func NewWebPushHandler(svc *webpush.Service, repo repository.WebPushSubscriptionRepository) *WebPushHandler {
	return &WebPushHandler{svc: svc, repo: repo}
}

// GetConfig returns whether push is enabled and the VAPID public key.
// Authenticated but harmless: the public key is not a secret.
func (h *WebPushHandler) GetConfig(
	ctx context.Context,
	_ *connect.Request[webpushv1.GetConfigRequest],
) (*connect.Response[webpushv1.GetConfigResponse], error) {
	if GetUserFromContext(ctx) == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	return connect.NewResponse(&webpushv1.GetConfigResponse{
		Enabled:        h.svc.Enabled(),
		VapidPublicKey: h.svc.PublicKey(),
	}), nil
}

// Subscribe stores (or refreshes) a browser push subscription for the caller.
func (h *WebPushHandler) Subscribe(
	ctx context.Context,
	req *connect.Request[webpushv1.SubscribeRequest],
) (*connect.Response[webpushv1.SubscribeResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	m := req.Msg
	if strings.TrimSpace(m.Endpoint) == "" || m.P256Dh == "" || m.Auth == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("endpoint, p256dh and auth are required"))
	}
	ua := m.UserAgent
	if len(ua) > 400 {
		ua = ua[:400]
	}
	if err := h.repo.Upsert(ctx, &models.WebPushSubscription{
		UserID:    user.ID,
		Endpoint:  m.Endpoint,
		P256dh:    m.P256Dh,
		Auth:      m.Auth,
		UserAgent: ua,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to store subscription: %w", err))
	}
	return connect.NewResponse(&webpushv1.SubscribeResponse{}), nil
}

// Unsubscribe removes a subscription by endpoint.
func (h *WebPushHandler) Unsubscribe(
	ctx context.Context,
	req *connect.Request[webpushv1.UnsubscribeRequest],
) (*connect.Response[webpushv1.UnsubscribeResponse], error) {
	if GetUserFromContext(ctx) == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if req.Msg.Endpoint != "" {
		if err := h.repo.DeleteByEndpoint(ctx, req.Msg.Endpoint); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove subscription: %w", err))
		}
	}
	return connect.NewResponse(&webpushv1.UnsubscribeResponse{}), nil
}

// SendTest delivers a test notification to the caller's own devices.
func (h *WebPushHandler) SendTest(
	ctx context.Context,
	_ *connect.Request[webpushv1.SendTestRequest],
) (*connect.Response[webpushv1.SendTestResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	delivered := h.svc.NotifyUser(ctx, user.ID, webpush.Notification{
		Title: "Notificações ativadas ✅",
		Body:  "Você vai receber avisos importantes aqui, mesmo com o app fechado.",
		URL:   "/dashboard",
	})
	return connect.NewResponse(&webpushv1.SendTestResponse{Delivered: int32(delivered)}), nil
}
