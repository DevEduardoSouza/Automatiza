#!/bin/bash
set -e

# Automatiza Rollback Script
# Usage: ./rollback.sh

DEPLOY_DIR="/opt/automatiza"
BACKUP_FILE="$DEPLOY_DIR/backend/server.bak"

echo "🔙 Starting rollback..."

if [ ! -f "$BACKUP_FILE" ]; then
    echo "❌ No backup found at $BACKUP_FILE"
    echo "   Cannot rollback."
    exit 1
fi

# Stop services
echo "🔄 Stopping services..."
systemctl stop automatiza-server automatiza-worker || true

# Restore backup
echo "💾 Restoring backup..."
cp "$BACKUP_FILE" "$DEPLOY_DIR/backend/server"

# Start services
echo "🚀 Starting services..."
systemctl start automatiza-server
sleep 5
systemctl start automatiza-worker

# Health check
echo "⏳ Running health check..."
sleep 5
if curl -sf http://127.0.0.1:3000/api/v1/health > /dev/null; then
    echo "✅ Rollback successful! Backend is healthy."
else
    echo "❌ Rollback may have failed. Check logs."
    journalctl -u automatiza-server -n 50 --no-pager
    exit 1
fi

echo ""
echo "📊 Service status:"
systemctl status automatiza-server --no-pager -l | head -5
