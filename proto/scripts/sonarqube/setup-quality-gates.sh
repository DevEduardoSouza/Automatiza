#!/bin/bash
# =============================================================================
# SonarQube Quality Gates Setup Script
# =============================================================================
# Creates aggressive quality gates for React 19 + Golang + gRPC/Proto stack
# Run this ONCE after SonarQube is up and running
# =============================================================================

set -euo pipefail

# Configuration
SONAR_URL="${SONAR_URL:-http://localhost:9000/sonarqube}"
SONAR_USER="${SONAR_USER:-admin}"
SONAR_PASSWORD="${SONAR_PASSWORD:-admin}"
QUALITY_GATE_NAME="Automatiza Aggressive Gate"

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

# Wait for SonarQube to be ready
wait_for_sonarqube() {
    log_info "Waiting for SonarQube to be ready..."
    local max_attempts=60
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl -s "${SONAR_URL}/api/system/status" | grep -q '"status":"UP"'; then
            log_success "SonarQube is ready!"
            return 0
        fi
        echo -n "."
        sleep 5
        ((attempt++))
    done

    log_error "SonarQube did not become ready in time"
    exit 1
}

# API helper function
sonar_api() {
    local method="$1"
    local endpoint="$2"
    shift 2
    local data="$*"

    if [ "$method" = "GET" ]; then
        curl -s -u "${SONAR_USER}:${SONAR_PASSWORD}" "${SONAR_URL}${endpoint}"
    else
        curl -s -X POST -u "${SONAR_USER}:${SONAR_PASSWORD}" "${SONAR_URL}${endpoint}" -d "$data"
    fi
}

# Check if quality gate exists
quality_gate_exists() {
    local name="$1"
    local result
    result=$(sonar_api GET "/api/qualitygates/list")
    echo "$result" | grep -q "\"name\":\"${name}\""
}

# Create quality gate
create_quality_gate() {
    log_info "Creating quality gate: ${QUALITY_GATE_NAME}"

    # Delete if exists (for idempotency)
    if quality_gate_exists "${QUALITY_GATE_NAME}"; then
        log_warn "Quality gate already exists, recreating..."
        local gate_id
        gate_id=$(sonar_api GET "/api/qualitygates/list" | \
            python3 -c "import sys,json; gates=json.load(sys.stdin)['qualitygates']; print([g['id'] for g in gates if g['name']=='${QUALITY_GATE_NAME}'][0])" 2>/dev/null || echo "")

        if [ -n "$gate_id" ]; then
            sonar_api POST "/api/qualitygates/destroy" "id=${gate_id}" >/dev/null 2>&1 || true
        fi
    fi

    # Create new quality gate
    local result
    result=$(sonar_api POST "/api/qualitygates/create" "name=${QUALITY_GATE_NAME}")
    local gate_id
    gate_id=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)

    if [ -z "$gate_id" ]; then
        log_error "Failed to create quality gate"
        echo "$result"
        exit 1
    fi

    log_success "Quality gate created with ID: ${gate_id}"
    echo "$gate_id"
}

# Add condition to quality gate
add_condition() {
    local gate_id="$1"
    local metric="$2"
    local op="$3"
    local error="$4"

    log_info "  Adding condition: ${metric} ${op} ${error}"
    sonar_api POST "/api/qualitygates/create_condition" \
        "gateId=${gate_id}&metric=${metric}&op=${op}&error=${error}" >/dev/null
}

# Configure aggressive quality gate conditions
configure_conditions() {
    local gate_id="$1"

    log_info "Configuring aggressive quality gate conditions..."

    # ===========================================================================
    # NEW CODE CONDITIONS (Focus Period)
    # ===========================================================================
    log_info "Setting NEW CODE conditions..."

    # Coverage on New Code >= 80%
    add_condition "$gate_id" "new_coverage" "LT" "80"

    # Duplicated Lines on New Code <= 3%
    add_condition "$gate_id" "new_duplicated_lines_density" "GT" "3"

    # Maintainability Rating on New Code = A
    add_condition "$gate_id" "new_maintainability_rating" "GT" "1"

    # Reliability Rating on New Code = A
    add_condition "$gate_id" "new_reliability_rating" "GT" "1"

    # Security Rating on New Code = A
    add_condition "$gate_id" "new_security_rating" "GT" "1"

    # Security Hotspots Reviewed = 100%
    add_condition "$gate_id" "new_security_hotspots_reviewed" "LT" "100"

    # ===========================================================================
    # OVERALL CODE CONDITIONS
    # ===========================================================================
    log_info "Setting OVERALL CODE conditions..."

    # Coverage >= 70%
    add_condition "$gate_id" "coverage" "LT" "70"

    # Duplicated Lines <= 5%
    add_condition "$gate_id" "duplicated_lines_density" "GT" "5"

    # Maintainability Rating = A (Technical Debt Ratio < 5%)
    add_condition "$gate_id" "sqale_rating" "GT" "1"

    # Reliability Rating = A (0 bugs)
    add_condition "$gate_id" "reliability_rating" "GT" "1"

    # Security Rating = A (0 vulnerabilities)
    add_condition "$gate_id" "security_rating" "GT" "1"

    # ===========================================================================
    # BLOCKER CONDITIONS (Zero Tolerance)
    # ===========================================================================
    log_info "Setting ZERO TOLERANCE conditions..."

    # No Blocker Issues
    add_condition "$gate_id" "blocker_violations" "GT" "0"

    # No Critical Issues (NEW CODE)
    add_condition "$gate_id" "new_critical_violations" "GT" "0"

    # No Major Bugs
    add_condition "$gate_id" "major_violations" "GT" "5"

    log_success "All conditions configured!"
}

# Set as default quality gate
set_default_gate() {
    local gate_id="$1"
    log_info "Setting as default quality gate..."
    sonar_api POST "/api/qualitygates/set_as_default" "id=${gate_id}" >/dev/null
    log_success "Quality gate set as default!"
}

# Create projects
create_projects() {
    log_info "Creating SonarQube projects..."

    # Client (React)
    if ! sonar_api GET "/api/projects/search?projects=automatiza-client" | grep -q "automatiza-client"; then
        sonar_api POST "/api/projects/create" "project=automatiza-client&name=Automatiza%20-%20React%20Frontend" >/dev/null
        log_success "Created project: automatiza-client"
    else
        log_warn "Project automatiza-client already exists"
    fi

    # Backend (Go)
    if ! sonar_api GET "/api/projects/search?projects=automatiza-go" | grep -q "automatiza-go"; then
        sonar_api POST "/api/projects/create" "project=automatiza-go&name=Automatiza%20-%20Go%20Backend" >/dev/null
        log_success "Created project: automatiza-go"
    else
        log_warn "Project automatiza-go already exists"
    fi
}

# Assign quality gate to projects
assign_gate_to_projects() {
    local gate_id="$1"
    log_info "Assigning quality gate to projects..."

    sonar_api POST "/api/qualitygates/select" "gateId=${gate_id}&projectKey=automatiza-client" >/dev/null
    log_success "Assigned to automatiza-client"

    sonar_api POST "/api/qualitygates/select" "gateId=${gate_id}&projectKey=automatiza-go" >/dev/null
    log_success "Assigned to automatiza-go"
}

# Generate authentication token
generate_token() {
    local token_name="automatiza-scanner"
    log_info "Generating authentication token..."

    # Revoke existing token if exists
    sonar_api POST "/api/user_tokens/revoke" "name=${token_name}" >/dev/null 2>&1 || true

    # Generate new token
    local result
    result=$(sonar_api POST "/api/user_tokens/generate" "name=${token_name}")
    local token
    token=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token', ''))" 2>/dev/null)

    if [ -n "$token" ]; then
        log_success "Token generated!"
        echo ""
        echo -e "${GREEN}=============================================${NC}"
        echo -e "${GREEN}SAVE THIS TOKEN - IT WON'T BE SHOWN AGAIN!${NC}"
        echo -e "${GREEN}=============================================${NC}"
        echo -e "SONAR_TOKEN=${token}"
        echo -e "${GREEN}=============================================${NC}"
        echo ""
        echo "Add to your environment or .env file:"
        echo "export SONAR_TOKEN=${token}"
    else
        log_warn "Could not generate token. You may need to create one manually."
    fi
}

# Print summary
print_summary() {
    echo ""
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}SonarQube Quality Gates Setup Complete!${NC}"
    echo -e "${GREEN}=============================================${NC}"
    echo ""
    echo "Quality Gate: ${QUALITY_GATE_NAME}"
    echo ""
    echo "Conditions configured:"
    echo "  NEW CODE:"
    echo "    - Coverage >= 80%"
    echo "    - Duplicated Lines <= 3%"
    echo "    - Maintainability Rating = A"
    echo "    - Reliability Rating = A"
    echo "    - Security Rating = A"
    echo "    - Security Hotspots Reviewed = 100%"
    echo ""
    echo "  OVERALL CODE:"
    echo "    - Coverage >= 70%"
    echo "    - Duplicated Lines <= 5%"
    echo "    - Maintainability Rating = A"
    echo "    - Reliability Rating = A"
    echo "    - Security Rating = A"
    echo ""
    echo "  ZERO TOLERANCE:"
    echo "    - Blocker Issues = 0"
    echo "    - Critical Issues (new code) = 0"
    echo "    - Major Issues <= 5"
    echo ""
    echo "Projects:"
    echo "  - automatiza-client (React Frontend)"
    echo "  - automatiza-go (Go Backend)"
    echo ""
    echo "Access SonarQube at: ${SONAR_URL}"
    echo ""
}

# Main
main() {
    echo ""
    echo -e "${CYAN}=============================================${NC}"
    echo -e "${CYAN}SonarQube Quality Gates Setup${NC}"
    echo -e "${CYAN}Stack: React 19 + Golang + gRPC/Proto${NC}"
    echo -e "${CYAN}=============================================${NC}"
    echo ""

    wait_for_sonarqube

    local gate_id
    gate_id=$(create_quality_gate)

    configure_conditions "$gate_id"
    set_default_gate "$gate_id"
    create_projects
    assign_gate_to_projects "$gate_id"
    generate_token
    print_summary
}

main "$@"
