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
    # ON_ERROR_STOP=1 (produção): erro de SQL aborta o psql com exit != 0, o
    # arquivo NÃO entra no ledger e o deploy FALHA ruidosamente (o CI não
    # mascara mais — deploy.yml). Antes (=0) a migração quebrada saía com
    # exit 0, era gravada como aplicada e o drift ficava invisível até virar
    # incidente (precedente: constraint 23514 / migração 071). Migração deve
    # ser idempotente (IF NOT EXISTS etc.) — regra do repo, agora mecânica.
    # NÃO adicionar -1/single-transaction: CREATE INDEX CONCURRENTLY não roda
    # em transação.
    PSQL_CMD="psql -v ON_ERROR_STOP=1 -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"
fi

echo "=== Running database migrations ==="

# Discover migrations dynamically in numeric order.
# Includes: NNN_name.sql (legacy) and NNN_name.up.sql (post-010).
# Excludes: *.down.sql (rollbacks) and *_rollback.sql (legacy rollback variant).
# All migrations are expected to be idempotent (IF NOT EXISTS, etc).
#
# Uses a bash glob instead of `find -printf` because this script runs inside
# the backend image (Alpine + BusyBox find), and BusyBox find does NOT
# support -printf. Under BusyBox the flag fails silently, leaving the array
# empty and skipping ALL migrations — exactly what happened between
# 2026-04-17 (PR #57) and this fix.
shopt -s nullglob
MIGRATION_FILES=()
for filepath in "$MIGRATIONS_DIR"/*.sql; do
    name="${filepath##*/}"
    case "$name" in
        *.down.sql|*_rollback.sql) continue ;;
    esac
    MIGRATION_FILES+=("$name")
done
shopt -u nullglob

# Bash glob already sorts alphabetically; with zero-padded NNN_ prefixes
# this is equivalent to numeric order.

if [ "${#MIGRATION_FILES[@]}" -eq 0 ]; then
    echo "  ERROR: no migration files found in ${MIGRATIONS_DIR}" >&2
    exit 1
fi

# Applied-migrations ledger: with 135+ files, re-running every one on every
# deploy cost ~60s of the pipeline. A file is recorded only when psql exits 0.
# Em produção (ON_ERROR_STOP=1) qualquer erro — de SQL ou de conexão — sai
# non-zero, NÃO é gravado e ABORTA o script (migrações seguintes podem
# depender da que falhou); o deploy fica vermelho e a migração é re-tentada
# no próximo run. If the ledger itself is unreachable, APPLIED stays empty
# and we fall back to running everything, exactly as before.
TRACK_TABLE="_deploy_migrations"
$PSQL_CMD -q -c "CREATE TABLE IF NOT EXISTS ${TRACK_TABLE} (filename text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())" >/dev/null 2>&1 || true
APPLIED=$($PSQL_CMD -tA -c "SELECT filename FROM ${TRACK_TABLE}" 2>/dev/null || true)

SKIPPED=0
for migration in "${MIGRATION_FILES[@]}"; do
    if printf '%s\n' "$APPLIED" | grep -qxF "$migration"; then
        SKIPPED=$((SKIPPED + 1))
        continue
    fi
    filepath="${MIGRATIONS_DIR}/${migration}"
    echo "  Applying: ${migration}"
    if $PSQL_CMD -f "$filepath" 2>&1; then
        $PSQL_CMD -q -c "INSERT INTO ${TRACK_TABLE} (filename) VALUES ('${migration}') ON CONFLICT (filename) DO NOTHING" >/dev/null 2>&1 || true
    else
        # A condição do if é imune ao set -e: sem este else, uma migração
        # quebrada seguiria para as próximas e o script sairia 0 (falha
        # silenciosa de novo). Aborta aqui — o CI trata como deploy vermelho.
        echo "  ERROR: migration failed: ${migration}" >&2
        exit 1
    fi
done

if [ "$SKIPPED" -gt 0 ]; then
    echo "  Skipped ${SKIPPED} already-applied migrations"
fi

echo "=== Migrations complete ==="
