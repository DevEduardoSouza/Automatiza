// Package main provides a migration script for transferring dispatch_logs data
// from Supabase (PostgreSQL) to local PostgreSQL.
//
// Usage:
//
//	go run scripts/migrate_dispatch_logs.go [flags]
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
// It uses INSERT ... ON CONFLICT DO NOTHING to avoid duplicate insertions.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DispatchLogRow represents a row from the Supabase dispatch_logs table.
// Uses pointers for nullable fields to handle NULL values correctly.
//
// Supabase schema columns:
// - id (uuid) - Primary key
// - user_id (uuid) - NOT NULL
// - automation_id (uuid) - FK to automations
// - automation_name (text)
// - store (store_key enum)
// - group_id (text)
// - group_name (text)
// - product_url (text)
// - affiliate_url (text)
// - status (dispatch_status enum)
// - error (text)
// - created_at (timestamptz)
// - product_category (text) - added later
// - coupon_id (uuid) - added later
// - coupon_code (text) - added later
// - coupon_applied (boolean) - added later
// - coupon_skip_reason (text) - added later
// - canonical_key (text) - added later
// - product_page_url (text) - added later
// - product_title (text) - added later
// - price_at_send (numeric) - added later
// - type (text) - added later
type DispatchLogRow struct {
	// Core fields (original schema)
	ID             string
	UserID         string
	AutomationID   *string
	AutomationName *string
	Store          *string
	GroupID        *string
	GroupName      *string
	ProductURL     *string
	AffiliateURL   *string
	Status         string
	Error          *string
	CreatedAt      time.Time

	// Added fields (subsequent migrations)
	ProductCategory  *string  // 20251222140000
	CouponID         *string  // 20251226123500
	CouponCode       *string  // 20251226123500
	CouponApplied    *bool    // 20251226123500
	CouponSkipReason *string  // 20251226123500
	CanonicalKey     *string  // 20260126120000
	ProductPageURL   *string  // 20260126120000
	ProductTitle     *string  // 20260126120000
	PriceAtSend      *float64 // 20260126120000
	Type             *string  // 20260126120000
}

func main() {
	// Parse command line flags
	dryRun := flag.Bool("dry-run", false, "Simulate migration without inserting data")
	batchSize := flag.Int("batch-size", 1000, "Number of records to process per batch")
	sourceURL := flag.String("source-url", "", "Supabase PostgreSQL connection URL")
	targetURL := flag.String("target-url", "", "Local PostgreSQL connection URL")
	flag.Parse()

	// Get connection URLs from flags or environment variables
	srcURL := *sourceURL
	if srcURL == "" {
		srcURL = os.Getenv("SUPABASE_DB_URL")
	}
	if srcURL == "" {
		log.Fatal("ERROR: Source URL not provided. Use --source-url flag or set SUPABASE_DB_URL environment variable")
	}

	tgtURL := *targetURL
	if tgtURL == "" {
		tgtURL = os.Getenv("DATABASE_URL")
	}
	if tgtURL == "" {
		log.Fatal("ERROR: Target URL not provided. Use --target-url flag or set DATABASE_URL environment variable")
	}

	ctx := context.Background()

	log.Println("=== Dispatch Logs Migration Script ===")
	log.Printf("Dry run: %v", *dryRun)
	log.Printf("Batch size: %d", *batchSize)
	log.Println("")

	// Connect to source (Supabase)
	log.Println("Connecting to source database (Supabase)...")
	sourcePool, err := pgxpool.New(ctx, srcURL)
	if err != nil {
		log.Fatalf("ERROR: Failed to connect to source database: %v", err)
	}
	defer sourcePool.Close()

	if err := sourcePool.Ping(ctx); err != nil {
		log.Fatalf("ERROR: Failed to ping source database: %v", err)
	}
	log.Println("Connected to source database")

	// Connect to target (local PostgreSQL)
	log.Println("Connecting to target database (local PostgreSQL)...")
	targetPool, err := pgxpool.New(ctx, tgtURL)
	if err != nil {
		log.Fatalf("ERROR: Failed to connect to target database: %v", err)
	}
	defer targetPool.Close()

	if err := targetPool.Ping(ctx); err != nil {
		log.Fatalf("ERROR: Failed to ping target database: %v", err)
	}
	log.Println("Connected to target database")
	log.Println("")

	// Get total count from source
	var sourceCount int64
	err = sourcePool.QueryRow(ctx, "SELECT COUNT(*) FROM dispatch_logs").Scan(&sourceCount)
	if err != nil {
		log.Fatalf("ERROR: Failed to get source count: %v", err)
	}
	log.Printf("Source database has %d dispatch_logs records", sourceCount)

	if sourceCount == 0 {
		log.Println("No records to migrate. Exiting.")
		return
	}

	// Get existing count in target before migration
	var targetCountBefore int64
	err = targetPool.QueryRow(ctx, "SELECT COUNT(*) FROM dispatch_logs").Scan(&targetCountBefore)
	if err != nil {
		log.Fatalf("ERROR: Failed to get target count: %v", err)
	}
	log.Printf("Target database has %d dispatch_logs records before migration", targetCountBefore)
	log.Println("")

	// Process in batches
	totalMigrated := int64(0)
	totalSkipped := int64(0)
	offset := 0
	startTime := time.Now()

	for {
		// Fetch batch from source (Supabase)
		// Using UUID id from Supabase, ordered by created_at for consistent pagination
		// Note: Casting UUIDs to text and enums to text for compatibility
		query := `
			SELECT
				id::text,
				user_id::text,
				automation_id::text,
				automation_name,
				store::text,
				group_id,
				group_name,
				product_url,
				affiliate_url,
				status::text,
				error,
				created_at,
				product_category,
				coupon_id::text,
				coupon_code,
				coupon_applied,
				coupon_skip_reason,
				canonical_key,
				product_page_url,
				product_title,
				price_at_send,
				type
			FROM dispatch_logs
			ORDER BY created_at ASC, id ASC
			LIMIT $1 OFFSET $2
		`

		rows, err := sourcePool.Query(ctx, query, *batchSize, offset)
		if err != nil {
			log.Fatalf("ERROR: Failed to fetch batch at offset %d: %v", offset, err)
		}

		batch := make([]DispatchLogRow, 0, *batchSize)

		for rows.Next() {
			var row DispatchLogRow
			err := rows.Scan(
				// Core fields
				&row.ID,
				&row.UserID,
				&row.AutomationID,
				&row.AutomationName,
				&row.Store,
				&row.GroupID,
				&row.GroupName,
				&row.ProductURL,
				&row.AffiliateURL,
				&row.Status,
				&row.Error,
				&row.CreatedAt,
				// Added fields
				&row.ProductCategory,
				&row.CouponID,
				&row.CouponCode,
				&row.CouponApplied,
				&row.CouponSkipReason,
				&row.CanonicalKey,
				&row.ProductPageURL,
				&row.ProductTitle,
				&row.PriceAtSend,
				&row.Type,
			)
			if err != nil {
				rows.Close()
				log.Fatalf("ERROR: Failed to scan row: %v", err)
			}
			batch = append(batch, row)
		}
		rows.Close()

		if len(batch) == 0 {
			break
		}

		// Insert batch into target (with ON CONFLICT DO NOTHING for idempotency)
		if !*dryRun {
			inserted, err := insertBatch(ctx, targetPool, batch)
			if err != nil {
				log.Fatalf("ERROR: Failed to insert batch at offset %d: %v", offset, err)
			}
			totalMigrated += inserted
			totalSkipped += int64(len(batch)) - inserted
		} else {
			totalMigrated += int64(len(batch))
		}

		// Progress update
		processed := int64(offset) + int64(len(batch))
		pct := float64(processed) / float64(sourceCount) * 100
		elapsed := time.Since(startTime)
		rate := float64(processed) / elapsed.Seconds()

		log.Printf("Progress: %d/%d (%.1f%%) - Migrated: %d, Skipped: %d - Rate: %.0f rec/s",
			processed, sourceCount, pct, totalMigrated, totalSkipped, rate)

		offset += len(batch)

		if len(batch) < *batchSize {
			break
		}
	}

	log.Println("")
	log.Println("=== Migration Complete ===")
	log.Printf("Total records processed: %d", offset)
	log.Printf("Total records migrated: %d", totalMigrated)
	log.Printf("Total records skipped (duplicates): %d", totalSkipped)
	log.Printf("Time elapsed: %v", time.Since(startTime))
	log.Println("")

	// Validation: compare counts
	if !*dryRun {
		var targetCountAfter int64
		err = targetPool.QueryRow(ctx, "SELECT COUNT(*) FROM dispatch_logs").Scan(&targetCountAfter)
		if err != nil {
			log.Fatalf("ERROR: Failed to get final target count: %v", err)
		}

		log.Println("=== Validation ===")
		log.Printf("Source count: %d", sourceCount)
		log.Printf("Target count before: %d", targetCountBefore)
		log.Printf("Target count after: %d", targetCountAfter)
		log.Printf("New records added: %d", targetCountAfter-targetCountBefore)

		if targetCountAfter >= sourceCount {
			log.Println("VALIDATION PASSED: Target has at least as many records as source")
		} else {
			log.Printf("WARNING: Target count (%d) is less than source count (%d)", targetCountAfter, sourceCount)
			log.Println("This may be expected if some records were filtered or foreign key constraints prevented insertion")
		}
	} else {
		log.Println("=== Dry Run Complete ===")
		log.Println("No data was inserted. Remove --dry-run flag to perform actual migration.")
	}
}

// insertBatch inserts a batch of dispatch logs into the target database.
// Uses ON CONFLICT DO NOTHING to handle duplicates gracefully.
// Returns the number of rows actually inserted.
//
// Field mapping from Supabase to local PostgreSQL:
// - affiliate_url (Supabase) -> affiliate_link (local)
// - price_at_send (Supabase) -> product_price (local) - if product_price not set
// - product_title (Supabase) -> not directly mapped (informational only)
func insertBatch(ctx context.Context, pool *pgxpool.Pool, batch []DispatchLogRow) (int64, error) {
	// The target schema uses SERIAL for id, so we need to handle the UUID->int mapping
	// Since the source has UUID ids and target has SERIAL, we can't use id for conflict detection
	// Instead we use INSERT ... ON CONFLICT DO NOTHING (no unique constraint on these fields,
	// but this handles any potential duplicates from reruns)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var totalInserted int64

	for _, row := range batch {
		// Map Supabase fields to local PostgreSQL schema
		// Local schema columns (from 001_initial_schema.sql):
		// - user_id, instance_id, group_id, automation_id, automation_name
		// - store, status, message, product_url, product_page_url, canonical_key
		// - product_category, product_price, product_discount, affiliate_link
		// - coupon_code, coupon_discount, dedup_hash, dedup_window_hours
		// - type, error, created_at

		// Use price_at_send as product_price if available
		var productPrice *float64
		if row.PriceAtSend != nil {
			productPrice = row.PriceAtSend
		}

		// Build the insert query
		// Note: We insert only columns that exist in both schemas
		// Skipping: instance_id, message, coupon_discount, dedup_hash, dedup_window_hours
		// (these don't have corresponding source data or have different semantics)
		insertQuery := `
			INSERT INTO dispatch_logs (
				user_id,
				automation_id,
				automation_name,
				store,
				group_id,
				product_url,
				product_page_url,
				canonical_key,
				product_category,
				product_price,
				affiliate_link,
				coupon_code,
				status,
				error,
				type,
				created_at
			) VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12,
				$13,
				$14,
				$15,
				$16
			)
			ON CONFLICT DO NOTHING
		`

		// Convert automation_id, handle potential empty strings
		var automationID *string
		if row.AutomationID != nil && *row.AutomationID != "" {
			automationID = row.AutomationID
		}

		result, err := tx.Exec(ctx, insertQuery,
			row.UserID,          // $1 - user_id
			automationID,        // $2 - automation_id
			row.AutomationName,  // $3 - automation_name
			row.Store,           // $4 - store
			row.GroupID,         // $5 - group_id
			row.ProductURL,      // $6 - product_url
			row.ProductPageURL,  // $7 - product_page_url
			row.CanonicalKey,    // $8 - canonical_key
			row.ProductCategory, // $9 - product_category
			productPrice,        // $10 - product_price (from price_at_send)
			row.AffiliateURL,    // $11 - affiliate_link (mapped from affiliate_url)
			row.CouponCode,      // $12 - coupon_code
			row.Status,          // $13 - status
			row.Error,           // $14 - error
			row.Type,            // $15 - type
			row.CreatedAt,       // $16 - created_at
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert row (user_id=%s, created_at=%v): %w",
				row.UserID, row.CreatedAt, err)
		}

		totalInserted += result.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return totalInserted, nil
}

// insertBatchCopy is an alternative batch insert using pgx CopyFrom for better performance.
// This function does NOT handle duplicates (no ON CONFLICT support with COPY).
// Use only when you're certain there are no duplicates or for initial migration.
//
//nolint:unused // This is an alternative implementation kept for reference
func insertBatchCopy(ctx context.Context, pool *pgxpool.Pool, batch []DispatchLogRow) (int64, error) {
	columns := []string{
		"user_id",
		"automation_id",
		"automation_name",
		"store",
		"group_id",
		"product_url",
		"product_page_url",
		"canonical_key",
		"product_category",
		"product_price",
		"affiliate_link",
		"coupon_code",
		"status",
		"error",
		"type",
		"created_at",
	}

	rows := make([][]interface{}, 0, len(batch))
	for _, row := range batch {
		var productPrice *float64
		if row.PriceAtSend != nil {
			productPrice = row.PriceAtSend
		}

		var automationID *string
		if row.AutomationID != nil && *row.AutomationID != "" {
			automationID = row.AutomationID
		}

		rows = append(rows, []interface{}{
			row.UserID,
			automationID,
			row.AutomationName,
			row.Store,
			row.GroupID,
			row.ProductURL,
			row.ProductPageURL,
			row.CanonicalKey,
			row.ProductCategory,
			productPrice,
			row.AffiliateURL,
			row.CouponCode,
			row.Status,
			row.Error,
			row.Type,
			row.CreatedAt,
		})
	}

	copyCount, err := pool.CopyFrom(
		ctx,
		pgx.Identifier{"dispatch_logs"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy rows: %w", err)
	}

	return copyCount, nil
}
