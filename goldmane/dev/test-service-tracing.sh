#!/bin/bash

echo "🔍 Testing Service Flow Tracing in Goldmane"
echo "==========================================="

# Check if services are running
echo "📊 Checking service status..."
docker ps --format "table {{.Names}}\t{{.Status}}" | grep goldmane

echo ""
echo "🔗 Testing connectivity..."

# Test echo server
echo "Testing echo server..."
curl -s -X POST http://localhost:3000/test -H "Content-Type: application/json" -d '{"test": "connectivity"}' | jq -r '.http.method + " " + .http.originalUrl' || echo "Echo server not responsive"

# Test OpenTelemetry collector health
echo "Testing OpenTelemetry collector..."
curl -s http://localhost:13133/ | jq -r '.status' || echo "OTel collector not responsive"

# Test Jaeger health  
echo "Testing Jaeger..."
curl -s http://localhost:16686/api/services | jq -r '.data | length' || echo "Jaeger not responsive"

echo ""
echo "📈 To verify service flow traces:"
echo "1. Open Jaeger UI: http://localhost:16686"
echo "2. Look for services named like: client-X.namespace-Y → server-Z.namespace-W"
echo "3. Check for spans with network flow attributes"
echo ""
echo "🔄 If no traces appear, restart flowgen:"
echo "   docker-compose restart flowgen"
