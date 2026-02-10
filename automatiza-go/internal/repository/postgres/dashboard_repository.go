package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type dashboardRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewDashboardRepository creates a new PostgreSQL dashboard repository
func NewDashboardRepository(db *DB, logger *zap.Logger) repository.DashboardRepository {
	return &dashboardRepository{
		db:     db,
		logger: logger,
	}
}

// GetGroupsStats retrieves aggregated group statistics for a user's dashboard
func (r *dashboardRepository) GetGroupsStats(ctx context.Context, userID string) (*repository.DashboardStats, error) {
	query := `
		SELECT
			COUNT(*)::int as total_groups,
			COUNT(*) FILTER (WHERE status = 'full')::int as full_groups,
			COALESCE(SUM(members_count), 0)::int as total_members
		FROM groups
		WHERE user_id = $1
	`

	r.logger.Info("Executing GetGroupsStats query",
		zap.String("userID", userID))

	var stats repository.DashboardStats
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&stats.TotalGroups,
		&stats.FullGroups,
		&stats.TotalMembers,
	)
	if err != nil {
		r.logger.Error("Failed to get groups stats",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	r.logger.Info("GetGroupsStats result",
		zap.String("userID", userID),
		zap.Int("totalGroups", stats.TotalGroups),
		zap.Int("fullGroups", stats.FullGroups),
		zap.Int("totalMembers", stats.TotalMembers))

	return &stats, nil
}

// GetActiveLinksCount retrieves the count of active redirect links for a user
func (r *dashboardRepository) GetActiveLinksCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)::int
		FROM saved_redirect_links
		WHERE user_id = $1 AND is_active = true
	`

	r.logger.Info("Executing GetActiveLinksCount query",
		zap.String("userID", userID))

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get active links count",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, err
	}

	r.logger.Info("GetActiveLinksCount result",
		zap.String("userID", userID),
		zap.Int("count", count))

	return count, nil
}

// GetAnnouncementSettings retrieves the dashboard announcement settings
func (r *dashboardRepository) GetAnnouncementSettings(ctx context.Context) (*repository.AnnouncementSettings, error) {
	query := `
		SELECT key, value
		FROM settings
		WHERE key IN (
			'dashboard_announcement_enabled',
			'dashboard_announcement_title',
			'dashboard_announcement_body'
		)
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get announcement settings", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	settings := &repository.AnnouncementSettings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		switch key {
		case "dashboard_announcement_enabled":
			settings.Enabled = value == "1" || value == "true"
		case "dashboard_announcement_title":
			settings.Title = value
		case "dashboard_announcement_body":
			settings.Body = value
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return settings, nil
}

// GetHistoryStats retrieves historical stats for the dashboard chart
func (r *dashboardRepository) GetHistoryStats(ctx context.Context, userID string, period repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
	r.logger.Info("Executing GetHistoryStats query",
		zap.String("userID", userID),
		zap.String("period", string(period)))

	switch period {
	case repository.HistoryPeriodWeekly:
		return r.getWeeklyStats(ctx, userID)
	case repository.HistoryPeriodMonthly:
		return r.getMonthlyStats(ctx, userID)
	case repository.HistoryPeriodYearly:
		return r.getYearlyStats(ctx, userID)
	default:
		return r.getWeeklyStats(ctx, userID)
	}
}

// getWeeklyStats retrieves stats for the last 7 days, grouped by day
func (r *dashboardRepository) getWeeklyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	// Portuguese day abbreviations
	dayLabels := []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}

	query := `
		WITH date_series AS (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '6 days',
				CURRENT_DATE,
				INTERVAL '1 day'
			)::date AS day
		),
		daily_new_groups AS (
			SELECT
				DATE(created_at) as day,
				COUNT(*) as new_groups,
				COALESCE(SUM(members_count), 0) as new_members
			FROM groups
			WHERE user_id = $1
				AND created_at >= CURRENT_DATE - INTERVAL '6 days'
			GROUP BY DATE(created_at)
		),
		cumulative_totals AS (
			SELECT
				ds.day,
				(SELECT COUNT(*) FROM groups WHERE user_id = $1 AND DATE(created_at) <= ds.day) as total_groups,
				(SELECT COALESCE(SUM(members_count), 0) FROM groups WHERE user_id = $1 AND DATE(created_at) <= ds.day) as total_members
			FROM date_series ds
		)
		SELECT
			ct.day,
			EXTRACT(DOW FROM ct.day)::int as dow,
			ct.total_groups::int,
			ct.total_members::int,
			COALESCE(dng.new_groups, 0)::int as new_groups,
			COALESCE(dng.new_members, 0)::int as new_members
		FROM cumulative_totals ct
		LEFT JOIN daily_new_groups dng ON ct.day = dng.day
		ORDER BY ct.day
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get weekly stats",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var result []repository.HistoryDataPoint
	for rows.Next() {
		var day time.Time
		var dow int
		var dp repository.HistoryDataPoint
		if err := rows.Scan(&day, &dow, &dp.Groups, &dp.Members, &dp.NewGroups, &dp.NewMembers); err != nil {
			return nil, err
		}
		dp.Label = dayLabels[dow]
		result = append(result, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.logger.Info("GetHistoryStats weekly result",
		zap.String("userID", userID),
		zap.Int("dataPoints", len(result)))

	return result, nil
}

// getMonthlyStats retrieves stats for the last 4 weeks, grouped by week
func (r *dashboardRepository) getMonthlyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	query := `
		WITH week_series AS (
			SELECT generate_series(0, 3) AS week_offset
		),
		week_ranges AS (
			SELECT
				week_offset,
				DATE_TRUNC('week', CURRENT_DATE) - (week_offset * INTERVAL '1 week') AS week_start,
				DATE_TRUNC('week', CURRENT_DATE) - (week_offset * INTERVAL '1 week') + INTERVAL '6 days' AS week_end
			FROM week_series
		),
		weekly_new_groups AS (
			SELECT
				DATE_TRUNC('week', created_at)::date as week_start,
				COUNT(*) as new_groups,
				COALESCE(SUM(members_count), 0) as new_members
			FROM groups
			WHERE user_id = $1
				AND created_at >= DATE_TRUNC('week', CURRENT_DATE) - INTERVAL '3 weeks'
			GROUP BY DATE_TRUNC('week', created_at)
		),
		cumulative_totals AS (
			SELECT
				wr.week_offset,
				wr.week_start,
				(SELECT COUNT(*) FROM groups WHERE user_id = $1 AND DATE(created_at) <= wr.week_end) as total_groups,
				(SELECT COALESCE(SUM(members_count), 0) FROM groups WHERE user_id = $1 AND DATE(created_at) <= wr.week_end) as total_members
			FROM week_ranges wr
		)
		SELECT
			ct.week_offset,
			ct.week_start,
			ct.total_groups::int,
			ct.total_members::int,
			COALESCE(wng.new_groups, 0)::int as new_groups,
			COALESCE(wng.new_members, 0)::int as new_members
		FROM cumulative_totals ct
		LEFT JOIN weekly_new_groups wng ON ct.week_start::date = wng.week_start
		ORDER BY ct.week_offset DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get monthly stats",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var result []repository.HistoryDataPoint
	for rows.Next() {
		var weekOffset int
		var weekStart time.Time
		var dp repository.HistoryDataPoint
		if err := rows.Scan(&weekOffset, &weekStart, &dp.Groups, &dp.Members, &dp.NewGroups, &dp.NewMembers); err != nil {
			return nil, err
		}
		// Format: "Sem 1", "Sem 2", etc.
		dp.Label = fmt.Sprintf("Sem %d", 4-weekOffset)
		result = append(result, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.logger.Info("GetHistoryStats monthly result",
		zap.String("userID", userID),
		zap.Int("dataPoints", len(result)))

	return result, nil
}

// getYearlyStats retrieves stats for the last 12 months, grouped by month
func (r *dashboardRepository) getYearlyStats(ctx context.Context, userID string) ([]repository.HistoryDataPoint, error) {
	monthLabels := []string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}

	query := `
		WITH month_series AS (
			SELECT generate_series(0, 11) AS month_offset
		),
		month_ranges AS (
			SELECT
				month_offset,
				DATE_TRUNC('month', CURRENT_DATE) - (month_offset * INTERVAL '1 month') AS month_start,
				(DATE_TRUNC('month', CURRENT_DATE) - (month_offset * INTERVAL '1 month') + INTERVAL '1 month' - INTERVAL '1 day')::date AS month_end
			FROM month_series
		),
		monthly_new_groups AS (
			SELECT
				DATE_TRUNC('month', created_at)::date as month_start,
				COUNT(*) as new_groups,
				COALESCE(SUM(members_count), 0) as new_members
			FROM groups
			WHERE user_id = $1
				AND created_at >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '11 months'
			GROUP BY DATE_TRUNC('month', created_at)
		),
		cumulative_totals AS (
			SELECT
				mr.month_offset,
				mr.month_start,
				EXTRACT(MONTH FROM mr.month_start)::int as month_num,
				(SELECT COUNT(*) FROM groups WHERE user_id = $1 AND DATE(created_at) <= mr.month_end) as total_groups,
				(SELECT COALESCE(SUM(members_count), 0) FROM groups WHERE user_id = $1 AND DATE(created_at) <= mr.month_end) as total_members
			FROM month_ranges mr
		)
		SELECT
			ct.month_num,
			ct.total_groups::int,
			ct.total_members::int,
			COALESCE(mng.new_groups, 0)::int as new_groups,
			COALESCE(mng.new_members, 0)::int as new_members
		FROM cumulative_totals ct
		LEFT JOIN monthly_new_groups mng ON ct.month_start::date = mng.month_start
		ORDER BY ct.month_offset DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get yearly stats",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var result []repository.HistoryDataPoint
	for rows.Next() {
		var monthNum int
		var dp repository.HistoryDataPoint
		if err := rows.Scan(&monthNum, &dp.Groups, &dp.Members, &dp.NewGroups, &dp.NewMembers); err != nil {
			return nil, err
		}
		dp.Label = monthLabels[monthNum-1]
		result = append(result, dp)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.logger.Info("GetHistoryStats yearly result",
		zap.String("userID", userID),
		zap.Int("dataPoints", len(result)))

	return result, nil
}
