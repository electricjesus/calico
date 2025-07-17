#!/bin/bash

# Example script to deploy Goldmane with OpenTelemetry integration

set -e

NAMESPACE="calico-system"
CLUSTER_NAME="my-cluster"
OTEL_ENDPOINT="http://otel-collector.${NAMESPACE}.svc.cluster.local:4317"

echo "Deploying OpenTelemetry Collector..."
kubectl apply -f manifests/otel-collector.yaml

echo "Waiting for OpenTelemetry Collector to be ready..."
kubectl wait --for=condition=available deployment/otel-collector -n ${NAMESPACE} --timeout=300s

echo "Updating Goldmane deployment with OpenTelemetry configuration..."

# Update the existing Goldmane deployment with OpenTelemetry environment variables
kubectl patch deployment goldmane -n ${NAMESPACE} --patch "
spec:
  template:
    spec:
      containers:
      - name: goldmane
        env:
        - name: OTEL_ENABLED
          value: 'true'
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: '${OTEL_ENDPOINT}'
        - name: OTEL_SERVICE_NAME
          value: 'goldmane'
        - name: OTEL_SERVICE_NAMESPACE
          value: '${NAMESPACE}'
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: CLUSTER_NAME
          value: '${CLUSTER_NAME}'
        - name: OTEL_SAMPLING_RATE
          value: '0.1'
"

echo "Waiting for Goldmane to be ready..."
kubectl wait --for=condition=available deployment/goldmane -n ${NAMESPACE} --timeout=300s

echo "Deployment complete!"
echo ""
echo "OpenTelemetry Collector endpoints:"
echo "  - OTLP gRPC: ${OTEL_ENDPOINT}"
echo "  - OTLP HTTP: http://otel-collector.${NAMESPACE}.svc.cluster.local:4318"
echo "  - Prometheus metrics: http://otel-collector.${NAMESPACE}.svc.cluster.local:8889/metrics"
echo ""
echo "To view traces, connect to your configured tracing backend (Jaeger, Zipkin, etc.)"
echo "To view service graph metrics, check the Prometheus endpoint above"

# Optional: Set up port forwarding for local access
read -p "Set up port forwarding for local access? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Setting up port forwarding..."
    echo "  - Jaeger UI: http://localhost:16686"
    echo "  - OpenTelemetry Collector: http://localhost:4317"
    echo "  - Prometheus metrics: http://localhost:8889"
    
    kubectl port-forward -n ${NAMESPACE} svc/jaeger-query 16686:16686 &
    kubectl port-forward -n ${NAMESPACE} svc/otel-collector 4317:4317 &
    kubectl port-forward -n ${NAMESPACE} svc/otel-collector 8889:8889 &
    
    echo "Port forwarding active. Press Ctrl+C to stop."
    wait
fi
