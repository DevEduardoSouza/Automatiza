#!/bin/bash

# Automatiza Health Check Script
# Usage: ./health-check.sh [--verbose]

VERBOSE="${1:-}"
EXIT_CODE=0

echo "🔍 Automatiza Health Check"
echo "=========================="
echo ""

# Check systemd services
check_service() {
    local service=$1
    if systemctl is-active --quiet "$service"; then
        echo "✅ $service: running"
        if [ "$VERBOSE" = "--verbose" ]; then
            systemctl status "$service" --no-pager -l | head -10
            echo ""
        fi
    else
        echo "❌ $service: stopped"
        EXIT_CODE=1
    fi
}

echo "📊 Systemd Services:"
check_service "automatiza-server"
check_service "automatiza-worker"
check_service "automatiza-frontend"
echo ""

# Check Docker containers
echo "🐳 Docker Containers:"
for container in automatiza-postgres automatiza-redis automatiza-waha; do
    if docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
        echo "✅ $container: running"
    else
        echo "❌ $container: not running"
        EXIT_CODE=1
    fi
done
echo ""

# Check endpoints
echo "🌐 Endpoints:"

# Backend health
if curl -sf http://127.0.0.1:3000/api/v1/health > /dev/null 2>&1; then
    echo "✅ Backend (localhost:3000): healthy"
else
    echo "❌ Backend (localhost:3000): unreachable"
    EXIT_CODE=1
fi

# Frontend
if curl -sf http://127.0.0.1:8000 > /dev/null 2>&1; then
    echo "✅ Frontend (localhost:8000): accessible"
else
    echo "❌ Frontend (localhost:8000): unreachable"
    EXIT_CODE=1
fi

# PostgreSQL
if docker exec automatiza-postgres pg_isready -U automatiza > /dev/null 2>&1; then
    echo "✅ PostgreSQL: ready"
else
    echo "❌ PostgreSQL: not ready"
    EXIT_CODE=1
fi

# Redis
if docker exec automatiza-redis redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis: ready"
else
    echo "❌ Redis: not ready"
    EXIT_CODE=1
fi

# WAHA
if curl -sf http://127.0.0.1:3001/api/health > /dev/null 2>&1; then
    echo "✅ WAHA (localhost:3001): healthy"
else
    echo "⚠️ WAHA (localhost:3001): unreachable (may need session)"
fi

echo ""

# External access (if domain is configured)
echo "🔐 External Access:"
if curl -sf https://astecwms.com.br/api/v1/health > /dev/null 2>&1; then
    echo "✅ HTTPS (astecwms.com.br): accessible"
else
    echo "⚠️ HTTPS (astecwms.com.br): not accessible (DNS/SSL may not be configured)"
fi

echo ""

# SSL Certificate
echo "📜 SSL Certificate:"
if [ -f /etc/letsencrypt/live/astecwms.com.br/fullchain.pem ]; then
    EXPIRY=$(openssl x509 -enddate -noout -in /etc/letsencrypt/live/astecwms.com.br/fullchain.pem | cut -d= -f2)
    echo "✅ Certificate expires: $EXPIRY"
else
    echo "⚠️ SSL certificate not found"
fi

echo ""

# Disk usage
echo "💾 Disk Usage:"
df -h /opt/automatiza | tail -1 | awk '{print "   Used: "$3" / "$2" ("$5")"}'

# Memory usage
echo ""
echo "🧠 Memory Usage:"
free -h | grep Mem | awk '{print "   Used: "$3" / "$2}'

echo ""
echo "=========================="
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All checks passed!"
else
    echo "❌ Some checks failed!"
fi

exit $EXIT_CODE
