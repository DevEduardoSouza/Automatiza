package connectrpc

import (
	"context"
	"fmt"

	instagramv1 "automatiza-go/internal/gen/instagram/v1"
	"automatiza-go/internal/services/instagram"

	"connectrpc.com/connect"
)

// InstagramHandler implements instagramv1connect.InstagramServiceHandler.
type InstagramHandler struct {
	svc *instagram.Service
}

// NewInstagramHandler creates the Instagram ConnectRPC handler.
func NewInstagramHandler(svc *instagram.Service) *InstagramHandler {
	return &InstagramHandler{svc: svc}
}

// GetStatus returns feature availability plus the caller's connection state.
func (h *InstagramHandler) GetStatus(
	ctx context.Context,
	_ *connect.Request[instagramv1.GetStatusRequest],
) (*connect.Response[instagramv1.GetStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	st, err := h.svc.Status(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load status: %w", err))
	}
	recent := make([]*instagramv1.StoryPost, 0, len(st.Recent))
	for i := range st.Recent {
		p := &st.Recent[i]
		recent = append(recent, &instagramv1.StoryPost{
			Id:          p.ID,
			Status:      p.Status,
			MediaSource: p.MediaSource,
			SourceGroup: p.SourceGroup,
			IgMediaId:   p.IGMediaID,
			Error:       p.Error,
			CreatedAt:   p.CreatedAt,
		})
	}
	return connect.NewResponse(&instagramv1.GetStatusResponse{
		FeatureEnabled: st.FeatureEnabled,
		Connected:      st.Connected,
		Username:       st.Username,
		IgUserId:       st.IGUserID,
		TokenExpiresAt: st.TokenExpiresAt,
		RecentStories:  recent,
	}), nil
}

// GetAuthorizeUrl returns the Instagram OAuth URL to start the connect flow.
func (h *InstagramHandler) GetAuthorizeUrl(
	ctx context.Context,
	_ *connect.Request[instagramv1.GetAuthorizeUrlRequest],
) (*connect.Response[instagramv1.GetAuthorizeUrlResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	url, err := h.svc.BuildAuthorizeURL(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("failed to build authorize url: %w", err))
	}
	return connect.NewResponse(&instagramv1.GetAuthorizeUrlResponse{Url: url}), nil
}

// Disconnect removes the caller's stored Instagram credentials.
func (h *InstagramHandler) Disconnect(
	ctx context.Context,
	_ *connect.Request[instagramv1.DisconnectRequest],
) (*connect.Response[instagramv1.DisconnectResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.svc.Disconnect(ctx, user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to disconnect: %w", err))
	}
	return connect.NewResponse(&instagramv1.DisconnectResponse{}), nil
}
