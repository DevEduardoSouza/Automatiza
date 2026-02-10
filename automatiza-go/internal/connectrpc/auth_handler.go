package connectrpc

import (
	"context"
	"errors"
	"net/http"

	"automatiza-go/internal/auth"
	authv1 "automatiza-go/internal/gen/auth/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthHandler implements the authv1connect.AuthServiceHandler interface.
type AuthHandler struct {
	authService  *auth.AuthService
	cookieConfig *auth.CookieConfig
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authService *auth.AuthService, cookieConfig *auth.CookieConfig) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		cookieConfig: cookieConfig,
	}
}

// Helper functions to convert between internal and proto types

func userToProto(user *auth.User) *authv1.AuthUser {
	if user == nil {
		return nil
	}
	protoUser := &authv1.AuthUser{
		Id:            user.ID,
		Email:         user.Email,
		Role:          user.Role,
		EmailVerified: user.EmailVerified,
		CreatedAt:     timestamppb.New(user.CreatedAt),
	}
	if user.DisplayName != "" {
		protoUser.DisplayName = &user.DisplayName
	}
	return protoUser
}

func sessionToProto(session *auth.Session, isCurrent bool) *authv1.SessionInfo {
	if session == nil {
		return nil
	}
	return &authv1.SessionInfo{
		SessionId:         session.ID,
		ExpiresAt:         timestamppb.New(session.ExpiresAt),
		DeviceFingerprint: session.DeviceFingerprint,
		IpAddress:         session.IPAddress,
		UserAgent:         session.UserAgent,
		CreatedAt:         timestamppb.New(session.CreatedAt),
		LastActivityAt:    timestamppb.New(session.LastActivityAt),
		IsCurrent:         isCurrent,
	}
}

// extractRequestMetadata extracts common request metadata for auth operations.
func extractRequestMetadata(header http.Header) (ipAddress, userAgent string) {
	// Get IP from forwarded headers or direct connection
	ipAddress = header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = header.Get("X-Real-IP")
	}

	userAgent = header.Get("User-Agent")
	return
}

// mapAuthError maps internal auth errors to ConnectRPC errors.
func mapAuthError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, auth.ErrAccountLocked):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, auth.ErrEmailNotVerified):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, auth.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, auth.ErrEmailAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, auth.ErrInvalidToken):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, auth.ErrRateLimited):
		return connect.NewError(connect.CodeResourceExhausted, err)
	case errors.Is(err, auth.ErrUnauthorized):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, auth.ErrSessionNotFound), errors.Is(err, auth.ErrSessionExpired), errors.Is(err, auth.ErrSessionRevoked):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, auth.ErrInvalidRefreshToken):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, auth.ErrPasswordTooShort), errors.Is(err, auth.ErrPasswordTooLong),
		errors.Is(err, auth.ErrPasswordNoLowercase), errors.Is(err, auth.ErrPasswordNoUppercase), errors.Is(err, auth.ErrPasswordNoDigit):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

// getSessionFromCookie extracts the session ID from the request cookie.
func getSessionFromCookie(header http.Header) string {
	cookies := header.Get("Cookie")
	if cookies == "" {
		return ""
	}
	// Parse cookies manually to find session_id
	req := &http.Request{Header: http.Header{"Cookie": []string{cookies}}}
	cookie, err := req.Cookie(auth.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// getRefreshTokenFromCookie extracts the refresh token from the request cookie.
func getRefreshTokenFromCookie(header http.Header) string {
	cookies := header.Get("Cookie")
	if cookies == "" {
		return ""
	}
	req := &http.Request{Header: http.Header{"Cookie": []string{cookies}}}
	cookie, err := req.Cookie(auth.RefreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// Login authenticates user with email and password.
func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	ipAddress, userAgent := extractRequestMetadata(req.Header())
	deviceFingerprint := req.Msg.GetDeviceFingerprint()

	result, err := h.authService.Login(ctx, req.Msg.Email, req.Msg.Password, ipAddress, userAgent, deviceFingerprint)
	if err != nil {
		return nil, mapAuthError(err)
	}

	resp := connect.NewResponse(&authv1.LoginResponse{
		User:      userToProto(result.User),
		Session:   sessionToProto(result.Session, true),
		CsrfToken: result.CSRFToken,
	})

	// Set auth cookies directly on response headers
	// Using http.Cookie.String() to properly format the cookies
	sessionCookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    result.Session.ID,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.SessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	refreshCookie := &http.Cookie{
		Name:     auth.RefreshCookieName,
		Value:    result.RefreshToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	csrfCookie := &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    result.CSRFToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: false, // JavaScript needs access
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}

	// Add cookies to response headers
	resp.Header().Add("Set-Cookie", sessionCookie.String())
	resp.Header().Add("Set-Cookie", refreshCookie.String())
	resp.Header().Add("Set-Cookie", csrfCookie.String())

	return resp, nil
}

// Register creates a new user account.
func (h *AuthHandler) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {
	displayName := req.Msg.GetDisplayName()

	result, err := h.authService.Register(ctx, req.Msg.Email, req.Msg.Password, displayName)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.RegisterResponse{
		User:                  userToProto(result.User),
		EmailVerificationSent: result.EmailVerificationSent,
	}), nil
}

// Logout invalidates the current session.
func (h *AuthHandler) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	sessionsRevoked, err := h.authService.Logout(ctx, sessionID, req.Msg.LogoutAllDevices)
	if err != nil {
		return nil, mapAuthError(err)
	}

	resp := connect.NewResponse(&authv1.LogoutResponse{
		Success:         true,
		SessionsRevoked: int32(sessionsRevoked),
	})

	// Clear auth cookies by setting them with MaxAge=-1
	clearCookies := []*http.Cookie{
		{Name: auth.SessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.cookieConfig.Secure, SameSite: h.cookieConfig.SameSite},
		{Name: auth.RefreshCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.cookieConfig.Secure, SameSite: h.cookieConfig.SameSite},
		{Name: auth.CSRFCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: false, Secure: h.cookieConfig.Secure, SameSite: h.cookieConfig.SameSite},
	}
	for _, cookie := range clearCookies {
		resp.Header().Add("Set-Cookie", cookie.String())
	}

	return resp, nil
}

// RefreshSession refreshes the current session.
func (h *AuthHandler) RefreshSession(ctx context.Context, req *connect.Request[authv1.RefreshSessionRequest]) (*connect.Response[authv1.RefreshSessionResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	refreshToken := getRefreshTokenFromCookie(req.Header())
	if refreshToken == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrInvalidRefreshToken)
	}

	result, err := h.authService.RefreshSession(ctx, sessionID, refreshToken)
	if err != nil {
		return nil, mapAuthError(err)
	}

	resp := connect.NewResponse(&authv1.RefreshSessionResponse{
		Session:   sessionToProto(result.Session, true),
		CsrfToken: result.CSRFToken,
	})

	// Set updated auth cookies directly on response headers
	sessionCookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    result.Session.ID,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.SessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	refreshCookie := &http.Cookie{
		Name:     auth.RefreshCookieName,
		Value:    result.RefreshToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	csrfCookie := &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    result.CSRFToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: false,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}

	resp.Header().Add("Set-Cookie", sessionCookie.String())
	resp.Header().Add("Set-Cookie", refreshCookie.String())
	resp.Header().Add("Set-Cookie", csrfCookie.String())

	return resp, nil
}

// GetCurrentUser returns the currently authenticated user and session info.
func (h *AuthHandler) GetCurrentUser(ctx context.Context, req *connect.Request[authv1.GetCurrentUserRequest]) (*connect.Response[authv1.GetCurrentUserResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	user, session, err := h.authService.GetCurrentUser(ctx, sessionID)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.GetCurrentUserResponse{
		User:    userToProto(user),
		Session: sessionToProto(session, true),
	}), nil
}

// ListSessions returns all active sessions for the current user.
func (h *AuthHandler) ListSessions(ctx context.Context, req *connect.Request[authv1.ListSessionsRequest]) (*connect.Response[authv1.ListSessionsResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	sessions, currentSessionID, err := h.authService.ListSessions(ctx, sessionID)
	if err != nil {
		return nil, mapAuthError(err)
	}

	protoSessions := make([]*authv1.SessionInfo, len(sessions))
	for i, s := range sessions {
		protoSessions[i] = sessionToProto(s, s.ID == currentSessionID)
	}

	return connect.NewResponse(&authv1.ListSessionsResponse{
		Sessions:         protoSessions,
		CurrentSessionId: currentSessionID,
	}), nil
}

// RevokeSession revokes a specific session.
func (h *AuthHandler) RevokeSession(ctx context.Context, req *connect.Request[authv1.RevokeSessionRequest]) (*connect.Response[authv1.RevokeSessionResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	err := h.authService.RevokeSession(ctx, sessionID, req.Msg.SessionId)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.RevokeSessionResponse{
		Success: true,
	}), nil
}

// ForgotPassword initiates password reset flow.
func (h *AuthHandler) ForgotPassword(ctx context.Context, req *connect.Request[authv1.ForgotPasswordRequest]) (*connect.Response[authv1.ForgotPasswordResponse], error) {
	err := h.authService.ForgotPassword(ctx, req.Msg.Email)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.ForgotPasswordResponse{
		Success: true,
	}), nil
}

// ResetPassword completes password reset with token.
func (h *AuthHandler) ResetPassword(ctx context.Context, req *connect.Request[authv1.ResetPasswordRequest]) (*connect.Response[authv1.ResetPasswordResponse], error) {
	err := h.authService.ResetPassword(ctx, req.Msg.Token, req.Msg.NewPassword)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.ResetPasswordResponse{
		Success: true,
	}), nil
}

// ChangePassword changes password for authenticated user.
func (h *AuthHandler) ChangePassword(ctx context.Context, req *connect.Request[authv1.ChangePasswordRequest]) (*connect.Response[authv1.ChangePasswordResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	err := h.authService.ChangePassword(ctx, sessionID, req.Msg.CurrentPassword, req.Msg.NewPassword)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.ChangePasswordResponse{
		Success: true,
	}), nil
}

// VerifyEmail verifies email address with token.
func (h *AuthHandler) VerifyEmail(ctx context.Context, req *connect.Request[authv1.VerifyEmailRequest]) (*connect.Response[authv1.VerifyEmailResponse], error) {
	user, err := h.authService.VerifyEmail(ctx, req.Msg.Token)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.VerifyEmailResponse{
		Success: true,
		User:    userToProto(user),
	}), nil
}

// ResendVerificationEmail resends verification email to current user.
func (h *AuthHandler) ResendVerificationEmail(ctx context.Context, req *connect.Request[authv1.ResendVerificationEmailRequest]) (*connect.Response[authv1.ResendVerificationEmailResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	err := h.authService.ResendVerificationEmail(ctx, sessionID)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.ResendVerificationEmailResponse{
		Success: true,
	}), nil
}

// GetCsrfToken returns a new CSRF token for the current session.
func (h *AuthHandler) GetCsrfToken(ctx context.Context, req *connect.Request[authv1.GetCsrfTokenRequest]) (*connect.Response[authv1.GetCsrfTokenResponse], error) {
	sessionID := getSessionFromCookie(req.Header())
	if sessionID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized)
	}

	csrfToken, err := h.authService.GetCSRFToken(ctx, sessionID)
	if err != nil {
		return nil, mapAuthError(err)
	}

	return connect.NewResponse(&authv1.GetCsrfTokenResponse{
		CsrfToken: csrfToken,
	}), nil
}

// SendMagicLink sends a passwordless magic link login email.
func (h *AuthHandler) SendMagicLink(ctx context.Context, req *connect.Request[authv1.SendMagicLinkRequest]) (*connect.Response[authv1.SendMagicLinkResponse], error) {
	err := h.authService.SendMagicLink(ctx, req.Msg.Email)
	if err != nil {
		// Don't reveal if email exists - return success either way
		// to prevent email enumeration attacks (except for rate limiting)
		if errors.Is(err, auth.ErrRateLimited) {
			return nil, mapAuthError(err)
		}
		// For user not found, return success to prevent enumeration
		// The email simply won't be sent
		return connect.NewResponse(&authv1.SendMagicLinkResponse{
			Success: true,
		}), nil
	}

	return connect.NewResponse(&authv1.SendMagicLinkResponse{
		Success: true,
	}), nil
}

// VerifyMagicLink verifies a magic link token and creates a session.
func (h *AuthHandler) VerifyMagicLink(ctx context.Context, req *connect.Request[authv1.VerifyMagicLinkRequest]) (*connect.Response[authv1.VerifyMagicLinkResponse], error) {
	ipAddress, userAgent := extractRequestMetadata(req.Header())
	deviceFingerprint := req.Msg.GetDeviceFingerprint()

	result, err := h.authService.VerifyMagicLink(ctx, req.Msg.Token, ipAddress, userAgent, deviceFingerprint)
	if err != nil {
		return nil, mapAuthError(err)
	}

	resp := connect.NewResponse(&authv1.VerifyMagicLinkResponse{
		User:      userToProto(result.User),
		Session:   sessionToProto(result.Session, true),
		CsrfToken: result.CSRFToken,
	})

	// Set auth cookies directly on response headers (same as Login)
	sessionCookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    result.Session.ID,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.SessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	refreshCookie := &http.Cookie{
		Name:     auth.RefreshCookieName,
		Value:    result.RefreshToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}
	csrfCookie := &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    result.CSRFToken,
		Path:     "/",
		MaxAge:   int(h.cookieConfig.RefreshMaxAge.Seconds()),
		HttpOnly: false, // JavaScript needs access
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
	}

	// Add cookies to response headers
	resp.Header().Add("Set-Cookie", sessionCookie.String())
	resp.Header().Add("Set-Cookie", refreshCookie.String())
	resp.Header().Add("Set-Cookie", csrfCookie.String())

	return resp, nil
}
