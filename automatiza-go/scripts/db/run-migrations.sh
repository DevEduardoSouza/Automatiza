#!/usr/bin/env bash
# Runs ordered PostgreSQL migrations with explicit transaction modes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-${SCRIPT_DIR}/migrations}"
TRACE_ID="${MIGRATION_TRACE_ID:-local-$$}"
TRACK_TABLE="_deploy_migrations"
ATTEMPT_TABLE="_deploy_migration_attempts"

if [[ ! "$TRACE_ID" =~ ^[A-Za-z0-9._:-]{1,128}$ ]]; then
    echo "error_code=migration_trace_invalid event=migration_runner_blocked" >&2
    exit 64
fi

psql_args=(-X -v ON_ERROR_STOP=1 -v VERBOSITY=terse)
if [[ -n "${POSTGRES_USER:-}" && -d "/docker-entrypoint-initdb.d/migrations" && -z "${MIGRATIONS_DIR_OVERRIDE:-}" ]]; then
    MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"
    psql_args+=(--username "$POSTGRES_USER" --dbname "${POSTGRES_DB:-$POSTGRES_USER}")
else
    DB_HOST="${1:-${DB_HOST:-localhost}}"
    DB_PORT="${2:-${DB_PORT:-5432}}"
    DB_USER="${3:-${DB_USER:-automatiza}}"
    DB_NAME="${4:-${DB_NAME:-automatiza}}"
    psql_args+=(-h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME")
fi

run_psql() {
    PGAPPNAME="automatiza-migration-${TRACE_ID}" psql "${psql_args[@]}" "$@"
}

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

mode_for() {
    local marker
    marker="$(grep -m1 -E '^-- automatiza:migration-mode=(transactional|nontransactional)[[:space:]]*$' "$1" || true)"
    case "$marker" in
        "-- automatiza:migration-mode=transactional") echo "transactional" ;;
        "-- automatiza:migration-mode=nontransactional") echo "nontransactional" ;;
        "") echo "legacy-applied" ;;
        *)
            echo "error_code=migration_mode_invalid event=migration_runner_blocked migration=${1##*/} trace_id=${TRACE_ID}" >&2
            exit 64
            ;;
    esac
}

verifier_for() {
    local migration="$1"
    if [[ "$migration" == *.up.sql ]]; then
        printf '%s.verify.sql\n' "${migration%.up.sql}"
    else
        printf '%s.verify.sql\n' "${migration%.sql}"
    fi
}

append_verifier() {
    local control_file="$1"
    local verifier_path="$2"
    local migration="$3"
    cat >>"$control_file" <<EOF
\\set migration_verified false
\\i '$verifier_path'
\\if :migration_verified
\\else
UPDATE ${ATTEMPT_TABLE}
SET status = 'blocked',
    trace_id = '$TRACE_ID',
    finished_at = clock_timestamp(),
    error_code = 'migration_verify_failed'
WHERE filename = :'migration_filename';
\\echo error_code=migration_verify_failed event=migration_runner_blocked migration=$migration trace_id=$TRACE_ID
SELECT 1 / 0;
\\endif
EOF
}

shopt -s nullglob
migration_files=()
for filepath in "$MIGRATIONS_DIR"/*.sql; do
    name="${filepath##*/}"
    case "$name" in
        *.down.sql|*.verify.sql|*_rollback.sql) continue ;;
    esac
    migration_files+=("$name")
done
shopt -u nullglob

if [[ "${#migration_files[@]}" -eq 0 ]]; then
    echo "error_code=migration_files_missing event=migration_runner_blocked directory=${MIGRATIONS_DIR} trace_id=${TRACE_ID}" >&2
    exit 64
fi

control_file="$(mktemp "${TMPDIR:-/tmp}/automatiza-migrations.XXXXXX")"
runner_log="$(mktemp "${TMPDIR:-/tmp}/automatiza-migration-log.XXXXXX")"
cleanup() {
    rm -f "$control_file" "$runner_log"
}
trap cleanup EXIT

cat >"$control_file" <<EOF
\\set ON_ERROR_STOP on
\\set VERBOSITY terse
SET search_path = public, pg_catalog;
SELECT pg_try_advisory_lock(1096111177, 2) AS runner_lock_acquired \\gset
\\if :runner_lock_acquired
\\else
\\echo error_code=migration_lock_busy event=migration_runner_blocked trace_id=$TRACE_ID
SELECT 1 / 0;
\\endif

CREATE TABLE IF NOT EXISTS ${TRACK_TABLE} (
    filename text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ${ATTEMPT_TABLE} (
    filename text PRIMARY KEY,
    checksum text NOT NULL,
    mode text NOT NULL,
    status text NOT NULL,
    trace_id text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    finished_at timestamptz,
    error_code text,
    CHECK (mode IN ('transactional', 'nontransactional', 'legacy-applied')),
    CHECK (status IN ('running', 'applied', 'failed', 'blocked'))
);
EOF

for migration in "${migration_files[@]}"; do
    if [[ ! "$migration" =~ ^[0-9]{3}_[A-Za-z0-9_]+(\.up)?\.sql$ ]]; then
        echo "error_code=migration_filename_invalid event=migration_runner_blocked migration=${migration} trace_id=${TRACE_ID}" >&2
        exit 64
    fi

    filepath="${MIGRATIONS_DIR}/${migration}"
    mode="$(mode_for "$filepath")"
    checksum="$(sha256_file "$filepath")"
    verifier="$(verifier_for "$filepath")"
    has_verifier=false
    guard_sql="$(sed '/^[[:space:]]*--/d' "$filepath" | tr '\n' ' ')"
    [[ -f "$verifier" ]] && has_verifier=true

    if grep -Eiq '(^|[[:space:]])(CREATE|DROP|REINDEX)[[:space:]].*INDEX[[:space:]]+CONCURRENTLY' <<<"$guard_sql"; then
        if [[ "$mode" != "nontransactional" ]]; then
            echo "error_code=migration_mode_invalid event=migration_runner_blocked migration=${migration} trace_id=${TRACE_ID}" >&2
            exit 64
        fi
    fi

    if [[ "$mode" == "nontransactional" && "$has_verifier" != true ]]; then
        echo "error_code=migration_verifier_missing event=migration_runner_blocked migration=${migration} trace_id=${TRACE_ID}" >&2
        exit 64
    fi

    if grep -Eiq 'CREATE[[:space:]]+(UNIQUE[[:space:]]+)?INDEX[[:space:]]+(CONCURRENTLY[[:space:]]+)?(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?idx_dispatch_logs_product_url_trgm' <<<"$guard_sql"; then
        echo "error_code=migration_blocked_ddl event=migration_runner_blocked migration=${migration} trace_id=${TRACE_ID}" >&2
        exit 64
    fi

    cat >>"$control_file" <<EOF

\\set migration_filename '$migration'
\\set migration_checksum '$checksum'
\\set migration_mode '$mode'
SELECT EXISTS (
    SELECT 1 FROM ${TRACK_TABLE} WHERE filename = :'migration_filename'
) AS migration_applied \\gset

\\if :migration_applied
INSERT INTO ${ATTEMPT_TABLE}
    (filename, checksum, mode, status, trace_id, started_at, finished_at)
VALUES
    (:'migration_filename', :'migration_checksum', :'migration_mode', 'applied', '$TRACE_ID', clock_timestamp(), clock_timestamp())
ON CONFLICT (filename) DO NOTHING;

SELECT COALESCE((
    SELECT checksum = :'migration_checksum' AND status = 'applied'
    FROM ${ATTEMPT_TABLE}
    WHERE filename = :'migration_filename'
), false) AS migration_state_valid \\gset
\\if :migration_state_valid
EOF
    if [[ "$has_verifier" == true ]]; then
        append_verifier "$control_file" "$verifier" "$migration"
    fi
    cat >>"$control_file" <<EOF
\\echo event=migration_skipped migration=$migration mode=$mode trace_id=$TRACE_ID
\\else
UPDATE ${ATTEMPT_TABLE}
SET status = 'blocked',
    trace_id = '$TRACE_ID',
    finished_at = clock_timestamp(),
    error_code = CASE
        WHEN checksum <> :'migration_checksum' THEN 'migration_checksum_mismatch'
        ELSE 'migration_intervention_required'
    END
WHERE filename = :'migration_filename';
SELECT error_code AS migration_block_code
FROM ${ATTEMPT_TABLE}
WHERE filename = :'migration_filename'
\\gset
\\echo error_code=:migration_block_code event=migration_runner_blocked migration=$migration trace_id=$TRACE_ID
SELECT 1 / 0;
\\endif
\\else
EOF

    if [[ "$mode" == "legacy-applied" ]]; then
        cat >>"$control_file" <<EOF
INSERT INTO ${ATTEMPT_TABLE}
    (filename, checksum, mode, status, trace_id, error_code)
VALUES
    (:'migration_filename', :'migration_checksum', :'migration_mode', 'blocked', '$TRACE_ID', 'migration_mode_invalid')
ON CONFLICT (filename) DO NOTHING;
\\echo error_code=migration_mode_invalid event=migration_runner_blocked migration=$migration trace_id=$TRACE_ID
SELECT 1 / 0;
EOF
    else
        cat >>"$control_file" <<EOF
SELECT NOT EXISTS (
    SELECT 1 FROM ${ATTEMPT_TABLE} WHERE filename = :'migration_filename'
) AS migration_claim_available \\gset
\\if :migration_claim_available
INSERT INTO ${ATTEMPT_TABLE}
    (filename, checksum, mode, status, trace_id)
VALUES
    (:'migration_filename', :'migration_checksum', :'migration_mode', 'running', '$TRACE_ID');
\\else
UPDATE ${ATTEMPT_TABLE}
SET status = 'blocked',
    trace_id = '$TRACE_ID',
    finished_at = clock_timestamp(),
    error_code = 'migration_intervention_required'
WHERE filename = :'migration_filename';
\\echo error_code=migration_intervention_required event=migration_runner_blocked migration=$migration trace_id=$TRACE_ID
SELECT 1 / 0;
\\endif

\\echo event=migration_started migration=$migration mode=$mode trace_id=$TRACE_ID
EOF

        if [[ "$mode" == "transactional" ]]; then
            cat >>"$control_file" <<EOF
BEGIN;
\\i '$filepath'
INSERT INTO ${TRACK_TABLE} (filename) VALUES (:'migration_filename');
UPDATE ${ATTEMPT_TABLE}
SET status = 'applied',
    finished_at = clock_timestamp(),
    error_code = NULL
WHERE filename = :'migration_filename';
COMMIT;
EOF
        else
            cat >>"$control_file" <<EOF
\\i '$filepath'
EOF
            append_verifier "$control_file" "$verifier" "$migration"
            cat >>"$control_file" <<EOF
BEGIN;
INSERT INTO ${TRACK_TABLE} (filename) VALUES (:'migration_filename');
UPDATE ${ATTEMPT_TABLE}
SET status = 'applied',
    finished_at = clock_timestamp(),
    error_code = NULL
WHERE filename = :'migration_filename';
COMMIT;
EOF
        fi

        cat >>"$control_file" <<EOF
\\echo event=migration_applied migration=$migration mode=$mode trace_id=$TRACE_ID
EOF
    fi

    cat >>"$control_file" <<'EOF'
\endif
EOF
done

cat >>"$control_file" <<EOF

SELECT pg_advisory_unlock(1096111177, 2);
\\echo event=migrations_complete trace_id=$TRACE_ID
EOF

set +e
run_psql -f "$control_file" 2>&1 | tee "$runner_log"
runner_exit=${PIPESTATUS[0]}
set -e

if [[ "$runner_exit" -ne 0 ]]; then
    failure_code="$(
        sed -n 's/.*error_code=\([A-Za-z0-9_]*\).*/\1/p' "$runner_log" \
            | tail -n 1
    )"
    if [[ -z "$failure_code" ]]; then
        failure_code="$(run_psql -tA -c "
            SELECT COALESCE((
                SELECT error_code
                FROM ${ATTEMPT_TABLE}
                WHERE trace_id = '${TRACE_ID}' AND error_code IS NOT NULL
                ORDER BY started_at DESC
                LIMIT 1
            ), '')
        " 2>/dev/null || true)"
    fi

    if [[ -z "$failure_code" ]]; then
        if ! run_psql -q -c "
            UPDATE ${ATTEMPT_TABLE}
            SET status = 'failed',
                finished_at = clock_timestamp(),
                error_code = 'migration_apply_failed'
            WHERE trace_id = '${TRACE_ID}' AND status = 'running'
        " >/dev/null 2>&1; then
            echo "error_code=migration_ledger_unavailable event=migration_failure_record_failed trace_id=${TRACE_ID}" >&2
        fi
        failure_code="migration_apply_failed"
    fi

    echo "error_code=${failure_code} event=migration_runner_failed trace_id=${TRACE_ID}" >&2
    exit "$runner_exit"
fi
