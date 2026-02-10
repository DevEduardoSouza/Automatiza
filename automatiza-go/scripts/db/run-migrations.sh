#!/bin/bash
# =============================================================================
# Run all database migrations in order
# =============================================================================
# Contexts:
#   1. docker-entrypoint-initdb.d: POSTGRES_USER and POSTGRES_DB are set
#   2. Manual: pass host/port/user/db as arguments or use DB_* env vars
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Detect if running inside docker-entrypoint-initdb.d
if [ -n "$POSTGRES_USER" ] && [ -d "/docker-entrypoint-initdb.d/migrations" ]; then
    MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"
    PSQL_CMD="psql -v ON_ERROR_STOP=0 --username $POSTGRES_USER --dbname ${POSTGRES_DB:-$POSTGRES_USER}"
else
    MIGRATIONS_DIR="${SCRIPT_DIR}/migrations"
    DB_HOST="${1:-${DB_HOST:-localhost}}"
    DB_PORT="${2:-${DB_PORT:-5432}}"
    DB_USER="${3:-${DB_USER:-automatiza}}"
    DB_NAME="${4:-${DB_NAME:-automatiza}}"
    PSQL_CMD="psql -v ON_ERROR_STOP=0 -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"
fi

echo "=== Running database migrations ==="

# Migration files in order
# 000 is the consolidated init (creates all 33 tables for fresh databases)
# 001-029 are incremental (idempotent, safe to run after 000)
MIGRATION_FILES=(
    "000_postgresql_init.sql"
    "001_initial_schema.sql"
    "002_dashboard_settings.sql"
    "003_webhook_events.sql"
    "004_reconciliation_checkpoints.sql"
    "005_password_reset.sql"
    "006_groups_favorites.sql"
    "007_redirect_links_parity.sql"
    "008_jobs_table.sql"
    "009_ml_link_lists_parity.sql"
    "010_add_lock_until_to_automation_runs.up.sql"
    "011_ml_link_list_items_aliases.up.sql"
    "012_ml_links_unconsumed_index.up.sql"
    "013_automations_coupon_id.up.sql"
    "014_affiliate_credentials_update.up.sql"
    "015_plans_extended_fields.up.sql"
    "016_sync_jobs.up.sql"
    "017_sessions_schema_parity.up.sql"
    "018_dispatch_logs_coupon_fields.up.sql"
    "019_dispatch_logs_performance_indexes.up.sql"
    "020_dispatch_texts_schema_parity.up.sql"
    "021_jobs_retry.up.sql"
    "022_ml_link_list_items_unique_constraint.up.sql"
    "023_action_history_count_fields.up.sql"
    "024_plans_features_to_text_array.up.sql"
    "025_email_template_description.up.sql"
    "026_admin_settings_seed.up.sql"
    "027_rename_evolution_to_waha.up.sql"
    "027_webhook_logs_kiwify_fields.up.sql"
    "028_categories_table.up.sql"
    "029_automations_offer_texts_columns.up.sql"
)

for migration in "${MIGRATION_FILES[@]}"; do
    filepath="${MIGRATIONS_DIR}/${migration}"
    if [ -f "$filepath" ]; then
        echo "  Applying: ${migration}"
        $PSQL_CMD -f "$filepath" 2>&1 || true
    else
        echo "  SKIP: ${migration} (not found)"
    fi
done

echo "=== Migrations complete ==="
