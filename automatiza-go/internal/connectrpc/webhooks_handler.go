package connectrpc

import (
	"context"
	"encoding/json"
	"fmt"

	webhooksv1 "automatiza-go/internal/gen/webhooks/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/repository/postgres"
	"automatiza-go/internal/services/kiwify"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

// WebhooksHandler implements the webhooksv1connect.WebhooksServiceHandler interface.
type WebhooksHandler struct {
	userRoleRepo   repository.UserRoleRepository
	kiwifyService  kiwify.Service
	webhookLogRepo postgres.WebhookLogRepository
}

// NewWebhooksHandler creates a new webhooks handler.
func NewWebhooksHandler(userRoleRepo repository.UserRoleRepository, kiwifyService kiwify.Service, webhookLogRepo postgres.WebhookLogRepository) *WebhooksHandler {
	return &WebhooksHandler{
		userRoleRepo:   userRoleRepo,
		kiwifyService:  kiwifyService,
		webhookLogRepo: webhookLogRepo,
	}
}

// ProcessKiwifyWebhook processes a Kiwify webhook (subscription events).
// Note: This endpoint does not require authentication as it receives external webhooks.
func (h *WebhooksHandler) ProcessKiwifyWebhook(
	ctx context.Context,
	req *connect.Request[webhooksv1.ProcessKiwifyWebhookRequest],
) (*connect.Response[webhooksv1.ProcessKiwifyWebhookResponse], error) {
	webhookData := req.Msg.GetWebhookData()
	if webhookData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("webhook_data is required"))
	}

	// Convert proto Struct to KiwifyWebhookPayload
	payload, err := structToKiwifyPayload(webhookData)
	if err != nil {
		errMsg := fmt.Sprintf("invalid webhook data: %v", err)
		return connect.NewResponse(&webhooksv1.ProcessKiwifyWebhookResponse{
			Success: false,
			Error:   &errMsg,
		}), nil
	}

	// Extract signature from headers if present
	signature := ""
	if req.Header() != nil {
		signature = req.Header().Get("X-Kiwify-Signature")
		if signature == "" {
			signature = req.Header().Get("X-Webhook-Signature")
		}
	}

	// Process the webhook using the Kiwify service
	result, err := h.kiwifyService.ProcessWebhook(ctx, payload, signature)
	if err != nil {
		errMsg := fmt.Sprintf("failed to process webhook: %v", err)
		// Log failed webhook
		if h.webhookLogRepo != nil {
			var email *string
			if payload.Customer != nil && payload.Customer.Email != "" {
				email = &payload.Customer.Email
			}
			eventType := payload.WebhookEventType
			logEntry, _ := h.webhookLogRepo.Create(ctx, "kiwify", &eventType, email, nil, nil)
			if logEntry != nil {
				_ = h.webhookLogRepo.UpdateStatus(ctx, logEntry.ID, "failed", &errMsg)
			}
		}
		return connect.NewResponse(&webhooksv1.ProcessKiwifyWebhookResponse{
			Success: false,
			Error:   &errMsg,
		}), nil
	}

	// Log successful webhook
	if h.webhookLogRepo != nil {
		var email *string
		if payload.Customer != nil && payload.Customer.Email != "" {
			email = &payload.Customer.Email
		}
		var plan *string
		if result.Plan != "" {
			plan = &result.Plan
		}
		logEntry, _ := h.webhookLogRepo.Create(ctx, "kiwify", &result.Event, email, plan, nil)
		if logEntry != nil {
			_ = h.webhookLogRepo.UpdateStatus(ctx, logEntry.ID, "processed", nil)
		}
	}

	return connect.NewResponse(&webhooksv1.ProcessKiwifyWebhookResponse{
		Success:   result.Success,
		Message:   &result.Message,
		EventType: &result.Event,
	}), nil
}

// SimulateKiwifyWebhook simulates a Kiwify webhook for testing purposes (admin only).
func (h *WebhooksHandler) SimulateKiwifyWebhook(
	ctx context.Context,
	req *connect.Request[webhooksv1.SimulateKiwifyWebhookRequest],
) (*connect.Response[webhooksv1.SimulateKiwifyWebhookResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Check if user is admin
	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, user.ID)
	if err != nil || !isAdmin {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
	}

	email := req.Msg.GetEmail()
	event := req.Msg.GetEvent()
	product := req.Msg.GetProduct()

	if email == "" || event == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email and event are required"))
	}

	// Default product name if not provided
	if product == "" {
		product = "Produto Teste"
	}

	// Create a simulated Kiwify payload
	simulatedPayload := &models.KiwifyWebhookPayload{
		Customer: &models.KiwifyCustomer{
			Email:    email,
			FullName: email[:min(len(email), 10)], // Use part of email as name
		},
		Product: &models.KiwifyProduct{
			ProductName: product,
			ProductID:   "TEST123",
		},
		WebhookEventType: event,
		CheckoutLink:     fmt.Sprintf("https://pay.kiwify.com.br/TEST123"),
	}

	// Process the simulated webhook
	result, err := h.kiwifyService.ProcessWebhook(ctx, simulatedPayload, "")
	if err != nil {
		errMsg := fmt.Sprintf("simulation failed: %v", err)
		// Log failed simulation
		if h.webhookLogRepo != nil {
			logEntry, _ := h.webhookLogRepo.Create(ctx, "kiwify", &event, &email, nil, nil)
			if logEntry != nil {
				_ = h.webhookLogRepo.UpdateStatus(ctx, logEntry.ID, "failed", &errMsg)
			}
		}
		return connect.NewResponse(&webhooksv1.SimulateKiwifyWebhookResponse{
			Success: false,
			Error:   &errMsg,
		}), nil
	}

	// Log successful simulation
	if h.webhookLogRepo != nil {
		var plan *string
		if result.Plan != "" {
			plan = &result.Plan
		}
		logEntry, _ := h.webhookLogRepo.Create(ctx, "kiwify", &result.Event, &email, plan, nil)
		if logEntry != nil {
			_ = h.webhookLogRepo.UpdateStatus(ctx, logEntry.ID, "processed", nil)
		}
	}

	eventDetails := fmt.Sprintf("Event: %s, User: %s, Plan: %s", result.Event, result.UserID, result.Plan)
	return connect.NewResponse(&webhooksv1.SimulateKiwifyWebhookResponse{
		Success:      result.Success,
		Message:      &result.Message,
		EventDetails: &eventDetails,
	}), nil
}

// structToKiwifyPayload converts a proto Struct to KiwifyWebhookPayload.
func structToKiwifyPayload(s *structpb.Struct) (*models.KiwifyWebhookPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("webhook_data is nil")
	}

	// Convert Struct to JSON bytes
	jsonBytes, err := s.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	// Unmarshal into KiwifyWebhookPayload
	var payload models.KiwifyWebhookPayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into KiwifyWebhookPayload: %w", err)
	}

	return &payload, nil
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
