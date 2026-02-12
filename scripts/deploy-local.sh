#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Deploy Local - Build Docker images locally, push to GHCR, deploy via SSH
# Usage: ./scripts/deploy-local.sh
# =============================================================================

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DOMAIN="astecwms.com.br"
IMAGE_PREFIX="ghcr.io/bruno-vilefort-tech-consulting/carlos-automatisa"
VERSION="$(date +%Y%m%d-%H%M%S)-local"

VPS_HOST="${VPS_HOST:-astecwms.com.br}"
VPS_USER="${VPS_USER:-root}"

# --- SSH auth ---
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=30"
if [ -n "${VPS_SSH_KEY:-}" ]; then
    SSH_CMD="ssh ${SSH_OPTS} -i ${VPS_SSH_KEY}"
    SCP_CMD="scp ${SSH_OPTS} -i ${VPS_SSH_KEY}"
elif [ -f "$HOME/.ssh/deploy_key" ]; then
    SSH_CMD="ssh ${SSH_OPTS} -i $HOME/.ssh/deploy_key"
    SCP_CMD="scp ${SSH_OPTS} -i $HOME/.ssh/deploy_key"
else
    echo "ERROR: No SSH key found. Set VPS_SSH_KEY or place key at ~/.ssh/deploy_key"
    exit 1
fi

echo "=== Deploy Local para ${VPS_HOST} ==="
echo "    Version: ${VERSION}"
echo ""

# --- Build & push backend image ---
echo "[1/5] Building backend image..."
docker build \
    --platform linux/amd64 \
    --build-arg VERSION="${VERSION}" \
    -t "${IMAGE_PREFIX}/backend:${VERSION}" \
    -t "${IMAGE_PREFIX}/backend:latest" \
    "${PROJECT_ROOT}/automatiza-go"

echo "[2/5] Building frontend image..."
docker build \
    --platform linux/amd64 \
    -t "${IMAGE_PREFIX}/frontend:${VERSION}" \
    -t "${IMAGE_PREFIX}/frontend:latest" \
    "${PROJECT_ROOT}/client"

echo "[3/5] Pushing images to GHCR..."
docker push "${IMAGE_PREFIX}/backend:${VERSION}"
docker push "${IMAGE_PREFIX}/backend:latest"
docker push "${IMAGE_PREFIX}/frontend:${VERSION}"
docker push "${IMAGE_PREFIX}/frontend:latest"

echo "[4/5] Copying docker-compose to VPS..."
$SCP_CMD "${PROJECT_ROOT}/docker/docker-compose.prod.yml" \
    "${VPS_USER}@${VPS_HOST}:/opt/automatiza/docker/docker-compose.yml"

echo "[5/5] Deploying on VPS..."
$SSH_CMD "${VPS_USER}@${VPS_HOST}" << DEPLOY
    set -e
    cd /opt/automatiza/docker

    export BACKEND_IMAGE="${IMAGE_PREFIX}/backend:${VERSION}"
    export FRONTEND_IMAGE="${IMAGE_PREFIX}/frontend:${VERSION}"

    echo "  Pulling images..."
    docker compose pull

    echo "  Running migrations..."
    docker compose run --rm migrate

    echo "  Starting services..."
    docker compose up -d server worker frontend

    echo "  Health check..."
    sleep 5
    if curl -sf http://127.0.0.1:3000/api/v1/health > /dev/null; then
        echo "  Backend is healthy!"
    else
        echo "  WARNING: Health check failed"
        docker compose logs --tail=20 server
    fi

    docker compose ps
DEPLOY

echo ""
echo "=== Deploy complete! ==="
echo "    https://${DOMAIN}"
