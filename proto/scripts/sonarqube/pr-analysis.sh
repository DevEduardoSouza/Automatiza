#!/bin/bash
# =============================================================================
# SonarQube PR Analysis
# =============================================================================
# Runs SonarQube analysis for Pull Requests with branch comparison
# =============================================================================

set -euo pipefail

# Configuration
SONAR_URL="${SONAR_URL:-http://localhost:9000/sonarqube}"
SONAR_TOKEN="${SONAR_TOKEN:-}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# PR Configuration (set by CI or manually)
PR_KEY="${PR_KEY:-}"
PR_BRANCH="${PR_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
PR_BASE="${PR_BASE:-main}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check requirements
check_requirements() {
    if [ -z "$SONAR_TOKEN" ]; then
        log_error "SONAR_TOKEN is required"
        exit 1
    fi

    if ! command -v sonar-scanner &>/dev/null; then
        log_error "sonar-scanner CLI not found"
        exit 1
    fi
}

# Analyze client with PR decoration
analyze_client_pr() {
    log_info "Analyzing React client (PR mode)..."

    cd "${PROJECT_ROOT}/client"

    # Generate coverage
    npm run test:coverage 2>/dev/null || log_warn "Coverage generation failed"

    local pr_args=""
    if [ -n "$PR_KEY" ]; then
        pr_args="-Dsonar.pullrequest.key=${PR_KEY} -Dsonar.pullrequest.branch=${PR_BRANCH} -Dsonar.pullrequest.base=${PR_BASE}"
    else
        pr_args="-Dsonar.branch.name=${PR_BRANCH}"
    fi

    sonar-scanner \
        -Dsonar.host.url="${SONAR_URL}" \
        -Dsonar.token="${SONAR_TOKEN}" \
        -Dsonar.projectBaseDir="${PROJECT_ROOT}/client" \
        ${pr_args}

    log_success "Client PR analysis complete!"
}

# Analyze Go with PR decoration
analyze_go_pr() {
    log_info "Analyzing Go backend (PR mode)..."

    cd "${PROJECT_ROOT}/automatiza-go"

    # Generate reports
    go test -coverprofile=coverage.out -json ./... > test-report.json 2>/dev/null || log_warn "Some tests failed"

    if command -v golangci-lint &>/dev/null; then
        golangci-lint run --out-format checkstyle ./... > golangci-lint-report.xml 2>/dev/null || true
    fi

    local pr_args=""
    if [ -n "$PR_KEY" ]; then
        pr_args="-Dsonar.pullrequest.key=${PR_KEY} -Dsonar.pullrequest.branch=${PR_BRANCH} -Dsonar.pullrequest.base=${PR_BASE}"
    else
        pr_args="-Dsonar.branch.name=${PR_BRANCH}"
    fi

    sonar-scanner \
        -Dsonar.host.url="${SONAR_URL}" \
        -Dsonar.token="${SONAR_TOKEN}" \
        -Dsonar.projectBaseDir="${PROJECT_ROOT}/automatiza-go" \
        ${pr_args}

    log_success "Go PR analysis complete!"
}

# Main
main() {
    echo ""
    echo -e "${CYAN}=============================================${NC}"
    echo -e "${CYAN}SonarQube PR Analysis${NC}"
    echo -e "${CYAN}Branch: ${PR_BRANCH} -> ${PR_BASE}${NC}"
    echo -e "${CYAN}=============================================${NC}"
    echo ""

    check_requirements
    analyze_client_pr
    analyze_go_pr

    echo ""
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}PR Analysis Complete!${NC}"
    echo -e "${GREEN}=============================================${NC}"
    echo ""
    echo "View results at: ${SONAR_URL}/projects"
    echo ""
}

main "$@"
