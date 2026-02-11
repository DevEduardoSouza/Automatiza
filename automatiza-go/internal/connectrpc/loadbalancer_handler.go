package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	adminv1 "automatiza-go/internal/gen/admin/v1"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/loadbalancer"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// LoadBalancerHandler implements the adminv1connect.LoadBalancerServiceHandler interface.
// This handler provides operations for managing WhatsApp API server load balancing.
type LoadBalancerHandler struct {
	logger       *zap.Logger
	userRoleRepo repository.UserRoleRepository
	lbService    loadbalancer.Service
}

// NewLoadBalancerHandler creates a new load balancer handler.
func NewLoadBalancerHandler(
	logger *zap.Logger,
	userRoleRepo repository.UserRoleRepository,
	lbService loadbalancer.Service,
) *LoadBalancerHandler {
	return &LoadBalancerHandler{
		logger:       logger,
		userRoleRepo: userRoleRepo,
		lbService:    lbService,
	}
}

// requireAdmin checks if the user is an admin and returns an error if not.
func (h *LoadBalancerHandler) requireAdmin(ctx context.Context) error {
	user := GetUserFromContext(ctx)
	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, user.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}

	if !isAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("admin access required"))
	}

	return nil
}

// GetStats returns statistics for all load balancer servers.
// This is an admin-only endpoint.
func (h *LoadBalancerHandler) GetStats(
	ctx context.Context,
	req *connect.Request[adminv1.GetStatsRequest],
) (*connect.Response[adminv1.GetStatsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	h.logger.Debug("LoadBalancer.GetStats called",
		zap.String("userID", GetUserFromContext(ctx).ID),
	)

	stats, err := h.lbService.GetServerStats(ctx)
	if err != nil {
		h.logger.Error("failed to get server stats", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get server stats: %w", err))
	}

	servers := make([]*adminv1.Server, 0, len(stats.Servers))
	var totalCapacity, totalUsed int32

	for i, srv := range stats.Servers {
		serverID, parseErr := strconv.ParseInt(srv.ID, 10, 32)
		if parseErr != nil {
			observability.LogAndCapture(h.logger, sentry.LevelWarning, "failed to parse server ID", parseErr, map[string]string{
				"module": "loadbalancer", "operation": "parse_server_id", "raw_id": srv.ID,
			})
			serverID = int64(i + 1)
		}
		if serverID == 0 {
			serverID = int64(i + 1)
		}

		available := int32(srv.MaxUsers - srv.UserCount)
		if available < 0 {
			available = 0
		}

		status := "offline"
		if srv.IsActive {
			status = "online"
		}

		servers = append(servers, &adminv1.Server{
			ServerId:       int32(serverID),
			ServerName:     srv.Name,
			Url:            srv.URL,
			CurrentUsers:   int32(srv.UserCount),
			MaxUsers:       int32(srv.MaxUsers),
			AvailableSlots: available,
			UsagePercent:   float64(srv.Utilization),
			Status:         status,
		})

		totalCapacity += int32(srv.MaxUsers)
		totalUsed += int32(srv.UserCount)
	}

	totalAvailable := totalCapacity - totalUsed
	if totalAvailable < 0 {
		totalAvailable = 0
	}

	var usagePercent float64
	if totalCapacity > 0 {
		usagePercent = float64(totalUsed) / float64(totalCapacity) * 100
	}

	summary := &adminv1.LoadBalancerSummary{
		TotalServers:   int32(stats.TotalServers),
		TotalCapacity:  totalCapacity,
		TotalUsed:      totalUsed,
		TotalAvailable: totalAvailable,
		UsagePercent:   usagePercent,
	}

	return connect.NewResponse(&adminv1.GetStatsResponse{
		Servers: servers,
		Summary: summary,
	}), nil
}

// GetUserServer returns the server assigned to a specific user.
// This is an admin-only endpoint.
func (h *LoadBalancerHandler) GetUserServer(
	ctx context.Context,
	req *connect.Request[adminv1.GetUserServerRequest],
) (*connect.Response[adminv1.GetUserServerResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	h.logger.Debug("LoadBalancer.GetUserServer called",
		zap.String("adminID", GetUserFromContext(ctx).ID),
		zap.String("targetUserID", userID),
	)

	server, err := h.lbService.GetServerForUser(ctx, userID)
	if err != nil {
		h.logger.Warn("failed to get server for user",
			zap.String("userID", userID),
			zap.Error(err))
		return connect.NewResponse(&adminv1.GetUserServerResponse{
			Server: nil,
		}), nil
	}

	if server == nil {
		return connect.NewResponse(&adminv1.GetUserServerResponse{
			Server: nil,
		}), nil
	}

	serverID, parseErr := strconv.ParseInt(server.ID, 10, 32)
	if parseErr != nil {
		observability.LogAndCapture(h.logger, sentry.LevelWarning, "failed to parse server ID for user server", parseErr, map[string]string{
			"module": "loadbalancer", "operation": "parse_server_id", "raw_id": server.ID,
		})
	}
	status := "offline"
	if server.IsActive {
		status = "online"
	}

	return connect.NewResponse(&adminv1.GetUserServerResponse{
		Server: &adminv1.Server{
			ServerId:   int32(serverID),
			ServerName: server.Name,
			Url:        server.URL,
			Status:     status,
		},
	}), nil
}

// MigrateUser migrates a user to a different server.
// This is an admin-only endpoint.
func (h *LoadBalancerHandler) MigrateUser(
	ctx context.Context,
	req *connect.Request[adminv1.MigrateUserRequest],
) (*connect.Response[adminv1.MigrateUserResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	userID := req.Msg.GetUserId()
	targetServerID := req.Msg.GetTargetServerId()

	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if targetServerID <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_server_id must be positive"))
	}

	adminUser := GetUserFromContext(ctx)
	h.logger.Info("LoadBalancer.MigrateUser called",
		zap.String("adminID", adminUser.ID),
		zap.String("targetUserID", userID),
		zap.Int32("targetServerID", targetServerID),
	)

	// Get current server assignment before migration
	var previousServerID int32
	currentServer, err := h.lbService.GetServerForUser(ctx, userID)
	if err == nil && currentServer != nil {
		prevID, parseErr := strconv.ParseInt(currentServer.ID, 10, 32)
		if parseErr != nil {
			observability.LogAndCapture(h.logger, sentry.LevelWarning, "failed to parse previous server ID during migration", parseErr, map[string]string{
				"module": "loadbalancer", "operation": "parse_server_id", "raw_id": currentServer.ID,
			})
		}
		previousServerID = int32(prevID)
	}

	// Perform migration
	targetServerIDStr := strconv.FormatInt(int64(targetServerID), 10)
	if err := h.lbService.MigrateUser(ctx, userID, targetServerIDStr); err != nil {
		h.logger.Error("failed to migrate user",
			zap.String("userID", userID),
			zap.String("targetServerID", targetServerIDStr),
			zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to migrate user: %w", err))
	}

	h.logger.Info("User migrated successfully",
		zap.String("userID", userID),
		zap.Int32("previousServerID", previousServerID),
		zap.Int32("newServerID", targetServerID))

	return connect.NewResponse(&adminv1.MigrateUserResponse{
		PreviousServerId: previousServerID,
		NewServerId:      targetServerID,
	}), nil
}
