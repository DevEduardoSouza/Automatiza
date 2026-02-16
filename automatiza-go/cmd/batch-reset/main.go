// Package main provides a CLI tool to send password reset emails in bulk
// to users with placeholder password hashes from the legacy Supabase migration.
//
// Usage:
//
//	go run ./cmd/batch-reset/main.go --dry-run          # preview without sending
//	go run ./cmd/batch-reset/main.go                    # send emails
//	go run ./cmd/batch-reset/main.go --batch-size=20    # custom batch size
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"automatiza-go/internal/auth"
	"automatiza-go/internal/config"
	emailpkg "automatiza-go/internal/services/email"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const placeholderHash = "$2a$10$MIGRATED.FROM.LEGACY.SYSTEM.REQUIRES.PASSWORD.RESET.000000"

type affectedUser struct {
	UserID      string
	Email       string
	DisplayName string
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Simulate without sending emails or writing to DB")
	batchSize := flag.Int("batch-size", 10, "Number of emails per batch")
	delay := flag.Duration("delay", 2*time.Second, "Delay between batches")
	flag.Parse()

	// --- Logger ---
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	// --- Config ---
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// --- HMAC init ---
	hmacKey := cfg.GetTokenHMACKey()
	if len(hmacKey) == 0 {
		logger.Fatal("TOKEN_HMAC_KEY or JWT_SECRET must be set for token hashing")
	}
	if err := auth.InitTokenHMAC(hmacKey); err != nil {
		logger.Fatal("failed to init HMAC", zap.Error(err))
	}

	// --- Database ---
	pgCfg := cfg.GetPostgresConfig()
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logger.Fatal("failed to open database", zap.Error(err))
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}

	// --- Email service ---
	var emailSvc emailpkg.Service
	if !*dryRun {
		if !cfg.HasSMTP() {
			logger.Fatal("SMTP not configured — set SMTP_HOST, SMTP_USER, SMTP_PASSWORD in .env (or use --dry-run)")
		}
		emailSvc = emailpkg.NewEmailService(&emailpkg.Config{
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			User:      cfg.SMTPUser,
			Password:  cfg.SMTPPassword,
			FromEmail: cfg.SMTPFromEmail,
			FromName:  cfg.SMTPFromName,
		}, logger)
	}

	// --- Frontend base URL ---
	baseURL := cfg.PublicBaseURL
	if baseURL == "" {
		baseURL = "https://astecwms.com.br"
	}

	// --- Query affected users ---
	queryCtx := context.Background()
	rows, err := db.QueryContext(queryCtx, `
		SELECT uc.user_id, p.email, COALESCE(p.display_name, '')
		FROM user_credentials uc
		JOIN profiles p ON p.id = uc.user_id
		WHERE uc.password_hash = $1
		ORDER BY p.email
	`, placeholderHash)
	if err != nil {
		logger.Fatal("failed to query affected users", zap.Error(err))
	}

	var users []affectedUser
	for rows.Next() {
		var u affectedUser
		if err := rows.Scan(&u.UserID, &u.Email, &u.DisplayName); err != nil {
			logger.Fatal("failed to scan row", zap.Error(err))
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		logger.Fatal("rows iteration error", zap.Error(err))
	}
	rows.Close()

	total := len(users)
	logger.Info("found users with placeholder hash",
		zap.Int("total", total),
		zap.Bool("dry_run", *dryRun))

	if total == 0 {
		logger.Info("no users to process — exiting")
		return
	}

	// --- Process in batches ---
	var sent, failed int
	for i := 0; i < total; i += *batchSize {
		end := i + *batchSize
		if end > total {
			end = total
		}
		batch := users[i:end]

		for _, u := range batch {
			if *dryRun {
				logger.Info("DRY RUN: would send reset email",
					zap.String("user_id", u.UserID),
					zap.String("email", u.Email),
					zap.String("display_name", u.DisplayName))
				sent++
				continue
			}

			if err := processUser(queryCtx, db, emailSvc, logger, baseURL, u); err != nil {
				logger.Error("failed to process user",
					zap.String("user_id", u.UserID),
					zap.String("email", u.Email),
					zap.Error(err))
				failed++
				continue
			}
			sent++
		}

		batchNum := (i / *batchSize) + 1
		totalBatches := (total + *batchSize - 1) / *batchSize
		logger.Info("batch complete",
			zap.Int("batch", batchNum),
			zap.Int("total_batches", totalBatches),
			zap.Int("sent", sent),
			zap.Int("failed", failed))

		// Delay between batches (skip after last batch)
		if end < total {
			time.Sleep(*delay)
		}
	}

	// --- Reset failed_attempts for all affected users ---
	if !*dryRun {
		result, err := db.ExecContext(queryCtx, `
			UPDATE user_credentials
			SET failed_attempts = 0, locked_until = NULL
			WHERE password_hash = $1
			  AND (failed_attempts > 0 OR locked_until IS NOT NULL)
		`, placeholderHash)
		if err != nil {
			logger.Error("failed to reset failed_attempts", zap.Error(err))
		} else {
			rowsAffected, _ := result.RowsAffected()
			logger.Info("reset failed_attempts", zap.Int64("rows_affected", rowsAffected))
		}
	}

	logger.Info("batch reset complete",
		zap.Int("total", total),
		zap.Int("sent", sent),
		zap.Int("failed", failed),
		zap.Bool("dry_run", *dryRun))
}

func processUser(ctx context.Context, db *sql.DB, emailSvc emailpkg.Service, logger *zap.Logger, baseURL string, u affectedUser) error {
	// Generate reset token
	token, err := auth.GenerateResetToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	// Hash token for storage
	tokenHash := auth.HashToken(token)

	// Insert into password_reset_tokens with 24h expiry (extended for batch migration)
	_, err = db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '24 hours')
	`, u.UserID, tokenHash)
	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}

	// Build reset link
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)

	// Send email
	name := u.DisplayName
	if name == "" {
		name = u.Email
	}

	if err := emailSvc.SendPasswordResetEmail(ctx, u.Email, name, resetLink); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	logger.Info("reset email sent",
		zap.String("user_id", u.UserID),
		zap.String("email", u.Email))

	return nil
}
