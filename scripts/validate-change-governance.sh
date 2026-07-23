#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 [--base <git-ref>] [file ...]"
}

failures=0
inspected=0
base_ref=""
files=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --base)
            [[ $# -ge 2 ]] || { usage >&2; exit 2; }
            base_ref="$2"
            shift 2
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        --*)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
        *)
            files+=("$1")
            shift
            ;;
    esac
done

if [[ ${#files[@]} -eq 0 ]]; then
    if [[ -n "$base_ref" ]]; then
        git rev-parse --verify "${base_ref}^{commit}" >/dev/null
        while IFS= read -r file; do
            [[ -n "$file" ]] && files+=("$file")
        done < <(git diff --name-only --diff-filter=ACMR "${base_ref}...HEAD")
    else
        while IFS= read -r file; do
            [[ -n "$file" ]] && files+=("$file")
        done < <(
            {
                git diff --name-only --diff-filter=ACMR HEAD
                git ls-files --others --exclude-standard
            } | awk 'NF && !seen[$0]++'
        )
    fi
fi

report() {
    local rule="$1"
    local file="$2"
    local message="$3"
    echo "${rule}: ${file}: ${message}" >&2
    failures=$((failures + 1))
}

require_literal() {
    local file="$1"
    local literal="$2"
    if ! grep -Fq -- "$literal" "$file"; then
        report "GOV-CR-001" "$file" "missing required field: ${literal}"
    fi
}

validate_change_record() {
    local file="$1"
    local required=(
        "## Metadata"
        "- Owner:"
        "- Risk axis:"
        "## Hypothesis"
        "## Scope"
        "### Files"
        "### Tables"
        "## Baseline"
        "## Expand / migrate / observe / contract"
        "## Canary"
        "## Abort thresholds"
        "## Rollout"
        "## Rollback"
        "## Evidence"
        "### Commands and results"
        "### Approvals"
        "## Residual risks and blockers"
    )
    local field

    for field in "${required[@]}"; do
        require_literal "$file" "$field"
    done

    if ! grep -Eq '^- Risk axis: (functional|schema|data-backfill|data-cleanup|dependency|infrastructure|security|governance)$' "$file"; then
        report "GOV-CR-002" "$file" "risk axis must be exactly one supported value"
    fi
}

validate_up_migration() {
    local file="$1"
    local mode
    local verifier="${file%.up.sql}.verify.sql"
    local statements
    local statement

    mode="$(
        grep -m1 -E '^-- automatiza:migration-mode=(transactional|nontransactional)[[:space:]]*$' "$file" \
            | sed 's/^-- automatiza:migration-mode=//' \
            | tr -d '[:space:]' \
            || true
    )"
    if [[ "$mode" != "transactional" && "$mode" != "nontransactional" ]]; then
        report "GOV-SQL-005" "$file" "new up migrations must declare transactional or nontransactional mode"
    fi

    statements="$(
        sed '/^[[:space:]]*--/d' "$file" \
            | tr '\n' ' ' \
            | sed 's/;/;\n/g' \
            | tr '[:upper:]' '[:lower:]'
    )"

    if grep -Eq 'drop[[:space:]]+column|drop[[:space:]]+table|(^|[;[:space:]])truncate[[:space:]]|alter[[:space:]]+table[^;]*alter[[:space:]]+[^;]*[[:space:]]type[[:space:]]|alter[[:space:]]+table[^;]*set[[:space:]]+not[[:space:]]+null|vacuum[[:space:]]+full|(^|[;[:space:]])cluster[[:space:]]' <<<"$statements"; then
        report "GOV-SQL-001" "$file" "destructive or blocking DDL is prohibited in a new up migration"
    fi

    while IFS= read -r statement; do
        [[ -n "$statement" ]] || continue
        if ! grep -Fq 'concurrently' <<<"$statement"; then
            report "GOV-SQL-001" "$file" "blocking REINDEX is prohibited; use REINDEX ... CONCURRENTLY"
        fi
    done < <(grep -E '(^|[;[:space:]])reindex[[:space:]]' <<<"$statements" || true)

    while IFS= read -r statement; do
        [[ -n "$statement" ]] || continue
        if ! grep -Eq 'create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]+concurrently[[:space:]]' <<<"$statement"; then
            report "GOV-SQL-002" "$file" "indexes on rollout migrations must use CREATE INDEX CONCURRENTLY"
        fi
    done < <(grep -E 'create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]' <<<"$statements" || true)

    if grep -Eq 'alter[[:space:]]+table|create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]' <<<"$statements" \
        && ! grep -Eq 'set([[:space:]]+local)?[[:space:]]+lock_timeout[[:space:]]*=' <<<"$statements"; then
        report "GOV-SQL-003" "$file" "online ALTER TABLE/CREATE INDEX requires an explicit short lock_timeout"
    fi

    if grep -Eq 'create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]+concurrently[[:space:]]' <<<"$statements" \
        && grep -Eq '(^|[;[:space:]])(begin|start[[:space:]]+transaction)[[:space:]]*;' <<<"$statements"; then
        report "GOV-SQL-004" "$file" "CREATE INDEX CONCURRENTLY must run outside a transaction wrapper"
    fi

    if grep -Eq '(^|[;[:space:]])(create|drop|reindex)[[:space:]][^;]*index[[:space:]]+concurrently' <<<"$statements" \
        && [[ "$mode" != "nontransactional" ]]; then
        report "GOV-SQL-005" "$file" "concurrent index DDL requires nontransactional mode"
    fi

    if [[ "$mode" == "nontransactional" ]]; then
        if [[ ! -f "$verifier" ]]; then
            report "GOV-SQL-006" "$file" "nontransactional migration requires companion verifier: ${verifier##*/}"
        elif ! grep -Eiq 'as[[:space:]]+migration_verified' "$verifier" \
            || ! grep -Fq '\gset' "$verifier"; then
            report "GOV-SQL-006" "$verifier" "verifier must set the migration_verified psql variable"
        fi
    fi

    if grep -Eq 'create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]+(concurrently[[:space:]]+)?(if[[:space:]]+not[[:space:]]+exists[[:space:]]+)?idx_dispatch_logs_product_url_trgm' <<<"$statements"; then
        report "GOV-SQL-007" "$file" "retired product_url trigram index is blocked from automatic migrations"
    fi
}

for file in "${files[@]}"; do
    [[ -f "$file" ]] || continue
    case "$file" in
        docs/change-records/TEMPLATE.md|*/docs/change-records/TEMPLATE.md)
            # The template carries placeholders and is not a change record.
            ;;
        docs/change-records/*.md|*/docs/change-records/*.md)
            inspected=$((inspected + 1))
            validate_change_record "$file"
            ;;
        automatiza-go/scripts/db/migrations/*.up.sql|*/automatiza-go/scripts/db/migrations/*.up.sql)
            inspected=$((inspected + 1))
            validate_up_migration "$file"
            ;;
    esac
done

if [[ "$failures" -gt 0 ]]; then
    echo "Governance validation failed with ${failures} finding(s)." >&2
    exit 1
fi

echo "Governance validation passed (${inspected} relevant changed file(s) inspected)."
