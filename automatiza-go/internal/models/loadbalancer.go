package models

// Server represents a WAHA server for load balancing
type Server struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	APIKey   string `json:"apiKey"`
	MaxUsers int    `json:"maxUsers"`
	IsActive bool   `json:"isActive"`
	Priority int    `json:"priority"`
}

// UserServer represents a user-server assignment in the database
type UserServer struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	ServerID  string `json:"server_id" db:"server_id"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

// ServerStats represents statistics for a server
type ServerStats struct {
	ServerID    string  `json:"serverId"`
	URL         string  `json:"url"`
	TotalUsers  int     `json:"totalUsers"`
	MaxUsers    int     `json:"maxUsers"`
	Utilization float64 `json:"utilization"`
	IsActive    bool    `json:"isActive"`
}

// LoadBalancerStats represents overall load balancer statistics
type LoadBalancerStats struct {
	Servers            []ServerStats `json:"servers"`
	TotalUsers         int           `json:"totalUsers"`
	TotalCapacity      int           `json:"totalCapacity"`
	OverallUtilization float64       `json:"overallUtilization"`
}

// GetUserServerRequest represents a request to get user's server
type GetUserServerRequest struct {
	UserID string `json:"userId" validate:"required"`
}

// MigrateUserRequest represents a request to migrate a user to a different server
type MigrateUserRequest struct {
	UserID         string `json:"userId" validate:"required"`
	TargetServerID string `json:"targetServerId" validate:"required"`
}

// MigrateUserResponse represents the result of a user migration
type MigrateUserResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	PreviousServer string `json:"previousServer,omitempty"`
	NewServer      string `json:"newServer"`
}
