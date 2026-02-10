package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Service defines the load balancer service interface
type Service interface {
	GetServerForUser(ctx context.Context, userID string) (*models.WahaServer, error)
	AssignUserToServer(ctx context.Context, userID string) (*models.WahaServer, error)
	MigrateUser(ctx context.Context, userID, targetServerID string) error
	RemoveUserServer(ctx context.Context, userID string) error
	GetServerStats(ctx context.Context) (*ServerStats, error)
	GetLeastLoadedServer(ctx context.Context) (*models.WahaServer, error)
	HealthCheck(ctx context.Context) (*HealthCheckResult, error)
}

// ServerStats represents statistics for all servers
type ServerStats struct {
	TotalServers int          `json:"totalServers"`
	TotalUsers   int          `json:"totalUsers"`
	Servers      []ServerInfo `json:"servers"`
}

// ServerInfo represents info about a single server
type ServerInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	UserCount   int     `json:"userCount"`
	MaxUsers    int     `json:"maxUsers"`
	Utilization float64 `json:"utilization"`
	IsActive    bool    `json:"isHealthy"`
	LastChecked string  `json:"lastChecked,omitempty"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Healthy   int            `json:"healthy"`
	Unhealthy int            `json:"unhealthy"`
	Details   []ServerHealth `json:"details"`
}

// ServerHealth represents health info for a server
type ServerHealth struct {
	ServerID string `json:"serverId"`
	IsActive bool   `json:"isHealthy"`
	Latency  int64  `json:"latencyMs"`
	Error    string `json:"error,omitempty"`
}

type lbService struct {
	userServerRepo    repository.UserServerRepository
	settingsRepo      repository.SettingsRepository
	cache             *cache.Cache
	logger            *zap.Logger
	maxUsersPerServer int
}

// NewLoadBalancerService creates a new load balancer service
func NewLoadBalancerService(
	userServerRepo repository.UserServerRepository,
	settingsRepo repository.SettingsRepository,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &lbService{
		userServerRepo:    userServerRepo,
		settingsRepo:      settingsRepo,
		cache:             cache,
		logger:            logger,
		maxUsersPerServer: 100, // Default
	}
}

func (s *lbService) GetServerForUser(ctx context.Context, userID string) (*models.WahaServer, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("user_server:%s", userID)
	var cached models.WahaServer
	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	// Get from database
	mapping, err := s.userServerRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user server mapping: %w", err)
	}

	if mapping == nil {
		// User not assigned to any server, assign one
		return s.AssignUserToServer(ctx, userID)
	}

	// Get server details
	server, err := s.getServerByID(ctx, mapping.ServerID)
	if err != nil {
		return nil, err
	}

	// Cache for 5 minutes
	if s.cache != nil && server != nil {
		_ = s.cache.SetJSON(ctx, cacheKey, server, 5*time.Minute)
	}

	return server, nil
}

func (s *lbService) AssignUserToServer(ctx context.Context, userID string) (*models.WahaServer, error) {
	// Get least loaded server
	server, err := s.GetLeastLoadedServer(ctx)
	if err != nil {
		return nil, err
	}

	if server == nil {
		return nil, fmt.Errorf("no available servers")
	}

	// Create mapping
	now := time.Now().UTC().Format(time.RFC3339)
	mapping := &models.UserServerMapping{
		UserID:    userID,
		ServerID:  server.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = s.userServerRepo.Create(ctx, mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create user server mapping: %w", err)
	}

	s.logger.Info("Assigned user to server",
		zap.String("userID", userID),
		zap.String("serverID", server.ID))

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, fmt.Sprintf("user_server:%s", userID))
	}

	return server, nil
}

func (s *lbService) MigrateUser(ctx context.Context, userID, targetServerID string) error {
	// Verify target server exists
	targetServer, err := s.getServerByID(ctx, targetServerID)
	if err != nil || targetServer == nil {
		return fmt.Errorf("target server not found: %s", targetServerID)
	}

	// Check if target server has capacity
	count, err := s.userServerRepo.CountByServer(ctx, targetServerID)
	if err != nil {
		return fmt.Errorf("failed to check server capacity: %w", err)
	}

	if count >= s.maxUsersPerServer {
		return fmt.Errorf("target server is at capacity")
	}

	// Update mapping
	err = s.userServerRepo.Update(ctx, userID, targetServerID)
	if err != nil {
		return fmt.Errorf("failed to update user server mapping: %w", err)
	}

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, fmt.Sprintf("user_server:%s", userID))
	}

	s.logger.Info("Migrated user to new server",
		zap.String("userID", userID),
		zap.String("targetServerID", targetServerID))

	return nil
}

func (s *lbService) RemoveUserServer(ctx context.Context, userID string) error {
	err := s.userServerRepo.Delete(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user server mapping: %w", err)
	}

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, fmt.Sprintf("user_server:%s", userID))
	}

	s.logger.Info("Removed user server mapping", zap.String("userID", userID))

	return nil
}

func (s *lbService) GetServerStats(ctx context.Context) (*ServerStats, error) {
	servers, err := s.loadServers(ctx)
	if err != nil {
		return nil, err
	}

	stats := &ServerStats{
		TotalServers: len(servers),
		Servers:      make([]ServerInfo, 0, len(servers)),
	}

	for _, server := range servers {
		count, _ := s.userServerRepo.CountByServer(ctx, server.ID)

		utilization := 0.0
		if s.maxUsersPerServer > 0 {
			utilization = float64(count) / float64(s.maxUsersPerServer) * 100
		}

		stats.Servers = append(stats.Servers, ServerInfo{
			ID:          server.ID,
			Name:        server.Name,
			URL:         server.URL,
			UserCount:   count,
			MaxUsers:    s.maxUsersPerServer,
			Utilization: utilization,
			IsActive:    server.IsActive,
		})

		stats.TotalUsers += count
	}

	return stats, nil
}

func (s *lbService) GetLeastLoadedServer(ctx context.Context) (*models.WahaServer, error) {
	servers, err := s.loadServers(ctx)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers configured")
	}

	// Get user counts for each server
	type serverLoad struct {
		server *models.WahaServer
		count  int
	}

	loads := make([]serverLoad, 0, len(servers))
	for i := range servers {
		if !servers[i].IsActive {
			continue
		}

		count, _ := s.userServerRepo.CountByServer(ctx, servers[i].ID)
		if count >= s.maxUsersPerServer {
			continue // Skip servers at capacity
		}

		loads = append(loads, serverLoad{
			server: &servers[i],
			count:  count,
		})
	}

	if len(loads) == 0 {
		return nil, fmt.Errorf("no available servers with capacity")
	}

	// Sort by load (ascending)
	sort.Slice(loads, func(i, j int) bool {
		return loads[i].count < loads[j].count
	})

	return loads[0].server, nil
}

func (s *lbService) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	servers, err := s.loadServers(ctx)
	if err != nil {
		return nil, err
	}

	result := &HealthCheckResult{
		Details: make([]ServerHealth, 0, len(servers)),
	}

	for _, server := range servers {
		health := ServerHealth{
			ServerID: server.ID,
			IsActive: server.IsActive,
		}

		// TODO: Actually ping the server
		// For now, just use the stored health status

		if health.IsActive {
			result.Healthy++
		} else {
			result.Unhealthy++
		}

		result.Details = append(result.Details, health)
	}

	return result, nil
}

func (s *lbService) loadServers(ctx context.Context) ([]models.WahaServer, error) {
	// Check cache
	cacheKey := "waha_servers"
	var cached []models.WahaServer
	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	// Load from settings
	settings, err := s.settingsRepo.Get(ctx, "waha_servers", "max_users_per_server")
	if err != nil {
		return nil, fmt.Errorf("failed to load server settings: %w", err)
	}

	// Parse servers from settings
	// Expected format: JSON array of WahaServer objects
	if serversJSON, ok := settings["waha_servers"].(string); ok && serversJSON != "" {
		var configuredServers []models.WahaServer
		if err := json.Unmarshal([]byte(serversJSON), &configuredServers); err == nil && len(configuredServers) > 0 {
			if s.cache != nil {
				_ = s.cache.SetJSON(ctx, cacheKey, configuredServers, time.Minute)
			}
			return configuredServers, nil
		}
		s.logger.Warn("failed to parse waha_servers settings, using default", zap.Error(err))
	}

	if maxUsers, ok := settings["max_users_per_server"].(float64); ok {
		s.maxUsersPerServer = int(maxUsers)
	}

	// Default: return single server from env
	servers := []models.WahaServer{
		{
			ID:       "default",
			Name:     "Default Server",
			IsActive: true,
		},
	}

	// Cache for 1 minute
	if s.cache != nil {
		_ = s.cache.SetJSON(ctx, cacheKey, servers, time.Minute)
	}

	return servers, nil
}

func (s *lbService) getServerByID(ctx context.Context, serverID string) (*models.WahaServer, error) {
	servers, err := s.loadServers(ctx)
	if err != nil {
		return nil, err
	}

	for _, server := range servers {
		if server.ID == serverID {
			return &server, nil
		}
	}

	return nil, fmt.Errorf("server not found: %s", serverID)
}
