//go:build ignore

// Package main provides a migration script for transferring affiliate_credentials data
// from Supabase (PostgreSQL) to local PostgreSQL.
//
// Usage:
//
//	go run scripts/migrate_affiliate_credentials.go [flags]
//
// Flags:
//
//	--dry-run        Simulate migration without inserting data
//	--batch-size     Number of records to process per batch (default: 1000)
//	--source-url     Supabase connection URL (or use SUPABASE_DB_URL env)
//	--target-url     Local PostgreSQL URL (or use DATABASE_URL env)
//
// Environment Variables:
//
//	SUPABASE_DB_URL  - Supabase PostgreSQL connection string
//	DATABASE_URL     - Local PostgreSQL connection string
//
// The script is idempotent and can be run multiple times safely.
// It uses INSERT ... ON CONFLICT DO UPDATE to handle existing records.
//
// IMPORTANT: ML credentials are stored encrypted. The encryption key must be
// the same between Supabase and local PostgreSQL for credentials to work.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AffiliateCredentialRow represents a row from the Supabase affiliate_credentials table.
type AffiliateCredentialRow struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	Store        string          `json:"store"`
	Credentials  json.RawMessage `json:"credentials"`
	AutoGenerate bool            `json:"auto_generate"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// MigrationStats tracks migration progress
type MigrationStats struct {
	TotalRead    int
	TotalSkipped int
	TotalMigrated int
	TotalFailed  int
	Errors       []string
}

func main() {
	// Parse flags
	dryRun := flag.Bool("dry-run", false, "Simulate migration without inserting data")
	batchSize := flag.Int("batch-size", 1000, "Number of records to process per batch")
	sourceURL := flag.String("source-url", "", "Supabase connection URL")
	targetURL := flag.String("target-url", "", "Local PostgreSQL URL")
	flag.Parse()

	// Get connection URLs from flags or environment
	supabaseURL := *sourceURL
	if supabaseURL == "" {
		supabaseURL = os.Getenv("SUPABASE_DB_URL")
	}
	if supabaseURL == "" {
		log.Fatal("SUPABASE_DB_URL environment variable or --source-url flag is required")
	}

	localURL := *targetURL
	if localURL == "" {
		localURL = os.Getenv("DATABASE_URL")
	}
	if localURL == "" {
		log.Fatal("DATABASE_URL environment variable or --target-url flag is required")
	}

	ctx := context.Background()

	// Connect to Supabase
	log.Println("Connecting to Supabase...")
	sourcePool, err := pgxpool.New(ctx, supabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to Supabase: %v", err)
	}
	defer sourcePool.Close()

	// Test Supabase connection
	if err := sourcePool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping Supabase: %v", err)
	}
	log.Println("Connected to Supabase successfully")

	// Connect to local PostgreSQL
	log.Println("Connecting to local PostgreSQL...")
	targetPool, err := pgxpool.New(ctx, localURL)
	if err != nil {
		log.Fatalf("Failed to connect to local PostgreSQL: %v", err)
	}
	defer targetPool.Close()

	// Test local connection
	if err := targetPool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping local PostgreSQL: %v", err)
	}
	log.Println("Connected to local PostgreSQL successfully")

	// Run migration
	stats := &MigrationStats{}
	if err := migrateAffiliateCredentials(ctx, sourcePool, targetPool, *batchSize, *dryRun, stats); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Print summary
	printSummary(stats, *dryRun)
}

func migrateAffiliateCredentials(
	ctx context.Context,
	source *pgxpool.Pool,
	target *pgxpool.Pool,
	batchSize int,
	dryRun bool,
	stats *MigrationStats,
) error {
	// Count total records in source
	var totalCount int
	err := source.QueryRow(ctx, "SELECT COUNT(*) FROM affiliate_credentials").Scan(&totalCount)
	if err != nil {
		return fmt.Errorf("failed to count source records: %w", err)
	}
	log.Printf("Found %d affiliate_credentials records to migrate\n", totalCount)

	if totalCount == 0 {
		log.Println("No records to migrate")
		return nil
	}

	// Get list of existing user IDs in target (to skip orphaned credentials)
	existingUsers := make(map[string]bool)
	rows, err := target.Query(ctx, "SELECT id FROM profiles")
	if err != nil {
		log.Printf("Warning: Could not fetch existing users: %v\n", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var userID string
			if err := rows.Scan(&userID); err == nil {
				existingUsers[userID] = true
			}
		}
	}
	log.Printf("Found %d existing users in target database\n", len(existingUsers))

	// Process in batches
	offset := 0
	for offset < totalCount {
		log.Printf("Processing batch: offset=%d, batchSize=%d\n", offset, batchSize)

		query := `
			SELECT id, user_id, store, credentials, auto_generate, is_active, created_at, updated_at
			FROM affiliate_credentials
			ORDER BY created_at ASC
			LIMIT $1 OFFSET $2
		`

		rows, err := source.Query(ctx, query, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to query source: %w", err)
		}

		batch := make([]AffiliateCredentialRow, 0, batchSize)
		for rows.Next() {
			var row AffiliateCredentialRow
			err := rows.Scan(
				&row.ID,
				&row.UserID,
				&row.Store,
				&row.Credentials,
				&row.AutoGenerate,
				&row.IsActive,
				&row.CreatedAt,
				&row.UpdatedAt,
			)
			if err != nil {
				stats.TotalFailed++
				stats.Errors = append(stats.Errors, fmt.Sprintf("scan error: %v", err))
				continue
			}
			batch = append(batch, row)
			stats.TotalRead++
		}
		rows.Close()

		if len(batch) == 0 {
			break
		}

		// Insert batch into target
		for _, row := range batch {
			// Skip if user doesn't exist in target (orphaned credential)
			if len(existingUsers) > 0 && !existingUsers[row.UserID] {
				log.Printf("Skipping credential for non-existent user: %s (store: %s)\n", row.UserID, row.Store)
				stats.TotalSkipped++
				continue
			}

			if dryRun {
				log.Printf("[DRY RUN] Would migrate: user_id=%s, store=%s, is_active=%v\n",
					row.UserID, row.Store, row.IsActive)
				stats.TotalMigrated++
				continue
			}

			// Insert or update
			insertQuery := `
				INSERT INTO affiliate_credentials (
					id, user_id, store, credentials, auto_generate, is_active, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (user_id, store) DO UPDATE SET
					credentials = EXCLUDED.credentials,
					auto_generate = EXCLUDED.auto_generate,
					is_active = EXCLUDED.is_active,
					updated_at = EXCLUDED.updated_at
			`

			_, err := target.Exec(ctx, insertQuery,
				row.ID,
				row.UserID,
				row.Store,
				row.Credentials,
				row.AutoGenerate,
				row.IsActive,
				row.CreatedAt,
				row.UpdatedAt,
			)
			if err != nil {
				stats.TotalFailed++
				stats.Errors = append(stats.Errors, fmt.Sprintf("insert error for %s/%s: %v", row.UserID, row.Store, err))
				log.Printf("Failed to insert credential: user_id=%s, store=%s, error=%v\n", row.UserID, row.Store, err)
				continue
			}

			stats.TotalMigrated++
		}

		offset += batchSize
	}

	// Verify migration
	if !dryRun {
		var targetCount int
		err := target.QueryRow(ctx, "SELECT COUNT(*) FROM affiliate_credentials").Scan(&targetCount)
		if err != nil {
			log.Printf("Warning: Could not verify target count: %v\n", err)
		} else {
			log.Printf("Verification: Source=%d, Target=%d\n", totalCount, targetCount)
		}
	}

	return nil
}

func printSummary(stats *MigrationStats, dryRun bool) {
	fmt.Println("\n========== Migration Summary ==========")
	if dryRun {
		fmt.Println("MODE: DRY RUN (no data was written)")
	}
	fmt.Printf("Total records read:    %d\n", stats.TotalRead)
	fmt.Printf("Total records skipped: %d (orphaned users)\n", stats.TotalSkipped)
	fmt.Printf("Total records migrated: %d\n", stats.TotalMigrated)
	fmt.Printf("Total records failed:  %d\n", stats.TotalFailed)

	if len(stats.Errors) > 0 {
		fmt.Println("\nErrors encountered:")
		for i, err := range stats.Errors {
			if i >= 10 {
				fmt.Printf("  ... and %d more errors\n", len(stats.Errors)-10)
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}

	fmt.Println("========================================")
}
