package connectrpc

import (
	"context"
	"fmt"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/services/mercadolivre"

	"connectrpc.com/connect"
)

// MercadoLivreHandler implements the affiliatesv1connect.MercadoLivreServiceHandler interface.
type MercadoLivreHandler struct {
	mlService mercadolivre.Service
}

// NewMercadoLivreHandler creates a new MercadoLivre handler.
func NewMercadoLivreHandler(mlService mercadolivre.Service) *MercadoLivreHandler {
	return &MercadoLivreHandler{
		mlService: mlService,
	}
}

// Connect saves ML affiliate account credentials for a user.
func (h *MercadoLivreHandler) Connect(
	ctx context.Context,
	req *connect.Request[affiliatesv1.MlConnectRequest],
) (*connect.Response[affiliatesv1.MlConnectResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Get fields from request
	cookie := req.Msg.GetCookie()
	csrfToken := req.Msg.GetCsrfToken()
	tag := req.Msg.GetTag()

	// Validate required fields
	if tag == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("tag is required"))
	}
	if cookie == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cookie is required"))
	}

	// Save credentials using the ML service (CSRF token is optional, auto-refreshed)
	err := h.mlService.SaveCredentials(ctx, user.ID, cookie, csrfToken, tag)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save ML credentials: %w", err))
	}

	msg := "Mercado Livre credentials saved successfully"
	return connect.NewResponse(&affiliatesv1.MlConnectResponse{
		Success: true,
		Message: &msg,
	}), nil
}

// CreateLink creates an affiliate link for a Mercado Livre product.
func (h *MercadoLivreHandler) CreateLink(
	ctx context.Context,
	req *connect.Request[affiliatesv1.MlCreateLinkRequest],
) (*connect.Response[affiliatesv1.MlCreateLinkResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	originURL := req.Msg.GetOriginUrl()
	if originURL == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("origin_url is required"))
	}

	// Create affiliate link using the ML service
	link, err := h.mlService.CreateAffiliateLink(ctx, originURL, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create affiliate link: %w", err))
	}

	// Build response with the generated URLs
	urls := []*affiliatesv1.MlLinkUrl{
		{
			LongUrl:  &link.OriginalURL,
			ShortUrl: &link.AffiliateLink,
		},
	}

	return connect.NewResponse(&affiliatesv1.MlCreateLinkResponse{
		Urls: urls,
	}), nil
}

// Preview retrieves product preview information from a Mercado Livre URL.
func (h *MercadoLivreHandler) Preview(
	ctx context.Context,
	req *connect.Request[affiliatesv1.MlPreviewRequest],
) (*connect.Response[affiliatesv1.MlPreviewResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	originURL := req.Msg.GetOriginUrl()
	if originURL == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("origin_url is required"))
	}

	// Get product preview using the ML service
	preview, err := h.mlService.GetProductPreview(ctx, originURL, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get product preview: %w", err))
	}

	if preview == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("product not found"))
	}

	// Convert domain model to proto
	protoPreview := &affiliatesv1.MlProductPreview{
		Title:    &preview.Title,
		Price:    &preview.Price,
		ImageUrl: &preview.Image,
	}

	// Set currency to BRL (default for Mercado Livre Brazil)
	currency := "BRL"
	protoPreview.Currency = &currency

	// Map optional fields if available
	if preview.Condition != nil {
		protoPreview.Condition = preview.Condition
	}
	if preview.SoldQuantity != nil {
		soldQty := int32(*preview.SoldQuantity)
		protoPreview.SoldQuantity = &soldQty
	}
	if preview.RatingAverage != nil {
		protoPreview.RatingAverage = preview.RatingAverage
	}
	if preview.ReviewsTotal != nil {
		reviewsTotal := int32(*preview.ReviewsTotal)
		protoPreview.ReviewsTotal = &reviewsTotal
	}
	if preview.OriginalPrice != nil {
		protoPreview.OriginalPrice = preview.OriginalPrice
	}
	if preview.DiscountPercent != nil {
		protoPreview.DiscountPercent = preview.DiscountPercent
	}
	if preview.Installments != nil {
		protoPreview.InstallmentsText = preview.Installments
	}

	return connect.NewResponse(&affiliatesv1.MlPreviewResponse{
		Preview: protoPreview,
	}), nil
}

// InvalidatePreviewCache invalidates the cached preview for a product.
// This forces a fresh fetch on the next GetProductPreview call.
func (h *MercadoLivreHandler) InvalidatePreviewCache(
	ctx context.Context,
	req *connect.Request[affiliatesv1.MlInvalidatePreviewCacheRequest],
) (*connect.Response[affiliatesv1.MlInvalidatePreviewCacheResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	originURL := req.Msg.GetOriginUrl()
	if originURL == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("origin_url is required"))
	}

	// Invalidate the preview cache
	err := h.mlService.InvalidatePreviewCache(ctx, originURL)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to invalidate preview cache: %w", err))
	}

	msg := "Preview cache invalidated successfully"
	return connect.NewResponse(&affiliatesv1.MlInvalidatePreviewCacheResponse{
		Success: true,
		Message: &msg,
	}), nil
}
