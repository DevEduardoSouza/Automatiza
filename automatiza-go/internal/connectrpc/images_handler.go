package connectrpc

import (
	"context"
	"errors"

	imagesv1 "automatiza-go/internal/gen/images/v1"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// ImagesHandler implements the imagesv1connect.ImagesServiceHandler interface.
// This handler provides operations for generating marketing images.
type ImagesHandler struct {
	logger *zap.Logger
	// TODO: Add image generation service dependency when implemented
	// imageService images.Service
}

// NewImagesHandler creates a new images handler.
func NewImagesHandler(logger *zap.Logger) *ImagesHandler {
	return &ImagesHandler{
		logger: logger,
	}
}

// GenerateOfferImage generates an offer image with product information.
// TODO: Integrate with image generation service (e.g., canvas rendering, external API).
func (h *ImagesHandler) GenerateOfferImage(
	ctx context.Context,
	req *connect.Request[imagesv1.GenerateOfferImageRequest],
) (*connect.Response[imagesv1.GenerateOfferImageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	title := req.Msg.GetTitle()
	price := req.Msg.GetPrice()
	discount := req.Msg.GetDiscount()
	imageURL := req.Msg.GetImageUrl()
	store := req.Msg.GetStore()

	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}
	if price <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("price must be greater than 0"))
	}

	h.logger.Debug("GenerateOfferImage called",
		zap.String("userID", user.ID),
		zap.String("title", title),
		zap.Float64("price", price),
		zap.Float64("discount", discount),
		zap.String("store", store),
		zap.Bool("hasImageURL", imageURL != ""),
	)

	// TODO: Implement real image generation
	// This would typically:
	// 1. Download product image if URL provided
	// 2. Generate a styled offer image with price, discount badge, etc.
	// 3. Upload to storage (S3/MinIO) and return URL
	// 4. Or return base64 encoded image

	// For now, return a stub response indicating the feature is not yet implemented
	errorMsg := "Image generation service not yet implemented"
	return connect.NewResponse(&imagesv1.GenerateOfferImageResponse{
		Success: false,
		Error:   &errorMsg,
	}), nil
}

// GenerateOfferPostImage generates a social media post image for an offer.
// TODO: Integrate with image generation service.
func (h *ImagesHandler) GenerateOfferPostImage(
	ctx context.Context,
	req *connect.Request[imagesv1.GenerateOfferPostImageRequest],
) (*connect.Response[imagesv1.GenerateOfferPostImageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	productImageURL := req.Msg.GetProductImageUrl()
	topText := req.Msg.GetTopText()
	bottomText := req.Msg.GetBottomText()
	backgroundColor := req.Msg.GetBackgroundColor()
	layout := req.Msg.GetLayout()
	template := req.Msg.GetTemplate()

	if productImageURL == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_image_url is required"))
	}
	if topText == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("top_text is required"))
	}

	h.logger.Debug("GenerateOfferPostImage called",
		zap.String("userID", user.ID),
		zap.String("productImageURL", productImageURL),
		zap.String("topText", topText),
		zap.String("bottomText", bottomText),
		zap.String("backgroundColor", backgroundColor),
		zap.Int32("layout", int32(layout)),
		zap.Int32("template", int32(template)),
	)

	// TODO: Implement real post image generation
	// This would typically:
	// 1. Download the product image
	// 2. Apply the selected template and layout
	// 3. Add text overlays (top and bottom)
	// 4. Apply background color
	// 5. Generate final composite image
	// 6. Upload to storage and return URL or base64

	// For now, return a stub response indicating the feature is not yet implemented
	errorMsg := "Post image generation service not yet implemented"
	return connect.NewResponse(&imagesv1.GenerateOfferPostImageResponse{
		Success: false,
		Error:   &errorMsg,
	}), nil
}
