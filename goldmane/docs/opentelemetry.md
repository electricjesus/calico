# OpenTelemetry Integration for Goldmane

## Overview

Goldmane now includes OpenTelemetry integration to provide enhanced observability for flow aggregation and processing. This integration enables distributed tracing, improved metrics, and better service graph visibility.

## Configuration

OpenTelemetry support is configured through environment variables:

### Required Environment Variables

```bash
# Enable OpenTelemetry (set to "true" to enable)
OTEL_ENABLED=true

# OpenTelemetry collector endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317

# Service identification
OTEL_SERVICE_NAME=goldmane
OTEL_SERVICE_VERSION=1.0.0
OTEL_SERVICE_NAMESPACE=calico-system

# Kubernetes metadata
NODE_NAME=node-1
CLUSTER_NAME=my-cluster

# Sampling rate (0.0 to 1.0)
OTEL_SAMPLING_RATE=0.1
```

### Legacy Configuration (for backwards compatibility)

The following environment variables are also supported:

```bash
# OTLP endpoint (legacy)
OTLP_URL=localhost:4317
OTLP_INSECURE=true
OTLP_HEADERS=""
OTLP_TIMEOUT=10
OTLP_RETRY=3
OTLP_COMPRESSION=false
OTLP_LOG_LEVEL=info
```

## Features

### 1. Distributed Tracing

The integration provides distributed tracing across the entire flow processing pipeline:

- **Flow Reception**: Traces flows as they arrive from cluster nodes
- **Flow Aggregation**: Tracks flow processing and aggregation across time buckets
- **Flow Emission**: Traces the emission of aggregated flows to external endpoints
- **Flow Queries**: Traces API requests for flow data

### 2. Service Graph Generation

OpenTelemetry spans include attributes that enable service graph generation:

- Source and destination service identification
- Protocol and port information
- Connection statistics
- Service relationship mapping

### 3. Enhanced Metrics

Beyond the existing Prometheus metrics, OpenTelemetry provides:

- Request latency distribution
- Error rate tracking
- Resource utilization
- Custom business metrics

### 4. Structured Logging

Log entries are correlated with traces for better debugging:

- Trace and span IDs in log entries
- Contextual flow information
- Error correlation across components

## Span Operations

The following span operations are instrumented:

### Flow Collector
- `goldmane.flow.receive`: Receiving flows from cluster nodes
- Attributes: client node, flow metadata, service graph info

### Flow Aggregator
- `goldmane.flow.aggregate`: Aggregating flows across time buckets
- Attributes: bucket timestamp, flow count, aggregation window

### Flow Emitter
- `goldmane.flow.emit`: Emitting aggregated flows to external endpoints
- Attributes: emission URL, bucket metadata, success/failure status

### Flow API
- `goldmane.flow.query`: Handling API requests for flow data
- `goldmane.flow.stream`: Streaming flow updates to clients
- Attributes: query parameters, result count, operation type

## Deployment

### OpenTelemetry Collector

Deploy an OpenTelemetry collector in your cluster to receive telemetry data:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: calico-system
data:
  otel-config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    
    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
      memory_limiter:
        limit_mib: 512
    
    exporters:
      # Configure your preferred backend (Jaeger, Zipkin, etc.)
      jaeger:
        endpoint: jaeger-collector:14250
        tls:
          insecure: true
      
      # Or export to a observability platform
      otlp:
        endpoint: https://api.honeycomb.io
        headers:
          "x-honeycomb-team": "your-api-key"
    
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [jaeger]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [jaeger]
```

### Goldmane Configuration

Update your Goldmane deployment to include OpenTelemetry configuration:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goldmane
  namespace: calico-system
spec:
  template:
    spec:
      containers:
      - name: goldmane
        image: calico/goldmane:latest
        env:
        - name: OTEL_ENABLED
          value: "true"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4317"
        - name: OTEL_SERVICE_NAME
          value: "goldmane"
        - name: OTEL_SERVICE_NAMESPACE
          value: "calico-system"
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: CLUSTER_NAME
          value: "my-cluster"
        - name: OTEL_SAMPLING_RATE
          value: "0.1"
```

## Observability Benefits

### 1. Service Maps
- Visualize service-to-service communication
- Identify bottlenecks and dependencies
- Track service health and performance

### 2. Distributed Tracing
- Follow requests across component boundaries
- Identify slow operations and errors
- Understand system behavior under load

### 3. Performance Monitoring
- Track flow processing latency
- Monitor aggregation performance
- Identify resource constraints

### 4. Debugging and Troubleshooting
- Correlate logs with traces
- Understand error propagation
- Identify root causes quickly

## Best Practices

### 1. Sampling
- Use appropriate sampling rates to balance observability with performance
- Higher sampling in development, lower in production
- Consider adaptive sampling based on system load

### 2. Attribute Management
- Use semantic conventions for consistent attribute naming
- Avoid high-cardinality attributes that can cause performance issues
- Include relevant business context in spans

### 3. Error Handling
- Always record errors in spans
- Include error context and correlation IDs
- Use appropriate span status codes

### 4. Resource Management
- Monitor OpenTelemetry overhead
- Configure appropriate batch sizes and timeouts
- Use memory limiters to prevent resource exhaustion

## Migration Guide

If you're upgrading from a previous version of Goldmane:

1. **No Breaking Changes**: OpenTelemetry is optional and disabled by default
2. **Existing Metrics**: All existing Prometheus metrics continue to work
3. **Gradual Rollout**: Enable OpenTelemetry gradually across your deployment
4. **Monitoring**: Monitor OpenTelemetry overhead during initial deployment

## Troubleshooting

### Common Issues

1. **High Resource Usage**
   - Reduce sampling rate
   - Increase batch sizes
   - Configure memory limiters

2. **Missing Traces**
   - Check OpenTelemetry collector connectivity
   - Verify endpoint configuration
   - Review sampling configuration

3. **Incomplete Service Graph**
   - Ensure all services are instrumented
   - Check attribute extraction logic
   - Verify service name consistency

### Debug Configuration

Enable debug logging for OpenTelemetry:

```bash
OTEL_LOG_LEVEL=debug
```

Check collector health:

```bash
curl http://otel-collector:13133/
```

## Future Enhancements

- **Metrics Integration**: Direct OTLP metrics export
- **Log Correlation**: Automatic log-trace correlation
- **Custom Dashboards**: Pre-built Grafana dashboards
- **Alerting**: OpenTelemetry-based alerting rules
- **Profiling**: Continuous profiling integration
