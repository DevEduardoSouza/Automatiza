#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
validator="${repo_root}/scripts/validate-change-governance.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/automatiza-governance.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

mkdir -p \
    "$test_root/docs/change-records" \
    "$test_root/automatiza-go/scripts/db/migrations"

write_valid_record() {
    local destination="$1"
    {
        echo "# Test record"
        echo "## Metadata"
        echo "- Owner: test"
        echo "- Risk axis: governance"
        echo "## Hypothesis"
        echo "test"
        echo "## Scope"
        echo "### Files"
        echo "- test"
        echo "### Tables"
        echo "- None"
        echo "## Baseline"
        echo "test"
        echo "## Expand / migrate / observe / contract"
        echo "test"
        echo "## Canary"
        echo "test"
        echo "## Abort thresholds"
        echo "test"
        echo "## Rollout"
        echo "test"
        echo "## Rollback"
        echo "test"
        echo "## Evidence"
        echo "### Commands and results"
        echo "test"
        echo "### Approvals"
        echo "test"
        echo "## Residual risks and blockers"
        echo "None"
    } >"$destination"
}

assert_rejected() {
    local rule="$1"
    local file="$2"
    local output

    if output="$("$validator" "$file" 2>&1)"; then
        echo "Expected rejection for $file" >&2
        exit 1
    fi
    if ! grep -Fq "$rule" <<<"$output"; then
        echo "Expected $rule for $file; got: $output" >&2
        exit 1
    fi
}

write_verifier() {
    local migration="$1"
    local verifier="${migration%.up.sql}.verify.sql"
    {
        echo "SELECT true AS migration_verified"
        echo '\gset'
    } >"$verifier"
}

valid_record="$test_root/docs/change-records/valid.md"
valid_migration="$test_root/automatiza-go/scripts/db/migrations/900_safe.up.sql"
write_valid_record "$valid_record"
{
    echo "-- automatiza:migration-mode=nontransactional"
    echo "SET lock_timeout = '5s';"
    echo "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_safe ON test_table (id);"
} >"$valid_migration"
write_verifier "$valid_migration"
"$validator" "$valid_record" "$valid_migration" >/dev/null

dangerous_migration="$test_root/automatiza-go/scripts/db/migrations/901_drop.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "ALTER TABLE test_table DROP COLUMN payload;"
} >"$dangerous_migration"
assert_rejected "GOV-SQL-001" "$dangerous_migration"

blocking_index="$test_root/automatiza-go/scripts/db/migrations/902_index.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "SET lock_timeout = '5s';"
    echo "CREATE INDEX idx_blocking ON test_table (id);"
} >"$blocking_index"
assert_rejected "GOV-SQL-002" "$blocking_index"

missing_timeout="$test_root/automatiza-go/scripts/db/migrations/903_timeout.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "ALTER TABLE test_table ADD COLUMN safe_value text;"
} >"$missing_timeout"
assert_rejected "GOV-SQL-003" "$missing_timeout"

transaction_index="$test_root/automatiza-go/scripts/db/migrations/904_transaction.up.sql"
{
    echo "-- automatiza:migration-mode=nontransactional"
    echo "SET lock_timeout = '5s';"
    echo "BEGIN;"
    echo "CREATE INDEX CONCURRENTLY idx_transaction ON test_table (id);"
    echo "COMMIT;"
} >"$transaction_index"
write_verifier "$transaction_index"
assert_rejected "GOV-SQL-004" "$transaction_index"

drop_table="$test_root/automatiza-go/scripts/db/migrations/905_droptable.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "DROP TABLE test_table;"
} >"$drop_table"
assert_rejected "GOV-SQL-001" "$drop_table"

truncate_table="$test_root/automatiza-go/scripts/db/migrations/906_truncate.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "TRUNCATE test_table;"
} >"$truncate_table"
assert_rejected "GOV-SQL-001" "$truncate_table"

notnull_short="$test_root/automatiza-go/scripts/db/migrations/907_notnull.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "SET lock_timeout = '5s';"
    echo "ALTER TABLE test_table ALTER payload SET NOT NULL;"
} >"$notnull_short"
assert_rejected "GOV-SQL-001" "$notnull_short"

blocking_reindex="$test_root/automatiza-go/scripts/db/migrations/908_reindex.up.sql"
{
    echo "-- automatiza:migration-mode=transactional"
    echo "REINDEX INDEX idx_blocking_reindex;"
} >"$blocking_reindex"
assert_rejected "GOV-SQL-001" "$blocking_reindex"

concurrent_reindex="$test_root/automatiza-go/scripts/db/migrations/909_reindex_conc.up.sql"
{
    echo "-- automatiza:migration-mode=nontransactional"
    echo "REINDEX INDEX CONCURRENTLY idx_safe_reindex;"
} >"$concurrent_reindex"
write_verifier "$concurrent_reindex"
"$validator" "$concurrent_reindex" >/dev/null

missing_mode="$test_root/automatiza-go/scripts/db/migrations/910_missing_mode.up.sql"
echo "SELECT 1;" >"$missing_mode"
assert_rejected "GOV-SQL-005" "$missing_mode"

missing_verifier="$test_root/automatiza-go/scripts/db/migrations/911_missing_verifier.up.sql"
{
    echo "-- automatiza:migration-mode=nontransactional"
    echo "SELECT 1;"
} >"$missing_verifier"
assert_rejected "GOV-SQL-006" "$missing_verifier"

blocked_product_index="$test_root/automatiza-go/scripts/db/migrations/912_product_url.up.sql"
{
    echo "-- automatiza:migration-mode=nontransactional"
    echo "SET lock_timeout = '5s';"
    echo "CREATE INDEX CONCURRENTLY idx_dispatch_logs_product_url_trgm ON dispatch_logs USING gin(product_url gin_trgm_ops);"
} >"$blocked_product_index"
write_verifier "$blocked_product_index"
assert_rejected "GOV-SQL-007" "$blocked_product_index"

template_copy="$test_root/docs/change-records/TEMPLATE.md"
echo "# Placeholder-only template must be skipped" >"$template_copy"
"$validator" "$template_copy" >/dev/null

invalid_record="$test_root/docs/change-records/invalid.md"
{
    echo "# Incomplete"
    echo "## Metadata"
    echo "- Owner: test"
    echo "- Risk axis: governance"
} >"$invalid_record"
assert_rejected "GOV-CR-001" "$invalid_record"

invalid_axis="$test_root/docs/change-records/invalid-axis.md"
write_valid_record "$invalid_axis"
sed -i.bak 's/- Risk axis: governance/- Risk axis: functional, dependency/' "$invalid_axis"
rm "$invalid_axis.bak"
assert_rejected "GOV-CR-002" "$invalid_axis"

placeholder_axis="$test_root/docs/change-records/placeholder-axis.md"
write_valid_record "$placeholder_axis"
sed -i.bak 's/- Risk axis: governance/- Risk axis: <functional|schema|data-backfill|data-cleanup|dependency|infrastructure|security|governance>/' "$placeholder_axis"
rm "$placeholder_axis.bak"
assert_rejected "GOV-CR-002" "$placeholder_axis"

echo "Governance validator self-test passed."
