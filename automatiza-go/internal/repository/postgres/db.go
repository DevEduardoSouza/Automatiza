package postgres

import (
	"context"
	"fmt"
	"time"

	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DB represents a PostgreSQL database connection pool
type DB struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// ConnectionString returns the PostgreSQL connection string
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// NewDB creates a new database connection pool
func NewDB(ctx context.Context, cfg *Config, logger *zap.Logger) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = cfg.MaxConns
	if poolConfig.MaxConns == 0 {
		poolConfig.MaxConns = 25
	}
	poolConfig.MinConns = cfg.MinConns
	if poolConfig.MinConns == 0 {
		poolConfig.MinConns = 5
	}
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Set statement_timeout to prevent runaway queries (30s default)
	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	if _, exists := poolConfig.ConnConfig.RuntimeParams["statement_timeout"]; !exists {
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Connected to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
	)

	db := &DB{pool: pool, logger: logger}

	// Start pool stat monitoring goroutine
	observability.SafeGo("db-pool-monitor", func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stat := pool.Stat()
			utilization := float64(stat.AcquiredConns()) / float64(stat.MaxConns())
			if utilization > 0.8 {
				observability.LogAndCapture(logger, sentry.LevelWarning,
					"database pool utilization above 80%", nil,
					map[string]string{
						"module":        "postgres",
						"flow":          "pool-monitor",
						"acquired":      fmt.Sprintf("%d", stat.AcquiredConns()),
						"max":           fmt.Sprintf("%d", stat.MaxConns()),
						"idle":          fmt.Sprintf("%d", stat.IdleConns()),
						"total":         fmt.Sprintf("%d", stat.TotalConns()),
						"utilization_pct": fmt.Sprintf("%.0f", utilization*100),
					})
			}
		}
	})

	return db, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying connection pool
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

// WithTx executes a function within a transaction
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			db.logger.Error("Failed to rollback transaction",
				zap.Error(rbErr),
				zap.Error(err))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Exec executes a query without returning rows
func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

// QueryRow executes a query that returns at most one row
func (db *DB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// Query executes a query that returns rows
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// ErrNoRows is returned when a query returns no rows
var ErrNoRows = pgx.ErrNoRows
