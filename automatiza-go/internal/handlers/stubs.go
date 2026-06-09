package handlers

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/services/coupons"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // diagnostic only: detect whether Kiwify signs with HMAC-SHA1
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SettingsHandler handles settings operations
type SettingsHandler struct{ deps *Dependencies }

func NewSettingsHandler(deps *Dependencies) *SettingsHandler { return &SettingsHandler{deps: deps} }

func (h *SettingsHandler) Get(c *gin.Context) {
	settings, err := h.deps.SettingsRepo.GetAll(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar configurações")
		return
	}
	middleware.RespondSuccess(c, settings)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var req models.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if err := h.deps.SettingsRepo.Update(c.Request.Context(), req.Key, req.Value); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar configuração")
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"updated": true})
}

// HistoryHandler handles action history operations
type HistoryHandler struct{ deps *Dependencies }

func NewHistoryHandler(deps *Dependencies) *HistoryHandler { return &HistoryHandler{deps: deps} }

func (h *HistoryHandler) List(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	history, err := h.deps.ActionHistoryRepo.GetUserHistory(c.Request.Context(), user.ID, 50, 0)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar histórico")
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"history": history, "total": len(history)})
}

// ContactsHandler handles contact operations
type ContactsHandler struct{ deps *Dependencies }

func NewContactsHandler(deps *Dependencies) *ContactsHandler { return &ContactsHandler{deps: deps} }

func (h *ContactsHandler) Extract(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.ContactExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil {
		// S24: Capture instance lookup errors
		observability.LogAndCapture(h.deps.Logger, sentry.LevelWarning,
			"Failed to get instance by user and name",
			fmt.Errorf("GetInstanceByUserAndName user=%s instance=%s: %w", user.ID, req.InstanceName, err),
			map[string]string{
				"module":       "contacts-handler",
				"userID":       user.ID,
				"instanceName": req.InstanceName,
			})
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar instância")
		return
	}
	if instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}
	participants, err := h.deps.WhatsAppProvider.GetGroupParticipants(c.Request.Context(), req.InstanceName, req.GroupID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao extrair contatos")
		return
	}
	// Convert to Contact format
	contacts := make([]models.Contact, len(participants))
	for i, p := range participants {
		isAdmin := p.IsAdmin || p.IsSuperAdmin
		contacts[i] = models.Contact{
			ID:      p.ID,
			IsAdmin: &isAdmin,
		}
	}
	middleware.RespondSuccess(c, models.ContactExtractResponse{Contacts: contacts, Total: len(contacts)})
}

// ShopeeHandler handles Shopee operations
type ShopeeHandler struct{ deps *Dependencies }

func NewShopeeHandler(deps *Dependencies) *ShopeeHandler { return &ShopeeHandler{deps: deps} }

func (h *ShopeeHandler) SearchOffers(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var params models.ShopeeSearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	if h.deps.ShopeeService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço Shopee não configurado")
		return
	}

	result, err := h.deps.ShopeeService.SearchOffers(c.Request.Context(), user.ID, params)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar ofertas: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

func (h *ShopeeHandler) GenerateAffiliateLink(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.ShopeeGenerateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "productUrl é obrigatório")
		return
	}

	if h.deps.ShopeeService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço Shopee não configurado")
		return
	}

	result, err := h.deps.ShopeeService.GenerateAffiliateLink(c.Request.Context(), req.ProductURL, user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao gerar link: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

// MLHandler handles Mercado Livre operations
type MLHandler struct{ deps *Dependencies }

func NewMLHandler(deps *Dependencies) *MLHandler { return &MLHandler{deps: deps} }

func (h *MLHandler) CreateLink(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.MLCreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "productUrl é obrigatório")
		return
	}

	if h.deps.MLService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço ML não configurado")
		return
	}

	result, err := h.deps.MLService.CreateAffiliateLink(c.Request.Context(), req.ProductURL, user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao criar link: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

func (h *MLHandler) Preview(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.MLPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "productUrl é obrigatório")
		return
	}

	if h.deps.MLService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço ML não configurado")
		return
	}

	result, err := h.deps.MLService.GetProductPreview(c.Request.Context(), req.ProductURL, user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar preview: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

func (h *MLHandler) Connect(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.MLConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	if h.deps.MLService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço ML não configurado")
		return
	}

	err := h.deps.MLService.SaveCredentials(c.Request.Context(), user.ID, req.Cookie, req.CSRF, req.Tag)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao conectar: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"connected": true})
}

// JobsHandler handles job operations
type JobsHandler struct{ deps *Dependencies }

func NewJobsHandler(deps *Dependencies) *JobsHandler { return &JobsHandler{deps: deps} }

func (h *JobsHandler) ProcessScheduled(c *gin.Context) {
	if h.deps.JobsService == nil {
		middleware.RespondSuccess(c, models.ProcessScheduledJobsResult{ProcessedCount: 0})
		return
	}

	result, err := h.deps.JobsService.ProcessScheduledJobs(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao processar jobs: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, models.ProcessScheduledJobsResult{
		ProcessedCount: result.Processed,
		FailedCount:    result.Failed,
	})
}

func (h *JobsHandler) QueueStatus(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	if h.deps.JobsService == nil {
		middleware.RespondSuccess(c, models.QueueStatus{Pending: 0, Processing: 0})
		return
	}

	status, err := h.deps.JobsService.GetQueueStatus(c.Request.Context(), user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar status: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, models.QueueStatus{
		Pending:    status.Pending,
		Processing: status.Running,
	})
}

// WebhooksHandler handles webhook operations
type WebhooksHandler struct{ deps *Dependencies }

func NewWebhooksHandler(deps *Dependencies) *WebhooksHandler { return &WebhooksHandler{deps: deps} }

func (h *WebhooksHandler) WhatsApp(c *gin.Context) {
	var msg models.IncomingMessage
	if err := c.ShouldBindJSON(&msg); err != nil {
		// Log but don't fail - webhooks should always return 200
		h.deps.Logger.Warn("Failed to parse WhatsApp webhook", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if h.deps.MonitoringService != nil {
		// S67: Use SafeGo for panic recovery in monitoring goroutine
		observability.SafeGo("monitoring-processor", func() {
			ctx := context.Background()
			if err := h.deps.MonitoringService.ProcessIncomingMessage(ctx, &msg); err != nil {
				observability.LogAndCapture(h.deps.Logger, sentry.LevelError,
					"Failed to process WhatsApp message",
					fmt.Errorf("process incoming message instance=%s: %w", msg.InstanceName, err),
					map[string]string{
						"module":       "webhooks-handler",
						"instanceName": msg.InstanceName,
					})
			}
		})
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *WebhooksHandler) Kiwify(c *gin.Context) {
	// Log incoming webhook for debugging
	h.deps.Logger.Info("Kiwify webhook received",
		zap.String("method", c.Request.Method),
		zap.String("contentType", c.ContentType()),
	)

	// Read the raw body (needed for HMAC) then unmarshal — ShouldBindJSON would
	// consume the stream and leave nothing to sign.
	rawBody, readErr := io.ReadAll(c.Request.Body)
	if readErr != nil {
		h.deps.Logger.Error("Failed to read Kiwify webhook body", zap.Error(readErr))
		c.JSON(http.StatusBadRequest, models.KiwifyWebhookResult{
			Success: false,
			Message: "Failed to read webhook body",
		})
		return
	}

	var payload models.KiwifyWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		h.deps.Logger.Error("Failed to parse Kiwify webhook payload",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, models.KiwifyWebhookResult{
			Success: false,
			Message: "Invalid webhook payload: " + err.Error(),
		})
		return
	}

	// Get signature from query or body
	signature := c.Query("signature")
	if signature == "" {
		signature = payload.Signature
	}

	// OBSERVE-ONLY: log whether the signature matches HMAC-SHA256/SHA1 of the
	// raw body with the configured token. Does NOT reject — this lets us confirm
	// Kiwify's actual algorithm from real webhooks before turning on strict
	// validation in a follow-up (turning strict on blindly with the wrong algo
	// would reject every purchase).
	h.observeKiwifySignature(rawBody, signature)

	// Check if Kiwify service is configured
	if h.deps.KiwifyService == nil {
		h.deps.Logger.Warn("Kiwify service not configured")
		c.JSON(http.StatusOK, models.KiwifyWebhookResult{
			Success: true,
			Message: "Webhook received but Kiwify service not configured",
		})
		return
	}

	// Process the webhook
	result, err := h.deps.KiwifyService.ProcessWebhook(c.Request.Context(), &payload, signature)
	if err != nil {
		h.deps.Logger.Error("Failed to process Kiwify webhook",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.KiwifyWebhookResult{
			Success: false,
			Message: "Failed to process webhook: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// observeKiwifySignature logs (without rejecting) whether the received
// signature matches HMAC-SHA256 or HMAC-SHA1 of the raw body with the
// configured Kiwify token. Diagnostic step toward strict validation: the
// signature length + which algo matches reveal Kiwify's scheme from real
// webhooks. sha256-hex = 64 chars, sha1-hex = 40 chars.
func (h *WebhooksHandler) observeKiwifySignature(rawBody []byte, signature string) {
	if h.deps.Config == nil {
		return
	}
	token := h.deps.Config.KiwifyWebhookToken
	if token == "" || signature == "" {
		h.deps.Logger.Warn("Kiwify signature observe: skipped",
			zap.Bool("hasToken", token != ""),
			zap.Bool("hasSignature", signature != ""))
		return
	}

	sha256mac := hmac.New(sha256.New, []byte(token))
	sha256mac.Write(rawBody)
	sha256hex := hex.EncodeToString(sha256mac.Sum(nil))

	sha1mac := hmac.New(sha1.New, []byte(token))
	sha1mac.Write(rawBody)
	sha1hex := hex.EncodeToString(sha1mac.Sum(nil))

	matched := "none"
	switch {
	case hmac.Equal([]byte(strings.ToLower(signature)), []byte(sha256hex)):
		matched = "sha256"
	case hmac.Equal([]byte(strings.ToLower(signature)), []byte(sha1hex)):
		matched = "sha1"
	}

	h.deps.Logger.Info("Kiwify signature observe",
		zap.String("matched", matched),
		zap.Int("receivedSigLen", len(signature)),
		zap.Int("bodyLen", len(rawBody)))
}

func (h *WebhooksHandler) KiwifySimulate(c *gin.Context) {
	// Simulation request structure
	type SimulateRequest struct {
		Email   string `json:"email" binding:"required,email"`
		Event   string `json:"evento" binding:"required"`
		Product string `json:"produto"`
		Token   string `json:"token"`
	}

	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Email and event are required")
		return
	}

	// Validate token if configured
	if h.deps.KiwifyService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Kiwify service not configured")
		return
	}

	// Create simulated payload
	productName := req.Product
	if productName == "" {
		productName = "Test Product"
	}

	payload := &models.KiwifyWebhookPayload{
		Customer: &models.KiwifyCustomer{
			Email:    req.Email,
			FullName: strings.Split(req.Email, "@")[0],
		},
		Product: &models.KiwifyProduct{
			ProductName: productName,
			ProductID:   "TEST123",
		},
		WebhookEventType: req.Event,
		CheckoutLink:     "https://pay.kiwify.com.br/TEST123",
	}

	// Process as a real webhook
	result, err := h.deps.KiwifyService.ProcessWebhook(c.Request.Context(), payload, req.Token)
	if err != nil {
		h.deps.Logger.Error("Failed to process simulated Kiwify webhook",
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Failed to process webhook: "+err.Error())
		return
	}

	middleware.RespondSuccess(c, result)
}

// RedirectHandler handles redirect operations
type RedirectHandler struct{ deps *Dependencies }

func NewRedirectHandler(deps *Dependencies) *RedirectHandler { return &RedirectHandler{deps: deps} }

func (h *RedirectHandler) Process(c *gin.Context) {
	var req models.RedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "url é obrigatório")
		return
	}
	middleware.RespondSuccess(c, models.RedirectResponse{OriginalURL: req.URL, FinalURL: req.URL})
}

// InstancesHandler handles instance webhook operations
type InstancesHandler struct{ deps *Dependencies }

func NewInstancesHandler(deps *Dependencies) *InstancesHandler { return &InstancesHandler{deps: deps} }

func (h *InstancesHandler) SyncWebhook(c *gin.Context) {
	instanceName := c.Param("id")

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	// Get instance to verify it exists
	instance, err := h.deps.InstanceRepo.GetInstanceByName(c.Request.Context(), instanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	// Build webhook URL — use /waha endpoint (new handler with Asynq + SSE pipeline)
	webhookURL := h.deps.Config.BackendPublicURL + "/api/v1/webhooks/waha"
	events := []string{"message", "message.ack", "session.status"}

	err = h.deps.WhatsAppProvider.SetWebhook(c.Request.Context(), whatsapp.SetWebhookInput{
		SessionName: instance.InstanceID,
		URL:         webhookURL,
		Events:      events,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao sincronizar webhook: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"synced": true, "instanceName": instance.InstanceID})
}

func (h *InstancesHandler) SyncAllWebhooks(c *gin.Context) {
	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	// Get all connected instances
	instances, err := h.deps.InstanceRepo.GetAllConnectedInstances(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar instâncias: "+err.Error())
		return
	}

	webhookURL := h.deps.Config.BackendPublicURL + "/api/v1/webhooks/waha"
	events := []string{"message", "message.ack", "session.status"}

	synced := 0
	failed := 0
	for _, instance := range instances {
		err := h.deps.WhatsAppProvider.SetWebhook(c.Request.Context(), whatsapp.SetWebhookInput{
			SessionName: instance.InstanceID,
			URL:         webhookURL,
			Events:      events,
		})
		if err != nil {
			h.deps.Logger.Warn("Failed to sync webhook",
				zap.String("instanceID", instance.InstanceID),
				zap.Error(err))
			failed++
		} else {
			synced++
		}
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"synced": synced,
		"failed": failed,
		"total":  len(instances),
	})
}

// AdminHandler handles admin operations
type AdminHandler struct{ deps *Dependencies }

func NewAdminHandler(deps *Dependencies) *AdminHandler { return &AdminHandler{deps: deps} }

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	admin := middleware.MustGetAuthenticatedUser(c)
	userID := c.Param("userId")

	if h.deps.UserService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço de usuários não configurado")
		return
	}

	result, err := h.deps.UserService.ResetUserPassword(c.Request.Context(), userID, admin.ID, admin.Email)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao resetar senha: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

func (h *AdminHandler) Observability(c *gin.Context) {
	middleware.RespondSuccess(c, map[string]interface{}{"status": "ok"})
}

func (h *AdminHandler) EnsureWebhooks(c *gin.Context) {
	middleware.RespondSuccess(c, map[string]interface{}{"ensured": true})
}

func (h *AdminHandler) ShopeeAffiliateAudit(c *gin.Context) {
	middleware.RespondSuccess(c, map[string]interface{}{"audit": []interface{}{}})
}

func (h *AdminHandler) SystemEvents(c *gin.Context) {
	events, err := h.deps.SystemEventRepo.GetRecent(c.Request.Context(), 100)
	if err != nil {
		// S27: Capture system events retrieval errors
		observability.LogAndCapture(h.deps.Logger, sentry.LevelWarning,
			"Failed to get recent system events",
			fmt.Errorf("GetRecent system events: %w", err),
			map[string]string{
				"module": "admin-handler",
			})
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar eventos do sistema")
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"events": events})
}

// CouponsHandler handles coupon operations
type CouponsHandler struct{ deps *Dependencies }

func NewCouponsHandler(deps *Dependencies) *CouponsHandler { return &CouponsHandler{deps: deps} }

func (h *CouponsHandler) List(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	if h.deps.CouponsService == nil {
		middleware.RespondSuccess(c, map[string]interface{}{"coupons": []interface{}{}, "total": 0})
		return
	}
	coupons, err := h.deps.CouponsService.GetUserCoupons(c.Request.Context(), user.ID, nil)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar cupons")
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"coupons": coupons, "total": len(coupons)})
}

func (h *CouponsHandler) Get(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	if h.deps.CouponsService == nil {
		middleware.RespondError(c, http.StatusNotFound, "Cupom não encontrado")
		return
	}
	coupon, err := h.deps.CouponsService.GetCouponByID(c.Request.Context(), c.Param("id"), user.ID)
	if err != nil || coupon == nil {
		middleware.RespondError(c, http.StatusNotFound, "Cupom não encontrado")
		return
	}
	middleware.RespondSuccess(c, coupon)
}

func (h *CouponsHandler) GetByStore(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	if h.deps.CouponsService == nil {
		middleware.RespondSuccess(c, map[string]interface{}{"coupons": []interface{}{}, "total": 0})
		return
	}
	coupons, err := h.deps.CouponsService.GetCouponsByStore(c.Request.Context(), user.ID, c.Param("store"))
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar cupons")
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"coupons": coupons, "total": len(coupons)})
}

func (h *CouponsHandler) Create(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if h.deps.CouponsService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço de cupons não configurado")
		return
	}
	result, err := h.deps.CouponsService.CreateCoupon(c.Request.Context(), user.ID, &coupons.CreateCouponData{
		Code:          req.Code,
		Title:         req.Title,
		Store:         req.Store,
		AffiliateLink: stringVal(req.AffiliateLink),
		ExpiresAt:     stringVal(req.ExpiresAt),
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao criar cupom: "+err.Error())
		return
	}
	middleware.RespondCreated(c, result)
}

func (h *CouponsHandler) Update(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	var req models.UpdateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if h.deps.CouponsService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço de cupons não configurado")
		return
	}
	result, err := h.deps.CouponsService.UpdateCoupon(c.Request.Context(), c.Param("id"), user.ID, &coupons.UpdateCouponData{
		Code:     req.Code,
		Title:    req.Title,
		IsActive: req.IsActive,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar cupom: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, result)
}

func (h *CouponsHandler) Delete(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	if h.deps.CouponsService == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "Serviço de cupons não configurado")
		return
	}
	if err := h.deps.CouponsService.DeleteCoupon(c.Request.Context(), c.Param("id"), user.ID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao excluir cupom: "+err.Error())
		return
	}
	middleware.RespondSuccess(c, map[string]interface{}{"deleted": true})
}

// Helper function to get string value from pointer
func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// LoadBalancerHandler handles load balancer operations
type LoadBalancerHandler struct{ deps *Dependencies }

func NewLoadBalancerHandler(deps *Dependencies) *LoadBalancerHandler {
	return &LoadBalancerHandler{deps: deps}
}

func (h *LoadBalancerHandler) Stats(c *gin.Context) {
	if h.deps.LBService == nil {
		middleware.RespondSuccess(c, models.LoadBalancerStats{Servers: []models.ServerStats{}})
		return
	}

	stats, err := h.deps.LBService.GetServerStats(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar stats: "+err.Error())
		return
	}

	serverStats := make([]models.ServerStats, 0, len(stats.Servers))
	for _, s := range stats.Servers {
		serverStats = append(serverStats, models.ServerStats{
			ServerID:    s.ID,
			URL:         s.URL,
			TotalUsers:  s.UserCount,
			MaxUsers:    s.MaxUsers,
			Utilization: s.Utilization,
			IsActive:    s.IsActive,
		})
	}

	middleware.RespondSuccess(c, models.LoadBalancerStats{
		TotalUsers: stats.TotalUsers,
		Servers:    serverStats,
	})
}

func (h *LoadBalancerHandler) GetUserServer(c *gin.Context) {
	userID := c.Param("userId")

	if h.deps.LBService == nil {
		middleware.RespondSuccess(c, map[string]interface{}{"server": nil})
		return
	}

	server, err := h.deps.LBService.GetServerForUser(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar servidor: "+err.Error())
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{"server": server})
}

func (h *LoadBalancerHandler) Migrate(c *gin.Context) {
	var req models.MigrateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	if h.deps.LBService == nil {
		middleware.RespondSuccess(c, models.MigrateUserResponse{Success: false, Message: "Serviço não configurado"})
		return
	}

	err := h.deps.LBService.MigrateUser(c.Request.Context(), req.UserID, req.TargetServerID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha na migração: "+err.Error())
		return
	}

	middleware.RespondSuccess(c, models.MigrateUserResponse{Success: true, Message: "Usuário migrado com sucesso"})
}
