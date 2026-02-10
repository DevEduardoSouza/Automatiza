#!/bin/bash
# =============================================================================
# SonarQube Analysis Runner
# =============================================================================
# Runs SonarQube analysis for React client and Go backend
# Requires sonar-scanner CLI to be installed
# =============================================================================

set -euo pipefail

# Configuration
SONAR_URL="${SONAR_URL:-http://localhost:9000/sonarqube}"
SONAR_TOKEN="${SONAR_TOKEN:-}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

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
    log_info "Checking requirements..."

    if [ -z "$SONAR_TOKEN" ]; then
        log_error "SONAR_TOKEN environment variable is required"
        log_info "Run: export SONAR_TOKEN=<your-token>"
        log_info "Generate token at: ${SONAR_URL}/account/security"
        exit 1
    fi

    if ! command -v sonar-scanner &>/dev/null; then
        log_error "sonar-scanner CLI not found"
        log_info "Install with: brew install sonar-scanner"
        log_info "Or download from: https://docs.sonarsource.com/sonarqube/latest/analyzing-source-code/scanners/sonarscanner/"
        exit 1
    fi

    log_success "All requirements met"
}

# Run client analysis
analyze_client() {
    log_info "Analyzing React client..."

    cd "${PROJECT_ROOT}/client"

    # Generate coverage report if not exists
    if [ ! -f "coverage/lcov.info" ]; then
        log_info "Generating coverage report..."
        npm run test:coverage 2>/dev/null || log_warn "Coverage generation failed, continuing without coverage"
    fi

    # Run SonarQube scanner
    sonar-scanner \
        -Dsonar.host.url="${SONAR_URL}" \
        -Dsonar.token="${SONAR_TOKEN}" \
        -Dsonar.projectBaseDir="${PROJECT_ROOT}/client"

    log_success "Client analysis complete!"
}

# Run Go backend analysis
analyze_go() {
    log_info "Analyzing Go backend..."

    cd "${PROJECT_ROOT}/automatiza-go"

    # Generate coverage report
    log_info "Running tests with coverage..."
    go test -coverprofile=coverage.out -json ./... > test-report.json 2>/dev/null || log_warn "Some tests failed"

    # Generate golangci-lint report for SonarQube
    log_info "Running golangci-lint..."
    if command -v golangci-lint &>/dev/null; then
        golangci-lint run --out-format checkstyle ./... > golangci-lint-report.xml 2>/dev/null || true
    else
        log_warn "golangci-lint not found, skipping lint report"
    fi

    # Run SonarQube scanner
    sonar-scanner \
        -Dsonar.host.url="${SONAR_URL}" \
        -Dsonar.token="${SONAR_TOKEN}" \
        -Dsonar.projectBaseDir="${PROJECT_ROOT}/automatiza-go"

    log_success "Go backend analysis complete!"
}

# Show help
show_help() {
    echo "Usage: $0 [OPTIONS] [TARGET]"
    echo ""
    echo "Targets:"
    echo "  all       Analyze both client and backend (default)"
    echo "  client    Analyze React client only"
    echo "  go        Analyze Go backend only"
    echo ""
    echo "Options:"
    echo "  -h, --help    Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  SONAR_URL     SonarQube server URL (default: http://localhost:9000/sonarqube)"
    echo "  SONAR_TOKEN   Authentication token (required)"
    echo ""
}

# Main
main() {
    local target="${1:-all}"

    case "$target" in
        -h|--help)
            show_help
            exit 0
            ;;
        all)
            check_requirements
            analyze_client
            analyze_go
            ;;
        client)
            check_requirements
            analyze_client
            ;;
        go)
            check_requirements
            analyze_go
            ;;
        *)
            log_error "Unknown target: $target"
            show_help
            exit 1
            ;;
    esac

    echo ""
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}Analysis Complete!${NC}"
    echo -e "${GREEN}=============================================${NC}"
    echo ""
    echo "View results at: ${SONAR_URL}/projects"
    echo ""
}

main "$@"
