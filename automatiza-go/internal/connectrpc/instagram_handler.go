package connectrpc

import (
	"context"
	"errors"
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
		HasMetaApp:     st.HasMetaApp,
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

// GetMetaApp returns the caller's registered Meta app. The app secret is never
// included — the UI only learns that one exists.
func (h *InstagramHandler) GetMetaApp(
	ctx context.Context,
	_ *connect.Request[instagramv1.GetMetaAppRequest],
) (*connect.Response[instagramv1.GetMetaAppResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	view, err := h.svc.GetMetaApp(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load meta app: %w", err))
	}
	return connect.NewResponse(&instagramv1.GetMetaAppResponse{App: metaAppToProto(view)}), nil
}

// SaveMetaApp stores the client's own Meta app credentials.
func (h *InstagramHandler) SaveMetaApp(
	ctx context.Context,
	req *connect.Request[instagramv1.SaveMetaAppRequest],
) (*connect.Response[instagramv1.SaveMetaAppResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	view, err := h.svc.SaveMetaApp(ctx, user.ID, req.Msg.GetAppId(), req.Msg.GetAppSecret())
	if err != nil {
		if errors.Is(err, instagram.ErrInstagramInvalidAppInput) {
			// InvalidArgument (não Internal): é erro do preenchimento, e a tela
			// precisa distinguir para mostrar a mensagem certa no campo.
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("dados do aplicativo inválidos"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save meta app: %w", err))
	}
	return connect.NewResponse(&instagramv1.SaveMetaAppResponse{App: metaAppToProto(view)}), nil
}

// DeleteMetaApp removes the client's Meta app registration.
func (h *InstagramHandler) DeleteMetaApp(
	ctx context.Context,
	_ *connect.Request[instagramv1.DeleteMetaAppRequest],
) (*connect.Response[instagramv1.DeleteMetaAppResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.svc.DeleteMetaApp(ctx, user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete meta app: %w", err))
	}
	return connect.NewResponse(&instagramv1.DeleteMetaAppResponse{}), nil
}

func metaAppToProto(v *instagram.MetaAppView) *instagramv1.MetaApp {
	if v == nil {
		return nil
	}
	return &instagramv1.MetaApp{
		AppId:       v.AppID,
		HasSecret:   v.HasSecret,
		WebhookUrl:  v.WebhookURL,
		VerifyToken: v.VerifyToken,
		RedirectUri: v.RedirectURI,
		IsVerified:  v.IsVerified,
		UpdatedAt:   v.UpdatedAt,
	}
}
