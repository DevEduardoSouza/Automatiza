package connectrpc

import (
	"context"
	"fmt"
	"sort"

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

// ListCategories devolve o catálogo curado de grupos de categoria da
// Shopee (nome + IDs oficiais) que a UI usa para montar regras de cupom
// por categoria. Reusa o GetCategories que já existia — não há método
// novo na interface do serviço.
//
// O mesmo ID pode pertencer a MAIS DE UM grupo: o catálogo é um mapeamento
// curado, não uma partição. Por isso a UI persiste apenas IDs e nunca
// tenta reconstruir os nomes selecionados a partir da união gravada.
func (h *ShopeeHandler) ListCategories(
	ctx context.Context,
	_ *connect.Request[affiliatesv1.ListCategoriesRequest],
) (*connect.Response[affiliatesv1.ListCategoriesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	categories, err := h.shopeeService.GetCategories(ctx, nil)
	if err != nil {
		// Erro sanitizado: o catálogo é estático, qualquer falha aqui é
		// interna e não deve vazar detalhe do serviço para o cliente.
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list categories"))
	}

	groups := make([]*affiliatesv1.ShopeeCategoryGroup, 0, len(categories))
	for _, cat := range categories {
		if cat.Name == "" || len(cat.IDs) == 0 {
			continue
		}
		ids := dedupSortedInt64(cat.IDs)
		if len(ids) == 0 {
			continue
		}
		groups = append(groups, &affiliatesv1.ShopeeCategoryGroup{
			Name: cat.Name,
			Ids:  ids,
		})
	}
	// GetCategories itera um map — a ordem é aleatória a cada chamada.
	// Ordenar por nome dá uma lista estável para a UI.
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	return connect.NewResponse(&affiliatesv1.ListCategoriesResponse{Groups: groups}), nil
}

// dedupSortedInt64 devolve os valores únicos, em ordem crescente, num
// slice novo (não mexe na entrada).
func dedupSortedInt64(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
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
