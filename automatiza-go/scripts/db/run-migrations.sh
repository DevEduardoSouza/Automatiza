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

# Discover migrations dynamically in numeric order.
# Includes: NNN_name.sql (legacy) and NNN_name.up.sql (post-010).
# Excludes: *.down.sql (rollbacks) and *_rollback.sql (legacy rollback variant).
# All migrations are expected to be idempotent (IF NOT EXISTS, etc).
mapfile -t MIGRATION_FILES < <(
    find "$MIGRATIONS_DIR" -maxdepth 1 -type f -name "*.sql" \
        ! -name "*.down.sql" ! -name "*_rollback.sql" \
        -printf "%f\n" | sort
)

if [ "${#MIGRATION_FILES[@]}" -eq 0 ]; then
    echo "  WARN: no migration files found in ${MIGRATIONS_DIR}"
fi

for migration in "${MIGRATION_FILES[@]}"; do
    filepath="${MIGRATIONS_DIR}/${migration}"
    echo "  Applying: ${migration}"
    $PSQL_CMD -f "$filepath" 2>&1 || true
done

echo "=== Migrations complete ==="
