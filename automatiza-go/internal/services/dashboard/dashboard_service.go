package dashboard

import (
	"automatiza-go/internal/repository"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Stats represents the dashboard statistics returned to clients
type Stats struct {
	TotalGroups  int32
	FullGroups   int32
	TotalMembers int32
	ActiveLinks  int32
}

// Announcement represents the dashboard announcement returned to clients
type Announcement struct {
	Enabled bool
	Title   string
	Body    string
}

// HistoryDataPoint represents a single data point in the dashboard history
type HistoryDataPoint struct {
	Label      string
	Members    int32
	Groups     int32
	NewMembers int32
	NewGroups  int32
	Joins      int32
	Leaves     int32
}

// Service defines the dashboard service interface
type Service interface {
	GetStats(ctx context.Context, userID string) (*Stats, error)
	GetAnnouncement(ctx context.Context) (*Announcement, error)
	GetHistory(ctx context.Context, userID string, period repository.HistoryPeriod) ([]HistoryDataPoint, error)
}

// announcementCache holds cached announcement data
type announcementCache struct {
	mu          sync.RWMutex
	data        *Announcement
	lastUpdated time.Time
	ttl         time.Duration
}

type dashboardService struct {
	dashboardRepo     repository.DashboardRepository
	logger            *zap.Logger
	announcementCache *announcementCache
}

// NewDashboardService creates a new dashboard service
func NewDashboardService(
	dashboardRepo repository.DashboardRepository,
	logger *zap.Logger,
) Service {
	return &dashboardService{
		dashboardRepo: dashboardRepo,
		logger:        logger,
		announcementCache: &announcementCache{
			ttl: 5 * time.Minute, // Cache announcement for 5 minutes
		},
	}
}

// GetStats retrieves aggregated dashboard statistics for a user
// Queries are executed in parallel to minimize latency
func (s *dashboardService) GetStats(ctx context.Context, userID string) (*Stats, error) {
	// Add timeout to prevent long-running queries
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		groupStats  *repository.DashboardStats
		activeLinks int
		groupErr    error
		linksErr    error
		wg          sync.WaitGroup
	)

	// Execute queries in parallel
	wg.Add(2)

	go func() {
		defer wg.Done()
		groupStats, groupErr = s.dashboardRepo.GetGroupsStats(ctx, userID)
	}()

	go func() {
		defer wg.Done()
		activeLinks, linksErr = s.dashboardRepo.GetActiveLinksCount(ctx, userID)
	}()

	wg.Wait()

	// Check for context cancellation first
	if ctx.Err() != nil {
		s.logger.Error("Dashboard stats request timed out or canceled",
			zap.String("userID", userID),
			zap.Error(ctx.Err()))
		return nil, ctx.Err()
	}

	// Handle errors from parallel queries
	if groupErr != nil {
		s.logger.Error("Failed to get groups stats",
			zap.String("userID", userID),
			zap.Error(groupErr))
		return nil, groupErr
	}

	if linksErr != nil {
		s.logger.Error("Failed to get active links count",
			zap.String("userID", userID),
			zap.Error(linksErr))
		return nil, linksErr
	}

	return &Stats{
		TotalGroups:  int32(groupStats.TotalGroups),
		FullGroups:   int32(groupStats.FullGroups),
		TotalMembers: int32(groupStats.TotalMembers),
		ActiveLinks:  int32(activeLinks),
	}, nil
}

// GetAnnouncement retrieves the current dashboard announcement
// Uses in-memory cache to reduce database load (announcement rarely changes)
func (s *dashboardService) GetAnnouncement(ctx context.Context) (*Announcement, error) {
	// Check cache first
	s.announcementCache.mu.RLock()
	if s.announcementCache.data != nil && time.Since(s.announcementCache.lastUpdated) < s.announcementCache.ttl {
		cached := s.announcementCache.data
		s.announcementCache.mu.RUnlock()
		return cached, nil
	}
	s.announcementCache.mu.RUnlock()

	// Add timeout to prevent long-running queries
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	settings, err := s.dashboardRepo.GetAnnouncementSettings(ctx)
	if err != nil {
		s.logger.Error("Failed to get announcement settings", zap.Error(err))
		return nil, err
	}

	announcement := &Announcement{
		Enabled: settings.Enabled,
		Title:   settings.Title,
		Body:    settings.Body,
	}

	// Update cache
	s.announcementCache.mu.Lock()
	s.announcementCache.data = announcement
	s.announcementCache.lastUpdated = time.Now()
	s.announcementCache.mu.Unlock()

	return announcement, nil
}

// GetHistory retrieves historical data for the dashboard chart
func (s *dashboardService) GetHistory(ctx context.Context, userID string, period repository.HistoryPeriod) ([]HistoryDataPoint, error) {
	// Add timeout to prevent long-running queries
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dataPoints, err := s.dashboardRepo.GetHistoryStats(ctx, userID, period)
	if err != nil {
		s.logger.Error("Failed to get history stats",
			zap.String("userID", userID),
			zap.String("period", string(period)),
			zap.Error(err))
		return nil, err
	}

	// Convert repository data points to service data points
	result := make([]HistoryDataPoint, len(dataPoints))
	for i, dp := range dataPoints {
		result[i] = HistoryDataPoint{
			Label:      dp.Label,
			Members:    int32(dp.Members),
			Groups:     int32(dp.Groups),
			NewMembers: int32(dp.NewMembers),
			NewGroups:  int32(dp.NewGroups),
			Joins:      int32(dp.Joins),
			Leaves:     int32(dp.Leaves),
		}
	}

	return result, nil
}
