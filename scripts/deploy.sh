#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Deploy Script (runs on VPS)
# Usage: ./deploy.sh [TAG]
# =============================================================================

DEPLOY_DIR="/opt/automatiza/docker"
TAG="${1:-latest}"
IMAGE_PREFIX="ghcr.io/bruno-vilefort-tech-consulting/carlos-automatisa"

cd "$DEPLOY_DIR"

echo "=== Deploy: tag=${TAG} ==="

# Pull images
echo "[1/4] Pulling images..."
export BACKEND_IMAGE="${IMAGE_PREFIX}/backend:${TAG}"
export FRONTEND_IMAGE="${IMAGE_PREFIX}/frontend:${TAG}"
docker compose pull

# Run migrations
echo "[2/4] Running migrations..."
docker compose run --rm migrate

# Start/update services
echo "[3/4] Starting services..."
docker compose up -d server worker frontend

# Health check
echo "[4/4] Health check..."
sleep 5

if curl -sf http://127.0.0.1:3000/api/v1/health > /dev/null; then
    echo "Backend is healthy!"
else
    echo "WARNING: Backend health check failed"
    docker compose logs --tail=20 server
    exit 1
fi

echo ""
docker compose ps
echo ""
echo "=== Deploy complete! ==="
