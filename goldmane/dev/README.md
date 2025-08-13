# Goldmane Development Stack

This Docker Compose stack provides a complete development environment for Goldmane with OpenTelemetry tracing and flow generation.

## Services

### 1. **Goldmane** (`goldmane:7443`)
- Main flow aggregation service
- Receives flows from flowgen
- Pushes aggregated data to echo-server
- Sends traces to OpenTelemetry collector
- Health check: http://localhost:8080/healthz
- Prometheus metrics: http://localhost:9090/metrics

### 2. **Echo Server** (`echo-server:3000`)
- HTTP echo server that receives goldmane's pushed flow data
- Useful for debugging what goldmane is sending
- Web UI: http://localhost:3000

### 3. **OpenTelemetry Collector** (`otel-collector:4317`)
- Receives traces from goldmane
- Exports to Jaeger for visualization
- Exports to file for analysis
- Health check: http://localhost:13133
- Prometheus metrics: http://localhost:8888/metrics
- ZPages debug: http://localhost:55679/debug/tracez

### 4. **Jaeger** (`jaeger:16686`)
- Trace visualization UI
- Web UI: http://localhost:16686
- Search and analyze goldmane traces

### 5. **Flow Generator** (`flowgen`)
- Generates synthetic flow data
- Sends flows to goldmane
- Simulates multiple nodes

## Usage

### Start the Stack

```bash
cd /Users/seth/code/calico/goldmane/dev
docker-compose up -d
```

### Build and Start (if images need building)

```bash
cd /Users/seth/code/calico/goldmane/dev
docker-compose up --build -d
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f goldmane
docker-compose logs -f flowgen
docker-compose logs -f otel-collector
```

### Stop the Stack

```bash
docker-compose down
```

### Clean Up (remove volumes)

```bash
docker-compose down -v
```

## Accessing Services

| Service | URL | Purpose |
|---------|-----|---------|
| Jaeger UI | http://localhost:16686 | View and analyze traces |
| Echo Server | http://localhost:3000 | See what goldmane pushes |
| Goldmane Health | http://localhost:8080 | Health status |
| Goldmane Metrics | http://localhost:9090/metrics | Prometheus metrics |
| OTel Collector Health | http://localhost:13133 | Collector health |
| OTel Collector Metrics | http://localhost:8888/metrics | Collector metrics |
| OTel ZPages | http://localhost:55679/debug/tracez | Trace debugging |

## Service Dependencies

```
flowgen → goldmane → echo-server
       ↘         ↘
         ↘       otel-collector → jaeger
```

1. **echo-server** and **otel-collector** start first
2. **jaeger** starts (for otel-collector dependency)
3. **goldmane** starts after dependencies are healthy
4. **flowgen** starts after goldmane is healthy

## Configuration

### Goldmane Environment Variables

Key configuration in `docker-compose.yaml`:

- `PUSH_URL=http://echo-server/flows` - Where to push aggregated flows
- `OTLP_URL=otel-collector:4317` - OpenTelemetry collector endpoint
- `OTLP_INSECURE=true` - Use insecure connection (dev only)
- `OTLP_SAMPLING_RATE=1.0` - Trace 100% of requests (dev only)
- `AGGREGATION_WINDOW=15s` - Flow aggregation window
- `EMIT_AFTER_SECONDS=30` - How long to wait before emitting flows

### OpenTelemetry Collector

Configuration in `otel-collector-config.yaml`:

- Receives OTLP traces on port 4317 (gRPC) and 4318 (HTTP)
- Exports to Jaeger for visualization
- Exports to file (`/tmp/otel-data/traces.json`) for analysis
- Provides health check and debug endpoints

## Troubleshooting

### Check Service Health

```bash
# All services status
docker-compose ps

# Specific health checks
curl http://localhost:8080      # Goldmane health
curl http://localhost:13133     # OTel Collector health
curl http://localhost:3000      # Echo server
```

### View Trace Data

1. **Jaeger UI**: http://localhost:16686
   - Service: "goldmane"
   - Look for operations like "goldmane.flow.receive", "goldmane.flow.aggregate", "goldmane.flow.emit"

2. **Raw trace files**: 
   ```bash
   docker-compose exec otel-collector cat /tmp/otel-data/traces.json
   ```

3. **OTel Collector ZPages**: http://localhost:55679/debug/tracez

### Debug Flow Data

1. **Echo Server**: http://localhost:3000
   - Shows all HTTP requests goldmane makes
   - Check the request body to see aggregated flow data

2. **Goldmane Logs**:
   ```bash
   docker-compose logs -f goldmane
   ```

### Common Issues

1. **Services not starting**: Check dependencies and health checks
2. **No traces**: Verify OTLP_URL and collector configuration
3. **No flows**: Check flowgen logs and goldmane connectivity
4. **Build failures**: Ensure you're in the correct directory and have Docker BuildKit enabled

## Development

To modify configurations:

1. Edit `docker-compose.yaml` for service configuration
2. Edit `otel-collector-config.yaml` for OpenTelemetry settings
3. Restart affected services: `docker-compose restart <service>`

For code changes, rebuild the affected service:

```bash
docker-compose up --build goldmane
docker-compose up --build flowgen
```
