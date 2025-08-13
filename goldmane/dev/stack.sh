#!/bin/bash

# Goldmane Development Stack Manager

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

function usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start     Start the development stack"
    echo "  stop      Stop the development stack"
    echo "  restart   Restart the development stack"
    echo "  build     Build and start the stack"
    echo "  logs      Follow logs from all services"
    echo "  status    Show status of all services"
    echo "  clean     Stop and remove all containers and volumes"
    echo "  traces    Open Jaeger UI in browser"
    echo "  echo      Open Echo Server UI in browser"
    echo "  health    Check health of all services"
    echo ""
}

function start_stack() {
    echo "🚀 Starting Goldmane development stack..."
    docker-compose up -d
    echo "✅ Stack started!"
    echo ""
    show_urls
}

function stop_stack() {
    echo "🛑 Stopping Goldmane development stack..."
    docker-compose down
    echo "✅ Stack stopped!"
}

function restart_stack() {
    echo "🔄 Restarting Goldmane development stack..."
    docker-compose restart
    echo "✅ Stack restarted!"
}

function build_stack() {
    echo "🔨 Building and starting Goldmane development stack..."
    docker-compose up --build -d
    echo "✅ Stack built and started!"
    echo ""
    show_urls
}

function show_logs() {
    echo "📋 Showing logs from all services (Ctrl+C to exit)..."
    docker-compose logs -f
}

function show_status() {
    echo "📊 Service Status:"
    docker-compose ps
}

function clean_stack() {
    echo "🧹 Cleaning up Goldmane development stack..."
    docker-compose down -v --remove-orphans
    echo "✅ Stack cleaned!"
}

function open_traces() {
    echo "🔍 Opening Jaeger UI..."
    if command -v open >/dev/null 2>&1; then
        open http://localhost:16686
    elif command -v xdg-open >/dev/null 2>&1; then
        xdg-open http://localhost:16686
    else
        echo "Please open http://localhost:16686 in your browser"
    fi
}

function open_echo() {
    echo "📡 Opening Echo Server UI..."
    if command -v open >/dev/null 2>&1; then
        open http://localhost:3000
    elif command -v xdg-open >/dev/null 2>&1; then
        xdg-open http://localhost:3000
    else
        echo "Please open http://localhost:3000 in your browser"
    fi
}

function check_health() {
    echo "🏥 Checking service health..."
    echo ""
    
    echo "Goldmane Health:"
    if curl -s http://localhost:8080 >/dev/null 2>&1; then
        echo "  ✅ Goldmane: Healthy"
    else
        echo "  ❌ Goldmane: Unhealthy"
    fi
    
    echo "OpenTelemetry Collector Health:"
    if curl -s http://localhost:13133 >/dev/null 2>&1; then
        echo "  ✅ OTel Collector: Healthy"
    else
        echo "  ❌ OTel Collector: Unhealthy"
    fi
    
    echo "Echo Server Health:"
    if curl -s http://localhost:3000 >/dev/null 2>&1; then
        echo "  ✅ Echo Server: Healthy"
    else
        echo "  ❌ Echo Server: Unhealthy"
    fi
    
    echo "Jaeger Health:"
    if curl -s http://localhost:16686 >/dev/null 2>&1; then
        echo "  ✅ Jaeger: Healthy"
    else
        echo "  ❌ Jaeger: Unhealthy"
    fi
}

function show_urls() {
    echo "🌐 Service URLs:"
    echo "  Jaeger UI:           http://localhost:16686"
    echo "  Echo Server:         http://localhost:3000"
    echo "  Goldmane Health:     http://localhost:8080"
    echo "  Goldmane Metrics:    http://localhost:9090/metrics"
    echo "  OTel Health:         http://localhost:13133"
    echo "  OTel Metrics:        http://localhost:8888/metrics"
    echo "  OTel ZPages:         http://localhost:55679/debug/tracez"
    echo ""
}

# Main script logic
case "${1:-}" in
    start)
        start_stack
        ;;
    stop)
        stop_stack
        ;;
    restart)
        restart_stack
        ;;
    build)
        build_stack
        ;;
    logs)
        show_logs
        ;;
    status)
        show_status
        ;;
    clean)
        clean_stack
        ;;
    traces)
        open_traces
        ;;
    echo)
        open_echo
        ;;
    health)
        check_health
        ;;
    *)
        usage
        exit 1
        ;;
esac
