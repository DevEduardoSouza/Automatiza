#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/automatiza-migration-runner.XXXXXX")"
container="automatiza-migration-test-$$"
postgres_password="migration-test-only"

cleanup() {
    docker rm -f "$container" >/dev/null 2>&1 || true
    rm -rf "$test_root"
}
trap cleanup EXIT

fail() {
    echo "Migration runner test failed: $*" >&2
    exit 1
}

docker info >/dev/null 2>&1 || fail "Docker is required"
docker run --rm -d \
    --name "$container" \
    -e POSTGRES_PASSWORD="$postgres_password" \
    -e POSTGRES_DB=postgres \
    -v "$test_root:/test-fixtures" \
    -v "$script_dir:/runner:ro" \
    postgres:16-alpine >/dev/null

for _ in $(seq 1 60); do
    if docker exec "$container" pg_isready -h 127.0.0.1 -U postgres -d postgres >/dev/null 2>&1; then
        break
    fi
    sleep 1
done
docker exec "$container" pg_isready -h 127.0.0.1 -U postgres -d postgres >/dev/null 2>&1 \
    || fail "PostgreSQL 16 did not become ready"

psql_test() {
    local database="$1"
    shift
    docker exec -e PGPASSWORD="$postgres_password" "$container" psql \
        -X -v ON_ERROR_STOP=1 \
        -h 127.0.0.1 -U postgres -d "$database" \
        "$@"
}

create_database() {
    local database="$1"
    psql_test postgres -c "CREATE DATABASE ${database}" >/dev/null
}

run_migrations() {
    local directory="$1"
    local database="$2"
    local trace_id="$3"
    local container_directory="/test-fixtures/${directory#"$test_root"/}"
    docker exec \
        -e PGPASSWORD="$postgres_password" \
        -e MIGRATIONS_DIR="$container_directory" \
        -e MIGRATIONS_DIR_OVERRIDE=1 \
        -e MIGRATION_TRACE_ID="$trace_id" \
        -e DB_HOST=127.0.0.1 \
        -e DB_PORT=5432 \
        -e DB_USER=postgres \
        -e DB_NAME="$database" \
        "$container" /runner/run-migrations.sh
}

assert_query() {
    local database="$1"
    local expected="$2"
    local sql="$3"
    local actual
    actual="$(psql_test "$database" -tA -c "$sql")"
    [[ "$actual" == "$expected" ]] \
        || fail "query returned '$actual', expected '$expected': $sql"
}

assert_runner_fails() {
    local expected_code="$1"
    local directory="$2"
    local database="$3"
    local trace_id="$4"
    local output
    local status

    set +e
    output="$(run_migrations "$directory" "$database" "$trace_id" 2>&1)"
    status=$?
    set -e

    [[ "$status" -ne 0 ]] || fail "runner unexpectedly succeeded for $trace_id"
    grep -Fq "error_code=${expected_code}" <<<"$output" \
        || fail "missing error_code=${expected_code} for $trace_id: $output"
}

assert_operator_script_is_inert() {
    local expected_code="$1"
    local database="$2"
    local script="$3"
    local output
    local status

    set +e
    output="$(psql_test "$database" -f "$script" 2>&1)"
    status=$?
    set -e

    [[ "$status" -ne 0 ]] || fail "operator script ran without confirmation: $script"
    grep -Fq "error_code=${expected_code}" <<<"$output" \
        || fail "operator script did not report ${expected_code}: $output"
}

success_dir="$test_root/success"
mkdir -p "$success_dir"
cat >"$success_dir/001_create_events.up.sql" <<'SQL'
-- automatiza:migration-mode=transactional
CREATE TABLE migration_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payload text NOT NULL
);
INSERT INTO migration_events (payload) VALUES ('applied-once');
SQL
cat >"$success_dir/002_events_index.up.sql" <<'SQL'
-- automatiza:migration-mode=nontransactional
SET lock_timeout = '1s';
CREATE INDEX CONCURRENTLY idx_migration_events_payload
    ON migration_events (payload);
RESET lock_timeout;
SQL
cat >"$success_dir/002_events_index.verify.sql" <<'SQL'
SELECT COALESCE((
    SELECT i.indisvalid AND i.indisready
    FROM pg_index i
    JOIN pg_class c ON c.oid = i.indexrelid
    WHERE c.relname = 'idx_migration_events_payload'
), false) AS migration_verified
\gset
SQL
create_database migration_success
run_migrations "$success_dir" migration_success success-first >/dev/null
run_migrations "$success_dir" migration_success success-restart >/dev/null
assert_query migration_success "1" "SELECT count(*) FROM migration_events"
assert_query migration_success "2" "SELECT count(*) FROM _deploy_migrations"
assert_query migration_success "2" \
    "SELECT count(*) FROM _deploy_migration_attempts WHERE status = 'applied'"

failure_dir="$test_root/transaction-failure"
mkdir -p "$failure_dir"
cat >"$failure_dir/001_rollback.up.sql" <<'SQL'
-- automatiza:migration-mode=transactional
CREATE TABLE must_rollback (id bigint);
INSERT INTO must_rollback VALUES (1);
SELECT * FROM table_that_does_not_exist;
SQL
create_database migration_failure
assert_runner_fails migration_apply_failed "$failure_dir" migration_failure failure-first
assert_query migration_failure "" "SELECT to_regclass('public.must_rollback')"
assert_query migration_failure "failed|migration_apply_failed" \
    "SELECT status || '|' || error_code FROM _deploy_migration_attempts WHERE filename = '001_rollback.up.sql'"
assert_runner_fails migration_intervention_required \
    "$failure_dir" migration_failure failure-restart
assert_query migration_failure "" "SELECT to_regclass('public.must_rollback')"

verify_dir="$test_root/verifier-failure"
mkdir -p "$verify_dir"
cat >"$verify_dir/001_create_items.up.sql" <<'SQL'
-- automatiza:migration-mode=transactional
CREATE TABLE migration_items (id bigint);
SQL
cat >"$verify_dir/002_bad_verifier.up.sql" <<'SQL'
-- automatiza:migration-mode=nontransactional
CREATE INDEX CONCURRENTLY idx_migration_items_id ON migration_items (id);
SQL
cat >"$verify_dir/002_bad_verifier.verify.sql" <<'SQL'
SELECT false AS migration_verified
\gset
SQL
create_database migration_verify
assert_runner_fails migration_verify_failed "$verify_dir" migration_verify verify-first
assert_query migration_verify "blocked|migration_verify_failed" \
    "SELECT status || '|' || error_code FROM _deploy_migration_attempts WHERE filename = '002_bad_verifier.up.sql'"
assert_query migration_verify "1" \
    "SELECT count(*) FROM _deploy_migrations WHERE filename = '001_create_items.up.sql'"
assert_query migration_verify "0" \
    "SELECT count(*) FROM _deploy_migrations WHERE filename = '002_bad_verifier.up.sql'"
assert_runner_fails migration_intervention_required \
    "$verify_dir" migration_verify verify-restart

checksum_dir="$test_root/checksum"
mkdir -p "$checksum_dir"
cat >"$checksum_dir/001_checksum.up.sql" <<'SQL'
-- automatiza:migration-mode=transactional
CREATE TABLE checksum_guard (id bigint);
SQL
create_database migration_checksum
run_migrations "$checksum_dir" migration_checksum checksum-first >/dev/null
printf '\n-- changed after application\n' >>"$checksum_dir/001_checksum.up.sql"
assert_runner_fails migration_checksum_mismatch \
    "$checksum_dir" migration_checksum checksum-second
assert_query migration_checksum "blocked|migration_checksum_mismatch" \
    "SELECT status || '|' || error_code FROM _deploy_migration_attempts WHERE filename = '001_checksum.up.sql'"

concurrency_dir="$test_root/concurrency"
mkdir -p "$concurrency_dir"
cat >"$concurrency_dir/001_slow.up.sql" <<'SQL'
-- automatiza:migration-mode=transactional
SELECT pg_sleep(4);
CREATE TABLE concurrency_guard (id bigint);
SQL
create_database migration_concurrency
run_migrations "$concurrency_dir" migration_concurrency concurrency-first \
    >"$test_root/concurrency-first.log" 2>&1 &
first_runner=$!
for _ in $(seq 1 40); do
    if [[ "$(psql_test migration_concurrency -tA -c \
        "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'automatiza-migration-concurrency-first'")" == "1" ]]; then
        break
    fi
    sleep 0.1
done
assert_runner_fails migration_lock_busy \
    "$concurrency_dir" migration_concurrency concurrency-second
wait "$first_runner" || fail "first concurrent runner failed"
assert_query migration_concurrency "concurrency_guard" \
    "SELECT to_regclass('public.concurrency_guard')"

crash_dir="$test_root/crash"
mkdir -p "$crash_dir"
cat >"$crash_dir/001_partial_effect.up.sql" <<'SQL'
-- automatiza:migration-mode=nontransactional
CREATE TABLE partial_effect (id bigint);
SELECT pg_sleep(30);
SQL
cat >"$crash_dir/001_partial_effect.verify.sql" <<'SQL'
SELECT to_regclass('public.partial_effect') IS NOT NULL AS migration_verified
\gset
SQL
create_database migration_crash
run_migrations "$crash_dir" migration_crash crash-first \
    >"$test_root/crash-first.log" 2>&1 &
crash_runner=$!
for _ in $(seq 1 50); do
    backend_pid="$(psql_test migration_crash -tA -c "
        SELECT COALESCE(max(pid), 0)
        FROM pg_stat_activity
        WHERE application_name = 'automatiza-migration-crash-first'
          AND query LIKE '%pg_sleep(30)%'
    ")"
    [[ "$backend_pid" != "0" ]] && break
    sleep 0.1
done
[[ "${backend_pid:-0}" != "0" ]] || fail "could not locate crash-test backend"
psql_test migration_crash -c "SELECT pg_terminate_backend(${backend_pid})" >/dev/null
set +e
wait "$crash_runner"
crash_status=$?
set -e
[[ "$crash_status" -ne 0 ]] || fail "terminated migration unexpectedly succeeded"
assert_query migration_crash "partial_effect" \
    "SELECT to_regclass('public.partial_effect')"
assert_query migration_crash "failed|migration_apply_failed" \
    "SELECT status || '|' || error_code FROM _deploy_migration_attempts WHERE filename = '001_partial_effect.up.sql'"
assert_runner_fails migration_intervention_required \
    "$crash_dir" migration_crash crash-restart

legacy_dir="$test_root/legacy"
mkdir -p "$legacy_dir"
cat >"$legacy_dir/001_unmarked.up.sql" <<'SQL'
CREATE TABLE legacy_must_not_run (id bigint);
SQL
create_database migration_legacy
assert_runner_fails migration_mode_invalid \
    "$legacy_dir" migration_legacy legacy-first
assert_query migration_legacy "" "SELECT to_regclass('public.legacy_must_not_run')"

product_dir="$test_root/product-url"
mkdir -p "$product_dir"
cat >"$product_dir/001_product_url.up.sql" <<'SQL'
-- automatiza:migration-mode=nontransactional
CREATE INDEX CONCURRENTLY idx_dispatch_logs_product_url_trgm
    ON dispatch_logs USING gin(product_url gin_trgm_ops);
SQL
cat >"$product_dir/001_product_url.verify.sql" <<'SQL'
SELECT true AS migration_verified
\gset
SQL
create_database migration_product
assert_runner_fails migration_blocked_ddl \
    "$product_dir" migration_product product-first
assert_query migration_product "" \
    "SELECT to_regclass('public.idx_dispatch_logs_product_url_trgm')"

operations_dir="/runner/operations/wp-002"
psql_test migration_success \
    -f "$operations_dir/diagnose-product-url-index.sql" >/dev/null
assert_operator_script_is_inert wp002_operator_confirmation_missing \
    migration_success "$operations_dir/drop-invalid-product-url-index.sql"
assert_operator_script_is_inert wp002_operator_confirmation_missing \
    migration_success "$operations_dir/create-product-url-index-recovery.sql"

create_database migration_operations
psql_test migration_operations -c "CREATE EXTENSION pg_trgm" >/dev/null
psql_test migration_operations -c \
    "CREATE TABLE dispatch_logs (product_url text)" >/dev/null
psql_test migration_operations \
    -v wp002_confirm=CREATE_PRODUCT_URL_INDEX_RECOVERY \
    -f "$operations_dir/create-product-url-index-recovery.sql" >/dev/null
assert_query migration_operations "1|1" \
    "SELECT indisvalid::int || '|' || indisready::int FROM pg_index WHERE indexrelid = 'idx_dispatch_logs_product_url_trgm'::regclass"

create_database migration_repository_files
psql_test migration_repository_files -c "
    CREATE TABLE dispatch_logs (
        user_id uuid,
        created_at timestamptz,
        group_id text,
        store text,
        product_url text
    )
" >/dev/null
repository_verify="$test_root/repository-verify.sql"
cat >"$repository_verify" <<SQL
\\set ON_ERROR_STOP on
\\set migration_verified false
\\i '/runner/migrations/019_dispatch_logs_performance_indexes.up.sql'
\\i '/runner/migrations/019_dispatch_logs_performance_indexes.verify.sql'
\\if :migration_verified
\\else
SELECT 1 / 0;
\\endif
\\set migration_verified false
\\i '/runner/migrations/064_dispatch_logs_list_item_id.up.sql'
\\i '/runner/migrations/064_dispatch_logs_list_item_id.verify.sql'
\\if :migration_verified
\\else
SELECT 1 / 0;
\\endif
SQL
psql_test migration_repository_files \
    -f "/test-fixtures/${repository_verify#"$test_root"/}" >/dev/null
assert_query migration_repository_files "" \
    "SELECT to_regclass('public.idx_dispatch_logs_product_url_trgm')"
assert_query migration_repository_files "4" "
    SELECT count(*)
    FROM pg_index i
    JOIN pg_class c ON c.oid = i.indexrelid
    WHERE c.relname IN (
        'idx_dispatch_logs_user_created',
        'idx_dispatch_logs_group',
        'idx_dispatch_logs_store',
        'idx_dispatch_logs_list_item'
    )
      AND i.indisvalid
      AND i.indisready
"

echo "Migration runner integration tests passed on PostgreSQL 16."
