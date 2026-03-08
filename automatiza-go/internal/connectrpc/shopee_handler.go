package connectrpc

import (
	"context"
	"fmt"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/shopee"

	"connectrpc.com/connect"
)

// ShopeeHandler implements the affiliatesv1connect.ShopeeServiceHandler interface.
type ShopeeHandler struct {
	shopeeService shopee.Service
}

// NewShopeeHandler creates a new Shopee handler.
func NewShopeeHandler(shopeeService shopee.Service) *ShopeeHandler {
	return &ShopeeHandler{
		shopeeService: shopeeService,
	}
}

// SearchOffers searches for Shopee offers based on the provided filters.
func (h *ShopeeHandler) SearchOffers(
	ctx context.Context,
	req *connect.Request[affiliatesv1.SearchOffersRequest],
) (*connect.Response[affiliatesv1.SearchOffersResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Build search params from request
	params := models.ShopeeSearchParams{
		Limit: 20, // default
	}

	if req.Msg.Keyword != nil {
		params.Keyword = req.Msg.Keyword
	}

	if req.Msg.MinPrice != nil {
		params.MinPrice = req.Msg.MinPrice
	}

	if req.Msg.MaxPrice != nil {
		params.MaxPrice = req.Msg.MaxPrice
	}

	if req.Msg.MinDiscount != nil {
		minDiscount := float64(*req.Msg.MinDiscount)
		params.MinDiscount = &minDiscount
	}

	if req.Msg.SortBy != nil {
		sortBy := mapProtoSortBy(*req.Msg.SortBy)
		if sortBy != "" {
			params.SortBy = &sortBy
		}
	}

	if req.Msg.Limit != nil {
		params.Limit = int(*req.Msg.Limit)
	}

	// Call the service
	result, err := h.shopeeService.SearchOffers(ctx, user.ID, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to search offers: %w", err))
	}

	// Convert offers to proto format
	protoOffers := make([]*affiliatesv1.ShopeeOffer, 0, len(result.Offers))
	for _, offer := range result.Offers {
		protoOffer := &affiliatesv1.ShopeeOffer{
			ProductUrl:      offer.ProductURL,
			Title:           offer.Name,
			OriginalPrice:   offer.OriginalPrice,
			FinalPrice:      offer.Price,
			DiscountPercent: int32(offer.Discount),
		}

		if offer.Image != "" {
			protoOffer.ImageUrl = &offer.Image
		}

		if offer.ShopName != "" {
			protoOffer.StoreName = &offer.ShopName
		}

		if offer.CommissionRate > 0 {
			protoOffer.CommissionRate = &offer.CommissionRate
		}

		protoOffers = append(protoOffers, protoOffer)
	}

	return connect.NewResponse(&affiliatesv1.SearchOffersResponse{
		Offers: protoOffers,
	}), nil
}

// GenerateAffiliateLink generates an affiliate link for a Shopee product.
func (h *ShopeeHandler) GenerateAffiliateLink(
	ctx context.Context,
	req *connect.Request[affiliatesv1.GenerateAffiliateLinkRequest],
) (*connect.Response[affiliatesv1.GenerateAffiliateLinkResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	productURL := req.Msg.GetProductUrl()
	if productURL == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("product_url is required"))
	}

	// Call the service
	result, err := h.shopeeService.GenerateAffiliateLink(ctx, productURL, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate affiliate link: %w", err))
	}

	resp := &affiliatesv1.GenerateAffiliateLinkResponse{
		AffiliateUrl: result.AffiliateLink,
	}

	// Fetch product details if we have item/shop IDs
	if result.ItemID > 0 && result.ShopID > 0 {
		product, prodErr := h.shopeeService.GetProduct(ctx, result.ItemID, result.ShopID, user.ID)
		if prodErr == nil && product != nil {
			if product.Name != "" {
				resp.ProductTitle = &product.Name
			}
			if product.Price > 0 {
				resp.ProductPrice = &product.Price
			}
			if product.OriginalPrice > 0 {
				resp.ProductOriginalPrice = &product.OriginalPrice
			}
			imgURL := product.Image
			if imgURL == "" {
				imgURL = result.ImageURL
			}
			if imgURL != "" {
				resp.ProductImageUrl = &imgURL
			}
		}
	}

	return connect.NewResponse(resp), nil
}

// mapProtoSortBy converts proto SortBy enum to string for the service.
func mapProtoSortBy(sortBy affiliatesv1.SortBy) string {
	switch sortBy {
	case affiliatesv1.SortBy_SORT_BY_COMMISSION:
		return "commission"
	case affiliatesv1.SortBy_SORT_BY_PRICE_LOW:
		return "price_low"
	case affiliatesv1.SortBy_SORT_BY_PRICE_HIGH:
		return "price_high"
	case affiliatesv1.SortBy_SORT_BY_SALES:
		return "sales"
	case affiliatesv1.SortBy_SORT_BY_DISCOUNT:
		return "discount"
	case affiliatesv1.SortBy_SORT_BY_SAVING:
		return "saving"
	default:
		return ""
	}
}
