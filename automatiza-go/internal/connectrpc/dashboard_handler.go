package connectrpc

import (
	"context"
	"errors"

	dashboardv1 "automatiza-go/internal/gen/dashboard/v1"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/dashboard"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// DashboardHandler implements the dashboardv1connect.DashboardServiceHandler interface.
type DashboardHandler struct {
	dashboardService dashboard.Service
	logger           *zap.Logger
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(dashboardService dashboard.Service, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
		logger:           logger,
	}
}

// GetDashboardStats returns aggregated statistics for the user's dashboard.
func (h *DashboardHandler) GetDashboardStats(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetDashboardStatsRequest],
) (*connect.Response[dashboardv1.GetDashboardStatsResponse], error) {
	// Get user from auth context
	user := GetUserFromContext(ctx)
	if user == nil {
		h.logger.Warn("Dashboard stats request without authenticated user")
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	h.logger.Info("Fetching dashboard stats",
		zap.String("userID", user.ID),
		zap.String("userEmail", user.Email))

	// Get dashboard stats
	stats, err := h.dashboardService.GetStats(ctx, user.ID)
	if err != nil {
		// Check for context timeout/cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("request timed out"))
		}
		// Return generic error to client (details logged in service layer)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to load dashboard statistics"))
	}

	h.logger.Info("Dashboard stats retrieved",
		zap.String("userID", user.ID),
		zap.Int32("totalGroups", stats.TotalGroups),
		zap.Int32("fullGroups", stats.FullGroups),
		zap.Int32("totalMembers", stats.TotalMembers),
		zap.Int32("activeLinks", stats.ActiveLinks))

	return connect.NewResponse(&dashboardv1.GetDashboardStatsResponse{
		Stats: &dashboardv1.DashboardStats{
			TotalGroups:  stats.TotalGroups,
			FullGroups:   stats.FullGroups,
			TotalMembers: stats.TotalMembers,
			ActiveLinks:  stats.ActiveLinks,
		},
	}), nil
}

// GetDashboardHistory returns historical data for the dashboard chart.
func (h *DashboardHandler) GetDashboardHistory(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetDashboardHistoryRequest],
) (*connect.Response[dashboardv1.GetDashboardHistoryResponse], error) {
	// Get user from auth context
	user := GetUserFromContext(ctx)
	if user == nil {
		h.logger.Warn("Dashboard history request without authenticated user")
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	// Convert proto period to repository period
	var period repository.HistoryPeriod
	switch req.Msg.Period {
	case dashboardv1.Period_PERIOD_WEEKLY:
		period = repository.HistoryPeriodWeekly
	case dashboardv1.Period_PERIOD_MONTHLY:
		period = repository.HistoryPeriodMonthly
	case dashboardv1.Period_PERIOD_YEARLY:
		period = repository.HistoryPeriodYearly
	default:
		period = repository.HistoryPeriodWeekly
	}

	h.logger.Info("Fetching dashboard history",
		zap.String("userID", user.ID),
		zap.String("period", string(period)))

	// Get dashboard history
	dataPoints, err := h.dashboardService.GetHistory(ctx, user.ID, period)
	if err != nil {
		// Check for context timeout/cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("request timed out"))
		}
		// Return generic error to client (details logged in service layer)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to load dashboard history"))
	}

	// Convert to proto response
	protoDataPoints := make([]*dashboardv1.HistoryDataPoint, len(dataPoints))
	for i, dp := range dataPoints {
		protoDataPoints[i] = &dashboardv1.HistoryDataPoint{
			Label:      dp.Label,
			Members:    dp.Members,
			Groups:     dp.Groups,
			NewMembers: dp.NewMembers,
			NewGroups:  dp.NewGroups,
		}
	}

	h.logger.Info("Dashboard history retrieved",
		zap.String("userID", user.ID),
		zap.String("period", string(period)),
		zap.Int("dataPoints", len(protoDataPoints)))

	return connect.NewResponse(&dashboardv1.GetDashboardHistoryResponse{
		DataPoints: protoDataPoints,
	}), nil
}

// GetAnnouncement returns the current dashboard announcement.
func (h *DashboardHandler) GetAnnouncement(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetAnnouncementRequest],
) (*connect.Response[dashboardv1.GetAnnouncementResponse], error) {
	// Get announcement settings (public, no auth required for announcement)
	announcement, err := h.dashboardService.GetAnnouncement(ctx)
	if err != nil {
		// Check for context timeout/cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("request timed out"))
		}
		// Return generic error to client (details logged in service layer)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to load announcement"))
	}

	return connect.NewResponse(&dashboardv1.GetAnnouncementResponse{
		Announcement: &dashboardv1.Announcement{
			Enabled: announcement.Enabled,
			Title:   announcement.Title,
			Body:    announcement.Body,
		},
	}), nil
}
